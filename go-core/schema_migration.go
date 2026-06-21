//go:build lecore_tier3 || lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

// Package core — Schema 迁移链 (v4.0)
package core

import "fmt"

// MigrationFunc is a function that transforms data from one version to another.
type MigrationFunc func(source any) (any, error)

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

func (m *MigrationChain) Migrate(fromVersion, toVersion string, data any) (any, error) {
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
