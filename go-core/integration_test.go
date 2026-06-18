//go:build lecore_tier5 || lecore_tier6 || lecore_tier7

package core_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	. "low-entropy-core/go-core"
)

// =============================================================================
// Test 1: Full Pipeline Lifecycle
// =============================================================================

func TestIntegration_V3FullPipeline(t *testing.T) {
	ctx := context.Background()

	// Define a simple Atom (doubles an int) and a Port (validates input > 0)
	doubleAtom := Atom[int, int](func(input int) int { return input * 2 })

	validatePort := NewPort(func(ctx context.Context, input int) (int, error) {
		if input <= 0 {
			return 0, errors.New("must be positive")
		}
		return input, nil
	})

	// Build a Pipeline with Port -> Atom -> Atom (double twice)
	obs := &InMemoryObservationAdapter{}
	pipeline := NewPipeline[int](obs,
		PortAsStep[int, int](validatePort),
		AtomAsStep[int, int](doubleAtom),
		AtomAsStep[int, int](doubleAtom),
	)

	// Run the pipeline with input 5
	result, steps, err := pipeline.Run(ctx, 5)
	if err != nil {
		t.Fatalf("pipeline.Run failed: %v", err)
	}
	if result != 20 {
		t.Errorf("expected 5 -> validate -> double -> double = 20, got %d", result)
	}

	// Verify ExecutionSteps were recorded
	recordedSteps := obs.GetSteps()
	if len(recordedSteps) != 3 {
		t.Errorf("expected 3 recorded steps, got %d", len(recordedSteps))
	}

	// Verify steps from Run() are returned
	if len(steps) != 3 {
		t.Errorf("expected 3 steps from Run(), got %d", len(steps))
	}

	// Verify step unit types
	expectedUnits := []string{"Port", "Atom", "Atom"}
	for i, unit := range expectedUnits {
		if steps[i].Unit != unit {
			t.Errorf("step %d: expected unit %q, got %q", i, unit, steps[i].Unit)
		}
	}

	// Verify TraceTree has the correct structure
	tree := obs.GetTraceTree()
	if tree == nil {
		t.Fatal("expected non-nil trace tree")
	}
	if tree.TotalNodes() != 3 {
		t.Errorf("expected 3 total nodes in trace tree, got %d", tree.TotalNodes())
	}
	// Each step in the pipeline shares the same parent span ID,
	// but since the parent span is not a recorded step, each
	// step appears as a root node in the trace tree.
	if len(tree.Roots) != 3 {
		t.Errorf("expected 3 roots in trace tree, got %d", len(tree.Roots))
	}
	// Verify all steps share the same TraceID
	if len(tree.Roots) > 0 {
		traceID := tree.Roots[0].Step.TraceID
		for i, root := range tree.Roots {
			if root.Step.TraceID != traceID {
				t.Errorf("step %d: expected TraceID %q, got %q", i, traceID, root.Step.TraceID)
			}
		}
	}
}

// =============================================================================
// Test 2: Guardian Decision Engine
// =============================================================================

func TestIntegration_GuardianDecision(t *testing.T) {
	obs := &InMemoryObservationAdapter{}
	engine := NewDecisionEngine(obs)

	// Test 1: All healthy -> Allow
	t.Run("Allow", func(t *testing.T) {
		input := GuardianInput{
			EntropyAlert: EntropyAlert{Level: EntropyOK, Score: 5},
			TranspAlert:  TransparencyAlert{IsHealthy: true, CoverageRate: 100},
			DriftResult:  DriftOutput{DriftScore: 0.0, ShouldQuarantine: false},
			ArchAlert:    ArchitectureAlert{Violations: 0},
		}
		decision, err := engine.Run(context.Background(), input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if decision.Action != ActionAllow {
			t.Errorf("expected ActionAllow, got %s", decision.Action)
		}
		if !decision.AllOK {
			t.Error("expected AllOK=true")
		}
	})

	// Test 2: Yellow entropy + unhealthy transparency -> Warn
	t.Run("Warn", func(t *testing.T) {
		input := GuardianInput{
			EntropyAlert: EntropyAlert{Level: EntropyYellow, Score: 30},
			TranspAlert:  TransparencyAlert{IsHealthy: false, CoverageRate: 50},
			DriftResult:  DriftOutput{DriftScore: 0.1, ShouldQuarantine: false},
			ArchAlert:    ArchitectureAlert{Violations: 0},
		}
		decision, err := engine.Run(context.Background(), input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if decision.Action != ActionWarn {
			t.Errorf("expected ActionWarn, got %s", decision.Action)
		}
	})

	// Test 3: Red entropy -> Rollback
	t.Run("Rollback", func(t *testing.T) {
		input := GuardianInput{
			EntropyAlert: EntropyAlert{Level: EntropyRed, Score: 120},
			TranspAlert:  TransparencyAlert{IsHealthy: true, CoverageRate: 100},
			DriftResult:  DriftOutput{DriftScore: 0.0, ShouldQuarantine: false},
			ArchAlert:    ArchitectureAlert{Violations: 0},
		}
		decision, err := engine.Run(context.Background(), input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if decision.Action != ActionRollback {
			t.Errorf("expected ActionRollback, got %s", decision.Action)
		}
	})

	// Test 4: DriftScore > 0.7 -> Block
	t.Run("Block", func(t *testing.T) {
		input := GuardianInput{
			EntropyAlert: EntropyAlert{Level: EntropyOK, Score: 5},
			TranspAlert:  TransparencyAlert{IsHealthy: true, CoverageRate: 100},
			DriftResult:  DriftOutput{DriftScore: 0.85, ShouldQuarantine: false},
			ArchAlert:    ArchitectureAlert{Violations: 0},
		}
		decision, err := engine.Run(context.Background(), input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if decision.Action != ActionBlock {
			t.Errorf("expected ActionBlock, got %s", decision.Action)
		}
	})

	// Verify the decision engine records ExecutionSteps
	steps := obs.GetSteps()
	if len(steps) != 4 {
		t.Errorf("expected 4 recorded steps (one per decision), got %d", len(steps))
	}
	for i, step := range steps {
		if step.Unit != "DecisionEngine" {
			t.Errorf("step %d: expected unit 'DecisionEngine', got %q", i, step.Unit)
		}
	}
}

// =============================================================================
// Test 3: Event Sourcing
// =============================================================================

func TestIntegration_EventSourcing(t *testing.T) {
	ctx := context.Background()
	store := NewEventStore()

	aggregateID := "agg-001"

	// Append 3 events
	events := []struct {
		eventType string
		data      string
	}{
		{"created", "{}"},
		{"updated", `{"name":"test"}`},
		{"deleted", `{"reason":"done"}`},
	}

	for _, e := range events {
		result, err := store.Execute(ctx, EventEnvelope{
			AggregateID:   aggregateID,
			AggregateType: "TestAggregate",
			EventType:     e.eventType,
			EventData:     []byte(e.data),
		})
		if err != nil {
			t.Fatalf("append event %q failed: %v", e.eventType, err)
		}
		if !result.Success {
			t.Errorf("expected success for event %q", e.eventType)
		}
	}

	// Verify versions are 1, 2, 3
	allEvents := store.StreamAll(aggregateID)
	if len(allEvents) != 3 {
		t.Fatalf("expected 3 events, got %d", len(allEvents))
	}
	for i, expectedVersion := range []int64{1, 2, 3} {
		if allEvents[i].Version != expectedVersion {
			t.Errorf("event %d: expected version %d, got %d", i, expectedVersion, allEvents[i].Version)
		}
	}

	// Stream events and verify order
	streamed := store.Stream(aggregateID, 1)
	if len(streamed) != 3 {
		t.Errorf("expected 3 streamed events from version 1, got %d", len(streamed))
	}
	for i, e := range streamed {
		expectedVersion := int64(i + 1)
		if e.Version != expectedVersion {
			t.Errorf("streamed event %d: expected version %d, got %d", i, expectedVersion, e.Version)
		}
	}

	// Create a Projection that accumulates values
	projection := NewProjection(func(state []byte, event EventEnvelope) ([]byte, error) {
		current := 0
		if len(state) > 0 {
			fmt.Sscanf(string(state), "%d", &current)
		}
		// Accumulate: each event adds its version
		newVal := current + int(event.Version)
		return []byte(fmt.Sprintf("%d", newVal)), nil
	})

	output, err := projection.Execute(ProjectionInput{
		AggregateID:  aggregateID,
		Events:       allEvents,
		FromVersion:  0,
		CurrentState: []byte("0"),
	})
	if err != nil {
		t.Fatalf("projection failed: %v", err)
	}

	// 1 + 2 + 3 = 6
	expectedSum := "6"
	if string(output.State) != expectedSum {
		t.Errorf("expected projection state %q, got %q", expectedSum, string(output.State))
	}
	if output.EventsProcessed != 3 {
		t.Errorf("expected 3 events processed, got %d", output.EventsProcessed)
	}
	if output.Version != 3 {
		t.Errorf("expected version 3, got %d", output.Version)
	}
}

// =============================================================================
// Test 4: Idempotent Port
// =============================================================================

func TestIntegration_Idempotent(t *testing.T) {
	ctx := context.Background()

	// Create a simple inner step
	innerStep := NewStepFunc[int, int]("Test", func(ctx context.Context, input int) (int, error) {
		return input * 2, nil
	})

	store := NewInMemoryIdempotentStore()
	idemPort := NewIdempotentPort[int, int](innerStep, store, 1*time.Minute)

	// Send same key twice -> second call returns FromCache=true
	req1 := IdempotentRequest[int]{Key: "key-1", Input: 5}
	result1, err := idemPort.Validate(ctx, req1)
	if err != nil {
		t.Fatalf("first call failed: %v", err)
	}
	if result1.FromCache {
		t.Error("first call should not be from cache")
	}
	if result1.Output != 10 {
		t.Errorf("expected 10, got %d", result1.Output)
	}

	result2, err := idemPort.Validate(ctx, req1)
	if err != nil {
		t.Fatalf("second call failed: %v", err)
	}
	if !result2.FromCache {
		t.Error("second call with same key should be from cache")
	}
	if result2.Output != 10 {
		t.Errorf("cached result: expected 10, got %d", result2.Output)
	}

	// Send different key -> fresh execution
	req2 := IdempotentRequest[int]{Key: "key-2", Input: 7}
	result3, err := idemPort.Validate(ctx, req2)
	if err != nil {
		t.Fatalf("different key call failed: %v", err)
	}
	if result3.FromCache {
		t.Error("different key should not be from cache")
	}
	if result3.Output != 14 {
		t.Errorf("expected 14, got %d", result3.Output)
	}

	// Verify mutex safety with concurrent calls
	var wg sync.WaitGroup
	concurrentKey := "concurrent-key"
	results := make([]IdempotentResult[int], 10)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			req := IdempotentRequest[int]{Key: concurrentKey, Input: 3}
			res, err := idemPort.Validate(ctx, req)
			if err != nil {
				t.Errorf("concurrent call %d failed: %v", idx, err)
				return
			}
			results[idx] = res
		}(i)
	}
	wg.Wait()

	// All concurrent calls with same key should return the same output
	for i, res := range results {
		if res.Output != 6 {
			t.Errorf("concurrent result %d: expected 6, got %d", i, res.Output)
		}
	}
}

// =============================================================================
// Test 5: Merkle Audit Chain
// =============================================================================

func TestIntegration_MerkleChain(t *testing.T) {
	chain := NewMerkleAuditChain()

	// Append 10 entries
	now := time.Now()
	for i := 0; i < 10; i++ {
		entry := AuditEntry{
			ID:        fmt.Sprintf("entry-%d", i),
			AgentID:   "agent-1",
			Action:    fmt.Sprintf("action-%d", i),
			Resource:  "resource-1",
			Result:    "success",
			Details:   fmt.Sprintf("details-%d", i),
			Timestamp: now.Add(time.Duration(i) * time.Second),
		}
		err := chain.Append(entry)
		if err != nil {
			t.Fatalf("append entry %d failed: %v", i, err)
		}
	}

	// Verify root hash is not empty
	rootHash := chain.RootHash()
	if rootHash == "" {
		t.Error("root hash should not be empty")
	}

	// Verify count
	if chain.Count() != 10 {
		t.Errorf("expected 10 entries, got %d", chain.Count())
	}

	// Generate proof for entry 5 and verify it
	proof, err := chain.GenerateProof(5)
	if err != nil {
		t.Fatalf("generate proof for entry 5 failed: %v", err)
	}
	if proof == nil {
		t.Fatal("expected non-nil proof")
	}
	if proof.EntryIndex != 5 {
		t.Errorf("expected entry index 5, got %d", proof.EntryIndex)
	}

	// Verify the proof against the root hash
	if !VerifyProof(proof, rootHash) {
		t.Error("merkle proof verification failed for valid entry")
	}

	// Tamper with entry 5: create a new chain with tampered data
	// The original root hash should differ from the tampered chain's root hash
	entries := chain.GetEntries()
	originalRootHash := chain.RootHash()

	entries[5].Details = "TAMPERED-DATA"
	tamperedChain := NewMerkleAuditChain()
	for _, e := range entries {
		_ = tamperedChain.Append(e)
	}
	tamperedRootHash := tamperedChain.RootHash()

	// Root hash should change when data is tampered
	if tamperedRootHash == originalRootHash {
		t.Error("root hash should change when data is tampered")
	}

	// Verify proof from original chain fails against tampered root hash
	// (proof was generated against original root hash)
	if VerifyProof(proof, tamperedRootHash) {
		t.Error("merkle proof verification should fail for tampered entry")
	}

	// Verify proof from original chain still works against original root hash
	if !VerifyProof(proof, originalRootHash) {
		t.Error("merkle proof verification failed for valid entry against original root hash")
	}
}

// =============================================================================
// Test 6: Saga Transaction
// =============================================================================

func TestIntegration_SagaTransaction(t *testing.T) {
	ctx := context.Background()
	obs := &InMemoryObservationAdapter{}

	type compensationTracker struct {
		mu           sync.Mutex
		compensated  []string
	}
	tracker := &compensationTracker{}

	composer := NewSagaComposer(obs)

	// Step 1: succeeds
	step1 := SagaStep{
		Name: "step-1",
		Execute: NewStepFunc[any, any]("step1", func(ctx context.Context, input any) (any, error) {
			return "step1-result", nil
		}),
		Compensate: NewStepFunc[any, any]("compensate1", func(ctx context.Context, input any) (any, error) {
			tracker.mu.Lock()
			tracker.compensated = append(tracker.compensated, "step-1")
			tracker.mu.Unlock()
			return nil, nil
		}),
	}

	// Step 2: fails
	step2 := SagaStep{
		Name: "step-2",
		Execute: NewStepFunc[any, any]("step2", func(ctx context.Context, input any) (any, error) {
			return nil, errors.New("step 2 intentional failure")
		}),
		Compensate: NewStepFunc[any, any]("compensate2", func(ctx context.Context, input any) (any, error) {
			tracker.mu.Lock()
			tracker.compensated = append(tracker.compensated, "step-2")
			tracker.mu.Unlock()
			return nil, nil
		}),
	}

	// Step 3: would succeed but never reached
	step3 := SagaStep{
		Name: "step-3",
		Execute: NewStepFunc[any, any]("step3", func(ctx context.Context, input any) (any, error) {
			return "step3-result", nil
		}),
		Compensate: NewStepFunc[any, any]("compensate3", func(ctx context.Context, input any) (any, error) {
			tracker.mu.Lock()
			tracker.compensated = append(tracker.compensated, "step-3")
			tracker.mu.Unlock()
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

	// Verify compensation runs for step 1 (reverse order)
	tracker.mu.Lock()
	compensated := append([]string{}, tracker.compensated...)
	tracker.mu.Unlock()

	if len(compensated) != 1 {
		t.Errorf("expected 1 compensation call, got %d: %v", len(compensated), compensated)
	}
	if len(compensated) > 0 && compensated[0] != "step-1" {
		t.Errorf("expected step-1 to be compensated, got %v", compensated)
	}

	// Verify step 2 and step 3 were NOT compensated (step 2 failed, step 3 never ran)
	for _, c := range compensated {
		if c == "step-2" || c == "step-3" {
			t.Errorf("step %s should not have been compensated", c)
		}
	}
}

// =============================================================================
// Test 7: Degradation Manager
// =============================================================================

func TestIntegration_Degradation(t *testing.T) {
	obs := &InMemoryObservationAdapter{}
	dm := NewDegradationManager(obs)

	// Start in none mode -> all operations allowed
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

	// Degrade to emergency -> only critical operations allowed
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

	// Recover -> all operations allowed again
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

	// Verify execution steps were recorded for degradation events
	steps := obs.GetSteps()
	if len(steps) != 2 {
		t.Errorf("expected 2 degradation steps (degrade + recover), got %d", len(steps))
	}
}

// =============================================================================
// Test 8: FastPipeline vs Normal Pipeline
// =============================================================================

func TestIntegration_FastPipeline(t *testing.T) {
	ctx := context.Background()

	doubleAtom := Atom[int, int](func(input int) int { return input * 2 })
	addOneAtom := Atom[int, int](func(input int) int { return input + 1 })

	doubleStep := NewStepFunc[any, any]("Atom", func(ctx context.Context, input any) (any, error) {
		v, ok := input.(int)
		if !ok {
			return nil, fmt.Errorf("expected int, got %T", input)
		}
		return doubleAtom(v), nil
	})
	addOneStep := NewStepFunc[any, any]("Atom", func(ctx context.Context, input any) (any, error) {
		v, ok := input.(int)
		if !ok {
			return nil, fmt.Errorf("expected int, got %T", input)
		}
		return addOneAtom(v), nil
	})

	// FastPipeline
	fastPipeline := NewFastPipeline[int]("fast-test")
	fastPipeline.AddStep(doubleStep)
	fastPipeline.AddStep(addOneStep)

	fastResult, fastErr := fastPipeline.Run(ctx, 5)
	if fastErr != nil {
		t.Fatalf("FastPipeline.Run failed: %v", fastErr)
	}
	// 5 * 2 = 10, 10 + 1 = 11
	if fastResult != 11 {
		t.Errorf("FastPipeline: expected 11, got %d", fastResult)
	}

	// Normal Pipeline
	obs := &InMemoryObservationAdapter{}
	normalPipeline := NewPipeline[int](obs,
		AtomAsStep[int, int](doubleAtom),
		AtomAsStep[int, int](addOneAtom),
	)
	normalResult, steps, normalErr := normalPipeline.Run(ctx, 5)
	if normalErr != nil {
		t.Fatalf("Pipeline.Run failed: %v", normalErr)
	}
	if normalResult != 11 {
		t.Errorf("Pipeline: expected 11, got %d", normalResult)
	}

	// Both produce the same output
	if fastResult != normalResult {
		t.Errorf("FastPipeline and Pipeline should produce the same output: %d vs %d", fastResult, normalResult)
	}

	// FastPipeline should not produce ExecutionSteps
	// (FastPipeline returns no steps, Pipeline returns 2)
	if len(steps) != 2 {
		t.Errorf("Pipeline should produce 2 ExecutionSteps, got %d", len(steps))
	}
	// FastPipeline does not return steps (it returns nil from FastComposer)
}

// =============================================================================
// Test 9: Tenant Isolation
// =============================================================================

func TestIntegration_TenantIsolation(t *testing.T) {
	ctx := context.Background()
	port := NewTenantIsolationPort()

	// Valid tenant ID -> passes through
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

	// Empty tenant ID -> error
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

// =============================================================================
// Test 10: Stream Processing
// =============================================================================

func TestIntegration_StreamProcessing(t *testing.T) {
	// Create a stream from a slice
	numbers := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	stream := FromSlice(numbers)

	// Apply StreamMap: double each number
	doubled := StreamMap(stream, func(n int) int { return n * 2 })

	// Apply StreamFilter: keep only numbers > 10
	filtered := StreamFilter(doubled, func(n int) bool { return n > 10 })

	// Apply StreamReduce: sum all remaining numbers
	sum := StreamReduce(filtered, 0, func(acc, n int) int { return acc + n })

	// Expected: 2,4,6,8,10,12,14,16,18,20 -> filter >10: 12,14,16,18,20 -> sum = 80
	if sum != 80 {
		t.Errorf("expected sum 80, got %d", sum)
	}

	// Test Collect with a direct stream
	directStream := FromSlice([]string{"a", "b", "c"})
	collected := Collect(directStream)
	if len(collected) != 3 {
		t.Errorf("expected 3 collected items, got %d", len(collected))
	}
	expected := []string{"a", "b", "c"}
	for i, s := range expected {
		if collected[i] != s {
			t.Errorf("collected[%d]: expected %q, got %q", i, s, collected[i])
		}
	}
}