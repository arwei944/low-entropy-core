//go:build lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

// Package core — 透明度监控 + 行为偏差检测 (v4.0)
//
// 合并自: guardian_transparency.go + guardian_drift.go
//
// 包含:
//   - TransparencyWatcher: 审计链完整性 + 观测覆盖率 + 开放 Trace + 时间戳单调性 (Port)
//   - DriftDetector: 行为偏差检测 (纯函数 Atom)
//   - TransparencyInput / TransparencyAlert: 透明度相关类型
//   - DriftInput / DriftOutput / DriftType: 偏差检测相关类型
//
// DriftDetector 是纯函数，无副作用，无状态，相同输入始终产生相同输出。
package core

import (
	"fmt"
	"math"
	"strings"
)

// ============================================================================
// SECTION 2: DriftDetector — 行为偏差检测 (纯函数 Atom)
// ============================================================================

// DriftInput contains agent behavior data for drift detection.
type DriftInput struct {
	AgentID               string
	CurrentSuccessRate    float64
	BaselineSuccessRate   float64
	CurrentAvgDurationMs  float64
	BaselineAvgDurationMs float64
	CurrentPhase          string
	UsualPhase            string
	ExecutedCapabilities  []string
	AllowedCapabilities   []string
	TotalTasks            int
	SuccessfulTasks       int
}

// DriftType categorizes the type of behavioral drift.
type DriftType string

const (
	DriftNone           DriftType = "none"
	DriftQualityDrop    DriftType = "quality_drop"
	DriftDecisionDev    DriftType = "decision_deviation"
	DriftCapabilityLoss DriftType = "capability_loss"
	DriftOverreach      DriftType = "overreach"
	DriftSlowdown       DriftType = "slowdown"
)

// DriftOutput is the result of drift detection.
type DriftOutput struct {
	AgentID           string
	DriftScore        float64
	DriftType         DriftType
	ReputationScore   float64
	Details           string
	OverreachDetected bool
	ShouldQuarantine  bool
}

// DriftDetector is a pure-function Atom that evaluates behavioral drift.
type DriftDetector struct{}

func NewDriftDetector() *DriftDetector {
	return &DriftDetector{}
}

// Execute runs the drift detection algorithm on the given input.
// This is a pure function: same input always yields the same output.
func (d *DriftDetector) Execute(input DriftInput) DriftOutput {
	if input.TotalTasks == 0 {
		return DriftOutput{
			AgentID:           input.AgentID,
			DriftScore:        0,
			DriftType:         DriftNone,
			ReputationScore:   1.0,
			Details:           "no tasks executed; no drift assessment possible",
			OverreachDetected: false,
			ShouldQuarantine:  false,
		}
	}

	var driftScore float64
	var detailsParts []string

	// Factor 1: Success rate drop (weight 0.35)
	successDrop := 0.0
	if input.BaselineSuccessRate > input.CurrentSuccessRate {
		successDrop = input.BaselineSuccessRate - input.CurrentSuccessRate
		driftScore += successDrop * 0.35
		detailsParts = append(detailsParts,
			fmt.Sprintf("success rate dropped by %.2f (baseline=%.2f, current=%.2f)",
				successDrop, input.BaselineSuccessRate, input.CurrentSuccessRate))
	}

	// Factor 2: Duration increase (weight 0.25)
	durationIncrease := 0.0
	if input.CurrentAvgDurationMs > input.BaselineAvgDurationMs && input.BaselineAvgDurationMs > 0 {
		durationIncrease = (input.CurrentAvgDurationMs - input.BaselineAvgDurationMs) / input.BaselineAvgDurationMs
		driftScore += durationIncrease * 0.25
		detailsParts = append(detailsParts,
			fmt.Sprintf("duration increased by %.0f%% (baseline=%.0fms, current=%.0fms)",
				durationIncrease*100, input.BaselineAvgDurationMs, input.CurrentAvgDurationMs))
	}

	// Factor 3: Phase mismatch (weight 0.15)
	phaseMismatch := input.CurrentPhase != "" && input.UsualPhase != "" &&
		input.CurrentPhase != input.UsualPhase
	if phaseMismatch {
		driftScore += 0.15
		detailsParts = append(detailsParts,
			fmt.Sprintf("phase mismatch: current=%q, usual=%q", input.CurrentPhase, input.UsualPhase))
	}

	// Factor 4: Capability overreach (weight 0.40)
	overreachDetected := detectOverreach(input.ExecutedCapabilities, input.AllowedCapabilities)
	if overreachDetected {
		driftScore += 0.40
		unauthorized := findUnauthorized(input.ExecutedCapabilities, input.AllowedCapabilities)
		detailsParts = append(detailsParts,
			fmt.Sprintf("capability overreach: unauthorized capabilities %v", unauthorized))
	}

	driftScore = clamp(driftScore, 0.0, 1.0)
	driftType := determineDriftType(overreachDetected, input.BaselineSuccessRate,
		input.CurrentSuccessRate, input.BaselineAvgDurationMs,
		input.CurrentAvgDurationMs, phaseMismatch)

	repScore := (float64(input.SuccessfulTasks) / float64(input.TotalTasks)) * (1.0 - driftScore*0.5)
	repScore = clamp(repScore, 0.0, 1.0)
	shouldQuarantine := driftScore > 0.7 || overreachDetected

	details := "no drift detected"
	if len(detailsParts) > 0 {
		details = strings.Join(detailsParts, "; ")
	}

	return DriftOutput{
		AgentID:           input.AgentID,
		DriftScore:        math.Round(driftScore*10000) / 10000,
		DriftType:         driftType,
		ReputationScore:   math.Round(repScore*10000) / 10000,
		Details:           details,
		OverreachDetected: overreachDetected,
		ShouldQuarantine:  shouldQuarantine,
	}
}

// ── Pure helper functions ──

func detectOverreach(executed, allowed []string) bool {
	if len(executed) == 0 {
		return false
	}
	if len(allowed) == 0 {
		return len(executed) > 0
	}
	allowedSet := make(map[string]struct{}, len(allowed))
	for _, c := range allowed {
		allowedSet[c] = struct{}{}
	}
	for _, c := range executed {
		if _, ok := allowedSet[c]; !ok {
			return true
		}
	}
	return false
}

func findUnauthorized(executed, allowed []string) []string {
	if len(allowed) == 0 {
		return executed
	}
	allowedSet := make(map[string]struct{}, len(allowed))
	for _, c := range allowed {
		allowedSet[c] = struct{}{}
	}
	var unauthorized []string
	for _, c := range executed {
		if _, ok := allowedSet[c]; !ok {
			unauthorized = append(unauthorized, c)
		}
	}
	return unauthorized
}

func determineDriftType(
	overreachDetected bool,
	baselineSuccess, currentSuccess float64,
	baselineDuration, currentDuration float64,
	phaseMismatch bool,
) DriftType {
	if overreachDetected {
		return DriftOverreach
	}
	if baselineSuccess > 0 {
		relativeDrop := (baselineSuccess - currentSuccess) / baselineSuccess
		if relativeDrop > 0.30 {
			return DriftQualityDrop
		}
	}
	if baselineDuration > 0 {
		relativeIncrease := (currentDuration - baselineDuration) / baselineDuration
		if relativeIncrease > 2.0 {
			return DriftSlowdown
		}
	}
	if phaseMismatch {
		return DriftDecisionDev
	}
	return DriftNone
}

func clamp(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

// ── Step adapter ──

// DriftDetectorAsStep wraps a DriftDetector as a Step[DriftInput, DriftOutput].
func DriftDetectorAsStep(d *DriftDetector) Step[DriftInput, DriftOutput] {
	return AtomAsStep(Atom[DriftInput, DriftOutput](d.Execute))
}
