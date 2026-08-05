package scheduler

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/2panel-dev/2panel/internal/model"
	"github.com/robfig/cron/v3"
)

type Runner struct {
	dataDir string
	cron    *cron.Cron
	cancel  map[string]context.CancelFunc
	mu      sync.RWMutex
}

var (
	runner *Runner
	once   sync.Once
)

func GetRunner() *Runner {
	return runner
}

func Init(dataDir string) *Runner {
	once.Do(func() {
		runner = &Runner{
			dataDir: dataDir,
			cron: cron.New(
				cron.WithChain(cron.Recover(cron.DefaultLogger)),
				cron.WithChain(cron.DelayIfStillRunning(cron.DefaultLogger)),
			),
			cancel: make(map[string]context.CancelFunc),
		}
		runner.cron.Start()
	})
	return runner
}

func (r *Runner) Cron() *cron.Cron {
	return r.cron
}

func (r *Runner) DataDir() string {
	return r.dataDir
}

// Register adds cron entries for the job and returns comma-joined entry IDs.
// The job spec may contain multiple specs joined by "&&" (like 1Panel), each
// of which is registered as its own cron entry.
func (r *Runner) Register(job *model.Cronjob, handle func(*model.Cronjob)) (string, error) {
	specs := strings.Split(job.Spec, "&&")
	ids := make([]string, 0, len(specs))
	for _, spec := range specs {
		spec = strings.TrimSpace(spec)
		if len(spec) == 0 {
			continue
		}
		entryID, err := r.cron.AddFunc(spec, func() {
			handle(job)
		})
		if err != nil {
			for _, id := range ids {
				var entryID cron.EntryID
				if _, err := fmt.Sscanf(id, "%d", &entryID); err == nil {
					r.cron.Remove(entryID)
				}
			}
			return "", err
		}
		ids = append(ids, strconv.Itoa(int(entryID)))
	}
	if len(ids) == 0 {
		return "", fmt.Errorf("empty cron spec")
	}
	return strings.Join(ids, ","), nil
}

// Remove removes cron entries by the stored entry ID string.
func (r *Runner) Remove(entryIDs string) {
	if len(entryIDs) == 0 {
		return
	}
	for _, id := range strings.Split(entryIDs, ",") {
		var entryID cron.EntryID
		if _, err := fmt.Sscanf(id, "%d", &entryID); err != nil {
			continue
		}
		r.cron.Remove(entryID)
	}
}

func (r *Runner) Stop(taskID string) {
	r.mu.RLock()
	cancel, ok := r.cancel[taskID]
	r.mu.RUnlock()
	if ok {
		cancel()
	}
}

func (r *Runner) RunJob(job *model.Cronjob, record *model.JobRecord, log *LogWriter) error {
	ctx, cancel := context.WithCancel(context.Background())
	r.mu.Lock()
	r.cancel[record.TaskID] = cancel
	r.mu.Unlock()

	defer func() {
		cancel()
		r.mu.Lock()
		delete(r.cancel, record.TaskID)
		r.mu.Unlock()
	}()

	timeout := time.Duration(job.Timeout) * time.Second
	if timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	switch job.Type {
	case model.TypeShell:
		return r.runShell(ctx, job, log)
	case model.TypeCurl:
		return r.runCurl(ctx, job, log)
	}
	return fmt.Errorf("unsupported cronjob type: %s", job.Type)
}

func (r *Runner) runShell(ctx context.Context, job *model.Cronjob, log *LogWriter) error {
	executor := job.Executor
	if len(executor) == 0 {
		executor = "bash"
	}
	script := job.Script
	if len(strings.TrimSpace(script)) == 0 {
		return fmt.Errorf("the script content is empty")
	}

	ext := ".sh"
	if strings.HasPrefix(executor, "python") {
		ext = ".py"
	}
	jobDir := filepath.Join(r.dataDir, "task", job.Name)
	if err := os.MkdirAll(jobDir, 0755); err != nil {
		return err
	}
	scriptFile := filepath.Join(jobDir, job.Name+ext)
	if err := os.WriteFile(scriptFile, []byte(script), 0755); err != nil {
		return err
	}

	var cmd *exec.Cmd
	if len(job.User) == 0 {
		cmd = exec.CommandContext(ctx, executor, scriptFile)
	} else {
		cmd = exec.CommandContext(ctx, "sudo", "-u", job.User, executor, scriptFile)
	}
	return r.exec(ctx, cmd, log)
}

func (r *Runner) runCurl(ctx context.Context, job *model.Cronjob, log *LogWriter) error {
	urls := strings.Split(job.URL, ",")
	for _, url := range urls {
		url = strings.TrimSpace(url)
		if len(url) == 0 {
			continue
		}
		log.Logf("handle curl %s", url)
		cmd := exec.CommandContext(ctx, "curl", "-sS", "-L", url)
		if err := r.exec(ctx, cmd, log); err != nil {
			return err
		}
	}
	return nil
}

func (r *Runner) exec(ctx context.Context, cmd *exec.Cmd, log *LogWriter) error {
	cmd.Stdout = log
	cmd.Stderr = log
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
		// Kill the whole process group so grandchildren (e.g. `sh` forking
		// `sleep`) release the stdout pipe and Wait() can return.
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		select {
		case <-done:
		case <-time.After(5 * time.Second):
		}
		return ctx.Err()
	}
}
