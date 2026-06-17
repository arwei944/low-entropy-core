package main

import (
	"context"
	"fmt"
	"time"

	core "low-entropy-core/go-core"
)

// Task represents a scheduled task.
type Task struct {
	ID     string
	Status string
	Data   map[string]interface{}
}

// ─── Atom: pure state transition ───

func transitionState() core.Atom[Task, Task] {
	return func(t Task) Task {
		if t.Status == "pending" {
			t.Status = "running"
		}
		return t
	}
}

// ─── Port: validation gateway ───

type TaskPort struct{}

func (p *TaskPort) Validate(ctx context.Context, input Task) (Task, error) {
	if input.ID == "" {
		return input, &core.StepError{Code: "INVALID_TASK", Message: "task ID is required", Recoverable: false}
	}
	return input, nil
}

// ─── Adapter: persistence side effect ───

type PersistenceAdapter struct{}

func (a *PersistenceAdapter) Execute(ctx context.Context, input Task) (Task, error) {
	fmt.Printf("[Adapter] Persisted task %s status: %s\n", input.ID, input.Status)
	return input, nil
}

func main() {
	fmt.Println("=== Task Scheduler v2.0 — 4 Primitives + Patterns + Handoff + Observation ===")
	ctx := context.Background()

	// ─── Build Steps ───

	validate := core.PortAsStep(&TaskPort{})
	transition := core.AtomAsStep(transitionState())
	persist := core.AdapterAsStep(&PersistenceAdapter{})

	// ─── Base Pipeline ───
	obs := &core.InMemoryObservationAdapter{}
	base := core.NewPipeline[Task](obs, validate, transition, persist)

	// ─── Branch Pattern ───
	branchStep := core.NewBranch[Task](
		func(t Task) bool { return t.Status == "pending" },
		base, // truePath: process the task
		core.NewPipeline[Task](obs, core.AtomAsStep(core.Atom[Task, Task](func(t Task) Task {
			fmt.Printf("[Branch] Skipping task %s (status=%s)\n", t.ID, t.Status)
			return t
		}))), // falsePath: skip
	)
	branchPipeline := core.NewPipeline[Task](obs, branchStep)

	// ─── WithRetry Pattern ───
	retryConfig := core.RetryConfig{
		MaxAttempts: 3,
		BaseDelay:   50 * time.Millisecond,
		MaxDelay:    1 * time.Second,
		Multiplier:  2.0,
	}
	retryComp := core.WithRetry[Task](branchPipeline, retryConfig)

	// ─── WithTimeout Pattern ───
	timeoutComp := core.WithTimeout[Task](retryComp, 5*time.Second)

	// ─── Handoff: Scheduler → Worker ───
	scheduler := core.NewPipeline[any](obs,
		core.AtomAsStep(core.Atom[any, any](func(i any) any {
			fmt.Println("[Composer] Scheduler preparing handoff")
			return i
		})),
	)

	worker := core.NewPipeline[any](obs,
		core.AtomAsStep(core.Atom[any, any](func(i any) any {
			fmt.Println("[Composer] Worker processing after handoff")
			return i
		})),
	)

	snap := &core.DefaultSnapshotAdapter{}
	handoff := core.NewHandoff(scheduler, worker, snap, core.InProcTransport)

	// ─── Execute ───

	// 1. Run a task through the pipeline
	task := Task{ID: "task-042", Status: "pending", Data: map[string]interface{}{}}
	result, steps, err := timeoutComp.Run(ctx, task)
	if err != nil {
		fmt.Printf("Pipeline error: %v\n", err)
	}
	fmt.Printf("Result: %+v\n", result)
	fmt.Printf("Steps recorded: %d\n", len(steps))

	// 2. Run a skipped task (Branch false path)
	task2 := Task{ID: "task-043", Status: "completed", Data: map[string]interface{}{}}
	result2, steps2, err2 := timeoutComp.Run(ctx, task2)
	if err2 != nil {
		fmt.Printf("Pipeline error: %v\n", err2)
	}
	fmt.Printf("Skipped result: %+v\n", result2)
	fmt.Printf("Steps recorded: %d\n", len(steps2))

	// 3. Handoff demo
	handoffResult, handoffSteps, handoffErr := handoff.Run(ctx, core.HandoffRequest{
		SourceID: "scheduler",
		TargetID: "worker",
		TaskType: "task",
		Payload:  task,
		Token:    "handoff-001",
	})
	if handoffErr != nil {
		fmt.Printf("Handoff error: %v\n", handoffErr)
	}
	fmt.Printf("Handoff result: %+v\n", handoffResult)
	fmt.Printf("Handoff steps: %d\n", len(handoffSteps))

	// ─── Observation: Trace Tree ───
	fmt.Println("\n=== Observation X-Ray ===")
	fmt.Printf("Total steps in observation store: %d\n", obs.StepCount())

	tree := obs.GetTraceTree()
	fmt.Printf("Trace tree roots: %d, total nodes: %d\n", len(tree.Roots), tree.TotalNodes())

	// Print all steps
	for i, s := range obs.GetSteps() {
		fmt.Printf("  [%d] %s/%s | %s | %dms | trace=%s\n",
			i, s.Unit, s.Action, s.Details, s.DurationMs, s.TraceID[:8])
	}
}