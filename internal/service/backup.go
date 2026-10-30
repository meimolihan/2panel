package service

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/2panel-dev/2panel/internal/database"
	"github.com/2panel-dev/2panel/internal/model"
	"github.com/2panel-dev/2panel/internal/scheduler"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const backupDBName = "2panel.db"

// PrepareBackupSnapshot writes a consistent copy of the live database to a
// temporary file and returns its path. The caller owns the file and must
// remove it afterwards. Generating the snapshot up front lets the HTTP handler
// return a clean error before any zip bytes are written.
func PrepareBackupSnapshot() (string, error) {
	dataDir := scheduler.GetRunner().DataDir()
	snap, err := os.CreateTemp("", "fn-cronjob-backup-*.db")
	if err != nil {
		return "", err
	}
	snap.Close()
	if err := database.SnapshotDB(filepath.Join(dataDir, backupDBName), snap.Name()); err != nil {
		os.Remove(snap.Name())
		return "", err
	}
	return snap.Name(), nil
}

// WriteBackupZip packs the data directory (db + log/ + task/) into w as a zip
// archive, sharing the layout of the `2panel backup` CLI command so the two
// are interchangeable. The database entry is served from the snapshot file.
func WriteBackupZip(w io.Writer, snapPath string) error {
	dataDir := scheduler.GetRunner().DataDir()
	zw := zip.NewWriter(w)
	defer zw.Close()

	return filepath.Walk(dataDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dataDir, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		// Skip live WAL/SHM/journal companions: the snapshot is authoritative
		// and restoring a stale -wal could replay outdated transactions.
		if strings.HasPrefix(rel, backupDBName+"-") {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if info.IsDir() {
			hdr := &zip.FileHeader{Name: rel + "/", Method: zip.Deflate}
			hdr.SetMode(0755)
			_, err := zw.CreateHeader(hdr)
			return err
		}
		hdr, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		hdr.Name = rel
		hdr.Method = zip.Deflate
		hdr.SetMode(info.Mode())
		f, err := zw.CreateHeader(hdr)
		if err != nil {
			return err
		}
		src := path
		if rel == backupDBName {
			src = snapPath
		}
		srcFile, err := os.Open(src)
		if err != nil {
			return err
		}
		_, cErr := io.Copy(f, srcFile)
		srcFile.Close()
		return cErr
	})
}

// RestoreZip replaces the current data with the contents of a backup produced
// by WriteBackupZip or `2panel backup`. The database content is swapped
// in-place (so the running session keeps working), every non-database data
// file is written to disk, and the scheduler is re-registered from the
// restored database.
func RestoreZip(zipBytes []byte) error {
	r, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	if err != nil {
		return fmt.Errorf("无效的备份文件：不是有效的 zip 压缩包")
	}

	dbFile := findRootBackupDB(r)
	if dbFile == nil {
		return fmt.Errorf("备份文件根目录中未找到 %s，可能不是 2Panel 备份", backupDBName)
	}

	cronjobs, records, settings, scripts, scriptRecords, groups, watches, watchRecords, err := readBackupDB(dbFile)
	if err != nil {
		return err
	}

	dataDir := scheduler.GetRunner().DataDir()

	// Stage every non-database file before touching the current data, so a
	// truncated/corrupt archive cannot destroy the existing log/task files.
	base := strings.TrimRight(dataDir, "/")
	staging, err := os.MkdirTemp(filepath.Dir(base), filepath.Base(base)+".restore-staging-")
	if err != nil {
		return fmt.Errorf("创建还原临时目录失败: %v", err)
	}
	defer os.RemoveAll(staging)
	if err := extractDataFiles(r, staging); err != nil {
		return fmt.Errorf("解压备份文件失败: %v", err)
	}

	// Stop every scheduled entry BEFORE touching the database so a cron tick
	// cannot fire mid-swap and interleave writes with the restore transaction.
	scheduler.GetRunner().ResetEntries()
	// Stop every inotify watcher for the same reason: a file event could start
	// a run while the database is being swapped underneath it.
	scheduler.GetRunner().StopAllFileWatches()

	if err := ensureIdle(); err != nil {
		return err
	}

	// safety net: keep a copy of the current database before overwriting. The
	// copy lives next to the data dir (never inside it) so a trailing slash on
	// -data cannot leave it in the data dir where a later backup would pack it.
	pruneOldRestoreBackups(base)
	// Use the same consistent snapshot path as regular backups; a raw file copy
	// would miss committed transactions still sitting in the live -wal file.
	if err := database.SnapshotDB(filepath.Join(dataDir, backupDBName), base+".pre-restore-"+time.Now().Format("20060102-150405")+".db"); err != nil {
		return fmt.Errorf("备份当前数据库失败: %v", err)
	}

	// Move the current non-database files aside, then move the staged files in.
	// Doing file operations before the DB swap means a failure here can restore
	// the old files while the database is still untouched.
	fileBackup, err := os.MkdirTemp(filepath.Dir(base), filepath.Base(base)+".pre-restore-files-")
	if err != nil {
		return fmt.Errorf("创建还原回滚目录失败: %v", err)
	}
	defer os.RemoveAll(fileBackup)
	if err := moveDataEntries(dataDir, fileBackup); err != nil {
		_ = moveDataEntries(fileBackup, dataDir)
		return fmt.Errorf("暂存现有数据文件失败: %v", err)
	}
	if err := moveDataEntries(staging, dataDir); err != nil {
		_ = removeDataEntries(dataDir)
		_ = moveDataEntries(fileBackup, dataDir)
		return fmt.Errorf("写入还原文件失败: %v", err)
	}

	// swap data in a single transaction
	err = database.DB.Transaction(func(tx *gorm.DB) error {
		for _, m := range []interface{}{&model.Cronjob{}, &model.JobRecord{}, &model.Setting{}, &model.ScriptLibrary{}, &model.ScriptRecord{}, &model.Group{}, &model.FileWatch{}, &model.FileWatchRecord{}} {
			if err := tx.Where("1 = 1").Delete(m).Error; err != nil {
				return err
			}
		}
		if len(cronjobs) > 0 {
			if err := tx.Create(&cronjobs).Error; err != nil {
				return err
			}
		}
		if len(scripts) > 0 {
			if err := tx.Create(&scripts).Error; err != nil {
				return err
			}
		}
		if len(settings) > 0 {
			if err := tx.Create(&settings).Error; err != nil {
				return err
			}
		}
		if len(records) > 0 {
			if err := tx.Create(&records).Error; err != nil {
				return err
			}
		}
		if len(scriptRecords) > 0 {
			if err := tx.Create(&scriptRecords).Error; err != nil {
				return err
			}
		}
		if len(groups) > 0 {
			if err := tx.Create(&groups).Error; err != nil {
				return err
			}
		}
		if len(watches) > 0 {
			if err := tx.Create(&watches).Error; err != nil {
				return err
			}
		}
		if len(watchRecords) > 0 {
			if err := tx.Create(&watchRecords).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		// The transaction rolled back, so the old database is still intact.
		// Put the old data files back to keep DB and files consistent.
		_ = removeDataEntries(dataDir)
		_ = moveDataEntries(fileBackup, dataDir)
		return fmt.Errorf("写入还原数据失败: %v", err)
	}

	// restored data files are now in place and the DB swap committed
	_ = os.RemoveAll(fileBackup)

	// reload Bark notification config from the restored settings table
	InitNotify()

	// re-register schedulers from the restored database
	RestoreCronjobs()
	RestoreScriptRecords()
	RestoreFilewatches()
	return nil
}

// findRootBackupDB returns the database entry only when it sits at the archive
// root, matching the layout produced by WriteBackupZip and `2panel backup`.
func findRootBackupDB(r *zip.Reader) *zip.File {
	for _, f := range r.File {
		if filepath.Clean(filepath.FromSlash(f.Name)) == backupDBName {
			return f
		}
	}
	return nil
}

// pruneOldRestoreBackups keeps only the two most recent pre-restore database
// snapshots, file rollback dirs and staging dirs, so repeated restores or a
// crash cannot accumulate unbounded copies next to the data directory.
func pruneOldRestoreBackups(base string) {
	patterns := []string{
		filepath.Base(base) + ".pre-restore-*.db",
		filepath.Base(base) + ".pre-restore-files-*",
		filepath.Base(base) + ".restore-staging-*",
	}
	for _, pattern := range patterns {
		matches, err := filepath.Glob(filepath.Join(filepath.Dir(base), pattern))
		if err != nil {
			continue
		}
		sort.Sort(sort.Reverse(sort.StringSlice(matches)))
		if len(matches) <= 2 {
			continue
		}
		for _, m := range matches[2:] {
			_ = os.RemoveAll(m)
		}
	}
}

// isDBEntry reports whether a top-level data-dir name is the live database or
// one of its WAL/SHM/journal companions.
func isDBEntry(name string) bool {
	return name == backupDBName || strings.HasPrefix(name, backupDBName+"-")
}

// moveDataEntries renames every non-database top-level entry from src to dst.
// The caller is responsible for keeping both paths on the same filesystem.
func moveDataEntries(src, dst string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if isDBEntry(entry.Name()) {
			continue
		}
		if err := os.Rename(filepath.Join(src, entry.Name()), filepath.Join(dst, entry.Name())); err != nil {
			return err
		}
	}
	return nil
}

// removeDataEntries deletes every non-database top-level entry in dir.
func removeDataEntries(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if isDBEntry(entry.Name()) {
			continue
		}
		if err := os.RemoveAll(filepath.Join(dir, entry.Name())); err != nil {
			return err
		}
	}
	return nil
}

// pathWithin reports whether target stays inside base after cleaning.
func pathWithin(base, target string) bool {
	rel, err := filepath.Rel(base, target)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)) && !filepath.IsAbs(rel)
}

// ensureIdle rejects a restore while any task/script is executing or waiting,
// since replacing records mid-run would corrupt the running execution.
func ensureIdle() error {
	var count int64
	if err := database.DB.Model(&model.Cronjob{}).Where("is_executing = ?", true).Count(&count).Error; err == nil && count > 0 {
		return fmt.Errorf("有任务正在执行，请等待完成或先停止任务后再还原")
	}
	if err := database.DB.Model(&model.JobRecord{}).Where("status = ?", model.StatusWaiting).Count(&count).Error; err == nil && count > 0 {
		return fmt.Errorf("有待执行的任务，请稍后再还原")
	}
	if err := database.DB.Model(&model.ScriptRecord{}).Where("status IN ?", []string{model.StatusRunning, model.StatusWaiting}).Count(&count).Error; err == nil && count > 0 {
		return fmt.Errorf("有脚本正在执行，请稍后再还原")
	}
	return ensureFileWatchesIdle()
}

// readBackupDB extracts 2panel.db from the archive into a temp file and reads
// every row into memory.
func readBackupDB(dbFile *zip.File) ([]model.Cronjob, []model.JobRecord, []model.Setting, []model.ScriptLibrary, []model.ScriptRecord, []model.Group, []model.FileWatch, []model.FileWatchRecord, error) {
	tmp, err := os.CreateTemp("", "2panel-restore-*.db")
	if err != nil {
		return nil, nil, nil, nil, nil, nil, nil, nil, err
	}
	defer os.Remove(tmp.Name())

	rc, err := dbFile.Open()
	if err != nil {
		tmp.Close()
		return nil, nil, nil, nil, nil, nil, nil, nil, err
	}
	if _, err := io.Copy(tmp, rc); err != nil {
		rc.Close()
		tmp.Close()
		return nil, nil, nil, nil, nil, nil, nil, nil, err
	}
	rc.Close()
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return nil, nil, nil, nil, nil, nil, nil, nil, err
	}
	tmp.Close()

	bdb, err := gorm.Open(sqlite.Open(tmp.Name()), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		return nil, nil, nil, nil, nil, nil, nil, nil, fmt.Errorf("读取备份数据库失败: %v", err)
	}
	defer func() {
		if sqlDB, e := bdb.DB(); e == nil {
			sqlDB.Close()
		}
	}()

	var cronjobs []model.Cronjob
	var records []model.JobRecord
	var settings []model.Setting
	var scripts []model.ScriptLibrary
	var scriptRecords []model.ScriptRecord
	var groups []model.Group
	var watches []model.FileWatch
	var watchRecords []model.FileWatchRecord
	// Backups taken before a table existed (e.g. file_watches, added later)
	// simply lack that table; skip it so older archives still restore.
	loadTable := func(m interface{}, dst interface{}) error {
		if !bdb.Migrator().HasTable(m) {
			return nil
		}
		return bdb.Find(dst).Error
	}
	if err := loadTable(&model.Cronjob{}, &cronjobs); err != nil {
		return nil, nil, nil, nil, nil, nil, nil, nil, err
	}
	if err := loadTable(&model.JobRecord{}, &records); err != nil {
		return nil, nil, nil, nil, nil, nil, nil, nil, err
	}
	if err := loadTable(&model.Setting{}, &settings); err != nil {
		return nil, nil, nil, nil, nil, nil, nil, nil, err
	}
	if err := loadTable(&model.ScriptLibrary{}, &scripts); err != nil {
		return nil, nil, nil, nil, nil, nil, nil, nil, err
	}
	if err := loadTable(&model.ScriptRecord{}, &scriptRecords); err != nil {
		return nil, nil, nil, nil, nil, nil, nil, nil, err
	}
	if err := loadTable(&model.Group{}, &groups); err != nil {
		return nil, nil, nil, nil, nil, nil, nil, nil, err
	}
	if err := loadTable(&model.FileWatch{}, &watches); err != nil {
		return nil, nil, nil, nil, nil, nil, nil, nil, err
	}
	if err := loadTable(&model.FileWatchRecord{}, &watchRecords); err != nil {
		return nil, nil, nil, nil, nil, nil, nil, nil, err
	}
	return cronjobs, records, settings, scripts, scriptRecords, groups, watches, watchRecords, nil
}

// extractDataFiles writes every non-database entry from the archive into dest,
// preserving the archive's relative layout and guarding against zip-slip. The
// database itself is restored separately through the live DB connection.
func extractDataFiles(r *zip.Reader, dest string) error {
	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}
		name := filepath.Clean(filepath.FromSlash(f.Name))
		if name == "." || name == ".." ||
			strings.HasPrefix(name, ".."+string(os.PathSeparator)) ||
			filepath.IsAbs(name) {
			return fmt.Errorf("备份文件包含非法路径: %s", f.Name)
		}
		baseName := filepath.Base(name)
		if baseName == backupDBName || strings.HasPrefix(baseName, backupDBName+"-") {
			continue
		}
		target := filepath.Join(dest, name)
		if !pathWithin(dest, target) {
			return fmt.Errorf("备份文件路径越界: %s", f.Name)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		mode := f.Mode() & 0777
		if mode == 0 {
			mode = 0644
		}
		dst, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
		if err != nil {
			rc.Close()
			return err
		}
		_, cErr := io.Copy(dst, rc)
		rc.Close()
		dst.Close()
		if cErr != nil {
			return cErr
		}
	}
	return nil
}
