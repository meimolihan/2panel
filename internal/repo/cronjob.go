package repo

import (
	"errors"
	"time"

	"github.com/2panel-dev/2panel/internal/database"
	"github.com/2panel-dev/2panel/internal/model"
	"gorm.io/gorm"
)

// ErrJobExecuting is returned by StartRecord when the cronjob is already
// running, so a manual trigger and a scheduled tick can never double-run it.
var ErrJobExecuting = errors.New("cronjob is executing")

// maxPageSize caps the number of rows a single page may request.
const maxPageSize = 200

type DBOption func(*gorm.DB) *gorm.DB

func WithByID(id uint) DBOption {
	return func(g *gorm.DB) *gorm.DB {
		return g.Where("id = ?", id)
	}
}

func WithByIDs(ids []uint) DBOption {
	return func(g *gorm.DB) *gorm.DB {
		return g.Where("id IN ?", ids)
	}
}

func WithByName(name string) DBOption {
	return func(g *gorm.DB) *gorm.DB {
		return g.Where("name = ?", name)
	}
}

func WithByType(typ string) DBOption {
	return func(g *gorm.DB) *gorm.DB {
		return g.Where("type = ?", typ)
	}
}

func WithByStatus(status string) DBOption {
	return func(g *gorm.DB) *gorm.DB {
		return g.Where("status = ?", status)
	}
}

func WithIsExecuting(v bool) DBOption {
	return func(g *gorm.DB) *gorm.DB {
		return g.Where("is_executing = ?", v)
	}
}

// WithStartTimeAfter filters job records that started at or after the given
// time, used for per-day dashboard stats.
func WithStartTimeAfter(t time.Time) DBOption {
	return func(g *gorm.DB) *gorm.DB {
		return g.Where("start_time >= ?", t)
	}
}

func WithByLikeName(info string) DBOption {
	return func(g *gorm.DB) *gorm.DB {
		if len(info) == 0 {
			return g
		}
		return g.Where("name LIKE ?", "%"+info+"%")
	}
}

func WithOrderBy(orderBy, order string) DBOption {
	return func(g *gorm.DB) *gorm.DB {
		if len(orderBy) == 0 {
			orderBy = "created_at"
		}
		if len(order) == 0 {
			order = "desc"
		}
		return g.Order(orderBy + " " + order)
	}
}

type CronjobRepo struct{}

func (u *CronjobRepo) Get(opts ...DBOption) (model.Cronjob, error) {
	var cronjob model.Cronjob
	db := database.DB
	for _, opt := range opts {
		db = opt(db)
	}
	err := db.First(&cronjob).Error
	return cronjob, err
}

func (u *CronjobRepo) Page(page, size int, opts ...DBOption) (int64, []model.Cronjob, error) {
	var cronjobs []model.Cronjob
	db := database.DB.Model(&model.Cronjob{})
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
	err := db.Limit(size).Offset(size * (page - 1)).Find(&cronjobs).Error
	return count, cronjobs, err
}

// normalizePageSize clamps the page size into [1, maxPageSize].
func normalizePageSize(size int) int {
	if size <= 0 {
		size = 10
	}
	if size > maxPageSize {
		size = maxPageSize
	}
	return size
}

func (u *CronjobRepo) List(opts ...DBOption) ([]model.Cronjob, error) {
	var cronjobs []model.Cronjob
	db := database.DB.Model(&model.Cronjob{})
	for _, opt := range opts {
		db = opt(db)
	}
	err := db.Find(&cronjobs).Error
	return cronjobs, err
}

// Count returns the number of cronjobs matching the given options.
func (u *CronjobRepo) Count(opts ...DBOption) (int64, error) {
	db := database.DB.Model(&model.Cronjob{})
	for _, opt := range opts {
		db = opt(db)
	}
	var count int64
	err := db.Count(&count).Error
	return count, err
}

func (u *CronjobRepo) Create(cronjob *model.Cronjob) error {
	return database.DB.Create(cronjob).Error
}

func (u *CronjobRepo) Update(id uint, vars map[string]interface{}) error {
	return database.DB.Model(&model.Cronjob{}).Where("id = ?", id).Updates(vars).Error
}

func (u *CronjobRepo) Delete(opts ...DBOption) error {
	db := database.DB
	for _, opt := range opts {
		db = opt(db)
	}
	return db.Delete(&model.Cronjob{}).Error
}

func (u *CronjobRepo) GetRecord(opts ...DBOption) (model.JobRecord, error) {
	var record model.JobRecord
	db := database.DB
	for _, opt := range opts {
		db = opt(db)
	}
	err := db.First(&record).Error
	return record, err
}

func (u *CronjobRepo) RecordFirst(cronjobID uint) (model.JobRecord, error) {
	var record model.JobRecord
	err := database.DB.Where("cronjob_id = ?", cronjobID).Order("created_at desc").First(&record).Error
	return record, err
}

func (u *CronjobRepo) PageRecords(page, size int, opts ...DBOption) (int64, []model.JobRecord, error) {
	var records []model.JobRecord
	db := database.DB.Model(&model.JobRecord{})
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

func (u *CronjobRepo) ListRecords(opts ...DBOption) ([]model.JobRecord, error) {
	var records []model.JobRecord
	db := database.DB.Model(&model.JobRecord{})
	for _, opt := range opts {
		db = opt(db)
	}
	err := db.Find(&records).Error
	return records, err
}

// CountRecords returns the number of job records matching the given options.
func (u *CronjobRepo) CountRecords(opts ...DBOption) (int64, error) {
	db := database.DB.Model(&model.JobRecord{})
	for _, opt := range opts {
		db = opt(db)
	}
	var count int64
	err := db.Count(&count).Error
	return count, err
}

func (u *CronjobRepo) CreateRecord(record *model.JobRecord) error {
	return database.DB.Create(record).Error
}

func (u *CronjobRepo) UpdateRecord(id uint, vars map[string]interface{}) error {
	return database.DB.Model(&model.JobRecord{}).Where("id = ?", id).Updates(vars).Error
}

func (u *CronjobRepo) DeleteRecords(opts ...DBOption) error {
	db := database.DB
	for _, opt := range opts {
		db = opt(db)
	}
	return db.Delete(&model.JobRecord{}).Error
}

func WithByCronjobID(id uint) DBOption {
	return func(g *gorm.DB) *gorm.DB {
		return g.Where("cronjob_id = ?", id)
	}
}

func WithByTaskID(taskID string) DBOption {
	return func(g *gorm.DB) *gorm.DB {
		return g.Where("task_id = ?", taskID)
	}
}

// StartRecord atomically marks the cronjob as executing and creates its
// waiting record. The single UPDATE guards against a manual trigger and a
// scheduled tick both starting the job; the loser gets ErrJobExecuting.
func (u *CronjobRepo) StartRecord(cronjobID uint) (model.JobRecord, error) {
	var record model.JobRecord
	err := database.DB.Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&model.Cronjob{}).
			Where("id = ? AND is_executing = ?", cronjobID, false).
			Update("is_executing", true)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return ErrJobExecuting
		}
		record = model.JobRecord{
			CronjobID: cronjobID,
			TaskID:    newTaskID(),
			StartTime: now(),
			Status:    model.StatusWaiting,
		}
		return tx.Create(&record).Error
	})
	return record, err
}

func (u *CronjobRepo) EndRecord(record model.JobRecord, status, message, records string) {
	vars := map[string]interface{}{
		"records":  records,
		"status":   status,
		"message":  message,
		"task_id":  record.TaskID,
		"interval": float64(now().Sub(record.StartTime).Milliseconds()),
	}
	_ = u.UpdateRecord(record.ID, vars)
	_ = u.Update(record.CronjobID, map[string]interface{}{"is_executing": false})
}
