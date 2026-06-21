//go:build lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

package core

import (
	"context"
	"fmt"
	"sync"
	"testing"
)

// ──────────────────────────────────────────────
// AccessControl Tests
// ──────────────────────────────────────────────

func TestAccessControlPort_Valid(t *testing.T) {
	ctx := context.Background()
	secret := []byte("test-secret")
	port := NewAccessControlPort(secret)

	token := NewCapabilityToken("agent-a", []string{"pipeline:write", "pipeline:read"})
	token.Sign(secret)

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
}

func TestAccessControlPort_NoToken(t *testing.T) {
	ctx := context.Background()
	port := NewAccessControlPort([]byte("secret"))

	req := AccessRequest{AgentID: "agent-a", Action: "read"}
	_, err := port.Validate(ctx, req)
	if err == nil {
		t.Fatal("expected error for missing token")
	}
}

func TestAccessControlPort_AgentIDMismatch(t *testing.T) {
	ctx := context.Background()
	secret := []byte("test")
	port := NewAccessControlPort(secret)

	token := NewCapabilityToken("agent-a", []string{"read"})
	token.Sign(secret)

	req := AccessRequest{AgentID: "agent-b", Action: "read", Token: token}
	_, err := port.Validate(ctx, req)
	if err == nil {
		t.Fatal("expected error for agent ID mismatch")
	}
}

func TestAccessPolicy_CheckAccess(t *testing.T) {
	policy := DefaultAccessPolicy()
	token := NewCapabilityToken("agent-a", []string{"pipeline:read", "pipeline:write"})
	token.Sign([]byte("secret"))

	if !policy.CheckAccess(token, "read") {
		t.Error("expected access for read")
	}
	if !policy.CheckAccess(token, "write") {
		t.Error("expected access for write")
	}
	if policy.CheckAccess(token, "delete") {
		t.Error("should NOT have access for delete")
	}
}

func TestAccessPolicy_Wildcard(t *testing.T) {
	policy := DefaultAccessPolicy()
	token := NewCapabilityToken("admin", []string{"pipeline:*"})
	token.Sign([]byte("secret"))

	if !policy.CheckAccess(token, "read") {
		t.Error("wildcard should grant read")
	}
	if !policy.CheckAccess(token, "delete") {
		t.Error("wildcard should grant delete")
	}
}

// ──────────────────────────────────────────────
// AuditTrail Tests
// ──────────────────────────────────────────────

func TestAuditTrailAdapter_Execute(t *testing.T) {
	ctx := context.Background()
	audit := NewAuditTrailAdapter()

	entry := AuditSuccess("agent-a", "deploy", "pipeline", "p1", "deployed successfully")
	result, err := audit.Execute(ctx, entry)
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if result.Result != "success" {
		t.Errorf("expected success, got %s", result.Result)
	}
	if audit.Count() != 1 {
		t.Errorf("expected 1 entry, got %d", audit.Count())
	}
}

func TestAuditTrailAdapter_Query(t *testing.T) {
	ctx := context.Background()
	audit := NewAuditTrailAdapter()

	audit.Execute(ctx, AuditSuccess("agent-a", "read", "pipeline", "p1", "ok"))
	audit.Execute(ctx, AuditFailure("agent-a", "write", "task", "t1", "failed", nil))
	audit.Execute(ctx, AuditDenied("agent-b", "delete", "pipeline", "p1", "no permission"))

	// Query by agent
	entries := audit.QueryByAgent("agent-a")
	if len(entries) != 2 {
		t.Errorf("expected 2 entries for agent-a, got %d", len(entries))
	}

	// Query by result
	denied := audit.QueryByResult("denied")
	if len(denied) != 1 {
		t.Errorf("expected 1 denied entry, got %d", len(denied))
	}

	// Query by resource
	entries = audit.QueryEntries("", "", "pipeline", "")
	if len(entries) != 2 {
		t.Errorf("expected 2 pipeline entries, got %d", len(entries))
	}
}

func TestAuditTrailAdapter_Clear(t *testing.T) {
	ctx := context.Background()
	audit := NewAuditTrailAdapter()
	audit.Execute(ctx, AuditSuccess("a", "r", "res", "r1", "ok"))
	audit.Clear()
	if audit.Count() != 0 {
		t.Errorf("expected 0 after clear, got %d", audit.Count())
	}
}

func TestAuditTrailAdapter_Concurrency(t *testing.T) {
	ctx := context.Background()
	audit := NewAuditTrailAdapter()
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			audit.Execute(ctx, AuditSuccess(fmt.Sprintf("agent-%d", id), "op", "res", "r1", "ok"))
		}(i)
	}
	wg.Wait()

	if audit.Count() != 100 {
		t.Errorf("expected 100 entries, got %d", audit.Count())
	}
}
