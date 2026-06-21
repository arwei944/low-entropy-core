package migrate

import "fmt"

// ValidationIssue represents a single finding from enhanced validation.
type ValidationIssue struct {
	Severity string `json:"severity"` // "error" | "warning" | "info"
	Message  string `json:"message"`
}

// ValidationReport is the aggregate result of an enhanced validation run.
type ValidationReport struct {
	Passed bool              `json:"passed"`
	Issues []ValidationIssue `json:"issues"`
}

// EnhancedValidator runs additional validation checks beyond the gate chain,
// including pattern-map unknowns analysis and gate-chain constraint evaluation.
type EnhancedValidator struct {
	Log *MigrationLog
}

// Validate performs enhanced validation on the given MigrationContext.
// It checks the number of unknowns in the PatternMap, runs the default gate
// chain, and records the results as a log entry (Phase="validate",
// ActionType="validate_run").
func (v *EnhancedValidator) Validate(ctx *MigrationContext) *ValidationReport {
	report := &ValidationReport{
		Passed: true,
		Issues: make([]ValidationIssue, 0),
	}

	// --- Check 1: Unknowns count in PatternMap ---
	if ctx.PatternMap != nil {
		unknownCount := len(ctx.PatternMap.Unknowns)
		if unknownCount > 0 {
			report.Issues = append(report.Issues, ValidationIssue{
				Severity: "warning",
				Message:  fmt.Sprintf("pattern map contains %d unknown function(s)", unknownCount),
			})
			report.Passed = false
		}
	}

	// --- Check 2: Default gate chain ---
	chain := DefaultGateChain()
	decision := chain.Evaluate(ctx)
	if !decision.Pass {
		report.Passed = false
		for _, rule := range decision.BlockedRules {
			report.Issues = append(report.Issues, ValidationIssue{
				Severity: "error",
				Message:  fmt.Sprintf("gate %s blocked: %s", decision.GateID, rule),
			})
		}
		for _, w := range decision.Warnings {
			report.Issues = append(report.Issues, ValidationIssue{
				Severity: "warning",
				Message:  fmt.Sprintf("gate %s warning: %s", decision.GateID, w),
			})
		}
	}

	// --- Record validation result in the log ---
	if v.Log != nil {
		summary := "passed"
		if !report.Passed {
			summary = fmt.Sprintf("failed (%d issue(s))", len(report.Issues))
		}
		_ = v.Log.Append(MigrationLogEntry{
			Phase:      "validate",
			ActionType: "validate_run",
			Metadata: map[string]string{
				"result":  summary,
				"issues":  fmt.Sprintf("%d", len(report.Issues)),
				"passed":  fmt.Sprintf("%v", report.Passed),
			},
		})
	}

	return report
}
