package repo

import (
	"github.com/2panel-dev/2panel/internal/database"
	"github.com/2panel-dev/2panel/internal/model"
	"gorm.io/gorm"
)

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
	if size <= 0 {
		size = 10
	}
	if page <= 0 {
		page = 1
	}
	err := db.Limit(size).Offset(size * (page - 1)).Find(&cronjobs).Error
	return count, cronjobs, err
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
	if size <= 0 {
		size = 10
	}
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

func (u *CronjobRepo) StartRecord(cronjobID uint) model.JobRecord {
	record := model.JobRecord{
		CronjobID: cronjobID,
		TaskID:    newTaskID(),
		StartTime: now(),
		Status:    model.StatusWaiting,
	}
	_ = u.CreateRecord(&record)
	_ = u.Update(cronjobID, map[string]interface{}{"is_executing": true})
	return record
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
