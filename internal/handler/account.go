package handler

import (
	"net/http"

	"github.com/2panel-dev/2panel/internal/service"
)

type AccountApi struct{}

func (a *AccountApi) List(w http.ResponseWriter, r *http.Request) {
	resp, err := service.ListAccounts()
	if err != nil {
		InternalServer(w, err)
		return
	}
	SuccessWithData(w, resp)
}
