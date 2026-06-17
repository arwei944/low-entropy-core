package core_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	. "low-entropy-core/go-core"
)

// ──────────────────────────────────────────────
// Test structs for compatibility checking
// ──────────────────────────────────────────────

type TestSchemaV1 struct {
	Name string
	Age  int
}

type TestSchemaV2Added struct {
	Name  string
	Age   int
	Email string // added field (non-breaking)
}

type TestSchemaV2Removed struct {
	Name string // Age removed (breaking)
}

type TestSchemaV2TypeChanged struct {
	Name string
	Age  float64 // type changed from int (breaking)
}

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
// SchemaRegistry Tests
// ──────────────────────────────────────────────

func TestSchemaRegistry_RegisterAndGet(t *testing.T) {
	reg := NewSchemaRegistry()

	schema := TestSchemaV1{Name: "Alice", Age: 30}
	reg.Register("User", "v1", schema)

	got, err := reg.Get("User", "v1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	user, ok := got.(TestSchemaV1)
	if !ok {
		t.Fatalf("expected TestSchemaV1, got %T", got)
	}
	if user.Name != "Alice" {
		t.Errorf("expected Name='Alice', got '%s'", user.Name)
	}
	if user.Age != 30 {
		t.Errorf("expected Age=30, got %d", user.Age)
	}
}

func TestSchemaRegistry_GetNonExistent(t *testing.T) {
	reg := NewSchemaRegistry()

	_, err := reg.Get("Unknown", "v1")
	if err == nil {
		t.Fatal("expected error for non-existent schema, got nil")
	}
}

func TestSchemaRegistry_ListVersions(t *testing.T) {
	reg := NewSchemaRegistry()

	reg.Register("User", "v1", TestSchemaV1{})
	reg.Register("User", "v2", TestSchemaV2Added{})
	reg.Register("User", "v3", TestSchemaV2Removed{})

	versions := reg.ListVersions("User")
	if len(versions) != 3 {
		t.Errorf("expected 3 versions, got %d: %v", len(versions), versions)
	}
}

func TestSchemaRegistry_ListTypes(t *testing.T) {
	reg := NewSchemaRegistry()

	reg.Register("User", "v1", TestSchemaV1{})
	reg.Register("Order", "v1", struct{ ID string }{ID: "1"})

	types := reg.ListTypes()
	if len(types) != 2 {
		t.Errorf("expected 2 types, got %d: %v", len(types), types)
	}
}

func TestSchemaRegistry_Count(t *testing.T) {
	reg := NewSchemaRegistry()

	reg.Register("User", "v1", TestSchemaV1{})
	reg.Register("User", "v2", TestSchemaV2Added{})
	reg.Register("Order", "v1", TestSchemaV1{})

	if count := reg.Count(); count != 3 {
		t.Errorf("expected Count=3, got %d", count)
	}
}

func TestSchemaRegistry_Concurrent(t *testing.T) {
	reg := NewSchemaRegistry()

	var wg sync.WaitGroup
	numGoroutines := 50
	numPerGoroutine := 20

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numPerGoroutine; j++ {
				typeName := "Type"
				version := "v1"
				reg.Register(typeName, version, TestSchemaV1{Name: "test", Age: id*numPerGoroutine + j})
			}
		}(i)
	}

	wg.Wait()

	// After concurrent registration, the registry should be functional
	_, err := reg.Get("Type", "v1")
	if err != nil {
		t.Fatalf("registry should be functional after concurrent writes: %v", err)
	}

	// Count should be at least 1 (last write wins for same key)
	if count := reg.Count(); count < 1 {
		t.Errorf("expected at least 1 entry, got %d", count)
	}

	// Also test concurrent reads
	var readWg sync.WaitGroup
	errCount := 0
	var mu sync.Mutex

	for i := 0; i < numGoroutines; i++ {
		readWg.Add(1)
		go func() {
			defer readWg.Done()
			_, err := reg.Get("Type", "v1")
			if err != nil {
				mu.Lock()
				errCount++
				mu.Unlock()
			}
		}()
	}

	readWg.Wait()
	if errCount > 0 {
		t.Errorf("unexpected errors during concurrent reads: %d", errCount)
	}
}

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