package handler

import (
	"net/http"

	"github.com/2panel-dev/2panel/internal/dto"
	"github.com/2panel-dev/2panel/internal/service"
)

var fileWatchService service.FileWatchService

type FileWatchApi struct{}

func (a *FileWatchApi) Create(w http.ResponseWriter, r *http.Request) {
	var req dto.FileWatchOperate
	if err := decode(&req, w, r); err != nil {
		return
	}
	if err := fileWatchService.Create(req); err != nil {
		Error(w, http.StatusBadRequest, err)
		return
	}
	Success(w)
}

func (a *FileWatchApi) Search(w http.ResponseWriter, r *http.Request) {
	var req dto.SearchCronjob
	if err := decode(&req, w, r); err != nil {
		return
	}
	total, list, err := fileWatchService.SearchWithPage(req)
	if err != nil {
		InternalServer(w, err)
		return
	}
	SuccessWithData(w, dto.PageResult{Items: list, Total: total})
}

func (a *FileWatchApi) Stats(w http.ResponseWriter, r *http.Request) {
	stats, err := fileWatchService.Stats()
	if err != nil {
		InternalServer(w, err)
		return
	}
	SuccessWithData(w, stats)
}

func (a *FileWatchApi) LoadInfo(w http.ResponseWriter, r *http.Request) {
	var req dto.OperateByID
	if err := decode(&req, w, r); err != nil {
		return
	}
	data, err := fileWatchService.LoadInfo(req.ID)
	if err != nil {
		InternalServer(w, err)
		return
	}
	SuccessWithData(w, data)
}

func (a *FileWatchApi) Update(w http.ResponseWriter, r *http.Request) {
	var req dto.FileWatchOperate
	if err := decode(&req, w, r); err != nil {
		return
	}
	if err := fileWatchService.Update(req.ID, req); err != nil {
		Error(w, http.StatusBadRequest, err)
		return
	}
	Success(w)
}

func (a *FileWatchApi) Delete(w http.ResponseWriter, r *http.Request) {
	var req dto.CronjobBatchDelete
	if err := decode(&req, w, r); err != nil {
		return
	}
	if err := fileWatchService.Delete(req); err != nil {
		InternalServer(w, err)
		return
	}
	Success(w)
}

func (a *FileWatchApi) UpdateStatus(w http.ResponseWriter, r *http.Request) {
	var req dto.CronjobUpdateStatus
	if err := decode(&req, w, r); err != nil {
		return
	}
	if err := fileWatchService.UpdateStatus(req.ID, req.Status); err != nil {
		InternalServer(w, err)
		return
	}
	Success(w)
}

func (a *FileWatchApi) HandleOnce(w http.ResponseWriter, r *http.Request) {
	var req dto.OperateByID
	if err := decode(&req, w, r); err != nil {
		return
	}
	if err := fileWatchService.HandleOnce(req.ID); err != nil {
		InternalServer(w, err)
		return
	}
	Success(w)
}

func (a *FileWatchApi) SearchRecords(w http.ResponseWriter, r *http.Request) {
	var req dto.SearchFileWatchRecord
	if err := decode(&req, w, r); err != nil {
		return
	}
	total, list, err := fileWatchService.SearchRecords(req)
	if err != nil {
		InternalServer(w, err)
		return
	}
	SuccessWithData(w, dto.PageResult{Items: list, Total: total})
}

func (a *FileWatchApi) RecordLog(w http.ResponseWriter, r *http.Request) {
	var req dto.OperateByID
	if err := decode(&req, w, r); err != nil {
		return
	}
	content := fileWatchService.LoadRecordLog(req.ID)
	SuccessWithData(w, content)
}

func (a *FileWatchApi) RecordLogTail(w http.ResponseWriter, r *http.Request) {
	var req dto.RecordLogTailReq
	if err := decode(&req, w, r); err != nil {
		return
	}
	data, err := fileWatchService.ReadRecordLogTail(req)
	if err != nil {
		InternalServer(w, err)
		return
	}
	SuccessWithData(w, data)
}

func (a *FileWatchApi) ScriptOptions(w http.ResponseWriter, r *http.Request) {
	items, err := fileWatchService.ScriptOptions()
	if err != nil {
		InternalServer(w, err)
		return
	}
	SuccessWithData(w, items)
}