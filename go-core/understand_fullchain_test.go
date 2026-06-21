package core

import (
	"context"
	"encoding/json"
	"testing"
)

func TestFullChain_ParseToSupervise(t *testing.T) {
	kg := makeTestGraph(20)

	data, err := json.Marshal(kg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	parsed, err := ParseKnowledgeGraph(data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	warnings := ValidateKnowledgeGraph(parsed)
	if len(warnings) > 0 {
		t.Logf("validation warnings: %v", warnings)
	}

	supervisor := NewGraphSupervisor(nil)
	report := supervisor.ValidateAll(parsed)
	if len(report.Results) != 6 {
		t.Errorf("expected 6 constraint results, got %d", len(report.Results))
	}

	observer := NewGraphObserver(parsed)
	obs := observer.ObserveStructure()
	if obs.NodeCount != 20 {
		t.Errorf("expected 20 nodes, got %d", obs.NodeCount)
	}
}

func TestFullChain_MigrationFlow(t *testing.T) {
	before := makeTestGraph(10)
	after := makeTestGraph(10)

	after.Nodes[0].Name = "Modified"
	after.Nodes = append(after.Nodes, GraphNode{
		ID:   "new-node",
		Type: NodeTypeFunction,
		Name: "NewFunction",
	})

	diff := DiffKnowledgeGraphs(before, after)
	if diff.Summary.TotalChanges == 0 {
		t.Error("expected changes in diff")
	}

	drifts := DetectLayerDrifts(before, after)
	if len(drifts) == 0 {
		t.Error("expected layer drifts")
	}

	supervisor := NewGraphSupervisor(nil)
	report := supervisor.ValidateAll(after)
	if len(report.Results) != 6 {
		t.Errorf("expected 6 constraint results, got %d", len(report.Results))
	}

	observer := NewGraphObserver(after)
	change := observer.ObserveChanges(before)
	if change.Diff == nil {
		t.Error("expected diff in change observation")
	}
}

func TestFullChain_SearchAndObserve(t *testing.T) {
	kg := makeTestGraph(30)
	kg.Nodes[10].Name = "TargetComponent"
	kg.Nodes[10].Summary = "Critical component for the system"
	kg.Nodes[10].Tags = []string{"critical", "core"}

	observer := NewGraphObserver(kg)

	obs := observer.ObserveStructure()
	if obs.NodeCount != 30 {
		t.Errorf("expected 30 nodes, got %d", obs.NodeCount)
	}

	result := observer.SearchGraph(SearchQuery{Keyword: "TargetComponent", Limit: 5})
	if result.Total == 0 {
		t.Error("expected search results")
	}

	found := result.Results[0]
	if found.Node.Name != "TargetComponent" {
		t.Errorf("expected TargetComponent, got %s", found.Node.Name)
	}
}
