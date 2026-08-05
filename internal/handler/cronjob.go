package handler

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/2panel-dev/2panel/internal/dto"
	"github.com/2panel-dev/2panel/internal/service"
)

var cronjobService service.CronjobService

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func Success(w http.ResponseWriter) {
	writeJSON(w, http.StatusOK, map[string]interface{}{"code": 0, "msg": "success"})
}

func SuccessWithData(w http.ResponseWriter, data interface{}) {
	writeJSON(w, http.StatusOK, map[string]interface{}{"code": 0, "msg": "success", "data": data})
}

func Error(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]interface{}{"code": 1, "msg": err.Error()})
}

func InternalServer(w http.ResponseWriter, err error) {
	log.Printf("cronjob api error: %v", err)
	Error(w, http.StatusInternalServerError, err)
}

func decode(req interface{}, w http.ResponseWriter, r *http.Request) error {
	if err := json.NewDecoder(r.Body).Decode(req); err != nil {
		Error(w, http.StatusBadRequest, err)
		return err
	}
	return nil
}

type CronjobApi struct{}

func (a *CronjobApi) Create(w http.ResponseWriter, r *http.Request) {
	var req dto.CronjobOperate
	if err := decode(&req, w, r); err != nil {
		return
	}
	if err := cronjobService.Create(req); err != nil {
		InternalServer(w, err)
		return
	}
	Success(w)
}

func (a *CronjobApi) Search(w http.ResponseWriter, r *http.Request) {
	var req dto.SearchCronjob
	if err := decode(&req, w, r); err != nil {
		return
	}
	total, list, err := cronjobService.SearchWithPage(req)
	if err != nil {
		InternalServer(w, err)
		return
	}
	SuccessWithData(w, dto.PageResult{Items: list, Total: total})
}

func (a *CronjobApi) LoadInfo(w http.ResponseWriter, r *http.Request) {
	var req dto.OperateByID
	if err := decode(&req, w, r); err != nil {
		return
	}
	data, err := cronjobService.LoadInfo(req.ID)
	if err != nil {
		InternalServer(w, err)
		return
	}
	SuccessWithData(w, data)
}

func (a *CronjobApi) Update(w http.ResponseWriter, r *http.Request) {
	var req dto.CronjobOperate
	if err := decode(&req, w, r); err != nil {
		return
	}
	if err := cronjobService.Update(req.ID, req); err != nil {
		InternalServer(w, err)
		return
	}
	Success(w)
}

func (a *CronjobApi) Delete(w http.ResponseWriter, r *http.Request) {
	var req dto.CronjobBatchDelete
	if err := decode(&req, w, r); err != nil {
		return
	}
	if err := cronjobService.Delete(req); err != nil {
		InternalServer(w, err)
		return
	}
	Success(w)
}

func (a *CronjobApi) UpdateStatus(w http.ResponseWriter, r *http.Request) {
	var req dto.CronjobUpdateStatus
	if err := decode(&req, w, r); err != nil {
		return
	}
	if err := cronjobService.UpdateStatus(req.ID, req.Status); err != nil {
		InternalServer(w, err)
		return
	}
	Success(w)
}

func (a *CronjobApi) HandleOnce(w http.ResponseWriter, r *http.Request) {
	var req dto.OperateByID
	if err := decode(&req, w, r); err != nil {
		return
	}
	if err := cronjobService.HandleOnce(req.ID); err != nil {
		InternalServer(w, err)
		return
	}
	Success(w)
}

func (a *CronjobApi) Stop(w http.ResponseWriter, r *http.Request) {
	var req dto.OperateByID
	if err := decode(&req, w, r); err != nil {
		return
	}
	if err := cronjobService.HandleStop(req.ID); err != nil {
		InternalServer(w, err)
		return
	}
	Success(w)
}

func (a *CronjobApi) Next(w http.ResponseWriter, r *http.Request) {
	var req dto.CronjobSpec
	if err := decode(&req, w, r); err != nil {
		return
	}
	list, err := cronjobService.LoadNextHandle(req.Spec)
	if err != nil {
		InternalServer(w, err)
		return
	}
	SuccessWithData(w, list)
}

func (a *CronjobApi) SearchRecords(w http.ResponseWriter, r *http.Request) {
	var req dto.SearchRecord
	if err := decode(&req, w, r); err != nil {
		return
	}
	total, list, err := cronjobService.SearchRecords(req)
	if err != nil {
		InternalServer(w, err)
		return
	}
	SuccessWithData(w, dto.PageResult{Items: list, Total: total})
}

func (a *CronjobApi) RecordLog(w http.ResponseWriter, r *http.Request) {
	var req dto.OperateByID
	if err := decode(&req, w, r); err != nil {
		return
	}
	content := cronjobService.LoadRecordLog(req.ID)
	SuccessWithData(w, content)
}

func (a *CronjobApi) Export(w http.ResponseWriter, r *http.Request) {
	items, err := cronjobService.Export()
	if err != nil {
		InternalServer(w, err)
		return
	}
	SuccessWithData(w, items)
}

func (a *CronjobApi) Import(w http.ResponseWriter, r *http.Request) {
	var req dto.CronjobImport
	if err := decode(&req, w, r); err != nil {
		return
	}
	result, err := cronjobService.Import(req)
	if err != nil {
		InternalServer(w, err)
		return
	}
	SuccessWithData(w, result)
}

func (a *CronjobApi) ScriptOptions(w http.ResponseWriter, r *http.Request) {
	items, err := cronjobService.ScriptOptions()
	if err != nil {
		InternalServer(w, err)
		return
	}
	SuccessWithData(w, items)
}
