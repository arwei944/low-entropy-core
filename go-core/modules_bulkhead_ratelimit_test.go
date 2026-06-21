//go:build lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

package core

import (
	"context"
	"testing"
	"time"
)

// ──────────────────────────────────────────────
// Bulkhead Tests
// ──────────────────────────────────────────────

func TestBulkhead_WithinLimit(t *testing.T) {
	ctx := context.Background()
	obs := &InMemoryObservationAdapter{}

	inner := NewPipeline[int](obs,
		AtomAsStep(Atom[int, int](func(x int) int { return x * 2 })),
	)
	bh := NewBulkhead[int](inner, 5)

	result, _, err := bh.Run(ctx, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != 20 {
		t.Errorf("expected 20, got %d", result)
	}
}

func TestBulkhead_Exceeded(t *testing.T) {
	ctx := context.Background()
	obs := &InMemoryObservationAdapter{}

	inner := NewPipeline[int](obs,
		StepFunc[int, int]{
			execute: func(ctx context.Context, input int) (int, error) {
				time.Sleep(100 * time.Millisecond)
				return input, nil
			},
			unitType: "Slow",
		},
	)
	bh := NewBulkhead[int](inner, 1)

	// Fill the only slot
	go bh.Run(ctx, 1)
	time.Sleep(10 * time.Millisecond) // Let goroutine start

	// This should be rejected
	_, _, err := bh.Run(ctx, 2)
	if err == nil {
		t.Fatal("expected BULKHEAD_FULL error")
	}
}

func TestBulkhead_Available(t *testing.T) {
	inner := NewPipeline[int](nil, AtomAsStep(Atom[int, int](func(x int) int { return x })))
	bh := NewBulkhead[int](inner, 3)

	if bh.Available() != 3 {
		t.Errorf("expected 3 available, got %d", bh.Available())
	}
}

// ──────────────────────────────────────────────
// RateLimiter Tests
// ──────────────────────────────────────────────

func TestRateLimiter_WithinLimit(t *testing.T) {
	ctx := context.Background()
	obs := &InMemoryObservationAdapter{}

	inner := NewPipeline[int](obs,
		AtomAsStep(Atom[int, int](func(x int) int { return x * 2 })),
	)
	rl := NewRateLimiter[int](inner, 100, 200)

	result, _, err := rl.Run(ctx, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != 20 {
		t.Errorf("expected 20, got %d", result)
	}
}

func TestRateLimiter_Exceeded(t *testing.T) {
	ctx := context.Background()
	obs := &InMemoryObservationAdapter{}

	inner := NewPipeline[int](obs,
		AtomAsStep(Atom[int, int](func(x int) int { return x })),
	)
	rl := NewRateLimiter[int](inner, 1, 1) // 1 token per second

	// Use the only token
	rl.Run(ctx, 1)

	// Second call should be rate limited
	_, _, err := rl.Run(ctx, 2)
	if err == nil {
		t.Fatal("expected RATE_LIMITED error")
	}
}

func TestRateLimiter_Tokens(t *testing.T) {
	inner := NewPipeline[int](nil, AtomAsStep(Atom[int, int](func(x int) int { return x })))
	rl := NewRateLimiter[int](inner, 10, 20)
	tokens := rl.Tokens()
	if tokens < 19 || tokens > 20 {
		t.Errorf("expected ~20 tokens, got %f", tokens)
	}
}
