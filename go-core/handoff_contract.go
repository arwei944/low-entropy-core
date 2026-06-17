package core

import (
	"context"
	"time"
)

// ──────────────────────────────────────────────
// HandoffContract — the agreement between agents
// ──────────────────────────────────────────────

// HandoffContract is a formal agreement between two agents for a handoff.
// It carries the identity of both parties, the task being transferred,
// and a checksum to verify snapshot integrity.
type HandoffContract struct {
	// SourceID identifies the agent handing off the task.
	SourceID string `json:"source_id"`

	// TargetID identifies the agent receiving the task.
	TargetID string `json:"target_id"`

	// TaskID identifies the task being transferred.
	TaskID string `json:"task_id"`

	// Phase is the development phase the task is in.
	Phase string `json:"phase"`

	// SnapshotChecksum is the SHA256 hash of the DevSnapshot being transferred.
	SnapshotChecksum string `json:"snapshot_checksum"`

	// Timestamp is when the contract was created.
	Timestamp time.Time `json:"timestamp"`
}

// ContractExpiry is the default validity period for a HandoffContract.
const ContractExpiry = 5 * time.Minute

// IsExpired checks whether the contract has exceeded its validity period.
func (c *HandoffContract) IsExpired() bool {
	return time.Since(c.Timestamp) > ContractExpiry
}

// NewHandoffContract creates a new HandoffContract.
func NewHandoffContract(sourceID, targetID, taskID, phase, checksum string) *HandoffContract {
	return &HandoffContract{
		SourceID:         sourceID,
		TargetID:         targetID,
		TaskID:           taskID,
		Phase:            phase,
		SnapshotChecksum: checksum,
		Timestamp:        time.Now(),
	}
}

// ──────────────────────────────────────────────
// ContractValidator — Port for contract validation
// ──────────────────────────────────────────────

// ContractValidator is a Port that validates HandoffContracts.
// It ensures the contract is complete, not expired, and has a valid checksum.
type ContractValidator struct {
	// expectedChecksum is the checksum to validate against.
	expectedChecksum string
}

// NewContractValidator creates a ContractValidator with the expected checksum.
func NewContractValidator(expectedChecksum string) *ContractValidator {
	return &ContractValidator{expectedChecksum: expectedChecksum}
}

// Validate implements the Port[HandoffContract, HandoffContract] interface.
// It checks:
//   1. SourceID and TargetID are non-empty
//   2. Contract is not expired (5 minute window)
//   3. SnapshotChecksum matches the expected value
func (v *ContractValidator) Validate(ctx context.Context, input HandoffContract) (HandoffContract, error) {
	// 1. Identity validation
	if input.SourceID == "" {
		return input, NewStepError("CONTRACT_INVALID", "source agent ID is empty", false)
	}
	if input.TargetID == "" {
		return input, NewStepError("CONTRACT_INVALID", "target agent ID is empty", false)
	}
	if input.TaskID == "" {
		return input, NewStepError("CONTRACT_INVALID", "task ID is empty", false)
	}

	// 2. Expiry check
	if input.IsExpired() {
		return input, NewStepError("CONTRACT_EXPIRED",
			"handoff contract has expired (older than 5 minutes)", false)
	}

	// 3. Checksum validation
	if input.SnapshotChecksum == "" {
		return input, NewStepError("CONTRACT_INVALID", "snapshot checksum is empty", false)
	}
	if v.expectedChecksum != "" && input.SnapshotChecksum != v.expectedChecksum {
		return input, NewStepError("CONTRACT_CHECKSUM_MISMATCH",
			"snapshot checksum does not match expected value", false)
	}

	return input, nil
}

// ContractValidatorAsStep wraps the ContractValidator as a Step.
func ContractValidatorAsStep(v *ContractValidator) Step[HandoffContract, HandoffContract] {
	return PortAsStep[HandoffContract, HandoffContract](v)
}