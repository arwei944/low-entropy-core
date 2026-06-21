//go:build lecore_tier3 || lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

// Package core — Schema 兼容性检查 (v4.0)
package core

import (
	"context"
	"fmt"
	"reflect"
	"strings"
)

// SchemaDiffRequest is the input to the compatibility checker.
type SchemaDiffRequest struct {
	TypeName   string
	OldSchema  any
	NewSchema  any
	OldVersion string
	NewVersion string
}

// SchemaDiffResult is the output of the compatibility checker.
type SchemaDiffResult struct {
	Compatible bool
	Changes    []SchemaChange
	Summary    string
}

// SchemaChange describes a single change between versions.
type SchemaChange struct {
	Field    string `json:"field"`
	Kind     string `json:"kind"`
	Detail   string `json:"detail"`
	Breaking bool   `json:"breaking"`
}

// CompatibilityChecker is a Port that compares two schema versions
// and determines if they are compatible.
type CompatibilityChecker struct{}

func NewCompatibilityChecker() *CompatibilityChecker {
	return &CompatibilityChecker{}
}

func (c *CompatibilityChecker) Validate(ctx context.Context, input SchemaDiffRequest) (SchemaDiffResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	select {
	case <-ctx.Done():
		return SchemaDiffResult{}, ctx.Err()
	default:
	}

	oldType := reflect.TypeOf(input.OldSchema)
	newType := reflect.TypeOf(input.NewSchema)

	if oldType == nil || oldType.Kind() != reflect.Struct {
		return SchemaDiffResult{},
			NewStepError("SCHEMA_INCOMPATIBLE",
				fmt.Sprintf("OldSchema for type %q must be a non-nil struct, got %v", input.TypeName, oldType), false)
	}
	if newType == nil || newType.Kind() != reflect.Struct {
		return SchemaDiffResult{},
			NewStepError("SCHEMA_INCOMPATIBLE",
				fmt.Sprintf("NewSchema for type %q must be a non-nil struct, got %v", input.TypeName, newType), false)
	}

	oldFields := collectFields(oldType)
	newFields := collectFields(newType)

	var changes []SchemaChange
	breaking := false

	// Detect removed fields
	for name, oldFT := range oldFields {
		if _, ok := newFields[name]; !ok {
			changes = append(changes, SchemaChange{
				Field:    name,
				Kind:     "removed",
				Detail:   fmt.Sprintf("field %q of type %v was removed in version %s", name, oldFT, input.NewVersion),
				Breaking: true,
			})
			breaking = true
		}
	}

	// Detect added fields
	for name, newFT := range newFields {
		if _, ok := oldFields[name]; !ok {
			changes = append(changes, SchemaChange{
				Field:    name,
				Kind:     "added",
				Detail:   fmt.Sprintf("field %q of type %v was added in version %s", name, newFT, input.NewVersion),
				Breaking: false,
			})
		}
	}

	// Detect type changes
	for name, newFT := range newFields {
		oldFT, ok := oldFields[name]
		if !ok {
			continue
		}
		if oldFT != newFT {
			changes = append(changes, SchemaChange{
				Field:    name,
				Kind:     "type_changed",
				Detail:   fmt.Sprintf("field %q changed type from %v to %v in version %s", name, oldFT, newFT, input.NewVersion),
				Breaking: true,
			})
			breaking = true
		}
	}

	summary := buildSummary(input.TypeName, input.OldVersion, input.NewVersion, breaking, changes)

	result := SchemaDiffResult{
		Compatible: !breaking,
		Changes:    changes,
		Summary:    summary,
	}

	if !result.Compatible {
		return result, NewStepError("SCHEMA_INCOMPATIBLE", summary, false)
	}

	return result, nil
}

func CompatibilityCheckerAsStep(c *CompatibilityChecker) Step[SchemaDiffRequest, SchemaDiffResult] {
	return PortAsStep[SchemaDiffRequest, SchemaDiffResult](c)
}

func collectFields(t reflect.Type) map[string]reflect.Type {
	fields := make(map[string]reflect.Type)
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.PkgPath != "" {
			continue
		}
		fields[f.Name] = f.Type
	}
	return fields
}

func buildSummary(typeName, oldVer, newVer string, breaking bool, changes []SchemaChange) string {
	if len(changes) == 0 {
		return fmt.Sprintf("%s: %s -> %s: no changes detected", typeName, oldVer, newVer)
	}
	if breaking {
		var names []string
		for _, ch := range changes {
			if ch.Breaking {
				names = append(names, ch.Field)
			}
		}
		return fmt.Sprintf("%s: %s -> %s: INCOMPATIBLE — breaking changes on field(s): %s",
			typeName, oldVer, newVer, strings.Join(names, ", "))
	}
	return fmt.Sprintf("%s: %s -> %s: compatible — %d non-breaking change(s)",
		typeName, oldVer, newVer, len(changes))
}
