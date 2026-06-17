package core

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"time"
)

// ──────────────────────────────────────────────
// DevSnapshot — the architecture-deposited state
// ──────────────────────────────────────────────

// DevSnapshot is the self-describing state that an agent deposits at the
// architecture boundary for the next agent to withdraw. It is the medium
// through which agents communicate without direct coupling.
//
// Every field is designed to answer: "What does the next agent need to know
// to continue without any ambiguity?"
type DevSnapshot struct {
	// TaskID uniquely identifies the task being handed off.
	TaskID string `json:"task_id"`

	// AgentID identifies the agent that created this snapshot.
	AgentID string `json:"agent_id"`

	// Phase is the current development phase: design/coding/testing/review/deploy.
	Phase string `json:"phase"`

	// Checkpoint is a human-readable description of what was accomplished.
	Checkpoint string `json:"checkpoint"`

	// Artifacts are the concrete outputs produced so far (files, modules, etc.).
	Artifacts []Artifact `json:"artifacts"`

	// Pending is the list of work items the next agent must complete.
	Pending []WorkItem `json:"pending"`

	// Constraints are rules the next agent must follow.
	Constraints []string `json:"constraints"`

	// Decisions are key architectural decisions made so far.
	Decisions []Decision `json:"decisions"`

	// Dependencies are external dependencies needed by the next agent.
	Dependencies []string `json:"dependencies"`

	// SchemaVersion is the version of the DevSnapshot schema.
	SchemaVersion string `json:"schema_version"`

	// CreatedAt is the timestamp when this snapshot was created.
	CreatedAt time.Time `json:"created_at"`

	// Checksum is the SHA256 hash of the JSON representation (excluding this field).
	// Used to verify data integrity during transfer.
	Checksum string `json:"checksum"`
}

// Artifact represents a concrete output produced during development.
type Artifact struct {
	// Path is the relative file path of the artifact.
	Path string `json:"path"`

	// Type is the kind of artifact: file, module, package, test.
	Type string `json:"type"`

	// Description is a human-readable summary of the artifact.
	Description string `json:"description"`

	// Hash is the SHA256 hash of the artifact content.
	Hash string `json:"hash"`
}

// WorkItem represents a task the next agent must complete.
type WorkItem struct {
	// ID uniquely identifies the work item.
	ID string `json:"id"`

	// Title is a short description of the work item.
	Title string `json:"title"`

	// Priority is the importance: high, medium, low.
	Priority string `json:"priority"`

	// EstimatedHours is the estimated effort in hours.
	EstimatedHours float64 `json:"estimated_hours"`
}

// Decision represents a key architectural or design decision.
type Decision struct {
	// ID uniquely identifies the decision.
	ID string `json:"id"`

	// Title is a short description of the decision.
	Title string `json:"title"`

	// Rationale explains why this decision was made.
	Rationale string `json:"rationale"`

	// Alternatives are other options that were considered.
	Alternatives []string `json:"alternatives"`
}

// ──────────────────────────────────────────────
// Checksum operations
// ──────────────────────────────────────────────

// ComputeChecksum calculates the SHA256 hash of the DevSnapshot's JSON
// representation (excluding the Checksum field itself).
// It sets the Checksum field on the snapshot and returns the computed value.
func (s *DevSnapshot) ComputeChecksum() (string, error) {
	// Save the current checksum, clear it for computation
	original := s.Checksum
	s.Checksum = ""

	data, err := json.Marshal(s)
	if err != nil {
		s.Checksum = original
		return "", fmt.Errorf("failed to marshal snapshot: %w", err)
	}

	hash := sha256.Sum256(data)
	checksum := fmt.Sprintf("%x", hash)

	// Set the checksum
	s.Checksum = checksum
	return checksum, nil
}

// VerifyChecksum recomputes the checksum and compares it to the stored value.
// Returns true if the snapshot has not been tampered with.
func (s *DevSnapshot) VerifyChecksum() bool {
	stored := s.Checksum
	computed, err := s.ComputeChecksum()
	if err != nil {
		return false
	}
	s.Checksum = stored
	return computed == stored
}

// NewDevSnapshot creates a new DevSnapshot with default values.
func NewDevSnapshot(taskID, agentID, phase, checkpoint string) *DevSnapshot {
	return &DevSnapshot{
		TaskID:        taskID,
		AgentID:       agentID,
		Phase:         phase,
		Checkpoint:    checkpoint,
		Artifacts:     make([]Artifact, 0),
		Pending:       make([]WorkItem, 0),
		Constraints:   make([]string, 0),
		Decisions:     make([]Decision, 0),
		Dependencies:  make([]string, 0),
		SchemaVersion: "1.0",
		CreatedAt:     time.Now(),
	}
}

// ToJSON serializes the DevSnapshot to JSON bytes.
func (s *DevSnapshot) ToJSON() ([]byte, error) {
	return json.Marshal(s)
}

// DevSnapshotFromJSON deserializes a DevSnapshot from JSON bytes.
func DevSnapshotFromJSON(data []byte) (*DevSnapshot, error) {
	var snap DevSnapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return nil, fmt.Errorf("failed to unmarshal DevSnapshot: %w", err)
	}
	return &snap, nil
}