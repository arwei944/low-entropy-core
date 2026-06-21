//go:build lecore_tier3 || lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

package core

import (
	"context"
	"testing"
)

func TestNewHandoff_RecordsExecutionSteps(t *testing.T) {
	obs := &InMemoryObservationAdapter{}

	source := NewPipeline[any](obs,
		AtomAsStep(Atom[any, any](func(x any) any { return x })),
	)
	target := NewPipeline[any](obs,
		AtomAsStep(Atom[any, any](func(x any) any { return x })),
	)

	snapshot := &DefaultSnapshotAdapter{}
	transport := InProcTransport

	handoff := NewHandoff(source, target, snapshot, transport, obs)

	req := HandoffRequest{
		SourceID: "agent-a",
		TargetID: "agent-b",
		TaskType: "test",
		Payload:  "hello",
		Token:    "tok-001",
	}

	result, steps, err := handoff.Run(context.Background(), req)
	if err != nil {
		t.Fatalf("handoff failed: %v", err)
	}

	hr, ok := result.(HandoffResult)
	if !ok {
		t.Fatalf("expected HandoffResult, got %T", result)
	}
	if !hr.Success {
		t.Errorf("expected successful handoff, got error: %s", hr.Error)
	}

	if len(steps) == 0 {
		t.Error("expected execution steps to be recorded")
	}

	hasHandoffPattern := false
	for _, s := range steps {
		if s.Pattern == "Handoff" {
			hasHandoffPattern = true
			break
		}
	}
	if !hasHandoffPattern {
		t.Error("expected at least one step with Pattern 'Handoff'")
	}

	if obs.StepCount() == 0 {
		t.Error("expected ObservationAdapter to have recorded steps")
	}
}

func TestNewHandoff_SourceError(t *testing.T) {
	obs := &InMemoryObservationAdapter{}

	source := NewPipeline[any](obs,
		StepFunc[any, any]{
			execute: func(ctx context.Context, input any) (any, error) {
				return nil, NewStepError("SOURCE_FAIL", "source failed", false)
			},
			unitType: "Failing",
		},
	)
	target := NewPipeline[any](obs,
		AtomAsStep(Atom[any, any](func(x any) any { return x })),
	)

	handoff := NewHandoff(source, target, &DefaultSnapshotAdapter{}, InProcTransport, obs)

	_, _, err := handoff.Run(context.Background(), HandoffRequest{
		SourceID: "a", TargetID: "b", TaskType: "test", Payload: "x", Token: "t",
	})
	if err == nil {
		t.Fatal("expected error from source failure")
	}
}
