//go:build lecore_tier3 || lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

package core

import (
	"testing"
)

func TestValidationResult_Passed(t *testing.T) {
	result := &ValidationResult{Passed: true}
	if !result.Passed {
		t.Error("expected passed")
	}
	result.Passed = false
	if result.Passed {
		t.Error("expected not passed")
	}
}

func TestValidationResult_FailureCount(t *testing.T) {
	result := &ValidationResult{
		Failures: []ValidationFailure{
			{Stage: "compile", Message: "error 1"},
			{Stage: "test", Message: "error 2"},
		},
	}
	if len(result.Failures) != 2 {
		t.Errorf("expected 2 failures, got %d", len(result.Failures))
	}
}

func TestValidationFailure_Fields(t *testing.T) {
	f := ValidationFailure{
		Stage:   "compile",
		Message: "build failed",
		Detail:  "error: undefined: Foo",
	}
	if f.Stage != "compile" {
		t.Errorf("stage: %s", f.Stage)
	}
	if f.Message == "" {
		t.Error("message should not be empty")
	}
}

func TestValidationFailure_EmptyDetail(t *testing.T) {
	f := ValidationFailure{
		Stage:   "test",
		Message: "test failed",
	}
	if f.Stage != "test" {
		t.Errorf("stage: %s", f.Stage)
	}
	// Detail can be empty
}

func TestValidationResult_CheckCount(t *testing.T) {
	result := &ValidationResult{CheckCount: 3}
	if result.CheckCount != 3 {
		t.Errorf("expected 3, got %d", result.CheckCount)
	}
}

func TestValidationResult_Warnings(t *testing.T) {
	result := &ValidationResult{
		Warnings: []string{"warning 1", "warning 2"},
	}
	if len(result.Warnings) != 2 {
		t.Errorf("expected 2 warnings, got %d", len(result.Warnings))
	}
}