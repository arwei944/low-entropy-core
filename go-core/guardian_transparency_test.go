package core

import (
	"context"
	"testing"
	"time"
)

func TestTransparencyWatcher_Healthy(t *testing.T) {
	watcher := NewTransparencyWatcher()
	ctx := context.Background()

	now := time.Now()
	// Build a valid hash chain: entry0 → entry1
	entry0 := AuditEntry{
		ID: "entry-0", AgentID: "agent-1", Action: "op1",
		Resource: "res", ResourceID: "r1", Result: "ok",
		Timestamp: now, TraceID: "trace-1",
	}
	entry0.Hash = computeAuditEntryHash(entry0)

	entry1 := AuditEntry{
		ID: "entry-1", AgentID: "agent-1", Action: "op2",
		Resource: "res", ResourceID: "r2", Result: "ok",
		Timestamp: now.Add(1 * time.Second), TraceID: "trace-1",
		PrevHash: entry0.Hash,
	}
	entry1.Hash = computeAuditEntryHash(entry1)

	input := TransparencyInput{
		AuditEntries: []AuditEntry{entry0, entry1},
		ExecutionSteps: []ExecutionStep{
			{Action: "step1"},
			{Action: "step2"},
		},
		ExpectedOperations: 2,
	}

	alert, err := watcher.Validate(ctx, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !alert.IsHealthy {
		t.Errorf("expected healthy, got: %s (breakpoints=%d, anomalies=%d, coverage=%f)",
			alert.Message, len(alert.BreakPoints), len(alert.TimeAnomalies), alert.CoverageRate)
	}
	if alert.CoverageRate != 100.0 {
		t.Errorf("expected 100.0 coverage, got %f", alert.CoverageRate)
	}
}

func TestTransparencyWatcher_Unhealthy(t *testing.T) {
	watcher := NewTransparencyWatcher()
	ctx := context.Background()

	now := time.Now()
	input := TransparencyInput{
		AuditEntries: []AuditEntry{
			{Action: "op1", Timestamp: now},
		},
		ExecutionSteps: []ExecutionStep{
			{Action: "step1"},
			{Action: "step2"},
			{Action: "step3"},
			{Action: "step4"},
		},
		ExpectedOperations: 4,
	}

	alert, err := watcher.Validate(ctx, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if alert.IsHealthy {
		t.Error("expected unhealthy")
	}
	if alert.CoverageRate >= 50.0 {
		t.Errorf("expected low coverage, got %f", alert.CoverageRate)
	}
}

func TestTransparencyWatcher_EmptySteps(t *testing.T) {
	watcher := NewTransparencyWatcher()
	ctx := context.Background()

	input := TransparencyInput{
		AuditEntries:       nil,
		ExecutionSteps:     nil,
		ExpectedOperations: 0,
	}

	alert, err := watcher.Validate(ctx, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if alert.IsHealthy {
		// empty input: 覆盖率 100%（0/0），但可能被标记为 unhealthy
		t.Logf("empty input: healthy=%v, coverage=%f", alert.IsHealthy, alert.CoverageRate)
	}
}

func TestDriftDetector_NoDrift(t *testing.T) {
	detector := NewDriftDetector()

	input := DriftInput{
		AgentID:               "agent-1",
		CurrentSuccessRate:    0.95,
		BaselineSuccessRate:   0.95,
		CurrentAvgDurationMs:  100,
		BaselineAvgDurationMs: 100,
		CurrentPhase:          "execute",
		UsualPhase:            "execute",
		AllowedCapabilities:   []string{"read", "write"},
		ExecutedCapabilities:  []string{"read"},
		TotalTasks:            10,
	}

	output := detector.Execute(input)
	if output.DriftType != DriftNone {
		t.Errorf("expected no drift, got %s", output.DriftType)
	}
}

func TestDriftDetector_PhaseDrift(t *testing.T) {
	detector := NewDriftDetector()

	input := DriftInput{
		AgentID:               "agent-2",
		CurrentSuccessRate:    0.95,
		BaselineSuccessRate:   0.95,
		CurrentAvgDurationMs:  100,
		BaselineAvgDurationMs: 100,
		CurrentPhase:          "deploy",
		UsualPhase:            "execute",
		AllowedCapabilities:   []string{"read", "write"},
		ExecutedCapabilities:  []string{"read"},
		TotalTasks:            10,
	}

	output := detector.Execute(input)
	if output.DriftType == DriftNone {
		t.Error("expected phase drift detected")
	}
}

func TestDriftDetector_CapabilityDrift(t *testing.T) {
	detector := NewDriftDetector()

	input := DriftInput{
		AgentID:               "agent-3",
		CurrentSuccessRate:    0.95,
		BaselineSuccessRate:   0.95,
		CurrentAvgDurationMs:  100,
		BaselineAvgDurationMs: 100,
		CurrentPhase:          "execute",
		UsualPhase:            "execute",
		AllowedCapabilities:   []string{"read"},
		ExecutedCapabilities:  []string{"read", "delete"},
		TotalTasks:            10,
	}

	output := detector.Execute(input)
	if output.DriftType == DriftNone {
		t.Error("expected capability drift detected")
	}
}

func TestDriftDetector_SuccessRateDrift(t *testing.T) {
	detector := NewDriftDetector()

	input := DriftInput{
		AgentID:               "agent-4",
		CurrentSuccessRate:    0.5,
		BaselineSuccessRate:   0.95,
		CurrentAvgDurationMs:  100,
		BaselineAvgDurationMs: 100,
		CurrentPhase:          "execute",
		UsualPhase:            "execute",
		AllowedCapabilities:   []string{"read"},
		ExecutedCapabilities:  []string{"read"},
		TotalTasks:            10,
	}

	output := detector.Execute(input)
	if output.DriftType == DriftNone {
		t.Error("expected success rate drift detected")
	}
}