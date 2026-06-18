// Tier L0 — Prototype: kernel-only pipeline (types, atom, port, adapter, step, composer, errors)
// Build: go build -tags lecore_tier0
// Suitable for: <10 files, 1 developer, scripts and prototypes.
//
// Demonstrates:
//   - Pipeline creation with Atom -> Port -> Adapter pattern
//   - AutoDetect for tier selection
//   - NoOpObservationAdapter for zero-overhead observation
//   - ComplexityTier awareness

package main

import (
	"context"
	"fmt"

	core "low-entropy-core/go-core"
)

func main() {
	// Step 1: Auto-detect the appropriate tier for this project
	tier := core.AutoDetect(".")
	fmt.Printf("Auto-detected tier: %s (L%d)\n", tier, tier)
	fmt.Printf("Framework files at this tier: %d\n", tier.FrameworkFileCount())

	// Step 2: Create a minimal pipeline with NoOp observation
	obs := &core.NoOpObservationAdapter{}
	pipeline := core.NewPipeline[any](obs)

	// Add an Atom (pure function, no side effects)
	upperCaseAtom := core.Atom[any, any](func(input any) any {
		s := fmt.Sprintf("%v", input)
		result := ""
		for _, ch := range s {
			if ch >= 'a' && ch <= 'z' {
				result += string(ch - 32)
			} else {
				result += string(ch)
			}
		}
		return result
	})
	pipeline.AddStep(core.AtomAsStep(upperCaseAtom))

	// Add a Port (contract validation gateway)
	validatePort := core.NewPort[any, any](func(ctx context.Context, input any) (any, error) {
		s := fmt.Sprintf("%v", input)
		if len(s) == 0 {
			return nil, fmt.Errorf("validation: empty input")
		}
		fmt.Printf("  [Port] validated: %s\n", s)
		return input, nil
	})
	pipeline.AddStep(core.PortAsStep(validatePort))

	// Add an Adapter (side-effect boundary)
	logAdapter := core.NewAdapter[any, any](func(ctx context.Context, input any) (any, error) {
		fmt.Printf("  [Adapter] logging: %v\n", input)
		return input, nil
	})
	pipeline.AddStep(core.AdapterAsStep(logAdapter))

	// Step 3: Execute the pipeline
	ctx := context.Background()
	result, steps, err := pipeline.Run(ctx, "hello-world")
	if err != nil {
		fmt.Printf("Pipeline failed: %v\n", err)
		return
	}

	fmt.Printf("\nResult: %v\n", result)
	fmt.Printf("Steps executed: %d\n", len(steps))
	for _, s := range steps {
		fmt.Printf("  [%s/%s] %s (%dms)\n", s.Unit, s.Pattern, s.Action, s.DurationMs)
	}
}