//go:build lecore_tier2 || lecore_tier3 || lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

package core

import (
	"context"
	"sync/atomic"
	"time"
)

// ──────────────────────────────────────────────
// RateLimiter — pattern for rate limiting (v4.0 atomic)
// ──────────────────────────────────────────────

// RateLimiter limits the rate of Composer execution using a token bucket.
// It allows bursts up to the bucket capacity, then refills at a steady rate.
//
// v4.0: 使用 atomic 操作替代 sync.Mutex，消除锁持有期间的浮点计算开销。
// 令牌以微令牌（×1e6）存储在 atomic.Int64 中，使用 CAS 循环进行令牌消费。
type RateLimiter[T any] struct {
	inner      Composer[T]
	rate       int64 // 微令牌/秒 (rate × 1e6)
	capacity   int64 // 微令牌容量 (capacity × 1e6)

	tokens     atomic.Int64 // 当前微令牌数
	lastRefill atomic.Int64 // 上次填充时间 (unix nano)
}

// NewRateLimiter creates a rate limiter.
// rate: tokens per second (e.g., 10.0 = 10 requests/second).
// capacity: maximum burst size (e.g., 20 = allow bursts of up to 20).
func NewRateLimiter[T any](inner Composer[T], rate, capacity float64) *RateLimiter[T] {
	if rate <= 0 {
		rate = 1
	}
	if capacity <= 0 {
		capacity = rate
	}
	rl := &RateLimiter[T]{
		inner:    inner,
		rate:     int64(rate * 1e6),
		capacity: int64(capacity * 1e6),
	}
	rl.tokens.Store(rl.capacity)
	rl.lastRefill.Store(time.Now().UnixNano())
	return rl
}

// Run implements the Composer interface with rate limiting.
// v4.0: 无锁 CAS 令牌消费，避免锁竞争。
func (rl *RateLimiter[T]) Run(ctx context.Context, input T) (T, []ExecutionStep, error) {
	// 尝试消费一个令牌
	for {
		// 先填充令牌
		rl.refillTokens()

		// CAS 消费一个令牌
		current := rl.tokens.Load()
		if current < 1e6 { // 不足 1 个令牌（微令牌 < 1e6）
			var zero T
			return zero, nil, NewStepError("RATE_LIMITED",
				"rate limit exceeded", true)
		}
		if rl.tokens.CompareAndSwap(current, current-1e6) {
			break
		}
		// CAS 失败，重试（其他 goroutine 可能也在消费）
	}

	return rl.inner.Run(ctx, input)
}

// refillTokens 根据经过的时间填充令牌。
// 从 lastRefill 到 now 的时间差计算应填充的微令牌数。
func (rl *RateLimiter[T]) refillTokens() {
	now := time.Now().UnixNano()
	lastRefill := rl.lastRefill.Load()

	// 只有当前时间大于上次填充时间时才填充
	if now <= lastRefill {
		return
	}

	elapsedNano := now - lastRefill
	// 微令牌 = 速率(微令牌/秒) × 经过时间(纳秒) / 1e9
	newTokens := rl.rate * elapsedNano / 1e9

	if newTokens <= 0 {
		return
	}

	// 尝试更新 lastRefill
	if !rl.lastRefill.CompareAndSwap(lastRefill, now) {
		// 其他 goroutine 已更新，跳过本次填充
		return
	}

	// 更新令牌数，上限为容量
	for {
		current := rl.tokens.Load()
		next := current + newTokens
		if next > rl.capacity {
			next = rl.capacity
		}
		if rl.tokens.CompareAndSwap(current, next) {
			break
		}
	}
}

// Tokens returns the current number of tokens in the bucket.
func (rl *RateLimiter[T]) Tokens() float64 {
	rl.refillTokens()
	return float64(rl.tokens.Load()) / 1e6
}
