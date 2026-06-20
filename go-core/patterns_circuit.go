//go:build lecore_tier2 || lecore_tier3 || lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

package core

import (
	"context"
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
