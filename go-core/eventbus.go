//go:build lecore_tier2 || lecore_tier3 || lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

package core

import (
	"context"
	"crypto/rand"
	"fmt"
	"log"
	"sync"
)

// PublishResult is the result of publishing an event.
type PublishResult struct {
	EventID        string
	SubscriberCount int
	ErrorCount     int
}

// EventHandler is a function that handles an event.
type EventHandler func(event EventEnvelope) error

// Subscription represents a subscription to an event type.
type Subscription struct {
	ID        string
	EventType string
	Handler   EventHandler
	Async     bool
}

// EventBus is an Adapter that dispatches events to registered subscribers.
// Implements Adapter[EventEnvelope, PublishResult].
type EventBus struct {
	mu            sync.RWMutex
	subscriptions map[string][]*Subscription // eventType -> subscriptions
}

// NewEventBus creates a new EventBus.
func NewEventBus() *EventBus {
	return &EventBus{
		subscriptions: make(map[string][]*Subscription),
	}
}

// Execute publishes an event to all matching subscribers.
// Implements Adapter[EventEnvelope, PublishResult].
//
// Rules:
//  1. Find all subscriptions matching input.EventType.
//  2. For each subscription:
//     - If Async: launch goroutine, call handler, log errors but don't block.
//     - If Sync: call handler, collect errors.
//  3. Return PublishResult with SubscriberCount and ErrorCount.
//  4. If no subscribers, return SubscriberCount=0 (not an error).
func (eb *EventBus) Execute(ctx context.Context, input EventEnvelope) (PublishResult, error) {
	eb.mu.RLock()
	subs := eb.getSubscriptionsLocked(input.EventType)
	eb.mu.RUnlock()

	if len(subs) == 0 {
		return PublishResult{
			EventID:        input.EventID,
			SubscriberCount: 0,
			ErrorCount:     0,
		}, nil
	}

	var (
		errorCount int
		mu         sync.Mutex
		wg         sync.WaitGroup
	)

	for _, sub := range subs {
		sub := sub // capture for closure
		if sub.Async {
			wg.Add(1)
			go func() {
				defer wg.Done()
				// 使用 context 时注意：ctx 可能已取消，不阻塞 Execute 的快速返回
				if err := sub.Handler(input); err != nil {
					mu.Lock()
					errorCount++
					mu.Unlock()
					log.Printf("[EventBus] async handler error (sub=%s, event=%s, eventType=%s): %v",
						sub.ID, input.EventID, input.EventType, err)
				}
			}()
		} else {
			if err := sub.Handler(input); err != nil {
				errorCount++
			}
		}
	}

	// 等待异步处理器完成，确保报告准确。不使用 ctx 取消（ctx 可能已取消，
	// 但 Execute 应在 ctx 取消后仍等待异步处理器完成以避免数据竞争）。
	wg.Wait()

	return PublishResult{
		EventID:        input.EventID,
		SubscriberCount: len(subs),
		ErrorCount:     errorCount,
	}, nil
}

// Subscribe registers a synchronous handler for the given event type.
// Returns the created Subscription.
func (eb *EventBus) Subscribe(eventType string, handler EventHandler) *Subscription {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	sub := &Subscription{
		ID:        generateSubscriptionID(),
		EventType: eventType,
		Handler:   handler,
		Async:     false,
	}
	eb.subscriptions[eventType] = append(eb.subscriptions[eventType], sub)
	return sub
}

// SubscribeAsync registers an asynchronous handler for the given event type.
// Returns the created Subscription.
func (eb *EventBus) SubscribeAsync(eventType string, handler EventHandler) *Subscription {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	sub := &Subscription{
		ID:        generateSubscriptionID(),
		EventType: eventType,
		Handler:   handler,
		Async:     true,
	}
	eb.subscriptions[eventType] = append(eb.subscriptions[eventType], sub)
	return sub
}

// Unsubscribe removes the subscription with the given ID.
// After this call, the subscriber no longer receives events.
func (eb *EventBus) Unsubscribe(subscriptionID string) {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	for eventType, subs := range eb.subscriptions {
		filtered := subs[:0]
		for _, sub := range subs {
			if sub.ID != subscriptionID {
				filtered = append(filtered, sub)
			}
		}
		if len(filtered) != len(subs) {
			if len(filtered) == 0 {
				delete(eb.subscriptions, eventType)
			} else {
				eb.subscriptions[eventType] = filtered
			}
			return
		}
	}
}

// SubscriberCount returns the number of subscribers for the given event type.
func (eb *EventBus) SubscriberCount(eventType string) int {
	eb.mu.RLock()
	defer eb.mu.RUnlock()
	return len(eb.subscriptions[eventType])
}

// ──────────────────────────────────────────────
// Internal helpers
// ──────────────────────────────────────────────

// getSubscriptionsLocked returns a copy of the subscriptions for the given event type.
// Caller must hold at least a read lock.
func (eb *EventBus) getSubscriptionsLocked(eventType string) []*Subscription {
	subs := eb.subscriptions[eventType]
	if len(subs) == 0 {
		return nil
	}
	result := make([]*Subscription, len(subs))
	copy(result, subs)
	return result
}

// generateSubscriptionID generates a UUID v4 string for subscription identification.
func generateSubscriptionID() string {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		// Fallback: use a timestamp-based identifier
		return fmt.Sprintf("sub-%d", 0)
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}