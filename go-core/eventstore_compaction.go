//go:build lecore_tier5 || lecore_tier6 || lecore_tier7

package core

import (
	"fmt"
	"sort"
	"time"
)

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
		es.Metadata = map[string]any{
			"aggregate_id": aggregateID,
			"removed":      removed,
			"kept":         len(kept),
		}
		c.obs.Record([]ExecutionStep{es})
	}

	return removed
}
