package model

import "time"

// ScriptRecord records a single execution ("install"/run) of a script from the
// script library, mirroring the cronjob run history so every run can be
// inspected later.
type ScriptRecord struct {
	BaseModel

	TaskID     string    `json:"taskID"`
	ScriptID   uint      `json:"scriptID"`
	ScriptName string    `json:"scriptName"`
	StartTime  time.Time `json:"startTime"`
	Interval   float64   `json:"interval"`
	Records    string    `json:"records"`
	Status     string    `json:"status"`
	Message    string    `json:"message"`
}
