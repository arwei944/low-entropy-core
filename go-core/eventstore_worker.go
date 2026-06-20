//go:build lecore_tier5 || lecore_tier6 || lecore_tier7

package core

import (
	"sync"
)

// =============================================================================
// EventBusWorkerPool — EventBus 优化 (T6.4)
// =============================================================================

// EventBusWorkerPool 替代 EventBus 中每个事件一个 goroutine 的模式。
// 使用固定大小的 worker pool 处理异步订阅，减少 goroutine 创建开销。
type EventBusWorkerPool struct {
	taskCh   chan func()
	workers  int
	stopCh   chan struct{}
	wg       sync.WaitGroup
}

// NewEventBusWorkerPool 创建 worker pool。
// workers: goroutine 数量（默认 10）。
func NewEventBusWorkerPool(workers int) *EventBusWorkerPool {
	if workers <= 0 {
		workers = 10
	}
	pool := &EventBusWorkerPool{
		taskCh:  make(chan func(), 1000),
		workers: workers,
		stopCh:  make(chan struct{}),
	}
	for i := 0; i < workers; i++ {
		pool.wg.Add(1)
		go pool.worker()
	}
	return pool
}

// worker 是 worker pool 中的 goroutine。
func (p *EventBusWorkerPool) worker() {
	defer p.wg.Done()
	for {
		select {
		case task := <-p.taskCh:
			defer func() {
				if r := recover(); r != nil {
					// 忽略 panic，继续处理后续任务
				}
			}()
			task()
		case <-p.stopCh:
			return
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
	close(p.stopCh)
	p.wg.Wait()
}
