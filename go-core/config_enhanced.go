//go:build lecore_tier2 || lecore_tier3 || lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

// Package core — 配置增强模块 (v0.9.0)
//
// 提供企业级配置管理能力：
//   - 文件加载: JSON/YAML 配置文件解析
//   - 热重载: fsnotify 监听配置变更
//   - 密钥解析: 从环境变量/文件/Vault 解析敏感配置
//   - 配置验证: 启动时校验配置完整性

package core

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

// ============================================================================
// 配置加载器
// ============================================================================

// ConfigLoader 从文件加载配置。
type ConfigLoader struct {
	mu      sync.RWMutex
	cfg     *AppConfig
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

// ============================================================================
// 密钥解析
// ============================================================================

// SecretResolver 解析配置中的密钥引用。
// 支持格式:
//   - ${ENV:VAR_NAME} — 从环境变量读取
//   - ${FILE:/path/to/file} — 从文件读取
//   - ${VAULT:path/to/secret} — 从 HashiCorp Vault 读取（需实现 VaultResolver）
type SecretResolver interface {
	Resolve(reference string) (string, error)
}

// EnvSecretResolver 从环境变量解析密钥。
type EnvSecretResolver struct{}

func (r *EnvSecretResolver) Resolve(reference string) (string, error) {
	val := os.Getenv(reference)
	if val == "" {
		return "", fmt.Errorf("secret: env var %s is empty", reference)
	}
	return val, nil
}

// FileSecretResolver 从文件解析密钥。
type FileSecretResolver struct{}

func (r *FileSecretResolver) Resolve(reference string) (string, error) {
	data, err := os.ReadFile(reference)
	if err != nil {
		return "", fmt.Errorf("secret: read file %s: %w", reference, err)
	}
	return strings.TrimSpace(string(data)), nil
}

// resolveSecrets 解析配置中的密钥引用。
// 格式: ${ENV:VAR} 或 ${FILE:path}
func (l *ConfigLoader) resolveSecrets(cfg *AppConfig) error {
	resolvers := map[string]SecretResolver{
		"ENV":  &EnvSecretResolver{},
		"FILE": &FileSecretResolver{},
	}

	// 解析需要密钥的字段
	if cfg.PostgresDSN != "" {
		resolved, err := resolveSecretRef(cfg.PostgresDSN, resolvers)
		if err != nil {
			return fmt.Errorf("postgres_dsn: %w", err)
		}
		cfg.PostgresDSN = resolved
	}

	if cfg.RedisPassword != "" {
		resolved, err := resolveSecretRef(cfg.RedisPassword, resolvers)
		if err != nil {
			return fmt.Errorf("redis_password: %w", err)
		}
		cfg.RedisPassword = resolved
	}

	if cfg.JWTSecret != "" {
		resolved, err := resolveSecretRef(cfg.JWTSecret, resolvers)
		if err != nil {
			return fmt.Errorf("jwt_secret: %w", err)
		}
		cfg.JWTSecret = resolved
	}

	return nil
}

// resolveSecretRef 解析单个密钥引用。
func resolveSecretRef(value string, resolvers map[string]SecretResolver) (string, error) {
	// 查找 ${TYPE:reference} 格式
	if !strings.HasPrefix(value, "${") || !strings.HasSuffix(value, "}") {
		return value, nil
	}

	ref := value[2 : len(value)-1]
	parts := strings.SplitN(ref, ":", 2)
	if len(parts) != 2 {
		return value, fmt.Errorf("invalid secret reference format: %s", value)
	}

	resolverType := strings.ToUpper(parts[0])
	reference := parts[1]

	resolver, ok := resolvers[resolverType]
	if !ok {
		return value, fmt.Errorf("unknown secret resolver: %s", resolverType)
	}

	return resolver.Resolve(reference)
}

// ============================================================================
// 配置验证
// ============================================================================

// ValidateConfig 验证配置完整性。
// 返回所有验证错误，便于一次性修复。
func ValidateConfig(cfg *AppConfig) []error {
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

// MustValidateConfig 验证配置，失败时 panic。
func MustValidateConfig(cfg *AppConfig) {
	errs := ValidateConfig(cfg)
	if len(errs) > 0 {
		msg := "config validation failed:\n"
		for _, err := range errs {
			msg += "  - " + err.Error() + "\n"
		}
		panic(msg)
	}
}

// ============================================================================
// 热重载文件监听
// ============================================================================

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