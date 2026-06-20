package core

import (
	"context"
	"sync"
	"time"
)

// ============================================================================
// SECTION 10: FanOut — 扇出广播
// ============================================================================

// FanOut 将输入广播到多个 Composer 并行执行，收集所有结果。
// 任一分支失败则整体失败。
type FanOut[T any] struct {
	branches []Composer[T]
	obs      ObservationAdapter
}

// NewFanOut 创建扇出编排器。
func NewFanOut[T any](obs ObservationAdapter, branches ...Composer[T]) *FanOut[T] {
	return &FanOut[T]{branches: branches, obs: obs}
}

func (fo *FanOut[T]) Run(ctx context.Context, input T) (T, []ExecutionStep, error) {
	type branchResult struct {
		steps []ExecutionStep
		err   error
	}
	results := make([]branchResult, len(fo.branches))
	var wg sync.WaitGroup
	for i, b := range fo.branches {
		wg.Add(1)
		go func(idx int, branch Composer[T]) {
			defer wg.Done()
			_, s, e := branch.Run(ctx, input)
			results[idx] = branchResult{s, e}
		}(i, b)
	}
	wg.Wait()

	var allSteps []ExecutionStep
	for _, br := range results {
		allSteps = append(allSteps, br.steps...)
		if br.err != nil {
			return input, allSteps, br.err
		}
	}
	return input, allSteps, nil
}

// ============================================================================
// SECTION 11: Debounce — 防抖
// ============================================================================

// Debounce 防抖编排器：在静默期内忽略重复调用，仅执行最后一次。
type Debounce[T any] struct {
	inner    Composer[T]
	interval time.Duration
	lastCall time.Time
	mu       sync.Mutex
}

// NewDebounce 创建防抖编排器。
func NewDebounce[T any](inner Composer[T], interval time.Duration) *Debounce[T] {
	return &Debounce[T]{inner: inner, interval: interval}
}

func (db *Debounce[T]) Run(ctx context.Context, input T) (T, []ExecutionStep, error) {
	// 原子地检查并更新 lastCall，确保静默期内的调用被跳过。
	db.mu.Lock()
	if time.Since(db.lastCall) < db.interval {
		db.mu.Unlock()
		return input, nil, nil
	}
	db.lastCall = time.Now()
	db.mu.Unlock()
	return db.inner.Run(ctx, input)
}

// ============================================================================
// SECTION 12: Throttle — 节流
// ============================================================================

// Throttle 节流编排器：限制调用频率，确保相邻调用间隔不小于 interval。
type Throttle[T any] struct {
	inner    Composer[T]
	interval time.Duration
	lastCall time.Time
	mu       sync.Mutex
}

// NewThrottle 创建节流编排器。
func NewThrottle[T any](inner Composer[T], interval time.Duration) *Throttle[T] {
	return &Throttle[T]{inner: inner, interval: interval}
}

func (th *Throttle[T]) Run(ctx context.Context, input T) (T, []ExecutionStep, error) {
	th.mu.Lock()
	elapsed := time.Since(th.lastCall)
	if elapsed < th.interval {
		waitTime := th.interval - elapsed
		th.mu.Unlock()
		select {
		case <-time.After(waitTime):
		case <-ctx.Done():
			return input, nil, ctx.Err()
		}
	} else {
		th.mu.Unlock()
	}
	th.mu.Lock()
	th.lastCall = time.Now()
	th.mu.Unlock()
	return th.inner.Run(ctx, input)
}
