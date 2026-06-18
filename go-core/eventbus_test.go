//go:build lecore_tier2 || lecore_tier3 || lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

package core

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestEventBus_PublishToSubscriber(t *testing.T) {
	bus := NewEventBus()
	ctx := context.Background()

	var received EventEnvelope
	var mu sync.Mutex

	bus.Subscribe("OrderCreated", func(env EventEnvelope) error {
		mu.Lock()
		received = env
		mu.Unlock()
		return nil
	})

	env := EventEnvelope{
		EventID:   "evt-1",
		EventType: "OrderCreated",
		EventData: []byte("test"),
	}

	result, err := bus.Execute(ctx, env)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.SubscriberCount != 1 {
		t.Errorf("expected 1 subscriber, got %d", result.SubscriberCount)
	}

	mu.Lock()
	if received.EventID != "evt-1" {
		t.Errorf("expected evt-1, got %s", received.EventID)
	}
	mu.Unlock()
}

func TestEventBus_NoSubscribers(t *testing.T) {
	bus := NewEventBus()
	ctx := context.Background()

	env := EventEnvelope{
		EventID:   "evt-2",
		EventType: "NoSubscribers",
	}

	result, err := bus.Execute(ctx, env)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.SubscriberCount != 0 {
		t.Errorf("expected 0 subscribers, got %d", result.SubscriberCount)
	}
}

func TestEventBus_MultipleSubscribers(t *testing.T) {
	bus := NewEventBus()
	ctx := context.Background()

	var count int
	var mu sync.Mutex

	for i := 0; i < 3; i++ {
		bus.Subscribe("MultiEvent", func(env EventEnvelope) error {
			mu.Lock()
			count++
			mu.Unlock()
			return nil
		})
	}

	env := EventEnvelope{
		EventID:   "evt-3",
		EventType: "MultiEvent",
	}

	result, err := bus.Execute(ctx, env)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.SubscriberCount != 3 {
		t.Errorf("expected 3 subscribers, got %d", result.SubscriberCount)
	}

	mu.Lock()
	if count != 3 {
		t.Errorf("expected 3 calls, got %d", count)
	}
	mu.Unlock()
}

func TestEventBus_AsyncSubscriber(t *testing.T) {
	bus := NewEventBus()
	ctx := context.Background()

	var received bool
	done := make(chan struct{})

	bus.SubscribeAsync("AsyncEvent", func(env EventEnvelope) error {
		received = true
		close(done)
		return nil
	})

	env := EventEnvelope{
		EventID:   "evt-4",
		EventType: "AsyncEvent",
	}

	result, err := bus.Execute(ctx, env)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.SubscriberCount != 1 {
		t.Errorf("expected 1 subscriber, got %d", result.SubscriberCount)
	}

	select {
	case <-done:
		if !received {
			t.Error("expected async handler to be called")
		}
	case <-time.After(1 * time.Second):
		t.Error("async handler timed out")
	}
}

func TestEventBus_Unsubscribe(t *testing.T) {
	bus := NewEventBus()
	ctx := context.Background()

	sub := bus.Subscribe("UnsubEvent", func(env EventEnvelope) error {
		return nil
	})

	bus.Unsubscribe(sub.ID)

	env := EventEnvelope{
		EventID:   "evt-5",
		EventType: "UnsubEvent",
	}

	result, err := bus.Execute(ctx, env)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.SubscriberCount != 0 {
		t.Errorf("expected 0 subscribers after unsubscribe, got %d", result.SubscriberCount)
	}
}

func TestEventBus_SubscriberCount(t *testing.T) {
	bus := NewEventBus()

	bus.Subscribe("CountEvent", func(env EventEnvelope) error { return nil })
	bus.Subscribe("CountEvent", func(env EventEnvelope) error { return nil })

	if bus.SubscriberCount("CountEvent") != 2 {
		t.Errorf("expected 2 subscribers, got %d", bus.SubscriberCount("CountEvent"))
	}
	if bus.SubscriberCount("Nonexistent") != 0 {
		t.Errorf("expected 0, got %d", bus.SubscriberCount("Nonexistent"))
	}
}