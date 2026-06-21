//go:build lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

package core

import (
	"context"
	"fmt"
	"testing"
)

// ──────────────────────────────────────────────
// ArchitectureRegistry Tests
// ──────────────────────────────────────────────

func TestArchitectureRegistry_Register(t *testing.T) {
	registry := NewArchitectureRegistry()
	desc := PipelineDescriptor{
		Name:        "test-pipeline",
		Description: "a test pipeline",
		StepCount:   3,
		Patterns:    []string{"Pipeline"},
	}
	registry.Register(desc)

	if registry.Count() != 1 {
		t.Errorf("expected 1 pipeline, got %d", registry.Count())
	}

	retrieved, ok := registry.Get("test-pipeline")
	if !ok {
		t.Fatal("expected pipeline to be found")
	}
	if retrieved.Description != "a test pipeline" {
		t.Errorf("expected 'a test pipeline', got '%s'", retrieved.Description)
	}
}

func TestArchitectureRegistry_RegisterContract(t *testing.T) {
	registry := NewArchitectureRegistry()
	contract := PortContract{
		Name:         "validate-task",
		InputType:    "Task",
		OutputType:   "Task",
		PipelineName: "task-pipeline",
		Validations: []ValidationRule{
			{Field: "ID", Rule: "required", Message: "ID is required"},
		},
	}
	registry.RegisterContract(contract)

	if registry.ContractCount() != 1 {
		t.Errorf("expected 1 contract, got %d", registry.ContractCount())
	}

	retrieved, ok := registry.GetContract("validate-task")
	if !ok {
		t.Fatal("expected contract to be found")
	}
	if retrieved.InputType != "Task" {
		t.Errorf("expected InputType='Task', got '%s'", retrieved.InputType)
	}
}

func TestArchitectureRegistry_List(t *testing.T) {
	registry := NewArchitectureRegistry()
	registry.Register(PipelineDescriptor{Name: "p1"})
	registry.Register(PipelineDescriptor{Name: "p2"})
	registry.Register(PipelineDescriptor{Name: "p3"})

	list := registry.List()
	if len(list) != 3 {
		t.Errorf("expected 3 pipelines, got %d", len(list))
	}
}

// ──────────────────────────────────────────────
// PortContract Tests
// ──────────────────────────────────────────────

func TestDescribePortContract(t *testing.T) {
	port := NewPort[int, string](func(ctx context.Context, input int) (string, error) {
		return fmt.Sprintf("%d", input), nil
	})
	contract := DescribePortContract("int-to-string", port, "test")

	if contract.Name != "int-to-string" {
		t.Errorf("expected Name='int-to-string', got '%s'", contract.Name)
	}
	if contract.InputType != "int" {
		t.Errorf("expected InputType='int', got '%s'", contract.InputType)
	}
	if contract.OutputType != "string" {
		t.Errorf("expected OutputType='string', got '%s'", contract.OutputType)
	}
}

func TestCheckContractCompliance(t *testing.T) {
	contract := PortContract{
		Name:      "validate",
		InputType: "struct { ID string }",
		Validations: []ValidationRule{
			{Field: "ID", Rule: "required", Message: "ID is required"},
		},
	}

	type testStruct struct {
		ID string
	}

	// Valid input
	result := CheckContractCompliance(contract, testStruct{ID: "abc"})
	if !result.Compliant {
		t.Errorf("expected compliant, got violations: %v", result.Violations)
	}

	// Invalid input
	result = CheckContractCompliance(contract, testStruct{ID: ""})
	if result.Compliant {
		t.Error("expected non-compliant for empty ID")
	}
}
