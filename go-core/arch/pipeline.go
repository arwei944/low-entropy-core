// Package arch — Architecture Pipeline Composer (L1)
//
// 统一入口：解析 → 分析 → 校验 → (可选)生成。
// Composer：编排前几个原语，形成完整工作流。
//
// 设计约束: 文件 ≤ 300 行
package arch

import (
	"context"
	"time"
)

// PipelineResult 一次完整 pipeline 的输出。
type PipelineResult struct {
	GeneratedAt  time.Time         `json:"generated_at"`
	Duration     time.Duration     `json:"duration"`
	ArchData     *ArchData         `json:"arch_data"`
	Validation   *ValidationResult `json:"validation"`
	GenResult    *GenResult        `json:"gen_result,omitempty"`
	Error        string            `json:"error,omitempty"`
}

// Pipeline 架构分析与生成流水线。
type Pipeline interface {
	// Run 执行完整流程：解析 → 分析 → 校验。
	Run(ctx context.Context, projectPath string) (*PipelineResult, error)
	// Analyze 仅执行解析与分析（不校验和生成）。
	Analyze(ctx context.Context, projectPath string) (*ArchData, error)
	// Validate 仅执行解析与校验。
	Validate(ctx context.Context, projectPath string) (*ValidationResult, error)
}

// defaultPipeline 默认实现。
type defaultPipeline struct {
	validator  Validator
	generator  Generator
	// 可注入观测
	onStep func(step string, startedAt time.Time, err error)
}

// NewPipeline 创建默认 pipeline。
// 若 generator 为 nil，则 Generate 步骤会报错提示。
func NewPipeline() (Pipeline, error) {
	gen, err := NewGenerator()
	if err != nil {
		// generator 非必需 — 失败时仅记录但不阻塞
		return &defaultPipeline{
			validator: NewValidator(),
			generator: nil,
		}, nil
	}
	return &defaultPipeline{
		validator: NewValidator(),
		generator: gen,
	}, nil
}

// Run 完整的架构分析流水线。
func (p *defaultPipeline) Run(ctx context.Context, projectPath string) (*PipelineResult, error) {
	start := time.Now()

	select {
	case <-ctx.Done():
		return &PipelineResult{
			GeneratedAt: time.Now(),
			Duration:    time.Since(start),
			Error:       ctx.Err().Error(),
		}, ctx.Err()
	default:
	}

	result := &PipelineResult{GeneratedAt: start}

	// 步骤 1+2+3: 解析 → 分析 → 校验（由 Validator 内部调用）
	validation, err := p.validator.Validate(ctx, projectPath)
	if err != nil {
		result.Duration = time.Since(start)
		result.Error = err.Error()
		return result, err
	}
	result.Validation = validation
	if validation.ArchSnapshot != nil {
		result.ArchData = validation.ArchSnapshot
	}

	result.Duration = time.Since(start)
	return result, nil
}

// Analyze 仅解析与分析。
func (p *defaultPipeline) Analyze(ctx context.Context, projectPath string) (*ArchData, error) {
	files, err := ParseDirectory(projectPath)
	if err != nil {
		return nil, err
	}
	return AnalyzeArchitecture(files), nil
}

// Validate 仅解析与校验。
func (p *defaultPipeline) Validate(ctx context.Context, projectPath string) (*ValidationResult, error) {
	return p.validator.Validate(ctx, projectPath)
}

// 辅助：将 errors 转为字符串
func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
