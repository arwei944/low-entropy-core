//go:build lecore_tier3 || lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

package core

import (
	"context"
	"fmt"
	"testing"
	"time"
)

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

func TestSchedulerComposer_ScheduleNext(t *testing.T) {
	ctx := context.Background()
	obs := &InMemoryObservationAdapter{}

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
			ID:            fmt.Sprintf("agent-%d", i),
			Phase:         "coding",
			Capabilities:  []string{"go", "grpc", "test"},
			Status:        AgentStatusIdle,
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
