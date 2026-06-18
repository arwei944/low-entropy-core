//go:build lecore_tier2 || lecore_tier3 || lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

package core

import (
	"fmt"
	"sync"
)

// TransitionPhase 表示迁移过程中的一个阶段。
type TransitionPhase struct {
	Name     string
	Modules  []string
	Validate func() error
	Rollback func() error
}

// TierTransitionManager 管理从 fromTier 到 toTier 的平滑过渡。
type TierTransitionManager struct {
	mu       sync.Mutex
	fromTier ComplexityTier
	toTier   ComplexityTier
	phases   []TransitionPhase
	current  int
	flags    map[string]bool
	done     bool
}

// NewTierTransition 创建 tier 过渡管理器。
// 自动生成从 fromTier 到 toTier 的过渡阶段。
func NewTierTransition(fromTier, toTier ComplexityTier) *TierTransitionManager {
	m := &TierTransitionManager{
		fromTier: fromTier,
		toTier:   toTier,
		current:  -1,
		flags:    make(map[string]bool),
		done:     false,
	}
	m.phases = m.generatePhases()
	return m
}

// generatePhases 根据 fromTier 和 toTier 生成过渡阶段列表。
func (m *TierTransitionManager) generatePhases() []TransitionPhase {
	type moduleInfo struct {
		name string
		tier ComplexityTier
	}

	allModules := []moduleInfo{
		{"degradation", TierL1},
		{"fastpath", TierL1},
		{"eventstore", TierL2},
		{"eventbus", TierL2},
		{"config", TierL2},
		{"patterns_resilience", TierL2},
		{"port_contract", TierL2},
		{"architecture_registry", TierL2},
		{"eventstore_persistent", TierL3},
		{"eventbus_persistent", TierL3},
		{"storage_fs", TierL3},
		{"security", TierL3},
		{"transaction", TierL3},
		{"handoff", TierL3},
		{"handoff_persistence", TierL3},
		{"schema", TierL3},
		{"scheduler", TierL3},
		{"guardian", TierL4},
		{"observation_pipeline", TierL4},
		{"observation_store", TierL4},
		{"agent_submit", TierL4},
		{"projection", TierL4},
		{"idempotent", TierL4},
		{"tenant", TierL4},
		{"patterns_distributed", TierL5},
		{"eventstore_upgrade", TierL5},
		{"remote_composer", TierL5},
		{"app", TierL5},
	}

	var phases []TransitionPhase
	for _, mod := range allModules {
		if mod.tier > m.fromTier && mod.tier <= m.toTier {
			phase := TransitionPhase{
				Name:    fmt.Sprintf("enable_%s", mod.name),
				Modules: []string{mod.name},
			}
			phases = append(phases, phase)
		}
	}
	return phases
}

// Advance 推进到下一个迁移阶段。
func (m *TierTransitionManager) Advance() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.done {
		return fmt.Errorf("transition already complete")
	}

	next := m.current + 1
	if next >= len(m.phases) {
		m.done = true
		return nil
	}

	phase := m.phases[next]
	if phase.Validate != nil {
		if err := phase.Validate(); err != nil {
			return fmt.Errorf("phase %s validation failed: %w", phase.Name, err)
		}
	}

	for _, mod := range phase.Modules {
		m.flags[mod] = true
	}
	m.current = next

	if m.current == len(m.phases)-1 {
		m.done = true
	}

	return nil
}

// Rollback 回滚到上一个迁移阶段。
func (m *TierTransitionManager) Rollback() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.current < 0 {
		return fmt.Errorf("no phase to rollback")
	}

	phase := m.phases[m.current]
	if phase.Rollback != nil {
		if err := phase.Rollback(); err != nil {
			return fmt.Errorf("rollback phase %s failed: %w", phase.Name, err)
		}
	}

	for _, mod := range phase.Modules {
		delete(m.flags, mod)
	}
	m.current--
	m.done = false

	return nil
}

// AdvanceAll 一次性推进所有阶段（跳过验证）。
func (m *TierTransitionManager) AdvanceAll() error {
	for !m.done {
		if err := m.Advance(); err != nil {
			return err
		}
	}
	return nil
}

// Progress 返回迁移进度 (0.0 - 1.0)。
func (m *TierTransitionManager) Progress() float64 {
	m.mu.Lock()
	defer m.mu.Unlock()

	total := len(m.phases)
	if total == 0 {
		return 1.0
	}
	return float64(m.current+1) / float64(total)
}

// IsDone 返回 true 表示迁移已完成。
func (m *TierTransitionManager) IsDone() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.done
}

// IsEnabled 检查某个模块是否已启用。
func (m *TierTransitionManager) IsEnabled(module string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.flags[module]
}

// EnabledModules 返回已启用的模块列表。
func (m *TierTransitionManager) EnabledModules() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []string
	for mod := range m.flags {
		result = append(result, mod)
	}
	return result
}

// PhaseCount 返回过渡阶段总数。
func (m *TierTransitionManager) PhaseCount() int {
	return len(m.phases)
}

// CurrentPhase 返回当前阶段索引（-1 表示尚未开始）。
func (m *TierTransitionManager) CurrentPhase() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.current
}

// FromTier 返回起始 tier。
func (m *TierTransitionManager) FromTier() ComplexityTier { return m.fromTier }

// ToTier 返回目标 tier。
func (m *TierTransitionManager) ToTier() ComplexityTier { return m.toTier }