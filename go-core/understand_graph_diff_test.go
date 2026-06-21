package core

import "testing"

func TestDiffKnowledgeGraphs(t *testing.T) {
	before := makeTestGraph(5)
	after := makeTestGraph(5)

	after.Nodes[1].Name = "ModifiedNode"
	after.Nodes[1].Complexity = "complex"

	after.Nodes = append(after.Nodes, GraphNode{
		ID:   "node-new",
		Type: NodeTypeFunction,
		Name: "NewNode",
	})

	after.Nodes = append(after.Nodes[:0], after.Nodes[1:]...)

	diff := DiffKnowledgeGraphs(before, after)

	if diff.Summary.NodesAdded == 0 {
		t.Error("expected nodes_added > 0")
	}
	if diff.Summary.NodesRemoved == 0 {
		t.Error("expected nodes_removed > 0")
	}
	if diff.Summary.NodesModified == 0 {
		t.Error("expected nodes_modified > 0")
	}
	if diff.Summary.TotalChanges == 0 {
		t.Error("expected total_changes > 0")
	}
}

func TestDiffKnowledgeGraphs_Nil(t *testing.T) {
	diff := DiffKnowledgeGraphs(nil, nil)
	if diff.Summary.TotalChanges != 0 {
		t.Error("expected 0 changes for nil inputs")
	}

	kg := makeTestGraph(3)
	diff = DiffKnowledgeGraphs(nil, kg)
	if diff.Summary.TotalChanges != 0 {
		t.Error("expected 0 changes for nil before")
	}
}

func TestDiffKnowledgeGraphs_Deterministic(t *testing.T) {
	before := makeTestGraph(5)
	after := makeTestGraph(5)
	after.Nodes[0].Name = "Changed"

	diff1 := DiffKnowledgeGraphs(before, after)
	diff2 := DiffKnowledgeGraphs(before, after)

	if diff1.Summary != diff2.Summary {
		t.Error("DiffKnowledgeGraphs should be deterministic")
	}
}

func TestDetectLayerDrifts(t *testing.T) {
	before := makeTestGraph(10)
	after := makeTestGraph(10)

	after.Nodes = append(after.Nodes, GraphNode{
		ID:       "node-extra",
		Type:     NodeTypeFunction,
		Name:     "ExtraNode",
		FilePath: "layer/service/extra.go",
	})

	drifts := DetectLayerDrifts(before, after)
	if len(drifts) == 0 {
		t.Error("expected layer drifts")
	}
}
