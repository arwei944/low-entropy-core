//go:build lecore_tier5 || lecore_tier6 || lecore_tier7

// Package core — 联邦降级协调 (v4.0)
//
// 包含:
//   - FederatedDegradationManager: 多实例联邦降级协调
//
// 多实例通过 EventBus 协调降级决策，支持投票机制。
package core

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// =============================================================================
// FederatedDegradationManager — 联邦降级协调 (T5.2)
// =============================================================================

// FederatedDegradationManager 包装 DegradationManager，增加跨实例协调。
// 多实例通过 EventBus 协调降级决策，支持投票机制。
type FederatedDegradationManager struct {
	inner       *DegradationManager
	instanceID  string
	eventBus    *EventBus
	obs         ObservationAdapter

	// 投票跟踪
	mu           sync.Mutex
	pendingVotes map[string]*degradationVote // proposalID -> vote
}

// degradationVote 跟踪降级投票。
type degradationVote struct {
	proposalID    string
	mode          DegradationMode
	proposer      string
	votesFor      int
	votesAgainst  int
	totalInstances int
	deadline      time.Time
	resolved      bool
}

// NewFederatedDegradationManager 创建联邦降级管理器。
func NewFederatedDegradationManager(instanceID string, inner *DegradationManager, eventBus *EventBus, obs ObservationAdapter) *FederatedDegradationManager {
	if obs == nil {
		obs = &NoOpObservationAdapter{}
	}
	return &FederatedDegradationManager{
		inner:        inner,
		instanceID:   instanceID,
		eventBus:     eventBus,
		obs:          obs,
		pendingVotes: make(map[string]*degradationVote),
	}
}

// ProposeDegradation 提议降级到指定模式。
// 提议通过 EventBus 广播，需要超过半数实例同意才执行。
func (fdm *FederatedDegradationManager) ProposeDegradation(mode DegradationMode, totalInstances int) string {
	proposalID := fmt.Sprintf("degrade_%s_%d", fdm.instanceID, time.Now().UnixNano())

	payload := map[string]interface{}{
		"proposal_id":     proposalID,
		"mode":            string(mode),
		"proposer":        fdm.instanceID,
		"total_instances": totalInstances,
		"timestamp":       time.Now(),
	}
	payloadBytes, _ := json.Marshal(payload)
	event := EventEnvelope{
		EventID:      getGlobalUUIDGen().NextString(),
		EventType:    "degradation.proposed",
		AggregateID:  fdm.instanceID,
		EventData:    payloadBytes,
		Version:      1,
		Timestamp:    time.Now(),
	}
	fdm.eventBus.Execute(context.Background(), event)

	fdm.mu.Lock()
	fdm.pendingVotes[proposalID] = &degradationVote{
		proposalID:     proposalID,
		mode:           mode,
		proposer:       fdm.instanceID,
		votesFor:       1, // 提议者自动投票赞成
		totalInstances: totalInstances,
		deadline:       time.Now().Add(30 * time.Second),
	}
	fdm.mu.Unlock()

	return proposalID
}

// VoteOnDegradation 对降级提议投票。
// agree: true 表示同意，false 表示反对。
func (fdm *FederatedDegradationManager) VoteOnDegradation(proposalID string, agree bool) {
	fdm.mu.Lock()
	defer fdm.mu.Unlock()

	vote, ok := fdm.pendingVotes[proposalID]
	if !ok || vote.resolved {
		return
	}

	if agree {
		vote.votesFor++
	} else {
		vote.votesAgainst++
	}

	// 检查是否达成决议
	required := vote.totalInstances/2 + 1
	if vote.votesFor >= required {
		fdm.executeDegradation(vote.mode)
		vote.resolved = true
		fdm.broadcastDegradationConfirmation(proposalID, vote.mode, true)
	} else if vote.votesAgainst > vote.totalInstances-required {
		vote.resolved = true
		fdm.broadcastDegradationConfirmation(proposalID, vote.mode, false)
	}
}

// executeDegradation 执行降级决策。
func (fdm *FederatedDegradationManager) executeDegradation(mode DegradationMode) {
	fdm.inner.Degrade(mode)

	es := NewExecutionStep("FederatedDegradation", "degrade",
		fmt.Sprintf("federated degradation: mode=%s, instance=%s", mode, fdm.instanceID),
		"degradation",
	)
	es.Metadata = map[string]interface{}{
		"mode":       string(mode),
		"instance":   fdm.instanceID,
		"federated":  true,
		"timestamp":  time.Now(),
	}
	fdm.obs.Record([]ExecutionStep{es})
}

// broadcastDegradationConfirmation 广播降级决议。
func (fdm *FederatedDegradationManager) broadcastDegradationConfirmation(proposalID string, mode DegradationMode, confirmed bool) {
	eventType := "degradation.confirmed"
	if !confirmed {
		eventType = "degradation.rejected"
	}

	payload := map[string]interface{}{
		"proposal_id": proposalID,
		"mode":        string(mode),
		"confirmed":   confirmed,
		"timestamp":   time.Now(),
	}
	payloadBytes, _ := json.Marshal(payload)
	event := EventEnvelope{
		EventID:      getGlobalUUIDGen().NextString(),
		EventType:    eventType,
		AggregateID:   fdm.instanceID,
		EventData:    payloadBytes,
		Version:      1,
		Timestamp:    time.Now(),
	}
	fdm.eventBus.Execute(context.Background(), event)
}

// Recover 恢复所有降级。
func (fdm *FederatedDegradationManager) Recover() {
	fdm.inner.Recover()
}

// CurrentMode 返回当前降级模式。
func (fdm *FederatedDegradationManager) CurrentMode() DegradationMode {
	return fdm.inner.CurrentMode()
}

// ShouldProcess 检查操作是否应被处理。
func (fdm *FederatedDegradationManager) ShouldProcess(criticality string) bool {
	return fdm.inner.ShouldProcess(criticality)
}
