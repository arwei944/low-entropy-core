// Tier L1 — Microservice: kernel + basic observation, degradation
// Build: go build -tags lecore_tier1
// Suitable for: 10-100 files, 1-3 developers, independent services.
//
// Demonstrates:
//   - Pipeline with InMemoryObservationAdapter (real buffered observation)
//   - Degradation strategy when pipeline steps fail
//   - ExecutionStep tracing and inspection
//   - Multi-step pipeline with error handling

package main

import (
	"context"
	"fmt"

	core "low-entropy-core/go-core"
)

func main() {
	fmt.Println("=== Tier L1 — Microservice Demo ===")

	// Step 1: Create a pipeline with in-memory observation
	obs := &core.InMemoryObservationAdapter{}
	pipeline := core.NewPipeline[any](obs)

	// Add validation Port
	validatePort := core.NewPort[any, any](func(ctx context.Context, input any) (any, error) {
		fmt.Printf("  [Validate] input: %v\n", input)
		return input, nil
	})
	pipeline.AddStep(core.PortAsStep(validatePort))

	// Add transform Atom
	transformAtom := core.Atom[any, any](func(input any) any {
		return fmt.Sprintf("{\"data\": \"%v\", \"ts\": \"now\"}", input)
	})
	pipeline.AddStep(core.AtomAsStep(transformAtom))

	// Add storage Adapter
	storeAdapter := core.NewAdapter[any, any](func(ctx context.Context, input any) (any, error) {
		fmt.Printf("  [Store] persisting: %v\n", input)
		return input, nil
	})
	pipeline.AddStep(core.AdapterAsStep(storeAdapter))

	// Step 2: Execute pipeline multiple times
	ctx := context.Background()
	for i := 1; i <= 3; i++ {
		req := fmt.Sprintf("request-%d", i)
		result, _, err := pipeline.Run(ctx, req)
		if err != nil {
			// Degradation: fall back to simpler processing
			fmt.Printf("  Pipeline failed for %s, degrading: %v\n", req, err)
			result = "degraded-response"
		} else {
			fmt.Printf("  Pipeline result: %v\n", result)
		}
	}

	// Step 3: Inspect observation records
	steps := obs.GetSteps()
	fmt.Printf("\nObservation records: %d\n", len(steps))
	for _, step := range steps {
		status := "ok"
		if step.Error != nil {
			status = "error"
		}
		fmt.Printf("  [%s/%s] %s (duration: %dms)\n", step.Unit, step.Pattern, status, step.DurationMs)
	}

	// Step 4: Check tier info
	tier := core.AutoDetect(".")
	fmt.Printf("\nProject tier: %s (L%d), framework files: %d\n",
		tier, tier, tier.FrameworkFileCount())
}