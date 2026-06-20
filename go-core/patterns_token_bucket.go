// Package core — 韧性增强模块 (v0.9.0)
//
// 提供企业级韧性能力：
//   - 限流器: Token Bucket 算法，支持突发流量
//   - 熔断器: 滑动窗口算法，三种状态 (Closed/Open/HalfOpen)
//   - 退避策略: 指数退避 + 随机抖动
//   - 健康端点: 聚合多个 HealthChecker

package core

import (
	"context"
	"sync"
	"time"
)

// ============================================================================
// 限流器 — Token Bucket
// ============================================================================

// TokenBucketRateLimiter Token Bucket 限流器。
// 线程安全，支持突发流量。
type TokenBucketRateLimiter struct {
	mu        sync.Mutex
	rate      float64    // 每秒填充 token 数
	burst     float64    // 最大 token 容量
	tokens    float64    // 当前 token 数
	lastTime  time.Time  // 上次填充时间
}

// NewRateLimiter 创建限流器。
// rate: 每秒允许的请求数
// burst: 允许的最大突发请求数（0 = rate）
func NewTokenBucketRateLimiter(rate, burst float64) *TokenBucketRateLimiter {
	if burst <= 0 {
		burst = rate
	}
	if burst < 1 {
		burst = 1
	}
	return &TokenBucketRateLimiter{
		rate:     rate,
		burst:    burst,
		tokens:   burst,
		lastTime: time.Now(),
	}
}

// Allow 检查是否允许请求通过。
// 消耗 1 个 token，如果 token 不足返回 false。
func (rl *TokenBucketRateLimiter) Allow() bool {
	return rl.AllowN(1)
}

// AllowN 检查是否允许 N 个请求通过。
func (rl *TokenBucketRateLimiter) AllowN(n float64) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(rl.lastTime).Seconds()
	rl.tokens += elapsed * rl.rate
	if rl.tokens > rl.burst {
		rl.tokens = rl.burst
	}
	rl.lastTime = now

	if rl.tokens >= n {
		rl.tokens -= n
		return true
	}
	return false
}

// Wait 阻塞等待直到允许通过或 ctx 取消。
func (rl *TokenBucketRateLimiter) Wait(ctx context.Context) error {
	for {
		if rl.Allow() {
			return nil
		}

		rl.mu.Lock()
		tokens := rl.tokens
		rl.mu.Unlock()

		// 计算需要等待的时间
		waitTime := time.Duration((1.0-tokens)/rl.rate) * time.Second
		if waitTime < time.Millisecond {
			waitTime = time.Millisecond
		}
		if waitTime > time.Second {
			waitTime = time.Second
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(waitTime):
		}
	}
}

// Tokens 返回当前可用 token 数。
func (rl *TokenBucketRateLimiter) Tokens() float64 {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	return rl.tokens
}

// SetRate 动态调整速率。
func (rl *TokenBucketRateLimiter) SetRate(rate float64) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.rate = rate
}
