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
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"strings"
	"time"
)

// ============================================================================
// SECTION 1: TransparencyWatcher — 透明度监控 Port
// ============================================================================

// TransparencyInput contains the data needed for transparency checking.
type TransparencyInput struct {
	AuditEntries       []AuditEntry
	ExecutionSteps     []ExecutionStep
	ExpectedOperations int
}

// TransparencyAlert is the output of the transparency watcher.
type TransparencyAlert struct {
	CoverageRate    float64
	OpenTraces      []string
	BreakPoints     []int
	TimeAnomalies   []int
	TotalAuditCount int
	TotalStepCount  int
	IsHealthy       bool
	Message         string
}

// TransparencyWatcher is a Port that monitors transparency.
type TransparencyWatcher struct{}

func NewTransparencyWatcher() *TransparencyWatcher {
	return &TransparencyWatcher{}
}

func (w *TransparencyWatcher) Validate(ctx context.Context, input TransparencyInput) (TransparencyAlert, error) {
	alert := TransparencyAlert{
		TotalAuditCount: len(input.AuditEntries),
		TotalStepCount:  len(input.ExecutionSteps),
	}

	if len(input.AuditEntries) == 0 && len(input.ExecutionSteps) == 0 {
		alert.CoverageRate = 0
		alert.IsHealthy = false
		alert.Message = "empty input: no audit entries or execution steps"
		return alert, nil
	}

	alert.BreakPoints = w.checkChainIntegrity(input.AuditEntries)
	alert.CoverageRate = w.computeCoverageRate(len(input.AuditEntries), input.ExpectedOperations)
	alert.OpenTraces = w.checkOpenTraces(input.ExecutionSteps)
	alert.TimeAnomalies = w.checkTimestampMonotonicity(input.AuditEntries)
	alert.IsHealthy = alert.CoverageRate >= 95 && len(alert.BreakPoints) == 0 && len(alert.TimeAnomalies) == 0

	if alert.IsHealthy {
		alert.Message = fmt.Sprintf("healthy: coverage=%.2f%%, breakpoints=%d, time_anomalies=%d, open_traces=%d",
			alert.CoverageRate, len(alert.BreakPoints), len(alert.TimeAnomalies), len(alert.OpenTraces))
	} else {
		alert.Message = fmt.Sprintf("unhealthy: coverage=%.2f%%, breakpoints=%d, time_anomalies=%d, open_traces=%d",
			alert.CoverageRate, len(alert.BreakPoints), len(alert.TimeAnomalies), len(alert.OpenTraces))
	}

	return alert, nil
}

func (w *TransparencyWatcher) checkChainIntegrity(entries []AuditEntry) []int {
	if len(entries) == 0 {
		return nil
	}

	breakPoints := make([]int, 0)
	for i := range entries {
		computedHash := computeAuditEntryHash(entries[i])
		if computedHash != entries[i].Hash {
			breakPoints = append(breakPoints, i)
			continue
		}
		if i > 0 && entries[i].PrevHash != entries[i-1].Hash {
			breakPoints = append(breakPoints, i)
		}
	}
	return breakPoints
}

func computeAuditEntryHash(entry AuditEntry) string {
	h := sha256.New()
	h.Write([]byte(entry.ID))
	h.Write([]byte(entry.AgentID))
	h.Write([]byte(entry.Action))
	h.Write([]byte(entry.Resource))
	h.Write([]byte(entry.ResourceID))
	h.Write([]byte(entry.Result))
	h.Write([]byte(entry.Timestamp.Format(time.RFC3339Nano)))
	h.Write([]byte(entry.Details))
	h.Write([]byte(entry.TraceID))
	h.Write([]byte(entry.PrevHash))
	return hex.EncodeToString(h.Sum(nil))
}

func (w *TransparencyWatcher) computeCoverageRate(auditCount, expectedOps int) float64 {
	denominator := expectedOps
	if denominator <= 0 {
		denominator = auditCount
	}
	if denominator <= 0 {
		return 0
	}
	rate := float64(auditCount) / float64(denominator) * 100
	if rate > 100 {
		rate = 100
	}
	return rate
}

func (w *TransparencyWatcher) checkOpenTraces(steps []ExecutionStep) []string {
	if len(steps) == 0 {
		return nil
	}

	traceGroups := make(map[string][]ExecutionStep)
	for _, step := range steps {
		if step.TraceID == "" {
			continue
		}
		traceGroups[step.TraceID] = append(traceGroups[step.TraceID], step)
	}

	openTraces := make([]string, 0)
	for traceID, group := range traceGroups {
		if len(group) <= 1 {
			openTraces = append(openTraces, traceID)
			continue
		}

		parentRefs := make(map[string]bool)
		for _, step := range group {
			if step.ParentID != "" {
				parentRefs[step.ParentID] = true
			}
		}

		hasOpen := false
		for _, step := range group {
			if !parentRefs[step.SpanID] {
				hasOpen = true
				break
			}
		}
		if hasOpen {
			openTraces = append(openTraces, traceID)
		}
	}

	return openTraces
}

func (w *TransparencyWatcher) checkTimestampMonotonicity(entries []AuditEntry) []int {
	if len(entries) <= 1 {
		return nil
	}

	anomalies := make([]int, 0)
	for i := 1; i < len(entries); i++ {
		if !entries[i].Timestamp.After(entries[i-1].Timestamp) {
			anomalies = append(anomalies, i)
		}
	}
	return anomalies
}

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