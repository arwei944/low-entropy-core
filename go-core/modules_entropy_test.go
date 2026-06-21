//go:build lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

package core

import (
	"testing"
)

// ──────────────────────────────────────────────
// EntropyMetrics Tests
// ──────────────────────────────────────────────

func TestEntropyCollector_Collect(t *testing.T) {
	store := NewInMemoryStepStore(1000)
	store.Record([]ExecutionStep{
		{Unit: "Atom", Pattern: "Pipeline", DurationMs: 10},
		{Unit: "Atom", Pattern: "Pipeline", DurationMs: 20},
		{Unit: "Port", Pattern: "Handoff", DurationMs: 5, Error: NewStepError("E", "err", true)},
		{Unit: "Adapter", Pattern: "Pipeline", DurationMs: 30},
	})

	collector := NewEntropyCollector()
	snap := collector.Collect(store)

	if snap.TotalSteps != 4 {
		t.Errorf("expected TotalSteps=4, got %d", snap.TotalSteps)
	}
	if snap.ErrorSteps != 1 {
		t.Errorf("expected ErrorSteps=1, got %d", snap.ErrorSteps)
	}
	if snap.UniquePatterns != 2 {
		t.Errorf("expected UniquePatterns=2, got %d", snap.UniquePatterns)
	}
	if snap.UniqueUnits != 3 {
		t.Errorf("expected UniqueUnits=3, got %d", snap.UniqueUnits)
	}
	if snap.ErrorRate != 0.25 {
		t.Errorf("expected ErrorRate=0.25, got %f", snap.ErrorRate)
	}
	if snap.EntropyScore <= 0 {
		t.Error("expected positive entropy score")
	}
}

func TestEntropyCollector_Empty(t *testing.T) {
	store := NewInMemoryStepStore(1000)
	collector := NewEntropyCollector()
	snap := collector.Collect(store)

	if snap.TotalSteps != 0 {
		t.Errorf("expected 0 steps, got %d", snap.TotalSteps)
	}
	if snap.EntropyScore != 0 {
		t.Errorf("expected 0 entropy score, got %f", snap.EntropyScore)
	}
}
