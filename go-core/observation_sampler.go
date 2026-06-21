//go:build lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

package core

import (
	"math/rand"
	"sync"
)

// ──────────────────────────────────────────────
// SamplingPolicy — step sampling interface
// ──────────────────────────────────────────────

// SamplingPolicy decides whether a given ExecutionStep should be kept.
type SamplingPolicy interface {
	// ShouldKeep returns true if the step should be retained.
	ShouldKeep(step ExecutionStep) bool

	// Name returns the policy name for identification.
	Name() string
}

// ──────────────────────────────────────────────
// RateSampler — probabilistic sampling
// ──────────────────────────────────────────────

// RateSampler keeps steps at the given rate (0.0 to 1.0).
// A rate of 0.1 means approximately 10% of steps are kept.
type RateSampler struct {
	rate float64
	rng  *rand.Rand
	mu   sync.Mutex
}

// NewRateSampler creates a rate-based sampler.
func NewRateSampler(rate float64) *RateSampler {
	if rate < 0 {
		rate = 0
	}
	if rate > 1 {
		rate = 1
	}
	return &RateSampler{
		rate: rate,
		rng:  rand.New(rand.NewSource(rand.Int63())),
	}
}

// ShouldKeep returns true with probability equal to the rate.
func (s *RateSampler) ShouldKeep(step ExecutionStep) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.rng.Float64() < s.rate
}

// Name returns the policy name.
func (s *RateSampler) Name() string {
	return "RateSampler"
}

// ──────────────────────────────────────────────
// ErrorAlwaysSampler — always keeps error steps
// ──────────────────────────────────────────────

// ErrorAlwaysSampler keeps all steps that have a non-nil Error field.
type ErrorAlwaysSampler struct{}

// NewErrorAlwaysSampler creates an error-always sampler.
func NewErrorAlwaysSampler() *ErrorAlwaysSampler {
	return &ErrorAlwaysSampler{}
}

// ShouldKeep returns true for steps with errors.
func (s *ErrorAlwaysSampler) ShouldKeep(step ExecutionStep) bool {
	return step.Error != nil
}

// Name returns the policy name.
func (s *ErrorAlwaysSampler) Name() string {
	return "ErrorAlwaysSampler"
}

// ──────────────────────────────────────────────
// CompositeSampler — combines multiple policies
// ──────────────────────────────────────────────

// CompositeSampler keeps a step if ANY of its policies vote to keep it.
// Policies are evaluated in order; the first "keep" vote wins.
type CompositeSampler struct {
	policies []SamplingPolicy
}

// NewCompositeSampler creates a composite sampler from multiple policies.
func NewCompositeSampler(policies ...SamplingPolicy) *CompositeSampler {
	return &CompositeSampler{policies: policies}
}

// ShouldKeep returns true if any policy votes to keep.
func (s *CompositeSampler) ShouldKeep(step ExecutionStep) bool {
	for _, p := range s.policies {
		if p.ShouldKeep(step) {
			return true
		}
	}
	return false
}

// Name returns the policy name.
func (s *CompositeSampler) Name() string {
	return "CompositeSampler"
}

// AddPolicy appends a policy to the composite.
func (s *CompositeSampler) AddPolicy(p SamplingPolicy) {
	s.policies = append(s.policies, p)
}

// ──────────────────────────────────────────────
// Sampler — applies sampling to a step stream
// ──────────────────────────────────────────────

// Sampler applies a SamplingPolicy to a stream of ExecutionSteps.
// Steps that pass the policy are forwarded; dropped steps produce a SummaryStep.
type Sampler struct {
	policy     SamplingPolicy
	dropped    int
	droppedMu  sync.Mutex
}

// NewSampler creates a new Sampler with the given policy.
func NewSampler(policy SamplingPolicy) *Sampler {
	return &Sampler{policy: policy}
}

// Apply filters the steps through the policy.
// Returns the kept steps. Dropped steps are counted.
func (s *Sampler) Apply(steps []ExecutionStep) []ExecutionStep {
	kept := make([]ExecutionStep, 0, len(steps))
	dropped := 0

	for _, step := range steps {
		if s.policy.ShouldKeep(step) {
			kept = append(kept, step)
		} else {
			dropped++
		}
	}

	s.droppedMu.Lock()
	s.dropped += dropped
	s.droppedMu.Unlock()

	if dropped > 0 {
		// Create a summary step for the dropped batch
		summary := NewExecutionStep("Sampler", "Sampled", "dropped steps", "Sampled")
		summary.Metadata = map[string]any{
			"dropped_count": dropped,
			"kept_count":    len(kept),
			"policy":        s.policy.Name(),
		}
		kept = append(kept, summary)
	}

	return kept
}

// DroppedCount returns the total number of dropped steps.
func (s *Sampler) DroppedCount() int {
	s.droppedMu.Lock()
	defer s.droppedMu.Unlock()
	return s.dropped
}