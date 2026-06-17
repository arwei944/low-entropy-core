package core

import "context"

// Step is the unified execution unit that all primitives can be wrapped into.
// It provides a consistent interface for the Composer to orchestrate any unit
// without knowing its concrete type.
//
// Every primitive (Atom, Port, Adapter) can be adapted to Step[In, Out]
// for uniform composition.
type Step[In, Out any] interface {
	// Execute runs the step with the given input and context.
	// Returns the output and any error encountered.
	Execute(ctx context.Context, input In) (Out, error)

	// UnitType identifies the primitive type backing this step.
	// One of: "Atom", "Port", "Adapter", "Composer"
	UnitType() string
}

// StepFunc is a convenience adapter that converts a function pair
// into a Step[In, Out]. Useful for inline step definitions and testing.
type StepFunc[In, Out any] struct {
	execute  func(ctx context.Context, input In) (Out, error)
	unitType string
}

// Execute delegates to the underlying function.
func (s StepFunc[In, Out]) Execute(ctx context.Context, input In) (Out, error) {
	return s.execute(ctx, input)
}

// UnitType returns the primitive type label.
func (s StepFunc[In, Out]) UnitType() string {
	return s.unitType
}

// NewStepFunc creates a Step from a function and unit type label.
func NewStepFunc[In, Out any](unitType string, fn func(ctx context.Context, input In) (Out, error)) Step[In, Out] {
	return StepFunc[In, Out]{execute: fn, unitType: unitType}
}

// AtomAsStep wraps an Atom[In, Out] as a Step[In, Out].
func AtomAsStep[In, Out any](a Atom[In, Out]) Step[In, Out] {
	return StepFunc[In, Out]{
		execute: func(ctx context.Context, input In) (Out, error) {
			return a(input), nil
		},
		unitType: "Atom",
	}
}