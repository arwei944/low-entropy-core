//go:build lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

package core

import (
	"sort"
	"sync"
	"time"
)

// Aggregator — time-window step aggregation

type AggregateResult struct {
	WindowStart      time.Time        `json:"window_start"`
	WindowEnd        time.Time        `json:"window_end"`
	WindowDuration   string           `json:"window_duration"`
	Pattern          string           `json:"pattern,omitempty"`
	Unit             string           `json:"unit,omitempty"`
	Count            int              `json:"count"`
	ErrorCount       int              `json:"error_count"`
	AvgDurationMs    float64          `json:"avg_duration_ms"`
	P50DurationMs    int64            `json:"p50_duration_ms"`
	P99DurationMs    int64            `json:"p99_duration_ms"`
	MinDurationMs    int64            `json:"min_duration_ms"`
	MaxDurationMs    int64            `json:"max_duration_ms"`
	DistributionDrifted bool            `json:"distribution_drifted,omitempty"`
	DriftDirection   DriftDirection  `json:"drift_direction,omitempty"`
	AnomalyLabel     AnomalyLabelType `json:"anomaly_label,omitempty"`
	AnomalyScore     float64          `json:"anomaly_score,omitempty"`
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
