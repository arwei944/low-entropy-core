//go:build lecore_tier3 || lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

package core_test

import (
	"context"
	"testing"

	. "low-entropy-core/go-core"
)

func TestAuditTrail_ChainIntegrity(t *testing.T) {
	ctx := context.Background()
	audit := NewAuditTrailAdapter()

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

	audit.Execute(ctx, AuditSuccess("agent-a", "read", "pipeline", "p1", "successful read"))
	audit.Execute(ctx, AuditSuccess("agent-a", "write", "task", "t1", "successful write"))
	audit.Execute(ctx, AuditFailure("agent-a", "deploy", "snapshot", "s1", "deploy failed", nil))
	audit.Execute(ctx, AuditDenied("agent-b", "delete", "pipeline", "p1", "no permission"))
	audit.Execute(ctx, AuditSuccess("agent-b", "read", "pipeline", "p2", "successful read"))
	audit.Execute(ctx, AuditFailure("agent-c", "write", "task", "t2", "write failed", nil))

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

	readEntries := audit.QueryEntries("", "read", "", "")
	if len(readEntries) != 2 {
		t.Errorf("expected 2 read entries, got %d", len(readEntries))
	}

	writeEntries := audit.QueryEntries("", "write", "", "")
	if len(writeEntries) != 2 {
		t.Errorf("expected 2 write entries, got %d", len(writeEntries))
	}

	pipelineEntries := audit.QueryEntries("", "", "pipeline", "")
	if len(pipelineEntries) != 3 {
		t.Errorf("expected 3 pipeline entries, got %d", len(pipelineEntries))
	}

	taskEntries := audit.QueryEntries("", "", "task", "")
	if len(taskEntries) != 2 {
		t.Errorf("expected 2 task entries, got %d", len(taskEntries))
	}

	combined := audit.QueryEntries("agent-a", "read", "", "success")
	if len(combined) != 1 {
		t.Errorf("expected 1 combined entry, got %d", len(combined))
	}
}

func TestAuditTrail_Helpers(t *testing.T) {
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

	failure := AuditFailure("agent-b", "write", "task", "t1", "write attempt", nil)
	if failure.Result != "failure" {
		t.Errorf("expected Result='failure', got '%s'", failure.Result)
	}
	if failure.AgentID != "agent-b" {
		t.Errorf("expected AgentID='agent-b', got '%s'", failure.AgentID)
	}

	testErr := &StepError{Code: "WRITE_ERR", Message: "disk full", Recoverable: true}
	failureWithErr := AuditFailure("agent-c", "deploy", "snapshot", "s1", "deploy attempt", testErr)
	if failureWithErr.Result != "failure" {
		t.Errorf("expected Result='failure', got '%s'", failureWithErr.Result)
	}
	if failureWithErr.Details != "deploy attempt: WRITE_ERR: disk full" {
		t.Errorf("expected Details with error, got '%s'", failureWithErr.Details)
	}

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
