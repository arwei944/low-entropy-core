package core

import (
	"context"
	"fmt"
	"time"
)

// ──────────────────────────────────────────────
// GuardianAction — 监督决策动作
// ──────────────────────────────────────────────

// GuardianAction is the action to take based on the decision.
type GuardianAction string

const (
	ActionAllow    GuardianAction = "Allow"
	ActionWarn     GuardianAction = "Warn"
	ActionBlock    GuardianAction = "Block"
	ActionRollback GuardianAction = "Rollback"
)

// ──────────────────────────────────────────────
// GuardianInput — 全部监督器结果的组合输入
// ──────────────────────────────────────────────

// GuardianInput is the combined input from all guardian watchers.
type GuardianInput struct {
	EntropyAlert EntropyAlert
	TranspAlert  TransparencyAlert
	DriftResult  DriftOutput
	ArchAlert    ArchitectureAlert
}

// ──────────────────────────────────────────────
// GuardianDecision — 决策引擎输出
// ──────────────────────────────────────────────

// GuardianDecision is the output of the decision engine.
type GuardianDecision struct {
	Action       GuardianAction
	EntropyAlert EntropyAlert
	TranspAlert  TransparencyAlert
	DriftResult  DriftOutput
	ArchAlert    ArchitectureAlert
	Reason       string
	Timestamp    time.Time
	AllOK        bool
}

// ──────────────────────────────────────────────
// DecisionEngine — 监督决策引擎 (TASK-1.5)
// ──────────────────────────────────────────────

// DecisionEngine implements Composer[GuardianDecision] — it composes the
// results from all 4 watchers and makes a decision. Internally it evaluates
// the combined GuardianInput against a set of priority-ordered rules to
// produce a GuardianDecision.
//
// Decision rules (evaluated in priority order):
//  1. Rollback: entropy Red OR ShouldQuarantine OR violations >= 3
//  2. Block:    entropy Orange OR drift score > 0.7
//  3. Warn:     entropy Yellow OR drift score > 0.3 OR !IsHealthy
//  4. Allow:    otherwise (all OK)
type DecisionEngine struct {
	obs ObservationAdapter
}

// NewDecisionEngine creates a new DecisionEngine with the given observation adapter.
// If obs is nil, a NoOpObservationAdapter is used.
func NewDecisionEngine(obs ObservationAdapter) *DecisionEngine {
	if obs == nil {
		obs = &NoOpObservationAdapter{}
	}
	return &DecisionEngine{obs: obs}
}

// Run evaluates the combined guardian results and produces a decision.
// It records an ExecutionStep via the ObservationAdapter for full observability.
func (d *DecisionEngine) Run(ctx context.Context, input GuardianInput) (GuardianDecision, error) {
	// Normalize context.
	if ctx == nil {
		ctx = context.Background()
	}

	// Respect context cancellation.
	select {
	case <-ctx.Done():
		return GuardianDecision{}, ctx.Err()
	default:
	}

	start := time.Now()

	decision := d.evaluate(input)

	// Build and record the execution step.
	es := NewExecutionStep("DecisionEngine", "evaluate", "guardian decision evaluation", "Composer")
	es.DurationMs = time.Since(start).Milliseconds()
	es.Metadata = map[string]interface{}{
		"action":        string(decision.Action),
		"all_ok":        decision.AllOK,
		"reason":        decision.Reason,
		"entropy_level": string(decision.EntropyAlert.Level),
		"drift_score":   decision.DriftResult.DriftScore,
		"violations":    decision.ArchAlert.Violations,
		"is_healthy":    decision.TranspAlert.IsHealthy,
	}

	d.obs.Record([]ExecutionStep{es})

	return decision, nil
}

// evaluate applies the priority-ordered decision rules to the combined input.
// This is a pure function: same input always yields the same decision.
func (d *DecisionEngine) evaluate(input GuardianInput) GuardianDecision {
	decision := GuardianDecision{
		EntropyAlert: input.EntropyAlert,
		TranspAlert:  input.TranspAlert,
		DriftResult:  input.DriftResult,
		ArchAlert:    input.ArchAlert,
		Timestamp:    time.Now(),
	}

	// Rule 1: Rollback — entropy red, agent must be quarantined, or
	// architecture violations are critical (>= 3).
	if input.EntropyAlert.Level == EntropyRed ||
		input.DriftResult.ShouldQuarantine ||
		input.ArchAlert.Violations >= 3 {
		decision.Action = ActionRollback
		decision.Reason = buildRollbackReason(input)
		decision.AllOK = false
		return decision
	}

	// Rule 2: Block — entropy orange or drift score exceeds the block threshold.
	if input.EntropyAlert.Level == EntropyOrange ||
		input.DriftResult.DriftScore > 0.7 {
		decision.Action = ActionBlock
		decision.Reason = buildBlockReason(input)
		decision.AllOK = false
		return decision
	}

	// Rule 3: Warn — entropy yellow, elevated drift, or transparency is unhealthy.
	if input.EntropyAlert.Level == EntropyYellow ||
		input.DriftResult.DriftScore > 0.3 ||
		!input.TranspAlert.IsHealthy {
		decision.Action = ActionWarn
		decision.Reason = buildWarnReason(input)
		decision.AllOK = false
		return decision
	}

	// Rule 4: Allow — all checks passed.
	decision.Action = ActionAllow
	decision.Reason = "all guardian checks passed: entropy within normal range, no significant drift, transparency healthy, architecture compliant"
	decision.AllOK = true
	return decision
}

// ──────────────────────────────────────────────
// Reason builders (pure functions)
// ──────────────────────────────────────────────

// buildRollbackReason constructs a detailed reason for a rollback decision.
func buildRollbackReason(input GuardianInput) string {
	var reasons []string

	if input.EntropyAlert.Level == EntropyRed {
		reasons = append(reasons,
			fmt.Sprintf("entropy at red level (score=%.2f)", input.EntropyAlert.Score),
		)
	}
	if input.DriftResult.ShouldQuarantine {
		reasons = append(reasons,
			fmt.Sprintf("agent %q requires quarantine (drift_score=%.2f, overreach=%v)",
				input.DriftResult.AgentID, input.DriftResult.DriftScore, input.DriftResult.OverreachDetected),
		)
	}
	if input.ArchAlert.Violations >= 3 {
		reasons = append(reasons,
			fmt.Sprintf("architecture violations critical (%d violations)", input.ArchAlert.Violations),
		)
	}

	return "ROLLBACK: " + joinReasons(reasons)
}

// buildBlockReason constructs a detailed reason for a block decision.
func buildBlockReason(input GuardianInput) string {
	var reasons []string

	if input.EntropyAlert.Level == EntropyOrange {
		reasons = append(reasons,
			fmt.Sprintf("entropy at orange level (score=%.2f)", input.EntropyAlert.Score),
		)
	}
	if input.DriftResult.DriftScore > 0.7 {
		reasons = append(reasons,
			fmt.Sprintf("drift score critical (%.2f)", input.DriftResult.DriftScore),
		)
	}

	return "BLOCK: " + joinReasons(reasons)
}

// buildWarnReason constructs a detailed reason for a warn decision.
func buildWarnReason(input GuardianInput) string {
	var reasons []string

	if input.EntropyAlert.Level == EntropyYellow {
		reasons = append(reasons,
			fmt.Sprintf("entropy at yellow level (score=%.2f)", input.EntropyAlert.Score),
		)
	}
	if input.DriftResult.DriftScore > 0.3 {
		reasons = append(reasons,
			fmt.Sprintf("drift score elevated (%.2f)", input.DriftResult.DriftScore),
		)
	}
	if !input.TranspAlert.IsHealthy {
		reasons = append(reasons,
			fmt.Sprintf("transparency unhealthy (coverage=%.2f%%, breakpoints=%d, time_anomalies=%d, open_traces=%d)",
				input.TranspAlert.CoverageRate,
				len(input.TranspAlert.BreakPoints),
				len(input.TranspAlert.TimeAnomalies),
				len(input.TranspAlert.OpenTraces),
			),
		)
	}

	return "WARN: " + joinReasons(reasons)
}

// joinReasons joins reason strings with "; ".
func joinReasons(reasons []string) string {
	if len(reasons) == 0 {
		return "no specific reason"
	}
	result := reasons[0]
	for i := 1; i < len(reasons); i++ {
		result += "; " + reasons[i]
	}
	return result
}