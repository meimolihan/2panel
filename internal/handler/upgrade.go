package handler

import (
	"net/http"

	"github.com/2panel-dev/2panel/internal/dto"
	"github.com/2panel-dev/2panel/internal/upgrade"
)

type UpgradeApi struct{}

// Version reports the running version/build and whether OTA is available.
func (a *UpgradeApi) Version(w http.ResponseWriter, r *http.Request) {
	SuccessWithData(w, dto.VersionInfo{
		Version:   upgrade.CurrentVersion(),
		Build:     upgrade.CurrentBuild(),
		Updatable: upgrade.Enabled(),
	})
}

// Check queries the release source for a newer version.
func (a *UpgradeApi) Check(w http.ResponseWriter, r *http.Request) {
	info, err := upgrade.Check()
	if err != nil {
		InternalServer(w, err)
		return
	}
	SuccessWithData(w, info)
}

// Upgrade starts the download + verify + swap + restart pipeline.
func (a *UpgradeApi) Upgrade(w http.ResponseWriter, r *http.Request) {
	if err := upgrade.Perform(); err != nil {
		Error(w, http.StatusBadRequest, err)
		return
	}
	SuccessWithData(w, upgrade.Status())
}

// Status returns the current upgrade progress for polling.
func (a *UpgradeApi) Status(w http.ResponseWriter, r *http.Request) {
	SuccessWithData(w, upgrade.Status())
}

// Restart restarts the 2Panel service.
func (a *UpgradeApi) Restart(w http.ResponseWriter, r *http.Request) {
	if err := upgrade.Restart(); err != nil {
		Error(w, http.StatusBadRequest, err)
		return
	}
	SuccessWithData(w, upgrade.Status())
}
