//go:build lecore_redis && (lecore_tier3 || lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7)

// Package core — Redis 后端工厂函数 (v0.9.0)
//
// 通过 build tag lecore_redis 条件编译，仅在引入 go-redis 依赖时可用。

package core

import (
	"fmt"

	"github.com/redis/go-redis/v9"
)

func newRedisBackend(cfg AppConfig) (StorageBackend, error) {
	redisCfg := RedisConfig{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	}
	return NewRedisStorageBackend(redisCfg)
}

// NewRedisDistributedLockFromConfig 根据 AppConfig 创建 Redis 分布式锁。
func NewRedisDistributedLockFromConfig(cfg AppConfig, key string) (*RedisDistributedLock, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})
	// 注意：调用者负责关闭 client
	return NewRedisDistributedLock(client, key, 0), nil
}