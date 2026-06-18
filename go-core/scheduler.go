//go:build lecore_tier3 || lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

package core

import (
	"container/heap"
	"context"
	"sync"
	"time"
)

// ──────────────────────────────────────────────
// Scheduler — task scheduling subsystem
// 合并自: scheduler_agent_pool.go + scheduler_queue.go +
//         scheduler_composer.go + scheduler_match.go
// ──────────────────────────────────────────────

// ============================================================================
// SECTION 1: AgentPool
// ============================================================================

type AgentStatus string

const (
	AgentStatusIdle    AgentStatus = "idle"
	AgentStatusBusy    AgentStatus = "busy"
	AgentStatusOffline AgentStatus = "offline"
)

const AgentHeartbeatTimeout = 30 * time.Second

type AgentInfo struct {
	ID            string      `json:"id"`
	Capabilities  []string    `json:"capabilities"`
	Status        AgentStatus `json:"status"`
	LastHeartbeat time.Time   `json:"last_heartbeat"`
	Phase         string      `json:"phase"`
}

type AgentPool struct {
	mu     sync.RWMutex
	agents map[string]*AgentInfo
}

func NewAgentPool() *AgentPool { return &AgentPool{agents: make(map[string]*AgentInfo)} }

func (p *AgentPool) Add(agent *AgentInfo) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, exists := p.agents[agent.ID]; exists {
		return NewStepError("AGENT_EXISTS", "agent already registered: "+agent.ID, false)
	}
	if agent.LastHeartbeat.IsZero() {
		agent.LastHeartbeat = time.Now()
	}
	if agent.Status == "" {
		agent.Status = AgentStatusIdle
	}
	p.agents[agent.ID] = agent
	return nil
}

func (p *AgentPool) Remove(agentID string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.agents, agentID)
}

func (p *AgentPool) UpdateStatus(agentID string, status AgentStatus) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	agent, ok := p.agents[agentID]
	if !ok {
		return NewStepError("AGENT_NOT_FOUND", "agent not found: "+agentID, false)
	}
	agent.Status = status
	agent.LastHeartbeat = time.Now()
	return nil
}

func (p *AgentPool) Heartbeat(agentID string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	agent, ok := p.agents[agentID]
	if !ok {
		return NewStepError("AGENT_NOT_FOUND", "agent not found: "+agentID, false)
	}
	agent.LastHeartbeat = time.Now()
	if agent.Status == AgentStatusOffline {
		agent.Status = AgentStatusIdle
	}
	return nil
}

func (p *AgentPool) Get(agentID string) (*AgentInfo, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	agent, ok := p.agents[agentID]
	return agent, ok
}

func (p *AgentPool) ListAvailable() []*AgentInfo {
	p.mu.Lock()
	defer p.mu.Unlock()
	now := time.Now()
	available := make([]*AgentInfo, 0)
	for _, agent := range p.agents {
		if now.Sub(agent.LastHeartbeat) > AgentHeartbeatTimeout {
			agent.Status = AgentStatusOffline
		}
		if agent.Status == AgentStatusIdle {
			available = append(available, agent)
		}
	}
	return available
}

func (p *AgentPool) ListByCapability(capability string) []*AgentInfo {
	available := p.ListAvailable()
	result := make([]*AgentInfo, 0)
	for _, agent := range available {
		for _, cap := range agent.Capabilities {
			if cap == capability {
				result = append(result, agent)
				break
			}
		}
	}
	return result
}

func (p *AgentPool) ListByPhase(phase string) []*AgentInfo {
	available := p.ListAvailable()
	result := make([]*AgentInfo, 0)
	for _, agent := range available {
		if agent.Phase == phase {
			result = append(result, agent)
		}
	}
	return result
}

func (p *AgentPool) Count() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.agents)
}

type AgentPoolAdapter struct{ pool *AgentPool }

func NewAgentPoolAdapter(pool *AgentPool) *AgentPoolAdapter { return &AgentPoolAdapter{pool: pool} }

type AgentPoolOp struct {
	Op      string      `json:"op"`
	AgentID string      `json:"agent_id,omitempty"`
	Agent   *AgentInfo  `json:"agent,omitempty"`
	Status  AgentStatus `json:"status,omitempty"`
	Error   error       `json:"-"`
}

func (a *AgentPoolAdapter) Execute(ctx context.Context, input AgentPoolOp) (AgentPoolOp, error) {
	switch input.Op {
	case "add":
		input.Error = a.pool.Add(input.Agent)
	case "remove":
		a.pool.Remove(input.AgentID)
	case "heartbeat":
		input.Error = a.pool.Heartbeat(input.AgentID)
	case "update_status":
		input.Error = a.pool.UpdateStatus(input.AgentID, input.Status)
	}
	return input, nil
}

// ============================================================================
// SECTION 2: TaskQueue — priority queue
// ============================================================================

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

// ============================================================================
// SECTION 3: MatchEngine — pure function agent matching
// ============================================================================

type MatchInput struct {
	Task *QueuedTask
	Pool *AgentPool
}

type MatchOutput struct {
	Matched    *AgentInfo
	Candidates []*AgentInfo
	Reason     string
}

func MatchEngine(input MatchInput) MatchOutput {
	if input.Pool == nil || input.Task == nil {
		return MatchOutput{Reason: "invalid input: pool or task is nil"}
	}
	phaseCandidates := input.Pool.ListByPhase(input.Task.Phase)
	if len(phaseCandidates) == 0 {
		return MatchOutput{Reason: "no agents available for phase: " + input.Task.Phase}
	}
	capabilityCandidates := make([]*AgentInfo, 0)
	for _, agent := range phaseCandidates {
		if hasAllCapabilities(agent.Capabilities, input.Task.RequiredCapabilities) {
			capabilityCandidates = append(capabilityCandidates, agent)
		}
	}
	if len(capabilityCandidates) == 0 {
		return MatchOutput{Candidates: phaseCandidates, Reason: "no agents with required capabilities"}
	}
	best := findLongestIdle(capabilityCandidates)
	return MatchOutput{
		Matched: best, Candidates: capabilityCandidates,
		Reason: "matched agent " + best.ID + " (idle since " + best.LastHeartbeat.Format(time.RFC3339) + ")",
	}
}

func hasAllCapabilities(agentCaps, requiredCaps []string) bool {
	if len(requiredCaps) == 0 {
		return true
	}
	capSet := make(map[string]bool, len(agentCaps))
	for _, c := range agentCaps {
		capSet[c] = true
	}
	for _, required := range requiredCaps {
		if !capSet[required] {
			return false
		}
	}
	return true
}

func findLongestIdle(agents []*AgentInfo) *AgentInfo {
	if len(agents) == 0 {
		return nil
	}
	best := agents[0]
	for _, a := range agents[1:] {
		if a.LastHeartbeat.Before(best.LastHeartbeat) {
			best = a
		}
	}
	return best
}

func MatchEngineAsStep() Step[MatchInput, MatchOutput] {
	return AtomAsStep(Atom[MatchInput, MatchOutput](MatchEngine))
}

// ============================================================================
// SECTION 4: SchedulerComposer
// ============================================================================

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