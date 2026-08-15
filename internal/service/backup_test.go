package service

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"

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