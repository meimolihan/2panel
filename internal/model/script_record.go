package model

import "time"

// ScriptRecord records a single execution ("install"/run) of a script from the
// script library, mirroring the cronjob run history so every run can be
// inspected later.
type ScriptRecord struct {
	BaseModel

	TaskID     string `json:"taskID"`
	ScriptID   uint   `json:"scriptID"`
	ScriptName string `json:"scriptName"`
	// Status is declared before StartTime on purpose: GORM emits index
	// columns in field order, and status = ? must lead the composite index
	// for the planner to seek into it. Index names are distinct per table
	// because SQLite scopes index names across the whole database.
	Status    string    `gorm:"index:idx_scriptrec_status_start" json:"status"`
	StartTime time.Time `gorm:"index:idx_scriptrec_status_start" json:"startTime"`
	Interval  float64   `json:"interval"`
	Records   string    `json:"records"`
	Message   string    `json:"message"`
}
