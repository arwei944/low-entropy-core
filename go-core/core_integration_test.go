package core

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

// ──────────────────────────────────────────────
// Integration Tests
// ──────────────────────────────────────────────

func TestIntegration_FullPipeline(t *testing.T) {
	// Simulate a real-world pipeline: Port → Atom → Adapter
	ctx := context.Background()
	obs := &InMemoryObservationAdapter{}

	type Order struct {
		ID     string
		Amount float64
		Status string
	}

	// Port: validate
	validatePort := NewPort[Order, Order](func(ctx context.Context, o Order) (Order, error) {
		if o.Amount <= 0 {
			return o, NewStepError("INVALID_AMOUNT", "amount must be positive", false)
		}
		return o, nil
	})

	// Atom: apply discount
	applyDiscount := func(o Order) Order {
		if o.Amount > 100 {
			o.Amount *= 0.9
		}
		return o
	}

	// Adapter: save to "database"
	var saved Order
	saveAdapter := NewAdapter[Order, Order](func(ctx context.Context, o Order) (Order, error) {
		saved = o
		o.Status = "saved"
		return o, nil
	})

	pipeline := NewPipeline[Order](obs,
		PortAsStep(validatePort),
		AtomAsStep(Atom[Order, Order](applyDiscount)),
		AdapterAsStep(saveAdapter),
	)

	order := Order{ID: "ORD-001", Amount: 200, Status: "new"}
	result, steps, err := pipeline.Run(ctx, order)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != "saved" {
		t.Errorf("expected Status='saved', got '%s'", result.Status)
	}
	if result.Amount != 180 {
		t.Errorf("expected Amount=180, got %f", result.Amount)
	}
	if saved.Amount != 180 {
		t.Errorf("expected saved Amount=180, got %f", saved.Amount)
	}
	if len(steps) != 3 {
		t.Errorf("expected 3 steps, got %d", len(steps))
	}
	// Verify each step has correct UnitType
	expectedUnits := []string{"Port", "Atom", "Adapter"}
	for i, step := range steps {
		if step.Unit != expectedUnits[i] {
			t.Errorf("step %d: expected Unit='%s', got '%s'", i, expectedUnits[i], step.Unit)
		}
		if step.DurationMs < 0 {
			t.Errorf("step %d: DurationMs should be >= 0, got %d", i, step.DurationMs)
		}
	}
}

func TestIntegration_WithTimeoutAndRetry(t *testing.T) {
	ctx := context.Background()
	obs := &InMemoryObservationAdapter{}

	var attempts int32
	comp := NewPipeline[int](obs,
		StepFunc[int, int]{
			execute: func(ctx context.Context, input int) (int, error) {
				count := atomic.AddInt32(&attempts, 1)
				if count < 2 {
					return 0, NewStepError("RETRY", "fail", true)
				}
				return input * 3, nil
			},
			unitType: "Flaky",
		},
	)

	// Wrap with retry then timeout
	retryComp := WithRetry[int](comp, RetryConfig{
		MaxAttempts: 3,
		BaseDelay:   1 * time.Millisecond,
		MaxDelay:    10 * time.Millisecond,
		Multiplier:  2.0,
	})
	timeoutComp := WithTimeout[int](retryComp, 5*time.Second)

	result, _, err := timeoutComp.Run(ctx, 7)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != 21 {
		t.Errorf("expected 21, got %d", result)
	}
	if atomic.LoadInt32(&attempts) != 2 {
		t.Errorf("expected 2 attempts, got %d", attempts)
	}
}

func TestIntegration_BranchWithPipeline(t *testing.T) {
	ctx := context.Background()
	obs := &InMemoryObservationAdapter{}

	// Complex pipeline: Branch → different processing chains
	standardProcess := NewPipeline[string](obs,
		AtomAsStep(Atom[string, string](func(s string) string { return "STD:" + s })),
	)
	priorityProcess := NewPipeline[string](obs,
		AtomAsStep(Atom[string, string](func(s string) string { return "PRI:" + s })),
		AtomAsStep(Atom[string, string](func(s string) string { return s + ":URGENT" })),
	)

	branch := NewBranch[string](
		func(s string) bool { return len(s) > 10 },
		priorityProcess,
		standardProcess,
	)

	// Test priority path
	result, steps, err := branch.Run(ctx, "this-is-a-long-message")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "PRI:this-is-a-long-message:URGENT" {
		t.Errorf("unexpected result: '%s'", result)
	}
	if len(steps) == 0 {
		t.Error("expected child steps, got 0")
	}

	// Test standard path
	result2, steps2, err2 := branch.Run(ctx, "short")
	if err2 != nil {
		t.Fatalf("unexpected error: %v", err2)
	}
	if result2 != "STD:short" {
		t.Errorf("unexpected result: '%s'", result2)
	}
	if len(steps2) == 0 {
		t.Error("expected child steps, got 0")
	}
}
