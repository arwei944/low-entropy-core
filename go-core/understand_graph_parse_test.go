package core

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

func makeTestGraph(nodeCount int) *KnowledgeGraph {
	nodes := make([]GraphNode, 0, nodeCount)
	for i := 0; i < nodeCount; i++ {
		nodeTypes := []string{NodeTypeFile, NodeTypeFunction, NodeTypeClass, NodeTypeModule, NodeTypeService}
		complexities := []string{"simple", "moderate", "complex"}
		nodes = append(nodes, GraphNode{
			ID:         fmt.Sprintf("node-%d", i),
			Type:       nodeTypes[i%len(nodeTypes)],
			Name:       fmt.Sprintf("Node%d", i),
			FilePath:   fmt.Sprintf("layer/%s/node%d.go", nodeTypes[i%len(nodeTypes)], i),
			Summary:    fmt.Sprintf("Summary for node %d", i),
			Tags:       []string{nodeTypes[i%len(nodeTypes)], fmt.Sprintf("tag-%d", i%5)},
			Complexity: complexities[i%len(complexities)],
		})
	}

	edges := make([]GraphEdge, 0, nodeCount*2)
	for i := 0; i < nodeCount*2 && i+1 < nodeCount; i++ {
		edgeTypes := []string{EdgeTypeCalls, EdgeTypeDependsOn, EdgeTypeContains, EdgeTypeImports, EdgeTypeRelated}
		edges = append(edges, GraphEdge{
			Source:      fmt.Sprintf("node-%d", i%nodeCount),
			Target:      fmt.Sprintf("node-%d", (i+1)%nodeCount),
			Type:        edgeTypes[i%len(edgeTypes)],
			Direction:   "forward",
			Description: fmt.Sprintf("Edge %d", i),
			Weight:      0.5,
		})
	}

	layers := []GraphLayer{
		{ID: "L0", Name: "Base", NodeIDs: make([]string, 0)},
		{ID: "L1", Name: "Primitives", NodeIDs: make([]string, 0)},
		{ID: "L4", Name: "Supervisor", NodeIDs: make([]string, 0)},
	}
	for i := range nodes {
		layers[i%len(layers)].NodeIDs = append(layers[i%len(layers)].NodeIDs, nodes[i].ID)
	}

	return &KnowledgeGraph{
		Version: "1.0.0",
		Kind:    "codebase",
		Project: ProjectMeta{
			Name:        "test-project",
			Languages:   []string{"go"},
			Description: "Test project",
			AnalyzedAt:  time.Now().Format(time.RFC3339),
		},
		Nodes:  nodes,
		Edges:  edges,
		Layers: layers,
	}
}

func makeAtomGraph() *KnowledgeGraph {
	kg := makeTestGraph(10)
	kg.Nodes[0].Tags = append(kg.Nodes[0].Tags, "atom")
	kg.Nodes[1].Tags = append(kg.Nodes[1].Tags, "atom")
	kg.Nodes[2].Tags = append(kg.Nodes[2].Tags, "adapter")
	return kg
}

func TestParseKnowledgeGraph(t *testing.T) {
	kg := makeTestGraph(5)
	data, err := json.Marshal(kg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	parsed, err := ParseKnowledgeGraph(data)
	if err != nil {
		t.Fatalf("ParseKnowledgeGraph: %v", err)
	}

	if parsed.Version != kg.Version {
		t.Errorf("version: got %q, want %q", parsed.Version, kg.Version)
	}
	if parsed.Project.Name != kg.Project.Name {
		t.Errorf("project name: got %q, want %q", parsed.Project.Name, kg.Project.Name)
	}
	if len(parsed.Nodes) != len(kg.Nodes) {
		t.Errorf("nodes: got %d, want %d", len(parsed.Nodes), len(kg.Nodes))
	}
	if len(parsed.Edges) != len(kg.Edges) {
		t.Errorf("edges: got %d, want %d", len(parsed.Edges), len(kg.Edges))
	}
}

func TestParseKnowledgeGraph_Empty(t *testing.T) {
	_, err := ParseKnowledgeGraph([]byte{})
	if err == nil {
		t.Error("expected error for empty input")
	}
}

func TestParseKnowledgeGraph_Invalid(t *testing.T) {
	_, err := ParseKnowledgeGraph([]byte("{invalid"))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestParseKnowledgeGraph_MissingVersion(t *testing.T) {
	_, err := ParseKnowledgeGraph([]byte(`{"project": {"name": "test"}}`))
	if err == nil {
		t.Error("expected error for missing version")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func makeString(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = 'x'
	}
	return string(b)
}
