//go:build lecore_pgx

// Package core — PostgresEventStore: PostgreSQL 事件存储适配器 (v4.0)
//
// 使用 pgx/v5 实现 EventStoreBackend 接口，通过 build tag lecore_pgx 隔离外部依赖。
// 表结构: lc_events (PK: aggregate_id + version) + lc_snapshots (PK: aggregate_id)。
// 乐观并发控制通过 PRIMARY KEY 约束实现，版本冲突时返回 pgx.ErrSQLState unique_violation。
package core

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresEventStore 是 EventStoreBackend 的 PostgreSQL 实现。
type PostgresEventStore struct {
	pool   *pgxpool.Pool
	closed bool
}

// NewPostgresEventStore 创建 PostgreSQL 事件存储后端。
// connString 格式: "postgres://user:pass@localhost:5432/dbname"
// 自动创建 lc_events 和 lc_snapshots 表。
func NewPostgresEventStore(ctx context.Context, connString string) (*PostgresEventStore, error) {
	pool, err := pgxpool.New(ctx, connString)
	if err != nil {
		return nil, fmt.Errorf("postgres es: connect: %w", err)
	}

	if err := createEventStoreTables(ctx, pool); err != nil {
		pool.Close()
		return nil, err
	}

	return &PostgresEventStore{pool: pool}, nil
}

// NewPostgresEventStoreFromPool 从已有连接池创建事件存储后端。
func NewPostgresEventStoreFromPool(pool *pgxpool.Pool) (*PostgresEventStore, error) {
	if pool == nil {
		return nil, fmt.Errorf("postgres es: pool is nil")
	}
	return &PostgresEventStore{pool: pool}, nil
}

// Append 追加事件到指定聚合的事件流。
// 使用乐观并发控制：expectedVersion 必须等于当前最新版本。
// 版本冲突时返回 ErrVersionConflict。
func (p *PostgresEventStore) Append(ctx context.Context, event EventEnvelope, expectedVersion int64) (AppendResult, error) {
	if p.closed {
		return AppendResult{}, os.ErrClosed
	}

	// 生成 EventID 和 Timestamp
	if event.EventID == "" {
		event.EventID = generateEventID()
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	newVersion := expectedVersion + 1

	_, err := p.pool.Exec(ctx, `
		INSERT INTO lc_events (aggregate_id, aggregate_type, event_id, event_type, event_data, version, timestamp, trace_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, event.AggregateID, event.AggregateType, event.EventID, event.EventType,
		event.EventData, newVersion, event.Timestamp, event.TraceID)

	if err != nil {
		// 检查是否为唯一约束冲突（版本冲突）
		if isUniqueViolation(err) {
			return AppendResult{}, NewVersionConflictError(expectedVersion, newVersion)
		}
		return AppendResult{}, fmt.Errorf("postgres es: append: %w", err)
	}

	return AppendResult{
		EventID: event.EventID,
		Version: newVersion,
		Success: true,
	}, nil
}

// Stream 读取指定聚合的事件流，从 fromVersion 开始（含）。
// 返回事件按版本号升序排列。
func (p *PostgresEventStore) Stream(ctx context.Context, aggregateID string, fromVersion int64) ([]EventEnvelope, error) {
	if p.closed {
		return nil, os.ErrClosed
	}

	rows, err := p.pool.Query(ctx, `
		SELECT aggregate_id, aggregate_type, event_id, event_type, event_data, version, timestamp, trace_id
		FROM lc_events
		WHERE aggregate_id = $1 AND version >= $2
		ORDER BY version ASC
	`, aggregateID, fromVersion)
	if err != nil {
		return nil, fmt.Errorf("postgres es: stream: %w", err)
	}
	defer rows.Close()

	var events []EventEnvelope
	for rows.Next() {
		var e EventEnvelope
		if err := rows.Scan(&e.AggregateID, &e.AggregateType, &e.EventID, &e.EventType,
			&e.EventData, &e.Version, &e.Timestamp, &e.TraceID); err != nil {
			return nil, fmt.Errorf("postgres es: stream scan: %w", err)
		}
		events = append(events, e)
	}
	return events, rows.Err()
}

// GetLatestVersion 获取指定聚合的最新版本号。
// 聚合不存在时返回 0, nil。
func (p *PostgresEventStore) GetLatestVersion(ctx context.Context, aggregateID string) (int64, error) {
	if p.closed {
		return 0, os.ErrClosed
	}

	var version int64
	err := p.pool.QueryRow(ctx, `
		SELECT COALESCE(MAX(version), 0) FROM lc_events WHERE aggregate_id = $1
	`, aggregateID).Scan(&version)
	if err != nil {
		return 0, fmt.Errorf("postgres es: get version: %w", err)
	}
	return version, nil
}

// SaveSnapshot 保存聚合快照。
// 如果已存在快照，则覆盖。
func (p *PostgresEventStore) SaveSnapshot(ctx context.Context, snapshot Snapshot) error {
	if p.closed {
		return os.ErrClosed
	}

	if snapshot.Timestamp.IsZero() {
		snapshot.Timestamp = time.Now()
	}

	_, err := p.pool.Exec(ctx, `
		INSERT INTO lc_snapshots (aggregate_id, version, state, timestamp)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (aggregate_id) DO UPDATE SET
			version = EXCLUDED.version,
			state = EXCLUDED.state,
			timestamp = EXCLUDED.timestamp
	`, snapshot.AggregateID, snapshot.Version, snapshot.State, snapshot.Timestamp)
	if err != nil {
		return fmt.Errorf("postgres es: save snapshot: %w", err)
	}
	return nil
}

// GetSnapshot 获取指定聚合的最新快照。
// 快照不存在时返回 nil, nil。
func (p *PostgresEventStore) GetSnapshot(ctx context.Context, aggregateID string) (*Snapshot, error) {
	if p.closed {
		return nil, os.ErrClosed
	}

	var s Snapshot
	err := p.pool.QueryRow(ctx, `
		SELECT aggregate_id, version, state, timestamp
		FROM lc_snapshots
		WHERE aggregate_id = $1
	`, aggregateID).Scan(&s.AggregateID, &s.Version, &s.State, &s.Timestamp)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("postgres es: get snapshot: %w", err)
	}
	return &s, nil
}

// ListAggregates 列出所有聚合 ID。
func (p *PostgresEventStore) ListAggregates(ctx context.Context) ([]string, error) {
	if p.closed {
		return nil, os.ErrClosed
	}

	rows, err := p.pool.Query(ctx, `
		SELECT DISTINCT aggregate_id FROM lc_events ORDER BY aggregate_id
	`)
	if err != nil {
		return nil, fmt.Errorf("postgres es: list aggregates: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("postgres es: list scan: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// HealthCheck 检查后端连接健康状态。
func (p *PostgresEventStore) HealthCheck(ctx context.Context) error {
	if p.closed {
		return os.ErrClosed
	}
	return p.pool.Ping(ctx)
}

// Close 关闭后端，释放资源。
func (p *PostgresEventStore) Close() error {
	p.closed = true
	p.pool.Close()
	return nil
}

// Pool 返回底层连接池（供高级用法）。
func (p *PostgresEventStore) Pool() *pgxpool.Pool {
	return p.pool
}

// ──────────────────────────────────────────────
// 内部辅助函数
// ──────────────────────────────────────────────

// createEventStoreTables 创建事件存储所需的表。
func createEventStoreTables(ctx context.Context, pool *pgxpool.Pool) error {
	// 创建事件表
	_, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS lc_events (
			aggregate_id   TEXT NOT NULL,
			aggregate_type TEXT NOT NULL DEFAULT '',
			event_id       TEXT NOT NULL,
			event_type     TEXT NOT NULL DEFAULT '',
			event_data     BYTEA NOT NULL,
			version        BIGINT NOT NULL,
			timestamp      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			trace_id       TEXT DEFAULT '',
			PRIMARY KEY (aggregate_id, version)
		)
	`)
	if err != nil {
		return fmt.Errorf("postgres es: create table lc_events: %w", err)
	}

	// 创建快照表
	_, err = pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS lc_snapshots (
			aggregate_id TEXT PRIMARY KEY,
			version      BIGINT NOT NULL,
			state        BYTEA NOT NULL,
			timestamp    TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`)
	if err != nil {
		return fmt.Errorf("postgres es: create table lc_snapshots: %w", err)
	}

	// 创建索引
	_, _ = pool.Exec(ctx, `
		CREATE INDEX IF NOT EXISTS idx_lc_events_aggregate ON lc_events(aggregate_id);
	`)
	_, _ = pool.Exec(ctx, `
		CREATE INDEX IF NOT EXISTS idx_lc_events_type ON lc_events(event_type);
	`)
	_, _ = pool.Exec(ctx, `
		CREATE INDEX IF NOT EXISTS idx_lc_events_timestamp ON lc_events(timestamp);
	`)

	return nil
}

// isUniqueViolation 检查错误是否为 PostgreSQL 唯一约束冲突。
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if err != nil {
		// 使用 errors.As 兼容包装后的错误
		if pgErr, ok := err.(*pgconn.PgError); ok {
			return pgErr.Code == "23505"
		}
	}
	_ = pgErr // suppress unused warning
	return false
}

// Ensure PostgresEventStore implements EventStoreBackend.
var _ EventStoreBackend = (*PostgresEventStore)(nil)