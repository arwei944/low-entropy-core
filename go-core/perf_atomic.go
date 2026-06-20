// Package core — AtomicState (无锁状态机)
package core

import (
	"sync/atomic"
)

// ──────────────────────────────────────────────────────────────────────────────
// AtomicState — 无锁状态机
// ──────────────────────────────────────────────────────────────────────────────

// 状态常量定义。
// 使用 uint64 表示状态值，支持自定义扩展。
const (
	// StateClosed 表示 CircuitBreaker 关闭（正常通行）。
	StateClosed uint64 = iota

	// StateOpen 表示 CircuitBreaker 打开（拒绝请求）。
	StateOpen

	// StateHalfOpen 表示 CircuitBreaker 半开（探测性放行）。
	StateHalfOpen

	// StateNormal 表示 RateLimiter/DegradationManager 正常运行。
	StateNormal

	// StateDegraded 表示降级模式运行。
	StateDegraded

	// StateThrottled 表示限流模式运行。
	StateThrottled
)

// AtomicState 是一个基于 atomic.Uint64 的无锁状态机。
//
// 它提供线程安全的状态管理，无需任何互斥锁。适用于：
//   - CircuitBreaker 的状态转换（Closed / Open / HalfOpen）
//   - RateLimiter 的工作模式（Normal / Throttled）
//   - DegradationManager 的降级状态（Normal / Degraded）
//
// 支持原子性的 CompareAndSwap、Store、Load 操作，
// 以及状态转换追踪（记录上一次状态）。
//
// 零值表示 StateClosed（0），可直接使用。
//
// 使用示例：
//
//	state := &AtomicState{}
//	state.Store(StateNormal)
//	if state.CompareAndSwap(StateNormal, StateDegraded) {
//	    // 成功从 Normal 切换到 Degraded
//	}
type AtomicState struct {
	current atomic.Uint64
	last    atomic.Uint64
}

// Load 原子性地读取当前状态值。
//
// 返回当前状态，不产生任何锁开销。
func (as *AtomicState) Load() uint64 {
	return as.current.Load()
}

// Store 原子性地设置当前状态值，并记录旧状态。
//
// 旧状态被保存到 last 字段中，用于状态追踪和审计。
// 此操作对观察者立即可见。
func (as *AtomicState) Store(state uint64) {
	as.last.Store(as.current.Load())
	as.current.Store(state)
}

// CompareAndSwap 原子性地比较并交换状态。
//
// 如果当前状态等于 old，则将其设置为 new 并返回 true。
// 否则不执行任何操作并返回 false。
//
// 这是实现状态机转换的核心方法，保证状态转换的原子性。
func (as *AtomicState) CompareAndSwap(old, new uint64) bool {
	swapped := as.current.CompareAndSwap(old, new)
	if swapped {
		as.last.Store(old)
	}
	return swapped
}

// LastState 返回上一次状态值。
//
// 用于追踪状态转换历史，辅助诊断和监控。
func (as *AtomicState) LastState() uint64 {
	return as.last.Load()
}

// Transition 执行状态转换并返回是否成功。
//
// 这是一个便捷方法，内部调用 CompareAndSwap。
// 同时返回转换后的新状态（若成功）或当前状态（若失败）。
func (as *AtomicState) Transition(old, new uint64) (current uint64, ok bool) {
	if as.CompareAndSwap(old, new) {
		return new, true
	}
	return as.Load(), false
}
