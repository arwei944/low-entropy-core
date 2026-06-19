// Package core — 通用版本管理模块 (v0.8.0)
//
// version_types.go: 核心类型定义
//
// 定义所有版本管理相关的核心数据结构，是整个版本管理模块的基石。
// 包含 SemVer、ConventionalCommit、ChangeIntent、ADR、ReleasePlan 等类型。

package core

import "time"

// ============================================================================
// SECTION 1: SemVer — 语义化版本号 (SemVer 2.0.0)
// ============================================================================

// Semver 表示一个语义化版本号。
// 格式: MAJOR.MINOR.PATCH[-PRERELEASE][+BUILD]
type Semver struct {
	Major      int    `json:"major"`
	Minor      int    `json:"minor"`
	Patch      int    `json:"patch"`
	Prerelease string `json:"prerelease,omitempty"` // 如 "alpha.1", "beta.2", "rc.1"
	Build      string `json:"build,omitempty"`      // 如 "build.123", "20260619"
}

// VersionBump 表示版本号增量类型。
type VersionBump struct {
	Major      bool `json:"major"`      // 不兼容的 API 变更
	Minor      bool `json:"minor"`      // 向后兼容的新功能
	Patch      bool `json:"patch"`      // 向后兼容的修复
	Prerelease bool `json:"prerelease"` // 预发布标识变更
}

// ============================================================================
// SECTION 2: ConventionalCommit — 规范化提交 (Conventional Commits 1.0.0)
// ============================================================================

// ConventionalCommit 表示一条符合 Conventional Commits 规范的提交。
// 格式: type(scope): description
//       [body]
//       [footer]
type ConventionalCommit struct {
	Type           string `json:"type"`            // feat, fix, docs, style, refactor, perf, test, chore, ci, build, revert
	Scope          string `json:"scope,omitempty"` // 影响范围
	Description    string `json:"description"`     // 简短描述
	Body           string `json:"body,omitempty"`  // 详细描述
	Footer         string `json:"footer,omitempty"` // 尾部信息
	BreakingChange bool   `json:"breaking_change"` // 是否包含 BREAKING CHANGE
	Hash           string `json:"hash,omitempty"`   // Git commit hash
	Raw            string `json:"raw,omitempty"`   // 原始提交消息
}

// CommitClassification 提交分类统计。
type CommitClassification struct {
	Feat        int            `json:"feat"`
	Fix         int            `json:"fix"`
	Docs        int            `json:"docs"`
	Style       int            `json:"style"`
	Refactor    int            `json:"refactor"`
	Perf        int            `json:"perf"`
	Test        int            `json:"test"`
	Chore       int            `json:"chore"`
	CI          int            `json:"ci"`
	Build       int            `json:"build"`
	Revert      int            `json:"revert"`
	Breaking    int            `json:"breaking"`
	ByType      map[string]int `json:"by_type"`
	Total       int            `json:"total"`
}

// ============================================================================
// SECTION 3: ChangeIntent — 变更意图 (ArchChange 文件系统)
// ============================================================================

// ChangeIntent 表示一个变更意图，对应 changesets 模式中的单个变更文件。
// 存储在 .arch-change/ 目录下，每个变更一个 Markdown 文件。
type ChangeIntent struct {
	ID          string   `json:"id"`                    // 唯一标识，如 "change-20260619-001"
	Title       string   `json:"title"`                 // 变更标题
	Type        string   `json:"type"`                  // feat, fix, refactor, docs, test, chore, breaking
	Scope       string   `json:"scope,omitempty"`       // 影响范围
	Description string   `json:"description"`           // 详细描述
	Files       []string `json:"files,omitempty"`       // 影响文件列表
	Breaking    bool     `json:"breaking"`              // 是否包含破坏性变更
	Migration   string   `json:"migration,omitempty"`   // 迁移指南
	Author      string   `json:"author,omitempty"`      // 作者
	CreatedAt   time.Time `json:"created_at"`           // 创建时间
}

// ArchChange 表示一次架构变更，可合并多个 ChangeIntent。
type ArchChange struct {
	ID        string         `json:"id"`
	Version   string         `json:"version"`   // 关联的版本号
	Intents   []ChangeIntent `json:"intents"`   // 合并的变更意图
	Timestamp time.Time      `json:"timestamp"` // 合并时间
	Status    string         `json:"status"`    // pending, merged, released
}

// ============================================================================
// SECTION 4: ADR — 架构决策记录 (Architecture Decision Records)
// ============================================================================

// ADR 表示一条架构决策记录。
type ADR struct {
	ID           string    `json:"id"`                     // 如 "ADR-0001"
	Title        string    `json:"title"`                  // 决策标题
	Status       string    `json:"status"`                 // Proposed, Accepted, Deprecated, Superseded
	Version      string    `json:"version"`                // 关联的版本号
	Date         time.Time `json:"date"`                   // 决策日期
	Context      string    `json:"context"`                // 背景和问题描述
	Decision     string    `json:"decision"`               // 决策内容
	Consequences string    `json:"consequences"`           // 影响和后果
	SupersededBy string    `json:"superseded_by,omitempty"` // 被哪个 ADR 替代
	FilePath     string    `json:"file_path,omitempty"`    // 文件路径
}

// ADRStatus 定义 ADR 状态常量。
const (
	ADRStatusProposed   = "Proposed"
	ADRStatusAccepted   = "Accepted"
	ADRStatusDeprecated = "Deprecated"
	ADRStatusSuperseded = "Superseded"
)

// ValidADRStatuses 返回所有有效的 ADR 状态。
func ValidADRStatuses() []string {
	return []string{ADRStatusProposed, ADRStatusAccepted, ADRStatusDeprecated, ADRStatusSuperseded}
}

// ============================================================================
// SECTION 5: ReleasePlan — 发布计划
// ============================================================================

// ReleasePlan 表示一次发布的完整计划。
type ReleasePlan struct {
	Version   Semver              `json:"version"`   // 目标版本号
	Bump      VersionBump         `json:"bump"`      // 版本增量
	Commits   []ConventionalCommit `json:"commits"`  // 包含的提交
	Changes   []ChangeIntent      `json:"changes"`   // 变更意图
	Changelog string              `json:"changelog"` // 生成的 Changelog
	ADRs      []ADR               `json:"adrs"`      // 关联的 ADR
	Tag       string              `json:"tag"`       // Git tag 名称
	Date      time.Time           `json:"date"`      // 发布日期
	DryRun    bool                `json:"dry_run"`   // 是否为预演
}

// ReleaseResult 表示发布执行结果。
type ReleaseResult struct {
	Success      bool      `json:"success"`
	Version      string    `json:"version"`
	Tag          string    `json:"tag,omitempty"`
	Changelog    string    `json:"changelog,omitempty"`
	ADRsCreated  []string  `json:"adrs_created,omitempty"`
	Steps        []ReleaseStepResult `json:"steps"`
	Error        string    `json:"error,omitempty"`
	CompletedAt  time.Time `json:"completed_at"`
}

// ReleaseStepResult 表示单个发布步骤的执行结果。
type ReleaseStepResult struct {
	Name    string `json:"name"`
	Status  string `json:"status"` // pending, running, success, failed, skipped
	Output  string `json:"output,omitempty"`
	Error   string `json:"error,omitempty"`
}

// ============================================================================
// SECTION 6: Changelog 相关类型
// ============================================================================

// ChangelogSection 表示 Changelog 的一个章节（Added/Changed/Fixed/Removed 等）。
type ChangelogSection struct {
	Title   string   `json:"title"`   // "Added", "Changed", "Fixed", "Removed", "Security", "Deprecated"
	Entries []string `json:"entries"` // 条目列表
}

// ChangelogRelease 表示 Changelog 中的一个版本发布条目。
type ChangelogRelease struct {
	Version  string            `json:"version"`
	Date     time.Time         `json:"date"`
	Sections []ChangelogSection `json:"sections"`
}

// ============================================================================
// SECTION 7: 版本快照相关类型（供 arch-manager 使用）
// ============================================================================

// VersionSnapshot 表示一个版本快照。
type VersionSnapshot struct {
	Version   string           `json:"version"`
	Timestamp time.Time        `json:"timestamp"`
	Semver    Semver           `json:"semver"`
	Snapshot  SnapshotData     `json:"snapshot"`
	Changelog []ChangelogEntry `json:"changelog"`
}

// SnapshotData 版本快照数据。
type SnapshotData struct {
	Files      map[string]FileSnapshot `json:"files"`
	LayerStats map[string]LayerStat    `json:"layer_stats"`
	Total      TotalStat               `json:"total"`
}

// FileSnapshot 单个文件的快照信息。
type FileSnapshot struct {
	Hash    string `json:"hash"`
	Lines   int    `json:"lines"`
	Symbols int    `json:"symbols"`
}

// TotalStat 全局统计。
type TotalStat struct {
	Files   int `json:"files"`
	Lines   int `json:"lines"`
	Symbols int `json:"symbols"`
}

// LayerStat 层级统计。
type LayerStat struct {
	Layer   string `json:"layer"`
	Name    string `json:"name"`
	Files   int    `json:"files"`
	Lines   int    `json:"lines"`
	Symbols int    `json:"symbols"`
	Color   string `json:"color"`
}

// ChangelogEntry 一条 Changelog 记录。
type ChangelogEntry struct {
	Type    string `json:"type"`
	Scope   string `json:"scope"`
	Message string `json:"message"`
}

// VersionInfo 版本列表中的简要信息。
type VersionInfo struct {
	Version      string `json:"version"`
	Timestamp    string `json:"timestamp"`
	Files        int    `json:"files"`
	Lines        int    `json:"lines"`
	Symbols      int    `json:"symbols"`
	ChangelogLen int    `json:"changelog_len"`
}

// VersionDiff 两个版本之间的差异。
type VersionDiff struct {
	VersionFrom  string                   `json:"version_from"`
	VersionTo    string                   `json:"version_to"`
	FilesAdded   []string                 `json:"files_added"`
	FilesRemoved []string                 `json:"files_removed"`
	FilesChanged []FileDiffEntry          `json:"files_changed"`
	LayerDrift   map[string]LayerDriftEntry `json:"layer_drift"`
	TotalDiff    TotalDiffStat            `json:"total_diff"`
}

// FileDiffEntry 文件差异详情。
type FileDiffEntry struct {
	Name          string `json:"name"`
	LinesBefore   int    `json:"lines_before"`
	LinesAfter    int    `json:"lines_after"`
	SymbolsBefore int    `json:"symbols_before"`
	SymbolsAfter  int    `json:"symbols_after"`
}

// LayerDriftEntry 层级漂移。
type LayerDriftEntry struct {
	FilesBefore int `json:"files_before"`
	FilesAfter  int `json:"files_after"`
}

// TotalDiffStat 全局差异统计。
type TotalDiffStat struct {
	LinesAdded    int `json:"lines_added"`
	LinesRemoved  int `json:"lines_removed"`
	SymbolsAdded  int `json:"symbols_added"`
	SymbolsRemoved int `json:"symbols_removed"`
}

// ============================================================================
// SECTION 8: Git 操作类型
// ============================================================================

// GitLogOptions 配置 git log 查询参数。
type GitLogOptions struct {
	MaxCount   int    `json:"max_count,omitempty"`   // -n 限制数量
	SkipMerges bool   `json:"skip_merges"`           // --no-merges
	Format     string `json:"format,omitempty"`      // --format 格式
	Since      string `json:"since,omitempty"`       // --since 起始时间/标签
	Until      string `json:"until,omitempty"`        // --until 截止时间/标签
	Reverse    bool   `json:"reverse,omitempty"`      // --reverse 倒序
}