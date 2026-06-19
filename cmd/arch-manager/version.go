// Architecture Manager v0.8.0 — 版本管理模块（重构版）
//
// 重构说明：
//   - 删除 parseSemver()、parseConventionalCommit()、findPreviousTag() 等重复逻辑
//   - 全部调用 go-core 通用版本管理模块
//   - 版本快照 CRUD 保持不变（依赖 arch-manager 特有的 ArchData 结构）

package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	core "low-entropy-core/go-core"
)

// ============================================================================
// 版本管理数据模型（使用 go-core 类型）
// ============================================================================

// VersionSnapshot 表示一个版本快照。
type VersionSnapshot struct {
	Version   string              `json:"version"`
	Timestamp time.Time           `json:"timestamp"`
	Semver    core.Semver         `json:"semver"`
	Snapshot  SnapshotData        `json:"snapshot"`
	Changelog []core.ChangelogEntry `json:"changelog"`
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
	VersionFrom   string                     `json:"version_from"`
	VersionTo     string                     `json:"version_to"`
	FilesAdded    []string                   `json:"files_added"`
	FilesRemoved  []string                   `json:"files_removed"`
	FilesChanged  []FileDiffEntry            `json:"files_changed"`
	LayerDrift    map[string]LayerDriftEntry `json:"layer_drift"`
	TotalDiff     TotalDiffStat              `json:"total_diff"`
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
// 版本快照 CRUD
// ============================================================================

// versionsDir 返回版本快照存储目录。
func versionsDir() string {
	return filepath.Join(sourceDir, "..", "versions")
}

// CreateSnapshot 扫描 go-core 目录，生成版本快照并保存。
func CreateSnapshot(version string) (*VersionSnapshot, error) {
	// 使用 go-core 解析语义版本号
	semver, err := core.ParseSemver(version)
	if err != nil {
		return nil, fmt.Errorf("parse semver: %w", err)
	}

	// 构建架构数据
	data, err := buildArchData(sourceDir)
	if err != nil {
		return nil, fmt.Errorf("build arch data: %w", err)
	}

	// 计算文件哈希
	files := make(map[string]FileSnapshot)
	for _, f := range data.Files {
		content, err := os.ReadFile(f.Path)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", f.Path, err)
		}
		hash := fmt.Sprintf("sha256:%x", sha256.Sum256(content))
		files[f.Name] = FileSnapshot{
			Hash:    hash,
			Lines:   f.Lines,
			Symbols: len(f.Symbols),
		}
	}

	// 层级统计
	layerStats := make(map[string]LayerStat)
	for _, l := range data.Layers {
		layerStats[l.Layer] = l
	}

	// 提取 Changelog（使用 go-core 的 Git 操作）
	changelog, _ := LoadChangelog(version)

	snapshot := &VersionSnapshot{
		Version:   version,
		Timestamp: time.Now(),
		Semver:    semver,
		Snapshot: SnapshotData{
			Files:      files,
			LayerStats: layerStats,
			Total: TotalStat{
				Files:   data.TotalFiles,
				Lines:   data.TotalLines,
				Symbols: data.TotalSymbols,
			},
		},
		Changelog: changelog,
	}

	// 保存到磁盘
	if err := saveSnapshot(snapshot); err != nil {
		return nil, fmt.Errorf("save snapshot: %w", err)
	}

	return snapshot, nil
}

// saveSnapshot 将版本快照保存到磁盘。
func saveSnapshot(s *VersionSnapshot) error {
	dir := versionsDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	path := filepath.Join(dir, fmt.Sprintf("v%s.json", s.Version))
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

// LoadSnapshot 从磁盘加载指定版本快照。
func LoadSnapshot(version string) (*VersionSnapshot, error) {
	path := filepath.Join(versionsDir(), fmt.Sprintf("v%s.json", version))
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("load snapshot %s: %w", version, err)
	}

	var s VersionSnapshot
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("unmarshal snapshot %s: %w", version, err)
	}

	return &s, nil
}

// ListVersions 列出所有已记录的版本。
func ListVersions() ([]VersionInfo, error) {
	dir := versionsDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []VersionInfo{}, nil
		}
		return nil, err
	}

	versions := make([]VersionInfo, 0)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		name := strings.TrimPrefix(entry.Name(), "v")
		name = strings.TrimSuffix(name, ".json")

		snapshot, err := LoadSnapshot(name)
		if err != nil {
			continue
		}

		versions = append(versions, VersionInfo{
			Version:      snapshot.Version,
			Timestamp:    snapshot.Timestamp.Format(time.RFC3339),
			Files:        snapshot.Snapshot.Total.Files,
			Lines:        snapshot.Snapshot.Total.Lines,
			Symbols:      snapshot.Snapshot.Total.Symbols,
			ChangelogLen: len(snapshot.Changelog),
		})
	}

	// 使用 go-core 的 SemVer 比较排序
	sort.Slice(versions, func(i, j int) bool {
		vi, _ := core.ParseSemver(versions[i].Version)
		vj, _ := core.ParseSemver(versions[j].Version)
		return vi.Compare(vj) > 0
	})

	return versions, nil
}

// DiffVersions 对比两个版本的差异。
func DiffVersions(v1, v2 string) (*VersionDiff, error) {
	s1, err := LoadSnapshot(v1)
	if err != nil {
		return nil, fmt.Errorf("load v1: %w", err)
	}
	s2, err := LoadSnapshot(v2)
	if err != nil {
		return nil, fmt.Errorf("load v2: %w", err)
	}

	diff := &VersionDiff{
		VersionFrom:  v1,
		VersionTo:    v2,
		FilesAdded:   []string{},
		FilesRemoved: []string{},
		FilesChanged: []FileDiffEntry{},
		LayerDrift:   make(map[string]LayerDriftEntry),
	}

	for name, f2 := range s2.Snapshot.Files {
		if f1, ok := s1.Snapshot.Files[name]; ok {
			if f1.Hash != f2.Hash {
				diff.FilesChanged = append(diff.FilesChanged, FileDiffEntry{
					Name:          name,
					LinesBefore:   f1.Lines,
					LinesAfter:    f2.Lines,
					SymbolsBefore: f1.Symbols,
					SymbolsAfter:  f2.Symbols,
				})
			}
		} else {
			diff.FilesAdded = append(diff.FilesAdded, name)
		}
	}

	for name := range s1.Snapshot.Files {
		if _, ok := s2.Snapshot.Files[name]; !ok {
			diff.FilesRemoved = append(diff.FilesRemoved, name)
		}
	}

	for layer, l2 := range s2.Snapshot.LayerStats {
		l1, ok := s1.Snapshot.LayerStats[layer]
		if !ok {
			diff.LayerDrift[layer] = LayerDriftEntry{
				FilesBefore: 0,
				FilesAfter:  l2.Files,
			}
			continue
		}
		if l1.Files != l2.Files {
			diff.LayerDrift[layer] = LayerDriftEntry{
				FilesBefore: l1.Files,
				FilesAfter:  l2.Files,
			}
		}
	}

	diff.TotalDiff = TotalDiffStat{
		LinesAdded:   s2.Snapshot.Total.Lines - s1.Snapshot.Total.Lines,
		SymbolsAdded: s2.Snapshot.Total.Symbols - s1.Snapshot.Total.Symbols,
	}

	sort.Strings(diff.FilesAdded)
	sort.Strings(diff.FilesRemoved)
	sort.Slice(diff.FilesChanged, func(i, j int) bool {
		return diff.FilesChanged[i].Name < diff.FilesChanged[j].Name
	})

	return diff, nil
}

// LoadChangelog 从 Git log 中提取 Conventional Commits 格式的变更记录。
// 重构为使用 go-core 的 Git 操作和 Commit 解析。
func LoadChangelog(version string) ([]core.ChangelogEntry, error) {
	entries := make([]core.ChangelogEntry, 0)

	// 使用 go-core 获取最近 tag
	lastTag, _ := core.GitLastTag(sourceDir)
	var log string
	var err error

	if lastTag != "" {
		log, err = core.GitCommitsSinceTag(sourceDir, lastTag)
	} else {
		log, err = core.GitLog(sourceDir, core.GitLogOptions{MaxCount: 50, SkipMerges: true})
	}
	if err != nil {
		return entries, nil
	}

	// 使用 go-core 解析 Conventional Commits
	commits, err := core.ParseCommitsFromLog(log)
	if err != nil {
		return entries, nil
	}

	for _, c := range commits {
		entries = append(entries, core.ChangelogEntry{
			Type:    c.Type,
			Scope:   c.Scope,
			Message: c.Description,
		})
	}

	return entries, nil
}

// ============================================================================
// 新增：版本分析 API 辅助函数
// ============================================================================

// AnalyzeCommitsWrapper 分析 Git 提交并返回版本推断。
func AnalyzeCommitsWrapper(since string) (map[string]interface{}, error) {
	repoDir := filepath.Join(sourceDir, "..")

	var log string
	var err error
	if since != "" {
		log, err = core.GitCommitsSinceTag(repoDir, since)
	} else {
		log, err = core.GitCommitsSinceTag(repoDir, "")
	}
	if err != nil {
		return nil, fmt.Errorf("git log: %w", err)
	}

	commits, _ := core.ParseCommitsFromLog(log)
	class := core.ClassifyCommits(commits)
	bump := core.InferBump(commits)

	current := core.Semver{Major: 0, Minor: 7, Patch: 0}
	lastTag, _ := core.GitLastTag(repoDir)
	if lastTag != "" {
		if parsed, err := core.ParseSemver(lastTag); err == nil {
			current = parsed
		}
	}
	next := core.InferNextVersion(commits, current)

	return map[string]interface{}{
		"commits":      commits,
		"total":        len(commits),
		"classification": class,
		"bump":         bump,
		"current":      current.String(),
		"next_version": next.String(),
	}, nil
}

// NextVersionWrapper 推断下一版本号。
func NextVersionWrapper(currentStr string) (map[string]interface{}, error) {
	repoDir := filepath.Join(sourceDir, "..")

	current := core.Semver{Major: 0, Minor: 7, Patch: 0}
	if currentStr != "" {
		if parsed, err := core.ParseSemver(currentStr); err == nil {
			current = parsed
		}
	}

	log, err := core.GitCommitsSinceTag(repoDir, "")
	if err != nil {
		// 无法获取提交，保持当前版本
		return map[string]interface{}{
			"current": current.String(),
			"next":    current.String(),
			"bump":    "none",
		}, nil
	}

	commits, _ := core.ParseCommitsFromLog(log)
	bump := core.InferBump(commits)
	next := core.InferNextVersion(commits, current)

	bumpType := "patch"
	switch {
	case bump.Major:
		bumpType = "major"
	case bump.Minor:
		bumpType = "minor"
	}

	return map[string]interface{}{
		"current":  current.String(),
		"next":     next.String(),
		"bump":     bumpType,
		"breaking": bump.Major,
		"commits":  len(commits),
	}, nil
}

// GetCurrentVersion 动态获取当前版本号（从快照或 Git tag）。
func GetCurrentVersion() string {
	// 优先从版本快照获取
	versions, err := ListVersions()
	if err == nil && len(versions) > 0 {
		return versions[0].Version
	}

	// 其次从 Git tag 获取
	repoDir := filepath.Join(sourceDir, "..")
	tag, err := core.GitLastTag(repoDir)
	if err == nil && tag != "" {
		if sv, err := core.ParseSemver(tag); err == nil {
			return sv.String()
		}
	}

	return "0.8.0"
}

// ============================================================================
// 复杂度评分
// ============================================================================

// ComplexityInfo 文件复杂度信息。
type ComplexityInfo struct {
	Score float64 `json:"score"`
}

// ComputeComplexityScores 为所有文件计算复杂度评分。
func ComputeComplexityScores(data *ArchData) map[string]float64 {
	scores := make(map[string]float64)

	if len(data.Files) == 0 {
		return scores
	}

	maxLines := 1
	maxSymbols := 1
	maxDeps := 1
	for _, f := range data.Files {
		if f.Lines > maxLines {
			maxLines = f.Lines
		}
		if len(f.Symbols) > maxSymbols {
			maxSymbols = len(f.Symbols)
		}
		if len(f.DependsOn) > maxDeps {
			maxDeps = len(f.DependsOn)
		}
	}

	for _, f := range data.Files {
		score := (float64(f.Lines)/float64(maxLines))*0.4 +
			(float64(len(f.Symbols))/float64(maxSymbols))*0.3 +
			(float64(len(f.DependsOn))/float64(maxDeps))*0.3
		scores[f.Name] = mathRound(score*100, 1)
	}

	return scores
}

func mathRound(val float64, decimals int) float64 {
	pow := 1.0
	for i := 0; i < decimals; i++ {
		pow *= 10
	}
	return float64(int(val*pow+0.5)) / pow
}