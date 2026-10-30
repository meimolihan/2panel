// Package notify 提供统一的外部通知能力，目前实现 Bark（iOS 推送）。
//
// ============================================================================
// Bark 配置区域（环境变量作为默认值 / 兜底；Web 界面保存后以数据库配置为准）
//
// Web 入口：右上角头像 -> 推送通知，可在界面中配置并发送测试推送。
// 注意：BARK_ENABLE 关闭时，所有 Notify / NotifyAsync 调用会立即返回，
// 不会发送任何请求，也不会影响原有业务逻辑。
//
//	BARK_ENABLE          总开关。true/1/on/yes 启用，其余（含未设置）关闭。
//	                    默认：false
//	BARK_KEY             Bark 设备 Key（必填）。在 Bark App 首页复制。
//	BARK_SERVER          Bark 服务地址。默认：https://api.day.app
//	                     自建服务示例：http://bark.example.com:8080
//	BARK_ICON            默认推送图标 URL（可选，iOS 15+）。
//	BARK_GROUP           默认通知分组（可选，建议不超过 8 个字符）。
//	BARK_SOUND_SUCCESS   成功通知铃声。默认：bell
//	BARK_SOUND_ERROR     失败告警铃声。默认：alarm
//	BARK_SOUND_INFO      普通提示铃声。默认：none
//	BARK_TIMEOUT         单次 HTTP 请求超时（秒）。默认：5
//
// 说明：
//   - 中文标题/内容会自动做 URL 编码，避免乱码。
//   - 仅发起一次请求，失败只记录日志，不重试、不阻塞调用方。
//   - 异步入口 NotifyAsync 在独立 goroutine 中执行，调用方立即返回。
//
// 使用示例：
//
//	notify.Success("计划任务成功", "备份任务已完成，耗时 3.2s")
//	notify.Error("计划任务失败", "备份任务退出码 1")
//	notify.Info("系统提示", "服务将于 10 分钟后重启")
//
//	// 完整参数：级别、标题、内容、图标、跳转URL、分组、是否静音
//	notify.NotifyAsync(notify.LevelError, "磁盘告警", "根分区使用率 95%",
//		"https://example.com/icon.png", "https://panel.example.com",
//		"2panel", false)
//
// ============================================================================
package notify

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// 通知级别。不同级别会使用配置里对应的铃声。
const (
	LevelSuccess = "success"
	LevelError   = "error"
	LevelInfo    = "info"
)

const (
	defaultServer  = "https://api.day.app"
	defaultTimeout = 5 * time.Second
)

// Config 是 Bark 的独立配置。
// 默认从环境变量读取；Web 界面保存后会通过 SetConfig 覆盖为数据库中的配置。
type Config struct {
	Enable       bool
	Server       string
	Key          string
	Icon         string
	Group        string
	SoundSuccess string
	SoundError   string
	SoundInfo    string
	Timeout      time.Duration
}

var (
	// configMu 保护 config，支持运行期通过 Web 界面热更新配置。
	configMu     sync.RWMutex
	config       Config
	configLoaded bool

	// httpClient 复用连接；单次请求超时由 context 控制。
	httpClient = &http.Client{}
)

// loadEnvConfig 读取环境变量并填充默认值。
func loadEnvConfig() Config {
	timeout := defaultTimeout
	if v := strings.TrimSpace(os.Getenv("BARK_TIMEOUT")); v != "" {
		if seconds, err := strconv.Atoi(v); err == nil && seconds > 0 {
			timeout = time.Duration(seconds) * time.Second
		}
	}
	cfg := Config{
		Enable:       parseBool(os.Getenv("BARK_ENABLE")),
		Server:       strings.TrimRight(strings.TrimSpace(os.Getenv("BARK_SERVER")), "/"),
		Key:          strings.TrimSpace(os.Getenv("BARK_KEY")),
		Icon:         strings.TrimSpace(os.Getenv("BARK_ICON")),
		Group:        strings.TrimSpace(os.Getenv("BARK_GROUP")),
		SoundSuccess: envOr("BARK_SOUND_SUCCESS", "bell"),
		SoundError:   envOr("BARK_SOUND_ERROR", "alarm"),
		SoundInfo:    envOr("BARK_SOUND_INFO", "none"),
		Timeout:      timeout,
	}
	if cfg.Server == "" {
		cfg.Server = defaultServer
	}
	return cfg
}

func envOr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func parseBool(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "on", "yes", "enabled":
		return true
	default:
		return false
	}
}

func getConfig() Config {
	configMu.RLock()
	if configLoaded {
		cfg := config
		configMu.RUnlock()
		return cfg
	}
	configMu.RUnlock()

	configMu.Lock()
	defer configMu.Unlock()
	if !configLoaded {
		config = normalizeConfig(loadEnvConfig())
		configLoaded = true
	}
	return config
}

func normalizeConfig(cfg Config) Config {
	cfg.Server = strings.TrimRight(strings.TrimSpace(cfg.Server), "/")
	if cfg.Server == "" {
		cfg.Server = defaultServer
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = defaultTimeout
	}
	if cfg.SoundSuccess == "" {
		cfg.SoundSuccess = "bell"
	}
	if cfg.SoundError == "" {
		cfg.SoundError = "alarm"
	}
	if cfg.SoundInfo == "" {
		cfg.SoundInfo = "none"
	}
	return cfg
}

// SetConfig 在运行期覆盖配置（Web 界面保存后调用），立即生效，无需重启。
func SetConfig(cfg Config) {
	cfg = normalizeConfig(cfg)
	configMu.Lock()
	config = cfg
	configLoaded = true
	configMu.Unlock()
}

// CurrentConfig 返回当前生效的配置（只读副本）。
func CurrentConfig() Config {
	return getConfig()
}

// ResetConfig 丢弃当前内存配置并重新从环境变量加载。
// 用于备份还原后，若新数据库中没有 Bark 配置，避免旧配置残留。
func ResetConfig() {
	cfg := normalizeConfig(loadEnvConfig())
	configMu.Lock()
	config = cfg
	configLoaded = true
	configMu.Unlock()
}

// Enabled 报告 Bark 推送是否已启用（BARK_ENABLE 或 Web 界面开关）。
func Enabled() bool {
	return getConfig().Enable
}

// Notify 是统一通知函数。
//
// 参数：
//   - level:    通知级别，LevelSuccess / LevelError / LevelInfo，决定使用哪个铃声
//   - title:    推送标题
//   - content:  推送内容（中文自动 URL 编码）
//   - icon:     推送图标 URL，为空时使用 BARK_ICON
//   - jumpURL:  点击通知跳转的 URL
//   - group:    通知分组，为空时使用 BARK_GROUP
//   - mute:     是否静音；为 true 时强制 sound=none 且 level=passive
//
// 该函数同步执行，只请求一次，失败返回 error，不做重试。
// 业务代码通常应使用 NotifyAsync，避免网络问题阻塞主流程。
func Notify(level, title, content, icon, jumpURL, group string, mute bool) (err error) {
	// 统一在这里记录失败日志，调用方无需重复处理；只记录一次，不重试。
	defer func() {
		if err != nil {
			log.Printf("[bark] 推送失败 level=%s title=%q: %v", level, title, err)
		}
	}()

	cfg := getConfig()
	if !cfg.Enable {
		return nil
	}
	if cfg.Key == "" {
		return errors.New("bark key is not configured (BARK_KEY)")
	}
	if icon == "" {
		icon = cfg.Icon
	}
	if group == "" {
		group = cfg.Group
	}

	target, err := buildURL(cfg, level, title, content, icon, jumpURL, group, mute)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return fmt.Errorf("build bark request: %w", err)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request bark server: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("bark server returned http %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	// Bark 正常时 HTTP 200 + {"code":200,...}；逻辑失败时可能返回非 200 的 code。
	var result struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &result); err == nil && result.Code != 0 && result.Code != http.StatusOK {
		return fmt.Errorf("bark error code %d: %s", result.Code, result.Message)
	}
	return nil
}

// NotifyAsync 异步发送通知，失败只写日志，绝不会阻塞或影响主流程。
func NotifyAsync(level, title, content, icon, jumpURL, group string, mute bool) {
	if !getConfig().Enable {
		return
	}
	go func() {
		_ = Notify(level, title, content, icon, jumpURL, group, mute)
	}()
}

// Success 发送成功通知（使用 BARK_SOUND_SUCCESS 铃声）。
func Success(title, content string) {
	NotifyAsync(LevelSuccess, title, content, "", "", "", false)
}

// Error 发送失败告警通知（使用 BARK_SOUND_ERROR 铃声）。
func Error(title, content string) {
	NotifyAsync(LevelError, title, content, "", "", "", false)
}

// Info 发送普通提示通知（使用 BARK_SOUND_INFO 铃声）。
func Info(title, content string) {
	NotifyAsync(LevelInfo, title, content, "", "", "", false)
}

func soundForLevel(cfg Config, level string) string {
	switch level {
	case LevelSuccess:
		return cfg.SoundSuccess
	case LevelError:
		return cfg.SoundError
	default:
		return cfg.SoundInfo
	}
}

// buildURL 按 Bark 的 /:key/:title/:body 路径格式拼接请求地址。
// 路径段使用 url.PathEscape，查询参数使用 url.Values 编码，
// 因此中文与特殊字符都会被正确转义。
func buildURL(cfg Config, level, title, content, icon, jumpURL, group string, mute bool) (string, error) {
	if cfg.Key == "" {
		return "", errors.New("bark key is empty")
	}
	server := strings.TrimRight(cfg.Server, "/")
	if server == "" {
		server = defaultServer
	}

	var b strings.Builder
	b.WriteString(server)
	b.WriteString("/")
	b.WriteString(url.PathEscape(cfg.Key))
	b.WriteString("/")
	b.WriteString(url.PathEscape(title))
	b.WriteString("/")
	b.WriteString(url.PathEscape(content))

	q := url.Values{}
	sound := soundForLevel(cfg, level)
	if mute {
		// sound=none 静音；passive 仅进通知列表，不亮屏打扰。
		sound = "none"
		q.Set("level", "passive")
	}
	if sound != "" {
		q.Set("sound", sound)
	}
	if icon != "" {
		q.Set("icon", icon)
	}
	if jumpURL != "" {
		q.Set("url", jumpURL)
	}
	if group != "" {
		q.Set("group", group)
	}
	if len(q) > 0 {
		b.WriteString("?")
		b.WriteString(q.Encode())
	}
	return b.String(), nil
}
