//go:build lecore_tier2 || lecore_tier3 || lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

package core

import (
	"context"
	"testing"
	"time"
)

func TestCircuitBreaker_ClosedPasses(t *testing.T) {
	ctx := context.Background()
	obs := &InMemoryObservationAdapter{}

	inner := NewPipeline[int](obs,
		AtomAsStep(Atom[int, int](func(x int) int { return x * 2 })),
	)
	cb := NewCircuitBreaker[int](inner, 3, 100*time.Millisecond)

	result, _, err := cb.Run(ctx, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != 10 {
		t.Errorf("expected 10, got %d", result)
	}
	if cb.State() != CircuitClosed {
		t.Errorf("expected closed, got %s", cb.State())
	}
}

func TestCircuitBreaker_OpensOnThreshold(t *testing.T) {
	ctx := context.Background()
	obs := &InMemoryObservationAdapter{}

	inner := NewPipeline[int](obs,
		StepFunc[int, int]{
			execute: func(ctx context.Context, input int) (int, error) {
				return 0, NewStepError("FAIL", "always fail", true)
			},
			unitType: "Failing",
		},
	)
	cb := NewCircuitBreaker[int](inner, 2, 1*time.Second)

	cb.Run(ctx, 1)
	cb.Run(ctx, 1)

	if cb.State() != CircuitOpen {
		t.Errorf("expected open after 2 failures, got %s", cb.State())
	}

	_, _, err := cb.Run(ctx, 1)
	if err == nil {
		t.Error("expected error when circuit is open")
	}
}

func TestFallback_UsesFallback(t *testing.T) {
	ctx := context.Background()
	obs := &InMemoryObservationAdapter{}

	primary := NewPipeline[int](obs,
		StepFunc[int, int]{
			execute: func(ctx context.Context, input int) (int, error) {
				return 0, NewStepError("FAIL", "primary failed", false)
			},
			unitType: "Failing",
		},
	)
	fallback := NewPipeline[int](obs,
		AtomAsStep(Atom[int, int](func(x int) int { return 999 })),
	)

	fb := NewFallback[int](primary, fallback)
	result, steps, err := fb.Run(ctx, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != 999 {
		t.Errorf("expected 999 (fallback), got %d", result)
	}
	if len(steps) == 0 {
		t.Error("expected steps to be collected")
	}
}

func TestFallback_WhenPrimaryFails(t *testing.T) {
	ctx := context.Background()
	obs := &InMemoryObservationAdapter{}

	primary := NewPipeline[int](obs,
		AtomAsStep(Atom[int, int](func(x int) int { return x * 2 })),
	)
	fallback := NewPipeline[int](obs,
		AtomAsStep(Atom[int, int](func(x int) int { return 999 })),
	)

	fb := NewFallback[int](primary, fallback)
	result, _, err := fb.Run(ctx, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != 10 {
		t.Errorf("expected 10 (primary), got %d", result)
	}
}

func TestBulkhead_RejectsWhenFull(t *testing.T) {
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

	go bh.Run(ctx, 1)
	time.Sleep(10 * time.Millisecond)

	_, _, err := bh.Run(ctx, 2)
	if err == nil {
		t.Error("expected bulkhead rejection")
	}
}

func TestResilienceChain_WrapsCorrectly(t *testing.T) {
	ctx := context.Background()
	obs := &InMemoryObservationAdapter{}

	inner := NewPipeline[int](obs,
		AtomAsStep(Atom[int, int](func(x int) int { return x * 2 })),
	)

	chain := ResilienceChain[int](inner, ResilienceConfig[int]{
		RateLimit:                100,
		RateLimitBurst:           200,
		BulkheadMax:              10,
		CircuitBreakerThreshold:  3,
		CircuitBreakerCooldown:   1 * time.Second,
	})

	result, _, err := chain.Run(ctx, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != 10 {
		t.Errorf("expected 10, got %d", result)
	}
}