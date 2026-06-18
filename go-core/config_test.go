//go:build lecore_tier2 || lecore_tier3 || lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

package core_test

import (
	"context"
	"io/ioutil"
	"os"
	"strings"
	"testing"
	"time"

	. "low-entropy-core/go-core"
)

// =============================================================================
// Config Parsing Tests
// =============================================================================

// validJSON is a reusable valid JSON config string for tests.
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
	jsonStr := `{"id":"","name":"test","steps":[{"type":"atom","name":"s1","params":{}}]}`
	_, err := ParseConfig([]byte(jsonStr))
	if err == nil {
		t.Fatal("expected error for empty ID, got nil")
	}
	if !strings.Contains(err.Error(), "ID must not be empty") {
		t.Errorf("expected error to mention 'ID must not be empty', got: %v", err)
	}
}

func TestParseConfig_EmptySteps(t *testing.T) {
	jsonStr := `{"id":"pipeline-1","name":"test","steps":[]}`
	_, err := ParseConfig([]byte(jsonStr))
	if err == nil {
		t.Fatal("expected error for empty steps, got nil")
	}
	if !strings.Contains(err.Error(), "no steps defined") {
		t.Errorf("expected error to mention 'no steps defined', got: %v", err)
	}
}

func TestParseConfig_InvalidStepType(t *testing.T) {
	jsonStr := `{"id":"pipeline-1","name":"test","steps":[{"type":"invalid_type","name":"s1","params":{}}]}`
	_, err := ParseConfig([]byte(jsonStr))
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

func TestValidateConfig_MultipleErrors(t *testing.T) {
	// Construct a config with multiple issues: empty ID, invalid step type, empty step names
	config := &PipelineConfig{
		ID:    "",
		Name:  "bad config",
		Steps: []StepConfig{
			{Type: "unknown", Name: "", Params: nil},
			{Type: "atom", Name: "", Params: nil},
		},
	}

	errs := ValidateConfig(config)
	if len(errs) < 3 {
		t.Fatalf("expected at least 3 validation errors, got %d", len(errs))
	}

	// Build a list of error messages for checking
	errMessages := make([]string, len(errs))
	for i, e := range errs {
		errMessages[i] = e.Error()
	}

	// Check for expected errors
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

// =============================================================================
// PipelineBuilder Tests
// =============================================================================

func TestPipelineBuilder_Build(t *testing.T) {
	ctx := context.Background()
	resolver := NewMapAdapterResolver()

	// Register a simple atom: double the value
	doubleAtom := Atom[any, any](func(input any) any {
		val := input.(int)
		return val * 2
	})
	resolver.Register("double", "dev", doubleAtom)

	// Create a PipelineBuilder with no observation
	builder := NewPipelineBuilder(resolver, nil)

	// Build a pipeline from a config using the "double" atom
	jsonStr := `{"id":"test-pipeline","name":"double pipeline","steps":[{"type":"atom","name":"double","params":{}}]}`
	config, err := ParseConfig([]byte(jsonStr))
	if err != nil {
		t.Fatalf("failed to parse config: %v", err)
	}

	composer, err := builder.Build(config, "dev")
	if err != nil {
		t.Fatalf("failed to build pipeline: %v", err)
	}
	if composer == nil {
		t.Fatal("expected non-nil composer")
	}

	// Run the pipeline with input 21, expect 42
	result, steps, err := composer.Run(ctx, 21)
	if err != nil {
		t.Fatalf("unexpected error running pipeline: %v", err)
	}
	if result != 42 {
		t.Errorf("expected 42, got %v", result)
	}
	if len(steps) == 0 {
		t.Error("expected at least 1 execution step")
	}
}

func TestPipelineBuilder_UnknownType(t *testing.T) {
	resolver := NewMapAdapterResolver()

	// Register a dummy adapter that will be resolved but then fail type switch
	dummyAdapter := Adapter[any, any](NewAdapter[any, any](func(ctx context.Context, input any) (any, error) {
		return input, nil
	}))
	resolver.Register("unknown_step", "dev", dummyAdapter)

	builder := NewPipelineBuilder(resolver, nil)

	// Build a config with an unknown type (not in allowed list)
	config := &PipelineConfig{
		ID:   "bad-pipeline",
		Name: "bad config",
		Steps: []StepConfig{
			{Type: "not_a_real_type", Name: "unknown_step", Params: nil},
		},
	}

	_, err := builder.Build(config, "dev")
	if err == nil {
		t.Fatal("expected error for unknown step type, got nil")
	}
	if !strings.Contains(err.Error(), "unknown type") {
		t.Errorf("expected error to mention 'unknown type', got: %v", err)
	}
}

// =============================================================================
// MapAdapterResolver Tests
// =============================================================================

func TestMapAdapterResolver_Resolve(t *testing.T) {
	resolver := NewMapAdapterResolver()

	// Register a simple atom
	doubleAtom := Atom[any, any](func(input any) any {
		val := input.(int)
		return val * 2
	})
	resolver.Register("double", "dev", doubleAtom)

	// Resolve the registered adapter
	resolved, err := resolver.Resolve("double", "dev")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if resolved == nil {
		t.Fatal("expected non-nil resolved adapter")
	}

	// Verify the resolved adapter works correctly
	atom, ok := resolved.(Atom[any, any])
	if !ok {
		t.Fatalf("expected Atom[any,any], got %T", resolved)
	}
	result := atom(21)
	if result != 42 {
		t.Errorf("expected 42, got %v", result)
	}
}

func TestMapAdapterResolver_ResolveNotFound(t *testing.T) {
	resolver := NewMapAdapterResolver()

	// Try to resolve a non-existent adapter
	_, err := resolver.Resolve("nonexistent", "dev")
	if err == nil {
		t.Fatal("expected error for non-existent adapter, got nil")
	}
	if !strings.Contains(err.Error(), "no adapter registered") {
		t.Errorf("expected error to mention 'no adapter registered', got: %v", err)
	}
	if !strings.Contains(err.Error(), "nonexistent") {
		t.Errorf("expected error to mention the adapter name, got: %v", err)
	}
}

// =============================================================================
// HotReload Tests
// =============================================================================

func TestHotReload_Start(t *testing.T) {
	ctx := context.Background()

	// Create a temporary config file
	tmpFile, err := os.CreateTemp("", "config-*.json")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	configContent := `{"id":"test","steps":[{"type":"atom","name":"double","params":{}}]}`
	if _, err := tmpFile.WriteString(configContent); err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		t.Fatalf("failed to write config: %v", err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	// Register the double atom
	resolver := NewMapAdapterResolver()
	doubleAtom := Atom[any, any](func(input any) any {
		val := input.(int)
		return val * 2
	})
	resolver.Register("double", "dev", doubleAtom)

	// Create builder and hot reload
	builder := NewPipelineBuilder(resolver, nil)
	hotReload := NewHotReload(tmpFile.Name(), builder, "dev", nil)

	// Start hot reload with a short check interval
	composer, err := hotReload.Start(ctx, 100*time.Millisecond)
	if err != nil {
		t.Fatalf("failed to start hot reload: %v", err)
	}
	defer hotReload.Stop()

	if composer == nil {
		t.Fatal("expected non-nil initial composer")
	}

	// Run the pipeline and verify output
	result, _, err := composer.Run(ctx, 21)
	if err != nil {
		t.Fatalf("unexpected error running pipeline: %v", err)
	}
	if result != 42 {
		t.Errorf("expected 42, got %v", result)
	}
}

func TestHotReload_Current(t *testing.T) {
	ctx := context.Background()

	// Create a temporary config file
	tmpFile, err := os.CreateTemp("", "config-*.json")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	configContent := `{"id":"test","steps":[{"type":"atom","name":"double","params":{}}]}`
	if _, err := tmpFile.WriteString(configContent); err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		t.Fatalf("failed to write config: %v", err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	// Register the double atom
	resolver := NewMapAdapterResolver()
	doubleAtom := Atom[any, any](func(input any) any {
		val := input.(int)
		return val * 2
	})
	resolver.Register("double", "dev", doubleAtom)

	// Create builder and hot reload
	builder := NewPipelineBuilder(resolver, nil)
	hotReload := NewHotReload(tmpFile.Name(), builder, "dev", nil)

	// Start hot reload
	initial, err := hotReload.Start(ctx, 100*time.Millisecond)
	if err != nil {
		t.Fatalf("failed to start hot reload: %v", err)
	}
	defer hotReload.Stop()

	// Current should return the same composer
	current := hotReload.Current()
	if current == nil {
		t.Fatal("expected non-nil composer from Current()")
	}

	// Both should produce the same result
	result1, _, err := initial.Run(ctx, 10)
	if err != nil {
		t.Fatalf("unexpected error from initial: %v", err)
	}
	result2, _, err := current.Run(ctx, 10)
	if err != nil {
		t.Fatalf("unexpected error from current: %v", err)
	}
	if result1 != result2 {
		t.Errorf("expected same result from initial and current, got %v and %v", result1, result2)
	}
	if result1 != 20 {
		t.Errorf("expected 20, got %v", result1)
	}
}

func TestHotReload_Stop(t *testing.T) {
	ctx := context.Background()

	// Create a temporary config file using ioutil
	tmpFile, err := os.CreateTemp("", "config-*.json")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	configContent := `{"id":"test","steps":[{"type":"atom","name":"double","params":{}}]}`
	if err := ioutil.WriteFile(tmpFile.Name(), []byte(configContent), 0644); err != nil {
		os.Remove(tmpFile.Name())
		t.Fatalf("failed to write config: %v", err)
	}
	// tmpFile is already closed by WriteFile, but we need to close the original handle
	// since we are using ioutil.WriteFile, we can close the original handle
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	// Register the double atom
	resolver := NewMapAdapterResolver()
	doubleAtom := Atom[any, any](func(input any) any {
		val := input.(int)
		return val * 2
	})
	resolver.Register("double", "dev", doubleAtom)

	// Create builder and hot reload
	builder := NewPipelineBuilder(resolver, nil)
	hotReload := NewHotReload(tmpFile.Name(), builder, "dev", nil)

	// Start hot reload
	_, err = hotReload.Start(ctx, 100*time.Millisecond)
	if err != nil {
		t.Fatalf("failed to start hot reload: %v", err)
	}

	// Stop should not panic and should complete gracefully
	hotReload.Stop()

	// Verify we can call Stop again without panic (idempotent - but may hang since
	// done channel is already closed; the implementation reads from done channel
	// in Stop, so second call would proceed immediately since channel is closed)
	// We just verify no panic occurs
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Stop() panicked: %v", r)
			}
		}()
		hotReload.Stop()
	}()
}