//go:build lecore_tier5 || lecore_tier6 || lecore_tier7

package core_test

import (
	"context"
	"errors"
	"testing"

	. "low-entropy-core/go-core"
)

func TestIntegration_V3FullPipeline(t *testing.T) {
	ctx := context.Background()

	doubleAtom := Atom[int, int](func(input int) int { return input * 2 })

	validatePort := NewPort(func(ctx context.Context, input int) (int, error) {
		if input <= 0 {
			return 0, errors.New("must be positive")
		}
		return input, nil
	})

	obs := &InMemoryObservationAdapter{}
	pipeline := NewPipeline[int](obs,
		PortAsStep[int, int](validatePort),
		AtomAsStep[int, int](doubleAtom),
		AtomAsStep[int, int](doubleAtom),
	)

	result, steps, err := pipeline.Run(ctx, 5)
	if err != nil {
		t.Fatalf("pipeline.Run failed: %v", err)
	}
	if result != 20 {
		t.Errorf("expected 5 -> validate -> double -> double = 20, got %d", result)
	}

	recordedSteps := obs.GetSteps()
	if len(recordedSteps) != 3 {
		t.Errorf("expected 3 recorded steps, got %d", len(recordedSteps))
	}

	if len(steps) != 3 {
		t.Errorf("expected 3 steps from Run(), got %d", len(steps))
	}

	expectedUnits := []string{"Port", "Atom", "Atom"}
	for i, unit := range expectedUnits {
		if steps[i].Unit != unit {
			t.Errorf("step %d: expected unit %q, got %q", i, unit, steps[i].Unit)
		}
	}

	tree := obs.GetTraceTree()
	if tree == nil {
		t.Fatal("expected non-nil trace tree")
	}
	if tree.TotalNodes() != 3 {
		t.Errorf("expected 3 total nodes in trace tree, got %d", tree.TotalNodes())
	}
	if len(tree.Roots) != 3 {
		t.Errorf("expected 3 roots in trace tree, got %d", len(tree.Roots))
	}
	if len(tree.Roots) > 0 {
		traceID := tree.Roots[0].Step.TraceID
		for i, root := range tree.Roots {
			if root.Step.TraceID != traceID {
				t.Errorf("step %d: expected TraceID %q, got %q", i, traceID, root.Step.TraceID)
			}
		}
	}
}

func TestIntegration_GuardianDecision(t *testing.T) {
	obs := &InMemoryObservationAdapter{}
	engine := NewDecisionEngine(obs)

	t.Run("Allow", func(t *testing.T) {
		input := GuardianInput{
			EntropyAlert: EntropyAlert{Level: EntropyOK, Score: 5},
			TranspAlert:  TransparencyAlert{IsHealthy: true, CoverageRate: 100},
			DriftResult:  DriftOutput{DriftScore: 0.0, ShouldQuarantine: false},
			ArchAlert:    ArchitectureAlert{Violations: 0},
		}
		decision, err := engine.Run(context.Background(), input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if decision.Action != ActionAllow {
			t.Errorf("expected ActionAllow, got %s", decision.Action)
		}
		if !decision.AllOK {
			t.Error("expected AllOK=true")
		}
	})

	t.Run("Warn", func(t *testing.T) {
		input := GuardianInput{
			EntropyAlert: EntropyAlert{Level: EntropyYellow, Score: 30},
			TranspAlert:  TransparencyAlert{IsHealthy: false, CoverageRate: 50},
			DriftResult:  DriftOutput{DriftScore: 0.1, ShouldQuarantine: false},
			ArchAlert:    ArchitectureAlert{Violations: 0},
		}
		decision, err := engine.Run(context.Background(), input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if decision.Action != ActionWarn {
			t.Errorf("expected ActionWarn, got %s", decision.Action)
		}
	})

	t.Run("Rollback", func(t *testing.T) {
		input := GuardianInput{
			EntropyAlert: EntropyAlert{Level: EntropyRed, Score: 120},
			TranspAlert:  TransparencyAlert{IsHealthy: true, CoverageRate: 100},
			DriftResult:  DriftOutput{DriftScore: 0.0, ShouldQuarantine: false},
			ArchAlert:    ArchitectureAlert{Violations: 0},
		}
		decision, err := engine.Run(context.Background(), input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if decision.Action != ActionRollback {
			t.Errorf("expected ActionRollback, got %s", decision.Action)
		}
	})

	t.Run("Block", func(t *testing.T) {
		input := GuardianInput{
			EntropyAlert: EntropyAlert{Level: EntropyOK, Score: 5},
			TranspAlert:  TransparencyAlert{IsHealthy: true, CoverageRate: 100},
			DriftResult:  DriftOutput{DriftScore: 0.85, ShouldQuarantine: false},
			ArchAlert:    ArchitectureAlert{Violations: 0},
		}
		decision, err := engine.Run(context.Background(), input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if decision.Action != ActionBlock {
			t.Errorf("expected ActionBlock, got %s", decision.Action)
		}
	})

	steps := obs.GetSteps()
	if len(steps) != 4 {
		t.Errorf("expected 4 recorded steps (one per decision), got %d", len(steps))
	}
	for i, step := range steps {
		if step.Unit != "DecisionEngine" {
			t.Errorf("step %d: expected unit 'DecisionEngine', got %q", i, step.Unit)
		}
	}
}
