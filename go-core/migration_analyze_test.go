//go:build lecore_tier3 || lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

package core

import (
	"testing"
	"time"
)

func TestMigrateAnalyze_L1toL3(t *testing.T) {
	plan := MigrateAnalyze(".", TierL1, TierL3)

	if plan.ModuleCount() == 0 {
		t.Error("expected modules to migrate from L1 to L3")
	}
	if plan.EstimateTime() <= 0 {
		t.Error("estimated time should be positive")
	}

	// 验证依赖排序：被依赖的模块应在前面
	for i, m := range plan.Modules {
		for _, dep := range m.Dependencies {
			depIdx := -1
			for j := 0; j < i; j++ {
				if plan.Modules[j].ModuleName == dep {
					depIdx = j
					break
				}
			}
			if depIdx == -1 {
				t.Errorf("module %s depends on %s but %s is not before it",
					m.ModuleName, dep, dep)
			}
		}
	}
}

func TestMigrateAnalyze_SameTier(t *testing.T) {
	plan := MigrateAnalyze(".", TierL3, TierL3)
	if plan.ModuleCount() != 0 {
		t.Errorf("same tier should have 0 modules, got %d", plan.ModuleCount())
	}
}

func TestMigrateAnalyze_GenerateReport(t *testing.T) {
	plan := MigrateAnalyze(".", TierL1, TierL2)
	report := plan.GenerateReport()

	if report == "" {
		t.Error("report should not be empty")
	}
	if len(report) < 50 {
		t.Error("report should be reasonably long")
	}
}

func TestMigrationPlan_ModuleCount(t *testing.T) {
	plan := MigrateAnalyze(".", TierL0, TierL2)
	count := plan.ModuleCount()

	if count < 6 {
		t.Errorf("L0→L2 should have at least 6 modules, got %d", count)
	}
}

func TestMigrationPlan_RiskCount(t *testing.T) {
	plan := MigrateAnalyze(".", TierL0, TierL3)
	_ = plan.RiskCount() // 至少不会 panic
}

func TestMigrationPlan_EstimateTime(t *testing.T) {
	plan := MigrateAnalyze(".", TierL1, TierL3)
	estimate := plan.EstimateTime()

	if estimate <= 0 {
		t.Error("estimated time should be positive")
	}
	if estimate > 24*time.Hour {
		t.Error("estimated time unreasonably long")
	}
}

func TestModuleStatus_String(t *testing.T) {
	tests := []struct {
		status ModuleStatus
		want   string
	}{
		{ModulePending, "pending"},
		{ModuleInProgress, "in_progress"},
		{ModuleDone, "done"},
		{ModuleFailed, "failed"},
		{ModuleSkipped, "skipped"},
		{ModuleStatus(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.status.String(); got != tt.want {
				t.Errorf("ModuleStatus.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRiskLevel_String(t *testing.T) {
	tests := []struct {
		level RiskLevel
		want  string
	}{
		{RiskLow, "low"},
		{RiskMedium, "medium"},
		{RiskHigh, "high"},
		{RiskCritical, "critical"},
		{RiskLevel(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.level.String(); got != tt.want {
				t.Errorf("RiskLevel.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSortModulesByDependency(t *testing.T) {
	modules := []ModuleMigration{
		{ModuleName: "eventbus", Dependencies: []string{"eventstore"}},
		{ModuleName: "eventstore", Dependencies: nil},
	}
	sorted := sortModulesByDependency(modules)

	// eventstore should come before eventbus
	if sorted[0].ModuleName != "eventstore" {
		t.Errorf("expected eventstore first, got %s", sorted[0].ModuleName)
	}
	if sorted[1].ModuleName != "eventbus" {
		t.Errorf("expected eventbus second, got %s", sorted[1].ModuleName)
	}
}