//go:build lecore_tier5 || lecore_tier6 || lecore_tier7

// Package core — 全局熔断器 (v4.0)
//
// 包含:
//   - GlobalCircuitBreaker: 跨服务实例的全局熔断
//
// 状态变更通过 EventBus 广播到所有订阅者。
package core

import (
	"context"
	"encoding/json"
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
	payload := map[string]any{
		"service":      serviceName,
		"state":        newState,
		"failure_rate": failureRate,
		"timestamp":    time.Now(),
	}
	payloadBytes, _ := json.Marshal(payload)
	event := EventEnvelope{
		EventID:      getGlobalUUIDGen().NextString(),
		EventType:    "circuit_breaker.state_change",
		AggregateID:   serviceName,
		EventData:    payloadBytes,
		Version:      1,
		Timestamp:    time.Now(),
	}
	gcb.eventBus.Execute(context.Background(), event)
}
