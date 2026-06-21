package core

import (
	"context"
	"testing"
)

func TestGraphObserver_ObserveStructure(t *testing.T) {
	kg := makeTestGraph(10)
	observer := NewGraphObserver(kg)
	obs := observer.ObserveStructure()

	if obs.NodeCount != len(kg.Nodes) {
		t.Errorf("node_count: got %d, want %d", obs.NodeCount, len(kg.Nodes))
	}
	if obs.EdgeCount != len(kg.Edges) {
		t.Errorf("edge_count: got %d, want %d", obs.EdgeCount, len(kg.Edges))
	}
	if obs.LayerCount != len(kg.Layers) {
		t.Errorf("layer_count: got %d, want %d", obs.LayerCount, len(kg.Layers))
	}
	if obs.GraphDensity <= 0 {
		t.Error("graph_density should be > 0")
	}
	if obs.Complexity.Simple+obs.Complexity.Moderate+obs.Complexity.Complex != len(kg.Nodes) {
		t.Error("complexity stats should sum to node count")
	}
}

func TestGraphObserver_Nil(t *testing.T) {
	observer := NewGraphObserver(nil)
	obs := observer.ObserveStructure()
	if obs.NodeCount != 0 {
		t.Error("expected 0 nodes for nil graph")
	}
}

func TestGraphObserver_ObserveChanges(t *testing.T) {
	before := makeTestGraph(5)
	after := makeTestGraph(5)
	after.Nodes[0].Name = "Modified"
	after.Nodes = append(after.Nodes, GraphNode{
		ID:   "node-new",
		Type: NodeTypeFunction,
		Name: "NewNode",
	})

	observer := NewGraphObserver(after)
	change := observer.ObserveChanges(before)

	if change.Diff == nil {
		t.Error("expected diff in change observation")
	}
	if change.Diff.Summary.NodesAdded == 0 {
		t.Error("expected nodes added")
	}
}

func TestGraphObserver_ObserveChanges_Nil(t *testing.T) {
	observer := NewGraphObserver(nil)
	change := observer.ObserveChanges(nil)
	if change.Diff != nil {
		t.Error("expected nil diff for nil graphs")
	}
}

func TestGraphObserver_SearchGraph(t *testing.T) {
	kg := makeTestGraph(20)
	kg.Nodes[5].Name = "SearchTarget"
	kg.Nodes[5].Summary = "This is a very specific search target for testing"
	kg.Nodes[5].Tags = []string{"searchable", "test"}

	observer := NewGraphObserver(kg)

	result := observer.SearchGraph(SearchQuery{Keyword: "SearchTarget", Limit: 5})
	if result.Total == 0 {
		t.Error("expected search results for 'SearchTarget'")
	}
	if len(result.Results) == 0 {
		t.Error("expected at least 1 result")
	}
	if result.Results[0].Score <= 0 {
		t.Error("expected positive score")
	}
}

func TestGraphObserver_SearchGraph_TagMatch(t *testing.T) {
	kg := makeTestGraph(10)
	kg.Nodes[3].Tags = []string{"unique-tag-for-testing"}
	observer := NewGraphObserver(kg)

	result := observer.SearchGraph(SearchQuery{Keyword: "unique-tag-for-testing", Limit: 5})
	if result.Total == 0 {
		t.Error("expected search results for tag match")
	}
}

func TestGraphObserver_SearchGraph_NoMatch(t *testing.T) {
	kg := makeTestGraph(5)
	observer := NewGraphObserver(kg)

	result := observer.SearchGraph(SearchQuery{Keyword: "99999999", Limit: 5})
	if result.Total != 0 {
		t.Errorf("expected 0 results, got %d", result.Total)
	}
}

func TestGraphObserver_SetGraph(t *testing.T) {
	kg1 := makeTestGraph(5)
	observer := NewGraphObserver(kg1)

	kg2 := makeTestGraph(10)
	observer.SetGraph(kg2)

	obs := observer.ObserveStructure()
	if obs.NodeCount != 10 {
		t.Errorf("expected 10 nodes after SetGraph, got %d", obs.NodeCount)
	}
}

func TestGraphIndex_Search(t *testing.T) {
	kg := makeTestGraph(20)
	index := NewGraphIndex(kg)

	result := index.Search(SearchQuery{Keyword: "Node", Limit: 10})
	if result.Total == 0 {
		t.Error("expected search results")
	}
	if len(result.Results) > 10 {
		t.Errorf("expected at most 10 results, got %d", len(result.Results))
	}
}

func TestGraphIndex_DefaultLimit(t *testing.T) {
	kg := makeTestGraph(20)
	index := NewGraphIndex(kg)

	result := index.Search(SearchQuery{Keyword: "Node", Limit: 0})
	if len(result.Results) > 10 {
		t.Errorf("expected default limit 10, got %d", len(result.Results))
	}
}

func TestTokenize(t *testing.T) {
	tokens := tokenize("Hello World Test")
	if len(tokens) == 0 {
		t.Error("expected tokens for 'Hello World Test'")
	}

	tokens = tokenize("搜索测试")
	if len(tokens) == 0 {
		t.Error("expected tokens for Chinese text")
	}
}

func TestNewObserverStructureStep(t *testing.T) {
	kg := makeTestGraph(5)
	observer := NewGraphObserver(kg)
	step := NewObserverStructureStep(observer)

	ctx := context.Background()
	obs, err := step.Execute(ctx, StructureObservationInput{})
	if err != nil {
		t.Fatalf("observer structure step: %v", err)
	}
	if obs.NodeCount != 5 {
		t.Errorf("expected 5 nodes, got %d", obs.NodeCount)
	}
}

func TestNewObserverSearchStep(t *testing.T) {
	kg := makeTestGraph(10)
	kg.Nodes[0].Name = "UniqueSearchTarget"
	observer := NewGraphObserver(kg)
	step := NewObserverSearchStep(observer)

	ctx := context.Background()
	result, err := step.Execute(ctx, SearchQuery{Keyword: "UniqueSearchTarget", Limit: 5})
	if err != nil {
		t.Fatalf("observer search step: %v", err)
	}
	if result.Total == 0 {
		t.Error("expected search results")
	}
}
