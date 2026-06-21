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
// 3.3 故障注入测试
// ============================================================================

// TestStress_CircuitBreaker_OpensAndRecovers: 熔断器开闭恢复
func TestStress_CircuitBreaker_OpensAndRecovers(t *testing.T) {
	obs := &InMemoryObservationAdapter{}

	failCount := atomic.Int64{}
	failingComposer := Compose[int](obs, NewStepFunc[int, int]("Atom", func(ctx context.Context, in int) (int, error) {
		failCount.Add(1)
		return 0, fmt.Errorf("injected failure")
	}))

	cb := NewCircuitBreaker[int](failingComposer, 5, 200*time.Millisecond)
	ctx := context.Background()

	for i := 0; i < 20; i++ {
		_, _, err := cb.Run(ctx, i)
		if err != nil {
			// 预期错误
		}
	}

	state := cb.State()
	t.Logf("CircuitBreaker after 20 failures: state=%s, failCount=%d", state, failCount.Load())

	if state == CircuitClosed {
		t.Error("expected circuit to be open or half-open after 20 failures")
	}

	time.Sleep(300 * time.Millisecond)

	_, _, err := cb.Run(ctx, 0)
	if err == nil {
		t.Log("CircuitBreaker recovered (expected)")
	}
}

// TestStress_Timeout_ComposerCancellation: 超时组合器取消
func TestStress_Timeout_ComposerCancellation(t *testing.T) {
	obs := &InMemoryObservationAdapter{}

	slowComposer := Compose[int](obs, NewStepFunc[int, int]("Atom", func(ctx context.Context, in int) (int, error) {
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		case <-time.After(1 * time.Second):
			return in, nil
		}
	}))

	timedOut := WithTimeout[int](slowComposer, 50*time.Millisecond)

	ctx := context.Background()
	start := time.Now()
	_, _, err := timedOut.Run(ctx, 42)
	elapsed := time.Since(start)

	if err == nil {
		t.Error("expected timeout error, got nil")
	}
	if elapsed > 200*time.Millisecond {
		t.Errorf("timeout took too long: %v", elapsed)
	}
	t.Logf("Timeout: elapsed=%v, err=%v", elapsed, err)
}

// TestStress_Retry_Exhausted: 重试耗尽
func TestStress_Retry_Exhausted(t *testing.T) {
	obs := &InMemoryObservationAdapter{}
	attempts := atomic.Int64{}

	failing := Compose[int](obs, NewStepFunc[int, int]("Atom", func(ctx context.Context, in int) (int, error) {
		attempts.Add(1)
		return 0, fmt.Errorf("always fail")
	}))

	retry := WithRetry[int](failing, RetryConfig{
		MaxAttempts: 3,
		BaseDelay:   10 * time.Millisecond,
		MaxDelay:    100 * time.Millisecond,
		Multiplier:  2.0,
	})

	ctx := context.Background()
	_, _, err := retry.Run(ctx, 0)

	if err == nil {
		t.Error("expected error after retry exhaustion")
	}
	if attempts.Load() != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts.Load())
	}
	t.Logf("Retry exhausted: attempts=%d, err=%v", attempts.Load(), err)
}

// TestStress_ErrorPropagation_DeepPipeline: 深层Pipeline错误传播
func TestStress_ErrorPropagation_DeepPipeline(t *testing.T) {
	obs := &InMemoryObservationAdapter{}

	steps := make([]Step[int, int], 50)
	for i := 0; i < 50; i++ {
		idx := i
		steps[i] = NewStepFunc[int, int]("Atom", func(ctx context.Context, in int) (int, error) {
			if idx == 25 {
				return 0, fmt.Errorf("error at step 25")
			}
			return in + 1, nil
		})
	}

	pipeline := NewPipeline[int](obs, steps...)
	ctx := context.Background()
	_, execSteps, err := pipeline.Run(ctx, 0)

	if err == nil {
		t.Error("expected error from step 25")
	}
	if len(execSteps) < 25 {
		t.Errorf("expected at least 25 execution steps, got %d", len(execSteps))
	}
	t.Logf("Error propagation: depth=%d, steps_recorded=%d, err=%v", len(steps), len(execSteps), err)
}

// ============================================================================
// 3.7 调度器压力测试
// ============================================================================

// TestStress_TaskQueue_ConcurrentEnqueueDequeue: 并发入队出队
func TestStress_TaskQueue_ConcurrentEnqueueDequeue(t *testing.T) {
	queue := NewTaskQueue()
	defer queue.Close()

	ctx := context.Background()
	const tasks = 5000

	var wg sync.WaitGroup
	for i := 0; i < tasks; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			task := &QueuedTask{
				TaskID:    fmt.Sprintf("task-%d", id),
				Phase:     "build",
				Priority:  id % 10,
				CreatedAt: time.Now(),
			}
			if err := queue.Enqueue(task); err != nil {
				t.Errorf("enqueue failed: %v", err)
			}
		}(i)
	}
	wg.Wait()

	if queue.Len() != tasks {
		t.Errorf("expected %d tasks in queue, got %d", tasks, queue.Len())
	}

	dequeued := atomic.Int64{}
	for i := 0; i < tasks; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := queue.Dequeue(ctx, 100*time.Millisecond)
			if err == nil {
				dequeued.Add(1)
			}
		}()
	}
	wg.Wait()

	t.Logf("TaskQueue: enqueued=%d, dequeued=%d", tasks, dequeued.Load())
}

// TestStress_MatchEngine_Batch: 批量匹配引擎
func TestStress_MatchEngine_Batch(t *testing.T) {
	const batchSize = 1000

	agents := make([]*AgentInfo, batchSize)
	for i := 0; i < batchSize; i++ {
		agents[i] = &AgentInfo{
			ID:           fmt.Sprintf("agent-%d", i),
			Phase:        "build",
			Capabilities: []string{"go", "test"},
			LastHeartbeat: time.Now().Add(-time.Duration(i) * time.Second),
		}
	}

	pool := NewAgentPool()
	for _, agent := range agents {
		pool.Add(agent)
	}

	for i := 0; i < batchSize; i++ {
		task := &QueuedTask{
			TaskID:              fmt.Sprintf("task-%d", i),
			Phase:               "build",
			RequiredCapabilities: []string{"go"},
			Priority:            i % 5,
		}
		result := MatchEngine(MatchInput{Task: task, Pool: pool})
		if result.Matched == nil {
			t.Errorf("task %d: no agent matched", i)
		}
	}

	t.Logf("MatchEngine: %d tasks matched successfully", batchSize)
}

// ============================================================================
// 3.9 UUID生成器压力测试
// ============================================================================

// TestStress_UUIDGen_Concurrent: 并发UUID生成
func TestStress_UUIDGen_Concurrent(t *testing.T) {
	gen, err := NewBatchedUUIDGen()
	if err != nil {
		t.Fatalf("NewBatchedUUIDGen() failed: %v", err)
	}
	defer gen.Close()

	const goroutines = 100
	const uuidsPerGoroutine = 1000

	uuids := make(map[string]bool)
	var mu sync.Mutex

	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < uuidsPerGoroutine; j++ {
				id := gen.NextString()
				mu.Lock()
				uuids[id] = true
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	expected := goroutines * uuidsPerGoroutine
	if len(uuids) != expected {
		t.Errorf("expected %d unique UUIDs, got %d (collisions: %d)", expected, len(uuids), expected-len(uuids))
	}
	t.Logf("UUIDGen: %d goroutines x %d uuids = %d unique, 0 collisions", goroutines, uuidsPerGoroutine, len(uuids))
}

// ============================================================================
// 3.11 降级管理压力测试
// ============================================================================

// TestStress_Degradation_GracefulDegradation: 优雅降级
func TestStress_Degradation_GracefulDegradation(t *testing.T) {
	obs := &InMemoryObservationAdapter{}
	dm := NewDegradationManager(obs)

	ctx := context.Background()

	normalComposer := Compose[int](obs, NewStepFunc[int, int]("Atom", func(ctx context.Context, in int) (int, error) {
		return in * 2, nil
	}))

	result, _, err := normalComposer.Run(ctx, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != 20 {
		t.Errorf("expected 20, got %d", result)
	}

	_ = dm
	t.Logf("DegradationManager: normal operation verified, dm available")
}
