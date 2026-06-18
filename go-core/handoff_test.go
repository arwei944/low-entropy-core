package core

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// ──────────────────────────────────────────────
// DevSnapshot Tests
// ──────────────────────────────────────────────

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

	// Verify checksum
	if !snap.VerifyChecksum() {
		t.Error("checksum verification failed")
	}

	// Tamper with data and verify
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

// ──────────────────────────────────────────────
// HandoffContract Tests
// ──────────────────────────────────────────────

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

// ──────────────────────────────────────────────
// HandoffTransport Tests
// ──────────────────────────────────────────────

func TestInProcHandoffTransport_TransferReceive(t *testing.T) {
	ctx := context.Background()
	transport := NewInProcHandoffTransport()

	data := []byte("test snapshot data")
	checksum := "abc123"

	err := transport.Transfer(ctx, checksum, data)
	if err != nil {
		t.Fatalf("transfer failed: %v", err)
	}

	received, err := transport.Receive(ctx, checksum)
	if err != nil {
		t.Fatalf("receive failed: %v", err)
	}
	if string(received) != string(data) {
		t.Errorf("data mismatch: '%s' != '%s'", string(received), string(data))
	}
}

func TestInProcHandoffTransport_ReceiveNotFound(t *testing.T) {
	ctx := context.Background()
	transport := NewInProcHandoffTransport()

	_, err := transport.Receive(ctx, "nonexistent")
	if err == nil {
		t.Fatal("expected error for missing snapshot")
	}
}

func TestInProcHandoffTransport_Delete(t *testing.T) {
	ctx := context.Background()
	transport := NewInProcHandoffTransport()

	transport.Transfer(ctx, "abc", []byte("data"))
	transport.Delete("abc")

	_, err := transport.Receive(ctx, "abc")
	if err == nil {
		t.Fatal("expected error after delete")
	}
}

func TestInProcHandoffTransport_Count(t *testing.T) {
	ctx := context.Background()
	transport := NewInProcHandoffTransport()

	transport.Transfer(ctx, "a", []byte("1"))
	transport.Transfer(ctx, "b", []byte("2"))

	if transport.Count() != 2 {
		t.Errorf("expected count=2, got %d", transport.Count())
	}
}

func TestInProcHandoffTransport_Concurrency(t *testing.T) {
	ctx := context.Background()
	transport := NewInProcHandoffTransport()
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			key := fmt.Sprintf("key-%d", id)
			transport.Transfer(ctx, key, []byte("data"))
			transport.Receive(ctx, key)
		}(i)
	}
	wg.Wait()

	if transport.Count() != 100 {
		t.Errorf("expected 100 entries, got %d", transport.Count())
	}
}

// ──────────────────────────────────────────────
// Snapshot Persistence Tests
// ──────────────────────────────────────────────

func TestInMemorySnapshotAdapter_CreateRestore(t *testing.T) {
	snap := NewDevSnapshot("task-1", "agent-a", "coding", "done")
	snap.Artifacts = []Artifact{
		{Path: "a.go", Type: "file", Description: "main"},
	}
	snap.Pending = []WorkItem{
		{ID: "w-1", Title: "test", Priority: "high"},
	}

	adapter := NewInMemorySnapshotAdapter()

	handoff, err := adapter.CreateSnapshot(snap)
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}
	if handoff.Token == "" {
		t.Error("expected non-empty token")
	}

	restored, err := adapter.RestoreSnapshot(handoff)
	if err != nil {
		t.Fatalf("restore failed: %v", err)
	}
	if restored.TaskID != "task-1" {
		t.Errorf("TaskID mismatch: %s", restored.TaskID)
	}
	if len(restored.Artifacts) != 1 {
		t.Errorf("expected 1 artifact, got %d", len(restored.Artifacts))
	}
	if len(restored.Pending) != 1 {
		t.Errorf("expected 1 work item, got %d", len(restored.Pending))
	}
}

func TestInMemorySnapshotAdapter_RestoreNotFound(t *testing.T) {
	adapter := NewInMemorySnapshotAdapter()
	_, err := adapter.RestoreSnapshot(HandoffSnapshot{Token: "nonexistent"})
	if err == nil {
		t.Fatal("expected error for missing snapshot")
	}
}

func TestInMemorySnapshotAdapter_Delete(t *testing.T) {
	snap := NewDevSnapshot("task-1", "agent-a", "coding", "done")
	adapter := NewInMemorySnapshotAdapter()

	handoff, _ := adapter.CreateSnapshot(snap)
	adapter.DeleteSnapshot(handoff.Token)

	if adapter.Count() != 0 {
		t.Errorf("expected count=0, got %d", adapter.Count())
	}
}

func TestJSONSnapshotAdapter_CreateRestore(t *testing.T) {
	// Use a temp directory
	tmpDir := filepath.Join(os.TempDir(), "low-entropy-test-snapshots")
	defer os.RemoveAll(tmpDir)

	snap := NewDevSnapshot("task-json", "agent-b", "testing", "persisted")
	snap.Artifacts = []Artifact{
		{Path: "test.go", Type: "file", Description: "test file"},
	}

	adapter := NewJSONSnapshotAdapter(tmpDir)

	handoff, err := adapter.CreateSnapshot(snap)
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	restored, err := adapter.RestoreSnapshot(handoff)
	if err != nil {
		t.Fatalf("restore failed: %v", err)
	}
	if restored.TaskID != "task-json" {
		t.Errorf("expected TaskID='task-json', got '%s'", restored.TaskID)
	}
	if !restored.VerifyChecksum() {
		t.Error("restored snapshot checksum verification failed")
	}
}

func TestJSONSnapshotAdapter_Delete(t *testing.T) {
	tmpDir := filepath.Join(os.TempDir(), "low-entropy-test-snapshots-delete")
	defer os.RemoveAll(tmpDir)

	snap := NewDevSnapshot("task-del", "agent-c", "review", "delete test")
	adapter := NewJSONSnapshotAdapter(tmpDir)

	handoff, _ := adapter.CreateSnapshot(snap)
	err := adapter.DeleteSnapshot(handoff.Token)
	if err != nil {
		t.Fatalf("delete failed: %v", err)
	}

	_, err = adapter.RestoreSnapshot(handoff)
	if err == nil {
		t.Fatal("expected error after delete")
	}
}

// ──────────────────────────────────────────────
// HandoffComposer Tests
// ──────────────────────────────────────────────

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

	// Tamper with the snapshot
	snap.Checkpoint = "tampered!"
	data, _ := snap.ToJSON()
	transport.Transfer(ctx, checksum, data)

	_, _, err := composer.ReceiveSnapshot(ctx, checksum)
	if err == nil {
		t.Fatal("expected error for tampered snapshot")
	}
}

// ──────────────────────────────────────────────
// Rollback Tests
// ──────────────────────────────────────────────

func TestRollbackHandoff_Success(t *testing.T) {
	ctx := context.Background()
	obs := &InMemoryObservationAdapter{}
	persistence := NewInMemorySnapshotAdapter()
	transport := NewInProcHandoffTransport()

	snap := NewDevSnapshot("task-rb", "agent-src", "coding", "rollback test")
	checksum, _ := snap.ComputeChecksum()

	// Persist the snapshot
	persistence.CreateSnapshot(snap)

	// Transfer the snapshot
	data, _ := snap.ToJSON()
	transport.Transfer(ctx, checksum, data)

	result, steps, err := RollbackHandoff(ctx, persistence, transport, checksum, obs)
	if err != nil {
		t.Fatalf("rollback failed: %v", err)
	}
	if !result.Success {
		t.Fatal("expected successful rollback")
	}
	if !result.ChecksumMatch {
		t.Error("expected checksum match")
	}
	if len(steps) != 3 {
		t.Errorf("expected 3 steps, got %d", len(steps))
	}

	// Transport should be cleaned up
	_, transportErr := transport.Receive(ctx, checksum)
	if transportErr == nil {
		t.Error("expected transport to be cleaned up after rollback")
	}
}

func TestRollbackHandoff_SnapshotNotFound(t *testing.T) {
	ctx := context.Background()
	persistence := NewInMemorySnapshotAdapter()
	transport := NewInProcHandoffTransport()

	_, _, err := RollbackHandoff(ctx, persistence, transport, "nonexistent", nil)
	if err == nil {
		t.Fatal("expected error for missing snapshot")
	}
}

func TestHandoffWithRollback_Normal(t *testing.T) {
	ctx := context.Background()
	obs := &InMemoryObservationAdapter{}
	persistence := NewInMemorySnapshotAdapter()
	transport := NewInProcHandoffTransport()

	composer := NewHandoffComposer(obs, persistence, transport)

	snap := NewDevSnapshot("task-hwr", "agent-src", "coding", "with rollback")
	input := HandoffInput{
		SourceAgent:   snap,
		TargetAgentID: "agent-tgt",
		TaskID:        "task-hwr",
		Phase:         "coding",
	}

	output, _, rollback, err := HandoffWithRollback(ctx, composer, input)
	if err != nil {
		t.Fatalf("handoff with rollback failed: %v", err)
	}
	if !output.Success {
		t.Fatal("expected successful handoff")
	}
	if rollback != nil {
		t.Error("expected nil rollback for successful handoff")
	}
}

// ──────────────────────────────────────────────
// Security Tests
// ──────────────────────────────────────────────

func TestCapabilityToken_SignVerify(t *testing.T) {
	secret := []byte("my-secret-key")
	token := NewCapabilityToken("agent-a", []string{"read", "write", "deploy"})
	token.Sign(secret)

	if !token.Verify(secret) {
		t.Error("signature verification failed")
	}

	// Wrong secret should fail
	if token.Verify([]byte("wrong-secret")) {
		t.Error("verification should fail with wrong secret")
	}
}

func TestCapabilityToken_Expired(t *testing.T) {
	token := NewCapabilityToken("agent-a", []string{"read"})
	if token.IsExpired() {
		t.Error("new token should not be expired")
	}

	token.ExpiresAt = time.Now().Add(-1 * time.Minute)
	if !token.IsExpired() {
		t.Error("token should be expired")
	}
}

func TestCapabilityToken_HasCapability(t *testing.T) {
	token := NewCapabilityToken("agent-a", []string{"read", "write"})

	if !token.HasCapability("read") {
		t.Error("expected to have 'read' capability")
	}
	if token.HasCapability("admin") {
		t.Error("should not have 'admin' capability")
	}
}

func TestCapabilityPort_ValidToken(t *testing.T) {
	ctx := context.Background()
	secret := []byte("test-secret")

	token := NewCapabilityToken("agent-a", []string{"read", "write"})
	token.Sign(secret)

	port := NewCapabilityPort(secret, "read")
	result, err := port.Validate(ctx, *token)
	if err != nil {
		t.Fatalf("expected valid token, got error: %v", err)
	}
	if result.AgentID != "agent-a" {
		t.Errorf("expected AgentID='agent-a', got '%s'", result.AgentID)
	}
}

func TestCapabilityPort_MissingCapability(t *testing.T) {
	ctx := context.Background()
	secret := []byte("test-secret")

	token := NewCapabilityToken("agent-a", []string{"read"})
	token.Sign(secret)

	port := NewCapabilityPort(secret, "admin")
	_, err := port.Validate(ctx, *token)
	if err == nil {
		t.Fatal("expected error for missing capability")
	}
}

func TestCapabilityPort_InvalidSignature(t *testing.T) {
	ctx := context.Background()
	secret := []byte("test-secret")

	token := NewCapabilityToken("agent-a", []string{"read"})
	token.Sign([]byte("different-secret"))

	port := NewCapabilityPort(secret, "read")
	_, err := port.Validate(ctx, *token)
	if err == nil {
		t.Fatal("expected error for invalid signature")
	}
}

func TestCapabilityPort_ExpiredToken(t *testing.T) {
	ctx := context.Background()
	secret := []byte("test-secret")

	token := NewCapabilityToken("agent-a", []string{"read"})
	token.Sign(secret)
	token.ExpiresAt = time.Now().Add(-1 * time.Minute)

	port := NewCapabilityPort(secret, "read")
	_, err := port.Validate(ctx, *token)
	if err == nil {
		t.Fatal("expected error for expired token")
	}
}

func TestCapabilityToken_PayloadUniqueness(t *testing.T) {
	secret := []byte("secret")
	token1 := NewCapabilityToken("agent-a", []string{"read"})
	token1.Sign(secret)

	token2 := NewCapabilityToken("agent-b", []string{"read"})
	token2.Sign(secret)

	if token1.Signature == token2.Signature {
		t.Error("different tokens should have different signatures")
	}

	// Verify token1's signature doesn't work for token2's payload
	token2Copy := NewCapabilityToken("agent-b", []string{"read"})
	token2Copy.Signature = token1.Signature
	if token2Copy.Verify(secret) {
		t.Error("cross-token signature should not verify")
	}
}

// ──────────────────────────────────────────────
// NewHandoff Protocol Tests
// ──────────────────────────────────────────────

func TestNewHandoff_RecordsExecutionSteps(t *testing.T) {
	obs := &InMemoryObservationAdapter{}

	source := NewPipeline[any](obs,
		AtomAsStep(Atom[any, any](func(x any) any { return x })),
	)
	target := NewPipeline[any](obs,
		AtomAsStep(Atom[any, any](func(x any) any { return x })),
	)

	snapshot := &DefaultSnapshotAdapter{}
	transport := InProcTransport

	handoff := NewHandoff(source, target, snapshot, transport, obs)

	req := HandoffRequest{
		SourceID: "agent-a",
		TargetID: "agent-b",
		TaskType: "test",
		Payload:  "hello",
		Token:    "tok-001",
	}

	result, steps, err := handoff.Run(context.Background(), req)
	if err != nil {
		t.Fatalf("handoff failed: %v", err)
	}

	hr, ok := result.(HandoffResult)
	if !ok {
		t.Fatalf("expected HandoffResult, got %T", result)
	}
	if !hr.Success {
		t.Errorf("expected successful handoff, got error: %s", hr.Error)
	}

	// 验证产生了 ExecutionStep
	if len(steps) == 0 {
		t.Error("expected execution steps to be recorded")
	}

	// 验证 Pattern 为 "Handoff"
	hasHandoffPattern := false
	for _, s := range steps {
		if s.Pattern == "Handoff" {
			hasHandoffPattern = true
			break
		}
	}
	if !hasHandoffPattern {
		t.Error("expected at least one step with Pattern 'Handoff'")
	}

	// 验证 ObservationAdapter 被调用
	if obs.StepCount() == 0 {
		t.Error("expected ObservationAdapter to have recorded steps")
	}
}

func TestNewHandoff_SourceError(t *testing.T) {
	obs := &InMemoryObservationAdapter{}

	source := NewPipeline[any](obs,
		StepFunc[any, any]{
			execute: func(ctx context.Context, input any) (any, error) {
				return nil, NewStepError("SOURCE_FAIL", "source failed", false)
			},
			unitType: "Failing",
		},
	)
	target := NewPipeline[any](obs,
		AtomAsStep(Atom[any, any](func(x any) any { return x })),
	)

	handoff := NewHandoff(source, target, &DefaultSnapshotAdapter{}, InProcTransport, obs)

	_, _, err := handoff.Run(context.Background(), HandoffRequest{
		SourceID: "a", TargetID: "b", TaskType: "test", Payload: "x", Token: "t",
	})
	if err == nil {
		t.Fatal("expected error from source failure")
	}
}