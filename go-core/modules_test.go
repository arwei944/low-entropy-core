//go:build lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

package core

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
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

// ──────────────────────────────────────────────
// Sampler Tests
// ──────────────────────────────────────────────

func TestRateSampler_Rate1(t *testing.T) {
	sampler := NewRateSampler(1.0)
	for i := 0; i < 100; i++ {
		if !sampler.ShouldKeep(ExecutionStep{}) {
			t.Error("rate=1.0 should always keep")
		}
	}
}

func TestRateSampler_Rate0(t *testing.T) {
	sampler := NewRateSampler(0.0)
	for i := 0; i < 100; i++ {
		if sampler.ShouldKeep(ExecutionStep{}) {
			t.Error("rate=0.0 should never keep")
		}
	}
}

func TestErrorAlwaysSampler(t *testing.T) {
	sampler := NewErrorAlwaysSampler()
	if sampler.ShouldKeep(ExecutionStep{}) {
		t.Error("should not keep step without error")
	}
	if !sampler.ShouldKeep(ExecutionStep{Error: NewStepError("E", "err", true)}) {
		t.Error("should keep step with error")
	}
}

func TestCompositeSampler(t *testing.T) {
	errorSampler := NewErrorAlwaysSampler()
	rateSampler := NewRateSampler(0.0) // never keep on rate
	composite := NewCompositeSampler(errorSampler, rateSampler)

	// Error step: error sampler says keep, rate says drop -> composite says keep
	if !composite.ShouldKeep(ExecutionStep{Error: NewStepError("E", "err", true)}) {
		t.Error("composite should keep error step")
	}
}

func TestSampler_Apply(t *testing.T) {
	steps := []ExecutionStep{
		{Unit: "Atom", Error: NewStepError("E1", "err", true)},
		{Unit: "Port"},
		{Unit: "Adapter"},
		{Unit: "Atom", Error: NewStepError("E2", "err", true)},
		{Unit: "Port"},
	}

	sampler := NewSampler(NewCompositeSampler(NewErrorAlwaysSampler(), NewRateSampler(0.0)))
	kept := sampler.Apply(steps)

	// Should keep 2 error steps + 1 summary step
	if len(kept) < 2 {
		t.Errorf("expected at least 2 kept steps, got %d", len(kept))
	}
	if sampler.DroppedCount() != 3 {
		t.Errorf("expected 3 dropped, got %d", sampler.DroppedCount())
	}
}

// ──────────────────────────────────────────────
// Aggregator Tests
// ──────────────────────────────────────────────

func TestAggregator_Aggregate(t *testing.T) {
	config := AggregatorConfig{
		WindowDurations: []time.Duration{1 * time.Minute},
		MaxWindows:      100,
	}
	agg := NewAggregator(config)

	now := time.Now()
	steps := []ExecutionStep{
		{Timestamp: now, Unit: "Atom", Pattern: "Pipeline", DurationMs: 10},
		{Timestamp: now, Unit: "Atom", Pattern: "Pipeline", DurationMs: 20},
		{Timestamp: now, Unit: "Port", Pattern: "Handoff", DurationMs: 5, Error: NewStepError("E", "err", true)},
		{Timestamp: now, Unit: "Adapter", Pattern: "Pipeline", DurationMs: 30},
		{Timestamp: now, Unit: "Atom", Pattern: "Pipeline", DurationMs: 15},
	}

	results := agg.Aggregate(steps)
	if len(results) == 0 {
		t.Fatal("expected aggregate results")
	}

	// Should have overall aggregate + per-pattern + per-unit
	foundOverall := false
	for _, r := range results {
		if r.Pattern == "" && r.Unit == "" {
			foundOverall = true
			if r.Count != 5 {
				t.Errorf("expected Count=5, got %d", r.Count)
			}
			if r.ErrorCount != 1 {
				t.Errorf("expected ErrorCount=1, got %d", r.ErrorCount)
			}
			break
		}
	}
	if !foundOverall {
		t.Error("expected overall aggregate result")
	}
}

func TestAggregator_QueryResults(t *testing.T) {
	config := AggregatorConfig{
		WindowDurations: []time.Duration{1 * time.Minute},
		MaxWindows:      100,
	}
	agg := NewAggregator(config)
	now := time.Now()
	agg.Aggregate([]ExecutionStep{{Timestamp: now, Unit: "Atom", DurationMs: 10}})

	results := agg.QueryResults("1m0s", "Atom", "")
	if len(results) == 0 {
		t.Error("expected results for Atom unit")
	}

	results = agg.QueryResults("5m0s", "", "")
	if len(results) != 0 {
		t.Error("expected no results for 5m window")
	}
}

func TestAggregator_EmptyInput(t *testing.T) {
	config := DefaultAggregatorConfig()
	agg := NewAggregator(config)
	results := agg.Aggregate([]ExecutionStep{})
	if results != nil {
		t.Error("expected nil for empty input")
	}
}

func TestPercentile(t *testing.T) {
	data := []int64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	p50 := percentile(data, 0.50)
	// idx = int(9 * 0.50) = 4, data[4] = 5
	if p50 != 5 {
		t.Errorf("expected P50=5, got %d", p50)
	}
	p99 := percentile(data, 0.99)
	// idx = int(9 * 0.99) = 8, data[8] = 9
	if p99 != 9 {
		t.Errorf("expected P99=9, got %d", p99)
	}
}

// ──────────────────────────────────────────────
// ObservationPipeline Tests
// ──────────────────────────────────────────────

func TestObservationPipeline_FeedFlush(t *testing.T) {
	store := NewInMemoryStepStore(1000)
	config := ObservationPipelineConfig{
		BufferSize:    10,
		Store:         store,
		Sampler:       nil,
		Aggregator:    nil,
		FlushInterval: 100 * time.Millisecond,
	}

	pipeline := NewObservationPipeline(config)
	ctx, cancel := context.WithCancel(context.Background())
	pipeline.Start(ctx)

	pipeline.Feed([]ExecutionStep{{Unit: "Atom"}, {Unit: "Port"}})
	time.Sleep(200 * time.Millisecond) // Wait for flush

	cancel()
	pipeline.Wait()

	if store.Count() != 2 {
		t.Errorf("expected 2 steps in store, got %d", store.Count())
	}
}

func TestObservationPipeline_FeedNonBlocking(t *testing.T) {
	store := NewInMemoryStepStore(1000)
	config := ObservationPipelineConfig{
		BufferSize:    1,
		Store:         store,
		FlushInterval: 1 * time.Hour, // Long flush, won't drain
	}

	pipeline := NewObservationPipeline(config)
	ctx, cancel := context.WithCancel(context.Background())
	pipeline.Start(ctx)

	// Fill the buffer
	ok := pipeline.FeedNonBlocking([]ExecutionStep{{Unit: "A"}})
	if !ok {
		t.Fatal("first feed should succeed")
	}

	// Buffer is full — should drop
	ok = pipeline.FeedNonBlocking([]ExecutionStep{{Unit: "B"}})
	if ok {
		t.Error("second feed should be dropped (buffer full)")
	}

	cancel()
	pipeline.Wait()
}

func TestObservationPipeline_Stop(t *testing.T) {
	store := NewInMemoryStepStore(1000)
	pipeline := NewObservationPipeline(ObservationPipelineConfig{
		BufferSize:    10,
		Store:         store,
		FlushInterval: 100 * time.Millisecond,
	})

	ctx := context.Background()
	pipeline.Start(ctx)

	if !pipeline.IsRunning() {
		t.Error("expected pipeline to be running")
	}

	pipeline.Stop()
	pipeline.Wait()

	if pipeline.IsRunning() {
		t.Error("expected pipeline to be stopped")
	}
}

// ──────────────────────────────────────────────
// ArchitectureRegistry Tests
// ──────────────────────────────────────────────

func TestArchitectureRegistry_Register(t *testing.T) {
	registry := NewArchitectureRegistry()
	desc := PipelineDescriptor{
		Name:        "test-pipeline",
		Description: "a test pipeline",
		StepCount:   3,
		Patterns:    []string{"Pipeline"},
	}
	registry.Register(desc)

	if registry.Count() != 1 {
		t.Errorf("expected 1 pipeline, got %d", registry.Count())
	}

	retrieved, ok := registry.Get("test-pipeline")
	if !ok {
		t.Fatal("expected pipeline to be found")
	}
	if retrieved.Description != "a test pipeline" {
		t.Errorf("expected 'a test pipeline', got '%s'", retrieved.Description)
	}
}

func TestArchitectureRegistry_RegisterContract(t *testing.T) {
	registry := NewArchitectureRegistry()
	contract := PortContract{
		Name:         "validate-task",
		InputType:    "Task",
		OutputType:   "Task",
		PipelineName: "task-pipeline",
		Validations: []ValidationRule{
			{Field: "ID", Rule: "required", Message: "ID is required"},
		},
	}
	registry.RegisterContract(contract)

	if registry.ContractCount() != 1 {
		t.Errorf("expected 1 contract, got %d", registry.ContractCount())
	}

	retrieved, ok := registry.GetContract("validate-task")
	if !ok {
		t.Fatal("expected contract to be found")
	}
	if retrieved.InputType != "Task" {
		t.Errorf("expected InputType='Task', got '%s'", retrieved.InputType)
	}
}

func TestArchitectureRegistry_List(t *testing.T) {
	registry := NewArchitectureRegistry()
	registry.Register(PipelineDescriptor{Name: "p1"})
	registry.Register(PipelineDescriptor{Name: "p2"})
	registry.Register(PipelineDescriptor{Name: "p3"})

	list := registry.List()
	if len(list) != 3 {
		t.Errorf("expected 3 pipelines, got %d", len(list))
	}
}

// ──────────────────────────────────────────────
// PortContract Tests
// ──────────────────────────────────────────────

func TestDescribePortContract(t *testing.T) {
	port := NewPort[int, string](func(ctx context.Context, input int) (string, error) {
		return fmt.Sprintf("%d", input), nil
	})
	contract := DescribePortContract("int-to-string", port, "test")

	if contract.Name != "int-to-string" {
		t.Errorf("expected Name='int-to-string', got '%s'", contract.Name)
	}
	if contract.InputType != "int" {
		t.Errorf("expected InputType='int', got '%s'", contract.InputType)
	}
	if contract.OutputType != "string" {
		t.Errorf("expected OutputType='string', got '%s'", contract.OutputType)
	}
}

func TestCheckContractCompliance(t *testing.T) {
	contract := PortContract{
		Name:      "validate",
		InputType: "struct { ID string }",
		Validations: []ValidationRule{
			{Field: "ID", Rule: "required", Message: "ID is required"},
		},
	}

	type testStruct struct {
		ID string
	}

	// Valid input
	result := CheckContractCompliance(contract, testStruct{ID: "abc"})
	if !result.Compliant {
		t.Errorf("expected compliant, got violations: %v", result.Violations)
	}

	// Invalid input
	result = CheckContractCompliance(contract, testStruct{ID: ""})
	if result.Compliant {
		t.Error("expected non-compliant for empty ID")
	}
}

// ──────────────────────────────────────────────
// EntropyMetrics Tests
// ──────────────────────────────────────────────

func TestEntropyCollector_Collect(t *testing.T) {
	store := NewInMemoryStepStore(1000)
	store.Record([]ExecutionStep{
		{Unit: "Atom", Pattern: "Pipeline", DurationMs: 10},
		{Unit: "Atom", Pattern: "Pipeline", DurationMs: 20},
		{Unit: "Port", Pattern: "Handoff", DurationMs: 5, Error: NewStepError("E", "err", true)},
		{Unit: "Adapter", Pattern: "Pipeline", DurationMs: 30},
	})

	collector := NewEntropyCollector()
	snap := collector.Collect(store)

	if snap.TotalSteps != 4 {
		t.Errorf("expected TotalSteps=4, got %d", snap.TotalSteps)
	}
	if snap.ErrorSteps != 1 {
		t.Errorf("expected ErrorSteps=1, got %d", snap.ErrorSteps)
	}
	if snap.UniquePatterns != 2 {
		t.Errorf("expected UniquePatterns=2, got %d", snap.UniquePatterns)
	}
	if snap.UniqueUnits != 3 {
		t.Errorf("expected UniqueUnits=3, got %d", snap.UniqueUnits)
	}
	if snap.ErrorRate != 0.25 {
		t.Errorf("expected ErrorRate=0.25, got %f", snap.ErrorRate)
	}
	if snap.EntropyScore <= 0 {
		t.Error("expected positive entropy score")
	}
}

func TestEntropyCollector_Empty(t *testing.T) {
	store := NewInMemoryStepStore(1000)
	collector := NewEntropyCollector()
	snap := collector.Collect(store)

	if snap.TotalSteps != 0 {
		t.Errorf("expected 0 steps, got %d", snap.TotalSteps)
	}
	if snap.EntropyScore != 0 {
		t.Errorf("expected 0 entropy score, got %f", snap.EntropyScore)
	}
}

// ──────────────────────────────────────────────
// AccessControl Tests
// ──────────────────────────────────────────────

func TestAccessControlPort_Valid(t *testing.T) {
	ctx := context.Background()
	secret := []byte("test-secret")
	port := NewAccessControlPort(secret)

	token := NewCapabilityToken("agent-a", []string{"pipeline:write", "pipeline:read"})
	token.Sign(secret)

	req := AccessRequest{
		AgentID: "agent-a",
		Action:   "write",
		Resource: "pipeline",
		Token:    token,
	}

	decision, err := port.Validate(ctx, req)
	if err != nil {
		t.Fatalf("expected access granted, got error: %v", err)
	}
	if !decision.Allowed {
		t.Fatal("expected access allowed")
	}
}

func TestAccessControlPort_NoToken(t *testing.T) {
	ctx := context.Background()
	port := NewAccessControlPort([]byte("secret"))

	req := AccessRequest{AgentID: "agent-a", Action: "read"}
	_, err := port.Validate(ctx, req)
	if err == nil {
		t.Fatal("expected error for missing token")
	}
}

func TestAccessControlPort_AgentIDMismatch(t *testing.T) {
	ctx := context.Background()
	secret := []byte("test")
	port := NewAccessControlPort(secret)

	token := NewCapabilityToken("agent-a", []string{"read"})
	token.Sign(secret)

	req := AccessRequest{AgentID: "agent-b", Action: "read", Token: token}
	_, err := port.Validate(ctx, req)
	if err == nil {
		t.Fatal("expected error for agent ID mismatch")
	}
}

func TestAccessPolicy_CheckAccess(t *testing.T) {
	policy := DefaultAccessPolicy()
	token := NewCapabilityToken("agent-a", []string{"pipeline:read", "pipeline:write"})
	token.Sign([]byte("secret"))

	if !policy.CheckAccess(token, "read") {
		t.Error("expected access for read")
	}
	if !policy.CheckAccess(token, "write") {
		t.Error("expected access for write")
	}
	if policy.CheckAccess(token, "delete") {
		t.Error("should NOT have access for delete")
	}
}

func TestAccessPolicy_Wildcard(t *testing.T) {
	policy := DefaultAccessPolicy()
	token := NewCapabilityToken("admin", []string{"pipeline:*"})
	token.Sign([]byte("secret"))

	if !policy.CheckAccess(token, "read") {
		t.Error("wildcard should grant read")
	}
	if !policy.CheckAccess(token, "delete") {
		t.Error("wildcard should grant delete")
	}
}

// ──────────────────────────────────────────────
// AuditTrail Tests
// ──────────────────────────────────────────────

func TestAuditTrailAdapter_Execute(t *testing.T) {
	ctx := context.Background()
	audit := NewAuditTrailAdapter()

	entry := AuditSuccess("agent-a", "deploy", "pipeline", "p1", "deployed successfully")
	result, err := audit.Execute(ctx, entry)
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if result.Result != "success" {
		t.Errorf("expected success, got %s", result.Result)
	}
	if audit.Count() != 1 {
		t.Errorf("expected 1 entry, got %d", audit.Count())
	}
}

func TestAuditTrailAdapter_Query(t *testing.T) {
	ctx := context.Background()
	audit := NewAuditTrailAdapter()

	audit.Execute(ctx, AuditSuccess("agent-a", "read", "pipeline", "p1", "ok"))
	audit.Execute(ctx, AuditFailure("agent-a", "write", "task", "t1", "failed", nil))
	audit.Execute(ctx, AuditDenied("agent-b", "delete", "pipeline", "p1", "no permission"))

	// Query by agent
	entries := audit.QueryByAgent("agent-a")
	if len(entries) != 2 {
		t.Errorf("expected 2 entries for agent-a, got %d", len(entries))
	}

	// Query by result
	denied := audit.QueryByResult("denied")
	if len(denied) != 1 {
		t.Errorf("expected 1 denied entry, got %d", len(denied))
	}

	// Query by resource
	entries = audit.QueryEntries("", "", "pipeline", "")
	if len(entries) != 2 {
		t.Errorf("expected 2 pipeline entries, got %d", len(entries))
	}
}

func TestAuditTrailAdapter_Clear(t *testing.T) {
	ctx := context.Background()
	audit := NewAuditTrailAdapter()
	audit.Execute(ctx, AuditSuccess("a", "r", "res", "r1", "ok"))
	audit.Clear()
	if audit.Count() != 0 {
		t.Errorf("expected 0 after clear, got %d", audit.Count())
	}
}

func TestAuditTrailAdapter_Concurrency(t *testing.T) {
	ctx := context.Background()
	audit := NewAuditTrailAdapter()
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			audit.Execute(ctx, AuditSuccess(fmt.Sprintf("agent-%d", id), "op", "res", "r1", "ok"))
		}(i)
	}
	wg.Wait()

	if audit.Count() != 100 {
		t.Errorf("expected 100 entries, got %d", audit.Count())
	}
}

// ──────────────────────────────────────────────
// CircuitBreaker Tests
// ──────────────────────────────────────────────

func TestCircuitBreaker_ClosedToOpen(t *testing.T) {
	ctx := context.Background()
	obs := &InMemoryObservationAdapter{}

	failCount := 0
	inner := NewPipeline[int](obs,
		StepFunc[int, int]{
			execute: func(ctx context.Context, input int) (int, error) {
				failCount++
				return 0, NewStepError("FAIL", "always fails", true)
			},
			unitType: "Failing",
		},
	)

	cb := NewCircuitBreaker[int](inner, 3, 100*time.Millisecond)

	// First 3 failures should transition to open
	for i := 0; i < 3; i++ {
		_, _, err := cb.Run(ctx, 1)
		if err == nil {
			t.Fatal("expected error")
		}
	}

	if cb.State() != CircuitOpen {
		t.Errorf("expected CircuitOpen, got %s", cb.State())
	}

	// 4th call should fail immediately with CIRCUIT_OPEN
	_, _, err := cb.Run(ctx, 1)
	if err == nil {
		t.Fatal("expected CIRCUIT_OPEN error")
	}
	if se, ok := err.(*StepError); !ok || se.Code != "CIRCUIT_OPEN" {
		t.Errorf("expected CIRCUIT_OPEN, got %v", err)
	}
}

func TestCircuitBreaker_HalfOpenRecovery(t *testing.T) {
	ctx := context.Background()
	obs := &InMemoryObservationAdapter{}

	failCount := 0
	inner := NewPipeline[int](obs,
		StepFunc[int, int]{
			execute: func(ctx context.Context, input int) (int, error) {
				failCount++
				if failCount <= 3 {
					return 0, NewStepError("FAIL", "fail", true)
				}
				return input * 2, nil
			},
			unitType: "Flaky",
		},
	)

	cb := NewCircuitBreaker[int](inner, 3, 50*time.Millisecond)

	// Fail 3 times to open the circuit
	for i := 0; i < 3; i++ {
		cb.Run(ctx, 1)
	}

	// Wait for half-open transition
	time.Sleep(100 * time.Millisecond)

	// Next call should succeed (half-open → closed)
	result, _, err := cb.Run(ctx, 5)
	if err != nil {
		t.Fatalf("expected recovery, got error: %v", err)
	}
	if result != 10 {
		t.Errorf("expected 10, got %d", result)
	}
	if cb.State() != CircuitClosed {
		t.Errorf("expected CircuitClosed after recovery, got %s", cb.State())
	}
}

// ──────────────────────────────────────────────
// Fallback Tests
// ──────────────────────────────────────────────

func TestFallback_PrimarySucceeds(t *testing.T) {
	ctx := context.Background()
	obs := &InMemoryObservationAdapter{}

	primary := NewPipeline[int](obs,
		AtomAsStep(Atom[int, int](func(x int) int { return x * 2 })),
	)
	fallback := NewPipeline[int](obs,
		AtomAsStep(Atom[int, int](func(x int) int { return x * -1 })),
	)

	fb := NewFallback[int](primary, fallback)
	result, _, err := fb.Run(ctx, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != 10 {
		t.Errorf("expected 10 (primary), got %d", result)
	}
}

func TestFallback_PrimaryFails(t *testing.T) {
	ctx := context.Background()
	obs := &InMemoryObservationAdapter{}

	primary := NewPipeline[int](obs,
		StepFunc[int, int]{
			execute: func(ctx context.Context, input int) (int, error) {
				return 0, NewStepError("FAIL", "primary failed", false)
			},
			unitType: "Failing",
		},
	)
	fallback := NewPipeline[int](obs,
		AtomAsStep(Atom[int, int](func(x int) int { return 999 })),
	)

	fb := NewFallback[int](primary, fallback)
	result, _, err := fb.Run(ctx, 5)
	if err != nil {
		t.Fatalf("fallback should not error: %v", err)
	}
	if result != 999 {
		t.Errorf("expected 999 (fallback), got %d", result)
	}
}

// ──────────────────────────────────────────────
// Bulkhead Tests
// ──────────────────────────────────────────────

func TestBulkhead_WithinLimit(t *testing.T) {
	ctx := context.Background()
	obs := &InMemoryObservationAdapter{}

	inner := NewPipeline[int](obs,
		AtomAsStep(Atom[int, int](func(x int) int { return x * 2 })),
	)
	bh := NewBulkhead[int](inner, 5)

	result, _, err := bh.Run(ctx, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != 20 {
		t.Errorf("expected 20, got %d", result)
	}
}

func TestBulkhead_Exceeded(t *testing.T) {
	ctx := context.Background()
	obs := &InMemoryObservationAdapter{}

	inner := NewPipeline[int](obs,
		StepFunc[int, int]{
			execute: func(ctx context.Context, input int) (int, error) {
				time.Sleep(100 * time.Millisecond)
				return input, nil
			},
			unitType: "Slow",
		},
	)
	bh := NewBulkhead[int](inner, 1)

	// Fill the only slot
	go bh.Run(ctx, 1)
	time.Sleep(10 * time.Millisecond) // Let goroutine start

	// This should be rejected
	_, _, err := bh.Run(ctx, 2)
	if err == nil {
		t.Fatal("expected BULKHEAD_FULL error")
	}
}

func TestBulkhead_Available(t *testing.T) {
	inner := NewPipeline[int](nil, AtomAsStep(Atom[int, int](func(x int) int { return x })))
	bh := NewBulkhead[int](inner, 3)

	if bh.Available() != 3 {
		t.Errorf("expected 3 available, got %d", bh.Available())
	}
}

// ──────────────────────────────────────────────
// RateLimiter Tests
// ──────────────────────────────────────────────

func TestRateLimiter_WithinLimit(t *testing.T) {
	ctx := context.Background()
	obs := &InMemoryObservationAdapter{}

	inner := NewPipeline[int](obs,
		AtomAsStep(Atom[int, int](func(x int) int { return x * 2 })),
	)
	rl := NewRateLimiter[int](inner, 100, 200)

	result, _, err := rl.Run(ctx, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != 20 {
		t.Errorf("expected 20, got %d", result)
	}
}

func TestRateLimiter_Exceeded(t *testing.T) {
	ctx := context.Background()
	obs := &InMemoryObservationAdapter{}

	inner := NewPipeline[int](obs,
		AtomAsStep(Atom[int, int](func(x int) int { return x })),
	)
	rl := NewRateLimiter[int](inner, 1, 1) // 1 token per second

	// Use the only token
	rl.Run(ctx, 1)

	// Second call should be rate limited
	_, _, err := rl.Run(ctx, 2)
	if err == nil {
		t.Fatal("expected RATE_LIMITED error")
	}
}

func TestRateLimiter_Tokens(t *testing.T) {
	inner := NewPipeline[int](nil, AtomAsStep(Atom[int, int](func(x int) int { return x })))
	rl := NewRateLimiter[int](inner, 10, 20)
	tokens := rl.Tokens()
	if tokens < 19 || tokens > 20 {
		t.Errorf("expected ~20 tokens, got %f", tokens)
	}
}

// ──────────────────────────────────────────────
// ResilienceChain Integration Test
// ──────────────────────────────────────────────

func TestResilienceChain_Normal(t *testing.T) {
	ctx := context.Background()
	obs := &InMemoryObservationAdapter{}

	inner := NewPipeline[int](obs,
		AtomAsStep(Atom[int, int](func(x int) int { return x * 3 })),
	)

	config := ResilienceConfig[int]{
		RateLimit:                100,
		RateLimitBurst:           200,
		BulkheadMax:              10,
		CircuitBreakerThreshold:  5,
		CircuitBreakerCooldown:   10 * time.Second,
	}

	chain := ResilienceChain[int](inner, config)
	result, _, err := chain.Run(ctx, 7)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != 21 {
		t.Errorf("expected 21, got %d", result)
	}
}

// ──────────────────────────────────────────────
// Benchmark Tests
// ──────────────────────────────────────────────

func BenchmarkStepStore_Record(b *testing.B) {
	store := NewInMemoryStepStore(100000)
	step := ExecutionStep{Unit: "Atom", Pattern: "Pipeline", DurationMs: 1}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		store.Record([]ExecutionStep{step})
	}
}

func BenchmarkStepStore_Query(b *testing.B) {
	store := NewInMemoryStepStore(10000)
	for i := 0; i < 10000; i++ {
		store.Record([]ExecutionStep{{Unit: "Atom", TraceID: fmt.Sprintf("t%d", i%10)}})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		store.Query(StepQuery{TraceID: "t5", Limit: 100})
	}
}

func BenchmarkAggregator_Aggregate(b *testing.B) {
	config := AggregatorConfig{
		WindowDurations: []time.Duration{1 * time.Minute},
		MaxWindows:      100,
	}
	agg := NewAggregator(config)
	steps := make([]ExecutionStep, 100)
	for i := 0; i < 100; i++ {
		steps[i] = ExecutionStep{Unit: "Atom", Pattern: "Pipeline", DurationMs: int64(i)}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		agg.Aggregate(steps)
	}
}

func BenchmarkEntropyCollector(b *testing.B) {
	store := NewInMemoryStepStore(10000)
	for i := 0; i < 1000; i++ {
		store.Record([]ExecutionStep{{
			Unit:       "Atom",
			Pattern:    fmt.Sprintf("P%d", i%10),
			DurationMs: int64(i % 100),
		}})
	}
	collector := NewEntropyCollector()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		collector.Collect(store)
	}
}

func BenchmarkCircuitBreaker(b *testing.B) {
	ctx := context.Background()
	inner := NewPipeline[int](nil,
		AtomAsStep(Atom[int, int](func(x int) int { return x * 2 })),
	)
	cb := NewCircuitBreaker[int](inner, 100, 1*time.Hour)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cb.Run(ctx, i)
	}
}