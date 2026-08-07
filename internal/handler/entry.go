package handler

type ApiGroup struct {
	CronjobApi CronjobApi
	AuthApi    AuthApi
	ScriptApi  ScriptApi
	GroupApi   GroupApi
}

var BaseApi = ApiGroup{
	CronjobApi: CronjobApi{},
	AuthApi:    AuthApi{},
	ScriptApi:  ScriptApi{},
	GroupApi:   GroupApi{},
}
