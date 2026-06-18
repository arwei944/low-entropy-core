//go:build lecore_tier3 || lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

package core

import (
	"context"
	"testing"
)

func TestPersistentEventBus_Subscribe(t *testing.T) {
	dir := t.TempDir()
	fs, _ := NewFileStorageBackend(dir)
	peb, err := NewPersistentEventBus(fs)
	if err != nil {
		t.Fatal(err)
	}

	handler := func(event EventEnvelope) error { return nil }
	peb.Subscribe("test.event", handler)

	if n := peb.SubscriberCount("test.event"); n != 1 {
		t.Errorf("expected 1 subscriber, got %d", n)
	}
}

func TestPersistentEventBus_Publish(t *testing.T) {
	dir := t.TempDir()
	fs, _ := NewFileStorageBackend(dir)
	peb, _ := NewPersistentEventBus(fs)

	received := false
	handler := func(event EventEnvelope) error {
		received = true
		return nil
	}
	peb.Subscribe("test.event", handler)

	ctx := context.Background()
	result, err := peb.Execute(ctx, EventEnvelope{
		EventID:  "e1",
		EventType: "test.event",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.SubscriberCount != 1 {
		t.Errorf("expected 1 subscriber handled, got %d", result.SubscriberCount)
	}
	if !received {
		t.Error("expected handler to be called")
	}
}

func TestPersistentEventBus_Unsubscribe(t *testing.T) {
	dir := t.TempDir()
	fs, _ := NewFileStorageBackend(dir)
	peb, _ := NewPersistentEventBus(fs)

	handler := func(event EventEnvelope) error { return nil }
	sub := peb.Subscribe("test.event", handler)
	peb.Unsubscribe(sub.ID)

	if n := peb.SubscriberCount("test.event"); n != 0 {
		t.Errorf("expected 0 subscribers after unsubscribe, got %d", n)
	}
}

func TestPersistentEventBus_RestoreMetadata(t *testing.T) {
	dir := t.TempDir()
	fs, _ := NewFileStorageBackend(dir)

	// 第一轮：创建订阅
	peb1, _ := NewPersistentEventBus(fs)
	handler := func(event EventEnvelope) error { return nil }
	peb1.Subscribe("test.event", handler)
	peb1.SubscribeAsync("test.async", handler)

	// 第二轮：重新创建 —— 应恢复元数据
	peb2, err := NewPersistentEventBus(fs)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	subs, err := peb2.RestoredSubscriptions(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(subs) != 2 {
		t.Errorf("expected 2 restored subscriptions, got %d", len(subs))
	}
}

func TestPersistentEventBus_Async(t *testing.T) {
	dir := t.TempDir()
	fs, _ := NewFileStorageBackend(dir)
	peb, _ := NewPersistentEventBus(fs)

	done := make(chan bool, 1)
	handler := func(event EventEnvelope) error {
		done <- true
		return nil
	}
	peb.SubscribeAsync("test.async", handler)

	ctx := context.Background()
	peb.Execute(ctx, EventEnvelope{
		EventID:  "e1",
		EventType: "test.async",
	})

	select {
	case <-done:
		// ok
	default:
		t.Error("expected async handler to be called")
	}
}