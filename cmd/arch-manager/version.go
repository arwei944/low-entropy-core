// Architecture Manager v0.6.0 — 版本管理模块
//
// 功能：
//   - 版本快照创建、存储、列表
//   - 版本间 diff 对比
//   - Git Conventional Commits Changelog 提取
//   - 版本快照存储在 versions/ 目录下

package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ============================================================================
// 版本管理数据模型
// ============================================================================

// VersionSnapshot 表示一个版本快照
type VersionSnapshot struct {
	Version   string           `json:"version"`
	Timestamp time.Time        `json:"timestamp"`
	Semver    SemverInfo       `json:"semver"`
	Snapshot  SnapshotData     `json:"snapshot"`
	Changelog []ChangelogEntry `json:"changelog"`
}

// SemverInfo 语义化版本号分解
type SemverInfo struct {
	Major int `json:"major"`
	Minor int `json:"minor"`
	Patch int `json:"patch"`
}

// SnapshotData 版本快照数据
type SnapshotData struct {
	Files       map[string]FileSnapshot `json:"files"`
	LayerStats  map[string]LayerStat    `json:"layer_stats"`
	Total       TotalStat               `json:"total"`
}

// FileSnapshot 单个文件的快照信息
type FileSnapshot struct {
	Hash    string `json:"hash"`
	Lines   int    `json:"lines"`
	Symbols int    `json:"symbols"`
}

// TotalStat 全局统计
type TotalStat struct {
	Files   int `json:"files"`
	Lines   int `json:"lines"`
	Symbols int `json:"symbols"`
}

// ChangelogEntry 一条 Changelog 记录
type ChangelogEntry struct {
	Type    string `json:"type"`    // feat, fix, refactor, docs, test, chore
	Scope   string `json:"scope"`
	Message string `json:"message"`
}

// VersionInfo 版本列表中的简要信息
type VersionInfo struct {
	Version      string `json:"version"`
	Timestamp    string `json:"timestamp"`
	Files        int    `json:"files"`
	Lines        int    `json:"lines"`
	Symbols      int    `json:"symbols"`
	ChangelogLen int    `json:"changelog_len"`
}

// VersionDiff 两个版本之间的差异
type VersionDiff struct {
	VersionFrom string            `json:"version_from"`
	VersionTo   string            `json:"version_to"`
	FilesAdded  []string          `json:"files_added"`
	FilesRemoved []string         `json:"files_removed"`
	FilesChanged []FileDiffEntry  `json:"files_changed"`
	LayerDrift  map[string]LayerDriftEntry `json:"layer_drift"`
	TotalDiff   TotalDiffStat     `json:"total_diff"`
}

// FileDiffEntry 文件差异详情
type FileDiffEntry struct {
	Name          string `json:"name"`
	LinesBefore   int    `json:"lines_before"`
	LinesAfter    int    `json:"lines_after"`
	SymbolsBefore int    `json:"symbols_before"`
	SymbolsAfter  int    `json:"symbols_after"`
}

// LayerDriftEntry 层级漂移
type LayerDriftEntry struct {
	FilesBefore int `json:"files_before"`
	FilesAfter  int `json:"files_after"`
}

// TotalDiffStat 全局差异统计
type TotalDiffStat struct {
	LinesAdded   int `json:"lines_added"`
	LinesRemoved int `json:"lines_removed"`
	SymbolsAdded int `json:"symbols_added"`
	SymbolsRemoved int `json:"symbols_removed"`
}

// ============================================================================
// 版本快照 CRUD
// ============================================================================

// versionsDir 返回版本快照存储目录
func versionsDir() string {
	return filepath.Join(sourceDir, "..", "versions")
}

// CreateSnapshot 扫描 go-core 目录，生成版本快照并保存
func CreateSnapshot(version string) (*VersionSnapshot, error) {
	// 解析语义版本号
	semver := parseSemver(version)

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

	// 提取 Changelog
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

// parseSemver 解析语义版本号
func parseSemver(v string) SemverInfo {
	info := SemverInfo{}
	// 去除 v 前缀
	v = strings.TrimPrefix(v, "v")
	_, err := fmt.Sscanf(v, "%d.%d.%d", &info.Major, &info.Minor, &info.Patch)
	if err != nil {
		// 尝试部分解析
		parts := strings.Split(v, ".")
		if len(parts) >= 1 {
			fmt.Sscanf(parts[0], "%d", &info.Major)
		}
		if len(parts) >= 2 {
			fmt.Sscanf(parts[1], "%d", &info.Minor)
		}
		if len(parts) >= 3 {
			fmt.Sscanf(parts[2], "%d", &info.Patch)
		}
	}
	return info
}

// saveSnapshot 将版本快照保存到磁盘
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

// LoadSnapshot 从磁盘加载指定版本快照
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

// ListVersions 列出所有已记录的版本
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

		// 解析文件名: v0.5.0.json -> 0.5.0
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

	// 按版本号降序排列
	sort.Slice(versions, func(i, j int) bool {
		vi := parseSemver(versions[i].Version)
		vj := parseSemver(versions[j].Version)
		if vi.Major != vj.Major {
			return vi.Major > vj.Major
		}
		if vi.Minor != vj.Minor {
			return vi.Minor > vj.Minor
		}
		return vi.Patch > vj.Patch
	})

	return versions, nil
}

// DiffVersions 对比两个版本的差异
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

	// 检测新增和修改的文件
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

	// 检测删除的文件
	for name := range s1.Snapshot.Files {
		if _, ok := s2.Snapshot.Files[name]; !ok {
			diff.FilesRemoved = append(diff.FilesRemoved, name)
		}
	}

	// 层级漂移
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

	// 全局差异
	diff.TotalDiff = TotalDiffStat{
		LinesAdded:    s2.Snapshot.Total.Lines - s1.Snapshot.Total.Lines,
		SymbolsAdded:  s2.Snapshot.Total.Symbols - s1.Snapshot.Total.Symbols,
	}

	sort.Strings(diff.FilesAdded)
	sort.Strings(diff.FilesRemoved)
	sort.Slice(diff.FilesChanged, func(i, j int) bool {
		return diff.FilesChanged[i].Name < diff.FilesChanged[j].Name
	})

	return diff, nil
}

// LoadChangelog 从 Git log 中提取 Conventional Commits 格式的变更记录
func LoadChangelog(version string) ([]ChangelogEntry, error) {
	entries := make([]ChangelogEntry, 0)

	// 尝试查找上一个版本的 tag
	prevTag := findPreviousTag(version)
	var rangeSpec string
	if prevTag != "" {
		rangeSpec = fmt.Sprintf("%s..HEAD", prevTag)
	} else {
		rangeSpec = "-50" // 最近 50 条提交
	}

	// 执行 git log
	cmd := exec.Command("git", "log", rangeSpec, "--oneline", "--no-merges")
	cmd.Dir = sourceDir
	output, err := cmd.Output()
	if err != nil {
		// git 不可用，返回空列表
		return entries, nil
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		entry := parseConventionalCommit(line)
		if entry != nil {
			entries = append(entries, *entry)
		}
	}

	return entries, nil
}

// findPreviousTag 查找指定版本之前的 Git tag
func findPreviousTag(version string) string {
	cmd := exec.Command("git", "tag", "--sort=-v:refname")
	cmd.Dir = sourceDir
	output, err := cmd.Output()
	if err != nil {
		return ""
	}

	tags := strings.Split(strings.TrimSpace(string(output)), "\n")
	currentTag := "v" + version

	found := false
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			continue
		}
		if found {
			return tag
		}
		if tag == currentTag {
			found = true
		}
	}

	return ""
}

// parseConventionalCommit 解析 Conventional Commit 格式：
// <type>(<scope>): <message>
// 也支持简化的 git log --oneline 格式：<hash> <type>(<scope>): <message>
func parseConventionalCommit(line string) *ChangelogEntry {
	// 跳过 hash 前缀（git log --oneline 格式）
	parts := strings.SplitN(line, " ", 2)
	if len(parts) == 2 {
		line = parts[1]
	}

	// 匹配 type(scope): message 或 type: message
	colonIdx := strings.Index(line, ":")
	if colonIdx < 0 {
		return nil
	}

	prefix := strings.TrimSpace(line[:colonIdx])
	message := strings.TrimSpace(line[colonIdx+1:])

	// 解析 type 和 scope
	entry := &ChangelogEntry{Message: message}
	openParen := strings.Index(prefix, "(")
	closeParen := strings.Index(prefix, ")")

	if openParen > 0 && closeParen > openParen {
		entry.Type = strings.TrimSpace(prefix[:openParen])
		entry.Scope = prefix[openParen+1 : closeParen]
	} else {
		entry.Type = prefix
	}

	// 验证 type 是否为有效的 Conventional Commit 类型
	validTypes := map[string]bool{
		"feat": true, "fix": true, "docs": true, "style": true,
		"refactor": true, "perf": true, "test": true, "chore": true,
		"ci": true, "build": true, "revert": true,
	}
	if !validTypes[entry.Type] {
		entry.Type = "chore"
	}

	return entry
}

// ============================================================================
// 复杂度评分 (BE-03)
// ============================================================================

// ComplexityInfo 文件复杂度信息
type ComplexityInfo struct {
	Score float64 `json:"score"` // 0-100 复杂度评分
}

// ComputeComplexityScores 为所有文件计算复杂度评分
func ComputeComplexityScores(data *ArchData) map[string]float64 {
	scores := make(map[string]float64)

	if len(data.Files) == 0 {
		return scores
	}

	// 计算最大值用于归一化
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

	// 计算每个文件的复杂度评分
	for _, f := range data.Files {
		score := (float64(f.Lines)/float64(maxLines))*0.4 +
			(float64(len(f.Symbols))/float64(maxSymbols))*0.3 +
			(float64(len(f.DependsOn))/float64(maxDeps))*0.3
		scores[f.Name] = mathRound(score*100, 1)
	}

	return scores
}

// mathRound 四舍五入到指定精度
func mathRound(val float64, decimals int) float64 {
	pow := 1.0
	for i := 0; i < decimals; i++ {
		pow *= 10
	}
	return float64(int(val*pow+0.5)) / pow
}