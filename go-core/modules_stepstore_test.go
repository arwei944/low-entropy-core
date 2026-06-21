//go:build lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

package core

import (
	"fmt"
	"sync"
	"testing"
)

// ──────────────────────────────────────────────
// StepStore Tests
// ──────────────────────────────────────────────

func TestInMemoryStepStore_RecordQuery(t *testing.T) {
	store := NewInMemoryStepStore(100)
	steps := []ExecutionStep{
		{TraceID: "t1", Unit: "Atom", Pattern: "Pipeline", DurationMs: 10},
		{TraceID: "t2", Unit: "Port", Pattern: "Handoff", DurationMs: 5, Error: NewStepError("E1", "err", true)},
		{TraceID: "t1", Unit: "Adapter", Pattern: "Pipeline", DurationMs: 20},
	}
	store.Record(steps)

	if store.Count() != 3 {
		t.Errorf("expected 3 steps, got %d", store.Count())
	}

	// Query by TraceID
	q1 := store.Query(StepQuery{TraceID: "t1"})
	if len(q1) != 2 {
		t.Errorf("expected 2 steps for t1, got %d", len(q1))
	}

	// Query by Unit
	q2 := store.Query(StepQuery{Unit: "Port"})
	if len(q2) != 1 {
		t.Errorf("expected 1 Port step, got %d", len(q2))
	}

	// Query errors only
	q3 := store.Query(StepQuery{ErrorOnly: true})
	if len(q3) != 1 {
		t.Errorf("expected 1 error step, got %d", len(q3))
	}

	// Query with limit
	q4 := store.Query(StepQuery{Limit: 2})
	if len(q4) != 2 {
		t.Errorf("expected 2 steps (limit), got %d", len(q4))
	}
}

func TestInMemoryStepStore_RingBuffer(t *testing.T) {
	store := NewInMemoryStepStore(10)
	for i := 0; i < 15; i++ {
		store.Record([]ExecutionStep{{TraceID: fmt.Sprintf("t%d", i), Unit: "Atom"}})
	}
	if store.Count() != 10 {
		t.Errorf("expected 10 steps (capacity), got %d", store.Count())
	}
}

func TestInMemoryStepStore_Clear(t *testing.T) {
	store := NewInMemoryStepStore(100)
	store.Record([]ExecutionStep{{Unit: "Atom"}})
	store.Clear()
	if store.Count() != 0 {
		t.Errorf("expected 0 after clear, got %d", store.Count())
	}
}

func TestInMemoryStepStore_GetSteps(t *testing.T) {
	store := NewInMemoryStepStore(100)
	store.Record([]ExecutionStep{{Unit: "Atom"}, {Unit: "Port"}})
	all := store.GetSteps()
	if len(all) != 2 {
		t.Errorf("expected 2 steps, got %d", len(all))
	}
}

func TestInMemoryStepStore_Concurrency(t *testing.T) {
	store := NewInMemoryStepStore(1000)
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			store.Record([]ExecutionStep{{TraceID: fmt.Sprintf("t%d", id), Unit: "Atom"}})
		}(i)
	}
	wg.Wait()
	if store.Count() != 100 {
		t.Errorf("expected 100 steps, got %d", store.Count())
	}
}
