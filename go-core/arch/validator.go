// Package arch — Validator Port (L1)
//
// 校验输入项目的架构合规性。
// Port：输入项目路径 → 输出 ValidationResult。
//
// 来源:
//   cmd/arch-dev/arch_check.go
//
// 设计约束: 文件 ≤ 300 行
package arch

import (
	"context"
	"fmt"
	"time"
)

// ValidationResult 一次完整校验的输出。
// 包含文件数、违规数、违规明细、健康评分、耗时。
type ValidationResult struct {
	ProjectRoot  string        `json:"project_root"`
	FileCount    int           `json:"file_count"`
	ViolationCount int         `json:"violation_count"`
	Violations   []Violation   `json:"violations"`
	HealthScore  float64       `json:"health_score"`
	HealthGrade  string        `json:"health_grade"`
	Duration     time.Duration `json:"duration"`
	ArchSnapshot *ArchData     `json:"arch_snapshot,omitempty"`
}

// Validator 定义架构校验 Port 接口。
type Validator interface {
	Validate(ctx context.Context, projectPath string) (*ValidationResult, error)
	ValidateFile(ctx context.Context, filePath string) (*FileValidationResult, error)
}

// FileValidationResult 单个文件的校验结果（较粗粒度的校验）。
type FileValidationResult struct {
	File       string      `json:"file"`
	Lines      int         `json:"lines"`
	Layer      string      `json:"layer"`
	Violations []Violation `json:"violations"`
	Valid      bool        `json:"valid"`
}

// ──────────────────────────────────────────────
// 默认实现
// ──────────────────────────────────────────────

type defaultValidator struct{}

// NewValidator 创建默认校验器。
func NewValidator() Validator {
	return &defaultValidator{}
}

// Validate 执行完整的项目架构校验。
func (v *defaultValidator) Validate(ctx context.Context, projectPath string) (*ValidationResult, error) {
	start := time.Now()

	// 1. 解析
	files, err := ParseDirectory(projectPath)
	if err != nil {
		return nil, fmt.Errorf("解析项目失败: %w", err)
	}

	// 2. 分析
	data := AnalyzeArchitecture(files)

	// 3. 违规检测
	violations := DetectViolations(data)

	// 4. 计算健康度
	score := computeHealthScore(violations, len(files))

	duration := time.Since(start)

	return &ValidationResult{
		ProjectRoot:    projectPath,
		FileCount:      len(files),
		ViolationCount: len(violations),
		Violations:     violations,
		HealthScore:    score.Overall,
		HealthGrade:    score.Grade,
		Duration:       duration,
		ArchSnapshot:   data,
	}, nil
}

// ValidateFile 校验单个文件。
func (v *defaultValidator) ValidateFile(ctx context.Context, filePath string) (*FileValidationResult, error) {
	f, err := ParseFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("解析文件失败: %w", err)
	}

	// 简单的单文件违规检测
	var violations []Violation
	if f.Lines > 300 {
		violations = append(violations, Violation{
			Type:       ViolationFileTooLong,
			Severity:   SeverityWarn,
			File:       f.Name,
			Message:    "文件超过 300 行",
			Detail:     "实际 " + itoa(f.Lines) + " 行",
			Suggestion: "按功能/层级拆分",
		})
	}

	return &FileValidationResult{
		File:       f.Name,
		Lines:      f.Lines,
		Layer:      f.Layer,
		Violations: violations,
		Valid:      len(violations) == 0,
	}, nil
}

// computeHealthScore 根据违规情况和文件数计算健康度（0.0-1.0）。
func computeHealthScore(violations []Violation, fileCount int) HealthScore {
	if fileCount == 0 {
		return HealthScore{Overall: 0.5, Grade: "C", Factors: map[string]float64{"files": 0}}
	}

	score := 1.0
	factors := make(map[string]float64)

	// 按严重程度扣分
	errorCount := 0
	warnCount := 0
	infoCount := 0
	for _, v := range violations {
		switch v.Severity {
		case SeverityError:
			score -= 0.05
			errorCount++
		case SeverityWarn:
			score -= 0.02
			warnCount++
		case SeverityInfo:
			score -= 0.01
			infoCount++
		}
	}
	if score < 0 {
		score = 0
	}

	factors["errors"] = float64(errorCount)
	factors["warnings"] = float64(warnCount)
	factors["info"] = float64(infoCount)
	factors["files"] = float64(fileCount)

	suggestions := make([]string, 0)
	if errorCount > 0 {
		suggestions = append(suggestions, "有 "+itoa(errorCount)+" 个严重违规，建议优先修复")
	}
	if warnCount > 3 {
		suggestions = append(suggestions, "警告较多，建议系统性审查")
	}

	return HealthScore{
		Overall:     score,
		Grade:       ComputeGrade(score),
		Factors:     factors,
		Violations:  len(violations),
		Suggestions: suggestions,
	}
}
