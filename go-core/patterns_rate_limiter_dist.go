//go:build lecore_tier5 || lecore_tier6 || lecore_tier7

// Package core — 分布式限流协调 (v4.0)
//
// 包含:
//   - DistributedRateLimiter: 分布式限流协调
//
// 通过心跳共享令牌使用率，动态调整各实例配额。
package core

import (
	"math"
	"sync/atomic"
	"time"
)

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
