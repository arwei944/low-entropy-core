package core

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// ──────────────────────────────────────────────
// Types Tests
// ──────────────────────────────────────────────

func TestStepError(t *testing.T) {
	err := NewStepError("TEST_ERR", "test message", true)
	if err.Code != "TEST_ERR" {
		t.Errorf("expected Code='TEST_ERR', got '%s'", err.Code)
	}
	if err.Message != "test message" {
		t.Errorf("expected Message='test message', got '%s'", err.Message)
	}
	if !err.Recoverable {
		t.Error("expected Recoverable=true")
	}
	if err.Error() != "TEST_ERR: test message" {
		t.Errorf("expected Error()='TEST_ERR: test message', got '%s'", err.Error())
	}
}

func TestNewTraceID_Uniqueness(t *testing.T) {
	ids := make(map[TraceID]bool, 1000)
	for i := 0; i < 1000; i++ {
		id := NewTraceID()
		if ids[id] {
			t.Errorf("duplicate TraceID generated: %s", id)
		}
		ids[id] = true
	}
}

func TestNewSpanID_Uniqueness(t *testing.T) {
	ids := make(map[SpanID]bool, 1000)
	for i := 0; i < 1000; i++ {
		id := NewSpanID()
		if ids[id] {
			t.Errorf("duplicate SpanID generated: %s", id)
		}
		ids[id] = true
	}
}

func TestAtomType(t *testing.T) {
	// Verify Atom works as a typed function
	double := func(x int) int { return x * 2 }
	var a Atom[int, int] = double
	result := a(21)
	if result != 42 {
		t.Errorf("expected 42, got %d", result)
	}
}

func TestAtomAny(t *testing.T) {
	// Verify AtomAny compatibility
	var a AtomAny = func(x any) any { return x }
	result := a("hello")
	if result != "hello" {
		t.Errorf("expected 'hello', got %v", result)
	}
}

// ──────────────────────────────────────────────
// Step Tests
// ──────────────────────────────────────────────

func TestStepFunc(t *testing.T) {
	ctx := context.Background()
	step := NewStepFunc[int, int]("Test", func(ctx context.Context, input int) (int, error) {
		return input * 2, nil
	})

	result, err := step.Execute(ctx, 21)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != 42 {
		t.Errorf("expected 42, got %d", result)
	}
	if step.UnitType() != "Test" {
		t.Errorf("expected UnitType='Test', got '%s'", step.UnitType())
	}
}

func TestStepFunc_Error(t *testing.T) {
	ctx := context.Background()
	expectedErr := errors.New("simulated failure")
	step := NewStepFunc[int, int]("Failing", func(ctx context.Context, input int) (int, error) {
		return 0, expectedErr
	})

	_, err := step.Execute(ctx, 1)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err != expectedErr {
		t.Errorf("expected %v, got %v", expectedErr, err)
	}
}

func TestAtomAsStep(t *testing.T) {
	ctx := context.Background()
	atom := func(x string) string { return x + " world" }
	step := AtomAsStep(Atom[string, string](atom))

	result, err := step.Execute(ctx, "hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "hello world" {
		t.Errorf("expected 'hello world', got '%s'", result)
	}
	if step.UnitType() != "Atom" {
		t.Errorf("expected UnitType='Atom', got '%s'", step.UnitType())
	}
}

func TestPortAsStep(t *testing.T) {
	ctx := context.Background()
	port := NewPort[int, int](func(ctx context.Context, input int) (int, error) {
		if input < 0 {
			return 0, NewStepError("NEGATIVE", "negative input", false)
		}
		return input * 10, nil
	})
	step := PortAsStep(port)

	result, err := step.Execute(ctx, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != 50 {
		t.Errorf("expected 50, got %d", result)
	}

	_, err = step.Execute(ctx, -1)
	if err == nil {
		t.Fatal("expected error for negative input, got nil")
	}
}

func TestAdapterAsStep(t *testing.T) {
	ctx := context.Background()
	adapter := NewAdapter[string, string](func(ctx context.Context, input string) (string, error) {
		return "[" + input + "]", nil
	})
	step := AdapterAsStep(adapter)

	result, err := step.Execute(ctx, "data")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "[data]" {
		t.Errorf("expected '[data]', got '%s'", result)
	}
	if step.UnitType() != "Adapter" {
		t.Errorf("expected UnitType='Adapter', got '%s'", step.UnitType())
	}
}

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

	pipeline := NewPipeline[int](obs, branch)
	result, _, err := pipeline.Run(ctx, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// 5 > 0 → true path: 5 * 10 = 50
	if result != 50 {
		t.Errorf("expected 50, got %d", result)
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

	pipeline := NewPipeline[int](obs, branch)
	result, _, err := pipeline.Run(ctx, -5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// -5 <= 0 → false path: -5 * -1 = 5
	if result != 5 {
		t.Errorf("expected 5, got %d", result)
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

	pipeline := NewPipeline[int](obs, branch)
	_, _, err := pipeline.Run(ctx, 10)
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

// ──────────────────────────────────────────────
// WithRetry Tests
// ──────────────────────────────────────────────

func TestWithRetry_Success(t *testing.T) {
	ctx := context.Background()
	obs := &InMemoryObservationAdapter{}

	var attempts int32
	comp := NewPipeline[int](obs,
		StepFunc[int, int]{
			execute: func(ctx context.Context, input int) (int, error) {
				count := atomic.AddInt32(&attempts, 1)
				if count < 3 {
					return 0, NewStepError("RETRY", "temporary failure", true)
				}
				return input * 10, nil
			},
			unitType: "Flaky",
		},
	)

	retryComp := WithRetry[int](comp, RetryConfig{
		MaxAttempts: 5,
		BaseDelay:   1 * time.Millisecond,
		MaxDelay:    10 * time.Millisecond,
		Multiplier:  2.0,
	})

	result, _, err := retryComp.Run(ctx, 5)
	if err != nil {
		t.Fatalf("unexpected error after retries: %v", err)
	}
	if result != 50 {
		t.Errorf("expected 50, got %d", result)
	}
	if atomic.LoadInt32(&attempts) != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
}

func TestWithRetry_MaxAttempts(t *testing.T) {
	ctx := context.Background()
	obs := &InMemoryObservationAdapter{}

	var attempts int32
	comp := NewPipeline[int](obs,
		StepFunc[int, int]{
			execute: func(ctx context.Context, input int) (int, error) {
				atomic.AddInt32(&attempts, 1)
				return 0, NewStepError("PERSISTENT", "always fails", true)
			},
			unitType: "AlwaysFailing",
		},
	)

	retryComp := WithRetry[int](comp, RetryConfig{
		MaxAttempts: 3,
		BaseDelay:   1 * time.Millisecond,
		MaxDelay:    10 * time.Millisecond,
		Multiplier:  2.0,
	})

	_, _, err := retryComp.Run(ctx, 5)
	if err == nil {
		t.Fatal("expected error after max attempts, got nil")
	}
	if atomic.LoadInt32(&attempts) != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
}

func TestWithRetry_NonRecoverable(t *testing.T) {
	ctx := context.Background()
	obs := &InMemoryObservationAdapter{}

	var attempts int32
	comp := NewPipeline[int](obs,
		StepFunc[int, int]{
			execute: func(ctx context.Context, input int) (int, error) {
				atomic.AddInt32(&attempts, 1)
				return 0, NewStepError("FATAL", "non-recoverable", false)
			},
			unitType: "Fatal",
		},
	)

	retryComp := WithRetry[int](comp, RetryConfig{
		MaxAttempts: 5,
		BaseDelay:   1 * time.Millisecond,
		MaxDelay:    10 * time.Millisecond,
		Multiplier:  2.0,
	})

	_, _, err := retryComp.Run(ctx, 5)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	// Non-recoverable → should NOT retry, only 1 attempt
	if atomic.LoadInt32(&attempts) != 1 {
		t.Errorf("expected 1 attempt (non-recoverable), got %d", attempts)
	}
}

func TestWithRetry_ContextCancellation(t *testing.T) {
	obs := &InMemoryObservationAdapter{}

	comp := NewPipeline[int](obs,
		StepFunc[int, int]{
			execute: func(ctx context.Context, input int) (int, error) {
				return 0, NewStepError("RETRY", "fail", true)
			},
			unitType: "Failing",
		},
	)

	retryComp := WithRetry[int](comp, RetryConfig{
		MaxAttempts: 10,
		BaseDelay:   100 * time.Millisecond,
		MaxDelay:    1 * time.Second,
		Multiplier:  2.0,
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, _, err := retryComp.Run(ctx, 5)
	if err == nil {
		t.Fatal("expected context cancellation error, got nil")
	}
}

func TestDefaultRetryConfig(t *testing.T) {
	config := DefaultRetryConfig()
	if config.MaxAttempts != 3 {
		t.Errorf("expected MaxAttempts=3, got %d", config.MaxAttempts)
	}
	if config.BaseDelay != 100*time.Millisecond {
		t.Errorf("expected BaseDelay=100ms, got %v", config.BaseDelay)
	}
	if config.Multiplier != 2.0 {
		t.Errorf("expected Multiplier=2.0, got %f", config.Multiplier)
	}
}

// ──────────────────────────────────────────────
// WithTimeout Tests
// ──────────────────────────────────────────────

func TestWithTimeout_WithinDeadline(t *testing.T) {
	ctx := context.Background()
	obs := &InMemoryObservationAdapter{}

	comp := NewPipeline[int](obs,
		AtomAsStep(Atom[int, int](func(x int) int { return x * 2 })),
	)

	timeoutComp := WithTimeout[int](comp, 1*time.Second)
	result, _, err := timeoutComp.Run(ctx, 21)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != 42 {
		t.Errorf("expected 42, got %d", result)
	}
}

func TestWithTimeout_Exceeded(t *testing.T) {
	obs := &InMemoryObservationAdapter{}

	comp := NewPipeline[int](obs,
		StepFunc[int, int]{
			execute: func(ctx context.Context, input int) (int, error) {
				time.Sleep(200 * time.Millisecond)
				return input, nil
			},
			unitType: "Slow",
		},
	)

	timeoutComp := WithTimeout[int](comp, 10*time.Millisecond)
	_, _, err := timeoutComp.Run(context.Background(), 10)
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	se, ok := err.(*StepError)
	if !ok {
		t.Fatalf("expected StepError, got %T", err)
	}
	if se.Code != "TIMEOUT" {
		t.Errorf("expected TIMEOUT code, got '%s'", se.Code)
	}
	if !se.Recoverable {
		t.Error("expected timeout to be recoverable")
	}
}

// ──────────────────────────────────────────────
// Observation Tests
// ──────────────────────────────────────────────

func TestInMemoryObservationAdapter_Record(t *testing.T) {
	obs := &InMemoryObservationAdapter{}
	step := NewExecutionStep("Atom", "test", "testing", "")

	obs.Record([]ExecutionStep{step})
	if obs.StepCount() != 1 {
		t.Errorf("expected 1 step, got %d", obs.StepCount())
	}
}

func TestInMemoryObservationAdapter_Concurrency(t *testing.T) {
	obs := &InMemoryObservationAdapter{}
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			obs.Record([]ExecutionStep{NewExecutionStep("Test", "concurrent", "testing", "")})
		}()
	}

	wg.Wait()
	if obs.StepCount() != 100 {
		t.Errorf("expected 100 steps, got %d", obs.StepCount())
	}
}

func TestInMemoryObservationAdapter_Clear(t *testing.T) {
	obs := &InMemoryObservationAdapter{}
	obs.Record([]ExecutionStep{NewExecutionStep("Test", "clear", "test", "")})
	obs.Clear()
	if obs.StepCount() != 0 {
		t.Errorf("expected 0 steps after clear, got %d", obs.StepCount())
	}
}

func TestInMemoryObservationAdapter_GetSteps_Copy(t *testing.T) {
	obs := &InMemoryObservationAdapter{}
	obs.Record([]ExecutionStep{NewExecutionStep("Test", "copy", "test", "")})

	steps := obs.GetSteps()
	// Modify the copy
	steps[0].Unit = "Modified"

	// Original should be unchanged
	original := obs.GetSteps()
	if original[0].Unit == "Modified" {
		t.Error("GetSteps should return a copy, not a reference")
	}
}

func TestTraceTree_Build(t *testing.T) {
	// Create a parent-child relationship
	parentSpan := string(NewSpanID())
	childSpan := string(NewSpanID())

	steps := []ExecutionStep{
		{SpanID: parentSpan, Unit: "Composer", Action: "Run", TraceID: "trace-1"},
		{SpanID: childSpan, ParentID: parentSpan, Unit: "Atom", Action: "Exec", TraceID: "trace-1"},
		{SpanID: string(NewSpanID()), Unit: "Adapter", Action: "Out", TraceID: "trace-1"}, // orphan
	}

	tree := BuildTraceTree(steps)
	if tree.TotalNodes() != 3 {
		t.Errorf("expected 3 nodes, got %d", tree.TotalNodes())
	}

	// Parent should have 1 child
	if len(tree.Roots) != 2 {
		t.Errorf("expected 2 roots (parent + orphan), got %d", len(tree.Roots))
	}

	// Find the parent root
	var parentRoot *TraceNode
	for _, root := range tree.Roots {
		if root.Step.SpanID == parentSpan {
			parentRoot = root
			break
		}
	}
	if parentRoot == nil {
		t.Fatal("parent root not found")
	}
	if len(parentRoot.Children) != 1 {
		t.Errorf("expected 1 child, got %d", len(parentRoot.Children))
	}
	if parentRoot.Children[0].Step.SpanID != childSpan {
		t.Errorf("expected child span %s, got %s", childSpan, parentRoot.Children[0].Step.SpanID)
	}
	if parentRoot.Children[0].Depth != 1 {
		t.Errorf("expected child depth=1, got %d", parentRoot.Children[0].Depth)
	}
}

func TestTraceTree_Empty(t *testing.T) {
	tree := BuildTraceTree([]ExecutionStep{})
	if tree.TotalNodes() != 0 {
		t.Errorf("expected 0 nodes, got %d", tree.TotalNodes())
	}
	if len(tree.Roots) != 0 {
		t.Errorf("expected 0 roots, got %d", len(tree.Roots))
	}
}

func TestTraceTree_Flatten(t *testing.T) {
	parentSpan := string(NewSpanID())
	steps := []ExecutionStep{
		{SpanID: parentSpan, Unit: "Root", Action: "A", TraceID: "t1"},
		{SpanID: string(NewSpanID()), ParentID: parentSpan, Unit: "Child", Action: "B", TraceID: "t1"},
		{SpanID: string(NewSpanID()), ParentID: parentSpan, Unit: "Child", Action: "C", TraceID: "t1"},
	}

	tree := BuildTraceTree(steps)
	flat := tree.Flatten()

	if len(flat) != 3 {
		t.Errorf("expected 3 nodes in flatten, got %d", len(flat))
	}
	// DFS order: root, child1, child2
	if flat[0].Step.Action != "A" {
		t.Errorf("expected first='A', got '%s'", flat[0].Step.Action)
	}
}

func TestNoOpObservationAdapter(t *testing.T) {
	obs := &NoOpObservationAdapter{}
	// Should not panic
	obs.Record([]ExecutionStep{NewExecutionStep("Test", "noop", "test", "")})
	// No assertion needed — just verify no panic
}

func TestNewExecutionStep_HasUUIDs(t *testing.T) {
	step := NewExecutionStep("Atom", "test", "testing", "TestPattern")
	if step.TraceID == "" {
		t.Error("expected non-empty TraceID")
	}
	if step.SpanID == "" {
		t.Error("expected non-empty SpanID")
	}
	if step.Unit != "Atom" {
		t.Errorf("expected Unit='Atom', got '%s'", step.Unit)
	}
	if step.Pattern != "TestPattern" {
		t.Errorf("expected Pattern='TestPattern', got '%s'", step.Pattern)
	}
}

func TestNewExecutionStepWithTrace(t *testing.T) {
	step := NewExecutionStepWithTrace("parent-123", "Port", "validate", "test", "")
	if step.ParentID != "parent-123" {
		t.Errorf("expected ParentID='parent-123', got '%s'", step.ParentID)
	}
}

func TestNewExecutionStepWithError(t *testing.T) {
	err := NewStepError("E001", "something wrong", false)
	step := NewExecutionStepWithError("Adapter", "call", "external call", "", err)
	if step.Error == nil {
		t.Fatal("expected error in step")
	}
	if step.Error.Code != "E001" {
		t.Errorf("expected error code 'E001', got '%s'", step.Error.Code)
	}
}

func TestTraceContext(t *testing.T) {
	ctx := context.Background()
	tc := TraceContext{TraceID: "trace-abc", SpanID: "span-xyz"}

	ctx = WithTraceContext(ctx, tc)
	got, ok := GetTraceContext(ctx)
	if !ok {
		t.Fatal("expected TraceContext in context")
	}
	if got.TraceID != "trace-abc" {
		t.Errorf("expected TraceID='trace-abc', got '%s'", got.TraceID)
	}
	if got.SpanID != "span-xyz" {
		t.Errorf("expected SpanID='span-xyz', got '%s'", got.SpanID)
	}
}

func TestTraceContext_Missing(t *testing.T) {
	ctx := context.Background()
	_, ok := GetTraceContext(ctx)
	if ok {
		t.Error("expected no TraceContext in empty context")
	}
}

// ──────────────────────────────────────────────
// Integration Tests
// ──────────────────────────────────────────────

func TestIntegration_FullPipeline(t *testing.T) {
	// Simulate a real-world pipeline: Port → Atom → Adapter
	ctx := context.Background()
	obs := &InMemoryObservationAdapter{}

	type Order struct {
		ID     string
		Amount float64
		Status string
	}

	// Port: validate
	validatePort := NewPort[Order, Order](func(ctx context.Context, o Order) (Order, error) {
		if o.Amount <= 0 {
			return o, NewStepError("INVALID_AMOUNT", "amount must be positive", false)
		}
		return o, nil
	})

	// Atom: apply discount
	applyDiscount := func(o Order) Order {
		if o.Amount > 100 {
			o.Amount *= 0.9
		}
		return o
	}

	// Adapter: save to "database"
	var saved Order
	saveAdapter := NewAdapter[Order, Order](func(ctx context.Context, o Order) (Order, error) {
		saved = o
		o.Status = "saved"
		return o, nil
	})

	pipeline := NewPipeline[Order](obs,
		PortAsStep(validatePort),
		AtomAsStep(Atom[Order, Order](applyDiscount)),
		AdapterAsStep(saveAdapter),
	)

	order := Order{ID: "ORD-001", Amount: 200, Status: "new"}
	result, steps, err := pipeline.Run(ctx, order)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != "saved" {
		t.Errorf("expected Status='saved', got '%s'", result.Status)
	}
	if result.Amount != 180 { // 200 * 0.9
		t.Errorf("expected Amount=180, got %f", result.Amount)
	}
	if saved.Amount != 180 {
		t.Errorf("expected saved Amount=180, got %f", saved.Amount)
	}
	if len(steps) != 3 {
		t.Errorf("expected 3 steps, got %d", len(steps))
	}
	// Verify each step has correct UnitType
	expectedUnits := []string{"Port", "Atom", "Adapter"}
	for i, step := range steps {
		if step.Unit != expectedUnits[i] {
			t.Errorf("step %d: expected Unit='%s', got '%s'", i, expectedUnits[i], step.Unit)
		}
		if step.DurationMs < 0 {
			t.Errorf("step %d: DurationMs should be >= 0, got %d", i, step.DurationMs)
		}
	}
}

func TestIntegration_WithTimeoutAndRetry(t *testing.T) {
	ctx := context.Background()
	obs := &InMemoryObservationAdapter{}

	var attempts int32
	comp := NewPipeline[int](obs,
		StepFunc[int, int]{
			execute: func(ctx context.Context, input int) (int, error) {
				count := atomic.AddInt32(&attempts, 1)
				if count < 2 {
					return 0, NewStepError("RETRY", "fail", true)
				}
				return input * 3, nil
			},
			unitType: "Flaky",
		},
	)

	// Wrap with retry then timeout
	retryComp := WithRetry[int](comp, RetryConfig{
		MaxAttempts: 3,
		BaseDelay:   1 * time.Millisecond,
		MaxDelay:    10 * time.Millisecond,
		Multiplier:  2.0,
	})
	timeoutComp := WithTimeout[int](retryComp, 5*time.Second)

	result, _, err := timeoutComp.Run(ctx, 7)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != 21 {
		t.Errorf("expected 21, got %d", result)
	}
	if atomic.LoadInt32(&attempts) != 2 {
		t.Errorf("expected 2 attempts, got %d", attempts)
	}
}

func TestIntegration_BranchWithPipeline(t *testing.T) {
	ctx := context.Background()
	obs := &InMemoryObservationAdapter{}

	// Complex pipeline: Branch → different processing chains
	standardProcess := NewPipeline[string](obs,
		AtomAsStep(Atom[string, string](func(s string) string { return "STD:" + s })),
	)
	priorityProcess := NewPipeline[string](obs,
		AtomAsStep(Atom[string, string](func(s string) string { return "PRI:" + s })),
		AtomAsStep(Atom[string, string](func(s string) string { return s + ":URGENT" })),
	)

	branch := NewBranch[string](
		func(s string) bool { return len(s) > 10 },
		priorityProcess,
		standardProcess,
	)

	pipeline := NewPipeline[string](obs, branch)

	// Test priority path
	result, _, err := pipeline.Run(ctx, "this-is-a-long-message")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "PRI:this-is-a-long-message:URGENT" {
		t.Errorf("unexpected result: '%s'", result)
	}

	// Test standard path
	result2, _, err2 := pipeline.Run(ctx, "short")
	if err2 != nil {
		t.Fatalf("unexpected error: %v", err2)
	}
	if result2 != "STD:short" {
		t.Errorf("unexpected result: '%s'", result2)
	}
}

// ──────────────────────────────────────────────
// Benchmark Tests
// ──────────────────────────────────────────────

func BenchmarkPipeline_Simple(b *testing.B) {
	ctx := context.Background()
	pipeline := NewPipeline[int](nil,
		AtomAsStep(Atom[int, int](func(x int) int { return x + 1 })),
		AtomAsStep(Atom[int, int](func(x int) int { return x * 2 })),
	)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pipeline.Run(ctx, i)
	}
}

func BenchmarkPipeline_WithObservation(b *testing.B) {
	ctx := context.Background()
	obs := &InMemoryObservationAdapter{}
	pipeline := NewPipeline[int](obs,
		AtomAsStep(Atom[int, int](func(x int) int { return x + 1 })),
		AtomAsStep(Atom[int, int](func(x int) int { return x * 2 })),
	)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pipeline.Run(ctx, i)
	}
}

func BenchmarkParallel(b *testing.B) {
	ctx := context.Background()
	comps := make([]Composer[int], 4)
	for i := 0; i < 4; i++ {
		comps[i] = NewPipeline[int](nil,
			AtomAsStep(Atom[int, int](func(x int) int { return x * 2 })),
		)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		RunParallel(ctx, i, comps...)
	}
}

func BenchmarkTraceTree_Build(b *testing.B) {
	steps := make([]ExecutionStep, 100)
	parentSpan := string(NewSpanID())
	for i := 0; i < 100; i++ {
		steps[i] = ExecutionStep{
			SpanID:  string(NewSpanID()),
			TraceID: "bench-trace",
			Unit:    "Atom",
			Action:  "bench",
		}
		if i > 0 {
			steps[i].ParentID = parentSpan
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		BuildTraceTree(steps)
	}
}