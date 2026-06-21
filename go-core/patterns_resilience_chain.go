//go:build lecore_tier2 || lecore_tier3 || lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

// Package core — 韧性链 (v4.0)
//
// 原语归属: L1 Composer
// 编排多个韧性原语（熔断、重试、退避、降级）的执行顺序。
// Run 方法属于 Composer（仅编排，无业务逻辑）。
package core

import (
	"time"
)

// ──────────────────────────────────────────────
// ResilienceChain — compose multiple resilience patterns (v4.0 fixed order)
// ──────────────────────────────────────────────

// ResilienceChain wraps a Composer with rate limiter, bulkhead, circuit breaker,
// and fallback in the correct order: RateLimiter → Bulkhead → CircuitBreaker → Fallback → inner.
//
// 包装顺序的设计理由：
//   1. RateLimiter（最外层）：最早拒绝超限请求，保护下游所有组件
//   2. Bulkhead：限制并发数，防止资源耗尽
//   3. CircuitBreaker：检测下游故障，熔断保护
//   4. Fallback（最内层）：兜底处理，在熔断或失败时提供降级响应
//
// v4.0: 修正了之前错误的包装顺序（原为 Fallback → CircuitBreaker → Bulkhead → RateLimiter）。
func ResilienceChain[T any](inner Composer[T], config ResilienceConfig[T]) Composer[T] {
	var comp Composer[T] = inner

	// 最内层：Fallback（兜底处理）
	if config.Fallback != nil {
		comp = NewFallback[T](comp, config.Fallback)
	}
	// 第二层：CircuitBreaker（熔断保护）
	if config.CircuitBreakerThreshold > 0 {
		comp = NewCircuitBreaker[T](comp, config.CircuitBreakerThreshold, config.CircuitBreakerCooldown)
	}
	// 第三层：Bulkhead（并发限制）
	if config.BulkheadMax > 0 {
		comp = NewBulkhead[T](comp, config.BulkheadMax)
	}
	// 最外层：RateLimiter（流量整形）
	if config.RateLimit > 0 {
		comp = NewRateLimiter[T](comp, config.RateLimit, config.RateLimitBurst)
	}

	return comp
}

// ResilienceConfig configures the resilience chain.
type ResilienceConfig[T any] struct {
	// RateLimit is the rate limit in requests/second (0 = unlimited).
	RateLimit float64

	// RateLimitBurst is the maximum burst size.
	RateLimitBurst float64

	// BulkheadMax is the maximum concurrency (0 = unlimited).
	BulkheadMax int

	// CircuitBreakerThreshold is the failure count before opening (0 = disabled).
	CircuitBreakerThreshold int

	// CircuitBreakerCooldown is the cooldown period before half-open.
	CircuitBreakerCooldown time.Duration

	// Fallback is the fallback composer (nil = no fallback).
	Fallback Composer[T]
}

// DefaultResilienceConfig returns a sensible default resilience config.
func DefaultResilienceConfig[T any]() ResilienceConfig[T] {
	return ResilienceConfig[T]{
		RateLimit:                100,
		RateLimitBurst:           200,
		BulkheadMax:              50,
		CircuitBreakerThreshold:  5,
		CircuitBreakerCooldown:   30 * time.Second,
	}
}
