package core

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

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
