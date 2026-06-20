//go:build lecore_tier3 || lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

package core

import (
	"context"
	"fmt"
	"time"
)

// ============================================================================
// NewHandoff + handoffComposer + HandoffComposer + Execute + ReceiveSnapshot
// ============================================================================

// NewHandoff 创建 Agent 交接 Composer。
// 遵守 docs/upgrade/03-agent-handoff-protocol.md 协议：
//   - 产生 Pattern: "Handoff" 的 ExecutionStep
//   - 调用 ObservationAdapter 记录所有步骤
//   - source Composer 的结果作为 snapshot 的输入
//   - target Composer 的执行步骤被完整收集
func NewHandoff(source, target Composer[any], snapshot SnapshotAdapter[any], transport TransportFunc, obs ObservationAdapter) Composer[any] {
	if obs == nil {
		obs = &NoOpObservationAdapter{}
	}
	return &handoffComposer[any]{
		source:    source,
		target:    target,
		snapshot:  snapshot,
		transport: transport,
		obs:       obs,
	}
}

type handoffComposer[T any] struct {
	source    Composer[T]
	target    Composer[T]
	snapshot  SnapshotAdapter[T]
	transport TransportFunc
	obs       ObservationAdapter
}

func (h *handoffComposer[T]) Run(ctx context.Context, input T) (T, []ExecutionStep, error) {
	steps := make([]ExecutionStep, 0, 4)
	traceID := string(NewTraceID())
	parentSpanID := string(NewSpanID())

	// 提取 HandoffRequest 的 Payload 作为 source 的输入
	req, ok := any(input).(HandoffRequest)
	if !ok {
		return input, nil, NewStepError("HANDOFF_INVALID_INPUT", "input must be HandoffRequest", false)
	}
	sourceInput := any(req.Payload).(T)

	// Step 1: 执行 source Composer
	step1 := NewExecutionStepWithTrace(parentSpanID, "Handoff", "SourceComposer", "executing source composer", "Handoff")
	step1.TraceID = traceID
	now := time.Now()
	sourceResult, sourceSteps, sourceErr := h.source.Run(ctx, sourceInput)
	step1.DurationMs = time.Since(now).Milliseconds()
	if sourceErr != nil {
		step1.Error = NewStepError("SOURCE_FAILED", sourceErr.Error(), false)
		steps = append(steps, step1)
		steps = append(steps, sourceSteps...)
		h.obs.Record(steps)
		return input, steps, sourceErr
	}
	steps = append(steps, step1)
	steps = append(steps, sourceSteps...)

	// Step 2: 创建快照
	step2 := NewExecutionStepWithTrace(parentSpanID, "Handoff", "CreateSnapshot", "creating handoff snapshot", "Handoff")
	step2.TraceID = traceID
	now = time.Now()
	snap := h.snapshot.CreateSnapshot(sourceResult)
	step2.DurationMs = time.Since(now).Milliseconds()
	steps = append(steps, step2)

	// Step 3: 传输快照
	step3 := NewExecutionStepWithTrace(parentSpanID, "Handoff", "TransportSnapshot", "transporting snapshot", "Handoff")
	step3.TraceID = traceID
	now = time.Now()
	transported := h.transport(snap)
	_ = transported
	step3.DurationMs = time.Since(now).Milliseconds()
	steps = append(steps, step3)

	// Step 4: 恢复并执行 target
	step4 := NewExecutionStepWithTrace(parentSpanID, "Handoff", "RestoreAndExecute", "restoring snapshot and executing target", "Handoff")
	step4.TraceID = traceID
	now = time.Now()
	state, restoreErr := h.snapshot.RestoreSnapshot(snap)
	if restoreErr != nil {
		step4.Error = NewStepError("RESTORE_FAILED", restoreErr.Error(), false)
		step4.DurationMs = time.Since(now).Milliseconds()
		steps = append(steps, step4)
		h.obs.Record(steps)
		return input, steps, restoreErr
	}
	targetResult, targetSteps, targetErr := h.target.Run(ctx, state)
	step4.DurationMs = time.Since(now).Milliseconds()
	if targetErr != nil {
		step4.Error = NewStepError("TARGET_FAILED", targetErr.Error(), false)
		steps = append(steps, step4)
		steps = append(steps, targetSteps...)
		h.obs.Record(steps)
		_ = targetResult
		return input, steps, targetErr
	}
	steps = append(steps, step4)
	steps = append(steps, targetSteps...)

	h.obs.Record(steps)
	// 返回 HandoffResult 而非 raw input
	result := any(HandoffResult{Success: true, Token: req.Token}).(T)
	return result, steps, nil
}

// ============================================================================
// SECTION 4: HandoffComposer — full handoff orchestration
// ============================================================================

type HandoffComposer struct {
	obs         ObservationAdapter
	persistence SnapshotPersistence
	transport   HandoffTransport
}

type HandoffInput struct {
	SourceAgent   *DevSnapshot
	TargetAgentID string
	TaskID        string
	Phase         string
}

type HandoffOutput struct {
	Success          bool
	Contract         *HandoffContract
	SnapshotChecksum string
	TargetConfirmed  bool
	Error            string
}

func NewHandoffComposer(obs ObservationAdapter, persistence SnapshotPersistence, transport HandoffTransport) *HandoffComposer {
	if obs == nil {
		obs = &NoOpObservationAdapter{}
	}
	return &HandoffComposer{obs: obs, persistence: persistence, transport: transport}
}

func (h *HandoffComposer) Execute(ctx context.Context, input HandoffInput) (HandoffOutput, []ExecutionStep, error) {
	steps := make([]ExecutionStep, 0, 4)
	traceID := string(NewTraceID())
	parentSpanID := string(NewSpanID())

	// Step 1: Create Snapshot
	step1 := h.recordStep(traceID, parentSpanID, "Handoff", "RequestHandoff", "creating handoff snapshot", "Handoff")
	now := time.Now()
	checksum, err := input.SourceAgent.ComputeChecksum()
	if err != nil {
		step1.Error = NewStepError("SNAPSHOT_CHECKSUM_FAILED", err.Error(), false)
		step1.DurationMs = time.Since(now).Milliseconds()
		steps = append(steps, step1)
		h.obs.Record(steps)
		return HandoffOutput{Success: false, Error: err.Error()}, steps, err
	}
	step1.DurationMs = time.Since(now).Milliseconds()
	steps = append(steps, step1)

	// Step 2: Persist
	step2 := h.recordStep(traceID, parentSpanID, "Adapter", "SnapshotCreated", "persisting snapshot", "Handoff")
	now = time.Now()
	_, persistErr := h.persistence.CreateSnapshot(input.SourceAgent)
	if persistErr != nil {
		step2.Error = NewStepError("SNAPSHOT_PERSIST_FAILED", persistErr.Error(), false)
		step2.DurationMs = time.Since(now).Milliseconds()
		steps = append(steps, step2)
		h.obs.Record(steps)
		return HandoffOutput{Success: false, Error: persistErr.Error()}, steps, persistErr
	}
	step2.DurationMs = time.Since(now).Milliseconds()
	steps = append(steps, step2)

	// Step 3: Transfer
	step3 := h.recordStep(traceID, parentSpanID, "Adapter", "TransferSnapshot", "transferring snapshot", "Handoff")
	now = time.Now()
	data, err := input.SourceAgent.ToJSON()
	if err != nil {
		step3.Error = NewStepError("SNAPSHOT_SERIALIZE_FAILED", err.Error(), false)
		step3.DurationMs = time.Since(now).Milliseconds()
		steps = append(steps, step3)
		h.obs.Record(steps)
		return HandoffOutput{Success: false, Error: err.Error()}, steps, err
	}
	transferErr := h.transport.Transfer(ctx, checksum, data)
	if transferErr != nil {
		step3.Error = NewStepError("SNAPSHOT_TRANSFER_FAILED", transferErr.Error(), true)
		step3.DurationMs = time.Since(now).Milliseconds()
		steps = append(steps, step3)
		h.obs.Record(steps)
		return HandoffOutput{Success: false, Error: transferErr.Error()}, steps, transferErr
	}
	step3.DurationMs = time.Since(now).Milliseconds()
	steps = append(steps, step3)

	// Step 4: Confirm
	step4 := h.recordStep(traceID, parentSpanID, "Handoff", "ConfirmHandoff", "target confirming handoff", "Handoff")
	now = time.Now()
	contract := NewHandoffContract(input.SourceAgent.AgentID, input.TargetAgentID, input.TaskID, input.Phase, checksum)
	validator := NewContractValidator(checksum)
	_, validateErr := validator.Validate(ctx, *contract)
	if validateErr != nil {
		step4.Error = NewStepError("CONTRACT_VALIDATION_FAILED", validateErr.Error(), false)
		step4.DurationMs = time.Since(now).Milliseconds()
		steps = append(steps, step4)
		h.obs.Record(steps)
		return HandoffOutput{Success: false, Error: validateErr.Error(), Contract: contract}, steps, validateErr
	}
	step4.DurationMs = time.Since(now).Milliseconds()
	steps = append(steps, step4)
	h.obs.Record(steps)

	return HandoffOutput{Success: true, Contract: contract, SnapshotChecksum: checksum, TargetConfirmed: true}, steps, nil
}

func (h *HandoffComposer) ReceiveSnapshot(ctx context.Context, checksum string) (*DevSnapshot, []ExecutionStep, error) {
	steps := make([]ExecutionStep, 0, 2)
	traceID := string(NewTraceID())
	parentSpanID := string(NewSpanID())

	step1 := h.recordStep(traceID, parentSpanID, "Adapter", "ReceiveSnapshot", "receiving snapshot", "Handoff")
	now := time.Now()
	data, err := h.transport.Receive(ctx, checksum)
	if err != nil {
		step1.Error = NewStepError("SNAPSHOT_RECEIVE_FAILED", err.Error(), false)
		step1.DurationMs = time.Since(now).Milliseconds()
		steps = append(steps, step1)
		h.obs.Record(steps)
		return nil, steps, err
	}
	step1.DurationMs = time.Since(now).Milliseconds()
	steps = append(steps, step1)

	step2 := h.recordStep(traceID, parentSpanID, "Port", "VerifySnapshot", "verifying snapshot integrity", "Handoff")
	now = time.Now()
	snapshot, err := DevSnapshotFromJSON(data)
	if err != nil {
		step2.Error = NewStepError("SNAPSHOT_DESERIALIZE_FAILED", err.Error(), false)
		step2.DurationMs = time.Since(now).Milliseconds()
		steps = append(steps, step2)
		h.obs.Record(steps)
		return nil, steps, err
	}
	if !snapshot.VerifyChecksum() {
		err := fmt.Errorf("snapshot checksum verification failed")
		step2.Error = NewStepError("SNAPSHOT_CHECKSUM_MISMATCH", err.Error(), false)
		step2.DurationMs = time.Since(now).Milliseconds()
		steps = append(steps, step2)
		h.obs.Record(steps)
		return nil, steps, err
	}
	step2.DurationMs = time.Since(now).Milliseconds()
	steps = append(steps, step2)
	h.obs.Record(steps)
	return snapshot, steps, nil
}

func (h *HandoffComposer) recordStep(traceID, parentSpanID, unit, action, details, pattern string) ExecutionStep {
	step := NewExecutionStepWithTrace(parentSpanID, unit, action, details, pattern)
	step.TraceID = traceID
	return step
}
