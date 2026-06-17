package core

import (
	"container/heap"
	"context"
	"sync"
	"time"
)

// ──────────────────────────────────────────────
// TaskQueue — priority queue for task scheduling
// ──────────────────────────────────────────────

// QueuedTask represents a task waiting in the scheduling queue.
type QueuedTask struct {
	// TaskID uniquely identifies the task.
	TaskID string `json:"task_id"`

	// SnapshotChecksum is the checksum of the task's snapshot.
	SnapshotChecksum string `json:"snapshot_checksum"`

	// Priority is the task priority (higher = more urgent).
	Priority int `json:"priority"`

	// Phase is the development phase this task belongs to.
	Phase string `json:"phase"`

	// RequiredCapabilities are the capabilities the target agent must have.
	RequiredCapabilities []string `json:"required_capabilities"`

	// CreatedAt is when the task was enqueued.
	CreatedAt time.Time `json:"created_at"`

	// index is used by the heap implementation.
	index int
}

// ──────────────────────────────────────────────
// Priority Queue (heap.Interface implementation)
// ──────────────────────────────────────────────

// taskHeap implements heap.Interface for QueuedTask priority ordering.
// Higher priority tasks are dequeued first.
// For equal priority, older tasks are dequeued first.
type taskHeap []*QueuedTask

func (h taskHeap) Len() int { return len(h) }

func (h taskHeap) Less(i, j int) bool {
	// Higher priority first
	if h[i].Priority != h[j].Priority {
		return h[i].Priority > h[j].Priority
	}
	// Older tasks first for equal priority
	return h[i].CreatedAt.Before(h[j].CreatedAt)
}

func (h taskHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].index = i
	h[j].index = j
}

func (h *taskHeap) Push(x interface{}) {
	n := len(*h)
	item := x.(*QueuedTask)
	item.index = n
	*h = append(*h, item)
}

func (h *taskHeap) Pop() interface{} {
	old := *h
	n := len(old)
	item := old[n-1]
	old[n-1] = nil
	item.index = -1
	*h = old[0 : n-1]
	return item
}

// ──────────────────────────────────────────────
// TaskQueue
// ──────────────────────────────────────────────

// TaskQueue is a priority queue for tasks waiting to be scheduled.
// Thread-safe for concurrent use. Supports blocking dequeue with timeout.
type TaskQueue struct {
	mu    sync.Mutex
	heap  taskHeap
	cond  *sync.Cond
	closed bool
}

// NewTaskQueue creates a new task queue.
func NewTaskQueue() *TaskQueue {
	q := &TaskQueue{
		heap: make(taskHeap, 0),
	}
	q.cond = sync.NewCond(&q.mu)
	return q
}

// Enqueue adds a task to the queue.
// Returns an error if the queue is closed.
func (q *TaskQueue) Enqueue(task *QueuedTask) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.closed {
		return NewStepError("QUEUE_CLOSED", "task queue is closed", false)
	}

	task.CreatedAt = time.Now()
	heap.Push(&q.heap, task)
	q.cond.Signal() // Wake up one waiting dequeuer
	return nil
}

// Dequeue removes and returns the highest priority task.
// If the queue is empty and timeout > 0, it blocks until a task is available
// or the timeout expires, polling at 10ms intervals.
// If timeout is 0, it returns immediately.
func (q *TaskQueue) Dequeue(ctx context.Context, timeout time.Duration) (*QueuedTask, error) {
	deadline := time.Now().Add(timeout)

	for {
		q.mu.Lock()
		// Check if we have a task
		if q.heap.Len() > 0 {
			task := heap.Pop(&q.heap).(*QueuedTask)
			q.mu.Unlock()
			return task, nil
		}
		// Check if closed
		if q.closed {
			q.mu.Unlock()
			return nil, NewStepError("QUEUE_CLOSED", "task queue is closed", false)
		}
		q.mu.Unlock()

		// No task available
		if timeout == 0 {
			return nil, NewStepError("QUEUE_EMPTY", "task queue is empty", true)
		}

		remaining := time.Until(deadline)
		if remaining <= 0 {
			return nil, NewStepError("DEQUEUE_TIMEOUT", "dequeue timed out", true)
		}

		// Poll interval: 10ms or remaining time, whichever is smaller
		pollInterval := 10 * time.Millisecond
		if remaining < pollInterval {
			pollInterval = remaining
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(pollInterval):
			// Continue loop to check again
		}
	}
}

// Peek returns the highest priority task without removing it.
func (q *TaskQueue) Peek() (*QueuedTask, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.heap.Len() == 0 {
		return nil, false
	}
	return q.heap[0], true
}

// Len returns the number of tasks in the queue.
func (q *TaskQueue) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.heap.Len()
}

// Close closes the queue and wakes up all waiting dequeuers.
func (q *TaskQueue) Close() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.closed = true
	q.cond.Broadcast()
}

// IsClosed returns whether the queue is closed.
func (q *TaskQueue) IsClosed() bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.closed
}