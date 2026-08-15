package service

import (
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/2panel-dev/2panel/internal/database"
	"github.com/2panel-dev/2panel/internal/dto"
	"github.com/2panel-dev/2panel/internal/model"
	"github.com/2panel-dev/2panel/internal/repo"
	"github.com/2panel-dev/2panel/internal/scheduler"
)

// FileWatchService implements the file-change triggered conditional tasks,
// backed by fsnotify/inotify watchers managed by the scheduler Runner.
type FileWatchService struct{}

var fileWatchRepo repo.FileWatchRepo

// joinPaths flattens a path slice into the newline-separated storage form.
func joinPaths(paths []string) string {
	var cleaned []string
	for _, p := range paths {
		p = strings.TrimSpace(p)
		if len(p) != 0 {
			cleaned = append(cleaned, p)
		}
	}
	return strings.Join(cleaned, "\n")
}

// splitPathsFromForm splits the stored newline-separated paths back into a
// slice for the JSON API.
func splitPathsFromForm(paths string) []string {
	var out []string
	for _, line := range strings.Split(paths, "\n") {
		if len(strings.TrimSpace(line)) != 0 {
			out = append(out, line)
		}
	}
	return out
}

// eventNames validates and normalizes the event slice into a comma-joined
// storage string, e.g. ["write","create"] -> "write,create".
func eventNames(events []string) (string, error) {
	if len(events) == 0 {
		return "", fmt.Errorf("the watch events are required")
	}
	seen := make(map[string]bool)
	var names []string
	for _, e := range events {
		e = strings.TrimSpace(e)
		switch e {
		case model.EventWrite, model.EventCreate, model.EventRemove:
			if !seen[e] {
				seen[e] = true
				names = append(names, e)
			}
		default:
			return "", fmt.Errorf("unsupported watch event: %s", e)
		}
	}
	if len(names) == 0 {
		return "", fmt.Errorf("the watch events are required")
	}
	return strings.Join(names, ","), nil
}

func eventNamesFromForm(events string) []string {
	var out []string
	for _, e := range strings.Split(events, ",") {
		e = strings.TrimSpace(e)
		if len(e) != 0 {
			out = append(out, e)
		}
	}
	return out
}

func (u *FileWatchService) SearchWithPage(search dto.SearchCronjob) (int64, []dto.FileWatchInfo, error) {
	var opts []repo.DBOption
	if len(search.Type) != 0 {
		opts = append(opts, repo.WithByType(search.Type))
	}
	opts = append(opts, repo.WithByLikeName(search.Info))
	opts = append(opts, repo.WithOrderBy(search.OrderBy, search.Order))

	total, watches, err := fileWatchRepo.Page(search.Page.Page, search.Page.PageSize, opts...)
	if err != nil {
		return 0, nil, err
	}
	ids := make([]uint, 0, len(watches))
	for _, watch := range watches {
		ids = append(ids, watch.ID)
	}
	recordMap, err := fileWatchRepo.RecordFirstBatch(ids)
	if err != nil {
		return 0, nil, err
	}
	items := make([]dto.FileWatchInfo, 0)
	for _, watch := range watches {
		items = append(items, u.toInfo(watch, recordMap[watch.ID]))
	}
	return total, items, nil
}

func (u *FileWatchService) toInfo(watch model.FileWatch, record model.FileWatchRecord) dto.FileWatchInfo {
	item := dto.FileWatchInfo{
		ID:           watch.ID,
		Name:         watch.Name,
		Paths:        splitPathsFromForm(watch.Paths),
		Events:       eventNamesFromForm(watch.Events),
		Comment:      watch.Comment,
		Executor:     watch.Executor,
		Script:       watch.Script,
		ScriptName:   watch.ScriptName,
		User:         watch.User,
		Debounce:     watch.Debounce,
		Timeout:      watch.Timeout,
		Status:       watch.Status,
		Watching:     scheduler.GetRunner().IsWatchingFileWatch(watch.ID),
		CreatedAt:    watch.CreatedAt,
		RetainCopies: int(watch.RetainCopies),
	}
	if record.ID != 0 {
		item.LastRecordStatus = record.Status
		item.LastRecordTime = record.StartTime.Format("2006-01-02 15:04:05")
	}
	return item
}

func (u *FileWatchService) Stats() (dto.CronjobStats, error) {
	stats := dto.CronjobStats{}
	now := time.Now()
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	if err := database.DB.Model(&model.FileWatch{}).
		Select(`COUNT(*) AS total,
			COALESCE(SUM(CASE WHEN status = ? THEN 1 ELSE 0 END), 0) AS enabled`,
			model.StatusEnable).Scan(&stats).Error; err != nil {
		return stats, err
	}
	var records dto.CronjobStats
	if err := database.DB.Model(&model.FileWatchRecord{}).
		Select(`COALESCE(SUM(CASE WHEN status = ? AND start_time >= ? THEN 1 ELSE 0 END), 0) AS today_success,
			COALESCE(SUM(CASE WHEN status = ? AND start_time >= ? THEN 1 ELSE 0 END), 0) AS today_failed`,
			model.StatusSuccess, start, model.StatusFailed, start).Scan(&records).Error; err != nil {
		return stats, err
	}
	stats.TodaySuccess = records.TodaySuccess
	stats.TodayFailed = records.TodayFailed
	// "executing" for a conditional task means its inotify watcher is live
	// right now; count them via the scheduler rather than enabled-status only.
	runner := scheduler.GetRunner()
	if watches, err := fileWatchRepo.List(repo.WithByStatus(model.StatusEnable)); err == nil {
		for _, w := range watches {
			if runner.IsWatchingFileWatch(w.ID) {
				stats.Executing++
			}
		}
	}
	if done := stats.TodaySuccess + stats.TodayFailed; done > 0 {
		stats.TodayRate = math.Round(float64(stats.TodaySuccess)/float64(done)*1000) / 10
	}
	return stats, nil
}

func (u *FileWatchService) LoadInfo(id uint) (dto.FileWatchOperate, error) {
	watch, err := fileWatchRepo.Get(repo.WithByID(id))
	if err != nil {
		return dto.FileWatchOperate{}, err
	}
	return dto.FileWatchOperate{
		ID:           watch.ID,
		Name:         watch.Name,
		Paths:        splitPathsFromForm(watch.Paths),
		Events:       eventNamesFromForm(watch.Events),
		Comment:      watch.Comment,
		Executor:     watch.Executor,
		Script:       watch.Script,
		ScriptName:   watch.ScriptName,
		User:         watch.User,
		Debounce:     watch.Debounce,
		Timeout:      watch.Timeout,
		RetainCopies: int(watch.RetainCopies),
	}, nil
}

func (u *FileWatchService) Create(req dto.FileWatchOperate) error {
	if _, err := fileWatchRepo.Get(repo.WithByName(req.Name)); err == nil {
		return fmt.Errorf("the file watch name already exists")
	}
	if err := u.validateOperate(req); err != nil {
		return err
	}
	watch := model.FileWatch{
		Name:         req.Name,
		Paths:        joinPaths(req.Paths),
		Events:       strings.Join(req.Events, ","),
		Comment:      req.Comment,
		Executor:     req.Executor,
		Script:       req.Script,
		ScriptName:   req.ScriptName,
		User:         req.User,
		Debounce:     req.Debounce,
		Timeout:      req.Timeout,
		RetainCopies: uint64(req.RetainCopies),
		Status:       model.StatusEnable,
	}
	if err := fileWatchRepo.Create(&watch); err != nil {
		return err
	}
	if err := u.register(&watch); err != nil {
		return err
	}
	return nil
}

func (u *FileWatchService) Update(id uint, req dto.FileWatchOperate) error {
	watch, err := fileWatchRepo.Get(repo.WithByID(id))
	if err != nil {
		return fmt.Errorf("file watch not found")
	}
	if err := u.validateOperate(req); err != nil {
		return err
	}
	// stop the live watcher before touching config so the running entry never
	// observes a half-updated task
	scheduler.GetRunner().StopFileWatch(id)

	vars := map[string]interface{}{
		"name":          req.Name,
		"paths":         joinPaths(req.Paths),
		"events":        strings.Join(req.Events, ","),
		"comment":       req.Comment,
		"executor":      req.Executor,
		"script":        req.Script,
		"script_name":   req.ScriptName,
		"user":          req.User,
		"debounce":      req.Debounce,
		"timeout":       req.Timeout,
		"retain_copies": req.RetainCopies,
	}
	if err := fileWatchRepo.Update(id, vars); err != nil {
		return err
	}
	watch.Name = req.Name
	watch.Paths = joinPaths(req.Paths)
	watch.Events = strings.Join(req.Events, ",")
	watch.Executor = req.Executor
	watch.Script = req.Script
	watch.ScriptName = req.ScriptName
	watch.User = req.User
	watch.Debounce = req.Debounce
	watch.Timeout = req.Timeout
	watch.RetainCopies = uint64(req.RetainCopies)

	if watch.Status == model.StatusEnable {
		if err := u.register(&watch); err != nil {
			return err
		}
	}
	u.removeExpiredLog(watch)
	return nil
}

func (u *FileWatchService) UpdateStatus(id uint, status string) error {
	if status != model.StatusEnable && status != model.StatusDisable {
		return fmt.Errorf("无效的任务状态: %s", status)
	}
	watch, err := fileWatchRepo.Get(repo.WithByID(id))
	if err != nil {
		return fmt.Errorf("file watch not found")
	}
	// set the target status on the in-memory copy first so register() sees an
	// enabled task; the DB row is updated after the watcher starts/stopped.
	watch.Status = status
	if status == model.StatusEnable {
		if err := u.register(&watch); err != nil {
			return err
		}
	} else {
		// closing the watcher here releases the inotify fd immediately
		scheduler.GetRunner().StopFileWatch(id)
	}
	return fileWatchRepo.Update(id, map[string]interface{}{"status": status})
}

func (u *FileWatchService) Delete(req dto.CronjobBatchDelete) error {
	for _, id := range req.IDs {
		watch, err := fileWatchRepo.Get(repo.WithByID(id))
		if err != nil {
			continue
		}
		scheduler.GetRunner().StopFileWatch(id)
		_ = os.RemoveAll(filepath.Join(scheduler.GetRunner().DataDir(), "task", "watch", watch.Name))
		if err := u.cleanRecord(id); err != nil {
			return err
		}
		if err := fileWatchRepo.Delete(repo.WithByID(id)); err != nil {
			return err
		}
	}
	return nil
}

// register starts the inotify watcher for an enabled task and records the
// watcher key so the list view can tell "enabled but not watching".
func (u *FileWatchService) register(watch *model.FileWatch) error {
	if watch.Status != model.StatusEnable {
		return nil
	}
	if err := scheduler.GetRunner().StartFileWatch(watch, u.HandleWatch); err != nil {
		return fmt.Errorf("register file watch failed: %v", err)
	}
	return fileWatchRepo.Update(watch.ID, map[string]interface{}{"watcher_key": fmt.Sprintf("watch-%d", watch.ID)})
}

// HandleWatch runs the task's command once for a matching file event and
// records a FileWatchRecord with a full stdout/stderr log. Invoked by the
// watcher loop synchronously; the execution itself runs in a goroutine.
func (u *FileWatchService) HandleWatch(watch *model.FileWatch, eventType, eventPath string) {
	watchItem, err := fileWatchRepo.Get(repo.WithByID(watch.ID))
	if err != nil {
		return
	}
	record := model.FileWatchRecord{
		WatchID:   watchItem.ID,
		WatchName: watchItem.Name,
		TaskID:    newTaskID(),
		EventPath: eventPath,
		EventType: eventType,
		StartTime: time.Now(),
		Status:    model.StatusWaiting,
	}
	if err := fileWatchRepo.CreateRecord(&record); err != nil {
		return
	}
	logPath := filepath.Join(scheduler.GetRunner().DataDir(), "log", record.TaskID+".log")
	logWriter, err := scheduler.NewLogWriter(logPath)
	if err != nil {
		fileWatchRepo.UpdateRecord(record.ID, map[string]interface{}{
			"records": logPath, "status": model.StatusFailed, "message": err.Error(),
		})
		u.removeExpiredLog(watchItem)
		return
	}

	go func() {
		defer logWriter.Close()
		defer u.removeExpiredLog(watchItem)
		logWriter.Logf("文件监控触发任务 [%s] %s（%s: %s）", eventType, watchItem.Name, eventType, eventPath)
		start := time.Now()
		runWatch := watchItem
		u.resolveScriptContent(&runWatch)
		err := scheduler.GetRunner().RunWatchJob(&runWatch, eventType, eventPath, logWriter)
		interval := time.Since(start).Seconds()
		if err != nil {
			fileWatchRepo.UpdateRecord(record.ID, map[string]interface{}{
				"records": logPath, "status": model.StatusFailed, "message": err.Error(),
				"interval": interval,
			})
			logWriter.LogLevelf("错误", "任务 [%s] 执行失败，耗时 %.2fs，错误: %v", watchItem.Name, interval, err)
			return
		}
		fileWatchRepo.UpdateRecord(record.ID, map[string]interface{}{
			"records": logPath, "status": model.StatusSuccess, "message": "", "interval": interval,
		})
		logWriter.LogLevelf("成功", "任务 [%s] 执行成功，耗时 %.2fs", watchItem.Name, interval)
	}()
}

// HandleOnce triggers the task's command manually (without a file event), for
// the "立即执行" button. Only possible when no run is currently pending.
func (u *FileWatchService) HandleOnce(id uint) error {
	watch, err := fileWatchRepo.Get(repo.WithByID(id))
	if err != nil {
		return fmt.Errorf("file watch not found")
	}
	u.HandleWatch(&watch, model.EventCreate, "(manual)")
	return nil
}

func (u *FileWatchService) removeExpiredLog(watch model.FileWatch) {
	if watch.RetainCopies == 0 {
		return
	}
	records, err := fileWatchRepo.ListRecordsLimit(int(watch.RetainCopies)+1, repo.WithByWatchID(watch.ID))
	if err != nil {
		return
	}
	if uint64(len(records)) <= watch.RetainCopies {
		return
	}
	for _, record := range records[watch.RetainCopies:] {
		_ = os.RemoveAll(record.Records)
		_ = fileWatchRepo.DeleteRecords(repo.WithByID(record.ID))
	}
}

func (u *FileWatchService) cleanRecord(watchID uint) error {
	records, _ := fileWatchRepo.ListRecords(repo.WithByWatchID(watchID))
	for _, record := range records {
		_ = os.RemoveAll(record.Records)
	}
	return fileWatchRepo.DeleteRecords(repo.WithByWatchID(watchID))
}

func (u *FileWatchService) SearchRecords(search dto.SearchFileWatchRecord) (int64, []dto.FileWatchRecord, error) {
	var opts []repo.DBOption
	if search.WatchID != 0 {
		opts = append(opts, repo.WithByWatchID(search.WatchID))
	}
	if len(search.Status) != 0 {
		opts = append(opts, repo.WithByStatus(search.Status))
	}
	total, records, err := fileWatchRepo.PageRecords(search.Page.Page, search.Page.PageSize, opts...)
	if err != nil {
		return 0, nil, err
	}
	items := make([]dto.FileWatchRecord, 0)
	for _, record := range records {
		items = append(items, dto.FileWatchRecord{
			ID:        record.ID,
			WatchID:   record.WatchID,
			WatchName: record.WatchName,
			TaskID:    record.TaskID,
			EventPath: record.EventPath,
			EventType: record.EventType,
			StartTime: record.StartTime.Format("2006-01-02 15:04:05"),
			Interval:  record.Interval,
			Status:    record.Status,
			Message:   record.Message,
			CreatedAt: record.CreatedAt,
		})
	}
	return total, items, nil
}

func (u *FileWatchService) LoadRecordLog(id uint) string {
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

func (u *FileWatchService) recordLogPath(id uint) (string, error) {
	record, err := fileWatchRepo.GetRecord(repo.WithByID(id))
	if err != nil {
		return "", fmt.Errorf("record not found")
	}
	if record.Records != "" {
		return record.Records, nil
	}
	return filepath.Join(scheduler.GetRunner().DataDir(), "log", record.TaskID+".log"), nil
}

// ReadRecordLogTail mirrors the cronjob live-log viewer for file-watch records.
func (u *FileWatchService) ReadRecordLogTail(req dto.RecordLogTailReq) (dto.RecordLogTail, error) {
	record, err := fileWatchRepo.GetRecord(repo.WithByID(req.ID))
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

func (u *FileWatchService) resolveScriptContent(watch *model.FileWatch) {
	if len(watch.ScriptName) == 0 {
		return
	}
	if content := scriptService.ResolveByName(watch.ScriptName); len(content) != 0 {
		watch.Script = content
	}
}

// ScriptOptions lists the library scripts for the file-watch editor's select.
func (u *FileWatchService) ScriptOptions() ([]dto.ScriptOption, error) {
	return scriptService.Options()
}

func (u *FileWatchService) validateOperate(req dto.FileWatchOperate) error {
	if len(strings.TrimSpace(req.Name)) == 0 {
		return fmt.Errorf("the file watch name is required")
	}
	if len(joinPaths(req.Paths)) == 0 {
		return fmt.Errorf("至少需要一个监控路径")
	}
	if _, err := eventNames(req.Events); err != nil {
		return err
	}
	if len(strings.TrimSpace(req.Script)) == 0 && len(strings.TrimSpace(req.ScriptName)) == 0 {
		return fmt.Errorf("the shell script is required")
	}
	return nil
}

// RestoreFilewatches re-registers all enabled file-watch tasks after a service
// restart. Any watcher key left over from a previous process is cleared so the
// UI switch state stays consistent. Callers must have scheduler.Init done.
func RestoreFilewatches() {
	database.DB.Model(&model.FileWatch{}).
		Where("status = ?", model.StatusEnable).Update("watcher_key", "")

	var fwSvc FileWatchService
	watches, err := fileWatchRepo.List(repo.WithByStatus(model.StatusEnable))
	if err != nil {
		return
	}
	for i := range watches {
		watch := watches[i]
		if err := fwSvc.register(&watch); err != nil {
			continue
		}
	}
}

// ensureFileWatchesIdle is used by the restore path to reject a restore while
// a file-watch execution is in flight.
func ensureFileWatchesIdle() error {
	var count int64
	if err := database.DB.Model(&model.FileWatchRecord{}).
		Where("status = ?", model.StatusWaiting).Count(&count).Error; err == nil && count > 0 {
		return fmt.Errorf("有文件监控任务正在执行，请稍后再还原")
	}
	return nil
}