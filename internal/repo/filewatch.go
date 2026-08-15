package repo

import (
	"github.com/2panel-dev/2panel/internal/database"
	"github.com/2panel-dev/2panel/internal/model"
	"gorm.io/gorm"
)

// FileWatchRepo persists file-watch conditional tasks and their execution
// records. Most CRUD mirrors CronjobRepo; records are keyed by watch id.
type FileWatchRepo struct{}

func (u *FileWatchRepo) Get(opts ...DBOption) (model.FileWatch, error) {
	var watch model.FileWatch
	db := database.DB
	for _, opt := range opts {
		db = opt(db)
	}
	err := db.First(&watch).Error
	return watch, err
}

func (u *FileWatchRepo) Page(page, size int, opts ...DBOption) (int64, []model.FileWatch, error) {
	var watches []model.FileWatch
	db := database.DB.Model(&model.FileWatch{})
	for _, opt := range opts {
		db = opt(db)
	}
	var count int64
	if err := db.Count(&count).Error; err != nil {
		return 0, nil, err
	}
	size = normalizePageSize(size)
	if page <= 0 {
		page = 1
	}
	err := db.Order("created_at desc").Limit(size).Offset(size * (page - 1)).Find(&watches).Error
	return count, watches, err
}

func (u *FileWatchRepo) List(opts ...DBOption) ([]model.FileWatch, error) {
	var watches []model.FileWatch
	db := database.DB.Model(&model.FileWatch{})
	for _, opt := range opts {
		db = opt(db)
	}
	err := db.Find(&watches).Error
	return watches, err
}

func (u *FileWatchRepo) Create(watch *model.FileWatch) error {
	return database.DB.Create(watch).Error
}

func (u *FileWatchRepo) Update(id uint, vars map[string]interface{}) error {
	return database.DB.Model(&model.FileWatch{}).Where("id = ?", id).Updates(vars).Error
}

func (u *FileWatchRepo) Delete(opts ...DBOption) error {
	db := database.DB
	for _, opt := range opts {
		db = opt(db)
	}
	return db.Delete(&model.FileWatch{}).Error
}

func (u *FileWatchRepo) PageRecords(page, size int, opts ...DBOption) (int64, []model.FileWatchRecord, error) {
	var records []model.FileWatchRecord
	db := database.DB.Model(&model.FileWatchRecord{})
	for _, opt := range opts {
		db = opt(db)
	}
	var count int64
	if err := db.Count(&count).Error; err != nil {
		return 0, nil, err
	}
	size = normalizePageSize(size)
	if page <= 0 {
		page = 1
	}
	err := db.Order("created_at desc").Limit(size).Offset(size * (page - 1)).Find(&records).Error
	return count, records, err
}

func (u *FileWatchRepo) GetRecord(opts ...DBOption) (model.FileWatchRecord, error) {
	var record model.FileWatchRecord
	db := database.DB
	for _, opt := range opts {
		db = opt(db)
	}
	err := db.First(&record).Error
	return record, err
}

func (u *FileWatchRepo) CreateRecord(record *model.FileWatchRecord) error {
	return database.DB.Create(record).Error
}

func (u *FileWatchRepo) ListRecords(opts ...DBOption) ([]model.FileWatchRecord, error) {
	var records []model.FileWatchRecord
	db := database.DB.Model(&model.FileWatchRecord{})
	for _, opt := range opts {
		db = opt(db)
	}
	err := db.Find(&records).Error
	return records, err
}

// ListRecordsLimit returns at most limit records matching the options, newest
// first, used by the retention pruning so it keeps exactly the latest records
// and never loads the whole history.
func (u *FileWatchRepo) ListRecordsLimit(limit int, opts ...DBOption) ([]model.FileWatchRecord, error) {
	var records []model.FileWatchRecord
	db := database.DB.Model(&model.FileWatchRecord{})
	for _, opt := range opts {
		db = opt(db)
	}
	if limit <= 0 {
		return records, nil
	}
	err := db.Order("id desc").Limit(limit).Find(&records).Error
	return records, err
}

func (u *FileWatchRepo) UpdateRecord(id uint, vars map[string]interface{}) error {
	return database.DB.Model(&model.FileWatchRecord{}).Where("id = ?", id).Updates(vars).Error
}

func (u *FileWatchRepo) DeleteRecords(opts ...DBOption) error {
	db := database.DB
	for _, opt := range opts {
		db = opt(db)
	}
	return db.Delete(&model.FileWatchRecord{}).Error
}

// RecordFirstBatch returns the most recent record per watch ID in one query,
// so the list view avoids an N+1 lookup for "last executed".
func (u *FileWatchRepo) RecordFirstBatch(watchIDs []uint) (map[uint]model.FileWatchRecord, error) {
	result := make(map[uint]model.FileWatchRecord)
	if len(watchIDs) == 0 {
		return result, nil
	}
	var records []model.FileWatchRecord
	err := database.DB.Model(&model.FileWatchRecord{}).
		Where("id IN (SELECT MAX(id) FROM file_watch_records WHERE watch_id IN ? GROUP BY watch_id)", watchIDs).
		Find(&records).Error
	if err != nil {
		return nil, err
	}
	for _, rec := range records {
		result[rec.WatchID] = rec
	}
	return result, nil
}

func WithByWatchID(id uint) DBOption {
	return func(g *gorm.DB) *gorm.DB {
		return g.Where("watch_id = ?", id)
	}
}