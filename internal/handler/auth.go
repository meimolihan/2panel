package handler

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/2panel-dev/2panel/internal/dto"
	"github.com/2panel-dev/2panel/internal/service"
)

var authService service.AuthService

const tokenCookie = "2panel_token"

type AuthApi struct{}

func (a *AuthApi) Status(w http.ResponseWriter, r *http.Request) {
	SuccessWithData(w, authService.Status())
}

func (a *AuthApi) Login(w http.ResponseWriter, r *http.Request) {
	var req dto.Login
	if err := decode(&req, w, r); err != nil {
		return
	}
	info, err := authService.Login(req)
	if err != nil {
		Error(w, http.StatusBadRequest, err)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     tokenCookie,
		Value:    info.Token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   service.TokenTTLSeconds(),
	})
	SuccessWithData(w, info)
}

// Me reports the authenticated user; the frontend calls it on boot to decide
// whether to render the app or the login screen without exposing secrets.
func (a *AuthApi) Me(w http.ResponseWriter, r *http.Request) {
	SuccessWithData(w, map[string]string{"name": "admin"})
}

func (a *AuthApi) Logout(w http.ResponseWriter, r *http.Request) {
	authService.Logout(tokenFromRequest(r))
	http.SetCookie(w, &http.Cookie{
		Name:     tokenCookie,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
	Success(w)
}

func (a *AuthApi) ChangePassword(w http.ResponseWriter, r *http.Request) {
	var req dto.ChangePassword
	if err := decode(&req, w, r); err != nil {
		return
	}
	if err := authService.ChangePassword(req); err != nil {
		Error(w, http.StatusBadRequest, err)
		return
	}
	Success(w)
}

// Backup streams a zip of the whole data directory (db + log + task) for
// download. Must be authenticated. The database is snapshotted first so the
// archive is consistent even while cron jobs are writing.
func (a *AuthApi) Backup(w http.ResponseWriter, r *http.Request) {
	snap, err := service.PrepareBackupSnapshot()
	if err != nil {
		InternalServer(w, err)
		return
	}
	defer os.Remove(snap)

	stamp := time.Now().Format("20060102-150405")
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=2panel-backup-%s.zip", stamp))
	if err := service.WriteBackupZip(w, snap); err != nil {
		// headers are already sent; abort the stream
		log.Printf("backup stream failed: %v", err)
		return
	}
}

// Restore accepts an uploaded backup zip (multipart field "file") and swaps
// the current data with the backup content. Must be authenticated.
func (a *AuthApi) Restore(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(128 << 20); err != nil {
		Error(w, http.StatusBadRequest, fmt.Errorf("请选择备份文件（.zip）"))
		return
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		Error(w, http.StatusBadRequest, fmt.Errorf("请选择备份文件（.zip）"))
		return
	}
	defer file.Close()
	zipBytes, err := io.ReadAll(file)
	if err != nil {
		InternalServer(w, err)
		return
	}
	if err := service.RestoreZip(zipBytes); err != nil {
		Error(w, http.StatusBadRequest, err)
		return
	}
	Success(w)
}

// AuthMiddleware protects the cronjob API. Static files and auth endpoints
// stay public so the login page can be served and submitted.
func AuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !authService.VerifyToken(tokenFromRequest(r)) {
			writeJSON(w, http.StatusUnauthorized, map[string]interface{}{"code": 401, "msg": "未登录或登录已过期"})
			return
		}
		next(w, r)
	}
}

// tokenFromRequest resolves the bearer token from the Authorization header or
// the httpOnly session cookie.
func tokenFromRequest(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if len(h) > 7 && h[:7] == "Bearer " {
		return h[7:]
	}
	if c, err := r.Cookie(tokenCookie); err == nil {
		return c.Value
	}
	return h
}
