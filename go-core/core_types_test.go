package core

import "testing"

// ──────────────────────────────────────────────
// Types Tests
// ──────────────────────────────────────────────

func TestStepError(t *testing.T) {
	err := NewStepError("TEST_ERR", "test message", true)
	if err.Code != "TEST_ERR" {
		t.Errorf("expected Code='TEST_ERR', got '%s'", err.Code)
	}
	if err.Message != "test message" {
		t.Errorf("expected Message='test message', got '%s'", err.Message)
	}
	if !err.Recoverable {
		t.Error("expected Recoverable=true")
	}
	if err.Error() != "TEST_ERR: test message" {
		t.Errorf("expected Error()='TEST_ERR: test message', got '%s'", err.Error())
	}
}

func TestNewTraceID_Uniqueness(t *testing.T) {
	ids := make(map[TraceID]bool, 1000)
	for i := 0; i < 1000; i++ {
		id := NewTraceID()
		if ids[id] {
			t.Errorf("duplicate TraceID generated: %s", id)
		}
		ids[id] = true
	}
}

func TestNewSpanID_Uniqueness(t *testing.T) {
	ids := make(map[SpanID]bool, 1000)
	for i := 0; i < 1000; i++ {
		id := NewSpanID()
		if ids[id] {
			t.Errorf("duplicate SpanID generated: %s", id)
		}
		ids[id] = true
	}
}

func TestAtomType(t *testing.T) {
	// Verify Atom works as a typed function
	double := func(x int) int { return x * 2 }
	var a Atom[int, int] = double
	result := a(21)
	if result != 42 {
		t.Errorf("expected 42, got %d", result)
	}
}

func TestAtomAny(t *testing.T) {
	// Verify AtomAny compatibility
	var a AtomAny = func(x any) any { return x }
	result := a("hello")
	if result != "hello" {
		t.Errorf("expected 'hello', got %v", result)
	}
}
