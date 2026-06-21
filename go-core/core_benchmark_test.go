package core

import (
	"context"
	"testing"
	"time"
)

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
