//go:build lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

// Package core — 监督层熵值监控 (v4.0)
//
// 合并自: guardian_entropy.go + guardian_module_entropy.go
//
// 包含:
//   - EntropyWatcher: 全局熵值监控 Port
//   - ModuleEntropyTracker: 模块级熵值独立追踪
//   - PipelineStepGrowthDetector: Pipeline 步数增长检测
//   - AgentBehaviorDriftDetector: Agent 行为漂移追踪
//   - AdaptiveThresholdEngine: 自适应阈值引擎
//   - MultiDimensionEntropySnapshot: 多维度熵值快照
//   - MultiDimensionEntropyCollector: 多维度熵值收集器
//
// 所有类型均为线程安全，使用 ShardedLock、atomic 和分片锁。
package core

import (
	"context"
	"fmt"
	"math"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// ============================================================================
// SECTION 1: EntropyWatcher — 全局熵值监控 Port
// ============================================================================

// EntropyLevel represents the severity of entropy alert.
type EntropyLevel string

const (
	EntropyOK     EntropyLevel = "OK"
	EntropyYellow EntropyLevel = "Yellow"
	EntropyOrange EntropyLevel = "Orange"
	EntropyRed    EntropyLevel = "Red"
)

// EntropyAlert is the output of the entropy watcher.
type EntropyAlert struct {
	Level                EntropyLevel
	Score                float64
	PreviousScore        float64
	AccelerationDetected bool
	Thresholds           map[string]bool
	Message              string
	Timestamp            time.Time
}

// EntropyWatcher implements Port[EntropySnapshot, EntropyAlert].
// v4.0: 使用 atomic 操作替代 sync.Mutex，热路径无锁。
type EntropyWatcher struct {
	previousScore    atomic.Uint64
	hasPrevious      atomic.Bool
	thresholdYellow  float64
	thresholdOrange  float64
	thresholdRed     float64
}

func (w *EntropyWatcher) loadPreviousScore() (float64, bool) {
	if !w.hasPrevious.Load() {
		return 0, false
	}
	return math.Float64frombits(w.previousScore.Load()), true
}

func (w *EntropyWatcher) storePreviousScore(score float64) {
	w.previousScore.Store(math.Float64bits(score))
	w.hasPrevious.Store(true)
}

func NewEntropyWatcher() *EntropyWatcher {
	return &EntropyWatcher{
		thresholdYellow: 20,
		thresholdOrange: 50,
		thresholdRed:    100,
	}
}

func NewEntropyWatcherWithThresholds(yellow, orange, red float64) *EntropyWatcher {
	return &EntropyWatcher{
		thresholdYellow: yellow,
		thresholdOrange: orange,
		thresholdRed:    red,
	}
}

func (w *EntropyWatcher) Validate(ctx context.Context, input EntropySnapshot) (EntropyAlert, error) {
	if ctx.Err() != nil {
		return EntropyAlert{}, ctx.Err()
	}

	score := input.EntropyScore
	alert := EntropyAlert{
		Score:     score,
		Timestamp: input.Timestamp,
	}

	prevScore, hasPrev := w.loadPreviousScore()
	if hasPrev {
		alert.PreviousScore = prevScore
	}

	thresholds := make(map[string]bool)
	switch {
	case score >= w.thresholdRed:
		alert.Level = EntropyRed
		thresholds["red"] = true
		thresholds["orange"] = true
		thresholds["yellow"] = true
	case score >= w.thresholdOrange:
		alert.Level = EntropyOrange
		thresholds["orange"] = true
		thresholds["yellow"] = true
	case score >= w.thresholdYellow:
		alert.Level = EntropyYellow
		thresholds["yellow"] = true
	default:
		alert.Level = EntropyOK
	}
	alert.Thresholds = thresholds

	if hasPrev && score > prevScore {
		alert.AccelerationDetected = true
	}

	switch alert.Level {
	case EntropyRed:
		alert.Message = fmt.Sprintf(
			"CRITICAL: entropy score %.2f exceeds red threshold (%.2f). Immediate action required.",
			score, w.thresholdRed,
		)
	case EntropyOrange:
		alert.Message = fmt.Sprintf(
			"WARNING: entropy score %.2f exceeds orange threshold (%.2f). Investigation recommended.",
			score, w.thresholdOrange,
		)
	case EntropyYellow:
		alert.Message = fmt.Sprintf(
			"NOTICE: entropy score %.2f exceeds yellow threshold (%.2f). Monitor closely.",
			score, w.thresholdYellow,
		)
	default:
		alert.Message = fmt.Sprintf("Entropy score %.2f is within normal range.", score)
	}

	if alert.AccelerationDetected {
		alert.Message += fmt.Sprintf(
			" Acceleration detected: score increased from %.2f to %.2f.",
			prevScore, score,
		)
	}

	w.storePreviousScore(score)
	return alert, nil
}

var _ Port[EntropySnapshot, EntropyAlert] = (*EntropyWatcher)(nil)

// ============================================================================
// SECTION 2: ModuleEntropyTracker — 模块级熵值追踪
// ============================================================================

// ModuleID 唯一标识一个模块。
type ModuleID string

// ModuleEntropyState 是单个模块的熵值状态。
type ModuleEntropyState struct {
	ModuleID      ModuleID
	EntropyScore  float64
	PreviousScore float64
	HasPrevious   bool
	LastUpdated   time.Time
	EntropyLevel  EntropyLevel
}

// ModuleEntropyTracker 按模块独立追踪熵值。
// 使用 ShardedLock[ModuleID] 分片，不同模块的更新完全并行。
type ModuleEntropyTracker struct {
	shards      [shardCount]*moduleEntropyShard
	moduleCount atomic.Int64
}

type moduleEntropyShard struct {
	mu      sync.RWMutex
	modules map[ModuleID]*ModuleEntropyState
}

func NewModuleEntropyTracker() *ModuleEntropyTracker {
	t := &ModuleEntropyTracker{}
	for i := 0; i < shardCount; i++ {
		t.shards[i] = &moduleEntropyShard{
			modules: make(map[ModuleID]*ModuleEntropyState),
		}
	}
	return t
}

func (t *ModuleEntropyTracker) RegisterModule(moduleID ModuleID) {
	idx := hashString(string(moduleID)) & 0xFF
	shard := t.shards[idx]
	shard.mu.Lock()
	if _, exists := shard.modules[moduleID]; !exists {
		shard.modules[moduleID] = &ModuleEntropyState{
			ModuleID:     moduleID,
			EntropyLevel: EntropyOK,
		}
		t.moduleCount.Add(1)
	}
	shard.mu.Unlock()
}

func (t *ModuleEntropyTracker) UnregisterModule(moduleID ModuleID) {
	idx := hashString(string(moduleID)) & 0xFF
	shard := t.shards[idx]
	shard.mu.Lock()
	if _, exists := shard.modules[moduleID]; exists {
		delete(shard.modules, moduleID)
		t.moduleCount.Add(-1)
	}
	shard.mu.Unlock()
}

func (t *ModuleEntropyTracker) UpdateEntropy(moduleID ModuleID, score float64) *ModuleEntropyState {
	idx := hashString(string(moduleID)) & 0xFF
	shard := t.shards[idx]
	shard.mu.Lock()
	defer shard.mu.Unlock()

	state, ok := shard.modules[moduleID]
	if !ok {
		return nil
	}

	state.PreviousScore = state.EntropyScore
	state.HasPrevious = true
	state.EntropyScore = score
	state.LastUpdated = time.Now()

	switch {
	case score >= 100:
		state.EntropyLevel = EntropyRed
	case score >= 50:
		state.EntropyLevel = EntropyOrange
	case score >= 20:
		state.EntropyLevel = EntropyYellow
	default:
		state.EntropyLevel = EntropyOK
	}

	return state
}

func (t *ModuleEntropyTracker) GetModuleEntropy(moduleID ModuleID) *ModuleEntropyState {
	idx := hashString(string(moduleID)) & 0xFF
	shard := t.shards[idx]
	shard.mu.RLock()
	defer shard.mu.RUnlock()
	return shard.modules[moduleID]
}

func (t *ModuleEntropyTracker) GetAllModules() []*ModuleEntropyState {
	var results []*ModuleEntropyState
	for i := 0; i < shardCount; i++ {
		shard := t.shards[i]
		shard.mu.RLock()
		for _, state := range shard.modules {
			results = append(results, state)
		}
		shard.mu.RUnlock()
	}
	return results
}

func (t *ModuleEntropyTracker) ModuleCount() int {
	return int(t.moduleCount.Load())
}

// ============================================================================
// SECTION 3: PipelineStepGrowthDetector — 步数增长检测
// ============================================================================

// PipelineID 唯一标识一个 Pipeline。
type PipelineID string

// PipelineGrowthState 是单个 Pipeline 的步数增长状态。
type PipelineGrowthState struct {
	PipelineID     PipelineID
	CurrentSteps   int
	BaselineMean   float64
	BaselineStd    float64
	Alpha          float64
	LastUpdateTime time.Time
	HistoryCount   int
}

// PipelineStepGrowthDetector 检测每个 Pipeline 的步数增长趋势。
// 使用指数移动平均（EMA）建立基线，超过基线 2σ 触发告警。
type PipelineStepGrowthDetector struct {
	shards [shardCount]*growthShard
}

type growthShard struct {
	mu        sync.RWMutex
	pipelines map[PipelineID]*PipelineGrowthState
}

func NewPipelineStepGrowthDetector() *PipelineStepGrowthDetector {
	d := &PipelineStepGrowthDetector{}
	for i := 0; i < shardCount; i++ {
		d.shards[i] = &growthShard{
			pipelines: make(map[PipelineID]*PipelineGrowthState),
		}
	}
	return d
}

func (d *PipelineStepGrowthDetector) RegisterPipeline(pipelineID PipelineID, alpha float64) {
	if alpha <= 0 {
		alpha = 0.1
	}
	idx := hashString(string(pipelineID)) & 0xFF
	shard := d.shards[idx]
	shard.mu.Lock()
	if _, exists := shard.pipelines[pipelineID]; !exists {
		shard.pipelines[pipelineID] = &PipelineGrowthState{
			PipelineID: pipelineID,
			Alpha:      alpha,
		}
	}
	shard.mu.Unlock()
}

func (d *PipelineStepGrowthDetector) RecordStepCount(pipelineID PipelineID, stepCount int) (bool, *PipelineGrowthState) {
	idx := hashString(string(pipelineID)) & 0xFF
	shard := d.shards[idx]
	shard.mu.Lock()
	defer shard.mu.Unlock()

	state, ok := shard.pipelines[pipelineID]
	if !ok {
		state = &PipelineGrowthState{
			PipelineID: pipelineID,
			Alpha:      0.1,
		}
		shard.pipelines[pipelineID] = state
	}

	state.CurrentSteps = stepCount
	state.LastUpdateTime = time.Now()
	state.HistoryCount++

	count := float64(state.HistoryCount)
	step := float64(stepCount)

	if state.HistoryCount == 1 {
		state.BaselineMean = step
		state.BaselineStd = 0
	} else {
		state.BaselineMean = state.Alpha*step + (1-state.Alpha)*state.BaselineMean
		diff := step - state.BaselineMean
		state.BaselineStd = math.Sqrt(state.Alpha*diff*diff + (1-state.Alpha)*state.BaselineStd*state.BaselineStd)
	}

	if count >= 10 && state.BaselineStd > 0 {
		upperLimit := state.BaselineMean + 2*state.BaselineStd
		if step > upperLimit {
			return true, state
		}
	}

	return false, state
}

func (d *PipelineStepGrowthDetector) GetGrowthState(pipelineID PipelineID) *PipelineGrowthState {
	idx := hashString(string(pipelineID)) & 0xFF
	shard := d.shards[idx]
	shard.mu.RLock()
	defer shard.mu.RUnlock()
	return shard.pipelines[pipelineID]
}

func (state *PipelineGrowthState) GetGrowthRate() float64 {
	if state.BaselineMean == 0 || state.HistoryCount < 5 {
		return 0
	}
	return (float64(state.CurrentSteps) - state.BaselineMean) / state.BaselineMean
}

// ============================================================================
// SECTION 4: AgentBehaviorDriftDetector — Agent 行为漂移追踪
// ============================================================================

// AgentID 唯一标识一个 Agent。
type AgentID string

// DriftTrend 表示漂移趋势方向。
type DriftTrend string

const (
	DriftTrendStable    DriftTrend = "stable"
	DriftTrendRising    DriftTrend = "rising"
	DriftTrendDeclining DriftTrend = "declining"
)

// AgentDriftHistory 是单个 Agent 的漂移历史。
type AgentDriftHistory struct {
	AgentID           AgentID
	RecentScores      []float64
	CurrentTrend      DriftTrend
	ConsecutiveRising int
	LastUpdated       time.Time
}

// AgentBehaviorDriftDetector 按 AgentID 追踪行为漂移。
type AgentBehaviorDriftDetector struct {
	shards [shardCount]*driftShard
}

type driftShard struct {
	mu     sync.RWMutex
	agents map[AgentID]*AgentDriftHistory
}

const driftHistorySize = 100

func NewAgentBehaviorDriftDetector() *AgentBehaviorDriftDetector {
	d := &AgentBehaviorDriftDetector{}
	for i := 0; i < shardCount; i++ {
		d.shards[i] = &driftShard{
			agents: make(map[AgentID]*AgentDriftHistory),
		}
	}
	return d
}

func (d *AgentBehaviorDriftDetector) RecordDriftScore(agentID AgentID, score float64) *AgentDriftHistory {
	idx := hashString(string(agentID)) & 0xFF
	shard := d.shards[idx]
	shard.mu.Lock()
	defer shard.mu.Unlock()

	history, ok := shard.agents[agentID]
	if !ok {
		history = &AgentDriftHistory{
			AgentID:      agentID,
			RecentScores: make([]float64, 0, driftHistorySize),
			CurrentTrend: DriftTrendStable,
		}
		shard.agents[agentID] = history
	}

	if len(history.RecentScores) >= driftHistorySize {
		history.RecentScores = append(history.RecentScores[1:], score)
	} else {
		history.RecentScores = append(history.RecentScores, score)
	}

	history.LastUpdated = time.Now()

	if len(history.RecentScores) >= 5 {
		history.CurrentTrend = d.computeTrend(history.RecentScores)
	}

	if history.CurrentTrend == DriftTrendRising {
		history.ConsecutiveRising++
	} else {
		history.ConsecutiveRising = 0
	}

	return history
}

func (d *AgentBehaviorDriftDetector) computeTrend(scores []float64) DriftTrend {
	n := float64(len(scores))
	if n < 3 {
		return DriftTrendStable
	}

	var sumX, sumY, sumXY, sumX2 float64
	for i, y := range scores {
		x := float64(i)
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
	}

	denominator := n*sumX2 - sumX*sumX
	if denominator == 0 {
		return DriftTrendStable
	}
	slope := (n*sumXY - sumX*sumY) / denominator

	if slope > 0.001 {
		return DriftTrendRising
	} else if slope < -0.001 {
		return DriftTrendDeclining
	}
	return DriftTrendStable
}

func (d *AgentBehaviorDriftDetector) GetAgentDriftHistory(agentID AgentID) *AgentDriftHistory {
	idx := hashString(string(agentID)) & 0xFF
	shard := d.shards[idx]
	shard.mu.RLock()
	defer shard.mu.RUnlock()
	return shard.agents[agentID]
}

func (d *AgentBehaviorDriftDetector) ResetAgent(agentID AgentID) {
	idx := hashString(string(agentID)) & 0xFF
	shard := d.shards[idx]
	shard.mu.Lock()
	defer shard.mu.Unlock()
	delete(shard.agents, agentID)
}

func (h *AgentDriftHistory) ShouldEarlyWarn() bool {
	if len(h.RecentScores) == 0 {
		return false
	}
	latest := h.RecentScores[len(h.RecentScores)-1]
	return h.ConsecutiveRising >= 3 || latest > 0.7
}

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

// ============================================================================
// SECTION 6: MultiDimensionEntropySnapshot — 多维度熵值快照
// ============================================================================

// MultiDimensionEntropySnapshot 是多维度熵值快照，统一提供给 Guardian 层。
type MultiDimensionEntropySnapshot struct {
	GlobalEntropyScore float64
	GlobalEntropyLevel EntropyLevel
	ModuleCount        int
	ModuleEntropies    []*ModuleEntropyState
	MaxModuleEntropy   float64
	AvgModuleEntropy   float64
	PipelineCount      int
	PipelineGrowthStates []*PipelineGrowthState
	PipelinesWithGrowth int
	AgentCount           int
	AgentDriftHistories  []*AgentDriftHistory
	AgentsWithEarlyWarn  int
	SystemEntropyScore   float64
	Timestamp            time.Time
}

// MultiDimensionEntropyCollector 收集多维度熵值数据。
type MultiDimensionEntropyCollector struct {
	moduleTracker  *ModuleEntropyTracker
	growthDetector *PipelineStepGrowthDetector
	driftDetector  *AgentBehaviorDriftDetector
}

func NewMultiDimensionEntropyCollector(
	moduleTracker *ModuleEntropyTracker,
	growthDetector *PipelineStepGrowthDetector,
	driftDetector *AgentBehaviorDriftDetector,
) *MultiDimensionEntropyCollector {
	return &MultiDimensionEntropyCollector{
		moduleTracker:  moduleTracker,
		growthDetector: growthDetector,
		driftDetector:  driftDetector,
	}
}

func (c *MultiDimensionEntropyCollector) Collect() *MultiDimensionEntropySnapshot {
	snapshot := &MultiDimensionEntropySnapshot{
		Timestamp: time.Now(),
	}

	modules := c.moduleTracker.GetAllModules()
	snapshot.PipelineCount = len(modules)
	snapshot.ModuleEntropies = modules
	if len(modules) > 0 {
		var maxEntropy, sumEntropy float64
		for _, m := range modules {
			sumEntropy += m.EntropyScore
			if m.EntropyScore > maxEntropy {
				maxEntropy = m.EntropyScore
			}
		}
		snapshot.MaxModuleEntropy = maxEntropy
		snapshot.AvgModuleEntropy = sumEntropy / float64(len(modules))
	}

	return snapshot
}

func (m *MultiDimensionEntropySnapshot) ToFlatSnapshot() EntropySnapshot {
	score := m.GlobalEntropyScore
	if score == 0 {
		score = m.AvgModuleEntropy*0.3 +
			float64(m.PipelinesWithGrowth)*10*0.3 +
			float64(m.AgentsWithEarlyWarn)*20*0.2 +
			m.SystemEntropyScore*0.2
	}

	return EntropySnapshot{
		EntropyScore: score,
		Timestamp:    m.Timestamp,
	}
}

// DimensionEnabled 配置多维度快照中启用的维度。
type DimensionEnabled struct {
	Global   bool
	Module   bool
	Pipeline bool
	Agent    bool
	System   bool
}

func DefaultDimensionEnabled() DimensionEnabled {
	return DimensionEnabled{
		Global:   true,
		Module:   true,
		Pipeline: true,
		Agent:    true,
		System:   true,
	}
}