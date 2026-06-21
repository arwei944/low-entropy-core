//go:build lecore_tier3 || lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

package core

import (
	"container/heap"
	"context"
	"sync"
	"time"
)

type QueuedTask struct {
	TaskID               string    `json:"task_id"`
	SnapshotChecksum     string    `json:"snapshot_checksum"`
	Priority             int       `json:"priority"`
	Phase                string    `json:"phase"`
	RequiredCapabilities []string  `json:"required_capabilities"`
	CreatedAt            time.Time `json:"created_at"`
	index                int
}

type taskHeap []*QueuedTask

func (h taskHeap) Len() int { return len(h) }
func (h taskHeap) Less(i, j int) bool {
	if h[i].Priority != h[j].Priority {
		return h[i].Priority > h[j].Priority
	}
	return h[i].CreatedAt.Before(h[j].CreatedAt)
}
func (h taskHeap) Swap(i, j int) { h[i], h[j] = h[j], h[i]; h[i].index = i; h[j].index = j }
func (h *taskHeap) Push(x any) {
	n := len(*h)
	item := x.(*QueuedTask)
	item.index = n
	*h = append(*h, item)
}
func (h *taskHeap) Pop() any {
	old := *h
	n := len(old)
	item := old[n-1]
	old[n-1] = nil
	item.index = -1
	*h = old[0 : n-1]
	return item
}

type TaskQueue struct {
	mu     sync.Mutex
	heap   taskHeap
	cond   *sync.Cond
	closed bool
}

func NewTaskQueue() *TaskQueue {
	q := &TaskQueue{heap: make(taskHeap, 0)}
	q.cond = sync.NewCond(&q.mu)
	return q
}

func (q *TaskQueue) Enqueue(task *QueuedTask) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed {
		return NewStepError("QUEUE_CLOSED", "task queue is closed", false)
	}
	task.CreatedAt = time.Now()
	heap.Push(&q.heap, task)
	q.cond.Signal()
	return nil
}

func (q *TaskQueue) Dequeue(ctx context.Context, timeout time.Duration) (*QueuedTask, error) {
	deadline := time.Now().Add(timeout)
	for {
		q.mu.Lock()
		if q.heap.Len() > 0 {
			task := heap.Pop(&q.heap).(*QueuedTask)
			q.mu.Unlock()
			return task, nil
		}
		if q.closed {
			q.mu.Unlock()
			return nil, NewStepError("QUEUE_CLOSED", "task queue is closed", false)
		}
		q.mu.Unlock()
		if timeout == 0 {
			return nil, NewStepError("QUEUE_EMPTY", "task queue is empty", true)
		}
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return nil, NewStepError("DEQUEUE_TIMEOUT", "dequeue timed out", true)
		}
		pollInterval := 10 * time.Millisecond
		if remaining < pollInterval {
			pollInterval = remaining
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(pollInterval):
		}
	}
}

func (q *TaskQueue) Peek() (*QueuedTask, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.heap.Len() == 0 {
		return nil, false
	}
	return q.heap[0], true
}

func (q *TaskQueue) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.heap.Len()
}

func (q *TaskQueue) Close() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.closed = true
	q.cond.Broadcast()
}

func (q *TaskQueue) IsClosed() bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.closed
}
