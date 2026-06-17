package core

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"
)

// ──────────────────────────────────────────────
// CircuitBreaker — pattern for resilience (v4.0 atomic)
// ──────────────────────────────────────────────

// circuitStateVal 是 CircuitBreaker 状态的内部表示。
const (
	circuitStateClosed   uint32 = 0
	circuitStateOpen     uint32 = 1
	circuitStateHalfOpen uint32 = 2
)

// CircuitState represents the state of a circuit breaker (public API, string type).
type CircuitState string

const (
	CircuitClosed   CircuitState = "closed"
	CircuitOpen     CircuitState = "open"
	CircuitHalfOpen CircuitState = "half_open"
)

// CircuitBreaker implements the circuit breaker pattern as a Composer[T].
// When failures exceed a threshold, the circuit opens and requests are rejected
// immediately. After a cooldown period, the circuit transitions to half-open,
// allowing a single probe request to test if the service has recovered.
//
// v4.0: 使用 atomic 操作替代 sync.Mutex，消除双锁问题。
// state、failures、lastFailure 均为独立原子变量，热路径无锁。
type CircuitBreaker[T any] struct {
	inner       Composer[T]
	threshold   int
	cooldownNano int64 // cooldown in nanoseconds (atomic)

	state       atomic.Uint32 // circuitStateVal
	failures    atomic.Uint64 // 连续失败计数
	lastFailure atomic.Int64  // 最后一次失败的时间戳 (unix nano)
}

// NewCircuitBreaker creates a circuit breaker wrapping the given Composer.
// threshold: number of consecutive failures before opening the circuit.
// cooldown: how long to wait before transitioning to half-open.
func NewCircuitBreaker[T any](inner Composer[T], threshold int, cooldown time.Duration) *CircuitBreaker[T] {
	if threshold <= 0 {
		threshold = 5
	}
	if cooldown <= 0 {
		cooldown = 30 * time.Second
	}
	cb := &CircuitBreaker[T]{
		inner:        inner,
		threshold:    threshold,
		cooldownNano: int64(cooldown),
	}
	cb.state.Store(circuitStateClosed)
	return cb
}

// Run implements the Composer interface with circuit breaker logic.
// v4.0: 无锁热路径，仅在状态转换时使用 CAS。
func (cb *CircuitBreaker[T]) Run(ctx context.Context, input T) (T, []ExecutionStep, error) {
	state := cb.state.Load()

	// 快速路径：熔断器打开时检查冷却时间
	if state == circuitStateOpen {
		lastFailure := cb.lastFailure.Load()
		cooldownNano := atomic.LoadInt64(&cb.cooldownNano)
		now := time.Now().UnixNano()

		if now-lastFailure > cooldownNano {
			// 尝试转换到半开状态
			if !cb.state.CompareAndSwap(circuitStateOpen, circuitStateHalfOpen) {
				// CAS 失败，重新读取状态
				state = cb.state.Load()
				if state == circuitStateOpen {
					// 冷却时间未到，拒绝请求
					var zero T
					return zero, nil, NewStepError("CIRCUIT_OPEN",
						"circuit breaker is open", true)
				}
				// 状态已变化，继续执行
			}
		} else {
			// 冷却时间未到，拒绝请求
			var zero T
			return zero, nil, NewStepError("CIRCUIT_OPEN",
				"circuit breaker is open", true)
		}
	}

	// 执行内部 Composer
	result, steps, err := cb.inner.Run(ctx, input)

	if err != nil {
		// 失败处理
		cb.failures.Add(1)
		cb.lastFailure.Store(time.Now().UnixNano())
		failures := cb.failures.Load()
		currentState := cb.state.Load()

		if currentState == circuitStateHalfOpen {
			// 探测失败：回到打开状态
			cb.state.Store(circuitStateOpen)
		} else if int(failures) >= cb.threshold {
			// 失败次数达到阈值：打开熔断器
			cb.state.Store(circuitStateOpen)
		}

		return result, steps, err
	}

	// 成功处理
	cb.failures.Store(0)
	currentState := cb.state.Load()
	if currentState == circuitStateHalfOpen {
		cb.state.Store(circuitStateClosed)
	}

	return result, steps, nil
}

// State returns the current circuit state.
func (cb *CircuitBreaker[T]) State() CircuitState {
	switch cb.state.Load() {
	case circuitStateOpen:
		return CircuitOpen
	case circuitStateHalfOpen:
		return CircuitHalfOpen
	default:
		return CircuitClosed
	}
}

// ──────────────────────────────────────────────
// Fallback — pattern for graceful degradation
// ──────────────────────────────────────────────

// Fallback wraps a primary Composer with a fallback that executes when the primary fails.
// This implements the graceful degradation pattern.
type Fallback[T any] struct {
	primary  Composer[T]
	fallback Composer[T]
}

// NewFallback creates a fallback composer.
func NewFallback[T any](primary, fallback Composer[T]) *Fallback[T] {
	return &Fallback[T]{primary: primary, fallback: fallback}
}

// Run implements the Composer interface with fallback logic.
func (f *Fallback[T]) Run(ctx context.Context, input T) (T, []ExecutionStep, error) {
	result, steps, err := f.primary.Run(ctx, input)
	if err == nil {
		return result, steps, nil
	}

	// Primary failed — execute fallback
	fallbackResult, fallbackSteps, fallbackErr := f.fallback.Run(ctx, input)
	allSteps := append(steps, fallbackSteps...)
	return fallbackResult, allSteps, fallbackErr
}

// ──────────────────────────────────────────────
// Bulkhead — pattern for resource isolation
// ──────────────────────────────────────────────

// Bulkhead limits concurrent execution of a Composer to a fixed number of
// goroutines. Requests beyond the limit are rejected immediately.
type Bulkhead[T any] struct {
	inner      Composer[T]
	semaphore  chan struct{}
}

// NewBulkhead creates a bulkhead with the given concurrency limit.
func NewBulkhead[T any](inner Composer[T], maxConcurrency int) *Bulkhead[T] {
	if maxConcurrency <= 0 {
		maxConcurrency = 10
	}
	return &Bulkhead[T]{
		inner:     inner,
		semaphore: make(chan struct{}, maxConcurrency),
	}
}

// Run implements the Composer interface with bulkhead isolation.
// Requests are rejected if the semaphore is full.
func (b *Bulkhead[T]) Run(ctx context.Context, input T) (T, []ExecutionStep, error) {
	select {
	case b.semaphore <- struct{}{}:
		defer func() { <-b.semaphore }()
		return b.inner.Run(ctx, input)
	case <-ctx.Done():
		var zero T
		return zero, nil, ctx.Err()
	default:
		var zero T
		return zero, nil, NewStepError("BULKHEAD_FULL",
			"bulkhead is full, request rejected", true)
	}
}

// Available returns the number of available slots in the bulkhead.
func (b *Bulkhead[T]) Available() int {
	return cap(b.semaphore) - len(b.semaphore)
}

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

// ──────────────────────────────────────────────
// ShardedRateLimiter — 分片限流器 (v4.0)
// ──────────────────────────────────────────────

// ShardedRateLimiter 按 key 分片的限流器，用于多租户/多 API Key 场景。
// 每个分片有独立的原子令牌桶，不同 key 的请求完全并行，无竞争。
//
// 设计原理：
//   - 使用 FNV-1a 哈希将 key 映射到 256 个分片之一
//   - 每个分片维护独立的 atomic 令牌桶
//   - 不同 key 的请求不会互相阻塞
//   - 适合十亿级调用量下按租户/用户限流
type ShardedRateLimiter[K comparable] struct {
	shards [shardCount]*shardedTokenBucket
}

// shardedTokenBucket 是单个分片的令牌桶。
type shardedTokenBucket struct {
	rate       int64 // 微令牌/秒
	capacity   int64 // 微令牌容量
	tokens     atomic.Int64
	lastRefill atomic.Int64
}

// NewShardedRateLimiter 创建分片限流器。
// rate: 每个 key 的每秒令牌数。
// capacity: 每个 key 的突发容量。
func NewShardedRateLimiter[K comparable](rate, capacity float64) *ShardedRateLimiter[K] {
	if rate <= 0 {
		rate = 1
	}
	if capacity <= 0 {
		capacity = rate
	}
	sr := &ShardedRateLimiter[K]{}
	for i := 0; i < shardCount; i++ {
		b := &shardedTokenBucket{
			rate:     int64(rate * 1e6),
			capacity: int64(capacity * 1e6),
		}
		b.tokens.Store(b.capacity)
		b.lastRefill.Store(time.Now().UnixNano())
		sr.shards[i] = b
	}
	return sr
}

// Allow 检查 key 是否允许通过（消费 1 个令牌）。
// 返回 true 表示允许，false 表示被限流。
// 使用 FNV-1a 哈希选择分片，无锁 CAS 令牌消费。
func (sr *ShardedRateLimiter[K]) Allow(key K) bool {
	idx := hashKey(key) & 0xFF
	b := sr.shards[idx]

	refillBucket(b)
	current := b.tokens.Load()
	if current < 1e6 {
		return false
	}
	return b.tokens.CompareAndSwap(current, current-1e6)
}

// AllowN 检查 key 是否允许消费 n 个令牌。
// 返回 true 表示允许，false 表示被限流。
func (sr *ShardedRateLimiter[K]) AllowN(key K, n float64) bool {
	idx := hashKey(key) & 0xFF
	b := sr.shards[idx]

	need := int64(n * 1e6)
	refillBucket(b)
	current := b.tokens.Load()
	if current < need {
		return false
	}
	return b.tokens.CompareAndSwap(current, current-need)
}

// Tokens 返回 key 的当前令牌数。
func (sr *ShardedRateLimiter[K]) Tokens(key K) float64 {
	idx := hashKey(key) & 0xFF
	b := sr.shards[idx]
	refillBucket(b)
	return float64(b.tokens.Load()) / 1e6
}

// hashKey 计算泛型 key 的哈希值。
// 支持 string 和整数类型的 key。
func hashKey[K comparable](key K) uint64 {
	switch v := any(key).(type) {
	case string:
		return hashString64(v)
	case int:
		return uint64(v)
	case int64:
		return uint64(v)
	case uint64:
		return v
	default:
		// 回退到 fmt.Sprintf 后哈希（非热路径，用于非标准类型）
		return hashString64(fmt.Sprintf("%v", key))
	}
}

// 注意：需要 fmt 包用于 hashKey 的 fallback 分支
// 该 import 已在文件顶部声明

// refillBucket 根据经过的时间填充令牌桶。
func refillBucket(b *shardedTokenBucket) {
	now := time.Now().UnixNano()
	lastRefill := b.lastRefill.Load()

	if now <= lastRefill {
		return
	}

	elapsedNano := now - lastRefill
	newTokens := b.rate * elapsedNano / 1e9

	if newTokens <= 0 {
		return
	}

	if !b.lastRefill.CompareAndSwap(lastRefill, now) {
		return
	}

	for {
		current := b.tokens.Load()
		next := current + newTokens
		if next > b.capacity {
			next = b.capacity
		}
		if b.tokens.CompareAndSwap(current, next) {
			break
		}
	}
}

// hashString64 计算字符串的 FNV-1a 64 位哈希。
func hashString64(s string) uint64 {
	var h uint64 = fnvOffsetBasis64
	for i := 0; i < len(s); i++ {
		h ^= uint64(s[i])
		h *= fnvPrime64
	}
	return h
}

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