// Package core — Understand-Anything CLI Adapter (v0.7.0)
//
// T-03: UnderstandAdapter — 包装 UA CLI 调用为 Adapter[UnderstandRequest, UnderstandResponse]
//
// 约束遵循:
//   C4: Port-Adapter — 所有外部交互（CLI 调用、文件 I/O）通过 Adapter
//   C6: 泛型优先 — Adapter[UnderstandRequest, UnderstandResponse]

package core

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// ============================================================================
// T-03: UnderstandAdapter — UA CLI 调用适配器
// ============================================================================

// UnderstandConfig UA 适配器配置
type UnderstandConfig struct {
	UAPath      string        // Node.js CLI 路径 (understand-anything-plugin)
	ProjectRoot string        // 分析目标目录
	OutputDir   string        // 图谱输出目录 (.understand-anything/)
	Timeout     time.Duration // CLI 执行超时
}

// DefaultUnderstandConfig 返回默认配置
func DefaultUnderstandConfig(projectRoot string) UnderstandConfig {
	return UnderstandConfig{
		UAPath:      "", // 留空表示使用 PATH 中的 UA
		ProjectRoot: projectRoot,
		OutputDir:   filepath.Join(projectRoot, ".understand-anything"),
		Timeout:     120 * time.Second,
	}
}

// UnderstandRequest UA 操作请求
type UnderstandRequest struct {
	Action   string   // "analyze" | "diff" | "validate" | "search"
	Path     string   // 目标路径（相对于 ProjectRoot）
	Language string   // 输出语言 (zh/en/ja/ko/...)
	Args     []string // 额外 CLI 参数
	Full     bool     // 是否全量分析 (--full)
}

// UnderstandResponse UA 操作响应
type UnderstandResponse struct {
	GraphPath string          // 图谱 JSON 文件路径
	Graph     *KnowledgeGraph // 解析后的图谱（仅在 action=analyze/diff 时填充）
	Warnings  []string        // 验证警告
	Duration  time.Duration   // 执行耗时
	Error     error           // 执行错误
}

// UnderstandAdapter 将 UA CLI 包装为 Adapter[UnderstandRequest, UnderstandResponse]
type UnderstandAdapter struct {
	config UnderstandConfig
}

// NewUnderstandAdapter 创建 UA 适配器
func NewUnderstandAdapter(config UnderstandConfig) *UnderstandAdapter {
	return &UnderstandAdapter{config: config}
}

// Execute 实现 Adapter[UnderstandRequest, UnderstandResponse]
func (a *UnderstandAdapter) Execute(ctx context.Context, req UnderstandRequest) (UnderstandResponse, error) {
	start := time.Now()

	// 1. 确保输出目录存在
	if err := os.MkdirAll(a.config.OutputDir, 0755); err != nil {
		return UnderstandResponse{Error: err, Duration: time.Since(start)},
			fmt.Errorf("understand: create output dir: %w", err)
	}

	// 2. 构建 CLI 命令（如果 UA 不可用，返回友好错误）
	args := a.buildArgs(req)
	cmdPath := a.resolveUAPath()
	cmd := exec.CommandContext(ctx, cmdPath, args...)
	cmd.Dir = a.config.ProjectRoot

	// 3. 执行（在超时上下文中）
	timeoutCtx, cancel := context.WithTimeout(ctx, a.config.Timeout)
	defer cancel()
	cmd = exec.CommandContext(timeoutCtx, cmdPath, args...)
	cmd.Dir = a.config.ProjectRoot

	output, err := cmd.CombinedOutput()
	duration := time.Since(start)

	if err != nil {
		return UnderstandResponse{
			Error:    fmt.Errorf("understand: CLI execution failed: %w\nOutput: %s", err, string(output)),
			Duration: duration,
		}, err
	}

	// 4. 解析图谱 JSON
	graphPath := filepath.Join(a.config.OutputDir, "knowledge-graph.json")
	graphData, readErr := os.ReadFile(graphPath)
	if readErr != nil {
		return UnderstandResponse{
			GraphPath: graphPath,
			Error:     fmt.Errorf("understand: read graph file: %w", readErr),
			Duration:  duration,
		}, readErr
	}

	kg, parseErr := ParseKnowledgeGraph(graphData)
	if parseErr != nil {
		return UnderstandResponse{
			GraphPath: graphPath,
			Error:     fmt.Errorf("understand: parse graph: %w", parseErr),
			Duration:  duration,
		}, parseErr
	}

	// 5. 验证图谱
	warnings := ValidateKnowledgeGraph(kg)

	return UnderstandResponse{
		GraphPath: graphPath,
		Graph:     kg,
		Warnings:  warnings,
		Duration:  duration,
	}, nil
}

// buildArgs 根据请求构建 CLI 参数
func (a *UnderstandAdapter) buildArgs(req UnderstandRequest) []string {
	var args []string

	// 根据 action 构建命令
	switch req.Action {
	case "analyze":
		// 默认就是 analyze
	default:
		// 其他 action 暂不支持
	}

	// --full 全量分析
	if req.Full {
		args = append(args, "--full")
	}

	// --language
	if req.Language != "" {
		args = append(args, "--language", req.Language)
	}

	// 额外参数
	args = append(args, req.Args...)

	return args
}

// resolveUAPath 解析 UA CLI 路径
func (a *UnderstandAdapter) resolveUAPath() string {
	if a.config.UAPath != "" {
		return a.config.UAPath
	}

	// 尝试常见路径
	candidates := []string{
		"understand-anything",
		"ua",
		filepath.Join(os.Getenv("HOME"), ".understand-anything", "repo", "understand-anything-plugin"),
		filepath.Join(os.Getenv("USERPROFILE"), ".understand-anything", "repo", "understand-anything-plugin"),
	}

	for _, c := range candidates {
		if _, err := exec.LookPath(c); err == nil {
			return c
		}
	}

	// 默认返回 "understand-anything"，让 exec 报错
	return "understand-anything"
}

// RunAnalysis 便捷方法：运行完整分析并返回解析后的图谱
func (a *UnderstandAdapter) RunAnalysis(ctx context.Context, opts ...string) (*KnowledgeGraph, error) {
	req := UnderstandRequest{
		Action:   "analyze",
		Language: "zh",
		Args:     opts,
	}
	resp, err := a.Execute(ctx, req)
	return resp.Graph, err
}

// LoadGraph 从已有图谱文件加载（不调用 UA CLI）
func (a *UnderstandAdapter) LoadGraph(ctx context.Context) (*KnowledgeGraph, error) {
	graphPath := filepath.Join(a.config.OutputDir, "knowledge-graph.json")
	data, err := os.ReadFile(graphPath)
	if err != nil {
		return nil, fmt.Errorf("understand: load graph: %w", err)
	}
	return ParseKnowledgeGraph(data)
}

// HasGraph 检查图谱文件是否存在
func (a *UnderstandAdapter) HasGraph() bool {
	graphPath := filepath.Join(a.config.OutputDir, "knowledge-graph.json")
	_, err := os.Stat(graphPath)
	return err == nil
}