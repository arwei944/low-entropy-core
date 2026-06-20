//go:build lecore_tier5 || lecore_tier6 || lecore_tier7

package core

import (
	"fmt"
	"sync"
	"time"
)

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
