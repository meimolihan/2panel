package scheduler

import (
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/2panel-dev/2panel/internal/model"
)

// watchEventOp maps the persisted event names (model.EventWrite/Create/Remove)
// to the fsnotify.Op bits that should trigger a run. A rename away from the
// watched path is treated as a remove because the original path is gone.
var watchEventOp = map[string]fsnotify.Op{
	model.EventWrite:  fsnotify.Write,
	model.EventCreate: fsnotify.Create,
	model.EventRemove: fsnotify.Remove | fsnotify.Rename,
}

// watchEntry is a single live inotify watcher for one enabled FileWatch task.
type watchEntry struct {
	watch    *model.FileWatch
	watcher  *fsnotify.Watcher
	roots    []string
	mu       sync.Mutex
	lastSeen map[string]time.Time
	done     chan struct{}
	once     sync.Once
}

// WatchManager owns every registered fsnotify.Watcher. Each enabled FileWatch
// task gets exactly one watcher; Stop removes and closes it so inotify file
// descriptors are never leaked when a task is disabled or deleted.
type WatchManager struct {
	mu      sync.Mutex
	entries map[uint]*watchEntry
}

func newWatchManager() *WatchManager {
	return &WatchManager{entries: make(map[uint]*watchEntry)}
}

// Start registers a watcher for every path of the task and spawns the event
// loop. Paths that do not exist yet are re-added in the background so the task
// starts watching them as soon as they appear. handle is invoked synchronously
// per accepted event and must not block; the service layer launches the actual
// command asynchronously.
func (m *WatchManager) Start(watch *model.FileWatch, handle func(*model.FileWatch, string, string)) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// stop any previous watcher for the same task id before taking over
	if old, ok := m.entries[watch.ID]; ok {
		old.close()
	}

	entry := &watchEntry{
		watch: watch,
		roots: splitPaths(watch.Paths),
		done:  make(chan struct{}),
		// lastSeen stays empty so the very first event for any path after
		// registration is never suppressed by the debounce window.
		lastSeen: make(map[string]time.Time),
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	entry.watcher = watcher

	var retry []string
	for _, p := range entry.roots {
		if err := watcher.Add(p); err != nil {
			log.Printf("filewatch [%s] add path %s failed: %v", watch.Name, p, err)
			retry = append(retry, p)
		}
	}
	// Missing/un-addable paths are retried in the background rather than
	// failing the whole task; a directory may legitimately not exist yet.
	if len(retry) > 0 {
		go retryAdd(entry, retry)
	}

	m.entries[watch.ID] = entry
	go entry.loop(handle)
	return nil
}

// Stop closes and forgets the watcher for the given task id. It is idempotent
// and safe to call for disabled / never-started tasks.
func (m *WatchManager) Stop(id uint) {
	m.mu.Lock()
	entry, ok := m.entries[id]
	if ok {
		delete(m.entries, id)
	}
	m.mu.Unlock()
	if ok {
		entry.close()
	}
}

// StopAll closes every registered watcher. Used on graceful shutdown and before
// restoring a backup so no inotify descriptor survives.
func (m *WatchManager) StopAll() {
	m.mu.Lock()
	entries := m.entries
	m.entries = make(map[uint]*watchEntry)
	m.mu.Unlock()
	for _, e := range entries {
		e.close()
	}
}

// IsWatching reports whether a live watcher is registered for the task id.
func (m *WatchManager) IsWatching(id uint) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.entries[id]
	return ok
}

func (e *watchEntry) close() {
	e.once.Do(func() {
		close(e.done)
		_ = e.watcher.Close()
	})
}

// loop consumes fsnotify events for the lifetime of the watcher. It filters by
// the task's configured event types and applies the debounce window before
// invoking handle.
func (e *watchEntry) loop(handle func(*model.FileWatch, string, string)) {
	defer e.close()
	for {
		select {
		case ev, ok := <-e.watcher.Events:
			if !ok {
				return
			}
			// fsnotify reports IN_MOVED_FROM / IN_MOVED_TO as a single Rename
			// op. Editors and screenshot tools write a temp file and rename it
			// into place, so a rename INTO the watched tree means a new file
			// appeared (create) while a rename away means it is gone (remove).
			// Decide by whether the reported path still exists on disk.
			op := ev.Op
			if op&fsnotify.Rename != 0 {
				if _, err := os.Stat(ev.Name); err != nil {
					op = fsnotify.Remove
				} else {
					op = fsnotify.Create
				}
			}
			eventType := matchEvent(e.watch.Events, op)
			if eventType == "" {
				continue
			}
			// apply per-path debounce so a burst of events for the same file
			// (e.g. a busy log being written) collapses into a single run,
			// while distinct files still trigger independently.
			if e.watch.Debounce > 0 {
				e.mu.Lock()
				now := time.Now()
				if last, ok := e.lastSeen[ev.Name]; ok && now.Sub(last) < time.Duration(e.watch.Debounce)*time.Second {
					e.mu.Unlock()
					continue
				}
				e.lastSeen[ev.Name] = now
				e.mu.Unlock()
			}
			// if a configured root path itself disappeared, re-add it in the
			// background so the watch resumes when it comes back
			if eventType == model.EventRemove && contains(e.roots, ev.Name) {
				go retryAdd(e, []string{ev.Name})
			}
			handle(e.watch, eventType, ev.Name)
		case err, ok := <-e.watcher.Errors:
			if !ok {
				return
			}
			log.Printf("filewatch [%s] watcher error: %v", e.watch.Name, err)
		case <-e.done:
			return
		}
	}
}

// matchEvent translates an fsnotify.Op into the configured event name, or ""
// when the op is not enabled for this task.
func matchEvent(enabledEvents string, op fsnotify.Op) string {
	for _, name := range strings.Split(enabledEvents, ",") {
		name = strings.TrimSpace(name)
		if mask, ok := watchEventOp[name]; ok && op&mask != 0 {
			return name
		}
	}
	return ""
}

// splitPaths splits the newline-separated paths field into cleaned absolute
// paths, dropping empty lines.
func splitPaths(paths string) []string {
	var out []string
	for _, line := range strings.Split(paths, "\n") {
		p := strings.TrimSpace(line)
		if len(p) == 0 {
			continue
		}
		out = append(out, p)
	}
	return out
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// retryAdd periodically re-adds paths that could not be watched yet (they did
// not exist when the watcher started, or a watched root was deleted), until
// they register or the entry is closed. It only re-registers inotify watches;
// it never inspects file contents, so it is not a polling change-detection
// loop.
func retryAdd(e *watchEntry, paths []string) {
	for {
		select {
		case <-e.done:
			return
		case <-time.After(2 * time.Second):
		}
		missing := false
		for _, p := range paths {
			if err := e.watcher.Add(p); err != nil {
				missing = true
			}
		}
		if !missing {
			return
		}
	}
}