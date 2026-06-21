package core

import (
	"encoding/json"
	"testing"
)

func TestEdgeCases_EmptyNodes(t *testing.T) {
	kg := &KnowledgeGraph{
		Version: "1.0",
		Project: ProjectMeta{Name: "empty"},
		Nodes:   []GraphNode{},
		Edges:   []GraphEdge{},
	}

	_ = ValidateKnowledgeGraph(kg)
	_ = CountByType(kg)
	_ = CountByEdgeType(kg)

	observer := NewGraphObserver(kg)
	obs := observer.ObserveStructure()
	if obs.NodeCount != 0 {
		t.Error("expected 0 nodes")
	}

	supervisor := NewGraphSupervisor(nil)
	report := supervisor.ValidateAll(kg)
	if report.PassCount != 6 {
		t.Errorf("expected 6 passes, got %d", report.PassCount)
	}
}

func TestEdgeCases_LargeIDs(t *testing.T) {
	longID := "very-long-node-id-that-exceeds-normal-length-" + makeString(100)
	kg := &KnowledgeGraph{
		Version: "1.0",
		Project: ProjectMeta{Name: "long-id"},
		Nodes: []GraphNode{
			{ID: longID, Type: NodeTypeFunction, Name: "LongIDNode"},
			{ID: "node-2", Type: NodeTypeFile, Name: "File2"},
		},
		Edges: []GraphEdge{
			{Source: longID, Target: "node-2", Type: EdgeTypeCalls, Weight: 0.5},
		},
	}

	warnings := ValidateKnowledgeGraph(kg)
	if len(warnings) > 0 {
		t.Logf("warnings: %v", warnings)
	}
}

func TestEdgeCases_DuplicateNodes(t *testing.T) {
	kg := &KnowledgeGraph{
		Version: "1.0",
		Project: ProjectMeta{Name: "duplicate"},
		Nodes: []GraphNode{
			{ID: "dup-1", Type: NodeTypeFunction, Name: "Func1"},
			{ID: "dup-1", Type: NodeTypeClass, Name: "Class1"},
		},
		Edges: []GraphEdge{
			{Source: "dup-1", Target: "dup-1", Type: EdgeTypeCalls, Weight: 0.5},
		},
	}

	diff := DiffKnowledgeGraphs(kg, kg)
	if diff.Summary.TotalChanges != 0 {
		t.Error("expected 0 changes for identical graphs")
	}
}

func TestDeterministic_ParseKnowledgeGraph(t *testing.T) {
	kg := makeTestGraph(10)
	data, _ := json.Marshal(kg)

	for i := 0; i < 10; i++ {
		parsed, err := ParseKnowledgeGraph(data)
		if err != nil {
			t.Fatalf("iteration %d: %v", i, err)
		}
		if len(parsed.Nodes) != len(kg.Nodes) {
			t.Errorf("iteration %d: node count mismatch", i)
		}
	}
}

func TestDeterministic_DiffKnowledgeGraphs(t *testing.T) {
	before := makeTestGraph(20)
	after := makeTestGraph(20)
	after.Nodes[5].Name = "Changed"

	var firstSummary DiffSummary
	for i := 0; i < 10; i++ {
		diff := DiffKnowledgeGraphs(before, after)
		if i == 0 {
			firstSummary = diff.Summary
		} else if diff.Summary != firstSummary {
			t.Errorf("iteration %d: non-deterministic diff summary", i)
		}
	}
}

func TestDeterministic_ValidateAll(t *testing.T) {
	kg := makeTestGraph(10)
	supervisor := NewGraphSupervisor(nil)

	var firstSummary string
	for i := 0; i < 10; i++ {
		report := supervisor.ValidateAll(kg)
		if i == 0 {
			firstSummary = report.Summary
		} else if report.Summary != firstSummary {
			t.Errorf("iteration %d: non-deterministic supervisor summary", i)
		}
	}
}
