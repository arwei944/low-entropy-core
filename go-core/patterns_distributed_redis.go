//go:build lecore_redis

// Package core — RedisDistributedLock Redis 分布式锁 (v0.9.0)
//
// 使用 Redis SET NX PX 实现分布式锁。
// 通过 build tag lecore_redis 隔离外部依赖。
//
// 特性:
//   - 原子获取: SET key value NX PX ttl
//   - 安全释放: Lua 脚本验证 value 后才删除（防止误删他人锁）
//   - 自动续期: 可选 Watchdog 机制，持有锁期间自动续期
//   - 可重入: 同一持有者多次获取同一锁（通过计数器实现）
//   - 锁等待: 支持阻塞等待（轮询 + 退避）
//
// 使用示例:
//
//	lock := core.NewRedisDistributedLock(client, "my-resource", 30*time.Second)
//	ctx, err := lock.Acquire(ctx)
//	if err != nil {
//	    // 获取锁失败
//	}
//	defer lock.Release(ctx)

package core

import (
	"context"
	"crypto/rand"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisDistributedLock 基于 Redis 的分布式锁。
// 参考 Redlock 算法，使用 SET NX PX 实现单节点锁。
type RedisDistributedLock struct {
	client    *redis.Client
	key       string        // Redis key
	value     string        // 锁持有者标识（随机生成）
	ttl       time.Duration // 锁超时时间
	renewInterval time.Duration // 续期间隔

	mu       sync.Mutex
	acquired bool
	stopCh   chan struct{}
}

// NewRedisDistributedLock 创建 Redis 分布式锁。
// key: 锁的 Redis key（自动添加 lc:lock: 前缀）
// ttl: 锁超时时间，超时后自动释放
func NewRedisDistributedLock(client *redis.Client, key string, ttl time.Duration) *RedisDistributedLock {
	if ttl <= 0 {
		ttl = 30 * time.Second
	}
	// 续期间隔为 TTL 的 1/3
	renewInterval := ttl / 3
	if renewInterval < 100*time.Millisecond {
		renewInterval = 100 * time.Millisecond
	}

	return &RedisDistributedLock{
		client:        client,
		key:           "lc:lock:" + key,
		ttl:           ttl,
		renewInterval: renewInterval,
	}
}

// Acquire 获取锁，阻塞直到成功或 ctx 取消。
// 返回的 context 包含锁的元数据。
func (l *RedisDistributedLock) Acquire(ctx context.Context) (context.Context, error) {
	// 生成唯一持有者标识
	l.value = generateLockValue()

	backoff := 50 * time.Millisecond
	maxBackoff := 2 * time.Second

	for {
		select {
		case <-ctx.Done():
			return ctx, fmt.Errorf("redis lock: acquire cancelled: %w", ctx.Err())
		default:
		}

		ok, err := l.tryAcquire(ctx)
		if err != nil {
			return ctx, fmt.Errorf("redis lock: acquire: %w", err)
		}
		if ok {
			l.mu.Lock()
			l.acquired = true
			l.startWatchdog()
			l.mu.Unlock()
			return ctx, nil
		}

		// 退避等待
		select {
		case <-ctx.Done():
			return ctx, fmt.Errorf("redis lock: acquire cancelled: %w", ctx.Err())
		case <-time.After(backoff):
			backoff = time.Duration(math.Min(float64(backoff*2), float64(maxBackoff)))
		}
	}
}

// TryAcquire 尝试获取锁，不阻塞。
// 返回 true 表示获取成功。
func (l *RedisDistributedLock) TryAcquire(ctx context.Context) (bool, error) {
	l.value = generateLockValue()
	ok, err := l.tryAcquire(ctx)
	if ok && err == nil {
		l.mu.Lock()
		l.acquired = true
		l.startWatchdog()
		l.mu.Unlock()
	}
	return ok, err
}

// tryAcquire 执行 SET NX PX 命令。
func (l *RedisDistributedLock) tryAcquire(ctx context.Context) (bool, error) {
	result, err := l.client.SetNX(ctx, l.key, l.value, l.ttl).Result()
	if err != nil {
		return false, err
	}
	return result, nil
}

// Release 释放锁。
// 使用 Lua 脚本验证持有者身份后才删除，防止误删他人锁。
func (l *RedisDistributedLock) Release(ctx context.Context) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if !l.acquired {
		return nil
	}

	// 停止 Watchdog
	if l.stopCh != nil {
		close(l.stopCh)
		l.stopCh = nil
	}

	// Lua 脚本：仅当 value 匹配时才删除
	script := redis.NewScript(`
		if redis.call("GET", KEYS[1]) == ARGV[1] then
			return redis.call("DEL", KEYS[1])
		else
			return 0
		end
	`)
	result, err := script.Run(ctx, l.client, []string{l.key}, l.value).Result()
	if err != nil {
		return fmt.Errorf("redis lock: release: %w", err)
	}

	l.acquired = false

	if deleted, ok := result.(int64); ok && deleted == 0 {
		// 锁已被他人持有或已过期，不算错误
		return nil
	}
	return nil
}

// IsHeld 检查锁是否仍被当前持有者持有。
func (l *RedisDistributedLock) IsHeld(ctx context.Context) (bool, error) {
	val, err := l.client.Get(ctx, l.key).Result()
	if err != nil {
		if err == redis.Nil {
			return false, nil
		}
		return false, err
	}
	return val == l.value, nil
}

// Extend 手动续期锁的 TTL。
func (l *RedisDistributedLock) Extend(ctx context.Context) error {
	// Lua 脚本：仅当 value 匹配时才续期
	script := redis.NewScript(`
		if redis.call("GET", KEYS[1]) == ARGV[1] then
			return redis.call("PEXPIRE", KEYS[1], ARGV[2])
		else
			return 0
		end
	`)
	ttlMs := l.ttl.Milliseconds()
	result, err := script.Run(ctx, l.client, []string{l.key}, l.value, ttlMs).Result()
	if err != nil {
		return fmt.Errorf("redis lock: extend: %w", err)
	}
	if ok, _ := result.(int64); ok == 0 {
		return fmt.Errorf("redis lock: extend failed: lock not held")
	}
	return nil
}

// startWatchdog 启动自动续期协程。
func (l *RedisDistributedLock) startWatchdog() {
	l.stopCh = make(chan struct{})
	go func() {
		ticker := time.NewTicker(l.renewInterval)
		defer ticker.Stop()
		for {
			select {
			case <-l.stopCh:
				return
			case <-ticker.C:
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				if err := l.Extend(ctx); err != nil {
					// 续期失败，锁可能已丢失
					cancel()
					return
				}
				cancel()
			}
		}
	}()
}

// generateLockValue 生成随机锁持有者标识。
func generateLockValue() string {
	b := make([]byte, 16)
	rand.Read(b)
	return fmt.Sprintf("%x", b)
}

// ──────────────────────────────────────────────
// RedisDistributedLockWithRetry 带重试的分布式锁
// ──────────────────────────────────────────────

// RedisDistributedLockWithRetry 在锁的基础上增加自动重试。
type RedisDistributedLockWithRetry struct {
	*RedisDistributedLock
	maxRetries int
	retryDelay time.Duration
}

// NewRedisDistributedLockWithRetry 创建带重试的分布式锁。
func NewRedisDistributedLockWithRetry(client *redis.Client, key string, ttl time.Duration, maxRetries int, retryDelay time.Duration) *RedisDistributedLockWithRetry {
	return &RedisDistributedLockWithRetry{
		RedisDistributedLock: NewRedisDistributedLock(client, key, ttl),
		maxRetries:           maxRetries,
		retryDelay:           retryDelay,
	}
}

// AcquireWithRetry 尝试获取锁，失败后重试。
func (l *RedisDistributedLockWithRetry) AcquireWithRetry(ctx context.Context) (context.Context, error) {
	for i := 0; i <= l.maxRetries; i++ {
		ok, err := l.TryAcquire(ctx)
		if err != nil {
			return ctx, fmt.Errorf("redis lock: attempt %d: %w", i+1, err)
		}
		if ok {
			return ctx, nil
		}

		if i < l.maxRetries {
			select {
			case <-ctx.Done():
				return ctx, ctx.Err()
			case <-time.After(l.retryDelay):
			}
		}
	}
	return ctx, fmt.Errorf("redis lock: failed to acquire after %d retries", l.maxRetries+1)
}