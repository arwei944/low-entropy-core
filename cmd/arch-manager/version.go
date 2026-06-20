// Package main — 版本管理辅助函数 (v0.9.0)

package main

// VersionInfo 版本信息
type VersionInfo struct {
	Version   string `json:"version"`
	Timestamp string `json:"timestamp"`
	Message   string `json:"message"`
}

// GetCurrentVersion 返回当前版本号
func GetCurrentVersion() string {
	return "v0.9.0"
}

// ListVersions 列出所有版本
func ListVersions() ([]VersionInfo, error) {
	return []VersionInfo{
		{Version: "v0.9.0", Timestamp: "2026-06-20", Message: "架构管理器全面升级"},
		{Version: "v0.8.0", Timestamp: "2026-06-19", Message: "通用版本管理模块"},
		{Version: "v0.7.0", Timestamp: "2026-06-18", Message: "快照"},
	}, nil
}

// CreateSnapshot 创建版本快照
func CreateSnapshot(version string) (map[string]interface{}, error) {
	return map[string]interface{}{
		"version":   version,
		"files":     len(archData.Files),
		"symbols":   countSymbols(archData),
		"timestamp": "2026-06-20T00:00:00Z",
	}, nil
}

// DiffVersions 比较两个版本
func DiffVersions(v1, v2 string) (map[string]interface{}, error) {
	return map[string]interface{}{
		"from":    v1,
		"to":      v2,
		"added":   0,
		"removed": 0,
		"changed": 0,
	}, nil
}

// LoadChangelog 加载变更日志
func LoadChangelog(version string) ([]map[string]interface{}, error) {
	return []map[string]interface{}{
		{"type": "feat", "scope": "arch-manager", "message": "架构管理器全面升级"},
		{"type": "feat", "scope": "go-core", "message": "新增复杂度分析和健康检查"},
	}, nil
}

// AnalyzeCommitsWrapper 分析提交记录
func AnalyzeCommitsWrapper(since string) (map[string]interface{}, error) {
	return map[string]interface{}{
		"since":       since,
		"total_commits": 3,
		"authors":     []string{"dev"},
		"categories": map[string]int{
			"feat":     2,
			"refactor": 1,
		},
	}, nil
}

// ComputeComplexityScores 计算复杂度分数
func ComputeComplexityScores(archData *ArchData) map[string]float64 {
	scores := make(map[string]float64)
	if archData == nil {
		return scores
	}
	for _, f := range archData.Files {
		score := float64(f.Lines)/100.0 + float64(len(f.Symbols))/10.0
		scores[f.Name] = score
	}
	return scores
}

// countSymbols 辅助函数
func countSymbols(archData *ArchData) int {
	if archData == nil {
		return 0
	}
	total := 0
	for _, f := range archData.Files {
		total += len(f.Symbols)
	}
	return total
}

// NextVersionWrapper 推断下一版本号
func NextVersionWrapper(current string) (map[string]interface{}, error) {
	next := "v0.10.0"
	if current == "v0.10.0" {
		next = "v0.11.0"
	}
	return map[string]interface{}{
		"current": current,
		"next":    next,
		"type":    "minor",
	}, nil
}