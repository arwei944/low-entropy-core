//go:build lecore_tier2 || lecore_tier3 || lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

package core

import (
	"fmt"
	"testing"
)

func TestNewTierTransition(t *testing.T) {
	tt := NewTierTransition(TierL1, TierL3)
	if tt == nil {
		t.Fatal("NewTierTransition returned nil")
	}
	if tt.FromTier() != TierL1 {
		t.Errorf("expected FromTier TierL1, got %s", tt.FromTier())
	}
	if tt.ToTier() != TierL3 {
		t.Errorf("expected ToTier TierL3, got %s", tt.ToTier())
	}
}

func TestTierTransition_PhaseCount(t *testing.T) {
	tests := []struct {
		name     string
		fromTier ComplexityTier
		toTier   ComplexityTier
		minCount int
	}{
		{"L0 to L1", TierL0, TierL1, 2},   // degradation + fastpath
		{"L0 to L2", TierL0, TierL2, 8},   // L1 modules + L2 modules
		{"L1 to L3", TierL1, TierL3, 8},   // L2 modules + L3 modules
		{"L0 to L3", TierL0, TierL3, 16},  // L1 + L2 + L3 modules
		{"L5 to L5", TierL5, TierL5, 0},   // same tier
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewTierTransition(tt.fromTier, tt.toTier)
			if m.PhaseCount() < tt.minCount {
				t.Errorf("PhaseCount() = %d, want >= %d", m.PhaseCount(), tt.minCount)
			}
		})
	}
}

func TestTierTransition_Advance(t *testing.T) {
	tt := NewTierTransition(TierL1, TierL2)

	if tt.IsDone() {
		t.Error("should not be done initially")
	}

	// Advance one phase
	err := tt.Advance()
	if err != nil {
		t.Fatalf("Advance() failed: %v", err)
	}

	if tt.Progress() <= 0 {
		t.Error("progress should be > 0 after first advance")
	}
}

func TestTierTransition_AdvanceAll(t *testing.T) {
	tt := NewTierTransition(TierL1, TierL2)

	err := tt.AdvanceAll()
	if err != nil {
		t.Fatalf("AdvanceAll() failed: %v", err)
	}

	if !tt.IsDone() {
		t.Error("should be done after AdvanceAll")
	}
	if tt.Progress() != 1.0 {
		t.Errorf("progress should be 1.0, got %f", tt.Progress())
	}
}

func TestTierTransition_Rollback(t *testing.T) {
	tt := NewTierTransition(TierL1, TierL2)

	// Advance one
	if err := tt.Advance(); err != nil {
		t.Fatalf("Advance() failed: %v", err)
	}

	phase := tt.CurrentPhase()
	if phase < 0 {
		t.Fatal("should have advanced past phase -1")
	}

	// Rollback
	if err := tt.Rollback(); err != nil {
		t.Fatalf("Rollback() failed: %v", err)
	}

	if tt.CurrentPhase() != phase-1 {
		t.Errorf("expected phase %d after rollback, got %d", phase-1, tt.CurrentPhase())
	}
}

func TestTierTransition_RollbackFromStart(t *testing.T) {
	tt := NewTierTransition(TierL1, TierL2)
	err := tt.Rollback()
	if err == nil {
		t.Error("Rollback from start should return error")
	}
}

func TestTierTransition_AdvancePastEnd(t *testing.T) {
	tt := NewTierTransition(TierL1, TierL2)

	// Complete all phases
	_ = tt.AdvanceAll()

	// Try to advance again
	err := tt.Advance()
	if err != nil {
		// This is expected — "transition already complete"
	} else {
		t.Log("Advance after completion returned nil (no more phases)")
	}
}

func TestTierTransition_IsEnabled(t *testing.T) {
	tt := NewTierTransition(TierL1, TierL2)

	// Initially nothing enabled
	if tt.IsEnabled("eventstore") {
		t.Error("eventstore should not be enabled initially")
	}

	_ = tt.AdvanceAll()

	// After all advances, some modules should be enabled
	enabled := tt.EnabledModules()
	if len(enabled) == 0 {
		t.Error("expected some enabled modules after AdvanceAll")
	}
}

func TestTierTransition_Progress(t *testing.T) {
	tt := NewTierTransition(TierL1, TierL2)
	total := tt.PhaseCount()

	if total == 0 {
		t.Skip("no phases to test")
	}

	_ = tt.Advance()
	progress := tt.Progress()
	expected := 1.0 / float64(total)

	if progress < expected-0.01 || progress > expected+0.01 {
		t.Errorf("progress = %f, want ~%f", progress, expected)
	}
}

func TestTierTransition_EmptyTransition(t *testing.T) {
	// Same tier — no phases
	tt := NewTierTransition(TierL3, TierL3)
	if tt.PhaseCount() != 0 {
		t.Errorf("same tier should have 0 phases, got %d", tt.PhaseCount())
	}
	if tt.Progress() != 1.0 {
		t.Errorf("empty transition progress should be 1.0, got %f", tt.Progress())
	}
}

func TestTierTransition_Validate(t *testing.T) {
	tt := NewTierTransition(TierL1, TierL2)

	// Add a validate function to the first phase
	if len(tt.phases) > 0 {
		tt.phases[0].Validate = func() error {
			return nil // simulate validation passing
		}
		err := tt.Advance()
		if err != nil {
			t.Errorf("Advance with validate should succeed: %v", err)
		}
	}
}

func TestTierTransition_ValidateFails(t *testing.T) {
	tt := NewTierTransition(TierL1, TierL2)

	if len(tt.phases) > 0 {
		tt.phases[0].Validate = func() error {
			return fmt.Errorf("validation failed")
		}
		err := tt.Advance()
		if err == nil {
			t.Error("Advance with failing validate should return error")
		}
	}
}

func TestTierTransition_RollbackWithRollbackFn(t *testing.T) {
	tt := NewTierTransition(TierL1, TierL2)
	rolledBack := false

	if len(tt.phases) > 0 {
		tt.phases[0].Rollback = func() error {
			rolledBack = true
			return nil
		}
		_ = tt.Advance()
		_ = tt.Rollback()

		if !rolledBack {
			t.Error("rollback function should have been called")
		}
	}
}

func TestTierTransition_CurrentPhase(t *testing.T) {
	tt := NewTierTransition(TierL1, TierL2)
	if tt.CurrentPhase() != -1 {
		t.Errorf("initial phase should be -1, got %d", tt.CurrentPhase())
	}
	_ = tt.Advance()
	if tt.CurrentPhase() != 0 {
		t.Errorf("phase after first advance should be 0, got %d", tt.CurrentPhase())
	}
}