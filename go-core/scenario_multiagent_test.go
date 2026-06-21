//go:build lecore_tier7

package core

import (
	"context"
	"fmt"
	"testing"
)

func TestScenario_MultiAgentPipeline(t *testing.T) {
	obs := &InMemoryObservationAdapter{}
	persistence := NewInMemorySnapshotAdapter()
	transport := NewInProcHandoffTransport()
	handoffComposer := NewHandoffComposer(obs, persistence, transport)
	ctx := context.Background()

	phases := []string{"analysis", "design", "coding", "review", "testing", "deployment"}
	agents := []string{"analyst-1", "architect-1", "dev-1", "reviewer-1", "tester-1", "devops-1"}

	snapshot := NewDevSnapshot("task-dev-001", agents[0], phases[0], "start")
	snapshot.Artifacts = []Artifact{
		{Path: "requirements.md", Type: "doc", Description: "User requirements for feature X", Hash: "req-hash-001"},
	}

	completedPhases := 0
	currentSnapshot := snapshot

	for i := 0; i < len(phases)-1; i++ {
		currentPhase := phases[i]
		nextPhase := phases[i+1]
		sourceAgent := agents[i]
		targetAgent := agents[i+1]

		currentSnapshot.Phase = currentPhase
		currentSnapshot.AgentID = sourceAgent
		currentSnapshot.Checkpoint = fmt.Sprintf("%s-complete", currentPhase)
		currentSnapshot.Artifacts = append(currentSnapshot.Artifacts, Artifact{
			Path:        fmt.Sprintf("%s-output.md", currentPhase),
			Type:        "doc",
			Description: fmt.Sprintf("Output of %s phase", currentPhase),
			Hash:        fmt.Sprintf("%s-hash", currentPhase),
		})

		input := HandoffInput{
			SourceAgent:   currentSnapshot,
			TargetAgentID: targetAgent,
			TaskID:        "task-dev-001",
			Phase:         nextPhase,
		}

		output, _, err := handoffComposer.Execute(ctx, input)
		if err != nil {
			t.Fatalf("handoff from %s to %s failed: %v", currentPhase, nextPhase, err)
		}
		if !output.Success {
			t.Fatalf("handoff from %s to %s not successful: %s", currentPhase, nextPhase, output.Error)
		}

		received, _, err := handoffComposer.ReceiveSnapshot(ctx, output.SnapshotChecksum)
		if err != nil {
			t.Fatalf("receive failed for %s: %v", nextPhase, err)
		}
		if !received.VerifyChecksum() {
			t.Fatalf("checksum verification failed for %s", nextPhase)
		}

		currentSnapshot = received
		completedPhases++
	}

	if completedPhases != 5 {
		t.Errorf("expected 5 handoffs, got %d", completedPhases)
	}

	t.Logf("MultiAgent: %d phases, %d handoffs, all checksums verified", len(phases), completedPhases)
}
