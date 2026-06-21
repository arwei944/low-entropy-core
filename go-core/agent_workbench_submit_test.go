//go:build lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

package core

import (
	"context"
	"testing"
)

// TestAgentWorkbench_Submit_Approved tests a compliant submission.
func TestAgentWorkbench_Submit_Approved(t *testing.T) {
	wb := NewDefaultAgentWorkbench(nil, nil, nil, nil)
	ctx := context.Background()

	sub := AgentCodeSubmission{
		AgentID: "agent-test-01",
		TaskID:  "task-001",
		SourceCode: `package main

import "go-core"

type myAtom struct{}
func (a *myAtom) Run(ctx context.Context, input int) (int, error) {
	return input * 2, nil
}

func main() {}`,
		Manifest: []PrimitiveManifest{
			{
				PrimitiveType: "Atom",
				Name:          "myAtom",
				Layer:         "L1",
				InputType:     "int",
				OutputType:    "int",
			},
		},
		Attempt: 1,
	}

	result, err := wb.Submit(ctx, sub)
	if err != nil {
		t.Fatalf("Submit() unexpected error: %v", err)
	}

	if result.Status != SubmissionApproved {
		t.Errorf("expected Approved, got %s", result.Status)
	}

	if result.SubmissionID != sub.SubmissionID() {
		t.Errorf("SubmissionID mismatch: %s vs %s", result.SubmissionID, sub.SubmissionID())
	}

	stored, ok := wb.GetSubmission(result.SubmissionID)
	if !ok {
		t.Error("GetSubmission() should find the submission")
	}
	if stored.Status != SubmissionApproved {
		t.Errorf("stored status = %s, want Approved", stored.Status)
	}
}

// TestAgentWorkbench_Submit_Rejected tests a rejected submission.
func TestAgentWorkbench_Submit_Rejected(t *testing.T) {
	wb := NewDefaultAgentWorkbench(nil, nil, nil, nil)
	ctx := context.Background()

	sub := AgentCodeSubmission{
		AgentID:    "agent-bad-01",
		TaskID:     "task-001",
		SourceCode: `package main`,
		Manifest:   []PrimitiveManifest{},
		Attempt:    1,
	}

	result, err := wb.Submit(ctx, sub)
	if err != nil {
		t.Fatalf("Submit() unexpected error: %v", err)
	}

	if result.Status != SubmissionRejected {
		t.Errorf("expected Rejected, got %s", result.Status)
	}

	if !result.HasErrors() {
		t.Error("expected HasErrors() = true")
	}

	if len(result.Violations) == 0 {
		t.Error("expected violations, got none")
	}

	for _, v := range result.Violations {
		if v.Suggestion == "" {
			t.Errorf("violation %s has no suggestion", v.Rule)
		}
	}
}

// TestAgentWorkbench_Submit_InvalidManifestType tests invalid primitive type.
func TestAgentWorkbench_Submit_InvalidManifestType(t *testing.T) {
	wb := NewDefaultAgentWorkbench(nil, nil, nil, nil)
	ctx := context.Background()

	sub := AgentCodeSubmission{
		AgentID: "agent-bad-02",
		TaskID:  "task-001",
		SourceCode: `package main

func main() {}`,
		Manifest: []PrimitiveManifest{
			{
				PrimitiveType: "Handler",
				Name:          "myHandler",
				Layer:         "L1",
				InputType:     "any",
				OutputType:    "any",
			},
		},
		Attempt: 1,
	}

	result, err := wb.Submit(ctx, sub)
	if err != nil {
		t.Fatalf("Submit() unexpected error: %v", err)
	}

	if result.Status != SubmissionRejected {
		t.Errorf("expected Rejected for invalid primitive type, got %s", result.Status)
	}
}

// TestAgentWorkbench_Submit_MissingAgentID tests missing AgentID.
func TestAgentWorkbench_Submit_MissingAgentID(t *testing.T) {
	wb := NewDefaultAgentWorkbench(nil, nil, nil, nil)
	ctx := context.Background()

	sub := AgentCodeSubmission{
		AgentID:    "",
		TaskID:     "task-001",
		SourceCode: `package main`,
		Manifest: []PrimitiveManifest{
			{PrimitiveType: "Atom", Name: "test", Layer: "L1", InputType: "int", OutputType: "int"},
		},
		Attempt: 1,
	}

	result, err := wb.Submit(ctx, sub)
	if err != nil {
		t.Fatalf("Submit() unexpected error: %v", err)
	}

	if result.Status != SubmissionRejected {
		t.Errorf("expected Rejected for missing AgentID, got %s", result.Status)
	}
}

// TestAgentWorkbench_Submit_DuplicateManifestName tests duplicate manifest name.
func TestAgentWorkbench_Submit_DuplicateManifestName(t *testing.T) {
	wb := NewDefaultAgentWorkbench(nil, nil, nil, nil)
	ctx := context.Background()

	sub := AgentCodeSubmission{
		AgentID: "agent-dup-01",
		TaskID:  "task-001",
		SourceCode: `package main

func main() {}`,
		Manifest: []PrimitiveManifest{
			{PrimitiveType: "Atom", Name: "sameName", Layer: "L1", InputType: "int", OutputType: "int"},
			{PrimitiveType: "Port", Name: "sameName", Layer: "L1", InputType: "int", OutputType: "int"},
		},
		Attempt: 1,
	}

	result, err := wb.Submit(ctx, sub)
	if err != nil {
		t.Fatalf("Submit() unexpected error: %v", err)
	}

	if result.Status != SubmissionRejected {
		t.Errorf("expected Rejected for duplicate manifest name, got %s", result.Status)
	}
}
