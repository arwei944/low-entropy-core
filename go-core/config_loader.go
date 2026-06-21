//go:build lecore_tier2 || lecore_tier3 || lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

// Package core — 配置加载器 (v0.9.0)
package core

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
)

// ConfigLoader 从文件加载配置。
type ConfigLoader struct {
	mu       sync.RWMutex
	cfg      *AppConfig
	onReload []func(*AppConfig)
}

// NewConfigLoader 创建配置加载器。
func NewConfigLoader() *ConfigLoader {
	return &ConfigLoader{}
}

// LoadFromFile 从 JSON 文件加载配置。
func (l *ConfigLoader) LoadFromFile(path string) (*AppConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("config: read file %s: %w", path, err)
	}

	cfg := DefaultAppConfig()

	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("config: parse json: %w", err)
	}

	// 应用环境变量覆盖
	ApplyEnvOverrides(&cfg)

	// 解析密钥引用
	if err := l.resolveSecrets(&cfg); err != nil {
		return nil, fmt.Errorf("config: resolve secrets: %w", err)
	}

	l.mu.Lock()
	l.cfg = &cfg
	l.mu.Unlock()

	return &cfg, nil
}

// Get 获取当前配置（线程安全）。
func (l *ConfigLoader) Get() *AppConfig {
	l.mu.RLock()
	defer l.mu.RUnlock()
	if l.cfg == nil {
		cfg := DefaultAppConfig()
		return &cfg
	}
	cfg := *l.cfg
	return &cfg
}

// OnReload 注册配置变更回调。
func (l *ConfigLoader) OnReload(fn func(*AppConfig)) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.onReload = append(l.onReload, fn)
}

// reload 触发重载回调。
func (l *ConfigLoader) reload(cfg *AppConfig) {
	l.mu.RLock()
	callbacks := make([]func(*AppConfig), len(l.onReload))
	copy(callbacks, l.onReload)
	l.mu.RUnlock()

	for _, fn := range callbacks {
		fn(cfg)
	}
}
