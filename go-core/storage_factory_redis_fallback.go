//go:build (!lecore_redis) && (lecore_tier3 || lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7)

// Package core — Redis 后端工厂函数回退 (v0.9.0)
//
// 当 lecore_redis build tag 未启用时，返回明确的错误信息。

package core

import (
	"fmt"
)

func newRedisBackend(cfg AppConfig) (StorageBackend, error) {
	return nil, fmt.Errorf("redis: build tag 'lecore_redis' not enabled; add 'github.com/redis/go-redis/v9' to go.mod and build with -tags lecore_redis")
}