package dto

import (
	"github.com/2Panel-dev/2Panel/agent/app/model"
)

type SearchTaskLogReq struct {
	Status string `json:"status"`
	Type   string `json:"type"`
	TaskID string `json:"taskID"`
	PageInfo
}

type TaskDTO struct {
	model.Task
}
