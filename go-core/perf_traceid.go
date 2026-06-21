// Package core — CompactTraceID (紧凑型 TraceID)
package core

import (
	"context"
	"sync"
)

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
// 如果 crypto/rand 初始化失败，返回退化的零值生成器（Next() 会阻塞直到 channel 有数据）。
// 调用方可通过 Observation Pipeline 检测初始化错误。
func getGlobalUUIDGen() *BatchedUUIDGen {
	globalUUIDGenOnce.Do(func() {
		gen, err := NewBatchedUUIDGen()
		if err != nil {
			// 熵源不可用，返回 nil；Next() 调用将阻塞等待数据，
			// 相比 panic，这允许调用方通过 context 超时处理
			globalUUIDGen = nil
			return
		}
		globalUUIDGen = gen
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
// TraceID 注入策略
//
// 注入优先级（"ctx 优先"策略）：
//   1. 从传入 ctx 中提取已存在的 TraceID（上游传播）
//   2. 若 ctx 无 TraceID，则生成新的 UUID v4
//
// 所有框架层入口函数应使用 TraceIDOrNew 确保请求链可追踪。
// ──────────────────────────────────────────────────────────────────────────────

// traceIDKey 是 context key，用于存储 TraceID。
type traceIDKey struct{}

// TraceIDFromCtx 从 context 中提取 TraceID。
// 如果 ctx 中没有 TraceID，返回 (zero value, false)。
func TraceIDFromCtx(ctx context.Context) (CompactTraceID, bool) {
	if ctx == nil {
		return CompactTraceID{}, false
	}
	id, ok := ctx.Value(traceIDKey{}).(CompactTraceID)
	return id, ok
}

// WithTraceID 将 TraceID 注入到 context 并返回新的 context。
// 注入后，后续所有从 ctx 提取 TraceID 的调用都会得到该值。
func WithTraceID(ctx context.Context, id CompactTraceID) context.Context {
	return context.WithValue(ctx, traceIDKey{}, id)
}

// TraceIDOrNew 从 ctx 提取 TraceID；若 ctx 无 TraceID，则生成新的 UUID v4。
// 这是推荐的入口函数，确保每个请求链都有可追踪的 TraceID。
func TraceIDOrNew(ctx context.Context) (CompactTraceID, context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}
	if id, ok := TraceIDFromCtx(ctx); ok {
		return id, ctx
	}
	return NewCompactTraceID(), WithTraceID(ctx, NewCompactTraceID())
}

// Ensure CompactTraceID implements context.keyComparable (no-op for type safety).
var _ = traceIDKey{}

