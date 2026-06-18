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
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

// ──────────────────────────────────────────────
// SECTION 1: AgentCodeSubmission — Agent 的提交
// ──────────────────────────────────────────────

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

// ──────────────────────────────────────────────
// SECTION 2: PrimitiveManifest — Agent 的"承诺"
// ──────────────────────────────────────────────

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

// ──────────────────────────────────────────────
// SECTION 3: SubmissionResult — 框架的返回
// ──────────────────────────────────────────────

// SubmissionStatus 表示提交的审核状态。
type SubmissionStatus string

const (
	SubmissionApproved     SubmissionStatus = "Approved"
	SubmissionRejected     SubmissionStatus = "Rejected"
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

// ──────────────────────────────────────────────
// SECTION 4: Violation — 违规描述
// ──────────────────────────────────────────────

// Violation 描述一个具体的违规，包含修复建议。
// Agent 收到 Block 后根据 Suggestion 修改代码，然后重新提交。
type Violation struct {
	Rule       string `json:"rule"`       // 违规规则名（如 "primitive_compliance"）
	Severity   string `json:"severity"`   // "error" | "warn"
	Location   string `json:"location"`   // 代码位置（如 "line 42"）
	Detail     string `json:"detail"`     // 具体描述
	Suggestion string `json:"suggestion"` // 修复建议（Agent 据此修改代码）
}

// ──────────────────────────────────────────────
// SECTION 5: AgentWorkbench — Agent 唯一入口
// ──────────────────────────────────────────────

// AgentWorkbench 是 Agent 与框架之间的唯一入口接口。
// 所有 Agent 代码提交都通过这个接口。
type AgentWorkbench interface {
	// Submit 提交代码，经过审核，返回结果。
	// 审核不通过时返回 Rejected + Violations（含修复建议）。
	// Agent 根据 Suggestion 修改代码后重新 Submit。
	Submit(ctx context.Context, submission AgentCodeSubmission) (SubmissionResult, error)

	// SubmitAndRun 提交代码，审核通过后编译执行，返回 ExecutionStep 流。
	// 这是 Agent 最常用的方法：一次调用完成提交到执行的完整流程。
	SubmitAndRun(ctx context.Context, submission AgentCodeSubmission) (SubmissionResult, error)
}

// ──────────────────────────────────────────────
// SECTION 6: DefaultAgentWorkbench — 默认实现
// ──────────────────────────────────────────────

// DefaultAgentWorkbench 是 AgentWorkbench 的默认实现。
// 串联完整管道：提交 → 审核 → 编译 → 执行。
//
// P1 阶段：仅做 Manifest 基本校验。
// P2 阶段：接入 StaticGuardPort 做深度审核。
// P3 阶段：接入 AgentRunner 做编译执行。
type DefaultAgentWorkbench struct {
	mu             sync.RWMutex
	submissions    map[string]SubmissionResult // 提交历史
	obs            ObservationAdapter          // 已有：观察适配器
	decisionEngine *DecisionEngine             // 已有：决策引擎
	staticGuard    *StaticGuardPort            // P2: 静态审核器
	runner         *AgentRunner                // P3: 编译执行器
}

// NewDefaultAgentWorkbench 创建 DefaultAgentWorkbench。
func NewDefaultAgentWorkbench(obs ObservationAdapter, staticGuard *StaticGuardPort, decisionEngine *DecisionEngine, runner *AgentRunner) *DefaultAgentWorkbench {
	if obs == nil {
		obs = &NoOpObservationAdapter{}
	}
	return &DefaultAgentWorkbench{
		submissions:    make(map[string]SubmissionResult),
		obs:            obs,
		staticGuard:    staticGuard,
		decisionEngine: decisionEngine,
		runner:         runner,
	}
}

// Submit 实现 AgentWorkbench.Submit。
// P1 阶段：仅做 Manifest 基本校验 + 基本完整性检查。
// P2 阶段：接入 StaticGuardPort 做深度审核。
func (w *DefaultAgentWorkbench) Submit(ctx context.Context, submission AgentCodeSubmission) (SubmissionResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	select {
	case <-ctx.Done():
		return SubmissionResult{}, ctx.Err()
	default:
	}

	start := time.Now()
	submissionID := submission.SubmissionID()
	submission.SubmittedAt = time.Now()

	// P1: 基本校验
	violations := w.validateBasic(submission)

	// P2: StaticGuardPort 深度审核
	if w.staticGuard != nil {
		reviewResult, err := w.staticGuard.Validate(ctx, submission)
		if err != nil {
			return SubmissionResult{}, err
		}
		violations = append(violations, reviewResult.Violations...)
	}

	var status SubmissionStatus
	if len(violations) == 0 {
		status = SubmissionApproved
	} else if hasOnlyWarnings(violations) {
		status = SubmissionApproved // warn 不阻断
	} else {
		status = SubmissionRejected
	}

	result := SubmissionResult{
		SubmissionID: submissionID,
		AgentID:      submission.AgentID,
		TaskID:       submission.TaskID,
		Status:       status,
		Violations:   violations,
		ReviewedAt:   time.Now(),
	}

	// 记录 ExecutionStep
	es := NewExecutionStep("AgentWorkbench", "Submit", "submission reviewed", "Composer")
	es.DurationMs = time.Since(start).Milliseconds()
	es.Metadata = map[string]interface{}{
		"submission_id": submissionID,
		"agent_id":      submission.AgentID,
		"task_id":       submission.TaskID,
		"attempt":       submission.Attempt,
		"status":        string(status),
		"violations":    len(violations),
	}
	w.obs.Record([]ExecutionStep{es})

	// 存储提交历史
	w.mu.Lock()
	w.submissions[submissionID] = result
	w.mu.Unlock()

	return result, nil
}

// SubmitAndRun 实现 AgentWorkbench.SubmitAndRun。
// Submit 通过后，AgentRunner 编译执行。
func (w *DefaultAgentWorkbench) SubmitAndRun(ctx context.Context, submission AgentCodeSubmission) (SubmissionResult, error) {
	result, err := w.Submit(ctx, submission)
	if err != nil {
		return result, err
	}

	if result.Status == SubmissionRejected {
		return result, nil
	}

	// P3: AgentRunner 编译执行
	if w.runner != nil {
		steps, runErr := w.runner.BuildAndRun(ctx, submission)
		result.ExecutionSteps = steps
		if runErr != nil {
			result.Error = runErr.Error()
		}
	} else {
		// P1 fallback: 返回空 ExecutionStep（标记为 Pending Execution）
		es := NewExecutionStep("AgentWorkbench", "SubmitAndRun", "execution pending (AgentRunner not configured)", "Composer")
		es.Metadata = map[string]interface{}{
			"submission_id": result.SubmissionID,
			"note":          "Configure AgentRunner for compilation and execution",
		}
		result.ExecutionSteps = []ExecutionStep{es}
		w.obs.Record(result.ExecutionSteps)
	}

	return result, nil
}

// GetSubmission 获取提交历史。
func (w *DefaultAgentWorkbench) GetSubmission(submissionID string) (SubmissionResult, bool) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	result, ok := w.submissions[submissionID]
	return result, ok
}

// ListSubmissionsByAgent 获取某个 Agent 的所有提交历史。
func (w *DefaultAgentWorkbench) ListSubmissionsByAgent(agentID string) []SubmissionResult {
	w.mu.RLock()
	defer w.mu.RUnlock()
	var results []SubmissionResult
	for _, r := range w.submissions {
		if r.AgentID == agentID {
			results = append(results, r)
		}
	}
	return results
}

// ──────────────────────────────────────────────
// SECTION 7: 基本校验（P1 阶段）
// ──────────────────────────────────────────────

// validateBasic 做 AgentCodeSubmission 的基本完整性校验。
// 不涉及 AST 解析（P2 的 StaticGuardPort 会做深度校验）。
func (w *DefaultAgentWorkbench) validateBasic(sub AgentCodeSubmission) []Violation {
	var violations []Violation

	// 检查 1: AgentID 不能为空
	if sub.AgentID == "" {
		violations = append(violations, Violation{
			Rule:       "missing_agent_id",
			Severity:   "error",
			Detail:     "AgentID 不能为空",
			Suggestion: "在 AgentCodeSubmission 中设置 AgentID 字段",
		})
	}

	// 检查 2: TaskID 不能为空
	if sub.TaskID == "" {
		violations = append(violations, Violation{
			Rule:       "missing_task_id",
			Severity:   "error",
			Detail:     "TaskID 不能为空",
			Suggestion: "在 AgentCodeSubmission 中设置 TaskID 字段",
		})
	}

	// 检查 3: SourceCode 不能为空
	if strings.TrimSpace(sub.SourceCode) == "" {
		violations = append(violations, Violation{
			Rule:       "empty_source_code",
			Severity:   "error",
			Detail:     "SourceCode 不能为空",
			Suggestion: "Agent 必须编写并提交 Go 源代码",
		})
	}

	// 检查 4: Manifest 不能为空
	if len(sub.Manifest) == 0 {
		violations = append(violations, Violation{
			Rule:       "empty_manifest",
			Severity:   "error",
			Detail:     "Manifest 不能为空，必须声明使用了哪些原语",
			Suggestion: "在 Manifest 中列出每个原语的 PrimitiveManifest 声明",
		})
	}

	// 检查 5: 每个 Manifest 条目自检
	for _, m := range sub.Manifest {
		violations = append(violations, m.Validate()...)
	}

	// 检查 6: Manifest 声明类型一致性
	manifestNames := make(map[string]bool)
	for _, m := range sub.Manifest {
		if manifestNames[m.Name] {
			violations = append(violations, Violation{
				Rule:       "duplicate_manifest_name",
				Severity:   "error",
				Location:   fmt.Sprintf("Manifest[%s]", m.Name),
				Detail:     fmt.Sprintf("Manifest 中存在重复的原语名称: %s", m.Name),
				Suggestion: "确保每个原语有唯一的 Name",
			})
		}
		manifestNames[m.Name] = true
	}

	// 检查 7: SourceCode 中是否包含 "package main"（基本 Go 语法检查）
	if !strings.Contains(sub.SourceCode, "package main") {
		violations = append(violations, Violation{
			Rule:       "missing_package_main",
			Severity:   "warn",
			Detail:     "SourceCode 中未找到 'package main' 声明",
			Suggestion: "确保 Go 源代码以 'package main' 开头",
		})
	}

	return violations
}

// ──────────────────────────────────────────────
// SECTION 8: Agent 注册与心跳 (Phase 2 P4)
// ──────────────────────────────────────────────

// RegisterAgent 注册 Agent 到 AgentPool。
// 通常由 Agent 初始化时调用。
func RegisterAgent(pool *AgentPool, agentID string, capabilities []string, phase string) error {
	if pool == nil {
		return NewStepError("NO_POOL", "AgentPool is nil", false)
	}
	return pool.Add(&AgentInfo{
		ID:           agentID,
		Capabilities: capabilities,
		Status:       AgentStatusIdle,
		Phase:        phase,
	})
}

// AgentHeartbeat 发送 Agent 心跳。
// Agent 应定期调用（如每 10 秒）。
func AgentHeartbeat(pool *AgentPool, agentID string) error {
	if pool == nil {
		return NewStepError("NO_POOL", "AgentPool is nil", false)
	}
	return pool.Heartbeat(agentID)
}

// DeregisterAgent 从 AgentPool 注销 Agent。
// 通常由 Agent 退出时调用。
func DeregisterAgent(pool *AgentPool, agentID string) {
	if pool != nil {
		pool.Remove(agentID)
	}
}

// ──────────────────────────────────────────────
// SECTION 9: 辅助函数
// ──────────────────────────────────────────────

// hasOnlyWarnings 判断违规列表是否只包含 warn 级别。
func hasOnlyWarnings(violations []Violation) bool {
	for _, v := range violations {
		if v.Severity == "error" {
			return false
		}
	}
	return len(violations) > 0
}