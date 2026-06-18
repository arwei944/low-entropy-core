// Tier Migration Demo — 演示完整的 L1→L3 迁移流程
// Build: go build -tags lecore_tier3
//
// 演示内容：
//   1. TierCheck — 检测当前 tier 是否匹配项目规模
//   2. TierDrift — 持续监控 tier 漂移趋势
//   3. TierTransition — Feature Flag 渐进式迁移
//   4. MigrateAnalyze — 生成迁移计划
//   5. MigrateAdopt — 生成兼容层代码
//   6. MigrateValidate — 验证迁移结果

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	core "low-entropy-core/go-core"
)

func main() {
	fmt.Println("╔══════════════════════════════════════════════════╗")
	fmt.Println("║    Tier Migration Demo — L1 → L3 迁移流程       ║")
	fmt.Println("╚══════════════════════════════════════════════════╝")
	fmt.Println()

	// ──── Step 1: TierCheck — 编译时漂移检测 ────
	fmt.Println("━━━ Step 1: TierCheck ━━━")

	// 用 L0 检查当前项目 → 应该检测到漂移
	result := core.TierCheck(".", core.TierL0)
	fmt.Printf("  Status:        %s\n", result.Status)
	fmt.Printf("  Current Tier:  %s\n", result.CurrentTier)
	fmt.Printf("  Detected Tier: %s\n", result.DetectedTier)
	fmt.Printf("  Drift Level:   %d\n", result.DriftLevel)
	fmt.Printf("  Needs Migration: %v\n", result.NeedsMigration())
	fmt.Printf("  Is Critical:     %v\n", result.IsCritical())
	fmt.Printf("  Suggestion: %s\n", result.Suggestion)
	fmt.Println()

	// ──── Step 2: TierDrift — 持续监控 ────
	fmt.Println("━━━ Step 2: TierDrift Monitor ━━━")
	monitor := core.NewTierDriftMonitor(".", core.TierL0)

	// 模拟多次检查，观察趋势
	for i := 0; i < 3; i++ {
		report := monitor.Check()
		fmt.Printf("  Check %d: detected=%s drift=%d files=%d\n",
			i+1, report.DetectedTier, report.DriftLevel, report.ProjectStats.TotalFiles)
		time.Sleep(50 * time.Millisecond)
	}

	history := monitor.History()
	fmt.Printf("  History points: %d\n", len(history))

	pred := monitor.PredictNextTier()
	fmt.Printf("  Prediction: next tier=%s, growth=%0.1f files/week, confidence=%0.2f\n",
		pred.EstimatedNextTier, pred.GrowthRate, pred.Confidence)
	fmt.Println()

	// ──── Step 3: MigrateAnalyze — 生成迁移计划 ────
	fmt.Println("━━━ Step 3: MigrateAnalyze ━━━")
	plan := core.MigrateAnalyze(".", core.TierL1, core.TierL3)
	fmt.Printf("  From: %s → To: %s\n", plan.FromTier, plan.ToTier)
	fmt.Printf("  Modules to migrate: %d\n", plan.ModuleCount())
	fmt.Printf("  Risks identified: %d\n", plan.RiskCount())
	fmt.Printf("  Estimated time: %s\n", plan.EstimatedTime.Round(time.Minute))

	// 显示前 5 个模块
	fmt.Println("  Top 5 modules (by dependency order):")
	for i, m := range plan.Modules {
		if i >= 5 {
			break
		}
		fmt.Printf("    %d. %s (tier: %s, impact: %d, deps: %v)\n",
			i+1, m.ModuleName, m.SourceTier, m.ImpactScore, m.Dependencies)
	}
	fmt.Println()

	// ──── Step 4: TierTransition — Feature Flag 渐进式迁移 ────
	fmt.Println("━━━ Step 4: TierTransition ━━━")
	tt := core.NewTierTransition(core.TierL1, core.TierL2)
	fmt.Printf("  From: %s → To: %s\n", tt.FromTier(), tt.ToTier())
	fmt.Printf("  Total phases: %d\n", tt.PhaseCount())

	// 逐步推进
	for i := 0; i < 3 && !tt.IsDone(); i++ {
		if err := tt.Advance(); err != nil {
			fmt.Printf("  Phase %d failed: %v\n", tt.CurrentPhase(), err)
		} else {
			fmt.Printf("  Phase %d complete: %s (progress: %0.0f%%)\n",
				tt.CurrentPhase(), tt.EnabledModules(), tt.Progress()*100)
		}
	}

	// 回滚测试
	fmt.Println("  Testing rollback...")
	if err := tt.Rollback(); err != nil {
		fmt.Printf("  Rollback failed: %v\n", err)
	} else {
		fmt.Printf("  Rollback success, phase now: %d\n", tt.CurrentPhase())
	}

	// 全量推进
	fmt.Println("  Advancing all remaining phases...")
	if err := tt.AdvanceAll(); err != nil {
		fmt.Printf("  AdvanceAll failed: %v\n", err)
	} else {
		fmt.Printf("  All phases complete. Done: %v\n", tt.IsDone())
	}
	fmt.Println()

	// ──── Step 5: MigrateAdopt + TierBridge — 兼容层生成 ────
	fmt.Println("━━━ Step 5: MigrateAdopt + TierBridge ━━━")

	// 列出可用的兼容层
	bridges := core.ListAvailableBridges(core.TierL1, core.TierL3)
	fmt.Printf("  Available bridges: %d\n", len(bridges))
	for _, b := range bridges {
		fmt.Printf("    - %s: %s (%s → %s)\n", b.Name, b.Description, b.OldAPI, b.NewAPI)
	}

	// 生成单个 bridge 代码
	code, err := core.MigrateAdoptSingle(core.TierL1, core.TierL2, "eventstore")
	if err != nil {
		fmt.Printf("  Bridge generation failed: %v\n", err)
	} else {
		fmt.Printf("  EventStore bridge code: %d bytes\n", len(code))
	}

	// 输出兼容层到文件
	outputDir := filepath.Join(os.TempDir(), "lecore-bridges")
	adoptPlan := core.MigrateAnalyze(".", core.TierL1, core.TierL2)
	if err := core.MigrateAdoptWithOutput(adoptPlan, outputDir); err != nil {
		fmt.Printf("  AdoptWithOutput failed: %v\n", err)
	} else {
		fmt.Printf("  Bridges written to: %s\n", outputDir)
		entries, _ := os.ReadDir(outputDir)
		fmt.Printf("  Files generated: %d\n", len(entries))
		for _, e := range entries {
			fmt.Printf("    - %s\n", e.Name())
		}
	}
	fmt.Println()

	// ──── Step 6: 报告生成 ────
	fmt.Println("━━━ Step 6: Migration Report ━━━")
	fmt.Println()
	fmt.Print(plan.GenerateReport())
}