//go:build lecore_tier2 || lecore_tier3 || lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

// Package core — 密钥解析 (v0.9.0)
package core

import (
	"fmt"
	"os"
	"strings"
)

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
