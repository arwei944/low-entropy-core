//go:build lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

// Package core — Handoff 快照摘要存储 (v4.0)
//
// 包含:
//   - SnapshotSummaryStore: 快照摘要存储接口
//   - InMemorySnapshotStore: 内存快照摘要存储
//   - SnapshotDiff: 快照差异
//   - HandoffSnapshotSummary: Handoff 快照摘要
//
// 所有类型均为线程安全。
package core

import (
	"fmt"
	"sync"
	"time"
)

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
