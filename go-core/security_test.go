package core_test

import (
	"context"
	"testing"
	"time"

	. "low-entropy-core/go-core"
)

// ──────────────────────────────────────────────
// Token Signature Verification Tests
// ──────────────────────────────────────────────

func TestCapabilityToken_SignAndVerify(t *testing.T) {
	secretKey := []byte("test-secret-key-12345")
	token := NewCapabilityToken("agent-1", []string{"pipeline:read", "pipeline:write"})

	// Sign the token
	token.Sign(secretKey)

	// Verify the signature
	if !token.Verify(secretKey) {
		t.Fatal("expected signature verification to pass")
	}
}

func TestCapabilityToken_TamperedToken(t *testing.T) {
	secretKey := []byte("test-secret-key-12345")
	token := NewCapabilityToken("agent-1", []string{"pipeline:read", "pipeline:write"})

	// Sign the token
	token.Sign(secretKey)

	// Tamper with the token's capabilities
	token.Capabilities = append(token.Capabilities, "pipeline:admin")

	// Verify should fail because the payload has changed
	if token.Verify(secretKey) {
		t.Fatal("expected signature verification to fail for tampered token")
	}
}

func TestCapabilityToken_WrongKey(t *testing.T) {
	correctKey := []byte("correct-secret-key")
	wrongKey := []byte("wrong-secret-key-123")

	token := NewCapabilityToken("agent-1", []string{"pipeline:read"})

	// Sign with the correct key
	token.Sign(correctKey)

	// Verify with the wrong key should fail
	if token.Verify(wrongKey) {
		t.Fatal("expected signature verification to fail with wrong key")
	}
}

// ──────────────────────────────────────────────
// Token Expiration Tests
// ──────────────────────────────────────────────

func TestCapabilityToken_ExpiredWithValidSig(t *testing.T) {
	secretKey := []byte("test-secret-key")
	token := NewCapabilityToken("agent-1", []string{"pipeline:read"})
	token.Sign(secretKey)

	// Manually set expiration to 1 hour in the past
	token.ExpiresAt = time.Now().Add(-1 * time.Hour)

	if !token.IsExpired() {
		t.Fatal("expected token to be expired")
	}

	// Re-sign with updated expiration so signature is still valid
	token.Sign(secretKey)

	// Verify still works (payload matches)
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

// ──────────────────────────────────────────────
// AccessControl Tests
// ──────────────────────────────────────────────

func TestAccessControl_Granted(t *testing.T) {
	ctx := context.Background()
	secretKey := []byte("test-secret-key")
	port := NewAccessControlPort(secretKey)

	token := NewCapabilityToken("agent-a", []string{"pipeline:write", "pipeline:read"})
	token.Sign(secretKey)

	req := AccessRequest{
		AgentID: "agent-a",
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

	// Agent tries to write but only has read capability
	req := AccessRequest{
		AgentID: "agent-a",
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
		AgentID: "agent-a",
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

	// Agent-b tries to use agent-a's token
	req := AccessRequest{
		AgentID: "agent-b",
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

	// Set the token to have already expired
	token.ExpiresAt = time.Now().Add(-1 * time.Hour)
	token.Sign(secretKey)

	req := AccessRequest{
		AgentID: "agent-a",
		Action:   "read",
		Resource: "pipeline",
		Token:    token,
	}

	_, err := port.Validate(ctx, req)
	if err == nil {
		t.Fatal("expected access denied for expired token")
	}
}

// ──────────────────────────────────────────────
// Audit Trail Chain Integrity Tests
// ──────────────────────────────────────────────

func TestAuditTrail_ChainIntegrity(t *testing.T) {
	ctx := context.Background()
	audit := NewAuditTrailAdapter()

	// Record 10 entries
	for i := 0; i < 10; i++ {
		_, err := audit.Execute(ctx, AuditSuccess(
			"agent-a",
			"read",
			"pipeline",
			"p1",
			"entry-"+string(rune('0'+i%10)),
		))
		if err != nil {
			t.Fatalf("failed to record entry %d: %v", i, err)
		}
	}

	// Verify 10 entries
	if audit.Count() != 10 {
		t.Errorf("expected 10 entries, got %d", audit.Count())
	}

	entries := audit.GetEntries()
	if len(entries) != 10 {
		t.Errorf("expected 10 entries from GetEntries, got %d", len(entries))
	}
}

func TestAuditTrail_Query(t *testing.T) {
	ctx := context.Background()
	audit := NewAuditTrailAdapter()

	// Record diverse entries
	audit.Execute(ctx, AuditSuccess("agent-a", "read", "pipeline", "p1", "successful read"))
	audit.Execute(ctx, AuditSuccess("agent-a", "write", "task", "t1", "successful write"))
	audit.Execute(ctx, AuditFailure("agent-a", "deploy", "snapshot", "s1", "deploy failed", nil))
	audit.Execute(ctx, AuditDenied("agent-b", "delete", "pipeline", "p1", "no permission"))
	audit.Execute(ctx, AuditSuccess("agent-b", "read", "pipeline", "p2", "successful read"))
	audit.Execute(ctx, AuditFailure("agent-c", "write", "task", "t2", "write failed", nil))

	// Query by agent
	agentAEntries := audit.QueryByAgent("agent-a")
	if len(agentAEntries) != 3 {
		t.Errorf("expected 3 entries for agent-a, got %d", len(agentAEntries))
	}

	agentBEntries := audit.QueryByAgent("agent-b")
	if len(agentBEntries) != 2 {
		t.Errorf("expected 2 entries for agent-b, got %d", len(agentBEntries))
	}

	agentCEntries := audit.QueryByAgent("agent-c")
	if len(agentCEntries) != 1 {
		t.Errorf("expected 1 entry for agent-c, got %d", len(agentCEntries))
	}

	// Query by result
	successEntries := audit.QueryByResult("success")
	if len(successEntries) != 3 {
		t.Errorf("expected 3 success entries, got %d", len(successEntries))
	}

	failureEntries := audit.QueryByResult("failure")
	if len(failureEntries) != 2 {
		t.Errorf("expected 2 failure entries, got %d", len(failureEntries))
	}

	deniedEntries := audit.QueryByResult("denied")
	if len(deniedEntries) != 1 {
		t.Errorf("expected 1 denied entry, got %d", len(deniedEntries))
	}

	// Query by action
	readEntries := audit.QueryEntries("", "read", "", "")
	if len(readEntries) != 2 {
		t.Errorf("expected 2 read entries, got %d", len(readEntries))
	}

	writeEntries := audit.QueryEntries("", "write", "", "")
	if len(writeEntries) != 2 {
		t.Errorf("expected 2 write entries, got %d", len(writeEntries))
	}

	// Query by resource
	pipelineEntries := audit.QueryEntries("", "", "pipeline", "")
	if len(pipelineEntries) != 3 {
		t.Errorf("expected 3 pipeline entries, got %d", len(pipelineEntries))
	}

	taskEntries := audit.QueryEntries("", "", "task", "")
	if len(taskEntries) != 2 {
		t.Errorf("expected 2 task entries, got %d", len(taskEntries))
	}

	// Combined query: agent-a + action=read + result=success
	combined := audit.QueryEntries("agent-a", "read", "", "success")
	if len(combined) != 1 {
		t.Errorf("expected 1 combined entry, got %d", len(combined))
	}
}

func TestAuditTrail_Helpers(t *testing.T) {
	// Test AuditSuccess
	success := AuditSuccess("agent-a", "read", "pipeline", "p1", "read completed")
	if success.AgentID != "agent-a" {
		t.Errorf("expected AgentID='agent-a', got '%s'", success.AgentID)
	}
	if success.Action != "read" {
		t.Errorf("expected Action='read', got '%s'", success.Action)
	}
	if success.Resource != "pipeline" {
		t.Errorf("expected Resource='pipeline', got '%s'", success.Resource)
	}
	if success.ResourceID != "p1" {
		t.Errorf("expected ResourceID='p1', got '%s'", success.ResourceID)
	}
	if success.Result != "success" {
		t.Errorf("expected Result='success', got '%s'", success.Result)
	}
	if success.Details != "read completed" {
		t.Errorf("expected Details='read completed', got '%s'", success.Details)
	}
	if success.ID == "" {
		t.Error("expected non-empty ID")
	}
	if success.Timestamp.IsZero() {
		t.Error("expected non-zero Timestamp")
	}

	// Test AuditFailure
	failure := AuditFailure("agent-b", "write", "task", "t1", "write attempt", nil)
	if failure.Result != "failure" {
		t.Errorf("expected Result='failure', got '%s'", failure.Result)
	}
	if failure.AgentID != "agent-b" {
		t.Errorf("expected AgentID='agent-b', got '%s'", failure.AgentID)
	}

	// Test AuditFailure with error
	testErr := &StepError{Code: "WRITE_ERR", Message: "disk full", Recoverable: true}
	failureWithErr := AuditFailure("agent-c", "deploy", "snapshot", "s1", "deploy attempt", testErr)
	if failureWithErr.Result != "failure" {
		t.Errorf("expected Result='failure', got '%s'", failureWithErr.Result)
	}
	if failureWithErr.Details != "deploy attempt: WRITE_ERR: disk full" {
		t.Errorf("expected Details with error, got '%s'", failureWithErr.Details)
	}

	// Test AuditDenied
	denied := AuditDenied("agent-d", "delete", "pipeline", "p2", "insufficient permissions")
	if denied.Result != "denied" {
		t.Errorf("expected Result='denied', got '%s'", denied.Result)
	}
	if denied.AgentID != "agent-d" {
		t.Errorf("expected AgentID='agent-d', got '%s'", denied.AgentID)
	}
	if denied.Details != "insufficient permissions" {
		t.Errorf("expected Details='insufficient permissions', got '%s'", denied.Details)
	}
}