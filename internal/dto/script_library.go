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
