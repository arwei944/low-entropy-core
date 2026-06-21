//go:build lecore_tier3 || lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

// Package core — 迁移分析类型 (v4.0)
package core

import "time"

// RiskLevel 表示迁移风险级别。
type RiskLevel int

const (
	RiskLow RiskLevel = iota
	RiskMedium
	RiskHigh
	RiskCritical
)

func (r RiskLevel) String() string {
	switch r {
	case RiskLow:
		return "low"
	case RiskMedium:
		return "medium"
	case RiskHigh:
		return "high"
	case RiskCritical:
		return "critical"
	default:
		return "unknown"
	}
}

// ModuleStatus 表示模块迁移状态。
type ModuleStatus int

const (
	ModulePending ModuleStatus = iota
	ModuleInProgress
	ModuleDone
	ModuleFailed
	ModuleSkipped
)

func (s ModuleStatus) String() string {
	switch s {
	case ModulePending:
		return "pending"
	case ModuleInProgress:
		return "in_progress"
	case ModuleDone:
		return "done"
	case ModuleFailed:
		return "failed"
	case ModuleSkipped:
		return "skipped"
	default:
		return "unknown"
	}
}

// ModuleMigration 描述单个模块的迁移信息。
type ModuleMigration struct {
	ModuleName   string
	SourceTier   ComplexityTier
	Dependencies []string
	Files        []string
	ImpactScore  int
	Status       ModuleStatus
}

// MigrationRisk 描述迁移风险点。
type MigrationRisk struct {
	Level       RiskLevel
	Module      string
	Description string
	File        string
	Mitigation  string
}

// RollbackStep 描述回滚步骤。
type RollbackStep struct {
	Order       int
	Module      string
	Description string
	Action      string
}

// MigrationPlan 完整的迁移计划。
type MigrationPlan struct {
	FromTier      ComplexityTier
	ToTier        ComplexityTier
	Modules       []ModuleMigration
	Risks         []MigrationRisk
	EstimatedTime time.Duration
	RollbackPlan  []RollbackStep
}
