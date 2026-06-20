//go:build lecore_tier5 || lecore_tier6 || lecore_tier7

// Package core — 事件溯源升级 (v4.0)
//
// 本文件包含事件溯源层的全面升级：
//   - ShardedEventStore：分片锁 EventStore，按 AggregateID 分片
//   - AutoSnapshotTrigger：自动快照触发
//   - EventCompaction：事件压缩
//   - EventBus 优化：Worker Pool + 分片锁订阅管理
//   - Projection 检查点与批量处理
package core

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// =============================================================================
// ShardedEventStore — 分片锁 EventStore (T6.1)
// =============================================================================

// ShardedEventStore 是 EventStore 的分片版本。
// 使用 ShardedLock[AggregateID] 按 AggregateID 分片，每个 Aggregate 独立锁。
// 不同 Aggregate 的读写完全并行，消除全局锁瓶颈。
type ShardedEventStore struct {
	shards [shardCount]*eventStoreShard
}

// eventStoreShard 是单个事件存储分片。
type eventStoreShard struct {
	mu        sync.RWMutex
	events    map[string][]EventEnvelope
	snapshots map[string]*Snapshot
	versions  map[string]int64 // aggregateID -> latest version (cache)
}

// NewShardedEventStore 创建分片事件存储。
func NewShardedEventStore() *ShardedEventStore {
	s := &ShardedEventStore{}
	for i := 0; i < shardCount; i++ {
		s.shards[i] = &eventStoreShard{
			events:    make(map[string][]EventEnvelope),
			snapshots: make(map[string]*Snapshot),
			versions:  make(map[string]int64),
		}
	}
	return s
}

// Execute 追加事件到分片存储。
// 使用 AggregateID 哈希选择分片，不同 Aggregate 并发无竞争。
func (s *ShardedEventStore) Execute(ctx context.Context, input EventEnvelope) (AppendResult, error) {
	select {
	case <-ctx.Done():
		return AppendResult{}, ctx.Err()
	default:
	}

	if input.EventID == "" {
		input.EventID = getGlobalUUIDGen().NextString()
	}
	if input.Timestamp.IsZero() {
		input.Timestamp = time.Now()
	}

	idx := hashString(input.AggregateID) & 0xFF
	shard := s.shards[idx]
	shard.mu.Lock()
	defer shard.mu.Unlock()

	// 获取最新版本（O(1) 缓存读取）
	latest := shard.versions[input.AggregateID]
	nextVersion := latest + 1

	// 乐观并发控制
	if input.Version > 0 && input.Version != nextVersion {
		return AppendResult{}, fmt.Errorf(
			"version conflict: expected %d, got %d for aggregate %s",
			nextVersion, input.Version, input.AggregateID,
		)
	}

	input.Version = nextVersion
	shard.events[input.AggregateID] = append(shard.events[input.AggregateID], input)
	shard.versions[input.AggregateID] = nextVersion

	return AppendResult{
		EventID: input.EventID,
		Version: nextVersion,
		Success: true,
	}, nil
}

// Stream 返回指定 Aggregate 的事件流。
// fromVersion=0 表示从第一个事件开始。
func (s *ShardedEventStore) Stream(aggregateID string, fromVersion int64) []EventEnvelope {
	idx := hashString(aggregateID) & 0xFF
	shard := s.shards[idx]
	shard.mu.RLock()
	defer shard.mu.RUnlock()

	events := shard.events[aggregateID]
	if fromVersion <= 0 {
		result := make([]EventEnvelope, len(events))
		copy(result, events)
		return result
	}

	var result []EventEnvelope
	for _, e := range events {
		if e.Version >= fromVersion {
			result = append(result, e)
		}
	}
	return result
}

// GetLatestVersion 返回指定 Aggregate 的最新版本（O(1)）。
func (s *ShardedEventStore) GetLatestVersion(aggregateID string) int64 {
	idx := hashString(aggregateID) & 0xFF
	shard := s.shards[idx]
	shard.mu.RLock()
	defer shard.mu.RUnlock()
	return shard.versions[aggregateID]
}

// SaveSnapshot 保存 Aggregate 快照。
func (s *ShardedEventStore) SaveSnapshot(aggregateID string, snapshot *Snapshot) {
	idx := hashString(aggregateID) & 0xFF
	shard := s.shards[idx]
	shard.mu.Lock()
	defer shard.mu.Unlock()
	shard.snapshots[aggregateID] = snapshot
}

// GetSnapshot 获取 Aggregate 快照。
func (s *ShardedEventStore) GetSnapshot(aggregateID string) *Snapshot {
	idx := hashString(aggregateID) & 0xFF
	shard := s.shards[idx]
	shard.mu.RLock()
	defer shard.mu.RUnlock()
	return shard.snapshots[aggregateID]
}

// Clear 清空所有事件。
func (s *ShardedEventStore) Clear() {
	for i := 0; i < shardCount; i++ {
		shard := s.shards[i]
		shard.mu.Lock()
		shard.events = make(map[string][]EventEnvelope)
		shard.snapshots = make(map[string]*Snapshot)
		shard.versions = make(map[string]int64)
		shard.mu.Unlock()
	}
}
