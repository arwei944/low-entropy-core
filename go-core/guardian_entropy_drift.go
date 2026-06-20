//go:build lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

// SECTION 4: AgentBehaviorDriftDetector — Agent 行为漂移追踪
package core

import (
	"sync"
	"time"
)

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
