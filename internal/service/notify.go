package service

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/2panel-dev/2panel/internal/dto"
	"github.com/2panel-dev/2panel/internal/notify"
)

// SettingBarkConfig 是 Bark 配置在 settings 表中的键。
const SettingBarkConfig = "bark_config"

// NotifyService 负责 Bark 通知配置的读写与测试推送。
type NotifyService struct{}

// InitNotify 在服务启动（以及备份还原）后，把数据库中的 Bark 配置加载到
// notify 包。若数据库中没有配置（或读取失败），则回退到环境变量，
// 同时清除可能残留的旧内存配置，保证还原后运行时状态与数据库一致。
func InitNotify() {
	cfg, ok, err := loadStoredBarkConfig()
	if err != nil || !ok {
		notify.ResetConfig()
		return
	}
	notify.SetConfig(cfg)
}

// LoadConfig 返回当前生效的 Bark 配置（数据库优先，环境变量兜底）。
func (u *NotifyService) LoadConfig() dto.BarkConfig {
	if cfg, ok, err := loadStoredBarkConfig(); err == nil && ok {
		return toBarkDTO(cfg)
	}
	return toBarkDTO(notify.CurrentConfig())
}

// SaveConfig 校验并持久化 Bark 配置，保存后立即生效。
func (u *NotifyService) SaveConfig(req dto.BarkConfig) error {
	req.Key = strings.TrimSpace(req.Key)
	req.Server = strings.TrimRight(strings.TrimSpace(req.Server), "/")
	if req.Enable && req.Key == "" {
		return fmt.Errorf("启用 Bark 推送时必须填写设备 Key")
	}
	if req.Timeout <= 0 {
		req.Timeout = 5
	}
	if req.Timeout > 60 {
		return fmt.Errorf("超时时间不能超过 60 秒")
	}
	if req.Server == "" {
		req.Server = "https://api.day.app"
	}
	if req.SoundSuccess == "" {
		req.SoundSuccess = "bell"
	}
	if req.SoundError == "" {
		req.SoundError = "alarm"
	}
	if req.SoundInfo == "" {
		req.SoundInfo = "none"
	}

	data, err := json.Marshal(req)
	if err != nil {
		return err
	}
	if err := settingRepo.Set(SettingBarkConfig, string(data)); err != nil {
		return err
	}
	notify.SetConfig(toNotifyConfig(req))
	return nil
}

// Test 使用当前配置发送一条测试推送，便于在界面中确认配置是否正确。
func (u *NotifyService) Test() error {
	if !notify.Enabled() {
		return fmt.Errorf("请先启用 Bark 推送并保存配置")
	}
	return notify.Notify(
		notify.LevelInfo,
		"2Panel 测试通知",
		fmt.Sprintf("Bark 推送配置正常\n时间：%s", time.Now().Format("2006-01-02 15:04:05")),
		"", "", "", false,
	)
}

func loadStoredBarkConfig() (notify.Config, bool, error) {
	setting, err := settingRepo.Get(SettingBarkConfig)
	if err != nil {
		return notify.Config{}, false, err
	}
	var cfg dto.BarkConfig
	if err := json.Unmarshal([]byte(setting.Value), &cfg); err != nil {
		return notify.Config{}, false, fmt.Errorf("解析 Bark 配置失败: %w", err)
	}
	return toNotifyConfig(cfg), true, nil
}

func toNotifyConfig(cfg dto.BarkConfig) notify.Config {
	return notify.Config{
		Enable:       cfg.Enable,
		Server:       cfg.Server,
		Key:          cfg.Key,
		Icon:         cfg.Icon,
		Group:        cfg.Group,
		SoundSuccess: cfg.SoundSuccess,
		SoundError:   cfg.SoundError,
		SoundInfo:    cfg.SoundInfo,
		Timeout:      time.Duration(cfg.Timeout) * time.Second,
	}
}

func toBarkDTO(cfg notify.Config) dto.BarkConfig {
	timeout := int(cfg.Timeout / time.Second)
	if timeout <= 0 {
		timeout = 5
	}
	return dto.BarkConfig{
		Enable:       cfg.Enable,
		Key:          cfg.Key,
		Server:       cfg.Server,
		Icon:         cfg.Icon,
		Group:        cfg.Group,
		SoundSuccess: cfg.SoundSuccess,
		SoundError:   cfg.SoundError,
		SoundInfo:    cfg.SoundInfo,
		Timeout:      timeout,
	}
}
