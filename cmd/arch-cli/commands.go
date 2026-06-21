// arch-cli - 高级命令实现
//
// 包含 guardian/entropy/agent/migrate 四个命令的实现。
// 与 main.go 同包共享 parseFlag / fatal / arch 等函数。
package main

import (
	"context"
	"fmt"

	"low-entropy-core/go-core/arch"
)

// ──────────────────────────────────────────────
// guardian 命令
// ──────────────────────────────────────────────
func cmdGuardian(ctx context.Context, args []string) {
	dir := parseFlag(args, "--dir", ".")

	p, err := arch.NewPipeline()
	if err != nil {
		fatal("创建 pipeline 失败:", err)
	}

	data, err := p.Analyze(ctx, dir)
	if err != nil {
		fatal("分析失败:", err)
	}

	result, err := p.Validate(ctx, dir)
	if err != nil {
		fatal("校验失败:", err)
	}

	decision := arch.Decide(result.HealthScore, result.Violations, arch.DefaultThresholds())

	fmt.Println("=== Guardian 决策报告 ===")
	fmt.Printf("  行动: %s\n", decision.Action)
	fmt.Printf("  理由: %s\n", decision.Reason)
	fmt.Printf("  健康度: %.2f / 阈值: %.2f\n", decision.Score, decision.Threshold)
	fmt.Printf("  违规数: %d\n", len(decision.Violations))
	fmt.Printf("  文件总数: %d | 行数: %d\n", data.TotalFiles, data.TotalLines)
}

// ──────────────────────────────────────────────
// entropy 命令
// ──────────────────────────────────────────────
func cmdEntropy(ctx context.Context, args []string) {
	dir := parseFlag(args, "--dir", ".")

	p, err := arch.NewPipeline()
	if err != nil {
		fatal("创建 pipeline 失败:", err)
	}

	data, err := p.Analyze(ctx, dir)
	if err != nil {
		fatal("分析失败:", err)
	}

	metrics := arch.ComputeEntropy(data)
	overall := arch.ComputeOverallEntropy(data)

	fmt.Println("=== 代码熵值分析 ===")
	fmt.Printf("  综合风险: %s (圈复杂度 %.2f | 依赖深度 %.2f | 漂移 %.2f)\n",
		overall.RiskLevel, overall.Cyclomatic, overall.Depth, overall.DriftScore)
	fmt.Println("")
	fmt.Println("=== 各层级指标 ===")
	fmt.Printf("  %-4s %-10s %-8s %-12s %-10s %-10s\n",
		"层级", "文件数", "行数", "圈复杂度", "依赖深度", "风险")
	for _, m := range metrics {
		fmt.Printf("  %-4s %-10d %-8d %-12.2f %-10.2f %-10s\n",
			m.Layer, m.FileCount, m.LineCount, m.Cyclomatic, m.Depth, m.RiskLevel)
	}

	_, suggestions := arch.ValidateStructure(data)
	if len(suggestions) > 0 {
		fmt.Println("")
		fmt.Println("=== 改进建议 ===")
		for _, s := range suggestions {
			fmt.Printf("  - %s\n", s)
		}
	}
}

// ──────────────────────────────────────────────
// agent 命令
// ──────────────────────────────────────────────
func cmdAgent(args []string) {
	if len(args) == 0 || args[0] == "list" {
		agents := agentPool.GetAgents()
		fmt.Println("=== Agent 列表 ===")
		if len(agents) == 0 {
			fmt.Println("  (无已注册 Agent)")
		}
		for _, a := range agents {
			fmt.Printf("  ID: %-20s | 状态: %-8s | 阶段: %s\n",
				a.ID, a.Status, a.Phase)
		}
		return
	}
	fmt.Println("用法: arch agent list")
}

// ──────────────────────────────────────────────
// migrate 命令
// ──────────────────────────────────────────────
func cmdMigrate(ctx context.Context, args []string) {
	dir := parseFlag(args, "--dir", ".")

	p, err := arch.NewPipeline()
	if err != nil {
		fatal("创建 pipeline 失败:", err)
	}

	data, err := p.Analyze(ctx, dir)
	if err != nil {
		fatal("分析失败:", err)
	}

	plan := arch.AnalyzeMigration(nil, data)

	fmt.Println("=== 迁移分析 ===")
	fmt.Printf("  ID: %s\n", plan.ID)
	fmt.Printf("  状态: %s (进度 %d%%)\n", plan.Status, plan.Progress)
	fmt.Println("")
	fmt.Println("=== 迁移步骤 ===")
	for i, step := range plan.Steps {
		icon := "○"
		switch step.Status {
		case "done":
			icon = "✓"
		case "running":
			icon = "▶"
		case "failed":
			icon = "✗"
		}
		fmt.Printf("  [%d] %s %-10s - %s\n",
			i+1, icon, step.Name, step.Description)
	}

	nextVersion := arch.ComputeNextVersion("1.0.0",
		arch.DetectDrift(nil, data))
	fmt.Printf("\n建议下一版本: %s\n", nextVersion)
}
