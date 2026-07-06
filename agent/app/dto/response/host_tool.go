package response

type HostToolRes struct {
	Type   string     `json:"type"`
	Config *Supervisor `json:"config"`
}

type Supervisor struct {
	ServiceName string `json:"serviceName"`
	IsExist     bool   `json:"isExist"`
	CtlExist    bool   `json:"ctlExist"`
	Status      string `json:"status"`
	Version     string `json:"version"`
	ConfigPath  string `json:"configPath"`
	Init        bool   `json:"init"`
}

type HostToolConfig struct {
	Content string `json:"content"`
}

type SupervisorProcessConfig struct {
	Name        string          `json:"name"`
	Numprocs    string          `json:"numprocs"`
	Dir         string          `json:"dir"`
	User        string          `json:"user"`
	AutoRestart string          `json:"autoRestart"`
	AutoStart   string          `json:"autoStart"`
	Status      []ProcessStatus `json:"status"`
	Command     string          `json:"command"`
	Environment string          `json:"environment"`
}

type ProcessStatus struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	PID    string `json:"pid"`
	Uptime string `json:"uptime"`
	Msg    string `json:"msg"`
}
