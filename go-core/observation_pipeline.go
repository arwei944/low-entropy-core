package core

import (
	"context"
	"sync"
	"time"
)

// ──────────────────────────────────────────────
// ObservationPipeline — L0→L5 data pipeline
// ──────────────────────────────────────────────

// ObservationPipeline orchestrates the full observation data flow:
//   Buffer(L0) → Sampler(L1) → Aggregator(L2) → Store(L3/L4)
//
// All components are connected via Go channels. The pipeline runs as a
// background goroutine and can be cancelled via context.
type ObservationPipeline struct {
	cfg       ObservationPipelineConfig
	store     StepStore
	sampler   *Sampler
	aggregator *Aggregator

	input chan []ExecutionStep
	done  chan struct{}

	mu      sync.Mutex
	running bool
}

// ObservationPipelineConfig configures the observation pipeline.
type ObservationPipelineConfig struct {
	// BufferSize is the capacity of the input channel (default: 100).
	BufferSize int

	// Store is the persistent step store.
	Store StepStore

	// Sampler is the sampling policy (nil = keep all).
	Sampler *Sampler

	// Aggregator is the time-window aggregator (nil = no aggregation).
	Aggregator *Aggregator

	// FlushInterval is how often to flush the buffer to downstream stages.
	FlushInterval time.Duration
}

// DefaultObservationPipelineConfig returns a sensible default config.
func DefaultObservationPipelineConfig() ObservationPipelineConfig {
	return ObservationPipelineConfig{
		BufferSize:    100,
		Store:         NewInMemoryStepStore(10000),
		Sampler:       NewSampler(NewCompositeSampler(NewErrorAlwaysSampler(), NewRateSampler(0.1))),
		Aggregator:    NewAggregator(DefaultAggregatorConfig()),
		FlushInterval: 1 * time.Second,
	}
}

// NewObservationPipeline creates a new observation pipeline.
func NewObservationPipeline(config ObservationPipelineConfig) *ObservationPipeline {
	if config.BufferSize <= 0 {
		config.BufferSize = 100
	}
	if config.Store == nil {
		config.Store = NewInMemoryStepStore(10000)
	}
	if config.FlushInterval <= 0 {
		config.FlushInterval = 1 * time.Second
	}

	return &ObservationPipeline{
		cfg:        config,
		store:      config.Store,
		sampler:    config.Sampler,
		aggregator: config.Aggregator,
		input:      make(chan []ExecutionStep, config.BufferSize),
		done:       make(chan struct{}),
	}
}

// Start launches the pipeline goroutine. It processes steps from the input
// channel and flushes them through the sampling, aggregation, and storage stages.
// The pipeline stops when ctx is cancelled.
func (p *ObservationPipeline) Start(ctx context.Context) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.running {
		return
	}
	p.running = true

	go func() {
		defer close(p.done)
		buffer := make([]ExecutionStep, 0, 1000)
		ticker := time.NewTicker(p.cfg.FlushInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				// Flush remaining buffer
				p.flush(buffer)
				return

			case steps, ok := <-p.input:
				if !ok {
					// Channel closed — flush and exit
					p.flush(buffer)
					return
				}
				buffer = append(buffer, steps...)

			case <-ticker.C:
				if len(buffer) > 0 {
					p.flush(buffer)
					buffer = buffer[:0]
				}
			}
		}
	}()
}

// flush processes a batch through the pipeline stages.
func (p *ObservationPipeline) flush(steps []ExecutionStep) {
	if len(steps) == 0 {
		return
	}

	// L1: Sampling
	kept := steps
	if p.sampler != nil {
		kept = p.sampler.Apply(steps)
	}

	// L2: Aggregation
	if p.aggregator != nil {
		p.aggregator.Aggregate(steps)
	}

	// L3/L4: Store
	if p.store != nil {
		p.store.Record(kept)
	}
}

// Feed submits steps into the pipeline. Blocks if the buffer is full.
func (p *ObservationPipeline) Feed(steps []ExecutionStep) {
	p.input <- steps
}

// FeedNonBlocking submits steps without blocking. Drops if buffer is full.
func (p *ObservationPipeline) FeedNonBlocking(steps []ExecutionStep) bool {
	select {
	case p.input <- steps:
		return true
	default:
		return false
	}
}

// Stop gracefully stops the pipeline by closing the input channel.
func (p *ObservationPipeline) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.running {
		return
	}
	p.running = false
	close(p.input)
}

// Wait blocks until the pipeline goroutine exits.
func (p *ObservationPipeline) Wait() {
	<-p.done
}

// Store returns the underlying StepStore.
func (p *ObservationPipeline) Store() StepStore {
	return p.store
}

// Aggregator returns the underlying Aggregator.
func (p *ObservationPipeline) Aggregator() *Aggregator {
	return p.aggregator
}

// Sampler returns the underlying Sampler.
func (p *ObservationPipeline) Sampler() *Sampler {
	return p.sampler
}

// IsRunning returns whether the pipeline is running.
func (p *ObservationPipeline) IsRunning() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.running
}