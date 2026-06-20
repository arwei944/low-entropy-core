//go:build lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

// Package core — 共享并发工具
// 提供分片锁、哈希分区等通用并发原语，供 Guardian 等模块使用。
// v0.9.0+ 从 guardian_entropy.go 提取。

package core

import (
	"fmt"
	"sync"
	"sync/atomic"
)

// DefaultShardCount 默认分片数（256 分片，适合 10K+ 并发 goroutine）。
const DefaultShardCount = 256

// ShardedMap 泛型分片并发 Map。
// 通过哈希分区将 key 映射到固定数量的分片，
// 不同分片之间的操作完全并行，无需全局锁。
type ShardedMap[K comparable, V any] struct {
	shards [DefaultShardCount]*shard[K, V]
	count  atomic.Int64
}

type shard[K comparable, V any] struct {
	mu   sync.RWMutex
	data map[K]V
}

// NewShardedMap 创建分片 Map。
func NewShardedMap[K comparable, V any]() *ShardedMap[K, V] {
	m := &ShardedMap[K, V]{}
	for i := 0; i < DefaultShardCount; i++ {
		m.shards[i] = &shard[K, V]{
			data: make(map[K]V),
		}
	}
	return m
}

// shardIndex 计算 key 的分片索引。
func (m *ShardedMap[K, V]) shardIndex(key K) int {
	return hashAny(key) % DefaultShardCount
}

// Get 获取 key 对应的值。
func (m *ShardedMap[K, V]) Get(key K) (V, bool) {
	s := m.shards[m.shardIndex(key)]
	s.mu.RLock()
	v, ok := s.data[key]
	s.mu.RUnlock()
	return v, ok
}

// Set 设置 key 对应的值。
func (m *ShardedMap[K, V]) Set(key K, value V) {
	s := m.shards[m.shardIndex(key)]
	s.mu.Lock()
	if _, exists := s.data[key]; !exists {
		m.count.Add(1)
	}
	s.data[key] = value
	s.mu.Unlock()
}

// Delete 删除 key。
func (m *ShardedMap[K, V]) Delete(key K) {
	s := m.shards[m.shardIndex(key)]
	s.mu.Lock()
	if _, exists := s.data[key]; exists {
		m.count.Add(-1)
		delete(s.data, key)
	}
	s.mu.Unlock()
}

// Update 原子更新 key 对应的值。如果 key 不存在，fn 不会被调用。
func (m *ShardedMap[K, V]) Update(key K, fn func(v V) V) bool {
	s := m.shards[m.shardIndex(key)]
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.data[key]
	if !ok {
		return false
	}
	s.data[key] = fn(v)
	return true
}

// GetOrSet 获取 key 的值，如果不存在则设置并返回默认值。
func (m *ShardedMap[K, V]) GetOrSet(key K, defaultVal V) (V, bool) {
	s := m.shards[m.shardIndex(key)]
	s.mu.Lock()
	defer s.mu.Unlock()
	if v, ok := s.data[key]; ok {
		return v, true
	}
	s.data[key] = defaultVal
	m.count.Add(1)
	return defaultVal, false
}

// Len 返回当前元素数量（近似值，非原子快照）。
func (m *ShardedMap[K, V]) Len() int64 {
	return m.count.Load()
}

// Range 遍历所有元素。fn 返回 false 时停止遍历。
// 注意：遍历期间不持有全局锁，fn 内不应调用可能死锁的 ShardedMap 方法。
func (m *ShardedMap[K, V]) Range(fn func(K, V) bool) {
	for i := 0; i < DefaultShardCount; i++ {
		s := m.shards[i]
		s.mu.RLock()
		for k, v := range s.data {
			if !fn(k, v) {
				s.mu.RUnlock()
				return
			}
		}
		s.mu.RUnlock()
	}
}

// ShardedCounter 分片计数器，适合高并发计数场景。
type ShardedCounter struct {
	shards [DefaultShardCount]atomic.Int64
}

// Add 增加计数。
func (c *ShardedCounter) Add(delta int64) {
	c.shards[hashGoroutineID()%DefaultShardCount].Add(delta)
}

// Load 读取总计数值（近似值，非原子快照）。
func (c *ShardedCounter) Load() int64 {
	var total int64
	for i := 0; i < DefaultShardCount; i++ {
		total += c.shards[i].Load()
	}
	return total
}

// ──────────────────────────────────────────────
// 内部辅助函数
// ──────────────────────────────────────────────

// hashAny 对任意 comparable 类型计算哈希。
func hashAny[K comparable](key K) int {
	return int(hashString(fmt.Sprintf("%v", key)))
}


// goroutineID 返回当前 goroutine 的近似 ID（用于 ShardedCounter 分片）。
// 使用 runtime 的快速路径，避免 cgo 调用。
func hashGoroutineID() int {
	// 使用简单的基于时间的哈希，避免 runtime.Stack 开销
	// 在生产环境中可替换为更精确的 goroutine ID 获取方式
	return int(gidCounter.Add(1))
}

var gidCounter atomic.Int64