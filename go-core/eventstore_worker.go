//go:build lecore_tier5 || lecore_tier6 || lecore_tier7

package core

import (
	"context"
	"sync"
)

// =============================================================================
// EventBusWorkerPool — EventBus 优化 (T6.4)
// =============================================================================

// EventBusWorkerPool 替代 EventBus 中每个事件一个 goroutine 的模式。
// 使用固定大小的 worker pool 处理异步订阅，减少 goroutine 创建开销。
type EventBusWorkerPool struct {
	taskCh  chan func()
	workers int
	ctx     context.Context
	cancel  context.CancelFunc
	stopCh  chan struct{}
	wg      sync.WaitGroup
}

// NewEventBusWorkerPool 创建 worker pool。
// workers: goroutine 数量（默认 10）。
// ctx: 外部 context，cancel 时所有 worker 退出。
func NewEventBusWorkerPool(ctx context.Context, workers int) *EventBusWorkerPool {
	if workers <= 0 {
		workers = 10
	}
	ctx, cancel := context.WithCancel(ctx)
	pool := &EventBusWorkerPool{
		taskCh:  make(chan func(), 1000),
		workers: workers,
		ctx:     ctx,
		cancel:  cancel,
		stopCh:  make(chan struct{}),
	}
	for i := 0; i < workers; i++ {
		pool.wg.Add(1)
		go pool.worker()
	}
	return pool
}

// worker 是 worker pool 中的 goroutine，通过 ctx.Done() 管理生命周期。
func (p *EventBusWorkerPool) worker() {
	defer p.wg.Done()
	for {
		select {
		case <-p.ctx.Done():
			return
		case <-p.stopCh:
			return
		case task := <-p.taskCh:
			defer func() {
				if r := recover(); r != nil {
					// 忽略 panic，继续处理后续任务
				}
			}()
			task()
		}
	}
}

// Submit 提交任务到 worker pool。
// 非阻塞提交：如果 channel 满了，在调用 goroutine 中同步执行。
func (p *EventBusWorkerPool) Submit(task func()) {
	select {
	case p.taskCh <- task:
	default:
		// 回退到同步执行
		task()
	}
}

// Stop 停止 worker pool。
func (p *EventBusWorkerPool) Stop() {
	if p.cancel != nil {
		p.cancel()
	}
	close(p.stopCh)
	p.wg.Wait()
}
