package handler

import (
	"net/http"

	"github.com/2panel-dev/2panel/internal/dto"
	"github.com/2panel-dev/2panel/internal/service"
)

var groupService service.GroupService

type GroupApi struct{}

func (a *GroupApi) Search(w http.ResponseWriter, r *http.Request) {
	var req dto.GroupSearch
	if err := decode(&req, w, r); err != nil {
		return
	}
	list, err := groupService.List(req)
	if err != nil {
		InternalServer(w, err)
		return
	}
	SuccessWithData(w, list)
}

func (a *GroupApi) Create(w http.ResponseWriter, r *http.Request) {
	var req dto.GroupCreate
	if err := decode(&req, w, r); err != nil {
		return
	}
	if err := groupService.Create(req); err != nil {
		Error(w, http.StatusBadRequest, err)
		return
	}
	Success(w)
}

func (a *GroupApi) Update(w http.ResponseWriter, r *http.Request) {
	var req dto.GroupUpdate
	if err := decode(&req, w, r); err != nil {
		return
	}
	if err := groupService.Update(req); err != nil {
		Error(w, http.StatusBadRequest, err)
		return
	}
	Success(w)
}

func (a *GroupApi) Delete(w http.ResponseWriter, r *http.Request) {
	var req dto.OperateByID
	if err := decode(&req, w, r); err != nil {
		return
	}
	if err := groupService.Delete(req.ID); err != nil {
		Error(w, http.StatusBadRequest, err)
		return
	}
	Success(w)
}
