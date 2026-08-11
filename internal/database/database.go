package database

import (
	"database/sql"
	"fmt"
	"os"
	"strings"

	"github.com/2panel-dev/2panel/internal/model"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

func Init(dbPath string) error {
	dsn := dbPath +
		"?_pragma=journal_mode(WAL)" +
		"&_pragma=busy_timeout(5000)" +
		"&_pragma=synchronous(NORMAL)" +
		"&_pragma=foreign_keys(1)" +
		"&_pragma=temp_store(MEMORY)" +
		"&_pragma=mmap_size(268435456)" +
		"&_pragma=wal_autocheckpoint(1000)"
	var err error
	DB, err = gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return fmt.Errorf("open database failed: %v", err)
	}
	sqlDB, err := DB.DB()
	if err != nil {
		return err
	}
	// SQLite serializes writers at the file level; a single pooled connection
	// makes concurrent cron goroutines and HTTP handlers queue instead of
	// hitting SQLITE_BUSY.
	sqlDB.SetMaxOpenConns(1)
	if err := sqlDB.Ping(); err != nil {
		return err
	}
	if err := os.Chmod(dbPath, 0600); err != nil {
		return fmt.Errorf("chmod database failed: %v", err)
	}
	if err := DB.AutoMigrate(&model.Cronjob{}, &model.JobRecord{}, &model.Setting{}, &model.ScriptLibrary{}, &model.ScriptRecord{}, &model.Group{}); err != nil {
		return fmt.Errorf("migrate database failed: %v", err)
	}
	return nil
}

// Close releases the pooled database handle. Mainly used by tests.
func Close() error {
	if DB == nil {
		return nil
	}
	sqlDB, err := DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// SnapshotDB writes a consistent point-in-time copy of the SQLite database at
// src into dest via VACUUM INTO, so backups taken while the server is running
// are never torn (WAL or not).
func SnapshotDB(src, dest string) error {
	db, err := sql.Open("sqlite", src+"?_pragma=busy_timeout(10000)")
	if err != nil {
		return fmt.Errorf("open database failed: %v", err)
	}
	defer db.Close()
	if err := os.Remove(dest); err != nil && !os.IsNotExist(err) {
		return err
	}
	escaped := strings.ReplaceAll(dest, "'", "''")
	if _, err := db.Exec("VACUUM INTO '" + escaped + "'"); err != nil {
		return fmt.Errorf("create snapshot failed: %v", err)
	}
	return os.Chmod(dest, 0600)
}
