//go:build lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

package core

import (
	"context"
	"fmt"
)

// ──────────────────────────────────────────────
// Projection — TASK-3.3
// ──────────────────────────────────────────────

// ProjectionHandler is a user-defined function that applies an event to state.
// It takes the current state (as []byte) and an event, and returns the new state.
// If the handler returns an error, the projection stops and the error is propagated.
type ProjectionHandler func(state []byte, event EventEnvelope) ([]byte, error)

// ProjectionInput is the input for computing a projection.
type ProjectionInput struct {
	AggregateID  string          // identifier of the aggregate being projected
	Events       []EventEnvelope // all events for this aggregate, sorted by version ascending
	FromVersion  int64           // start from this version (inclusive); 0 means full rebuild
	CurrentState []byte          // current known state (non-nil for incremental); empty for full rebuild
}

// ProjectionOutput is the result of a projection.
type ProjectionOutput struct {
	AggregateID     string // identifier of the aggregate
	State           []byte // final computed state after applying events
	Version         int64  // latest event version that was applied; 0 if no events processed
	EventsProcessed int    // number of events that were actually processed
}

// Projection is an Atom (pure function, no side effects) that computes the
// current state of an aggregate by folding events through a ProjectionHandler.
// Same input always yields the same output.
type Projection struct {
	handler ProjectionHandler
}

// NewProjection creates a new Projection with the given handler.
func NewProjection(handler ProjectionHandler) *Projection {
	return &Projection{handler: handler}
}

// Execute is a pure function that computes the aggregate state from events.
//
// Semantics:
//   - If Events is empty, returns CurrentState as-is with Version=0 and EventsProcessed=0.
//   - If FromVersion > 0, only events with Version >= FromVersion are processed (incremental).
//     The handler receives CurrentState as the starting state.
//   - If FromVersion == 0, all events are processed (full rebuild).
//     The handler receives CurrentState as the starting state (typically nil/empty for full rebuild).
//   - Events are applied in order. Each event's output state becomes the next event's input state.
//   - If the handler returns an error, execution stops immediately and the error is returned.
//   - Pure function: no I/O, no randomness, no shared mutable state. Same input always
//     returns the same output.
func (p *Projection) Execute(input ProjectionInput) (ProjectionOutput, error) {
	state := input.CurrentState
	version := int64(0)
	processed := 0

	// Empty events: return current state as-is
	if len(input.Events) == 0 {
		return ProjectionOutput{
			AggregateID:     input.AggregateID,
			State:           state,
			Version:         0,
			EventsProcessed: 0,
		}, nil
	}

	// Determine the starting version filter.
	// FromVersion == 0 means full rebuild (process all events).
	// FromVersion > 0 means incremental (only process events with Version >= FromVersion).
	fromVersion := input.FromVersion

	for _, event := range input.Events {
		// Skip events below the fromVersion threshold
		if event.Version < fromVersion {
			continue
		}

		newState, err := p.handler(state, event)
		if err != nil {
			return ProjectionOutput{}, fmt.Errorf(
				"projection: handler error on event %s (version %d): %w",
				event.EventID, event.Version, err,
			)
		}
		state = newState
		version = event.Version
		processed++
	}

	return ProjectionOutput{
		AggregateID:     input.AggregateID,
		State:           state,
		Version:         version,
		EventsProcessed: processed,
	}, nil
}

// ProjectionAsStep wraps a Projection as a Step[ProjectionInput, ProjectionOutput].
// The resulting Step has UnitType "Atom" since Projection is a pure function.
func ProjectionAsStep(p *Projection) Step[ProjectionInput, ProjectionOutput] {
	return StepFunc[ProjectionInput, ProjectionOutput]{
		execute: func(ctx context.Context, input ProjectionInput) (ProjectionOutput, error) {
			return p.Execute(input)
		},
		unitType: "Atom",
	}
}