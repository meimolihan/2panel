package dto

import "time"

// FileWatchOperate is the create/update payload for a file-watch conditional
// task. Paths is a slice of absolute file/directory paths; it is stored
// newline-separated on the model and round-tripped by LoadInfo.
type FileWatchOperate struct {
	ID       uint     `json:"id"`
	Name     string   `json:"name"`
	Paths    []string `json:"paths"`
	Events   []string `json:"events"`
	Comment  string   `json:"comment"`
	Executor string   `json:"executor"`
	Script   string   `json:"script"`
	// ScriptName references a script from the script library.
	ScriptName string `json:"scriptName"`
	User       string `json:"user"`

	Debounce     uint `json:"debounce"`
	Timeout      uint `json:"timeout"`
	RetainCopies int  `json:"retainCopies"`
}

// FileWatchInfo is a row of the file-watch list view. Watching reports whether
// a live inotify watcher is registered for the task right now (enabled AND
// successfully registered), which is what the UI switch should reflect.
type FileWatchInfo struct {
	ID         uint      `json:"id"`
	Name       string    `json:"name"`
	Paths      []string  `json:"paths"`
	Events     []string  `json:"events"`
	Comment    string    `json:"comment"`
	Executor   string    `json:"executor"`
	Script     string    `json:"script"`
	ScriptName string    `json:"scriptName"`
	User       string    `json:"user"`
	Debounce   uint      `json:"debounce"`
	Timeout    uint      `json:"timeout"`
	Status     string    `json:"status"`
	Watching   bool      `json:"watching"`
	CreatedAt  time.Time `json:"createdAt"`

	RetainCopies int `json:"retainCopies"`

	LastRecordStatus string `json:"lastRecordStatus"`
	LastRecordTime   string `json:"lastRecordTime"`
}

// FileWatchRecord is a single triggered execution for the record list.
type FileWatchRecord struct {
	ID        uint      `json:"id"`
	WatchID   uint      `json:"watchID"`
	WatchName string    `json:"watchName"`
	TaskID    string    `json:"taskID"`
	EventPath string    `json:"eventPath"`
	EventType string    `json:"eventType"`
	StartTime string    `json:"startTime"`
	Interval  float64   `json:"interval"`
	Status    string    `json:"status"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"createdAt"`
}

// SearchFileWatchRecord is the paged record search for one file-watch task.
type SearchFileWatchRecord struct {
	Page
	WatchID uint   `json:"watchID"`
	Status  string `json:"status"`
}