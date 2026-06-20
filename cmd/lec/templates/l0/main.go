// {{.Project}} — Tier L0 Prototype
// Build: go run main.go
// Tier: L0 (Prototype) — <10 files, scripts, PoC
//
// Low-Entropy Core: 4-primitive architecture (Atom/Port/Adapter/Composer)
// All business logic MUST use one of the 4 primitives. No raw func allowed.

package main

import (
	"context"
	"fmt"

	core "{{.CoreModule}}"
)

// ─── Business State ───
// Define your business state type here. This flows through the entire pipeline.
type State struct {
	Input  string
	Output string
	OK     bool
	Err    string
}

func main() {
	fmt.Println("=== {{.Project}} (Tier L0 Prototype) ===")
	fmt.Println()

	// Auto-detect tier
	tier := core.AutoDetect(".")
	fmt.Printf("Detected tier: %s (L%d)\n", tier, tier)

	// Create observation adapter (NoOp for zero-overhead in prototype)
	obs := &core.NoOpObservationAdapter{}

	// ─── Build Pipeline ───
	// Chain the 4 primitives: Port → Atom → Adapter
	pipeline := core.NewPipeline[State](obs,

		// Port: Input validation (boundary contract)
		core.PortAsStep(core.NewPort[State, State](func(ctx context.Context, input State) (State, error) {
			if input.Input == "" {
				return input, fmt.Errorf("input cannot be empty")
			}
			input.OK = true
			return input, nil
		})),

		// Atom: Pure computation (no side effects)
		core.AtomAsStep(core.Atom[State, State](func(s State) State {
			// TODO: Replace with your business logic
			s.Output = "processed: " + s.Input
			return s
		})),

		// Adapter: Side-effect boundary (logging, persistence, external calls)
		core.AdapterAsStep(core.NewAdapter[State, State](func(ctx context.Context, input State) (State, error) {
			fmt.Printf("  [Adapter] Result: %s\n", input.Output)
			return input, nil
		})),
	)

	// ─── Execute ───
	ctx := context.Background()
	result, steps, err := pipeline.Run(ctx, State{Input: "hello world"})
	if err != nil {
		fmt.Printf("Pipeline error: %v\n", err)
		return
	}

	fmt.Printf("\nResult: %s\n", result.Output)
	fmt.Printf("Steps executed: %d\n", len(steps))
	for _, s := range steps {
		fmt.Printf("  [%s/%s] %s (%dms)\n", s.Unit, s.Pattern, s.Action, s.DurationMs)
	}
}
