//go:build lecore_tier3 || lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

package core_test

import (
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
