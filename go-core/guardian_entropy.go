package core

import (
	"context"
	"fmt"
	"math"
	"sync/atomic"
	"time"
)

// ──────────────────────────────────────────────
// EntropyWatcher — entropy monitoring port
// ──────────────────────────────────────────────

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
	Thresholds           map[string]bool // which thresholds were exceeded
	Message              string
	Timestamp            time.Time
}

// EntropyWatcher implements Port[EntropySnapshot, EntropyAlert].
// It monitors incoming entropy snapshots, compares scores against configurable
// thresholds, and emits alerts when thresholds are exceeded. It also tracks
// the previous score to detect acceleration (rapid entropy increase).
//
// v4.0: 使用 atomic 操作替代 sync.Mutex，热路径无锁。
// previousScore 使用 atomic.Uint64 存储 float64 的位表示，hasPrevious 使用 atomic.Bool。
type EntropyWatcher struct {
	previousScore   atomic.Uint64 // math.Float64bits(score)
	hasPrevious     atomic.Bool
	thresholdYellow  float64
	thresholdOrange  float64
	thresholdRed     float64
}

// loadPreviousScore 原子读取上一次的熵值分数。
func (w *EntropyWatcher) loadPreviousScore() (float64, bool) {
	if !w.hasPrevious.Load() {
		return 0, false
	}
	return math.Float64frombits(w.previousScore.Load()), true
}

// storePreviousScore 原子存储当前的熵值分数。
func (w *EntropyWatcher) storePreviousScore(score float64) {
	w.previousScore.Store(math.Float64bits(score))
	w.hasPrevious.Store(true)
}

// NewEntropyWatcher creates a new EntropyWatcher with default thresholds.
// Defaults: Yellow=20, Orange=50, Red=100.
func NewEntropyWatcher() *EntropyWatcher {
	return &EntropyWatcher{
		thresholdYellow: 20,
		thresholdOrange: 50,
		thresholdRed:    100,
	}
}

// NewEntropyWatcherWithThresholds creates a new EntropyWatcher with custom thresholds.
func NewEntropyWatcherWithThresholds(yellow, orange, red float64) *EntropyWatcher {
	return &EntropyWatcher{
		thresholdYellow: yellow,
		thresholdOrange: orange,
		thresholdRed:    red,
	}
}

// Validate checks the entropy snapshot against configured thresholds and
// returns an alert. It is thread-safe using atomic operations.
// v4.0: 使用 atomic 操作替代 sync.Mutex，无锁热路径。
func (w *EntropyWatcher) Validate(ctx context.Context, input EntropySnapshot) (EntropyAlert, error) {
	if ctx.Err() != nil {
		return EntropyAlert{}, ctx.Err()
	}

	score := input.EntropyScore
	alert := EntropyAlert{
		Score:     score,
		Timestamp: input.Timestamp,
	}

	// Carry forward the previous score for transparency.
	prevScore, hasPrev := w.loadPreviousScore()
	if hasPrev {
		alert.PreviousScore = prevScore
	}

	// Determine which thresholds are exceeded.
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

	// Detect acceleration: score increased compared to the previous call.
	if hasPrev && score > prevScore {
		alert.AccelerationDetected = true
	}

	// Build a human-readable message.
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
		alert.Message = fmt.Sprintf(
			"Entropy score %.2f is within normal range.", score,
		)
	}

	if alert.AccelerationDetected {
		alert.Message += fmt.Sprintf(
			" Acceleration detected: score increased from %.2f to %.2f.",
			prevScore, score,
		)
	}

	// Update internal state for the next call.
	w.storePreviousScore(score)

	return alert, nil
}

// Ensure EntropyWatcher satisfies the Port interface at compile time.
var _ Port[EntropySnapshot, EntropyAlert] = (*EntropyWatcher)(nil)