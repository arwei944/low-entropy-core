//go:build lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

// Package core — 架构合规检查 (v4.0)
//
// 包含:
//   - ArchitectureGuard: 架构合规检查 (Port)
//   - ArchitectureInput: 架构检查输入
//   - ArchitectureAlert: 架构检查输出
//
// 所有类型均为线程安全。
package core

import (
	"context"
	"fmt"
	"strings"
)

// ============================================================================
// SECTION 4: ArchitectureGuard — 架构合规检查 (Port)
// ============================================================================

// ArchitectureInput contains data for architecture compliance checking.
type ArchitectureInput struct {
	Pipelines       []PipelineDescriptor
	SchemaChanges   []SchemaChange
	NewDependencies []string
	ConfigChanges   []string
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
	"pipeline": true, "branch": true, "parallel": true, "timeout": true,
	"retry": true, "circuit_breaker": true, "fallback": true, "bulkhead": true,
	"rate_limit": true, "handoff": true, "scheduler": true, "match": true,
	"config_change": true, "resilience_chain": true,
}

// ArchitectureGuard is a Port that checks architecture compliance.
type ArchitectureGuard struct{}

func NewArchitectureGuard() *ArchitectureGuard {
	return &ArchitectureGuard{}
}

func (g *ArchitectureGuard) Validate(ctx context.Context, input ArchitectureInput) (ArchitectureAlert, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	select {
	case <-ctx.Done():
		return ArchitectureAlert{}, ctx.Err()
	default:
	}

	var alert ArchitectureAlert
	alert.NonPrimitiveTypes = checkNonPrimitiveTypes(input.Pipelines)
	alert.UncheckedSchemaChanges = checkUncheckedSchemaChanges(input.SchemaChanges)
	alert.NewDependencies = append([]string{}, input.NewDependencies...)
	alert.UnauthorizedConfigChanges = append([]string{}, input.ConfigChanges...)
	alert.Violations = len(alert.NonPrimitiveTypes) + len(alert.UncheckedSchemaChanges) +
		len(alert.NewDependencies) + len(alert.UnauthorizedConfigChanges)
	alert.Message = buildArchAlertMessage(alert)
	return alert, nil
}

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

func buildArchAlertMessage(alert ArchitectureAlert) string {
	if alert.Violations == 0 {
		return "architecture compliance check passed — no violations detected"
	}
	var parts []string
	if len(alert.NonPrimitiveTypes) > 0 {
		parts = append(parts, fmt.Sprintf("non-primitive types: %s", strings.Join(alert.NonPrimitiveTypes, "; ")))
	}
	if len(alert.UncheckedSchemaChanges) > 0 {
		parts = append(parts, fmt.Sprintf("unchecked schema changes: %s", strings.Join(alert.UncheckedSchemaChanges, "; ")))
	}
	if len(alert.NewDependencies) > 0 {
		parts = append(parts, fmt.Sprintf("new dependencies: %s", strings.Join(alert.NewDependencies, ", ")))
	}
	if len(alert.UnauthorizedConfigChanges) > 0 {
		parts = append(parts, fmt.Sprintf("unauthorized config changes: %s", strings.Join(alert.UnauthorizedConfigChanges, ", ")))
	}
	return fmt.Sprintf("architecture violations (%d total): %s", alert.Violations, strings.Join(parts, " | "))
}
