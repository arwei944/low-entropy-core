//go:build lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

// Package core — 持久化索引分片步骤存储
//
// 本文件提供 ShardedIndexedStepStore：基于 sync.Map 的持久化索引版本，
// 支持并发安全的索引更新和查询。适合高并发写入场景。
package core

import (
	"sort"
	"sync"
	"sync/atomic"
)

// ──────────────────────────────────────────────────────────────────────────────
// ShardedIndexedStepStore — 持久化索引版本（sync.Map）
// ──────────────────────────────────────────────────────────────────────────────

// ShardedIndexedStepStore 是 ShardedStepStore 的增强版本，
// 使用 sync.Map 作为索引存储，支持并发安全的索引更新和查询。
//
// 与 ShardedStepStore 的区别：
//   - 索引使用 sync.Map，允许多个 goroutine 并发更新同一索引
//   - 适合高并发写入场景，索引更新不会阻塞读取
//   - 索引条目使用 sync.Pool 复用，减少 GC 压力
type ShardedIndexedStepStore struct {
	shards     [shardCount]*indexedShard
	totalCount atomic.Int64
}

// indexedShard 是带有持久化索引的存储分片。
type indexedShard struct {
	mu       sync.RWMutex
	steps    []ExecutionStep
	capacity int
	head     int
	size     int
	// 持久化索引使用 sync.Map
	idxTraceID sync.Map // map[string]*indexEntry
	idxPattern sync.Map // map[string]*indexEntry
	idxUnit    sync.Map // map[string]*indexEntry
}

// indexEntry 是索引条目，存储位置列表。
// 使用 sync.Pool 复用，减少 GC 压力。
type indexEntry struct {
	positions []int
}

// indexEntryPool 是 indexEntry 的 sync.Pool。
var indexEntryPool = sync.Pool{
	New: func() any {
		return &indexEntry{positions: make([]int, 0, 16)}
	},
}

// NewShardedIndexedStepStore 创建一个新的持久化索引分片存储。
func NewShardedIndexedStepStore(capacity int) *ShardedIndexedStepStore {
	if capacity <= 0 {
		capacity = 1000
	}
	s := &ShardedIndexedStepStore{}
	for i := 0; i < shardCount; i++ {
		s.shards[i] = &indexedShard{
			steps:    make([]ExecutionStep, capacity),
			capacity: capacity,
		}
	}
	return s
}

// Record 存储执行步骤到分片，同步更新持久化索引。
func (s *ShardedIndexedStepStore) Record(steps []ExecutionStep) {
	if len(steps) == 0 {
		return
	}

	shardGroups := make([][]ExecutionStep, shardCount)
	for i := range steps {
		idx := hashString(steps[i].SpanID) & 0xFF
		shardGroups[idx] = append(shardGroups[idx], steps[i])
	}

	for i := 0; i < shardCount; i++ {
		if len(shardGroups[i]) == 0 {
			continue
		}
		shard := s.shards[i]
		shard.mu.Lock()
		for _, step := range shardGroups[i] {
			pos := shard.head
			if shard.size == shard.capacity {
				old := shard.steps[shard.head]
				s.removeFromIndexedLocked(shard, old, shard.head)
			}
			shard.steps[shard.head] = step
			s.addToIndexedLocked(shard, step, pos)
			shard.head = (shard.head + 1) % shard.capacity
			if shard.size < shard.capacity {
				shard.size++
			}
		}
		shard.mu.Unlock()
	}

	s.totalCount.Add(int64(len(steps)))
}

// addToIndexedLocked 将步骤添加到持久化索引（调用者持有写锁）。
func (s *ShardedIndexedStepStore) addToIndexedLocked(shard *indexedShard, step ExecutionStep, pos int) {
	if step.TraceID != "" {
		s.appendToIndex(&shard.idxTraceID, step.TraceID, pos)
	}
	if step.Pattern != "" {
		s.appendToIndex(&shard.idxPattern, step.Pattern, pos)
	}
	if step.Unit != "" {
		s.appendToIndex(&shard.idxUnit, step.Unit, pos)
	}
}

// removeFromIndexedLocked 从持久化索引中移除条目（调用者持有写锁）。
func (s *ShardedIndexedStepStore) removeFromIndexedLocked(shard *indexedShard, step ExecutionStep, pos int) {
	if step.TraceID != "" {
		s.removeFromIndex(&shard.idxTraceID, step.TraceID, pos)
	}
	if step.Pattern != "" {
		s.removeFromIndex(&shard.idxPattern, step.Pattern, pos)
	}
	if step.Unit != "" {
		s.removeFromIndex(&shard.idxUnit, step.Unit, pos)
	}
}

// appendToIndex 向 sync.Map 索引追加位置。
func (s *ShardedIndexedStepStore) appendToIndex(m *sync.Map, key string, pos int) {
	entry := indexEntryPool.Get().(*indexEntry)
	entry.positions = entry.positions[:0]

	actual, loaded := m.LoadOrStore(key, entry)
	if loaded {
		// 键已存在，归还新分配的 entry
		indexEntryPool.Put(entry)
		e := actual.(*indexEntry)
		// 注意：sync.Map 的值是共享的，这里需要小心并发
		// 由于调用者持有写锁，此时是安全的
		e.positions = append(e.positions, pos)
	} else {
		// 键不存在，entry 已存储
		entry.positions = append(entry.positions, pos)
	}
}

// removeFromIndex 从 sync.Map 索引中移除位置。
func (s *ShardedIndexedStepStore) removeFromIndex(m *sync.Map, key string, pos int) {
	actual, ok := m.Load(key)
	if !ok {
		return
	}
	e := actual.(*indexEntry)
	for i, p := range e.positions {
		if p == pos {
			e.positions = append(e.positions[:i], e.positions[i+1:]...)
			break
		}
	}
	// 如果位置列表为空，删除键
	if len(e.positions) == 0 {
		m.Delete(key)
	}
}

// Query 使用持久化索引加速查询。
func (s *ShardedIndexedStepStore) Query(q StepQuery) ([]ExecutionStep, int) {
	useIdx := s.selectBestIndex(q)

	var allResults []ExecutionStep

	for i := 0; i < shardCount; i++ {
		shard := s.shards[i]
		shard.mu.RLock()
		shardResults := s.queryIndexedShard(shard, q, useIdx)
		shard.mu.RUnlock()
		allResults = append(allResults, shardResults...)
	}

	sort.Slice(allResults, func(i, j int) bool {
		return allResults[i].Timestamp.Before(allResults[j].Timestamp)
	})

	total := len(allResults)
	if q.Limit > 0 && len(allResults) > q.Limit {
		allResults = allResults[:q.Limit]
	}

	return allResults, total
}

// selectBestIndex 选择最选择性索引。
func (s *ShardedIndexedStepStore) selectBestIndex(q StepQuery) string {
	if q.TraceID != "" {
		return "trace_id"
	}
	if q.Pattern != "" {
		return "pattern"
	}
	if q.Unit != "" {
		return "unit"
	}
	return ""
}

// queryIndexedShard 在单个分片中执行查询，使用持久化索引。
func (s *ShardedIndexedStepStore) queryIndexedShard(shard *indexedShard, q StepQuery, useIdx string) []ExecutionStep {
	var candidates []int
	switch useIdx {
	case "trace_id":
		if actual, ok := shard.idxTraceID.Load(q.TraceID); ok {
			candidates = actual.(*indexEntry).positions
		}
	case "pattern":
		if actual, ok := shard.idxPattern.Load(q.Pattern); ok {
			candidates = actual.(*indexEntry).positions
		}
	case "unit":
		if actual, ok := shard.idxUnit.Load(q.Unit); ok {
			candidates = actual.(*indexEntry).positions
		}
	default:
		candidates = s.allIndexedPositions(shard)
	}

	if len(candidates) == 0 {
		return nil
	}

	result := make([]ExecutionStep, 0, len(candidates))
	// 收集当前有效位置
	validPositions := s.allIndexedPositions(shard)
	validSet := make(map[int]bool, len(validPositions))
	for _, p := range validPositions {
		validSet[p] = true
	}

	for _, pos := range candidates {
		if !validSet[pos] {
			continue
		}
		step := shard.steps[pos]
		if q.TraceID != "" && step.TraceID != q.TraceID {
			continue
		}
		if q.Pattern != "" && step.Pattern != q.Pattern {
			continue
		}
		if q.Unit != "" && step.Unit != q.Unit {
			continue
		}
		if !q.Since.IsZero() && step.Timestamp.Before(q.Since) {
			continue
		}
		if q.ErrorOnly && step.Error == nil {
			continue
		}
		result = append(result, step)
	}
	return result
}

// allIndexedPositions 返回分片中所有有效条目位置。
func (s *ShardedIndexedStepStore) allIndexedPositions(shard *indexedShard) []int {
	if shard.size == 0 {
		return nil
	}
	start := shard.head - shard.size
	if start < 0 {
		start += shard.capacity
	}
	positions := make([]int, shard.size)
	for i := 0; i < shard.size; i++ {
		positions[i] = (start + i) % shard.capacity
	}
	return positions
}

// Count 返回总步骤数。
func (s *ShardedIndexedStepStore) Count() int {
	return int(s.totalCount.Load())
}

// Clear 清空所有分片。
func (s *ShardedIndexedStepStore) Clear() {
	for i := 0; i < shardCount; i++ {
		shard := s.shards[i]
		shard.mu.Lock()
		shard.head = 0
		shard.size = 0
		shard.idxTraceID = sync.Map{}
		shard.idxPattern = sync.Map{}
		shard.idxUnit = sync.Map{}
		shard.mu.Unlock()
	}
	s.totalCount.Store(0)
}
