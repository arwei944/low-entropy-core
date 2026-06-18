package core

import "context"

// Adapter is the third primitive: the side-effect boundary.
// All I/O, persistence, logging, external API calls, and other side effects
// MUST go through Adapters. The pure function core (Atom + Composer) never
// calls external systems directly.
//
// Adapters are the only place where non-determinism is allowed.
type Adapter[In, Out any] interface {
	// Execute performs the side-effect operation.
	// Returns the result and any error encountered.
	Execute(ctx context.Context, input In) (Out, error)
}

// AdapterFunc is a convenience adapter wrapping a function into an Adapter.
type AdapterFunc[In, Out any] struct {
	execute func(ctx context.Context, input In) (Out, error)
}

// Execute delegates to the underlying function.
func (a AdapterFunc[In, Out]) Execute(ctx context.Context, input In) (Out, error) {
	return a.execute(ctx, input)
}

// NewAdapter creates an Adapter from a side-effect function.
func NewAdapter[In, Out any](fn func(ctx context.Context, input In) (Out, error)) Adapter[In, Out] {
	return AdapterFunc[In, Out]{execute: fn}
}

// AdapterAsStep wraps an Adapter[In, Out] as a Step[In, Out].
func AdapterAsStep[In, Out any](a Adapter[In, Out]) Step[In, Out] {
	return StepFunc[In, Out]{
		execute: func(ctx context.Context, input In) (Out, error) {
			return a.Execute(ctx, input)
		},
		unitType: "Adapter",
	}
}

