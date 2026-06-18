//go:build lecore_tier1 || lecore_tier2 || lecore_tier3 || lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

package core

import (
	"fmt"
	"sync"
	"time"
)

// ──────────────────────────────────────────────
// TASK-6.3: Graceful Degradation Strategy
// ──────────────────────────────────────────────

// DegradationMode represents the current degradation level.
type DegradationMode string

const (
	DegradationNone         DegradationMode = "none"
	DegradationNonCritical  DegradationMode = "non_critical"
	DegradationSafe         DegradationMode = "safe"
	DegradationEmergency    DegradationMode = "emergency"
)

// DegradationManager manages graceful degradation of system operations.
// It controls which operations should be processed based on their criticality
// and the current degradation mode.
type DegradationManager struct {
	mu   sync.RWMutex
	mode DegradationMode
	obs  ObservationAdapter
}

// NewDegradationManager creates a new DegradationManager starting in normal mode.
// If obs is nil, a NoOpObservationAdapter is used.
func NewDegradationManager(obs ObservationAdapter) *DegradationManager {
	if obs == nil {
		obs = &NoOpObservationAdapter{}
	}
	return &DegradationManager{
		mode: DegradationNone,
		obs:  obs,
	}
}

// CurrentMode returns the current degradation mode.
func (d *DegradationManager) CurrentMode() DegradationMode {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.mode
}

// Degrade sets the degradation mode and records an audit step.
func (d *DegradationManager) Degrade(mode DegradationMode) {
	d.mu.Lock()
	defer d.mu.Unlock()

	previous := d.mode
	d.mode = mode

	es := NewExecutionStep("DegradationManager", "degrade",
		fmt.Sprintf("degradation mode changed from %q to %q", previous, mode),
		"degradation",
	)
	es.Metadata = map[string]interface{}{
		"previous_mode": string(previous),
		"new_mode":      string(mode),
		"timestamp":     time.Now(),
	}
	d.obs.Record([]ExecutionStep{es})
}

// Recover restores the degradation mode to normal (none).
func (d *DegradationManager) Recover() {
	d.Degrade(DegradationNone)
}

// ShouldProcess determines whether an operation with the given criticality
// should be processed under the current degradation mode.
//
// Rules:
//   - "critical" operations: always true
//   - "high" operations: false only in emergency mode
//   - "normal" operations: false in safe or emergency mode
//   - "low" operations: false in non_critical, safe, or emergency mode
func (d *DegradationManager) ShouldProcess(criticality string) bool {
	d.mu.RLock()
	defer d.mu.RUnlock()

	switch criticality {
	case "critical":
		return true
	case "high":
		return d.mode != DegradationEmergency
	case "normal":
		return d.mode != DegradationSafe && d.mode != DegradationEmergency
	case "low":
		return d.mode == DegradationNone
	default:
		// Treat unknown criticality as normal
		return d.mode != DegradationSafe && d.mode != DegradationEmergency
	}
}