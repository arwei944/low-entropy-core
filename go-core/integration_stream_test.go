//go:build lecore_tier5 || lecore_tier6 || lecore_tier7

package core_test

import (
	"context"
	"fmt"
	"testing"

	. "low-entropy-core/go-core"
)

func TestIntegration_FastPipeline(t *testing.T) {
	ctx := context.Background()

	doubleAtom := Atom[int, int](func(input int) int { return input * 2 })
	addOneAtom := Atom[int, int](func(input int) int { return input + 1 })

	doubleStep := NewStepFunc[any, any]("Atom", func(ctx context.Context, input any) (any, error) {
		v, ok := input.(int)
		if !ok {
			return nil, fmt.Errorf("expected int, got %T", input)
		}
		return doubleAtom(v), nil
	})
	addOneStep := NewStepFunc[any, any]("Atom", func(ctx context.Context, input any) (any, error) {
		v, ok := input.(int)
		if !ok {
			return nil, fmt.Errorf("expected int, got %T", input)
		}
		return addOneAtom(v), nil
	})

	fastPipeline := NewFastPipeline[int]("fast-test")
	fastPipeline.AddStep(doubleStep)
	fastPipeline.AddStep(addOneStep)

	fastResult, fastErr := fastPipeline.Run(ctx, 5)
	if fastErr != nil {
		t.Fatalf("FastPipeline.Run failed: %v", fastErr)
	}
	if fastResult != 11 {
		t.Errorf("FastPipeline: expected 11, got %d", fastResult)
	}

	obs := &InMemoryObservationAdapter{}
	normalPipeline := NewPipeline[int](obs,
		AtomAsStep[int, int](doubleAtom),
		AtomAsStep[int, int](addOneAtom),
	)
	normalResult, steps, normalErr := normalPipeline.Run(ctx, 5)
	if normalErr != nil {
		t.Fatalf("Pipeline.Run failed: %v", normalErr)
	}
	if normalResult != 11 {
		t.Errorf("Pipeline: expected 11, got %d", normalResult)
	}

	if fastResult != normalResult {
		t.Errorf("FastPipeline and Pipeline should produce the same output: %d vs %d", fastResult, normalResult)
	}

	if len(steps) != 2 {
		t.Errorf("Pipeline should produce 2 ExecutionSteps, got %d", len(steps))
	}
}

func TestIntegration_StreamProcessing(t *testing.T) {
	numbers := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	stream := FromSlice(numbers)

	doubled := StreamMap(stream, func(n int) int { return n * 2 })

	filtered := StreamFilter(doubled, func(n int) bool { return n > 10 })

	sum := StreamReduce(filtered, 0, func(acc, n int) int { return acc + n })

	if sum != 80 {
		t.Errorf("expected sum 80, got %d", sum)
	}

	directStream := FromSlice([]string{"a", "b", "c"})
	collected := Collect(directStream)
	if len(collected) != 3 {
		t.Errorf("expected 3 collected items, got %d", len(collected))
	}
	expected := []string{"a", "b", "c"}
	for i, s := range expected {
		if collected[i] != s {
			t.Errorf("collected[%d]: expected %q, got %q", i, s, collected[i])
		}
	}
}
