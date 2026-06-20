//go:build lecore_tier3 || lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

// Package core — 持久化事件存储 (Phase 1, Task 1.2)
//
// PersistentEventStore 将 EventStore 的事件持久化到 StorageBackend。
// 支持：
//   - 按 Aggregate 存储事件流（每个 Aggregate 一个 JSON 文件）
//   - 快照持久化
//   - 从持久化后端恢复（启动时加载所有事件）
//
// 设计原则：
//   - 每次 Execute 后立即持久化（同步写入，保证一致性）
//   - 使用 JSON 格式存储（人类可读、可调试）
//   - 快照单独存储，避免与事件流混淆
package core

import (
	"context"
	"fmt"
	"time"
)

// ──────────────────────────────────────────────
// SECTION 1: PersistentEventStore
// ──────────────────────────────────────────────

// PersistentEventStore 在 EventStore 基础上增加了持久化支持。
// 每次 Execute 后自动同步到 StorageBackend。
// 启动时从 StorageBackend 恢复事件。
type PersistentEventStore struct {
	inner   *EventStore
	backend StorageBackend
}

// NewPersistentEventStore 创建持久化事件存储。
// 自动从后端恢复已有事件。
func NewPersistentEventStore(backend StorageBackend) (*PersistentEventStore, error) {
	if backend == nil {
		return nil, fmt.Errorf("persistent eventstore: StorageBackend is nil")
	}

	pes := &PersistentEventStore{
		inner:   NewEventStore(),
		backend: backend,
	}

	// 启动时恢复
	if err := pes.restore(context.Background()); err != nil {
		return nil, fmt.Errorf("persistent eventstore: restore: %w", err)
	}

	return pes, nil
}

// Execute 追加事件到事件存储并持久化。
// 继承 EventStore 的所有版本控制规则。
func (pes *PersistentEventStore) Execute(ctx context.Context, input EventEnvelope) (AppendResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	// 1. 追加到内存存储
	result, err := pes.inner.Execute(ctx, input)
	if err != nil {
		return result, err
	}

	// 2. 持久化到后端
	eventFile := pes.eventKey(input.AggregateID)
	events := pes.inner.StreamAll(input.AggregateID)

	if err := SaveJSON(ctx, pes.backend, eventFile, events); err != nil {
		return result, fmt.Errorf("persistent eventstore: save %s: %w", eventFile, err)
	}

	return result, nil
}

// Stream 返回指定 Aggregate 的事件流（从内存读取）。
func (pes *PersistentEventStore) Stream(aggregateID string, fromVersion int64) []EventEnvelope {
	return pes.inner.streamNoCtx(aggregateID, fromVersion)
}

// StreamAll 返回指定 Aggregate 的所有事件（从内存读取）。
func (pes *PersistentEventStore) StreamAll(aggregateID string) []EventEnvelope {
	return pes.inner.StreamAll(aggregateID)
}

// SaveSnapshot 保存快照到内存和后端。
func (pes *PersistentEventStore) SaveSnapshot(aggregateID string, version int64, state []byte) {
	pes.inner.saveSnapshotNoCtx(aggregateID, version, state)

	// 持久化快照
	snapshot := Snapshot{
		AggregateID: aggregateID,
		Version:     version,
		State:       state,
		Timestamp:   time.Now(),
	}
	snapshotKey := pes.snapshotKey(aggregateID)
	SaveJSON(context.Background(), pes.backend, snapshotKey, snapshot)
}

// GetSnapshot 获取快照（从内存读取）。
func (pes *PersistentEventStore) GetSnapshot(aggregateID string) (*Snapshot, bool) {
	return pes.inner.getSnapshotNoCtx(aggregateID)
}

// GetLatestVersion 获取最新版本。
func (pes *PersistentEventStore) GetLatestVersion(aggregateID string) int64 {
	return pes.inner.getLatestVersionNoCtx(aggregateID)
}

// Count 返回事件数量。
func (pes *PersistentEventStore) Count(aggregateID string) int {
	return pes.inner.Count(aggregateID)
}

// ListAggregates 列出所有已知的 Aggregate ID。
func (pes *PersistentEventStore) ListAggregates(ctx context.Context) ([]string, error) {
	keys, err := pes.backend.List(ctx, "events/")
	if err != nil {
		return nil, err
	}
	var ids []string
	for _, key := range keys {
		// key 格式: events/{aggregateID}.json
		id := key[len("events/"):]
		if len(id) > 5 {
			id = id[:len(id)-5] // 去掉 .json
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// ──────────────────────────────────────────────
// SECTION 2: 恢复逻辑
// ──────────────────────────────────────────────

// restore 从后端恢复所有事件和快照。
func (pes *PersistentEventStore) restore(ctx context.Context) error {
	// 恢复事件
	keys, err := pes.backend.List(ctx, "events/")
	if err != nil {
		return err
	}

	for _, key := range keys {
		var events []EventEnvelope
		if err := LoadJSON(ctx, pes.backend, key, &events); err != nil {
			continue // 跳过损坏的文件
		}
		for _, e := range events {
			pes.inner.Execute(ctx, e)
		}
	}

	// 恢复快照
	snapshotKeys, err := pes.backend.List(ctx, "snapshots/")
	if err != nil {
		return err
	}

	for _, key := range snapshotKeys {
		var snapshot Snapshot
		if err := LoadJSON(ctx, pes.backend, key, &snapshot); err != nil {
			continue
		}
		pes.inner.saveSnapshotNoCtx(snapshot.AggregateID, snapshot.Version, snapshot.State)
	}

	return nil
}

// ──────────────────────────────────────────────
// SECTION 3: 存储 key 管理
// ──────────────────────────────────────────────

// eventKey 生成事件存储 key。
func (pes *PersistentEventStore) eventKey(aggregateID string) string {
	return fmt.Sprintf("events/%s.json", aggregateID)
}

// snapshotKey 生成快照存储 key。
func (pes *PersistentEventStore) snapshotKey(aggregateID string) string {
	return fmt.Sprintf("snapshots/%s.json", aggregateID)
}

// ──────────────────────────────────────────────
// SECTION 4: 持久化事件存储统计
// ──────────────────────────────────────────────

// EventStoreStat 持久化事件存储统计。
type EventStoreStat struct {
	AggregateCount int `json:"aggregate_count"`
	TotalEvents    int `json:"total_events"`
}

// Stat 返回持久化事件存储统计。
func (pes *PersistentEventStore) Stat(ctx context.Context) (*EventStoreStat, error) {
	aggregates, err := pes.ListAggregates(ctx)
	if err != nil {
		return nil, err
	}

	totalEvents := 0
	for _, id := range aggregates {
		totalEvents += pes.Count(id)
	}

	return &EventStoreStat{
		AggregateCount: len(aggregates),
		TotalEvents:    totalEvents,
	}, nil
}