//go:build lecore_tier3 || lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

package core_test

import (
	"context"
	"testing"

	. "low-entropy-core/go-core"
)

// ──────────────────────────────────────────────
// CompatibilityChecker Tests
// ──────────────────────────────────────────────

func TestCompatibilityChecker_Compatible(t *testing.T) {
	checker := NewCompatibilityChecker()
	ctx := context.Background()

	req := SchemaDiffRequest{
		TypeName:   "TestSchema",
		OldSchema:  TestSchemaV1{Name: "test", Age: 25},
		NewSchema:  TestSchemaV1{Name: "test", Age: 30},
		OldVersion: "v1",
		NewVersion: "v1",
	}

	result, err := checker.Validate(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error for identical schemas: %v", err)
	}
	if !result.Compatible {
		t.Error("expected identical schemas to be compatible")
	}
	if len(result.Changes) != 0 {
		t.Errorf("expected 0 changes, got %d: %v", len(result.Changes), result.Changes)
	}
}

func TestCompatibilityChecker_FieldAdded(t *testing.T) {
	checker := NewCompatibilityChecker()
	ctx := context.Background()

	req := SchemaDiffRequest{
		TypeName:   "TestSchema",
		OldSchema:  TestSchemaV1{},
		NewSchema:  TestSchemaV2Added{},
		OldVersion: "v1",
		NewVersion: "v2",
	}

	result, err := checker.Validate(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error for field addition: %v", err)
	}
	if !result.Compatible {
		t.Error("expected field addition to be compatible (non-breaking)")
	}
	if len(result.Changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(result.Changes))
	}
	change := result.Changes[0]
	if change.Kind != "added" {
		t.Errorf("expected change kind='added', got '%s'", change.Kind)
	}
	if change.Field != "Email" {
		t.Errorf("expected changed field='Email', got '%s'", change.Field)
	}
	if change.Breaking {
		t.Error("expected field addition to be non-breaking")
	}
}

func TestCompatibilityChecker_FieldRemoved(t *testing.T) {
	checker := NewCompatibilityChecker()
	ctx := context.Background()

	req := SchemaDiffRequest{
		TypeName:   "TestSchema",
		OldSchema:  TestSchemaV1{},
		NewSchema:  TestSchemaV2Removed{},
		OldVersion: "v1",
		NewVersion: "v2",
	}

	result, err := checker.Validate(ctx, req)
	if err == nil {
		t.Fatal("expected error for field removal (breaking change)")
	}
	if result.Compatible {
		t.Error("expected field removal to be incompatible (breaking)")
	}
	if len(result.Changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(result.Changes))
	}
	change := result.Changes[0]
	if change.Kind != "removed" {
		t.Errorf("expected change kind='removed', got '%s'", change.Kind)
	}
	if change.Field != "Age" {
		t.Errorf("expected changed field='Age', got '%s'", change.Field)
	}
	if !change.Breaking {
		t.Error("expected field removal to be breaking")
	}
}

func TestCompatibilityChecker_TypeChanged(t *testing.T) {
	checker := NewCompatibilityChecker()
	ctx := context.Background()

	req := SchemaDiffRequest{
		TypeName:   "TestSchema",
		OldSchema:  TestSchemaV1{},
		NewSchema:  TestSchemaV2TypeChanged{},
		OldVersion: "v1",
		NewVersion: "v2",
	}

	result, err := checker.Validate(ctx, req)
	if err == nil {
		t.Fatal("expected error for type change (breaking change)")
	}
	if result.Compatible {
		t.Error("expected type change to be incompatible (breaking)")
	}
	if len(result.Changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(result.Changes))
	}
	change := result.Changes[0]
	if change.Kind != "type_changed" {
		t.Errorf("expected change kind='type_changed', got '%s'", change.Kind)
	}
	if change.Field != "Age" {
		t.Errorf("expected changed field='Age', got '%s'", change.Field)
	}
	if !change.Breaking {
		t.Error("expected type change to be breaking")
	}
}
