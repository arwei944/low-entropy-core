// Package core — 通用版本管理模块 (v0.8.0)
//
// version_composer.go: 发布流水线编排器
//
// 功能：
//   - 使用 Composer 模式编排发布流程
//   - 6 步流水线：AnalyzeCommits → InferVersion → MergeChanges → GenerateChangelog → UpdateADR → CreateTag
//   - 支持 DryRun 预演模式

package core

import (
	"context"
	"fmt"
	"time"
)

// ============================================================================
// ReleaseComposer — 发布流水线编排器
// ============================================================================

// ReleaseComposer 编排发布流程的编排器。
type ReleaseComposer struct {
	repoDir   string // 项目根目录
	changeDir string // .arch-change 目录
	adrDir    string // docs/adr 目录
}

// NewReleaseComposer 创建发布编排器。
// repoDir: 项目根目录（Git 仓库根目录）
func NewReleaseComposer(repoDir string) *ReleaseComposer {
	return &ReleaseComposer{
		repoDir:   repoDir,
		changeDir: repoDir,
		adrDir:    repoDir,
	}
}

// PlanRelease 规划发布：分析提交、推断版本、生成 Changelog。
// 返回 ReleasePlan 但不执行任何变更。
func (rc *ReleaseComposer) PlanRelease() (ReleasePlan, error) {
	plan := ReleasePlan{
		Date:   time.Now(),
		DryRun: true,
	}

	// Step 1: 分析提交
	lastTag, _ := GitLastTag(rc.repoDir)
	log, err := GitCommitsSinceTag(rc.repoDir, lastTag)
	if err != nil {
		return plan, fmt.Errorf("analyze commits: %w", err)
	}

	commits, err := ParseCommitsFromLog(log)
	if err != nil {
		return plan, fmt.Errorf("parse commits: %w", err)
	}
	plan.Commits = commits

	if len(commits) == 0 {
		return plan, fmt.Errorf("no commits to release")
	}

	// Step 2: 推断版本
	current := Semver{Major: 0, Minor: 7, Patch: 0} // 默认当前版本
	if lastTag != "" {
		if parsed, err := ParseSemver(lastTag); err == nil {
			current = parsed
		}
	}
	plan.Version = InferNextVersion(commits, current)
	plan.Bump = InferBump(commits)
	plan.Tag = plan.Version.TagName()

	// Step 3: 合并变更意图
	archChange, _ := MergeChanges(rc.changeDir)
	plan.Changes = archChange.Intents

	// Step 4: 生成 Changelog
	plan.Changelog = GenerateChangelog(plan.Version, commits, plan.Date)

	// Step 5: 关联 ADR
	adrs, _ := ADRByVersion(rc.adrDir, plan.Version)
	plan.ADRs = adrs

	return plan, nil
}

// ExecuteRelease 执行完整的发布流水线。
func (rc *ReleaseComposer) ExecuteRelease(plan ReleasePlan) (ReleaseResult, error) {
	result := ReleaseResult{
		Version: plan.Version.String(),
		Steps:   make([]ReleaseStepResult, 0),
	}

	// Step 1: 分析提交（已在 PlanRelease 中完成）
	result.Steps = append(result.Steps, ReleaseStepResult{
		Name:   "AnalyzeCommits",
		Status: "success",
		Output: fmt.Sprintf("analyzed %d commits", len(plan.Commits)),
	})

	// Step 2: 推断版本（已在 PlanRelease 中完成）
	result.Steps = append(result.Steps, ReleaseStepResult{
		Name:   "InferVersion",
		Status: "success",
		Output: fmt.Sprintf("version: %s (bump: %v)", plan.Version.String(), plan.Bump),
	})

	// Step 3: 合并变更意图
	if len(plan.Changes) > 0 {
		result.Steps = append(result.Steps, ReleaseStepResult{
			Name:   "MergeChanges",
			Status: "success",
			Output: fmt.Sprintf("merged %d change intents", len(plan.Changes)),
		})
	} else {
		result.Steps = append(result.Steps, ReleaseStepResult{
			Name:   "MergeChanges",
			Status: "skipped",
			Output: "no change intents to merge",
		})
	}

	// Step 4: 生成 Changelog
	result.Changelog = plan.Changelog
	result.Steps = append(result.Steps, ReleaseStepResult{
		Name:   "GenerateChangelog",
		Status: "success",
		Output: "changelog generated",
	})

	// Step 5: 更新 ADR（如果有变更意图）
	if len(plan.Changes) > 0 {
		adr := ADR{
			Title:        fmt.Sprintf("Release %s", plan.Version.String()),
			Status:       ADRStatusAccepted,
			Version:      plan.Version.String(),
			Date:         time.Now(),
			Context:      fmt.Sprintf("Release %s with %d commits and %d change intents.", plan.Version.String(), len(plan.Commits), len(plan.Changes)),
			Decision:     "Proceed with release as planned.",
			Consequences: fmt.Sprintf("New version %s released with changelog.", plan.Version.String()),
		}
		if err := CreateADR(rc.adrDir, adr); err != nil {
			result.Steps = append(result.Steps, ReleaseStepResult{
				Name:   "UpdateADR",
				Status: "failed",
				Error:  err.Error(),
			})
		} else {
			result.ADRsCreated = append(result.ADRsCreated, adr.ID)
			result.Steps = append(result.Steps, ReleaseStepResult{
				Name:   "UpdateADR",
				Status: "success",
				Output: fmt.Sprintf("created ADR: %s", adr.ID),
			})
		}
	} else {
		result.Steps = append(result.Steps, ReleaseStepResult{
			Name:   "UpdateADR",
			Status: "skipped",
			Output: "no changes to document",
		})
	}

	// Step 6: 创建 Git Tag
	if !plan.DryRun {
		if err := GitCreateTag(rc.repoDir, plan.Tag, fmt.Sprintf("Release %s", plan.Version.String())); err != nil {
			result.Steps = append(result.Steps, ReleaseStepResult{
				Name:   "CreateTag",
				Status: "failed",
				Error:  err.Error(),
			})
			result.Error = fmt.Sprintf("create tag: %v", err)
			result.CompletedAt = time.Now()
			return result, nil
		}
		result.Tag = plan.Tag
		result.Steps = append(result.Steps, ReleaseStepResult{
			Name:   "CreateTag",
			Status: "success",
			Output: fmt.Sprintf("tag created: %s", plan.Tag),
		})
	} else {
		result.Steps = append(result.Steps, ReleaseStepResult{
			Name:   "CreateTag",
			Status: "skipped",
			Output: fmt.Sprintf("dry run: would create tag %s", plan.Tag),
		})
	}

	result.Success = true
	result.CompletedAt = time.Now()
	return result, nil
}

// DryRun 预演发布流程（不创建 tag）。
func (rc *ReleaseComposer) DryRun() (ReleasePlan, error) {
	plan, err := rc.PlanRelease()
	if err != nil {
		return plan, err
	}
	plan.DryRun = true
	return plan, nil
}

// Steps 返回发布流水线的步骤描述。
func (rc *ReleaseComposer) Steps() []string {
	return []string{
		"AnalyzeCommits",
		"InferVersion",
		"MergeChanges",
		"GenerateChangelog",
		"UpdateADR",
		"CreateTag",
	}
}

// ============================================================================
// Step 适配器（使发布流水线适配 Step[ReleasePlan, ReleasePlan] 接口）
// ============================================================================

// ReleaseStep 将发布流水线的一个步骤适配为 Step[ReleasePlan, ReleasePlan]。
type ReleaseStep struct {
	name string
	fn   func(context.Context, ReleasePlan) (ReleasePlan, error)
}

// Execute 执行发布步骤。
func (s ReleaseStep) Execute(ctx context.Context, input ReleasePlan) (ReleasePlan, error) {
	return s.fn(ctx, input)
}

// UnitType 返回步骤类型。
func (s ReleaseStep) UnitType() string {
	return s.name
}

// NewReleaseStep 创建发布步骤。
func NewReleaseStep(name string, fn func(context.Context, ReleasePlan) (ReleasePlan, error)) ReleaseStep {
	return ReleaseStep{name: name, fn: fn}
}