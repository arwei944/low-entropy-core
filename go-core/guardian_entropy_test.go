package core

import (
	"context"
	"testing"
	"time"
)

func TestEntropyWatcher_ValidateOK(t *testing.T) {
	watcher := NewEntropyWatcherWithThresholds(0.5, 0.7, 0.9)

	ctx := context.Background()
	snapshot := EntropySnapshot{
		EntropyScore: 0.1,
		Timestamp:    time.Now(),
	}

	alert, err := watcher.Validate(ctx, snapshot)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if alert.Level != EntropyOK {
		t.Errorf("expected OK, got %s", alert.Level)
	}
}

func TestEntropyWatcher_ValidateYellow(t *testing.T) {
	watcher := NewEntropyWatcherWithThresholds(0.5, 0.7, 0.9)

	ctx := context.Background()
	snapshot := EntropySnapshot{
		EntropyScore: 0.6,
		Timestamp:    time.Now(),
	}

	alert, err := watcher.Validate(ctx, snapshot)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if alert.Level != EntropyYellow {
		t.Errorf("expected Yellow, got %s", alert.Level)
	}
}

func TestEntropyWatcher_ValidateOrange(t *testing.T) {
	watcher := NewEntropyWatcherWithThresholds(0.5, 0.7, 0.9)

	ctx := context.Background()
	snapshot := EntropySnapshot{
		EntropyScore: 0.8,
		Timestamp:    time.Now(),
	}

	alert, err := watcher.Validate(ctx, snapshot)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if alert.Level != EntropyOrange {
		t.Errorf("expected Orange, got %s", alert.Level)
	}
}

func TestEntropyWatcher_ValidateRed(t *testing.T) {
	watcher := NewEntropyWatcherWithThresholds(0.5, 0.7, 0.9)

	ctx := context.Background()
	snapshot := EntropySnapshot{
		EntropyScore: 0.95,
		Timestamp:    time.Now(),
	}

	alert, err := watcher.Validate(ctx, snapshot)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if alert.Level != EntropyRed {
		t.Errorf("expected Red, got %s", alert.Level)
	}
}

func TestEntropyWatcher_Acceleration(t *testing.T) {
	watcher := NewEntropyWatcherWithThresholds(0.5, 0.7, 0.9)
	ctx := context.Background()

	// 连续两次高增长
	watcher.Validate(ctx, EntropySnapshot{
		EntropyScore: 0.2,
		Timestamp:    time.Now(),
	})
	time.Sleep(10 * time.Millisecond)
	alert, _ := watcher.Validate(ctx, EntropySnapshot{
		EntropyScore: 0.5,
		Timestamp:    time.Now(),
	})

	if !alert.AccelerationDetected {
		t.Error("expected acceleration detected")
	}
}

func TestEntropyWatcher_DefaultThresholds(t *testing.T) {
	watcher := NewEntropyWatcher()
	ctx := context.Background()

	snapshot := EntropySnapshot{
		EntropyScore: 0.3,
		Timestamp:    time.Now(),
	}
	alert, err := watcher.Validate(ctx, snapshot)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if alert.Level != EntropyOK {
		t.Errorf("expected OK with defaults, got %s", alert.Level)
	}
}

func TestEntropyWatcher_ContextCanceled(t *testing.T) {
	watcher := NewEntropyWatcher()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	snapshot := EntropySnapshot{
		EntropyScore: 0.5,
		Timestamp:    time.Now(),
	}
	_, err := watcher.Validate(ctx, snapshot)
	if err == nil {
		t.Error("expected error for canceled context")
	}
}