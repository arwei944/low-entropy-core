//go:build lecore_tier3 || lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

package core

import (
	"context"
	"testing"
	"time"
)

func TestNewDevSnapshot_Basic(t *testing.T) {
	snap := NewDevSnapshot("task-1", "agent-a", "coding", "completed module X")
	if snap.TaskID != "task-1" {
		t.Errorf("expected TaskID='task-1', got '%s'", snap.TaskID)
	}
	if snap.AgentID != "agent-a" {
		t.Errorf("expected AgentID='agent-a', got '%s'", snap.AgentID)
	}
	if snap.Phase != "coding" {
		t.Errorf("expected Phase='coding', got '%s'", snap.Phase)
	}
	if snap.SchemaVersion != "1.0" {
		t.Errorf("expected SchemaVersion='1.0', got '%s'", snap.SchemaVersion)
	}
	if snap.CreatedAt.IsZero() {
		t.Error("expected non-zero CreatedAt")
	}
}

func TestDevSnapshot_ChecksumRoundtrip(t *testing.T) {
	snap := NewDevSnapshot("task-1", "agent-a", "coding", "testing")
	snap.Artifacts = []Artifact{
		{Path: "src/main.go", Type: "file", Description: "main entry"},
	}
	snap.Pending = []WorkItem{
		{ID: "w-1", Title: "write tests", Priority: "high", EstimatedHours: 2},
	}

	checksum, err := snap.ComputeChecksum()
	if err != nil {
		t.Fatalf("failed to compute checksum: %v", err)
	}
	if checksum == "" {
		t.Error("expected non-empty checksum")
	}
	if len(checksum) != 64 {
		t.Errorf("expected 64-char hex checksum, got %d chars", len(checksum))
	}

	if !snap.VerifyChecksum() {
		t.Error("checksum verification failed")
	}

	snap.Checkpoint = "tampered"
	if snap.VerifyChecksum() {
		t.Error("checksum verification should fail after tampering")
	}
}

func TestDevSnapshot_ChecksumDeterministic(t *testing.T) {
	snap := NewDevSnapshot("task-1", "agent-a", "coding", "same")
	snap.Artifacts = []Artifact{
		{Path: "a.go", Type: "file", Description: "file A"},
	}

	checksum1, _ := snap.ComputeChecksum()
	checksum2, _ := snap.ComputeChecksum()

	if checksum1 != checksum2 {
		t.Errorf("checksums should be deterministic: %s != %s", checksum1, checksum2)
	}
}

func TestDevSnapshot_JSONRoundtrip(t *testing.T) {
	snap := NewDevSnapshot("task-1", "agent-a", "coding", "complete")
	snap.Artifacts = []Artifact{
		{Path: "a.go", Type: "file", Description: "main file", Hash: "abc123"},
	}
	snap.Decisions = []Decision{
		{ID: "d-1", Title: "use generics", Rationale: "type safety", Alternatives: []string{"interface{}", "any"}},
	}

	data, err := snap.ToJSON()
	if err != nil {
		t.Fatalf("failed to serialize: %v", err)
	}

	restored, err := DevSnapshotFromJSON(data)
	if err != nil {
		t.Fatalf("failed to deserialize: %v", err)
	}

	if restored.TaskID != snap.TaskID {
		t.Errorf("TaskID mismatch: %s != %s", restored.TaskID, snap.TaskID)
	}
	if len(restored.Artifacts) != 1 {
		t.Errorf("expected 1 artifact, got %d", len(restored.Artifacts))
	}
	if restored.Artifacts[0].Hash != "abc123" {
		t.Errorf("artifact hash mismatch: %s", restored.Artifacts[0].Hash)
	}
}

func TestDevSnapshot_FromJSON_Invalid(t *testing.T) {
	_, err := DevSnapshotFromJSON([]byte("not valid json"))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestHandoffContract_NotExpired(t *testing.T) {
	contract := NewHandoffContract("src", "tgt", "task-1", "coding", "abc123")
	if contract.IsExpired() {
		t.Error("new contract should not be expired")
	}
}

func TestHandoffContract_Expired(t *testing.T) {
	contract := NewHandoffContract("src", "tgt", "task-1", "coding", "abc123")
	contract.Timestamp = time.Now().Add(-6 * time.Minute)
	if !contract.IsExpired() {
		t.Error("6-minute-old contract should be expired")
	}
}

func TestContractValidator_Valid(t *testing.T) {
	ctx := context.Background()
	validator := NewContractValidator("abc123")
	contract := NewHandoffContract("src", "tgt", "task-1", "coding", "abc123")

	result, err := validator.Validate(ctx, *contract)
	if err != nil {
		t.Fatalf("expected valid contract, got error: %v", err)
	}
	if result.SourceID != "src" {
		t.Errorf("expected SourceID='src', got '%s'", result.SourceID)
	}
}

func TestContractValidator_EmptySource(t *testing.T) {
	ctx := context.Background()
	validator := NewContractValidator("abc123")
	contract := NewHandoffContract("", "tgt", "task-1", "coding", "abc123")

	_, err := validator.Validate(ctx, *contract)
	if err == nil {
		t.Fatal("expected error for empty source ID")
	}
}

func TestContractValidator_EmptyTarget(t *testing.T) {
	ctx := context.Background()
	validator := NewContractValidator("abc123")
	contract := NewHandoffContract("src", "", "task-1", "coding", "abc123")

	_, err := validator.Validate(ctx, *contract)
	if err == nil {
		t.Fatal("expected error for empty target ID")
	}
}

func TestContractValidator_Expired(t *testing.T) {
	ctx := context.Background()
	validator := NewContractValidator("abc123")
	contract := NewHandoffContract("src", "tgt", "task-1", "coding", "abc123")
	contract.Timestamp = time.Now().Add(-6 * time.Minute)

	_, err := validator.Validate(ctx, *contract)
	if err == nil {
		t.Fatal("expected error for expired contract")
	}
}

func TestContractValidator_ChecksumMismatch(t *testing.T) {
	ctx := context.Background()
	validator := NewContractValidator("expected")
	contract := NewHandoffContract("src", "tgt", "task-1", "coding", "wrong")

	_, err := validator.Validate(ctx, *contract)
	if err == nil {
		t.Fatal("expected error for checksum mismatch")
	}
}

func TestContractValidator_EmptyChecksum(t *testing.T) {
	ctx := context.Background()
	validator := NewContractValidator("")
	contract := NewHandoffContract("src", "tgt", "task-1", "coding", "")

	_, err := validator.Validate(ctx, *contract)
	if err == nil {
		t.Fatal("expected error for empty checksum")
	}
}
