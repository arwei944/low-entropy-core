//go:build lecore_tier3 || lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

package core

import (
	"context"
	"sync"
	"time"
)

// ============================================================================
// SECTION 3: Audit Trail — immutable audit logging
// ============================================================================

type AuditEntry struct {
	ID         string    `json:"id"`
	Timestamp  time.Time `json:"timestamp"`
	AgentID    string    `json:"agent_id"`
	Action     string    `json:"action"`
	Resource   string    `json:"resource"`
	ResourceID string    `json:"resource_id"`
	Result     string    `json:"result"`
	Details    string    `json:"details"`
	TraceID    string    `json:"trace_id,omitempty"`
	PrevHash   string    `json:"prev_hash,omitempty"`
	Hash       string    `json:"hash,omitempty"`
}

type AuditTrailAdapter struct {
	mu      sync.RWMutex
	entries []AuditEntry
}

func NewAuditTrailAdapter() *AuditTrailAdapter {
	return &AuditTrailAdapter{entries: make([]AuditEntry, 0)}
}

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

func (a *AuditTrailAdapter) GetEntries() []AuditEntry {
	a.mu.RLock()
	defer a.mu.RUnlock()
	result := make([]AuditEntry, len(a.entries))
	copy(result, a.entries)
	return result
}

func (a *AuditTrailAdapter) QueryEntries(agentID, action, resource, result string) []AuditEntry {
	a.mu.RLock()
	defer a.mu.RUnlock()
	filtered := make([]AuditEntry, 0)
	for _, entry := range a.entries {
		if agentID != "" && entry.AgentID != agentID { continue }
		if action != "" && entry.Action != action { continue }
		if resource != "" && entry.Resource != resource { continue }
		if result != "" && entry.Result != result { continue }
		filtered = append(filtered, entry)
	}
	resultEntries := make([]AuditEntry, len(filtered))
	copy(resultEntries, filtered)
	return resultEntries
}

func (a *AuditTrailAdapter) QueryByAgent(agentID string) []AuditEntry {
	return a.QueryEntries(agentID, "", "", "")
}

func (a *AuditTrailAdapter) QueryByResult(result string) []AuditEntry {
	return a.QueryEntries("", "", "", result)
}

func (a *AuditTrailAdapter) Count() int {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return len(a.entries)
}

func (a *AuditTrailAdapter) Clear() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.entries = a.entries[:0]
}

func NewAuditEntry(agentID, action, resource, resourceID, result, details string) AuditEntry {
	return AuditEntry{
		ID: string(NewSpanID()), Timestamp: time.Now(),
		AgentID: agentID, Action: action, Resource: resource,
		ResourceID: resourceID, Result: result, Details: details,
	}
}

func AuditSuccess(agentID, action, resource, resourceID, details string) AuditEntry {
	return NewAuditEntry(agentID, action, resource, resourceID, "success", details)
}

func AuditFailure(agentID, action, resource, resourceID, details string, err error) AuditEntry {
	detail := details
	if err != nil {
		detail = details + ": " + err.Error()
	}
	return NewAuditEntry(agentID, action, resource, resourceID, "failure", detail)
}

func AuditDenied(agentID, action, resource, resourceID, reason string) AuditEntry {
	return NewAuditEntry(agentID, action, resource, resourceID, "denied", reason)
}

func AuditTrailAsStep(a *AuditTrailAdapter) Step[AuditEntry, AuditEntry] {
	return AdapterAsStep[AuditEntry, AuditEntry](a)
}
