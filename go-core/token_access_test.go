//go:build lecore_tier3 || lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

package core_test

import (
	"context"
	"testing"
	"time"

	. "low-entropy-core/go-core"
)

func TestCapabilityToken_SignAndVerify(t *testing.T) {
	secretKey := []byte("test-secret-key-12345")
	token := NewCapabilityToken("agent-1", []string{"pipeline:read", "pipeline:write"})

	token.Sign(secretKey)

	if !token.Verify(secretKey) {
		t.Fatal("expected signature verification to pass")
	}
}

func TestCapabilityToken_TamperedToken(t *testing.T) {
	secretKey := []byte("test-secret-key-12345")
	token := NewCapabilityToken("agent-1", []string{"pipeline:read", "pipeline:write"})

	token.Sign(secretKey)

	token.Capabilities = append(token.Capabilities, "pipeline:admin")

	if token.Verify(secretKey) {
		t.Fatal("expected signature verification to fail for tampered token")
	}
}

func TestCapabilityToken_WrongKey(t *testing.T) {
	correctKey := []byte("correct-secret-key")
	wrongKey := []byte("wrong-secret-key-123")

	token := NewCapabilityToken("agent-1", []string{"pipeline:read"})

	token.Sign(correctKey)

	if token.Verify(wrongKey) {
		t.Fatal("expected signature verification to fail with wrong key")
	}
}

func TestCapabilityToken_ExpiredWithValidSig(t *testing.T) {
	secretKey := []byte("test-secret-key")
	token := NewCapabilityToken("agent-1", []string{"pipeline:read"})
	token.Sign(secretKey)

	token.ExpiresAt = time.Now().Add(-1 * time.Hour)

	if !token.IsExpired() {
		t.Fatal("expected token to be expired")
	}

	token.Sign(secretKey)

	if !token.Verify(secretKey) {
		t.Fatal("expected signature verification to pass for expired token")
	}
}

func TestCapabilityToken_NotExpired(t *testing.T) {
	secretKey := []byte("test-secret-key")
	token := NewCapabilityToken("agent-1", []string{"pipeline:read"})
	token.Sign(secretKey)

	if token.IsExpired() {
		t.Fatal("expected token to not be expired")
	}
}

func TestAccessControl_Granted(t *testing.T) {
	ctx := context.Background()
	secretKey := []byte("test-secret-key")
	port := NewAccessControlPort(secretKey)

	token := NewCapabilityToken("agent-a", []string{"pipeline:write", "pipeline:read"})
	token.Sign(secretKey)

	req := AccessRequest{
		AgentID:  "agent-a",
		Action:   "write",
		Resource: "pipeline",
		Token:    token,
	}

	decision, err := port.Validate(ctx, req)
	if err != nil {
		t.Fatalf("expected access granted, got error: %v", err)
	}
	if !decision.Allowed {
		t.Fatal("expected access allowed")
	}
	if decision.AgentID != "agent-a" {
		t.Errorf("expected AgentID='agent-a', got '%s'", decision.AgentID)
	}
	if decision.Action != "write" {
		t.Errorf("expected Action='write', got '%s'", decision.Action)
	}
}

func TestAccessControl_Denied(t *testing.T) {
	ctx := context.Background()
	secretKey := []byte("test-secret-key")
	port := NewAccessControlPort(secretKey)

	token := NewCapabilityToken("agent-a", []string{"pipeline:read"})
	token.Sign(secretKey)

	req := AccessRequest{
		AgentID:  "agent-a",
		Action:   "write",
		Resource: "pipeline",
		Token:    token,
	}

	_, err := port.Validate(ctx, req)
	if err == nil {
		t.Fatal("expected access denied for insufficient capability")
	}
}

func TestAccessControl_NoToken(t *testing.T) {
	ctx := context.Background()
	secretKey := []byte("test-secret-key")
	port := NewAccessControlPort(secretKey)

	req := AccessRequest{
		AgentID:  "agent-a",
		Action:   "read",
		Resource: "pipeline",
		Token:    nil,
	}

	_, err := port.Validate(ctx, req)
	if err == nil {
		t.Fatal("expected access denied for missing token")
	}
}

func TestAccessControl_WrongAgent(t *testing.T) {
	ctx := context.Background()
	secretKey := []byte("test-secret-key")
	port := NewAccessControlPort(secretKey)

	token := NewCapabilityToken("agent-a", []string{"pipeline:read"})
	token.Sign(secretKey)

	req := AccessRequest{
		AgentID:  "agent-b",
		Action:   "read",
		Resource: "pipeline",
		Token:    token,
	}

	_, err := port.Validate(ctx, req)
	if err == nil {
		t.Fatal("expected access denied for agent ID mismatch")
	}
}

func TestAccessControl_ExpiredToken(t *testing.T) {
	ctx := context.Background()
	secretKey := []byte("test-secret-key")
	port := NewAccessControlPort(secretKey)

	token := NewCapabilityToken("agent-a", []string{"pipeline:read"})

	token.ExpiresAt = time.Now().Add(-1 * time.Hour)
	token.Sign(secretKey)

	req := AccessRequest{
		AgentID:  "agent-a",
		Action:   "read",
		Resource: "pipeline",
		Token:    token,
	}

	_, err := port.Validate(ctx, req)
	if err == nil {
		t.Fatal("expected access denied for expired token")
	}
}
