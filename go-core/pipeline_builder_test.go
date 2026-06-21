//go:build lecore_tier2 || lecore_tier3 || lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

package core_test

import (
	"context"
	"strings"
	"testing"

	. "low-entropy-core/go-core"
)

func TestPipelineBuilder_Build(t *testing.T) {
	ctx := context.Background()
	resolver := NewMapAdapterResolver()

	// Register a simple atom: double the value
	doubleAtom := Atom[any, any](func(input any) any {
		val := input.(int)
		return val * 2
	})
	resolver.Register("double", "dev", doubleAtom)

	builder := NewPipelineBuilder(resolver, nil)

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

	dummyAdapter := Adapter[any, any](NewAdapter[any, any](func(ctx context.Context, input any) (any, error) {
		return input, nil
	}))
	resolver.Register("unknown_step", "dev", dummyAdapter)

	builder := NewPipelineBuilder(resolver, nil)

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

func TestMapAdapterResolver_Resolve(t *testing.T) {
	resolver := NewMapAdapterResolver()

	doubleAtom := Atom[any, any](func(input any) any {
		val := input.(int)
		return val * 2
	})
	resolver.Register("double", "dev", doubleAtom)

	resolved, err := resolver.Resolve("double", "dev")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if resolved == nil {
		t.Fatal("expected non-nil resolved adapter")
	}

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
