package core

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

// ──────────────────────────────────────────────
// TransparencyInput — input for transparency checking
// ──────────────────────────────────────────────

// TransparencyInput contains the data needed for transparency checking.
type TransparencyInput struct {
	// AuditEntries is the list of audit entries to check.
	AuditEntries []AuditEntry
	// ExecutionSteps is the list of execution steps to check.
	ExecutionSteps []ExecutionStep
	// ExpectedOperations is the number of operations that should have audit records.
	ExpectedOperations int
}

// ──────────────────────────────────────────────
// TransparencyAlert — output of the transparency watcher
// ──────────────────────────────────────────────

// TransparencyAlert is the output of the transparency watcher.
type TransparencyAlert struct {
	CoverageRate    float64  // percentage of operations with audit records
	OpenTraces      []string // TraceIDs with no matching end record
	BreakPoints     []int    // indices where audit chain is broken
	TimeAnomalies   []int    // indices where timestamp is out of order
	TotalAuditCount int
	TotalStepCount  int
	IsHealthy       bool
	Message         string
}

// ──────────────────────────────────────────────
// TransparencyWatcher — Port[TransparencyInput, TransparencyAlert]
// ──────────────────────────────────────────────

// TransparencyWatcher is a Port that monitors transparency by checking
// audit chain integrity, observation coverage, open traces, and
// timestamp monotonicity.
//
// It implements Port[TransparencyInput, TransparencyAlert].
type TransparencyWatcher struct{}

// NewTransparencyWatcher creates a new TransparencyWatcher.
func NewTransparencyWatcher() *TransparencyWatcher {
	return &TransparencyWatcher{}
}

// Validate implements Port[TransparencyInput, TransparencyAlert].
//
// It performs the following checks:
//  1. Audit chain integrity: verifies Hash consistency and PrevHash chaining.
//  2. Observation coverage: compares AuditEntries count vs ExpectedOperations.
//  3. Open traces: finds TraceIDs with incomplete span trees.
//  4. Timestamp monotonicity: verifies AuditEntry timestamps are ascending.
//
// Empty input returns CoverageRate=0, IsHealthy=false without panicking.
func (w *TransparencyWatcher) Validate(ctx context.Context, input TransparencyInput) (TransparencyAlert, error) {
	alert := TransparencyAlert{
		TotalAuditCount: len(input.AuditEntries),
		TotalStepCount:  len(input.ExecutionSteps),
	}

	// Empty input check: both AuditEntries and ExecutionSteps are empty.
	if len(input.AuditEntries) == 0 && len(input.ExecutionSteps) == 0 {
		alert.CoverageRate = 0
		alert.IsHealthy = false
		alert.Message = "empty input: no audit entries or execution steps"
		return alert, nil
	}

	// 1. Check audit chain integrity.
	alert.BreakPoints = w.checkChainIntegrity(input.AuditEntries)

	// 2. Check observation coverage.
	alert.CoverageRate = w.computeCoverageRate(len(input.AuditEntries), input.ExpectedOperations)

	// 3. Check open traces.
	alert.OpenTraces = w.checkOpenTraces(input.ExecutionSteps)

	// 4. Check timestamp monotonicity.
	alert.TimeAnomalies = w.checkTimestampMonotonicity(input.AuditEntries)

	// 5. Determine health.
	alert.IsHealthy = alert.CoverageRate >= 95 && len(alert.BreakPoints) == 0 && len(alert.TimeAnomalies) == 0

	// 6. Build message.
	if alert.IsHealthy {
		alert.Message = fmt.Sprintf("healthy: coverage=%.2f%%, breakpoints=%d, time_anomalies=%d, open_traces=%d",
			alert.CoverageRate, len(alert.BreakPoints), len(alert.TimeAnomalies), len(alert.OpenTraces))
	} else {
		alert.Message = fmt.Sprintf("unhealthy: coverage=%.2f%%, breakpoints=%d, time_anomalies=%d, open_traces=%d",
			alert.CoverageRate, len(alert.BreakPoints), len(alert.TimeAnomalies), len(alert.OpenTraces))
	}

	return alert, nil
}

// ──────────────────────────────────────────────
// Internal check methods
// ──────────────────────────────────────────────

// checkChainIntegrity verifies the audit chain integrity.
// For each entry, it verifies that the Hash field is consistent with the
// entry's content. It also checks that PrevHash correctly chains to the
// previous entry's Hash.
//
// Returns the indices of entries where the chain is broken.
func (w *TransparencyWatcher) checkChainIntegrity(entries []AuditEntry) []int {
	if len(entries) == 0 {
		return nil
	}

	breakPoints := make([]int, 0)
	for i := range entries {
		// Verify hash consistency: recompute hash from content and compare.
		computedHash := computeAuditEntryHash(entries[i])
		if computedHash != entries[i].Hash {
			breakPoints = append(breakPoints, i)
			continue
		}

		// Verify PrevHash chaining: entry[i].PrevHash must equal entry[i-1].Hash.
		if i > 0 && entries[i].PrevHash != entries[i-1].Hash {
			breakPoints = append(breakPoints, i)
		}
	}
	return breakPoints
}

// computeAuditEntryHash computes a SHA-256 hash of the audit entry's content.
// The hash covers all content fields of the entry, excluding Hash itself.
// PrevHash is included since it is part of the entry's content.
func computeAuditEntryHash(entry AuditEntry) string {
	h := sha256.New()

	// Write all content fields in a deterministic order.
	// Note: ResourceID and TraceID are part of the AuditEntry struct
	// but are not listed in the task description; we include them
	// for completeness since they are part of the entry's content.
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

// computeCoverageRate calculates the coverage rate as a percentage.
// CoverageRate = (auditCount / expectedOps) * 100, capped at 100.
// If expectedOps is zero or negative, auditCount is used as the denominator,
// yielding 100% when auditCount > 0, or 0% when auditCount is also 0.
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

// checkOpenTraces finds TraceIDs representing incomplete (open) traces.
// An open trace is one where at least one step has no child step within
// the same trace — i.e., no other step in the same TraceID references
// this step's SpanID as its ParentID. Single-step traces are also
// considered open.
//
// Steps with empty TraceID are skipped.
func (w *TransparencyWatcher) checkOpenTraces(steps []ExecutionStep) []string {
	if len(steps) == 0 {
		return nil
	}

	// Group steps by TraceID.
	traceGroups := make(map[string][]ExecutionStep)
	for _, step := range steps {
		if step.TraceID == "" {
			continue
		}
		traceGroups[step.TraceID] = append(traceGroups[step.TraceID], step)
	}

	openTraces := make([]string, 0)
	for traceID, group := range traceGroups {
		// Single-step traces are always open.
		if len(group) <= 1 {
			openTraces = append(openTraces, traceID)
			continue
		}

		// Build a set of SpanIDs that are referenced as ParentID by some step.
		parentRefs := make(map[string]bool)
		for _, step := range group {
			if step.ParentID != "" {
				parentRefs[step.ParentID] = true
			}
		}

		// Check if any step's SpanID is NOT referenced as a parent.
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

// checkTimestampMonotonicity verifies that audit entry timestamps are in
// strictly ascending order. Returns the indices of entries whose timestamp
// is not greater than the previous entry's timestamp (i.e., out of order).
func (w *TransparencyWatcher) checkTimestampMonotonicity(entries []AuditEntry) []int {
	if len(entries) <= 1 {
		return nil
	}

	anomalies := make([]int, 0)
	for i := 1; i < len(entries); i++ {
		// Timestamp must be strictly after the previous entry's timestamp.
		if !entries[i].Timestamp.After(entries[i-1].Timestamp) {
			anomalies = append(anomalies, i)
		}
	}
	return anomalies
}