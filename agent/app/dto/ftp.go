package dto

type FtpBaseInfo struct {
	IsActive bool `json:"isActive"`
	IsExist  bool `json:"isExist"`
}

type FtpCreate struct {
	User        string `json:"user" validate:"required"`
	Password    string `json:"password" validate:"required"`
	Path        string `json:"path" validate:"required"`
	Isolation   int    `json:"isolation"`
	Status      string `json:"status"`
	Description string `json:"description"`
}

type FtpUpdate struct {
	ID          uint   `json:"id" validate:"required"`
	Password    string `json:"password"`
	Path        string `json:"path"`
	Isolation   int    `json:"isolation"`
	Status      string `json:"status"`
	Description string `json:"description"`
}

type FtpLogSearch struct {
	User      string `json:"user"`
	Operation string `json:"operation"`
	IP        string `json:"ip"`
	PageInfo
}

type FtpInfo struct {
	ID        uint   `json:"id"`
	User      string `json:"user"`
	Password  string `json:"password"`
	RootPath  string `json:"rootPath"`
	Isolation int    `json:"isolation"`
	Status    string `json:"status"`
}
