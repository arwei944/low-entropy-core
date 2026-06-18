//go:build lecore_tier3 || lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

// Package core — Schema 注册 + 迁移 + 兼容性检查 (v4.0)
//
// 合并自: schema_registry.go + schema_migration.go + schema_compat.go
//
// 包含:
//   - SchemaRegistry: 线程安全的多版本 schema 注册
//   - MigrationChain: 有向图版本迁移链 (BFS 最短路径)
//   - CompatibilityChecker: 结构体字段级兼容性检查 (Port)
//   - SchemaDiffRequest / SchemaDiffResult / SchemaChange: 兼容性相关类型
package core

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"sync"
)

// ============================================================================
// SECTION 1: SchemaRegistry — 线程安全 schema 注册
// ============================================================================

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

// ============================================================================
// SECTION 2: MigrationChain — 版本迁移链
// ============================================================================

// MigrationFunc is a function that transforms data from one version to another.
type MigrationFunc func(source interface{}) (interface{}, error)

// MigrationChain is a chain of migration functions that transform data
// from one version to another. Uses BFS to find the shortest path.
type MigrationChain struct {
	migrations map[string]MigrationFunc // key: "v1->v2"
}

func NewMigrationChain() *MigrationChain {
	return &MigrationChain{
		migrations: make(map[string]MigrationFunc),
	}
}

func (m *MigrationChain) Register(fromVersion, toVersion string, fn MigrationFunc) {
	key := fromVersion + "->" + toVersion
	m.migrations[key] = fn
}

func (m *MigrationChain) Migrate(fromVersion, toVersion string, data interface{}) (interface{}, error) {
	if fromVersion == toVersion {
		return data, nil
	}

	adj := make(map[string]map[string]MigrationFunc)
	for key, fn := range m.migrations {
		from, to, ok := parseTransition(key)
		if !ok {
			continue
		}
		if adj[from] == nil {
			adj[from] = make(map[string]MigrationFunc)
		}
		adj[from][to] = fn
	}

	path, err := bfsPath(adj, fromVersion, toVersion)
	if err != nil {
		return nil, err
	}

	current := data
	for i := 0; i < len(path)-1; i++ {
		from := path[i]
		to := path[i+1]
		fn := adj[from][to]
		var stepErr error
		current, stepErr = fn(current)
		if stepErr != nil {
			return nil, fmt.Errorf("migration %s->%s: %w", from, to, stepErr)
		}
	}

	return current, nil
}

func parseTransition(key string) (from, to string, ok bool) {
	for i := 0; i < len(key)-1; i++ {
		if key[i:i+2] == "->" {
			return key[:i], key[i+2:], true
		}
	}
	return "", "", false
}

func bfsPath(adj map[string]map[string]MigrationFunc, start, end string) ([]string, error) {
	type pathEntry struct {
		path []string
	}

	queue := []pathEntry{{path: []string{start}}}
	visited := map[string]bool{start: true}

	for len(queue) > 0 {
		entry := queue[0]
		queue = queue[1:]

		last := entry.path[len(entry.path)-1]
		neighbors := adj[last]

		for next := range neighbors {
			if visited[next] {
				continue
			}

			newPath := make([]string, len(entry.path)+1)
			copy(newPath, entry.path)
			newPath[len(entry.path)] = next

			if next == end {
				return newPath, nil
			}

			visited[next] = true
			queue = append(queue, pathEntry{path: newPath})
		}
	}

	return nil, fmt.Errorf("no migration path from %s to %s", start, end)
}

// ============================================================================
// SECTION 3: CompatibilityChecker — Schema 兼容性检查 (Port)
// ============================================================================

// SchemaDiffRequest is the input to the compatibility checker.
type SchemaDiffRequest struct {
	TypeName   string
	OldSchema  interface{}
	NewSchema  interface{}
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