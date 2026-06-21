//go:build lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

package core

import (
	"fmt"
	"testing"
	"time"
)

// ──────────────────────────────────────────────
// Benchmark Tests
// ──────────────────────────────────────────────

func BenchmarkStepStore_Record(b *testing.B) {
	store := NewInMemoryStepStore(100000)
	step := ExecutionStep{Unit: "Atom", Pattern: "Pipeline", DurationMs: 1}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		store.Record([]ExecutionStep{step})
	}
}

func BenchmarkStepStore_Query(b *testing.B) {
	store := NewInMemoryStepStore(10000)
	for i := 0; i < 10000; i++ {
		store.Record([]ExecutionStep{{Unit: "Atom", TraceID: fmt.Sprintf("t%d", i%10)}})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		store.Query(StepQuery{TraceID: "t5", Limit: 100})
	}
}

func BenchmarkAggregator_Aggregate(b *testing.B) {
	config := AggregatorConfig{
		WindowDurations: []time.Duration{1 * time.Minute},
		MaxWindows:      100,
	}
	agg := NewAggregator(config)
	steps := make([]ExecutionStep, 100)
	for i := 0; i < 100; i++ {
		steps[i] = ExecutionStep{Unit: "Atom", Pattern: "Pipeline", DurationMs: int64(i)}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		agg.Aggregate(steps)
	}
}

func BenchmarkEntropyCollector(b *testing.B) {
	store := NewInMemoryStepStore(10000)
	for i := 0; i < 1000; i++ {
		store.Record([]ExecutionStep{{
			Unit:       "Atom",
			Pattern:    fmt.Sprintf("P%d", i%10),
			DurationMs: int64(i % 100),
		}})
	}
	collector := NewEntropyCollector()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		collector.Collect(store)
	}
}

func BenchmarkCircuitBreaker(b *testing.B) {
	ctx := context.Background()
	inner := NewPipeline[int](nil,
		AtomAsStep(Atom[int, int](func(x int) int { return x * 2 })),
	)
	cb := NewCircuitBreaker[int](inner, 100, 1*time.Hour)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cb.Run(ctx, i)
	}
}
