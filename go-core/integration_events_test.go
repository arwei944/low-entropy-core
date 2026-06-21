//go:build lecore_tier5 || lecore_tier6 || lecore_tier7

package core_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	. "low-entropy-core/go-core"
)

func TestIntegration_EventSourcing(t *testing.T) {
	ctx := context.Background()
	store := NewEventStore()

	aggregateID := "agg-001"

	events := []struct {
		eventType string
		data      string
	}{
		{"created", "{}"},
		{"updated", `{"name":"test"}`},
		{"deleted", `{"reason":"done"}`},
	}

	for _, e := range events {
		result, err := store.Execute(ctx, EventEnvelope{
			AggregateID:   aggregateID,
			AggregateType: "TestAggregate",
			EventType:     e.eventType,
			EventData:     []byte(e.data),
		})
		if err != nil {
			t.Fatalf("append event %q failed: %v", e.eventType, err)
		}
		if !result.Success {
			t.Errorf("expected success for event %q", e.eventType)
		}
	}

	allEvents := store.StreamAll(aggregateID)
	if len(allEvents) != 3 {
		t.Fatalf("expected 3 events, got %d", len(allEvents))
	}
	for i, expectedVersion := range []int64{1, 2, 3} {
		if allEvents[i].Version != expectedVersion {
			t.Errorf("event %d: expected version %d, got %d", i, expectedVersion, allEvents[i].Version)
		}
	}

	streamed := store.Stream(aggregateID, 1)
	if len(streamed) != 3 {
		t.Errorf("expected 3 streamed events from version 1, got %d", len(streamed))
	}
	for i, e := range streamed {
		expectedVersion := int64(i + 1)
		if e.Version != expectedVersion {
			t.Errorf("streamed event %d: expected version %d, got %d", i, expectedVersion, e.Version)
		}
	}

	projection := NewProjection(func(state []byte, event EventEnvelope) ([]byte, error) {
		current := 0
		if len(state) > 0 {
			fmt.Sscanf(string(state), "%d", &current)
		}
		newVal := current + int(event.Version)
		return []byte(fmt.Sprintf("%d", newVal)), nil
	})

	output, err := projection.Execute(ProjectionInput{
		AggregateID:  aggregateID,
		Events:       allEvents,
		FromVersion:  0,
		CurrentState: []byte("0"),
	})
	if err != nil {
		t.Fatalf("projection failed: %v", err)
	}

	expectedSum := "6"
	if string(output.State) != expectedSum {
		t.Errorf("expected projection state %q, got %q", expectedSum, string(output.State))
	}
	if output.EventsProcessed != 3 {
		t.Errorf("expected 3 events processed, got %d", output.EventsProcessed)
	}
	if output.Version != 3 {
		t.Errorf("expected version 3, got %d", output.Version)
	}
}

func TestIntegration_MerkleChain(t *testing.T) {
	chain := NewMerkleAuditChain()

	now := time.Now()
	for i := 0; i < 10; i++ {
		entry := AuditEntry{
			ID:        fmt.Sprintf("entry-%d", i),
			AgentID:   "agent-1",
			Action:    fmt.Sprintf("action-%d", i),
			Resource:  "resource-1",
			Result:    "success",
			Details:   fmt.Sprintf("details-%d", i),
			Timestamp: now.Add(time.Duration(i) * time.Second),
		}
		err := chain.Append(entry)
		if err != nil {
			t.Fatalf("append entry %d failed: %v", i, err)
		}
	}

	rootHash := chain.RootHash()
	if rootHash == "" {
		t.Error("root hash should not be empty")
	}

	if chain.Count() != 10 {
		t.Errorf("expected 10 entries, got %d", chain.Count())
	}

	proof, err := chain.GenerateProof(5)
	if err != nil {
		t.Fatalf("generate proof for entry 5 failed: %v", err)
	}
	if proof == nil {
		t.Fatal("expected non-nil proof")
	}
	if proof.EntryIndex != 5 {
		t.Errorf("expected entry index 5, got %d", proof.EntryIndex)
	}

	if !VerifyProof(proof, rootHash) {
		t.Error("merkle proof verification failed for valid entry")
	}

	entries := chain.GetEntries()
	originalRootHash := chain.RootHash()

	entries[5].Details = "TAMPERED-DATA"
	tamperedChain := NewMerkleAuditChain()
	for _, e := range entries {
		_ = tamperedChain.Append(e)
	}
	tamperedRootHash := tamperedChain.RootHash()

	if tamperedRootHash == originalRootHash {
		t.Error("root hash should change when data is tampered")
	}

	if VerifyProof(proof, tamperedRootHash) {
		t.Error("merkle proof verification should fail for tampered entry")
	}

	if !VerifyProof(proof, originalRootHash) {
		t.Error("merkle proof verification failed for valid entry against original root hash")
	}
}
