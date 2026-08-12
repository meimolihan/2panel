package scheduler

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
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

// everyTokenRe matches a numeric duration token (with optional unit) inside an
// @every spec. Go's time.ParseDuration lacks day/week units, so they are
// normalized to hours before scheduling or previewing.
var everyTokenRe = regexp.MustCompile(`^(\d+(?:\.\d+)?)(ns|us|µs|μs|ms|s|m|h|d|w)`)

// NormalizeEverySpec rewrites an "@every <duration>" spec into a Go-duration
// form (e.g. "@every 1d" -> "@every 24h0m0s") so it is accepted everywhere.
// A bare number is treated as minutes for compatibility with older saves.
// Non-every specs are returned unchanged.
func NormalizeEverySpec(spec string) (string, error) {
	rest, found := strings.CutPrefix(spec, "@every ")
	if !found {
		return spec, nil
	}
	rest = strings.TrimSpace(rest)
	if len(rest) == 0 {
		return "", fmt.Errorf("invalid @every interval")
	}
	// bare number = minutes, for compatibility with older saves
	if n, err := strconv.Atoi(rest); err == nil {
		return "@every " + (time.Duration(n) * time.Minute).String(), nil
	}
	var total time.Duration
	s := rest
	for len(s) > 0 {
		m := everyTokenRe.FindStringSubmatch(s)
		if m == nil {
			return "", fmt.Errorf("invalid @every interval: %s", rest)
		}
		n, err := strconv.ParseFloat(m[1], 64)
		if err != nil {
			return "", fmt.Errorf("invalid @every interval: %s", rest)
		}
		var mult time.Duration
		switch m[2] {
		case "ns":
			mult = time.Nanosecond
		case "us", "µs", "μs":
			mult = time.Microsecond
		case "ms":
			mult = time.Millisecond
		case "s":
			mult = time.Second
		case "m":
			mult = time.Minute
		case "h":
			mult = time.Hour
		case "d":
			mult = 24 * time.Hour
		case "w":
			mult = 7 * 24 * time.Hour
		}
		total += time.Duration(n * float64(mult))
		s = strings.TrimPrefix(s, m[0])
	}
	if total < time.Second {
		total = time.Second
	}
	return "@every " + total.String(), nil
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
		normalized, err := NormalizeEverySpec(spec)
		if err != nil {
			for _, id := range ids {
				var entryID cron.EntryID
				if _, err := fmt.Sscanf(id, "%d", &entryID); err == nil {
					r.cron.Remove(entryID)
				}
			}
			return "", err
		}
		entryID, err := r.cron.AddFunc(normalized, func() {
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

// ResetEntries removes every scheduled cron entry. It is used before
// re-registering jobs from a freshly restored database to avoid double
// scheduling of the old entries.
func (r *Runner) ResetEntries() {
	for _, e := range r.cron.Entries() {
		r.cron.Remove(e.ID)
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

// NonInteractiveEnv returns the current environment with TERM forced to "dumb"
// (replacing any inherited value, since a duplicate TERM entry may be ignored
// by the child). Scripts run by the scheduler are non-interactive: terminal
// tools like clear no longer error out, and well-behaved tools skip color.
func NonInteractiveEnv() []string {
	env := make([]string, 0, len(os.Environ())+1)
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, "TERM=") {
			continue
		}
		env = append(env, kv)
	}
	return append(env, "TERM=dumb")
}

func (r *Runner) exec(ctx context.Context, cmd *exec.Cmd, log *LogWriter) error {
	cmd.Stdout = log
	cmd.Stderr = log
	// Non-interactive execution: a single newline on stdin lets trailing
	// "press any key to continue" prompts (read returns 1 on EOF) finish with
	// exit code 0, and TERM=dumb stops terminal tools such as clear from
	// printing "TERM environment variable not set." into the log.
	if cmd.Stdin == nil {
		cmd.Stdin = strings.NewReader("\n")
	}
	if cmd.Env == nil {
		cmd.Env = NonInteractiveEnv()
	}
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
