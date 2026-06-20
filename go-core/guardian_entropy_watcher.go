//go:build lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

// Package core — 监督层熵值监控 (v4.0)
//
// SECTION 1: EntropyWatcher — 全局熵值监控 Port
package core

import (
	"context"
	"fmt"
	"math"
	"sync/atomic"
	"time"
)

// ============================================================================
// SECTION 1: EntropyWatcher — 全局熵值监控 Port
// ============================================================================

// EntropyLevel represents the severity of entropy alert.
type EntropyLevel string

const (
	EntropyOK     EntropyLevel = "OK"
	EntropyYellow EntropyLevel = "Yellow"
	EntropyOrange EntropyLevel = "Orange"
	EntropyRed    EntropyLevel = "Red"
)

// EntropyAlert is the output of the entropy watcher.
type EntropyAlert struct {
	Level                EntropyLevel
	Score                float64
	PreviousScore        float64
	AccelerationDetected bool
	Thresholds           map[string]bool
	Message              string
	Timestamp            time.Time
}

// EntropyWatcher implements Port[EntropySnapshot, EntropyAlert].
// v4.0: 使用 atomic 操作替代 sync.Mutex，热路径无锁。
type EntropyWatcher struct {
	previousScore    atomic.Uint64
	hasPrevious      atomic.Bool
	thresholdYellow  float64
	thresholdOrange  float64
	thresholdRed     float64
}

func (w *EntropyWatcher) loadPreviousScore() (float64, bool) {
	if !w.hasPrevious.Load() {
		return 0, false
	}
	return math.Float64frombits(w.previousScore.Load()), true
}

func (w *EntropyWatcher) storePreviousScore(score float64) {
	w.previousScore.Store(math.Float64bits(score))
	w.hasPrevious.Store(true)
}

func NewEntropyWatcher() *EntropyWatcher {
	return &EntropyWatcher{
		thresholdYellow: 20,
		thresholdOrange: 50,
		thresholdRed:    100,
	}
}

func NewEntropyWatcherWithThresholds(yellow, orange, red float64) *EntropyWatcher {
	return &EntropyWatcher{
		thresholdYellow: yellow,
		thresholdOrange: orange,
		thresholdRed:    red,
	}
}

func (w *EntropyWatcher) Validate(ctx context.Context, input EntropySnapshot) (EntropyAlert, error) {
	if ctx.Err() != nil {
		return EntropyAlert{}, ctx.Err()
	}

	score := input.EntropyScore
	alert := EntropyAlert{
		Score:     score,
		Timestamp: input.Timestamp,
	}

	prevScore, hasPrev := w.loadPreviousScore()
	if hasPrev {
		alert.PreviousScore = prevScore
	}

	thresholds := make(map[string]bool)
	switch {
	case score >= w.thresholdRed:
		alert.Level = EntropyRed
		thresholds["red"] = true
		thresholds["orange"] = true
		thresholds["yellow"] = true
	case score >= w.thresholdOrange:
		alert.Level = EntropyOrange
		thresholds["orange"] = true
		thresholds["yellow"] = true
	case score >= w.thresholdYellow:
		alert.Level = EntropyYellow
		thresholds["yellow"] = true
	default:
		alert.Level = EntropyOK
	}
	alert.Thresholds = thresholds

	if hasPrev && score > prevScore {
		alert.AccelerationDetected = true
	}

	switch alert.Level {
	case EntropyRed:
		alert.Message = fmt.Sprintf(
			"CRITICAL: entropy score %.2f exceeds red threshold (%.2f). Immediate action required.",
			score, w.thresholdRed,
		)
	case EntropyOrange:
		alert.Message = fmt.Sprintf(
			"WARNING: entropy score %.2f exceeds orange threshold (%.2f). Investigation recommended.",
			score, w.thresholdOrange,
		)
	case EntropyYellow:
		alert.Message = fmt.Sprintf(
			"NOTICE: entropy score %.2f exceeds yellow threshold (%.2f). Monitor closely.",
			score, w.thresholdYellow,
		)
	default:
		alert.Message = fmt.Sprintf("Entropy score %.2f is within normal range.", score)
	}

	if alert.AccelerationDetected {
		alert.Message += fmt.Sprintf(
			" Acceleration detected: score increased from %.2f to %.2f.",
			prevScore, score,
		)
	}

	w.storePreviousScore(score)
	return alert, nil
}

var _ Port[EntropySnapshot, EntropyAlert] = (*EntropyWatcher)(nil)
