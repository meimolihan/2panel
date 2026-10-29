package dto

// BarkConfig 是 Web 界面「推送通知」表单的读写结构。
type BarkConfig struct {
	Enable       bool   `json:"enable"`
	Key          string `json:"key"`
	Server       string `json:"server"`
	Icon         string `json:"icon"`
	Group        string `json:"group"`
	SoundSuccess string `json:"soundSuccess"`
	SoundError   string `json:"soundError"`
	SoundInfo    string `json:"soundInfo"`
	Timeout      int    `json:"timeout"`
}
