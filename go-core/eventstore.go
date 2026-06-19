//go:build lecore_tier2 || lecore_tier3 || lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

package core

import (
	"context"
	"crypto/rand"
	"fmt"
	"sort"
	"sync"
	"time"
)

// EventEnvelope wraps an event with metadata.
type EventEnvelope struct {
	EventID       string    `json:"event_id"`
	AggregateID   string    `json:"aggregate_id"`
	AggregateType string    `json:"aggregate_type"`
	EventType     string    `json:"event_type"`
	EventData     []byte    `json:"event_data"`
	Version       int64     `json:"version"`
	Timestamp     time.Time `json:"timestamp"`
	TraceID       string    `json:"trace_id,omitempty"`
}

// AppendResult is the result of appending an event.
type AppendResult struct {
	EventID string `json:"event_id"`
	Version int64  `json:"version"`
	Success bool   `json:"success"`
}

// EventStore is an Adapter that stores immutable events.
// Implements Adapter[EventEnvelope, AppendResult].
type EventStore struct {
	mu        sync.RWMutex
	events    map[string][]EventEnvelope // aggregateID -> events
	snapshots map[string]*Snapshot       // aggregateID -> latest snapshot
}

// Snapshot is a point-in-time snapshot of an aggregate's state.
type Snapshot struct {
	AggregateID string
	Version     int64
	State       []byte
	Timestamp   time.Time
}

// NewEventStore creates a new EventStore.
func NewEventStore() *EventStore {
	return &EventStore{
		events:    make(map[string][]EventEnvelope),
		snapshots: make(map[string]*Snapshot),
	}
}

// Execute appends an event to the store.
// Implements Adapter[EventEnvelope, AppendResult].
//
// Rules:
//  1. If EventID is empty, a UUID v4 is generated.
//  2. If Timestamp is zero, it is set to time.Now().
//  3. The write lock is acquired for the duration of the append.
//  4. If input.Version == 0, it is auto-set to currentVersion + 1.
//  5. If input.Version != currentVersion + 1, a version conflict error is returned.
//  6. The event is appended to the aggregate's event stream.
func (es *EventStore) Execute(ctx context.Context, input EventEnvelope) (AppendResult, error) {
	// 1. If EventID is empty, generate one
	if input.EventID == "" {
		input.EventID = generateEventID()
	}
	// 2. If Timestamp is zero, set to now
	if input.Timestamp.IsZero() {
		input.Timestamp = time.Now()
	}
	// 3. Lock for this aggregate
	es.mu.Lock()
	defer es.mu.Unlock()
	// 4. Get current version (max version of existing events for this aggregate)
	currentVersion := es.getLatestVersionLocked(input.AggregateID)
	// 5. If input.Version == 0, auto-set to currentVersion + 1
	if input.Version == 0 {
		input.Version = currentVersion + 1
	}
	// 6. If input.Version != currentVersion + 1, return error (version conflict)
	if input.Version != currentVersion+1 {
		return AppendResult{}, fmt.Errorf("version conflict: expected %d, got %d", currentVersion+1, input.Version)
	}
	// 7. Append the event
	es.events[input.AggregateID] = append(es.events[input.AggregateID], input)
	// 8. Return AppendResult with EventID, Version, Success=true
	return AppendResult{
		EventID: input.EventID,
		Version: input.Version,
		Success: true,
	}, nil
}

// Stream returns all events for the aggregate from the given version (inclusive),
// sorted by version in ascending order.
func (es *EventStore) Stream(aggregateID string, fromVersion int64) []EventEnvelope {
	es.mu.RLock()
	defer es.mu.RUnlock()
	all := es.events[aggregateID]
	result := make([]EventEnvelope, 0, len(all))
	for _, e := range all {
		if e.Version >= fromVersion {
			result = append(result, e)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Version < result[j].Version
	})
	return result
}

// StreamAll returns all events for the aggregate, sorted by version in ascending order.
func (es *EventStore) StreamAll(aggregateID string) []EventEnvelope {
	es.mu.RLock()
	defer es.mu.RUnlock()
	all := es.events[aggregateID]
	result := make([]EventEnvelope, len(all))
	copy(result, all)
	sort.Slice(result, func(i, j int) bool {
		return result[i].Version < result[j].Version
	})
	return result
}

// SaveSnapshot saves a snapshot for the aggregate.
func (es *EventStore) SaveSnapshot(aggregateID string, version int64, state []byte) {
	es.mu.Lock()
	defer es.mu.Unlock()
	es.snapshots[aggregateID] = &Snapshot{
		AggregateID: aggregateID,
		Version:     version,
		State:       state,
		Timestamp:   time.Now(),
	}
}

// GetSnapshot gets the latest snapshot for the aggregate.
// Returns the snapshot and true if found, or nil and false otherwise.
func (es *EventStore) GetSnapshot(aggregateID string) (*Snapshot, bool) {
	es.mu.RLock()
	defer es.mu.RUnlock()
	s, ok := es.snapshots[aggregateID]
	return s, ok
}

// GetLatestVersion returns the latest version number for the aggregate.
// Returns 0 if no events have been appended.
func (es *EventStore) GetLatestVersion(aggregateID string) int64 {
	es.mu.RLock()
	defer es.mu.RUnlock()
	return es.getLatestVersionLocked(aggregateID)
}

// Count returns the number of events for the aggregate.
func (es *EventStore) Count(aggregateID string) int {
	es.mu.RLock()
	defer es.mu.RUnlock()
	return len(es.events[aggregateID])
}

// ──────────────────────────────────────────────
// Internal helpers
// ──────────────────────────────────────────────

// getLatestVersionLocked returns the latest version for the aggregate.
// Caller must hold at least a read lock.
func (es *EventStore) getLatestVersionLocked(aggregateID string) int64 {
	events := es.events[aggregateID]
	if len(events) == 0 {
		return 0
	}
	maxVersion := int64(0)
	for _, e := range events {
		if e.Version > maxVersion {
			maxVersion = e.Version
		}
	}
	return maxVersion
}

// generateEventID generates a UUID v4 string for event identification.
func generateEventID() string {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		// Fallback: use a timestamp-based identifier
		return fmt.Sprintf("event-%d", time.Now().UnixNano())
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// ──────────────────────────────────────────────
// EventStoreBackend 接口适配 (v0.9.0)
// ──────────────────────────────────────────────

// Append 实现 EventStoreBackend 接口。
// 使用乐观并发控制：expectedVersion 必须等于当前最新版本。
func (es *EventStore) Append(ctx context.Context, event EventEnvelope, expectedVersion int64) (AppendResult, error) {
	// 生成 EventID 和 Timestamp
	if event.EventID == "" {
		event.EventID = generateEventID()
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	es.mu.Lock()
	defer es.mu.Unlock()

	currentVersion := es.getLatestVersionLocked(event.AggregateID)
	if currentVersion != expectedVersion {
		return AppendResult{}, NewVersionConflictError(expectedVersion, currentVersion)
	}

	event.Version = currentVersion + 1
	es.events[event.AggregateID] = append(es.events[event.AggregateID], event)

	return AppendResult{
		EventID: event.EventID,
		Version: event.Version,
		Success: true,
	}, nil
}

// StreamCtx 实现 EventStoreBackend.Stream（带 context）。
func (es *EventStore) StreamCtx(_ context.Context, aggregateID string, fromVersion int64) ([]EventEnvelope, error) {
	return es.Stream(aggregateID, fromVersion), nil
}

// GetLatestVersionCtx 实现 EventStoreBackend.GetLatestVersion（带 context）。
func (es *EventStore) GetLatestVersionCtx(_ context.Context, aggregateID string) (int64, error) {
	return es.GetLatestVersion(aggregateID), nil
}

// SaveSnapshotObj 实现 EventStoreBackend.SaveSnapshot（接受 Snapshot 对象）。
func (es *EventStore) SaveSnapshotObj(_ context.Context, snapshot Snapshot) error {
	es.mu.Lock()
	defer es.mu.Unlock()
	es.snapshots[snapshot.AggregateID] = &Snapshot{
		AggregateID: snapshot.AggregateID,
		Version:     snapshot.Version,
		State:       snapshot.State,
		Timestamp:   snapshot.Timestamp,
	}
	return nil
}

// GetSnapshotObj 实现 EventStoreBackend.GetSnapshot（返回 *Snapshot, error）。
func (es *EventStore) GetSnapshotObj(_ context.Context, aggregateID string) (*Snapshot, error) {
	es.mu.RLock()
	defer es.mu.RUnlock()
	s, ok := es.snapshots[aggregateID]
	if !ok {
		return nil, nil
	}
	return s, nil
}

// ListAggregates 实现 EventStoreBackend.ListAggregates。
func (es *EventStore) ListAggregates(_ context.Context) ([]string, error) {
	es.mu.RLock()
	defer es.mu.RUnlock()
	ids := make([]string, 0, len(es.events))
	for id := range es.events {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids, nil
}

// HealthCheck 实现 EventStoreBackend.HealthCheck。
func (es *EventStore) HealthCheck(_ context.Context) error {
	// 内存实现始终健康
	return nil
}

// Close 实现 EventStoreBackend.Close。
func (es *EventStore) Close() error {
	// 内存实现无需清理
	return nil
}

// Ensure EventStore implements EventStoreBackend.
var _ EventStoreBackend = (*EventStore)(nil)