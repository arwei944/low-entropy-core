//go:build lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

// Package core — 依赖分析包装器 (v4.0)
//
// 包含:
//   - DependencyGuard: 依赖分析包装器
//   - DependencyViolation: 依赖图中的违规
//
// 所有类型均为线程安全。
package core

import (
	"fmt"
	"sync/atomic"
	"time"
)

// ============================================================================
// SECTION 2: DependencyGuard — 依赖分析包装器
// ============================================================================

// DependencyViolation 表示依赖图中的违规。
type DependencyViolation struct {
	Type        string
	Description string
	Nodes       []string
	DetectedAt  time.Time
}

// DependencyGuard 包装依赖图，提供完整的依赖分析。
type DependencyGuard struct {
	graph      *DependencyGraph
	violations atomic.Value // []DependencyViolation
}

func NewDependencyGuard() *DependencyGuard {
	dg := &DependencyGuard{
		graph: NewDependencyGraph(),
	}
	dg.violations.Store([]DependencyViolation{})
	return dg
}

func (dg *DependencyGuard) AddPipelineDependency(from, to string) {
	dg.graph.AddEdge(from, to)
}

func (dg *DependencyGuard) RemovePipelineDependency(from, to string) {
	dg.graph.RemoveEdge(from, to)
}

func (dg *DependencyGuard) Analyze() []DependencyViolation {
	var violations []DependencyViolation

	cycles := dg.graph.DetectCycles()
	for _, cycle := range cycles {
		violations = append(violations, DependencyViolation{
			Type:        "cycle",
			Description: fmt.Sprintf("circular dependency detected: %v", cycle),
			Nodes:       cycle,
			DetectedAt:  time.Now(),
		})
	}

	redundant := dg.graph.DetectRedundant()
	for _, r := range redundant {
		violations = append(violations, DependencyViolation{
			Type:        "redundant",
			Description: fmt.Sprintf("redundant dependency: %s -> %s (via %s)", r.From, r.To, r.Intermediate),
			Nodes:       []string{r.From, r.To, r.Intermediate},
			DetectedAt:  time.Now(),
		})
	}

	islands := dg.graph.DetectIslands()
	for _, island := range islands {
		violations = append(violations, DependencyViolation{
			Type:        "island",
			Description: fmt.Sprintf("isolated pipeline: %s (no dependencies)", island),
			Nodes:       []string{island},
			DetectedAt:  time.Now(),
		})
	}

	dg.violations.Store(violations)
	return violations
}

func (dg *DependencyGuard) GetViolations() []DependencyViolation {
	v := dg.violations.Load()
	if v == nil {
		return nil
	}
	return v.([]DependencyViolation)
}
