package service

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"math/big"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/2panel-dev/2panel/internal/console"
	"github.com/2panel-dev/2panel/internal/dto"
	"github.com/2panel-dev/2panel/internal/repo"
	"golang.org/x/crypto/bcrypt"
)

var (
	settingRepo repo.SettingRepo

	tokenMutex sync.RWMutex
	tokens     = make(map[string]time.Time)
)

const (
	SettingInitialized     = "auth_initialized"
	SettingUserName        = "UserName"
	SettingPassword        = "Password"
	SettingDefaultPassword = "DefaultPassword"

	defaultUserName = "admin"
	tokenTTL        = 24 * time.Hour
)

// TokenTTLSeconds reports the lifetime of issued tokens, used by the HTTP
// layer to size the auth cookie.
func TokenTTLSeconds() int {
	return int(tokenTTL.Seconds())
}

type AuthService struct{}

var authService AuthService

// InitAuth initializes the admin account with a random default password on
// first launch, mirroring the behavior of 1Panel's install-time password.
func InitAuth() {
	initialized, err := authService.Initialized()
	if err != nil || initialized {
		return
	}
	pwd := generatePassword()
	pwdHash, err := hashPassword(pwd)
	if err != nil {
		log.Printf("init auth password hash failed: %v", err)
		return
	}
	if err := settingRepo.Set(SettingUserName, defaultUserName); err != nil {
		log.Printf("init auth username failed: %v", err)
		return
	}
	if err := settingRepo.Set(SettingPassword, pwdHash); err != nil {
		log.Printf("init auth password failed: %v", err)
		return
	}
	if err := settingRepo.Set(SettingDefaultPassword, pwd); err != nil {
		log.Printf("init auth default password failed: %v", err)
		return
	}
	if err := settingRepo.Set(SettingInitialized, "1"); err != nil {
		log.Printf("init auth flag failed: %v", err)
		return
	}
	log.Printf("admin account initialized, default password: %s (please change it after first login)", pwd)
	fmt.Printf("  %s\n", console.Paint("⚠ 默认密码 "+pwd+"，首次登录后请立即修改。", console.StyleYellow))
	startTokenSweeper()
}

func (u *AuthService) Initialized() (bool, error) {
	if _, err := settingRepo.Get(SettingInitialized); err != nil {
		return false, nil
	}
	return true, nil
}

func (u *AuthService) Status() dto.AuthStatus {
	initialized, _ := u.Initialized()
	status := dto.AuthStatus{Initialized: initialized}
	if !initialized {
		return status
	}
	if setting, err := settingRepo.Get(SettingUserName); err == nil {
		status.UserName = setting.Value
	}
	if setting, err := settingRepo.Get(SettingDefaultPassword); err == nil && len(setting.Value) != 0 {
		status.HasDefaultPasswd = true
		status.DefaultPassword = setting.Value
	}
	return status
}

func (u *AuthService) Login(req dto.Login, remoteAddr string) (dto.LoginInfo, error) {
	ip := clientIP(remoteAddr)
	name := strings.TrimSpace(req.Name)
	if len(name) == 0 {
		return dto.LoginInfo{}, errors.New("用户名不能为空")
	}
	if len(req.Password) == 0 {
		return dto.LoginInfo{}, errors.New("密码不能为空")
	}
	if loginLocked(ip) {
		return dto.LoginInfo{}, errors.New("尝试次数过多，请 5 分钟后再试")
	}
	userName, err := settingRepo.Get(SettingUserName)
	if err != nil || userName.Value != name {
		registerLoginFailure(ip)
		return dto.LoginInfo{}, errors.New("用户名或密码错误")
	}
	pwdSetting, err := settingRepo.Get(SettingPassword)
	if err != nil || !passwordMatches(pwdSetting.Value, req.Password) {
		registerLoginFailure(ip)
		return dto.LoginInfo{}, errors.New("用户名或密码错误")
	}
	clearLoginFailures(ip)
	token, err := generateToken()
	if err != nil {
		return dto.LoginInfo{}, err
	}
	tokenMutex.Lock()
	tokens[token] = time.Now().Add(tokenTTL)
	tokenMutex.Unlock()
	return dto.LoginInfo{Token: token, Name: name}, nil
}

func (u *AuthService) VerifyToken(token string) bool {
	if len(token) == 0 {
		return false
	}
	tokenMutex.RLock()
	expire, ok := tokens[token]
	tokenMutex.RUnlock()
	if !ok {
		return false
	}
	if time.Now().After(expire) {
		tokenMutex.Lock()
		delete(tokens, token)
		tokenMutex.Unlock()
		return false
	}
	return true
}

func (u *AuthService) Logout(token string) {
	tokenMutex.Lock()
	delete(tokens, token)
	tokenMutex.Unlock()
}

func (u *AuthService) ChangePassword(req dto.ChangePassword) error {
	pwdSetting, err := settingRepo.Get(SettingPassword)
	if err != nil || !passwordMatches(pwdSetting.Value, req.OldPassword) {
		return errors.New("旧密码错误")
	}
	if len(req.NewPassword) < 8 {
		return errors.New("新密码长度至少 8 位")
	}
	if len(req.NewPassword) > 64 {
		return errors.New("新密码长度不能超过 64 位")
	}
	newHash, err := hashPassword(req.NewPassword)
	if err != nil {
		return err
	}
	if err := settingRepo.Set(SettingPassword, newHash); err != nil {
		return err
	}
	if err := settingRepo.Set(SettingDefaultPassword, ""); err != nil {
		return err
	}
	return nil
}

// hashPassword produces a bcrypt hash for storage. bcrypt embeds its own salt
// and is intentionally slow against offline brute force.
func hashPassword(pwd string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(pwd), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// passwordMatches reports whether the candidate password matches the stored
// value. Both the current bcrypt format and the legacy unsalted SHA-256 hex
// format (written by older releases) are accepted, so existing accounts keep
// working without a forced password reset.
func passwordMatches(stored, candidate string) bool {
	if stored == "" {
		return false
	}
	if strings.HasPrefix(stored, "$2") {
		return bcrypt.CompareHashAndPassword([]byte(stored), []byte(candidate)) == nil
	}
	return stored == legacyHashPassword(candidate)
}

func legacyHashPassword(pwd string) string {
	sum := sha256.Sum256([]byte(pwd))
	return hex.EncodeToString(sum[:])
}

// ---------------------------------------------------------------------------
// 登录防爆破：按客户端 IP 计数连续失败，超过阈值后锁定一段时间。
// ---------------------------------------------------------------------------

const (
	maxLoginAttempts = 5
	loginLockWindow  = 5 * time.Minute
)

var loginAttempts = struct {
	mu       sync.Mutex
	failures map[string]int
	lockedAt map[string]time.Time
}{failures: make(map[string]int), lockedAt: make(map[string]time.Time)}

func clientIP(remoteAddr string) string {
	if host, _, err := net.SplitHostPort(remoteAddr); err == nil {
		return host
	}
	return remoteAddr
}

func loginLocked(ip string) bool {
	loginAttempts.mu.Lock()
	defer loginAttempts.mu.Unlock()
	if lockedAt, ok := loginAttempts.lockedAt[ip]; ok {
		if time.Since(lockedAt) < loginLockWindow {
			return true
		}
		delete(loginAttempts.lockedAt, ip)
		delete(loginAttempts.failures, ip)
	}
	return false
}

func registerLoginFailure(ip string) {
	loginAttempts.mu.Lock()
	defer loginAttempts.mu.Unlock()
	loginAttempts.failures[ip]++
	if loginAttempts.failures[ip] >= maxLoginAttempts {
		loginAttempts.lockedAt[ip] = time.Now()
	}
}

func clearLoginFailures(ip string) {
	loginAttempts.mu.Lock()
	defer loginAttempts.mu.Unlock()
	delete(loginAttempts.failures, ip)
	delete(loginAttempts.lockedAt, ip)
}

// ---------------------------------------------------------------------------
// token 定期清理：过期 token 只会在验签时被懒删除，这里兜底防止 map 无限增长。
// ---------------------------------------------------------------------------

var sweepOnce sync.Once

func startTokenSweeper() {
	sweepOnce.Do(func() {
		go func() {
			ticker := time.NewTicker(10 * time.Minute)
			defer ticker.Stop()
			for range ticker.C {
				sweepExpiredTokens()
			}
		}()
	})
}

func sweepExpiredTokens() {
	now := time.Now()
	tokenMutex.Lock()
	defer tokenMutex.Unlock()
	for token, expire := range tokens {
		if now.After(expire) {
			delete(tokens, token)
		}
	}
}

func generateToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate token failed: %v", err)
	}
	return hex.EncodeToString(buf), nil
}

var pwdAlphabet = []byte("abcdefghijkmnpqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789")

func generatePassword() string {
	buf := make([]byte, 12)
	for i := range buf {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(pwdAlphabet))))
		if err != nil {
			return "12345678"
		}
		buf[i] = pwdAlphabet[n.Int64()]
	}
	return string(buf)
}
