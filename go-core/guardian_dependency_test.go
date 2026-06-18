//go:build lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

package core

import (
	"testing"
)

func TestDependencyGraph_AddEdge(t *testing.T) {
	g := NewDependencyGraph()
	g.AddEdge("A", "B")
	g.AddEdge("B", "C")

	// 验证无环
	cycles := g.DetectCycles()
	if len(cycles) != 0 {
		t.Errorf("expected no cycles, got %v", cycles)
	}
}

func TestDependencyGraph_NoCycle(t *testing.T) {
	g := NewDependencyGraph()
	g.AddEdge("A", "B")
	g.AddEdge("B", "C")

	cycles := g.DetectCycles()
	if len(cycles) != 0 {
		t.Errorf("expected no cycles, got %v", cycles)
	}
}

func TestDependencyGraph_HasCycle(t *testing.T) {
	g := NewDependencyGraph()
	g.AddEdge("A", "B")
	g.AddEdge("B", "C")
	g.AddEdge("C", "A")

	cycles := g.DetectCycles()
	if len(cycles) == 0 {
		t.Error("expected cycle detected")
	}
}

func TestDependencyGraph_RemoveEdge(t *testing.T) {
	g := NewDependencyGraph()
	g.AddEdge("A", "B")
	g.AddEdge("B", "C")
	g.RemoveEdge("A", "B")

	// 检测环：A->B 被移除，应该是无环的
	cycles := g.DetectCycles()
	if len(cycles) != 0 {
		t.Errorf("expected no cycles after removing edge, got %v", cycles)
	}
}

func TestDependencyGraph_TopologicalSort(t *testing.T) {
	g := NewDependencyGraph()
	g.AddEdge("A", "B")
	g.AddEdge("B", "C")
	g.AddEdge("A", "C")

	sorted := g.TopologicalSort()
	if len(sorted) != 3 {
		t.Errorf("expected 3 nodes, got %d", len(sorted))
	}
	// A must come before B and C
	aIdx := indexOf(sorted, "A")
	bIdx := indexOf(sorted, "B")
	cIdx := indexOf(sorted, "C")
	if aIdx > bIdx || aIdx > cIdx {
		t.Error("A should come before B and C")
	}
	if bIdx > cIdx {
		t.Error("B should come before C")
	}
}

func TestDependencyGraph_TopologicalSortCycle(t *testing.T) {
	g := NewDependencyGraph()
	g.AddEdge("A", "B")
	g.AddEdge("B", "A")

	sorted := g.TopologicalSort()
	// 有环时返回空
	if len(sorted) != 0 {
		t.Errorf("expected empty for cycle, got %v", sorted)
	}
}

func indexOf(slice []string, item string) int {
	for i, s := range slice {
		if s == item {
			return i
		}
	}
	return -1
}

func TestDependencyGuard_Analyze(t *testing.T) {
	guard := NewDependencyGuard()

	guard.AddPipelineDependency("A", "B")
	guard.AddPipelineDependency("B", "C")

	violations := guard.Analyze()
	// 无环，应无违规
	if len(violations) != 0 {
		t.Errorf("expected 0 violations, got %d", len(violations))
	}
}

func TestDependencyGuard_CycleViolation(t *testing.T) {
	guard := NewDependencyGuard()

	guard.AddPipelineDependency("A", "B")
	guard.AddPipelineDependency("B", "A")

	violations := guard.Analyze()
	if len(violations) == 0 {
		t.Error("expected cycle violations")
	}
}

func TestDependencyGuard_Empty(t *testing.T) {
	guard := NewDependencyGuard()
	violations := guard.Analyze()
	if len(violations) != 0 {
		t.Errorf("expected 0 violations for empty, got %d", len(violations))
	}
}