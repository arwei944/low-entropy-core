// Package core — StorageBackend 增强接口 (v0.9.0)
//
// 从 storage_fs.go 抽离，增强为商用级存储接口。
// 新增: 批量操作、原子操作、健康检查。

package core

import (
	"context"
)

// StorageBackend 定义了持久化存储的统一接口。
// 实现可以是文件系统、PostgreSQL、Redis、对象存储等。
// 所有方法接受 context.Context 作为第一参数，支持超时和取消。
type StorageBackend interface {
	// Save 保存数据到指定 key。已存在的 key 会被覆盖。
	Save(ctx context.Context, key string, data []byte) error

	// Load 从指定 key 加载数据。
	// 数据不存在时返回 (nil, os.ErrNotExist) 或等价错误。
	Load(ctx context.Context, key string) ([]byte, error)

	// Delete 删除指定 key 的数据。
	// 删除不存在的 key 不返回错误。
	Delete(ctx context.Context, key string) error

	// List 列出指定前缀下的所有 key。
	// 返回的列表无序，可能不完整（取决于后端实现）。
	List(ctx context.Context, prefix string) ([]string, error)

	// SaveBatch 批量保存数据。原子性取决于后端实现。
	// 文件系统后端不保证原子性。
	SaveBatch(ctx context.Context, entries map[string][]byte) error

	// LoadBatch 批量加载数据。
	// 不存在的 key 不会出现在结果中。
	LoadBatch(ctx context.Context, keys []string) (map[string][]byte, error)

	// HealthCheck 检查后端连接健康状态。
	// 返回 nil 表示健康，否则返回错误描述。
	HealthCheck(ctx context.Context) error

	// Close 关闭后端，释放资源。
	Close() error
}

// StorageBackendType 存储后端类型枚举。
type StorageBackendType string

const (
	StorageBackendMemory   StorageBackendType = "memory"
	StorageBackendFile     StorageBackendType = "file"
	StorageBackendPostgres StorageBackendType = "postgres"
	StorageBackendRedis    StorageBackendType = "redis"
)