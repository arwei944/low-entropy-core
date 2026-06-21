//go:build lecore_tier2 || lecore_tier3 || lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

package core_test

import (
	"strings"
	"testing"

	. "low-entropy-core/go-core"
)

func TestValidateConfig_MultipleErrors(t *testing.T) {
	// Construct a config with multiple issues: empty ID, invalid step type, empty step names
	config := &PipelineConfig{
		ID:   "",
		Name: "bad config",
		Steps: []StepConfig{
			{Type: "unknown", Name: "", Params: nil},
			{Type: "atom", Name: "", Params: nil},
		},
	}

	errs := ValidateConfig(config)
	if len(errs) < 3 {
		t.Fatalf("expected at least 3 validation errors, got %d", len(errs))
	}

	errMessages := make([]string, len(errs))
	for i, e := range errs {
		errMessages[i] = e.Error()
	}

	foundEmptyID := false
	foundInvalidType := false
	foundEmptyName := false
	for _, msg := range errMessages {
		if strings.Contains(msg, "ID must not be empty") {
			foundEmptyID = true
		}
		if strings.Contains(msg, "invalid type") {
			foundInvalidType = true
		}
		if strings.Contains(msg, "empty name") {
			foundEmptyName = true
		}
	}

	if !foundEmptyID {
		t.Error("expected validation error for empty ID")
	}
	if !foundInvalidType {
		t.Error("expected validation error for invalid step type")
	}
	if !foundEmptyName {
		t.Error("expected validation error for empty step name")
	}
}

func TestParseAndValidateConfig(t *testing.T) {
	// Test with valid JSON - should return no validation errors
	config, errs, parseErr := ParseAndValidateConfig([]byte(validJSON))
	if parseErr != nil {
		t.Fatalf("expected no parse error, got: %v", parseErr)
	}
	if config == nil {
		t.Fatal("expected non-nil config")
	}
	if len(errs) != 0 {
		t.Fatalf("expected 0 validation errors, got %d: %v", len(errs), errs)
	}
	if config.ID != "pipeline-1" {
		t.Errorf("expected ID='pipeline-1', got '%s'", config.ID)
	}

	// Test with invalid JSON - should return parse error and nil config/errs
	config2, errs2, parseErr2 := ParseAndValidateConfig([]byte(`{invalid`))
	if parseErr2 == nil {
		t.Fatal("expected parse error for invalid JSON, got nil")
	}
	if config2 != nil {
		t.Error("expected nil config when JSON is invalid")
	}
	if errs2 != nil {
		t.Error("expected nil errs when JSON is invalid")
	}
}
