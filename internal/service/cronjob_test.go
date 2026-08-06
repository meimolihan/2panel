package service

import (
	"strings"
	"testing"
	"time"

	"github.com/2panel-dev/2panel/internal/dto"
	"github.com/2panel-dev/2panel/internal/model"
)

func TestNextRunTimeSingleEvery(t *testing.T) {
	svc := &CronjobService{}
	now := time.Date(2026, 8, 6, 10, 0, 0, 0, time.Local)

	hours, err := svc.nextRunTimeSingle("@every 1h", now)
	if err != nil {
		t.Fatalf("@every 1h: %v", err)
	}
	if len(hours) != 5 {
		t.Fatalf("expected 5 times, got %d", len(hours))
	}
	if hours[0] != "2026-08-06 11:00:00" {
		t.Errorf("@every 1h first slot = %s", hours[0])
	}
	if hours[4] != "2026-08-06 15:00:00" {
		t.Errorf("@every 1h last slot = %s", hours[4])
	}

	days, err := svc.nextRunTimeSingle("@every 1d", now)
	if err != nil {
		t.Fatalf("@every 1d: %v", err)
	}
	if days[0] != "2026-08-07 10:00:00" {
		t.Errorf("@every 1d first slot = %s", days[0])
	}

	combo, err := svc.nextRunTimeSingle("@every 90m", now)
	if err != nil {
		t.Fatalf("@every 90m: %v", err)
	}
	if combo[0] != "2026-08-06 11:30:00" {
		t.Errorf("@every 90m first slot = %s", combo[0])
	}

	bare, err := svc.nextRunTimeSingle("@every 5", now)
	if err != nil {
		t.Fatalf("@every 5: %v", err)
	}
	if bare[0] != "2026-08-06 10:05:00" {
		t.Errorf("@every 5 (minutes) first slot = %s", bare[0])
	}

	if _, err := svc.nextRunTimeSingle("@every garbage", now); err == nil {
		t.Error("@every garbage should fail")
	}
}

func TestNextRunTimeSingleStandard(t *testing.T) {
	svc := &CronjobService{}
	now := time.Date(2026, 8, 6, 10, 0, 0, 0, time.Local)

	// @daily -> next midnight
	times, err := svc.nextRunTimeSingle("@daily", now)
	if err != nil {
		t.Fatalf("@daily: %v", err)
	}
	if len(times) != 5 {
		t.Fatalf("expected 5 times, got %d", len(times))
	}
	if times[0] != "2026-08-07 00:00:00" {
		t.Errorf("@daily first slot = %s", times[0])
	}

	// standard 5-field cron: "0 9 * * *"
	cron, err := svc.nextRunTimeSingle("0 9 * * *", now)
	if err != nil {
		t.Fatalf("standard cron: %v", err)
	}
	if len(cron) != 5 {
		t.Fatalf("expected 5 times, got %d", len(cron))
	}
	for _, s := range cron {
		if !strings.HasSuffix(s, " 09:00:00") {
			t.Errorf("standard cron slot not at 09:00: %s", s)
		}
	}

	if _, err := svc.nextRunTimeSingle("not-a-cron", now); err == nil {
		t.Error("invalid spec should fail")
	}
}

func TestNextRunTimeDedup(t *testing.T) {
	svc := &CronjobService{}

	times, err := svc.nextRunTime("0 9 * * * && 0 10 * * *")
	if err != nil {
		t.Fatalf("combined spec: %v", err)
	}
	if len(times) != 5 {
		t.Fatalf("expected at most 5 deduped times, got %d: %v", len(times), times)
	}
}

func TestValidateOperate(t *testing.T) {
	svc := &CronjobService{}

	valid := dto.CronjobOperate{Name: "job", Type: model.TypeShell, Spec: "@every 1h", ScriptName: "demo"}
	if err := svc.validateOperate(valid); err != nil {
		t.Errorf("valid shell job rejected: %v", err)
	}

	validCurl := dto.CronjobOperate{Name: "ping", Type: model.TypeCurl, Spec: "0 9 * * *", URL: "https://example.com"}
	if err := svc.validateOperate(validCurl); err != nil {
		t.Errorf("valid curl job rejected: %v", err)
	}

	badName := valid
	badName.Name = "  "
	if err := svc.validateOperate(badName); err == nil {
		t.Error("empty name should fail")
	}

	badType := valid
	badType.Type = "powershell"
	if err := svc.validateOperate(badType); err == nil {
		t.Error("unsupported type should fail")
	}

	badSpec := valid
	badSpec.Spec = "bogus spec"
	if err := svc.validateOperate(badSpec); err == nil {
		t.Error("invalid spec should fail")
	}

	noURL := validCurl
	noURL.URL = ""
	if err := svc.validateOperate(noURL); err == nil {
		t.Error("curl without URL should fail")
	}

	noScript := valid
	noScript.ScriptName = ""
	noScript.Script = ""
	if err := svc.validateOperate(noScript); err == nil {
		t.Error("shell job without script should fail")
	}
}
