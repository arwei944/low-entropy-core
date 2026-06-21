//go:build lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

package core

import (
	"context"
	"testing"
	"time"
)

// ──────────────────────────────────────────────
// CircuitBreaker Tests
// ──────────────────────────────────────────────

func TestCircuitBreaker_ClosedToOpen(t *testing.T) {
	ctx := context.Background()
	obs := &InMemoryObservationAdapter{}

	failCount := 0
	inner := NewPipeline[int](obs,
		StepFunc[int, int]{
			execute: func(ctx context.Context, input int) (int, error) {
				failCount++
				return 0, NewStepError("FAIL", "always fails", true)
			},
			unitType: "Failing",
		},
	)

	cb := NewCircuitBreaker[int](inner, 3, 100*time.Millisecond)

	// First 3 failures should transition to open
	for i := 0; i < 3; i++ {
		_, _, err := cb.Run(ctx, 1)
		if err == nil {
			t.Fatal("expected error")
		}
	}

	if cb.State() != CircuitOpen {
		t.Errorf("expected CircuitOpen, got %s", cb.State())
	}

	// 4th call should fail immediately with CIRCUIT_OPEN
	_, _, err := cb.Run(ctx, 1)
	if err == nil {
		t.Fatal("expected CIRCUIT_OPEN error")
	}
	if se, ok := err.(*StepError); !ok || se.Code != "CIRCUIT_OPEN" {
		t.Errorf("expected CIRCUIT_OPEN, got %v", err)
	}
}

func TestCircuitBreaker_HalfOpenRecovery(t *testing.T) {
	ctx := context.Background()
	obs := &InMemoryObservationAdapter{}

	failCount := 0
	inner := NewPipeline[int](obs,
		StepFunc[int, int]{
			execute: func(ctx context.Context, input int) (int, error) {
				failCount++
				if failCount <= 3 {
					return 0, NewStepError("FAIL", "fail", true)
				}
				return input * 2, nil
			},
			unitType: "Flaky",
		},
	)

	cb := NewCircuitBreaker[int](inner, 3, 50*time.Millisecond)

	// Fail 3 times to open the circuit
	for i := 0; i < 3; i++ {
		cb.Run(ctx, 1)
	}

	// Wait for half-open transition
	time.Sleep(100 * time.Millisecond)

	// Next call should succeed (half-open → closed)
	result, _, err := cb.Run(ctx, 5)
	if err != nil {
		t.Fatalf("expected recovery, got error: %v", err)
	}
	if result != 10 {
		t.Errorf("expected 10, got %d", result)
	}
	if cb.State() != CircuitClosed {
		t.Errorf("expected CircuitClosed after recovery, got %s", cb.State())
	}
}

// ──────────────────────────────────────────────
// Fallback Tests
// ──────────────────────────────────────────────

func TestFallback_PrimarySucceeds(t *testing.T) {
	ctx := context.Background()
	obs := &InMemoryObservationAdapter{}

	primary := NewPipeline[int](obs,
		AtomAsStep(Atom[int, int](func(x int) int { return x * 2 })),
	)
	fallback := NewPipeline[int](obs,
		AtomAsStep(Atom[int, int](func(x int) int { return x * -1 })),
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

func TestFallback_PrimaryFails(t *testing.T) {
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
	result, _, err := fb.Run(ctx, 5)
	if err != nil {
		t.Fatalf("fallback should not error: %v", err)
	}
	if result != 999 {
		t.Errorf("expected 999 (fallback), got %d", result)
	}
}
