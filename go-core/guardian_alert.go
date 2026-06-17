package core

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// ──────────────────────────────────────────────
// AlertChannel — 告警通道类型
// ──────────────────────────────────────────────

// AlertChannel represents the type of alert channel.
type AlertChannel string

const (
	AlertChannelLog     AlertChannel = "log"
	AlertChannelWebhook AlertChannel = "webhook"
	AlertChannelChannel AlertChannel = "channel"
)

// ──────────────────────────────────────────────
// AlertResult — 告警适配器输出
// ──────────────────────────────────────────────

// AlertResult is the output of the alert adapter.
type AlertResult struct {
	Sent       bool
	Channel    AlertChannel
	Message    string
	Suppressed bool      // true if alert was suppressed due to cooldown
	Timestamp  time.Time
}

// ──────────────────────────────────────────────
// AlertConfig — 告警适配器配置
// ──────────────────────────────────────────────

// AlertConfig configures the alert adapter.
type AlertConfig struct {
	Channel          AlertChannel
	WebhookURL       string
	CooldownDuration time.Duration
	AlertCh          chan AlertResult // for channel-based alerting
}

// ──────────────────────────────────────────────
// AlertAdapter — 告警适配器 (TASK-1.6)
// ──────────────────────────────────────────────
//
// AlertAdapter implements Adapter[GuardianDecision, AlertResult].
// It is the side-effect boundary for alerting: it receives a GuardianDecision
// and dispatches alerts through the configured channel (log, webhook, or Go channel).
//
// Cooldown tracking:
//   - Alerts of the same action type (ActionAllow, ActionWarn, ActionBlock, ActionRollback)
//     are suppressed if they occur within CooldownDuration of the last alert of that type.
//   - Different action types are tracked independently — a Block alert does not suppress
//     a Warn alert.
//
// Thread safety: all cooldown state is protected by a mutex; concurrent calls are safe.

// AlertAdapter implements Adapter[GuardianDecision, AlertResult].
type AlertAdapter struct {
	config    AlertConfig
	mu        sync.Mutex
	lastAlert map[string]time.Time // actionKey -> last alert time
}

// NewAlertAdapter creates a new AlertAdapter with the given configuration.
func NewAlertAdapter(config AlertConfig) *AlertAdapter {
	return &AlertAdapter{
		config:    config,
		lastAlert: make(map[string]time.Time),
	}
}

// Execute implements Adapter[GuardianDecision, AlertResult].
//
// Processing steps:
//  1. If input.AllOK is true, skip alerting entirely (Sent=false, Message="all clear").
//  2. Build a formatted alert message from the decision fields.
//  3. Check cooldown: if the same action type was alerted within CooldownDuration,
//     suppress the alert (Suppressed=true).
//  4. Dispatch via the configured channel:
//     - Log: format the message (no actual I/O call).
//     - Webhook: prefix the message with the webhook URL (no actual HTTP call).
//     - Channel: write to the AlertCh Go channel (non-blocking, drops on full channel).
//  5. Record the alert time and return AlertResult with Sent=true.
func (a *AlertAdapter) Execute(ctx context.Context, input GuardianDecision) (AlertResult, error) {
	// Respect context cancellation.
	if ctx.Err() != nil {
		return AlertResult{}, ctx.Err()
	}

	// 1. AllOK: everything is fine, no alert needed.
	if input.AllOK {
		return AlertResult{
			Sent:      false,
			Channel:   a.config.Channel,
			Message:   "all clear",
			Timestamp: time.Now(),
		}, nil
	}

	// 2. Build the alert message from the decision.
	message := a.buildMessage(input)

	// 3. Cooldown check: suppress if the same action type was alerted recently.
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

	// 4. Dispatch via the configured channel.
	switch a.config.Channel {
	case AlertChannelLog:
		// Log channel: just format the message — no actual HTTP call.
		// The message is already built; nothing more to do.

	case AlertChannelWebhook:
		// Webhook channel: format the message with webhook URL mention.
		// No actual HTTP call to avoid external dependencies.
		message = fmt.Sprintf("[WEBHOOK %s] %s", a.config.WebhookURL, message)

	case AlertChannelChannel:
		// Channel channel: write to the configured Go channel.
		// Non-blocking send to avoid stalling the caller.
		if a.config.AlertCh != nil {
			select {
			case a.config.AlertCh <- AlertResult{
				Sent:      true,
				Channel:   a.config.Channel,
				Message:   message,
				Timestamp: time.Now(),
			}:
			default:
				// Channel is full; drop the alert to avoid blocking.
			}
		}
	}

	// 5. Record the alert time for cooldown tracking.
	a.recordAlert(actionKey)

	return AlertResult{
		Sent:      true,
		Channel:   a.config.Channel,
		Message:   message,
		Timestamp: time.Now(),
	}, nil
}

// ──────────────────────────────────────────────
// Internal helpers
// ──────────────────────────────────────────────

// buildMessage constructs a formatted alert message from the guardian decision.
// The message includes: action level, reason, entropy score, transparency coverage,
// drift score, and architecture violations.
func (a *AlertAdapter) buildMessage(input GuardianDecision) string {
	msg := fmt.Sprintf("[%s] %s", input.Action, input.Reason)

	// Entropy score and level.
	msg += fmt.Sprintf("\n  Entropy: score=%.2f (%s)",
		input.EntropyAlert.Score, input.EntropyAlert.Level)

	// Transparency coverage.
	msg += fmt.Sprintf("\n  Transparency: coverage=%.2f%%",
		input.TranspAlert.CoverageRate)

	// Drift score.
	msg += fmt.Sprintf("\n  Drift: score=%.4f",
		input.DriftResult.DriftScore)

	// Architecture violations.
	if input.ArchAlert.Violations > 0 {
		msg += fmt.Sprintf("\n  Architecture: %d violations", input.ArchAlert.Violations)
	} else {
		msg += "\n  Architecture: no violations"
	}

	return msg
}

// isInCooldown checks whether the given action type is still within the cooldown period.
// Returns true if an alert of the same action type was sent within CooldownDuration.
func (a *AlertAdapter) isInCooldown(actionKey string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()

	lastTime, ok := a.lastAlert[actionKey]
	if !ok {
		return false
	}
	return time.Since(lastTime) < a.config.CooldownDuration
}

// recordAlert records the current time as the last alert time for the given action type.
func (a *AlertAdapter) recordAlert(actionKey string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.lastAlert[actionKey] = time.Now()
}

// Compile-time check: AlertAdapter implements Adapter[GuardianDecision, AlertResult].
var _ Adapter[GuardianDecision, AlertResult] = (*AlertAdapter)(nil)