// Package core — 基准测试套件 (v4.0)
//
// 覆盖所有关键热路径的基准测试，对比 v3.0 和 v4.0 的性能差异。
// 运行方式：go test -bench=. -benchmem -benchtime=3s ./...
// 运行 race 检测：go test -race -bench=. ./...
package core

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

// =============================================================================
// Pipeline 基准测试
// =============================================================================

func BenchmarkPipeline_100Steps(b *testing.B) {
	pipeline := NewPipeline[int]("bench_pipeline_100")
	for i := 0; i < 100; i++ {
		atom := Atom[int, int](func(n int) int { return n + 1 })
		pipeline.AddStep(AtomAsStep(atom, fmt.Sprintf("step_%d", i)))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, err := pipeline.Run(context.Background(), 0)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkFastPipeline_100Steps(b *testing.B) {
	fp := NewFastPipeline[int]("bench_fast_100")
	for i := 0; i < 100; i++ {
		inc := StepFunc[any, any]{
			execute: func(ctx context.Context, input any) (any, error) {
				return input.(int) + 1, nil
			},
			unitType: "Atom",
		}
		fp.AddStep(inc)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := fp.Run(context.Background(), 0)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// =============================================================================
// 观测层基准测试
// =============================================================================

func BenchmarkObservation_10KGoroutines(b *testing.B) {
	adapter := NewShardedObservationAdapter()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var wg sync.WaitGroup
		for g := 0; g < 100; g++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				steps := []ExecutionStep{
					{
						Timestamp:  time.Now(),
						TraceID:    NewTraceID().String(),
						SpanID:     NewSpanID().String(),
						Unit:       "Atom",
						Action:     "test",
						DurationMs: 1,
					},
				}
				adapter.Record(steps)
			}()
		}
		wg.Wait()
	}
}

// =============================================================================
// 分片锁基准测试
// =============================================================================

func BenchmarkShardedLock_1000Keys_Read(b *testing.B) {
	var lock ShardedLock[int]
	keys := make([]int, 1000)
	for i := 0; i < 1000; i++ {
		keys[i] = i
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			lock.RLock(keys[i%1000])
			lock.RUnlock(keys[i%1000])
			i++
		}
	})
}

func BenchmarkShardedLock_1000Keys_Write(b *testing.B) {
	var lock ShardedLock[int]

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			lock.Lock(i % 1000)
			lock.Unlock(i % 1000)
			i++
		}
	})
}

func BenchmarkSingleMutex_Write(b *testing.B) {
	var mu sync.Mutex

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			mu.Lock()
			mu.Unlock()
		}
	})
}

// =============================================================================
// EventStore 基准测试
// =============================================================================

func BenchmarkShardedEventStore_1000Aggregates(b *testing.B) {
	store := NewShardedEventStore()
	ctx := context.Background()

	aggregates := make([]string, 1000)
	for i := 0; i < 1000; i++ {
		aggregates[i] = fmt.Sprintf("agg_%d", i)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			env := EventEnvelope{
				AggregateID:   aggregates[i%1000],
				AggregateType: "TestAggregate",
				EventType:     "TestEvent",
				EventData:     []byte("test"),
				Timestamp:     time.Now(),
			}
			store.Execute(ctx, env)
			i++
		}
	})
}

// =============================================================================
// UUID 生成基准测试
// =============================================================================

func BenchmarkUUIDGen_1M(b *testing.B) {
	gen := getGlobalUUIDGen()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		gen.Next()
	}
}

func BenchmarkUUIDGen_String_1M(b *testing.B) {
	gen := getGlobalUUIDGen()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		gen.NextString()
	}
}

// =============================================================================
// TDigest 基准测试
// =============================================================================

func BenchmarkTDigest_Add_100K(b *testing.B) {
	td := NewTDigestDefault()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		td.Add(float64(i % 100000))
	}
}

func BenchmarkTDigest_Quantile(b *testing.B) {
	td := NewTDigestDefault()
	for i := 0; i < 100000; i++ {
		td.Add(float64(i))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		td.Quantile(0.50)
		td.Quantile(0.95)
		td.Quantile(0.99)
	}
}

// =============================================================================
// CircuitBreaker 基准测试 (atomic vs mutex)
// =============================================================================

func BenchmarkCircuitBreaker_Atomic(b *testing.B) {
	inner := NewPipeline[int]("inner")
	inner.AddStep(AtomAsStep(Atom[int, int](func(n int) int { return n + 1 }), "inc"))
	cb := NewCircuitBreaker[int](inner, 5, 30*time.Second)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cb.Run(context.Background(), 0)
	}
}

// =============================================================================
// RateLimiter 基准测试
// =============================================================================

func BenchmarkRateLimiter_Atomic(b *testing.B) {
	inner := NewPipeline[int]("inner")
	inner.AddStep(AtomAsStep(Atom[int, int](func(n int) int { return n + 1 }), "inc"))
	rl := NewRateLimiter[int](inner, 10000, 10000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rl.Run(context.Background(), 0)
	}
}

// =============================================================================
// ShardedRateLimiter 基准测试
// =============================================================================

func BenchmarkShardedRateLimiter_1000Keys(b *testing.B) {
	rl := NewShardedRateLimiter[string](10000, 10000)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		keys := []string{"key_a", "key_b", "key_c", "key_d", "key_e"}
		for pb.Next() {
			rl.Allow(keys[i%5])
			i++
		}
	})
}