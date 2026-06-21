//go:build lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

package core

import (
	"context"
	"testing"
	"time"
)

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
