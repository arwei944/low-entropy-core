//go:build lecore_tier2 || lecore_tier3 || lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

package core

import (
	"fmt"
	"reflect"
	"strings"
)

// ──────────────────────────────────────────────
// PortContract — self-describing port contracts
// ──────────────────────────────────────────────

// PortContract describes the contract enforced by a Port.
// It captures the input type, output type, and validation rules
// so that the architecture registry can provide a complete
// static view of the system.
type PortContract struct {
	// Name is the unique identifier for this contract.
	Name string `json:"name"`

	// InputType is the Go type name of the input.
	InputType string `json:"input_type"`

	// OutputType is the Go type name of the output.
	OutputType string `json:"output_type"`

	// Validations are the rules enforced by the contract.
	Validations []ValidationRule `json:"validations"`

	// Description explains what this contract does.
	Description string `json:"description"`

	// PipelineName is the pipeline this contract belongs to.
	PipelineName string `json:"pipeline_name"`
}

// ValidationRule describes a single validation rule.
type ValidationRule struct {
	// Field is the field being validated.
	Field string `json:"field"`

	// Rule is the validation rule name (required, min, max, pattern, etc.).
	Rule string `json:"rule"`

	// Value is the rule parameter (e.g., "10" for min=10).
	Value string `json:"value"`

	// Message is the error message when validation fails.
	Message string `json:"message"`
}

// DescribePortContract extracts a PortContract from a Port instance using reflection.
// This enables automatic contract registration — agents don't need to manually
// describe contracts; the architecture does it for them.
func DescribePortContract[In, Out any](name string, port Port[In, Out], pipelineName string) PortContract {
	var inZero In
	var outZero Out

	contract := PortContract{
		Name:         name,
		InputType:    typeName(inZero),
		OutputType:   typeName(outZero),
		PipelineName: pipelineName,
		Description:  fmt.Sprintf("Port contract %s: %s → %s", name, typeName(inZero), typeName(outZero)),
	}

	return contract
}

// typeName returns a human-readable name for a Go type.
func typeName(v any) string {
	t := reflect.TypeOf(v)
	if t == nil {
		return "any"
	}
	return t.String()
}

// RegisterPortContract describes and registers a port contract.
// This is the recommended way to register contracts — it combines
// DescribePortContract and ArchitectureRegistry.RegisterContract.
func RegisterPortContract[In, Out any](registry *ArchitectureRegistry, name string, port Port[In, Out], pipelineName string) PortContract {
	contract := DescribePortContract(name, port, pipelineName)
	if registry != nil {
		registry.RegisterContract(contract)
	}
	return contract
}

// ──────────────────────────────────────────────
// Contract Compliance Checker
// ──────────────────────────────────────────────

// ContractComplianceResult is the result of a contract compliance check.
type ContractComplianceResult struct {
	// ContractName is the name of the contract being checked.
	ContractName string `json:"contract_name"`

	// Compliant indicates whether the contract is satisfied.
	Compliant bool `json:"compliant"`

	// Violations are the specific violations found.
	Violations []string `json:"violations,omitempty"`
}

// CheckContractCompliance verifies that a value satisfies all validation rules
// in a contract. This is a pure function — no side effects.
func CheckContractCompliance(contract PortContract, input any) ContractComplianceResult {
	result := ContractComplianceResult{
		ContractName: contract.Name,
		Compliant:    true,
	}

	v := reflect.ValueOf(input)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	for _, rule := range contract.Validations {
		field := v.FieldByName(rule.Field)
		if !field.IsValid() {
			result.Violations = append(result.Violations,
				fmt.Sprintf("field %s not found in %s", rule.Field, contract.InputType))
			result.Compliant = false
			continue
		}

		switch rule.Rule {
		case "required":
			if isZero(field) {
				result.Violations = append(result.Violations, rule.Message)
				result.Compliant = false
			}
		case "non_empty_string":
			if field.Kind() == reflect.String && strings.TrimSpace(field.String()) == "" {
				result.Violations = append(result.Violations, rule.Message)
				result.Compliant = false
			}
		}
	}

	return result
}

// isZero checks if a reflect.Value is the zero value for its type.
func isZero(v reflect.Value) bool {
	return !v.IsValid() || reflect.DeepEqual(v.Interface(), reflect.Zero(v.Type()).Interface())
}