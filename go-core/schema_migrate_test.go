//go:build lecore_tier3 || lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

package core_test

import (
	"errors"
	"testing"

	. "low-entropy-core/go-core"
)

// ──────────────────────────────────────────────
// Test structs for migration
// ──────────────────────────────────────────────

type MigrateV1 struct {
	Name string
	Age  int
}

type MigrateV2 struct {
	FullName string
	Age      int
}

type MigrateV3 struct {
	FullName string
	Age      int
	Email    string
}

// ──────────────────────────────────────────────
// SchemaMigration Tests
// ──────────────────────────────────────────────

func TestMigrationChain_SimpleMigration(t *testing.T) {
	chain := NewMigrationChain()

	// Register v1 -> v2: rename Name -> FullName
	chain.Register("v1", "v2", func(source interface{}) (interface{}, error) {
		v1, ok := source.(MigrateV1)
		if !ok {
			return nil, errors.New("invalid source type")
		}
		return MigrateV2{
			FullName: v1.Name,
			Age:      v1.Age,
		}, nil
	})

	data := MigrateV1{Name: "Alice", Age: 30}
	result, err := chain.Migrate("v1", "v2", data)
	if err != nil {
		t.Fatalf("unexpected migration error: %v", err)
	}

	v2, ok := result.(MigrateV2)
	if !ok {
		t.Fatalf("expected MigrateV2, got %T", result)
	}
	if v2.FullName != "Alice" {
		t.Errorf("expected FullName='Alice', got '%s'", v2.FullName)
	}
	if v2.Age != 30 {
		t.Errorf("expected Age=30, got %d", v2.Age)
	}
}

func TestMigrationChain_ChainMigration(t *testing.T) {
	chain := NewMigrationChain()

	// v1 -> v2: rename Name -> FullName
	chain.Register("v1", "v2", func(source interface{}) (interface{}, error) {
		v1, ok := source.(MigrateV1)
		if !ok {
			return nil, errors.New("invalid source type")
		}
		return MigrateV2{
			FullName: v1.Name,
			Age:      v1.Age,
		}, nil
	})

	// v2 -> v3: add Email field
	chain.Register("v2", "v3", func(source interface{}) (interface{}, error) {
		v2, ok := source.(MigrateV2)
		if !ok {
			return nil, errors.New("invalid source type")
		}
		return MigrateV3{
			FullName: v2.FullName,
			Age:      v2.Age,
			Email:    v2.FullName + "@example.com",
		}, nil
	})

	data := MigrateV1{Name: "Bob", Age: 25}
	result, err := chain.Migrate("v1", "v3", data)
	if err != nil {
		t.Fatalf("unexpected chain migration error: %v", err)
	}

	v3, ok := result.(MigrateV3)
	if !ok {
		t.Fatalf("expected MigrateV3, got %T", result)
	}
	if v3.FullName != "Bob" {
		t.Errorf("expected FullName='Bob', got '%s'", v3.FullName)
	}
	if v3.Age != 25 {
		t.Errorf("expected Age=25, got %d", v3.Age)
	}
	if v3.Email != "Bob@example.com" {
		t.Errorf("expected Email='Bob@example.com', got '%s'", v3.Email)
	}
}

func TestMigrationChain_NoPath(t *testing.T) {
	chain := NewMigrationChain()

	// Register only v1 -> v2
	chain.Register("v1", "v2", func(source interface{}) (interface{}, error) {
		return source, nil
	})

	// Try to migrate v1 -> v3 (no path exists)
	_, err := chain.Migrate("v1", "v3", MigrateV1{Name: "test", Age: 10})
	if err == nil {
		t.Fatal("expected error for missing migration path, got nil")
	}
}

func TestMigrationChain_SameVersion(t *testing.T) {
	chain := NewMigrationChain()

	data := MigrateV1{Name: "Charlie", Age: 40}
	result, err := chain.Migrate("v1", "v1", data)
	if err != nil {
		t.Fatalf("unexpected error for same-version migration: %v", err)
	}

	v1, ok := result.(MigrateV1)
	if !ok {
		t.Fatalf("expected MigrateV1, got %T", result)
	}
	if v1.Name != "Charlie" {
		t.Errorf("expected Name='Charlie', got '%s'", v1.Name)
	}
	if v1.Age != 40 {
		t.Errorf("expected Age=40, got %d", v1.Age)
	}
}

func TestMigrationChain_MigrationError(t *testing.T) {
	chain := NewMigrationChain()

	expectedErr := errors.New("migration failed: invalid data")

	chain.Register("v1", "v2", func(source interface{}) (interface{}, error) {
		return nil, expectedErr
	})

	data := MigrateV1{Name: "Dave", Age: 35}
	_, err := chain.Migrate("v1", "v2", data)
	if err == nil {
		t.Fatal("expected error from migration function, got nil")
	}
	// The error should wrap the original error
	if !errors.Is(err, expectedErr) {
		t.Errorf("expected error to wrap %v, got %v", expectedErr, err)
	}
}
