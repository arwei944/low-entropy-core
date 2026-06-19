// Package core — 通用版本管理模块 (v0.8.0)
//
// version_git.go: Git 操作适配器
//
// 功能：
//   - 通过 exec.Command 封装 Git 操作
//   - 作为 Sidecar 适配器模式接入架构
//   - Git 不可用时返回友好错误（非 panic）

package core

import (
	"fmt"
	"os/exec"
	"strings"
)

// ============================================================================
// Git 操作
// ============================================================================

// GitLog 获取 git log 输出。
func GitLog(repoDir string, opts GitLogOptions) (string, error) {
	args := []string{"log"}

	if opts.MaxCount > 0 {
		args = append(args, fmt.Sprintf("-n%d", opts.MaxCount))
	}
	if opts.SkipMerges {
		args = append(args, "--no-merges")
	}
	if opts.Format != "" {
		args = append(args, fmt.Sprintf("--format=%s", opts.Format))
	} else {
		args = append(args, "--oneline")
	}
	if opts.Since != "" {
		args = append(args, fmt.Sprintf("--since=%s", opts.Since))
	}
	if opts.Until != "" {
		args = append(args, fmt.Sprintf("--until=%s", opts.Until))
	}
	if opts.Reverse {
		args = append(args, "--reverse")
	}

	return runGitCommand(repoDir, args...)
}

// GitTags 获取所有 Git tag，按版本号降序排列。
func GitTags(repoDir string) ([]string, error) {
	output, err := runGitCommand(repoDir, "tag", "--sort=-v:refname")
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(output) == "" {
		return []string{}, nil
	}
	return strings.Split(strings.TrimSpace(output), "\n"), nil
}

// GitCreateTag 创建 Git tag。
func GitCreateTag(repoDir string, tag string, message string) error {
	args := []string{"tag", "-a", tag}
	if message != "" {
		args = append(args, "-m", message)
	}
	_, err := runGitCommand(repoDir, args...)
	return err
}

// GitCommitsBetween 获取两个 tag/commit 之间的提交。
func GitCommitsBetween(repoDir string, from string, to string) (string, error) {
	rangeSpec := fmt.Sprintf("%s..%s", from, to)
	return runGitCommand(repoDir, "log", rangeSpec, "--oneline", "--no-merges")
}

// GitCommitsSinceTag 获取从指定 tag 到 HEAD 的提交。
func GitCommitsSinceTag(repoDir string, tag string) (string, error) {
	if tag == "" {
		return runGitCommand(repoDir, "log", "--oneline", "--no-merges", "-n50")
	}
	rangeSpec := fmt.Sprintf("%s..HEAD", tag)
	return runGitCommand(repoDir, "log", rangeSpec, "--oneline", "--no-merges")
}

// GitCurrentBranch 获取当前分支名称。
func GitCurrentBranch(repoDir string) (string, error) {
	output, err := runGitCommand(repoDir, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(output), nil
}

// GitIsClean 检查工作区是否干净（无未提交的更改）。
func GitIsClean(repoDir string) (bool, error) {
	output, err := runGitCommand(repoDir, "status", "--porcelain")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(output) == "", nil
}

// GitLastTag 获取最近的 Git tag。
func GitLastTag(repoDir string) (string, error) {
	output, err := runGitCommand(repoDir, "describe", "--tags", "--abbrev=0")
	if err != nil {
		// 可能没有 tag，返回空字符串
		return "", nil
	}
	return strings.TrimSpace(output), nil
}

// GitTagExists 检查 tag 是否存在。
func GitTagExists(repoDir string, tag string) (bool, error) {
	tags, err := GitTags(repoDir)
	if err != nil {
		return false, err
	}
	for _, t := range tags {
		if strings.TrimSpace(t) == tag {
			return true, nil
		}
	}
	return false, nil
}

// GitFullLog 获取完整的 git log（包含 body），用于解析 Conventional Commits。
func GitFullLog(repoDir string, opts GitLogOptions) (string, error) {
	args := []string{"log"}

	if opts.MaxCount > 0 {
		args = append(args, fmt.Sprintf("-n%d", opts.MaxCount))
	}
	if opts.SkipMerges {
		args = append(args, "--no-merges")
	}
	// 使用 %B 获取完整提交消息（含 body）
	format := "--format=%H%n%B%n---END---"
	args = append(args, format)
	if opts.Since != "" {
		args = append(args, fmt.Sprintf("--since=%s", opts.Since))
	}
	if opts.Until != "" {
		args = append(args, fmt.Sprintf("--until=%s", opts.Until))
	}

	return runGitCommand(repoDir, args...)
}

// ============================================================================
// 内部辅助
// ============================================================================

// runGitCommand 执行 Git 命令并返回输出。
func runGitCommand(repoDir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = repoDir

	output, err := cmd.Output()
	if err != nil {
		// 返回友好错误，包含 stderr 信息
		errMsg := err.Error()
		if exitErr, ok := err.(*exec.ExitError); ok {
			errMsg = string(exitErr.Stderr)
		}
		return "", fmt.Errorf("git %s: %s", strings.Join(args, " "), strings.TrimSpace(errMsg))
	}

	return string(output), nil
}