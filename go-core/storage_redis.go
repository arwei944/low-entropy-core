//go:build lecore_redis

// Package core — RedisStorageBackend Redis 键值存储适配器 (v0.9.0)
//
// 使用 go-redis/v9 实现 StorageBackend 接口。
// 通过 build tag lecore_redis 隔离外部依赖。
//
// 特性:
//   - 连接池自动管理（go-redis 内置）
//   - SaveBatch 使用 Pipeline 批量操作
//   - 健康检查通过 PING 验证连接可用性
//   - 所有操作通过 context 支持超时和取消
//   - key 前缀隔离：所有 key 自动添加 lc: 前缀

package core

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisStorageBackend 是 StorageBackend 的 Redis 实现。
// 所有 key 自动添加 lc: 前缀以避免命名冲突。
type RedisStorageBackend struct {
	client *redis.Client
	closed bool
}

// RedisConfig Redis 连接配置。
type RedisConfig struct {
	Addr     string // 地址，格式: "host:port"
	Password string // 密码
	DB       int    // 数据库编号
	Prefix   string // key 前缀，默认 "lc:"
}

// NewRedisStorageBackend 创建 Redis 存储后端。
func NewRedisStorageBackend(cfg RedisConfig) (*RedisStorageBackend, error) {
	if cfg.Addr == "" {
		cfg.Addr = "localhost:6379"
	}
	if cfg.Prefix == "" {
		cfg.Prefix = "lc:"
	}

	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	// 验证连接
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis: connect: %w", err)
	}

	return &RedisStorageBackend{client: client}, nil
}

// NewRedisStorageBackendFromClient 从已有客户端创建后端。
func NewRedisStorageBackendFromClient(client *redis.Client) (*RedisStorageBackend, error) {
	if client == nil {
		return nil, fmt.Errorf("redis: client is nil")
	}
	return &RedisStorageBackend{client: client}, nil
}

// key 添加前缀。
func (r *RedisStorageBackend) key(k string) string {
	return "lc:" + k
}

func (r *RedisStorageBackend) Save(ctx context.Context, key string, data []byte) error {
	if r.closed {
		return os.ErrClosed
	}
	return r.client.Set(ctx, r.key(key), data, 0).Err()
}

func (r *RedisStorageBackend) Load(ctx context.Context, key string) ([]byte, error) {
	if r.closed {
		return nil, os.ErrClosed
	}
	data, err := r.client.Get(ctx, r.key(key)).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, os.ErrNotExist
		}
		return nil, fmt.Errorf("redis: load %s: %w", key, err)
	}
	return data, nil
}

func (r *RedisStorageBackend) Delete(ctx context.Context, key string) error {
	if r.closed {
		return os.ErrClosed
	}
	return r.client.Del(ctx, r.key(key)).Err()
}

func (r *RedisStorageBackend) List(ctx context.Context, prefix string) ([]string, error) {
	if r.closed {
		return nil, os.ErrClosed
	}

	pattern := r.key(prefix) + "*"
	var cursor uint64
	var keys []string

	for {
		var batch []string
		var err error
		batch, cursor, err = r.client.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return nil, fmt.Errorf("redis: list %s: %w", prefix, err)
		}
		// 去除前缀
		for _, k := range batch {
			keys = append(keys, strings.TrimPrefix(k, "lc:"))
		}
		if cursor == 0 {
			break
		}
	}
	return keys, nil
}

func (r *RedisStorageBackend) SaveBatch(ctx context.Context, entries map[string][]byte) error {
	if r.closed {
		return os.ErrClosed
	}

	pipe := r.client.Pipeline()
	for key, data := range entries {
		pipe.Set(ctx, r.key(key), data, 0)
	}
	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("redis: save batch: %w", err)
	}
	return nil
}

func (r *RedisStorageBackend) LoadBatch(ctx context.Context, keys []string) (map[string][]byte, error) {
	if r.closed {
		return nil, os.ErrClosed
	}

	if len(keys) == 0 {
		return make(map[string][]byte), nil
	}

	redisKeys := make([]string, len(keys))
	for i, k := range keys {
		redisKeys[i] = r.key(k)
	}

	values, err := r.client.MGet(ctx, redisKeys...).Result()
	if err != nil {
		return nil, fmt.Errorf("redis: load batch: %w", err)
	}

	result := make(map[string][]byte, len(keys))
	for i, v := range values {
		if v == nil {
			continue
		}
		switch val := v.(type) {
		case string:
			result[keys[i]] = []byte(val)
		case []byte:
			result[keys[i]] = val
		}
	}
	return result, nil
}

func (r *RedisStorageBackend) HealthCheck(ctx context.Context) error {
	if r.closed {
		return os.ErrClosed
	}
	return r.client.Ping(ctx).Err()
}

func (r *RedisStorageBackend) Close() error {
	r.closed = true
	return r.client.Close()
}

// Client 返回底层 Redis 客户端（供高级用法）。
func (r *RedisStorageBackend) Client() *redis.Client {
	return r.client
}

// Ensure RedisStorageBackend implements StorageBackend.
var _ StorageBackend = (*RedisStorageBackend)(nil)