//go:build lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

package core

import (
	"sync"
	"time"
)

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
