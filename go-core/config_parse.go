//go:build lecore_tier2 || lecore_tier3 || lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

// Package core — 配置解析 + 构建器 + 热重载 (v4.0)
//
// 合并自: config.go + config_builder.go + config_hotreload.go
//
// 包含:
//   - PipelineConfig / StepConfig: 配置类型定义
//   - ParseConfig / ValidateConfig / ParseAndValidateConfig: 配置解析与校验
//   - AdapterResolver / MapAdapterResolver: 适配器解析
//   - PipelineBuilder: 从配置构建 Pipeline
//   - HotReload: 配置热重载 (SHA256 轮询检测)
package core

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
)

func isAllowedStepType(t string) bool {
	for _, allowed := range AllowedStepTypes {
		if t == allowed {
			return true
		}
	}
	return false
}

func ParseConfig(jsonBytes []byte) (*PipelineConfig, error) {
	if len(jsonBytes) == 0 {
		return nil, fmt.Errorf("config: empty JSON input")
	}

	var config PipelineConfig
	if err := json.Unmarshal(jsonBytes, &config); err != nil {
		return nil, fmt.Errorf("config: invalid JSON: %w", err)
	}

	if config.ID == "" {
		return nil, fmt.Errorf("config: pipeline ID must not be empty")
	}

	if len(config.Steps) == 0 {
		return nil, fmt.Errorf("config: pipeline %q has no steps defined", config.ID)
	}

	for i, step := range config.Steps {
		if !isAllowedStepType(step.Type) {
			return nil, fmt.Errorf(
				"config: step %d (%q) has invalid type %q; must be one of [%s]",
				i, step.Name, step.Type, strings.Join(AllowedStepTypes, ", "),
			)
		}
	}

	return &config, nil
}

func ValidateConfig(config *PipelineConfig) []error {
	var errs []error

	if config == nil {
		return append(errs, fmt.Errorf("config: PipelineConfig is nil"))
	}

	if config.ID == "" {
		errs = append(errs, fmt.Errorf("config: pipeline ID must not be empty"))
	}

	if len(config.Steps) == 0 {
		errs = append(errs, fmt.Errorf("config: pipeline %q has no steps defined", config.ID))
	}

	for i, step := range config.Steps {
		if step.Name == "" {
			errs = append(errs, fmt.Errorf("config: step %d has an empty name", i))
		}
		if !isAllowedStepType(step.Type) {
			errs = append(errs, fmt.Errorf(
				"config: step %d (%q) has invalid type %q; must be one of [%s]",
				i, step.Name, step.Type, strings.Join(AllowedStepTypes, ", "),
			))
		}
	}

	return errs
}

func ParseAndValidateConfig(jsonBytes []byte) (*PipelineConfig, []error, error) {
	config, parseErr := ParseConfig(jsonBytes)
	if parseErr != nil {
		return nil, nil, parseErr
	}

	errs := ValidateConfig(config)
	return config, errs, nil
}

// ============================================================================
// AdapterResolver + PipelineBuilder
// ============================================================================

// AdapterResolver resolves adapter names to Adapter instances based on environment.
type AdapterResolver interface {
	Resolve(name string, env string) (interface{}, error)
	Register(name string, env string, factory interface{})
}

// MapAdapterResolver is a simple map-based AdapterResolver.
type MapAdapterResolver struct {
	mu       sync.RWMutex
	adapters map[string]interface{}
}

func NewMapAdapterResolver() *MapAdapterResolver {
	return &MapAdapterResolver{
		adapters: make(map[string]interface{}),
	}
}

func (r *MapAdapterResolver) key(name, env string) string {
	return name + ":" + env
}

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

func (r *MapAdapterResolver) Register(name string, env string, factory interface{}) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.adapters[r.key(name, env)] = factory
}

// PipelineBuilder builds Composer instances from PipelineConfig.
type PipelineBuilder struct {
	resolver AdapterResolver
	obs      ObservationAdapter
}

func NewPipelineBuilder(resolver AdapterResolver, obs ObservationAdapter) *PipelineBuilder {
	return &PipelineBuilder{
		resolver: resolver,
		obs:      obs,
	}
}

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

func composerAsStep(c Composer[any]) Step[any, any] {
	return StepFunc[any, any]{
		execute: func(ctx context.Context, input any) (any, error) {
			result, _, err := c.Run(ctx, input)
			return result, err
		},
		unitType: "Composer",
	}
}
