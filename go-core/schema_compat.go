package core

import (
	"context"
	"fmt"
	"reflect"
	"strings"
)

// ──────────────────────────────────────────────
// Schema Compatibility Types
// ──────────────────────────────────────────────

// SchemaDiffRequest is the input to the compatibility checker.
type SchemaDiffRequest struct {
	TypeName   string      // the type name
	OldSchema  interface{} // the old schema version
	NewSchema  interface{} // the new schema version
	OldVersion string      // old version string
	NewVersion string      // new version string
}

// SchemaDiffResult is the output of the compatibility checker.
type SchemaDiffResult struct {
	Compatible bool           // whether the two schemas are compatible
	Changes    []SchemaChange // list of changes detected
	Summary    string         // human-readable summary
}

// SchemaChange describes a single change between versions.
type SchemaChange struct {
	Field    string `json:"field"`    // field name
	Kind     string `json:"kind"`     // "added", "removed", "type_changed"
	Detail   string `json:"detail"`   // description of the change
	Breaking bool   `json:"breaking"` // whether this change is breaking
}

// ──────────────────────────────────────────────
// CompatibilityChecker — Port for schema diffing
// ──────────────────────────────────────────────

// CompatibilityChecker is a Port that compares two schema versions
// and determines if they are compatible. It uses reflection to analyze
// struct fields and detect breaking changes.
type CompatibilityChecker struct{}

// NewCompatibilityChecker creates a new CompatibilityChecker.
func NewCompatibilityChecker() *CompatibilityChecker {
	return &CompatibilityChecker{}
}

// Validate implements the Port[SchemaDiffRequest, SchemaDiffResult] interface.
// It uses reflect to compare the old and new schema structs field-by-field.
//
// Compatibility rules:
//   - Forward compatible: v1 -> v2 adds new fields that were not in v1 (non-breaking)
//   - Breaking: v1 -> v2 removes a field that existed in v1
//   - Breaking: v1 -> v2 changes a field's type
func (c *CompatibilityChecker) Validate(ctx context.Context, input SchemaDiffRequest) (SchemaDiffResult, error) {
	// Ensure ctx is non-nil
	if ctx == nil {
		ctx = context.Background()
	}

	// Respect context cancellation
	select {
	case <-ctx.Done():
		return SchemaDiffResult{}, ctx.Err()
	default:
	}

	oldType := reflect.TypeOf(input.OldSchema)
	newType := reflect.TypeOf(input.NewSchema)

	// Both schema values must be non-nil structs
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

	// Build field maps: exported field name -> reflect.Type
	oldFields := collectFields(oldType)
	newFields := collectFields(newType)

	var changes []SchemaChange
	breaking := false

	// Detect removed fields: in old but not in new
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

	// Detect added fields: in new but not in old
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

	// Detect type changes: same field name, different type
	for name, newFT := range newFields {
		oldFT, ok := oldFields[name]
		if !ok {
			continue // already captured as "added"
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

	// Build human-readable summary
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

// CompatibilityCheckerAsStep wraps the CompatibilityChecker as a Step.
func CompatibilityCheckerAsStep(c *CompatibilityChecker) Step[SchemaDiffRequest, SchemaDiffResult] {
	return PortAsStep[SchemaDiffRequest, SchemaDiffResult](c)
}

// ──────────────────────────────────────────────
// Internal helpers
// ──────────────────────────────────────────────

// collectFields returns a map of exported field name to reflect.Type
// for the given struct type.
func collectFields(t reflect.Type) map[string]reflect.Type {
	fields := make(map[string]reflect.Type)
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		// Skip unexported fields
		if f.PkgPath != "" {
			continue
		}
		fields[f.Name] = f.Type
	}
	return fields
}

// buildSummary produces a human-readable summary of the compatibility check.
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