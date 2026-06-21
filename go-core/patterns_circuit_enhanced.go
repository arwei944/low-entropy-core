// Package core — 增强熔断器 (v4.0)
//
// 原语归属: L1 Adapter
// 基于状态的熔断器，通过内部状态机管理外部 I/O 调用。
// Execute 方法属于 Adapter（有 I/O），内部状态查询属于 Port。
package core

import (
	"sync"
	"time"
)

// ============================================================================
// 熔断器 — 滑动窗口
// ============================================================================

// EnhancedCircuitState 熔断器状态。
type EnhancedCircuitState int

const (
	EnhancedCircuitClosed   EnhancedCircuitState = iota // 正常，请求通过
	EnhancedCircuitOpen                         // 熔断，快速失败
	EnhancedCircuitHalfOpen                     // 半开，探测恢复
)

func (s EnhancedCircuitState) String() string {
	switch s {
	case EnhancedCircuitClosed:
		return "closed"
	case EnhancedCircuitOpen:
		return "open"
	case EnhancedCircuitHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// EnhancedCircuitBreaker 滑动窗口熔断器。
// 在滑动时间窗口内统计失败率，超过阈值时熔断。
type EnhancedCircuitBreaker struct {
	mu sync.RWMutex

	// 配置
	failureThreshold float64       // 失败率阈值 (0.0-1.0)
	windowSize       time.Duration // 滑动窗口大小
	cooldownPeriod   time.Duration // 熔断冷却时间
	halfOpenMaxReqs  int           // 半开状态最大探测请求数

	// 状态
	state        EnhancedCircuitState
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
func NewEnhancedCircuitBreaker(cfg CircuitBreakerConfig) *EnhancedCircuitBreaker {
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

	return &EnhancedCircuitBreaker{
		failureThreshold: cfg.FailureThreshold,
		windowSize:       cfg.WindowSize,
		cooldownPeriod:   cfg.CooldownPeriod,
		halfOpenMaxReqs:  cfg.HalfOpenMaxReqs,
		buckets:          buckets,
		bucketSize:       cfg.WindowSize / time.Duration(cfg.NumBuckets),
		numBuckets:       cfg.NumBuckets,
		state:            EnhancedCircuitClosed,
	}
}

// Execute 执行受熔断器保护的操作。
// 如果熔断器打开，返回 ErrCircuitOpen。
// 如果操作成功，记录成功；失败则记录失败。
func (cb *EnhancedCircuitBreaker) Execute(fn func() error) error {
	cb.mu.Lock()

	// 检查状态
	if cb.state == EnhancedCircuitOpen {
		if time.Since(cb.openedAt) >= cb.cooldownPeriod {
			cb.state = EnhancedCircuitHalfOpen
			cb.halfOpenReqs = 0
		} else {
			cb.mu.Unlock()
			return NewStepErrorWithCode(ErrCircuitOpen, "")
		}
	}
	// 半开状态
	if cb.state == EnhancedCircuitHalfOpen {
		if cb.halfOpenReqs >= cb.halfOpenMaxReqs {
			cb.mu.Unlock()
			return NewStepErrorWithCode(ErrCircuitOpen, "")
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
		if cb.state == EnhancedCircuitClosed || cb.state == EnhancedCircuitHalfOpen {
			if cb.shouldOpen() {
				cb.state = EnhancedCircuitOpen
				cb.openedAt = time.Now()
			}
		}
	} else {
		bucket.successes++

		// 半开状态成功，恢复闭合
		if cb.state == EnhancedCircuitHalfOpen {
			cb.state = EnhancedCircuitClosed
			cb.resetBuckets()
		}
	}

	return err
}

// State 返回当前熔断器状态。
func (cb *EnhancedCircuitBreaker) State() EnhancedCircuitState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// Stats 返回当前统计信息。
func (cb *EnhancedCircuitBreaker) Stats() (successes, failures int64, failureRate float64) {
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
func (cb *EnhancedCircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.state = EnhancedCircuitClosed
	cb.halfOpenReqs = 0
	cb.resetBuckets()
}

// shouldOpen 检查是否应该熔断（需持有锁）。
func (cb *EnhancedCircuitBreaker) shouldOpen() bool {
	_, _, failureRate := cb.Stats()
	return failureRate >= cb.failureThreshold
}

// advanceWindow 推进滑动窗口（需持有锁）。
func (cb *EnhancedCircuitBreaker) advanceWindow() {
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
func (cb *EnhancedCircuitBreaker) advanceWindowUnsafe() {
	// 读锁版本不推进窗口，仅用于统计
}

// resetBuckets 重置所有桶。
func (cb *EnhancedCircuitBreaker) resetBuckets() {
	for i := range cb.buckets {
		cb.buckets[i] = &windowBucket{}
	}
}
