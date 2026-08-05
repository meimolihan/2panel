package dto

import (
	"time"
)

type Page struct {
	Page     int `json:"page"`
	PageSize int `json:"pageSize"`
}

type SearchCronjob struct {
	Page
	Info    string `json:"info"`
	Type    string `json:"type"`
	OrderBy string `json:"orderBy"`
	Order   string `json:"order"`
}

type CronjobOperate struct {
	ID         uint   `json:"id"`
	Name       string `json:"name" binding:"required"`
	Type       string `json:"type" binding:"required,oneof=shell curl"`
	Spec       string `json:"spec" binding:"required"`
	SpecCustom bool   `json:"specCustom"`

	Executor string `json:"executor"`
	Script   string `json:"script"`
	// ScriptName references a script from the script library.
	ScriptName string `json:"scriptName"`
	User       string `json:"user"`
	URL        string `json:"url"`

	RetryTimes uint `json:"retryTimes"`
	Timeout    uint `json:"timeout"`

	RetainCopies int `json:"retainCopies"`
}

type CronjobInfo struct {
	ID          uint      `json:"id"`
	Name        string    `json:"name"`
	Type        string    `json:"type"`
	Spec        string    `json:"spec"`
	SpecCustom  bool      `json:"specCustom"`
	Executor    string    `json:"executor"`
	Script      string    `json:"script"`
	ScriptName  string    `json:"scriptName"`
	User        string    `json:"user"`
	URL         string    `json:"url"`
	RetryTimes  uint      `json:"retryTimes"`
	Timeout     uint      `json:"timeout"`
	IsExecuting bool      `json:"isExecuting"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"createdAt"`

	RetainCopies int `json:"retainCopies"`

	LastRecordStatus string `json:"lastRecordStatus"`
	LastRecordTime   string `json:"lastRecordTime"`
	NextRunTime      string `json:"nextRunTime"`
}

type OperateByID struct {
	ID uint `json:"id" binding:"required"`
}

type CronjobSpec struct {
	Spec string `json:"spec" binding:"required"`
}

type CronjobBatchDelete struct {
	IDs []uint `json:"ids" binding:"required"`
}

type CronjobUpdateStatus struct {
	ID     uint   `json:"id" binding:"required"`
	Status string `json:"status" binding:"required,oneof=enabled disabled"`
}

type SearchRecord struct {
	Page
	CronjobID uint   `json:"cronjobID"`
	Status    string `json:"status"`
}

type Record struct {
	ID        uint      `json:"id"`
	CronjobID uint      `json:"cronjobID"`
	TaskID    string    `json:"taskID"`
	StartTime string    `json:"startTime"`
	Interval  float64   `json:"interval"`
	Status    string    `json:"status"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"createdAt"`
}

type PageResult struct {
	Items interface{} `json:"items"`
	Total int64       `json:"total"`
}

// CronjobImport holds an array of cronjob payloads to create on import.
type CronjobImport struct {
	Data []CronjobOperate `json:"data"`
}

type CronjobImportResult struct {
	Created int `json:"created"`
	Skipped int `json:"skipped"`
	Failed  int `json:"failed"`
}

// ScriptOption is a lightweight entry for the cronjob editor's library select.
type ScriptOption struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
}
