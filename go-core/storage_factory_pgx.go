//go:build lecore_pgx && (lecore_tier3 || lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7)

// Package core — PostgreSQL 后端工厂函数 (v0.9.0)
//
// 通过 build tag lecore_pgx 条件编译，仅在引入 pgx 依赖时可用。

package core

import (
	"context"
	"fmt"
)

func newPostgresBackend(ctx context.Context, cfg AppConfig) (StorageBackend, error) {
	if cfg.PostgresDSN == "" {
		return nil, fmt.Errorf("postgres: PostgresDSN is required")
	}
	return NewPostgresStorageBackend(ctx, cfg.PostgresDSN)
}

func newPostgresEventStore(ctx context.Context, cfg AppConfig) (EventStoreBackend, error) {
	if cfg.PostgresDSN == "" {
		return nil, fmt.Errorf("postgres es: PostgresDSN is required")
	}
	return NewPostgresEventStore(ctx, cfg.PostgresDSN)
}