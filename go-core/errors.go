package core

import (
	"errors"
	"fmt"
)

// ──────────────────────────────────────────────
// Error Category
// ──────────────────────────────────────────────

// ErrorCategory classifies the nature of an error.
type ErrorCategory string

const (
	CategoryRecoverable   ErrorCategory = "recoverable"
	CategoryUnrecoverable ErrorCategory = "unrecoverable"
	CategoryHumanRequired ErrorCategory = "human_required"
)

// ──────────────────────────────────────────────
// Error Code
// ──────────────────────────────────────────────

// ErrorCode is a standardized error code.
type ErrorCode struct {
	Code           string
	Category       ErrorCategory
	HTTPStatus     int
	DefaultMessage string
}

// Standard error codes.
var (
	ErrValidationFailed   = ErrorCode{"VALIDATION_FAILED", CategoryRecoverable, 400, "validation failed"}
	ErrIOError            = ErrorCode{"IO_ERROR", CategoryUnrecoverable, 500, "I/O error"}
	ErrTimeout            = ErrorCode{"TIMEOUT", CategoryRecoverable, 504, "operation timed out"}
	ErrRateLimited        = ErrorCode{"RATE_LIMITED", CategoryRecoverable, 429, "rate limit exceeded"}
	ErrCircuitOpen        = ErrorCode{"CIRCUIT_OPEN", CategoryRecoverable, 503, "circuit breaker open"}
	ErrAuthFailed         = ErrorCode{"AUTH_FAILED", CategoryRecoverable, 403, "authentication failed"}
	ErrSchemaIncompatible = ErrorCode{"SCHEMA_INCOMPATIBLE", CategoryHumanRequired, 400, "schema incompatible"}
	ErrEntropyHigh        = ErrorCode{"ENTROPY_HIGH", CategoryHumanRequired, 500, "system entropy too high"}
	ErrDriftDetected      = ErrorCode{"DRIFT_DETECTED", CategoryHumanRequired, 500, "agent drift detected"}
	ErrIdempotentConflict = ErrorCode{"IDEMPOTENT_CONFLICT", CategoryRecoverable, 409, "idempotency conflict"}
	ErrNotFound           = ErrorCode{"NOT_FOUND", CategoryRecoverable, 404, "resource not found"}
	ErrInternal           = ErrorCode{"INTERNAL_ERROR", CategoryUnrecoverable, 500, "internal error"}
)

// errorCodeLookup maps code strings to ErrorCode for fast lookup by NewStepError.
var errorCodeLookup = map[string]ErrorCode{
	ErrValidationFailed.Code:   ErrValidationFailed,
	ErrIOError.Code:            ErrIOError,
	ErrTimeout.Code:            ErrTimeout,
	ErrRateLimited.Code:        ErrRateLimited,
	ErrCircuitOpen.Code:        ErrCircuitOpen,
	ErrAuthFailed.Code:         ErrAuthFailed,
	ErrSchemaIncompatible.Code: ErrSchemaIncompatible,
	ErrEntropyHigh.Code:        ErrEntropyHigh,
	ErrDriftDetected.Code:      ErrDriftDetected,
	ErrIdempotentConflict.Code: ErrIdempotentConflict,
	ErrNotFound.Code:           ErrNotFound,
	ErrInternal.Code:           ErrInternal,
}

// lookupErrorCode returns the ErrorCode for a given code string.
// This is used by NewStepError in types.go to auto-set Category and HTTPStatus.
func lookupErrorCode(code string) (ErrorCode, bool) {
	ec, ok := errorCodeLookup[code]
	return ec, ok
}

// ──────────────────────────────────────────────
// StepError Constructors
// ──────────────────────────────────────────────

// NewStepErrorWithCode creates a StepError from an ErrorCode with a custom message.
// If message is empty, the ErrorCode's DefaultMessage is used.
func NewStepErrorWithCode(ec ErrorCode, message string) *StepError {
	if message == "" {
		message = ec.DefaultMessage
	}
	recoverable := ec.Category == CategoryRecoverable
	return &StepError{
		Code:        ec.Code,
		Message:     message,
		Recoverable: recoverable,
		Category:    ec.Category,
		HTTPStatus:  ec.HTTPStatus,
	}
}

// NewStepErrorWithWrap creates a StepError that wraps an existing error.
// If message is empty, the ErrorCode's DefaultMessage is used.
func NewStepErrorWithWrap(ec ErrorCode, message string, err error) *StepError {
	if message == "" {
		message = ec.DefaultMessage
	}
	recoverable := ec.Category == CategoryRecoverable
	return &StepError{
		Code:        ec.Code,
		Message:     message,
		Recoverable: recoverable,
		Category:    ec.Category,
		HTTPStatus:  ec.HTTPStatus,
		Err:         err,
	}
}

// ──────────────────────────────────────────────
// StepError Methods (error interface extensions)
// ──────────────────────────────────────────────

// Unwrap returns the wrapped error, enabling errors.Is and errors.As support.
func (e *StepError) Unwrap() error {
	return e.Err
}

// Is checks if target is a StepError with the same Code.
// This enables errors.Is(err, &StepError{Code: "TIMEOUT"}) to match by code.
func (e *StepError) Is(target error) bool {
	t, ok := target.(*StepError)
	if !ok {
		return false
	}
	return e.Code == t.Code
}

// ──────────────────────────────────────────────
// Error Helpers
// ──────────────────────────────────────────────

// IsErrorCode checks if any StepError in the error chain has the given code.
// It walks the error chain using errors.As to support wrapped errors.
func IsErrorCode(err error, code string) bool {
	var se *StepError
	if errors.As(err, &se) {
		return se.Code == code
	}
	return false
}

// AggregateErrors combines multiple errors into one.
// Returns nil for an empty slice, the single error for one element,
// and a joined error for multiple elements.
func AggregateErrors(errs []error) error {
	switch len(errs) {
	case 0:
		return nil
	case 1:
		return errs[0]
	default:
		return errors.Join(errs...)
	}
}

// ──────────────────────────────────────────────
// ErrorCode Methods
// ──────────────────────────────────────────────

// Code returns the error code string.
func (ec ErrorCode) String() string {
	return ec.Code
}

// Format implements fmt.Formatter for ErrorCode.
func (ec ErrorCode) Format(f fmt.State, verb rune) {
	switch verb {
	case 's':
		fmt.Fprint(f, ec.Code)
	case 'v':
		fmt.Fprint(f, ec.Code)
	default:
		fmt.Fprint(f, ec.Code)
	}
}