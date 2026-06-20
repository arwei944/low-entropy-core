//go:build lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

// SECTION 3: PipelineStepGrowthDetector — 步数增长检测
package core

import (
	"math"
	"sync"
	"time"
)

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
