//go:build lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

package core

import "testing"

func TestDecisionEngine_StaticReview(t *testing.T) {
	engine := NewDecisionEngine(nil)

	input := GuardianInput{
		StaticReviewResult: StaticReviewResult{
			Violations: []Violation{
				{Rule: "external_dependency", Severity: "error", Detail: "test"},
			},
		},
	}
	decision := engine.evaluate(input)
	if decision.Action != ActionBlock {
		t.Errorf("expected Block for 1 static error, got %s", decision.Action)
	}

	input2 := GuardianInput{
		StaticReviewResult: StaticReviewResult{
			Violations: []Violation{
				{Rule: "e1", Severity: "error", Detail: "test"},
				{Rule: "e2", Severity: "error", Detail: "test"},
				{Rule: "e3", Severity: "error", Detail: "test"},
			},
		},
	}
	decision2 := engine.evaluate(input2)
	if decision2.Action != ActionRollback {
		t.Errorf("expected Rollback for 3 static errors, got %s", decision2.Action)
	}

	input3 := GuardianInput{
		TranspAlert: TransparencyAlert{IsHealthy: true},
	}
	decision3 := engine.evaluate(input3)
	if decision3.Action != ActionAllow {
		t.Errorf("expected Allow for clean static review, got %s (reason: %s)", decision3.Action, decision3.Reason)
	}
}

func TestDecisionEngine_StaticReviewWarnOnly(t *testing.T) {
	engine := NewDecisionEngine(nil)

	input := GuardianInput{
		TranspAlert: TransparencyAlert{IsHealthy: true},
		StaticReviewResult: StaticReviewResult{
			Violations: []Violation{
				{Rule: "function_too_long", Severity: "warn", Detail: "test"},
				{Rule: "high_complexity", Severity: "warn", Detail: "test"},
			},
		},
	}
	decision := engine.evaluate(input)
	if decision.Action != ActionAllow {
		t.Errorf("expected Allow for warn-only violations, got %s", decision.Action)
	}
}
