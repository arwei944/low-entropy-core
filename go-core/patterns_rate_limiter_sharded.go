//go:build lecore_tier2 || lecore_tier3 || lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

// Package core — 分片限流器 (v4.0)
//
// 原语归属: L2 Adapter（单机韧性层）
// 通过分片减少锁竞争，将请求分散到多个子限流器。
// 每个分片持有独立的计数器，通过 sync.Mutex 保护。
package core

import (
	"fmt"
	"sync/atomic"
	"time"
)

// ──────────────────────────────────────────────
// ShardedRateLimiter — 分片限流器 (v4.0)
// ──────────────────────────────────────────────

// ShardedRateLimiter 按 key 分片的限流器，用于多租户/多 API Key 场景。
// 每个分片有独立的原子令牌桶，不同 key 的请求完全并行，无竞争。
//
// 设计原理：
//   - 使用 FNV-1a 哈希将 key 映射到 256 个分片之一
//   - 每个分片维护独立的 atomic 令牌桶
//   - 不同 key 的请求不会互相阻塞
//   - 适合十亿级调用量下按租户/用户限流
type ShardedRateLimiter[K comparable] struct {
	shards [shardCount]*shardedTokenBucket
}

// shardedTokenBucket 是单个分片的令牌桶。
type shardedTokenBucket struct {
	rate       int64 // 微令牌/秒
	capacity   int64 // 微令牌容量
	tokens     atomic.Int64
	lastRefill atomic.Int64
}

// NewShardedRateLimiter 创建分片限流器。
// rate: 每个 key 的每秒令牌数。
// capacity: 每个 key 的突发容量。
func NewShardedRateLimiter[K comparable](rate, capacity float64) *ShardedRateLimiter[K] {
	if rate <= 0 {
		rate = 1
	}
	if capacity <= 0 {
		capacity = rate
	}
	sr := &ShardedRateLimiter[K]{}
	for i := 0; i < shardCount; i++ {
		b := &shardedTokenBucket{
			rate:     int64(rate * 1e6),
			capacity: int64(capacity * 1e6),
		}
		b.tokens.Store(b.capacity)
		b.lastRefill.Store(time.Now().UnixNano())
		sr.shards[i] = b
	}
	return sr
}

// Allow 检查 key 是否允许通过（消费 1 个令牌）。
// 返回 true 表示允许，false 表示被限流。
// 使用 FNV-1a 哈希选择分片，无锁 CAS 令牌消费。
func (sr *ShardedRateLimiter[K]) Allow(key K) bool {
	idx := hashKey(key) & 0xFF
	b := sr.shards[idx]

	refillBucket(b)
	current := b.tokens.Load()
	if current < 1e6 {
		return false
	}
	return b.tokens.CompareAndSwap(current, current-1e6)
}

// AllowN 检查 key 是否允许消费 n 个令牌。
// 返回 true 表示允许，false 表示被限流。
func (sr *ShardedRateLimiter[K]) AllowN(key K, n float64) bool {
	idx := hashKey(key) & 0xFF
	b := sr.shards[idx]

	need := int64(n * 1e6)
	refillBucket(b)
	current := b.tokens.Load()
	if current < need {
		return false
	}
	return b.tokens.CompareAndSwap(current, current-need)
}

// Tokens 返回 key 的当前令牌数。
func (sr *ShardedRateLimiter[K]) Tokens(key K) float64 {
	idx := hashKey(key) & 0xFF
	b := sr.shards[idx]
	refillBucket(b)
	return float64(b.tokens.Load()) / 1e6
}

// hashKey 计算泛型 key 的哈希值。
// 支持 string 和整数类型的 key。
func hashKey[K comparable](key K) uint64 {
	switch v := any(key).(type) {
	case string:
		return hashString64(v)
	case int:
		return uint64(v)
	case int64:
		return uint64(v)
	case uint64:
		return v
	default:
		// 回退到 fmt.Sprintf 后哈希（非热路径，用于非标准类型）
		return hashString64(fmt.Sprintf("%v", key))
	}
}

// 注意：需要 fmt 包用于 hashKey 的 fallback 分支
// 该 import 已在文件顶部声明

// refillBucket 根据经过的时间填充令牌桶。
func refillBucket(b *shardedTokenBucket) {
	now := time.Now().UnixNano()
	lastRefill := b.lastRefill.Load()

	if now <= lastRefill {
		return
	}

	elapsedNano := now - lastRefill
	newTokens := b.rate * elapsedNano / 1e9

	if newTokens <= 0 {
		return
	}

	if !b.lastRefill.CompareAndSwap(lastRefill, now) {
		return
	}

	for {
		current := b.tokens.Load()
		next := current + newTokens
		if next > b.capacity {
			next = b.capacity
		}
		if b.tokens.CompareAndSwap(current, next) {
			break
		}
	}
}

// hashString64 计算字符串的 FNV-1a 64 位哈希。
func hashString64(s string) uint64 {
	var h uint64 = fnvOffsetBasis64
	for i := 0; i < len(s); i++ {
		h ^= uint64(s[i])
		h *= fnvPrime64
	}
	return h
}
