// {{.Project}} — Adapter Implementations (Side-Effect Boundaries)
// Adapters are the ONLY place where side effects are allowed (I/O, DB, external APIs).

package main

import (
	"context"
	"fmt"
)

// LogAdapter logs pipeline results.
type LogAdapter struct{}

func (a *LogAdapter) Execute(ctx context.Context, resp Response) (Response, error) {
	fmt.Printf("[LogAdapter] id=%s success=%v\n", resp.ID, resp.Success)
	return resp, nil
}

// PersistAdapter persists results (placeholder — connect to real storage).
type PersistAdapter struct{}

func (a *PersistAdapter) Execute(ctx context.Context, resp Response) (Response, error) {
	// TODO: Replace with actual persistence (PostgreSQL, Redis, etc.)
	// Example: core.GetStorage().Set(ctx, "result:"+resp.ID, resp)
	fmt.Printf("[PersistAdapter] id=%s persisted\n", resp.ID)
	return resp, nil
}

// NotifyAdapter sends notifications (placeholder).
type NotifyAdapter struct{}

func (a *NotifyAdapter) Execute(ctx context.Context, resp Response) (Response, error) {
	// TODO: Replace with actual notification (email, webhook, etc.)
	fmt.Printf("[NotifyAdapter] id=%s notification sent\n", resp.ID)
	return resp, nil
}
