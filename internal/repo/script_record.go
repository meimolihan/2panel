package repo

import (
	"github.com/2panel-dev/2panel/internal/database"
	"github.com/2panel-dev/2panel/internal/model"
	"gorm.io/gorm"
)

type ScriptRecordRepo struct{}

func WithByScriptID(id uint) DBOption {
	return func(g *gorm.DB) *gorm.DB {
		return g.Where("script_id = ?", id)
	}
}

func (u *ScriptRecordRepo) Get(opts ...DBOption) (model.ScriptRecord, error) {
	var record model.ScriptRecord
	db := database.DB
	for _, opt := range opts {
		db = opt(db)
	}
	err := db.First(&record).Error
	return record, err
}

func (u *ScriptRecordRepo) Page(page, size int, opts ...DBOption) (int64, []model.ScriptRecord, error) {
	var records []model.ScriptRecord
	db := database.DB.Model(&model.ScriptRecord{})
	for _, opt := range opts {
		db = opt(db)
	}
	var count int64
	if err := db.Count(&count).Error; err != nil {
		return 0, nil, err
	}
	if size <= 0 {
		size = 10
	}
	if page <= 0 {
		page = 1
	}
	err := db.Order("created_at desc").Limit(size).Offset(size * (page - 1)).Find(&records).Error
	return count, records, err
}

func (u *ScriptRecordRepo) Create(record *model.ScriptRecord) error {
	return database.DB.Create(record).Error
}

func (u *ScriptRecordRepo) Update(id uint, vars map[string]interface{}) error {
	return database.DB.Model(&model.ScriptRecord{}).Where("id = ?", id).Updates(vars).Error
}

func (u *ScriptRecordRepo) Delete(opts ...DBOption) error {
	db := database.DB
	for _, opt := range opts {
		db = opt(db)
	}
	return db.Delete(&model.ScriptRecord{}).Error
}
