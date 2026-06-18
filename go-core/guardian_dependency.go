// Package core — 依赖图守卫 + 架构守卫 + 快照摘要 (v4.0)
//
// 合并自: guardian_dependency.go + guardian_architecture.go
//
// 包含:
//   - DependencyGraph: Pipeline 依赖图 (DFS 循环检测, Kahn 拓扑排序, 冗余检测)
//   - DependencyGuard: 依赖分析包装器
//   - SnapshotSummaryStore: Handoff 快照摘要存储
//   - InMemorySnapshotStore: 内存快照摘要存储
//   - ArchitectureGuard: 架构合规检查 (Port)
//   - typeSet / GlobalPrimitiveTypeSet: 类型白名单 (O(1) 查找)
//
// 所有类型均为线程安全。
package core

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// ============================================================================
// SECTION 1: DependencyGraph — 循环依赖检测
// ============================================================================

// DependencyGraph 表示 Pipeline 之间的依赖关系。
// 使用 DFS 染色算法检测循环依赖，O(V+E) 时间复杂度。
type DependencyGraph struct {
	mu    sync.RWMutex
	edges map[string]map[string]bool
	nodes map[string]bool
}

func NewDependencyGraph() *DependencyGraph {
	return &DependencyGraph{
		edges: make(map[string]map[string]bool),
		nodes: make(map[string]bool),
	}
}

func (g *DependencyGraph) AddEdge(from, to string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.edges[from] == nil {
		g.edges[from] = make(map[string]bool)
	}
	g.edges[from][to] = true
	g.nodes[from] = true
	g.nodes[to] = true
}

func (g *DependencyGraph) RemoveEdge(from, to string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.edges[from] != nil {
		delete(g.edges[from], to)
		if len(g.edges[from]) == 0 {
			delete(g.edges, from)
		}
	}
}

// DetectCycles 检测依赖图中的循环依赖 (DFS 三色标记).
func (g *DependencyGraph) DetectCycles() [][]string {
	g.mu.RLock()
	defer g.mu.RUnlock()

	const (
		white = 0
		gray  = 1
		black = 2
	)

	colors := make(map[string]int)
	var cycles [][]string
	var path []string

	var dfs func(node string) bool
	dfs = func(node string) bool {
		colors[node] = gray
		path = append(path, node)

		for neighbor := range g.edges[node] {
			switch colors[neighbor] {
			case gray:
				cycleStart := -1
				for i, n := range path {
					if n == neighbor {
						cycleStart = i
						break
					}
				}
				if cycleStart >= 0 {
					cycle := make([]string, len(path)-cycleStart)
					copy(cycle, path[cycleStart:])
					cycles = append(cycles, cycle)
				}
			case white:
				if dfs(neighbor) {
					return true
				}
			}
		}

		colors[node] = black
		path = path[:len(path)-1]
		return false
	}

	for node := range g.nodes {
		colors[node] = white
	}

	for node := range g.nodes {
		if colors[node] == white {
			dfs(node)
		}
	}

	return cycles
}

// TopologicalSort 拓扑排序 (Kahn 算法).
func (g *DependencyGraph) TopologicalSort() []string {
	g.mu.RLock()
	defer g.mu.RUnlock()

	inDegree := make(map[string]int)
	for node := range g.nodes {
		inDegree[node] = 0
	}
	for from := range g.edges {
		for to := range g.edges[from] {
			inDegree[to]++
		}
	}

	queue := make([]string, 0)
	for node := range g.nodes {
		if inDegree[node] == 0 {
			queue = append(queue, node)
		}
	}

	var result []string
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		result = append(result, node)

		for neighbor := range g.edges[node] {
			inDegree[neighbor]--
			if inDegree[neighbor] == 0 {
				queue = append(queue, neighbor)
			}
		}
	}

	if len(result) != len(g.nodes) {
		return nil
	}
	return result
}

// DetectRedundant 检测冗余依赖 (A→B→C 且 A→C 直接存在).
func (g *DependencyGraph) DetectRedundant() []DependencyEdge {
	g.mu.RLock()
	defer g.mu.RUnlock()

	var redundant []DependencyEdge
	reachable := g.transitiveClosure()

	for from := range g.edges {
		for to := range g.edges[from] {
			for intermediate := range g.nodes {
				if intermediate == from || intermediate == to {
					continue
				}
				if reachable[from][intermediate] && reachable[intermediate][to] {
					redundant = append(redundant, DependencyEdge{
						From:         from,
						To:           to,
						Intermediate: intermediate,
					})
				}
			}
		}
	}
	return redundant
}

func (g *DependencyGraph) transitiveClosure() map[string]map[string]bool {
	reachable := make(map[string]map[string]bool)
	for node := range g.nodes {
		reachable[node] = make(map[string]bool)
	}
	for from := range g.edges {
		for to := range g.edges[from] {
			reachable[from][to] = true
		}
	}
	for k := range g.nodes {
		for i := range g.nodes {
			for j := range g.nodes {
				if reachable[i][k] && reachable[k][j] {
					reachable[i][j] = true
				}
			}
		}
	}
	return reachable
}

// DependencyEdge 表示一条依赖边。
type DependencyEdge struct {
	From         string
	To           string
	Intermediate string
}

// DetectIslands 检测孤岛 (无入度且无出度的节点).
func (g *DependencyGraph) DetectIslands() []string {
	g.mu.RLock()
	defer g.mu.RUnlock()

	var islands []string
	for node := range g.nodes {
		hasIn := false
		hasOut := len(g.edges[node]) > 0

		for from := range g.edges {
			if g.edges[from][node] {
				hasIn = true
				break
			}
		}

		if !hasIn && !hasOut {
			islands = append(islands, node)
		}
	}
	return islands
}

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

// ============================================================================
// SECTION 3: SnapshotSummaryStore — Handoff 快照摘要存储
// ============================================================================

// HandoffSnapshotSummary 是 Handoff 时的快照摘要。
type HandoffSnapshotSummary struct {
	PipelineID  string    `json:"pipeline_id"`
	StepCount   int       `json:"step_count"`
	StateHash   string    `json:"state_hash"`
	Timestamp   time.Time `json:"timestamp"`
	Version     int64     `json:"version"`
	Description string    `json:"description,omitempty"`
}

// SnapshotSummaryStore 是快照摘要的存储接口。
type SnapshotSummaryStore interface {
	Save(summary HandoffSnapshotSummary) error
	Load(pipelineID string, version int64) (*HandoffSnapshotSummary, error)
	List(pipelineID string) ([]HandoffSnapshotSummary, error)
	Compare(a, b *HandoffSnapshotSummary) *SnapshotDiff
}

// InMemorySnapshotStore 是内存中的快照摘要存储。
type InMemorySnapshotStore struct {
	mu        sync.RWMutex
	snapshots map[string][]HandoffSnapshotSummary
	latest    map[string]int64
}

func NewInMemorySnapshotStore() *InMemorySnapshotStore {
	return &InMemorySnapshotStore{
		snapshots: make(map[string][]HandoffSnapshotSummary),
		latest:    make(map[string]int64),
	}
}

func (s *InMemorySnapshotStore) Save(summary HandoffSnapshotSummary) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snapshots[summary.PipelineID] = append(s.snapshots[summary.PipelineID], summary)
	s.latest[summary.PipelineID] = summary.Version
	return nil
}

func (s *InMemorySnapshotStore) Load(pipelineID string, version int64) (*HandoffSnapshotSummary, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	snapshots, ok := s.snapshots[pipelineID]
	if !ok {
		return nil, fmt.Errorf("snapshot not found for pipeline %s", pipelineID)
	}
	for _, snap := range snapshots {
		if snap.Version == version {
			return &snap, nil
		}
	}
	return nil, fmt.Errorf("snapshot version %d not found for pipeline %s", version, pipelineID)
}

func (s *InMemorySnapshotStore) List(pipelineID string) ([]HandoffSnapshotSummary, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.snapshots[pipelineID], nil
}

// SnapshotDiff 表示两个快照之间的差异。
type SnapshotDiff struct {
	PipelineID    string
	FromVersion   int64
	ToVersion     int64
	StepCountDiff int
	HashChanged   bool
	AddedSteps    int
	RemovedSteps  int
	ModifiedSteps int
}

func (s *InMemorySnapshotStore) Compare(a, b *HandoffSnapshotSummary) *SnapshotDiff {
	if a == nil || b == nil {
		return nil
	}
	diff := &SnapshotDiff{
		PipelineID:    a.PipelineID,
		FromVersion:   a.Version,
		ToVersion:     b.Version,
		StepCountDiff: b.StepCount - a.StepCount,
		HashChanged:   a.StateHash != b.StateHash,
	}
	if a.StepCount != b.StepCount {
		if b.StepCount > a.StepCount {
			diff.AddedSteps = b.StepCount - a.StepCount
		} else {
			diff.RemovedSteps = a.StepCount - b.StepCount
		}
	}
	return diff
}

// ============================================================================
// SECTION 4: ArchitectureGuard — 架构合规检查 (Port)
// ============================================================================

// ArchitectureInput contains data for architecture compliance checking.
type ArchitectureInput struct {
	Pipelines       []PipelineDescriptor
	SchemaChanges   []SchemaChange
	NewDependencies []string
	ConfigChanges   []string
}

// ArchitectureAlert is the output of the architecture guard.
type ArchitectureAlert struct {
	Violations                int
	NonPrimitiveTypes         []string
	UncheckedSchemaChanges    []string
	NewDependencies           []string
	UnauthorizedConfigChanges []string
	Message                   string
}

// allowedPatterns is the set of patterns that conform to the 4-primitive model.
var allowedPatterns = map[string]bool{
	"pipeline": true, "branch": true, "parallel": true, "timeout": true,
	"retry": true, "circuit_breaker": true, "fallback": true, "bulkhead": true,
	"rate_limit": true, "handoff": true, "scheduler": true, "match": true,
	"config_change": true, "resilience_chain": true,
}

// ArchitectureGuard is a Port that checks architecture compliance.
type ArchitectureGuard struct{}

func NewArchitectureGuard() *ArchitectureGuard {
	return &ArchitectureGuard{}
}

func (g *ArchitectureGuard) Validate(ctx context.Context, input ArchitectureInput) (ArchitectureAlert, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	select {
	case <-ctx.Done():
		return ArchitectureAlert{}, ctx.Err()
	default:
	}

	var alert ArchitectureAlert
	alert.NonPrimitiveTypes = checkNonPrimitiveTypes(input.Pipelines)
	alert.UncheckedSchemaChanges = checkUncheckedSchemaChanges(input.SchemaChanges)
	alert.NewDependencies = append([]string{}, input.NewDependencies...)
	alert.UnauthorizedConfigChanges = append([]string{}, input.ConfigChanges...)
	alert.Violations = len(alert.NonPrimitiveTypes) + len(alert.UncheckedSchemaChanges) +
		len(alert.NewDependencies) + len(alert.UnauthorizedConfigChanges)
	alert.Message = buildArchAlertMessage(alert)
	return alert, nil
}

func checkNonPrimitiveTypes(pipelines []PipelineDescriptor) []string {
	var violations []string
	for _, p := range pipelines {
		for _, pattern := range p.Patterns {
			if !allowedPatterns[pattern] {
				violations = append(violations,
					fmt.Sprintf("pipeline %q uses non-primitive pattern %q", p.Name, pattern))
			}
		}
	}
	return violations
}

func checkUncheckedSchemaChanges(changes []SchemaChange) []string {
	var violations []string
	for _, ch := range changes {
		if ch.Breaking {
			violations = append(violations,
				fmt.Sprintf("field %q: %s — %s", ch.Field, ch.Kind, ch.Detail))
		}
	}
	return violations
}

func buildArchAlertMessage(alert ArchitectureAlert) string {
	if alert.Violations == 0 {
		return "architecture compliance check passed — no violations detected"
	}
	var parts []string
	if len(alert.NonPrimitiveTypes) > 0 {
		parts = append(parts, fmt.Sprintf("non-primitive types: %s", strings.Join(alert.NonPrimitiveTypes, "; ")))
	}
	if len(alert.UncheckedSchemaChanges) > 0 {
		parts = append(parts, fmt.Sprintf("unchecked schema changes: %s", strings.Join(alert.UncheckedSchemaChanges, "; ")))
	}
	if len(alert.NewDependencies) > 0 {
		parts = append(parts, fmt.Sprintf("new dependencies: %s", strings.Join(alert.NewDependencies, ", ")))
	}
	if len(alert.UnauthorizedConfigChanges) > 0 {
		parts = append(parts, fmt.Sprintf("unauthorized config changes: %s", strings.Join(alert.UnauthorizedConfigChanges, ", ")))
	}
	return fmt.Sprintf("architecture violations (%d total): %s", alert.Violations, strings.Join(parts, " | "))
}

// ============================================================================
// SECTION 5: typeSet — 哈希集合查找 (O(1))
// ============================================================================

// typeSet 用于 O(1) 类型白名单查找，替代线性扫描。
type typeSet struct {
	names map[string]bool
}

func newTypeSet(names []string) *typeSet {
	ts := &typeSet{names: make(map[string]bool, len(names))}
	for _, n := range names {
		ts.names[n] = true
	}
	return ts
}

func (ts *typeSet) contains(name string) bool {
	return ts.names[name]
}

// buildPrimitiveTypeSet 构建四原语类型的白名单。
func buildPrimitiveTypeSet() *typeSet {
	primitives := []string{
		"Atom", "Port", "Adapter", "Composer",
		"Step", "StepFunc", "StepError",
		"ExecutionStep", "CompactExecutionStep", "TraceID", "SpanID", "CompactTraceID",
		"Pipeline", "Branch", "Parallel", "Retry", "Timeout", "Map",
		"FastPipeline", "FastComposer",
		"ObservationAdapter", "InMemoryObservationAdapter", "NoOpObservationAdapter",
		"ShardedObservationAdapter", "ObservationPipeline", "ObservationAPI",
		"StepStore", "InMemoryStepStore", "ShardedStepStore", "ShardedIndexedStepStore",
		"StepQuery", "TraceTree", "TraceNode",
		"Aggregator", "AggregatorConfig", "AggregateResult",
		"IncrementalAggregator", "ShardedAggregator",
		"Sampler", "SamplingPolicy", "RateSampler", "ErrorAlwaysSampler", "CompositeSampler",
		"CircuitBreaker", "RateLimiter", "Bulkhead", "Fallback", "ResilienceChain",
		"ResilienceConfig", "ShardedRateLimiter",
		"CircuitState", "CircuitClosed", "CircuitOpen", "CircuitHalfOpen",
		"Capability", "CapabilityStore", "InMemoryCapabilityStore",
		"AuditEntry", "AuditTrail", "InMemoryAuditTrail",
		"MerkleAuditChain", "MerkleProof", "MerkleEntry",
		"EventEnvelope", "EventStore", "EventBus", "EventBus_Subscription",
		"Projection", "ProjectionInput", "ProjectionOutput", "ProjectionHandler",
		"PublishResult", "EventHandler",
		"EntropyWatcher", "EntropySnapshot", "EntropyAlert", "EntropyLevel",
		"TransparencyWatcher", "TransparencyInput", "TransparencyAlert",
		"DriftDetector", "DriftInput", "DriftOutput",
		"ArchitectureGuard", "ArchitectureInput", "ArchitectureAlert",
		"DecisionEngine", "GuardianDecision", "GuardianAction", "GuardianInput",
		"AlertAdapter", "AlertResult", "AlertChannel",
		"ModuleEntropyTracker", "ModuleEntropyState",
		"PipelineStepGrowthDetector", "PipelineGrowthState",
		"AgentBehaviorDriftDetector", "AgentDriftHistory",
		"AdaptiveThresholdEngine", "AdaptiveThresholdConfig",
		"MultiDimensionEntropySnapshot", "MultiDimensionEntropyCollector",
		"DependencyGuard", "DependencyGraph", "DependencyViolation",
		"Stream", "StreamConfig", "StreamMap", "StreamFilter", "StreamReduce",
		"Window", "WindowByTime", "Merge", "Split", "Collect", "FromSlice",
		"IdempotentPort", "IdempotentStore", "InMemoryIdempotentStore",
		"IdempotentRequest", "IdempotentResult",
		"TenantID", "TenantContext", "TenantRequest", "TenantIsolationPort",
		"TransactionContext", "SagaStep", "SagaComposer",
		"DegradationManager", "DegradationMode",
		"Config", "ConfigBuilder", "ConfigHotReload",
		"HandoffManager", "HandoffStrategy", "HandoffSnapshot", "HandoffCheckpoint",
		"AgentPool", "AgentPoolConfig", "AgentWorker", "AgentTask",
		"Scheduler", "SchedulerComposer", "SchedulerConfig",
		"SchemaRegistry", "SchemaMigration", "SchemaVersion",
		"ArchitectureRegistry", "ArchitectureRegistryEntry", "PipelineDescriptor",
		"PortContract", "PortContractResult",
		"EntropyMetrics", "EntropyMetricsResult",
		"HandoffSnapshotSummary", "SnapshotSummaryStore", "InMemorySnapshotStore",
		"SnapshotDiff",
		"ShardedLock", "AtomicState", "BatchedUUIDGen",
		"StepMetadataPool", "StepSlicePool", "StringBuilderPool", "HashPool",
		"TDigest", "DistributionDriftDetector", "DistributionDriftResult",
		"AnomalyAutoLabeler", "AnomalyLabelType",
		"GlobalCircuitBreaker", "FederatedDegradationManager",
		"DistributedRateLimiter", "HealthCheckResponse",
	}
	return newTypeSet(primitives)
}

// GlobalPrimitiveTypeSet 全局预构建的类型白名单。
var GlobalPrimitiveTypeSet = buildPrimitiveTypeSet()