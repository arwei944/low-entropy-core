//go:build lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

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
// SECTION 1: Guardian 核心类型
// ============================================================================

// GuardianAction 表示 Guardian 的决策动作。
type GuardianAction string

const (
	ActionAllow    GuardianAction = "Allow"
	ActionWarn     GuardianAction = "Warn"
	ActionBlock    GuardianAction = "Block"
	ActionRollback GuardianAction = "Rollback"
)

// TransparencyAlert 表示可观测性/透明度检查结果。
type TransparencyAlert struct {
	IsHealthy       bool
	CoverageRate    float64
	TotalAuditCount int
	Message         string
	Timestamp       time.Time
}

// GuardianInput 是 DecisionEngine 的输入，聚合四个维度的检查结果。
type GuardianInput struct {
	EntropyAlert       EntropyAlert
	TranspAlert        TransparencyAlert
	DriftResult        DriftOutput
	ArchAlert          ArchitectureAlert
	StaticReviewResult StaticReviewResult
}

// GuardianDecision 是 DecisionEngine 的输出，包含综合决策。
type GuardianDecision struct {
	Action GuardianAction
	AllOK  bool
	Reason string
	// 保留原始输入中的子警报，供 AlertAdapter 构建消息时使用。
	EntropyAlert EntropyAlert
	TranspAlert  TransparencyAlert
	DriftResult  DriftOutput
	ArchAlert    ArchitectureAlert
}

// ============================================================================
// SECTION 2: DecisionEngine — 四维度监督决策 Composer
// ============================================================================

// DecisionEngine 综合四个维度的监督结果，输出 GuardianDecision。
// 实现 Composer[GuardianInput]。
type DecisionEngine struct {
	mu  sync.RWMutex
	obs ObservationAdapter
}

func NewDecisionEngine(obs ObservationAdapter) *DecisionEngine {
	return &DecisionEngine{obs: obs}
}

// Run 实现 Composer[GuardianInput]。
// 记录执行步骤并调用内部 evaluate。
func (e *DecisionEngine) Run(ctx context.Context, input GuardianInput) (GuardianDecision, []ExecutionStep, error) {
	if ctx.Err() != nil {
		return GuardianDecision{}, nil, ctx.Err()
	}

	decision := e.evaluate(input)

	steps := []ExecutionStep{{
		Unit:    "DecisionEngine",
		Action:  string(decision.Action),
		Details: decision.Reason,
	}}

	if e.obs != nil {
		e.obs.Record(steps)
	}

	return decision, steps, nil
}

// evaluate 是决策核心逻辑，按优先级从高到低判定。
func (e *DecisionEngine) evaluate(input GuardianInput) GuardianDecision {
	// 规则 1: Rollback — entropy Red 或 ShouldQuarantine 或 static errors >= 3
	if input.EntropyAlert.Level == EntropyRed || input.DriftResult.ShouldQuarantine {
		return GuardianDecision{
			Action:       ActionRollback,
			AllOK:        false,
			Reason:       fmt.Sprintf("rollback: entropy=%s drift_quarantine=%v", input.EntropyAlert.Level, input.DriftResult.ShouldQuarantine),
			EntropyAlert: input.EntropyAlert,
			TranspAlert:  input.TranspAlert,
			DriftResult:  input.DriftResult,
			ArchAlert:    input.ArchAlert,
		}
	}

	// 统计静态审核的 error 数量
	staticErrorCount := 0
	for _, v := range input.StaticReviewResult.Violations {
		if v.Severity == "error" {
			staticErrorCount++
		}
	}
	if staticErrorCount >= 3 {
		return GuardianDecision{
			Action:       ActionRollback,
			AllOK:        false,
			Reason:       fmt.Sprintf("rollback: %d static review errors (>=3)", staticErrorCount),
			EntropyAlert: input.EntropyAlert,
			TranspAlert:  input.TranspAlert,
			DriftResult:  input.DriftResult,
			ArchAlert:    input.ArchAlert,
		}
	}

	// 规则 2: Block — entropy Orange 或 drift score > 0.7 或 有静态 error
	if input.EntropyAlert.Level == EntropyOrange || input.DriftResult.DriftScore > 0.7 || staticErrorCount >= 1 {
		return GuardianDecision{
			Action:       ActionBlock,
			AllOK:        false,
			Reason:       fmt.Sprintf("block: entropy=%s drift=%.2f static_errors=%d", input.EntropyAlert.Level, input.DriftResult.DriftScore, staticErrorCount),
			EntropyAlert: input.EntropyAlert,
			TranspAlert:  input.TranspAlert,
			DriftResult:  input.DriftResult,
			ArchAlert:    input.ArchAlert,
		}
	}

	// 规则 3: Warn — entropy Yellow 或 drift score > 0.3 或 transparency 不健康
	if input.EntropyAlert.Level == EntropyYellow || input.DriftResult.DriftScore > 0.3 || !input.TranspAlert.IsHealthy {
		return GuardianDecision{
			Action:       ActionWarn,
			AllOK:        false,
			Reason:       fmt.Sprintf("warn: entropy=%s drift=%.2f transp_healthy=%v", input.EntropyAlert.Level, input.DriftResult.DriftScore, input.TranspAlert.IsHealthy),
			EntropyAlert: input.EntropyAlert,
			TranspAlert:  input.TranspAlert,
			DriftResult:  input.DriftResult,
			ArchAlert:    input.ArchAlert,
		}
	}

	// 规则 4: Allow — 全部通过
	return GuardianDecision{
		Action:       ActionAllow,
		AllOK:        true,
		Reason:       "allow: all checks passed",
		EntropyAlert: input.EntropyAlert,
		TranspAlert:  input.TranspAlert,
		DriftResult:  input.DriftResult,
		ArchAlert:    input.ArchAlert,
	}
}

// ============================================================================
// SECTION 3: AlertAdapter — 告警分发适配器 (Adapter)
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
