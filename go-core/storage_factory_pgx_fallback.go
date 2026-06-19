//go:build (!lecore_pgx) && (lecore_tier3 || lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7)

// Package core — PostgreSQL 后端工厂函数回退 (v0.9.0)
//
// 当 lecore_pgx build tag 未启用时，返回明确的错误信息。

package core

import (
	"context"
	"fmt"
)

func newPostgresBackend(ctx context.Context, cfg AppConfig) (StorageBackend, error) {
	return nil, fmt.Errorf("postgres: build tag 'lecore_pgx' not enabled; add 'github.com/jackc/pgx/v5' to go.mod and build with -tags lecore_pgx")
}

func newPostgresEventStore(ctx context.Context, cfg AppConfig) (EventStoreBackend, error) {
	return nil, fmt.Errorf("postgres es: build tag 'lecore_pgx' not enabled; add 'github.com/jackc/pgx/v5' to go.mod and build with -tags lecore_pgx")
}