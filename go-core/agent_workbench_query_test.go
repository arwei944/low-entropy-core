//go:build lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

package core

import (
	"context"
	"testing"
)

// TestAgentWorkbench_SubmitAndRun tests the SubmitAndRun flow.
func TestAgentWorkbench_SubmitAndRun(t *testing.T) {
	wb := NewDefaultAgentWorkbench(nil, nil, nil, nil)
	ctx := context.Background()

	sub := AgentCodeSubmission{
		AgentID: "agent-test-01",
		TaskID:  "task-001",
		SourceCode: `package main

import "go-core"

func main() {}`,
		Manifest: []PrimitiveManifest{
			{PrimitiveType: "Atom", Name: "testAtom", Layer: "L1", InputType: "int", OutputType: "int"},
		},
		Attempt: 1,
	}

	result, err := wb.SubmitAndRun(ctx, sub)
	if err != nil {
		t.Fatalf("SubmitAndRun() unexpected error: %v", err)
	}

	if result.Status != SubmissionApproved {
		t.Errorf("expected Approved, got %s", result.Status)
	}

	if len(result.ExecutionSteps) == 0 {
		t.Error("expected ExecutionSteps, got none")
	}
}

// TestAgentWorkbench_SubmitAndRun_Rejected tests SubmitAndRun when rejected.
func TestAgentWorkbench_SubmitAndRun_Rejected(t *testing.T) {
	wb := NewDefaultAgentWorkbench(nil, nil, nil, nil)
	ctx := context.Background()

	sub := AgentCodeSubmission{
		AgentID:    "",
		TaskID:     "",
		SourceCode: "",
		Manifest:   []PrimitiveManifest{},
		Attempt:    1,
	}

	result, err := wb.SubmitAndRun(ctx, sub)
	if err != nil {
		t.Fatalf("SubmitAndRun() unexpected error: %v", err)
	}

	if result.Status != SubmissionRejected {
		t.Errorf("expected Rejected, got %s", result.Status)
	}

	if len(result.ExecutionSteps) != 0 {
		t.Errorf("expected no ExecutionSteps when rejected, got %d", len(result.ExecutionSteps))
	}
}

// TestAgentWorkbench_ListSubmissionsByAgent tests agent submission queries.
func TestAgentWorkbench_ListSubmissionsByAgent(t *testing.T) {
	wb := NewDefaultAgentWorkbench(nil, nil, nil, nil)
	ctx := context.Background()

	subA1 := AgentCodeSubmission{
		AgentID: "agent-a", TaskID: "task-1",
		SourceCode: `package main`,
		Manifest:   []PrimitiveManifest{{PrimitiveType: "Atom", Name: "a1", Layer: "L1", InputType: "int", OutputType: "int"}},
		Attempt:    1,
	}
	subA2 := AgentCodeSubmission{
		AgentID: "agent-a", TaskID: "task-2",
		SourceCode: `package main`,
		Manifest:   []PrimitiveManifest{{PrimitiveType: "Atom", Name: "a2", Layer: "L1", InputType: "int", OutputType: "int"}},
		Attempt:    1,
	}

	subB1 := AgentCodeSubmission{
		AgentID: "agent-b", TaskID: "task-1",
		SourceCode: `package main`,
		Manifest:   []PrimitiveManifest{{PrimitiveType: "Atom", Name: "b1", Layer: "L1", InputType: "int", OutputType: "int"}},
		Attempt:    1,
	}

	wb.Submit(ctx, subA1)
	wb.Submit(ctx, subA2)
	wb.Submit(ctx, subB1)

	resultsA := wb.ListSubmissionsByAgent("agent-a")
	if len(resultsA) != 2 {
		t.Errorf("expected 2 submissions for agent-a, got %d", len(resultsA))
	}

	resultsB := wb.ListSubmissionsByAgent("agent-b")
	if len(resultsB) != 1 {
		t.Errorf("expected 1 submission for agent-b, got %d", len(resultsB))
	}
}
