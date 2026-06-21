package core

import (
	"context"
	"testing"
)

func TestDefaultConstraints(t *testing.T) {
	rules := DefaultConstraints()
	if len(rules) != 6 {
		t.Errorf("expected 6 default constraints, got %d", len(rules))
	}

	expectedNames := []string{
		"C1: 单一包", "C2: 层级依赖", "C3: 原语纯度",
		"C4: Port-Adapter", "C5: Step 统一", "C6: 泛型优先",
	}
	for i, rule := range rules {
		if rule.Name != expectedNames[i] {
			t.Errorf("rule[%d]: got %q, want %q", i, rule.Name, expectedNames[i])
		}
		if rule.Check == nil {
			t.Errorf("rule[%d]: check function is nil", i)
		}
	}
}

func TestSupervisor_ValidateAll(t *testing.T) {
	kg := makeTestGraph(10)
	supervisor := NewGraphSupervisor(nil)
	report := supervisor.ValidateAll(kg)

	if len(report.Results) != 6 {
		t.Errorf("expected 6 results, got %d", len(report.Results))
	}
	if report.PassCount+report.WarnCount+report.FailCount != 6 {
		t.Error("pass + warn + fail should equal total results")
	}
	if report.Summary == "" {
		t.Error("summary should not be empty")
	}
}

func TestSupervisor_ValidateAll_Empty(t *testing.T) {
	kg := &KnowledgeGraph{
		Version: "1.0",
		Project: ProjectMeta{Name: "empty"},
		Nodes:   []GraphNode{},
		Edges:   []GraphEdge{},
		Layers:  []GraphLayer{},
	}
	supervisor := NewGraphSupervisor(nil)
	report := supervisor.ValidateAll(kg)

	if len(report.Results) != 6 {
		t.Errorf("expected 6 results, got %d", len(report.Results))
	}
	if report.PassCount != 6 {
		t.Errorf("expected 6 passes for empty graph, got %d", report.PassCount)
	}
}

func TestSupervisor_C1_SinglePackage(t *testing.T) {
	kg := makeTestGraph(5)
	kg.Nodes[0].FilePath = "pkg1/file.go"
	kg.Nodes[1].FilePath = "pkg2/file.go"
	kg.Nodes[0].Type = NodeTypeFile
	kg.Nodes[1].Type = NodeTypeFile

	rules := DefaultConstraints()
	result := rules[0].Check(kg)

	if result.Status != ConstraintWarn {
		t.Errorf("expected warn for multi-package, got %s", result.Status)
	}
}

func TestSupervisor_C2_LayerDependency(t *testing.T) {
	kg := makeTestGraph(5)
	kg.Edges = append(kg.Edges, GraphEdge{
		Source: kg.Layers[0].NodeIDs[0],
		Target: kg.Layers[2].NodeIDs[0],
		Type:   EdgeTypeCalls,
	})

	rules := DefaultConstraints()
	result := rules[1].Check(kg)

	if result.Status != ConstraintFail {
		t.Errorf("expected fail for reverse dependency, got %s", result.Status)
	}
}

func TestSupervisor_C3_PrimitivePurity(t *testing.T) {
	kg := makeAtomGraph()
	kg.Edges = append(kg.Edges, GraphEdge{
		Source: kg.Nodes[0].ID,
		Target: kg.Nodes[3].ID,
		Type:   EdgeTypeReadsFrom,
	})
	kg.Edges = append(kg.Edges, GraphEdge{
		Source: kg.Nodes[1].ID,
		Target: kg.Nodes[4].ID,
		Type:   EdgeTypeWritesTo,
	})

	rules := DefaultConstraints()
	result := rules[2].Check(kg)

	if result.Status != ConstraintWarn {
		t.Errorf("expected warn for atom with I/O, got %s", result.Status)
	}
}

func TestSupervisor_C4_PortAdapter(t *testing.T) {
	kg := makeTestGraph(5)
	kg.Nodes[3].Tags = []string{"function"}
	kg.Edges = append(kg.Edges, GraphEdge{
		Source: kg.Nodes[3].ID,
		Target: kg.Nodes[4].ID,
		Type:   EdgeTypeDeploys,
	})

	rules := DefaultConstraints()
	result := rules[3].Check(kg)

	if result.Status != ConstraintWarn {
		t.Errorf("expected warn for non-adapter external interaction, got %s", result.Status)
	}
}

func TestSupervisor_C5_StepUnification(t *testing.T) {
	kg := makeTestGraph(5)
	rules := DefaultConstraints()
	result := rules[4].Check(kg)

	if result.Status != ConstraintPass {
		t.Errorf("expected pass for step unification, got %s", result.Status)
	}
}

func TestSupervisor_C6_GenericsFirst(t *testing.T) {
	kg := makeTestGraph(5)
	kg.Nodes[0].LanguageNotes = "generics: uses type parameters"
	kg.Nodes[1].LanguageNotes = "generics: Step[In, Out]"

	rules := DefaultConstraints()
	result := rules[5].Check(kg)

	if result.Status != ConstraintPass {
		t.Errorf("expected pass, got %s", result.Status)
	}
}

func TestNewSupervisorStep(t *testing.T) {
	kg := makeTestGraph(5)
	supervisor := NewGraphSupervisor(nil)
	step := NewSupervisorStep(supervisor, kg)

	ctx := context.Background()
	report, err := step.Execute(ctx, struct{}{})
	if err != nil {
		t.Fatalf("supervisor step: %v", err)
	}
	if report.PassCount == 0 {
		t.Error("expected some passes")
	}
}
