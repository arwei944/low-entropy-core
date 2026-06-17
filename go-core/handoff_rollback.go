package core

import (
	"context"
	"time"
)

// ──────────────────────────────────────────────
// Handoff Rollback — recovery on failure
// ──────────────────────────────────────────────

// RollbackResult is the outcome of a rollback operation.
type RollbackResult struct {
	// Success indicates whether the rollback was successful.
	Success bool

	// SnapshotChecksum is the checksum of the snapshot that was rolled back.
	SnapshotChecksum string

	// ChecksumMatch indicates whether the restored state matches the original.
	ChecksumMatch bool

	// Error contains any error that occurred during rollback.
	Error string
}

// RollbackHandoff performs a rollback when a handoff fails.
// It restores the source agent's state from the persisted snapshot
// and verifies the checksum to ensure data integrity.
//
// This function is designed to be called when:
//   - Target validation fails
//   - Transport fails during transfer
//   - Contract expires
func RollbackHandoff(ctx context.Context, persistence SnapshotPersistence, transport HandoffTransport, checksum string, obs ObservationAdapter) (RollbackResult, []ExecutionStep, error) {
	if obs == nil {
		obs = &NoOpObservationAdapter{}
	}

	steps := make([]ExecutionStep, 0, 3)
	traceID := string(NewTraceID())
	parentSpanID := string(NewSpanID())

	// ─── Step 1: Load the original snapshot ───
	step1 := NewExecutionStepWithTrace(parentSpanID, "Adapter", "LoadSnapshot",
		"loading original snapshot for rollback", "HandoffRollback")
	step1.TraceID = traceID
	now := time.Now()

	snap := HandoffSnapshot{Token: checksum}
	original, err := persistence.RestoreSnapshot(snap)
	if err != nil {
		step1.Error = NewStepError("ROLLBACK_LOAD_FAILED", err.Error(), false)
		step1.DurationMs = time.Since(now).Milliseconds()
		step1.Details = "failed to load original snapshot"
		steps = append(steps, step1)
		obs.Record(steps)
		return RollbackResult{Success: false, Error: err.Error()}, steps, err
	}
	step1.DurationMs = time.Since(now).Milliseconds()
	step1.Details = "original snapshot loaded"
	steps = append(steps, step1)

	// ─── Step 2: Verify checksum integrity ───
	step2 := NewExecutionStepWithTrace(parentSpanID, "Port", "VerifyChecksum",
		"verifying snapshot integrity during rollback", "HandoffRollback")
	step2.TraceID = traceID
	now = time.Now()

	checksumMatch := original.VerifyChecksum()
	if !checksumMatch {
		step2.Error = NewStepError("ROLLBACK_CHECKSUM_MISMATCH",
			"snapshot checksum verification failed during rollback", false)
		step2.DurationMs = time.Since(now).Milliseconds()
		step2.Details = "checksum mismatch — snapshot may be corrupted"
		steps = append(steps, step2)
		obs.Record(steps)
		return RollbackResult{
			Success:          false,
			SnapshotChecksum: checksum,
			ChecksumMatch:    false,
			Error:            "snapshot checksum verification failed",
		}, steps, nil
	}
	step2.DurationMs = time.Since(now).Milliseconds()
	step2.Details = "checksum verified — state integrity confirmed"
	steps = append(steps, step2)

	// ─── Step 3: Clean up transport (remove transferred snapshot) ───
	step3 := NewExecutionStepWithTrace(parentSpanID, "Adapter", "CleanupTransport",
		"cleaning up transport after rollback", "HandoffRollback")
	step3.TraceID = traceID
	now = time.Now()

	// Attempt to delete the snapshot from the transport layer
	// Not all transports support deletion — failure here is non-fatal
	if delTransport, ok := transport.(interface{ Delete(string) }); ok {
		delTransport.Delete(checksum)
		step3.Details = "transport snapshot cleaned up"
	} else {
		step3.Details = "transport cleanup not supported (non-fatal)"
	}
	step3.DurationMs = time.Since(now).Milliseconds()
	steps = append(steps, step3)

	obs.Record(steps)

	return RollbackResult{
		Success:          true,
		SnapshotChecksum: checksum,
		ChecksumMatch:    true,
	}, steps, nil
}

// ──────────────────────────────────────────────
// HandoffWithRollback — Composer with rollback
// ──────────────────────────────────────────────

// HandoffWithRollback wraps a HandoffComposer with automatic rollback on failure.
// If the handoff fails at any step, the rollback is triggered automatically.
func HandoffWithRollback(ctx context.Context, composer *HandoffComposer, input HandoffInput) (HandoffOutput, []ExecutionStep, *RollbackResult, error) {
	allSteps := make([]ExecutionStep, 0)

	output, steps, err := composer.Execute(ctx, input)
	allSteps = append(allSteps, steps...)

	if err != nil || !output.Success {
		// Attempt rollback
		rollbackResult, rollbackSteps, rollbackErr := RollbackHandoff(
			ctx,
			composer.persistence,
			composer.transport,
			output.SnapshotChecksum,
			composer.obs,
		)
		allSteps = append(allSteps, rollbackSteps...)

		if rollbackErr != nil {
			return output, allSteps, &rollbackResult, rollbackErr
		}
		return output, allSteps, &rollbackResult, err
	}

	return output, allSteps, nil, nil
}