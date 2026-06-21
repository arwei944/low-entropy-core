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
