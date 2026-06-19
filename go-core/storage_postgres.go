//go:build lecore_pgx

// Package core — PostgresStorageBackend PostgreSQL 键值存储适配器 (v0.9.0)
//
// 使用 pgx/v5 连接池实现 StorageBackend 接口。
// 通过 build tag lecore_pgx 隔离外部依赖，未启用时零开销。
//
// 表结构:
//
//	CREATE TABLE IF NOT EXISTS lc_kv (
//	    key   TEXT PRIMARY KEY,
//	    value BYTEA NOT NULL
//	);
//
// 特性:
//   - 连接池（pgxpool）自动管理连接生命周期
//   - SaveBatch 使用事务保证原子性
//   - 健康检查通过 Ping 验证连接可用性
//   - 所有操作通过 context 支持超时和取消

package core

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresStorageBackend 是 StorageBackend 的 PostgreSQL 实现。
type PostgresStorageBackend struct {
	pool   *pgxpool.Pool
	closed bool
}

// NewPostgresStorageBackend 创建 PostgreSQL 存储后端。
// connString 格式: "postgres://user:pass@localhost:5432/dbname"
// 自动创建 lc_kv 表。
func NewPostgresStorageBackend(ctx context.Context, connString string) (*PostgresStorageBackend, error) {
	pool, err := pgxpool.New(ctx, connString)
	if err != nil {
		return nil, fmt.Errorf("postgres: connect: %w", err)
	}

	// 自动建表
	if err := createKVTable(ctx, pool); err != nil {
		pool.Close()
		return nil, err
	}

	return &PostgresStorageBackend{pool: pool}, nil
}

// NewPostgresStorageBackendFromPool 从已有连接池创建后端。
// 调用者负责管理连接池的生命周期。
func NewPostgresStorageBackendFromPool(pool *pgxpool.Pool) (*PostgresStorageBackend, error) {
	if pool == nil {
		return nil, fmt.Errorf("postgres: pool is nil")
	}
	return &PostgresStorageBackend{pool: pool}, nil
}

func (p *PostgresStorageBackend) Save(ctx context.Context, key string, data []byte) error {
	if p.closed {
		return os.ErrClosed
	}
	_, err := p.pool.Exec(ctx,
		`INSERT INTO lc_kv (key, value) VALUES ($1, $2)
		 ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value`,
		key, data,
	)
	if err != nil {
		return fmt.Errorf("postgres: save %s: %w", key, err)
	}
	return nil
}

func (p *PostgresStorageBackend) Load(ctx context.Context, key string) ([]byte, error) {
	if p.closed {
		return nil, os.ErrClosed
	}
	var data []byte
	err := p.pool.QueryRow(ctx,
		`SELECT value FROM lc_kv WHERE key = $1`, key,
	).Scan(&data)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, os.ErrNotExist
		}
		return nil, fmt.Errorf("postgres: load %s: %w", key, err)
	}
	return data, nil
}

func (p *PostgresStorageBackend) Delete(ctx context.Context, key string) error {
	if p.closed {
		return os.ErrClosed
	}
	_, err := p.pool.Exec(ctx, `DELETE FROM lc_kv WHERE key = $1`, key)
	if err != nil {
		return fmt.Errorf("postgres: delete %s: %w", key, err)
	}
	return nil
}

func (p *PostgresStorageBackend) List(ctx context.Context, prefix string) ([]string, error) {
	if p.closed {
		return nil, os.ErrClosed
	}
	rows, err := p.pool.Query(ctx,
		`SELECT key FROM lc_kv WHERE key LIKE $1 ORDER BY key`,
		prefix+"%",
	)
	if err != nil {
		return nil, fmt.Errorf("postgres: list %s: %w", prefix, err)
	}
	defer rows.Close()

	var keys []string
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			return nil, fmt.Errorf("postgres: list scan: %w", err)
		}
		keys = append(keys, k)
	}
	return keys, rows.Err()
}

func (p *PostgresStorageBackend) SaveBatch(ctx context.Context, entries map[string][]byte) error {
	if p.closed {
		return os.ErrClosed
	}

	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("postgres: begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	for key, data := range entries {
		_, err := tx.Exec(ctx,
			`INSERT INTO lc_kv (key, value) VALUES ($1, $2)
			 ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value`,
			key, data,
		)
		if err != nil {
			return fmt.Errorf("postgres: save batch %s: %w", key, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("postgres: commit tx: %w", err)
	}
	return nil
}

func (p *PostgresStorageBackend) LoadBatch(ctx context.Context, keys []string) (map[string][]byte, error) {
	if p.closed {
		return nil, os.ErrClosed
	}

	if len(keys) == 0 {
		return make(map[string][]byte), nil
	}

	// 构建 IN 查询
	placeholders := make([]string, len(keys))
	args := make([]any, len(keys))
	for i, k := range keys {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = k
	}

	query := fmt.Sprintf(
		`SELECT key, value FROM lc_kv WHERE key IN (%s)`,
		strings.Join(placeholders, ", "),
	)

	rows, err := p.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("postgres: load batch: %w", err)
	}
	defer rows.Close()

	result := make(map[string][]byte, len(keys))
	for rows.Next() {
		var k string
		var v []byte
		if err := rows.Scan(&k, &v); err != nil {
			return nil, fmt.Errorf("postgres: load batch scan: %w", err)
		}
		result[k] = v
	}
	return result, rows.Err()
}

func (p *PostgresStorageBackend) HealthCheck(ctx context.Context) error {
	if p.closed {
		return os.ErrClosed
	}
	return p.pool.Ping(ctx)
}

func (p *PostgresStorageBackend) Close() error {
	p.closed = true
	p.pool.Close()
	return nil
}

// Pool 返回底层连接池（供高级用法）。
func (p *PostgresStorageBackend) Pool() *pgxpool.Pool {
	return p.pool
}

// createKVTable 创建键值存储表。
func createKVTable(ctx context.Context, pool *pgxpool.Pool) error {
	_, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS lc_kv (
			key   TEXT PRIMARY KEY,
			value BYTEA NOT NULL
		)
	`)
	if err != nil {
		return fmt.Errorf("postgres: create table lc_kv: %w", err)
	}
	return nil
}

// Ensure PostgresStorageBackend implements StorageBackend.
var _ StorageBackend = (*PostgresStorageBackend)(nil)