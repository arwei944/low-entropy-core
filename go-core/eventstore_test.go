package core

import (
	"context"
	"testing"
)

func TestEventStore_Append(t *testing.T) {
	store := NewEventStore()
	ctx := context.Background()

	env := EventEnvelope{
		AggregateID:   "agg-1",
		AggregateType: "Order",
		EventType:     "OrderCreated",
		EventData:     []byte(`{"amount":100}`),
	}

	result, err := store.Execute(ctx, env)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Error("expected success")
	}
	if result.Version != 1 {
		t.Errorf("expected version 1, got %d", result.Version)
	}
	if result.EventID == "" {
		t.Error("expected non-empty EventID")
	}
}

func TestEventStore_AppendMultiple(t *testing.T) {
	store := NewEventStore()
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		env := EventEnvelope{
			AggregateID:   "agg-2",
			AggregateType: "Order",
			EventType:     "OrderUpdated",
			EventData:     []byte("{}"),
		}
		result, err := store.Execute(ctx, env)
		if err != nil {
			t.Fatalf("unexpected error at %d: %v", i, err)
		}
		if result.Version != int64(i+1) {
			t.Errorf("expected version %d, got %d", i+1, result.Version)
		}
	}
}

func TestEventStore_Stream(t *testing.T) {
	store := NewEventStore()
	ctx := context.Background()

	store.Execute(ctx, EventEnvelope{
		AggregateID: "agg-3", AggregateType: "Order", EventType: "Created", EventData: []byte("1"),
	})
	store.Execute(ctx, EventEnvelope{
		AggregateID: "agg-3", AggregateType: "Order", EventType: "Updated", EventData: []byte("2"),
	})

	events := store.Stream("agg-3", 0)
	if len(events) != 2 {
		t.Errorf("expected 2 events, got %d", len(events))
	}
}

func TestEventStore_StreamEmpty(t *testing.T) {
	store := NewEventStore()
	events := store.Stream("nonexistent", 0)
	if len(events) != 0 {
		t.Errorf("expected 0 events, got %d", len(events))
	}
}

func TestEventStore_Snapshot(t *testing.T) {
	store := NewEventStore()
	ctx := context.Background()

	store.Execute(ctx, EventEnvelope{
		AggregateID: "agg-4", AggregateType: "Order", EventType: "Created", EventData: []byte("1"),
	})

	store.SaveSnapshot("agg-4", 1, []byte(`{"state":"done"}`))

	snap, ok := store.GetSnapshot("agg-4")
	if !ok {
		t.Fatal("expected snapshot, got nil")
	}
	if snap.Version != 1 {
		t.Errorf("expected version 1, got %d", snap.Version)
	}
	if string(snap.State) != `{"state":"done"}` {
		t.Errorf("unexpected state: %s", snap.State)
	}
}

func TestEventStore_GetLatestVersion(t *testing.T) {
	store := NewEventStore()
	ctx := context.Background()

	store.Execute(ctx, EventEnvelope{
		AggregateID: "agg-5", AggregateType: "Order", EventType: "Created", EventData: []byte("1"),
	})

	v := store.GetLatestVersion("agg-5")
	if v != 1 {
		t.Errorf("expected version 1, got %d", v)
	}

	v = store.GetLatestVersion("nonexistent")
	if v != 0 {
		t.Errorf("expected version 0, got %d", v)
	}
}

func TestEventStore_Count(t *testing.T) {
	store := NewEventStore()
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		store.Execute(ctx, EventEnvelope{
			AggregateID: "agg-6", AggregateType: "Order", EventType: "Updated", EventData: []byte("{}"),
		})
	}

	if store.Count("agg-6") != 3 {
		t.Errorf("expected 3, got %d", store.Count("agg-6"))
	}
	if store.Count("nonexistent") != 0 {
		t.Errorf("expected 0, got %d", store.Count("nonexistent"))
	}
}