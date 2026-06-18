// Package core — 监督决策引擎 + 告警适配器 (v4.0)
//
// 合并自: guardian_decision.go + guardian_alert.go
//
// 包含:
//   - DecisionEngine: 四维度监督结果综合决策 (Composer)
//   - AlertAdapter: 告警分发适配器 (Adapter)
//   - GuardianDecision / GuardianAction / GuardianInput: 决策相关类型
//   - AlertResult / AlertConfig / AlertChannel: 告警相关类型
//
// 决策规则 (优先级序):
//   1. Rollback: entropy Red OR ShouldQuarantine OR violations >= 3
//   2. Block:    entropy Orange OR drift score > 0.7
//   3. Warn:     entropy Yellow OR drift score > 0.3 OR !IsHealthy
//   4. Allow:    全部通过
package core

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// ============================================================================
// SECTION 1: GuardianAction / GuardianDecision / GuardianInput
// ============================================================================

// GuardianAction is the action to take based on the decision.
type GuardianAction string

const (
	ActionAllow    GuardianAction = "Allow"
	ActionWarn     GuardianAction = "Warn"
	ActionBlock    GuardianAction = "Block"
	ActionRollback GuardianAction = "Rollback"
)

// GuardianInput is the combined input from all guardian watchers.
type GuardianInput struct {
	EntropyAlert       EntropyAlert
	TranspAlert        TransparencyAlert
	DriftResult        DriftOutput
	ArchAlert          ArchitectureAlert
	StaticReviewResult StaticReviewResult // P2: 静态审核结果
}

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

// ============================================================================
// SECTION 2: DecisionEngine — 监督决策引擎 (Composer)
// ============================================================================

// DecisionEngine implements Composer[GuardianDecision] — it composes the
// results from all 4 watchers and makes a decision.
type DecisionEngine struct {
	obs ObservationAdapter
}

// NewDecisionEngine creates a new DecisionEngine with the given observation adapter.
func NewDecisionEngine(obs ObservationAdapter) *DecisionEngine {
	if obs == nil {
		obs = &NoOpObservationAdapter{}
	}
	return &DecisionEngine{obs: obs}
}

// Run evaluates the combined guardian results and produces a decision.
func (d *DecisionEngine) Run(ctx context.Context, input GuardianInput) (GuardianDecision, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	select {
	case <-ctx.Done():
		return GuardianDecision{}, ctx.Err()
	default:
	}

	start := time.Now()
	decision := d.evaluate(input)

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

// evaluate is a pure function: same input always yields the same decision.
func (d *DecisionEngine) evaluate(input GuardianInput) GuardianDecision {
	decision := GuardianDecision{
		EntropyAlert: input.EntropyAlert,
		TranspAlert:  input.TranspAlert,
		DriftResult:  input.DriftResult,
		ArchAlert:    input.ArchAlert,
		Timestamp:    time.Now(),
	}

	// Rule 0: Static Review — 编译前审核（P2 新增）
	// 3+ error → Rollback, 1+ error → Block
	staticErrors := input.StaticReviewResult.ErrorCount()
	if staticErrors >= 3 {
		decision.Action = ActionRollback
		decision.Reason = fmt.Sprintf("ROLLBACK: static review found %d errors (>=3)", staticErrors)
		decision.AllOK = false
		return decision
	}
	if staticErrors > 0 {
		decision.Action = ActionBlock
		decision.Reason = fmt.Sprintf("BLOCK: static review found %d error(s)", staticErrors)
		decision.AllOK = false
		return decision
	}

	// Rule 1: Rollback
	if input.EntropyAlert.Level == EntropyRed ||
		input.DriftResult.ShouldQuarantine ||
		input.ArchAlert.Violations >= 3 {
		decision.Action = ActionRollback
		decision.Reason = buildRollbackReason(input)
		decision.AllOK = false
		return decision
	}

	// Rule 2: Block
	if input.EntropyAlert.Level == EntropyOrange ||
		input.DriftResult.DriftScore > 0.7 {
		decision.Action = ActionBlock
		decision.Reason = buildBlockReason(input)
		decision.AllOK = false
		return decision
	}

	// Rule 3: Warn
	if input.EntropyAlert.Level == EntropyYellow ||
		input.DriftResult.DriftScore > 0.3 ||
		!input.TranspAlert.IsHealthy {
		decision.Action = ActionWarn
		decision.Reason = buildWarnReason(input)
		decision.AllOK = false
		return decision
	}

	// Rule 4: Allow
	decision.Action = ActionAllow
	decision.Reason = "all guardian checks passed: static review clean, entropy within normal range, no significant drift, transparency healthy, architecture compliant"
	decision.AllOK = true
	return decision
}

// ============================================================================
// SECTION 3: Reason builders (pure functions)
// ============================================================================

func buildRollbackReason(input GuardianInput) string {
	var reasons []string
	if input.EntropyAlert.Level == EntropyRed {
		reasons = append(reasons,
			fmt.Sprintf("entropy at red level (score=%.2f)", input.EntropyAlert.Score))
	}
	if input.DriftResult.ShouldQuarantine {
		reasons = append(reasons,
			fmt.Sprintf("agent %q requires quarantine (drift_score=%.2f, overreach=%v)",
				input.DriftResult.AgentID, input.DriftResult.DriftScore, input.DriftResult.OverreachDetected))
	}
	if input.ArchAlert.Violations >= 3 {
		reasons = append(reasons,
			fmt.Sprintf("architecture violations critical (%d violations)", input.ArchAlert.Violations))
	}
	return "ROLLBACK: " + joinReasons(reasons)
}

func buildBlockReason(input GuardianInput) string {
	var reasons []string
	if input.EntropyAlert.Level == EntropyOrange {
		reasons = append(reasons,
			fmt.Sprintf("entropy at orange level (score=%.2f)", input.EntropyAlert.Score))
	}
	if input.DriftResult.DriftScore > 0.7 {
		reasons = append(reasons,
			fmt.Sprintf("drift score critical (%.2f)", input.DriftResult.DriftScore))
	}
	return "BLOCK: " + joinReasons(reasons)
}

func buildWarnReason(input GuardianInput) string {
	var reasons []string
	if input.EntropyAlert.Level == EntropyYellow {
		reasons = append(reasons,
			fmt.Sprintf("entropy at yellow level (score=%.2f)", input.EntropyAlert.Score))
	}
	if input.DriftResult.DriftScore > 0.3 {
		reasons = append(reasons,
			fmt.Sprintf("drift score elevated (%.2f)", input.DriftResult.DriftScore))
	}
	if !input.TranspAlert.IsHealthy {
		reasons = append(reasons,
			fmt.Sprintf("transparency unhealthy (coverage=%.2f%%, breakpoints=%d, time_anomalies=%d, open_traces=%d)",
				input.TranspAlert.CoverageRate,
				len(input.TranspAlert.BreakPoints),
				len(input.TranspAlert.TimeAnomalies),
				len(input.TranspAlert.OpenTraces),
			))
	}
	return "WARN: " + joinReasons(reasons)
}

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

// ============================================================================
// SECTION 4: AlertAdapter — 告警分发适配器 (Adapter)
// ============================================================================

// AlertChannel represents the type of alert channel.
type AlertChannel string

const (
	AlertChannelLog     AlertChannel = "log"
	AlertChannelWebhook AlertChannel = "webhook"
	AlertChannelChannel AlertChannel = "channel"
)

// AlertResult is the output of the alert adapter.
type AlertResult struct {
	Sent       bool
	Channel    AlertChannel
	Message    string
	Suppressed bool
	Timestamp  time.Time
}

// AlertConfig configures the alert adapter.
type AlertConfig struct {
	Channel          AlertChannel
	WebhookURL       string
	CooldownDuration time.Duration
	AlertCh          chan AlertResult
}

// AlertAdapter implements Adapter[GuardianDecision, AlertResult].
// It is the side-effect boundary for alerting.
type AlertAdapter struct {
	config    AlertConfig
	mu        sync.Mutex
	lastAlert map[string]time.Time
}

func NewAlertAdapter(config AlertConfig) *AlertAdapter {
	return &AlertAdapter{
		config:    config,
		lastAlert: make(map[string]time.Time),
	}
}

// Execute implements Adapter[GuardianDecision, AlertResult].
func (a *AlertAdapter) Execute(ctx context.Context, input GuardianDecision) (AlertResult, error) {
	if ctx.Err() != nil {
		return AlertResult{}, ctx.Err()
	}

	// AllOK: no alert needed
	if input.AllOK {
		return AlertResult{
			Sent:      false,
			Channel:   a.config.Channel,
			Message:   "all clear",
			Timestamp: time.Now(),
		}, nil
	}

	message := a.buildMessage(input)
	actionKey := string(input.Action)

	if a.isInCooldown(actionKey) {
		return AlertResult{
			Sent:       false,
			Channel:    a.config.Channel,
			Message:    message,
			Suppressed: true,
			Timestamp:  time.Now(),
		}, nil
	}

	switch a.config.Channel {
	case AlertChannelLog:
		// Log channel: message already built
	case AlertChannelWebhook:
		message = fmt.Sprintf("[WEBHOOK %s] %s", a.config.WebhookURL, message)
	case AlertChannelChannel:
		if a.config.AlertCh != nil {
			select {
			case a.config.AlertCh <- AlertResult{
				Sent:      true,
				Channel:   a.config.Channel,
				Message:   message,
				Timestamp: time.Now(),
			}:
			default:
				// Channel full; drop alert
			}
		}
	}

	a.recordAlert(actionKey)

	return AlertResult{
		Sent:      true,
		Channel:   a.config.Channel,
		Message:   message,
		Timestamp: time.Now(),
	}, nil
}

func (a *AlertAdapter) buildMessage(input GuardianDecision) string {
	msg := fmt.Sprintf("[%s] %s", input.Action, input.Reason)
	msg += fmt.Sprintf("\n  Entropy: score=%.2f (%s)", input.EntropyAlert.Score, input.EntropyAlert.Level)
	msg += fmt.Sprintf("\n  Transparency: coverage=%.2f%%", input.TranspAlert.CoverageRate)
	msg += fmt.Sprintf("\n  Drift: score=%.4f", input.DriftResult.DriftScore)
	if input.ArchAlert.Violations > 0 {
		msg += fmt.Sprintf("\n  Architecture: %d violations", input.ArchAlert.Violations)
	} else {
		msg += "\n  Architecture: no violations"
	}
	return msg
}

func (a *AlertAdapter) isInCooldown(actionKey string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	lastTime, ok := a.lastAlert[actionKey]
	if !ok {
		return false
	}
	return time.Since(lastTime) < a.config.CooldownDuration
}

func (a *AlertAdapter) recordAlert(actionKey string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.lastAlert[actionKey] = time.Now()
}

var _ Adapter[GuardianDecision, AlertResult] = (*AlertAdapter)(nil)