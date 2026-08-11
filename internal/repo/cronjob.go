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

// RecordFirstBatch returns the most recent record per cronjob ID in a single
// query. IDs auto-increment together with CreatedAt, so MAX(id) equals the
// newest record for each cronjob, avoiding the per-row lookup the list view
// used to trigger (N+1).
func (u *CronjobRepo) RecordFirstBatch(cronjobIDs []uint) (map[uint]model.JobRecord, error) {
	result := make(map[uint]model.JobRecord)
	if len(cronjobIDs) == 0 {
		return result, nil
	}
	var records []model.JobRecord
	err := database.DB.Model(&model.JobRecord{}).
		Where("id IN (SELECT MAX(id) FROM job_records WHERE cronjob_id IN ? GROUP BY cronjob_id)", cronjobIDs).
		Find(&records).Error
	if err != nil {
		return nil, err
	}
	for _, rec := range records {
		result[rec.CronjobID] = rec
	}
	return result, nil
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

// ListRecordsLimit returns at most limit records matching the options, used by
// the retention pruning so it never has to load the whole history.
func (u *CronjobRepo) ListRecordsLimit(limit int, opts ...DBOption) ([]model.JobRecord, error) {
	var records []model.JobRecord
	db := database.DB.Model(&model.JobRecord{})
	for _, opt := range opts {
		db = opt(db)
	}
	if limit <= 0 {
		return records, nil
	}
	err := db.Limit(limit).Find(&records).Error
	return records, err
}

// StatsSummary is the result of the single-pass dashboard aggregate.
type StatsSummary struct {
	Total        int64
	Enabled      int64
	Executing    int64
	TodaySuccess int64
	TodayFailed  int64
}

// Stats computes the dashboard numbers with two aggregate queries instead of
// five separate COUNT scans. Each query must scan into its own target: GORM's
// Scan resets the destination struct to its zero value, so scanning the second
// query into the same struct would wipe the first query's totals.
func (u *CronjobRepo) Stats(start time.Time) (StatsSummary, error) {
	var summary StatsSummary
	err := database.DB.Model(&model.Cronjob{}).
		Select(`COUNT(*) AS total,
			COALESCE(SUM(CASE WHEN status = ? THEN 1 ELSE 0 END), 0) AS enabled,
			COALESCE(SUM(CASE WHEN is_executing = ? THEN 1 ELSE 0 END), 0) AS executing`,
			model.StatusEnable, true).
		Scan(&summary).Error
	if err != nil {
		return summary, err
	}
	var records StatsSummary
	err = database.DB.Model(&model.JobRecord{}).
		Select(`COALESCE(SUM(CASE WHEN status = ? AND start_time >= ? THEN 1 ELSE 0 END), 0) AS today_success,
			COALESCE(SUM(CASE WHEN status = ? AND start_time >= ? THEN 1 ELSE 0 END), 0) AS today_failed`,
			model.StatusSuccess, start, model.StatusFailed, start).
		Scan(&records).Error
	if err != nil {
		return summary, err
	}
	summary.TodaySuccess = records.TodaySuccess
	summary.TodayFailed = records.TodayFailed
	return summary, nil
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
