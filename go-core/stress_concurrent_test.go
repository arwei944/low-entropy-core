//go:build lecore_tier7

package core

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// ============================================================================
// Phase 3: 极限压力测试
// ============================================================================

// ============================================================================
// 3.1 并发压力测试
// ============================================================================

// TestStress_ConcurrentPipeline_1000Goroutines: 1000个goroutine同时运行Pipeline
func TestStress_ConcurrentPipeline_1000Goroutines(t *testing.T) {
	obs := &InMemoryObservationAdapter{}
	ctx := context.Background()

	atom := func(ctx context.Context, in int) (int, error) {
		time.Sleep(time.Microsecond)
		return in * 2, nil
	}

	pipeline := NewPipeline[int](obs,
		NewStepFunc[int, int]("Atom", atom),
		NewStepFunc[int, int]("Atom", atom),
	)

	var wg sync.WaitGroup
	errCount := atomic.Int64{}
	successCount := atomic.Int64{}

	const goroutines = 1000
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			_, _, err := pipeline.Run(ctx, id)
			if err != nil {
				errCount.Add(1)
			} else {
				successCount.Add(1)
			}
		}(i)
	}
	wg.Wait()

	if errCount.Load() > 0 {
		t.Errorf("expected 0 errors, got %d", errCount.Load())
	}
	if successCount.Load() != goroutines {
		t.Errorf("expected %d successes, got %d", goroutines, successCount.Load())
	}
	t.Logf("ConcurrentPipeline: %d goroutines, %d success, %d errors", goroutines, successCount.Load(), errCount.Load())
}

// TestStress_ConcurrentFastPipeline_10000Goroutines: 10000 goroutine FastPipeline
func TestStress_ConcurrentFastPipeline_10000Goroutines(t *testing.T) {
	ctx := context.Background()

	fp := NewFastPipeline[int]("stress-fast")
	fp.AddStep(NewStepFunc[any, any]("Atom", func(ctx context.Context, in any) (any, error) {
		return in.(int) * 2, nil
	}))

	var wg sync.WaitGroup
	errCount := atomic.Int64{}
	successCount := atomic.Int64{}

	const goroutines = 10000
	start := time.Now()
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			_, err := fp.Run(ctx, id)
			if err != nil {
				errCount.Add(1)
			} else {
				successCount.Add(1)
			}
		}(i)
	}
	wg.Wait()
	elapsed := time.Since(start)

	if errCount.Load() > 0 {
		t.Errorf("expected 0 errors, got %d", errCount.Load())
	}
	t.Logf("FastPipeline: %d goroutines in %v, %d success, throughput: %.0f ops/s",
		goroutines, elapsed, successCount.Load(), float64(goroutines)/elapsed.Seconds())
}

// TestStress_ConcurrentShardedLock_HighContention: 分片锁高竞争场景
func TestStress_ConcurrentShardedLock_HighContention(t *testing.T) {
	lock := NewShardedLock[int]()
	var counter sync.Map

	var wg sync.WaitGroup
	const goroutines = 500
	const opsPerGoroutine = 1000

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < opsPerGoroutine; j++ {
				key := j % 100 // 100个热点key
				lock.Lock(key)
				val, _ := counter.LoadOrStore(key, 0)
				counter.Store(key, val.(int)+1)
				lock.Unlock(key)
			}
		}(i)
	}
	wg.Wait()

	total := 0
	counter.Range(func(key, value interface{}) bool {
		total += value.(int)
		return true
	})

	expected := goroutines * opsPerGoroutine
	if total != expected {
		t.Errorf("expected total %d, got %d", expected, total)
	}
	t.Logf("ShardedLock: %d goroutines x %d ops, total=%d, no data races", goroutines, opsPerGoroutine, total)
}

// TestStress_ConcurrentRateLimiter_Sharded: 分片限流器高并发
func TestStress_ConcurrentRateLimiter_Sharded(t *testing.T) {
	limiter := NewShardedRateLimiter[int](1000, 1000) // 1000 tokens/s

	var wg sync.WaitGroup
	allowed := atomic.Int64{}
	denied := atomic.Int64{}

	const goroutines = 100
	const attempts = 1000

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < attempts; j++ {
				if limiter.Allow(id) {
					allowed.Add(1)
				} else {
					denied.Add(1)
				}
			}
		}(i)
	}
	wg.Wait()

	total := allowed.Load() + denied.Load()
	if total != int64(goroutines*attempts) {
		t.Errorf("expected total %d, got %d", goroutines*attempts, total)
	}
	t.Logf("ShardedRateLimiter: %d goroutines, allowed=%d, denied=%d, total=%d",
		goroutines, allowed.Load(), denied.Load(), total)
}
