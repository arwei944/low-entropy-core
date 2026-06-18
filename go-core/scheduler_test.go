//go:build lecore_tier3 || lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

package core

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

// ──────────────────────────────────────────────
// AgentPool Tests
// ──────────────────────────────────────────────

func TestAgentPool_Add(t *testing.T) {
	pool := NewAgentPool()
	agent := &AgentInfo{ID: "agent-1", Capabilities: []string{"read"}, Phase: "coding"}

	err := pool.Add(agent)
	if err != nil {
		t.Fatalf("add failed: %v", err)
	}
	if pool.Count() != 1 {
		t.Errorf("expected count=1, got %d", pool.Count())
	}
}

func TestAgentPool_AddDuplicate(t *testing.T) {
	pool := NewAgentPool()
	pool.Add(&AgentInfo{ID: "agent-1", Phase: "coding"})
	err := pool.Add(&AgentInfo{ID: "agent-1", Phase: "testing"})
	if err == nil {
		t.Fatal("expected error for duplicate agent")
	}
}

func TestAgentPool_Remove(t *testing.T) {
	pool := NewAgentPool()
	pool.Add(&AgentInfo{ID: "agent-1", Phase: "coding"})
	pool.Remove("agent-1")
	if pool.Count() != 0 {
		t.Errorf("expected count=0, got %d", pool.Count())
	}
}

func TestAgentPool_UpdateStatus(t *testing.T) {
	pool := NewAgentPool()
	pool.Add(&AgentInfo{ID: "agent-1", Phase: "coding"})

	err := pool.UpdateStatus("agent-1", AgentStatusBusy)
	if err != nil {
		t.Fatalf("update failed: %v", err)
	}

	agent, _ := pool.Get("agent-1")
	if agent.Status != AgentStatusBusy {
		t.Errorf("expected status=busy, got %s", agent.Status)
	}
}

func TestAgentPool_Heartbeat(t *testing.T) {
	pool := NewAgentPool()
	pool.Add(&AgentInfo{ID: "agent-1", Phase: "coding"})

	// Set status to offline, then heartbeat should bring it back
	pool.UpdateStatus("agent-1", AgentStatusOffline)
	pool.Heartbeat("agent-1")

	agent, _ := pool.Get("agent-1")
	if agent.Status != AgentStatusIdle {
		t.Errorf("expected status=idle after heartbeat, got %s", agent.Status)
	}
}

func TestAgentPool_ListAvailable(t *testing.T) {
	pool := NewAgentPool()
	pool.Add(&AgentInfo{ID: "agent-1", Phase: "coding", Status: AgentStatusIdle})
	pool.Add(&AgentInfo{ID: "agent-2", Phase: "coding", Status: AgentStatusBusy})
	pool.Add(&AgentInfo{ID: "agent-3", Phase: "testing", Status: AgentStatusIdle})

	available := pool.ListAvailable()
	if len(available) != 2 {
		t.Errorf("expected 2 available agents, got %d", len(available))
	}
}

func TestAgentPool_ListByCapability(t *testing.T) {
	pool := NewAgentPool()
	pool.Add(&AgentInfo{ID: "a-1", Capabilities: []string{"read", "write"}, Phase: "coding", Status: AgentStatusIdle})
	pool.Add(&AgentInfo{ID: "a-2", Capabilities: []string{"read"}, Phase: "coding", Status: AgentStatusIdle})
	pool.Add(&AgentInfo{ID: "a-3", Capabilities: []string{"deploy"}, Phase: "testing", Status: AgentStatusIdle})

	writeAgents := pool.ListByCapability("write")
	if len(writeAgents) != 1 {
		t.Errorf("expected 1 agent with write capability, got %d", len(writeAgents))
	}

	readAgents := pool.ListByCapability("read")
	if len(readAgents) != 2 {
		t.Errorf("expected 2 agents with read capability, got %d", len(readAgents))
	}
}

func TestAgentPool_ListByPhase(t *testing.T) {
	pool := NewAgentPool()
	pool.Add(&AgentInfo{ID: "a-1", Phase: "coding", Status: AgentStatusIdle})
	pool.Add(&AgentInfo{ID: "a-2", Phase: "coding", Status: AgentStatusIdle})
	pool.Add(&AgentInfo{ID: "a-3", Phase: "testing", Status: AgentStatusIdle})

	codingAgents := pool.ListByPhase("coding")
	if len(codingAgents) != 2 {
		t.Errorf("expected 2 coding agents, got %d", len(codingAgents))
	}
}

func TestAgentPool_AutoMarkOffline(t *testing.T) {
	pool := NewAgentPool()
	pool.Add(&AgentInfo{ID: "agent-1", Phase: "coding", Status: AgentStatusIdle})

	// Set heartbeat to expired time
	agent, _ := pool.Get("agent-1")
	agent.LastHeartbeat = time.Now().Add(-AgentHeartbeatTimeout - time.Second)

	available := pool.ListAvailable()
	if len(available) != 0 {
		t.Errorf("expected 0 available agents (timed out), got %d", len(available))
	}

	// Verify it was marked offline
	agent, _ = pool.Get("agent-1")
	if agent.Status != AgentStatusOffline {
		t.Errorf("expected status=offline, got %s", agent.Status)
	}
}

func TestAgentPool_Concurrency(t *testing.T) {
	pool := NewAgentPool()
	var wg sync.WaitGroup

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			pool.Add(&AgentInfo{ID: fmt.Sprintf("agent-%d", id), Phase: "coding"})
		}(i)
	}
	wg.Wait()

	if pool.Count() != 50 {
		t.Errorf("expected 50 agents, got %d", pool.Count())
	}
}

// ──────────────────────────────────────────────
// TaskQueue Tests
// ──────────────────────────────────────────────

func TestTaskQueue_EnqueueDequeue(t *testing.T) {
	ctx := context.Background()
	queue := NewTaskQueue()

	task := &QueuedTask{TaskID: "task-1", Priority: 5, Phase: "coding"}
	queue.Enqueue(task)

	if queue.Len() != 1 {
		t.Errorf("expected len=1, got %d", queue.Len())
	}

	dequeued, err := queue.Dequeue(ctx, 0)
	if err != nil {
		t.Fatalf("dequeue failed: %v", err)
	}
	if dequeued.TaskID != "task-1" {
		t.Errorf("expected task-1, got %s", dequeued.TaskID)
	}
	if queue.Len() != 0 {
		t.Errorf("expected len=0, got %d", queue.Len())
	}
}

func TestTaskQueue_PriorityOrder(t *testing.T) {
	ctx := context.Background()
	queue := NewTaskQueue()

	queue.Enqueue(&QueuedTask{TaskID: "low", Priority: 1, Phase: "coding"})
	queue.Enqueue(&QueuedTask{TaskID: "high", Priority: 10, Phase: "coding"})
	queue.Enqueue(&QueuedTask{TaskID: "mid", Priority: 5, Phase: "coding"})

	// Should dequeue in priority order: high, mid, low
	task1, _ := queue.Dequeue(ctx, 0)
	task2, _ := queue.Dequeue(ctx, 0)
	task3, _ := queue.Dequeue(ctx, 0)

	if task1.TaskID != "high" {
		t.Errorf("expected 'high' first, got '%s'", task1.TaskID)
	}
	if task2.TaskID != "mid" {
		t.Errorf("expected 'mid' second, got '%s'", task2.TaskID)
	}
	if task3.TaskID != "low" {
		t.Errorf("expected 'low' third, got '%s'", task3.TaskID)
	}
}

func TestTaskQueue_EqualPriority_FIFO(t *testing.T) {
	ctx := context.Background()
	queue := NewTaskQueue()

	queue.Enqueue(&QueuedTask{TaskID: "first", Priority: 5, Phase: "coding"})
	time.Sleep(10 * time.Millisecond)
	queue.Enqueue(&QueuedTask{TaskID: "second", Priority: 5, Phase: "coding"})

	task1, _ := queue.Dequeue(ctx, 0)
	task2, _ := queue.Dequeue(ctx, 0)

	if task1.TaskID != "first" {
		t.Errorf("expected 'first', got '%s'", task1.TaskID)
	}
	if task2.TaskID != "second" {
		t.Errorf("expected 'second', got '%s'", task2.TaskID)
	}
}

func TestTaskQueue_DequeueEmpty(t *testing.T) {
	ctx := context.Background()
	queue := NewTaskQueue()

	_, err := queue.Dequeue(ctx, 0)
	if err == nil {
		t.Fatal("expected error for empty queue")
	}
}

func TestTaskQueue_DequeueTimeout(t *testing.T) {
	ctx := context.Background()
	queue := NewTaskQueue()

	start := time.Now()
	_, err := queue.Dequeue(ctx, 50*time.Millisecond)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected timeout error")
	}
	if elapsed < 50*time.Millisecond {
		t.Errorf("expected at least 50ms wait, got %v", elapsed)
	}
}

func TestTaskQueue_Peek(t *testing.T) {
	queue := NewTaskQueue()
	queue.Enqueue(&QueuedTask{TaskID: "task-1", Priority: 5, Phase: "coding"})

	task, ok := queue.Peek()
	if !ok {
		t.Fatal("expected task from peek")
	}
	if task.TaskID != "task-1" {
		t.Errorf("expected task-1, got %s", task.TaskID)
	}
	if queue.Len() != 1 {
		t.Errorf("peek should not remove: expected len=1, got %d", queue.Len())
	}
}

func TestTaskQueue_PeekEmpty(t *testing.T) {
	queue := NewTaskQueue()
	_, ok := queue.Peek()
	if ok {
		t.Error("expected false from empty queue peek")
	}
}

func TestTaskQueue_Close(t *testing.T) {
	ctx := context.Background()
	queue := NewTaskQueue()

	queue.Close()

	err := queue.Enqueue(&QueuedTask{TaskID: "t", Priority: 1})
	if err == nil {
		t.Fatal("expected error when enqueuing to closed queue")
	}

	_, err = queue.Dequeue(ctx, 0)
	if err == nil {
		t.Fatal("expected error when dequeuing from closed queue")
	}
}

func TestTaskQueue_Concurrency(t *testing.T) {
	ctx := context.Background()
	queue := NewTaskQueue()
	var wg sync.WaitGroup

	// Enqueue concurrently
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			queue.Enqueue(&QueuedTask{TaskID: fmt.Sprintf("task-%d", id), Priority: id, Phase: "coding"})
		}(i)
	}
	wg.Wait()

	if queue.Len() != 100 {
		t.Errorf("expected 100 tasks, got %d", queue.Len())
	}

	// Dequeue all
	for i := 0; i < 100; i++ {
		_, err := queue.Dequeue(ctx, 0)
		if err != nil {
			t.Fatalf("dequeue %d failed: %v", i, err)
		}
	}
}

// ──────────────────────────────────────────────
// MatchEngine Tests
// ──────────────────────────────────────────────

func TestMatchEngine_ExactMatch(t *testing.T) {
	pool := NewAgentPool()
	pool.Add(&AgentInfo{ID: "agent-1", Phase: "coding", Capabilities: []string{"go", "grpc"}, Status: AgentStatusIdle, LastHeartbeat: time.Now()})

	task := &QueuedTask{TaskID: "task-1", Phase: "coding", RequiredCapabilities: []string{"go"}}
	input := MatchInput{Task: task, Pool: pool}

	output := MatchEngine(input)
	if output.Matched == nil {
		t.Fatal("expected match, got none")
	}
	if output.Matched.ID != "agent-1" {
		t.Errorf("expected agent-1, got %s", output.Matched.ID)
	}
}

func TestMatchEngine_NoPhaseMatch(t *testing.T) {
	pool := NewAgentPool()
	pool.Add(&AgentInfo{ID: "agent-1", Phase: "coding", Status: AgentStatusIdle})

	task := &QueuedTask{TaskID: "task-1", Phase: "testing"}
	input := MatchInput{Task: task, Pool: pool}

	output := MatchEngine(input)
	if output.Matched != nil {
		t.Errorf("expected no match, got %s", output.Matched.ID)
	}
}

func TestMatchEngine_NoCapabilityMatch(t *testing.T) {
	pool := NewAgentPool()
	pool.Add(&AgentInfo{ID: "agent-1", Phase: "coding", Capabilities: []string{"python"}, Status: AgentStatusIdle, LastHeartbeat: time.Now()})

	task := &QueuedTask{TaskID: "task-1", Phase: "coding", RequiredCapabilities: []string{"go"}}
	input := MatchInput{Task: task, Pool: pool}

	output := MatchEngine(input)
	if output.Matched != nil {
		t.Errorf("expected no match, got %s", output.Matched.ID)
	}
	if len(output.Candidates) != 1 {
		t.Errorf("expected 1 candidate (phase matched), got %d", len(output.Candidates))
	}
}

func TestMatchEngine_LongestIdle(t *testing.T) {
	pool := NewAgentPool()
	// Use distinct times well within the heartbeat timeout
	now := time.Now()
	pool.Add(&AgentInfo{ID: "agent-1", Phase: "coding", Capabilities: []string{"go"}, Status: AgentStatusIdle, LastHeartbeat: now.Add(-10 * time.Second)})
	pool.Add(&AgentInfo{ID: "agent-2", Phase: "coding", Capabilities: []string{"go"}, Status: AgentStatusIdle, LastHeartbeat: now.Add(-25 * time.Second)})
	pool.Add(&AgentInfo{ID: "agent-3", Phase: "coding", Capabilities: []string{"go"}, Status: AgentStatusIdle, LastHeartbeat: now.Add(-5 * time.Second)})

	task := &QueuedTask{TaskID: "task-1", Phase: "coding", RequiredCapabilities: []string{"go"}}
	input := MatchInput{Task: task, Pool: pool}

	output := MatchEngine(input)
	if output.Matched == nil {
		t.Fatal("expected match, got none")
	}
	// agent-2 has been idle longest (30s ago)
	if output.Matched.ID != "agent-2" {
		t.Errorf("expected agent-2 (longest idle), got %s", output.Matched.ID)
	}
}

func TestMatchEngine_NilInput(t *testing.T) {
	output := MatchEngine(MatchInput{})
	if output.Matched != nil {
		t.Error("expected nil match for nil input")
	}
}

// ──────────────────────────────────────────────
// SchedulerComposer Integration Tests
// ──────────────────────────────────────────────

func TestSchedulerComposer_ScheduleNext(t *testing.T) {
	ctx := context.Background()
	obs := &InMemoryObservationAdapter{}

	// Setup
	pool := NewAgentPool()
	pool.Add(&AgentInfo{ID: "agent-1", Phase: "coding", Capabilities: []string{"go"}, Status: AgentStatusIdle, LastHeartbeat: time.Now()})

	queue := NewTaskQueue()
	queue.Enqueue(&QueuedTask{TaskID: "task-sched", Priority: 5, Phase: "coding", RequiredCapabilities: []string{"go"}})

	persistence := NewInMemorySnapshotAdapter()
	transport := NewInProcHandoffTransport()
	handoff := NewHandoffComposer(obs, persistence, transport)

	scheduler := NewSchedulerComposer(pool, queue, handoff, obs)

	result, steps, err := scheduler.ScheduleNext(ctx, 0)
	if err != nil {
		t.Fatalf("schedule failed: %v", err)
	}
	if !result.Dispatched {
		t.Fatal("expected dispatched=true")
	}
	if result.MatchedAgent != "agent-1" {
		t.Errorf("expected agent-1, got %s", result.MatchedAgent)
	}
	if result.TaskID != "task-sched" {
		t.Errorf("expected task-sched, got %s", result.TaskID)
	}
	if len(steps) == 0 {
		t.Error("expected non-empty steps")
	}
}

func TestSchedulerComposer_NoMatch(t *testing.T) {
	ctx := context.Background()
	obs := &InMemoryObservationAdapter{}

	pool := NewAgentPool()
	// No agents with "testing" phase
	pool.Add(&AgentInfo{ID: "agent-1", Phase: "coding", Capabilities: []string{"go"}, Status: AgentStatusIdle, LastHeartbeat: time.Now()})

	queue := NewTaskQueue()
	queue.Enqueue(&QueuedTask{TaskID: "task-nomatch", Priority: 5, Phase: "testing", RequiredCapabilities: []string{"go"}})

	persistence := NewInMemorySnapshotAdapter()
	transport := NewInProcHandoffTransport()
	handoff := NewHandoffComposer(obs, persistence, transport)

	scheduler := NewSchedulerComposer(pool, queue, handoff, obs)

	result, _, err := scheduler.ScheduleNext(ctx, 0)
	if err != nil {
		t.Fatalf("schedule failed: %v", err)
	}
	if result.Dispatched {
		t.Error("expected dispatched=false (no match)")
	}
	if !result.Requeued {
		t.Error("expected requeued=true (no match)")
	}
}

func TestSchedulerComposer_EmptyQueue(t *testing.T) {
	ctx := context.Background()
	obs := &InMemoryObservationAdapter{}

	pool := NewAgentPool()
	queue := NewTaskQueue()
	persistence := NewInMemorySnapshotAdapter()
	transport := NewInProcHandoffTransport()
	handoff := NewHandoffComposer(obs, persistence, transport)

	scheduler := NewSchedulerComposer(pool, queue, handoff, obs)

	_, _, err := scheduler.ScheduleNext(ctx, 0)
	if err == nil {
		t.Fatal("expected error for empty queue")
	}
}

func TestMatchEngineAsStep(t *testing.T) {
	ctx := context.Background()
	pool := NewAgentPool()
	pool.Add(&AgentInfo{ID: "agent-1", Phase: "coding", Capabilities: []string{"go"}, Status: AgentStatusIdle, LastHeartbeat: time.Now()})

	task := &QueuedTask{TaskID: "task-1", Phase: "coding", RequiredCapabilities: []string{"go"}}
	step := MatchEngineAsStep()

	output, err := step.Execute(ctx, MatchInput{Task: task, Pool: pool})
	if err != nil {
		t.Fatalf("step execution failed: %v", err)
	}
	if output.Matched == nil {
		t.Fatal("expected match")
	}
	if step.UnitType() != "Atom" {
		t.Errorf("expected UnitType='Atom', got '%s'", step.UnitType())
	}
}

// ──────────────────────────────────────────────
// Benchmark Tests
// ──────────────────────────────────────────────

func BenchmarkTaskQueue_EnqueueDequeue(b *testing.B) {
	ctx := context.Background()
	queue := NewTaskQueue()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		queue.Enqueue(&QueuedTask{TaskID: fmt.Sprintf("t-%d", i), Priority: i % 10, Phase: "coding"})
		queue.Dequeue(ctx, 0)
	}
}

func BenchmarkMatchEngine(b *testing.B) {
	pool := NewAgentPool()
	for i := 0; i < 100; i++ {
		pool.Add(&AgentInfo{
			ID:           fmt.Sprintf("agent-%d", i),
			Phase:        "coding",
			Capabilities: []string{"go", "grpc", "test"},
			Status:       AgentStatusIdle,
			LastHeartbeat: time.Now().Add(-time.Duration(i) * time.Second),
		})
	}

	task := &QueuedTask{TaskID: "task", Phase: "coding", RequiredCapabilities: []string{"go"}}
	input := MatchInput{Task: task, Pool: pool}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		MatchEngine(input)
	}
}

func BenchmarkDevSnapshot_Checksum(b *testing.B) {
	snap := NewDevSnapshot("task-1", "agent-a", "coding", "benchmark")
	snap.Artifacts = []Artifact{
		{Path: "main.go", Type: "file", Description: "main entry"},
		{Path: "handler.go", Type: "file", Description: "handler"},
	}
	snap.Pending = []WorkItem{
		{ID: "w-1", Title: "write tests", Priority: "high"},
		{ID: "w-2", Title: "add docs", Priority: "medium"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		snap.ComputeChecksum()
	}
}