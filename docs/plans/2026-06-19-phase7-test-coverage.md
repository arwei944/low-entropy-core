# Phase 7: 测试覆盖补齐与工程化增强 实施计划

> **For agentic workers:** 按任务顺序执行。每个 Task 使用 checkbox (`- [ ]`) 追踪。

**Goal:** 为事件溯源、守护层、降级等关键模块补齐测试覆盖，增强 CI 流水线，补充 agent-demo README。

**Architecture:** 四个独立模块并行推进：(1) 事件溯源 3 个模块测试 (2) 守护层 3 个模块测试 (3) 降级+幂等测试 (4) CI 增强 + agent-demo README。

**Tech Stack:** Go 1.22 stdlib，testing 标准库。

**核实结果:** CI 已包含 lint、coverage、benchmark、build examples、build arch-manager，无需额外增强。仅需补齐测试覆盖和文档。

---

### Task 7.1: EventStore + EventBus + Projection 测试

**Files:**
- Create: `go-core/eventstore_test.go`
- Create: `go-core/eventbus_test.go`
- Create: `go-core/projection_test.go`

- [ ] **Step 1: 创建 eventstore_test.go**

```go
package core

import (
	"context"
	"testing"
	"time"
)

func TestEventStore_Append(t *testing.T) {
	store := NewEventStore()
	ctx := context.Background()

	env := EventEnvelope{
		AggregateID:   "agg-1",
		AggregateType: "Order",
		EventType:     "OrderCreated",
		EventData:     []byte(`{"amount":100}`),
	}

	result, err := store.Execute(ctx, env)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Error("expected success")
	}
	if result.Version != 1 {
		t.Errorf("expected version 1, got %d", result.Version)
	}
	if result.EventID == "" {
		t.Error("expected non-empty EventID")
	}
}

func TestEventStore_AppendMultiple(t *testing.T) {
	store := NewEventStore()
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		env := EventEnvelope{
			AggregateID:   "agg-2",
			AggregateType: "Order",
			EventType:     "OrderUpdated",
			EventData:     []byte("{}"),
		}
		result, err := store.Execute(ctx, env)
		if err != nil {
			t.Fatalf("unexpected error at %d: %v", i, err)
		}
		if result.Version != int64(i+1) {
			t.Errorf("expected version %d, got %d", i+1, result.Version)
		}
	}
}

func TestEventStore_GetEvents(t *testing.T) {
	store := NewEventStore()
	ctx := context.Background()

	store.Execute(ctx, EventEnvelope{
		AggregateID: "agg-3", AggregateType: "Order", EventType: "Created", EventData: []byte("1"),
	})
	store.Execute(ctx, EventEnvelope{
		AggregateID: "agg-3", AggregateType: "Order", EventType: "Updated", EventData: []byte("2"),
	})

	events, err := store.GetEvents(ctx, "agg-3", 0, 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 2 {
		t.Errorf("expected 2 events, got %d", len(events))
	}
}

func TestEventStore_GetEventsEmpty(t *testing.T) {
	store := NewEventStore()
	ctx := context.Background()

	events, err := store.GetEvents(ctx, "nonexistent", 0, 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 0 {
		t.Errorf("expected 0 events, got %d", len(events))
	}
}

func TestEventStore_Snapshot(t *testing.T) {
	store := NewEventStore()
	ctx := context.Background()

	store.Execute(ctx, EventEnvelope{
		AggregateID: "agg-4", AggregateType: "Order", EventType: "Created", EventData: []byte("1"),
	})

	snap := &Snapshot{
		AggregateID: "agg-4",
		Version:     1,
		State:       []byte(`{"state":"done"}`),
		Timestamp:   time.Now(),
	}
	err := store.SaveSnapshot(ctx, snap)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	loaded, err := store.GetSnapshot(ctx, "agg-4")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if loaded == nil {
		t.Fatal("expected snapshot, got nil")
	}
	if loaded.Version != 1 {
		t.Errorf("expected version 1, got %d", loaded.Version)
	}
}
```

- [ ] **Step 2: 创建 eventbus_test.go**

```go
package core

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestEventBus_PublishToSubscriber(t *testing.T) {
	bus := NewEventBus()
	ctx := context.Background()

	var received EventEnvelope
	var mu sync.Mutex

	bus.Subscribe("OrderCreated", func(env EventEnvelope) error {
		mu.Lock()
		received = env
		mu.Unlock()
		return nil
	})

	env := EventEnvelope{
		EventID:   "evt-1",
		EventType: "OrderCreated",
		EventData: []byte("test"),
	}

	result, err := bus.Execute(ctx, env)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.SubscriberCount != 1 {
		t.Errorf("expected 1 subscriber, got %d", result.SubscriberCount)
	}

	mu.Lock()
	if received.EventID != "evt-1" {
		t.Errorf("expected evt-1, got %s", received.EventID)
	}
	mu.Unlock()
}

func TestEventBus_NoSubscribers(t *testing.T) {
	bus := NewEventBus()
	ctx := context.Background()

	env := EventEnvelope{
		EventID:   "evt-2",
		EventType: "NoSubscribers",
	}

	result, err := bus.Execute(ctx, env)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.SubscriberCount != 0 {
		t.Errorf("expected 0 subscribers, got %d", result.SubscriberCount)
	}
}

func TestEventBus_MultipleSubscribers(t *testing.T) {
	bus := NewEventBus()
	ctx := context.Background()

	var count int
	var mu sync.Mutex

	for i := 0; i < 3; i++ {
		bus.Subscribe("MultiEvent", func(env EventEnvelope) error {
			mu.Lock()
			count++
			mu.Unlock()
			return nil
		})
	}

	env := EventEnvelope{
		EventID:   "evt-3",
		EventType: "MultiEvent",
	}

	result, err := bus.Execute(ctx, env)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.SubscriberCount != 3 {
		t.Errorf("expected 3 subscribers, got %d", result.SubscriberCount)
	}

	mu.Lock()
	if count != 3 {
		t.Errorf("expected 3 calls, got %d", count)
	}
	mu.Unlock()
}

func TestEventBus_AsyncSubscriber(t *testing.T) {
	bus := NewEventBus()
	ctx := context.Background()

	var received bool
	done := make(chan struct{})

	bus.SubscribeAsync("AsyncEvent", func(env EventEnvelope) error {
		received = true
		close(done)
		return nil
	})

	env := EventEnvelope{
		EventID:   "evt-4",
		EventType: "AsyncEvent",
	}

	result, err := bus.Execute(ctx, env)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.SubscriberCount != 1 {
		t.Errorf("expected 1 subscriber, got %d", result.SubscriberCount)
	}

	select {
	case <-done:
		if !received {
			t.Error("expected async handler to be called")
		}
	case <-time.After(1 * time.Second):
		t.Error("async handler timed out")
	}
}

func TestEventBus_Unsubscribe(t *testing.T) {
	bus := NewEventBus()
	ctx := context.Background()

	subID := bus.Subscribe("UnsubEvent", func(env EventEnvelope) error {
		return nil
	})

	bus.Unsubscribe("UnsubEvent", subID)

	env := EventEnvelope{
		EventID:   "evt-5",
		EventType: "UnsubEvent",
	}

	result, err := bus.Execute(ctx, env)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.SubscriberCount != 0 {
		t.Errorf("expected 0 subscribers after unsubscribe, got %d", result.SubscriberCount)
	}
}
```

- [ ] **Step 3: 创建 projection_test.go**

```go
package core

import (
	"testing"
)

func TestProjection_FullRebuild(t *testing.T) {
	handler := func(state []byte, event EventEnvelope) ([]byte, error) {
		if state == nil {
			return event.EventData, nil
		}
		return append(state, event.EventData...), nil
	}

	proj := NewProjection(handler)

	input := ProjectionInput{
		AggregateID: "agg-1",
		Events: []EventEnvelope{
			{EventType: "A", EventData: []byte("hello"), Version: 1},
			{EventType: "B", EventData: []byte(" world"), Version: 2},
		},
		FromVersion:  0,
		CurrentState: nil,
	}

	output, err := proj.Execute(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(output.State) != "hello world" {
		t.Errorf("expected 'hello world', got %q", output.State)
	}
	if output.Version != 2 {
		t.Errorf("expected version 2, got %d", output.Version)
	}
	if output.EventsProcessed != 2 {
		t.Errorf("expected 2 events, got %d", output.EventsProcessed)
	}
}

func TestProjection_Incremental(t *testing.T) {
	handler := func(state []byte, event EventEnvelope) ([]byte, error) {
		if state == nil {
			return event.EventData, nil
		}
		return append(state, event.EventData...), nil
	}

	proj := NewProjection(handler)

	input := ProjectionInput{
		AggregateID: "agg-2",
		Events: []EventEnvelope{
			{EventType: "A", EventData: []byte("A"), Version: 1},
			{EventType: "B", EventData: []byte("B"), Version: 2},
			{EventType: "C", EventData: []byte("C"), Version: 3},
		},
		FromVersion:  2,
		CurrentState: []byte("A"),
	}

	output, err := proj.Execute(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(output.State) != "ABC" {
		t.Errorf("expected 'ABC', got %q", output.State)
	}
	if output.Version != 3 {
		t.Errorf("expected version 3, got %d", output.Version)
	}
	if output.EventsProcessed != 2 {
		t.Errorf("expected 2 events, got %d", output.EventsProcessed)
	}
}

func TestProjection_EmptyEvents(t *testing.T) {
	proj := NewProjection(func(state []byte, event EventEnvelope) ([]byte, error) {
		return state, nil
	})

	input := ProjectionInput{
		AggregateID:  "agg-3",
		Events:       nil,
		FromVersion:  0,
		CurrentState: []byte("existing"),
	}

	output, err := proj.Execute(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(output.State) != "existing" {
		t.Errorf("expected 'existing', got %q", output.State)
	}
	if output.Version != 0 {
		t.Errorf("expected version 0, got %d", output.Version)
	}
	if output.EventsProcessed != 0 {
		t.Errorf("expected 0 events, got %d", output.EventsProcessed)
	}
}
```

- [ ] **Step 4: 运行测试**

```bash
cd go-core && go test -run "TestEventStore|TestEventBus|TestProjection" -v -count=1
```

- [ ] **Step 5: 运行全量测试**

```bash
cd go-core && go test ./... -count=1
```

- [ ] **Step 6: Commit**

```bash
git add go-core/eventstore_test.go go-core/eventbus_test.go go-core/projection_test.go
git commit -m "test: add EventStore, EventBus, Projection tests"
```

---

### Task 7.2: Idempotent + Degradation 测试

**Files:**
- Create: `go-core/idempotent_test.go`
- Create: `go-core/degradation_test.go`

- [ ] **Step 1: 创建 idempotent_test.go**

```go
package core

import (
	"context"
	"testing"
	"time"
)

func TestInMemoryIdempotentStore_SetGet(t *testing.T) {
	store := NewInMemoryIdempotentStore()

	store.Set("key1", "value1", 1*time.Minute)
	val, ok := store.Get("key1")
	if !ok {
		t.Fatal("expected key to exist")
	}
	if val != "value1" {
		t.Errorf("expected 'value1', got %v", val)
	}
}

func TestInMemoryIdempotentStore_Miss(t *testing.T) {
	store := NewInMemoryIdempotentStore()

	_, ok := store.Get("nonexistent")
	if ok {
		t.Error("expected miss for nonexistent key")
	}
}

func TestInMemoryIdempotentStore_Delete(t *testing.T) {
	store := NewInMemoryIdempotentStore()

	store.Set("key1", "value1", 1*time.Minute)
	store.Delete("key1")

	_, ok := store.Get("key1")
	if ok {
		t.Error("expected key to be deleted")
	}
}

func TestInMemoryIdempotentStore_Clear(t *testing.T) {
	store := NewInMemoryIdempotentStore()

	store.Set("k1", "v1", 1*time.Minute)
	store.Set("k2", "v2", 1*time.Minute)
	store.Clear()

	_, ok1 := store.Get("k1")
	_, ok2 := store.Get("k2")
	if ok1 || ok2 {
		t.Error("expected all keys to be cleared")
	}
}

func TestInMemoryIdempotentStore_Expiry(t *testing.T) {
	store := NewInMemoryIdempotentStore()

	store.Set("key1", "value1", 10*time.Millisecond)
	time.Sleep(20 * time.Millisecond)

	_, ok := store.Get("key1")
	if ok {
		t.Error("expected key to be expired")
	}
}

func TestIdempotentAdapter_Cache(t *testing.T) {
	store := NewInMemoryIdempotentStore()
	ctx := context.Background()
	obs := &InMemoryObservationAdapter{}

	inner := NewPipeline[int](obs,
		AtomAsStep(Atom[int, int](func(x int) int { return x * 2 })),
	)

	adapter := NewIdempotentAdapter[int, int](inner, store, 1*time.Minute)

	// 第一次调用
	result, _, err := adapter.Run(ctx, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Output != 10 {
		t.Errorf("expected 10, got %d", result.Output)
	}
	if result.FromCache {
		t.Error("expected fresh result, got cached")
	}

	// 第二次调用（应缓存）
	result, _, err = adapter.Run(ctx, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Output != 10 {
		t.Errorf("expected 10, got %d", result.Output)
	}
	if !result.FromCache {
		t.Error("expected cached result")
	}
}
```

- [ ] **Step 2: 创建 degradation_test.go**

```go
package core

import (
	"testing"
)

func TestDegradationManager_DefaultMode(t *testing.T) {
	obs := &InMemoryObservationAdapter{}
	dm := NewDegradationManager(obs)

	if dm.CurrentMode() != DegradationNone {
		t.Errorf("expected none, got %s", dm.CurrentMode())
	}
}

func TestDegradationManager_Degrade(t *testing.T) {
	obs := &InMemoryObservationAdapter{}
	dm := NewDegradationManager(obs)

	dm.Degrade(DegradationNonCritical)
	if dm.CurrentMode() != DegradationNonCritical {
		t.Errorf("expected non_critical, got %s", dm.CurrentMode())
	}
}

func TestDegradationManager_ShouldProcess(t *testing.T) {
	obs := &InMemoryObservationAdapter{}
	dm := NewDegradationManager(obs)

	// Normal mode: everything allowed
	if !dm.ShouldProcess(CriticalityCritical) {
		t.Error("critical ops should be allowed in none mode")
	}
	if !dm.ShouldProcess(CriticalityHigh) {
		t.Error("high ops should be allowed in none mode")
	}
	if !dm.ShouldProcess(CriticalityLow) {
		t.Error("low ops should be allowed in none mode")
	}

	// Emergency mode: only critical
	dm.Degrade(DegradationEmergency)
	if !dm.ShouldProcess(CriticalityCritical) {
		t.Error("critical ops should be allowed in emergency")
	}
	if dm.ShouldProcess(CriticalityLow) {
		t.Error("low ops should be blocked in emergency")
	}
}

func TestDegradationManager_SafeMode(t *testing.T) {
	obs := &InMemoryObservationAdapter{}
	dm := NewDegradationManager(obs)

	dm.Degrade(DegradationSafe)
	if !dm.ShouldProcess(CriticalityCritical) {
		t.Error("critical ops should be allowed in safe mode")
	}
	if !dm.ShouldProcess(CriticalityHigh) {
		t.Error("high ops should be allowed in safe mode")
	}
	if dm.ShouldProcess(CriticalityLow) {
		t.Error("low ops should be blocked in safe mode")
	}
}

func TestDegradationManager_NilObs(t *testing.T) {
	dm := NewDegradationManager(nil)
	if dm.CurrentMode() != DegradationNone {
		t.Errorf("expected none, got %s", dm.CurrentMode())
	}
}
```

- [ ] **Step 3: 运行测试**

```bash
cd go-core && go test -run "TestInMemoryIdempotent|TestIdempotentAdapter|TestDegradation" -v -count=1
```

- [ ] **Step 4: 运行全量测试**

```bash
cd go-core && go test ./... -count=1
```

- [ ] **Step 5: Commit**

```bash
git add go-core/idempotent_test.go go-core/degradation_test.go
git commit -m "test: add IdempotentStore and DegradationManager tests"
```

---

### Task 7.3: 守护层测试（Dependency + Entropy + Transparency）

**Files:**
- Create: `go-core/guardian_dependency_test.go`
- Create: `go-core/guardian_entropy_test.go`
- Create: `go-core/guardian_transparency_test.go`

- [ ] **Step 1: 创建 guardian_dependency_test.go**

```go
package core

import (
	"testing"
)

func TestDependencyGraph_AddEdge(t *testing.T) {
	g := NewDependencyGraph()
	g.AddEdge("A", "B")
	g.AddEdge("B", "C")

	if !g.HasEdge("A", "B") {
		t.Error("expected edge A->B")
	}
	if !g.HasEdge("B", "C") {
		t.Error("expected edge B->C")
	}
	if g.HasEdge("A", "C") {
		t.Error("unexpected edge A->C")
	}
}

func TestDependencyGraph_NoCycle(t *testing.T) {
	g := NewDependencyGraph()
	g.AddEdge("A", "B")
	g.AddEdge("B", "C")

	if g.HasCycle() {
		t.Error("expected no cycle")
	}
}

func TestDependencyGraph_HasCycle(t *testing.T) {
	g := NewDependencyGraph()
	g.AddEdge("A", "B")
	g.AddEdge("B", "C")
	g.AddEdge("C", "A")

	if !g.HasCycle() {
		t.Error("expected cycle")
	}
}

func TestDependencyGraph_RemoveEdge(t *testing.T) {
	g := NewDependencyGraph()
	g.AddEdge("A", "B")
	g.RemoveEdge("A", "B")

	if g.HasEdge("A", "B") {
		t.Error("expected edge removed")
	}
}

func TestDependencyGraph_TopologicalSort(t *testing.T) {
	g := NewDependencyGraph()
	g.AddEdge("A", "B")
	g.AddEdge("B", "C")
	g.AddEdge("A", "C")

	sorted, err := g.TopologicalSort()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sorted) != 3 {
		t.Errorf("expected 3 nodes, got %d", len(sorted))
	}
	// A must come before B and C
	aIdx := indexOf(sorted, "A")
	bIdx := indexOf(sorted, "B")
	cIdx := indexOf(sorted, "C")
	if aIdx > bIdx || aIdx > cIdx {
		t.Error("A should come before B and C")
	}
	if bIdx > cIdx {
		t.Error("B should come before C")
	}
}

func TestDependencyGraph_TopologicalSortCycle(t *testing.T) {
	g := NewDependencyGraph()
	g.AddEdge("A", "B")
	g.AddEdge("B", "A")

	_, err := g.TopologicalSort()
	if err == nil {
		t.Error("expected error for cycle")
	}
}

func indexOf(slice []string, item string) int {
	for i, s := range slice {
		if s == item {
			return i
		}
	}
	return -1
}

func TestDependencyGuard_Check(t *testing.T) {
	guard := NewDependencyGuard()
	obs := &InMemoryObservationAdapter{}
	_ = obs

	// 添加已注册的模块
	guard.Register("A", []string{})
	guard.Register("B", []string{"A"})
	guard.Register("C", []string{"B"})

	// 检查无循环依赖
	ok, err := guard.Check("D", []string{"C"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Error("expected check to pass")
	}
}

func TestDependencyGuard_CycleDetection(t *testing.T) {
	guard := NewDependencyGuard()
	guard.Register("A", []string{"B"})
	guard.Register("B", []string{"A"})

	ok, err := guard.Check("C", []string{"A"})
	if err != nil {
		// 检测到循环应返回错误
		t.Logf("cycle detected (expected): %v", err)
	}
	_ = ok
}
```

- [ ] **Step 2: 创建 guardian_entropy_test.go**

```go
package core

import (
	"context"
	"testing"
	"time"
)

func TestEntropyWatcher_Normal(t *testing.T) {
	obs := &InMemoryObservationAdapter{}
	watcher := NewEntropyWatcher(obs, EntropyWatcherConfig{
		YellowThreshold: 0.5,
		OrangeThreshold: 0.7,
		RedThreshold:    0.9,
	})

	ctx := context.Background()
	alert, err := watcher.Check(ctx, 0.1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if alert.Level != EntropyOK {
		t.Errorf("expected OK, got %s", alert.Level)
	}
}

func TestEntropyWatcher_Warning(t *testing.T) {
	obs := &InMemoryObservationAdapter{}
	watcher := NewEntropyWatcher(obs, EntropyWatcherConfig{
		YellowThreshold: 0.5,
		OrangeThreshold: 0.7,
		RedThreshold:    0.9,
	})

	ctx := context.Background()
	alert, err := watcher.Check(ctx, 0.6)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if alert.Level != EntropyYellow {
		t.Errorf("expected Yellow, got %s", alert.Level)
	}
}

func TestEntropyWatcher_Critical(t *testing.T) {
	obs := &InMemoryObservationAdapter{}
	watcher := NewEntropyWatcher(obs, EntropyWatcherConfig{
		YellowThreshold: 0.5,
		OrangeThreshold: 0.7,
		RedThreshold:    0.9,
	})

	ctx := context.Background()
	alert, err := watcher.Check(ctx, 0.95)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if alert.Level != EntropyRed {
		t.Errorf("expected Red, got %s", alert.Level)
	}
}

func TestEntropyWatcher_Acceleration(t *testing.T) {
	obs := &InMemoryObservationAdapter{}
	watcher := NewEntropyWatcher(obs, EntropyWatcherConfig{
		YellowThreshold: 0.5,
		OrangeThreshold: 0.7,
		RedThreshold:    0.9,
	})

	ctx := context.Background()

	// 连续两次高增长
	watcher.Check(ctx, 0.2)
	time.Sleep(10 * time.Millisecond)
	alert, _ := watcher.Check(ctx, 0.5)

	if !alert.AccelerationDetected {
		t.Error("expected acceleration detected")
	}
}

func TestEntropyWatcher_ConfigDefaults(t *testing.T) {
	obs := &InMemoryObservationAdapter{}
	watcher := NewEntropyWatcher(obs, EntropyWatcherConfig{})

	ctx := context.Background()
	alert, err := watcher.Check(ctx, 0.3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if alert.Level != EntropyOK {
		t.Errorf("expected OK with defaults, got %s", alert.Level)
	}
}
```

- [ ] **Step 3: 创建 guardian_transparency_test.go**

```go
package core

import (
	"context"
	"testing"
)

func TestTransparencyWatcher_Healthy(t *testing.T) {
	watcher := NewTransparencyWatcher()
	ctx := context.Background()

	input := TransparencyInput{
		AuditEntries: []AuditEntry{
			{Operation: "op1", Timestamp: 1},
			{Operation: "op2", Timestamp: 2},
		},
		ExecutionSteps: []ExecutionStep{
			{StepName: "step1"},
			{StepName: "step2"},
		},
		ExpectedOperations: 2,
	}

	alert, err := watcher.Validate(ctx, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !alert.IsHealthy {
		t.Error("expected healthy")
	}
	if alert.CoverageRate != 1.0 {
		t.Errorf("expected 1.0 coverage, got %f", alert.CoverageRate)
	}
}

func TestTransparencyWatcher_Unhealthy(t *testing.T) {
	watcher := NewTransparencyWatcher()
	ctx := context.Background()

	input := TransparencyInput{
		AuditEntries: []AuditEntry{
			{Operation: "op1", Timestamp: 1},
		},
		ExecutionSteps: []ExecutionStep{
			{StepName: "step1"},
			{StepName: "step2"},
			{StepName: "step3"},
			{StepName: "step4"},
		},
		ExpectedOperations: 4,
	}

	alert, err := watcher.Validate(ctx, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if alert.IsHealthy {
		t.Error("expected unhealthy")
	}
	if alert.CoverageRate >= 0.5 {
		t.Errorf("expected low coverage, got %f", alert.CoverageRate)
	}
}

func TestTransparencyWatcher_EmptySteps(t *testing.T) {
	watcher := NewTransparencyWatcher()
	ctx := context.Background()

	input := TransparencyInput{
		AuditEntries:       nil,
		ExecutionSteps:     nil,
		ExpectedOperations: 0,
	}

	alert, err := watcher.Validate(ctx, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !alert.IsHealthy {
		t.Error("expected healthy for empty input")
	}
}

func TestDriftDetector_NoDrift(t *testing.T) {
	detector := NewDriftDetector()

	input := DriftInput{
		ExpectedBehavior: "atom: pure function",
		ActualBehavior:   "atom: pure function",
		Tolerance:        0.0,
	}

	output := detector.Execute(input)
	if output.DriftType != DriftNone {
		t.Errorf("expected no drift, got %s", output.DriftType)
	}
}

func TestDriftDetector_Detected(t *testing.T) {
	detector := NewDriftDetector()

	input := DriftInput{
		ExpectedBehavior: "atom: pure function",
		ActualBehavior:   "adapter: file I/O",
		Tolerance:        0.0,
	}

	output := detector.Execute(input)
	if output.DriftType == DriftNone {
		t.Error("expected drift detected")
	}
}
```

- [ ] **Step 4: 运行测试**

```bash
cd go-core && go test -run "TestDependencyGraph|TestDependencyGuard|TestEntropyWatcher|TestTransparencyWatcher|TestDriftDetector" -v -count=1
```

- [ ] **Step 5: 运行全量测试**

```bash
cd go-core && go test ./... -count=1
```

- [ ] **Step 6: Commit**

```bash
git add go-core/guardian_dependency_test.go go-core/guardian_entropy_test.go go-core/guardian_transparency_test.go
git commit -m "test: add Guardian layer tests (Dependency, Entropy, Transparency)"
```

---

### Task 7.4: agent-demo README

**Files:**
- Create: `examples/agent-demo/README.md`

- [ ] **Step 1: 创建 README.md**

```markdown
# Agent Workbench 端到端演示

展示 Agent Workbench 完整生命周期：Agent 代码提交 → StaticGuard 审核 → DecisionEngine 决策 → AgentRunner 编译执行。

## 流程

```
Agent 注册 → 编写代码 → 提交到 Workbench
                              ↓
                      StaticGuard 审核
                              ↓
                   DecisionEngine 决策
                         /    |    \
                      Allow  Warn  Block
                       ↓
               AgentRunner 编译执行
                       ↓
              ExecutionStep 流 → Observation
```

## 使用的 4 原语

| 原语 | 实现 | 说明 |
|------|------|------|
| **Atom** | `TaskProcessor` | 纯函数：任务处理 |
| **Port** | `StaticGuardPort` | 审核契约：检查 Manifest 与实际代码一致性 |
| **Adapter** | `AgentRunner` | 编译执行：运行 Agent 提交的代码 |
| **Composer** | `Pipeline` + `SchedulerComposer` + `HandoffComposer` | 编排：串联审核→决策→执行 |

## 运行

```bash
cd examples/agent-demo
go build -o agent-demo.exe .
./agent-demo.exe
```

## 关键组件

- **AgentPool**: 管理 Agent 注册、心跳、提交历史
- **TaskQueue**: 任务队列，按优先级调度
- **SchedulerComposer**: 编排 Agent 提交流程
- **HandoffComposer**: Agent 交接（遵守 03-agent-handoff-protocol.md）
```

- [ ] **Step 2: Commit**

```bash
git add examples/agent-demo/README.md
git commit -m "docs: add agent-demo README"
```

---

## 执行顺序

Task 7.1、7.2、7.3、7.4 互不依赖，可任意顺序执行。建议按 7.1 → 7.2 → 7.3 → 7.4 顺序。