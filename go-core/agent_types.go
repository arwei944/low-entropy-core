//go:build lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

// Package core — Agent 提交接口 (Phase 2 P1)
//
// 定义 AI Agent 与框架之间的"合同"：
//   - AgentCodeSubmission: Agent 提交的代码包（源码 + Manifest 声明）
//   - PrimitiveManifest: Agent 对每个原语的声明
//   - SubmissionResult: 框架对提交的完整反馈
//   - Violation: 具体的违规描述（含修复建议）
//   - AgentWorkbench: Agent 与框架的唯一入口接口
//   - DefaultAgentWorkbench: 串联提交 → 审核 → 编译 → 执行的管道
//
// 设计原则：
//   - Agent 写代码，框架做审核、编译、执行
//   - Manifest 是 Agent 的"承诺"，Guardian 会对比 Manifest 和实际代码
//   - 审核不通过时返回 Violation 列表（含修复建议），Agent 据此修改代码后重新提交
package core

import (
	"fmt"
	"time"
)

// AgentCodeSubmission 是 AI Agent 向 Workbench 提交的完整代码包。
// Agent 写代码 → 构造此结构体 → 调用 AgentWorkbench.Submit() 提交。
type AgentCodeSubmission struct {
	AgentID     string             `json:"agent_id"`
	TaskID      string             `json:"task_id"`
	SourceCode  string             `json:"source_code"`
	Manifest    []PrimitiveManifest `json:"manifest"`
	Attempt     int                `json:"attempt"`
	SubmittedAt time.Time          `json:"submitted_at"`
}

// SubmissionID 为每次提交生成唯一标识。
func (s *AgentCodeSubmission) SubmissionID() string {
	return fmt.Sprintf("sub-%s-%s-%d", s.AgentID, s.TaskID, s.Attempt)
}

// PrimitiveManifest 是 Agent 对每个原语的声明。
// Guardian 审核时会对比 Manifest 和实际代码，不一致则 Rejected。
// 这防止 Agent 声明一套、实际写另一套。
type PrimitiveManifest struct {
	PrimitiveType string   `json:"primitive_type"` // "Atom" | "Port" | "Adapter" | "Composer"
	Name          string   `json:"name"`           // 原语名称（如 "hashPasswordAtom"）
	Layer         string   `json:"layer"`          // 所在层级（如 "L1"）
	InputType     string   `json:"input_type"`     // 输入类型
	OutputType    string   `json:"output_type"`    // 输出类型
	Dependencies  []string `json:"dependencies"`   // 依赖的其他原语名称
}

// ValidPrimitiveTypes 定义了合法的原语类型集合。
var ValidPrimitiveTypes = map[string]bool{
	"Atom":     true,
	"Port":     true,
	"Adapter":  true,
	"Composer": true,
}

// Validate 对 Manifest 做基本校验（不涉及 AST 解析，P2 的 StaticGuardPort 会做深度校验）。
func (m *PrimitiveManifest) Validate() []Violation {
	var violations []Violation

	if m.Name == "" {
		violations = append(violations, Violation{
			Rule:       "manifest_incomplete",
			Severity:   "error",
			Detail:     "PrimitiveManifest.Name 不能为空",
			Suggestion: "为每个原语提供唯一的 Name 字段",
		})
	}

	if !ValidPrimitiveTypes[m.PrimitiveType] {
		violations = append(violations, Violation{
			Rule:       "invalid_primitive_type",
			Severity:   "error",
			Location:   fmt.Sprintf("Manifest[%s]", m.Name),
			Detail:     fmt.Sprintf("非法的原语类型: %s，必须是 Atom/Port/Adapter/Composer 之一", m.PrimitiveType),
			Suggestion: "将 primitive_type 改为 Atom、Port、Adapter 或 Composer",
		})
	}

	return violations
}

// SubmissionStatus 表示提交的审核状态。
type SubmissionStatus string

const (
	SubmissionApproved      SubmissionStatus = "Approved"
	SubmissionRejected      SubmissionStatus = "Rejected"
	SubmissionNeedsRevision SubmissionStatus = "NeedsRevision"
)

// SubmissionResult 是 AgentWorkbench 对提交的完整反馈。
// Agent 通过此结果了解提交是否通过、哪里违规、以及执行结果。
type SubmissionResult struct {
	SubmissionID   string           `json:"submission_id"`
	AgentID        string           `json:"agent_id"`
	TaskID         string           `json:"task_id"`
	Status         SubmissionStatus `json:"status"`
	Violations     []Violation      `json:"violations,omitempty"`
	CompiledBinary string           `json:"compiled_binary,omitempty"`
	ExecutionSteps []ExecutionStep  `json:"execution_steps,omitempty"`
	Error          string           `json:"error,omitempty"`
	ReviewedAt     time.Time        `json:"reviewed_at"`
}

// HasErrors 检查是否有 error 级别的违规。
func (r *SubmissionResult) HasErrors() bool {
	for _, v := range r.Violations {
		if v.Severity == "error" {
			return true
		}
	}
	return false
}

// Violation 描述一个具体的违规，包含修复建议。
// Agent 收到 Block 后根据 Suggestion 修改代码，然后重新提交。
type Violation struct {
	Rule       string `json:"rule"`       // 违规规则名（如 "primitive_compliance"）
	Severity   string `json:"severity"`   // "error" | "warn"
	Location   string `json:"location"`   // 代码位置（如 "line 42"）
	Detail     string `json:"detail"`     // 具体描述
	Suggestion string `json:"suggestion"` // 修复建议（Agent 据此修改代码）
}
