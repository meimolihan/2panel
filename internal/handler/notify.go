package handler

import (
	"net/http"

	"github.com/2panel-dev/2panel/internal/dto"
	"github.com/2panel-dev/2panel/internal/service"
)

var notifyService service.NotifyService

type NotifyApi struct{}

func (a *NotifyApi) LoadConfig(w http.ResponseWriter, r *http.Request) {
	SuccessWithData(w, notifyService.LoadConfig())
}

func (a *NotifyApi) SaveConfig(w http.ResponseWriter, r *http.Request) {
	var req dto.BarkConfig
	if err := decode(&req, w, r); err != nil {
		return
	}
	if err := notifyService.SaveConfig(req); err != nil {
		Error(w, http.StatusBadRequest, err)
		return
	}
	Success(w)
}

func (a *NotifyApi) Test(w http.ResponseWriter, r *http.Request) {
	if err := notifyService.Test(); err != nil {
		Error(w, http.StatusBadRequest, err)
		return
	}
	Success(w)
}
