package core

import "context"

// Port is the second primitive: the contract validation gateway.
// It sits at system boundaries, validating input before it enters the system
// and transforming output before it leaves. Every Port is a self-describing
// contract that guarantees type safety and data integrity.
//
// Ports are the first line of defense — they reject invalid data before
// it can enter the pure function core.
type Port[In, Out any] interface {
	// Validate checks the input against the port's contract and returns
	// the validated/transformed output or an error.
	Validate(ctx context.Context, input In) (Out, error)
}

// PortFunc is a convenience adapter wrapping a function into a Port.
type PortFunc[In, Out any] struct {
	validate func(ctx context.Context, input In) (Out, error)
}

// Validate delegates to the underlying function.
func (p PortFunc[In, Out]) Validate(ctx context.Context, input In) (Out, error) {
	return p.validate(ctx, input)
}

// NewPort creates a Port from a validation function.
func NewPort[In, Out any](fn func(ctx context.Context, input In) (Out, error)) Port[In, Out] {
	return PortFunc[In, Out]{validate: fn}
}

// PortAsStep wraps a Port[In, Out] as a Step[In, Out].
func PortAsStep[In, Out any](p Port[In, Out]) Step[In, Out] {
	return StepFunc[In, Out]{
		execute: func(ctx context.Context, input In) (Out, error) {
			return p.Validate(ctx, input)
		},
		unitType: "Port",
	}
}