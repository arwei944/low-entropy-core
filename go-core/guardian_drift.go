package core

import (
	"fmt"
	"math"
	"strings"
)

// ──────────────────────────────────────────────
// DriftDetector — 行为偏差检测器 (TASK-1.3)
// ──────────────────────────────────────────────
//
// DriftDetector 是一个 Atom（纯函数），无副作用，无状态。
// 相同输入始终产生相同输出，不依赖 I/O 或共享可变状态。
//
// 它评估 Agent 行为是否偏离了基线，并计算漂移分数、
// 声誉分数以及是否需要隔离。

// DriftInput contains agent behavior data for drift detection.
type DriftInput struct {
	AgentID               string
	CurrentSuccessRate    float64 // 0.0-1.0
	BaselineSuccessRate   float64 // 0.0-1.0
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
	DriftScore        float64 // 0.0-1.0, higher = more drift
	DriftType         DriftType
	ReputationScore   float64 // 0.0-1.0, higher = better reputation
	Details           string
	OverreachDetected bool
	ShouldQuarantine  bool // true if agent should be isolated
}

// DriftDetector is a pure-function Atom that evaluates behavioral drift.
// It has no state and produces deterministic output for the same input.
type DriftDetector struct{}

// NewDriftDetector creates a new DriftDetector.
func NewDriftDetector() *DriftDetector {
	return &DriftDetector{}
}

// Execute runs the drift detection algorithm on the given input.
// This is a pure function: same input always yields the same output.
func (d *DriftDetector) Execute(input DriftInput) DriftOutput {
	// Edge case: zero tasks means no data to evaluate.
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

	// ── Compute drift score from weighted factors ──

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

	// Clamp drift score to [0.0, 1.0]
	driftScore = clamp(driftScore, 0.0, 1.0)

	// ── Determine DriftType ──

	driftType := determineDriftType(overreachDetected, input.BaselineSuccessRate,
		input.CurrentSuccessRate, input.BaselineAvgDurationMs,
		input.CurrentAvgDurationMs, phaseMismatch)

	// ── Compute ReputationScore ──
	// ReputationScore = (successfulTasks / totalTasks) * (1.0 - driftScore * 0.5)
	repScore := (float64(input.SuccessfulTasks) / float64(input.TotalTasks)) * (1.0 - driftScore*0.5)
	repScore = clamp(repScore, 0.0, 1.0)

	// ── Determine ShouldQuarantine ──
	shouldQuarantine := driftScore > 0.7 || overreachDetected

	// ── Build details string ──
	details := "no drift detected"
	if len(detailsParts) > 0 {
		details = strings.Join(detailsParts, "; ")
	}

	return DriftOutput{
		AgentID:           input.AgentID,
		DriftScore:        math.Round(driftScore*10000) / 10000, // round to 4 decimal places
		DriftType:         driftType,
		ReputationScore:   math.Round(repScore*10000) / 10000,
		Details:           details,
		OverreachDetected: overreachDetected,
		ShouldQuarantine:  shouldQuarantine,
	}
}

// ── Pure helper functions ──

// detectOverreach returns true if any executed capability is not in the allowed list.
// If allowed list is empty, all executed capabilities are considered unauthorized.
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

// findUnauthorized returns the list of executed capabilities not in the allowed set.
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

// determineDriftType classifies the type of drift based on priority order.
// Priority: Overreach > QualityDrop > Slowdown > DecisionDev > None
func determineDriftType(
	overreachDetected bool,
	baselineSuccess, currentSuccess float64,
	baselineDuration, currentDuration float64,
	phaseMismatch bool,
) DriftType {
	if overreachDetected {
		return DriftOverreach
	}
	// Success rate dropped > 30% relative to baseline
	if baselineSuccess > 0 {
		relativeDrop := (baselineSuccess - currentSuccess) / baselineSuccess
		if relativeDrop > 0.30 {
			return DriftQualityDrop
		}
	}
	// Duration increased > 200% relative to baseline
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

// clamp restricts a value to the inclusive range [min, max].
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

// DriftDetectorAsStep wraps a DriftDetector as a Step[DriftInput, DriftOutput]
// for uniform composition in the pipeline.
func DriftDetectorAsStep(d *DriftDetector) Step[DriftInput, DriftOutput] {
	return AtomAsStep(Atom[DriftInput, DriftOutput](d.Execute))
}