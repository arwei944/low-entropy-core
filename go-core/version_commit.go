// Package core — 通用版本管理模块 (v0.8.0)
//
// version_commit.go: Conventional Commits 1.0.0 解析器
//
// 功能：
//   - 完整 Conventional Commits 1.0.0 规范解析
//   - 支持多行 body、footer、BREAKING CHANGE 标记
//   - 支持 ! 缩写语法（feat!: description）
//   - 提交分类统计

package core

import (
	"fmt"
	"regexp"
	"strings"
)

// validCommitTypes 有效的 Conventional Commits 类型。
var validCommitTypes = map[string]bool{
	"feat":     true,
	"fix":      true,
	"docs":     true,
	"style":    true,
	"refactor": true,
	"perf":     true,
	"test":     true,
	"chore":    true,
	"ci":       true,
	"build":    true,
	"revert":   true,
}

// ValidTypes 返回所有有效的 Conventional Commits 类型。
func ValidTypes() []string {
	types := make([]string, 0, len(validCommitTypes))
	for t := range validCommitTypes {
		types = append(types, t)
	}
	return types
}

// IsValidType 验证类型是否为有效的 Conventional Commits 类型。
func IsValidType(t string) bool {
	return validCommitTypes[strings.ToLower(strings.TrimSpace(t))]
}

// conventionalCommitRegex 匹配 Conventional Commits 格式。
// 格式: type(scope)!: description 或 type!: description 或 type(scope): description 或 type: description
var conventionalCommitRegex = regexp.MustCompile(
	`^(\w+)(?:\(([^)]*)\))?(!)?\s*:\s*(.+)$`,
)

// breakingFooterRegex 匹配 footer 中的 BREAKING CHANGE 或 BREAKING-CHANGE。
var breakingFooterRegex = regexp.MustCompile(
	`(?i)^BREAKING[-\s]CHANGE\s*:\s*(.+)$`,
)

// ParseCommit 解析单条 Conventional Commits 格式的提交消息。
// 支持完整格式: type(scope)!: description\n\nbody\n\nfooter
func ParseCommit(raw string) (ConventionalCommit, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ConventionalCommit{}, fmt.Errorf("empty commit message")
	}

	commit := ConventionalCommit{Raw: raw}

	// 分割 header、body、footer
	lines := strings.Split(raw, "\n")
	header := lines[0]

	// 解析 header
	matches := conventionalCommitRegex.FindStringSubmatch(header)
	if matches == nil {
		return ConventionalCommit{}, fmt.Errorf("not a conventional commit: %q", header)
	}

	commit.Type = strings.ToLower(matches[1])
	commit.Scope = matches[2]
	hasBang := matches[3] == "!"
	commit.Description = strings.TrimSpace(matches[4])

	// 验证类型
	if !IsValidType(commit.Type) {
		return ConventionalCommit{}, fmt.Errorf("invalid commit type: %q", commit.Type)
	}

	// 解析 body 和 footer
	// Conventional Commits 格式：
	//   type(scope): description
	//   <empty line>
	//   body (may be multiple lines)
	//   <empty line>
	//   footer (may be multiple lines)
	if len(lines) > 1 {
		bodyLines := []string{}
		footerLines := []string{}
		inFooter := false

		for i := 1; i < len(lines); i++ {
			line := lines[i]
			if line == "" {
				// 只有在已经收集了 body 内容后，再遇到空行才进入 footer 区域
				if len(bodyLines) > 0 && !inFooter {
					inFooter = true
				}
				continue
			}

			if inFooter {
				footerLines = append(footerLines, line)
			} else {
				bodyLines = append(bodyLines, line)
			}
		}

		commit.Body = strings.TrimSpace(strings.Join(bodyLines, "\n"))
		commit.Footer = strings.TrimSpace(strings.Join(footerLines, "\n"))
	}

	// 检测 BREAKING CHANGE
	commit.BreakingChange = hasBang || detectBreakingChange(commit.Body, commit.Footer)

	return commit, nil
}

// detectBreakingChange 检测是否包含 BREAKING CHANGE 标记。
func detectBreakingChange(body, footer string) bool {
	// 检查 body 中是否有 BREAKING CHANGE
	for _, line := range strings.Split(body, "\n") {
		if breakingFooterRegex.MatchString(strings.TrimSpace(line)) {
			return true
		}
	}
	// 检查 footer 中是否有 BREAKING CHANGE
	for _, line := range strings.Split(footer, "\n") {
		if breakingFooterRegex.MatchString(strings.TrimSpace(line)) {
			return true
		}
	}
	return false
}

// ParseCommitsFromLog 从 git log 输出中解析多条提交。
// 每行格式: <hash> <message>
// 或使用自定义分隔符分隔的批量提交
func ParseCommitsFromLog(log string) ([]ConventionalCommit, error) {
	if strings.TrimSpace(log) == "" {
		return []ConventionalCommit{}, nil
	}

	lines := strings.Split(strings.TrimSpace(log), "\n")
	commits := make([]ConventionalCommit, 0, len(lines))

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// 跳过 hash 前缀（git log --oneline 格式: <hash> <message>）
		commit, err := parseCommitLine(line)
		if err == nil {
			commits = append(commits, commit)
		}
		// 忽略无法解析的提交（非 Conventional Commits 格式）
	}

	return commits, nil
}

// parseCommitLine 解析单行 git log 输出。
// 格式: <hash> <type>(<scope>): <message> 或 <hash> <type>: <message>
func parseCommitLine(line string) (ConventionalCommit, error) {
	hash := ""
	msg := line

	// 提取 hash（如果有）
	parts := strings.SplitN(line, " ", 2)
	if len(parts) == 2 && isHexHash(parts[0]) {
		hash = parts[0]
		msg = parts[1]
	}

	commit, err := ParseCommit(msg)
	if err != nil {
		return ConventionalCommit{}, err
	}
	commit.Hash = hash
	return commit, nil
}

// isHexHash 检查字符串是否为 Git commit hash（至少 7 位十六进制）。
func isHexHash(s string) bool {
	if len(s) < 7 {
		return false
	}
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

// HasBreakingChange 检测提交是否包含 BREAKING CHANGE。
func (c ConventionalCommit) HasBreakingChange() bool {
	return c.BreakingChange
}

// IsFeat 检测提交是否为 feat 类型。
func (c ConventionalCommit) IsFeat() bool {
	return c.Type == "feat"
}

// IsFix 检测提交是否为 fix 类型。
func (c ConventionalCommit) IsFix() bool {
	return c.Type == "fix"
}

// IsBreaking 检测提交是否为破坏性变更（含 ! 或 BREAKING CHANGE）。
func (c ConventionalCommit) IsBreaking() bool {
	return c.BreakingChange
}

// ClassifyCommits 对提交列表进行分类统计。
func ClassifyCommits(commits []ConventionalCommit) CommitClassification {
	class := CommitClassification{
		ByType: make(map[string]int),
		Total:  len(commits),
	}

	for _, c := range commits {
		class.ByType[c.Type]++
		if c.BreakingChange {
			class.Breaking++
		}

		switch c.Type {
		case "feat":
			class.Feat++
		case "fix":
			class.Fix++
		case "docs":
			class.Docs++
		case "style":
			class.Style++
		case "refactor":
			class.Refactor++
		case "perf":
			class.Perf++
		case "test":
			class.Test++
		case "chore":
			class.Chore++
		case "ci":
			class.CI++
		case "build":
			class.Build++
		case "revert":
			class.Revert++
		}
	}

	return class
}

// Summary 返回提交的简短摘要。
func (c ConventionalCommit) Summary() string {
	if c.Scope != "" {
		return fmt.Sprintf("%s(%s): %s", c.Type, c.Scope, c.Description)
	}
	return fmt.Sprintf("%s: %s", c.Type, c.Description)
}