// Package core — 通用版本管理模块 (v0.8.0)
//
// version_changelog.go: Changelog 生成器
//
// 功能：
//   - 按 Keep a Changelog 格式生成 CHANGELOG.md
//   - 从 Conventional Commits 自动分组
//   - 支持追加到现有 Changelog

package core

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// commitTypeToSection 将 Conventional Commits 类型映射到 Changelog 章节。
var commitTypeToSection = map[string]string{
	"feat":     "Added",
	"fix":      "Fixed",
	"docs":     "Changed",
	"style":    "Changed",
	"refactor": "Changed",
	"perf":     "Changed",
	"test":     "Changed",
	"chore":    "Changed",
	"ci":       "Changed",
	"build":    "Changed",
	"revert":   "Removed",
}

// sectionOrder 定义 Changelog 章节的显示顺序。
var sectionOrder = []string{"Added", "Changed", "Deprecated", "Removed", "Fixed", "Security"}

// GenerateChangelog 从提交列表生成完整 Changelog（Markdown 格式）。
func GenerateChangelog(version Semver, commits []ConventionalCommit, date time.Time) string {
	var b strings.Builder

	// 版本标题
	b.WriteString(fmt.Sprintf("## [%s] - %s\n\n", version.String(), date.Format("2006-01-02")))

	// 按类型分组
	grouped := GroupByType(commits)

	// 按章节顺序输出
	for _, section := range sectionOrder {
		sectionCommits := grouped[section]
		if len(sectionCommits) == 0 {
			continue
		}

		b.WriteString(fmt.Sprintf("### %s\n", section))
		for _, c := range sectionCommits {
			entry := formatChangelogEntry(c)
			b.WriteString(fmt.Sprintf("- %s\n", entry))
		}
		b.WriteString("\n")
	}

	return b.String()
}

// GenerateChangelogEntry 生成单条 Changelog 条目（不含版本标题）。
func GenerateChangelogEntry(version Semver, commits []ConventionalCommit, date time.Time) ChangelogRelease {
	release := ChangelogRelease{
		Version: version.String(),
		Date:    date,
	}

	grouped := GroupByType(commits)
	for _, section := range sectionOrder {
		sectionCommits := grouped[section]
		if len(sectionCommits) == 0 {
			continue
		}

		entries := make([]string, 0, len(sectionCommits))
		for _, c := range sectionCommits {
			entries = append(entries, formatChangelogEntry(c))
		}

		release.Sections = append(release.Sections, ChangelogSection{
			Title:   section,
			Entries: entries,
		})
	}

	return release
}

// AppendChangelog 将新条目追加到现有 Changelog 内容。
// 新条目插入到标题行之后、第一个版本条目之前。
func AppendChangelog(existing string, newEntry string) string {
	existing = strings.TrimSpace(existing)
	if existing == "" {
		return "# Changelog\n\n" + newEntry
	}

	// 找到第一个 ## [ 版本条目的位置，在此之前插入
	lines := strings.Split(existing, "\n")
	var result []string

	inserted := false
	for _, line := range lines {
		result = append(result, line)
		// 在第一个版本条目之前插入
		if !inserted && strings.HasPrefix(line, "## [") {
			inserted = true
		}
	}

	if !inserted {
		// 在末尾追加
		return existing + "\n" + newEntry
	}

	return strings.Join(result, "\n")
}

// GroupByType 按 Conventional Commits 类型分组，映射到 Changelog 章节。
func GroupByType(commits []ConventionalCommit) map[string][]ConventionalCommit {
	grouped := make(map[string][]ConventionalCommit)

	for _, c := range commits {
		section := commitTypeToSection[c.Type]

		// BREAKING CHANGE 特殊处理
		if c.BreakingChange {
			section = "Changed"
			// 在描述中添加 BREAKING CHANGE 标记
			c.Description = "**BREAKING CHANGE**: " + c.Description
		}

		grouped[section] = append(grouped[section], c)
	}

	// 每组内按 scope 排序
	for section := range grouped {
		sort.Slice(grouped[section], func(i, j int) bool {
			return grouped[section][i].Scope < grouped[section][j].Scope
		})
	}

	return grouped
}

// FormatChangelogSection 格式化单个 Changelog 章节为 Markdown。
func FormatChangelogSection(commits []ConventionalCommit) string {
	if len(commits) == 0 {
		return ""
	}

	var b strings.Builder
	for _, c := range commits {
		b.WriteString(fmt.Sprintf("- %s\n", formatChangelogEntry(c)))
	}
	return b.String()
}

// formatChangelogEntry 格式化单条 Changelog 条目。
func formatChangelogEntry(c ConventionalCommit) string {
	if c.Scope != "" {
		return fmt.Sprintf("**%s**: %s", c.Scope, c.Description)
	}
	return c.Description
}

// PrependChangelog 将新条目插入到 Changelog 的最前面（在标题之后）。
func PrependChangelog(existing string, newEntry string) string {
	existing = strings.TrimSpace(existing)
	title := "# Changelog"

	if strings.HasPrefix(existing, title) {
		rest := strings.TrimSpace(strings.TrimPrefix(existing, title))
		return title + "\n\n" + newEntry + "\n" + rest
	}

	return newEntry + "\n\n" + existing
}