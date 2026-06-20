//go:build lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

// SECTION 5: AdaptiveThresholdEngine — 自适应阈值引擎
package core

import (
	"math"
	"sort"
	"sync"
	"sync/atomic"
)

// ============================================================================
// SECTION 5: AdaptiveThresholdEngine — 自适应阈值引擎
// ============================================================================

// AdaptiveThresholdConfig 配置自适应阈值的行为。
type AdaptiveThresholdConfig struct {
	WindowSize            int
	UpdateInterval        int
	Multiplier            float64
	MinThreshold          float64
	EnableManualOverride  bool
}

func DefaultAdaptiveThresholdConfig() AdaptiveThresholdConfig {
	return AdaptiveThresholdConfig{
		WindowSize:     1000,
		UpdateInterval: 100,
		Multiplier:     1.5,
		MinThreshold:   5.0,
	}
}

// AdaptiveThresholdEngine 基于历史基线自动调整阈值。
type AdaptiveThresholdEngine struct {
	config          AdaptiveThresholdConfig
	mu              sync.RWMutex
	samples         []float64
	writeIdx        int
	count           int
	thresholdYellow atomic.Uint64
	thresholdOrange atomic.Uint64
	thresholdRed    atomic.Uint64
	manualOverride  bool
	manualYellow    float64
	manualOrange    float64
	manualRed       float64
}

func NewAdaptiveThresholdEngine(config AdaptiveThresholdConfig) *AdaptiveThresholdEngine {
	if config.WindowSize <= 0 {
		config.WindowSize = 1000
	}
	if config.UpdateInterval <= 0 {
		config.UpdateInterval = 100
	}
	if config.Multiplier <= 0 {
		config.Multiplier = 1.5
	}
	e := &AdaptiveThresholdEngine{
		config:  config,
		samples: make([]float64, config.WindowSize),
	}
	e.thresholdYellow.Store(math.Float64bits(20))
	e.thresholdOrange.Store(math.Float64bits(50))
	e.thresholdRed.Store(math.Float64bits(100))
	return e
}

func (e *AdaptiveThresholdEngine) RecordSample(score float64) {
	e.mu.Lock()
	if e.count < e.config.WindowSize {
		e.samples[e.count] = score
		e.count++
	} else {
		e.samples[e.writeIdx] = score
		e.writeIdx = (e.writeIdx + 1) % e.config.WindowSize
	}
	totalCount := e.count
	if totalCount > e.config.WindowSize {
		totalCount = e.config.WindowSize
	}
	e.mu.Unlock()

	if totalCount >= e.config.UpdateInterval && totalCount%e.config.UpdateInterval == 0 {
		e.updateThresholds()
	}
}

func (e *AdaptiveThresholdEngine) updateThresholds() {
	if e.config.EnableManualOverride && e.manualOverride {
		return
	}

	e.mu.RLock()
	totalCount := e.count
	if totalCount > e.config.WindowSize {
		totalCount = e.config.WindowSize
	}
	if totalCount < 10 {
		e.mu.RUnlock()
		return
	}
	sorted := make([]float64, totalCount)
	if e.count <= e.config.WindowSize {
		copy(sorted, e.samples[:e.count])
	} else {
		idx := e.writeIdx
		for i := 0; i < totalCount; i++ {
			sorted[i] = e.samples[(idx+i)%e.config.WindowSize]
		}
	}
	e.mu.RUnlock()

	sort.Float64s(sorted)

	p95 := sorted[int(float64(totalCount)*0.95)]
	p99 := sorted[int(float64(totalCount)*0.99)]

	m := e.config.Multiplier
	yellow := math.Max(p95*m, e.config.MinThreshold)
	orange := math.Max(p95*m*2, yellow*1.5)
	red := math.Max(p99*m*3, orange*2)

	e.thresholdYellow.Store(math.Float64bits(yellow))
	e.thresholdOrange.Store(math.Float64bits(orange))
	e.thresholdRed.Store(math.Float64bits(red))
}

func (e *AdaptiveThresholdEngine) GetRecommendedThresholds() (yellow, orange, red float64) {
	return math.Float64frombits(e.thresholdYellow.Load()),
		math.Float64frombits(e.thresholdOrange.Load()),
		math.Float64frombits(e.thresholdRed.Load())
}

func (e *AdaptiveThresholdEngine) ManualOverride(yellow, orange, red float64) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.manualOverride = true
	e.manualYellow = yellow
	e.manualOrange = orange
	e.manualRed = red
	e.thresholdYellow.Store(math.Float64bits(yellow))
	e.thresholdOrange.Store(math.Float64bits(orange))
	e.thresholdRed.Store(math.Float64bits(red))
}

func (e *AdaptiveThresholdEngine) ResetBaseline() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.count = 0
	e.writeIdx = 0
	e.manualOverride = false
}
