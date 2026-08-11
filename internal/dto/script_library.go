package dto

import "time"

type ScriptSearch struct {
	Page
	Info    string `json:"info"`
	GroupID uint   `json:"groupID"`
}

type ScriptOperate struct {
	ID          uint   `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Script      string `json:"script"`
	Groups      string `json:"groups"`
}

type ScriptInfo struct {
	ID          uint      `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Script      string    `json:"script"`
	GroupList   []uint    `json:"groupList"`
	GroupBelong []string  `json:"groupBelong"`
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
	Offset int64  `json:"offset"`
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
	Offset  int64  `json:"offset"`
	Status  string `json:"status"`
}
