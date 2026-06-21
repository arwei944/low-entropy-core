//go:build lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

package core

import (
	"context"
	"testing"
	"time"
)

// ──────────────────────────────────────────────
// ResilienceChain Integration Test
// ──────────────────────────────────────────────

func TestResilienceChain_Normal(t *testing.T) {
	ctx := context.Background()
	obs := &InMemoryObservationAdapter{}

	inner := NewPipeline[int](obs,
		AtomAsStep(Atom[int, int](func(x int) int { return x * 3 })),
	)

	config := ResilienceConfig[int]{
		RateLimit:                100,
		RateLimitBurst:           200,
		BulkheadMax:              10,
		CircuitBreakerThreshold:  5,
		CircuitBreakerCooldown:   10 * time.Second,
	}

	chain := ResilienceChain[int](inner, config)
	result, _, err := chain.Run(ctx, 7)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != 21 {
		t.Errorf("expected 21, got %d", result)
	}
}
