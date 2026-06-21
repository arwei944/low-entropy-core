//go:build lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

// Package core — Agent 编译执行管道 (Phase 2 P3)
//
// AgentRunner 负责编译和执行 Agent 提交的代码。
// Agent 写的代码和普通 Go 代码一样，通过 go build 编译，
// 在框架的 Composer 运行时中执行，每一步自动产生 ExecutionStep。
//
// 流程：
//   1. 将 SourceCode 写入临时 Go 文件
//   2. go build 编译
//   3. 执行编译产物
//   4. 每一步产生 ExecutionStep，被 Observation 收集
package core

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// ──────────────────────────────────────────────
// SECTION 1: AgentRunner — 编译执行器
// ──────────────────────────────────────────────

// AgentRunner 负责编译和执行 Agent 提交的代码。
type AgentRunner struct {
	workDir string             // 临时工作目录
	obs     ObservationAdapter // 已有：观察适配器
}

// NewAgentRunner 创建 AgentRunner。
func NewAgentRunner(workDir string, obs ObservationAdapter) *AgentRunner {
	if obs == nil {
		obs = &NoOpObservationAdapter{}
	}
	if workDir == "" {
		workDir = os.TempDir()
	}
	return &AgentRunner{
		workDir: workDir,
		obs:     obs,
	}
}

// BuildAndRun 编译并执行 Agent 提交的代码。
//
// 流程：
//   1. 将 SourceCode 写入临时 Go 文件
//   2. go build 编译
//   3. 执行编译产物
//   4. 每一步产生 ExecutionStep
//
// 编译失败时返回编译错误（Agent 据此修改代码后重新提交）。
// 运行时 panic 被捕获为 ExecutionStep（失败），不会崩溃。
func (r *AgentRunner) BuildAndRun(ctx context.Context, submission AgentCodeSubmission) ([]ExecutionStep, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	steps := make([]ExecutionStep, 0, 4)

	// Step 1: 写入临时文件
	tmpDir := filepath.Join(r.workDir, submission.SubmissionID())
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		return steps, fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	mainFile := filepath.Join(tmpDir, "main.go")
	if err := os.WriteFile(mainFile, []byte(submission.SourceCode), 0644); err != nil {
		return steps, fmt.Errorf("write temp file: %w", err)
	}

	es1 := NewExecutionStep("AgentRunner", "WriteTempFile", "source code written to temp file", "Adapter")
	es1.Metadata = map[string]any{
		"file": mainFile,
		"size": len(submission.SourceCode),
	}
	steps = append(steps, es1)

	// Step 2: go build 编译
	output := filepath.Join(tmpDir, "agent_bin.exe")
	buildStart := time.Now()

	// 先初始化 go.mod
	modInit := exec.CommandContext(ctx, "go", "mod", "init", "agent_"+submission.AgentID)
	modInit.Dir = tmpDir
	modInit.CombinedOutput() // 忽略错误，可能已存在

	// 添加 go-core 依赖
	_ = os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte("module agent_"+submission.AgentID+"\n\ngo 1.21\n"), 0644)

	buildCmd := exec.CommandContext(ctx, "go", "build", "-o", output, mainFile)
	buildCmd.Dir = tmpDir
	buildOut, buildErr := buildCmd.CombinedOutput()

	buildDuration := time.Since(buildStart).Milliseconds()

	if buildErr != nil {
		es2 := NewExecutionStep("AgentRunner", "Compile", "compilation failed", "Adapter")
		es2.Error = NewStepError("COMPILE_ERROR", fmt.Sprintf("%v\n%s", buildErr, string(buildOut)), false)
		es2.DurationMs = buildDuration
		steps = append(steps, es2)
		r.obs.Record(steps)
		return steps, fmt.Errorf("compile error: %v\n%s", buildErr, string(buildOut))
	}

	es2 := NewExecutionStep("AgentRunner", "Compile", "compilation succeeded", "Adapter")
	es2.DurationMs = buildDuration
	es2.Metadata = map[string]any{
		"output": output,
	}
	steps = append(steps, es2)

	// Step 3: 执行
	runStart := time.Now()
	runCmd := exec.CommandContext(ctx, output)
	runCmd.Dir = tmpDir
	runOut, runErr := runCmd.CombinedOutput()
	runDuration := time.Since(runStart).Milliseconds()

	if runErr != nil {
		es3 := NewExecutionStep("AgentRunner", "Execute", "execution failed", "Composer")
		es3.Error = NewStepError("RUNTIME_ERROR", fmt.Sprintf("%v\n%s", runErr, string(runOut)), false)
		es3.DurationMs = runDuration
		es3.Metadata = map[string]any{
			"output": string(runOut),
		}
		steps = append(steps, es3)
		r.obs.Record(steps)
		return steps, fmt.Errorf("execute error: %v\n%s", runErr, string(runOut))
	}

	es3 := NewExecutionStep("AgentRunner", "Execute", "execution completed", "Composer")
	es3.DurationMs = runDuration
	es3.Metadata = map[string]any{
		"output": strings.TrimSpace(string(runOut)),
	}
	steps = append(steps, es3)

	r.obs.Record(steps)
	return steps, nil
}

// Compile 仅编译，不执行（用于预检查）。
func (r *AgentRunner) Compile(ctx context.Context, submission AgentCodeSubmission) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	tmpDir := filepath.Join(r.workDir, submission.SubmissionID()+"_compile")
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		return "", fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	mainFile := filepath.Join(tmpDir, "main.go")
	if err := os.WriteFile(mainFile, []byte(submission.SourceCode), 0644); err != nil {
		return "", fmt.Errorf("write temp file: %w", err)
	}

	// 初始化 go.mod
	_ = os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte("module agent_"+submission.AgentID+"\n\ngo 1.21\n"), 0644)

	output := filepath.Join(tmpDir, "agent_bin.exe")
	buildCmd := exec.CommandContext(ctx, "go", "build", "-o", output, mainFile)
	buildCmd.Dir = tmpDir
	buildOut, buildErr := buildCmd.CombinedOutput()

	if buildErr != nil {
		return "", fmt.Errorf("compile error: %v\n%s", buildErr, string(buildOut))
	}

	return output, nil
}