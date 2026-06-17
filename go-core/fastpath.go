package core

import "context"

// ──────────────────────────────────────────────
// FastPipeline — zero-allocation hot path
// ──────────────────────────────────────────────

// FastPipeline is a zero-allocation pipeline that skips observation recording.
// It implements Composer[T] for hot-path execution.
type FastPipeline[T any] struct {
	steps []Step[any, any]
	name  string
}

// NewFastPipeline creates a new FastPipeline with the given name.
func NewFastPipeline[T any](name string) *FastPipeline[T] {
	return &FastPipeline[T]{
		name:  name,
		steps: make([]Step[any, any], 0),
	}
}

// AddStep adds a step to the pipeline. Returns self for chaining.
func (p *FastPipeline[T]) AddStep(step Step[any, any]) *FastPipeline[T] {
	p.steps = append(p.steps, step)
	return p
}

// Run executes all steps in sequence without any observation recording.
// It checks context cancellation before each step. If any step returns
// an error, execution stops and the error is propagated immediately.
// No ExecutionStep records are produced — this is the zero-allocation hot path.
func (p *FastPipeline[T]) Run(ctx context.Context, input T) (T, error) {
	// Check context cancellation before starting
	select {
	case <-ctx.Done():
		var zero T
		return zero, ctx.Err()
	default:
	}

	// Use any for the intermediate value to work with Step[any, any]
	var current any = input
	for _, step := range p.steps {
		// Check context cancellation before each step
		select {
		case <-ctx.Done():
			var zero T
			return zero, ctx.Err()
		default:
		}

		output, err := step.Execute(ctx, current)
		if err != nil {
			var zero T
			return zero, err
		}
		current = output
	}

	// Convert the final value back to T
	result, ok := current.(T)
	if !ok {
		var zero T
		return zero, &StepError{
			Code:        "TYPE_ERROR",
			Message:     "FastPipeline: final type assertion failed",
			Recoverable: false,
		}
	}
	return result, nil
}

// GetSteps returns the list of steps in this pipeline.
func (p *FastPipeline[T]) GetSteps() []Step[any, any] {
	return p.steps
}

// GetName returns the pipeline name.
func (p *FastPipeline[T]) GetName() string {
	return p.name
}

// ──────────────────────────────────────────────
// fastComposerAdapter — adapts FastPipeline to Composer[T]
// ──────────────────────────────────────────────

// fastComposerAdapter wraps FastPipeline to satisfy the Composer[T] interface.
// It delegates Run to FastPipeline and returns nil for ExecutionSteps,
// ensuring zero observation overhead.
type fastComposerAdapter[T any] struct {
	pipeline *FastPipeline[T]
}

// Run executes the pipeline and returns nil ExecutionSteps.
func (a *fastComposerAdapter[T]) Run(ctx context.Context, input T) (T, []ExecutionStep, error) {
	result, err := a.pipeline.Run(ctx, input)
	return result, nil, err
}

// FastComposer wraps FastPipeline to satisfy the Composer[T] interface.
func FastComposer[T any](pipeline *FastPipeline[T]) Composer[T] {
	return &fastComposerAdapter[T]{pipeline: pipeline}
}