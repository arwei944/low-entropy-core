package core

import (
	"context"
	"fmt"
	"testing"
)

func TestPersistentEventStore_Basic(t *testing.T) {
	dir := t.TempDir()
	fs, _ := NewFileStorageBackend(dir)
	pes, err := NewPersistentEventStore(fs)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	evt := EventEnvelope{
		EventID:       "evt-1",
		AggregateID:   "agg-1",
		AggregateType: "Test",
		EventType:     "Created",
		EventData:     []byte("{}"),
		Version:       1,
	}
	result, err := pes.Execute(ctx, evt)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Success {
		t.Error("expected success")
	}
	if v := pes.GetLatestVersion("agg-1"); v != 1 {
		t.Errorf("expected version 1, got %d", v)
	}
	if c := pes.Count("agg-1"); c != 1 {
		t.Errorf("expected count 1, got %d", c)
	}
}

func TestPersistentEventStore_MultipleEvents(t *testing.T) {
	dir := t.TempDir()
	fs, _ := NewFileStorageBackend(dir)
	pes, _ := NewPersistentEventStore(fs)

	ctx := context.Background()
	for i := 1; i <= 5; i++ {
		pes.Execute(ctx, EventEnvelope{
			EventID:       fmt.Sprintf("evt-%d", i),
			AggregateID:   "agg-x",
			AggregateType: "Test",
			EventType:     "Updated",
			Version:       int64(i),
		})
	}
	if v := pes.GetLatestVersion("agg-x"); v != 5 {
		t.Errorf("expected version 5, got %d", v)
	}
	events := pes.StreamAll("agg-x")
	if len(events) != 5 {
		t.Errorf("expected 5 events, got %d", len(events))
	}
}

func TestPersistentEventStore_Restore(t *testing.T) {
	dir := t.TempDir()
	fs, _ := NewFileStorageBackend(dir)

	// 第一轮：写入事件
	pes1, _ := NewPersistentEventStore(fs)
	ctx := context.Background()
	pes1.Execute(ctx, EventEnvelope{
		EventID: "e1", AggregateID: "a", EventType: "X", Version: 1,
	})
	pes1.Execute(ctx, EventEnvelope{
		EventID: "e2", AggregateID: "a", EventType: "Y", Version: 2,
	})

	// 第二轮：重新创建 —— 应自动恢复
	pes2, err := NewPersistentEventStore(fs)
	if err != nil {
		t.Fatal(err)
	}
	if v := pes2.GetLatestVersion("a"); v != 2 {
		t.Errorf("restore failed: expected version 2, got %d", v)
	}
	if c := pes2.Count("a"); c != 2 {
		t.Errorf("restore failed: expected count 2, got %d", c)
	}
}

func TestPersistentEventStore_Snapshot(t *testing.T) {
	dir := t.TempDir()
	fs, _ := NewFileStorageBackend(dir)
	pes, _ := NewPersistentEventStore(fs)

	pes.SaveSnapshot("agg-s", 3, []byte("state"))
	snap, ok := pes.GetSnapshot("agg-s")
	if !ok {
		t.Fatal("expected snapshot to exist")
	}
	if snap.Version != 3 {
		t.Errorf("expected version 3, got %d", snap.Version)
	}
	if string(snap.State) != "state" {
		t.Errorf("expected state 'state', got %q", snap.State)
	}
}

func TestPersistentEventStore_Stream(t *testing.T) {
	dir := t.TempDir()
	fs, _ := NewFileStorageBackend(dir)
	pes, _ := NewPersistentEventStore(fs)

	ctx := context.Background()
	for i := 1; i <= 3; i++ {
		pes.Execute(ctx, EventEnvelope{
			EventID:       fmt.Sprintf("e%d", i),
			AggregateID:   "agg-s",
			AggregateType: "T",
			EventType:     "E",
			Version:       int64(i),
		})
	}
	// 从版本 2 开始读取
	events := pes.Stream("agg-s", 2)
	if len(events) != 2 {
		t.Errorf("expected 2 events from version 2, got %d", len(events))
	}
	if events[0].Version != 2 {
		t.Errorf("expected first event version 2, got %d", events[0].Version)
	}
}