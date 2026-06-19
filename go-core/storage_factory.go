//go:build lecore_tier3 || lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

// Package core — StorageBackend 工厂函数 (v0.9.0)
//
// 根据 AppConfig 创建合适的 StorageBackend 实现。
// memory 和 file 后端始终可用；postgres 和 redis 需要对应的 build tag。

package core

import (
	"context"
	"fmt"
)

// NewStorageBackendFromConfig 根据 AppConfig 创建 StorageBackend。
// 支持的 backend 类型: "memory", "file", "postgres" (需 lecore_pgx), "redis" (需 lecore_redis)
func NewStorageBackendFromConfig(ctx context.Context, cfg AppConfig) (StorageBackend, error) {
	switch cfg.StorageBackend {
	case "", "memory":
		return NewMemoryStorageBackend(), nil

	case "file":
		dir := cfg.StorageDir
		if dir == "" {
			dir = "./data"
		}
		return NewFileStorageBackend(dir)

	case "postgres":
		return newPostgresBackend(ctx, cfg)

	case "redis":
		return newRedisBackend(cfg)

	default:
		return nil, fmt.Errorf("storage: unknown backend type: %s", cfg.StorageBackend)
	}
}

// NewEventStoreBackendFromConfig 根据 AppConfig 创建 EventStoreBackend。
// 支持的 backend 类型: "memory", "file", "postgres" (需 lecore_pgx)
func NewEventStoreBackendFromConfig(ctx context.Context, cfg AppConfig) (EventStoreBackend, error) {
	switch cfg.StorageBackend {
	case "", "memory":
		return NewEventStore(), nil

	case "file", "postgres":
		return newPostgresEventStore(ctx, cfg)

	default:
		return nil, fmt.Errorf("eventstore: unknown backend type: %s", cfg.StorageBackend)
	}
}