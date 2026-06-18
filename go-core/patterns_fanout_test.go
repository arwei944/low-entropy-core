//go:build lecore_tier2 || lecore_tier3 || lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

package core

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestFanOut_Basic(t *testing.T) {
	ctx := context.Background()
	obs := &InMemoryObservationAdapter{}

	var count1, count2 atomic.Int32

	child1 := NewPipeline[int](obs,
		StepFunc[int, int]{
			execute: func(ctx context.Context, input int) (int, error) {
				count1.Add(1)
				return input * 2, nil
			},
			unitType: "Atom",
		},
	)
	child2 := NewPipeline[int](obs,
		StepFunc[int, int]{
			execute: func(ctx context.Context, input int) (int, error) {
				count2.Add(1)
				return input + 10, nil
			},
			unitType: "Atom",
		},
	)

	fanOut := NewFanOut[int](obs, child1, child2)
	result, steps, err := fanOut.Run(ctx, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != 5 {
		t.Errorf("expected 5 (input returned), got %d", result)
	}
	if count1.Load() != 1 || count2.Load() != 1 {
		t.Error("both children should have been called")
	}
	if len(steps) == 0 {
		t.Error("expected steps to be collected")
	}
}

func TestFanOut_Empty(t *testing.T) {
	obs := &InMemoryObservationAdapter{}
	fanOut := NewFanOut[int](obs)
	result, steps, err := fanOut.Run(context.Background(), 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != 5 {
		t.Errorf("expected 5, got %d", result)
	}
	if len(steps) != 0 {
		t.Error("expected no steps for empty fanout")
	}
}

func TestDebounce_SkipsDuplicates(t *testing.T) {
	ctx := context.Background()
	obs := &InMemoryObservationAdapter{}

	var callCount atomic.Int32
	inner := NewPipeline[int](obs,
		StepFunc[int, int]{
			execute: func(ctx context.Context, input int) (int, error) {
				callCount.Add(1)
				return input * 2, nil
			},
			unitType: "Atom",
		},
	)

	debounce := NewDebounce[int](inner, 200*time.Millisecond)

	// 第一次调用
	result, _, err := debounce.Run(ctx, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != 10 {
		t.Errorf("expected 10, got %d", result)
	}

	// 立即第二次调用（应被跳过）
	result, _, err = debounce.Run(ctx, 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != 100 {
		t.Errorf("expected 100 (input returned as-is), got %d", result)
	}
	if callCount.Load() != 1 {
		t.Errorf("expected 1 call, got %d", callCount.Load())
	}
}

func TestDebounce_AfterInterval(t *testing.T) {
	ctx := context.Background()
	obs := &InMemoryObservationAdapter{}

	var callCount atomic.Int32
	inner := NewPipeline[int](obs,
		StepFunc[int, int]{
			execute: func(ctx context.Context, input int) (int, error) {
				callCount.Add(1)
				return input * 2, nil
			},
			unitType: "Atom",
		},
	)

	debounce := NewDebounce[int](inner, 50*time.Millisecond)

	debounce.Run(ctx, 5)
	time.Sleep(60 * time.Millisecond)
	result, _, err := debounce.Run(ctx, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != 20 {
		t.Errorf("expected 20, got %d", result)
	}
	if callCount.Load() != 2 {
		t.Errorf("expected 2 calls, got %d", callCount.Load())
	}
}

func TestThrottle_DelaysCall(t *testing.T) {
	ctx := context.Background()
	obs := &InMemoryObservationAdapter{}

	var callCount atomic.Int32
	inner := NewPipeline[int](obs,
		StepFunc[int, int]{
			execute: func(ctx context.Context, input int) (int, error) {
				callCount.Add(1)
				return input * 2, nil
			},
			unitType: "Atom",
		},
	)

	throttle := NewThrottle[int](inner, 50*time.Millisecond)

	start := time.Now()
	result, _, err := throttle.Run(ctx, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != 10 {
		t.Errorf("expected 10, got %d", result)
	}

	// 第二次调用应等待
	result, _, err = throttle.Run(ctx, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != 20 {
		t.Errorf("expected 20, got %d", result)
	}
	elapsed := time.Since(start)
	if elapsed < 50*time.Millisecond {
		t.Errorf("expected at least 50ms between calls, got %v", elapsed)
	}
	if callCount.Load() != 2 {
		t.Errorf("expected 2 calls, got %d", callCount.Load())
	}
}

func TestThrottle_FirstCallImmediate(t *testing.T) {
	ctx := context.Background()
	obs := &InMemoryObservationAdapter{}

	inner := NewPipeline[int](obs,
		AtomAsStep(Atom[int, int](func(x int) int { return x + 1 })),
	)

	throttle := NewThrottle[int](inner, 1*time.Second)
	result, _, err := throttle.Run(ctx, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != 6 {
		t.Errorf("expected 6, got %d", result)
	}
}