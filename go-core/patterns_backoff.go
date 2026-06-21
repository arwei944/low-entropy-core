// Package core — 退避策略 (v4.0)
//
// 原语归属: L1 Atom（纯计算）
// 所有退避函数均为纯函数，无 I/O，无副作用。
//
// 包含:
//   - ExponentialBackoff: 指数退避（初始 delay × 2^attempt + jitter）
//   - LinearBackoff: 线性退避
//   - ConstantBackoff: 常数退避
//   - CalculateDelay: 策略分发器
package core

import (
	"math"
	"math/rand"
	"time"
)

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
