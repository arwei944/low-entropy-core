//go:build lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

package core

import (
	"testing"
)

func TestDecisionEngine_Allow(t *testing.T) {
	obs := &InMemoryObservationAdapter{}
	engine := NewDecisionEngine(obs)

	input := GuardianInput{
		TranspAlert: TransparencyAlert{
			IsHealthy:       true,
			CoverageRate:    0.95,
			TotalAuditCount: 10,
		},
	}

	decision := engine.evaluate(input)
	if decision.Action != ActionAllow {
		t.Errorf("expected Allow, got %s", decision.Action)
	}
}

func TestDecisionEngine_Block(t *testing.T) {
	obs := &InMemoryObservationAdapter{}
	engine := NewDecisionEngine(obs)

	input := GuardianInput{
		TranspAlert: TransparencyAlert{
			IsHealthy:       false,
			CoverageRate:    0.1,
			TotalAuditCount: 0,
		},
	}

	decision := engine.evaluate(input)
	if decision.Action == ActionAllow {
		t.Error("expected Block or Warn, got Allow")
	}
}

func TestDecisionEngine_NilObs(t *testing.T) {
	engine := NewDecisionEngine(nil)

	input := GuardianInput{
		TranspAlert: TransparencyAlert{
			IsHealthy: true,
		},
	}

	decision := engine.evaluate(input)
	if decision.Action != ActionAllow {
		t.Errorf("expected Allow, got %s", decision.Action)
	}
}

func TestDecisionEngine_StaticReviewViolation(t *testing.T) {
	obs := &InMemoryObservationAdapter{}
	engine := NewDecisionEngine(obs)

	input := GuardianInput{
		TranspAlert: TransparencyAlert{
			IsHealthy:       true,
			CoverageRate:    0.9,
			TotalAuditCount: 5,
		},
		StaticReviewResult: StaticReviewResult{
			Violations: []Violation{
				{Rule: "PRIMITIVE_MISMATCH", Severity: "error", Detail: "declared Atom but uses I/O"},
			},
		},
	}

	decision := engine.evaluate(input)
	if decision.Action == ActionAllow {
		t.Error("expected Block for static review violation, got Allow")
	}
}

func TestDecisionEngine_StaticReviewPassed(t *testing.T) {
	obs := &InMemoryObservationAdapter{}
	engine := NewDecisionEngine(obs)

	input := GuardianInput{
		TranspAlert: TransparencyAlert{
			IsHealthy:       true,
			CoverageRate:    0.95,
			TotalAuditCount: 10,
		},
		StaticReviewResult: StaticReviewResult{
			Violations: nil,
		},
	}

	decision := engine.evaluate(input)
	if decision.Action != ActionAllow {
		t.Errorf("expected Allow, got %s", decision.Action)
	}
}