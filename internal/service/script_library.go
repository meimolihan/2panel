package service

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/2panel-dev/2panel/internal/database"
	"github.com/2panel-dev/2panel/internal/dto"
	"github.com/2panel-dev/2panel/internal/model"
	"github.com/2panel-dev/2panel/internal/repo"
	"github.com/2panel-dev/2panel/internal/scheduler"
)

var scriptLibraryRepo repo.ScriptLibraryRepo

var scriptRecordRepo repo.ScriptRecordRepo

var scriptService ScriptService

// scriptRunner tracks running script tasks so they can be stopped from the UI,
// mirroring the scheduler's cancel map but scoped to one-shot library runs.
var scriptRunner = struct {
	mu     sync.Mutex
	cancel map[string]context.CancelFunc
}{cancel: make(map[string]context.CancelFunc)}

// maxScriptRecords caps how many library run records (and their log files)
// are kept, so frequent runs cannot exhaust the disk. Oldest entries are
// pruned after each completed run.
const maxScriptRecords = 200

type ScriptService struct{}

func (u *ScriptService) SearchWithPage(search dto.ScriptSearch) (int64, []dto.ScriptInfo, error) {
	var opts []repo.DBOption
	if len(search.Info) != 0 {
		opts = append(opts, repo.WithByInfo(search.Info))
	}
	if search.GroupID != 0 {
		opts = append(opts, repo.WithByScriptGroup(search.GroupID))
	}
	opts = append(opts, repo.WithOrderBy("created_at", "desc"))
	total, scripts, err := scriptLibraryRepo.Page(search.Page.Page, search.Page.PageSize, opts...)
	if err != nil {
		return 0, nil, err
	}
	groupMap, err := loadScriptGroupMap()
	if err != nil {
		return 0, nil, err
	}
	items := make([]dto.ScriptInfo, 0, len(scripts))
	for _, script := range scripts {
		groupList, groupBelong := parseScriptGroups(script.Groups, groupMap)
		items = append(items, dto.ScriptInfo{
			ID:          script.ID,
			Name:        script.Name,
			Description: script.Description,
			Script:      script.Script,
			GroupList:   groupList,
			GroupBelong: groupBelong,
			CreatedAt:   script.CreatedAt,
		})
	}
	return total, items, nil
}

func (u *ScriptService) LoadInfo(id uint) (dto.ScriptOperate, error) {
	script, err := scriptLibraryRepo.Get(repo.WithByID(id))
	if err != nil {
		return dto.ScriptOperate{}, err
	}
	return dto.ScriptOperate{
		ID:          script.ID,
		Name:        script.Name,
		Description: script.Description,
		Script:      script.Script,
		Groups:      script.Groups,
	}, nil
}

func (u *ScriptService) Create(req dto.ScriptOperate) error {
	name := strings.TrimSpace(req.Name)
	if len(name) == 0 {
		return fmt.Errorf("the script name is required")
	}
	if _, err := scriptLibraryRepo.Get(repo.WithByName(name)); err == nil {
		return fmt.Errorf("the script name already exists")
	}
	script := model.ScriptLibrary{
		Name:        name,
		Description: strings.TrimSpace(req.Description),
		Script:      req.Script,
		Groups:      normalizeScriptGroups(req.Groups),
	}
	return scriptLibraryRepo.Create(&script)
}

func (u *ScriptService) Update(req dto.ScriptOperate) error {
	script, err := scriptLibraryRepo.Get(repo.WithByID(req.ID))
	if err != nil {
		return fmt.Errorf("script not found")
	}
	name := strings.TrimSpace(req.Name)
	if len(name) == 0 {
		return fmt.Errorf("the script name is required")
	}
	if existing, err := scriptLibraryRepo.Get(repo.WithByName(name)); err == nil && existing.ID != script.ID {
		return fmt.Errorf("the script name already exists")
	}
	return scriptLibraryRepo.Update(script.ID, map[string]interface{}{
		"name":        name,
		"description": strings.TrimSpace(req.Description),
		"script":      req.Script,
		"groups":      normalizeScriptGroups(req.Groups),
	})
}

func (u *ScriptService) Delete(ids []uint) error {
	for _, id := range ids {
		script, err := scriptLibraryRepo.Get(repo.WithByID(id))
		if err != nil {
			continue
		}
		_, records, _ := scriptRecordRepo.Page(1, 100000, repo.WithByScriptID(id))
		for _, record := range records {
			if len(record.Records) != 0 {
				_ = os.Remove(record.Records)
			}
		}
		_ = os.RemoveAll(filepath.Join(scheduler.GetRunner().DataDir(), "task", "script-run", fmt.Sprintf("%d", script.ID)))
		if err := scriptRecordRepo.Delete(repo.WithByScriptID(id)); err != nil {
			return err
		}
		if err := scriptLibraryRepo.Delete(repo.WithByID(id)); err != nil {
			return err
		}
	}
	return nil
}

// Options returns the lightweight name list used by the cronjob editor.
func (u *ScriptService) Options() ([]dto.ScriptOption, error) {
	scripts, err := scriptLibraryRepo.List(repo.WithOrderBy("name", "asc"))
	if err != nil {
		return nil, err
	}
	items := make([]dto.ScriptOption, 0)
	for _, script := range scripts {
		items = append(items, dto.ScriptOption{ID: script.ID, Name: script.Name})
	}
	return items, nil
}

// ResolveByName returns the script content for a library script, or empty
// string when the script does not exist.
func (u *ScriptService) ResolveByName(name string) string {
	script, err := scriptLibraryRepo.Get(repo.WithByName(name))
	if err != nil {
		return ""
	}
	return script.Script
}

// Run executes a library script once (the "install" action). It returns the
// task ID used by the frontend to poll the live output.
func (u *ScriptService) Run(id uint) (string, error) {
	script, err := scriptLibraryRepo.Get(repo.WithByID(id))
	if err != nil {
		return "", fmt.Errorf("script not found")
	}
	if len(strings.TrimSpace(script.Script)) == 0 {
		return "", fmt.Errorf("the script content is empty")
	}

	taskID := newTaskID()
	record := model.ScriptRecord{
		TaskID:     taskID,
		ScriptID:   id,
		ScriptName: script.Name,
		StartTime:  time.Now(),
		Status:     model.StatusWaiting,
	}
	if err := scriptRecordRepo.Create(&record); err != nil {
		return "", err
	}

	logPath := filepath.Join(scheduler.GetRunner().DataDir(), "log", taskID+".log")
	logWriter, err := scheduler.NewLogWriter(logPath)
	if err != nil {
		return "", err
	}

	go func() {
		defer logWriter.Close()
		_ = scriptRecordRepo.Update(record.ID, map[string]interface{}{
			"status":  model.StatusRunning,
			"records": logPath,
		})
		logWriter.Logf("开始执行脚本 [%s]", record.ScriptName)
		start := time.Now()
		err := u.runScript(taskID, script.Script, logWriter)
		vars := map[string]interface{}{
			"records":  logPath,
			"task_id":  taskID,
			"interval": float64(time.Since(record.StartTime).Milliseconds()),
		}
		if err != nil {
			vars["status"] = model.StatusFailed
			vars["message"] = err.Error()
			logWriter.LogLevelf("错误", "脚本 [%s] 执行失败，耗时 %.2fs，错误: %v", record.ScriptName, time.Since(start).Seconds(), err)
		} else {
			vars["status"] = model.StatusSuccess
			logWriter.LogLevelf("成功", "脚本 [%s] 执行成功，耗时 %.2fs", record.ScriptName, time.Since(start).Seconds())
		}
		_ = scriptRecordRepo.Update(record.ID, vars)
		pruneScriptRecords()
	}()
	return taskID, nil
}

// pruneScriptRecords removes the oldest script run records beyond
// maxScriptRecords, along with their log files.
func pruneScriptRecords() {
	records, err := scriptRecordRepo.List(repo.WithOrderBy("created_at", "asc"))
	if err != nil {
		return
	}
	excess := len(records) - maxScriptRecords
	if excess <= 0 {
		return
	}
	for _, rec := range records[:excess] {
		if len(rec.Records) != 0 {
			_ = os.Remove(rec.Records)
		}
		_ = scriptRecordRepo.Delete(repo.WithByID(rec.ID))
	}
}

func (u *ScriptService) runScript(taskID, script string, log *scheduler.LogWriter) error {
	jobDir := filepath.Join(scheduler.GetRunner().DataDir(), "task", "script-run", taskID)
	if err := os.MkdirAll(jobDir, 0755); err != nil {
		return err
	}
	scriptFile := filepath.Join(jobDir, taskID+".sh")
	if err := os.WriteFile(scriptFile, []byte(script), 0755); err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	scriptRunner.mu.Lock()
	scriptRunner.cancel[taskID] = cancel
	scriptRunner.mu.Unlock()
	defer func() {
		cancel()
		scriptRunner.mu.Lock()
		delete(scriptRunner.cancel, taskID)
		scriptRunner.mu.Unlock()
		_ = os.RemoveAll(jobDir)
	}()

	cmd := exec.CommandContext(ctx, "bash", scriptFile)
	cmd.Stdout = log
	cmd.Stderr = log
	// Scripts run non-interactively by the scheduler: a single newline on stdin
	// lets trailing "press any key to continue" prompts (read returns 1 on EOF)
	// finish with exit code 0, and TERM=dumb stops terminal tools such as clear
	// from printing "TERM environment variable not set." into the log.
	cmd.Stdin = strings.NewReader("\n")
	cmd.Env = scheduler.NonInteractiveEnv()
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		return err
	}

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		// Kill the whole process group so grandchildren release the output
		// pipe and Wait() can return.
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		select {
		case <-done:
		case <-time.After(5 * time.Second):
		}
		return ctx.Err()
	}
}

// StopRun cancels a running script task by its task ID.
func (u *ScriptService) StopRun(taskID string) error {
	if len(taskID) == 0 {
		return fmt.Errorf("the task id is required")
	}
	scriptRunner.mu.Lock()
	cancel, ok := scriptRunner.cancel[taskID]
	scriptRunner.mu.Unlock()
	if !ok {
		return fmt.Errorf("task not found or already finished")
	}
	cancel()
	return nil
}

// SearchRunRecords returns the paginated run history of library scripts.
func (u *ScriptService) SearchRunRecords(search dto.ScriptRecordSearch) (int64, []dto.ScriptRecord, error) {
	var opts []repo.DBOption
	if search.ScriptID != 0 {
		opts = append(opts, repo.WithByScriptID(search.ScriptID))
	}
	if len(search.Status) != 0 {
		opts = append(opts, repo.WithByStatus(search.Status))
	}
	total, records, err := scriptRecordRepo.Page(search.Page.Page, search.Page.PageSize, opts...)
	if err != nil {
		return 0, nil, err
	}
	items := make([]dto.ScriptRecord, 0)
	for _, record := range records {
		items = append(items, dto.ScriptRecord{
			ID:         record.ID,
			ScriptID:   record.ScriptID,
			ScriptName: record.ScriptName,
			TaskID:     record.TaskID,
			StartTime:  record.StartTime.Format("2006-01-02 15:04:05"),
			Interval:   record.Interval,
			Status:     record.Status,
			Message:    record.Message,
		})
	}
	return total, items, nil
}

// LoadRunLog returns the log content appended since the given byte offset plus
// the next offset and the record status, resolved by task ID so the frontend
// can poll incrementally during execution instead of re-reading the whole file
// every second.
func (u *ScriptService) LoadRunLog(taskID string, offset int64) (dto.ScriptLog, error) {
	record, err := scriptRecordRepo.Get(repo.WithByTaskID(taskID))
	if err != nil {
		return dto.ScriptLog{}, fmt.Errorf("record not found")
	}
	result := dto.ScriptLog{Offset: offset, Status: record.Status}
	if offset < 0 {
		result.Offset = 0
	}
	if record.Records == "" {
		return result, nil
	}
	f, err := os.Open(record.Records)
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

// RestoreScriptRecords marks in-flight script runs as failed after a service
// restart, mirroring RestoreCronjobs.
func RestoreScriptRecords() {
	database.DB.Model(&model.ScriptRecord{}).Where("status IN ?", []string{model.StatusRunning, model.StatusWaiting}).
		Updates(map[string]interface{}{"status": model.StatusFailed, "message": "interrupted by service restart"})
}

// loadScriptGroupMap returns a map of script-group id -> name used to resolve
// the human-readable group names attached to library scripts.
func loadScriptGroupMap() (map[uint]string, error) {
	groups, err := groupRepo.GetList(repo.WithByType("script"))
	if err != nil {
		return nil, err
	}
	groupMap := make(map[uint]string, len(groups))
	for _, group := range groups {
		groupMap[group.ID] = group.Name
	}
	return groupMap, nil
}

// parseScriptGroups splits a comma-separated group id list into the sorted id
// slice and its matching names, ignoring unknown/empty entries.
func parseScriptGroups(groups string, groupMap map[uint]string) ([]uint, []string) {
	var groupList []uint
	var groupBelong []string
	for _, idItem := range strings.Split(groups, ",") {
		id := parseUint(idItem)
		if id == 0 {
			continue
		}
		groupList = append(groupList, id)
		if name, ok := groupMap[id]; ok {
			groupBelong = append(groupBelong, name)
		}
	}
	return groupList, groupBelong
}

// normalizeScriptGroups keeps only the ids that still exist as script groups,
// producing a clean comma-separated string for storage.
func normalizeScriptGroups(groups string) string {
	groupMap, err := loadScriptGroupMap()
	if err != nil {
		return strings.Trim(groups, ",")
	}
	seen := make(map[uint]struct{})
	var ids []string
	for _, idItem := range strings.Split(groups, ",") {
		id := parseUint(idItem)
		if id == 0 {
			continue
		}
		if _, ok := groupMap[id]; !ok {
			continue
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, idItem)
	}
	return strings.Join(ids, ",")
}

func parseUint(s string) uint {
	id, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return 0
	}
	if id <= 0 {
		return 0
	}
	return uint(id)
}
