package service

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/2panel-dev/2panel/internal/database"
	"github.com/2panel-dev/2panel/internal/dto"
	"github.com/2panel-dev/2panel/internal/notify"
	"github.com/2panel-dev/2panel/internal/repo"
	"github.com/2panel-dev/2panel/internal/scheduler"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// oldSchemaDB builds a sqlite database with the schema a 2Panel backup had
// before the filewatch feature was added (no file_watches / file_watch_records
// tables) and a couple of rows, so historical archives keep restoring.
func oldSchemaDB(t *testing.T) *os.File {
	t.Helper()
	f, err := os.CreateTemp("", "2panel-old-schema-*.db")
	if err != nil {
		t.Fatalf("create temp db: %v", err)
	}
	t.Cleanup(func() { os.Remove(f.Name()) })

	db, err := gorm.Open(sqlite.Open(f.Name()), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open temp db: %v", err)
	}
	schema := []string{
		`CREATE TABLE "cronjobs" ("id" integer PRIMARY KEY AUTOINCREMENT,"created_at" datetime,"updated_at" datetime,"name" text NOT NULL,"type" text NOT NULL,"spec" text NOT NULL,"spec_custom" numeric,"executor" text,"script" text,"script_name" text,"user" text,"url" text,"retry_times" integer,"timeout" integer,"retain_copies" integer,"is_executing" numeric,"status" text,"entry_ids" text);`,
		`CREATE TABLE "job_records" ("id" integer PRIMARY KEY AUTOINCREMENT,"created_at" datetime,"updated_at" datetime,"cronjob_id" integer,"task_id" text,"start_time" datetime,"interval" real,"records" text,"status" text,"message" text);`,
		`CREATE TABLE "settings" ("key" text,"value" text,PRIMARY KEY ("key"));`,
		`CREATE TABLE "script_libraries" ("id" integer PRIMARY KEY AUTOINCREMENT,"created_at" datetime,"updated_at" datetime,"name" text NOT NULL,"description" text,"script" text,"groups" text);`,
		`CREATE TABLE "script_records" ("id" integer PRIMARY KEY AUTOINCREMENT,"created_at" datetime,"updated_at" datetime,"task_id" text,"script_id" integer,"script_name" text,"start_time" datetime,"interval" real,"records" text,"status" text,"message" text);`,
		`CREATE TABLE "groups" ("id" integer PRIMARY KEY AUTOINCREMENT,"created_at" datetime,"updated_at" datetime,"is_default" numeric DEFAULT false,"name" text NOT NULL,"type" text NOT NULL);`,
	}
	for _, s := range schema {
		if err := db.Exec(s).Error; err != nil {
			t.Fatalf("create table: %v", err)
		}
	}
	if err := db.Exec(`INSERT INTO cronjobs (created_at,updated_at,name,type,spec,executor,script,timeout,retain_copies,is_executing,status) VALUES (datetime('now'),datetime('now'),'历史任务','shell','0 * * * *','sh','echo hi',60,10,0,'enabled')`).Error; err != nil {
		t.Fatalf("seed cronjob: %v", err)
	}
	if err := db.Exec(`INSERT INTO settings (key,value) VALUES ('siteName','历史站点')`).Error; err != nil {
		t.Fatalf("seed setting: %v", err)
	}
	if sqlDB, e := db.DB(); e == nil {
		sqlDB.Close()
	}
	return f
}

func backupZipWithDB(t *testing.T, dbPath string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	raw, err := os.ReadFile(dbPath)
	if err != nil {
		t.Fatalf("read db: %v", err)
	}
	w, err := zw.Create(filepath.ToSlash(backupDBName))
	if err != nil {
		t.Fatalf("zip create: %v", err)
	}
	if _, err := w.Write(raw); err != nil {
		t.Fatalf("zip write: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("zip close: %v", err)
	}
	return buf.Bytes()
}

// TestReadBackupDBWithoutFilewatchTables guards the historical-backup
// compatibility: a backup taken before the filewatch feature must still be
// restorable, i.e. readBackupDB must not fail on the missing tables and must
// report empty filewatch data.
func TestReadBackupDBWithoutFilewatchTables(t *testing.T) {
	dbFile := oldSchemaDB(t)
	zipBytes := backupZipWithDB(t, dbFile.Name())

	r, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}
	var entry *zip.File
	for _, f := range r.File {
		if f.Name == backupDBName {
			entry = f
			break
		}
	}
	if entry == nil {
		t.Fatal("2panel.db missing from backup")
	}

	cronjobs, _, settings, _, _, _, watches, watchRecords, err := readBackupDB(entry)
	if err != nil {
		t.Fatalf("readBackupDB on pre-filewatch backup failed: %v", err)
	}
	if len(cronjobs) != 1 {
		t.Errorf("cronjobs = %d, want 1", len(cronjobs))
	}
	if len(settings) != 1 {
		t.Errorf("settings = %d, want 1", len(settings))
	}
	if len(watches) != 0 {
		t.Errorf("watches = %d, want 0", len(watches))
	}
	if len(watchRecords) != 0 {
		t.Errorf("watchRecords = %d, want 0", len(watchRecords))
	}
}
func zipBytes(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("zip create %s: %v", name, err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatalf("zip write %s: %v", name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("zip close: %v", err)
	}
	return buf.Bytes()
}

func TestFindRootBackupDB(t *testing.T) {
	nested := zipBytes(t, map[string]string{"sub/2panel.db": "x"})
	r, err := zip.NewReader(bytes.NewReader(nested), int64(len(nested)))
	if err != nil {
		t.Fatalf("open nested zip: %v", err)
	}
	if got := findRootBackupDB(r); got != nil {
		t.Fatalf("nested 2panel.db should not be accepted, got %q", got.Name)
	}

	root := zipBytes(t, map[string]string{backupDBName: "x", "log/a.log": "a"})
	r, err = zip.NewReader(bytes.NewReader(root), int64(len(root)))
	if err != nil {
		t.Fatalf("open root zip: %v", err)
	}
	got := findRootBackupDB(r)
	if got == nil || got.Name != backupDBName {
		t.Fatalf("root 2panel.db not found: %+v", got)
	}
}

func TestExtractDataFilesRestoresAllNonDBEntries(t *testing.T) {
	dest := t.TempDir()
	raw := zipBytes(t, map[string]string{
		backupDBName:                   "db",
		backupDBName + "-wal":          "wal",
		"log/run.log":                  "log",
		"task/job/run.sh":              "script",
		"custom/config.json":           `{"v":1}`,
		"sub/2panel.db":                "nested db",
		"sub/" + backupDBName + "-wal": "nested wal",
	})
	r, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}
	if err := extractDataFiles(r, dest); err != nil {
		t.Fatalf("extractDataFiles: %v", err)
	}

	for name, want := range map[string]string{
		"log/run.log":        "log",
		"task/job/run.sh":    "script",
		"custom/config.json": `{"v":1}`,
	} {
		got, err := os.ReadFile(filepath.Join(dest, name))
		if err != nil {
			t.Errorf("read %s: %v", name, err)
			continue
		}
		if string(got) != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}
	for _, name := range []string{backupDBName, backupDBName + "-wal", "sub/" + backupDBName} {
		if _, err := os.Stat(filepath.Join(dest, name)); !os.IsNotExist(err) {
			t.Errorf("%s should not be extracted", name)
		}
	}
}

func TestExtractDataFilesRejectsZipSlip(t *testing.T) {
	raw := zipBytes(t, map[string]string{"../evil.txt": "x"})
	r, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}
	if err := extractDataFiles(r, t.TempDir()); err == nil {
		t.Fatal("zip-slip entry should be rejected")
	}
}

func TestMoveAndRemoveDataEntriesKeepsDatabase(t *testing.T) {
	dataDir := t.TempDir()
	backupDir := t.TempDir()
	for name, content := range map[string]string{
		backupDBName:          "db",
		backupDBName + "-wal": "wal",
		"log/a.log":           "log",
		"task/job/run.sh":     "script",
		"custom/config.json":  "config",
	} {
		path := filepath.Join(dataDir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(path, []byte(content), 0600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	if err := moveDataEntries(dataDir, backupDir); err != nil {
		t.Fatalf("move out: %v", err)
	}
	for _, name := range []string{backupDBName, backupDBName + "-wal"} {
		if _, err := os.Stat(filepath.Join(dataDir, name)); err != nil {
			t.Errorf("%s should remain in data dir: %v", name, err)
		}
	}
	for _, name := range []string{"log/a.log", "task/job/run.sh", "custom/config.json"} {
		if _, err := os.Stat(filepath.Join(backupDir, name)); err != nil {
			t.Errorf("%s should be moved aside: %v", name, err)
		}
	}

	if err := moveDataEntries(backupDir, dataDir); err != nil {
		t.Fatalf("move back: %v", err)
	}
	for _, name := range []string{"log/a.log", "task/job/run.sh", "custom/config.json"} {
		if _, err := os.Stat(filepath.Join(dataDir, name)); err != nil {
			t.Errorf("%s should be restored: %v", name, err)
		}
	}

	if err := removeDataEntries(dataDir); err != nil {
		t.Fatalf("remove entries: %v", err)
	}
	for _, name := range []string{backupDBName, backupDBName + "-wal"} {
		if _, err := os.Stat(filepath.Join(dataDir, name)); err != nil {
			t.Errorf("%s should survive cleanup: %v", name, err)
		}
	}
	if _, err := os.Stat(filepath.Join(dataDir, "log")); !os.IsNotExist(err) {
		t.Error("log/ should be removed")
	}
}

// TestRestoreZipRoundTrip exercises the web restore path end-to-end: settings
// (including Bark config), unknown root-level data files, logs and task files
// must all come back, while files created after the backup must be removed.
func TestRestoreZipRoundTrip(t *testing.T) {
	root := t.TempDir()
	dataDir := filepath.Join(root, "data")
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		t.Fatalf("mkdir data dir: %v", err)
	}
	if err := database.Init(filepath.Join(dataDir, backupDBName)); err != nil {
		t.Fatalf("init database: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	scheduler.Init(dataDir)
	t.Cleanup(func() {
		scheduler.GetRunner().ResetEntries()
		scheduler.GetRunner().StopAllFileWatches()
	})

	writeFile := func(name, content string) {
		t.Helper()
		path := filepath.Join(dataDir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatalf("mkdir %s: %v", name, err)
		}
		if err := os.WriteFile(path, []byte(content), 0600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	settingRepo := &repo.SettingRepo{}
	if err := settingRepo.Set("restore_test", "original"); err != nil {
		t.Fatalf("seed setting: %v", err)
	}
	wantBark := dto.BarkConfig{
		Enable:       true,
		Key:          "backup-key",
		Server:       "https://bark.example.com",
		SoundSuccess: "bell",
		SoundError:   "alarm",
		SoundInfo:    "none",
		Timeout:      5,
	}
	if err := (&NotifyService{}).SaveConfig(wantBark); err != nil {
		t.Fatalf("save bark config: %v", err)
	}
	writeFile("log/old.log", "old log")
	writeFile("task/job/run.sh", "echo old")
	writeFile("custom/config.json", `{"v":1}`)

	snap, err := PrepareBackupSnapshot()
	if err != nil {
		t.Fatalf("prepare snapshot: %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(snap) })
	var buf bytes.Buffer
	if err := WriteBackupZip(&buf, snap); err != nil {
		t.Fatalf("write backup: %v", err)
	}
	backup := buf.Bytes()

	// mutate everything after the backup
	if err := settingRepo.Set("restore_test", "mutated"); err != nil {
		t.Fatalf("mutate setting: %v", err)
	}
	if err := (&NotifyService{}).SaveConfig(dto.BarkConfig{Enable: false, Key: "stale", Timeout: 5}); err != nil {
		t.Fatalf("mutate bark: %v", err)
	}
	for _, name := range []string{"log", "task", "custom"} {
		if err := os.RemoveAll(filepath.Join(dataDir, name)); err != nil {
			t.Fatalf("remove %s: %v", name, err)
		}
	}
	writeFile("log/new.log", "new log")
	writeFile("custom/config.json", `{"v":2}`)

	if err := RestoreZip(backup); err != nil {
		t.Fatalf("restore: %v", err)
	}

	setting, err := settingRepo.Get("restore_test")
	if err != nil {
		t.Fatalf("read restored setting: %v", err)
	}
	if setting.Value != "original" {
		t.Errorf("restore_test = %q, want original", setting.Value)
	}
	if got := (&NotifyService{}).LoadConfig(); got != wantBark {
		t.Errorf("bark config = %+v, want %+v", got, wantBark)
	}
	if got := notify.CurrentConfig(); !got.Enable || got.Key != wantBark.Key {
		t.Errorf("runtime bark config not restored: %+v", got)
	}
	for name, want := range map[string]string{
		"log/old.log":        "old log",
		"task/job/run.sh":    "echo old",
		"custom/config.json": `{"v":1}`,
	} {
		got, err := os.ReadFile(filepath.Join(dataDir, name))
		if err != nil {
			t.Errorf("read %s: %v", name, err)
			continue
		}
		if string(got) != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}
	if _, err := os.Stat(filepath.Join(dataDir, "log/new.log")); !os.IsNotExist(err) {
		t.Error("log/new.log created after backup should be removed")
	}
}
