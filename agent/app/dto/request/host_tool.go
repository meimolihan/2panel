package request

type HostToolTypeReq struct {
	Type string `json:"type" validate:"required"`
}

type HostToolCreate struct {
	Type        string `json:"type" validate:"required"`
	ConfigPath  string `json:"configPath"`
	ServiceName string `json:"serviceName"`
}

type HostToolOperateReq struct {
	Type    string `json:"type" validate:"required"`
	Operate string `json:"operate" validate:"required"`
}

type HostToolConfigUpdate struct {
	Type    string `json:"type" validate:"required"`
	Content string `json:"content"`
}

type SupervisorProcessConfig struct {
	Operate     string `json:"operate"`
	Name        string `json:"name" validate:"required"`
	Command     string `json:"command"`
	Dir         string `json:"dir"`
	User        string `json:"user"`
	AutoRestart string `json:"autoRestart"`
	AutoStart   string `json:"autoStart"`
	Numprocs    string `json:"numprocs"`
	Environment string `json:"environment"`
	File        string `json:"file"`
}

type HostSupervisorProcessFileGetReq struct {
	Name string `json:"name" validate:"required"`
	File string `json:"file"`
}

type SupervisorProcessFileReq struct {
	Name    string `json:"name" validate:"required"`
	Operate string `json:"operate"`
	Content string `json:"content"`
	File    string `json:"file"`
}

type HostSupervisorProcessFileOperateReq struct {
	Name    string `json:"name" validate:"required"`
	Operate string `json:"operate"`
	Content string `json:"content"`
	File    string `json:"file"`
}
