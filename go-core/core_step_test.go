package core

import (
	"context"
	"errors"
	"testing"
)

// ──────────────────────────────────────────────
// Step Tests
// ──────────────────────────────────────────────

func TestStepFunc(t *testing.T) {
	ctx := context.Background()
	step := NewStepFunc[int, int]("Test", func(ctx context.Context, input int) (int, error) {
		return input * 2, nil
	})

	result, err := step.Execute(ctx, 21)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != 42 {
		t.Errorf("expected 42, got %d", result)
	}
	if step.UnitType() != "Test" {
		t.Errorf("expected UnitType='Test', got '%s'", step.UnitType())
	}
}

func TestStepFunc_Error(t *testing.T) {
	ctx := context.Background()
	expectedErr := errors.New("simulated failure")
	step := NewStepFunc[int, int]("Failing", func(ctx context.Context, input int) (int, error) {
		return 0, expectedErr
	})

	_, err := step.Execute(ctx, 1)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err != expectedErr {
		t.Errorf("expected %v, got %v", expectedErr, err)
	}
}

func TestAtomAsStep(t *testing.T) {
	ctx := context.Background()
	atom := func(x string) string { return x + " world" }
	step := AtomAsStep(Atom[string, string](atom))

	result, err := step.Execute(ctx, "hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "hello world" {
		t.Errorf("expected 'hello world', got '%s'", result)
	}
	if step.UnitType() != "Atom" {
		t.Errorf("expected UnitType='Atom', got '%s'", step.UnitType())
	}
}

func TestPortAsStep(t *testing.T) {
	ctx := context.Background()
	port := NewPort[int, int](func(ctx context.Context, input int) (int, error) {
		if input < 0 {
			return 0, NewStepError("NEGATIVE", "negative input", false)
		}
		return input * 10, nil
	})
	step := PortAsStep(port)

	result, err := step.Execute(ctx, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != 50 {
		t.Errorf("expected 50, got %d", result)
	}

	_, err = step.Execute(ctx, -1)
	if err == nil {
		t.Fatal("expected error for negative input, got nil")
	}
}

func TestAdapterAsStep(t *testing.T) {
	ctx := context.Background()
	adapter := NewAdapter[string, string](func(ctx context.Context, input string) (string, error) {
		return "[" + input + "]", nil
	})
	step := AdapterAsStep(adapter)

	result, err := step.Execute(ctx, "data")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "[data]" {
		t.Errorf("expected '[data]', got '%s'", result)
	}
	if step.UnitType() != "Adapter" {
		t.Errorf("expected UnitType='Adapter', got '%s'", step.UnitType())
	}
}
