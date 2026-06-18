//go:build lecore_tier3 || lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

package core

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// ──────────────────────────────────────────────
// Handoff — multi-agent relay protocol (Phase 3)
// 合并自: handoff.go + handoff_composer.go + handoff_rollback.go +
//         handoff_transport.go + handoff_snapshot.go + handoff_contract.go
// ──────────────────────────────────────────────

// ============================================================================
// SECTION 1: Core Types — DevSnapshot, Artifact, WorkItem, Decision
// ============================================================================

// DevSnapshot is the self-describing state that an agent deposits at the
// architecture boundary for the next agent to withdraw. It is the medium
// through which agents communicate without direct coupling.
type DevSnapshot struct {
	TaskID        string       `json:"task_id"`
	AgentID       string       `json:"agent_id"`
	Phase         string       `json:"phase"`
	Checkpoint    string       `json:"checkpoint"`
	Artifacts     []Artifact   `json:"artifacts"`
	Pending       []WorkItem   `json:"pending"`
	Constraints   []string     `json:"constraints"`
	Decisions     []Decision   `json:"decisions"`
	Dependencies  []string     `json:"dependencies"`
	SchemaVersion string       `json:"schema_version"`
	CreatedAt     time.Time    `json:"created_at"`
	Checksum      string       `json:"checksum"`
}

type Artifact struct {
	Path        string `json:"path"`
	Type        string `json:"type"`
	Description string `json:"description"`
	Hash        string `json:"hash"`
}

type WorkItem struct {
	ID             string  `json:"id"`
	Title          string  `json:"title"`
	Priority       string  `json:"priority"`
	EstimatedHours float64 `json:"estimated_hours"`
}

type Decision struct {
	ID           string   `json:"id"`
	Title        string   `json:"title"`
	Rationale    string   `json:"rationale"`
	Alternatives []string `json:"alternatives"`
}

// DevSnapshot constructors and checksum operations
func NewDevSnapshot(taskID, agentID, phase, checkpoint string) *DevSnapshot {
	return &DevSnapshot{
		TaskID: taskID, AgentID: agentID, Phase: phase, Checkpoint: checkpoint,
		Artifacts: make([]Artifact, 0), Pending: make([]WorkItem, 0),
		Constraints: make([]string, 0), Decisions: make([]Decision, 0),
		Dependencies: make([]string, 0), SchemaVersion: "1.0", CreatedAt: time.Now(),
	}
}

func (s *DevSnapshot) ComputeChecksum() (string, error) {
	original := s.Checksum
	s.Checksum = ""
	data, err := json.Marshal(s)
	if err != nil {
		s.Checksum = original
		return "", fmt.Errorf("failed to marshal snapshot: %w", err)
	}
	hash := sha256.Sum256(data)
	s.Checksum = fmt.Sprintf("%x", hash)
	return s.Checksum, nil
}

func (s *DevSnapshot) VerifyChecksum() bool {
	stored := s.Checksum
	computed, err := s.ComputeChecksum()
	if err != nil {
		return false
	}
	s.Checksum = stored
	return computed == stored
}

func (s *DevSnapshot) ToJSON() ([]byte, error) { return json.Marshal(s) }

func DevSnapshotFromJSON(data []byte) (*DevSnapshot, error) {
	var snap DevSnapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return nil, fmt.Errorf("failed to unmarshal DevSnapshot: %w", err)
	}
	return &snap, nil
}

// ============================================================================
// SECTION 2: HandoffContract — the agreement between agents
// ============================================================================

const ContractExpiry = 5 * time.Minute

type HandoffContract struct {
	SourceID         string    `json:"source_id"`
	TargetID         string    `json:"target_id"`
	TaskID           string    `json:"task_id"`
	Phase            string    `json:"phase"`
	SnapshotChecksum string    `json:"snapshot_checksum"`
	Timestamp        time.Time `json:"timestamp"`
}

func (c *HandoffContract) IsExpired() bool { return time.Since(c.Timestamp) > ContractExpiry }

func NewHandoffContract(sourceID, targetID, taskID, phase, checksum string) *HandoffContract {
	return &HandoffContract{
		SourceID: sourceID, TargetID: targetID, TaskID: taskID,
		Phase: phase, SnapshotChecksum: checksum, Timestamp: time.Now(),
	}
}

type ContractValidator struct{ expectedChecksum string }

func NewContractValidator(expectedChecksum string) *ContractValidator {
	return &ContractValidator{expectedChecksum: expectedChecksum}
}

func (v *ContractValidator) Validate(ctx context.Context, input HandoffContract) (HandoffContract, error) {
	if input.SourceID == "" || input.TargetID == "" || input.TaskID == "" {
		return input, NewStepError("CONTRACT_INVALID", "source/target/task ID must not be empty", false)
	}
	if input.IsExpired() {
		return input, NewStepError("CONTRACT_EXPIRED", "handoff contract has expired (older than 5 minutes)", false)
	}
	if input.SnapshotChecksum == "" {
		return input, NewStepError("CONTRACT_INVALID", "snapshot checksum is empty", false)
	}
	if v.expectedChecksum != "" && input.SnapshotChecksum != v.expectedChecksum {
		return input, NewStepError("CONTRACT_CHECKSUM_MISMATCH", "snapshot checksum does not match", false)
	}
	return input, nil
}

func ContractValidatorAsStep(v *ContractValidator) Step[HandoffContract, HandoffContract] {
	return PortAsStep[HandoffContract, HandoffContract](v)
}

// ============================================================================
// SECTION 3: HandoffSnapshot — legacy types for backward compat
// ============================================================================

type HandoffRequest struct {
	SourceID string
	TargetID string
	TaskType string
	Payload  interface{}
	Token    string
}

type HandoffSnapshot struct {
	Token string
	State interface{}
	Meta  map[string]string
}

type HandoffResult struct {
	Success bool
	Token   string
	Error   string
}

type TransportFunc func(snap HandoffSnapshot) interface{}

func InProcTransport(snap HandoffSnapshot) interface{} { return snap.State }

type DefaultSnapshotAdapter struct{}

func (d *DefaultSnapshotAdapter) CreateSnapshot(state interface{}) HandoffSnapshot {
	return HandoffSnapshot{
		Token: "snap-" + time.Now().Format("150405"), State: state,
		Meta: map[string]string{"created_at": time.Now().Format(time.RFC3339)},
	}
}

func (d *DefaultSnapshotAdapter) RestoreSnapshot(snap HandoffSnapshot) (interface{}, error) {
	return snap.State, nil
}

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

// ============================================================================
// SECTION 6: Transport — HandoffTransport interface + implementations
// ============================================================================

type HandoffTransport interface {
	Transfer(ctx context.Context, checksum string, snapshot []byte) error
	Receive(ctx context.Context, checksum string) ([]byte, error)
}

type TransportPayload struct {
	Checksum string `json:"checksum"`
	Data     []byte `json:"data"`
}

// InProcHandoffTransport
type InProcHandoffTransport struct {
	mu    sync.RWMutex
	store map[string][]byte
}

func NewInProcHandoffTransport() *InProcHandoffTransport {
	return &InProcHandoffTransport{store: make(map[string][]byte)}
}

func (t *InProcHandoffTransport) Transfer(ctx context.Context, checksum string, snapshot []byte) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.store[checksum] = make([]byte, len(snapshot))
	copy(t.store[checksum], snapshot)
	return nil
}

func (t *InProcHandoffTransport) Receive(ctx context.Context, checksum string) ([]byte, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	data, ok := t.store[checksum]
	if !ok {
		return nil, fmt.Errorf("snapshot not found: checksum=%s", checksum)
	}
	result := make([]byte, len(data))
	copy(result, data)
	return result, nil
}

func (t *InProcHandoffTransport) Delete(checksum string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.store, checksum)
}

func (t *InProcHandoffTransport) Count() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return len(t.store)
}

// HTTPTransport
type HTTPTransport struct {
	baseURL    string
	httpClient *http.Client
}

func NewHTTPTransport(baseURL string) *HTTPTransport {
	return &HTTPTransport{baseURL: baseURL, httpClient: &http.Client{Timeout: 30 * time.Second}}
}

func (t *HTTPTransport) Transfer(ctx context.Context, checksum string, snapshot []byte) error {
	url := fmt.Sprintf("%s/handoff/%s", t.baseURL, checksum)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(snapshot))
	if err != nil {
		return fmt.Errorf("failed to create transfer request: %w", err)
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	resp, err := t.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("transfer request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("transfer failed with status %d", resp.StatusCode)
	}
	return nil
}

func (t *HTTPTransport) Receive(ctx context.Context, checksum string) ([]byte, error) {
	url := fmt.Sprintf("%s/handoff/%s", t.baseURL, checksum)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create receive request: %w", err)
	}
	resp, err := t.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("receive request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("snapshot not found: checksum=%s", checksum)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("receive failed with status %d", resp.StatusCode)
	}
	buf := new(bytes.Buffer)
	_, err = buf.ReadFrom(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}
	return buf.Bytes(), nil
}

// Transport adapters
type TransportTransferAdapter struct{ transport HandoffTransport }

func NewTransportTransferAdapter(transport HandoffTransport) *TransportTransferAdapter {
	return &TransportTransferAdapter{transport: transport}
}

func (a *TransportTransferAdapter) Execute(ctx context.Context, input TransportPayload) (TransportPayload, error) {
	err := a.transport.Transfer(ctx, input.Checksum, input.Data)
	if err != nil {
		return input, fmt.Errorf("transport transfer failed: %w", err)
	}
	return input, nil
}

type TransportReceiveAdapter struct{ transport HandoffTransport }

func NewTransportReceiveAdapter(transport HandoffTransport) *TransportReceiveAdapter {
	return &TransportReceiveAdapter{transport: transport}
}

func (a *TransportReceiveAdapter) Execute(ctx context.Context, input TransportPayload) (TransportPayload, error) {
	data, err := a.transport.Receive(ctx, input.Checksum)
	if err != nil {
		return input, fmt.Errorf("transport receive failed: %w", err)
	}
	input.Data = data
	return input, nil
}