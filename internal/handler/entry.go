package handler

type ApiGroup struct {
	CronjobApi    CronjobApi
	AuthApi       AuthApi
	ScriptApi     ScriptApi
	GroupApi      GroupApi
	UpgradeApi    UpgradeApi
	FileWatchApi  FileWatchApi
	AccountApi    AccountApi
}

var BaseApi = ApiGroup{
	CronjobApi:   CronjobApi{},
	AuthApi:      AuthApi{},
	ScriptApi:    ScriptApi{},
	GroupApi:     GroupApi{},
	UpgradeApi:   UpgradeApi{},
	FileWatchApi: FileWatchApi{},
	AccountApi:   AccountApi{},
}
