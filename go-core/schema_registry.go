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

func (r *SchemaRegistry) Register(typeName string, version string, schema interface{}) {
	key := buildSchemaKey(typeName, version)
	r.entries.Store(key, schema)
}

func (r *SchemaRegistry) Get(typeName string, version string) (interface{}, error) {
	key := buildSchemaKey(typeName, version)
	val, ok := r.entries.Load(key)
	if !ok {
		return nil, fmt.Errorf("schema not found: type=%s version=%s", typeName, version)
	}
	return val, nil
}

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

func (r *SchemaRegistry) ListTypes() []string {
	typeSet := make(map[string]struct{})

	r.entries.Range(func(key, value interface{}) bool {
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

func (r *SchemaRegistry) Count() int {
	count := 0
	r.entries.Range(func(key, value interface{}) bool {
		count++
		return true
	})
	return count
}

func buildSchemaKey(typeName string, version string) string {
	return typeName + ":" + version
}
