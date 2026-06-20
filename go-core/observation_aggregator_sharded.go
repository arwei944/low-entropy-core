//go:build lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

package core

import (
	"sync"
	"time"
)

// =============================================================================
// ShardedAggregator — 分片聚合器 (T3.5)
// =============================================================================

// ShardedAggregator 将聚合器分片化，每分片独立维护 TDigest 和窗口。
// 查询时合并所有分片结果（TDigest 支持合并）。
//
// 在十亿级调用量下，单锁聚合器会成为瓶颈。256 分片使锁竞争降低 256 倍。
type ShardedAggregator struct {
	shards [shardCount]*shardedAggShard
	config AggregatorConfig
}

// shardedAggShard 是单个聚合分片。
type shardedAggShard struct {
	mu      sync.RWMutex
	digests map[string]*TDigest // key = duration_pattern_unit
	counts  map[string]int
	errors  map[string]int
}

// NewShardedAggregator 创建分片聚合器。
func NewShardedAggregator(config AggregatorConfig) *ShardedAggregator {
	if config.MaxWindows <= 0 {
		config.MaxWindows = 1000
	}
	sa := &ShardedAggregator{config: config}
	for i := 0; i < shardCount; i++ {
		sa.shards[i] = &shardedAggShard{
			digests: make(map[string]*TDigest),
			counts:  make(map[string]int),
			errors:  make(map[string]int),
		}
	}
	return sa
}

// Record 按 SpanID 哈希将步骤分发到对应分片，立即更新 TDigest。
func (sa *ShardedAggregator) Record(steps []ExecutionStep) {
	now := time.Now()

	for _, step := range steps {
		idx := hashString(step.SpanID) & 0xFF
		shard := sa.shards[idx]
		shard.mu.Lock()

		for _, windowDur := range sa.config.WindowDurations {
			windowStart := now.Add(-windowDur)
			if step.Timestamp.Before(windowStart) {
				continue
			}

			// 总体窗口
			sa.updateShardDigest(shard, step, windowDur, "", "")

			// 按 Pattern
			if step.Pattern != "" {
				sa.updateShardDigest(shard, step, windowDur, step.Pattern, "")
			}

			// 按 Unit
			if step.Unit != "" {
				sa.updateShardDigest(shard, step, windowDur, "", step.Unit)
			}
		}
		shard.mu.Unlock()
	}
}

// updateShardDigest 更新分片中的 TDigest。
func (sa *ShardedAggregator) updateShardDigest(shard *shardedAggShard, step ExecutionStep, dur time.Duration, pattern, unit string) {
	key := windowKey(dur, pattern, unit)
	d, ok := shard.digests[key]
	if !ok {
		d = NewTDigestDefault()
		shard.digests[key] = d
	}
	d.Add(float64(step.DurationMs))
	shard.counts[key]++
	if step.Error != nil {
		shard.errors[key]++
	}
}

// GetResults 合并所有分片的结果后返回。
// 每个 key 的 TDigest 跨分片合并。
func (sa *ShardedAggregator) GetResults() []AggregateResult {
	// 收集所有分片中的 key，合并 digests
	merged := make(map[string]*TDigest)
	mergedCounts := make(map[string]int)
	mergedErrors := make(map[string]int)

	for i := 0; i < shardCount; i++ {
		shard := sa.shards[i]
		shard.mu.RLock()
		for key, d := range shard.digests {
			if existing, ok := merged[key]; ok {
				existing.Merge(d)
			} else {
				merged[key] = d.Clone()
			}
			mergedCounts[key] += shard.counts[key]
			mergedErrors[key] += shard.errors[key]
		}
		shard.mu.RUnlock()
	}

	results := make([]AggregateResult, 0, len(merged))
	for key, d := range merged {
		count := mergedCounts[key]
		errCount := mergedErrors[key]

		r := AggregateResult{
			WindowDuration: key,
			Count:          count,
			ErrorCount:     errCount,
			AvgDurationMs:  d.Mean(),
			P50DurationMs:  int64(d.Quantile(0.50)),
			P99DurationMs:  int64(d.Quantile(0.99)),
			MinDurationMs:  int64(d.Min()),
			MaxDurationMs:  int64(d.Max()),
		}
		results = append(results, r)
	}

	return results
}

// QueryResults 按条件过滤分片聚合结果。
func (sa *ShardedAggregator) QueryResults(windowDur string, unit, pattern string) []AggregateResult {
	allResults := sa.GetResults()
	result := make([]AggregateResult, 0)
	for _, r := range allResults {
		if windowDur != "" && r.WindowDuration != windowDur {
			continue
		}
		if unit != "" && r.Unit != unit {
			continue
		}
		if pattern != "" && r.Pattern != pattern {
			continue
		}
		result = append(result, r)
	}
	return result
}
