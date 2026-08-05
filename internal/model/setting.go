package model

type Setting struct {
	Key   string `gorm:"primarykey" json:"key"`
	Value string `json:"value"`
}
