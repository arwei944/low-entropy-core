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

// hasOnlyWarnings 判断违规列表是否只包含 warn 级别。
func hasOnlyWarnings(violations []Violation) bool {
	for _, v := range violations {
		if v.Severity == "error" {
			return false
		}
	}
	return len(violations) > 0
}
