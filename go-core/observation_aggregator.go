package core

import (
	"sort"
	"sync"
	"time"
)

// ──────────────────────────────────────────────
// Aggregator — time-window step aggregation
// ──────────────────────────────────────────────

// AggregateResult is the output of an aggregation window.
type AggregateResult struct {
	// WindowStart is the start time of the aggregation window.
	WindowStart time.Time `json:"window_start"`

	// WindowEnd is the end time of the aggregation window.
	WindowEnd time.Time `json:"window_end"`

	// WindowDuration is the duration of the window (1m, 5m, 1h).
	WindowDuration string `json:"window_duration"`

	// Pattern is the pattern being aggregated (empty = all).
	Pattern string `json:"pattern,omitempty"`

	// Unit is the unit type being aggregated (empty = all).
	Unit string `json:"unit,omitempty"`

	// Count is the total number of steps in the window.
	Count int `json:"count"`

	// ErrorCount is the number of steps with errors.
	ErrorCount int `json:"error_count"`

	// AvgDurationMs is the average duration in milliseconds.
	AvgDurationMs float64 `json:"avg_duration_ms"`

	// P50DurationMs is the 50th percentile (median) duration.
	P50DurationMs int64 `json:"p50_duration_ms"`

	// P99DurationMs is the 99th percentile duration.
	P99DurationMs int64 `json:"p99_duration_ms"`

	// MinDurationMs is the minimum duration.
	MinDurationMs int64 `json:"min_duration_ms"`

	// MaxDurationMs is the maximum duration.
	MaxDurationMs int64 `json:"max_duration_ms"`

	// v4.0: 分布漂移与异常标注
	DistributionDrifted bool            `json:"distribution_drifted,omitempty"`
	DriftDirection      DriftDirection  `json:"drift_direction,omitempty"`
	AnomalyLabel        AnomalyLabelType `json:"anomaly_label,omitempty"`
	AnomalyScore        float64          `json:"anomaly_score,omitempty"`
}

// AggregatorConfig configures the aggregation windows.
type AggregatorConfig struct {
	// WindowDurations defines the aggregation windows (e.g., 1m, 5m, 1h).
	WindowDurations []time.Duration

	// MaxWindows limits the number of windows kept in memory.
	MaxWindows int
}

// DefaultAggregatorConfig returns a sensible default configuration.
func DefaultAggregatorConfig() AggregatorConfig {
	return AggregatorConfig{
		WindowDurations: []time.Duration{
			1 * time.Minute,
			5 * time.Minute,
			1 * time.Hour,
		},
		MaxWindows: 1000,
	}
}

// Aggregator aggregates ExecutionSteps into time-windowed AggregateResults.
// Thread-safe for concurrent use.
type Aggregator struct {
	mu       sync.RWMutex
	config   AggregatorConfig
	results  []AggregateResult
}

// NewAggregator creates a new aggregator with the given config.
func NewAggregator(config AggregatorConfig) *Aggregator {
	if config.MaxWindows <= 0 {
		config.MaxWindows = 1000
	}
	return &Aggregator{
		config:  config,
		results: make([]AggregateResult, 0),
	}
}

// Aggregate processes a batch of ExecutionSteps and produces AggregateResults
// for each configured window duration.
func (a *Aggregator) Aggregate(steps []ExecutionStep) []AggregateResult {
	if len(steps) == 0 {
		return nil
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	now := time.Now()
	newResults := make([]AggregateResult, 0)

	for _, windowDur := range a.config.WindowDurations {
		windowStart := now.Add(-windowDur)

		// Filter steps within the window
		windowSteps := make([]ExecutionStep, 0)
		for _, step := range steps {
			if step.Timestamp.After(windowStart) {
				windowSteps = append(windowSteps, step)
			}
		}

		if len(windowSteps) == 0 {
			continue
		}

		// Compute overall aggregate
		result := a.computeAggregate(windowSteps, windowStart, now, windowDur, "", "")
		newResults = append(newResults, result)

		// Compute per-pattern aggregates
		patterns := a.uniquePatterns(windowSteps)
		for _, pattern := range patterns {
			if pattern == "" {
				continue
			}
			patternSteps := a.filterByPattern(windowSteps, pattern)
			if len(patternSteps) > 0 {
				pr := a.computeAggregate(patternSteps, windowStart, now, windowDur, pattern, "")
				newResults = append(newResults, pr)
			}
		}

		// Compute per-unit aggregates
		units := a.uniqueUnits(windowSteps)
		for _, unit := range units {
			unitSteps := a.filterByUnit(windowSteps, unit)
			if len(unitSteps) > 0 {
				ur := a.computeAggregate(unitSteps, windowStart, now, windowDur, "", unit)
				newResults = append(newResults, ur)
			}
		}
	}

	// Append and trim
	a.results = append(a.results, newResults...)
	if len(a.results) > a.config.MaxWindows {
		a.results = a.results[len(a.results)-a.config.MaxWindows:]
	}

	return newResults
}

// computeAggregate computes a single AggregateResult from a slice of steps.
func (a *Aggregator) computeAggregate(steps []ExecutionStep, start, end time.Time, dur time.Duration, pattern, unit string) AggregateResult {
	count := len(steps)
	errorCount := 0
	durations := make([]int64, 0, len(steps))
	var totalDuration float64
	var minDur, maxDur int64 = int64(^uint64(0) >> 1), 0

	for _, step := range steps {
		if step.Error != nil {
			errorCount++
		}
		d := step.DurationMs
		durations = append(durations, d)
		totalDuration += float64(d)
		if d < minDur {
			minDur = d
		}
		if d > maxDur {
			maxDur = d
		}
	}

	if len(durations) == 0 {
		minDur = 0
	}

	sort.Slice(durations, func(i, j int) bool {
		return durations[i] < durations[j]
	})

	p50 := percentile(durations, 0.50)
	p99 := percentile(durations, 0.99)
	avg := totalDuration / float64(count)

	return AggregateResult{
		WindowStart:    start,
		WindowEnd:      end,
		WindowDuration: dur.String(),
		Pattern:        pattern,
		Unit:           unit,
		Count:          count,
		ErrorCount:     errorCount,
		AvgDurationMs:  avg,
		P50DurationMs:  p50,
		P99DurationMs:  p99,
		MinDurationMs:  minDur,
		MaxDurationMs:  maxDur,
	}
}

// percentile computes the given percentile from sorted data.
func percentile(sorted []int64, p float64) int64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(float64(len(sorted)-1) * p)
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	if idx < 0 {
		idx = 0
	}
	return sorted[idx]
}

// uniquePatterns returns unique non-empty patterns from steps.
func (a *Aggregator) uniquePatterns(steps []ExecutionStep) []string {
	seen := make(map[string]bool)
	result := make([]string, 0)
	for _, step := range steps {
		if step.Pattern != "" && !seen[step.Pattern] {
			seen[step.Pattern] = true
			result = append(result, step.Pattern)
		}
	}
	return result
}

// uniqueUnits returns unique unit types from steps.
func (a *Aggregator) uniqueUnits(steps []ExecutionStep) []string {
	seen := make(map[string]bool)
	result := make([]string, 0)
	for _, step := range steps {
		if step.Unit != "" && !seen[step.Unit] {
			seen[step.Unit] = true
			result = append(result, step.Unit)
		}
	}
	return result
}

// filterByPattern returns steps matching the given pattern.
func (a *Aggregator) filterByPattern(steps []ExecutionStep, pattern string) []ExecutionStep {
	result := make([]ExecutionStep, 0)
	for _, step := range steps {
		if step.Pattern == pattern {
			result = append(result, step)
		}
	}
	return result
}

// filterByUnit returns steps matching the given unit type.
func (a *Aggregator) filterByUnit(steps []ExecutionStep, unit string) []ExecutionStep {
	result := make([]ExecutionStep, 0)
	for _, step := range steps {
		if step.Unit == unit {
			result = append(result, step)
		}
	}
	return result
}

// GetResults returns all stored aggregate results.
func (a *Aggregator) GetResults() []AggregateResult {
	a.mu.RLock()
	defer a.mu.RUnlock()
	result := make([]AggregateResult, len(a.results))
	copy(result, a.results)
	return result
}

// QueryResults returns results matching the given filters.
func (a *Aggregator) QueryResults(windowDur string, unit, pattern string) []AggregateResult {
	a.mu.RLock()
	defer a.mu.RUnlock()

	result := make([]AggregateResult, 0)
	for _, r := range a.results {
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

// =============================================================================
// IncrementalAggregator — 增量聚合器 (T3.4)
// =============================================================================

// IncrementalAggregator 在 Record 时立即更新 t-digest，避免批量聚合时的全排序。
// 每个窗口维护独立的 TDigest，查询时 O(1) 返回分位数。
//
// 与 Aggregator 的区别：
//   - Aggregator: 批量 Aggregate() 时全排序，O(n log n)
//   - IncrementalAggregator: Record() 时 O(1) 更新 TDigest，GetResults() O(1)
type IncrementalAggregator struct {
	mu       sync.RWMutex
	config   AggregatorConfig
	labeler  *AnomalyAutoLabeler
	driftDet *DistributionDriftDetector
	baseline *TDigest

	// 每个窗口维护独立的 TDigest
	windows     map[string]*incrementalWindow // key = duration_pattern_unit
	windowList  []*incrementalWindow
}

// incrementalWindow 是增量聚合的单个窗口。
type incrementalWindow struct {
	start      time.Time
	end        time.Time
	duration   time.Duration
	pattern    string
	unit       string
	digest     *TDigest
	count      int
	errorCount int
	totalDur   float64
	minDur     int64
	maxDur     int64
}

// windowKey 构建窗口的唯一键。
func windowKey(dur time.Duration, pattern, unit string) string {
	return dur.String() + "_" + pattern + "_" + unit
}

// NewIncrementalAggregator 创建增量聚合器。
func NewIncrementalAggregator(config AggregatorConfig) *IncrementalAggregator {
	if config.MaxWindows <= 0 {
		config.MaxWindows = 1000
	}
	return &IncrementalAggregator{
		config:    config,
		labeler:   NewAnomalyAutoLabeler(),
		driftDet:  NewDistributionDriftDetector(0.1),
		windows:   make(map[string]*incrementalWindow),
	}
}

// Record 记录一个步骤到所有窗口的 TDigest 中。
// 每个步骤到达时立即更新对应窗口的 t-digest。
func (ia *IncrementalAggregator) Record(steps []ExecutionStep) {
	ia.mu.Lock()
	defer ia.mu.Unlock()

	now := time.Now()

	for _, step := range steps {
		for _, windowDur := range ia.config.WindowDurations {
			windowStart := now.Add(-windowDur)

			// 如果步骤不在窗口内，跳过
			if step.Timestamp.Before(windowStart) {
				continue
			}

			// 更新总体聚合窗口
			ia.updateWindow(step, windowStart, now, windowDur, "", "")

			// 更新按 Pattern 聚合的窗口
			if step.Pattern != "" {
				ia.updateWindow(step, windowStart, now, windowDur, step.Pattern, "")
			}

			// 更新按 Unit 聚合的窗口
			if step.Unit != "" {
				ia.updateWindow(step, windowStart, now, windowDur, "", step.Unit)
			}
		}
	}

	// 清理过期窗口
	ia.cleanupOldWindows()
}

// updateWindow 更新单个窗口的聚合数据。
func (ia *IncrementalAggregator) updateWindow(step ExecutionStep, start, end time.Time, dur time.Duration, pattern, unit string) {
	key := windowKey(dur, pattern, unit)
	w, ok := ia.windows[key]
	if !ok {
		w = &incrementalWindow{
			start:    start,
			end:      end,
			duration: dur,
			pattern:  pattern,
			unit:     unit,
			digest:   NewTDigestDefault(),
			minDur:   int64(^uint64(0) >> 1),
		}
		ia.windows[key] = w
		ia.windowList = append(ia.windowList, w)
	}

	w.count++
	w.digest.Add(float64(step.DurationMs))
	w.totalDur += float64(step.DurationMs)

	if step.Error != nil {
		w.errorCount++
	}
	if step.DurationMs < w.minDur {
		w.minDur = step.DurationMs
	}
	if step.DurationMs > w.maxDur {
		w.maxDur = step.DurationMs
	}
}

// cleanupOldWindows 清理超过 MaxWindows 的旧窗口。
func (ia *IncrementalAggregator) cleanupOldWindows() {
	if len(ia.windowList) <= ia.config.MaxWindows {
		return
	}

	excess := len(ia.windowList) - ia.config.MaxWindows
	for i := 0; i < excess; i++ {
		w := ia.windowList[i]
		key := windowKey(w.duration, w.pattern, w.unit)
		delete(ia.windows, key)
	}
	ia.windowList = ia.windowList[excess:]
}

// GetResults 返回所有窗口的聚合结果。
// 由于使用 TDigest，分位数查询 O(1)。
func (ia *IncrementalAggregator) GetResults() []AggregateResult {
	ia.mu.RLock()
	defer ia.mu.RUnlock()

	results := make([]AggregateResult, 0, len(ia.windowList))
	for _, w := range ia.windowList {
		r := ia.buildResult(w)
		results = append(results, r)
	}
	return results
}

// buildResult 从增量窗口构建 AggregateResult。
func (ia *IncrementalAggregator) buildResult(w *incrementalWindow) AggregateResult {
	minDur := w.minDur
	if w.count == 0 {
		minDur = 0
	}

	r := AggregateResult{
		WindowStart:    w.start,
		WindowEnd:      w.end,
		WindowDuration: w.duration.String(),
		Pattern:        w.pattern,
		Unit:           w.unit,
		Count:          w.count,
		ErrorCount:     w.errorCount,
		AvgDurationMs:  w.totalDur / float64(max(w.count, 1)),
		P50DurationMs:  int64(w.digest.Quantile(0.50)),
		P99DurationMs:  int64(w.digest.Quantile(0.99)),
		MinDurationMs:  minDur,
		MaxDurationMs:  w.maxDur,
	}

	// 分布漂移检测
	if w.pattern == "" && w.unit == "" {
		driftResult := ia.driftDet.CompareDistributions(w.digest)
		r.DistributionDrifted = driftResult.Drifted
		r.DriftDirection = driftResult.Direction
	}

	// 异常标注
	errorRate := float64(w.errorCount) / float64(max(w.count, 1))
	ia.labeler.UpdateBaseline(float64(int64(w.digest.Quantile(0.99))), errorRate)
	r.AnomalyLabel, r.AnomalyScore = ia.labeler.Label(
		float64(int64(w.digest.Quantile(0.99))),
		errorRate,
		r.DistributionDrifted,
		r.AnomalyScore,
	)

	return r
}

// SetBaseline 设置分布漂移检测的基线。
func (ia *IncrementalAggregator) SetBaseline(baseline *TDigest) {
	ia.driftDet.SetBaseline(baseline)
}

// QueryResults 按条件过滤聚合结果。
func (ia *IncrementalAggregator) QueryResults(windowDur string, unit, pattern string) []AggregateResult {
	ia.mu.RLock()
	defer ia.mu.RUnlock()

	result := make([]AggregateResult, 0)
	for _, w := range ia.windowList {
		if windowDur != "" && w.duration.String() != windowDur {
			continue
		}
		if unit != "" && w.unit != unit {
			continue
		}
		if pattern != "" && w.pattern != pattern {
			continue
		}
		result = append(result, ia.buildResult(w))
	}
	return result
}

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