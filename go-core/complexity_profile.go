// complexity_profile.go — Progressive Complexity Model tier system
// This file is part of the kernel (always compiled, no build tags).
// It defines the 8-tier complexity system that gates all non-kernel features,
// plus AppConfig — the universal configuration struct used by all tiers.

package core

import "time"

// ComplexityTier defines the architectural complexity level of a project.
// Higher tiers unlock more framework capabilities via Go build tags.
type ComplexityTier int

const (
	// TierL0 — Prototype: kernel only (types, atom, port, adapter, step, observation, composer, errors, perf_core).
	// Suitable for: <10 files, 1 developer, scripts and prototypes.
	TierL0 ComplexityTier = iota

	// TierL1 — Microservice: kernel + basic observation, error handling, degradation.
	// Suitable for: 10-100 files, 1-3 developers, independent services.
	TierL1

	// TierL2 — Mid-size Service: + event store, event bus, config, resilience, port contracts.
	// Suitable for: 100-1000 files, 3-10 developers, multi-module services.
	TierL2

	// TierL3 — Large Service: + guardian static analysis, persistent storage, security,
	// agent submission/execution, scheduler, handoff, schema, saga transactions.
	// Suitable for: 1000-10000 files, 10-50 developers, platform services.
	TierL3

	// TierL4 — Platform: + full guardian (entropy, dependency, transparency, decision),
	// observation pipeline (sampling, aggregation, API), performance infrastructure,
	// projection, idempotency, multi-tenancy.
	// Suitable for: 10000-100K files, 50-200 developers, multi-service platforms.
	TierL4

	// TierL5 — Enterprise Platform: + distributed resilience, remote composer,
	// event store upgrade utilities, application launcher.
	// Suitable for: 100K-1M files, 200-1000 developers, enterprise platforms.
	TierL5

	// TierL6 — System-level: + multi-language IDL, distributed workflow engine,
	// tiered event storage, OpenTelemetry, supply chain security.
	// Suitable for: 1M-10M files, 1000-4000 developers, operating system scale.
	TierL6

	// TierL7 — Windows-scale: + strangler fig patterns, NUMA-aware scheduling,
	// zero-copy pipelines, chaos engineering, multi-version compatibility.
	// Suitable for: 10M-50M+ files, 4000+ developers, the largest commercial projects.
	TierL7

	// TierAuto — Auto-detect the appropriate tier by scanning the project directory.
	// When set, the framework runs AutoDetect() on initialization.
	TierAuto ComplexityTier = -1
)

// TierNames maps each tier to its human-readable name.
var TierNames = map[ComplexityTier]string{
	TierL0: "Prototype",
	TierL1: "Microservice",
	TierL2: "Mid-size Service",
	TierL3: "Large Service",
	TierL4: "Platform",
	TierL5: "Enterprise Platform",
	TierL6: "System-level",
	TierL7: "Windows-scale",
}

// TierFileCounts tracks the approximate number of framework source files
// compiled at each tier level.
var TierFileCounts = map[ComplexityTier]int{
	TierL0: 12,
	TierL1: 14,
	TierL2: 19,
	TierL3: 30,
	TierL4: 49,
	TierL5: 48,
	TierL6: 48,
	TierL7: 48,
}

// String returns the human-readable name of the tier.
func (t ComplexityTier) String() string {
	if name, ok := TierNames[t]; ok {
		return name
	}
	return "Unknown"
}

// FrameworkFileCount returns the number of framework source files active at this tier.
func (t ComplexityTier) FrameworkFileCount() int {
	if count, ok := TierFileCounts[t]; ok {
		return count
	}
	return 0
}

// ──────────────────────────────────────────────────────────────────────────────
// AppConfig — universal application configuration (kernel, available at all tiers)
// ──────────────────────────────────────────────────────────────────────────────

// AppConfig is the universal application configuration.
// Always available regardless of ComplexityTier. Fields reference features
// that may be unavailable at lower tiers, but the struct itself is just data
// — zero-cost if unused.
type AppConfig struct {
	Name    string `json:"name"`
	Version string `json:"version"`

	// Persistence
	StorageDir string `json:"storage_dir"`

	// HTTP
	HTTPAddr string `json:"http_addr"`

	// Observation
	ObservationBufferSize int `json:"observation_buffer_size"`

	// Guardian
	GuardianEnabled bool    `json:"guardian_enabled"`
	EntropyCeiling  float64 `json:"entropy_ceiling"`

	// Scheduler
	SchedulerEnabled  bool          `json:"scheduler_enabled"`
	AgentHeartbeatTTL time.Duration `json:"agent_heartbeat_ttl"`
}

// DefaultAppConfig returns safe default configuration.
func DefaultAppConfig() AppConfig {
	return AppConfig{
		Name:                  "low-entropy-core",
		Version:               "5.0.0",
		StorageDir:            "./data",
		HTTPAddr:              ":8080",
		ObservationBufferSize: 10000,
		GuardianEnabled:       true,
		EntropyCeiling:        0.8,
		SchedulerEnabled:      false,
		AgentHeartbeatTTL:     30 * time.Second,
	}
}