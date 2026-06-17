package core

import (
	"context"
	"fmt"
	"strings"
)

// ArchitectureInput contains data for architecture compliance checking.
type ArchitectureInput struct {
	// Pipelines is the list of registered pipeline descriptors.
	Pipelines []PipelineDescriptor
	// SchemaChanges is the list of schema changes since last check.
	SchemaChanges []SchemaChange
	// NewDependencies is the list of newly introduced dependencies.
	NewDependencies []string
	// ConfigChanges is the list of pipeline config changes since last check.
	ConfigChanges []string
}

// ArchitectureAlert is the output of the architecture guard.
type ArchitectureAlert struct {
	Violations                int
	NonPrimitiveTypes         []string
	UncheckedSchemaChanges    []string
	NewDependencies           []string
	UnauthorizedConfigChanges []string
	Message                   string
}

// allowedPatterns is the set of patterns that conform to the 4-primitive model.
var allowedPatterns = map[string]bool{
	"pipeline":         true,
	"branch":           true,
	"parallel":         true,
	"timeout":          true,
	"retry":            true,
	"circuit_breaker":  true,
	"fallback":         true,
	"bulkhead":         true,
	"rate_limit":       true,
	"handoff":          true,
	"scheduler":        true,
	"match":            true,
	"config_change":    true,
	"resilience_chain": true,
}

// ArchitectureGuard is a Port that checks architecture compliance.
// It validates that all registered pipelines conform to the 4-primitive model,
// that schema changes are checked for compatibility, and that new dependencies
// and config changes are authorized.
type ArchitectureGuard struct{}

// NewArchitectureGuard creates a new ArchitectureGuard.
func NewArchitectureGuard() *ArchitectureGuard {
	return &ArchitectureGuard{}
}

// Validate implements the Port[ArchitectureInput, ArchitectureAlert] interface.
// It performs the following checks:
//  1. Non-primitive type detection: any pipeline with a pattern not in the
//     allowed list is flagged.
//  2. Unchecked schema changes: any SchemaChange with Breaking=true is flagged.
//  3. New dependencies: all newly introduced dependencies are flagged.
//  4. Unauthorized config changes: all config changes are flagged.
//
// Every violation is collected into the ArchitectureAlert. Violations is the
// total count across all categories. Empty input yields Violations=0 and no panic.
func (g *ArchitectureGuard) Validate(ctx context.Context, input ArchitectureInput) (ArchitectureAlert, error) {
	// Ensure ctx is non-nil
	if ctx == nil {
		ctx = context.Background()
	}

	// Respect context cancellation
	select {
	case <-ctx.Done():
		return ArchitectureAlert{}, ctx.Err()
	default:
	}

	var alert ArchitectureAlert

	// 1. Check for non-primitive types
	nonPrimitive := checkNonPrimitiveTypes(input.Pipelines)
	alert.NonPrimitiveTypes = nonPrimitive

	// 2. Check for unchecked schema changes
	unchecked := checkUncheckedSchemaChanges(input.SchemaChanges)
	alert.UncheckedSchemaChanges = unchecked

	// 3. New dependencies are all flagged
	alert.NewDependencies = append([]string{}, input.NewDependencies...)

	// 4. Unauthorized config changes are all flagged
	alert.UnauthorizedConfigChanges = append([]string{}, input.ConfigChanges...)

	// 5. Total violations
	alert.Violations = len(alert.NonPrimitiveTypes) +
		len(alert.UncheckedSchemaChanges) +
		len(alert.NewDependencies) +
		len(alert.UnauthorizedConfigChanges)

	// 6. Build message
	alert.Message = buildAlertMessage(alert)

	return alert, nil
}

// checkNonPrimitiveTypes scans PipelineDescriptors for patterns not in the
// allowed list. Returns a list of pipeline names (with their offending patterns)
// that violate the 4-primitive model.
func checkNonPrimitiveTypes(pipelines []PipelineDescriptor) []string {
	var violations []string
	for _, p := range pipelines {
		for _, pattern := range p.Patterns {
			if !allowedPatterns[pattern] {
				violations = append(violations,
					fmt.Sprintf("pipeline %q uses non-primitive pattern %q", p.Name, pattern))
			}
		}
	}
	return violations
}

// checkUncheckedSchemaChanges scans SchemaChanges for breaking changes that
// have not been checked for compatibility. Returns a list of descriptions.
func checkUncheckedSchemaChanges(changes []SchemaChange) []string {
	var violations []string
	for _, ch := range changes {
		if ch.Breaking {
			violations = append(violations,
				fmt.Sprintf("field %q: %s — %s", ch.Field, ch.Kind, ch.Detail))
		}
	}
	return violations
}

// buildAlertMessage constructs a human-readable summary of all violations.
func buildAlertMessage(alert ArchitectureAlert) string {
	if alert.Violations == 0 {
		return "architecture compliance check passed — no violations detected"
	}

	var parts []string

	if len(alert.NonPrimitiveTypes) > 0 {
		parts = append(parts, fmt.Sprintf("non-primitive types: %s",
			strings.Join(alert.NonPrimitiveTypes, "; ")))
	}
	if len(alert.UncheckedSchemaChanges) > 0 {
		parts = append(parts, fmt.Sprintf("unchecked schema changes: %s",
			strings.Join(alert.UncheckedSchemaChanges, "; ")))
	}
	if len(alert.NewDependencies) > 0 {
		parts = append(parts, fmt.Sprintf("new dependencies: %s",
			strings.Join(alert.NewDependencies, ", ")))
	}
	if len(alert.UnauthorizedConfigChanges) > 0 {
		parts = append(parts, fmt.Sprintf("unauthorized config changes: %s",
			strings.Join(alert.UnauthorizedConfigChanges, ", ")))
	}

	return fmt.Sprintf("architecture violations (%d total): %s",
		alert.Violations, strings.Join(parts, " | "))
}