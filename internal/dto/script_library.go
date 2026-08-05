package dto

import "time"

type ScriptSearch struct {
	Page
	Info string `json:"info"`
}

type ScriptOperate struct {
	ID          uint   `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Script      string `json:"script"`
}

type ScriptInfo struct {
	ID          uint      `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Script      string    `json:"script"`
	CreatedAt   time.Time `json:"createdAt"`
}

type ScriptRunReq struct {
	ID uint `json:"id"`
}

type ScriptRunStopReq struct {
	TaskID string `json:"taskID"`
}

type ScriptLogReq struct {
	TaskID string `json:"taskID"`
}

type ScriptRecordSearch struct {
	Page
	ScriptID uint   `json:"scriptID"`
	Status   string `json:"status"`
}

type ScriptRecord struct {
	ID         uint    `json:"id"`
	ScriptID   uint    `json:"scriptID"`
	ScriptName string  `json:"scriptName"`
	TaskID     string  `json:"taskID"`
	StartTime  string  `json:"startTime"`
	Interval   float64 `json:"interval"`
	Status     string  `json:"status"`
	Message    string  `json:"message"`
}

type ScriptLog struct {
	Content string `json:"content"`
	Status  string `json:"status"`
}
