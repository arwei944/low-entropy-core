package core

import "testing"

func TestValidateKnowledgeGraph(t *testing.T) {
	kg := makeTestGraph(5)
	warnings := ValidateKnowledgeGraph(kg)
	if len(warnings) > 0 {
		for _, w := range warnings {
			t.Logf("warning: %s", w)
		}
	}
}

func TestValidateKnowledgeGraph_Nil(t *testing.T) {
	warnings := ValidateKnowledgeGraph(nil)
	if len(warnings) == 0 {
		t.Error("expected warnings for nil graph")
	}
}

func TestValidateKnowledgeGraph_BadEdge(t *testing.T) {
	kg := makeTestGraph(3)
	kg.Edges = append(kg.Edges, GraphEdge{
		Source: "nonexistent-1",
		Target: "nonexistent-2",
		Type:   EdgeTypeCalls,
	})
	warnings := ValidateKnowledgeGraph(kg)
	found := false
	for _, w := range warnings {
		if contains(w, "not found in nodes") {
			found = true
		}
	}
	if !found {
		t.Error("expected warning about nonexistent node reference")
	}
}

func TestValidateKnowledgeGraph_BadEdgeType(t *testing.T) {
	kg := makeTestGraph(3)
	kg.Edges = append(kg.Edges, GraphEdge{
		Source: kg.Nodes[0].ID,
		Target: kg.Nodes[1].ID,
		Type:   "invalid_edge_type",
	})
	warnings := ValidateKnowledgeGraph(kg)
	found := false
	for _, w := range warnings {
		if contains(w, "unknown type") {
			found = true
		}
	}
	if !found {
		t.Error("expected warning about unknown edge type")
	}
}

func TestCountByType(t *testing.T) {
	kg := makeTestGraph(10)
	counts := CountByType(kg)
	total := 0
	for _, c := range counts {
		total += c
	}
	if total != len(kg.Nodes) {
		t.Errorf("total count: got %d, want %d", total, len(kg.Nodes))
	}
}

func TestCountByEdgeType(t *testing.T) {
	kg := makeTestGraph(5)
	counts := CountByEdgeType(kg)
	total := 0
	for _, c := range counts {
		total += c
	}
	if total != len(kg.Edges) {
		t.Errorf("total edge count: got %d, want %d", total, len(kg.Edges))
	}
}
