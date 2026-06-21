//go:build lecore_tier7

package core

import (
	"context"
	"fmt"
	"math/rand"
	"testing"
	"time"
)

// ============================================================================
// 3.2 内存压力测试
// ============================================================================

// TestStress_LargePipeline_1000Steps: 1000步深度Pipeline
func TestStress_LargePipeline_1000Steps(t *testing.T) {
	obs := &InMemoryObservationAdapter{}
	ctx := context.Background()

	steps := make([]Step[int, int], 1000)
	for i := 0; i < 1000; i++ {
		steps[i] = NewStepFunc[int, int]("Atom", func(ctx context.Context, in int) (int, error) {
			return in + 1, nil
		})
	}

	pipeline := NewPipeline[int](obs, steps...)

	start := time.Now()
	result, execSteps, err := pipeline.Run(ctx, 0)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != 1000 {
		t.Errorf("expected 1000, got %d", result)
	}
	if len(execSteps) != 1000 {
		t.Errorf("expected 1000 execution steps, got %d", len(execSteps))
	}
	t.Logf("1000-Step Pipeline: result=%d, steps=%d, elapsed=%v", result, len(execSteps), elapsed)
}

// TestStress_EventStore_HighVolume: 事件存储高容量测试
func TestStress_EventStore_HighVolume(t *testing.T) {
	es := NewEventStore()
	ctx := context.Background()

	const events = 10000
	start := time.Now()

	for i := 0; i < events; i++ {
		envelope := EventEnvelope{
			EventID:     fmt.Sprintf("evt-%d", i),
			AggregateID: "agg-1",
			EventType:   "test",
			EventData:   []byte(fmt.Sprintf(`{"value": %d}`, i)),
			Version:     int64(i + 1),
		}
		_, err := es.Execute(ctx, envelope)
		if err != nil {
			t.Fatalf("unexpected error at event %d: %v", i, err)
		}
	}
	elapsed := time.Since(start)

	streamed := es.StreamAll("agg-1")
	if len(streamed) != events {
		t.Errorf("expected %d events in stream, got %d", events, len(streamed))
	}

	t.Logf("EventStore: %d events in %v, throughput: %.0f events/s",
		events, elapsed, float64(events)/elapsed.Seconds())
}

// TestStress_StepStore_BufferOverflow: 步骤存储环形缓冲区溢出
func TestStress_StepStore_BufferOverflow(t *testing.T) {
	store := NewInMemoryStepStore(1000)

	const records = 10000
	steps := make([]ExecutionStep, records)
	for i := 0; i < records; i++ {
		steps[i] = NewExecutionStep("Atom", "test", fmt.Sprintf("step-%d", i), "stress")
	}

	store.Record(steps)

	count := store.Count()
	if count != 1000 {
		t.Errorf("expected 1000 records, got %d", count)
	}

	queried := store.Query(StepQuery{Limit: 2000})
	if len(queried) != 1000 {
		t.Errorf("expected 1000 queried, got %d", len(queried))
	}

	t.Logf("StepStore RingBuffer: %d records, stored=%d, queried=%d", records, count, len(queried))
}

// TestStress_TDigest_LargeDataset: TDigest大容量数据
func TestStress_TDigest_LargeDataset(t *testing.T) {
	td := NewTDigestDefault()

	const samples = 100000
	rng := rand.New(rand.NewSource(42))

	start := time.Now()
	for i := 0; i < samples; i++ {
		td.Add(rng.Float64() * 1000)
	}
	elapsed := time.Since(start)

	p50 := td.Quantile(0.5)
	p95 := td.Quantile(0.95)
	p99 := td.Quantile(0.99)
	mean := td.Mean()

	if p50 > p95 || p95 > p99 {
		t.Errorf("quantile ordering violated: p50=%.2f, p95=%.2f, p99=%.2f", p50, p95, p99)
	}

	t.Logf("TDigest: %d samples in %v, p50=%.2f, p95=%.2f, p99=%.2f, mean=%.2f",
		samples, elapsed, p50, p95, p99, mean)
}

// TestStress_ObservationPipeline_HighThroughput: 观测管道高吞吐
func TestStress_ObservationPipeline_HighThroughput(t *testing.T) {
	store := NewInMemoryStepStore(10000)
	aggregator := NewAggregator(DefaultAggregatorConfig())

	config := ObservationPipelineConfig{
		BufferSize:    5000,
		Store:         store,
		Aggregator:    aggregator,
		FlushInterval: 100 * time.Millisecond,
	}

	pipeline := NewObservationPipeline(config)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	pipeline.Start(ctx)

	const batches = 100
	const stepsPerBatch = 100

	for i := 0; i < batches; i++ {
		steps := make([]ExecutionStep, stepsPerBatch)
		for j := 0; j < stepsPerBatch; j++ {
			steps[j] = NewExecutionStep("Atom", "process", fmt.Sprintf("batch-%d-step-%d", i, j), "stress")
			if j%10 == 0 {
				steps[j].Error = &StepError{Code: "E001", Message: "injected error"}
			}
		}
		pipeline.Feed(steps)
	}

	time.Sleep(500 * time.Millisecond)
	pipeline.Stop()
	pipeline.Wait()

	count := store.Count()
	t.Logf("ObservationPipeline: %d batches x %d steps, stored=%d", batches, stepsPerBatch, count)
}
