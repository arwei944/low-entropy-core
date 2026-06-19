// Package core — 通用版本管理模块 (v0.8.0)
//
// version_change.go: ArchChange 变更意图文件系统
//
// 功能：
//   - 创建/读取/合并/删除 .arch-change/*.md 变更意图文件
//   - 类似 changesets 模式，每个变更一个 Markdown 文件
//   - 支持 YAML front matter + Markdown body 格式

package core

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ============================================================================
// 变更意图文件 CRUD
// ============================================================================

// changeDirName 变更意图文件存储目录。
const changeDirName = ".arch-change"

// GenerateChangeID 生成变更意图 ID。
// 格式: change-YYYYMMDD-NNN
func GenerateChangeID() string {
	now := time.Now()
	return fmt.Sprintf("change-%s-%03d", now.Format("20060102"), now.UnixMilli()%1000)
}

// CreateChange 在指定目录下创建变更意图文件。
// dir 为项目根目录（.arch-change/ 将创建在 dir 下）。
func CreateChange(dir string, intent ChangeIntent) error {
	changesDir := filepath.Join(dir, changeDirName)
	if err := os.MkdirAll(changesDir, 0755); err != nil {
		return fmt.Errorf("create .arch-change dir: %w", err)
	}

	// 验证变更意图
	if err := ValidateChange(intent); err != nil {
		return err
	}

	// 生成 ID（如果未提供）
	if intent.ID == "" {
		intent.ID = GenerateChangeID()
	}
	if intent.CreatedAt.IsZero() {
		intent.CreatedAt = time.Now()
	}

	// 生成文件名
	filename := fmt.Sprintf("%s.md", intent.ID)
	path := filepath.Join(changesDir, filename)

	// 序列化为 Markdown
	content := formatChangeToMarkdown(intent)
	return os.WriteFile(path, []byte(content), 0644)
}

// ReadChange 从文件路径读取变更意图。
func ReadChange(path string) (ChangeIntent, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return ChangeIntent{}, fmt.Errorf("read change file %s: %w", path, err)
	}
	return parseChangeFromMarkdown(string(data))
}

// ListChanges 列出指定目录下所有变更意图。
func ListChanges(dir string) ([]ChangeIntent, error) {
	changesDir := filepath.Join(dir, changeDirName)
	entries, err := os.ReadDir(changesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []ChangeIntent{}, nil
		}
		return nil, fmt.Errorf("read .arch-change dir: %w", err)
	}

	changes := make([]ChangeIntent, 0)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		path := filepath.Join(changesDir, entry.Name())
		change, err := ReadChange(path)
		if err != nil {
			continue // 跳过无法解析的文件
		}
		change.ID = strings.TrimSuffix(entry.Name(), ".md")
		changes = append(changes, change)
	}

	// 按创建时间降序
	sort.Slice(changes, func(i, j int) bool {
		return changes[i].CreatedAt.After(changes[j].CreatedAt)
	})

	return changes, nil
}

// MergeChanges 合并所有变更意图为一个 ArchChange。
func MergeChanges(dir string) (ArchChange, error) {
	changes, err := ListChanges(dir)
	if err != nil {
		return ArchChange{}, err
	}

	ac := ArchChange{
		ID:        GenerateChangeID(),
		Intents:   changes,
		Timestamp: time.Now(),
		Status:    "merged",
	}

	return ac, nil
}

// DeleteChange 删除指定变更意图文件。
func DeleteChange(path string) error {
	return os.Remove(path)
}

// DeleteChangeByID 根据 ID 删除变更意图。
func DeleteChangeByID(dir string, id string) error {
	changesDir := filepath.Join(dir, changeDirName)
	path := filepath.Join(changesDir, fmt.Sprintf("%s.md", id))
	return DeleteChange(path)
}

// ValidateChange 验证变更意图的完整性。
func ValidateChange(intent ChangeIntent) error {
	if strings.TrimSpace(intent.Title) == "" {
		return fmt.Errorf("change title is required")
	}
	if !IsValidType(intent.Type) {
		return fmt.Errorf("invalid change type: %q (valid: %s)", intent.Type, strings.Join(ValidTypes(), ", "))
	}
	return nil
}

// ============================================================================
// Markdown 序列化/反序列化
// ============================================================================

// formatChangeToMarkdown 将 ChangeIntent 序列化为 Markdown 格式。
// 使用 YAML front matter + Markdown body 格式。
func formatChangeToMarkdown(intent ChangeIntent) string {
	var b strings.Builder

	// Front matter
	b.WriteString("---\n")
	b.WriteString(fmt.Sprintf("title: %q\n", intent.Title))
	b.WriteString(fmt.Sprintf("type: %s\n", intent.Type))
	if intent.Scope != "" {
		b.WriteString(fmt.Sprintf("scope: %s\n", intent.Scope))
	}
	b.WriteString(fmt.Sprintf("breaking: %v\n", intent.Breaking))
	if len(intent.Files) > 0 {
		b.WriteString("files:\n")
		for _, f := range intent.Files {
			b.WriteString(fmt.Sprintf("  - %s\n", f))
		}
	}
	if intent.Migration != "" {
		b.WriteString(fmt.Sprintf("migration: %q\n", intent.Migration))
	}
	if intent.Author != "" {
		b.WriteString(fmt.Sprintf("author: %s\n", intent.Author))
	}
	b.WriteString(fmt.Sprintf("created_at: %s\n", intent.CreatedAt.Format(time.RFC3339)))
	b.WriteString("---\n\n")

	// Body
	b.WriteString(intent.Description)
	b.WriteString("\n")

	return b.String()
}

// parseChangeFromMarkdown 从 Markdown 字符串解析 ChangeIntent。
func parseChangeFromMarkdown(content string) (ChangeIntent, error) {
	intent := ChangeIntent{}

	// 简单解析 front matter (--- 分隔)
	parts := strings.SplitN(content, "---\n", 3)
	if len(parts) < 3 {
		return ChangeIntent{}, fmt.Errorf("invalid markdown format: missing front matter")
	}

	frontMatter := parts[1]
	body := strings.TrimSpace(parts[2])

	// 解析 front matter 字段
	lines := strings.Split(frontMatter, "\n")
	currentList := ""
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// 处理列表项
		if strings.HasPrefix(line, "- ") {
			if currentList == "files" {
				intent.Files = append(intent.Files, strings.TrimPrefix(line, "- "))
			}
			continue
		}

		currentList = ""

		// 处理键值对
		kv := strings.SplitN(line, ":", 2)
		if len(kv) != 2 {
			continue
		}

		key := strings.TrimSpace(kv[0])
		value := strings.TrimSpace(kv[1])
		// 去除引号
		value = strings.Trim(value, `"`)

		switch key {
		case "title":
			intent.Title = value
		case "type":
			intent.Type = value
		case "scope":
			intent.Scope = value
		case "breaking":
			intent.Breaking = value == "true"
		case "files":
			currentList = "files"
		case "migration":
			intent.Migration = value
		case "author":
			intent.Author = value
		case "created_at":
			if t, err := time.Parse(time.RFC3339, value); err == nil {
				intent.CreatedAt = t
			}
		}
	}

	intent.Description = body

	return intent, nil
}

// ChangeDir 返回变更意图文件存储目录路径。
func ChangeDir(rootDir string) string {
	return filepath.Join(rootDir, changeDirName)
}