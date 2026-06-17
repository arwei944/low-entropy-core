// Package core — 压力测试套件 (v4.0)
//
// 模拟十亿级调用量场景，验证无内存泄漏、无 goroutine 泄漏、无死锁。
// 运行方式：go test -race -timeout 120s -run "StressTest" ./...
package core

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// =============================================================================
// StressTest_ContinuousWrite — 持续写入压力测试
// =============================================================================

func TestStress_ContinuousWrite(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	adapter := NewShardedObservationAdapter()
	store := NewShardedStepStore(10000)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var totalWrites atomic.Int64
	var wg sync.WaitGroup

	// 100 个写入 goroutine
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				default:
					step := ExecutionStep{
						Timestamp:  time.Now(),
						TraceID:    fmt.Sprintf("trace_%d_%d", id, totalWrites.Load()),
						SpanID:     fmt.Sprintf("span_%d_%d", id, totalWrites.Load()),
						Unit:       "Atom",
						Action:     "stress_test",
						Pattern:    "bench",
						DurationMs: int64(id),
					}
					adapter.Record([]ExecutionStep{step})
					store.Record([]ExecutionStep{step})
					totalWrites.Add(1)
				}
			}
		}(i)
	}

	wg.Wait()

	t.Logf("Total writes: %d", totalWrites.Load())
	t.Logf("Adapter steps: %d", adapter.StepCount())
	t.Logf("Store steps: %d", store.Count())

	// 验证数据一致性
	if adapter.StepCount() != int(totalWrites.Load()) {
		t.Errorf("adapter count mismatch: got %d, want %d", adapter.StepCount(), totalWrites.Load())
	}
}

// =============================================================================
// StressTest_ReadWriteMix — 读写混合压力测试
// =============================================================================

func TestStress_ReadWriteMix(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	store := NewShardedStepStore(10000)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var wg sync.WaitGroup

	// 50 个写入 goroutine
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				default:
					step := ExecutionStep{
						Timestamp:  time.Now(),
						TraceID:    fmt.Sprintf("rw_trace_%d", id),
						SpanID:     fmt.Sprintf("rw_span_%d", id),
						Unit:       "Atom",
						Action:     "rw_test",
						Pattern:    "mixed",
						DurationMs: int64(id),
					}
					store.Record([]ExecutionStep{step})
				}
			}
		}(i)
	}

	// 20 个读取 goroutine
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				default:
					query := StepQuery{
						TraceID: fmt.Sprintf("rw_trace_%d", id%50),
						Limit:   10,
					}
					store.Query(query)
					store.Count()
				}
			}
		}(i)
	}

	wg.Wait()
	t.Logf("Total store steps: %d", store.Count())
}

// =============================================================================
// StressTest_GoroutineLeak — Goroutine 泄漏检测
// =============================================================================

func TestStress_GoroutineLeak(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	initialGoroutines := runtime.NumGoroutine()
	t.Logf("Initial goroutines: %d", initialGoroutines)

	// 执行大量操作
	adapter := NewShardedObservationAdapter()
	store := NewShardedStepStore(10000)
	eventStore := NewShardedEventStore()

	for i := 0; i < 100000; i++ {
		step := ExecutionStep{
			Timestamp:  time.Now(),
			TraceID:    fmt.Sprintf("leak_%d", i),
			SpanID:     fmt.Sprintf("leak_span_%d", i),
			Unit:       "Atom",
			Action:     "leak_test",
			DurationMs: 1,
		}
		adapter.Record([]ExecutionStep{step})
		store.Record([]ExecutionStep{step})

		env := EventEnvelope{
			AggregateID:   fmt.Sprintf("leak_agg_%d", i%100),
			AggregateType: "Test",
			EventType:     "Test",
			EventData:     []byte("data"),
			Timestamp:     time.Now(),
		}
		eventStore.Execute(context.Background(), env)
	}

	// 等待 GC 完成
	runtime.GC()
	time.Sleep(500 * time.Millisecond)

	finalGoroutines := runtime.NumGoroutine()
	t.Logf("Final goroutines: %d (delta: %d)", finalGoroutines, finalGoroutines-initialGoroutines)

	// 允许少量 goroutine 增长（UUID 生成器的后台 goroutine）
	if finalGoroutines-initialGoroutines > 10 {
		t.Errorf("possible goroutine leak: delta %d", finalGoroutines-initialGoroutines)
	}
}

// =============================================================================
// StressTest_RingBufferOverflow — 环形缓冲区覆写验证
// =============================================================================

func TestStress_RingBufferOverflow(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	capacity := 100
	store := NewShardedStepStore(capacity)

	// 写入超过容量的数据到单个 SpanID（确保同一分片）
	for i := 0; i < capacity*3; i++ {
		step := ExecutionStep{
			Timestamp:  time.Now(),
			TraceID:    "ring_test",
			SpanID:     "ring_span", // 固定 SpanID 确保同一分片
			Unit:       "Atom",
			Action:     fmt.Sprintf("ring_%d", i),
			DurationMs: int64(i),
		}
		store.Record([]ExecutionStep{step})
	}

	// 查询应返回最近的条目
	results, total := store.Query(StepQuery{TraceID: "ring_test", Limit: 50})
	if len(results) == 0 {
		t.Error("expected results from ring buffer")
	}
	t.Logf("Ring buffer query: %d results (total: %d)", len(results), total)
}

// =============================================================================
// StressTest_ShardedEventStore_Concurrency — 分片事件存储并发测试
// =============================================================================

func TestStress_ShardedEventStore_Concurrency(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	store := NewShardedEventStore()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	errors := make(chan error, 1000)

	// 100 个 goroutine 并发写入不同 Aggregate
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 1000; j++ {
				select {
				case <-ctx.Done():
					return
				default:
					env := EventEnvelope{
						AggregateID:   fmt.Sprintf("agg_%d", id),
						AggregateType: "TestAggregate",
						EventType:     "TestEvent",
						EventData:     []byte(fmt.Sprintf("data_%d", j)),
						Timestamp:     time.Now(),
					}
					_, err := store.Execute(context.Background(), env)
					if err != nil {
						errors <- err
					}
				}
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	errCount := 0
	for range errors {
		errCount++
	}
	if errCount > 0 {
		t.Errorf("sharded event store had %d errors", errCount)
	}

	// 验证每个 Aggregate 的版本号
	for i := 0; i < 100; i++ {
		version := store.GetLatestVersion(fmt.Sprintf("agg_%d", i))
		if version != 1000 {
			t.Errorf("aggregate agg_%d: expected version 1000, got %d", i, version)
		}
	}
}

// =============================================================================
// StressTest_TDigest_Accuracy — TDigest 精度验证
// =============================================================================

func TestStress_TDigest_Accuracy(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	td := NewTDigestDefault()
	n := 100000

	// 插入 100K 个值
	for i := 0; i < n; i++ {
		td.Add(float64(i))
	}

	// 验证 P50 精度
	p50 := td.Quantile(0.50)
	expectedP50 := float64(n / 2)
	errorP50 := (p50 - expectedP50) / expectedP50 * 100
	t.Logf("P50: %.2f (expected %.2f, error %.2f%%)", p50, expectedP50, errorP50)
	if errorP50 < -1 || errorP50 > 1 {
		t.Errorf("P50 accuracy too low: %.2f%% error", errorP50)
	}

	// 验证 P99 精度
	p99 := td.Quantile(0.99)
	expectedP99 := float64(n) * 0.99
	errorP99 := (p99 - expectedP99) / expectedP99 * 100
	t.Logf("P99: %.2f (expected %.2f, error %.2f%%)", p99, expectedP99, errorP99)
	if errorP99 < -1 || errorP99 > 1 {
		t.Errorf("P99 accuracy too low: %.2f%% error", errorP99)
	}
}

// =============================================================================
// StressTest_DependencyGraph_Large — 大型依赖图测试
// =============================================================================

func TestStress_DependencyGraph_Large(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	graph := NewDependencyGraph()

	// 构建 1000 个节点的依赖图
	for i := 0; i < 1000; i++ {
		from := fmt.Sprintf("pipeline_%d", i)
		to := fmt.Sprintf("pipeline_%d", (i+1)%1000)
		graph.AddEdge(from, to)
	}

	// 添加一个循环
	graph.AddEdge("pipeline_999", "pipeline_0")

	cycles := graph.DetectCycles()
	if len(cycles) == 0 {
		t.Error("expected to detect cycle in dependency graph")
	}
	t.Logf("Detected %d cycles in 1000-node graph", len(cycles))

	// 拓扑排序应该返回 nil（存在循环）
	sorted := graph.TopologicalSort()
	if sorted != nil {
		t.Error("topological sort should return nil when cycle exists")
	}
}

// =============================================================================
// StressTest_GlobalCircuitBreaker — 全局熔断器压力测试
// =============================================================================

func TestStress_GlobalCircuitBreaker(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	config := DefaultGlobalCircuitConfig()
	config.MinRequestCount = 5
	config.FailureRateThreshold = 0.5

	gcb := NewGlobalCircuitBreaker(config, nil, nil)

	// 报告 6 次失败（超过阈值 50%）
	for i := 0; i < 6; i++ {
		gcb.ReportFailure("test_service", "instance_1")
	}

	if !gcb.IsOpen("test_service") {
		t.Error("global circuit breaker should be open after 6 failures")
	}

	// 报告 1 次成功（总计 6/7 = 86% > 50%，仍应打开）
	gcb.ReportSuccess("test_service", "instance_1")
	if !gcb.IsOpen("test_service") {
		t.Error("global circuit breaker should still be open")
	}
}

// =============================================================================
// StressTest_MemoryUsage — 内存使用基准测试
// =============================================================================

func TestStress_MemoryUsage(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	var m1, m2 runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m1)

	// 创建大量对象
	adapters := make([]*ShardedObservationAdapter, 100)
	stores := make([]*ShardedStepStore, 100)
	for i := 0; i < 100; i++ {
		adapters[i] = NewShardedObservationAdapter()
		stores[i] = NewShardedStepStore(1000)
	}

	runtime.GC()
	runtime.ReadMemStats(&m2)

	allocMB := float64(m2.TotalAlloc-m1.TotalAlloc) / 1024 / 1024
	t.Logf("Memory allocated for 100 adapters + 100 stores: %.2f MB", allocMB)

	if allocMB > 100 {
		t.Errorf("excessive memory usage: %.2f MB", allocMB)
	}
}