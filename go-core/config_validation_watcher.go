//go:build lecore_tier2 || lecore_tier3 || lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

// Package core — 配置验证与热重载 (v0.9.0)
package core

import (
	"fmt"
	"os"
	"time"
)

// ValidateConfigEnhanced 验证配置完整性。
// 返回所有验证错误，便于一次性修复。
func ValidateConfigEnhanced(cfg *AppConfig) []error {
	var errs []error

	if cfg.Name == "" {
		errs = append(errs, fmt.Errorf("config: name is required"))
	}

	switch cfg.StorageBackend {
	case "file":
		if cfg.StorageDir == "" {
			errs = append(errs, fmt.Errorf("config: storage_dir is required for file backend"))
		}
	case "postgres":
		if cfg.PostgresDSN == "" {
			errs = append(errs, fmt.Errorf("config: postgres_dsn is required for postgres backend"))
		}
	case "redis":
		if cfg.RedisAddr == "" {
			errs = append(errs, fmt.Errorf("config: redis_addr is required for redis backend"))
		}
	}

	if cfg.ObservabilityEnabled && cfg.LogLevel == "" {
		errs = append(errs, fmt.Errorf("config: log_level is required when observability is enabled"))
	}

	if cfg.JWTSecret != "" && len(cfg.JWTSecret) < 16 {
		errs = append(errs, fmt.Errorf("config: jwt_secret must be at least 16 characters"))
	}

	if cfg.RateLimit <= 0 {
		errs = append(errs, fmt.Errorf("config: rate_limit must be positive"))
	}

	return errs
}

// MustValidateConfigEnhanced 验证配置，失败时 panic。
func MustValidateConfigEnhanced(cfg *AppConfig) {
	errs := ValidateConfigEnhanced(cfg)
	if len(errs) > 0 {
		msg := "config validation failed:\n"
		for _, err := range errs {
			msg += "  - " + err.Error() + "\n"
		}
		panic(msg)
	}
}

// ConfigWatcher 配置文件热重载监听器。
// 使用轮询方式检测文件变更（无需外部依赖）。
type ConfigWatcher struct {
	loader   *ConfigLoader
	path     string
	interval time.Duration
	stopCh   chan struct{}
	modTime  time.Time
}

// NewConfigWatcher 创建配置热重载监听器。
func NewConfigWatcher(loader *ConfigLoader, path string, interval time.Duration) *ConfigWatcher {
	if interval <= 0 {
		interval = 5 * time.Second
	}
	return &ConfigWatcher{
		loader:   loader,
		path:     path,
		interval: interval,
		stopCh:   make(chan struct{}),
	}
}

// Start 启动配置热重载监听。
func (w *ConfigWatcher) Start() error {
	// 记录初始修改时间
	info, err := os.Stat(w.path)
	if err != nil {
		return fmt.Errorf("config watch: stat %s: %w", w.path, err)
	}
	w.modTime = info.ModTime()

	go func() {
		ticker := time.NewTicker(w.interval)
		defer ticker.Stop()
		for {
			select {
			case <-w.stopCh:
				return
			case <-ticker.C:
				info, err := os.Stat(w.path)
				if err != nil {
					continue
				}
				if info.ModTime().After(w.modTime) {
					w.modTime = info.ModTime()
					cfg, err := w.loader.LoadFromFile(w.path)
					if err != nil {
						// 重载失败，保留旧配置
						continue
					}
					w.loader.reload(cfg)
				}
			}
		}
	}()
	return nil
}

// Stop 停止配置热重载监听。
func (w *ConfigWatcher) Stop() {
	select {
	case <-w.stopCh:
		// 已停止
	default:
		close(w.stopCh)
	}
}
