package service

import (
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/2panel-dev/2panel/internal/database"
	"github.com/2panel-dev/2panel/internal/dto"
	"github.com/2panel-dev/2panel/internal/model"
	"github.com/2panel-dev/2panel/internal/repo"
	"github.com/2panel-dev/2panel/internal/scheduler"
	"github.com/robfig/cron/v3"
)

type CronjobService struct{}

var cronjobRepo repo.CronjobRepo

func (u *CronjobService) SearchWithPage(search dto.SearchCronjob) (int64, []dto.CronjobInfo, error) {
	var opts []repo.DBOption
	if len(search.Type) != 0 {
		opts = append(opts, repo.WithByType(search.Type))
	}
	opts = append(opts, repo.WithByLikeName(search.Info))
	opts = append(opts, repo.WithOrderBy(search.OrderBy, search.Order))

	total, cronjobs, err := cronjobRepo.Page(search.Page.Page, search.Page.PageSize, opts...)
	if err != nil {
		return 0, nil, err
	}
	items := make([]dto.CronjobInfo, 0)
	for _, cronjob := range cronjobs {
		item := u.toInfo(cronjob)
		items = append(items, item)
	}
	return total, items, nil
}

// Stats aggregates dashboard numbers: total / enabled / executing cronjobs and
// today's execution success rate.
func (u *CronjobService) Stats() (dto.CronjobStats, error) {
	stats := dto.CronjobStats{}
	if total, err := cronjobRepo.Count(); err != nil {
		return stats, err
	} else {
		stats.Total = total
	}
	if enabled, err := cronjobRepo.Count(repo.WithByStatus(model.StatusEnable)); err != nil {
		return stats, err
	} else {
		stats.Enabled = enabled
	}
	if executing, err := cronjobRepo.Count(repo.WithIsExecuting(true)); err != nil {
		return stats, err
	} else {
		stats.Executing = executing
	}
	now := time.Now()
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	stats.TodaySuccess, _ = cronjobRepo.CountRecords(repo.WithByStatus(model.StatusSuccess), repo.WithStartTimeAfter(start))
	stats.TodayFailed, _ = cronjobRepo.CountRecords(repo.WithByStatus(model.StatusFailed), repo.WithStartTimeAfter(start))
	if done := stats.TodaySuccess + stats.TodayFailed; done > 0 {
		stats.TodayRate = math.Round(float64(stats.TodaySuccess)/float64(done)*1000) / 10
	}
	return stats, nil
}

func (u *CronjobService) LoadInfo(id uint) (dto.CronjobOperate, error) {
	cronjob, err := cronjobRepo.Get(repo.WithByID(id))
	if err != nil {
		return dto.CronjobOperate{}, err
	}
	return dto.CronjobOperate{
		ID:           cronjob.ID,
		Name:         cronjob.Name,
		Type:         cronjob.Type,
		Spec:         cronjob.Spec,
		SpecCustom:   cronjob.SpecCustom,
		Executor:     cronjob.Executor,
		Script:       cronjob.Script,
		ScriptName:   cronjob.ScriptName,
		User:         cronjob.User,
		URL:          cronjob.URL,
		RetryTimes:   cronjob.RetryTimes,
		Timeout:      cronjob.Timeout,
		RetainCopies: int(cronjob.RetainCopies),
	}, nil
}

func (u *CronjobService) toInfo(cronjob model.Cronjob) dto.CronjobInfo {
	item := dto.CronjobInfo{
		ID:           cronjob.ID,
		Name:         cronjob.Name,
		Type:         cronjob.Type,
		Spec:         cronjob.Spec,
		SpecCustom:   cronjob.SpecCustom,
		Executor:     cronjob.Executor,
		Script:       cronjob.Script,
		ScriptName:   cronjob.ScriptName,
		User:         cronjob.User,
		URL:          cronjob.URL,
		RetryTimes:   cronjob.RetryTimes,
		Timeout:      cronjob.Timeout,
		IsExecuting:  cronjob.IsExecuting,
		Status:       cronjob.Status,
		CreatedAt:    cronjob.CreatedAt,
		RetainCopies: int(cronjob.RetainCopies),
	}
	if record, err := cronjobRepo.RecordFirst(cronjob.ID); err == nil {
		item.LastRecordStatus = record.Status
		item.LastRecordTime = record.StartTime.Format("2006-01-02 15:04:05")
	}
	if next, err := u.nextRunTime(cronjob.Spec); err == nil && len(next) > 0 {
		item.NextRunTime = next[0]
	}
	return item
}

func (u *CronjobService) nextRunTime(spec string) ([]string, error) {
	now := time.Now()
	var next []string
	for _, s := range strings.Split(spec, "&&") {
		s = strings.TrimSpace(s)
		if len(s) == 0 {
			continue
		}
		times, err := u.nextRunTimeSingle(s, now)
		if err != nil {
			return nil, err
		}
		next = append(next, times...)
	}
	seen := make(map[string]bool)
	var uniq []string
	for _, t := range next {
		if !seen[t] {
			seen[t] = true
			uniq = append(uniq, t)
		}
	}
	sort.Strings(uniq)
	if len(uniq) > 5 {
		uniq = uniq[:5]
	}
	return uniq, nil
}

func (u *CronjobService) nextRunTimeSingle(spec string, now time.Time) ([]string, error) {
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
	var next []string
	if strings.HasPrefix(spec, "@every ") {
		normalized, err := scheduler.NormalizeEverySpec(spec)
		if err != nil {
			return nil, err
		}
		dur, err := time.ParseDuration(strings.TrimPrefix(normalized, "@every "))
		if err != nil {
			return nil, err
		}
		if dur < time.Second {
			dur = time.Second
		}
		for i := 0; i < 5; i++ {
			now = now.Add(dur)
			next = append(next, now.Format("2006-01-02 15:04:05"))
		}
		return next, nil
	}
	sched, err := parser.Parse(spec)
	if err != nil {
		return nil, err
	}
	for i := 0; i < 5; i++ {
		nextTime := sched.Next(now)
		next = append(next, nextTime.Format("2006-01-02 15:04:05"))
		now = nextTime
	}
	return next, nil
}

func (u *CronjobService) LoadNextHandle(spec string) ([]string, error) {
	return u.nextRunTime(spec)
}

func (u *CronjobService) Create(req dto.CronjobOperate) error {
	if _, err := cronjobRepo.Get(repo.WithByName(req.Name)); err == nil {
		return fmt.Errorf("the cronjob name already exists")
	}
	if err := u.validateOperate(req); err != nil {
		return err
	}
	cronjob := model.Cronjob{
		Name:         req.Name,
		Type:         req.Type,
		Spec:         req.Spec,
		SpecCustom:   req.SpecCustom,
		Executor:     req.Executor,
		Script:       req.Script,
		ScriptName:   req.ScriptName,
		User:         req.User,
		URL:          req.URL,
		RetryTimes:   req.RetryTimes,
		Timeout:      req.Timeout,
		RetainCopies: uint64(req.RetainCopies),
		Status:       model.StatusEnable,
	}

	entryIDs, err := scheduler.GetRunner().Register(&cronjob, u.HandleJob)
	if err != nil {
		return fmt.Errorf("register cronjob failed: %v", err)
	}
	cronjob.EntryIDs = entryIDs
	if err := cronjobRepo.Create(&cronjob); err != nil {
		scheduler.GetRunner().Remove(entryIDs)
		return err
	}
	return nil
}

func (u *CronjobService) Update(id uint, req dto.CronjobOperate) error {
	cronjob, err := cronjobRepo.Get(repo.WithByID(id))
	if err != nil {
		return fmt.Errorf("cronjob not found")
	}
	if err := u.validateOperate(req); err != nil {
		return err
	}

	if cronjob.Status == model.StatusEnable {
		scheduler.GetRunner().Remove(cronjob.EntryIDs)
		upCronjob := cronjob
		upCronjob.Spec = req.Spec
		entryIDs, err := scheduler.GetRunner().Register(&upCronjob, u.HandleJob)
		if err != nil {
			return fmt.Errorf("re-register cronjob failed: %v", err)
		}
		cronjob.EntryIDs = entryIDs
	}

	vars := map[string]interface{}{
		"name":          req.Name,
		"type":          req.Type,
		"spec":          req.Spec,
		"spec_custom":   req.SpecCustom,
		"executor":      req.Executor,
		"script":        req.Script,
		"script_name":   req.ScriptName,
		"user":          req.User,
		"url":           req.URL,
		"retry_times":   req.RetryTimes,
		"timeout":       req.Timeout,
		"retain_copies": req.RetainCopies,
		"entry_ids":     cronjob.EntryIDs,
	}
	if err := cronjobRepo.Update(id, vars); err != nil {
		return err
	}
	cronjob.RetainCopies = uint64(req.RetainCopies)
	u.removeExpiredLog(cronjob)
	return nil
}

func (u *CronjobService) UpdateStatus(id uint, status string) error {
	if status != model.StatusEnable && status != model.StatusDisable {
		return fmt.Errorf("无效的任务状态: %s", status)
	}
	cronjob, err := cronjobRepo.Get(repo.WithByID(id))
	if err != nil {
		return fmt.Errorf("cronjob not found")
	}
	entryIDs := cronjob.EntryIDs
	if status == model.StatusEnable {
		newEntryIDs, err := scheduler.GetRunner().Register(&cronjob, u.HandleJob)
		if err != nil {
			return fmt.Errorf("register cronjob failed: %v", err)
		}
		entryIDs = newEntryIDs
	} else {
		scheduler.GetRunner().Remove(cronjob.EntryIDs)
		entryIDs = ""
	}
	return cronjobRepo.Update(id, map[string]interface{}{
		"status":    status,
		"entry_ids": entryIDs,
	})
}

func (u *CronjobService) Delete(req dto.CronjobBatchDelete) error {
	for _, id := range req.IDs {
		cronjob, err := cronjobRepo.Get(repo.WithByID(id))
		if err != nil {
			continue
		}
		scheduler.GetRunner().Remove(cronjob.EntryIDs)
		_ = os.RemoveAll(filepath.Join(scheduler.GetRunner().DataDir(), "task", cronjob.Name))
		if err := u.cleanRecord(id); err != nil {
			return err
		}
		if err := cronjobRepo.Delete(repo.WithByID(id)); err != nil {
			return err
		}
	}
	return nil
}

func (u *CronjobService) removeExpiredLog(cronjob model.Cronjob) {
	if cronjob.RetainCopies == 0 {
		return
	}
	records, err := cronjobRepo.ListRecords(repo.WithByCronjobID(cronjob.ID), repo.WithOrderBy("created_at", "desc"))
	if err != nil {
		return
	}
	if uint64(len(records)) <= cronjob.RetainCopies {
		return
	}
	for _, record := range records[cronjob.RetainCopies:] {
		_ = os.RemoveAll(record.Records)
		_ = cronjobRepo.DeleteRecords(repo.WithByID(record.ID))
	}
}

func (u *CronjobService) cleanRecord(cronjobID uint) error {
	records, _ := cronjobRepo.ListRecords(repo.WithByCronjobID(cronjobID))
	for _, record := range records {
		_ = os.RemoveAll(record.Records)
	}
	return cronjobRepo.DeleteRecords(repo.WithByCronjobID(cronjobID))
}

func (u *CronjobService) HandleOnce(id uint) error {
	cronjob, err := cronjobRepo.Get(repo.WithByID(id))
	if err != nil {
		return fmt.Errorf("cronjob not found")
	}
	if cronjob.IsExecuting {
		return fmt.Errorf("the cronjob is executing, please wait")
	}
	go u.HandleJob(&cronjob)
	return nil
}

func (u *CronjobService) HandleStop(id uint) error {
	record, err := cronjobRepo.RecordFirst(id)
	if err != nil {
		return fmt.Errorf("record not found")
	}
	if len(record.TaskID) == 0 {
		return nil
	}
	scheduler.GetRunner().Stop(record.TaskID)
	return nil
}

func (u *CronjobService) HandleJob(cronjob *model.Cronjob) {
	cronjobItem, err := cronjobRepo.Get(repo.WithByID(cronjob.ID))
	if err != nil {
		return
	}
	record, err := cronjobRepo.StartRecord(cronjobItem.ID)
	if err != nil {
		u.recordSkipped(cronjobItem, err)
		return
	}
	logPath := filepath.Join(scheduler.GetRunner().DataDir(), "log", record.TaskID+".log")
	logWriter, err := scheduler.NewLogWriter(logPath)
	if err != nil {
		cronjobRepo.EndRecord(record, model.StatusFailed, err.Error(), logPath)
		u.removeExpiredLog(cronjobItem)
		return
	}

	go func() {
		defer logWriter.Close()
		defer u.removeExpiredLog(cronjobItem)
		logWriter.Logf("开始执行任务 [%s] %s", cronjobItem.Type, cronjobItem.Name)
		start := time.Now()
		runJob := cronjobItem
		u.resolveScriptContent(&runJob)
		err := scheduler.GetRunner().RunJob(&runJob, &record, logWriter)
		if err != nil {
			cronjobRepo.EndRecord(record, model.StatusFailed, err.Error(), logPath)
			logWriter.LogLevelf("错误", "任务 [%s] 执行失败，耗时 %.2fs，错误: %v", cronjobItem.Name, time.Since(start).Seconds(), err)
			return
		}
		cronjobRepo.EndRecord(record, model.StatusSuccess, "", logPath)
		logWriter.LogLevelf("成功", "任务 [%s] 执行成功，耗时 %.2fs", cronjobItem.Name, time.Since(start).Seconds())
	}()
}

// recordSkipped records an "unexecuted" marker when the job could not start
// because it is already executing, and prunes the job's history.
func (u *CronjobService) recordSkipped(cronjob model.Cronjob, cause error) {
	if errors.Is(cause, repo.ErrJobExecuting) {
		rec := model.JobRecord{
			CronjobID: cronjob.ID,
			TaskID:    newTaskID(),
			StartTime: time.Now(),
			Status:    model.StatusUnexecut,
			Message:   "the cronjob is executing, please wait",
		}
		_ = cronjobRepo.CreateRecord(&rec)
	}
	u.removeExpiredLog(cronjob)
}

func (u *CronjobService) SearchRecords(search dto.SearchRecord) (int64, []dto.Record, error) {
	var opts []repo.DBOption
	if search.CronjobID != 0 {
		opts = append(opts, repo.WithByCronjobID(search.CronjobID))
	}
	if len(search.Status) != 0 {
		opts = append(opts, repo.WithByStatus(search.Status))
	}
	total, records, err := cronjobRepo.PageRecords(search.Page.Page, search.Page.PageSize, opts...)
	if err != nil {
		return 0, nil, err
	}
	items := make([]dto.Record, 0)
	for _, record := range records {
		items = append(items, dto.Record{
			ID:        record.ID,
			CronjobID: record.CronjobID,
			TaskID:    record.TaskID,
			StartTime: record.StartTime.Format("2006-01-02 15:04:05"),
			Interval:  record.Interval,
			Status:    record.Status,
			Message:   record.Message,
			CreatedAt: record.CreatedAt,
		})
	}
	return total, items, nil
}

func (u *CronjobService) LoadRecordLog(id uint) string {
	path, err := u.recordLogPath(id)
	if err != nil {
		return ""
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(content)
}

// recordLogPath returns the log file path for an execution record. While a
// task is running the record's `records` field is still empty, so the path is
// derived from the taskID instead.
func (u *CronjobService) recordLogPath(id uint) (string, error) {
	record, err := cronjobRepo.GetRecord(repo.WithByID(id))
	if err != nil {
		return "", fmt.Errorf("record not found")
	}
	if record.Records != "" {
		return record.Records, nil
	}
	return filepath.Join(scheduler.GetRunner().DataDir(), "log", record.TaskID+".log"), nil
}

// ReadRecordLogTail returns the log content appended since the given byte
// offset plus the next offset and the record status. The frontend polls this
// endpoint while a task is running to render a live log view.
func (u *CronjobService) ReadRecordLogTail(req dto.RecordLogTailReq) (dto.RecordLogTail, error) {
	record, err := cronjobRepo.GetRecord(repo.WithByID(req.ID))
	if err != nil {
		return dto.RecordLogTail{}, fmt.Errorf("record not found")
	}
	result := dto.RecordLogTail{Offset: req.Offset, Status: record.Status}
	if req.Offset < 0 {
		result.Offset = 0
	}
	path := record.Records
	if path == "" {
		path = filepath.Join(scheduler.GetRunner().DataDir(), "log", record.TaskID+".log")
	}
	f, err := os.Open(path)
	if err != nil {
		return result, nil
	}
	defer f.Close()
	stat, err := f.Stat()
	if err != nil {
		return result, nil
	}
	size := stat.Size()
	if result.Offset > size {
		result.Offset = size
	}
	if _, err := f.Seek(result.Offset, io.SeekStart); err != nil {
		return result, nil
	}
	content, _ := io.ReadAll(f)
	result.Content = string(content)
	result.Offset = size
	return result, nil
}

// Export returns all cronjobs as importable payloads (created timestamps and
// runtime state are stripped so a re-import creates fresh entries).
func (u *CronjobService) Export() ([]dto.CronjobOperate, error) {
	cronjobs, err := cronjobRepo.List()
	if err != nil {
		return nil, err
	}
	items := make([]dto.CronjobOperate, 0)
	for _, cronjob := range cronjobs {
		items = append(items, dto.CronjobOperate{
			ID:           cronjob.ID,
			Name:         cronjob.Name,
			Type:         cronjob.Type,
			Spec:         cronjob.Spec,
			SpecCustom:   cronjob.SpecCustom,
			Executor:     cronjob.Executor,
			Script:       cronjob.Script,
			ScriptName:   cronjob.ScriptName,
			User:         cronjob.User,
			URL:          cronjob.URL,
			RetryTimes:   cronjob.RetryTimes,
			Timeout:      cronjob.Timeout,
			RetainCopies: int(cronjob.RetainCopies),
		})
	}
	return items, nil
}

// Import creates cronjobs from an exported payload. Existing names are
// skipped and per-item spec errors are counted so a partial import succeeds.
func (u *CronjobService) Import(req dto.CronjobImport) (dto.CronjobImportResult, error) {
	result := dto.CronjobImportResult{}
	for _, item := range req.Data {
		if _, err := cronjobRepo.Get(repo.WithByName(item.Name)); err == nil {
			result.Skipped++
			result.SkippedItems = append(result.SkippedItems, item.Name)
			continue
		}
		if item.Type == "" {
			item.Type = model.TypeShell
		}
		if item.Executor == "" {
			item.Executor = "bash"
		}
		if item.RetainCopies == 0 {
			item.RetainCopies = 7
		}
		item.ID = 0
		if err := u.Create(item); err != nil {
			result.Failed++
			result.FailedItems = append(result.FailedItems, item.Name)
			continue
		}
		result.Created++
	}
	return result, nil
}

// ScriptOptions lists the library scripts for the cronjob editor's select.
func (u *CronjobService) ScriptOptions() ([]dto.ScriptOption, error) {
	return scriptService.Options()
}

// resolveScriptContent fills the script content from the script library when
// the cronjob references a library script by name.
func (u *CronjobService) resolveScriptContent(cronjob *model.Cronjob) {
	if len(cronjob.ScriptName) == 0 {
		return
	}
	if content := scriptService.ResolveByName(cronjob.ScriptName); len(content) != 0 {
		cronjob.Script = content
	}
}

// validateOperate performs server-side validation of a create/update payload.
// The dto struct tags carry no binding framework, so every rule is enforced
// here rather than relying on tags that are never evaluated.
func (u *CronjobService) validateOperate(req dto.CronjobOperate) error {
	if len(strings.TrimSpace(req.Name)) == 0 {
		return fmt.Errorf("the cronjob name is required")
	}
	switch req.Type {
	case model.TypeShell:
		if len(strings.TrimSpace(req.Script)) == 0 && len(strings.TrimSpace(req.ScriptName)) == 0 {
			return fmt.Errorf("the shell script is required")
		}
	case model.TypeCurl:
		if len(strings.TrimSpace(req.URL)) == 0 {
			return fmt.Errorf("the curl url is required")
		}
	default:
		return fmt.Errorf("unsupported cronjob type: %s", req.Type)
	}
	return u.validateSpec(req.Spec)
}

func (u *CronjobService) validateSpec(spec string) error {
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
	for _, s := range strings.Split(spec, "&&") {
		s = strings.TrimSpace(s)
		if len(s) == 0 {
			continue
		}
		if strings.HasPrefix(s, "@every ") {
			if _, err := scheduler.NormalizeEverySpec(s); err != nil {
				return err
			}
			continue
		}
		if _, err := parser.Parse(s); err != nil {
			return err
		}
	}
	return nil
}

// RestoreCronjobs re-registers all enabled cronjobs in the scheduler after
// the service restarts. It also resets the runtime state that a previous
// process crash may have left behind: a stuck is_executing flag would
// otherwise block the job forever, and in-flight records would stay "waiting".
// Callers are responsible for stopping the scheduler first (ResetEntries).
func RestoreCronjobs() {
	database.DB.Model(&model.Cronjob{}).Where("is_executing = ?", true).Update("is_executing", false)
	database.DB.Model(&model.JobRecord{}).Where("status = ?", model.StatusWaiting).
		Updates(map[string]interface{}{"status": model.StatusFailed, "message": "interrupted by service restart"})

	var cronjobSvc CronjobService
	cronjobs, err := cronjobRepo.List(repo.WithByStatus(model.StatusEnable))
	if err != nil {
		return
	}
	for i := range cronjobs {
		cronjob := cronjobs[i]
		entryIDs, err := scheduler.GetRunner().Register(&cronjob, cronjobSvc.HandleJob)
		if err != nil {
			continue
		}
		_ = cronjobRepo.Update(cronjob.ID, map[string]interface{}{"entry_ids": entryIDs})
	}
}
