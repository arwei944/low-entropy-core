package core

import (
	"context"
	"fmt"
	"log"
)

// ──────────────────────────────────────────────
// TASK-6.2: Cross-Composer Transaction (Saga)
// ──────────────────────────────────────────────

// TransactionContext carries transaction identity.
type TransactionContext struct {
	TransactionID string
	Phase         string // "begin", "commit", "rollback"
}

// SagaStep represents a step in a saga transaction with its compensation.
type SagaStep struct {
	Name       string
	Execute    Step[any, any]
	Compensate Step[any, any] // compensation action
}

// SagaComposer orchestrates a saga transaction with compensation.
// Steps execute in order; on failure, previous steps are compensated in reverse.
type SagaComposer struct {
	steps []SagaStep
	obs   ObservationAdapter
}

// NewSagaComposer creates a new SagaComposer with the given observation adapter.
// If obs is nil, a NoOpObservationAdapter is used.
func NewSagaComposer(obs ObservationAdapter) *SagaComposer {
	if obs == nil {
		obs = &NoOpObservationAdapter{}
	}
	return &SagaComposer{obs: obs}
}

// AddStep appends a saga step and returns the composer for chaining.
func (s *SagaComposer) AddStep(step SagaStep) *SagaComposer {
	s.steps = append(s.steps, step)
	return s
}

// Run executes all steps in order. If any step fails, previously executed
// steps are compensated in reverse order. Compensation failures are logged
// but do not prevent the remaining compensations from running.
// Returns the error from the failed step (or nil on success).
func (s *SagaComposer) Run(ctx context.Context, input any) (any, error) {
	traceID := string(NewTraceID())
	spanID := string(NewSpanID())

	// Phase 1: Execute steps in order, tracking completion
	completed := 0
	result := input

	for i, step := range s.steps {
		select {
		case <-ctx.Done():
			es := NewExecutionStepWithTrace(spanID, "SagaComposer", "cancelled", "context done", "saga")
			es.TraceID = traceID
			s.obs.Record([]ExecutionStep{es})
			s.compensate(ctx, completed, traceID, spanID)
			return result, ctx.Err()
		default:
		}

		output, err := step.Execute.Execute(ctx, result)
		completed = i + 1

		if err != nil {
			es := NewExecutionStepWithTrace(spanID, "SagaComposer", "step_failed",
				fmt.Sprintf("saga step %q failed: %v", step.Name, err), "saga")
			es.TraceID = traceID
			if se, ok := err.(*StepError); ok {
				es.Error = se
			} else {
				es.Error = &StepError{Code: "SAGA_STEP_FAILED", Message: err.Error(), Recoverable: false}
			}
			s.obs.Record([]ExecutionStep{es})

			s.compensate(ctx, completed-1, traceID, spanID)
			return result, err
		}

		result = output
	}

	es := NewExecutionStepWithTrace(spanID, "SagaComposer", "completed",
		fmt.Sprintf("saga completed: %d steps", len(s.steps)), "saga")
	es.TraceID = traceID
	s.obs.Record([]ExecutionStep{es})

	return result, nil
}

// compensate runs compensation for all completed steps in reverse order.
// Compensation failures are logged but do not stop the remaining compensations.
func (s *SagaComposer) compensate(ctx context.Context, completed int, traceID, spanID string) {
	for i := completed - 1; i >= 0; i-- {
		step := s.steps[i]
		_, err := step.Compensate.Execute(ctx, nil)
		if err != nil {
			es := NewExecutionStepWithTrace(spanID, "SagaComposer", "compensate_failed",
				fmt.Sprintf("compensation for step %q failed: %v", step.Name, err), "saga")
			es.TraceID = traceID
			s.obs.Record([]ExecutionStep{es})
			log.Printf("saga compensation for step %q failed: %v", step.Name, err)
		}
	}
}