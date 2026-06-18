//go:build lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

package core

import (
	"context"
	"testing"
	"time"
)

// ──────────────────────────────────────────────
// P1 测试: AgentCodeSubmission + AgentWorkbench
// ──────────────────────────────────────────────

// TestAgentCodeSubmission_SubmissionID 测试 SubmissionID 生成。
func TestAgentCodeSubmission_SubmissionID(t *testing.T) {
	sub := AgentCodeSubmission{
		AgentID: "agent-calc-01",
		TaskID:  "task-001",
		Attempt: 1,
	}
	expected := "sub-agent-calc-01-task-001-1"
	if got := sub.SubmissionID(); got != expected {
		t.Errorf("SubmissionID() = %s, want %s", got, expected)
	}

	sub.Attempt = 3
	expected = "sub-agent-calc-01-task-001-3"
	if got := sub.SubmissionID(); got != expected {
		t.Errorf("SubmissionID() = %s, want %s", got, expected)
	}
}

// TestPrimitiveManifest_Validate 测试 Manifest 基本校验。
func TestPrimitiveManifest_Validate(t *testing.T) {
	tests := []struct {
		name     string
		manifest PrimitiveManifest
		wantErr  bool
	}{
		{
			name: "valid atom manifest",
			manifest: PrimitiveManifest{
				PrimitiveType: "Atom",
				Name:          "hashPasswordAtom",
				Layer:         "L1",
				InputType:     "RegisterReq",
				OutputType:    "User",
			},
			wantErr: false,
		},
		{
			name: "valid port manifest",
			manifest: PrimitiveManifest{
				PrimitiveType: "Port",
				Name:          "validateRegisterPort",
				Layer:         "L1",
				InputType:     "RegisterReq",
				OutputType:    "RegisterReq",
			},
			wantErr: false,
		},
		{
			name: "valid adapter manifest",
			manifest: PrimitiveManifest{
				PrimitiveType: "Adapter",
				Name:          "saveUserAdapter",
				Layer:         "L7",
				InputType:     "User",
				OutputType:    "error",
			},
			wantErr: false,
		},
		{
			name: "empty name",
			manifest: PrimitiveManifest{
				PrimitiveType: "Atom",
				Name:          "",
			},
			wantErr: true,
		},
		{
			name: "invalid primitive type",
			manifest: PrimitiveManifest{
				PrimitiveType: "Handler",
				Name:          "myHandler",
			},
			wantErr: true,
		},
		{
			name: "empty primitive type and name",
			manifest: PrimitiveManifest{
				PrimitiveType: "",
				Name:          "",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			violations := tt.manifest.Validate()
			hasErrors := false
			for _, v := range violations {
				if v.Severity == "error" {
					hasErrors = true
					break
				}
			}
			if tt.wantErr && !hasErrors {
				t.Errorf("Validate() expected errors but got none")
			}
			if !tt.wantErr && hasErrors {
				t.Errorf("Validate() unexpected errors: %v", violations)
			}
		})
	}
}

// TestAgentWorkbench_Submit_Approved 测试合规提交通过审核。
func TestAgentWorkbench_Submit_Approved(t *testing.T) {
	wb := NewDefaultAgentWorkbench(nil, nil, nil, nil)
	ctx := context.Background()

	sub := AgentCodeSubmission{
		AgentID: "agent-test-01",
		TaskID:  "task-001",
		SourceCode: `package main

import "go-core"

type myAtom struct{}
func (a *myAtom) Run(ctx context.Context, input int) (int, error) {
	return input * 2, nil
}

func main() {}`,
		Manifest: []PrimitiveManifest{
			{
				PrimitiveType: "Atom",
				Name:          "myAtom",
				Layer:         "L1",
				InputType:     "int",
				OutputType:    "int",
			},
		},
		Attempt: 1,
	}

	result, err := wb.Submit(ctx, sub)
	if err != nil {
		t.Fatalf("Submit() unexpected error: %v", err)
	}

	if result.Status != SubmissionApproved {
		t.Errorf("expected Approved, got %s", result.Status)
	}

	if result.SubmissionID != sub.SubmissionID() {
		t.Errorf("SubmissionID mismatch: %s vs %s", result.SubmissionID, sub.SubmissionID())
	}

	// 验证提交历史
	stored, ok := wb.GetSubmission(result.SubmissionID)
	if !ok {
		t.Error("GetSubmission() should find the submission")
	}
	if stored.Status != SubmissionApproved {
		t.Errorf("stored status = %s, want Approved", stored.Status)
	}
}

// TestAgentWorkbench_Submit_Rejected 测试违规提交被拒绝。
func TestAgentWorkbench_Submit_Rejected(t *testing.T) {
	wb := NewDefaultAgentWorkbench(nil, nil, nil, nil)
	ctx := context.Background()

	// 空 Manifest
	sub := AgentCodeSubmission{
		AgentID:    "agent-bad-01",
		TaskID:     "task-001",
		SourceCode: `package main`,
		Manifest:   []PrimitiveManifest{},
		Attempt:    1,
	}

	result, err := wb.Submit(ctx, sub)
	if err != nil {
		t.Fatalf("Submit() unexpected error: %v", err)
	}

	if result.Status != SubmissionRejected {
		t.Errorf("expected Rejected, got %s", result.Status)
	}

	if !result.HasErrors() {
		t.Error("expected HasErrors() = true")
	}

	if len(result.Violations) == 0 {
		t.Error("expected violations, got none")
	}

	// 验证违规包含修复建议
	for _, v := range result.Violations {
		if v.Suggestion == "" {
			t.Errorf("violation %s has no suggestion", v.Rule)
		}
	}
}

// TestAgentWorkbench_Submit_InvalidManifestType 测试非法原语类型。
func TestAgentWorkbench_Submit_InvalidManifestType(t *testing.T) {
	wb := NewDefaultAgentWorkbench(nil, nil, nil, nil)
	ctx := context.Background()

	sub := AgentCodeSubmission{
		AgentID: "agent-bad-02",
		TaskID:  "task-001",
		SourceCode: `package main

func main() {}`,
		Manifest: []PrimitiveManifest{
			{
				PrimitiveType: "Handler",
				Name:          "myHandler",
				Layer:         "L1",
				InputType:     "any",
				OutputType:    "any",
			},
		},
		Attempt: 1,
	}

	result, err := wb.Submit(ctx, sub)
	if err != nil {
		t.Fatalf("Submit() unexpected error: %v", err)
	}

	if result.Status != SubmissionRejected {
		t.Errorf("expected Rejected for invalid primitive type, got %s", result.Status)
	}
}

// TestAgentWorkbench_Submit_MissingAgentID 测试缺少 AgentID。
func TestAgentWorkbench_Submit_MissingAgentID(t *testing.T) {
	wb := NewDefaultAgentWorkbench(nil, nil, nil, nil)
	ctx := context.Background()

	sub := AgentCodeSubmission{
		AgentID:    "",
		TaskID:     "task-001",
		SourceCode: `package main`,
		Manifest: []PrimitiveManifest{
			{PrimitiveType: "Atom", Name: "test", Layer: "L1", InputType: "int", OutputType: "int"},
		},
		Attempt: 1,
	}

	result, err := wb.Submit(ctx, sub)
	if err != nil {
		t.Fatalf("Submit() unexpected error: %v", err)
	}

	if result.Status != SubmissionRejected {
		t.Errorf("expected Rejected for missing AgentID, got %s", result.Status)
	}
}

// TestAgentWorkbench_Submit_DuplicateManifestName 测试重复原语名称。
func TestAgentWorkbench_Submit_DuplicateManifestName(t *testing.T) {
	wb := NewDefaultAgentWorkbench(nil, nil, nil, nil)
	ctx := context.Background()

	sub := AgentCodeSubmission{
		AgentID: "agent-dup-01",
		TaskID:  "task-001",
		SourceCode: `package main

func main() {}`,
		Manifest: []PrimitiveManifest{
			{PrimitiveType: "Atom", Name: "sameName", Layer: "L1", InputType: "int", OutputType: "int"},
			{PrimitiveType: "Port", Name: "sameName", Layer: "L1", InputType: "int", OutputType: "int"},
		},
		Attempt: 1,
	}

	result, err := wb.Submit(ctx, sub)
	if err != nil {
		t.Fatalf("Submit() unexpected error: %v", err)
	}

	if result.Status != SubmissionRejected {
		t.Errorf("expected Rejected for duplicate manifest name, got %s", result.Status)
	}
}

// TestAgentWorkbench_SubmitAndRun 测试 SubmitAndRun 基本流程。
func TestAgentWorkbench_SubmitAndRun(t *testing.T) {
	wb := NewDefaultAgentWorkbench(nil, nil, nil, nil)
	ctx := context.Background()

	sub := AgentCodeSubmission{
		AgentID: "agent-test-01",
		TaskID:  "task-001",
		SourceCode: `package main

import "go-core"

func main() {}`,
		Manifest: []PrimitiveManifest{
			{PrimitiveType: "Atom", Name: "testAtom", Layer: "L1", InputType: "int", OutputType: "int"},
		},
		Attempt: 1,
	}

	result, err := wb.SubmitAndRun(ctx, sub)
	if err != nil {
		t.Fatalf("SubmitAndRun() unexpected error: %v", err)
	}

	if result.Status != SubmissionApproved {
		t.Errorf("expected Approved, got %s", result.Status)
	}

	// P1 阶段：ExecutionSteps 应包含 placeholder
	if len(result.ExecutionSteps) == 0 {
		t.Error("expected ExecutionSteps, got none")
	}
}

// TestAgentWorkbench_SubmitAndRun_Rejected 测试 SubmitAndRun 被拒绝时不会执行。
func TestAgentWorkbench_SubmitAndRun_Rejected(t *testing.T) {
	wb := NewDefaultAgentWorkbench(nil, nil, nil, nil)
	ctx := context.Background()

	sub := AgentCodeSubmission{
		AgentID:    "",
		TaskID:     "",
		SourceCode: "",
		Manifest:   []PrimitiveManifest{},
		Attempt:    1,
	}

	result, err := wb.SubmitAndRun(ctx, sub)
	if err != nil {
		t.Fatalf("SubmitAndRun() unexpected error: %v", err)
	}

	if result.Status != SubmissionRejected {
		t.Errorf("expected Rejected, got %s", result.Status)
	}

	// 被拒绝时不应有 ExecutionSteps
	if len(result.ExecutionSteps) != 0 {
		t.Errorf("expected no ExecutionSteps when rejected, got %d", len(result.ExecutionSteps))
	}
}

// TestAgentWorkbench_ListSubmissionsByAgent 测试按 Agent 查询提交历史。
func TestAgentWorkbench_ListSubmissionsByAgent(t *testing.T) {
	wb := NewDefaultAgentWorkbench(nil, nil, nil, nil)
	ctx := context.Background()

	// 提交 Agent A 的 2 个任务
	subA1 := AgentCodeSubmission{
		AgentID: "agent-a", TaskID: "task-1",
		SourceCode: `package main`,
		Manifest:   []PrimitiveManifest{{PrimitiveType: "Atom", Name: "a1", Layer: "L1", InputType: "int", OutputType: "int"}},
		Attempt:    1,
	}
	subA2 := AgentCodeSubmission{
		AgentID: "agent-a", TaskID: "task-2",
		SourceCode: `package main`,
		Manifest:   []PrimitiveManifest{{PrimitiveType: "Atom", Name: "a2", Layer: "L1", InputType: "int", OutputType: "int"}},
		Attempt:    1,
	}

	// 提交 Agent B 的 1 个任务
	subB1 := AgentCodeSubmission{
		AgentID: "agent-b", TaskID: "task-1",
		SourceCode: `package main`,
		Manifest:   []PrimitiveManifest{{PrimitiveType: "Atom", Name: "b1", Layer: "L1", InputType: "int", OutputType: "int"}},
		Attempt:    1,
	}

	wb.Submit(ctx, subA1)
	wb.Submit(ctx, subA2)
	wb.Submit(ctx, subB1)

	resultsA := wb.ListSubmissionsByAgent("agent-a")
	if len(resultsA) != 2 {
		t.Errorf("expected 2 submissions for agent-a, got %d", len(resultsA))
	}

	resultsB := wb.ListSubmissionsByAgent("agent-b")
	if len(resultsB) != 1 {
		t.Errorf("expected 1 submission for agent-b, got %d", len(resultsB))
	}
}

// TestAgentWorkbench_Submit_ContextCancellation 测试 Context 取消。
func TestAgentWorkbench_Submit_ContextCancellation(t *testing.T) {
	wb := NewDefaultAgentWorkbench(nil, nil, nil, nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 立即取消

	sub := AgentCodeSubmission{
		AgentID: "agent-test", TaskID: "task-001",
		SourceCode: `package main`,
		Manifest:   []PrimitiveManifest{{PrimitiveType: "Atom", Name: "test", Layer: "L1", InputType: "int", OutputType: "int"}},
		Attempt:    1,
	}

	_, err := wb.Submit(ctx, sub)
	if err == nil {
		t.Error("expected error for cancelled context")
	}
}

// TestAgentWorkbench_Submit_Concurrent 测试并发提交。
func TestAgentWorkbench_Submit_Concurrent(t *testing.T) {
	wb := NewDefaultAgentWorkbench(nil, nil, nil, nil)
	ctx := context.Background()

	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(idx int) {
			sub := AgentCodeSubmission{
				AgentID:    "agent-concurrent",
				TaskID:     "task-001",
				SourceCode: `package main`,
				Manifest: []PrimitiveManifest{
					{PrimitiveType: "Atom", Name: "test", Layer: "L1", InputType: "int", OutputType: "int"},
				},
				Attempt:    idx + 1,
				SubmittedAt: time.Now(),
			}
			_, err := wb.Submit(ctx, sub)
			if err != nil {
				t.Errorf("concurrent Submit() error: %v", err)
			}
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	// 验证所有提交都已记录
	results := wb.ListSubmissionsByAgent("agent-concurrent")
	if len(results) != 10 {
		t.Errorf("expected 10 submissions, got %d", len(results))
	}
}

// TestSubmissionResult_HasErrors 测试 HasErrors 方法。
func TestSubmissionResult_HasErrors(t *testing.T) {
	tests := []struct {
		name     string
		result   SubmissionResult
		expected bool
	}{
		{
			name: "no violations",
			result:   SubmissionResult{Violations: nil},
			expected: false,
		},
		{
			name: "only warnings",
			result: SubmissionResult{Violations: []Violation{
				{Rule: "test", Severity: "warn", Detail: "test"},
			}},
			expected: false,
		},
		{
			name: "has errors",
			result: SubmissionResult{Violations: []Violation{
				{Rule: "test", Severity: "error", Detail: "test"},
			}},
			expected: true,
		},
		{
			name: "mixed",
			result: SubmissionResult{Violations: []Violation{
				{Rule: "warn1", Severity: "warn", Detail: "test"},
				{Rule: "err1", Severity: "error", Detail: "test"},
			}},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.result.HasErrors(); got != tt.expected {
				t.Errorf("HasErrors() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// TestValidPrimitiveTypes 测试合法原语类型集合。
func TestValidPrimitiveTypes(t *testing.T) {
	validTypes := []string{"Atom", "Port", "Adapter", "Composer"}
	for _, pt := range validTypes {
		if !ValidPrimitiveTypes[pt] {
			t.Errorf("expected %s to be valid primitive type", pt)
		}
	}

	invalidTypes := []string{"", "Handler", "Service", "Controller", "Function"}
	for _, pt := range invalidTypes {
		if ValidPrimitiveTypes[pt] {
			t.Errorf("expected %s to be invalid primitive type", pt)
		}
	}
}