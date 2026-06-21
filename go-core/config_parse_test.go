//go:build lecore_tier2 || lecore_tier3 || lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

package core_test

import (
	"strings"
	"testing"

	. "low-entropy-core/go-core"
)

const validJSON = `{
  "id": "pipeline-1",
  "name": "test pipeline",
  "steps": [
    {"type": "atom", "name": "step1", "params": {"key": "value"}},
    {"type": "port", "name": "step2", "params": {}}
  ]
}`

func TestParseConfig_ValidJSON(t *testing.T) {
	config, err := ParseConfig([]byte(validJSON))
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if config == nil {
		t.Fatal("expected non-nil config")
	}
	if config.ID != "pipeline-1" {
		t.Errorf("expected ID='pipeline-1', got '%s'", config.ID)
	}
	if config.Name != "test pipeline" {
		t.Errorf("expected Name='test pipeline', got '%s'", config.Name)
	}
	if len(config.Steps) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(config.Steps))
	}

	// Verify first step
	if config.Steps[0].Type != "atom" {
		t.Errorf("expected step 0 type='atom', got '%s'", config.Steps[0].Type)
	}
	if config.Steps[0].Name != "step1" {
		t.Errorf("expected step 0 name='step1', got '%s'", config.Steps[0].Name)
	}
	if config.Steps[0].Params["key"] != "value" {
		t.Errorf("expected step 0 params[key]='value', got '%v'", config.Steps[0].Params["key"])
	}

	// Verify second step
	if config.Steps[1].Type != "port" {
		t.Errorf("expected step 1 type='port', got '%s'", config.Steps[1].Type)
	}
	if config.Steps[1].Name != "step2" {
		t.Errorf("expected step 1 name='step2', got '%s'", config.Steps[1].Name)
	}
	if len(config.Steps[1].Params) != 0 {
		t.Errorf("expected step 1 params to be empty, got %d entries", len(config.Steps[1].Params))
	}
}

func TestParseConfig_InvalidJSON(t *testing.T) {
	_, err := ParseConfig([]byte(`{invalid json`))
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
	if !strings.Contains(err.Error(), "invalid JSON") {
		t.Errorf("expected error to mention 'invalid JSON', got: %v", err)
	}
}

func TestParseConfig_EmptyID(t *testing.T) {
	_, err := ParseConfig([]byte(`{"id":"","name":"test","steps":[{"type":"atom","name":"s1","params":{}}]}`))
	if err == nil {
		t.Fatal("expected error for empty ID, got nil")
	}
	if !strings.Contains(err.Error(), "ID must not be empty") {
		t.Errorf("expected error to mention 'ID must not be empty', got: %v", err)
	}
}

func TestParseConfig_EmptySteps(t *testing.T) {
	_, err := ParseConfig([]byte(`{"id":"pipeline-1","name":"test","steps":[]}`))
	if err == nil {
		t.Fatal("expected error for empty steps, got nil")
	}
	if !strings.Contains(err.Error(), "no steps defined") {
		t.Errorf("expected error to mention 'no steps defined', got: %v", err)
	}
}

func TestParseConfig_InvalidStepType(t *testing.T) {
	_, err := ParseConfig([]byte(`{"id":"pipeline-1","name":"test","steps":[{"type":"invalid_type","name":"s1","params":{}}]}`))
	if err == nil {
		t.Fatal("expected error for invalid step type, got nil")
	}
	if !strings.Contains(err.Error(), "invalid type") {
		t.Errorf("expected error to mention 'invalid type', got: %v", err)
	}
	if !strings.Contains(err.Error(), "invalid_type") {
		t.Errorf("expected error to mention the invalid type name, got: %v", err)
	}
}
