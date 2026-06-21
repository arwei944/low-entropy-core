package core

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"
)

// ──────────────────────────────────────────────
// Branch Tests
// ──────────────────────────────────────────────

func TestBranch_TruePath(t *testing.T) {
	ctx := context.Background()
	obs := &InMemoryObservationAdapter{}

	truePath := NewPipeline[int](obs,
		AtomAsStep(Atom[int, int](func(x int) int { return x * 10 })),
	)
	falsePath := NewPipeline[int](obs,
		AtomAsStep(Atom[int, int](func(x int) int { return x * -1 })),
	)

	branch := NewBranch[int](
		func(x int) bool { return x > 0 },
		truePath,
		falsePath,
	)

	result, steps, err := branch.Run(ctx, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// 5 > 0 → true path: 5 * 10 = 50
	if result != 50 {
		t.Errorf("expected 50, got %d", result)
	}
	// 验证子步骤被收集
	if len(steps) == 0 {
		t.Error("expected child steps to be collected, got 0")
	}
}

func TestBranch_FalsePath(t *testing.T) {
	ctx := context.Background()
	obs := &InMemoryObservationAdapter{}

	truePath := NewPipeline[int](obs,
		AtomAsStep(Atom[int, int](func(x int) int { return x * 10 })),
	)
	falsePath := NewPipeline[int](obs,
		AtomAsStep(Atom[int, int](func(x int) int { return x * -1 })),
	)

	branch := NewBranch[int](
		func(x int) bool { return x > 0 },
		truePath,
		falsePath,
	)

	result, steps, err := branch.Run(ctx, -5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// -5 <= 0 → false path: -5 * -1 = 5
	if result != 5 {
		t.Errorf("expected 5, got %d", result)
	}
	if len(steps) == 0 {
		t.Error("expected child steps to be collected, got 0")
	}
}

func TestBranch_ErrorPropagation(t *testing.T) {
	ctx := context.Background()
	obs := &InMemoryObservationAdapter{}

	failingPath := NewPipeline[int](obs,
		StepFunc[int, int]{
			execute: func(ctx context.Context, input int) (int, error) {
				return 0, NewStepError("BRANCH_FAIL", "branch failed", false)
			},
			unitType: "Failing",
		},
	)
	okPath := NewPipeline[int](obs,
		AtomAsStep(Atom[int, int](func(x int) int { return x + 1 })),
	)

	branch := NewBranch[int](
		func(x int) bool { return x > 0 },
		failingPath,
		okPath,
	)

	_, _, err := branch.Run(ctx, 10)
	if err == nil {
		t.Fatal("expected error from true path, got nil")
	}
}

// ──────────────────────────────────────────────
// Parallel Tests
// ──────────────────────────────────────────────

func TestParallel_Basic(t *testing.T) {
	ctx := context.Background()
	obs := &InMemoryObservationAdapter{}

	comp1 := NewPipeline[int](obs,
		AtomAsStep(Atom[int, int](func(x int) int { return x * 2 })),
	)
	comp2 := NewPipeline[int](obs,
		AtomAsStep(Atom[int, int](func(x int) int { return x + 10 })),
	)
	comp3 := NewPipeline[int](obs,
		AtomAsStep(Atom[int, int](func(x int) int { return x - 5 })),
	)

	results, _, err := RunParallel(ctx, 10, comp1, comp2, comp3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results.Results) != 3 {
		t.Errorf("expected 3 results, got %d", len(results.Results))
	}
	if results.Results[0] != 20 {
		t.Errorf("expected 20, got %d", results.Results[0])
	}
	if results.Results[1] != 20 {
		t.Errorf("expected 20, got %d", results.Results[1])
	}
	if results.Results[2] != 5 {
		t.Errorf("expected 5, got %d", results.Results[2])
	}
}

func TestParallel_Concurrency(t *testing.T) {
	// Verify that Parallel actually runs concurrently
	var counter int32
	ctx := context.Background()
	obs := &InMemoryObservationAdapter{}

	// Each composer increments a shared counter with a delay
	makeComp := func(id int) Composer[int] {
		return NewPipeline[int](obs,
			StepFunc[int, int]{
				execute: func(ctx context.Context, input int) (int, error) {
					// Small delay to ensure interleaving
					time.Sleep(10 * time.Millisecond)
					atomic.AddInt32(&counter, 1)
					return id, nil
				},
				unitType: fmt.Sprintf("Worker-%d", id),
			},
		)
	}

	composers := make([]Composer[int], 5)
	for i := 0; i < 5; i++ {
		composers[i] = makeComp(i)
	}

	start := time.Now()
	results, _, err := RunParallel(ctx, 0, composers...)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results.Results) != 5 {
		t.Errorf("expected 5 results, got %d", len(results.Results))
	}
	if atomic.LoadInt32(&counter) != 5 {
		t.Errorf("expected counter=5, got %d", counter)
	}
	// If truly parallel, should complete in ~10ms, not ~50ms
	if elapsed > 100*time.Millisecond {
		t.Errorf("execution took %v, expected <100ms (parallel)", elapsed)
	}
}

func TestParallel_ErrorCollection(t *testing.T) {
	ctx := context.Background()
	obs := &InMemoryObservationAdapter{}

	okComp := NewPipeline[int](obs,
		AtomAsStep(Atom[int, int](func(x int) int { return x + 1 })),
	)
	failComp := NewPipeline[int](obs,
		StepFunc[int, int]{
			execute: func(ctx context.Context, input int) (int, error) {
				return 0, errors.New("parallel failure")
			},
			unitType: "Failing",
		},
	)

	results, _, err := RunParallel(ctx, 10, okComp, failComp)
	if err == nil {
		t.Fatal("expected PARALLEL_ERROR, got nil")
	}
	if len(results.Results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results.Results))
	}
	if results.Errors[0] != nil {
		t.Errorf("expected first error to be nil, got %v", results.Errors[0])
	}
	if results.Errors[1] == nil {
		t.Error("expected second error to be non-nil")
	}
}

func TestParallel_Empty(t *testing.T) {
	ctx := context.Background()
	results, steps, err := RunParallel[int](ctx, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results.Results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results.Results))
	}
	if steps != nil {
		t.Error("expected nil steps for empty parallel")
	}
}
