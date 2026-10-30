package service

import (
	"path/filepath"
	"testing"

	"github.com/2panel-dev/2panel/internal/database"
	"github.com/2panel-dev/2panel/internal/dto"
	"github.com/2panel-dev/2panel/internal/notify"
)

// TestBarkConfigPersistenceRoundTrip verifies the full persistence chain:
// SaveConfig writes to the settings table, InitNotify reloads it at startup,
// and removing the stored setting makes InitNotify fall back to env instead of
// keeping stale in-memory config.
func TestBarkConfigPersistenceRoundTrip(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "2panel.db")
	if err := database.Init(dbPath); err != nil {
		t.Fatalf("init database: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	svc := &NotifyService{}
	want := dto.BarkConfig{
		Enable:       true,
		Key:          "test-device-key",
		Server:       "https://bark.example.com",
		Icon:         "https://example.com/icon.png",
		Group:        "2panel",
		SoundSuccess: "bell",
		SoundError:   "alarm",
		SoundInfo:    "none",
		Timeout:      8,
	}
	if err := svc.SaveConfig(want); err != nil {
		t.Fatalf("save config: %v", err)
	}

	got := svc.LoadConfig()
	if got != want {
		t.Fatalf("load config mismatch:\n got = %+v\nwant = %+v", got, want)
	}

	// Simulate a process restart: clear memory, then reload from DB.
	notify.ResetConfig()
	InitNotify()
	cfg := notify.CurrentConfig()
	if !cfg.Enable || cfg.Key != want.Key || cfg.Server != want.Server {
		t.Fatalf("InitNotify did not restore persisted config: %+v", cfg)
	}

	// Simulate restoring an old backup without bark_config: stale memory config
	// must be cleared and env fallback used instead.
	t.Setenv("BARK_ENABLE", "false")
	if err := database.DB.Exec("DELETE FROM settings WHERE key = ?", SettingBarkConfig).Error; err != nil {
		t.Fatalf("delete setting: %v", err)
	}
	InitNotify()
	if notify.Enabled() {
		t.Fatal("stale bark config should be cleared when the restored DB has none")
	}
}
