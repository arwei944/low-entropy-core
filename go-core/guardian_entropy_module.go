//go:build lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

// SECTION 2: ModuleEntropyTracker — 模块级熵值追踪
package core

import (
	"sync"
	"sync/atomic"
	"time"
)

// ============================================================================
// SECTION 2: ModuleEntropyTracker — 模块级熵值追踪
// ============================================================================

// ModuleID 唯一标识一个模块。
type ModuleID string

// ModuleEntropyState 是单个模块的熵值状态。
type ModuleEntropyState struct {
	ModuleID      ModuleID
	EntropyScore  float64
	PreviousScore float64
	HasPrevious   bool
	LastUpdated   time.Time
	EntropyLevel  EntropyLevel
}

// ModuleEntropyTracker 按模块独立追踪熵值。
// 使用 ShardedLock[ModuleID] 分片，不同模块的更新完全并行。
type ModuleEntropyTracker struct {
	shards      [shardCount]*moduleEntropyShard
	moduleCount atomic.Int64
}

type moduleEntropyShard struct {
	mu      sync.RWMutex
	modules map[ModuleID]*ModuleEntropyState
}

func NewModuleEntropyTracker() *ModuleEntropyTracker {
	t := &ModuleEntropyTracker{}
	for i := 0; i < shardCount; i++ {
		t.shards[i] = &moduleEntropyShard{
			modules: make(map[ModuleID]*ModuleEntropyState),
		}
	}
	return t
}

func (t *ModuleEntropyTracker) RegisterModule(moduleID ModuleID) {
	idx := hashString(string(moduleID)) & 0xFF
	shard := t.shards[idx]
	shard.mu.Lock()
	if _, exists := shard.modules[moduleID]; !exists {
		shard.modules[moduleID] = &ModuleEntropyState{
			ModuleID:     moduleID,
			EntropyLevel: EntropyOK,
		}
		t.moduleCount.Add(1)
	}
	shard.mu.Unlock()
}

func (t *ModuleEntropyTracker) UnregisterModule(moduleID ModuleID) {
	idx := hashString(string(moduleID)) & 0xFF
	shard := t.shards[idx]
	shard.mu.Lock()
	if _, exists := shard.modules[moduleID]; exists {
		delete(shard.modules, moduleID)
		t.moduleCount.Add(-1)
	}
	shard.mu.Unlock()
}

func (t *ModuleEntropyTracker) UpdateEntropy(moduleID ModuleID, score float64) *ModuleEntropyState {
	idx := hashString(string(moduleID)) & 0xFF
	shard := t.shards[idx]
	shard.mu.Lock()
	defer shard.mu.Unlock()

	state, ok := shard.modules[moduleID]
	if !ok {
		return nil
	}

	state.PreviousScore = state.EntropyScore
	state.HasPrevious = true
	state.EntropyScore = score
	state.LastUpdated = time.Now()

	switch {
	case score >= 100:
		state.EntropyLevel = EntropyRed
	case score >= 50:
		state.EntropyLevel = EntropyOrange
	case score >= 20:
		state.EntropyLevel = EntropyYellow
	default:
		state.EntropyLevel = EntropyOK
	}

	return state
}

func (t *ModuleEntropyTracker) GetModuleEntropy(moduleID ModuleID) *ModuleEntropyState {
	idx := hashString(string(moduleID)) & 0xFF
	shard := t.shards[idx]
	shard.mu.RLock()
	defer shard.mu.RUnlock()
	return shard.modules[moduleID]
}

func (t *ModuleEntropyTracker) GetAllModules() []*ModuleEntropyState {
	var results []*ModuleEntropyState
	for i := 0; i < shardCount; i++ {
		shard := t.shards[i]
		shard.mu.RLock()
		for _, state := range shard.modules {
			results = append(results, state)
		}
		shard.mu.RUnlock()
	}
	return results
}

func (t *ModuleEntropyTracker) ModuleCount() int {
	return int(t.moduleCount.Load())
}
