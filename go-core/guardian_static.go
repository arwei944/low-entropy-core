//go:build lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

// Package core — Guardian 静态审核 (Phase 2 P2)
//
// StaticGuardPort 在编译前对 Agent 提交的代码进行 4 项静态检查：
//   1. 原语合规 — 代码中使用的类型是否在 GlobalPrimitiveTypeSet 白名单中
//   2. 外部依赖 — import 是否引入了 go-core 和标准库之外的包
//   3. 层级合规 — 声明所在层是否合理，是否越层调用
//   4. 复杂度 — 函数行数、嵌套深度、圈复杂度是否超标
//
// StaticGuardPort 实现 Port 接口，可嵌入任何 Pipeline。
// 审核结果通过 Violation 列表返回，包含具体代码位置和修复建议。
package core

import (
	"context"
	"fmt"
	"go/parser"
	"go/token"
	"time"
)

// ──────────────────────────────────────────────
// SECTION 1: StaticReviewResult — 静态审核结果
// ──────────────────────────────────────────────

// StaticReviewResult 是 StaticGuardPort 的审核输出。
// 会被接入 DecisionEngine 做统一决策。
type StaticReviewResult struct {
	Violations []Violation `json:"violations"`
	ReviewedAt time.Time   `json:"reviewed_at"`
}

// HasErrors 检查是否有 error 级别的违规。
func (r *StaticReviewResult) HasErrors() bool {
	for _, v := range r.Violations {
		if v.Severity == "error" {
			return true
		}
	}
	return false
}

// ErrorCount 返回 error 级别违规数量。
func (r *StaticReviewResult) ErrorCount() int {
	count := 0
	for _, v := range r.Violations {
		if v.Severity == "error" {
			count++
		}
	}
	return count
}

// ──────────────────────────────────────────────
// SECTION 2: StaticGuardPort — 静态审核器
// ──────────────────────────────────────────────

// StaticGuardPort 是编译前的静态代码审核器。
// 实现 Port 接口，可嵌入任何 Pipeline。
//
// 审核流程：
//   1. 解析 SourceCode 为 AST
//   2. 遍历 AST 节点，执行 4 项检查
//   3. 返回 Violation 列表（含修复建议）
type StaticGuardPort struct {
	primitiveSet *typeSet         // 已有：GlobalPrimitiveTypeSet
	archGuard    *ArchitectureGuard // 已有：架构守卫（层级合规检查）
	obs          ObservationAdapter
}

// NewStaticGuardPort 创建 StaticGuardPort。
func NewStaticGuardPort(primitiveSet *typeSet, archGuard *ArchitectureGuard, obs ObservationAdapter) *StaticGuardPort {
	if obs == nil {
		obs = &NoOpObservationAdapter{}
	}
	return &StaticGuardPort{
		primitiveSet: primitiveSet,
		archGuard:    archGuard,
		obs:          obs,
	}
}

// Validate 实现 Port 接口。
// 输入：AgentCodeSubmission
// 输出：StaticReviewResult（含违规列表）
func (g *StaticGuardPort) Validate(ctx context.Context, input AgentCodeSubmission) (StaticReviewResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	select {
	case <-ctx.Done():
		return StaticReviewResult{}, ctx.Err()
	default:
	}

	start := time.Now()
	result := g.review(input)

	es := NewExecutionStep("StaticGuardPort", "Validate", "static code review completed", "Port")
	es.DurationMs = time.Since(start).Milliseconds()
	es.Metadata = map[string]any{
		"agent_id":    input.AgentID,
		"task_id":     input.TaskID,
		"attempt":     input.Attempt,
		"violations":  len(result.Violations),
		"has_errors":  result.HasErrors(),
	}
	g.obs.Record([]ExecutionStep{es})

	return result, nil
}

// review 是核心审核逻辑（纯函数，便于测试）。
func (g *StaticGuardPort) review(sub AgentCodeSubmission) StaticReviewResult {
	var violations []Violation

	// 解析 AST
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "agent_code.go", sub.SourceCode, parser.ParseComments)
	if err != nil {
		violations = append(violations, Violation{
			Rule:       "parse_error",
			Severity:   "error",
			Detail:     fmt.Sprintf("代码解析失败: %v", err),
			Suggestion: "检查 Go 语法是否正确",
		})
		return StaticReviewResult{Violations: violations, ReviewedAt: time.Now()}
	}

	// 检查 1: 原语合规
	violations = append(violations, g.checkPrimitiveCompliance(fset, file, sub.Manifest)...)

	// 检查 2: 外部依赖
	violations = append(violations, g.checkExternalDeps(fset, file)...)

	// 检查 3: 层级合规
	violations = append(violations, g.checkLayerCompliance(sub.Manifest)...)

	// 检查 4: 复杂度
	violations = append(violations, g.checkComplexity(fset, file)...)

	// 检查 5: Manifest 与实际代码一致性
	violations = append(violations, g.checkManifestConsistency(fset, file, sub.Manifest)...)

	return StaticReviewResult{Violations: violations, ReviewedAt: time.Now()}
}
