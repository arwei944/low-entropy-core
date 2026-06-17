// Package core — 依赖图守卫 + 快照摘要 (v4.0)
//
// 本文件包含：
//   - DependencyGuard：Pipeline 依赖图循环检测
//   - SnapshotSummaryStore：Handoff 快照摘要存储
//   - ArchitectureGuard 性能优化（哈希集合查找）
//
// 所有类型均为线程安全，使用 ShardedLock 和 atomic 操作。
package core

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// =============================================================================
// DependencyGuard — 循环依赖检测 (T4.1)
// =============================================================================

// DependencyGraph 表示 Pipeline 之间的依赖关系。
// 使用 DFS 染色算法检测循环依赖，O(V+E) 时间复杂度。
type DependencyGraph struct {
	mu    sync.RWMutex
	edges map[string]map[string]bool // from -> set of to
	nodes map[string]bool
}

// NewDependencyGraph 创建依赖图。
func NewDependencyGraph() *DependencyGraph {
	return &DependencyGraph{
		edges: make(map[string]map[string]bool),
		nodes: make(map[string]bool),
	}
}

// AddEdge 添加一条依赖边（from 依赖 to）。
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

// RemoveEdge 移除一条依赖边。
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

// DetectCycles 检测依赖图中的循环依赖。
// 使用 DFS 三色标记算法（白/灰/黑）。
// 返回所有检测到的循环路径。
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
				// 找到循环
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

// TopologicalSort 拓扑排序（Kahn 算法）。
// 如果存在循环依赖，返回 nil。
func (g *DependencyGraph) TopologicalSort() []string {
	g.mu.RLock()
	defer g.mu.RUnlock()

	// 计算入度
	inDegree := make(map[string]int)
	for node := range g.nodes {
		inDegree[node] = 0
	}
	for from := range g.edges {
		for to := range g.edges[from] {
			inDegree[to]++
		}
	}

	// 入度为 0 的节点入队
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

	// 如果结果数量不等于节点数，存在循环
	if len(result) != len(g.nodes) {
		return nil
	}
	return result
}

// DetectRedundant 检测冗余依赖（A→B→C 且 A→C 直接存在）。
func (g *DependencyGraph) DetectRedundant() []DependencyEdge {
	g.mu.RLock()
	defer g.mu.RUnlock()

	var redundant []DependencyEdge
	// 计算传递闭包
	reachable := g.transitiveClosure()

	for from := range g.edges {
		for to := range g.edges[from] {
			// 检查是否存在间接路径
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

// transitiveClosure 计算传递闭包（Floyd-Warshall 简化版）。
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
	// Floyd-Warshall
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
	Intermediate string // 仅冗余依赖时有值
}

// DetectIslands 检测孤岛（无入度且无出度的节点）。
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

// =============================================================================
// DependencyViolation — 依赖违规
// =============================================================================

// DependencyViolation 表示依赖图中的违规。
type DependencyViolation struct {
	Type        string // cycle, redundant, island
	Description string
	Nodes       []string
	DetectedAt  time.Time
}

// DependencyGuard 包装依赖图，提供完整的依赖分析。
// 集成到 ArchitectureGuard 中，用于检测 Pipeline 依赖问题。
type DependencyGuard struct {
	graph      *DependencyGraph
	violations atomic.Value // []DependencyViolation
}

// NewDependencyGuard 创建依赖守卫。
func NewDependencyGuard() *DependencyGuard {
	dg := &DependencyGuard{
		graph: NewDependencyGraph(),
	}
	dg.violations.Store([]DependencyViolation{})
	return dg
}

// AddPipelineDependency 注册一条 Pipeline 依赖。
func (dg *DependencyGuard) AddPipelineDependency(from, to string) {
	dg.graph.AddEdge(from, to)
}

// RemovePipelineDependency 移除一条 Pipeline 依赖。
func (dg *DependencyGuard) RemovePipelineDependency(from, to string) {
	dg.graph.RemoveEdge(from, to)
}

// Analyze 分析依赖图，返回所有违规。
func (dg *DependencyGuard) Analyze() []DependencyViolation {
	var violations []DependencyViolation

	// 循环依赖检测
	cycles := dg.graph.DetectCycles()
	for _, cycle := range cycles {
		violations = append(violations, DependencyViolation{
			Type:        "cycle",
			Description: fmt.Sprintf("circular dependency detected: %v", cycle),
			Nodes:       cycle,
			DetectedAt:  time.Now(),
		})
	}

	// 冗余依赖检测
	redundant := dg.graph.DetectRedundant()
	for _, r := range redundant {
		violations = append(violations, DependencyViolation{
			Type:        "redundant",
			Description: fmt.Sprintf("redundant dependency: %s -> %s (via %s)", r.From, r.To, r.Intermediate),
			Nodes:       []string{r.From, r.To, r.Intermediate},
			DetectedAt:  time.Now(),
		})
	}

	// 孤岛检测
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

// GetViolations 返回最近的违规列表。
func (dg *DependencyGuard) GetViolations() []DependencyViolation {
	v := dg.violations.Load()
	if v == nil {
		return nil
	}
	return v.([]DependencyViolation)
}

// =============================================================================
// SnapshotSummaryStore — 快照摘要存储 (T4.3)
// =============================================================================

// HandoffSnapshotSummary 是 Handoff 时的快照摘要。
// 包含 Pipeline 状态的关键信息，用于快速恢复和审计对比。
type HandoffSnapshotSummary struct {
	PipelineID  string    `json:"pipeline_id"`
	StepCount   int       `json:"step_count"`
	StateHash   string    `json:"state_hash"`   // SHA256 of step sequence
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
	mu       sync.RWMutex
	snapshots map[string][]HandoffSnapshotSummary // pipelineID -> summaries
	latest   map[string]int64                     // pipelineID -> latest version
}

// NewInMemorySnapshotStore 创建内存快照摘要存储。
func NewInMemorySnapshotStore() *InMemorySnapshotStore {
	return &InMemorySnapshotStore{
		snapshots: make(map[string][]HandoffSnapshotSummary),
		latest:    make(map[string]int64),
	}
}

// Save 保存快照摘要。
func (s *InMemorySnapshotStore) Save(summary HandoffSnapshotSummary) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.snapshots[summary.PipelineID] = append(s.snapshots[summary.PipelineID], summary)
	s.latest[summary.PipelineID] = summary.Version
	return nil
}

// Load 加载指定版本的快照。
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

// List 返回指定 Pipeline 的所有快照摘要。
func (s *InMemorySnapshotStore) List(pipelineID string) ([]HandoffSnapshotSummary, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.snapshots[pipelineID], nil
}

// SnapshotDiff 表示两个快照之间的差异。
type SnapshotDiff struct {
	PipelineID  string
	FromVersion int64
	ToVersion   int64
	StepCountDiff int
	HashChanged bool
	AddedSteps   int
	RemovedSteps int
	ModifiedSteps int
}

// Compare 比较两个快照摘要的差异。
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

// =============================================================================
// ArchitectureGuard 性能优化 — 哈希集合查找 (T4.4)
// =============================================================================

// typeSet 用于 O(1) 类型白名单查找，替代线性扫描。
// 预构建的哈希集合，查找 O(1) 而非 O(n)。
type typeSet struct {
	names map[string]bool
}

// newTypeSet 从类型名称列表创建哈希集合。
func newTypeSet(names []string) *typeSet {
	ts := &typeSet{names: make(map[string]bool, len(names))}
	for _, n := range names {
		ts.names[n] = true
	}
	return ts
}

// contains 检查类型是否在白名单中（O(1)）。
func (ts *typeSet) contains(name string) bool {
	return ts.names[name]
}

// buildPrimitiveTypeSet 构建四原语类型的白名单。
// 包含所有标准 Atom/Port/Adapter/Composer 以及 v4.0 新增类型。
func buildPrimitiveTypeSet() *typeSet {
	primitives := []string{
		// 核心类型
		"Atom", "Port", "Adapter", "Composer",
		"Step", "StepFunc", "StepError",
		"ExecutionStep", "CompactExecutionStep", "TraceID", "SpanID", "CompactTraceID",
		// Composer 类型
		"Pipeline", "Branch", "Parallel", "Retry", "Timeout", "Map",
		"FastPipeline", "FastComposer",
		// 观测类型
		"ObservationAdapter", "InMemoryObservationAdapter", "NoOpObservationAdapter",
		"ShardedObservationAdapter", "ObservationPipeline", "ObservationAPI",
		"StepStore", "InMemoryStepStore", "ShardedStepStore", "ShardedIndexedStepStore",
		"StepQuery", "TraceTree", "TraceNode",
		"Aggregator", "AggregatorConfig", "AggregateResult",
		"IncrementalAggregator", "ShardedAggregator",
		"Sampler", "SamplingPolicy", "RateSampler", "ErrorAlwaysSampler", "CompositeSampler",
		// 韧性类型
		"CircuitBreaker", "RateLimiter", "Bulkhead", "Fallback", "ResilienceChain",
		"ResilienceConfig", "ShardedRateLimiter",
		"CircuitState", "CircuitClosed", "CircuitOpen", "CircuitHalfOpen",
		// 安全类型
		"Capability", "CapabilityStore", "InMemoryCapabilityStore",
		"AuditEntry", "AuditTrail", "InMemoryAuditTrail",
		"MerkleAuditChain", "MerkleProof", "MerkleEntry",
		// 事件溯源
		"EventEnvelope", "EventStore", "EventBus", "EventBus_Subscription",
		"Projection", "ProjectionInput", "ProjectionOutput", "ProjectionHandler",
		"PublishResult", "EventHandler",
		// 守卫类型
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
		// 流处理
		"Stream", "StreamConfig", "StreamMap", "StreamFilter", "StreamReduce",
		"Window", "WindowByTime", "Merge", "Split", "Collect", "FromSlice",
		// 幂等
		"IdempotentPort", "IdempotentStore", "InMemoryIdempotentStore",
		"IdempotentRequest", "IdempotentResult",
		// 租户
		"TenantID", "TenantContext", "TenantRequest", "TenantIsolationPort",
		// 事务
		"TransactionContext", "SagaStep", "SagaComposer",
		// 降级
		"DegradationManager", "DegradationMode",
		// 配置
		"Config", "ConfigBuilder", "ConfigHotReload",
		// Handoff
		"HandoffManager", "HandoffStrategy", "HandoffSnapshot", "HandoffCheckpoint",
		// 调度
		"AgentPool", "AgentPoolConfig", "AgentWorker", "AgentTask",
		"Scheduler", "SchedulerComposer", "SchedulerConfig",
		// Schema
		"SchemaRegistry", "SchemaMigration", "SchemaVersion",
		// 架构注册
		"ArchitectureRegistry", "ArchitectureRegistryEntry", "PipelineDescriptor",
		// 端口契约
		"PortContract", "PortContractResult",
		// 熵值度量
		"EntropyMetrics", "EntropyMetricsResult",
		// 快照
		"HandoffSnapshotSummary", "SnapshotSummaryStore", "InMemorySnapshotStore",
		"SnapshotDiff",
		// 性能核心
		"ShardedLock", "AtomicState", "BatchedUUIDGen",
		"StepMetadataPool", "StepSlicePool", "StringBuilderPool", "HashPool",
		"TDigest", "DistributionDriftDetector", "DistributionDriftResult",
		"AnomalyAutoLabeler", "AnomalyLabelType",
		// 分布式韧性
		"GlobalCircuitBreaker", "FederatedDegradationManager",
		"DistributedRateLimiter", "HealthCheckResponse",
	}
	return newTypeSet(primitives)
}

// GlobalPrimitiveTypeSet 全局预构建的类型白名单。
var GlobalPrimitiveTypeSet = buildPrimitiveTypeSet()