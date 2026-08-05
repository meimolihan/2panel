package handler

import (
	"net/http"

	"github.com/2panel-dev/2panel/internal/dto"
	"github.com/2panel-dev/2panel/internal/service"
)

var authService service.AuthService

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
	SuccessWithData(w, info)
}

func (a *AuthApi) Logout(w http.ResponseWriter, r *http.Request) {
	authService.Logout(tokenFromRequest(r))
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

func tokenFromRequest(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if len(h) > 7 && h[:7] == "Bearer " {
		return h[7:]
	}
	return h
}
