package repo

import (
	"github.com/2panel-dev/2panel/internal/database"
	"github.com/2panel-dev/2panel/internal/model"
)

type SettingRepo struct{}

func (u *SettingRepo) Get(key string) (model.Setting, error) {
	var setting model.Setting
	err := database.DB.First(&setting, "key = ?", key).Error
	return setting, err
}

func (u *SettingRepo) Set(key, value string) error {
	setting := model.Setting{Key: key, Value: value}
	return database.DB.Where("key = ?", key).Save(&setting).Error
}
