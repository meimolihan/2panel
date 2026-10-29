package notify

import (
	"net/url"
	"strings"
	"testing"
)

func TestBuildURLEncodesChinese(t *testing.T) {
	cfg := Config{
		Server:       "https://api.day.app",
		Key:          "device-key",
		SoundSuccess: "bell",
	}
	got, err := buildURL(cfg, LevelSuccess, "任务成功", "备份完成 & 正常", "https://example.com/icon.png", "https://panel.example.com", "2panel", false)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got, "任务成功") || strings.Contains(got, "备份完成") {
		t.Fatalf("chinese content was not url-encoded: %s", got)
	}
	u, err := url.Parse(got)
	if err != nil {
		t.Fatal(err)
	}
	if u.Path != "/device-key/任务成功/备份完成 & 正常" {
		t.Fatalf("unexpected decoded path: %q", u.Path)
	}
	if u.Query().Get("sound") != "bell" {
		t.Fatalf("unexpected sound: %q", u.Query().Get("sound"))
	}
	if u.Query().Get("group") != "2panel" {
		t.Fatalf("unexpected group: %q", u.Query().Get("group"))
	}
}

func TestBuildURLMute(t *testing.T) {
	cfg := Config{Server: "https://api.day.app", Key: "k", SoundError: "alarm"}
	got, err := buildURL(cfg, LevelError, "t", "c", "", "", "", true)
	if err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(got)
	if err != nil {
		t.Fatal(err)
	}
	if u.Query().Get("sound") != "none" {
		t.Fatalf("mute should force sound=none, got %q", u.Query().Get("sound"))
	}
	if u.Query().Get("level") != "passive" {
		t.Fatalf("mute should force level=passive, got %q", u.Query().Get("level"))
	}
}

func TestBuildURLSelfHosted(t *testing.T) {
	cfg := Config{Server: "https://bark.example.com/", Key: "k", SoundInfo: "none"}
	got, err := buildURL(cfg, LevelInfo, "标题", "内容", "", "", "", false)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(got, "https://bark.example.com/k/") {
		t.Fatalf("unexpected self-hosted url: %s", got)
	}
}
