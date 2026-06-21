//go:build lecore_tier7

package core

import (
	"context"
	"fmt"
	"math/rand"
	"testing"
	"time"
)

// ============================================================================
// 3.10 熵收集器压力测试 + 基准测试
// ============================================================================

// TestStress_EntropyCollector_LargeDataset: 大容量熵收集
func TestStress_EntropyCollector_LargeDataset(t *testing.T) {
	store := NewInMemoryStepStore(50000)
	collector := NewEntropyCollector()

	const steps = 20000
	execSteps := make([]ExecutionStep, steps)
	rng := rand.New(rand.NewSource(42))

	patterns := []string{"create", "update", "delete", "query"}
	units := []string{"Atom", "Port", "Adapter", "Composer"}

	for i := 0; i < steps; i++ {
		pattern := patterns[rng.Intn(len(patterns))]
		unit := units[rng.Intn(len(units))]
		execSteps[i] = NewExecutionStep(unit, pattern, fmt.Sprintf("detail-%d", i), pattern)
		if i%20 == 0 {
			execSteps[i].Error = &StepError{Code: "ERR", Message: "test error"}
		}
	}

	store.Record(execSteps)

	snapshot := collector.Collect(store)
	if snapshot.TotalSteps == 0 {
		t.Error("expected non-zero total steps")
	}

	t.Logf("EntropyCollector: steps=%d, error_rate=%.4f, unique_patterns=%d, unique_units=%d, entropy_score=%.4f",
		snapshot.TotalSteps, snapshot.ErrorRate, snapshot.UniquePatterns, snapshot.UniqueUnits, snapshot.EntropyScore)
}

func BenchmarkPipeline_100Steps(b *testing.B) {
	obs := &InMemoryObservationAdapter{}
	ctx := context.Background()

	steps := make([]Step[int, int], 100)
	for i := 0; i < 100; i++ {
		steps[i] = NewStepFunc[int, int]("Atom", func(ctx context.Context, in int) (int, error) {
			return in + 1, nil
		})
	}

	pipeline := NewPipeline[int](obs, steps...)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = pipeline.Run(ctx, 0)
	}
}

func BenchmarkCircuitBreaker_Throughput(b *testing.B) {
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

func BenchmarkShardedRateLimiter(b *testing.B) {
	limiter := NewShardedRateLimiter[int](1000000, 1000000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		limiter.Allow(i)
	}
}
