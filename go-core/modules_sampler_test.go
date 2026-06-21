//go:build lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

package core

import (
	"testing"
)

// ──────────────────────────────────────────────
// Sampler Tests
// ──────────────────────────────────────────────

func TestRateSampler_Rate1(t *testing.T) {
	sampler := NewRateSampler(1.0)
	for i := 0; i < 100; i++ {
		if !sampler.ShouldKeep(ExecutionStep{}) {
			t.Error("rate=1.0 should always keep")
		}
	}
}

func TestRateSampler_Rate0(t *testing.T) {
	sampler := NewRateSampler(0.0)
	for i := 0; i < 100; i++ {
		if sampler.ShouldKeep(ExecutionStep{}) {
			t.Error("rate=0.0 should never keep")
		}
	}
}

func TestErrorAlwaysSampler(t *testing.T) {
	sampler := NewErrorAlwaysSampler()
	if sampler.ShouldKeep(ExecutionStep{}) {
		t.Error("should not keep step without error")
	}
	if !sampler.ShouldKeep(ExecutionStep{Error: NewStepError("E", "err", true)}) {
		t.Error("should keep step with error")
	}
}

func TestCompositeSampler(t *testing.T) {
	errorSampler := NewErrorAlwaysSampler()
	rateSampler := NewRateSampler(0.0) // never keep on rate
	composite := NewCompositeSampler(errorSampler, rateSampler)

	// Error step: error sampler says keep, rate says drop -> composite says keep
	if !composite.ShouldKeep(ExecutionStep{Error: NewStepError("E", "err", true)}) {
		t.Error("composite should keep error step")
	}
}

func TestSampler_Apply(t *testing.T) {
	steps := []ExecutionStep{
		{Unit: "Atom", Error: NewStepError("E1", "err", true)},
		{Unit: "Port"},
		{Unit: "Adapter"},
		{Unit: "Atom", Error: NewStepError("E2", "err", true)},
		{Unit: "Port"},
	}

	sampler := NewSampler(NewCompositeSampler(NewErrorAlwaysSampler(), NewRateSampler(0.0)))
	kept := sampler.Apply(steps)

	// Should keep 2 error steps + 1 summary step
	if len(kept) < 2 {
		t.Errorf("expected at least 2 kept steps, got %d", len(kept))
	}
	if sampler.DroppedCount() != 3 {
		t.Errorf("expected 3 dropped, got %d", sampler.DroppedCount())
	}
}
