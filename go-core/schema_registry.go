package core

import (
	"fmt"
	"strings"
	"sync"
)

// ──────────────────────────────────────────────
// SchemaRegistry — thread-safe schema registration
// ──────────────────────────────────────────────

// SchemaRegistry is a thread-safe registry for managing schemas
// across different types and versions. It uses sync.Map internally
// to support concurrent reads and writes without external locking.
//
// Key format: "typeName:version"
type SchemaRegistry struct {
	entries sync.Map
}

// NewSchemaRegistry creates a new, empty SchemaRegistry.
func NewSchemaRegistry() *SchemaRegistry {
	return &SchemaRegistry{}
}

// Register stores a schema under the given typeName and version.
// It is safe for concurrent use.
func (r *SchemaRegistry) Register(typeName string, version string, schema interface{}) {
	key := buildKey(typeName, version)
	r.entries.Store(key, schema)
}

// Get retrieves a schema by typeName and version.
// Returns an error if no schema is registered under the given name and version.
func (r *SchemaRegistry) Get(typeName string, version string) (interface{}, error) {
	key := buildKey(typeName, version)
	val, ok := r.entries.Load(key)
	if !ok {
		return nil, fmt.Errorf("schema not found: type=%s version=%s", typeName, version)
	}
	return val, nil
}

// ListVersions returns all registered versions for the given typeName.
// The returned slice is sorted in no particular order.
func (r *SchemaRegistry) ListVersions(typeName string) []string {
	prefix := typeName + ":"
	var versions []string

	r.entries.Range(func(key, value interface{}) bool {
		k, ok := key.(string)
		if !ok {
			return true
		}
		if strings.HasPrefix(k, prefix) {
			version := strings.TrimPrefix(k, prefix)
			versions = append(versions, version)
		}
		return true
	})

	return versions
}

// ListTypes returns all unique registered type names.
// The returned slice is sorted in no particular order.
func (r *SchemaRegistry) ListTypes() []string {
	typeSet := make(map[string]struct{})

	r.entries.Range(func(key, value interface{}) bool {
		k, ok := key.(string)
		if !ok {
			return true
		}
		// Extract typeName from "typeName:version"
		if idx := strings.LastIndex(k, ":"); idx > 0 {
			typeSet[k[:idx]] = struct{}{}
		}
		return true
	})

	types := make([]string, 0, len(typeSet))
	for t := range typeSet {
		types = append(types, t)
	}
	return types
}

// Count returns the total number of registered schemas.
func (r *SchemaRegistry) Count() int {
	count := 0
	r.entries.Range(func(key, value interface{}) bool {
		count++
		return true
	})
	return count
}

// buildKey constructs the internal key from a type name and version.
func buildKey(typeName string, version string) string {
	return typeName + ":" + version
}