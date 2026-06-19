//go:build lecore_tier2 || lecore_tier3 || lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

// Package core — EventStoreBackend 事件存储后端接口 (v0.9.0)
//
// 独立于 StorageBackend 的专用事件溯源接口。
// 支持 PostgreSQL 原生实现（利用 UNIQUE 约束做乐观并发控制）。

package core

import (
	"context"
	"errors"
	"fmt"
)

// ErrVersionConflict 乐观并发冲突错误。
var ErrVersionConflict = errors.New("version conflict: expected version does not match")

// EventStoreBackend 事件存储后端接口。
// 为事件溯源专门设计，支持乐观并发控制、快照、聚合列举。
type EventStoreBackend interface {
	// Append 追加事件到指定聚合的事件流。
	// 使用乐观并发控制：如果 expectedVersion 不匹配，返回 ErrVersionConflict。
	Append(ctx context.Context, event EventEnvelope, expectedVersion int64) (AppendResult, error)

	// Stream 读取指定聚合的事件流，从 fromVersion 开始（含）。
	// 返回事件按版本号升序排列。
	Stream(ctx context.Context, aggregateID string, fromVersion int64) ([]EventEnvelope, error)

	// GetLatestVersion 获取指定聚合的最新版本号。
	// 聚合不存在时返回 0, nil。
	GetLatestVersion(ctx context.Context, aggregateID string) (int64, error)

	// SaveSnapshot 保存聚合快照。
	// 如果已存在快照，则覆盖。
	SaveSnapshot(ctx context.Context, snapshot Snapshot) error

	// GetSnapshot 获取指定聚合的最新快照。
	// 快照不存在时返回 nil, nil。
	GetSnapshot(ctx context.Context, aggregateID string) (*Snapshot, error)

	// ListAggregates 列出所有聚合 ID。
	ListAggregates(ctx context.Context) ([]string, error)

	// HealthCheck 检查后端连接健康状态。
	HealthCheck(ctx context.Context) error

	// Close 关闭后端，释放资源。
	Close() error
}

// EventStoreBackendType 事件存储后端类型枚举。
type EventStoreBackendType string

const (
	EventStoreBackendMemory   EventStoreBackendType = "memory"
	EventStoreBackendFile     EventStoreBackendType = "file"
	EventStoreBackendPostgres EventStoreBackendType = "postgres"
)

// NewVersionConflictError 创建版本冲突错误。
func NewVersionConflictError(expected, got int64) error {
	return fmt.Errorf("%w: expected %d, got %d", ErrVersionConflict, expected, got)
}