//go:build lecore_tier5 || lecore_tier6 || lecore_tier7

package core

import (
	"fmt"
	"sync"
	"time"
)

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
