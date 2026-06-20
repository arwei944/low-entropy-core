//go:build lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

// Package core — 依赖图 (v4.0)
//
// 包含:
//   - DependencyGraph: Pipeline 依赖图 (DFS 循环检测, Kahn 拓扑排序, 冗余检测)
//   - DependencyEdge: 依赖边
//
// 所有类型均为线程安全。
package core

import (
	"sync"
)

// ============================================================================
// SECTION 1: DependencyGraph — 循环依赖检测
// ============================================================================

// DependencyGraph 表示 Pipeline 之间的依赖关系。
// 使用 DFS 染色算法检测循环依赖，O(V+E) 时间复杂度。
type DependencyGraph struct {
	mu    sync.RWMutex
	edges map[string]map[string]bool
	nodes map[string]bool
}

func NewDependencyGraph() *DependencyGraph {
	return &DependencyGraph{
		edges: make(map[string]map[string]bool),
		nodes: make(map[string]bool),
	}
}

func (g *DependencyGraph) AddEdge(from, to string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.edges[from] == nil {
		g.edges[from] = make(map[string]bool)
	}
	g.edges[from][to] = true
	g.nodes[from] = true
	g.nodes[to] = true
}

func (g *DependencyGraph) RemoveEdge(from, to string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.edges[from] != nil {
		delete(g.edges[from], to)
		if len(g.edges[from]) == 0 {
			delete(g.edges, from)
		}
	}
}

// DetectCycles 检测依赖图中的循环依赖 (DFS 三色标记).
func (g *DependencyGraph) DetectCycles() [][]string {
	g.mu.RLock()
	defer g.mu.RUnlock()

	const (
		white = 0
		gray  = 1
		black = 2
	)

	colors := make(map[string]int)
	var cycles [][]string
	var path []string

	var dfs func(node string) bool
	dfs = func(node string) bool {
		colors[node] = gray
		path = append(path, node)

		for neighbor := range g.edges[node] {
			switch colors[neighbor] {
			case gray:
				cycleStart := -1
				for i, n := range path {
					if n == neighbor {
						cycleStart = i
						break
					}
				}
				if cycleStart >= 0 {
					cycle := make([]string, len(path)-cycleStart)
					copy(cycle, path[cycleStart:])
					cycles = append(cycles, cycle)
				}
			case white:
				if dfs(neighbor) {
					return true
				}
			}
		}

		colors[node] = black
		path = path[:len(path)-1]
		return false
	}

	for node := range g.nodes {
		colors[node] = white
	}

	for node := range g.nodes {
		if colors[node] == white {
			dfs(node)
		}
	}

	return cycles
}

// TopologicalSort 拓扑排序 (Kahn 算法).
func (g *DependencyGraph) TopologicalSort() []string {
	g.mu.RLock()
	defer g.mu.RUnlock()

	inDegree := make(map[string]int)
	for node := range g.nodes {
		inDegree[node] = 0
	}
	for from := range g.edges {
		for to := range g.edges[from] {
			inDegree[to]++
		}
	}

	queue := make([]string, 0)
	for node := range g.nodes {
		if inDegree[node] == 0 {
			queue = append(queue, node)
		}
	}

	var result []string
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		result = append(result, node)

		for neighbor := range g.edges[node] {
			inDegree[neighbor]--
			if inDegree[neighbor] == 0 {
				queue = append(queue, neighbor)
			}
		}
	}

	if len(result) != len(g.nodes) {
		return nil
	}
	return result
}

// DetectRedundant 检测冗余依赖 (A→B→C 且 A→C 直接存在).
func (g *DependencyGraph) DetectRedundant() []DependencyEdge {
	g.mu.RLock()
	defer g.mu.RUnlock()

	var redundant []DependencyEdge
	reachable := g.transitiveClosure()

	for from := range g.edges {
		for to := range g.edges[from] {
			for intermediate := range g.nodes {
				if intermediate == from || intermediate == to {
					continue
				}
				if reachable[from][intermediate] && reachable[intermediate][to] {
					redundant = append(redundant, DependencyEdge{
						From:         from,
						To:           to,
						Intermediate: intermediate,
					})
				}
			}
		}
	}
	return redundant
}

func (g *DependencyGraph) transitiveClosure() map[string]map[string]bool {
	reachable := make(map[string]map[string]bool)
	for node := range g.nodes {
		reachable[node] = make(map[string]bool)
	}
	for from := range g.edges {
		for to := range g.edges[from] {
			reachable[from][to] = true
		}
	}
	for k := range g.nodes {
		for i := range g.nodes {
			for j := range g.nodes {
				if reachable[i][k] && reachable[k][j] {
					reachable[i][j] = true
				}
			}
		}
	}
	return reachable
}

// DependencyEdge 表示一条依赖边。
type DependencyEdge struct {
	From         string
	To           string
	Intermediate string
}

// DetectIslands 检测孤岛 (无入度且无出度的节点).
func (g *DependencyGraph) DetectIslands() []string {
	g.mu.RLock()
	defer g.mu.RUnlock()

	var islands []string
	for node := range g.nodes {
		hasIn := false
		hasOut := len(g.edges[node]) > 0

		for from := range g.edges {
			if g.edges[from][node] {
				hasIn = true
				break
			}
		}

		if !hasIn && !hasOut {
			islands = append(islands, node)
		}
	}
	return islands
}
