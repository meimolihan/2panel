package handler

import (
	"net/http"

	"github.com/2panel-dev/2panel/internal/dto"
	"github.com/2panel-dev/2panel/internal/service"
)

var scriptService service.ScriptService

type ScriptApi struct{}

func (a *ScriptApi) Search(w http.ResponseWriter, r *http.Request) {
	var req dto.ScriptSearch
	if err := decode(&req, w, r); err != nil {
		return
	}
	total, list, err := scriptService.SearchWithPage(req)
	if err != nil {
		InternalServer(w, err)
		return
	}
	SuccessWithData(w, dto.PageResult{Items: list, Total: total})
}

func (a *ScriptApi) LoadInfo(w http.ResponseWriter, r *http.Request) {
	var req dto.OperateByID
	if err := decode(&req, w, r); err != nil {
		return
	}
	data, err := scriptService.LoadInfo(req.ID)
	if err != nil {
		InternalServer(w, err)
		return
	}
	SuccessWithData(w, data)
}

func (a *ScriptApi) Create(w http.ResponseWriter, r *http.Request) {
	var req dto.ScriptOperate
	if err := decode(&req, w, r); err != nil {
		return
	}
	if err := scriptService.Create(req); err != nil {
		InternalServer(w, err)
		return
	}
	Success(w)
}

func (a *ScriptApi) Update(w http.ResponseWriter, r *http.Request) {
	var req dto.ScriptOperate
	if err := decode(&req, w, r); err != nil {
		return
	}
	if err := scriptService.Update(req); err != nil {
		InternalServer(w, err)
		return
	}
	Success(w)
}

func (a *ScriptApi) Delete(w http.ResponseWriter, r *http.Request) {
	var req dto.CronjobBatchDelete
	if err := decode(&req, w, r); err != nil {
		return
	}
	if err := scriptService.Delete(req.IDs); err != nil {
		InternalServer(w, err)
		return
	}
	Success(w)
}

func (a *ScriptApi) Run(w http.ResponseWriter, r *http.Request) {
	var req dto.ScriptRunReq
	if err := decode(&req, w, r); err != nil {
		return
	}
	taskID, err := scriptService.Run(req.ID)
	if err != nil {
		InternalServer(w, err)
		return
	}
	SuccessWithData(w, taskID)
}

func (a *ScriptApi) StopRun(w http.ResponseWriter, r *http.Request) {
	var req dto.ScriptRunStopReq
	if err := decode(&req, w, r); err != nil {
		return
	}
	if err := scriptService.StopRun(req.TaskID); err != nil {
		InternalServer(w, err)
		return
	}
	Success(w)
}

func (a *ScriptApi) SearchRunRecords(w http.ResponseWriter, r *http.Request) {
	var req dto.ScriptRecordSearch
	if err := decode(&req, w, r); err != nil {
		return
	}
	total, list, err := scriptService.SearchRunRecords(req)
	if err != nil {
		InternalServer(w, err)
		return
	}
	SuccessWithData(w, dto.PageResult{Items: list, Total: total})
}

func (a *ScriptApi) ClearRunRecords(w http.ResponseWriter, r *http.Request) {
	var req dto.OperateByID
	if err := decode(&req, w, r); err != nil {
		return
	}
	if err := scriptService.ClearRecords(req.ID); err != nil {
		InternalServer(w, err)
		return
	}
	Success(w)
}

func (a *ScriptApi) RunLog(w http.ResponseWriter, r *http.Request) {
	var req dto.ScriptLogReq
	if err := decode(&req, w, r); err != nil {
		return
	}
	data, err := scriptService.LoadRunLog(req.TaskID, req.Offset)
	if err != nil {
		InternalServer(w, err)
		return
	}
	SuccessWithData(w, data)
}
