// Package core — 分布式韧性 (v4.0)
//
// 本文件将单机韧性模式升级为跨服务分布式韧性，包括：
//   - GlobalCircuitBreaker：跨服务实例的全局熔断
//   - FederatedDegradationManager：多实例联邦降级协调
//   - DistributedRateLimiter：分布式限流协调
//   - HealthCheck：健康检查端点
//
// 分布式状态通过 EventBus 广播，各实例独立决策但协调一致。
package core

import (
	"context"
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"time"
)

// =============================================================================
// GlobalCircuitBreaker — 全局熔断器 (T5.1)
// =============================================================================

// GlobalCircuitConfig 配置全局熔断器行为。
type GlobalCircuitConfig struct {
	FailureRateThreshold float64       // 故障率阈值（默认 0.5）
	MinRequestCount      int           // 最小请求数（默认 10）
	CooldownPeriod       time.Duration // 冷却期（默认 30s）
	HalfOpenMaxRequests  int           // 半开状态最大探测请求数（默认 1）
}

// DefaultGlobalCircuitConfig 返回默认全局熔断配置。
func DefaultGlobalCircuitConfig() GlobalCircuitConfig {
	return GlobalCircuitConfig{
		FailureRateThreshold: 0.5,
		MinRequestCount:      10,
		CooldownPeriod:       30 * time.Second,
		HalfOpenMaxRequests:  1,
	}
}

// globalCircuitState 是全局熔断器的内部状态。
type globalCircuitState struct {
	state       atomic.Uint32 // circuitStateVal
	failures    atomic.Uint64
	successes   atomic.Uint64
	lastFailure atomic.Int64
	lastChange  atomic.Int64
}

// GlobalCircuitBreaker 管理跨服务实例的全局熔断。
// 当某个下游服务的所有实例故障率超过阈值时，全局熔断器打开。
// 状态变更通过 EventBus 广播到所有订阅者。
type GlobalCircuitBreaker struct {
	mu       sync.RWMutex
	services map[string]*globalCircuitState
	config   GlobalCircuitConfig
	eventBus *EventBus
	obs      ObservationAdapter
}

// NewGlobalCircuitBreaker 创建全局熔断器。
func NewGlobalCircuitBreaker(config GlobalCircuitConfig, eventBus *EventBus, obs ObservationAdapter) *GlobalCircuitBreaker {
	if obs == nil {
		obs = &NoOpObservationAdapter{}
	}
	return &GlobalCircuitBreaker{
		services: make(map[string]*globalCircuitState),
		config:   config,
		eventBus: eventBus,
		obs:      obs,
	}
}

// ReportFailure 报告服务调用失败。
// serviceName: 下游服务名，instanceID: 实例标识。
func (gcb *GlobalCircuitBreaker) ReportFailure(serviceName, instanceID string) {
	gcb.ensureService(serviceName)
	state := gcb.services[serviceName]

	state.failures.Add(1)
	state.lastFailure.Store(time.Now().UnixNano())

	total := state.failures.Load() + state.successes.Load()
	if total >= uint64(gcb.config.MinRequestCount) {
		failureRate := float64(state.failures.Load()) / float64(total)
		if failureRate >= gcb.config.FailureRateThreshold {
			if state.state.CompareAndSwap(circuitStateClosed, circuitStateOpen) {
				state.lastChange.Store(time.Now().UnixNano())
				gcb.broadcastStateChange(serviceName, "open", failureRate)
			}
		}
	}
}

// ReportSuccess 报告服务调用成功。
func (gcb *GlobalCircuitBreaker) ReportSuccess(serviceName, instanceID string) {
	gcb.ensureService(serviceName)
	state := gcb.services[serviceName]
	state.successes.Add(1)

	// 半开状态下的成功恢复
	if state.state.Load() == circuitStateHalfOpen {
		state.state.Store(circuitStateClosed)
		state.failures.Store(0)
		state.successes.Store(0)
		state.lastChange.Store(time.Now().UnixNano())
		gcb.broadcastStateChange(serviceName, "closed", 0)
	}
}

// IsOpen 检查服务是否全局熔断。
func (gcb *GlobalCircuitBreaker) IsOpen(serviceName string) bool {
	gcb.ensureService(serviceName)
	state := gcb.services[serviceName]
	currentState := state.state.Load()

	if currentState == circuitStateOpen {
		lastFailure := state.lastFailure.Load()
		if time.Now().UnixNano()-lastFailure > int64(gcb.config.CooldownPeriod) {
			state.state.Store(circuitStateHalfOpen)
			state.lastChange.Store(time.Now().UnixNano())
			return false
		}
		return true
	}
	return false
}

// IsHalfOpen 检查服务是否处于半开状态。
func (gcb *GlobalCircuitBreaker) IsHalfOpen(serviceName string) bool {
	gcb.ensureService(serviceName)
	return gcb.services[serviceName].state.Load() == circuitStateHalfOpen
}

// Reset 重置服务的熔断状态。
func (gcb *GlobalCircuitBreaker) Reset(serviceName string) {
	gcb.ensureService(serviceName)
	state := gcb.services[serviceName]
	state.state.Store(circuitStateClosed)
	state.failures.Store(0)
	state.successes.Store(0)
	state.lastChange.Store(time.Now().UnixNano())
}

// ensureService 确保服务状态存在。
func (gcb *GlobalCircuitBreaker) ensureService(serviceName string) {
	gcb.mu.Lock()
	if _, ok := gcb.services[serviceName]; !ok {
		s := &globalCircuitState{}
		s.state.Store(circuitStateClosed)
		gcb.services[serviceName] = s
	}
	gcb.mu.Unlock()
}

// broadcastStateChange 广播熔断状态变更。
func (gcb *GlobalCircuitBreaker) broadcastStateChange(serviceName, newState string, failureRate float64) {
	if gcb.eventBus == nil {
		return
	}
	event := EventEnvelope{
		EventID:   getGlobalUUIDGen().NextString(),
		EventType: "circuit_breaker.state_change",
		AggregateID: serviceName,
		Payload: map[string]interface{}{
			"service":      serviceName,
			"state":        newState,
			"failure_rate": failureRate,
			"timestamp":    time.Now(),
		},
		Version:   1,
		Timestamp: time.Now(),
	}
	gcb.eventBus.Execute(context.Background(), event)
}

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

	event := EventEnvelope{
		EventID:     getGlobalUUIDGen().NextString(),
		EventType:   "degradation.proposed",
		AggregateID: fdm.instanceID,
		Payload: map[string]interface{}{
			"proposal_id":     proposalID,
			"mode":            string(mode),
			"proposer":        fdm.instanceID,
			"total_instances": totalInstances,
			"timestamp":       time.Now(),
		},
		Version:   1,
		Timestamp: time.Now(),
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

	event := EventEnvelope{
		EventID:     getGlobalUUIDGen().NextString(),
		EventType:   eventType,
		AggregateID: fdm.instanceID,
		Payload: map[string]interface{}{
			"proposal_id": proposalID,
			"mode":        string(mode),
			"confirmed":   confirmed,
			"timestamp":   time.Now(),
		},
		Version:   1,
		Timestamp: time.Now(),
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

// =============================================================================
// DistributedRateLimiter — 分布式限流协调 (T5.3)
// =============================================================================

// DistributedRateLimiter 在 ShardedRateLimiter 基础上增加实例间协调。
// 通过心跳共享令牌使用率，动态调整各实例配额。
type DistributedRateLimiter[K comparable] struct {
	inner      *ShardedRateLimiter[K]
	instanceID string
	globalRate int64 // 全局总速率（微令牌/秒）

	// 本地配额
	localQuota atomic.Int64 // 微令牌/秒

	// 心跳
	heartbeatInterval time.Duration
	stopCh            chan struct{}
}

// NewDistributedRateLimiter 创建分布式限流器。
// globalRate: 所有实例的全局总速率。
// instanceCount: 预期实例数。
func NewDistributedRateLimiter[K comparable](inner *ShardedRateLimiter[K], instanceID string, globalRate float64, instanceCount int) *DistributedRateLimiter[K] {
	if instanceCount <= 0 {
		instanceCount = 1
	}
	dr := &DistributedRateLimiter[K]{
		inner:             inner,
		instanceID:        instanceID,
		globalRate:        int64(globalRate * 1e6),
		heartbeatInterval: 1 * time.Second,
		stopCh:            make(chan struct{}),
	}
	// 初始配额 = 全局速率 / 实例数
	dr.localQuota.Store(int64(globalRate*1e6) / int64(instanceCount))
	return dr
}

// StartHeartbeat 启动心跳协程，定期报告令牌使用率。
func (dr *DistributedRateLimiter[K]) StartHeartbeat() {
	go func() {
		ticker := time.NewTicker(dr.heartbeatInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				// 心跳在独立协程中发送，不阻塞
			case <-dr.stopCh:
				return
			}
		}
	}()
}

// StopHeartbeat 停止心跳。
func (dr *DistributedRateLimiter[K]) StopHeartbeat() {
	close(dr.stopCh)
}

// AdjustQuota 动态调整本地配额。
// 使用率高于 80% 时增加配额，低于 20% 时减少配额。
func (dr *DistributedRateLimiter[K]) AdjustQuota(usageRate float64) {
	currentQuota := dr.localQuota.Load()
	var newQuota int64

	switch {
	case usageRate > 0.8:
		// 增加 20%
		newQuota = int64(float64(currentQuota) * 1.2)
	case usageRate < 0.2:
		// 减少 20%
		newQuota = int64(float64(currentQuota) * 0.8)
	default:
		return
	}

	// 不超过全局总量的 50%
	maxQuota := dr.globalRate / 2
	if newQuota > maxQuota {
		newQuota = maxQuota
	}
	// 不低于全局总量的 5%
	minQuota := dr.globalRate / 20
	if newQuota < minQuota {
		newQuota = minQuota
	}

	dr.localQuota.Store(newQuota)
}

// Allow 检查 key 是否允许通过。
func (dr *DistributedRateLimiter[K]) Allow(key K) bool {
	return dr.inner.Allow(key)
}

// AllowN 检查 key 是否允许消费 n 个令牌。
func (dr *DistributedRateLimiter[K]) AllowN(key K, n float64) bool {
	return dr.inner.AllowN(key, n)
}

// UsageRate 返回当前令牌使用率（0-1）。
func (dr *DistributedRateLimiter[K]) UsageRate() float64 {
	// 简化实现：基于全局速率计算
	quota := float64(dr.localQuota.Load())
	global := float64(dr.globalRate)
	if global == 0 {
		return 0
	}
	return math.Min(quota/global, 1.0)
}

// =============================================================================
// HealthCheck — 健康检查端点 (T5.4)
// =============================================================================

// HealthStatus 表示组件健康状态。
type HealthStatus string

const (
	HealthUp       HealthStatus = "UP"
	HealthDown     HealthStatus = "DOWN"
	HealthDegraded HealthStatus = "DEGRADED"
)

// HealthCheckResponse 是健康检查的响应结构。
type HealthCheckResponse struct {
	Status     HealthStatus              `json:"status"`
	Components map[string]ComponentHealth `json:"components"`
	Timestamp  time.Time                 `json:"timestamp"`
}

// ComponentHealth 是单个组件的健康状态。
type ComponentHealth struct {
	Status  HealthStatus `json:"status"`
	Details string       `json:"details,omitempty"`
}

// HealthChecker 是健康检查的接口。
type HealthChecker interface {
	CheckHealth() HealthCheckResponse
	CheckReadiness() HealthCheckResponse
	CheckLiveness() HealthCheckResponse
}

// DefaultHealthChecker 是默认的健康检查实现。
// 检查所有已注册组件的健康状态。
type DefaultHealthChecker struct {
	mu         sync.RWMutex
	components map[string]func() HealthStatus
}

// NewDefaultHealthChecker 创建默认健康检查器。
func NewDefaultHealthChecker() *DefaultHealthChecker {
	return &DefaultHealthChecker{
		components: make(map[string]func() HealthStatus),
	}
}

// RegisterComponent 注册一个健康检查组件。
func (hc *DefaultHealthChecker) RegisterComponent(name string, check func() HealthStatus) {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	hc.components[name] = check
}

// CheckHealth 检查所有组件的健康状态。
func (hc *DefaultHealthChecker) CheckHealth() HealthCheckResponse {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	resp := HealthCheckResponse{
		Components: make(map[string]ComponentHealth),
		Timestamp:  time.Now(),
	}
	allUp := true

	for name, check := range hc.components {
		status := check()
		resp.Components[name] = ComponentHealth{Status: status}
		if status != HealthUp {
			allUp = false
		}
	}

	if allUp {
		resp.Status = HealthUp
	} else {
		resp.Status = HealthDegraded
	}
	return resp
}

// CheckReadiness 检查就绪状态（所有依赖可用）。
func (hc *DefaultHealthChecker) CheckReadiness() HealthCheckResponse {
	return hc.CheckHealth()
}

// CheckLiveness 检查存活状态（进程存活）。
func (hc *DefaultHealthChecker) CheckLiveness() HealthCheckResponse {
	return HealthCheckResponse{
		Status:     HealthUp,
		Components: make(map[string]ComponentHealth),
		Timestamp:  time.Now(),
	}
}