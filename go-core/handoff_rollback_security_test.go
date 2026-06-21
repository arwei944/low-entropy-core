//go:build lecore_tier3 || lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

package core

import (
	"context"
	"testing"
	"time"
)

func TestRollbackHandoff_Success(t *testing.T) {
	ctx := context.Background()
	obs := &InMemoryObservationAdapter{}
	persistence := NewInMemorySnapshotAdapter()
	transport := NewInProcHandoffTransport()

	snap := NewDevSnapshot("task-rb", "agent-src", "coding", "rollback test")
	checksum, _ := snap.ComputeChecksum()

	persistence.CreateSnapshot(snap)

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

func TestCapabilityToken_SignVerify(t *testing.T) {
	secret := []byte("my-secret-key")
	token := NewCapabilityToken("agent-a", []string{"read", "write", "deploy"})
	token.Sign(secret)

	if !token.Verify(secret) {
		t.Error("signature verification failed")
	}

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

	token2Copy := NewCapabilityToken("agent-b", []string{"read"})
	token2Copy.Signature = token1.Signature
	if token2Copy.Verify(secret) {
		t.Error("cross-token signature should not verify")
	}
}
