package core

// ──────────────────────────────────────────────
// 4-Primitive Type Definitions (v2.0)
// ──────────────────────────────────────────────

// Atom is the first primitive: a pure function with no side effects.
// It transforms In to Out deterministically — same input always yields same output.
// No I/O, no randomness, no shared mutable state.
type Atom[In, Out any] func(In) Out

// AtomAny is a compatibility alias for untyped atoms.
// Prefer typed Atom[In, Out] in new code.
type AtomAny = Atom[any, any]

// ──────────────────────────────────────────────
// Shared Error Types
// ──────────────────────────────────────────────

// StepError represents a structured, observable error from any execution step.
// It carries enough context for the observation layer to classify and render errors.
type StepError struct {
	Code        string        `json:"code"`
	Message     string        `json:"message"`
	Recoverable bool          `json:"recoverable"`
	Category    ErrorCategory `json:"category"`
	HTTPStatus  int           `json:"http_status"`
	Err         error         `json:"-"`
}

// Error implements the error interface.
func (e *StepError) Error() string {
	return e.Code + ": " + e.Message
}

// NewStepError creates a StepError with the given code and message.
// If the code matches a known ErrorCode, Category and HTTPStatus are auto-set from it.
// Otherwise, Category is derived from the recoverable flag.
func NewStepError(code, message string, recoverable bool) *StepError {
	ec, found := lookupErrorCode(code)
	category := CategoryUnrecoverable
	if recoverable {
		category = CategoryRecoverable
	}
	httpStatus := 500
	if found {
		category = ec.Category
		httpStatus = ec.HTTPStatus
	}
	return &StepError{
		Code:        code,
		Message:     message,
		Recoverable: recoverable,
		Category:    category,
		HTTPStatus:  httpStatus,
	}
}

// ──────────────────────────────────────────────
// Trace Identity
// ──────────────────────────────────────────────

// TraceID is a unique identifier for a trace spanning multiple steps.
type TraceID string

// SpanID is a unique identifier for a single execution span.
type SpanID string

// NewTraceID generates a UUID v4 string for trace identification.
// Uses the global batched UUID generator for high performance.
func NewTraceID() TraceID {
	return TraceID(getGlobalUUIDGen().NextString())
}

// NewSpanID generates a UUID v4 string for span identification.
func NewSpanID() SpanID {
	return SpanID(getGlobalUUIDGen().NextString())
}

// NewTraceIDCompact generates a new CompactTraceID using the global batched UUID generator.
// Returns a UUID v4 as a CompactTraceID value, zero heap allocation.
// Note: NewCompactTraceID() is defined in perf_core.go (canonical version).
func NewTraceIDCompact() CompactTraceID {
	return CompactTraceID(getGlobalUUIDGen().Next())
}