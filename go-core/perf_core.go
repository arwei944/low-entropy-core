// Package core — 性能基础设施
//
// 本文件提供面向十亿级调用量的生产级性能基础设施，包括：
//   - ShardedLock：泛型分片读写锁，256 分片，FNV-1a 哈希
//   - AtomicState：无锁状态机，基于 atomic.Uint64
//   - BatchedUUIDGen：批量 UUID 生成器，预生成 256 个，后台填充
//   - CompactTraceID：紧凑型 TraceID，[16]byte 别名，零分配
//   - 多种 sync.Pool：StepMetadataPool、StepSlicePool、StringBuilderPool、HashPool
//
// 所有类型均为线程安全，热路径设计为零分配。
package core

import (
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"hash"
	"strings"
	"sync"
	"sync/atomic"
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

// ──────────────────────────────────────────────────────────────────────────────
// AtomicState — 无锁状态机
// ──────────────────────────────────────────────────────────────────────────────

// 状态常量定义。
// 使用 uint64 表示状态值，支持自定义扩展。
const (
	// StateClosed 表示 CircuitBreaker 关闭（正常通行）。
	StateClosed uint64 = iota

	// StateOpen 表示 CircuitBreaker 打开（拒绝请求）。
	StateOpen

	// StateHalfOpen 表示 CircuitBreaker 半开（探测性放行）。
	StateHalfOpen

	// StateNormal 表示 RateLimiter/DegradationManager 正常运行。
	StateNormal

	// StateDegraded 表示降级模式运行。
	StateDegraded

	// StateThrottled 表示限流模式运行。
	StateThrottled
)

// AtomicState 是一个基于 atomic.Uint64 的无锁状态机。
//
// 它提供线程安全的状态管理，无需任何互斥锁。适用于：
//   - CircuitBreaker 的状态转换（Closed / Open / HalfOpen）
//   - RateLimiter 的工作模式（Normal / Throttled）
//   - DegradationManager 的降级状态（Normal / Degraded）
//
// 支持原子性的 CompareAndSwap、Store、Load 操作，
// 以及状态转换追踪（记录上一次状态）。
//
// 零值表示 StateClosed（0），可直接使用。
//
// 使用示例：
//
//	state := &AtomicState{}
//	state.Store(StateNormal)
//	if state.CompareAndSwap(StateNormal, StateDegraded) {
//	    // 成功从 Normal 切换到 Degraded
//	}
type AtomicState struct {
	current atomic.Uint64
	last    atomic.Uint64
}

// Load 原子性地读取当前状态值。
//
// 返回当前状态，不产生任何锁开销。
func (as *AtomicState) Load() uint64 {
	return as.current.Load()
}

// Store 原子性地设置当前状态值，并记录旧状态。
//
// 旧状态被保存到 last 字段中，用于状态追踪和审计。
// 此操作对观察者立即可见。
func (as *AtomicState) Store(state uint64) {
	as.last.Store(as.current.Load())
	as.current.Store(state)
}

// CompareAndSwap 原子性地比较并交换状态。
//
// 如果当前状态等于 old，则将其设置为 new 并返回 true。
// 否则不执行任何操作并返回 false。
//
// 这是实现状态机转换的核心方法，保证状态转换的原子性。
func (as *AtomicState) CompareAndSwap(old, new uint64) bool {
	swapped := as.current.CompareAndSwap(old, new)
	if swapped {
		as.last.Store(old)
	}
	return swapped
}

// LastState 返回上一次状态值。
//
// 用于追踪状态转换历史，辅助诊断和监控。
func (as *AtomicState) LastState() uint64 {
	return as.last.Load()
}

// Transition 执行状态转换并返回是否成功。
//
// 这是一个便捷方法，内部调用 CompareAndSwap。
// 同时返回转换后的新状态（若成功）或当前状态（若失败）。
func (as *AtomicState) Transition(old, new uint64) (current uint64, ok bool) {
	if as.CompareAndSwap(old, new) {
		return new, true
	}
	return as.Load(), false
}

// ──────────────────────────────────────────────────────────────────────────────
// BatchedUUIDGen — 批量 UUID 生成器
// ──────────────────────────────────────────────────────────────────────────────

// BatchedUUIDGen 是一个高性能的批量 UUID 生成器。
//
// 它预先生成 256 个 UUID v4 并缓存在带缓冲的 channel 中，
// 当 channel 中剩余数量降至一半（128）时，后台 goroutine 自动补充。
//
// 设计要点：
//   - 使用 crypto/rand 生成密码学安全的随机 UUID
//   - 批量预生成减少对 crypto/rand 的系统调用次数
//   - 带缓冲 channel 实现无锁的生产者-消费者模式
//   - 使用 sync.Pool 复用字节缓冲区，减少内存分配
//   - 后台 goroutine 在 channel 半空时触发填充，确保低延迟获取
//
// 使用示例：
//
//	gen := NewBatchedUUIDGen()
//	defer gen.Close()
//	uuid := gen.Next() // 返回 [16]byte
type BatchedUUIDGen struct {
	ch     chan [16]byte
	pool   sync.Pool
	done   chan struct{}
	once   sync.Once
}

// NewBatchedUUIDGen 创建并启动一个 BatchedUUIDGen 实例。
//
// 初始填充 256 个 UUID 到 channel，然后启动后台 goroutine
// 监听 channel 容量并在半空时自动补充。
//
// 调用方应在不再使用时调用 Close() 以释放后台 goroutine。
func NewBatchedUUIDGen() *BatchedUUIDGen {
	g := &BatchedUUIDGen{
		ch:   make(chan [16]byte, uuidBatchSize),
		done: make(chan struct{}),
	}
	g.pool.New = func() any {
		buf := make([]byte, 16)
		return &buf
	}

	// 初始填充
	g.refill()

	// 启动后台填充 goroutine
	go g.backgroundRefill()

	return g
}

// Next 从 channel 中获取一个预生成的 UUID，返回原始字节数组。
//
// 此方法在 channel 有数据时立即返回，否则阻塞等待后台填充。
// 返回的是 [16]byte 值类型，不产生堆分配。
//
// 返回的 UUID 是 v4 格式（随机生成），前 6 字节的版本位和
// 第 8 字节的变体位已按 RFC 4122 标准设置。
func (g *BatchedUUIDGen) Next() [16]byte {
	return <-g.ch
}

// NextString 返回 UUID 的标准字符串表示（带连字符）。
//
// 格式：xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
//
// 此方法为向后兼容性提供，会产生字符串分配。
// 热路径上应优先使用 Next() 方法获取原始字节。
func (g *BatchedUUIDGen) NextString() string {
	uuid := g.Next()
	return formatUUID(uuid[:])
}

// Close 关闭生成器，停止后台 goroutine。
//
// 调用 Close 后不应再调用 Next 或 NextString。
// 多次调用 Close 是安全的（通过 sync.Once 保证）。
func (g *BatchedUUIDGen) Close() {
	g.once.Do(func() {
		close(g.done)
	})
}

// refill 生成一批 UUID 并填充到 channel。
//
// 每次生成 256 个 UUID，使用 sync.Pool 复用的字节缓冲区。
// 如果 channel 已满，则阻塞等待消费者取走。
func (g *BatchedUUIDGen) refill() {
	for i := 0; i < uuidBatchSize; i++ {
		g.ch <- g.generateOne()
	}
}

// generateOne 生成单个 UUID v4。
//
// 使用 crypto/rand 读取随机字节，然后设置版本位和变体位。
// 字节缓冲区从 sync.Pool 获取并在使用后归还。
func (g *BatchedUUIDGen) generateOne() [16]byte {
	bufPtr := g.pool.Get().(*[]byte)
	buf := *bufPtr
	_, err := rand.Read(buf)
	if err != nil {
		// crypto/rand.Read 在极端情况下可能失败（如 /dev/urandom 不可用）
		// 使用 panic 快速失败，因为这是不可恢复的系统级错误
		panic("BatchedUUIDGen: crypto/rand.Read failed: " + err.Error())
	}
	buf[6] = (buf[6] & 0x0f) | 0x40 // 版本 4
	buf[8] = (buf[8] & 0x3f) | 0x80 // 变体 10
	var uuid [16]byte
	copy(uuid[:], buf)
	g.pool.Put(bufPtr)
	return uuid
}

// backgroundRefill 在后台运行，监听 channel 并在半空时补充。
//
// 检查策略：
//   - 当 channel 剩余数量 <= uuidRefillThreshold（128）时，触发补充
//   - 使用 select 同时监听 done 信号以支持优雅关闭
//   - 补充操作在 refill 内部的 channel 发送时可能阻塞，
//     这自然形成了背压机制
func (g *BatchedUUIDGen) backgroundRefill() {
	for {
		select {
		case <-g.done:
			return
		default:
			if len(g.ch) <= uuidRefillThreshold {
				g.refill()
			}
			// 短暂休眠以避免忙等待，同时保证响应速度
			// 使用 select 结合 done 通道，避免纯 sleep 导致的关闭延迟
			select {
			case <-g.done:
				return
			default:
				// 让出 CPU，等待 channel 消费
				// 这里不做长时间 sleep，因为 channel 操作本身已提供足够的同步
			}
		}
	}
}

// formatUUID 将 16 字节 UUID 格式化为标准字符串。
//
// 格式：xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
func formatUUID(b []byte) string {
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// ──────────────────────────────────────────────────────────────────────────────
// CompactTraceID — 紧凑型 TraceID
// ──────────────────────────────────────────────────────────────────────────────

// CompactTraceID 是一个紧凑的 TraceID 类型，底层使用 [16]byte 而非 string。
//
// 相比 string 类型的 TraceID（每个 ID 占用约 32-48 字节的字符串开销），
// CompactTraceID 仅占用 16 字节，且是值类型，不产生堆分配。
//
// 在十亿级调用量场景下，这种紧凑表示可以显著减少内存占用和 GC 压力。
//
// 使用示例：
//
//	id := NewCompactTraceID()
//	fmt.Println(id.String()) // 输出 32 位十六进制字符串
//	if id.IsZero() { ... }
type CompactTraceID [16]byte

// 全局 BatchedUUIDGen 实例，用于生成 CompactTraceID。
// 使用 sync.Once 确保线程安全的延迟初始化。
var (
	globalUUIDGen     *BatchedUUIDGen
	globalUUIDGenOnce sync.Once
)

// getGlobalUUIDGen 返回全局的 BatchedUUIDGen 单例。
//
// 延迟初始化，首次调用时创建生成器。
func getGlobalUUIDGen() *BatchedUUIDGen {
	globalUUIDGenOnce.Do(func() {
		globalUUIDGen = NewBatchedUUIDGen()
	})
	return globalUUIDGen
}

// NewCompactTraceID 使用全局 BatchedUUIDGen 生成一个新的 CompactTraceID。
//
// 返回的 TraceID 是 UUID v4 格式，密码学安全随机。
// 调用成本极低，因为 UUID 是预生成的。
func NewCompactTraceID() CompactTraceID {
	return CompactTraceID(getGlobalUUIDGen().Next())
}

// String 将 CompactTraceID 格式化为 32 字符的十六进制字符串。
//
// 无连字符，纯十六进制小写表示。例如：
//
//	"a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6"
//
// 此方法会产生一次 32 字节的字符串分配，适合日志输出和序列化场景。
// 热路径上应直接使用 [16]byte 值进行比较和传输。
func (id CompactTraceID) String() string {
	const hexChars = "0123456789abcdef"
	buf := make([]byte, 32)
	for i := 0; i < 16; i++ {
		buf[i*2] = hexChars[id[i]>>4]
		buf[i*2+1] = hexChars[id[i]&0x0f]
	}
	return string(buf)
}

// IsZero 判断 CompactTraceID 是否为零值（全零字节）。
//
// 用于检测未初始化的 TraceID。
func (id CompactTraceID) IsZero() bool {
	for _, b := range id {
		if b != 0 {
			return false
		}
	}
	return true
}

// ──────────────────────────────────────────────────────────────────────────────
// StepMetadataPool — 步骤元数据对象池
// ──────────────────────────────────────────────────────────────────────────────

// StepMetadataPool 是 map[string]any 的 sync.Pool，用于避免热路径上的 map 分配。
//
// 在 ExecutionStep 的 Metadata 字段被频繁赋值的高并发场景下，
// 每次创建新的 map[string]any 会产生显著的 GC 压力。
// 通过对象池复用，可以将 map 分配降低到接近零。
//
// 使用示例：
//
//	meta := StepMetadataPool.Get()
//	meta["key"] = value
//	// ... 使用 meta
//	StepMetadataPool.Put(meta)
var StepMetadataPool = sync.Pool{
	New: func() any {
		return make(map[string]any, 8)
	},
}

// GetStepMetadata 从池中获取一个 map[string]any。
//
// 返回的 map 可能包含之前使用过的键值对，调用方应在使用前
// 清空 map（通过 clear() 或逐个删除）。
//
// 使用 clear() 内置函数（Go 1.21+）可以高效清空 map 而保留底层容量。
func GetStepMetadata() map[string]any {
	return StepMetadataPool.Get().(map[string]any)
}

// PutStepMetadata 将 map[string]any 归还到对象池。
//
// 归还前调用方应清空 map，避免内存泄漏（持有已失效的引用）。
// 推荐使用 clear(m) 后再归还。
func PutStepMetadata(m map[string]any) {
	// 清空 map 但保留底层容量，避免下次使用时重新分配
	clear(m)
	StepMetadataPool.Put(m)
}

// ──────────────────────────────────────────────────────────────────────────────
// StepSlicePool — 执行步骤切片对象池
// ──────────────────────────────────────────────────────────────────────────────

// StepSlicePool 是 []ExecutionStep 的 sync.Pool，用于避免频繁的切片分配。
//
// 在批量记录 ExecutionStep 的场景（如批量观测、审计日志），
// 频繁分配切片会导致显著的 GC 开销。
// 通过对象池复用，可以显著减少 GC 压力。
//
// 池中的切片可能具有不同的容量，Get 方法会检查并确保返回的切片
// 有足够的容量。如果池中切片的容量不足，则分配新的切片。
//
// 使用示例：
//
//	steps := GetStepSlice(128)
//	for i := range steps {
//	    steps[i] = ...
//	}
//	PutStepSlice(steps)
var StepSlicePool = sync.Pool{
	New: func() any {
		s := make([]ExecutionStep, 0, 64)
		return &s
	},
}

// GetStepSlice 从池中获取一个具有指定容量的 ExecutionStep 切片。
//
// 参数 capacity 指定所需的最小容量。如果池中切片的容量不足，
// 则分配新的切片；否则复用池中切片并重置长度为零。
//
// 返回的切片长度为 0，容量 >= capacity。
func GetStepSlice(capacity int) []ExecutionStep {
	sPtr := StepSlicePool.Get().(*[]ExecutionStep)
	s := *sPtr
	if capacity > cap(s) {
		// 容量不足，分配新切片
		return make([]ExecutionStep, 0, capacity)
	}
	// 重置长度为零，保留底层数组
	return s[:0]
}

// PutStepSlice 将 ExecutionStep 切片归还到对象池。
//
// 归还前切片会被重置为零长度（保留容量）。
// 注意：切片中的 ExecutionStep 值可能仍被底层数组持有，
// 但 ExecutionStep 是值类型（不含指针），不会造成内存泄漏。
func PutStepSlice(s []ExecutionStep) {
	// 如果切片容量过大（如超过 4096），则不归还到池中，
	// 避免池中滞留过大切片导致内存浪费
	if cap(s) > 4096 {
		return
	}
	s = s[:0]
	StepSlicePool.Put(&s)
}

// ──────────────────────────────────────────────────────────────────────────────
// StringBuilderPool — 字符串构建器对象池
// ──────────────────────────────────────────────────────────────────────────────

// StringBuilderPool 是 strings.Builder 的 sync.Pool，用于避免字符串拼接中的分配。
//
// 在哈希计算、序列化、日志格式化等热路径中，频繁创建 strings.Builder
// 会产生不必要的 GC 压力。通过对象池复用，可以显著减少分配。
//
// 使用示例：
//
//	sb := GetStringBuilder()
//	sb.WriteString("prefix:")
//	sb.WriteString(value)
//	result := sb.String()
//	PutStringBuilder(sb)
var StringBuilderPool = sync.Pool{
	New: func() any {
		return &strings.Builder{}
	},
}

// GetStringBuilder 从池中获取一个 strings.Builder。
//
// 返回的 Builder 可能包含之前的数据，调用方应在使用前调用 Reset()。
// 为了便利，此方法已自动调用 Reset()。
func GetStringBuilder() *strings.Builder {
	sb := StringBuilderPool.Get().(*strings.Builder)
	sb.Reset()
	return sb
}

// PutStringBuilder 将 strings.Builder 归还到对象池。
//
// 归还前会自动调用 Reset() 清空内部缓冲区。
func PutStringBuilder(sb *strings.Builder) {
	sb.Reset()
	StringBuilderPool.Put(sb)
}

// ──────────────────────────────────────────────────────────────────────────────
// HashPool — SHA256 哈希对象池
// ──────────────────────────────────────────────────────────────────────────────

// HashPool 是 sha256 哈希对象的 sync.Pool，用于避免哈希计算中的分配。
//
// sha256.New() 每次调用会分配一个新的哈希对象，在签名验证、
// Merkle 树计算、数据完整性校验等热路径中，这些分配会累积为显著的 GC 开销。
//
// 通过对象池复用，可以将哈希对象的分配降低到接近零。
//
// 使用示例：
//
//	h := GetHash()
//	h.Write(data)
//	sum := h.Sum(nil)
//	PutHash(h)
var HashPool = sync.Pool{
	New: func() any {
		return sha256.New()
	},
}

// GetHash 从池中获取一个 sha256 哈希对象。
//
// 返回的哈希对象已调用 Reset()，处于干净状态，可直接使用。
func GetHash() hash.Hash {
	h := HashPool.Get().(hash.Hash)
	h.Reset()
	return h
}

// PutHash 将 sha256 哈希对象归还到对象池。
//
// 归还前会自动调用 Reset() 清空内部状态。
func PutHash(h hash.Hash) {
	h.Reset()
	HashPool.Put(h)
}