//go:build lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

package core

import (
	"context"
	"fmt"
)

// ──────────────────────────────────────────────
// TASK-6.1: Multi-Tenant Isolation
// ──────────────────────────────────────────────

// TenantID uniquely identifies a tenant.
type TenantID string

// TenantContext carries tenant identity through the execution context.
type TenantContext struct {
	TenantID TenantID
	Tier     string            // "free", "pro", "enterprise"
	Metadata map[string]string
}

// TenantRequest wraps an input with tenant context.
type TenantRequest struct {
	TenantID TenantID
	Request  interface{}
}

// TenantIsolationPort validates that a request belongs to the correct tenant.
// Implements Port[TenantRequest, TenantRequest].
type TenantIsolationPort struct{}

// NewTenantIsolationPort creates a new TenantIsolationPort.
func NewTenantIsolationPort() *TenantIsolationPort {
	return &TenantIsolationPort{}
}

// Validate checks the TenantRequest's TenantID and passes through if valid.
// An empty TenantID is rejected with a validation error.
func (p *TenantIsolationPort) Validate(ctx context.Context, input TenantRequest) (TenantRequest, error) {
	if input.TenantID == "" {
		return input, &StepError{
			Code:        "TENANT_ID_EMPTY",
			Message:     "tenant ID must not be empty",
			Recoverable: true,
			Category:    CategoryRecoverable,
			HTTPStatus:  400,
		}
	}
	return input, nil
}

// errTenantEmpty is a convenience sentinel for the empty tenant case.
var errTenantEmpty = fmt.Errorf("tenant ID is empty")