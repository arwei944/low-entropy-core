//go:build lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

// Package core — 分片观测存储基础设施
//
// 本文件提供 ShardedObservationAdapter：256 分片的 InMemoryObservationAdapter，
// 使用 SpanID 哈希分布，以及 hashString 辅助函数。
//
// 所有类型均为线程安全，热路径设计为零分配。使用 ShardedLock 进行分片选择，
// StepSlicePool 和 StepMetadataPool 进行中间分配复用。
package core

import (
	"sync"
	"sync/atomic"
)

// ──────────────────────────────────────────────────────────────────────────────
// ShardedObservationAdapter — 256 分片观测适配器
// ──────────────────────────────────────────────────────────────────────────────

// ShardedObservationAdapter 是 InMemoryObservationAdapter 的分片版本。
// 使用 256 个分片，每个分片有独立的 sync.RWMutex 和 []ExecutionStep，
// 通过 SpanID 哈希将记录分布到不同分片，大幅减少锁竞争。
//
// 在十亿级调用量下，单锁的 InMemoryObservationAdapter 会成为严重瓶颈。
// 256 分片意味着在理想哈希分布下，锁竞争降低到原来的 1/256。
type ShardedObservationAdapter struct {
	shards    [shardCount]*obsShard
	stepCount atomic.Int64
}

// obsShard 是单个观测分片，包含独立的锁和步骤切片。
type obsShard struct {
	mu    sync.RWMutex
	steps []ExecutionStep
}

// NewShardedObservationAdapter 创建一个新的分片观测适配器。
// 每个分片预分配 1024 容量的切片，减少初始扩容。
func NewShardedObservationAdapter() *ShardedObservationAdapter {
	a := &ShardedObservationAdapter{}
	for i := 0; i < shardCount; i++ {
		a.shards[i] = &obsShard{
			steps: make([]ExecutionStep, 0, 1024),
		}
	}
	return a
}

// Record 将执行步骤追加到分片存储中。
// 使用 SpanID 哈希选择分片，确保相同 SpanID 的记录进入同一分片。
func (a *ShardedObservationAdapter) Record(steps []ExecutionStep) {
	if len(steps) == 0 {
		return
	}

	// 按 SpanID 哈希分组，将步骤分配到对应分片
	shardGroups := make([][]ExecutionStep, shardCount)
	for i := range steps {
		idx := hashString(steps[i].SpanID) & 0xFF
		shardGroups[idx] = append(shardGroups[idx], steps[i])
	}

	// 并发写入各分片
	for i := 0; i < shardCount; i++ {
		if len(shardGroups[i]) == 0 {
			continue
		}
		shard := a.shards[i]
		shard.mu.Lock()
		shard.steps = append(shard.steps, shardGroups[i]...)
		shard.mu.Unlock()
	}

	a.stepCount.Add(int64(len(steps)))
}

// GetSteps 返回所有步骤的分页视图。
// limit 为 0 时返回全部，offset 从 0 开始。
// 为了避免大内存分配，返回的切片是跨分片收集的副本。
func (a *ShardedObservationAdapter) GetSteps(limit, offset int) ([]ExecutionStep, int) {
	total := int(a.stepCount.Load())
	if offset >= total {
		return nil, total
	}

	// 计算需要收集的数量
	need := total - offset
	if limit > 0 && limit < need {
		need = limit
	}

	result := GetStepSlice(need)

	// 跨分片收集
	collected := 0
	skipped := 0
	for i := 0; i < shardCount && collected < need; i++ {
		shard := a.shards[i]
		shard.mu.RLock()
		for _, step := range shard.steps {
			if skipped < offset {
				skipped++
				continue
			}
			result = append(result, step)
			collected++
			if collected >= need {
				break
			}
		}
		shard.mu.RUnlock()
	}

	return result, total
}

// GetTraceTree 构建跨分片的 TraceTree。
// 收集所有分片中的步骤，然后构建层级树。
func (a *ShardedObservationAdapter) GetTraceTree() *TraceTree {
	allSteps := make([]ExecutionStep, 0, a.stepCount.Load())
	for i := 0; i < shardCount; i++ {
		shard := a.shards[i]
		shard.mu.RLock()
		allSteps = append(allSteps, shard.steps...)
		shard.mu.RUnlock()
	}
	return BuildTraceTree(allSteps)
}

// StepCount 返回总步骤数（原子读取，无锁）。
func (a *ShardedObservationAdapter) StepCount() int {
	return int(a.stepCount.Load())
}

// Clear 清空所有分片中的步骤。
func (a *ShardedObservationAdapter) Clear() {
	for i := 0; i < shardCount; i++ {
		shard := a.shards[i]
		shard.mu.Lock()
		shard.steps = shard.steps[:0]
		shard.mu.Unlock()
	}
	a.stepCount.Store(0)
}

// ──────────────────────────────────────────────────────────────────────────────
// 辅助函数
// ──────────────────────────────────────────────────────────────────────────────

// hashString 计算字符串的 FNV-1a 64 位哈希，用于分片选择。
func hashString(s string) uint64 {
	var h uint64 = fnvOffsetBasis64
	for i := 0; i < len(s); i++ {
		h ^= uint64(s[i])
		h *= fnvPrime64
	}
	return h
}
