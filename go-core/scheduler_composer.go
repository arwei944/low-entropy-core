package core

import (
	"context"
	"time"
)

// ──────────────────────────────────────────────
// SchedulerComposer — full scheduling orchestration
// ──────────────────────────────────────────────

// SchedulerComposer orchestrates the complete scheduling pipeline:
//   Dequeue → Match → Dispatch (via HandoffComposer)
//
// It composes AgentPool, TaskQueue, MatchEngine, and HandoffComposer
// into a unified scheduling workflow.
type SchedulerComposer struct {
	pool    *AgentPool
	queue   *TaskQueue
	handoff *HandoffComposer
	obs     ObservationAdapter
}

// NewSchedulerComposer creates a new scheduler composer.
func NewSchedulerComposer(pool *AgentPool, queue *TaskQueue, handoff *HandoffComposer, obs ObservationAdapter) *SchedulerComposer {
	if obs == nil {
		obs = &NoOpObservationAdapter{}
	}
	return &SchedulerComposer{
		pool:    pool,
		queue:   queue,
		handoff: handoff,
		obs:     obs,
	}
}

// ScheduleResult is the outcome of a scheduling operation.
type ScheduleResult struct {
	// TaskID is the task that was scheduled.
	TaskID string `json:"task_id"`

	// MatchedAgent is the agent the task was dispatched to.
	MatchedAgent string `json:"matched_agent,omitempty"`

	// Dispatched indicates whether the task was successfully dispatched.
	Dispatched bool `json:"dispatched"`

	// Requeued indicates whether the task was re-queued (no match found).
	Requeued bool `json:"requeued"`

	// Error contains any error that occurred.
	Error string `json:"error,omitempty"`
}

// ScheduleNext dequeues the next task and attempts to dispatch it.
// Returns the scheduling result and all execution steps.
//
// Flow:
//   1. Dequeue task from queue
//   2. Match task to agent (pure function)
//   3. If match found → dispatch via HandoffComposer
//   4. If no match → re-queue task with delay
func (s *SchedulerComposer) ScheduleNext(ctx context.Context, timeout time.Duration) (ScheduleResult, []ExecutionStep, error) {
	steps := make([]ExecutionStep, 0, 4)
	traceID := string(NewTraceID())
	parentSpanID := string(NewSpanID())

	// ─── Step 1: Dequeue ───
	step1 := NewExecutionStepWithTrace(parentSpanID, "Adapter", "Dequeue",
		"dequeuing task from queue", "Scheduler")
	step1.TraceID = traceID
	now := time.Now()

	task, err := s.queue.Dequeue(ctx, timeout)
	if err != nil {
		step1.Error = NewStepError("DEQUEUE_FAILED", err.Error(), true)
		step1.DurationMs = time.Since(now).Milliseconds()
		step1.Details = "failed to dequeue task"
		steps = append(steps, step1)
		s.obs.Record(steps)
		return ScheduleResult{Error: err.Error()}, steps, err
	}
	step1.DurationMs = time.Since(now).Milliseconds()
	step1.Details = "task dequeued: " + task.TaskID
	steps = append(steps, step1)

	// ─── Step 2: Match (pure function) ───
	step2 := NewExecutionStepWithTrace(parentSpanID, "Atom", "Match",
		"matching task to agent", "Scheduler")
	step2.TraceID = traceID
	now = time.Now()

	matchInput := MatchInput{Task: task, Pool: s.pool}
	matchOutput := MatchEngine(matchInput)
	step2.DurationMs = time.Since(now).Milliseconds()
	step2.Details = matchOutput.Reason

	if matchOutput.Matched == nil {
		step2.Details = "no match found: " + matchOutput.Reason
		steps = append(steps, step2)

		// Re-queue the task after a delay
		step3 := NewExecutionStepWithTrace(parentSpanID, "Adapter", "Requeue",
			"re-queuing task (no match)", "Scheduler")
		step3.TraceID = traceID
		now = time.Now()

		// Delay before re-queue
		select {
		case <-ctx.Done():
			step3.Error = NewStepError("CONTEXT_CANCELLED", ctx.Err().Error(), false)
			step3.DurationMs = time.Since(now).Milliseconds()
			steps = append(steps, step3)
			s.obs.Record(steps)
			return ScheduleResult{TaskID: task.TaskID, Requeued: false, Error: ctx.Err().Error()}, steps, ctx.Err()
		case <-time.After(5 * time.Second):
		}

		if err := s.queue.Enqueue(task); err != nil {
			step3.Error = NewStepError("REQUEUE_FAILED", err.Error(), false)
			step3.DurationMs = time.Since(now).Milliseconds()
			step3.Details = "failed to re-queue task"
			steps = append(steps, step3)
			s.obs.Record(steps)
			return ScheduleResult{TaskID: task.TaskID, Requeued: false, Error: err.Error()}, steps, err
		}

		step3.DurationMs = time.Since(now).Milliseconds()
		step3.Details = "task re-queued: " + task.TaskID
		steps = append(steps, step3)
		s.obs.Record(steps)

		return ScheduleResult{TaskID: task.TaskID, Requeued: true}, steps, nil
	}
	steps = append(steps, step2)

	// Mark the matched agent as busy
	_ = s.pool.UpdateStatus(matchOutput.Matched.ID, AgentStatusBusy)

	// ─── Step 3: Dispatch via HandoffComposer ───
	step3 := NewExecutionStepWithTrace(parentSpanID, "Composer", "Dispatch",
		"dispatching task to agent: "+matchOutput.Matched.ID, "Scheduler")
	step3.TraceID = traceID
	now = time.Now()

	// Build a snapshot for the dispatch
	snapshot := NewDevSnapshot(task.TaskID, "scheduler", task.Phase,
		"dispatched by scheduler to "+matchOutput.Matched.ID)
	_, _ = snapshot.ComputeChecksum()

	handoffInput := HandoffInput{
		SourceAgent:   snapshot,
		TargetAgentID: matchOutput.Matched.ID,
		TaskID:        task.TaskID,
		Phase:         task.Phase,
	}

	handoffOutput, handoffSteps, handoffErr := s.handoff.Execute(ctx, handoffInput)
	steps = append(steps, handoffSteps...)

	if handoffErr != nil || !handoffOutput.Success {
		step3.Error = NewStepError("DISPATCH_FAILED", "handoff dispatch failed", true)
		step3.DurationMs = time.Since(now).Milliseconds()
		step3.Details = "dispatch failed: " + handoffOutput.Error
		steps = append(steps, step3)
		s.obs.Record(steps)

		// Return agent to idle
		_ = s.pool.UpdateStatus(matchOutput.Matched.ID, AgentStatusIdle)

		return ScheduleResult{
			TaskID:       task.TaskID,
			MatchedAgent: matchOutput.Matched.ID,
			Error:        handoffOutput.Error,
		}, steps, handoffErr
	}

	step3.DurationMs = time.Since(now).Milliseconds()
	step3.Details = "task dispatched to agent: " + matchOutput.Matched.ID
	steps = append(steps, step3)
	s.obs.Record(steps)

	return ScheduleResult{
		TaskID:       task.TaskID,
		MatchedAgent: matchOutput.Matched.ID,
		Dispatched:   true,
	}, steps, nil
}

// ScheduleAll continuously schedules tasks until the queue is empty or context is cancelled.
func (s *SchedulerComposer) ScheduleAll(ctx context.Context, timeout time.Duration) ([]ScheduleResult, []ExecutionStep, error) {
	allResults := make([]ScheduleResult, 0)
	allSteps := make([]ExecutionStep, 0)

	for {
		select {
		case <-ctx.Done():
			return allResults, allSteps, ctx.Err()
		default:
		}

		result, steps, err := s.ScheduleNext(ctx, timeout)
		allSteps = append(allSteps, steps...)
		allResults = append(allResults, result)

		if err != nil {
			// If queue is empty or closed, stop
			if se, ok := err.(*StepError); ok && (se.Code == "QUEUE_EMPTY" || se.Code == "QUEUE_CLOSED" || se.Code == "DEQUEUE_TIMEOUT") {
				break
			}
			return allResults, allSteps, err
		}
	}

	return allResults, allSteps, nil
}