//go:build lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

// Package core — 类型白名单 (v4.0)
//
// 包含:
//   - typeSet: O(1) 类型查找哈希集合
//   - GlobalPrimitiveTypeSet: 全局预构建的类型白名单
//
// 所有类型均为线程安全。
package core

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
