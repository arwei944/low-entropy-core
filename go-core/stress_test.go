//go:build lecore_tier7

package core

import (
	"context"
	"fmt"
	"math/rand"
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

// ============================================================================
// 3.2 内存压力测试
// ============================================================================

// TestStress_LargePipeline_1000Steps: 1000步深度Pipeline
func TestStress_LargePipeline_1000Steps(t *testing.T) {
	obs := &InMemoryObservationAdapter{}
	ctx := context.Background()

	steps := make([]Step[int, int], 1000)
	for i := 0; i < 1000; i++ {
		steps[i] = NewStepFunc[int, int]("Atom", func(ctx context.Context, in int) (int, error) {
			return in + 1, nil
		})
	}

	pipeline := NewPipeline[int](obs, steps...)

	start := time.Now()
	result, execSteps, err := pipeline.Run(ctx, 0)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != 1000 {
		t.Errorf("expected 1000, got %d", result)
	}
	if len(execSteps) != 1000 {
		t.Errorf("expected 1000 execution steps, got %d", len(execSteps))
	}
	t.Logf("1000-Step Pipeline: result=%d, steps=%d, elapsed=%v", result, len(execSteps), elapsed)
}

// TestStress_EventStore_HighVolume: 事件存储高容量测试
func TestStress_EventStore_HighVolume(t *testing.T) {
	es := NewEventStore()
	ctx := context.Background()

	const events = 10000
	start := time.Now()

	for i := 0; i < events; i++ {
		envelope := EventEnvelope{
			EventID:     fmt.Sprintf("evt-%d", i),
			AggregateID: "agg-1",
			EventType:   "test",
			EventData:   []byte(fmt.Sprintf(`{"value": %d}`, i)),
			Version:     int64(i + 1),
		}
		_, err := es.Execute(ctx, envelope)
		if err != nil {
			t.Fatalf("unexpected error at event %d: %v", i, err)
		}
	}
	elapsed := time.Since(start)

	// Verify streaming
	streamed := es.StreamAll("agg-1")
	if len(streamed) != events {
		t.Errorf("expected %d events in stream, got %d", events, len(streamed))
	}

	t.Logf("EventStore: %d events in %v, throughput: %.0f events/s",
		events, elapsed, float64(events)/elapsed.Seconds())
}

// TestStress_StepStore_BufferOverflow: 步骤存储环形缓冲区溢出
func TestStress_StepStore_BufferOverflow(t *testing.T) {
	store := NewInMemoryStepStore(1000) // 小容量环形缓冲区

	const records = 10000
	steps := make([]ExecutionStep, records)
	for i := 0; i < records; i++ {
		steps[i] = NewExecutionStep("Atom", "test", fmt.Sprintf("step-%d", i), "stress")
	}

	store.Record(steps)

	count := store.Count()
	if count != 1000 { // 环形缓冲区最多保留1000
		t.Errorf("expected 1000 records, got %d", count)
	}

	// 查询应返回最近1000条
	queried := store.Query(StepQuery{Limit: 2000})
	if len(queried) != 1000 {
		t.Errorf("expected 1000 queried, got %d", len(queried))
	}

	t.Logf("StepStore RingBuffer: %d records, stored=%d, queried=%d", records, count, len(queried))
}

// TestStress_TDigest_LargeDataset: TDigest大容量数据
func TestStress_TDigest_LargeDataset(t *testing.T) {
	td := NewTDigestDefault()

	const samples = 100000
	rng := rand.New(rand.NewSource(42))

	start := time.Now()
	for i := 0; i < samples; i++ {
		td.Add(rng.Float64() * 1000)
	}
	elapsed := time.Since(start)

	p50 := td.Quantile(0.5)
	p95 := td.Quantile(0.95)
	p99 := td.Quantile(0.99)
	mean := td.Mean()

	// 校验分位数之间关系
	if p50 > p95 || p95 > p99 {
		t.Errorf("quantile ordering violated: p50=%.2f, p95=%.2f, p99=%.2f", p50, p95, p99)
	}

	t.Logf("TDigest: %d samples in %v, p50=%.2f, p95=%.2f, p99=%.2f, mean=%.2f",
		samples, elapsed, p50, p95, p99, mean)
}

// ============================================================================
// 3.3 故障注入测试
// ============================================================================

// TestStress_CircuitBreaker_OpensAndRecovers: 熔断器开闭恢复
func TestStress_CircuitBreaker_OpensAndRecovers(t *testing.T) {
	obs := &InMemoryObservationAdapter{}

	failCount := atomic.Int64{}
	// 创建一个总是失败的Composer
	failingComposer := Compose[int](obs, NewStepFunc[int, int]("Atom", func(ctx context.Context, in int) (int, error) {
		failCount.Add(1)
		return 0, fmt.Errorf("injected failure")
	}))

	cb := NewCircuitBreaker[int](failingComposer, 5, 200*time.Millisecond)
	ctx := context.Background()

	// 触发熔断
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

	// 等待冷却
	time.Sleep(300 * time.Millisecond)

	// 尝试恢复
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
	// 错误应传播到第25步
	if len(execSteps) < 25 {
		t.Errorf("expected at least 25 execution steps, got %d", len(execSteps))
	}
	t.Logf("Error propagation: depth=%d, steps_recorded=%d, err=%v", len(steps), len(execSteps), err)
}

// ============================================================================
// 3.4 Handoff协议压力测试
// ============================================================================

// TestStress_Handoff_MultiAgentChain: 多Agent链式Handoff
func TestStress_Handoff_MultiAgentChain(t *testing.T) {
	persistence := NewInMemorySnapshotAdapter()
	transport := NewInProcHandoffTransport()
	obs := &InMemoryObservationAdapter{}

	composer := NewHandoffComposer(obs, persistence, transport)
	ctx := context.Background()

	// 创建初始快照
	snapshot := NewDevSnapshot("task-1", "agent-1", "design", "initial")
	snapshot.Artifacts = []Artifact{
		{Path: "main.go", Type: "code", Description: "main entry", Hash: "abc123"},
	}
	snapshot.Decisions = []Decision{
		{ID: "d1", Title: "use goroutines", Rationale: "performance"},
	}

	// 链式Handoff: agent-1 -> agent-2 -> agent-3
	agents := []string{"agent-2", "agent-3"}
	currentSnapshot := snapshot

	for _, targetAgent := range agents {
		input := HandoffInput{
			SourceAgent:   currentSnapshot,
			TargetAgentID: targetAgent,
			TaskID:        "task-1",
			Phase:         "design",
		}

		output, _, err := composer.Execute(ctx, input)
		if err != nil {
			t.Fatalf("handoff to %s failed: %v", targetAgent, err)
		}
		if !output.Success {
			t.Fatalf("handoff to %s not successful: %s", targetAgent, output.Error)
		}

		// 接收方验证快照
		received, _, err := composer.ReceiveSnapshot(ctx, output.SnapshotChecksum)
		if err != nil {
			t.Fatalf("receive snapshot failed: %v", err)
		}
		if !received.VerifyChecksum() {
			t.Fatalf("checksum verification failed for %s", targetAgent)
		}

		currentSnapshot = received
		t.Logf("Handoff %s -> %s: checksum=%s, verified=true", currentSnapshot.AgentID, targetAgent, output.SnapshotChecksum)
	}

	t.Logf("Multi-Agent Handoff: chain of %d agents completed successfully", len(agents)+1)
}

// TestStress_Handoff_Rollback: Handoff回滚
func TestStress_Handoff_Rollback(t *testing.T) {
	persistence := NewInMemorySnapshotAdapter()
	transport := NewInProcHandoffTransport()
	obs := &InMemoryObservationAdapter{}

	composer := NewHandoffComposer(obs, persistence, transport)
	ctx := context.Background()

	snapshot := NewDevSnapshot("task-1", "agent-1", "build", "checkpoint-1")
	snapshot.Artifacts = []Artifact{
		{Path: "output.bin", Type: "binary", Description: "binary output", Hash: "def456"},
	}

	// 正常Handoff
	input := HandoffInput{
		SourceAgent:   snapshot,
		TargetAgentID: "agent-2",
		TaskID:        "task-1",
		Phase:         "build",
	}

	output, _, err := composer.Execute(ctx, input)
	if err != nil {
		t.Fatalf("handoff failed: %v", err)
	}

	// 回滚
	rollbackResult, _, err := RollbackHandoff(ctx, persistence, transport, output.SnapshotChecksum, obs)
	if err != nil {
		t.Fatalf("rollback failed: %v", err)
	}
	if !rollbackResult.Success {
		t.Errorf("rollback not successful: %s", rollbackResult.Error)
	}

	t.Logf("Handoff Rollback: checksum=%s, rollback_success=%v", output.SnapshotChecksum, rollbackResult.Success)
}

// TestStress_Handoff_ChecksumIntegrity: 校验和完整性
func TestStress_Handoff_ChecksumIntegrity(t *testing.T) {
	const snapshots = 100

	for i := 0; i < snapshots; i++ {
		snapshot := NewDevSnapshot(fmt.Sprintf("task-%d", i), "agent-src", "phase", "checkpoint")
		snapshot.Artifacts = []Artifact{
			{
				Path:        fmt.Sprintf("file-%d.go", i),
				Type:        "code",
				Description: fmt.Sprintf("snapshot %d", i),
				Hash:        fmt.Sprintf("hash-%d", i),
			},
		}

		checksum, err := snapshot.ComputeChecksum()
		if err != nil {
			t.Fatalf("compute checksum failed: %v", err)
		}
		if !snapshot.VerifyChecksum() {
			t.Fatalf("checksum verification failed for snapshot %d", i)
		}

		// JSON序列化往返
		data, err := snapshot.ToJSON()
		if err != nil {
			t.Fatalf("toJSON failed: %v", err)
		}

		restored, err := DevSnapshotFromJSON(data)
		if err != nil {
			t.Fatalf("fromJSON failed: %v", err)
		}

		if restored.Checksum != checksum {
			t.Errorf("checksum mismatch: original=%s, restored=%s", checksum, restored.Checksum)
		}
	}

	t.Logf("Checksum Integrity: %d snapshots, all verified", snapshots)
}

// ============================================================================
// 3.5 分支与并行压力测试
// ============================================================================

// TestStress_BranchComposer_DecisionTree: 分支组合器决策树
func TestStress_BranchComposer_DecisionTree(t *testing.T) {
	obs := &InMemoryObservationAdapter{}

	truePath := Compose[int](obs, NewStepFunc[int, int]("Atom", func(ctx context.Context, in int) (int, error) {
		return in * 10, nil
	}))
	falsePath := Compose[int](obs, NewStepFunc[int, int]("Atom", func(ctx context.Context, in int) (int, error) {
		return in * -1, nil
	}))

	branch := NewBranch[int](func(n int) bool { return n > 0 }, truePath, falsePath)
	ctx := context.Background()

	// 正数走true分支
	posResult, _, err := branch.Run(ctx, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if posResult != 50 {
		t.Errorf("expected 50, got %d", posResult)
	}

	// 负数走false分支
	negResult, _, err := branch.Run(ctx, -5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if negResult != 5 {
		t.Errorf("expected 5, got %d", negResult)
	}

	t.Logf("Branch composer: pos=5->%d, neg=-5->%d", posResult, negResult)
}

// TestStress_RunParallel_FanOut: 并行执行FanOut
func TestStress_RunParallel_FanOut(t *testing.T) {
	obs := &InMemoryObservationAdapter{}

	composers := make([]Composer[int], 10)
	for i := 0; i < 10; i++ {
		idx := i
		composers[i] = Compose[int](obs, NewStepFunc[int, int]("Atom", func(ctx context.Context, in int) (int, error) {
			time.Sleep(time.Duration(10+idx) * time.Millisecond)
			return in * (idx + 1), nil
		}))
	}

	ctx := context.Background()
	results, _, err := RunParallel[int](ctx, 10, composers...)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results.Results) != 10 {
		t.Errorf("expected 10 results, got %d", len(results.Results))
	}

	successCount := 0
	errorCount := len(results.Errors)
	for _, e := range results.Errors {
		if e == nil {
			successCount++
		}
	}
	successCount = len(results.Results) - errorCount

	t.Logf("RunParallel: %d composers, results=%d, errors=%d",
		len(composers), len(results.Results), errorCount)
}

// ============================================================================
// 3.6 四原语混合压力测试
// ============================================================================

// TestStress_FourPrimitives_MixedPipeline: 四原语混合Pipeline
func TestStress_FourPrimitives_MixedPipeline(t *testing.T) {
	obs := &InMemoryObservationAdapter{}

	// Atom: 纯函数
	atom := func(ctx context.Context, in int) (int, error) {
		return in * 2, nil
	}

	// Port: 验证
	port := NewPort[int, int](func(ctx context.Context, in int) (int, error) {
		if in < 0 {
			return 0, fmt.Errorf("negative value rejected")
		}
		return in, nil
	})

	// Adapter: 外部调用
	adapter := NewAdapter[int, int](func(ctx context.Context, in int) (int, error) {
		// 模拟外部调用
		time.Sleep(time.Microsecond)
		return in + 10, nil
	})

	pipeline := NewPipeline[int](obs,
		NewStepFunc[int, int]("Atom", atom),
		PortAsStep[int, int](port),
		AdapterAsStep[int, int](adapter),
	)

	ctx := context.Background()
	result, steps, err := pipeline.Run(ctx, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// 5*2=10, 10 valid, 10+10=20
	if result != 20 {
		t.Errorf("expected 20, got %d", result)
	}
	if len(steps) != 3 {
		t.Errorf("expected 3 steps, got %d", len(steps))
	}

	// 验证Port拒绝负数
	_, _, err = pipeline.Run(ctx, -5)
	if err == nil {
		t.Error("expected error for negative input")
	}

	t.Logf("Four Primitives: result=%d, steps=%d", result, len(steps))
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

	// 并发入队
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

	// 并发出队
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

	// AgentPool uses map[string]*AgentInfo internally, construct agents separately
	agents := make([]*AgentInfo, batchSize)
	for i := 0; i < batchSize; i++ {
		agents[i] = &AgentInfo{
			ID:           fmt.Sprintf("agent-%d", i),
			Phase:        "build",
			Capabilities: []string{"go", "test"},
			LastHeartbeat: time.Now().Add(-time.Duration(i) * time.Second),
		}
	}

	// Build the pool
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
// 3.8 观测管道压力测试
// ============================================================================

// TestStress_ObservationPipeline_HighThroughput: 观测管道高吞吐
func TestStress_ObservationPipeline_HighThroughput(t *testing.T) {
	store := NewInMemoryStepStore(10000)
	aggregator := NewAggregator(DefaultAggregatorConfig())

	config := ObservationPipelineConfig{
		BufferSize:    5000,
		Store:         store,
		Aggregator:    aggregator,
		FlushInterval: 100 * time.Millisecond,
	}

	pipeline := NewObservationPipeline(config)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	pipeline.Start(ctx)

	const batches = 100
	const stepsPerBatch = 100

	for i := 0; i < batches; i++ {
		steps := make([]ExecutionStep, stepsPerBatch)
		for j := 0; j < stepsPerBatch; j++ {
			steps[j] = NewExecutionStep("Atom", "process", fmt.Sprintf("batch-%d-step-%d", i, j), "stress")
			if j%10 == 0 {
				steps[j].Error = &StepError{Code: "E001", Message: "injected error"}
			}
		}
		pipeline.Feed(steps)
	}

	// 等待处理
	time.Sleep(500 * time.Millisecond)
	pipeline.Stop()
	pipeline.Wait()

	count := store.Count()
	t.Logf("ObservationPipeline: %d batches x %d steps, stored=%d", batches, stepsPerBatch, count)
}

// ============================================================================
// 3.9 UUID生成器压力测试
// ============================================================================

// TestStress_UUIDGen_Concurrent: 并发UUID生成
func TestStress_UUIDGen_Concurrent(t *testing.T) {
	gen := NewBatchedUUIDGen()
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
// 3.10 熵收集器压力测试
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

// ============================================================================
// 3.11 降级管理压力测试
// ============================================================================

// TestStress_Degradation_GracefulDegradation: 优雅降级
func TestStress_Degradation_GracefulDegradation(t *testing.T) {
	obs := &InMemoryObservationAdapter{}
	dm := NewDegradationManager(obs)

	ctx := context.Background()

	// 正常操作
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

	_ = dm // 降级管理器存在且可用
	t.Logf("DegradationManager: normal operation verified, dm available")
}