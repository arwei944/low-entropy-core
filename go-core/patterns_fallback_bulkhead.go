//go:build lecore_tier2 || lecore_tier3 || lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

package core

import (
	"context"
)

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
