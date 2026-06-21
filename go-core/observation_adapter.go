package core

import (
	"context"
	"sync"
	"time"
)

// ObservationAdapter is the sink for execution records.
// All primitives emit to this interface; the observation layer
// consumes it for the X-Ray dashboard.
type ObservationAdapter interface {
	Record(steps []ExecutionStep)
}

// InMemoryObservationAdapter stores execution steps in memory.
// Thread-safe for concurrent use.
type InMemoryObservationAdapter struct {
	mu    sync.RWMutex
	Steps []ExecutionStep
}

// Record appends execution steps to the in-memory store.
func (a *InMemoryObservationAdapter) Record(steps []ExecutionStep) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.Steps = append(a.Steps, steps...)
}

// GetSteps returns a copy of all recorded steps.
func (a *InMemoryObservationAdapter) GetSteps() []ExecutionStep {
	a.mu.RLock()
	defer a.mu.RUnlock()
	result := make([]ExecutionStep, len(a.Steps))
	copy(result, a.Steps)
	return result
}

// GetTraceTree builds a trace tree from all recorded steps.
func (a *InMemoryObservationAdapter) GetTraceTree() *TraceTree {
	return BuildTraceTree(a.GetSteps())
}

// Clear removes all recorded steps.
func (a *InMemoryObservationAdapter) Clear() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.Steps = a.Steps[:0]
}

// StepCount returns the number of recorded steps.
func (a *InMemoryObservationAdapter) StepCount() int {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return len(a.Steps)
}

// NoOpObservationAdapter silently discards all execution steps.
// Useful for testing or when observation is disabled.
type NoOpObservationAdapter struct{}

// Record is a no-op.
func (a *NoOpObservationAdapter) Record(steps []ExecutionStep) {}

// ExecutionStep constructors

// NewExecutionStep creates an ExecutionStep with UUID-based trace and span IDs.
// This is the canonical constructor; all primitives should use this.
func NewExecutionStep(unit, action, details, pattern string) ExecutionStep {
	now := time.Now()
	return ExecutionStep{
		Timestamp: now,
		TraceID:   string(NewTraceID()),
		SpanID:    string(NewSpanID()),
		Unit:      unit,
		Action:    action,
		Details:   details,
		Pattern:   pattern,
	}
}

// NewExecutionStepWithTrace creates an ExecutionStep as a child of a parent span.
func NewExecutionStepWithTrace(parentSpanID string, unit, action, details, pattern string) ExecutionStep {
	step := NewExecutionStep(unit, action, details, pattern)
	step.ParentID = parentSpanID
	return step
}

// NewExecutionStepWithError creates an ExecutionStep that records an error.
func NewExecutionStepWithError(unit, action, details, pattern string, err *StepError) ExecutionStep {
	step := NewExecutionStep(unit, action, details, pattern)
	step.Error = err
	return step
}

// Context helpers for trace propagation

type traceKeyType struct{}

var traceKey = traceKeyType{}

// TraceContext carries trace identity through context.
type TraceContext struct {
	TraceID string
	SpanID  string
}

// WithTraceContext injects trace identity into a context.
func WithTraceContext(ctx context.Context, tc TraceContext) context.Context {
	return context.WithValue(ctx, traceKey, tc)
}

// GetTraceContext extracts trace identity from a context.
func GetTraceContext(ctx context.Context) (TraceContext, bool) {
	tc, ok := ctx.Value(traceKey).(TraceContext)
	return tc, ok
}
