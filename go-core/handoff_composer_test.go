//go:build lecore_tier3 || lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

package core

import (
	"context"
	"testing"
)

func TestHandoffComposer_Execute_Success(t *testing.T) {
	ctx := context.Background()
	obs := &InMemoryObservationAdapter{}
	persistence := NewInMemorySnapshotAdapter()
	transport := NewInProcHandoffTransport()

	composer := NewHandoffComposer(obs, persistence, transport)

	snap := NewDevSnapshot("task-hc", "agent-src", "coding", "handoff test")
	snap.Artifacts = []Artifact{
		{Path: "module.go", Type: "file", Description: "module"},
	}

	input := HandoffInput{
		SourceAgent:   snap,
		TargetAgentID: "agent-tgt",
		TaskID:        "task-hc",
		Phase:         "coding",
	}

	output, steps, err := composer.Execute(ctx, input)
	if err != nil {
		t.Fatalf("handoff failed: %v", err)
	}
	if !output.Success {
		t.Fatal("expected successful handoff, got failure")
	}
	if !output.TargetConfirmed {
		t.Error("expected target confirmed")
	}
	if output.SnapshotChecksum == "" {
		t.Error("expected non-empty checksum")
	}
	if output.Contract == nil {
		t.Error("expected non-nil contract")
	}
	if len(steps) != 4 {
		t.Errorf("expected 4 steps, got %d", len(steps))
	}
}

func TestHandoffComposer_ReceiveSnapshot(t *testing.T) {
	ctx := context.Background()
	obs := &InMemoryObservationAdapter{}
	persistence := NewInMemorySnapshotAdapter()
	transport := NewInProcHandoffTransport()

	composer := NewHandoffComposer(obs, persistence, transport)

	snap := NewDevSnapshot("task-recv", "agent-src", "coding", "receive test")
	checksum, _ := snap.ComputeChecksum()
	data, _ := snap.ToJSON()
	transport.Transfer(ctx, checksum, data)

	received, steps, err := composer.ReceiveSnapshot(ctx, checksum)
	if err != nil {
		t.Fatalf("receive failed: %v", err)
	}
	if received.TaskID != "task-recv" {
		t.Errorf("expected TaskID='task-recv', got '%s'", received.TaskID)
	}
	if len(steps) != 2 {
		t.Errorf("expected 2 steps, got %d", len(steps))
	}
}

func TestHandoffComposer_ReceiveSnapshot_Tampered(t *testing.T) {
	ctx := context.Background()
	obs := &InMemoryObservationAdapter{}
	persistence := NewInMemorySnapshotAdapter()
	transport := NewInProcHandoffTransport()

	composer := NewHandoffComposer(obs, persistence, transport)

	snap := NewDevSnapshot("task-tamper", "agent-src", "coding", "tamper test")
	checksum, _ := snap.ComputeChecksum()

	snap.Checkpoint = "tampered!"
	data, _ := snap.ToJSON()
	transport.Transfer(ctx, checksum, data)

	_, _, err := composer.ReceiveSnapshot(ctx, checksum)
	if err == nil {
		t.Fatal("expected error for tampered snapshot")
	}
}
