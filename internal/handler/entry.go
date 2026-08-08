package handler

type ApiGroup struct {
	CronjobApi CronjobApi
	AuthApi    AuthApi
	ScriptApi  ScriptApi
	GroupApi   GroupApi
	UpgradeApi UpgradeApi
}

var BaseApi = ApiGroup{
	CronjobApi: CronjobApi{},
	AuthApi:    AuthApi{},
	ScriptApi:  ScriptApi{},
	GroupApi:   GroupApi{},
	UpgradeApi: UpgradeApi{},
}
