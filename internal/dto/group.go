package dto

type GroupCreate struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

type GroupUpdate struct {
	ID        uint   `json:"id"`
	Name      string `json:"name"`
	Type      string `json:"type"`
	IsDefault bool   `json:"isDefault"`
}

type GroupSearch struct {
	Type string `json:"type"`
}

type GroupInfo struct {
	ID        uint   `json:"id"`
	Name      string `json:"name"`
	Type      string `json:"type"`
	IsDefault bool   `json:"isDefault"`
}
