package model

import "time"

// File watch event types. Each maps to a fsnotify.Op and is persisted as the
// user-friendly name used by the frontend checkboxes.
const (
	EventWrite  = "write"
	EventCreate = "create"
	EventRemove = "remove"
)

// FileWatch is a conditional task triggered by file-system change events
// (inotify via fsnotify) on one or more watched paths.
//
// Paths holds one absolute path per line (file or directory). Events holds a
// comma-separated subset of write/create/remove. When an enabled watcher sees
// a matching event (after an optional debounce window) it asynchronously runs
// the configured command/script and records a FileWatchRecord with the full
// stdout/stderr log, mirroring the cronjob execution model.
//
// WatcherKey identifies the live inotify watcher inside the scheduler while
// the task is enabled and running; it is reset to "" when the watcher is
// stopped (disable/delete/restart) so the UI can always distinguish "enabled
// but not watching" from a leak.
type FileWatch struct {
	BaseModel

	Name    string `gorm:"not null;uniqueIndex" json:"name"`
	Paths   string `gorm:"type:text" json:"paths"`
	Events  string `gorm:"not null" json:"events"`
	Comment string `gorm:"type:text" json:"comment"`

	Executor string `gorm:"default:bash" json:"executor"`
	Script   string `gorm:"type:text" json:"script"`
	// ScriptName references a script from the script library; when set the
	// library content is resolved at run time.
	ScriptName string `json:"scriptName"`
	User       string `json:"user"`

	// Debounce is the minimum gap in seconds between two triggered runs for
	// the same path, collapsing a burst of events on one file (e.g. an editor
	// writing repeatedly) into a single run while distinct files still trigger.
	Debounce uint `json:"debounce"`
	Timeout  uint `json:"timeout"`

	RetainCopies uint64 `json:"retainCopies"`

	Status     string `json:"status"`
	WatcherKey string `json:"watcherKey"`
}

// FileWatchRecord is a single execution triggered by a file-system event.
// Records holds the log file path written by the LogWriter (same layout as
// JobRecord.Records).
type FileWatchRecord struct {
	BaseModel

	WatchID   uint      `gorm:"index" json:"watchID"`
	WatchName string    `json:"watchName"`
	TaskID    string    `gorm:"index" json:"taskID"`
	EventPath string    `json:"eventPath"`
	EventType string    `json:"eventType"`
	StartTime time.Time `json:"startTime"`
	Interval  float64   `json:"interval"`
	Records   string    `json:"records"`
	Status    string    `json:"status"`
	Message   string    `json:"message"`
}