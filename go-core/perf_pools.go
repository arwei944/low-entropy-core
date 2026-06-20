// Package core — sync.Pool 集合
package core

import (
	"crypto/sha256"
	"hash"
	"strings"
	"sync"
)

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
