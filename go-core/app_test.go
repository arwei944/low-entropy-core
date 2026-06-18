package core

import (
	"testing"
	"time"
)

func TestApp_NewDefaultConfig(t *testing.T) {
	cfg := DefaultAppConfig()
	cfg.StorageDir = t.TempDir()
	cfg.HTTPAddr = ":0"
	cfg.SchedulerEnabled = true

	app, err := NewApp(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if app.EventStore == nil {
		t.Error("expected EventStore")
	}
	if app.Observation == nil {
		t.Error("expected Observation")
	}
	if app.Guardian == nil {
		t.Error("expected Guardian")
	}
	if app.HTTP == nil {
		t.Error("expected HTTP")
	}
	if app.Scheduler == nil {
		t.Error("expected Scheduler")
	}
	if app.Storage == nil {
		t.Error("expected Storage")
	}
}

func TestApp_StartStop(t *testing.T) {
	cfg := DefaultAppConfig()
	cfg.StorageDir = t.TempDir()
	cfg.HTTPAddr = ":0"
	cfg.GuardianEnabled = false
	cfg.SchedulerEnabled = false

	app, err := NewApp(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := app.Start(); err != nil {
		t.Fatal(err)
	}
	time.Sleep(50 * time.Millisecond)
	app.HTTP.Close()
}

func TestApp_SchedulerDisabled(t *testing.T) {
	cfg := DefaultAppConfig()
	cfg.StorageDir = t.TempDir()
	cfg.HTTPAddr = ":0"
	cfg.SchedulerEnabled = false

	app, _ := NewApp(cfg)
	if app.Scheduler != nil {
		t.Error("expected nil Scheduler when disabled")
	}
}

func TestApp_GuardianDisabled(t *testing.T) {
	cfg := DefaultAppConfig()
	cfg.StorageDir = t.TempDir()
	cfg.HTTPAddr = ":0"
	cfg.GuardianEnabled = false

	app, _ := NewApp(cfg)
	if app.Guardian != nil {
		t.Error("expected nil Guardian when disabled")
	}
}

func TestApp_NoStorage(t *testing.T) {
	cfg := DefaultAppConfig()
	cfg.StorageDir = ""
	cfg.HTTPAddr = ":0"

	app, err := NewApp(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if app.Storage != nil {
		t.Error("expected nil Storage when no dir")
	}
	if app.EventStore != nil {
		t.Error("expected nil EventStore when no storage")
	}
}