//go:build lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

// SECTION 6: MultiDimensionEntropySnapshot — 多维度熵值快照
package core

import (
	"time"
)

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
