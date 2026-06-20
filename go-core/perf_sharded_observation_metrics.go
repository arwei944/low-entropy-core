//go:build lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

// Package core — 分片步骤存储（带预构建索引）
//
// 本文件提供 ShardedStepStore：256 分片的 InMemoryStepStore，
// 带有预构建索引和环形缓冲区。查询时使用最选择性索引避免全扫描，支持分页。
package core

import (
	"sort"
	"sync"
	"sync/atomic"
)

// ──────────────────────────────────────────────────────────────────────────────
// ShardedStepStore — 256 分片步骤存储（带预构建索引）
// ──────────────────────────────────────────────────────────────────────────────

// stepIndex 是单个分片的索引结构，存储位置映射。
type stepIndex struct {
	byTraceID map[string][]int // TraceID -> 位置列表
	byPattern map[string][]int // Pattern -> 位置列表
	byUnit    map[string][]int // Unit -> 位置列表
}

// ShardedStepStore 是 InMemoryStepStore 的分片版本。
// 每个分片有独立的环形缓冲区、锁和预构建索引。
// 查询时使用最选择性索引避免全扫描，支持分页。
type ShardedStepStore struct {
	shards    [shardCount]*stepStoreShard
	totalCount atomic.Int64
}

// stepStoreShard 是单个存储分片。
type stepStoreShard struct {
	mu       sync.RWMutex
	steps    []ExecutionStep
	capacity int
	head     int // 写位置
	size     int // 有效条目数
	index    *stepIndex
}

// NewShardedStepStore 创建一个新的分片步骤存储。
// capacity 为每个分片的环形缓冲区容量。
func NewShardedStepStore(capacity int) *ShardedStepStore {
	if capacity <= 0 {
		capacity = 1000
	}
	s := &ShardedStepStore{}
	for i := 0; i < shardCount; i++ {
		s.shards[i] = &stepStoreShard{
			steps:    make([]ExecutionStep, capacity),
			capacity: capacity,
			index: &stepIndex{
				byTraceID: make(map[string][]int),
				byPattern: make(map[string][]int),
				byUnit:    make(map[string][]int),
			},
		}
	}
	return s
}

// Record 存储执行步骤到分片环形缓冲区中。
// 使用 SpanID 哈希选择分片，同步更新索引。
func (s *ShardedStepStore) Record(steps []ExecutionStep) {
	if len(steps) == 0 {
		return
	}

	// 按分片分组
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
			// 如果覆盖旧条目，清理其索引
			if shard.size == shard.capacity {
				old := shard.steps[shard.head]
				s.removeFromIndex(shard.index, old, shard.head)
			}
			shard.steps[shard.head] = step
			// 更新索引
			s.addToIndex(shard.index, step, pos)
			shard.head = (shard.head + 1) % shard.capacity
			if shard.size < shard.capacity {
				shard.size++
			}
		}
		shard.mu.Unlock()
	}

	s.totalCount.Add(int64(len(steps)))
}

// addToIndex 将步骤添加到分片索引中。
func (s *ShardedStepStore) addToIndex(idx *stepIndex, step ExecutionStep, pos int) {
	if step.TraceID != "" {
		idx.byTraceID[step.TraceID] = append(idx.byTraceID[step.TraceID], pos)
	}
	if step.Pattern != "" {
		idx.byPattern[step.Pattern] = append(idx.byPattern[step.Pattern], pos)
	}
	if step.Unit != "" {
		idx.byUnit[step.Unit] = append(idx.byUnit[step.Unit], pos)
	}
}

// removeFromIndex 从分片索引中移除指定位置的条目。
func (s *ShardedStepStore) removeFromIndex(idx *stepIndex, step ExecutionStep, pos int) {
	if step.TraceID != "" {
		idx.byTraceID[step.TraceID] = removePos(idx.byTraceID[step.TraceID], pos)
	}
	if step.Pattern != "" {
		idx.byPattern[step.Pattern] = removePos(idx.byPattern[step.Pattern], pos)
	}
	if step.Unit != "" {
		idx.byUnit[step.Unit] = removePos(idx.byUnit[step.Unit], pos)
	}
}

// removePos 从位置列表中移除指定位置。
func removePos(positions []int, target int) []int {
	for i, p := range positions {
		if p == target {
			return append(positions[:i], positions[i+1:]...)
		}
	}
	return positions
}

// Query 使用索引加速查询，支持分页。
// 选择最选择性索引（索引基数最高的字段）来最小化扫描范围。
func (s *ShardedStepStore) Query(q StepQuery) ([]ExecutionStep, int) {
	// 确定使用哪个索引
	useIdx := s.selectBestIndex(q)

	// 跨分片收集结果
	var allResults []ExecutionStep
	var total int

	for i := 0; i < shardCount; i++ {
		shard := s.shards[i]
		shard.mu.RLock()
		shardResults := s.queryShard(shard, q, useIdx)
		shard.mu.RUnlock()
		allResults = append(allResults, shardResults...)
	}

	// 按时间戳排序
	sort.Slice(allResults, func(i, j int) bool {
		return allResults[i].Timestamp.Before(allResults[j].Timestamp)
	})

	total = len(allResults)

	// 应用分页
	if q.Limit > 0 && len(allResults) > q.Limit {
		allResults = allResults[:q.Limit]
	}

	return allResults, total
}

// selectBestIndex 选择最选择性索引。
// 优先级：TraceID > Pattern > Unit > 全扫描
func (s *ShardedStepStore) selectBestIndex(q StepQuery) string {
	if q.TraceID != "" {
		return "trace_id"
	}
	if q.Pattern != "" {
		return "pattern"
	}
	if q.Unit != "" {
		return "unit"
	}
	return "" // 全扫描
}

// queryShard 在单个分片中执行查询。
func (s *ShardedStepStore) queryShard(shard *stepStoreShard, q StepQuery, useIdx string) []ExecutionStep {
	// 确定候选位置
	var candidates []int
	switch useIdx {
	case "trace_id":
		candidates = shard.index.byTraceID[q.TraceID]
	case "pattern":
		candidates = shard.index.byPattern[q.Pattern]
	case "unit":
		candidates = shard.index.byUnit[q.Unit]
	default:
		// 全扫描：收集所有有效位置
		candidates = s.allPositions(shard)
	}

	if len(candidates) == 0 {
		return nil
	}

	result := make([]ExecutionStep, 0, len(candidates))
	for _, pos := range candidates {
		step := shard.steps[pos]
		// 使用索引后的二次过滤（因为索引可能包含已过期的条目）
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

// allPositions 返回分片中所有有效条目的位置。
func (s *ShardedStepStore) allPositions(shard *stepStoreShard) []int {
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

// Count 返回总步骤数（原子读取）。
func (s *ShardedStepStore) Count() int {
	return int(s.totalCount.Load())
}

// Clear 清空所有分片。
func (s *ShardedStepStore) Clear() {
	for i := 0; i < shardCount; i++ {
		shard := s.shards[i]
		shard.mu.Lock()
		shard.head = 0
		shard.size = 0
		shard.index = &stepIndex{
			byTraceID: make(map[string][]int),
			byPattern: make(map[string][]int),
			byUnit:    make(map[string][]int),
		}
		shard.mu.Unlock()
	}
	s.totalCount.Store(0)
}
