//go:build lecore_tier7

package core

import (
	"context"
	"fmt"
	"testing"
	"time"
)

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

	snapshot := NewDevSnapshot("task-1", "agent-1", "design", "initial")
	snapshot.Artifacts = []Artifact{
		{Path: "main.go", Type: "code", Description: "main entry", Hash: "abc123"},
	}
	snapshot.Decisions = []Decision{
		{ID: "d1", Title: "use goroutines", Rationale: "performance"},
	}

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

	posResult, _, err := branch.Run(ctx, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if posResult != 50 {
		t.Errorf("expected 50, got %d", posResult)
	}

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

	errorCount := len(results.Errors)
	successCount := len(results.Results) - errorCount

	t.Logf("RunParallel: %d composers, results=%d, successes=%d, errors=%d",
		len(composers), len(results.Results), successCount, errorCount)
}

// TestStress_FourPrimitives_MixedPipeline: 四原语混合Pipeline
func TestStress_FourPrimitives_MixedPipeline(t *testing.T) {
	obs := &InMemoryObservationAdapter{}

	atom := func(ctx context.Context, in int) (int, error) {
		return in * 2, nil
	}

	port := NewPort[int, int](func(ctx context.Context, in int) (int, error) {
		if in < 0 {
			return 0, fmt.Errorf("negative value rejected")
		}
		return in, nil
	})

	adapter := NewAdapter[int, int](func(ctx context.Context, in int) (int, error) {
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
	if result != 20 {
		t.Errorf("expected 20, got %d", result)
	}
	if len(steps) != 3 {
		t.Errorf("expected 3 steps, got %d", len(steps))
	}

	_, _, err = pipeline.Run(ctx, -5)
	if err == nil {
		t.Error("expected error for negative input")
	}

	t.Logf("Four Primitives: result=%d, steps=%d", result, len(steps))
}
