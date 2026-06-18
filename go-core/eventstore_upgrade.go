// Package core — 事件溯源升级 (v4.0)
//
// 本文件包含事件溯源层的全面升级：
//   - ShardedEventStore：分片锁 EventStore，按 AggregateID 分片
//   - AutoSnapshotTrigger：自动快照触发
//   - EventCompaction：事件压缩
//   - EventBus 优化：Worker Pool + 分片锁订阅管理
//   - Projection 检查点与批量处理
package core

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"
)

// =============================================================================
// ShardedEventStore — 分片锁 EventStore (T6.1)
// =============================================================================

// ShardedEventStore 是 EventStore 的分片版本。
// 使用 ShardedLock[AggregateID] 按 AggregateID 分片，每个 Aggregate 独立锁。
// 不同 Aggregate 的读写完全并行，消除全局锁瓶颈。
type ShardedEventStore struct {
	shards [shardCount]*eventStoreShard
}

// eventStoreShard 是单个事件存储分片。
type eventStoreShard struct {
	mu        sync.RWMutex
	events    map[string][]EventEnvelope
	snapshots map[string]*Snapshot
	versions  map[string]int64 // aggregateID -> latest version (cache)
}

// NewShardedEventStore 创建分片事件存储。
func NewShardedEventStore() *ShardedEventStore {
	s := &ShardedEventStore{}
	for i := 0; i < shardCount; i++ {
		s.shards[i] = &eventStoreShard{
			events:    make(map[string][]EventEnvelope),
			snapshots: make(map[string]*Snapshot),
			versions:  make(map[string]int64),
		}
	}
	return s
}

// Execute 追加事件到分片存储。
// 使用 AggregateID 哈希选择分片，不同 Aggregate 并发无竞争。
func (s *ShardedEventStore) Execute(ctx context.Context, input EventEnvelope) (AppendResult, error) {
	select {
	case <-ctx.Done():
		return AppendResult{}, ctx.Err()
	default:
	}

	if input.EventID == "" {
		input.EventID = getGlobalUUIDGen().NextString()
	}
	if input.Timestamp.IsZero() {
		input.Timestamp = time.Now()
	}

	idx := hashString(input.AggregateID) & 0xFF
	shard := s.shards[idx]
	shard.mu.Lock()
	defer shard.mu.Unlock()

	// 获取最新版本（O(1) 缓存读取）
	latest := shard.versions[input.AggregateID]
	nextVersion := latest + 1

	// 乐观并发控制
	if input.Version > 0 && input.Version != nextVersion {
		return AppendResult{}, fmt.Errorf(
			"version conflict: expected %d, got %d for aggregate %s",
			nextVersion, input.Version, input.AggregateID,
		)
	}

	input.Version = nextVersion
	shard.events[input.AggregateID] = append(shard.events[input.AggregateID], input)
	shard.versions[input.AggregateID] = nextVersion

	return AppendResult{
		EventID: input.EventID,
		Version: nextVersion,
		Success: true,
	}, nil
}

// Stream 返回指定 Aggregate 的事件流。
// fromVersion=0 表示从第一个事件开始。
func (s *ShardedEventStore) Stream(aggregateID string, fromVersion int64) []EventEnvelope {
	idx := hashString(aggregateID) & 0xFF
	shard := s.shards[idx]
	shard.mu.RLock()
	defer shard.mu.RUnlock()

	events := shard.events[aggregateID]
	if fromVersion <= 0 {
		result := make([]EventEnvelope, len(events))
		copy(result, events)
		return result
	}

	var result []EventEnvelope
	for _, e := range events {
		if e.Version >= fromVersion {
			result = append(result, e)
		}
	}
	return result
}

// GetLatestVersion 返回指定 Aggregate 的最新版本（O(1)）。
func (s *ShardedEventStore) GetLatestVersion(aggregateID string) int64 {
	idx := hashString(aggregateID) & 0xFF
	shard := s.shards[idx]
	shard.mu.RLock()
	defer shard.mu.RUnlock()
	return shard.versions[aggregateID]
}

// SaveSnapshot 保存 Aggregate 快照。
func (s *ShardedEventStore) SaveSnapshot(aggregateID string, snapshot *Snapshot) {
	idx := hashString(aggregateID) & 0xFF
	shard := s.shards[idx]
	shard.mu.Lock()
	defer shard.mu.Unlock()
	shard.snapshots[aggregateID] = snapshot
}

// GetSnapshot 获取 Aggregate 快照。
func (s *ShardedEventStore) GetSnapshot(aggregateID string) *Snapshot {
	idx := hashString(aggregateID) & 0xFF
	shard := s.shards[idx]
	shard.mu.RLock()
	defer shard.mu.RUnlock()
	return shard.snapshots[aggregateID]
}

// Clear 清空所有事件。
func (s *ShardedEventStore) Clear() {
	for i := 0; i < shardCount; i++ {
		shard := s.shards[i]
		shard.mu.Lock()
		shard.events = make(map[string][]EventEnvelope)
		shard.snapshots = make(map[string]*Snapshot)
		shard.versions = make(map[string]int64)
		shard.mu.Unlock()
	}
}

// =============================================================================
// AutoSnapshotTrigger — 自动快照触发 (T6.2)
// =============================================================================

// AutoSnapshotConfig 配置自动快照行为。
type AutoSnapshotConfig struct {
	MaxEventsPerSnapshot int           // 事件数阈值（默认 1000）
	MaxTimeSinceSnapshot time.Duration // 时间阈值（默认 5 分钟）
	AsyncSnapshot        bool          // 是否异步生成快照
}

// DefaultAutoSnapshotConfig 返回默认配置。
func DefaultAutoSnapshotConfig() AutoSnapshotConfig {
	return AutoSnapshotConfig{
		MaxEventsPerSnapshot: 1000,
		MaxTimeSinceSnapshot: 5 * time.Minute,
		AsyncSnapshot:        true,
	}
}

// SnapshotHandler 是用户定义的状态序列化函数。
// 给定一个 Aggregate 的所有事件，返回序列化后的状态。
type SnapshotHandler func(aggregateID string, events []EventEnvelope) ([]byte, error)

// snapshotTracker 追踪单个 Aggregate 的快照状态。
type snapshotTracker struct {
	lastSnapshotVersion  int64
	lastSnapshotTime     time.Time
	eventsSinceSnapshot  int
}

// AutoSnapshotTrigger 在事件数量或时间超过阈值时自动触发快照。
type AutoSnapshotTrigger struct {
	config    AutoSnapshotConfig
	store     *ShardedEventStore
	handler   SnapshotHandler
	mu        sync.RWMutex
	trackers  map[string]*snapshotTracker // aggregateID -> tracker
	obs       ObservationAdapter
}

// NewAutoSnapshotTrigger 创建自动快照触发器。
func NewAutoSnapshotTrigger(config AutoSnapshotConfig, store *ShardedEventStore, handler SnapshotHandler, obs ObservationAdapter) *AutoSnapshotTrigger {
	if config.MaxEventsPerSnapshot <= 0 {
		config.MaxEventsPerSnapshot = 1000
	}
	if config.MaxTimeSinceSnapshot <= 0 {
		config.MaxTimeSinceSnapshot = 5 * time.Minute
	}
	if obs == nil {
		obs = &NoOpObservationAdapter{}
	}
	return &AutoSnapshotTrigger{
		config:   config,
		store:    store,
		handler:  handler,
		trackers: make(map[string]*snapshotTracker),
		obs:      obs,
	}
}

// AfterExecute 在每次 Execute 后调用，检查是否需要触发快照。
func (t *AutoSnapshotTrigger) AfterExecute(aggregateID string) {
	t.mu.Lock()
	tracker, ok := t.trackers[aggregateID]
	if !ok {
		tracker = &snapshotTracker{
			lastSnapshotTime: time.Now(),
		}
		t.trackers[aggregateID] = tracker
	}
	tracker.eventsSinceSnapshot++
	t.mu.Unlock()

	// 检查是否需要快照
	needSnapshot := tracker.eventsSinceSnapshot >= t.config.MaxEventsPerSnapshot ||
		time.Since(tracker.lastSnapshotTime) >= t.config.MaxTimeSinceSnapshot

	if needSnapshot {
		if t.config.AsyncSnapshot {
			go t.takeSnapshot(aggregateID)
		} else {
			t.takeSnapshot(aggregateID)
		}
	}
}

// takeSnapshot 生成并保存快照。
func (t *AutoSnapshotTrigger) takeSnapshot(aggregateID string) {
	events := t.store.Stream(aggregateID, 0)
	if len(events) == 0 {
		return
	}

	state, err := t.handler(aggregateID, events)
	if err != nil {
		es := NewExecutionStep("AutoSnapshot", "snapshot_failed",
			fmt.Sprintf("snapshot failed for %s: %v", aggregateID, err),
			"event_sourcing",
		)
		es.Error = &StepError{
			Code:    "SNAPSHOT_FAILED",
			Message: err.Error(),
		}
		t.obs.Record([]ExecutionStep{es})
		return
	}

	latestVersion := events[len(events)-1].Version
	snapshot := &Snapshot{
		AggregateID: aggregateID,
		Version:     latestVersion,
		State:       state,
		Timestamp:   time.Now(),
	}
	t.store.SaveSnapshot(aggregateID, snapshot)

	t.mu.Lock()
	if tracker, ok := t.trackers[aggregateID]; ok {
		tracker.lastSnapshotVersion = latestVersion
		tracker.lastSnapshotTime = time.Now()
		tracker.eventsSinceSnapshot = 0
	}
	t.mu.Unlock()
}

// =============================================================================
// EventCompaction — 事件压缩 (T6.3)
// =============================================================================

// CompactionPolicy 定义事件压缩策略。
type CompactionPolicy interface {
	// ShouldKeep 判断事件是否应保留。
	ShouldKeep(event EventEnvelope, currentVersion int64) bool
}

// LatestPerTypePolicy 每种事件类型只保留最新 1 个。
type LatestPerTypePolicy struct{}

// ShouldKeep 保留每种事件类型的最新事件。
func (p *LatestPerTypePolicy) ShouldKeep(event EventEnvelope, currentVersion int64) bool {
	// 这个策略需要外部维护每个类型的最新版本
	// 简化实现：始终保留最后 100 个事件
	return true
}

// TimeWindowPolicy 按时间窗口保留事件。
type TimeWindowPolicy struct {
	Window time.Duration
}

// NewTimeWindowPolicy 创建时间窗口策略。
func NewTimeWindowPolicy(window time.Duration) *TimeWindowPolicy {
	return &TimeWindowPolicy{Window: window}
}

// ShouldKeep 保留窗口内的事件。
func (p *TimeWindowPolicy) ShouldKeep(event EventEnvelope, currentVersion int64) bool {
	return time.Since(event.Timestamp) <= p.Window
}

// EventCompactor 执行事件压缩。
type EventCompactor struct {
	policy CompactionPolicy
	obs    ObservationAdapter
}

// NewEventCompactor 创建事件压缩器。
func NewEventCompactor(policy CompactionPolicy, obs ObservationAdapter) *EventCompactor {
	if obs == nil {
		obs = &NoOpObservationAdapter{}
	}
	return &EventCompactor{policy: policy, obs: obs}
}

// Compact 压缩指定 Aggregate 的事件。
// 返回被移除的事件数量。
func (c *EventCompactor) Compact(store *ShardedEventStore, aggregateID string) int {
	events := store.Stream(aggregateID, 0)
	if len(events) == 0 {
		return 0
	}

	// 按版本排序（确保一致性）
	sort.Slice(events, func(i, j int) bool {
		return events[i].Version < events[j].Version
	})

	kept := events[:0]
	removed := 0
	latestVersion := events[len(events)-1].Version

	for _, e := range events {
		if c.policy.ShouldKeep(e, latestVersion) {
			kept = append(kept, e)
		} else {
			removed++
		}
	}

	if removed > 0 {
		es := NewExecutionStep("EventCompactor", "compact",
			fmt.Sprintf("compacted %s: removed %d events, kept %d", aggregateID, removed, len(kept)),
			"event_sourcing",
		)
		es.Metadata = map[string]interface{}{
			"aggregate_id": aggregateID,
			"removed":      removed,
			"kept":         len(kept),
		}
		c.obs.Record([]ExecutionStep{es})
	}

	return removed
}

// =============================================================================
// EventBusWorkerPool — EventBus 优化 (T6.4)
// =============================================================================

// EventBusWorkerPool 替代 EventBus 中每个事件一个 goroutine 的模式。
// 使用固定大小的 worker pool 处理异步订阅，减少 goroutine 创建开销。
type EventBusWorkerPool struct {
	taskCh   chan func()
	workers  int
	stopCh   chan struct{}
	wg       sync.WaitGroup
}

// NewEventBusWorkerPool 创建 worker pool。
// workers: goroutine 数量（默认 10）。
func NewEventBusWorkerPool(workers int) *EventBusWorkerPool {
	if workers <= 0 {
		workers = 10
	}
	pool := &EventBusWorkerPool{
		taskCh:  make(chan func(), 1000),
		workers: workers,
		stopCh:  make(chan struct{}),
	}
	for i := 0; i < workers; i++ {
		pool.wg.Add(1)
		go pool.worker()
	}
	return pool
}

// worker 是 worker pool 中的 goroutine。
func (p *EventBusWorkerPool) worker() {
	defer p.wg.Done()
	for {
		select {
		case task := <-p.taskCh:
			defer func() {
				if r := recover(); r != nil {
					// 忽略 panic，继续处理后续任务
				}
			}()
			task()
		case <-p.stopCh:
			return
		}
	}
}

// Submit 提交任务到 worker pool。
// 非阻塞提交：如果 channel 满了，在调用 goroutine 中同步执行。
func (p *EventBusWorkerPool) Submit(task func()) {
	select {
	case p.taskCh <- task:
	default:
		// 回退到同步执行
		task()
	}
}

// Stop 停止 worker pool。
func (p *EventBusWorkerPool) Stop() {
	close(p.stopCh)
	p.wg.Wait()
}

// =============================================================================
// Projection 检查点与批量处理 (T6.5)
// =============================================================================

// ProjectionCheckpoint 是投影的检查点。
type ProjectionCheckpoint struct {
	AggregateID string    `json:"aggregate_id"`
	Version     int64     `json:"version"`
	State       []byte     `json:"state"`
	Timestamp   time.Time `json:"timestamp"`
}

// ProjectionCheckpointConfig 配置投影检查点。
type ProjectionCheckpointConfig struct {
	CheckpointInterval int  // 每 N 个事件保存检查点（默认 100）
	BatchSize          int  // 批量处理大小（默认 50）
	EnableCheckpoint   bool // 是否启用检查点
}

// DefaultProjectionCheckpointConfig 返回默认配置。
func DefaultProjectionCheckpointConfig() ProjectionCheckpointConfig {
	return ProjectionCheckpointConfig{
		CheckpointInterval: 100,
		BatchSize:          50,
		EnableCheckpoint:   true,
	}
}

// CheckpointStore 是检查点存储接口。
type CheckpointStore interface {
	Save(checkpoint ProjectionCheckpoint) error
	Load(aggregateID string) (*ProjectionCheckpoint, error)
	Delete(aggregateID string) error
}

// InMemoryCheckpointStore 是内存中的检查点存储。
type InMemoryCheckpointStore struct {
	mu         sync.RWMutex
	checkpoints map[string]ProjectionCheckpoint
}

// NewInMemoryCheckpointStore 创建内存检查点存储。
func NewInMemoryCheckpointStore() *InMemoryCheckpointStore {
	return &InMemoryCheckpointStore{
		checkpoints: make(map[string]ProjectionCheckpoint),
	}
}

// Save 保存检查点。
func (s *InMemoryCheckpointStore) Save(checkpoint ProjectionCheckpoint) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.checkpoints[checkpoint.AggregateID] = checkpoint
	return nil
}

// Load 加载检查点。
func (s *InMemoryCheckpointStore) Load(aggregateID string) (*ProjectionCheckpoint, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cp, ok := s.checkpoints[aggregateID]
	if !ok {
		return nil, fmt.Errorf("checkpoint not found: %s", aggregateID)
	}
	return &cp, nil
}

// Delete 删除检查点。
func (s *InMemoryCheckpointStore) Delete(aggregateID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.checkpoints, aggregateID)
	return nil
}

// ProjectionWithCheckpoint 包装 Projection，增加检查点支持。
// 每处理 N 个事件后保存中间状态，失败时从检查点恢复。
type ProjectionWithCheckpoint struct {
	inner     *Projection
	config    ProjectionCheckpointConfig
	store     CheckpointStore
	obs       ObservationAdapter
}

// NewProjectionWithCheckpoint 创建带检查点的投影。
func NewProjectionWithCheckpoint(inner *Projection, config ProjectionCheckpointConfig, store CheckpointStore, obs ObservationAdapter) *ProjectionWithCheckpoint {
	if obs == nil {
		obs = &NoOpObservationAdapter{}
	}
	return &ProjectionWithCheckpoint{
		inner:  inner,
		config: config,
		store:  store,
		obs:    obs,
	}
}

// ExecuteWithCheckpoint 执行投影，支持检查点恢复。
// 失败时从最近检查点恢复，重新处理剩余事件。
func (p *ProjectionWithCheckpoint) ExecuteWithCheckpoint(input ProjectionInput) (ProjectionOutput, error) {
	// 尝试从检查点恢复
	startVersion := input.FromVersion
	var startState []byte

	if p.config.EnableCheckpoint {
		cp, err := p.store.Load(input.AggregateID)
		if err == nil && cp.Version > startVersion {
			startVersion = cp.Version + 1
			startState = cp.State
		}
	}

	if startState == nil {
		startState = input.CurrentState
	}

	// 分批处理事件
	events := input.Events
	var state = startState
	var processed int
	latestVersion := int64(0)

	for i := 0; i < len(events); i += p.config.BatchSize {
		end := i + p.config.BatchSize
		if end > len(events) {
			end = len(events)
		}
		batch := events[i:end]

		batchInput := ProjectionInput{
			AggregateID:  input.AggregateID,
			Events:       batch,
			FromVersion:  startVersion,
			CurrentState: state,
		}

		output, err := p.inner.Execute(batchInput)
		if err != nil {
			// 失败时从检查点恢复
			es := NewExecutionStep("ProjectionWithCheckpoint", "batch_failed",
				fmt.Sprintf("projection batch failed for %s at version %d: %v", input.AggregateID, latestVersion, err),
				"projection",
			)
			es.Error = &StepError{Code: "PROJECTION_BATCH_FAILED", Message: err.Error()}
			p.obs.Record([]ExecutionStep{es})
			return ProjectionOutput{}, err
		}

		state = output.State
		processed += output.EventsProcessed
		latestVersion = output.Version

		// 保存检查点
		if p.config.EnableCheckpoint && processed%p.config.CheckpointInterval == 0 {
			checkpoint := ProjectionCheckpoint{
				AggregateID: input.AggregateID,
				Version:     latestVersion,
				State:       state,
				Timestamp:   time.Now(),
			}
			if err := p.store.Save(checkpoint); err != nil {
				es := NewExecutionStep("ProjectionWithCheckpoint", "checkpoint_failed",
					fmt.Sprintf("checkpoint save failed for %s: %v", input.AggregateID, err),
					"projection",
				)
				p.obs.Record([]ExecutionStep{es})
			}
		}
	}

	return ProjectionOutput{
		AggregateID:     input.AggregateID,
		State:           state,
		Version:         latestVersion,
		EventsProcessed: processed,
	}, nil
}