//go:build lecore_tier3 || lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

// Package core — Schema 注册 (v4.0)
package core

import (
	"fmt"
	"strings"
	"sync"
)

// SchemaRegistry is a thread-safe registry for managing schemas
// across different types and versions. Key format: "typeName:version"
type SchemaRegistry struct {
	entries sync.Map
}

func NewSchemaRegistry() *SchemaRegistry {
	return &SchemaRegistry{}
}

func (r *SchemaRegistry) Register(typeName string, version string, schema any) {
	key := buildSchemaKey(typeName, version)
	r.entries.Store(key, schema)
}

func (r *SchemaRegistry) Get(typeName string, version string) (any, error) {
	key := buildSchemaKey(typeName, version)
	val, ok := r.entries.Load(key)
	if !ok {
		return nil, fmt.Errorf("schema not found: type=%s version=%s", typeName, version)
	}
	return val, nil
}

// SchemaRegistryT is a typed schema registry that preserves type safety.
// Use this instead of the untyped SchemaRegistry when possible.
type SchemaRegistryT[T any] struct {
	entries sync.Map
}

// NewSchemaRegistryT creates a typed schema registry.
func NewSchemaRegistryT[T any]() *SchemaRegistryT[T] {
	return &SchemaRegistryT[T]{}
}

// Register stores a schema for a given type name and version.
func (r *SchemaRegistryT[T]) Register(typeName string, version string, schema T) {
	key := buildSchemaKey(typeName, version)
	r.entries.Store(key, schema)
}

// Get retrieves a schema by type name and version. Returns (zero value, false) if not found.
func (r *SchemaRegistryT[T]) Get(typeName string, version string) (T, bool) {
	key := buildSchemaKey(typeName, version)
	val, ok := r.entries.Load(key)
	if !ok {
		var zero T
		return zero, false
	}
	schema, ok := val.(T)
	return schema, ok
}

// ListVersions lists all versions for a given type name.
func (r *SchemaRegistryT[T]) ListVersions(typeName string) []string {
	prefix := typeName + ":"
	var versions []string
	r.entries.Range(func(key, value any) bool {
		k, ok := key.(string)
		if !ok {
			return true
		}
		if strings.HasPrefix(k, prefix) {
			versions = append(versions, strings.TrimPrefix(k, prefix))
		}
		return true
	})
	return versions
}

// ListTypes lists all registered type names.
func (r *SchemaRegistryT[T]) ListTypes() []string {
	typeSet := make(map[string]struct{})
	r.entries.Range(func(key, value any) bool {
		k, ok := key.(string)
		if !ok {
			return true
		}
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
func (r *SchemaRegistryT[T]) Count() int {
	count := 0
	r.entries.Range(func(key, value any) bool {
		count++
		return true
	})
	return count
}

func buildSchemaKey(typeName string, version string) string {
	return typeName + ":" + version
}
