// Package main — Agent Demo 业务逻辑处理器。
package main

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// ──────────────────────────────────────────────
// init — 初始化 Agent 环境
// ──────────────────────────────────────────────

func init() {
	globalMetrics.StartTime = time.Now()
	fmt.Printf("[Agent] Initializing %s with model %s\n", defaultConfig.AgentName, defaultConfig.Model)
}

// ──────────────────────────────────────────────
// submitCode — 提交代码到执行环境
// ──────────────────────────────────────────────

// submitCode 向 Agent 提交代码，返回执行结果。
func submitCode(ctx context.Context, submission CodeSubmission) (string, error) {
	taskID := submission.TaskID
	code := submission.Code

	fmt.Printf("[Submit] Task %s: Submitting %d bytes of %s code\n",
		taskID, len(code), submission.Language)

	// 模拟 Agent 执行：实际通过 MCP 与 Agent 通信
	output, err := agentExecute(ctx, code, submission.Language)
	if err != nil {
		return "", fmt.Errorf("agent execution failed: %w", err)
	}

	// 流式输出处理（goroutine 由 caller 的 ctx 管理生命周期）
	go streamOutput(ctx, output, taskID)

	return output, nil
}

// agentExecute 模拟 Agent 执行代码。
// 实际通过 MCP (Model Context Protocol) 与 AI Agent 通信。
func agentExecute(ctx context.Context, code string, language string) (string, error) {
	// 模拟网络延迟
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case <-time.After(500 * time.Millisecond):
	}

	if strings.Contains(code, "error") {
		return "", fmt.Errorf("execution error: code contains 'error' keyword")
	}

	return fmt.Sprintf("[%s] Code executed successfully:\n%s\n[Exit Code] 0", language, code[:min(len(code), 100)]), nil
}

// streamOutput 流式输出到终端。
// goroutine 由 ctx 管理生命周期，ctx 取消时立即退出。
func streamOutput(ctx context.Context, output, taskID string) {
	for i := 0; i < len(output); i += 100 {
		select {
		case <-ctx.Done():
			return
		default:
			chunk := output[i:min(i+100, len(output))]
			fmt.Printf("[Output|%s] %s\n", taskID, chunk)
			time.Sleep(50 * time.Millisecond)
		}
	}
}

// ──────────────────────────────────────────────
// displayTaskProgress — 展示任务进度
// ──────────────────────────────────────────────

// displayTaskProgress 打印任务进度条。
func displayTaskProgress(task Task, progress float64) {
	barLen := 30
	filled := int(progress * float64(barLen))
	bar := strings.Repeat("█", filled) + strings.Repeat("░", barLen-filled)
	status := task.Status
	if status == "" {
		status = "running"
	}
	fmt.Printf("\r[%s] %s [%s] %.1f%%", status, task.Description, bar, progress*100)
	if progress >= 1.0 {
		fmt.Println()
	}
}

// ──────────────────────────────────────────────
// validateSubmission — 验证代码提交
// ──────────────────────────────────────────────

// validateSubmission 验证代码提交的合法性和安全性。
func validateSubmission(code string) error {
	if len(code) == 0 {
		return fmt.Errorf("code is empty")
	}
	if len(code) > 100000 {
		return fmt.Errorf("code exceeds maximum size (100KB)")
	}

	// 简单安全检查：禁止危险模式
	dangerousPatterns := []string{
		"rm -rf /",
		":(){ :|:& };:",
		"drop database",
	}
	for _, pattern := range dangerousPatterns {
		if strings.Contains(code, pattern) {
			return fmt.Errorf("code contains dangerous pattern: %s", pattern)
		}
	}

	// 检查括号匹配
	if !balancedBrackets(code) {
		return fmt.Errorf("unbalanced brackets in code")
	}

	return nil
}

// balancedBrackets 简单括号匹配检查。
func balancedBrackets(code string) bool {
	var stack []rune
	for _, ch := range code {
		switch ch {
		case '(', '[', '{':
			stack = append(stack, ch)
		case ')', ']', '}':
			if len(stack) == 0 {
				return false
			}
			open := stack[len(stack)-1]
			if (open == '(' && ch != ')') ||
				(open == '[' && ch != ']') ||
				(open == '{' && ch != '}') {
				return false
			}
			stack = stack[:len(stack)-1]
		}
	}
	return len(stack) == 0
}

// ──────────────────────────────────────────────
// displayOutput — 展示执行输出
// ──────────────────────────────────────────────

// displayOutput 格式化并展示代码执行输出。
func displayOutput(output string, taskID string) {
	lines := strings.Split(output, "\n")
	for i, line := range lines {
		if i >= defaultConfig.OutputLimit {
			fmt.Printf("[%s] ... (truncated, %d more lines)\n", taskID, len(lines)-i)
			break
		}
		prefix := "> "
		if strings.Contains(line, "[Error]") || strings.Contains(line, "Error") {
			prefix = "✗ "
		} else if strings.HasPrefix(line, "[Exit Code]") {
			prefix = "✓ "
		}
		fmt.Printf("[%s] %s%s\n", taskID, prefix, line)
	}
}

// ──────────────────────────────────────────────
// renderMarkdown — 渲染 Markdown（简化实现）
// ──────────────────────────────────────────────

// renderMarkdown 简化 Markdown 渲染。
func renderMarkdown(md string) string {
	// 移除 markdown 语法
	replacements := []struct {
		from *regexp.Regexp
		to   string
	}{
		{regexp.MustCompile(`\*\*(.+?)\*\*`), "$1"},
		{regexp.MustCompile(`_(.+?)_`), "$1"},
		{regexp.MustCompile(`#+\s`), ""},
	}
	result := md
	for _, r := range replacements {
		result = r.from.ReplaceAllString(result, r.to)
	}
	return strings.TrimSpace(result)
}

// ──────────────────────────────────────────────
// 辅助函数
// ──────────────────────────────────────────────

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
