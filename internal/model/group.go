package model

type Group struct {
	BaseModel

	IsDefault bool   `gorm:"default:false" json:"isDefault"`
	Name      string `gorm:"not null" json:"name"`
	Type      string `gorm:"not null;index" json:"type"`
}
