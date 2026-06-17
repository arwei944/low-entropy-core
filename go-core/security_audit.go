package core

import (
	"context"
	"sync"
	"time"
)

// ──────────────────────────────────────────────
// AuditTrailAdapter — immutable audit logging
// ──────────────────────────────────────────────

// AuditEntry is a single audit record in the trail.
type AuditEntry struct {
	// ID is a unique identifier for this audit entry.
	ID string `json:"id"`

	// Timestamp is when the event occurred.
	Timestamp time.Time `json:"timestamp"`

	// AgentID is the agent that performed the action.
	AgentID string `json:"agent_id"`

	// Action is the operation performed.
	Action string `json:"action"`

	// Resource is the resource being acted upon.
	Resource string `json:"resource"`

	// ResourceID is the specific resource identifier.
	ResourceID string `json:"resource_id"`

	// Result is the outcome (success, failure, denied).
	Result string `json:"result"`

	// Details provides additional context.
	Details string `json:"details"`

	// TraceID links this audit entry to an execution trace.
	TraceID string `json:"trace_id,omitempty"`

	// PrevHash is the hash of the previous audit entry in the chain.
	PrevHash string `json:"prev_hash,omitempty"`

	// Hash is the SHA-256 hash of this entry's content.
	Hash string `json:"hash,omitempty"`
}

// AuditTrailAdapter is an Adapter that records audit entries.
// It is the only place where audit log side effects occur.
// Thread-safe for concurrent use.
type AuditTrailAdapter struct {
	mu      sync.RWMutex
	entries []AuditEntry
}

// NewAuditTrailAdapter creates a new audit trail adapter.
func NewAuditTrailAdapter() *AuditTrailAdapter {
	return &AuditTrailAdapter{
		entries: make([]AuditEntry, 0),
	}
}

// Execute implements Adapter[AuditEntry, AuditEntry].
// It records the audit entry and returns it unchanged.
func (a *AuditTrailAdapter) Execute(ctx context.Context, input AuditEntry) (AuditEntry, error) {
	if input.ID == "" {
		input.ID = string(NewSpanID())
	}
	if input.Timestamp.IsZero() {
		input.Timestamp = time.Now()
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	a.entries = append(a.entries, input)

	return input, nil
}

// GetEntries returns a copy of all audit entries.
func (a *AuditTrailAdapter) GetEntries() []AuditEntry {
	a.mu.RLock()
	defer a.mu.RUnlock()
	result := make([]AuditEntry, len(a.entries))
	copy(result, a.entries)
	return result
}

// QueryEntries returns entries matching the given filters.
func (a *AuditTrailAdapter) QueryEntries(agentID, action, resource, result string) []AuditEntry {
	a.mu.RLock()
	defer a.mu.RUnlock()

	filtered := make([]AuditEntry, 0)
	for _, entry := range a.entries {
		if agentID != "" && entry.AgentID != agentID {
			continue
		}
		if action != "" && entry.Action != action {
			continue
		}
		if resource != "" && entry.Resource != resource {
			continue
		}
		if result != "" && entry.Result != result {
			continue
		}
		filtered = append(filtered, entry)
	}

	resultEntries := make([]AuditEntry, len(filtered))
	copy(resultEntries, filtered)
	return resultEntries
}

// QueryByAgent returns all entries for a specific agent.
func (a *AuditTrailAdapter) QueryByAgent(agentID string) []AuditEntry {
	return a.QueryEntries(agentID, "", "", "")
}

// QueryByResult returns all entries with a specific result.
func (a *AuditTrailAdapter) QueryByResult(result string) []AuditEntry {
	return a.QueryEntries("", "", "", result)
}

// Count returns the total number of audit entries.
func (a *AuditTrailAdapter) Count() int {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return len(a.entries)
}

// Clear removes all audit entries.
func (a *AuditTrailAdapter) Clear() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.entries = a.entries[:0]
}

// ──────────────────────────────────────────────
// Audit helpers
// ──────────────────────────────────────────────

// NewAuditEntry creates a new AuditEntry with defaults.
func NewAuditEntry(agentID, action, resource, resourceID, result, details string) AuditEntry {
	return AuditEntry{
		ID:         string(NewSpanID()),
		Timestamp:  time.Now(),
		AgentID:    agentID,
		Action:     action,
		Resource:   resource,
		ResourceID: resourceID,
		Result:     result,
		Details:    details,
	}
}

// AuditSuccess creates a success audit entry.
func AuditSuccess(agentID, action, resource, resourceID, details string) AuditEntry {
	return NewAuditEntry(agentID, action, resource, resourceID, "success", details)
}

// AuditFailure creates a failure audit entry.
func AuditFailure(agentID, action, resource, resourceID, details string, err error) AuditEntry {
	detail := details
	if err != nil {
		detail = details + ": " + err.Error()
	}
	return NewAuditEntry(agentID, action, resource, resourceID, "failure", detail)
}

// AuditDenied creates a denied audit entry.
func AuditDenied(agentID, action, resource, resourceID, reason string) AuditEntry {
	return NewAuditEntry(agentID, action, resource, resourceID, "denied", reason)
}

// AuditTrailAsStep wraps the audit trail adapter as a Step.
func AuditTrailAsStep(a *AuditTrailAdapter) Step[AuditEntry, AuditEntry] {
	return AdapterAsStep[AuditEntry, AuditEntry](a)
}