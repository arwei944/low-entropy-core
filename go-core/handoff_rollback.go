//go:build lecore_tier3 || lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

package core

import (
	"context"
)

// ============================================================================
// SECTION 5: Rollback
// ============================================================================

type RollbackResult struct {
	Success          bool
	SnapshotChecksum string
	ChecksumMatch    bool
	Error            string
}

func RollbackHandoff(ctx context.Context, persistence SnapshotPersistence, transport HandoffTransport, checksum string, obs ObservationAdapter) (RollbackResult, []ExecutionStep, error) {
	if obs == nil {
		obs = &NoOpObservationAdapter{}
	}
	steps := make([]ExecutionStep, 0, 3)
	traceID := string(NewTraceID())
	parentSpanID := string(NewSpanID())

	step1 := NewExecutionStepWithTrace(parentSpanID, "Adapter", "LoadSnapshot", "loading original snapshot for rollback", "HandoffRollback")
	step1.TraceID = traceID
	now := time.Now()
	snap := HandoffSnapshot{Token: checksum}
	original, err := persistence.RestoreSnapshot(snap)
	if err != nil {
		step1.Error = NewStepError("ROLLBACK_LOAD_FAILED", err.Error(), false)
		step1.DurationMs = time.Since(now).Milliseconds()
		steps = append(steps, step1)
		obs.Record(steps)
		return RollbackResult{Success: false, Error: err.Error()}, steps, err
	}
	step1.DurationMs = time.Since(now).Milliseconds()
	steps = append(steps, step1)

	step2 := NewExecutionStepWithTrace(parentSpanID, "Port", "VerifyChecksum", "verifying snapshot integrity during rollback", "HandoffRollback")
	step2.TraceID = traceID
	now = time.Now()
	checksumMatch := original.VerifyChecksum()
	if !checksumMatch {
		step2.Error = NewStepError("ROLLBACK_CHECKSUM_MISMATCH", "snapshot checksum verification failed", false)
		step2.DurationMs = time.Since(now).Milliseconds()
		steps = append(steps, step2)
		obs.Record(steps)
		return RollbackResult{Success: false, SnapshotChecksum: checksum, ChecksumMatch: false, Error: "snapshot checksum verification failed"}, steps, nil
	}
	step2.DurationMs = time.Since(now).Milliseconds()
	steps = append(steps, step2)

	step3 := NewExecutionStepWithTrace(parentSpanID, "Adapter", "CleanupTransport", "cleaning up transport after rollback", "HandoffRollback")
	step3.TraceID = traceID
	now = time.Now()
	if delTransport, ok := transport.(interface{ Delete(string) }); ok {
		delTransport.Delete(checksum)
		step3.Details = "transport snapshot cleaned up"
	} else {
		step3.Details = "transport cleanup not supported (non-fatal)"
	}
	step3.DurationMs = time.Since(now).Milliseconds()
	steps = append(steps, step3)
	obs.Record(steps)

	return RollbackResult{Success: true, SnapshotChecksum: checksum, ChecksumMatch: true}, steps, nil
}

func HandoffWithRollback(ctx context.Context, composer *HandoffComposer, input HandoffInput) (HandoffOutput, []ExecutionStep, *RollbackResult, error) {
	allSteps := make([]ExecutionStep, 0)
	output, steps, err := composer.Execute(ctx, input)
	allSteps = append(allSteps, steps...)
	if err != nil || !output.Success {
		rollbackResult, rollbackSteps, rollbackErr := RollbackHandoff(ctx, composer.persistence, composer.transport, output.SnapshotChecksum, composer.obs)
		allSteps = append(allSteps, rollbackSteps...)
		if rollbackErr != nil {
			return output, allSteps, &rollbackResult, rollbackErr
		}
		return output, allSteps, &rollbackResult, err
	}
	return output, allSteps, nil, nil
}
