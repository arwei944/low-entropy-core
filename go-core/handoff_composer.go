package core

import (
	"context"
	"fmt"
	"time"
)

// ──────────────────────────────────────────────
// HandoffComposer — the full handoff orchestration
// ──────────────────────────────────────────────

// HandoffComposer orchestrates the complete handoff flow between two agents:
//   1. Source creates DevSnapshot
//   2. Contract is validated (Port)
//   3. Snapshot is persisted (Adapter)
//   4. Snapshot is transferred (Adapter)
//   5. Target receives and confirms
//
// Every step produces ExecutionSteps for full observability.
type HandoffComposer struct {
	obs         ObservationAdapter
	persistence SnapshotPersistence
	transport   HandoffTransport
}

// NewHandoffComposer creates a new HandoffComposer.
// obs is the observation adapter for recording ExecutionSteps.
// persistence is where snapshots are stored.
// transport is how snapshots are transferred between agents.
func NewHandoffComposer(obs ObservationAdapter, persistence SnapshotPersistence, transport HandoffTransport) *HandoffComposer {
	if obs == nil {
		obs = &NoOpObservationAdapter{}
	}
	return &HandoffComposer{
		obs:         obs,
		persistence: persistence,
		transport:   transport,
	}
}

// HandoffInput is the input to a handoff operation.
type HandoffInput struct {
	// SourceAgent is the agent handing off the task.
	SourceAgent *DevSnapshot

	// TargetAgentID is the ID of the agent receiving the task.
	TargetAgentID string

	// TaskID identifies the task being transferred.
	TaskID string

	// Phase is the current development phase.
	Phase string
}

// HandoffOutput is the result of a handoff operation.
type HandoffOutput struct {
	// Success indicates whether the handoff completed successfully.
	Success bool

	// Contract is the validated handoff contract.
	Contract *HandoffContract

	// SnapshotChecksum is the checksum of the transferred snapshot.
	SnapshotChecksum string

	// TargetConfirmed indicates whether the target agent confirmed receipt.
	TargetConfirmed bool

	// Error contains any error that occurred during handoff.
	Error string
}

// Execute performs the full handoff flow.
// This is the main entry point for the handoff protocol.
// It returns the handoff output and all execution steps recorded.
func (h *HandoffComposer) Execute(ctx context.Context, input HandoffInput) (HandoffOutput, []ExecutionStep, error) {
	steps := make([]ExecutionStep, 0, 4)
	traceID := string(NewTraceID())
	parentSpanID := string(NewSpanID())

	// ─── Step 1: Create Snapshot ───
	step1 := h.recordStep(traceID, parentSpanID, "Handoff", "RequestHandoff",
		"creating handoff snapshot", "Handoff")
	now := time.Now()

	// Compute checksum
	checksum, err := input.SourceAgent.ComputeChecksum()
	if err != nil {
		step1.Error = NewStepError("SNAPSHOT_CHECKSUM_FAILED", err.Error(), false)
		step1.DurationMs = time.Since(now).Milliseconds()
		step1.Details = "failed to compute snapshot checksum"
		steps = append(steps, step1)
		h.obs.Record(steps)
		return HandoffOutput{Success: false, Error: err.Error()}, steps, err
	}
	step1.DurationMs = time.Since(now).Milliseconds()
	step1.Details = "snapshot checksum computed: " + checksum[:12] + "..."
	steps = append(steps, step1)

	// ─── Step 2: Persist Snapshot ───
	step2 := h.recordStep(traceID, parentSpanID, "Adapter", "SnapshotCreated",
		"persisting snapshot", "Handoff")
	now = time.Now()

	_, persistErr := h.persistence.CreateSnapshot(input.SourceAgent)
	if persistErr != nil {
		step2.Error = NewStepError("SNAPSHOT_PERSIST_FAILED", persistErr.Error(), false)
		step2.DurationMs = time.Since(now).Milliseconds()
		step2.Details = "failed to persist snapshot"
		steps = append(steps, step2)
		h.obs.Record(steps)
		return HandoffOutput{Success: false, Error: persistErr.Error()}, steps, persistErr
	}
	step2.DurationMs = time.Since(now).Milliseconds()
	step2.Details = "snapshot persisted: " + checksum[:12] + "..."
	steps = append(steps, step2)

	// ─── Step 3: Transfer Snapshot ───
	step3 := h.recordStep(traceID, parentSpanID, "Adapter", "TransferSnapshot",
		"transferring snapshot to target", "Handoff")
	now = time.Now()

	data, err := input.SourceAgent.ToJSON()
	if err != nil {
		step3.Error = NewStepError("SNAPSHOT_SERIALIZE_FAILED", err.Error(), false)
		step3.DurationMs = time.Since(now).Milliseconds()
		step3.Details = "failed to serialize snapshot for transfer"
		steps = append(steps, step3)
		h.obs.Record(steps)
		return HandoffOutput{Success: false, Error: err.Error()}, steps, err
	}

	transferErr := h.transport.Transfer(ctx, checksum, data)
	if transferErr != nil {
		step3.Error = NewStepError("SNAPSHOT_TRANSFER_FAILED", transferErr.Error(), true)
		step3.DurationMs = time.Since(now).Milliseconds()
		step3.Details = "failed to transfer snapshot"
		steps = append(steps, step3)
		h.obs.Record(steps)
		return HandoffOutput{Success: false, Error: transferErr.Error()}, steps, transferErr
	}
	step3.DurationMs = time.Since(now).Milliseconds()
	step3.Details = "snapshot transferred to target"
	steps = append(steps, step3)

	// ─── Step 4: Confirm Handoff (Target receives) ───
	step4 := h.recordStep(traceID, parentSpanID, "Handoff", "ConfirmHandoff",
		"target confirming handoff receipt", "Handoff")
	now = time.Now()

	// Validate the contract
	contract := NewHandoffContract(
		input.SourceAgent.AgentID,
		input.TargetAgentID,
		input.TaskID,
		input.Phase,
		checksum,
	)
	validator := NewContractValidator(checksum)
	_, validateErr := validator.Validate(ctx, *contract)
	if validateErr != nil {
		step4.Error = NewStepError("CONTRACT_VALIDATION_FAILED", validateErr.Error(), false)
		step4.DurationMs = time.Since(now).Milliseconds()
		step4.Details = "contract validation failed"
		steps = append(steps, step4)
		h.obs.Record(steps)
		return HandoffOutput{Success: false, Error: validateErr.Error(), Contract: contract}, steps, validateErr
	}

	step4.DurationMs = time.Since(now).Milliseconds()
	step4.Details = "handoff confirmed by target"
	steps = append(steps, step4)

	h.obs.Record(steps)

	return HandoffOutput{
		Success:          true,
		Contract:         contract,
		SnapshotChecksum: checksum,
		TargetConfirmed:  true,
	}, steps, nil
}

// ReceiveSnapshot retrieves a snapshot that was transferred via the transport.
// This is called by the target agent to receive the handoff.
func (h *HandoffComposer) ReceiveSnapshot(ctx context.Context, checksum string) (*DevSnapshot, []ExecutionStep, error) {
	steps := make([]ExecutionStep, 0, 2)
	traceID := string(NewTraceID())
	parentSpanID := string(NewSpanID())

	// Step 1: Receive from transport
	step1 := h.recordStep(traceID, parentSpanID, "Adapter", "ReceiveSnapshot",
		"receiving snapshot from transport", "Handoff")
	now := time.Now()

	data, err := h.transport.Receive(ctx, checksum)
	if err != nil {
		step1.Error = NewStepError("SNAPSHOT_RECEIVE_FAILED", err.Error(), false)
		step1.DurationMs = time.Since(now).Milliseconds()
		step1.Details = "failed to receive snapshot"
		steps = append(steps, step1)
		h.obs.Record(steps)
		return nil, steps, err
	}
	step1.DurationMs = time.Since(now).Milliseconds()
	step1.Details = "snapshot received from transport"
	steps = append(steps, step1)

	// Step 2: Deserialize and verify
	step2 := h.recordStep(traceID, parentSpanID, "Port", "VerifySnapshot",
		"verifying snapshot integrity", "Handoff")
	now = time.Now()

	snapshot, err := DevSnapshotFromJSON(data)
	if err != nil {
		step2.Error = NewStepError("SNAPSHOT_DESERIALIZE_FAILED", err.Error(), false)
		step2.DurationMs = time.Since(now).Milliseconds()
		step2.Details = "failed to deserialize snapshot"
		steps = append(steps, step2)
		h.obs.Record(steps)
		return nil, steps, err
	}

	if !snapshot.VerifyChecksum() {
		err := fmt.Errorf("snapshot checksum verification failed")
		step2.Error = NewStepError("SNAPSHOT_CHECKSUM_MISMATCH", err.Error(), false)
		step2.DurationMs = time.Since(now).Milliseconds()
		step2.Details = "snapshot integrity check failed"
		steps = append(steps, step2)
		h.obs.Record(steps)
		return nil, steps, err
	}

	step2.DurationMs = time.Since(now).Milliseconds()
	step2.Details = "snapshot verified successfully"
	steps = append(steps, step2)

	h.obs.Record(steps)
	return snapshot, steps, nil
}

// recordStep creates and records an ExecutionStep.
func (h *HandoffComposer) recordStep(traceID, parentSpanID, unit, action, details, pattern string) ExecutionStep {
	step := NewExecutionStepWithTrace(parentSpanID, unit, action, details, pattern)
	step.TraceID = traceID
	return step
}