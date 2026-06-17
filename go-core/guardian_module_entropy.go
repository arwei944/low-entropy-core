// Package core — 监督层细粒度化 (v4.0)
//
// 本文件提供模块/Pipeline/Agent三级细粒度熵值监控，包括：
//   - ModuleEntropyTracker：按模块独立追踪熵值
//   - PipelineStepGrowthDetector：Pipeline步数增长检测
//   - AgentBehaviorDriftDetector：Agent行为漂移追踪
//   - AdaptiveThresholdEngine：自适应阈值引擎
//   - MultiDimensionEntropySnapshot：多维度熵值快照
//
// 所有类型均为线程安全，使用 ShardedLock 和 atomic 操作。
package core

import (
	"math"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// =============================================================================
// T2.2: ModuleEntropyTracker — 模块级熵值追踪
// =============================================================================

// ModuleID 唯一标识一个模块。
type ModuleID string

// ModuleEntropyState 是单个模块的熵值状态。
type ModuleEntropyState struct {
	ModuleID       ModuleID
	EntropyScore   float64
	PreviousScore  float64
	HasPrevious    bool
	LastUpdated    time.Time
	EntropyLevel   EntropyLevel
}

// ModuleEntropyTracker 按模块独立追踪熵值。
// 使用 ShardedLock[ModuleID] 分片，不同模块的更新完全并行。
type ModuleEntropyTracker struct {
	shards       [shardCount]*moduleEntropyShard
	moduleCount  atomic.Int64
}

// moduleEntropyShard 是单个分片的模块熵值存储。
type moduleEntropyShard struct {
	mu      sync.RWMutex
	modules map[ModuleID]*ModuleEntropyState
}

// NewModuleEntropyTracker 创建模块级熵值追踪器。
func NewModuleEntropyTracker() *ModuleEntropyTracker {
	t := &ModuleEntropyTracker{}
	for i := 0; i < shardCount; i++ {
		t.shards[i] = &moduleEntropyShard{
			modules: make(map[ModuleID]*ModuleEntropyState),
		}
	}
	return t
}

// RegisterModule 注册一个模块，初始化熵值状态。
// 每个模块注册后即可独立追踪熵值。
func (t *ModuleEntropyTracker) RegisterModule(moduleID ModuleID) {
	idx := hashString(string(moduleID)) & 0xFF
	shard := t.shards[idx]
	shard.mu.Lock()
	if _, exists := shard.modules[moduleID]; !exists {
		shard.modules[moduleID] = &ModuleEntropyState{
			ModuleID:  moduleID,
			EntropyLevel: EntropyOK,
		}
		t.moduleCount.Add(1)
	}
	shard.mu.Unlock()
}

// UnregisterModule 注销模块，释放资源。
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

// UpdateEntropy 更新模块的熵值并返回当前状态。
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

	// 根据阈值计算级别
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

// GetModuleEntropy 获取模块的当前熵值状态。
func (t *ModuleEntropyTracker) GetModuleEntropy(moduleID ModuleID) *ModuleEntropyState {
	idx := hashString(string(moduleID)) & 0xFF
	shard := t.shards[idx]
	shard.mu.RLock()
	defer shard.mu.RUnlock()
	return shard.modules[moduleID]
}

// GetAllModules 返回所有模块的熵值状态列表。
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

// ModuleCount 返回注册的模块数量。
func (t *ModuleEntropyTracker) ModuleCount() int {
	return int(t.moduleCount.Load())
}

// =============================================================================
// T2.3: PipelineStepGrowthDetector — 步数增长检测
// =============================================================================

// PipelineID 唯一标识一个 Pipeline。
type PipelineID string

// PipelineGrowthState 是单个 Pipeline 的步数增长状态。
type PipelineGrowthState struct {
	PipelineID     PipelineID
	CurrentSteps   int
	BaselineMean   float64 // EMA 均值
	BaselineStd    float64 // EMA 标准差
	Alpha          float64 // EMA 平滑因子
	LastUpdateTime time.Time
	HistoryCount   int
}

// PipelineStepGrowthDetector 检测每个 Pipeline 的步数增长趋势。
// 使用指数移动平均（EMA）建立基线，超过基线 2σ 触发告警。
type PipelineStepGrowthDetector struct {
	shards [shardCount]*growthShard
}

// growthShard 是单个分片的 Pipeline 增长状态存储。
type growthShard struct {
	mu        sync.RWMutex
	pipelines map[PipelineID]*PipelineGrowthState
}

// NewPipelineStepGrowthDetector 创建步数增长检测器。
// alpha 是 EMA 平滑因子（默认 0.1）。
func NewPipelineStepGrowthDetector() *PipelineStepGrowthDetector {
	d := &PipelineStepGrowthDetector{}
	for i := 0; i < shardCount; i++ {
		d.shards[i] = &growthShard{
			pipelines: make(map[PipelineID]*PipelineGrowthState),
		}
	}
	return d
}

// RegisterPipeline 注册一个 Pipeline 用于步数追踪。
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

// RecordStepCount 记录 Pipeline 的当前步数。
// 更新 EMA 基线和标准差，返回是否触发增长告警。
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

	// EMA 更新
	if state.HistoryCount == 1 {
		state.BaselineMean = step
		state.BaselineStd = 0
	} else {
		// EMA 均值
		state.BaselineMean = state.Alpha*step + (1-state.Alpha)*state.BaselineMean
		// EMA 标准差（使用平方差的 EMA）
		diff := step - state.BaselineMean
		state.BaselineStd = math.Sqrt(state.Alpha*diff*diff + (1-state.Alpha)*state.BaselineStd*state.BaselineStd)
	}

	// 检测增长：当前步数超过基线均值 + 2σ
	if count >= 10 && state.BaselineStd > 0 {
		upperLimit := state.BaselineMean + 2*state.BaselineStd
		if step > upperLimit {
			return true, state
		}
	}

	return false, state
}

// GetGrowthState 获取 Pipeline 的步数增长状态。
func (d *PipelineStepGrowthDetector) GetGrowthState(pipelineID PipelineID) *PipelineGrowthState {
	idx := hashString(string(pipelineID)) & 0xFF
	shard := d.shards[idx]
	shard.mu.RLock()
	defer shard.mu.RUnlock()
	return shard.pipelines[pipelineID]
}

// GetGrowthRate 计算 Pipeline 的步数增长率。
// 增长率 = (当前步数 - 基线均值) / 基线均值
func (state *PipelineGrowthState) GetGrowthRate() float64 {
	if state.BaselineMean == 0 || state.HistoryCount < 5 {
		return 0
	}
	return (float64(state.CurrentSteps) - state.BaselineMean) / state.BaselineMean
}

// =============================================================================
// T2.4: AgentBehaviorDriftDetector — Agent行为漂移追踪
// =============================================================================

// AgentID 唯一标识一个 Agent。
type AgentID string

// DriftTrend 表示漂移趋势方向。
type DriftTrend string

const (
	DriftTrendStable   DriftTrend = "stable"
	DriftTrendRising   DriftTrend = "rising"
	DriftTrendDeclining DriftTrend = "declining"
)

// AgentDriftHistory 是单个 Agent 的漂移历史。
type AgentDriftHistory struct {
	AgentID         AgentID
	RecentScores    []float64 // 最近 100 次漂移分数
	CurrentTrend     DriftTrend
	ConsecutiveRising int     // 连续上升次数
	LastUpdated      time.Time
}

// AgentBehaviorDriftDetector 按 AgentID 追踪行为漂移。
// 维护每个 Agent 的漂移分数历史，支持趋势分析和早期预警。
type AgentBehaviorDriftDetector struct {
	shards [shardCount]*driftShard
}

// driftShard 是单个分片的 Agent 漂移存储。
type driftShard struct {
	mu     sync.RWMutex
	agents map[AgentID]*AgentDriftHistory
}

// NewAgentBehaviorDriftDetector 创建 Agent 行为漂移检测器。
func NewAgentBehaviorDriftDetector() *AgentBehaviorDriftDetector {
	d := &AgentBehaviorDriftDetector{}
	for i := 0; i < shardCount; i++ {
		d.shards[i] = &driftShard{
			agents: make(map[AgentID]*AgentDriftHistory),
		}
	}
	return d
}

// RecordDriftScore 记录 Agent 的漂移分数。
// 自动更新趋势分析和早期预警状态。
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

	// 维护最近 100 次漂移分数
	if len(history.RecentScores) >= driftHistorySize {
		history.RecentScores = append(history.RecentScores[1:], score)
	} else {
		history.RecentScores = append(history.RecentScores, score)
	}

	history.LastUpdated = time.Now()

	// 趋势分析：线性回归
	if len(history.RecentScores) >= 5 {
		history.CurrentTrend = d.computeTrend(history.RecentScores)
	}

	// 早期预警：连续 3 次上升趋势
	if history.CurrentTrend == DriftTrendRising {
		history.ConsecutiveRising++
	} else {
		history.ConsecutiveRising = 0
	}

	return history
}

// driftHistorySize 是漂移历史的最大记录数。
const driftHistorySize = 100

// computeTrend 使用线性回归判断趋势方向。
func (d *AgentBehaviorDriftDetector) computeTrend(scores []float64) DriftTrend {
	n := float64(len(scores))
	if n < 3 {
		return DriftTrendStable
	}

	// 简单线性回归（x 为索引，y 为分数）
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

	// 斜率阈值：> 0.001 为上升，< -0.001 为下降
	if slope > 0.001 {
		return DriftTrendRising
	} else if slope < -0.001 {
		return DriftTrendDeclining
	}
	return DriftTrendStable
}

// GetAgentDriftHistory 获取 Agent 的漂移历史。
func (d *AgentBehaviorDriftDetector) GetAgentDriftHistory(agentID AgentID) *AgentDriftHistory {
	idx := hashString(string(agentID)) & 0xFF
	shard := d.shards[idx]
	shard.mu.RLock()
	defer shard.mu.RUnlock()
	return shard.agents[agentID]
}

// ResetAgent 重置 Agent 的漂移历史。
func (d *AgentBehaviorDriftDetector) ResetAgent(agentID AgentID) {
	idx := hashString(string(agentID)) & 0xFF
	shard := d.shards[idx]
	shard.mu.Lock()
	defer shard.mu.Unlock()
	delete(shard.agents, agentID)
}

// ShouldEarlyWarn 检查 Agent 是否应触发早期预警。
// 连续 3 次上升趋势或当前分数 > 0.7 时触发。
func (h *AgentDriftHistory) ShouldEarlyWarn() bool {
	if len(h.RecentScores) == 0 {
		return false
	}
	latest := h.RecentScores[len(h.RecentScores)-1]
	return h.ConsecutiveRising >= 3 || latest > 0.7
}

// =============================================================================
// T2.5: AdaptiveThresholdEngine — 自适应阈值引擎
// =============================================================================

// AdaptiveThresholdConfig 配置自适应阈值的行为。
type AdaptiveThresholdConfig struct {
	WindowSize        int     // 滑动窗口大小
	UpdateInterval    int     // 每 N 个样本更新一次阈值
	Multiplier        float64 // 阈值倍率（默认 1.5）
	MinThreshold      float64 // 最小阈值下限
	EnableManualOverride bool
}

// DefaultAdaptiveThresholdConfig 返回默认配置。
func DefaultAdaptiveThresholdConfig() AdaptiveThresholdConfig {
	return AdaptiveThresholdConfig{
		WindowSize:     1000,
		UpdateInterval: 100,
		Multiplier:     1.5,
		MinThreshold:   5.0,
	}
}

// AdaptiveThresholdEngine 基于历史基线自动调整阈值。
// 使用滑动窗口计算 P50/P95/P99，阈值 = P95 × 倍率。
type AdaptiveThresholdEngine struct {
	config    AdaptiveThresholdConfig
	mu        sync.RWMutex
	samples   []float64 // 滑动窗口样本
	writeIdx  int
	count     int
	// 当前阈值
	thresholdYellow atomic.Uint64 // math.Float64bits
	thresholdOrange atomic.Uint64
	thresholdRed    atomic.Uint64
	// 手动覆盖
	manualOverride bool
	manualYellow   float64
	manualOrange   float64
	manualRed      float64
}

// NewAdaptiveThresholdEngine 创建自适应阈值引擎。
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
	// 初始阈值设为默认值
	e.thresholdYellow.Store(math.Float64bits(20))
	e.thresholdOrange.Store(math.Float64bits(50))
	e.thresholdRed.Store(math.Float64bits(100))
	return e
}

// RecordSample 记录一个熵值样本，满足更新间隔时自动更新阈值。
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

	// 每 UpdateInterval 个样本更新一次阈值
	if totalCount >= e.config.UpdateInterval && totalCount%e.config.UpdateInterval == 0 {
		e.updateThresholds()
	}
}

// updateThresholds 基于当前滑动窗口计算新阈值。
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
	// 复制样本用于排序
	sorted := make([]float64, totalCount)
	if e.count <= e.config.WindowSize {
		copy(sorted, e.samples[:e.count])
	} else {
		// 环形缓冲区——收集有效样本
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

// GetRecommendedThresholds 返回当前推荐阈值。
func (e *AdaptiveThresholdEngine) GetRecommendedThresholds() (yellow, orange, red float64) {
	return math.Float64frombits(e.thresholdYellow.Load()),
		math.Float64frombits(e.thresholdOrange.Load()),
		math.Float64frombits(e.thresholdRed.Load())
}

// ManualOverride 手动覆盖阈值（禁用自适应）。
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

// ResetBaseline 重置历史基线，重新开始学习。
func (e *AdaptiveThresholdEngine) ResetBaseline() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.count = 0
	e.writeIdx = 0
	e.manualOverride = false
}

// =============================================================================
// T2.6: MultiDimensionEntropySnapshot — 多维度熵值快照
// =============================================================================

// MultiDimensionEntropySnapshot 是多维度熵值快照，统一提供给 Guardian 层。
// 包含 5 个维度：全局熵值、模块熵值分布、Pipeline 步数增长、Agent 漂移分数、系统熵值。
type MultiDimensionEntropySnapshot struct {
	// 全局维度
	GlobalEntropyScore float64
	GlobalEntropyLevel EntropyLevel

	// 模块维度
	ModuleCount       int
	ModuleEntropies   []*ModuleEntropyState
	MaxModuleEntropy  float64
	AvgModuleEntropy  float64

	// Pipeline 维度
	PipelineCount          int
	PipelineGrowthStates   []*PipelineGrowthState
	PipelinesWithGrowth    int // 检测到增长的 Pipeline 数

	// Agent 维度
	AgentCount            int
	AgentDriftHistories   []*AgentDriftHistory
	AgentsWithEarlyWarn    int // 触发早期预警的 Agent 数

	// 系统维度
	SystemEntropyScore float64
	Timestamp          time.Time
}

// MultiDimensionEntropyCollector 收集多维度熵值数据。
type MultiDimensionEntropyCollector struct {
	moduleTracker  *ModuleEntropyTracker
	growthDetector *PipelineStepGrowthDetector
	driftDetector  *AgentBehaviorDriftDetector
}

// NewMultiDimensionEntropyCollector 创建多维度熵值收集器。
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

// Collect 收集所有维度的熵值快照。
func (c *MultiDimensionEntropyCollector) Collect() *MultiDimensionEntropySnapshot {
	snapshot := &MultiDimensionEntropySnapshot{
		Timestamp: time.Now(),
	}

	// 模块维度
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

	// Pipeline 维度
	// 注：growthDetector 的 GetAllPipelines 需要额外实现，此处简化
	// 实际使用时通过遍历分片收集

	// Agent 维度
	// 同理，driftDetector 需要遍历分片

	return snapshot
}

// ToFlatSnapshot 将多维度快照扁平化为标准 EntropySnapshot（向后兼容）。
func (m *MultiDimensionEntropySnapshot) ToFlatSnapshot() EntropySnapshot {
	// 全局熵值 = 加权平均
	score := m.GlobalEntropyScore
	if score == 0 {
		// 从各维度计算综合分数
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

// DefaultDimensionEnabled 返回所有维度启用的配置。
func DefaultDimensionEnabled() DimensionEnabled {
	return DimensionEnabled{
		Global:   true,
		Module:   true,
		Pipeline: true,
		Agent:    true,
		System:   true,
	}
}