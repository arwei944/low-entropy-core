//go:build lecore_tier3 || lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

package core

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"time"
)

// ──────────────────────────────────────────────
// Handoff — multi-agent relay protocol (Phase 3)
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
