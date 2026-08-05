package database

import (
	"fmt"

	"github.com/2panel-dev/2panel/internal/model"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

func Init(dbPath string) error {
	var err error
	DB, err = gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return fmt.Errorf("open database failed: %v", err)
	}
	if err := DB.AutoMigrate(&model.Cronjob{}, &model.JobRecord{}, &model.Setting{}, &model.ScriptLibrary{}); err != nil {
		return fmt.Errorf("migrate database failed: %v", err)
	}
	return nil
}
