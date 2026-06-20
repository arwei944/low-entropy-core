// Package core — 性能基础设施 (常量 + ShardedLock)
package core

import (
	"fmt"
	"sync"
)

// ──────────────────────────────────────────────────────────────────────────────
// 常量
// ──────────────────────────────────────────────────────────────────────────────

const (
	// shardCount 是 ShardedLock 的分片数量。使用 256 分片（2 的幂），
	// 可通过位运算 `hash & 0xFF` 快速定位分片，避免取模运算。
	shardCount = 256

	// fnvOffsetBasis64 是 FNV-1a 64 位哈希的初始偏移量。
	fnvOffsetBasis64 uint64 = 14695981039346656037

	// fnvPrime64 是 FNV-1a 64 位哈希的质数乘数。
	fnvPrime64 uint64 = 1099511628211

	// uuidBatchSize 是 BatchedUUIDGen 每次预生成的 UUID 数量。
	uuidBatchSize = 256

	// uuidRefillThreshold 是触发后台填充的通道剩余数量阈值。
	// 当通道中剩余数量 <= 此值时，触发后台填充。
	uuidRefillThreshold = uuidBatchSize / 2
)

// ──────────────────────────────────────────────────────────────────────────────
// ShardedLock — 泛型分片读写锁
// ──────────────────────────────────────────────────────────────────────────────

// ShardedLock 是一个泛型分片读写锁，用于减少高并发场景下的锁竞争。
//
// 它将键空间划分为 256 个分片，每个分片由一个独立的 sync.RWMutex 保护。
// 使用 FNV-1a 64 位哈希算法将键映射到分片，确保均匀分布。
//
// ShardedLock 的核心优势在于：对不相关键的并发操作完全并行，
// 避免了单个 sync.Map 或全局 sync.RWMutex 在高并发下的瓶颈。
//
// 内部使用固定大小的 [256]sync.RWMutex 数组而非切片，
// 避免热路径上的边界检查开销。
//
// 使用示例：
//
//	lock := NewShardedLock[string]()
//	lock.Lock("user:123")
//	defer lock.Unlock("user:123")
//	// 临界区代码
type ShardedLock[K comparable] struct {
	shards [shardCount]sync.RWMutex
}

// NewShardedLock 创建一个新的 ShardedLock 实例。
//
// 256 个 sync.RWMutex 的零值即可直接使用，无需额外初始化。
func NewShardedLock[K comparable]() *ShardedLock[K] {
	return &ShardedLock[K]{}
}

// hash 使用 FNV-1a 64 位哈希算法计算键的哈希值。
//
// 算法步骤：
//  1. 将键转换为字符串表示
//  2. 对字符串的每个字节应用 FNV-1a 算法：hash ^= byte; hash *= prime
//
// 注意：键到字符串的转换使用 fmt.Sprint，会产生一次分配。
// 对于热路径场景，可考虑实现自定义哈希接口来消除分配。
func (sl *ShardedLock[K]) hash(key K) uint64 {
	s := fmt.Sprint(key)
	h := fnvOffsetBasis64
	for i := 0; i < len(s); i++ {
		h ^= uint64(s[i])
		h *= fnvPrime64
	}
	return h
}

// shard 返回键对应的分片索引。
//
// 使用位运算 hash & (shardCount - 1) 替代取模运算，
// 因为 shardCount 是 2 的幂（256），此操作等价于 hash % 256。
func (sl *ShardedLock[K]) shard(key K) int {
	return int(sl.hash(key) & (shardCount - 1))
}

// Lock 获取指定键的写锁（排他锁）。
//
// 调用 goroutine 将阻塞直到获取到锁。
// 持有写锁期间，其他 goroutine 无法获取该分片的读锁或写锁。
//
// 必须与 Unlock 配对使用，使用 defer 确保释放：
//
//	lock.Lock(key)
//	defer lock.Unlock(key)
func (sl *ShardedLock[K]) Lock(key K) {
	sl.shards[sl.shard(key)].Lock()
}

// Unlock 释放指定键的写锁。
//
// 必须在 Lock 之后调用，且只能由持有锁的 goroutine 调用。
// 对未锁定的分片调用 Unlock 将导致 panic。
func (sl *ShardedLock[K]) Unlock(key K) {
	sl.shards[sl.shard(key)].Unlock()
}

// RLock 获取指定键的读锁（共享锁）。
//
// 多个 goroutine 可以同时持有同一分片的读锁，
// 但在有写锁持有者时，读锁请求将被阻塞。
//
// 必须与 RUnlock 配对使用。
func (sl *ShardedLock[K]) RLock(key K) {
	sl.shards[sl.shard(key)].RLock()
}

// RUnlock 释放指定键的读锁。
//
// 必须在 RLock 之后调用，且只能由持有读锁的 goroutine 调用。
func (sl *ShardedLock[K]) RUnlock(key K) {
	sl.shards[sl.shard(key)].RUnlock()
}
