//go:build lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

package core

import (
	"context"
	"testing"
)

// ──────────────────────────────────────────────
// P2 测试: StaticGuardPort
// ──────────────────────────────────────────────

// 创建测试用的 StaticGuardPort
func newTestStaticGuard() *StaticGuardPort {
	return NewStaticGuardPort(nil, nil, nil)
}

// TestStaticGuardPort_CompliantCode 测试合规代码通过审核。
func TestStaticGuardPort_CompliantCode(t *testing.T) {
	g := newTestStaticGuard()

	sub := AgentCodeSubmission{
		AgentID: "agent-test",
		TaskID:  "task-001",
		SourceCode: `package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"go-core"
)

type validateRegisterPort struct{}
func (p *validateRegisterPort) Run(ctx context.Context, req RegisterReq) (RegisterReq, error) {
	if len(req.Username) < 3 {
		return req, core.NewStepError("INVALID", "too short", false)
	}
	return req, nil
}

type hashPasswordAtom struct{}
func (a *hashPasswordAtom) Run(ctx context.Context, req RegisterReq) (User, error) {
	h := sha256.Sum256([]byte(req.Password))
	return User{Username: req.Username, PasswordHash: hex.EncodeToString(h[:])}, nil
}

type saveUserAdapter struct{
	connString string
}
func (a *saveUserAdapter) Execute(ctx context.Context, user User) error {
	// 在 Adapter 内部使用标准库是合法的
	_ = a.connString
	return nil
}

type RegisterReq struct {
	Username string
	Password string
}

type User struct {
	Username     string
	PasswordHash string
}

func main() {}`,
		Manifest: []PrimitiveManifest{
			{PrimitiveType: "Port",    Name: "validateRegisterPort", Layer: "L1", InputType: "RegisterReq", OutputType: "RegisterReq"},
			{PrimitiveType: "Atom",    Name: "hashPasswordAtom",     Layer: "L1", InputType: "RegisterReq", OutputType: "User"},
			{PrimitiveType: "Adapter", Name: "saveUserAdapter",      Layer: "L7", InputType: "User",        OutputType: "error"},
		},
		Attempt: 1,
	}

	result, err := g.Validate(context.Background(), sub)
	if err != nil {
		t.Fatalf("Validate() unexpected error: %v", err)
	}

	if result.HasErrors() {
		t.Errorf("expected no errors but got: %v", result.Violations)
	}
}

// TestStaticGuardPort_ExternalDependency 测试外部依赖被拦截。
func TestStaticGuardPort_ExternalDependency(t *testing.T) {
	g := newTestStaticGuard()

	sub := AgentCodeSubmission{
		AgentID: "agent-bad",
		TaskID:  "task-001",
		SourceCode: `package main

import (
	"go-core"
	"github.com/gin-gonic/gin"
)

type myHandler struct{}
func (h *myHandler) Run(ctx context.Context, input int) (int, error) {
	return input, nil
}

func main() {}`,
		Manifest: []PrimitiveManifest{
			{PrimitiveType: "Atom", Name: "myHandler", Layer: "L1", InputType: "int", OutputType: "int"},
		},
		Attempt: 1,
	}

	result, err := g.Validate(context.Background(), sub)
	if err != nil {
		t.Fatalf("Validate() unexpected error: %v", err)
	}

	if !result.HasErrors() {
		t.Error("expected errors for external dependency, got none")
	}

	// 验证违规包含修复建议
	hasExternalDepViolation := false
	for _, v := range result.Violations {
		if v.Rule == "external_dependency" {
			hasExternalDepViolation = true
			if v.Suggestion == "" {
				t.Error("external_dependency violation should have suggestion")
			}
		}
	}
	if !hasExternalDepViolation {
		t.Error("expected external_dependency violation")
	}
}

// TestStaticGuardPort_InvalidLayer 测试非法层级被拦截。
func TestStaticGuardPort_InvalidLayer(t *testing.T) {
	g := newTestStaticGuard()

	sub := AgentCodeSubmission{
		AgentID: "agent-bad",
		TaskID:  "task-001",
		SourceCode: `package main

type myAtom struct{}
func (a *myAtom) Run(ctx context.Context, input int) (int, error) {
	return input, nil
}

func main() {}`,
		Manifest: []PrimitiveManifest{
			{PrimitiveType: "Atom", Name: "myAtom", Layer: "L99", InputType: "int", OutputType: "int"},
		},
		Attempt: 1,
	}

	result, err := g.Validate(context.Background(), sub)
	if err != nil {
		t.Fatalf("Validate() unexpected error: %v", err)
	}

	if !result.HasErrors() {
		t.Error("expected errors for invalid layer, got none")
	}

	hasInvalidLayerViolation := false
	for _, v := range result.Violations {
		if v.Rule == "invalid_layer" {
			hasInvalidLayerViolation = true
		}
	}
	if !hasInvalidLayerViolation {
		t.Error("expected invalid_layer violation")
	}
}

// TestStaticGuardPort_LayerMismatch 测试原语类型与层级不匹配。
func TestStaticGuardPort_LayerMismatch(t *testing.T) {
	g := newTestStaticGuard()

	sub := AgentCodeSubmission{
		AgentID: "agent-bad",
		TaskID:  "task-001",
		SourceCode: `package main

type myAdapter struct{}
func (a *myAdapter) Execute(ctx context.Context, input int) (int, error) {
	return input, nil
}

func main() {}`,
		Manifest: []PrimitiveManifest{
			// Adapter 只能在 L5-L7，放在 L1 违规
			{PrimitiveType: "Adapter", Name: "myAdapter", Layer: "L1", InputType: "int", OutputType: "int"},
		},
		Attempt: 1,
	}

	result, err := g.Validate(context.Background(), sub)
	if err != nil {
		t.Fatalf("Validate() unexpected error: %v", err)
	}

	if !result.HasErrors() {
		t.Error("expected errors for layer mismatch, got none")
	}

	hasLayerMismatchViolation := false
	for _, v := range result.Violations {
		if v.Rule == "layer_mismatch" {
			hasLayerMismatchViolation = true
			if v.Suggestion == "" {
				t.Error("layer_mismatch violation should have suggestion")
			}
		}
	}
	if !hasLayerMismatchViolation {
		t.Error("expected layer_mismatch violation")
	}
}

// TestStaticGuardPort_PortInCorrectLayer 测试 Port 在合法层级和非法层级。
func TestStaticGuardPort_PortInCorrectLayer(t *testing.T) {
	g := newTestStaticGuard()

	// Port 在 L1-L3 合法
	sub := AgentCodeSubmission{
		AgentID: "agent-test",
		TaskID:  "task-001",
		SourceCode: `package main

type myPort struct{}
func (p *myPort) Run(ctx context.Context, input int) (int, error) {
	return input, nil
}

func main() {}`,
		Manifest: []PrimitiveManifest{
			{PrimitiveType: "Port", Name: "myPort", Layer: "L1", InputType: "int", OutputType: "int"},
		},
		Attempt: 1,
	}

	result, err := g.Validate(context.Background(), sub)
	if err != nil {
		t.Fatalf("Validate() unexpected error: %v", err)
	}

	if result.HasErrors() {
		t.Errorf("expected no errors for Port in L1, got: %v", result.Violations)
	}

	// Port 在 L7 非法
	sub2 := AgentCodeSubmission{
		AgentID: "agent-bad",
		TaskID:  "task-002",
		SourceCode: `package main

type myPort struct{}
func (p *myPort) Run(ctx context.Context, input int) (int, error) {
	return input, nil
}

func main() {}`,
		Manifest: []PrimitiveManifest{
			{PrimitiveType: "Port", Name: "myPort", Layer: "L7", InputType: "int", OutputType: "int"},
		},
		Attempt: 1,
	}

	result2, err := g.Validate(context.Background(), sub2)
	if err != nil {
		t.Fatalf("Validate() unexpected error: %v", err)
	}

	if !result2.HasErrors() {
		t.Error("expected errors for Port in L7, got none")
	}
}

// TestStaticGuardPort_ManifestMismatch 测试 Manifest 与代码不一致。
func TestStaticGuardPort_ManifestMismatch(t *testing.T) {
	g := newTestStaticGuard()

	// Manifest 声明了 a1，但代码中只有 a2
	sub := AgentCodeSubmission{
		AgentID: "agent-bad",
		TaskID:  "task-001",
		SourceCode: `package main

type myAtom2 struct{}
func (a *myAtom2) Run(ctx context.Context, input int) (int, error) {
	return input, nil
}

func main() {}`,
		Manifest: []PrimitiveManifest{
			{PrimitiveType: "Atom", Name: "myAtom1", Layer: "L1", InputType: "int", OutputType: "int"},
		},
		Attempt: 1,
	}

	result, err := g.Validate(context.Background(), sub)
	if err != nil {
		t.Fatalf("Validate() unexpected error: %v", err)
	}

	// 应该有 manifest_mismatch（声明了 myAtom1 但代码中没有）
	hasMismatch := false
	for _, v := range result.Violations {
		if v.Rule == "manifest_mismatch" {
			hasMismatch = true
		}
	}
	if !hasMismatch {
		t.Error("expected manifest_mismatch violation")
	}

	// 应该有 manifest_missing（代码中有 myAtom2 但未声明）
	hasMissing := false
	for _, v := range result.Violations {
		if v.Rule == "manifest_missing" {
			hasMissing = true
		}
	}
	if !hasMissing {
		t.Error("expected manifest_missing violation")
	}
}

// TestStaticGuardPort_ComplexityWarnings 测试复杂度超标产生 warn。
func TestStaticGuardPort_ComplexityWarnings(t *testing.T) {
	g := newTestStaticGuard()

	// 构造一个超长函数（> 50 行）
	sub := AgentCodeSubmission{
		AgentID: "agent-test",
		TaskID:  "task-001",
		SourceCode: `package main

import "context"

type longAtom struct{}
func (a *longAtom) Run(ctx context.Context, input int) (int, error) {
	// line 1
	x := input
	// line 2
	x = x + 1
	// line 3
	x = x + 1
	// line 4
	x = x + 1
	// line 5
	x = x + 1
	// line 6
	x = x + 1
	// line 7
	x = x + 1
	// line 8
	x = x + 1
	// line 9
	x = x + 1
	// line 10
	x = x + 1
	// line 11
	x = x + 1
	// line 12
	x = x + 1
	// line 13
	x = x + 1
	// line 14
	x = x + 1
	// line 15
	x = x + 1
	// line 16
	x = x + 1
	// line 17
	x = x + 1
	// line 18
	x = x + 1
	// line 19
	x = x + 1
	// line 20
	x = x + 1
	// line 21
	x = x + 1
	// line 22
	x = x + 1
	// line 23
	x = x + 1
	// line 24
	x = x + 1
	// line 25
	x = x + 1
	// line 26
	x = x + 1
	// line 27
	x = x + 1
	// line 28
	x = x + 1
	// line 29
	x = x + 1
	// line 30
	x = x + 1
	// line 31
	x = x + 1
	// line 32
	x = x + 1
	// line 33
	x = x + 1
	// line 34
	x = x + 1
	// line 35
	x = x + 1
	// line 36
	x = x + 1
	// line 37
	x = x + 1
	// line 38
	x = x + 1
	// line 39
	x = x + 1
	// line 40
	x = x + 1
	// line 41
	x = x + 1
	// line 42
	x = x + 1
	// line 43
	x = x + 1
	// line 44
	x = x + 1
	// line 45
	x = x + 1
	// line 46
	x = x + 1
	// line 47
	x = x + 1
	// line 48
	x = x + 1
	// line 49
	x = x + 1
	// line 50
	x = x + 1
	// line 51
	x = x + 1
	// line 52
	return x, nil
}

func main() {}`,
		Manifest: []PrimitiveManifest{
			{PrimitiveType: "Atom", Name: "longAtom", Layer: "L1", InputType: "int", OutputType: "int"},
		},
		Attempt: 1,
	}

	result, err := g.Validate(context.Background(), sub)
	if err != nil {
		t.Fatalf("Validate() unexpected error: %v", err)
	}

	// 复杂度是 warn 级别，不应有 error
	if result.HasErrors() {
		t.Errorf("complexity violations should be warnings, not errors: %v", result.Violations)
	}

	// 应该有 function_too_long 违规
	hasTooLong := false
	for _, v := range result.Violations {
		if v.Rule == "function_too_long" {
			hasTooLong = true
		}
	}
	if !hasTooLong {
		t.Error("expected function_too_long violation")
	}
}

// TestStaticGuardPort_ParseError 测试语法错误被捕获。
func TestStaticGuardPort_ParseError(t *testing.T) {
	g := newTestStaticGuard()

	sub := AgentCodeSubmission{
		AgentID: "agent-bad",
		TaskID:  "task-001",
		SourceCode: `package main

func main() { // 缺少闭合括号
`,
		Manifest: []PrimitiveManifest{
			{PrimitiveType: "Atom", Name: "test", Layer: "L1", InputType: "int", OutputType: "int"},
		},
		Attempt: 1,
	}

	result, err := g.Validate(context.Background(), sub)
	if err != nil {
		t.Fatalf("Validate() unexpected error: %v", err)
	}

	if !result.HasErrors() {
		t.Error("expected parse_error for malformed code")
	}

	hasParseError := false
	for _, v := range result.Violations {
		if v.Rule == "parse_error" {
			hasParseError = true
		}
	}
	if !hasParseError {
		t.Error("expected parse_error violation")
	}
}

// TestStaticGuardPort_EmptySourceCode 测试空代码。
func TestStaticGuardPort_EmptySourceCode(t *testing.T) {
	g := newTestStaticGuard()

	sub := AgentCodeSubmission{
		AgentID:    "agent-bad",
		TaskID:     "task-001",
		SourceCode: "",
		Manifest: []PrimitiveManifest{
			{PrimitiveType: "Atom", Name: "test", Layer: "L1", InputType: "int", OutputType: "int"},
		},
		Attempt: 1,
	}

	result, err := g.Validate(context.Background(), sub)
	if err != nil {
		t.Fatalf("Validate() unexpected error: %v", err)
	}

	if !result.HasErrors() {
		t.Error("expected parse_error for empty code")
	}
}

// TestStaticGuardPort_AllStandardLibImports 测试所有标准库 import 被接受。
func TestStaticGuardPort_AllStandardLibImports(t *testing.T) {
	g := newTestStaticGuard()

	sub := AgentCodeSubmission{
		AgentID: "agent-test",
		TaskID:  "task-001",
		SourceCode: `package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"go-core"
	"math"
	"strings"
	"sync"
	"time"
)

type myAtom struct{}
func (a *myAtom) Run(ctx context.Context, input int) (int, error) {
	return input, nil
}

func main() {}`,
		Manifest: []PrimitiveManifest{
			{PrimitiveType: "Atom", Name: "myAtom", Layer: "L1", InputType: "int", OutputType: "int"},
		},
		Attempt: 1,
	}

	result, err := g.Validate(context.Background(), sub)
	if err != nil {
		t.Fatalf("Validate() unexpected error: %v", err)
	}

	if result.HasErrors() {
		t.Errorf("standard library imports should be allowed, got: %v", result.Violations)
	}
}

// TestStaticGuardPort_ContextCancellation 测试 Context 取消。
func TestStaticGuardPort_ContextCancellation(t *testing.T) {
	g := newTestStaticGuard()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	sub := AgentCodeSubmission{
		AgentID: "agent-test",
		TaskID:  "task-001",
		SourceCode: `package main
func main() {}`,
		Manifest: []PrimitiveManifest{
			{PrimitiveType: "Atom", Name: "test", Layer: "L1", InputType: "int", OutputType: "int"},
		},
		Attempt: 1,
	}

	_, err := g.Validate(ctx, sub)
	if err == nil {
		t.Error("expected error for cancelled context")
	}
}

// TestStaticGuardPortAsStep 测试包装为 Step。
func TestStaticGuardPortAsStep(t *testing.T) {
	g := newTestStaticGuard()
	step := StaticGuardPortAsStep(g)

	sub := AgentCodeSubmission{
		AgentID: "agent-test",
		TaskID:  "task-001",
		SourceCode: `package main
type myAtom struct{}
func (a *myAtom) Run(ctx context.Context, input int) (int, error) { return input, nil }
func main() {}`,
		Manifest: []PrimitiveManifest{
			{PrimitiveType: "Atom", Name: "myAtom", Layer: "L1", InputType: "int", OutputType: "int"},
		},
		Attempt: 1,
	}

	result, err := step.Execute(context.Background(), sub)
	if err != nil {
		t.Fatalf("step.Execute() unexpected error: %v", err)
	}

	if result.HasErrors() {
		t.Errorf("expected no errors, got: %v", result.Violations)
	}
}

// TestStaticReviewResult_HasErrors 测试 StaticReviewResult.HasErrors()。
func TestStaticReviewResult_HasErrors(t *testing.T) {
	tests := []struct {
		name     string
		result   StaticReviewResult
		expected bool
	}{
		{"no violations", StaticReviewResult{Violations: nil}, false},
		{"only warnings", StaticReviewResult{Violations: []Violation{
			{Rule: "test", Severity: "warn", Detail: "test"},
		}}, false},
		{"has errors", StaticReviewResult{Violations: []Violation{
			{Rule: "test", Severity: "error", Detail: "test"},
		}}, true},
		{"mixed", StaticReviewResult{Violations: []Violation{
			{Rule: "warn1", Severity: "warn", Detail: "test"},
			{Rule: "err1", Severity: "error", Detail: "test"},
		}}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.result.HasErrors(); got != tt.expected {
				t.Errorf("HasErrors() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// TestStaticReviewResult_ErrorCount 测试 ErrorCount()。
func TestStaticReviewResult_ErrorCount(t *testing.T) {
	tests := []struct {
		name     string
		result   StaticReviewResult
		expected int
	}{
		{"no violations", StaticReviewResult{Violations: nil}, 0},
		{"only warnings", StaticReviewResult{Violations: []Violation{
			{Rule: "test", Severity: "warn", Detail: "test"},
		}}, 0},
		{"one error", StaticReviewResult{Violations: []Violation{
			{Rule: "test", Severity: "error", Detail: "test"},
		}}, 1},
		{"mixed", StaticReviewResult{Violations: []Violation{
			{Rule: "warn1", Severity: "warn", Detail: "test"},
			{Rule: "err1", Severity: "error", Detail: "test"},
			{Rule: "err2", Severity: "error", Detail: "test"},
		}}, 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.result.ErrorCount(); got != tt.expected {
				t.Errorf("ErrorCount() = %d, want %d", got, tt.expected)
			}
		})
	}
}

// TestAllowedImportPaths 测试白名单import路径。
func TestAllowedImportPaths(t *testing.T) {
	allowed := []string{
		"go-core", "fmt", "context", "sync", "time", "strings",
		"crypto/sha256", "encoding/json", "database/sql",
		"net/http", "os", "math", "errors",
	}
	for _, path := range allowed {
		if !isAllowedImport(path) {
			t.Errorf("expected %s to be allowed", path)
		}
	}

	disallowed := []string{
		"github.com/gin-gonic/gin",
		"github.com/gorilla/mux",
		"google.golang.org/grpc",
		"gopkg.in/yaml.v3",
	}
	for _, path := range disallowed {
		if isAllowedImport(path) {
			t.Errorf("expected %s to be disallowed", path)
		}
	}
}

// TestLayerOrder 测试层级顺序。
func TestLayerOrder(t *testing.T) {
	tests := []struct {
		layer string
		want  int
	}{
		{"L1", 1}, {"L2", 2}, {"L3", 3}, {"L4", 4},
		{"L5", 5}, {"L6", 6}, {"L7", 7},
		{"unknown", 0},
	}
	for _, tt := range tests {
		if got := layerOrder(tt.layer); got != tt.want {
			t.Errorf("layerOrder(%s) = %d, want %d", tt.layer, got, tt.want)
		}
	}
}

// TestFindManifest 测试查找 Manifest。
func TestFindManifest(t *testing.T) {
	manifest := []PrimitiveManifest{
		{Name: "atom1", PrimitiveType: "Atom", Layer: "L1"},
		{Name: "port1", PrimitiveType: "Port", Layer: "L1"},
		{Name: "adapter1", PrimitiveType: "Adapter", Layer: "L7"},
	}

	if found := findManifest(manifest, "atom1"); found == nil {
		t.Error("should find atom1")
	}
	if found := findManifest(manifest, "nonexistent"); found != nil {
		t.Error("should not find nonexistent")
	}
	if found := findManifest(manifest, "port1"); found.PrimitiveType != "Port" {
		t.Errorf("expected Port, got %s", found.PrimitiveType)
	}
}

// TestDecisionEngine_StaticReview 测试 DecisionEngine 处理静态审核结果。
func TestDecisionEngine_StaticReview(t *testing.T) {
	engine := NewDecisionEngine(nil)

	// 静态审核有 1 个 error → Block
	input := GuardianInput{
		StaticReviewResult: StaticReviewResult{
			Violations: []Violation{
				{Rule: "external_dependency", Severity: "error", Detail: "test"},
			},
		},
	}
	decision := engine.evaluate(input)
	if decision.Action != ActionBlock {
		t.Errorf("expected Block for 1 static error, got %s", decision.Action)
	}

	// 静态审核有 3 个 error → Rollback
	input2 := GuardianInput{
		StaticReviewResult: StaticReviewResult{
			Violations: []Violation{
				{Rule: "e1", Severity: "error", Detail: "test"},
				{Rule: "e2", Severity: "error", Detail: "test"},
				{Rule: "e3", Severity: "error", Detail: "test"},
			},
		},
	}
	decision2 := engine.evaluate(input2)
	if decision2.Action != ActionRollback {
		t.Errorf("expected Rollback for 3 static errors, got %s", decision2.Action)
	}

	// 静态审核无 error → Allow（其他维度也正常时）
	input3 := GuardianInput{
		TranspAlert: TransparencyAlert{IsHealthy: true},
	}
	decision3 := engine.evaluate(input3)
	if decision3.Action != ActionAllow {
		t.Errorf("expected Allow for clean static review, got %s (reason: %s)", decision3.Action, decision3.Reason)
	}
}

// TestDecisionEngine_StaticReviewWarnOnly 测试 warn 级别不阻断。
func TestDecisionEngine_StaticReviewWarnOnly(t *testing.T) {
	engine := NewDecisionEngine(nil)

	input := GuardianInput{
		TranspAlert: TransparencyAlert{IsHealthy: true},
		StaticReviewResult: StaticReviewResult{
			Violations: []Violation{
				{Rule: "function_too_long", Severity: "warn", Detail: "test"},
				{Rule: "high_complexity", Severity: "warn", Detail: "test"},
			},
		},
	}
	decision := engine.evaluate(input)
	if decision.Action != ActionAllow {
		t.Errorf("expected Allow for warn-only violations, got %s", decision.Action)
	}
}