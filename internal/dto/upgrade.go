package dto

// VersionInfo describes the running build reported by /api/version.
type VersionInfo struct {
	Version string `json:"version"`
	Build   string `json:"build"`
	// Updatable is false for dev builds or versions that were not released
	// through the OTA pipeline, in which case the UI hides the upgrade button.
	Updatable bool `json:"updatable"`
}

// UpdateInfo is the result of an upgrade check: whether a newer release
// exists, plus its changelog so the UI can render an update card.
type UpdateInfo struct {
	Current     string `json:"current"`
	Latest      string `json:"latest"`
	HasUpdate   bool   `json:"hasUpdate"`
	PublishedAt string `json:"publishedAt"`
	Changelog   string `json:"changelog"`
	Updatable   bool   `json:"updatable"`
}

// UpgradeStatus reports the in-progress (or last) upgrade so the frontend can
// render live progress and know when the service restarted.
type UpgradeStatus struct {
	Running    bool     `json:"running"`
	State      string   `json:"state"`
	NewVersion string   `json:"newVersion"`
	Log        []string `json:"log"`
}
