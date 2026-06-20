//go:build ignore

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
	"fmt"
	"math"
	"math/rand"
	"net/http"
	"sync"
	"time"
)

// ============================================================================
// 限流器 — Token Bucket
// ============================================================================

// RateLimiter Token Bucket 限流器。
// 线程安全，支持突发流量。
type RateLimiter struct {
	mu        sync.Mutex
	rate      float64    // 每秒填充 token 数
	burst     float64    // 最大 token 容量
	tokens    float64    // 当前 token 数
	lastTime  time.Time  // 上次填充时间
}

// NewRateLimiter 创建限流器。
// rate: 每秒允许的请求数
// burst: 允许的最大突发请求数（0 = rate）
func NewRateLimiter(rate, burst float64) *RateLimiter {
	if burst <= 0 {
		burst = rate
	}
	if burst < 1 {
		burst = 1
	}
	return &RateLimiter{
		rate:     rate,
		burst:    burst,
		tokens:   burst,
		lastTime: time.Now(),
	}
}

// Allow 检查是否允许请求通过。
// 消耗 1 个 token，如果 token 不足返回 false。
func (rl *RateLimiter) Allow() bool {
	return rl.AllowN(1)
}

// AllowN 检查是否允许 N 个请求通过。
func (rl *RateLimiter) AllowN(n float64) bool {
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
func (rl *RateLimiter) Wait(ctx context.Context) error {
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
func (rl *RateLimiter) Tokens() float64 {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	return rl.tokens
}

// SetRate 动态调整速率。
func (rl *RateLimiter) SetRate(rate float64) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.rate = rate
}

// ============================================================================
// 熔断器 — 滑动窗口
// ============================================================================

// CircuitState 熔断器状态。
type CircuitState int

const (
	CircuitClosed   CircuitState = iota // 正常，请求通过
	CircuitOpen                         // 熔断，快速失败
	CircuitHalfOpen                     // 半开，探测恢复
)

func (s CircuitState) String() string {
	switch s {
	case CircuitClosed:
		return "closed"
	case CircuitOpen:
		return "open"
	case CircuitHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// CircuitBreaker 滑动窗口熔断器。
// 在滑动时间窗口内统计失败率，超过阈值时熔断。
type CircuitBreaker struct {
	mu sync.RWMutex

	// 配置
	failureThreshold float64       // 失败率阈值 (0.0-1.0)
	windowSize       time.Duration // 滑动窗口大小
	cooldownPeriod   time.Duration // 熔断冷却时间
	halfOpenMaxReqs  int           // 半开状态最大探测请求数

	// 状态
	state        CircuitState
	lastFailure  time.Time
	openedAt     time.Time
	halfOpenReqs int

	// 滑动窗口统计
	buckets     []*windowBucket
	bucketSize  time.Duration
	numBuckets  int
	currentIdx  int
}

// windowBucket 滑动窗口中的一个桶。
type windowBucket struct {
	successes int64
	failures  int64
}

// CircuitBreakerConfig 熔断器配置。
type CircuitBreakerConfig struct {
	FailureThreshold float64       // 失败率阈值，默认 0.5
	WindowSize       time.Duration // 滑动窗口大小，默认 60s
	CooldownPeriod   time.Duration // 冷却时间，默认 30s
	HalfOpenMaxReqs  int           // 半开最大请求，默认 5
	NumBuckets       int           // 窗口桶数，默认 10
}

// DefaultCircuitBreakerConfig 返回默认熔断器配置。
func DefaultCircuitBreakerConfig() CircuitBreakerConfig {
	return CircuitBreakerConfig{
		FailureThreshold: 0.5,
		WindowSize:       60 * time.Second,
		CooldownPeriod:   30 * time.Second,
		HalfOpenMaxReqs:  5,
		NumBuckets:       10,
	}
}

// NewCircuitBreaker 创建熔断器。
func NewCircuitBreaker(cfg CircuitBreakerConfig) *CircuitBreaker {
	if cfg.FailureThreshold <= 0 {
		cfg.FailureThreshold = 0.5
	}
	if cfg.WindowSize <= 0 {
		cfg.WindowSize = 60 * time.Second
	}
	if cfg.CooldownPeriod <= 0 {
		cfg.CooldownPeriod = 30 * time.Second
	}
	if cfg.HalfOpenMaxReqs <= 0 {
		cfg.HalfOpenMaxReqs = 5
	}
	if cfg.NumBuckets <= 0 {
		cfg.NumBuckets = 10
	}

	buckets := make([]*windowBucket, cfg.NumBuckets)
	for i := range buckets {
		buckets[i] = &windowBucket{}
	}

	return &CircuitBreaker{
		failureThreshold: cfg.FailureThreshold,
		windowSize:       cfg.WindowSize,
		cooldownPeriod:   cfg.CooldownPeriod,
		halfOpenMaxReqs:  cfg.HalfOpenMaxReqs,
		buckets:          buckets,
		bucketSize:       cfg.WindowSize / time.Duration(cfg.NumBuckets),
		numBuckets:       cfg.NumBuckets,
		state:            CircuitClosed,
	}
}

// Execute 执行受熔断器保护的操作。
// 如果熔断器打开，返回 ErrCircuitBreakerOpen。
// 如果操作成功，记录成功；失败则记录失败。
func (cb *CircuitBreaker) Execute(fn func() error) error {
	cb.mu.Lock()

	// 检查状态
	if cb.state == CircuitOpen {
		if time.Since(cb.openedAt) >= cb.cooldownPeriod {
			cb.state = CircuitHalfOpen
			cb.halfOpenReqs = 0
		} else {
			cb.mu.Unlock()
			return ErrCircuitBreakerOpen
		}
	}

	if cb.state == CircuitHalfOpen {
		if cb.halfOpenReqs >= cb.halfOpenMaxReqs {
			cb.mu.Unlock()
			return ErrCircuitBreakerOpen
		}
		cb.halfOpenReqs++
	}
	cb.mu.Unlock()

	// 执行操作
	err := fn()

	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.advanceWindow()

	bucket := cb.buckets[cb.currentIdx]
	if err != nil {
		bucket.failures++
		cb.lastFailure = time.Now()

		// 检查是否需要熔断
		if cb.state == CircuitClosed || cb.state == CircuitHalfOpen {
			if cb.shouldOpen() {
				cb.state = CircuitOpen
				cb.openedAt = time.Now()
			}
		}
	} else {
		bucket.successes++

		// 半开状态成功，恢复闭合
		if cb.state == CircuitHalfOpen {
			cb.state = CircuitClosed
			cb.resetBuckets()
		}
	}

	return err
}

// State 返回当前熔断器状态。
func (cb *CircuitBreaker) State() CircuitState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// Stats 返回当前统计信息。
func (cb *CircuitBreaker) Stats() (successes, failures int64, failureRate float64) {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	cb.advanceWindowUnsafe()
	for _, b := range cb.buckets {
		successes += b.successes
		failures += b.failures
	}

	total := successes + failures
	if total > 0 {
		failureRate = float64(failures) / float64(total)
	}
	return
}

// Reset 重置熔断器到闭合状态。
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.state = CircuitClosed
	cb.halfOpenReqs = 0
	cb.resetBuckets()
}

// shouldOpen 检查是否应该熔断（需持有锁）。
func (cb *CircuitBreaker) shouldOpen() bool {
	_, _, failureRate := cb.Stats()
	return failureRate >= cb.failureThreshold
}

// advanceWindow 推进滑动窗口（需持有锁）。
func (cb *CircuitBreaker) advanceWindow() {
	now := time.Now()
	elapsed := now.Sub(cb.lastFailure)
	steps := int(elapsed / cb.bucketSize)
	if steps > cb.numBuckets {
		steps = cb.numBuckets
	}
	for i := 0; i < steps; i++ {
		cb.currentIdx = (cb.currentIdx + 1) % cb.numBuckets
		cb.buckets[cb.currentIdx] = &windowBucket{}
	}
}

// advanceWindowUnsafe 推进滑动窗口（读锁版本）。
func (cb *CircuitBreaker) advanceWindowUnsafe() {
	// 读锁版本不推进窗口，仅用于统计
}

// resetBuckets 重置所有桶。
func (cb *CircuitBreaker) resetBuckets() {
	for i := range cb.buckets {
		cb.buckets[i] = &windowBucket{}
	}
}

// ============================================================================
// 退避策略
// ============================================================================

// BackoffStrategy 退避策略接口。
type BackoffStrategy interface {
	// NextDelay 返回下一次重试的等待时间。
	// attempt 从 0 开始。
	NextDelay(attempt int) time.Duration
	// Reset 重置策略。
	Reset()
}

// ExponentialBackoff 指数退避策略。
type ExponentialBackoff struct {
	BaseDelay   time.Duration // 基础延迟
	MaxDelay    time.Duration // 最大延迟
	Multiplier  float64       // 倍增因子
	Jitter      bool          // 是否添加随机抖动
	attempt     int
}

// NewExponentialBackoff 创建指数退避策略。
func NewExponentialBackoff(base, max time.Duration, multiplier float64, jitter bool) *ExponentialBackoff {
	if base <= 0 {
		base = 100 * time.Millisecond
	}
	if max <= 0 {
		max = 30 * time.Second
	}
	if multiplier <= 0 {
		multiplier = 2.0
	}
	return &ExponentialBackoff{
		BaseDelay:  base,
		MaxDelay:   max,
		Multiplier: multiplier,
		Jitter:     jitter,
	}
}

func (e *ExponentialBackoff) NextDelay(attempt int) time.Duration {
	e.attempt = attempt
	delay := float64(e.BaseDelay) * math.Pow(e.Multiplier, float64(attempt))
	if delay > float64(e.MaxDelay) {
		delay = float64(e.MaxDelay)
	}

	// 随机抖动：在 [delay/2, delay] 范围内随机
	if e.Jitter {
		delay = delay/2 + delay*rand.Float64()/2
	}

	return time.Duration(delay)
}

func (e *ExponentialBackoff) Reset() {
	e.attempt = 0
}

// LinearBackoff 线性退避策略。
type LinearBackoff struct {
	BaseDelay time.Duration
	MaxDelay  time.Duration
	Increment time.Duration
	Jitter    bool
	attempt   int
}

// NewLinearBackoff 创建线性退避策略。
func NewLinearBackoff(base, max, increment time.Duration, jitter bool) *LinearBackoff {
	if base <= 0 {
		base = 100 * time.Millisecond
	}
	if max <= 0 {
		max = 30 * time.Second
	}
	if increment <= 0 {
		increment = 100 * time.Millisecond
	}
	return &LinearBackoff{
		BaseDelay: base,
		MaxDelay:  max,
		Increment: increment,
		Jitter:    jitter,
	}
}

func (l *LinearBackoff) NextDelay(attempt int) time.Duration {
	l.attempt = attempt
	delay := l.BaseDelay + time.Duration(attempt)*l.Increment
	if delay > l.MaxDelay {
		delay = l.MaxDelay
	}
	if l.Jitter {
		delay = delay/2 + time.Duration(rand.Int63n(int64(delay/2)))
	}
	return delay
}

func (l *LinearBackoff) Reset() {
	l.attempt = 0
}

// ============================================================================
// 重试执行器
// ============================================================================

// RetryWithBackoff 使用退避策略重试操作。
// maxRetries: 最大重试次数（0 = 无限重试直到 ctx 取消）
func RetryWithBackoff(ctx context.Context, strategy BackoffStrategy, maxRetries int, fn func() error) error {
	strategy.Reset()

	for attempt := 0; maxRetries == 0 || attempt <= maxRetries; attempt++ {
		err := fn()
		if err == nil {
			return nil
		}

		// 检查是否应该重试
		if IsRetryable(err) {
			if attempt == maxRetries {
				return fmt.Errorf("retry: max retries (%d) exceeded: %w", maxRetries, err)
			}

			delay := strategy.NextDelay(attempt)
			select {
			case <-ctx.Done():
				return fmt.Errorf("retry: cancelled: %w", ctx.Err())
			case <-time.After(delay):
				continue
			}
		}

		return err
	}

	return fmt.Errorf("retry: unexpected loop exit")
}

// IsRetryable 检查错误是否可重试。
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}
	// 熔断器打开、限流等临时错误可重试
	if err == ErrCircuitBreakerOpen || err == ErrTooManyRequests || err == ErrUnavailable {
		return true
	}
	if re, ok := err.(*RichError); ok {
		return re.Code == ErrCodeUnavailable ||
			re.Code == ErrCodeTimeout ||
			re.Code == ErrCodeCircuitOpen ||
			re.Code == ErrCodeRateLimited
	}
	return false
}

// RetryableError 包装错误为可重试。
func RetryableError(err error) error {
	return &RichError{
		Code:     ErrCodeUnavailable,
		Message:  err.Error(),
		Category: CatWarning,
		Stack:    CaptureStackTrace(1, 32),
	}
}

// ============================================================================
// HTTP 限流中间件
// ============================================================================

// RateLimitMiddleware 创建 HTTP 限流中间件。
func RateLimitMiddleware(limiter *RateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !limiter.Allow() {
				w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%.0f", limiter.rate))
				w.Header().Set("Retry-After", "1")
				http.Error(w, `{"error":"rate limited"}`, http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
