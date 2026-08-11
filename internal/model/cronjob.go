package model

import "time"

const (
	StatusEnable   = "enabled"
	StatusDisable  = "disabled"
	StatusPending  = "pending"
	StatusWaiting  = "waiting"
	StatusRunning  = "running"
	StatusSuccess  = "success"
	StatusFailed   = "failed"
	StatusUnexecut = "unexecuted"
)

const (
	TypeShell = "shell"
	TypeCurl  = "curl"
)

type Cronjob struct {
	BaseModel

	Name       string `gorm:"not null;uniqueIndex" json:"name"`
	Type       string `gorm:"not null" json:"type"`
	Spec       string `gorm:"not null" json:"spec"`
	SpecCustom bool   `json:"specCustom"`

	Executor string `json:"executor"`
	Script   string `gorm:"type:text" json:"script"`
	// ScriptName references a script from the script library; when set the
	// library content is resolved at run time.
	ScriptName string `json:"scriptName"`
	User       string `json:"user"`
	URL        string `json:"url"`

	RetryTimes uint `json:"retryTimes"`
	Timeout    uint `json:"timeout"`

	RetainCopies uint64 `json:"retainCopies"`

	IsExecuting bool   `json:"isExecuting"`
	Status      string `json:"status"`
	EntryIDs    string `json:"entryIDs"`
}

type JobRecord struct {
	BaseModel

	CronjobID uint      `gorm:"index" json:"cronjobID"`
	TaskID    string    `gorm:"index" json:"taskID"`
	StartTime time.Time `json:"startTime"`
	Interval  float64   `json:"interval"`
	Records   string    `json:"records"`
	Status    string    `json:"status"`
	Message   string    `json:"message"`
}
