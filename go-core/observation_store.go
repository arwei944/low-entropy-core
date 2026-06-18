//go:build lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

package core

import (
	"sort"
	"sync"
	"time"
)

// ──────────────────────────────────────────────
// StepStore — queryable observation storage
// ──────────────────────────────────────────────

// StepStore is a queryable store for ExecutionStep records.
// It supports filtering by TraceID, Pattern, Unit, time range, and error status.
type StepStore interface {
	// Record stores execution steps.
	Record(steps []ExecutionStep)

	// Query retrieves steps matching the given filter.
	Query(q StepQuery) []ExecutionStep

	// Count returns the total number of stored steps.
	Count() int

	// Clear removes all stored steps.
	Clear()
}

// StepQuery defines filter criteria for querying ExecutionSteps.
type StepQuery struct {
	// TraceID filters by trace identifier.
	TraceID string `json:"trace_id,omitempty"`

	// Pattern filters by pattern name.
	Pattern string `json:"pattern,omitempty"`

	// Unit filters by unit type (Atom, Port, Adapter, Composer, etc.).
	Unit string `json:"unit,omitempty"`

	// Since filters steps recorded after this time.
	Since time.Time `json:"since,omitempty"`

	// Limit caps the number of returned steps.
	Limit int `json:"limit,omitempty"`

	// ErrorOnly returns only steps with errors.
	ErrorOnly bool `json:"error_only,omitempty"`
}

// ──────────────────────────────────────────────
// InMemoryStepStore — in-memory implementation
// ──────────────────────────────────────────────

// InMemoryStepStore stores ExecutionSteps in memory with ring-buffer behavior.
// When the capacity is exceeded, the oldest records are overwritten.
// Thread-safe for concurrent use.
type InMemoryStepStore struct {
	mu       sync.RWMutex
	steps    []ExecutionStep
	capacity int
	head     int // write position
	size     int // number of valid entries
}

// NewInMemoryStepStore creates a new in-memory step store with the given capacity.
func NewInMemoryStepStore(capacity int) *InMemoryStepStore {
	if capacity <= 0 {
		capacity = 1000
	}
	return &InMemoryStepStore{
		steps:    make([]ExecutionStep, capacity),
		capacity: capacity,
	}
}

// Record stores execution steps. When capacity is exceeded, oldest records are overwritten.
func (s *InMemoryStepStore) Record(steps []ExecutionStep) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, step := range steps {
		s.steps[s.head] = step
		s.head = (s.head + 1) % s.capacity
		if s.size < s.capacity {
			s.size++
		}
	}
}

// Query retrieves steps matching the filter. Results are ordered by timestamp ascending.
func (s *InMemoryStepStore) Query(q StepQuery) []ExecutionStep {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]ExecutionStep, 0)

	// Build the ordered list of valid entries
	all := s.allOrdered()

	for _, step := range all {
		// Apply filters
		if q.TraceID != "" && step.TraceID != q.TraceID {
			continue
		}
		if q.Pattern != "" && step.Pattern != q.Pattern {
			continue
		}
		if q.Unit != "" && step.Unit != q.Unit {
			continue
		}
		if !q.Since.IsZero() && step.Timestamp.Before(q.Since) {
			continue
		}
		if q.ErrorOnly && step.Error == nil {
			continue
		}
		result = append(result, step)
	}

	// Sort by timestamp ascending
	sort.Slice(result, func(i, j int) bool {
		return result[i].Timestamp.Before(result[j].Timestamp)
	})

	// Apply limit
	if q.Limit > 0 && len(result) > q.Limit {
		result = result[:q.Limit]
	}

	return result
}

// allOrdered returns all valid entries in insertion order (oldest first).
func (s *InMemoryStepStore) allOrdered() []ExecutionStep {
	result := make([]ExecutionStep, 0, s.size)
	if s.size == 0 {
		return result
	}

	// Find the oldest entry
	start := s.head - s.size
	if start < 0 {
		start += s.capacity
	}

	for i := 0; i < s.size; i++ {
		idx := (start + i) % s.capacity
		result = append(result, s.steps[idx])
	}
	return result
}

// Count returns the number of stored steps.
func (s *InMemoryStepStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.size
}

// Clear removes all stored steps.
func (s *InMemoryStepStore) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.head = 0
	s.size = 0
}

// GetSteps returns all stored steps in insertion order.
func (s *InMemoryStepStore) GetSteps() []ExecutionStep {
	return s.Query(StepQuery{})
}