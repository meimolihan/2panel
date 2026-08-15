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
	snap, err := os.CreateTemp("", "2panel-backup-*.db")
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
// in-place (so the running session keeps working), log/task files are written
// to disk, and the scheduler is re-registered from the restored database.
func RestoreZip(zipBytes []byte) error {
	r, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	if err != nil {
		return fmt.Errorf("无效的备份文件：不是有效的 zip 压缩包")
	}

	var dbFile *zip.File
	for _, f := range r.File {
		if filepath.Base(f.Name) == backupDBName {
			dbFile = f
			break
		}
	}
	if dbFile == nil {
		return fmt.Errorf("备份文件中未找到 %s，可能不是 2Panel 备份", backupDBName)
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

	cronjobs, records, settings, scripts, scriptRecords, groups, watches, watchRecords, err := readBackupDB(dbFile)
	if err != nil {
		return err
	}

	dataDir := scheduler.GetRunner().DataDir()

	// safety net: keep a copy of the current database before overwriting. The
	// copy lives next to the data dir (never inside it) so a trailing slash on
	// -data cannot leave it in the data dir where a later backup would pack it.
	base := strings.TrimRight(dataDir, "/")
	pruneOldRestoreBackups(base)
	if err := copyFile(filepath.Join(dataDir, backupDBName), base+".pre-restore-"+time.Now().Format("20060102-150405")+".db"); err != nil {
		return fmt.Errorf("备份当前数据库失败: %v", err)
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
		return fmt.Errorf("写入还原数据失败: %v", err)
	}

	// Clear stale log/ and task/ files (created after the backup) so a restore
	// leaves the same state as the CLI one, which swaps the whole directory.
	if err := clearDataFiles(dataDir); err != nil {
		return fmt.Errorf("清理旧数据文件失败: %v", err)
	}

	// restore log/ and task/ files (skip the db, already handled above)
	if err := extractDataFiles(r, dataDir); err != nil {
		return fmt.Errorf("还原脚本/日志文件失败: %v", err)
	}

	// re-register schedulers from the restored database
	RestoreCronjobs()
	RestoreScriptRecords()
	RestoreFilewatches()
	return nil
}

// pruneOldRestoreBackups keeps only the two most recent pre-restore database
// snapshots, so repeated restores cannot accumulate unbounded copies next to
// the data directory.
func pruneOldRestoreBackups(base string) {
	pattern := filepath.Base(base) + ".pre-restore-*.db"
	matches, err := filepath.Glob(filepath.Join(filepath.Dir(base), pattern))
	if err != nil {
		return
	}
	sort.Sort(sort.Reverse(sort.StringSlice(matches)))
	if len(matches) <= 2 {
		return
	}
	for _, m := range matches[2:] {
		_ = os.Remove(m)
	}
}

// clearDataFiles removes and recreates the log/ and task/ directories so no
// stale files survive a restore.
func clearDataFiles(dataDir string) error {
	for _, sub := range []string{"log", "task"} {
		dir := filepath.Join(dataDir, sub)
		if err := os.RemoveAll(dir); err != nil {
			return err
		}
		if err := os.MkdirAll(dir, 0700); err != nil {
			return err
		}
	}
	return nil
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

// extractDataFiles writes log/ and task/ entries from the archive into the
// data dir, guarding against zip-slip.
func extractDataFiles(r *zip.Reader, dataDir string) error {
	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}
		name := filepath.Clean(filepath.FromSlash(f.Name))
		if name == backupDBName {
			continue
		}
		if !strings.HasPrefix(name, "log"+string(os.PathSeparator)) &&
			!strings.HasPrefix(name, "task"+string(os.PathSeparator)) {
			continue
		}
		if name == ".." || strings.HasPrefix(name, ".."+string(os.PathSeparator)) || filepath.IsAbs(name) {
			return fmt.Errorf("备份文件包含非法路径: %s", f.Name)
		}
		target := filepath.Join(dataDir, name)
		if !strings.HasPrefix(target, dataDir) {
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

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}
