//go:build lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

package core

import (
	"context"
	"testing"
	"time"
)

// TestAgentCodeSubmission_SubmissionID tests SubmissionID generation.
func TestAgentCodeSubmission_SubmissionID(t *testing.T) {
	sub := AgentCodeSubmission{
		AgentID: "agent-calc-01",
		TaskID:  "task-001",
		Attempt: 1,
	}
	expected := "sub-agent-calc-01-task-001-1"
	if got := sub.SubmissionID(); got != expected {
		t.Errorf("SubmissionID() = %s, want %s", got, expected)
	}

	sub.Attempt = 3
	expected = "sub-agent-calc-01-task-001-3"
	if got := sub.SubmissionID(); got != expected {
		t.Errorf("SubmissionID() = %s, want %s", got, expected)
	}
}

// TestPrimitiveManifest_Validate tests basic manifest validation.
func TestPrimitiveManifest_Validate(t *testing.T) {
	tests := []struct {
		name     string
		manifest PrimitiveManifest
		wantErr  bool
	}{
		{
			name: "valid atom manifest",
			manifest: PrimitiveManifest{
				PrimitiveType: "Atom",
				Name:          "hashPasswordAtom",
				Layer:         "L1",
				InputType:     "RegisterReq",
				OutputType:    "User",
			},
			wantErr: false,
		},
		{
			name: "valid port manifest",
			manifest: PrimitiveManifest{
				PrimitiveType: "Port",
				Name:          "validateRegisterPort",
				Layer:         "L1",
				InputType:     "RegisterReq",
				OutputType:    "RegisterReq",
			},
			wantErr: false,
		},
		{
			name: "valid adapter manifest",
			manifest: PrimitiveManifest{
				PrimitiveType: "Adapter",
				Name:          "saveUserAdapter",
				Layer:         "L7",
				InputType:     "User",
				OutputType:    "error",
			},
			wantErr: false,
		},
		{
			name: "empty name",
			manifest: PrimitiveManifest{
				PrimitiveType: "Atom",
				Name:          "",
			},
			wantErr: true,
		},
		{
			name: "invalid primitive type",
			manifest: PrimitiveManifest{
				PrimitiveType: "Handler",
				Name:          "myHandler",
			},
			wantErr: true,
		},
		{
			name: "empty primitive type and name",
			manifest: PrimitiveManifest{
				PrimitiveType: "",
				Name:          "",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			violations := tt.manifest.Validate()
			hasErrors := false
			for _, v := range violations {
				if v.Severity == "error" {
					hasErrors = true
					break
				}
			}
			if tt.wantErr && !hasErrors {
				t.Errorf("Validate() expected errors but got none")
			}
			if !tt.wantErr && hasErrors {
				t.Errorf("Validate() unexpected errors: %v", violations)
			}
		})
	}
}

// TestValidPrimitiveTypes tests valid primitive type set.
func TestValidPrimitiveTypes(t *testing.T) {
	validTypes := []string{"Atom", "Port", "Adapter", "Composer"}
	for _, pt := range validTypes {
		if !ValidPrimitiveTypes[pt] {
			t.Errorf("expected %s to be valid primitive type", pt)
		}
	}

	invalidTypes := []string{"", "Handler", "Service", "Controller", "Function"}
	for _, pt := range invalidTypes {
		if ValidPrimitiveTypes[pt] {
			t.Errorf("expected %s to be invalid primitive type", pt)
		}
	}
}

// TestSubmissionResult_HasErrors tests HasErrors method.
func TestSubmissionResult_HasErrors(t *testing.T) {
	tests := []struct {
		name     string
		result   SubmissionResult
		expected bool
	}{
		{
			name:     "no violations",
			result:   SubmissionResult{Violations: nil},
			expected: false,
		},
		{
			name: "only warnings",
			result: SubmissionResult{Violations: []Violation{
				{Rule: "test", Severity: "warn", Detail: "test"},
			}},
			expected: false,
		},
		{
			name: "has errors",
			result: SubmissionResult{Violations: []Violation{
				{Rule: "test", Severity: "error", Detail: "test"},
			}},
			expected: true,
		},
		{
			name: "mixed",
			result: SubmissionResult{Violations: []Violation{
				{Rule: "warn1", Severity: "warn", Detail: "test"},
				{Rule: "err1", Severity: "error", Detail: "test"},
			}},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.result.HasErrors(); got != tt.expected {
				t.Errorf("HasErrors() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// TestAgentWorkbench_Submit_ContextCancellation tests context cancellation.
func TestAgentWorkbench_Submit_ContextCancellation(t *testing.T) {
	wb := NewDefaultAgentWorkbench(nil, nil, nil, nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	sub := AgentCodeSubmission{
		AgentID: "agent-test", TaskID: "task-001",
		SourceCode: `package main`,
		Manifest:   []PrimitiveManifest{{PrimitiveType: "Atom", Name: "test", Layer: "L1", InputType: "int", OutputType: "int"}},
		Attempt:    1,
	}

	_, err := wb.Submit(ctx, sub)
	if err == nil {
		t.Error("expected error for cancelled context")
	}
}

// TestAgentWorkbench_Submit_Concurrent tests concurrent submissions.
func TestAgentWorkbench_Submit_Concurrent(t *testing.T) {
	wb := NewDefaultAgentWorkbench(nil, nil, nil, nil)
	ctx := context.Background()

	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(idx int) {
			sub := AgentCodeSubmission{
				AgentID:    "agent-concurrent",
				TaskID:     "task-001",
				SourceCode: `package main`,
				Manifest: []PrimitiveManifest{
					{PrimitiveType: "Atom", Name: "test", Layer: "L1", InputType: "int", OutputType: "int"},
				},
				Attempt:     idx + 1,
				SubmittedAt: time.Now(),
			}
			_, err := wb.Submit(ctx, sub)
			if err != nil {
				t.Errorf("concurrent Submit() error: %v", err)
			}
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	results := wb.ListSubmissionsByAgent("agent-concurrent")
	if len(results) != 10 {
		t.Errorf("expected 10 submissions, got %d", len(results))
	}
}
