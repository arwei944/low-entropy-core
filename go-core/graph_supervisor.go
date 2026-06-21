// Package core — Understand-Anything 监督层 (v0.7.0)
//
// T-08: ConstraintRule — 6 条约束规则定义
// T-09: GraphSupervisor — 约束验证执行器
//
// 约束遵循:
//   C3: 原语纯度 — 约束检查是纯函数
//   C5: Step 统一 — 监督流程包装为 Step

package core

import (
	"context"
	"fmt"
)

// ============================================================================
// T-09: GraphSupervisor
// ============================================================================

// GraphSupervisor 图谱监督器
type GraphSupervisor struct {
	rules []ConstraintRule
}

// NewGraphSupervisor 创建监督器
func NewGraphSupervisor(rules []ConstraintRule) *GraphSupervisor {
	if rules == nil {
		rules = DefaultConstraints()
	}
	return &GraphSupervisor{rules: rules}
}

// ValidateAll 执行全部约束检查
func (s *GraphSupervisor) ValidateAll(kg *KnowledgeGraph) ConstraintReport {
	report := ConstraintReport{
		Results: make([]ConstraintResult, 0, len(s.rules)),
	}

	for _, rule := range s.rules {
		result := rule.Check(kg)
		report.Results = append(report.Results, result)

		switch result.Status {
		case ConstraintPass:
			report.PassCount++
		case ConstraintWarn:
			report.WarnCount++
		case ConstraintFail:
			report.FailCount++
			if rule.Severity == "fatal" {
				report.HasFatal = true
			}
		}
	}

	// 生成摘要
	if report.HasFatal {
		report.Summary = fmt.Sprintf("FAIL: %d 通过, %d 警告, %d 失败 (含严重错误)",
			report.PassCount, report.WarnCount, report.FailCount)
	} else if report.FailCount > 0 || report.WarnCount > 0 {
		report.Summary = fmt.Sprintf("WARN: %d 通过, %d 警告, %d 失败",
			report.PassCount, report.WarnCount, report.FailCount)
	} else {
		report.Summary = fmt.Sprintf("PASS: %d 条约束全部通过", report.PassCount)
	}

	return report
}

// NewSupervisorStep 创建监督 Step
func NewSupervisorStep(supervisor *GraphSupervisor, kg *KnowledgeGraph) Step[struct{}, ConstraintReport] {
	return NewStepFunc("Atom", func(ctx context.Context, _ struct{}) (ConstraintReport, error) {
		report := supervisor.ValidateAll(kg)
		return report, nil
	})
}
