package server

import (
	"embed"
	"io/fs"
	"net/http"

	"github.com/2panel-dev/2panel/internal/handler"
)

//go:embed all:web
var webFS embed.FS

func New(debug bool) http.Handler {
	mux := http.NewServeMux()

	cronjobs := "/api/cronjobs"
	cronApi := handler.BaseApi.CronjobApi
	authApi := handler.BaseApi.AuthApi
	scriptApi := handler.BaseApi.ScriptApi
	groupApi := handler.BaseApi.GroupApi
	{
		mux.HandleFunc("POST /api/auth/status", authApi.Status)
		mux.HandleFunc("POST /api/auth/login", authApi.Login)
		mux.HandleFunc("POST /api/auth/logout", authApi.Logout)
		mux.HandleFunc("POST /api/auth/change-password", authApi.ChangePassword)
		mux.HandleFunc("POST /api/auth/me", handler.AuthMiddleware(authApi.Me))
		mux.HandleFunc("POST /api/auth/backup", handler.AuthMiddleware(authApi.Backup))
		mux.HandleFunc("POST /api/auth/restore", handler.AuthMiddleware(authApi.Restore))

		mux.HandleFunc("POST "+cronjobs, handler.AuthMiddleware(cronApi.Create))
		mux.HandleFunc("POST "+cronjobs+"/search", handler.AuthMiddleware(cronApi.Search))
		mux.HandleFunc("POST "+cronjobs+"/load/info", handler.AuthMiddleware(cronApi.LoadInfo))
		mux.HandleFunc("POST "+cronjobs+"/update", handler.AuthMiddleware(cronApi.Update))
		mux.HandleFunc("POST "+cronjobs+"/del", handler.AuthMiddleware(cronApi.Delete))
		mux.HandleFunc("POST "+cronjobs+"/status", handler.AuthMiddleware(cronApi.UpdateStatus))
		mux.HandleFunc("POST "+cronjobs+"/handle", handler.AuthMiddleware(cronApi.HandleOnce))
		mux.HandleFunc("POST "+cronjobs+"/stop", handler.AuthMiddleware(cronApi.Stop))
		mux.HandleFunc("POST "+cronjobs+"/next", handler.AuthMiddleware(cronApi.Next))
		mux.HandleFunc("POST "+cronjobs+"/search/records", handler.AuthMiddleware(cronApi.SearchRecords))
		mux.HandleFunc("POST "+cronjobs+"/records/log", handler.AuthMiddleware(cronApi.RecordLog))
		mux.HandleFunc("POST "+cronjobs+"/records/log/tail", handler.AuthMiddleware(cronApi.RecordLogTail))
		mux.HandleFunc("POST "+cronjobs+"/export", handler.AuthMiddleware(cronApi.Export))
		mux.HandleFunc("POST "+cronjobs+"/import", handler.AuthMiddleware(cronApi.Import))
		mux.HandleFunc("POST "+cronjobs+"/script/options", handler.AuthMiddleware(cronApi.ScriptOptions))

		mux.HandleFunc("POST /api/scripts/search", handler.AuthMiddleware(scriptApi.Search))
		mux.HandleFunc("POST /api/scripts/load/info", handler.AuthMiddleware(scriptApi.LoadInfo))
		mux.HandleFunc("POST /api/scripts/create", handler.AuthMiddleware(scriptApi.Create))
		mux.HandleFunc("POST /api/scripts/update", handler.AuthMiddleware(scriptApi.Update))
		mux.HandleFunc("POST /api/scripts/del", handler.AuthMiddleware(scriptApi.Delete))
		mux.HandleFunc("POST /api/scripts/run", handler.AuthMiddleware(scriptApi.Run))
		mux.HandleFunc("POST /api/scripts/run/stop", handler.AuthMiddleware(scriptApi.StopRun))
		mux.HandleFunc("POST /api/scripts/run/records", handler.AuthMiddleware(scriptApi.SearchRunRecords))
		mux.HandleFunc("POST /api/scripts/run/log", handler.AuthMiddleware(scriptApi.RunLog))

		mux.HandleFunc("POST /api/groups/search", handler.AuthMiddleware(groupApi.Search))
		mux.HandleFunc("POST /api/groups/create", handler.AuthMiddleware(groupApi.Create))
		mux.HandleFunc("POST /api/groups/update", handler.AuthMiddleware(groupApi.Update))
		mux.HandleFunc("POST /api/groups/del", handler.AuthMiddleware(groupApi.Delete))
	}

	web, err := fs.Sub(webFS, "web")
	if err != nil {
		panic(err)
	}
	mux.Handle("/", http.FileServerFS(web))
	return mux
}
