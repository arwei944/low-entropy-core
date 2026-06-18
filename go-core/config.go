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
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ============================================================================
// SECTION 1: Pipeline Configuration Types
// ============================================================================

// AllowedStepTypes defines the valid step type values for pipeline configuration.
var AllowedStepTypes = []string{"atom", "port", "adapter", "composer"}

// PipelineConfig defines the configuration for a pipeline.
type PipelineConfig struct {
	ID    string       `json:"id"`
	Name  string       `json:"name"`
	Steps []StepConfig `json:"steps"`
}

// StepConfig defines the configuration for a single step in a pipeline.
type StepConfig struct {
	Type   string         `json:"type"`
	Name   string         `json:"name"`
	Params map[string]any `json:"params"`
}

// ============================================================================
// SECTION 2: Configuration Parsing & Validation
// ============================================================================

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
// SECTION 3: AdapterResolver + PipelineBuilder
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

// ============================================================================
// SECTION 4: HotReload — 配置热重载
// ============================================================================

// HotReload manages hot-reloading of pipeline configurations.
// Uses SHA256 polling to detect config file changes.
type HotReload struct {
	mu         sync.RWMutex
	current    Composer[any]
	builder    *PipelineBuilder
	configPath string
	env        string
	obs        ObservationAdapter
	cancel     context.CancelFunc
	done       chan struct{}
}

func NewHotReload(configPath string, builder *PipelineBuilder, env string, obs ObservationAdapter) *HotReload {
	if obs == nil {
		obs = &NoOpObservationAdapter{}
	}
	return &HotReload{
		configPath: configPath,
		builder:    builder,
		env:        env,
		obs:        obs,
		done:       make(chan struct{}),
	}
}

func (h *HotReload) Start(ctx context.Context, checkInterval time.Duration) (Composer[any], error) {
	initial, initialHash, err := h.buildFromFile()
	if err != nil {
		return nil, fmt.Errorf("hotreload: initial build failed: %w", err)
	}

	h.mu.Lock()
	h.current = initial
	h.mu.Unlock()

	ctx, h.cancel = context.WithCancel(ctx)
	go h.watch(ctx, checkInterval, initialHash)
	return initial, nil
}

func (h *HotReload) Stop() {
	if h.cancel != nil {
		h.cancel()
	}
	<-h.done
}

func (h *HotReload) Current() Composer[any] {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.current
}

func (h *HotReload) watch(ctx context.Context, checkInterval time.Duration, lastHash string) {
	defer close(h.done)

	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			currentHash, err := computeFileHash(h.configPath)
			if err != nil {
				errStep := NewExecutionStep("HotReload", "hash", err.Error(), "ConfigChange")
				errStep.Error = NewStepError("HASH_COMPUTE_FAILED", err.Error(), true)
				h.obs.Record([]ExecutionStep{errStep})
				continue
			}

			if currentHash == lastHash {
				continue
			}

			newPipeline, _, err := h.buildFromFile()
			if err != nil {
				errStep := NewExecutionStep("HotReload", "rebuild", err.Error(), "ConfigChange")
				errStep.Error = NewStepError("REBUILD_FAILED", err.Error(), true)
				h.obs.Record([]ExecutionStep{errStep})
				continue
			}

			h.mu.Lock()
			h.current = newPipeline
			h.mu.Unlock()

			changeStep := NewConfigChangeStep(lastHash, currentHash)
			h.obs.Record([]ExecutionStep{changeStep})
			lastHash = currentHash
		}
	}
}

// ============================================================================
// SECTION 5: AppConfig File Loading and Environment Overrides
// ============================================================================

// LoadAppConfigFromFile 从 JSON 文件加载 AppConfig。
// 文件不存在时返回默认配置，不报错。
func LoadAppConfigFromFile(path string) (AppConfig, error) {
	cfg := DefaultAppConfig()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, fmt.Errorf("config: read %s: %w", path, err)
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("config: parse %s: %w", path, err)
	}
	return cfg, nil
}

// ApplyEnvOverrides 用环境变量覆盖 AppConfig。
// 支持的环境变量:
//   APP_NAME, APP_VERSION, APP_STORAGE_DIR, APP_HTTP_ADDR,
//   APP_GUARDIAN_ENABLED, APP_ENTROPY_CEILING, APP_SCHEDULER_ENABLED
func ApplyEnvOverrides(cfg *AppConfig) {
	if v := os.Getenv("APP_NAME"); v != "" {
		cfg.Name = v
	}
	if v := os.Getenv("APP_VERSION"); v != "" {
		cfg.Version = v
	}
	if v := os.Getenv("APP_STORAGE_DIR"); v != "" {
		cfg.StorageDir = v
	}
	if v := os.Getenv("APP_HTTP_ADDR"); v != "" {
		cfg.HTTPAddr = v
	}
	if v := os.Getenv("APP_GUARDIAN_ENABLED"); v != "" {
		cfg.GuardianEnabled, _ = strconv.ParseBool(v)
	}
	if v := os.Getenv("APP_ENTROPY_CEILING"); v != "" {
		cfg.EntropyCeiling, _ = strconv.ParseFloat(v, 64)
	}
	if v := os.Getenv("APP_SCHEDULER_ENABLED"); v != "" {
		cfg.SchedulerEnabled, _ = strconv.ParseBool(v)
	}
}

func (h *HotReload) buildFromFile() (Composer[any], string, error) {
	data, err := os.ReadFile(h.configPath)
	if err != nil {
		return nil, "", fmt.Errorf("reading config file: %w", err)
	}

	hash := fmt.Sprintf("%x", sha256.Sum256(data))

	config, err := ParseConfig(data)
	if err != nil {
		return nil, "", fmt.Errorf("parsing config: %w", err)
	}

	pipeline, err := h.builder.Build(config, h.env)
	if err != nil {
		return nil, "", fmt.Errorf("building pipeline: %w", err)
	}

	return pipeline, hash, nil
}

func computeFileHash(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("computeFileHash: open: %w", err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("computeFileHash: read: %w", err)
	}

	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

func NewConfigChangeStep(oldHash, newHash string) ExecutionStep {
	step := NewExecutionStep("HotReload", "reload", "config file changed", "ConfigChange")
	step.Metadata = map[string]interface{}{
		"old_hash": oldHash,
		"new_hash": newHash,
	}
	return step
}