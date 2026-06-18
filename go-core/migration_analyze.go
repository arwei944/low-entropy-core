//go:build lecore_tier3 || lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

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

// MigrateAnalyze 扫描项目并生成迁移计划。
func MigrateAnalyze(root string, fromTier, toTier ComplexityTier) *MigrationPlan {
	plan := &MigrationPlan{
		FromTier: fromTier,
		ToTier:   toTier,
	}

	plan.Modules = identifyModulesForMigration(fromTier, toTier)
	plan.Modules = sortModulesByDependency(plan.Modules)
	plan.Risks = assessMigrationRisks(root, plan.Modules)
	plan.EstimatedTime = estimateMigrationTime(plan.Modules)
	plan.RollbackPlan = generateRollbackPlan(plan.Modules)

	return plan
}

func identifyModulesForMigration(fromTier, toTier ComplexityTier) []ModuleMigration {
	moduleDefs := []struct {
		name      string
		tier      ComplexityTier
		deps      []string
		baseFiles []string
		impact    int
	}{
		{"degradation", TierL1, nil, []string{"degradation.go"}, 10},
		{"fastpath", TierL1, nil, []string{"fastpath.go"}, 10},
		{"eventstore", TierL2, nil, []string{"eventstore.go"}, 30},
		{"eventbus", TierL2, []string{"eventstore"}, []string{"eventbus.go"}, 30},
		{"config", TierL2, nil, []string{"config.go"}, 20},
		{"patterns_resilience", TierL2, nil, []string{"patterns_resilience.go"}, 20},
		{"port_contract", TierL2, nil, []string{"port_contract.go"}, 15},
		{"architecture_registry", TierL2, nil, []string{"architecture_registry.go"}, 15},
		{"eventstore_persistent", TierL3, []string{"eventstore"}, []string{"eventstore_persistent.go"}, 40},
		{"eventbus_persistent", TierL3, []string{"eventbus", "eventstore_persistent"}, []string{"eventbus_persistent.go"}, 40},
		{"storage_fs", TierL3, nil, []string{"storage_fs.go"}, 25},
		{"security", TierL3, nil, []string{"security.go"}, 35},
		{"transaction", TierL3, nil, []string{"transaction.go"}, 35},
		{"handoff", TierL3, nil, []string{"handoff.go"}, 25},
		{"handoff_persistence", TierL3, []string{"handoff"}, []string{"handoff_persistence.go"}, 20},
		{"schema", TierL3, nil, []string{"schema.go"}, 25},
		{"scheduler", TierL3, nil, []string{"scheduler.go"}, 30},
		{"guardian", TierL4, []string{"eventstore", "observation_store"}, []string{
			"guardian_decision.go", "guardian_dependency.go",
			"guardian_entropy.go", "guardian_transparency.go", "guardian_static.go"}, 60},
		{"observation_pipeline", TierL4, nil, []string{"observation_pipeline.go"}, 40},
		{"observation_store", TierL4, nil, []string{"observation_store.go"}, 35},
		{"agent_submit", TierL4, []string{"guardian", "handoff"}, []string{"agent_submit.go", "agent_runner.go"}, 55},
		{"projection", TierL4, []string{"eventstore_persistent"}, []string{"projection.go"}, 30},
		{"idempotent", TierL4, nil, []string{"idempotent.go"}, 25},
		{"tenant", TierL4, nil, []string{"tenant.go"}, 25},
		{"patterns_distributed", TierL5, []string{"eventbus"}, []string{"patterns_distributed.go"}, 45},
		{"eventstore_upgrade", TierL5, []string{"eventstore_persistent"}, []string{"eventstore_upgrade.go"}, 35},
		{"remote_composer", TierL5, []string{"eventbus"}, []string{"remote_composer.go"}, 30},
		{"app", TierL5, []string{"config", "eventstore", "eventbus"}, []string{"app.go"}, 50},
	}

	var result []ModuleMigration
	for _, def := range moduleDefs {
		if def.tier > fromTier && def.tier <= toTier {
			result = append(result, ModuleMigration{
				ModuleName:   def.name,
				SourceTier:   def.tier,
				Dependencies: def.deps,
				Files:        def.baseFiles,
				ImpactScore:  def.impact,
				Status:       ModulePending,
			})
		}
	}
	return result
}

func sortModulesByDependency(modules []ModuleMigration) []ModuleMigration {
	depths := make(map[string]int)
	for _, m := range modules {
		depths[m.ModuleName] = len(m.Dependencies)
	}

	sorted := make([]ModuleMigration, len(modules))
	copy(sorted, modules)
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if depths[sorted[i].ModuleName] > depths[sorted[j].ModuleName] {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}
	return sorted
}

func assessMigrationRisks(root string, modules []ModuleMigration) []MigrationRisk {
	var risks []MigrationRisk

	for _, m := range modules {
		if m.ImpactScore >= 50 {
			risks = append(risks, MigrationRisk{
				Level:       RiskHigh,
				Module:      m.ModuleName,
				Description: fmt.Sprintf("Module %s has high impact score (%d) and may require significant refactoring.", m.ModuleName, m.ImpactScore),
				Mitigation:  "Plan incremental rollout with feature flags and comprehensive testing.",
			})
		}

		if len(m.Dependencies) > 0 {
			risks = append(risks, MigrationRisk{
				Level:       RiskMedium,
				Module:      m.ModuleName,
				Description: fmt.Sprintf("Module %s depends on %v. Ensure dependencies are migrated first.", m.ModuleName, m.Dependencies),
				Mitigation:  "Migrate dependencies first, then verify integration.",
			})
		}
	}

	// 扫描项目中的潜在风险文件
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		text := string(content)
		for _, m := range modules {
			for _, f := range m.Files {
				base := strings.TrimSuffix(f, ".go")
				if strings.Contains(text, base) {
					risks = append(risks, MigrationRisk{
						Level:       RiskLow,
						Module:      m.ModuleName,
						Description: fmt.Sprintf("File %s references %s from module %s.", path, base, m.ModuleName),
						File:        path,
						Mitigation:  "Review usage and ensure compatibility after migration.",
					})
				}
			}
		}
		return nil
	})

	return risks
}

func estimateMigrationTime(modules []ModuleMigration) time.Duration {
	totalMinutes := 0
	for _, m := range modules {
		totalMinutes += 15 + m.ImpactScore/2
	}
	return time.Duration(totalMinutes) * time.Minute
}

func generateRollbackPlan(modules []ModuleMigration) []RollbackStep {
	var steps []RollbackStep
	for i := len(modules) - 1; i >= 0; i-- {
		m := modules[i]
		steps = append(steps, RollbackStep{
			Order:       len(modules) - i,
			Module:      m.ModuleName,
			Description: fmt.Sprintf("Rollback module %s migration.", m.ModuleName),
			Action:      fmt.Sprintf("Remove %s build tag and revert to original tier.", m.ModuleName),
		})
	}
	return steps
}

// EstimateTime 返回预估迁移时间。
func (p *MigrationPlan) EstimateTime() time.Duration {
	return p.EstimatedTime
}

// GenerateReport 生成迁移计划的可读报告。
func (p *MigrationPlan) GenerateReport() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("=== Migration Plan: %s → %s ===\n\n", p.FromTier, p.ToTier))
	sb.WriteString(fmt.Sprintf("Modules to migrate: %d\n", len(p.Modules)))
	sb.WriteString(fmt.Sprintf("Estimated time: %s\n", p.EstimatedTime.Round(time.Minute)))
	sb.WriteString(fmt.Sprintf("Risks identified: %d\n\n", len(p.Risks)))

	sb.WriteString("--- Modules (in dependency order) ---\n")
	for i, m := range p.Modules {
		sb.WriteString(fmt.Sprintf("%d. %s (tier: %s, impact: %d, deps: %v)\n",
			i+1, m.ModuleName, m.SourceTier, m.ImpactScore, m.Dependencies))
	}

	if len(p.Risks) > 0 {
		sb.WriteString("\n--- Risks ---\n")
		for _, r := range p.Risks {
			sb.WriteString(fmt.Sprintf("[%s] %s: %s\n  Mitigation: %s\n",
				r.Level, r.Module, r.Description, r.Mitigation))
		}
	}

	sb.WriteString("\n--- Rollback Plan ---\n")
	for _, r := range p.RollbackPlan {
		sb.WriteString(fmt.Sprintf("%d. %s: %s\n", r.Order, r.Module, r.Description))
	}

	return sb.String()
}

// ModuleCount 返回需要迁移的模块数。
func (p *MigrationPlan) ModuleCount() int {
	return len(p.Modules)
}

// RiskCount 返回风险点数量。
func (p *MigrationPlan) RiskCount() int {
	return len(p.Risks)
}