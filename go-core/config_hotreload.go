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
	"sync"
	"time"
)

// ============================================================================
// HotReload — 配置热重载
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
// AppConfig File Loading and Environment Overrides
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
//   APP_NAME, APP_VERSION, APP_STORAGE_DIR, APP_STORAGE_BACKEND,
//   APP_POSTGRES_DSN, APP_REDIS_ADDR, APP_REDIS_PASSWORD, APP_REDIS_DB,
//   APP_HTTP_ADDR, APP_OBSERVABILITY_ENABLED, APP_LOG_LEVEL,
//   APP_GUARDIAN_ENABLED, APP_ENTROPY_CEILING, APP_SCHEDULER_ENABLED,
//   APP_JWT_SECRET, APP_API_KEY_ENABLED, APP_RATE_LIMIT
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
	if v := os.Getenv("APP_STORAGE_BACKEND"); v != "" {
		cfg.StorageBackend = v
	}
	if v := os.Getenv("APP_POSTGRES_DSN"); v != "" {
		cfg.PostgresDSN = v
	}
	if v := os.Getenv("APP_REDIS_ADDR"); v != "" {
		cfg.RedisAddr = v
	}
	if v := os.Getenv("APP_REDIS_PASSWORD"); v != "" {
		cfg.RedisPassword = v
	}
	if v := os.Getenv("APP_REDIS_DB"); v != "" {
		cfg.RedisDB, _ = strconv.Atoi(v)
	}
	if v := os.Getenv("APP_HTTP_ADDR"); v != "" {
		cfg.HTTPAddr = v
	}
	if v := os.Getenv("APP_OBSERVABILITY_ENABLED"); v != "" {
		cfg.ObservabilityEnabled, _ = strconv.ParseBool(v)
	}
	if v := os.Getenv("APP_LOG_LEVEL"); v != "" {
		cfg.LogLevel = v
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
	if v := os.Getenv("APP_JWT_SECRET"); v != "" {
		cfg.JWTSecret = v
	}
	if v := os.Getenv("APP_API_KEY_ENABLED"); v != "" {
		cfg.APIKeyEnabled, _ = strconv.ParseBool(v)
	}
	if v := os.Getenv("APP_RATE_LIMIT"); v != "" {
		cfg.RateLimit, _ = strconv.ParseFloat(v, 64)
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
	step.Metadata = map[string]any{
		"old_hash": oldHash,
		"new_hash": newHash,
	}
	return step
}
