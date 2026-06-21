//go:build lecore_tier5 || lecore_tier6 || lecore_tier7

// Package core — 分布式健康检查 (v4.0)
//
// 原语归属: L3 Adapter（分布式韧性层）
// 通过 EventBus 广播健康状态变更，协调多实例降级。
// Check/UpdateStatus 方法属于 Adapter（有 I/O）。
package core

import (
	"sync"
	"time"
)

// =============================================================================
// HealthCheck — 健康检查端点 (T5.4)
// =============================================================================

// DistHealthStatus 表示组件健康状态。
type DistHealthStatus string

const (
	DistHealthUp       DistHealthStatus = "UP"
	DistHealthDown     DistHealthStatus = "DOWN"
	DistHealthDegraded DistHealthStatus = "DEGRADED"
)

// DistHealthCheckResponse 是健康检查的响应结构。
type DistHealthCheckResponse struct {
	Status     DistHealthStatus              `json:"status"`
	Components map[string]DistComponentHealth `json:"components"`
	Timestamp  time.Time                 `json:"timestamp"`
}

// DistComponentHealth 是单个组件的健康状态。
type DistComponentHealth struct {
	Status  DistHealthStatus `json:"status"`
	Details string       `json:"details,omitempty"`
}

// DistHealthChecker 是健康检查的接口。
type DistHealthChecker interface {
	CheckHealth() DistHealthCheckResponse
	CheckReadiness() DistHealthCheckResponse
	CheckLiveness() DistHealthCheckResponse
}

// DistDefaultDistHealthChecker 是默认的健康检查实现。
// 检查所有已注册组件的健康状态。
type DistDefaultDistHealthChecker struct {
	mu         sync.RWMutex
	components map[string]func() DistHealthStatus
}

// NewDistDistDefaultDistHealthChecker 创建默认健康检查器。
func NewDistDistDefaultDistHealthChecker() *DistDefaultDistHealthChecker {
	return &DistDefaultDistHealthChecker{
		components: make(map[string]func() DistHealthStatus),
	}
}

// RegisterComponent 注册一个健康检查组件。
func (hc *DistDefaultDistHealthChecker) RegisterComponent(name string, check func() DistHealthStatus) {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	hc.components[name] = check
}

// CheckHealth 检查所有组件的健康状态。
func (hc *DistDefaultDistHealthChecker) CheckHealth() DistHealthCheckResponse {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	resp := DistHealthCheckResponse{
		Components: make(map[string]DistComponentHealth),
		Timestamp:  time.Now(),
	}
	allUp := true

	for name, check := range hc.components {
		status := check()
		resp.Components[name] = DistComponentHealth{Status: status}
		if status != DistHealthUp {
			allUp = false
		}
	}

	if allUp {
		resp.Status = DistHealthUp
	} else {
		resp.Status = DistHealthDegraded
	}
	return resp
}

// CheckReadiness 检查就绪状态（所有依赖可用）。
func (hc *DistDefaultDistHealthChecker) CheckReadiness() DistHealthCheckResponse {
	return hc.CheckHealth()
}

// CheckLiveness 检查存活状态（进程存活）。
func (hc *DistDefaultDistHealthChecker) CheckLiveness() DistHealthCheckResponse {
	return DistHealthCheckResponse{
		Status:     DistHealthUp,
		Components: make(map[string]DistComponentHealth),
		Timestamp:  time.Now(),
	}
}
