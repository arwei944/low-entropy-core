package core

import (
	"context"
	"testing"
	"time"
)

// ──────────────────────────────────────────────
// Pipeline Tests
// ──────────────────────────────────────────────

func TestPipeline_BasicExecution(t *testing.T) {
	ctx := context.Background()
	obs := &InMemoryObservationAdapter{}

	pipeline := NewPipeline[int](obs,
		AtomAsStep(Atom[int, int](func(x int) int { return x + 1 })),
		AtomAsStep(Atom[int, int](func(x int) int { return x * 2 })),
		AtomAsStep(Atom[int, int](func(x int) int { return x - 3 })),
	)

	result, steps, err := pipeline.Run(ctx, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// (10 + 1) * 2 - 3 = 19
	if result != 19 {
		t.Errorf("expected 19, got %d", result)
	}
	if len(steps) != 3 {
		t.Errorf("expected 3 steps, got %d", len(steps))
	}
	// Verify observation recorded
	if obs.StepCount() != 3 {
		t.Errorf("expected 3 recorded steps, got %d", obs.StepCount())
	}
}

func TestPipeline_StepError(t *testing.T) {
	ctx := context.Background()
	obs := &InMemoryObservationAdapter{}

	pipeline := NewPipeline[int](obs,
		AtomAsStep(Atom[int, int](func(x int) int { return x + 1 })),
		StepFunc[int, int]{
			execute: func(ctx context.Context, input int) (int, error) {
				return 0, NewStepError("FAIL", "step failed", false)
			},
			unitType: "Failing",
		},
		AtomAsStep(Atom[int, int](func(x int) int { return x * 2 })), // should not execute
	)

	_, steps, err := pipeline.Run(ctx, 10)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	// Should have 2 steps: first succeeded, second failed, third skipped
	if len(steps) != 2 {
		t.Errorf("expected 2 steps (1 success + 1 failure), got %d", len(steps))
	}

	lastStep := steps[len(steps)-1]
	if lastStep.Error == nil {
		t.Error("expected last step to have error")
	}
	if lastStep.Error.Code != "FAIL" {
		t.Errorf("expected error code 'FAIL', got '%s'", lastStep.Error.Code)
	}
}

func TestPipeline_ContextCancellation(t *testing.T) {
	obs := &InMemoryObservationAdapter{}

	pipeline := NewPipeline[int](obs,
		AtomAsStep(Atom[int, int](func(x int) int { return x + 1 })),
		StepFunc[int, int]{
			execute: func(ctx context.Context, input int) (int, error) {
				// Simulate long-running operation
				time.Sleep(100 * time.Millisecond)
				return input, nil
			},
			unitType: "Slow",
		},
	)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, steps, err := pipeline.Run(ctx, 10)
	if err == nil {
		t.Fatal("expected context cancellation error, got nil")
	}
	if len(steps) != 1 {
		t.Errorf("expected 1 error step, got %d", len(steps))
	}
}

func TestPipeline_AddStep(t *testing.T) {
	ctx := context.Background()
	pipeline := NewPipeline[int](nil)
	pipeline.AddStep(AtomAsStep(Atom[int, int](func(x int) int { return x + 5 })))
	pipeline.AddStep(AtomAsStep(Atom[int, int](func(x int) int { return x * 3 })))

	if pipeline.StepCount() != 2 {
		t.Errorf("expected 2 steps, got %d", pipeline.StepCount())
	}

	result, _, err := pipeline.Run(ctx, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// (10 + 5) * 3 = 45
	if result != 45 {
		t.Errorf("expected 45, got %d", result)
	}
}

func TestPipeline_EmptyPipeline(t *testing.T) {
	ctx := context.Background()
	pipeline := NewPipeline[int](nil)

	result, steps, err := pipeline.Run(ctx, 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != 42 {
		t.Errorf("expected 42, got %d", result)
	}
	if len(steps) != 0 {
		t.Errorf("expected 0 steps, got %d", len(steps))
	}
}
