package core

import (
	"context"
	"fmt"
	"sync"
)

// ──────────────────────────────────────────────
// AdapterResolver — resolves adapters by name and environment
// ──────────────────────────────────────────────

// AdapterResolver resolves adapter names to Adapter instances based on environment.
// For example: env=dev resolves "store" to InMemoryStepStore, env=prod resolves to PostgresStore.
type AdapterResolver interface {
	Resolve(name string, env string) (interface{}, error)
	Register(name string, env string, factory interface{})
}

// ──────────────────────────────────────────────
// MapAdapterResolver — simple map-based resolver
// ──────────────────────────────────────────────

// MapAdapterResolver is a simple map-based AdapterResolver.
// It stores adapters keyed by "name:env" and is safe for concurrent use.
type MapAdapterResolver struct {
	mu       sync.RWMutex
	adapters map[string]interface{} // key: "name:env"
}

// NewMapAdapterResolver creates a new MapAdapterResolver.
func NewMapAdapterResolver() *MapAdapterResolver {
	return &MapAdapterResolver{
		adapters: make(map[string]interface{}),
	}
}

// key builds the lookup key from name and environment.
func (r *MapAdapterResolver) key(name, env string) string {
	return name + ":" + env
}

// Resolve looks up an adapter by name and environment.
// Returns an error if no adapter is registered for the given name:env pair.
func (r *MapAdapterResolver) Resolve(name string, env string) (interface{}, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	k := r.key(name, env)
	adapter, ok := r.adapters[k]
	if !ok {
		return nil, fmt.Errorf("MapAdapterResolver: no adapter registered for %q (env=%s)", name, env)
	}
	return adapter, nil
}

// Register stores an adapter factory for the given name and environment.
func (r *MapAdapterResolver) Register(name string, env string, factory interface{}) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.adapters[r.key(name, env)] = factory
}

// ──────────────────────────────────────────────
// PipelineBuilder — builds Composer from PipelineConfig
// ──────────────────────────────────────────────

// PipelineBuilder builds Composer instances from PipelineConfig.
type PipelineBuilder struct {
	resolver AdapterResolver
	obs      ObservationAdapter
}

// NewPipelineBuilder creates a new PipelineBuilder.
func NewPipelineBuilder(resolver AdapterResolver, obs ObservationAdapter) *PipelineBuilder {
	return &PipelineBuilder{
		resolver: resolver,
		obs:      obs,
	}
}

// Build creates a Composer from a PipelineConfig.
// For each step, it creates the appropriate Step based on the type:
//   - "atom": looks up the atom by name in the resolver
//   - "port": looks up the port by name in the resolver
//   - "adapter": looks up the adapter by name in the resolver
//   - "composer": looks up the composer by name in the resolver
//
// All steps are composed into a Pipeline.
func (b *PipelineBuilder) Build(config *PipelineConfig, env string) (Composer[any], error) {
	if config == nil {
		return nil, fmt.Errorf("PipelineBuilder.Build: PipelineConfig is nil")
	}

	pipeline := NewPipeline[any](b.obs)

	for i, stepCfg := range config.Steps {
		resolved, err := b.resolver.Resolve(stepCfg.Name, env)
		if err != nil {
			return nil, fmt.Errorf("PipelineBuilder.Build: step %d (%q): %w", i, stepCfg.Name, err)
		}

		var step Step[any, any]

		switch stepCfg.Type {
		case "atom":
			atom, ok := resolved.(Atom[any, any])
			if !ok {
				return nil, fmt.Errorf("PipelineBuilder.Build: step %d (%q): resolved value is not Atom[any,any], got %T", i, stepCfg.Name, resolved)
			}
			step = AtomAsStep[any, any](atom)

		case "port":
			port, ok := resolved.(Port[any, any])
			if !ok {
				return nil, fmt.Errorf("PipelineBuilder.Build: step %d (%q): resolved value is not Port[any,any], got %T", i, stepCfg.Name, resolved)
			}
			step = PortAsStep[any, any](port)

		case "adapter":
			adapter, ok := resolved.(Adapter[any, any])
			if !ok {
				return nil, fmt.Errorf("PipelineBuilder.Build: step %d (%q): resolved value is not Adapter[any,any], got %T", i, stepCfg.Name, resolved)
			}
			step = AdapterAsStep[any, any](adapter)

		case "composer":
			composer, ok := resolved.(Composer[any])
			if !ok {
				return nil, fmt.Errorf("PipelineBuilder.Build: step %d (%q): resolved value is not Composer[any], got %T", i, stepCfg.Name, resolved)
			}
			step = composerAsStep(composer)

		default:
			return nil, fmt.Errorf("PipelineBuilder.Build: step %d (%q): unknown type %q", i, stepCfg.Name, stepCfg.Type)
		}

		pipeline.AddStep(step)
	}

	return pipeline, nil
}

// composerAsStep wraps a Composer[any] as a Step[any, any].
// The Composer's Run method is called, and the returned ExecutionSteps
// are discarded since they are already recorded by the observation layer.
func composerAsStep(c Composer[any]) Step[any, any] {
	return StepFunc[any, any]{
		execute: func(ctx context.Context, input any) (any, error) {
			result, _, err := c.Run(ctx, input)
			return result, err
		},
		unitType: "Composer",
	}
}