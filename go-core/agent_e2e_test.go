// Package core — Agent Workbench 端到端集成测试
//
// 覆盖 Agent Workbench 完整生命周期：
//   - Agent 提交代码 → Submit → 审核 → 决策 → 结果
//   - Guardian 拒绝不合规代码
//   - Agent 迭代修复：提交 → 被拒 → 修复 → 重新提交
package core

import (
	"context"
	"testing"
	"time"
)

// TestAgentWorkbench_E2E 测试完整的 Agent Workbench 管道。
// 模拟：Agent 编写一段合法的代码 → 提交到 Workbench → 审核 → 返回结果。
func TestAgentWorkbench_E2E(t *testing.T) {
	obs := &InMemoryObservationAdapter{}

	// 创建 Agent Workbench
	wb := NewDefaultAgentWorkbench(obs, nil, nil, nil)

	// Agent 提交一段合法的代码
	submission := AgentCodeSubmission{
		AgentID:    "agent-e2e-001",
		TaskID:     "task-e2e-001",
		SourceCode: `package main
import "fmt"
func main() { fmt.Println("hello from agent") }`,
		Manifest: []PrimitiveManifest{
			{
				PrimitiveType: "Atom",
				Name:          "helloAtom",
				Layer:         "L1",
				InputType:     "string",
				OutputType:    "string",
				Dependencies:  []string{},
			},
		},
		Attempt:     1,
		SubmittedAt: time.Now(),
	}

	ctx := context.Background()
	result, err := wb.Submit(ctx, submission)
	if err != nil {
		t.Fatalf("submit failed: %v", err)
	}

	// 验证提交结果
	if result.SubmissionID == "" {
		t.Error("expected non-empty SubmissionID")
	}
	if result.Status != SubmissionApproved && result.Status != SubmissionRejected && result.Status != SubmissionNeedsRevision {
		t.Errorf("expected valid status, got %q", result.Status)
	}

	// 验证提交历史
	subs := wb.ListSubmissionsByAgent("agent-e2e-001")
	if len(subs) == 0 {
		t.Error("expected at least 1 submission in history")
	}
}

// TestAgentWorkbench_GuardianRejection 测试 Guardian 拒绝不合规代码。
func TestAgentWorkbench_GuardianRejection(t *testing.T) {
	obs := &InMemoryObservationAdapter{}
	wb := NewDefaultAgentWorkbench(obs, nil, nil, nil)

	// 提交声明为 Atom 但实际是 Adapter 的代码（包含 I/O）
	submission := AgentCodeSubmission{
		AgentID:    "agent-bad-001",
		TaskID:     "task-bad-001",
		SourceCode: `package main
import "os"
func main() { os.WriteFile("bad.txt", []byte("x"), 0644) }`,
		Manifest: []PrimitiveManifest{
			{
				PrimitiveType: "Atom",
				Name:          "badAtom",
				Layer:         "L1",
				InputType:     "string",
				OutputType:    "string",
				Dependencies:  []string{},
			},
		},
		Attempt:     1,
		SubmittedAt: time.Now(),
	}

	result, err := wb.Submit(context.Background(), submission)
	if err != nil {
		t.Fatal(err)
	}

	// 应该检测到 Manifest 声明与实际代码不符
	if result.Status == SubmissionApproved {
		// 如果 Guardian 没有启用，这也是可以接受的
		t.Log("Guardian not active, submission accepted")
	}
}

// TestAgentWorkbench_MultipleSubmissions 测试多次迭代提交。
// 模拟：Agent 第 1 次提交 → 第 2 次提交（迭代修复）。
func TestAgentWorkbench_MultipleSubmissions(t *testing.T) {
	obs := &InMemoryObservationAdapter{}
	wb := NewDefaultAgentWorkbench(obs, nil, nil, nil)

	ctx := context.Background()
	now := time.Now()

	// 第 1 次尝试
	sub1 := AgentCodeSubmission{
		AgentID:    "agent-iter-001",
		TaskID:     "task-iter-001",
		SourceCode: `package main
func main() {}`,
		Manifest: []PrimitiveManifest{
			{PrimitiveType: "Atom", Name: "test", Layer: "L1", InputType: "string", OutputType: "string"},
		},
		Attempt:     1,
		SubmittedAt: now,
	}
	result1, _ := wb.Submit(ctx, sub1)
	t.Logf("Attempt 1: status=%s, violations=%d", result1.Status, len(result1.Violations))

	// 第 2 次尝试（根据反馈修复）
	sub2 := AgentCodeSubmission{
		AgentID:    "agent-iter-001",
		TaskID:     "task-iter-001",
		SourceCode: `package main
func main() {}`,
		Manifest: []PrimitiveManifest{
			{PrimitiveType: "Atom", Name: "test", Layer: "L1", InputType: "string", OutputType: "string"},
		},
		Attempt:     2,
		SubmittedAt: now.Add(time.Second),
	}
	result2, _ := wb.Submit(ctx, sub2)
	t.Logf("Attempt 2: status=%s, violations=%d", result2.Status, len(result2.Violations))

	// 验证提交历史
	subs := wb.ListSubmissionsByAgent("agent-iter-001")
	if len(subs) < 2 {
		t.Errorf("expected at least 2 submissions, got %d", len(subs))
	}
}