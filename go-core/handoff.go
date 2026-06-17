package core

import (
	"context"
	"time"
)

// ──────────────────────────────────────────────
// Handoff — multi-agent relay protocol (Phase 3)
// ──────────────────────────────────────────────

// HandoffRequest is the transfer envelope between agents.
// It carries the source agent's identity, the target's identity,
// the task type, the payload, and a correlation token.
type HandoffRequest struct {
	SourceID string
	TargetID string
	TaskType string
	Payload  interface{}
	Token    string
}

// HandoffSnapshot is the architecture-deposited state that the next agent withdraws.
// It is the medium through which agents communicate without direct coupling.
type HandoffSnapshot struct {
	Token string
	State interface{}
	Meta  map[string]string
}

// HandoffResult is the outcome of a handoff operation.
type HandoffResult struct {
	Success bool
	Token   string
	Error   string
}

// TransportFunc is a function that transports a snapshot to the next agent.
// In production, this would be a queue, database, or RPC call.
type TransportFunc func(snap HandoffSnapshot) interface{}

// InProcTransport is a simple in-process transport for testing and single-process use.
func InProcTransport(snap HandoffSnapshot) interface{} {
	return snap.State
}

// DefaultSnapshotAdapter is a basic snapshot adapter for untyped state.
type DefaultSnapshotAdapter struct{}

// CreateSnapshot creates a snapshot with a timestamp-based token.
func (d *DefaultSnapshotAdapter) CreateSnapshot(state interface{}) HandoffSnapshot {
	return HandoffSnapshot{
		Token: "snap-" + time.Now().Format("150405"),
		State: state,
		Meta: map[string]string{
			"created_at": time.Now().Format(time.RFC3339),
		},
	}
}

// RestoreSnapshot restores state from a snapshot.
func (d *DefaultSnapshotAdapter) RestoreSnapshot(snap HandoffSnapshot) (interface{}, error) {
	return snap.State, nil
}

// NewHandoff creates a Composer that performs a handoff between two agents.
// It deposits a snapshot via the SnapshotAdapter and transports it to the target agent.
// This is a simplified Phase 1 implementation; full Handoff protocol comes in Phase 3.
func NewHandoff(source, target Composer[any], snapshot SnapshotAdapter[any], transport TransportFunc) Composer[any] {
	return NewPipeline[any](nil,
		StepFunc[any, any]{
			execute: func(ctx context.Context, input any) (any, error) {
				req, ok := input.(HandoffRequest)
				if !ok {
					return nil, &StepError{Code: "HANDOFF_INVALID_INPUT", Message: "input must be HandoffRequest", Recoverable: false}
				}

				// 1. Source agent deposits snapshot
				snap := snapshot.CreateSnapshot(req.Payload)

				// 2. Transport snapshot to target
				_ = transport(snap)

				// 3. Target agent withdraws and runs
				state, err := snapshot.RestoreSnapshot(snap)
				if err != nil {
					return HandoffResult{Success: false, Token: req.Token, Error: err.Error()}, nil
				}

				_, _, runErr := target.Run(ctx, state)
				if runErr != nil {
					return HandoffResult{Success: false, Token: req.Token, Error: runErr.Error()}, nil
				}

				return HandoffResult{Success: true, Token: req.Token}, nil
			},
			unitType: "Handoff",
		},
	)
}