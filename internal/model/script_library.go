package model

type ScriptLibrary struct {
	BaseModel

	Name        string `gorm:"not null;uniqueIndex" json:"name"`
	Description string `json:"description"`
	Script      string `gorm:"type:text" json:"script"`
}
