package core

import (
	"context"
	"math"
	"sync"
	"time"
)

// ============================================================================
// SECTION 4: Parallel — 并行执行
// ============================================================================

type ParallelResults[T any] struct {
	Results []T
	Errors  []error
	Steps   [][]ExecutionStep
}

func RunParallel[T any](ctx context.Context, input T, composers ...Composer[T]) (ParallelResults[T], []ExecutionStep, error) {
	if len(composers) == 0 {
		return ParallelResults[T]{}, nil, nil
	}

	type result struct {
		index int
		value T
		steps []ExecutionStep
		err   error
	}

	resultCh := make(chan result, len(composers))
	var wg sync.WaitGroup

	for i, c := range composers {
		wg.Add(1)
		go func(idx int, comp Composer[T]) {
			defer wg.Done()
			val, steps, err := comp.Run(ctx, input)
			resultCh <- result{index: idx, value: val, steps: steps, err: err}
		}(i, c)
	}

	go func() {
		wg.Wait()
		close(resultCh)
	}()

	results := make([]T, len(composers))
	errs := make([]error, len(composers))
	allSteps := make([][]ExecutionStep, len(composers))
	hasError := false

	for r := range resultCh {
		results[r.index] = r.value
		errs[r.index] = r.err
		allSteps[r.index] = r.steps
		if r.err != nil {
			hasError = true
		}
	}

	flatSteps := make([]ExecutionStep, 0)
	for _, s := range allSteps {
		flatSteps = append(flatSteps, s...)
	}

	var finalErr error
	if hasError {
		finalErr = &StepError{Code: "PARALLEL_ERROR", Message: "one or more parallel branches failed", Recoverable: false}
	}

	return ParallelResults[T]{Results: results, Errors: errs, Steps: allSteps}, flatSteps, finalErr
}

// ============================================================================
// SECTION 5: WithRetry — 指数退避重试
// ============================================================================

type RetryConfig struct {
	MaxAttempts int
	BaseDelay   time.Duration
	MaxDelay    time.Duration
	Multiplier  float64
}

func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts: 3,
		BaseDelay:   100 * time.Millisecond,
		MaxDelay:    10 * time.Second,
		Multiplier:  2.0,
	}
}

func WithRetry[T any](comp Composer[T], config RetryConfig) Composer[T] {
	return &retryComposer[T]{inner: comp, config: config}
}

type retryComposer[T any] struct {
	inner  Composer[T]
	config RetryConfig
}

func (r *retryComposer[T]) Run(ctx context.Context, input T) (T, []ExecutionStep, error) {
	allSteps := make([]ExecutionStep, 0)
	var lastErr error

	for attempt := 0; attempt < r.config.MaxAttempts; attempt++ {
		select {
		case <-ctx.Done():
			return input, allSteps, ctx.Err()
		default:
		}

		result, steps, err := r.inner.Run(ctx, input)
		allSteps = append(allSteps, steps...)

		if err == nil {
			return result, allSteps, nil
		}

		lastErr = err

		if se, ok := err.(*StepError); ok && !se.Recoverable {
			return result, allSteps, err
		}

		if attempt < r.config.MaxAttempts-1 {
			delay := r.computeDelay(attempt)
			select {
			case <-ctx.Done():
				return result, allSteps, ctx.Err()
			case <-time.After(delay):
			}
		}
	}

	var zero T
	return zero, allSteps, lastErr
}

func (r *retryComposer[T]) computeDelay(attempt int) time.Duration {
	delay := float64(r.config.BaseDelay) * math.Pow(r.config.Multiplier, float64(attempt))
	if delay > float64(r.config.MaxDelay) {
		delay = float64(r.config.MaxDelay)
	}
	return time.Duration(delay)
}

// ============================================================================
// SECTION 6: WithTimeout — 超时
// ============================================================================

func WithTimeout[T any](comp Composer[T], timeout time.Duration) Composer[T] {
	return &timeoutComposer[T]{inner: comp, timeout: timeout}
}

type timeoutComposer[T any] struct {
	inner   Composer[T]
	timeout time.Duration
}

func (t *timeoutComposer[T]) Run(ctx context.Context, input T) (T, []ExecutionStep, error) {
	ctx, cancel := context.WithTimeout(ctx, t.timeout)
	defer cancel()

	result, steps, err := t.inner.Run(ctx, input)

	if ctx.Err() != nil {
		se := &StepError{
			Code:        "TIMEOUT",
			Message:     "operation timed out after " + t.timeout.String(),
			Recoverable: true,
		}
		return result, steps, se
	}

	return result, steps, err
}
