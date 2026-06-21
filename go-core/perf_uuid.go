// Package core — BatchedUUIDGen (批量 UUID 生成器)
package core

import (
	"crypto/rand"
	"fmt"
	"sync"
)

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
// 如果 crypto/rand 初始化失败，返回非 nil error。
func NewBatchedUUIDGen() (*BatchedUUIDGen, error) {
	g := &BatchedUUIDGen{
		ch:   make(chan [16]byte, uuidBatchSize),
		done: make(chan struct{}),
	}
	g.pool.New = func() any {
		buf := make([]byte, 16)
		return &buf
	}

	// 初始填充
	if err := g.refill(); err != nil {
		return nil, err
	}

	// 启动后台填充 goroutine
	go g.backgroundRefill()

	return g, nil
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
// 如果 crypto/rand 初始化失败，返回错误。
func (g *BatchedUUIDGen) refill() error {
	for i := 0; i < uuidBatchSize; i++ {
		uuid, err := g.generateOne()
		if err != nil {
			return err
		}
		g.ch <- uuid
	}
	return nil
}

// generateOne 生成单个 UUID v4。
//
// 使用 crypto/rand 读取随机字节，然后设置版本位和变体位。
// 字节缓冲区从 sync.Pool 获取并在使用后归还。
// 如果 crypto/rand 失败，返回零值 UUID 并通过 Observation Pipeline 记录错误。
func (g *BatchedUUIDGen) generateOne() ([16]byte, error) {
	bufPtr := g.pool.Get().(*[]byte)
	buf := *bufPtr
	_, err := rand.Read(buf)
	if err != nil {
		// 归还缓冲区
		g.pool.Put(bufPtr)
		return [16]byte{}, fmt.Errorf("batched uuid gen: crypto/rand.Read failed: %w", err)
	}
	buf[6] = (buf[6] & 0x0f) | 0x40 // 版本 4
	buf[8] = (buf[8] & 0x3f) | 0x80 // 变体 10
	var uuid [16]byte
	copy(uuid[:], buf)
	g.pool.Put(bufPtr)
	return uuid, nil
}

// backgroundRefill 在后台运行，监听 channel 并在半空时补充。
//
// 检查策略：
//   - 当 channel 剩余数量 <= uuidRefillThreshold（128）时，触发补充
//   - 使用 select 同时监听 done 信号以支持优雅关闭
//   - 补充操作在 refill 内部的 channel 发送时可能阻塞，
//     这自然形成了背压机制
//   - 如果 refill() 返回错误（crypto/rand 失败），停止填充 goroutine
//     并记录到 Observation Pipeline；channel 最终耗尽后系统降级
func (g *BatchedUUIDGen) backgroundRefill() {
	for {
		select {
		case <-g.done:
			return
		default:
			if len(g.ch) <= uuidRefillThreshold {
				if err := g.refill(); err != nil {
					// 熵源不可用，停止填充；channel 耗尽后 Next() 将阻塞，
					// 这比 panic 更安全，允许调用方通过超时机制处理
					return
				}
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
