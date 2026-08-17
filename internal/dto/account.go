package dto

// AccountListResp is the response from POST /api/accounts.
type AccountListResp struct {
	Data    []string `json:"data"`
	Default string   `json:"default"`
	POSIX   bool     `json:"posixSupported"`
}
