package core

import (
	"sync"
	"time"
)

// ──────────────────────────────────────────────
// EntropyMetrics — system entropy tracking
// ──────────────────────────────────────────────

// EntropySnapshot captures the entropy state of the system at a point in time.
// Entropy is measured by: total steps, error rate, unit type distribution,
// pattern diversity, and observation coverage.
type EntropySnapshot struct {
	// Timestamp is when this snapshot was taken.
	Timestamp time.Time `json:"timestamp"`

	// TotalSteps is the total number of execution steps recorded.
	TotalSteps int `json:"total_steps"`

	// ErrorSteps is the number of steps with errors.
	ErrorSteps int `json:"error_steps"`

	// ErrorRate is the ratio of error steps to total steps.
	ErrorRate float64 `json:"error_rate"`

	// UnitDistribution is the count of steps per unit type.
	UnitDistribution map[string]int `json:"unit_distribution"`

	// PatternDistribution is the count of steps per pattern.
	PatternDistribution map[string]int `json:"pattern_distribution"`

	// UniquePatterns is the number of distinct patterns observed.
	UniquePatterns int `json:"unique_patterns"`

	// UniqueUnits is the number of distinct unit types observed.
	UniqueUnits int `json:"unique_units"`

	// AvgDurationMs is the average step duration in milliseconds.
	AvgDurationMs float64 `json:"avg_duration_ms"`

	// EntropyScore is a composite score (0-100) representing system entropy.
	// Higher = more entropy (more complex, more diverse).
	// Formula: (uniquePatterns * errorRate * log(totalSteps)) normalized.
	EntropyScore float64 `json:"entropy_score"`
}

// EntropyCollector computes entropy snapshots from a StepStore.
// It is a pure function adapter — no side effects, no I/O.
type EntropyCollector struct {
	mu sync.Mutex
}

// NewEntropyCollector creates a new entropy collector.
func NewEntropyCollector() *EntropyCollector {
	return &EntropyCollector{}
}

// Collect computes an entropy snapshot from the given StepStore.
func (e *EntropyCollector) Collect(store StepStore) EntropySnapshot {
	e.mu.Lock()
	defer e.mu.Unlock()

	steps := store.Query(StepQuery{})
	return e.computeFromSteps(steps)
}

// CollectFromSteps computes an entropy snapshot from a slice of ExecutionSteps.
func (e *EntropyCollector) CollectFromSteps(steps []ExecutionStep) EntropySnapshot {
	return e.computeFromSteps(steps)
}

func (e *EntropyCollector) computeFromSteps(steps []ExecutionStep) EntropySnapshot {
	snap := EntropySnapshot{
		Timestamp:          time.Now(),
		TotalSteps:         len(steps),
		UnitDistribution:   make(map[string]int),
		PatternDistribution: make(map[string]int),
	}

	if len(steps) == 0 {
		return snap
	}

	var totalDuration float64
	patternSet := make(map[string]bool)
	unitSet := make(map[string]bool)

	for _, step := range steps {
		snap.UnitDistribution[step.Unit]++
		snap.PatternDistribution[step.Pattern]++

		if step.Pattern != "" {
			patternSet[step.Pattern] = true
		}
		if step.Unit != "" {
			unitSet[step.Unit] = true
		}

		if step.Error != nil {
			snap.ErrorSteps++
		}
		totalDuration += float64(step.DurationMs)
	}

	snap.UniquePatterns = len(patternSet)
	snap.UniqueUnits = len(unitSet)
	snap.ErrorRate = float64(snap.ErrorSteps) / float64(snap.TotalSteps)
	snap.AvgDurationMs = totalDuration / float64(snap.TotalSteps)

	// Compute entropy score
	// Higher unique patterns + higher error rate + more steps = higher entropy
	patternFactor := float64(snap.UniquePatterns) / 10.0
	if patternFactor > 1 {
		patternFactor = 1
	}
	errorFactor := snap.ErrorRate * 10
	if errorFactor > 1 {
		errorFactor = 1
	}
	stepFactor := float64(snap.TotalSteps) / 10000.0
	if stepFactor > 1 {
		stepFactor = 1
	}

	snap.EntropyScore = (patternFactor*0.4 + errorFactor*0.3 + stepFactor*0.3) * 100

	return snap
}

// EntropyAtom creates an Atom that computes entropy from the current step store.
func EntropyAtom(store StepStore) Atom[struct{}, EntropySnapshot] {
	collector := NewEntropyCollector()
	return func(_ struct{}) EntropySnapshot {
		return collector.Collect(store)
	}
}

// EntropyCollectorAsStep wraps the collector as a Step.
func EntropyCollectorAsStep(collector *EntropyCollector, store StepStore) Step[struct{}, EntropySnapshot] {
	return AtomAsStep(EntropyAtom(store))
}