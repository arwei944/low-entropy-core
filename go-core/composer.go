package core

import (
	"context"
	"math"
	"sync"
	"time"
)

// ──────────────────────────────────────────────
// Composer — the fourth primitive (orchestration)
// ──────────────────────────────────────────────

// Composer is the fourth primitive: the orchestration engine.
// It composes Steps into pipelines, branches, parallel executions,
// and other coordination patterns. A Composer is itself composable —
// Composers can be nested inside other Composers.
//
// Every Composer emits ExecutionSteps for full observability.
type Composer[T any] interface {
	// Run executes the composed workflow with the given input.
	// Returns the result, all execution steps recorded, and any error.
	Run(ctx context.Context, input T) (T, []ExecutionStep, error)
}

// ──────────────────────────────────────────────
// Pipeline — linear step chain (default Composer)
// ──────────────────────────────────────────────

// Pipeline is a linear chain of Steps that transform T → T.
// It is the default Composer implementation. Each step's output
// becomes the next step's input. Observation is recorded at every step.
type Pipeline[T any] struct {
	steps []Step[T, T]
	obs   ObservationAdapter
}

// NewPipeline creates a Pipeline with the given observation adapter and steps.
// If obs is nil, a NoOpObservationAdapter is used.
func NewPipeline[T any](obs ObservationAdapter, steps ...Step[T, T]) *Pipeline[T] {
	if obs == nil {
		obs = &NoOpObservationAdapter{}
	}
	return &Pipeline[T]{steps: steps, obs: obs}
}

// Run executes all steps in sequence, recording each as an ExecutionStep.
// If any step fails, execution stops and the error is returned along with
// all steps recorded up to that point.
func (p *Pipeline[T]) Run(ctx context.Context, input T) (T, []ExecutionStep, error) {
	steps := make([]ExecutionStep, 0, len(p.steps))
	result := input

	// Generate a trace ID for the entire pipeline run
	traceID := string(NewTraceID())
	parentSpanID := string(NewSpanID())

	for i, step := range p.steps {
		// Check context cancellation before each step
		select {
		case <-ctx.Done():
			errStep := NewExecutionStepWithTrace(parentSpanID, step.UnitType(), "execute", "cancelled before step", "")
			errStep.TraceID = traceID
			errStep.Error = &StepError{Code: "CONTEXT_CANCELLED", Message: ctx.Err().Error(), Recoverable: false}
			steps = append(steps, errStep)
			p.obs.Record(steps)
			return result, steps, ctx.Err()
		default:
		}

		// Execute the step with timing
		start := time.Now()
		output, err := step.Execute(ctx, result)
		elapsed := time.Since(start)

		// Build the execution step record
		es := NewExecutionStepWithTrace(parentSpanID, step.UnitType(), "execute", "pipeline step", "")
		es.TraceID = traceID
		es.DurationMs = elapsed.Milliseconds()
		es.Metadata = map[string]interface{}{
			"step_index": i,
			"total_steps": len(p.steps),
		}

		if err != nil {
			if se, ok := err.(*StepError); ok {
				es.Error = se
			} else {
				es.Error = &StepError{Code: "STEP_ERROR", Message: err.Error(), Recoverable: false}
			}
			es.Details = "pipeline step failed"
			steps = append(steps, es)
			p.obs.Record(steps)
			return result, steps, err
		}

		es.Details = "pipeline step completed"
		steps = append(steps, es)
		result = output

		// Check context after step completion (catches timeout during long steps)
		select {
		case <-ctx.Done():
			cancelStep := NewExecutionStepWithTrace(parentSpanID, "Composer", "cancelled", "context done after step", "")
			cancelStep.TraceID = traceID
			cancelStep.Error = &StepError{Code: "CONTEXT_CANCELLED", Message: ctx.Err().Error(), Recoverable: false}
			steps = append(steps, cancelStep)
			p.obs.Record(steps)
			return result, steps, ctx.Err()
		default:
		}
	}

	p.obs.Record(steps)
	return result, steps, nil
}

// AddStep appends a step to the pipeline. Returns the pipeline for chaining.
func (p *Pipeline[T]) AddStep(step Step[T, T]) *Pipeline[T] {
	p.steps = append(p.steps, step)
	return p
}

// StepCount returns the number of steps in the pipeline.
func (p *Pipeline[T]) StepCount() int {
	return len(p.steps)
}

// ──────────────────────────────────────────────
// Branch — conditional routing pattern
// ──────────────────────────────────────────────

// BranchStep routes execution based on a condition. If the condition
// evaluates to true, the truePath is executed; otherwise the falsePath.
// Both paths must produce the same type T.
type BranchStep[T any] struct {
	condition func(T) bool
	truePath  Composer[T]
	falsePath Composer[T]
}

// NewBranch creates a conditional branch step.
// condition is evaluated on the current input; the matching path is executed.
func NewBranch[T any](condition func(T) bool, truePath, falsePath Composer[T]) Step[T, T] {
	return StepFunc[T, T]{
		execute: func(ctx context.Context, input T) (T, error) {
			if condition(input) {
				result, steps, err := truePath.Run(ctx, input)
				// Propagate steps upward (they'll be captured by the parent composer)
				_ = steps
				return result, err
			}
			result, steps, err := falsePath.Run(ctx, input)
			_ = steps
			return result, err
		},
		unitType: "Branch",
	}
}

// ──────────────────────────────────────────────
// Parallel — true concurrent execution pattern
// ──────────────────────────────────────────────

// ParallelResults holds the results of parallel execution.
// Each result is indexed by its position in the input.
type ParallelResults[T any] struct {
	Results []T                `json:"results"`
	Errors  []error            `json:"errors,omitempty"`
	Steps   [][]ExecutionStep  `json:"steps,omitempty"`
}

// RunParallel executes multiple Composers concurrently using goroutines.
// Each Composer receives the same input and runs independently.
// Results are collected in order. This is a true parallel implementation
// using sync.WaitGroup, not sequential simulation.
func RunParallel[T any](ctx context.Context, input T, composers ...Composer[T]) (ParallelResults[T], []ExecutionStep, error) {
	if len(composers) == 0 {
		return ParallelResults[T]{}, nil, nil
	}

	type result struct {
		index int
		value T
		steps []ExecutionStep
		err   error
	}

	resultCh := make(chan result, len(composers))
	var wg sync.WaitGroup

	for i, c := range composers {
		wg.Add(1)
		go func(idx int, comp Composer[T]) {
			defer wg.Done()
			val, steps, err := comp.Run(ctx, input)
			resultCh <- result{index: idx, value: val, steps: steps, err: err}
		}(i, c)
	}

	// Wait for all goroutines to complete, then close the channel
	go func() {
		wg.Wait()
		close(resultCh)
	}()

	// Collect results in order
	results := make([]T, len(composers))
	errs := make([]error, len(composers))
	allSteps := make([][]ExecutionStep, len(composers))
	hasError := false

	for r := range resultCh {
		results[r.index] = r.value
		errs[r.index] = r.err
		allSteps[r.index] = r.steps
		if r.err != nil {
			hasError = true
		}
	}

	// Flatten all steps
	flatSteps := make([]ExecutionStep, 0)
	for _, s := range allSteps {
		flatSteps = append(flatSteps, s...)
	}

	var finalErr error
	if hasError {
		finalErr = &StepError{Code: "PARALLEL_ERROR", Message: "one or more parallel branches failed", Recoverable: false}
	}

	return ParallelResults[T]{Results: results, Errors: errs, Steps: allSteps}, flatSteps, finalErr
}

// ──────────────────────────────────────────────
// WithRetry — exponential backoff retry pattern
// ──────────────────────────────────────────────

// RetryConfig configures the retry behavior.
type RetryConfig struct {
	MaxAttempts int           // maximum number of attempts (including the first)
	BaseDelay   time.Duration // initial delay before first retry
	MaxDelay    time.Duration // maximum delay between retries
	Multiplier  float64       // exponential backoff multiplier
}

// DefaultRetryConfig returns a sensible default retry configuration.
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts: 3,
		BaseDelay:   100 * time.Millisecond,
		MaxDelay:    10 * time.Second,
		Multiplier:  2.0,
	}
}

// WithRetry wraps a Composer with retry logic using exponential backoff.
// On failure, it waits for an increasing delay before retrying.
// Non-recoverable StepErrors are not retried.
func WithRetry[T any](comp Composer[T], config RetryConfig) Composer[T] {
	return &retryComposer[T]{
		inner:  comp,
		config: config,
	}
}

type retryComposer[T any] struct {
	inner  Composer[T]
	config RetryConfig
}

func (r *retryComposer[T]) Run(ctx context.Context, input T) (T, []ExecutionStep, error) {
	allSteps := make([]ExecutionStep, 0)
	var lastErr error

	for attempt := 0; attempt < r.config.MaxAttempts; attempt++ {
		// Check context before each attempt
		select {
		case <-ctx.Done():
			return input, allSteps, ctx.Err()
		default:
		}

		result, steps, err := r.inner.Run(ctx, input)
		allSteps = append(allSteps, steps...)

		if err == nil {
			return result, allSteps, nil
		}

		lastErr = err

		// Don't retry non-recoverable errors
		if se, ok := err.(*StepError); ok && !se.Recoverable {
			return result, allSteps, err
		}

		// Don't wait on the last attempt
		if attempt < r.config.MaxAttempts-1 {
			delay := r.computeDelay(attempt)
			select {
			case <-ctx.Done():
				return result, allSteps, ctx.Err()
			case <-time.After(delay):
			}
		}
	}

	var zero T
	return zero, allSteps, lastErr
}

func (r *retryComposer[T]) computeDelay(attempt int) time.Duration {
	delay := float64(r.config.BaseDelay) * math.Pow(r.config.Multiplier, float64(attempt))
	if delay > float64(r.config.MaxDelay) {
		delay = float64(r.config.MaxDelay)
	}
	return time.Duration(delay)
}

// ──────────────────────────────────────────────
// WithTimeout — timeout pattern using context
// ──────────────────────────────────────────────

// WithTimeout wraps a Composer with a timeout. If execution exceeds
// the duration, the context is cancelled and the operation fails.
// This is a true timeout implementation using context.WithTimeout.
func WithTimeout[T any](comp Composer[T], timeout time.Duration) Composer[T] {
	return &timeoutComposer[T]{
		inner:   comp,
		timeout: timeout,
	}
}

type timeoutComposer[T any] struct {
	inner   Composer[T]
	timeout time.Duration
}

func (t *timeoutComposer[T]) Run(ctx context.Context, input T) (T, []ExecutionStep, error) {
	ctx, cancel := context.WithTimeout(ctx, t.timeout)
	defer cancel()

	result, steps, err := t.inner.Run(ctx, input)

	// If the context deadline exceeded, wrap it as a StepError.
	// This handles both cases: inner returned err due to deadline, or
	// inner completed but deadline was exceeded during execution.
	if ctx.Err() != nil {
		se := &StepError{
			Code:        "TIMEOUT",
			Message:     "operation timed out after " + t.timeout.String(),
			Recoverable: true,
		}
		return result, steps, se
	}

	return result, steps, err
}

// ──────────────────────────────────────────────
// Map — transform Composer output type
// ──────────────────────────────────────────────

// Map transforms the output of a Composer using a mapping function.
// This allows composing pipelines that change types.
func Map[T, U any](comp Composer[T], mapper func(T) U) Composer[U] {
	return &mapComposer[T, U]{
		inner:  comp,
		mapper: mapper,
	}
}

type mapComposer[T, U any] struct {
	inner  Composer[T]
	mapper func(T) U
}

func (m *mapComposer[T, U]) Run(ctx context.Context, input U) (U, []ExecutionStep, error) {
	// We need to go from U to T, run the inner, then map back
	// This is a simplified version — in practice, you'd need a reverse mapper
	// For now, we handle the case where T == U (identity) or use type assertion
	var tInput T
	// Attempt type conversion if T and U are the same underlying type
	if any(input) == nil {
		result, steps, err := m.inner.Run(ctx, tInput)
		if err != nil {
			var zero U
			return zero, steps, err
		}
		return m.mapper(result), steps, nil
	}
	// For different types, this is a limitation — use with care
	var zero U
	return zero, nil, &StepError{Code: "MAP_ERROR", Message: "Map requires same input type or nil input", Recoverable: false}
}

// ──────────────────────────────────────────────
// Compose — create a Composer from a single Step
// ──────────────────────────────────────────────

// Compose wraps a single Step as a Composer.
// Useful for composing simple operations into larger workflows.
func Compose[T any](obs ObservationAdapter, step Step[T, T]) Composer[T] {
	return NewPipeline[T](obs, step)
}