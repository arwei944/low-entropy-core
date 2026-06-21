//go:build lecore_tier5 || lecore_tier6 || lecore_tier7

package core_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	. "low-entropy-core/go-core"
)

func TestIntegration_Idempotent(t *testing.T) {
	ctx := context.Background()

	innerStep := NewStepFunc[int, int]("Test", func(ctx context.Context, input int) (int, error) {
		return input * 2, nil
	})

	store := NewInMemoryIdempotentStore()
	idempotentPort := NewIdempotentPort[int, int](innerStep, store, 1*time.Minute)

	req1 := IdempotentRequest[int]{Key: "key-1", Input: 5}
	result1, err := idempotentPort.Validate(ctx, req1)
	if err != nil {
		t.Fatalf("first call failed: %v", err)
	}
	if result1.FromCache {
		t.Error("first call should not be from cache")
	}
	if result1.Output != 10 {
		t.Errorf("expected 10, got %d", result1.Output)
	}

	result2, err := idempotentPort.Validate(ctx, req1)
	if err != nil {
		t.Fatalf("second call failed: %v", err)
	}
	if !result2.FromCache {
		t.Error("second call with same key should be from cache")
	}
	if result2.Output != 10 {
		t.Errorf("cached result: expected 10, got %d", result2.Output)
	}

	req2 := IdempotentRequest[int]{Key: "key-2", Input: 7}
	result3, err := idempotentPort.Validate(ctx, req2)
	if err != nil {
		t.Fatalf("different key call failed: %v", err)
	}
	if result3.FromCache {
		t.Error("different key should not be from cache")
	}
	if result3.Output != 14 {
		t.Errorf("expected 14, got %d", result3.Output)
	}

	var wg sync.WaitGroup
	concurrentKey := "concurrent-key"
	results := make([]IdempotentResult[int], 10)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			req := IdempotentRequest[int]{Key: concurrentKey, Input: 3}
			res, err := idempotentPort.Validate(ctx, req)
			if err != nil {
				t.Errorf("concurrent call %d failed: %v", idx, err)
				return
			}
			results[idx] = res
		}(i)
	}
	wg.Wait()

	for i, res := range results {
		if res.Output != 6 {
			t.Errorf("concurrent result %d: expected 6, got %d", i, res.Output)
		}
	}
}

func TestIntegration_SagaTransaction(t *testing.T) {
	ctx := context.Background()
	obs := &InMemoryObservationAdapter{}

	type compensationTracker struct {
		compensated []string
	}
	tracker := &compensationTracker{}

	composer := NewSagaComposer(obs)

	step1 := SagaStep{
		Name: "step-1",
		Execute: NewStepFunc[any, any]("step1", func(ctx context.Context, input any) (any, error) {
			return "step1-result", nil
		}),
		Compensate: NewStepFunc[any, any]("compensate1", func(ctx context.Context, input any) (any, error) {
			tracker.compensated = append(tracker.compensated, "step-1")
			return nil, nil
		}),
	}

	step2 := SagaStep{
		Name: "step-2",
		Execute: NewStepFunc[any, any]("step2", func(ctx context.Context, input any) (any, error) {
			return nil, errors.New("step 2 intentional failure")
		}),
		Compensate: NewStepFunc[any, any]("compensate2", func(ctx context.Context, input any) (any, error) {
			tracker.compensated = append(tracker.compensated, "step-2")
			return nil, nil
		}),
	}

	step3 := SagaStep{
		Name: "step-3",
		Execute: NewStepFunc[any, any]("step3", func(ctx context.Context, input any) (any, error) {
			return "step3-result", nil
		}),
		Compensate: NewStepFunc[any, any]("compensate3", func(ctx context.Context, input any) (any, error) {
			tracker.compensated = append(tracker.compensated, "step-3")
			return nil, nil
		}),
	}

	composer.AddStep(step1).AddStep(step2).AddStep(step3)

	_, err := composer.Run(ctx, "initial-input")
	if err == nil {
		t.Fatal("expected error from saga, got nil")
	}
	if err.Error() != "step 2 intentional failure" {
		t.Errorf("expected error 'step 2 intentional failure', got %q", err.Error())
	}

	if len(tracker.compensated) != 1 {
		t.Errorf("expected 1 compensation call, got %d: %v", len(tracker.compensated), tracker.compensated)
	}
	if len(tracker.compensated) > 0 && tracker.compensated[0] != "step-1" {
		t.Errorf("expected step-1 to be compensated, got %v", tracker.compensated)
	}

	for _, c := range tracker.compensated {
		if c == "step-2" || c == "step-3" {
			t.Errorf("step %s should not have been compensated", c)
		}
	}
}

func TestIntegration_Degradation(t *testing.T) {
	obs := &InMemoryObservationAdapter{}
	dm := NewDegradationManager(obs)

	if dm.CurrentMode() != DegradationNone {
		t.Errorf("expected DegradationNone, got %s", dm.CurrentMode())
	}
	if !dm.ShouldProcess("critical") {
		t.Error("critical should be allowed in none mode")
	}
	if !dm.ShouldProcess("high") {
		t.Error("high should be allowed in none mode")
	}
	if !dm.ShouldProcess("normal") {
		t.Error("normal should be allowed in none mode")
	}
	if !dm.ShouldProcess("low") {
		t.Error("low should be allowed in none mode")
	}

	dm.Degrade(DegradationEmergency)
	if dm.CurrentMode() != DegradationEmergency {
		t.Errorf("expected DegradationEmergency, got %s", dm.CurrentMode())
	}
	if !dm.ShouldProcess("critical") {
		t.Error("critical should be allowed in emergency mode")
	}
	if dm.ShouldProcess("high") {
		t.Error("high should NOT be allowed in emergency mode")
	}
	if dm.ShouldProcess("normal") {
		t.Error("normal should NOT be allowed in emergency mode")
	}
	if dm.ShouldProcess("low") {
		t.Error("low should NOT be allowed in emergency mode")
	}

	dm.Recover()
	if dm.CurrentMode() != DegradationNone {
		t.Errorf("expected DegradationNone after recover, got %s", dm.CurrentMode())
	}
	if !dm.ShouldProcess("critical") {
		t.Error("critical should be allowed after recover")
	}
	if !dm.ShouldProcess("high") {
		t.Error("high should be allowed after recover")
	}
	if !dm.ShouldProcess("normal") {
		t.Error("normal should be allowed after recover")
	}
	if !dm.ShouldProcess("low") {
		t.Error("low should be allowed after recover")
	}

	steps := obs.GetSteps()
	if len(steps) != 2 {
		t.Errorf("expected 2 degradation steps (degrade + recover), got %d", len(steps))
	}
}

func TestIntegration_TenantIsolation(t *testing.T) {
	ctx := context.Background()
	port := NewTenantIsolationPort()

	req := TenantRequest{
		TenantID: "tenant-001",
		Request:  map[string]string{"key": "value"},
	}
	result, err := port.Validate(ctx, req)
	if err != nil {
		t.Fatalf("valid tenant ID should pass: %v", err)
	}
	if result.TenantID != "tenant-001" {
		t.Errorf("expected TenantID 'tenant-001', got %q", result.TenantID)
	}

	emptyReq := TenantRequest{
		TenantID: "",
		Request:  "some-data",
	}
	_, err = port.Validate(ctx, emptyReq)
	if err == nil {
		t.Fatal("empty tenant ID should return error")
	}
	stepErr, ok := err.(*StepError)
	if !ok {
		t.Fatalf("expected *StepError, got %T", err)
	}
	if stepErr.Code != "TENANT_ID_EMPTY" {
		t.Errorf("expected code 'TENANT_ID_EMPTY', got %q", stepErr.Code)
	}
}
