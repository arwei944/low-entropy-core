package core

import (
	"fmt"
)

// ──────────────────────────────────────────────
// SchemaMigration — versioned data migration chain
// ──────────────────────────────────────────────

// MigrationFunc is a function that transforms data from one version to another.
// It takes the source data and returns the migrated data.
type MigrationFunc func(source interface{}) (interface{}, error)

// MigrationChain is a chain of migration functions that transform data
// from one version to another. It automatically chains v1→v2→v3.
// Internally, migrations are stored as a directed graph of version transitions.
// The Migrate method uses BFS to find the shortest path through the graph.
type MigrationChain struct {
	migrations map[string]MigrationFunc // key: "v1->v2", "v2->v3"
}

// NewMigrationChain creates a new, empty migration chain.
func NewMigrationChain() *MigrationChain {
	return &MigrationChain{
		migrations: make(map[string]MigrationFunc),
	}
}

// Register adds a migration function for a specific version transition.
// fromVersion and toVersion are strings like "v1", "v2".
// The migration function transforms data from the fromVersion to the toVersion.
// If a migration for the same transition already exists, it is overwritten.
func (m *MigrationChain) Register(fromVersion, toVersion string, fn MigrationFunc) {
	key := fromVersion + "->" + toVersion
	m.migrations[key] = fn
}

// Migrate applies the migration chain to transform data from fromVersion to toVersion.
// It automatically chains migrations: v1→v2→v3 when asked for v1→v3.
// Uses BFS to find the shortest migration path through the registered transitions.
// Returns the migrated data or an error if any migration step fails or no path exists.
func (m *MigrationChain) Migrate(fromVersion, toVersion string, data interface{}) (interface{}, error) {
	if fromVersion == toVersion {
		return data, nil
	}

	// Build adjacency list: for each version, map of next versions with their migration functions
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

	// Find the shortest path from fromVersion to toVersion using BFS
	path, err := bfsPath(adj, fromVersion, toVersion)
	if err != nil {
		return nil, err
	}

	// Apply each migration step in sequence
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

// parseTransition splits a key like "v1->v2" into ("v1", "v2", true).
// Returns ("", "", false) if the key is not a valid transition string.
func parseTransition(key string) (from, to string, ok bool) {
	for i := 0; i < len(key)-1; i++ {
		if key[i:i+2] == "->" {
			return key[:i], key[i+2:], true
		}
	}
	return "", "", false
}

// bfsPath performs a breadth-first search on the adjacency graph to find
// the shortest path from start to end. Returns the path as a slice of
// version strings (including both start and end), or an error if no path exists.
func bfsPath(adj map[string]map[string]MigrationFunc, start, end string) ([]string, error) {
	// Each queue entry is a path (slice of version strings)
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

			// Build new path by appending next version
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