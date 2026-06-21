package core

import (
	"context"
	"sync"
	"testing"
)

// ──────────────────────────────────────────────
// Observation Tests
// ──────────────────────────────────────────────

func TestInMemoryObservationAdapter_Record(t *testing.T) {
	obs := &InMemoryObservationAdapter{}
	step := NewExecutionStep("Atom", "test", "testing", "")

	obs.Record([]ExecutionStep{step})
	if obs.StepCount() != 1 {
		t.Errorf("expected 1 step, got %d", obs.StepCount())
	}
}

func TestInMemoryObservationAdapter_Concurrency(t *testing.T) {
	obs := &InMemoryObservationAdapter{}
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			obs.Record([]ExecutionStep{NewExecutionStep("Test", "concurrent", "testing", "")})
		}()
	}

	wg.Wait()
	if obs.StepCount() != 100 {
		t.Errorf("expected 100 steps, got %d", obs.StepCount())
	}
}

func TestInMemoryObservationAdapter_Clear(t *testing.T) {
	obs := &InMemoryObservationAdapter{}
	obs.Record([]ExecutionStep{NewExecutionStep("Test", "clear", "test", "")})
	obs.Clear()
	if obs.StepCount() != 0 {
		t.Errorf("expected 0 steps after clear, got %d", obs.StepCount())
	}
}

func TestInMemoryObservationAdapter_GetSteps_Copy(t *testing.T) {
	obs := &InMemoryObservationAdapter{}
	obs.Record([]ExecutionStep{NewExecutionStep("Test", "copy", "test", "")})

	steps := obs.GetSteps()
	// Modify the copy
	steps[0].Unit = "Modified"

	// Original should be unchanged
	original := obs.GetSteps()
	if original[0].Unit == "Modified" {
		t.Error("GetSteps should return a copy, not a reference")
	}
}

func TestTraceTree_Build(t *testing.T) {
	// Create a parent-child relationship
	parentSpan := string(NewSpanID())
	childSpan := string(NewSpanID())

	steps := []ExecutionStep{
		{SpanID: parentSpan, Unit: "Composer", Action: "Run", TraceID: "trace-1"},
		{SpanID: childSpan, ParentID: parentSpan, Unit: "Atom", Action: "Exec", TraceID: "trace-1"},
		{SpanID: string(NewSpanID()), Unit: "Adapter", Action: "Out", TraceID: "trace-1"}, // orphan
	}

	tree := BuildTraceTree(steps)
	if tree.TotalNodes() != 3 {
		t.Errorf("expected 3 nodes, got %d", tree.TotalNodes())
	}

	// Parent should have 1 child
	if len(tree.Roots) != 2 {
		t.Errorf("expected 2 roots (parent + orphan), got %d", len(tree.Roots))
	}

	// Find the parent root
	var parentRoot *TraceNode
	for _, root := range tree.Roots {
		if root.Step.SpanID == parentSpan {
			parentRoot = root
			break
		}
	}
	if parentRoot == nil {
		t.Fatal("parent root not found")
	}
	if len(parentRoot.Children) != 1 {
		t.Errorf("expected 1 child, got %d", len(parentRoot.Children))
	}
	if parentRoot.Children[0].Step.SpanID != childSpan {
		t.Errorf("expected child span %s, got %s", childSpan, parentRoot.Children[0].Step.SpanID)
	}
	if parentRoot.Children[0].Depth != 1 {
		t.Errorf("expected child depth=1, got %d", parentRoot.Children[0].Depth)
	}
}

func TestTraceTree_Empty(t *testing.T) {
	tree := BuildTraceTree([]ExecutionStep{})
	if tree.TotalNodes() != 0 {
		t.Errorf("expected 0 nodes, got %d", tree.TotalNodes())
	}
	if len(tree.Roots) != 0 {
		t.Errorf("expected 0 roots, got %d", len(tree.Roots))
	}
}

func TestTraceTree_Flatten(t *testing.T) {
	parentSpan := string(NewSpanID())
	steps := []ExecutionStep{
		{SpanID: parentSpan, Unit: "Root", Action: "A", TraceID: "t1"},
		{SpanID: string(NewSpanID()), ParentID: parentSpan, Unit: "Child", Action: "B", TraceID: "t1"},
		{SpanID: string(NewSpanID()), ParentID: parentSpan, Unit: "Child", Action: "C", TraceID: "t1"},
	}

	tree := BuildTraceTree(steps)
	flat := tree.Flatten()

	if len(flat) != 3 {
		t.Errorf("expected 3 nodes in flatten, got %d", len(flat))
	}
	// DFS order: root, child1, child2
	if flat[0].Step.Action != "A" {
		t.Errorf("expected first='A', got '%s'", flat[0].Step.Action)
	}
}

func TestNoOpObservationAdapter(t *testing.T) {
	obs := &NoOpObservationAdapter{}
	// Should not panic
	obs.Record([]ExecutionStep{NewExecutionStep("Test", "noop", "test", "")})
	// No assertion needed — just verify no panic
}

func TestNewExecutionStep_HasUUIDs(t *testing.T) {
	step := NewExecutionStep("Atom", "test", "testing", "TestPattern")
	if step.TraceID == "" {
		t.Error("expected non-empty TraceID")
	}
	if step.SpanID == "" {
		t.Error("expected non-empty SpanID")
	}
	if step.Unit != "Atom" {
		t.Errorf("expected Unit='Atom', got '%s'", step.Unit)
	}
	if step.Pattern != "TestPattern" {
		t.Errorf("expected Pattern='TestPattern', got '%s'", step.Pattern)
	}
}

func TestNewExecutionStepWithTrace(t *testing.T) {
	step := NewExecutionStepWithTrace("parent-123", "Port", "validate", "test", "")
	if step.ParentID != "parent-123" {
		t.Errorf("expected ParentID='parent-123', got '%s'", step.ParentID)
	}
}

func TestNewExecutionStepWithError(t *testing.T) {
	err := NewStepError("E001", "something wrong", false)
	step := NewExecutionStepWithError("Adapter", "call", "external call", "", err)
	if step.Error == nil {
		t.Fatal("expected error in step")
	}
	if step.Error.Code != "E001" {
		t.Errorf("expected error code 'E001', got '%s'", step.Error.Code)
	}
}

func TestTraceContext(t *testing.T) {
	ctx := context.Background()
	tc := TraceContext{TraceID: "trace-abc", SpanID: "span-xyz"}

	ctx = WithTraceContext(ctx, tc)
	got, ok := GetTraceContext(ctx)
	if !ok {
		t.Fatal("expected TraceContext in context")
	}
	if got.TraceID != "trace-abc" {
		t.Errorf("expected TraceID='trace-abc', got '%s'", got.TraceID)
	}
	if got.SpanID != "span-xyz" {
		t.Errorf("expected SpanID='span-xyz', got '%s'", got.SpanID)
	}
}

func TestTraceContext_Missing(t *testing.T) {
	ctx := context.Background()
	_, ok := GetTraceContext(ctx)
	if ok {
		t.Error("expected no TraceContext in empty context")
	}
}
