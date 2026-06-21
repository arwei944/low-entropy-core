//go:build lecore_tier3 || lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

package core

import (
	"context"
	"time"
)

type SchedulerComposer struct {
	pool    *AgentPool
	queue   *TaskQueue
	handoff *HandoffComposer
	obs     ObservationAdapter
}

type ScheduleResult struct {
	TaskID       string `json:"task_id"`
	MatchedAgent string `json:"matched_agent,omitempty"`
	Dispatched   bool   `json:"dispatched"`
	Requeued     bool   `json:"requeued"`
	Error        string `json:"error,omitempty"`
}

func NewSchedulerComposer(pool *AgentPool, queue *TaskQueue, handoff *HandoffComposer, obs ObservationAdapter) *SchedulerComposer {
	if obs == nil {
		obs = &NoOpObservationAdapter{}
	}
	return &SchedulerComposer{pool: pool, queue: queue, handoff: handoff, obs: obs}
}

func (s *SchedulerComposer) ScheduleNext(ctx context.Context, timeout time.Duration) (ScheduleResult, []ExecutionStep, error) {
	steps := make([]ExecutionStep, 0, 4)
	traceID := string(NewTraceID())
	parentSpanID := string(NewSpanID())

	step1 := NewExecutionStepWithTrace(parentSpanID, "Adapter", "Dequeue", "dequeuing task", "Scheduler")
	step1.TraceID = traceID
	now := time.Now()
	task, err := s.queue.Dequeue(ctx, timeout)
	if err != nil {
		step1.Error = NewStepError("DEQUEUE_FAILED", err.Error(), true)
		step1.DurationMs = time.Since(now).Milliseconds()
		steps = append(steps, step1)
		s.obs.Record(steps)
		return ScheduleResult{Error: err.Error()}, steps, err
	}
	step1.DurationMs = time.Since(now).Milliseconds()
	steps = append(steps, step1)

	step2 := NewExecutionStepWithTrace(parentSpanID, "Atom", "Match", "matching task to agent", "Scheduler")
	step2.TraceID = traceID
	now = time.Now()
	matchOutput := MatchEngine(MatchInput{Task: task, Pool: s.pool})
	step2.DurationMs = time.Since(now).Milliseconds()
	step2.Details = matchOutput.Reason
	if matchOutput.Matched == nil {
		steps = append(steps, step2)
		step3 := NewExecutionStepWithTrace(parentSpanID, "Adapter", "Requeue", "re-queuing task (no match)", "Scheduler")
		step3.TraceID = traceID
		now = time.Now()
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
			steps = append(steps, step3)
			s.obs.Record(steps)
			return ScheduleResult{TaskID: task.TaskID, Requeued: false, Error: err.Error()}, steps, err
		}
		step3.DurationMs = time.Since(now).Milliseconds()
		steps = append(steps, step3)
		s.obs.Record(steps)
		return ScheduleResult{TaskID: task.TaskID, Requeued: true}, steps, nil
	}
	steps = append(steps, step2)
	_ = s.pool.UpdateStatus(matchOutput.Matched.ID, AgentStatusBusy)

	step3 := NewExecutionStepWithTrace(parentSpanID, "Composer", "Dispatch", "dispatching to agent: "+matchOutput.Matched.ID, "Scheduler")
	step3.TraceID = traceID
	now = time.Now()
	snapshot := NewDevSnapshot(task.TaskID, "scheduler", task.Phase, "dispatched by scheduler")
	_, _ = snapshot.ComputeChecksum()
	handoffOutput, handoffSteps, handoffErr := s.handoff.Execute(ctx, HandoffInput{
		SourceAgent: snapshot, TargetAgentID: matchOutput.Matched.ID,
		TaskID: task.TaskID, Phase: task.Phase,
	})
	steps = append(steps, handoffSteps...)
	if handoffErr != nil || !handoffOutput.Success {
		step3.Error = NewStepError("DISPATCH_FAILED", "handoff dispatch failed", true)
		step3.DurationMs = time.Since(now).Milliseconds()
		steps = append(steps, step3)
		s.obs.Record(steps)
		_ = s.pool.UpdateStatus(matchOutput.Matched.ID, AgentStatusIdle)
		return ScheduleResult{TaskID: task.TaskID, MatchedAgent: matchOutput.Matched.ID, Error: handoffOutput.Error}, steps, handoffErr
	}
	step3.DurationMs = time.Since(now).Milliseconds()
	steps = append(steps, step3)
	s.obs.Record(steps)
	return ScheduleResult{TaskID: task.TaskID, MatchedAgent: matchOutput.Matched.ID, Dispatched: true}, steps, nil
}

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
			if se, ok := err.(*StepError); ok && (se.Code == "QUEUE_EMPTY" || se.Code == "QUEUE_CLOSED" || se.Code == "DEQUEUE_TIMEOUT") {
				break
			}
			return allResults, allSteps, err
		}
	}
	return allResults, allSteps, nil
}
