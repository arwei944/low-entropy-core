//go:build lecore_tier3 || lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

package core

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestTaskQueue_EnqueueDequeue(t *testing.T) {
	ctx := context.Background()
	queue := NewTaskQueue()

	task := &QueuedTask{TaskID: "task-1", Priority: 5, Phase: "coding"}
	queue.Enqueue(task)

	if queue.Len() != 1 {
		t.Errorf("expected len=1, got %d", queue.Len())
	}

	dequeued, err := queue.Dequeue(ctx, 0)
	if err != nil {
		t.Fatalf("dequeue failed: %v", err)
	}
	if dequeued.TaskID != "task-1" {
		t.Errorf("expected task-1, got %s", dequeued.TaskID)
	}
	if queue.Len() != 0 {
		t.Errorf("expected len=0, got %d", queue.Len())
	}
}

func TestTaskQueue_PriorityOrder(t *testing.T) {
	ctx := context.Background()
	queue := NewTaskQueue()

	queue.Enqueue(&QueuedTask{TaskID: "low", Priority: 1, Phase: "coding"})
	queue.Enqueue(&QueuedTask{TaskID: "high", Priority: 10, Phase: "coding"})
	queue.Enqueue(&QueuedTask{TaskID: "mid", Priority: 5, Phase: "coding"})

	task1, _ := queue.Dequeue(ctx, 0)
	task2, _ := queue.Dequeue(ctx, 0)
	task3, _ := queue.Dequeue(ctx, 0)

	if task1.TaskID != "high" {
		t.Errorf("expected 'high' first, got '%s'", task1.TaskID)
	}
	if task2.TaskID != "mid" {
		t.Errorf("expected 'mid' second, got '%s'", task2.TaskID)
	}
	if task3.TaskID != "low" {
		t.Errorf("expected 'low' third, got '%s'", task3.TaskID)
	}
}

func TestTaskQueue_EqualPriority_FIFO(t *testing.T) {
	ctx := context.Background()
	queue := NewTaskQueue()

	queue.Enqueue(&QueuedTask{TaskID: "first", Priority: 5, Phase: "coding"})
	time.Sleep(10 * time.Millisecond)
	queue.Enqueue(&QueuedTask{TaskID: "second", Priority: 5, Phase: "coding"})

	task1, _ := queue.Dequeue(ctx, 0)
	task2, _ := queue.Dequeue(ctx, 0)

	if task1.TaskID != "first" {
		t.Errorf("expected 'first', got '%s'", task1.TaskID)
	}
	if task2.TaskID != "second" {
		t.Errorf("expected 'second', got '%s'", task2.TaskID)
	}
}

func TestTaskQueue_DequeueEmpty(t *testing.T) {
	ctx := context.Background()
	queue := NewTaskQueue()

	_, err := queue.Dequeue(ctx, 0)
	if err == nil {
		t.Fatal("expected error for empty queue")
	}
}

func TestTaskQueue_DequeueTimeout(t *testing.T) {
	ctx := context.Background()
	queue := NewTaskQueue()

	start := time.Now()
	_, err := queue.Dequeue(ctx, 50*time.Millisecond)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected timeout error")
	}
	if elapsed < 50*time.Millisecond {
		t.Errorf("expected at least 50ms wait, got %v", elapsed)
	}
}

func TestTaskQueue_Peek(t *testing.T) {
	queue := NewTaskQueue()
	queue.Enqueue(&QueuedTask{TaskID: "task-1", Priority: 5, Phase: "coding"})

	task, ok := queue.Peek()
	if !ok {
		t.Fatal("expected task from peek")
	}
	if task.TaskID != "task-1" {
		t.Errorf("expected task-1, got %s", task.TaskID)
	}
	if queue.Len() != 1 {
		t.Errorf("peek should not remove: expected len=1, got %d", queue.Len())
	}
}

func TestTaskQueue_PeekEmpty(t *testing.T) {
	queue := NewTaskQueue()
	_, ok := queue.Peek()
	if ok {
		t.Error("expected false from empty queue peek")
	}
}

func TestTaskQueue_Close(t *testing.T) {
	ctx := context.Background()
	queue := NewTaskQueue()

	queue.Close()

	err := queue.Enqueue(&QueuedTask{TaskID: "t", Priority: 1})
	if err == nil {
		t.Fatal("expected error when enqueuing to closed queue")
	}

	_, err = queue.Dequeue(ctx, 0)
	if err == nil {
		t.Fatal("expected error when dequeuing from closed queue")
	}
}

func TestTaskQueue_Concurrency(t *testing.T) {
	ctx := context.Background()
	queue := NewTaskQueue()
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			queue.Enqueue(&QueuedTask{TaskID: fmt.Sprintf("task-%d", id), Priority: id, Phase: "coding"})
		}(i)
	}
	wg.Wait()

	if queue.Len() != 100 {
		t.Errorf("expected 100 tasks, got %d", queue.Len())
	}

	for i := 0; i < 100; i++ {
		_, err := queue.Dequeue(ctx, 0)
		if err != nil {
			t.Fatalf("dequeue %d failed: %v", i, err)
		}
	}
}
