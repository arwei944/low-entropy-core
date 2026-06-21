// Package core — ADR (Architecture Decision Records) 管理
package core

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// adrDirName is the ADR file storage directory.
const adrDirName = "docs/adr"

// NextADRID generates the next ADR ID in format ADR-NNNN.
func NextADRID(dir string) (string, error) {
	adrs, err := ListADRs(dir)
	if err != nil {
		return "", err
	}

	maxNum := 0
	for _, adr := range adrs {
		// 解析 ADR-NNNN 格式
		if strings.HasPrefix(adr.ID, "ADR-") {
			num := 0
			fmt.Sscanf(adr.ID, "ADR-%d", &num)
			if num > maxNum {
				maxNum = num
			}
		}
	}

	return fmt.Sprintf("ADR-%04d", maxNum+1), nil
}

// CreateADR 创建 ADR 文件。
func CreateADR(dir string, adr ADR) error {
	adrDir := filepath.Join(dir, adrDirName)
	if err := os.MkdirAll(adrDir, 0755); err != nil {
		return fmt.Errorf("create adr dir: %w", err)
	}

	// 验证 ADR
	if err := validateADR(adr); err != nil {
		return err
	}

	// 生成 ID（如果未提供）
	if adr.ID == "" {
		id, err := NextADRID(dir)
		if err != nil {
			id = fmt.Sprintf("ADR-%04d", time.Now().UnixMilli()%10000)
		}
		adr.ID = id
	}
	if adr.Date.IsZero() {
		adr.Date = time.Now()
	}
	if adr.Status == "" {
		adr.Status = ADRStatusProposed
	}

	// 生成文件名
	filename := fmt.Sprintf("%s-%s.md", adr.ID, sanitizeFilename(adr.Title))
	path := filepath.Join(adrDir, filename)

	// 序列化为 Markdown
	content := formatADRToMarkdown(adr)
	return os.WriteFile(path, []byte(content), 0644)
}

// ReadADR 从文件路径读取 ADR。
func ReadADR(path string) (ADR, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return ADR{}, fmt.Errorf("read adr file %s: %w", path, err)
	}
	return parseADRFromMarkdown(string(data), path)
}

// ListADRs 列出所有 ADR。
func ListADRs(dir string) ([]ADR, error) {
	adrDir := filepath.Join(dir, adrDirName)
	entries, err := os.ReadDir(adrDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []ADR{}, nil
		}
		return nil, fmt.Errorf("read adr dir: %w", err)
	}

	adrs := make([]ADR, 0)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		path := filepath.Join(adrDir, entry.Name())
		adr, err := ReadADR(path)
		if err != nil {
			continue
		}
		adr.FilePath = path
		adrs = append(adrs, adr)
	}

	// 按 ID 降序排列
	sort.Slice(adrs, func(i, j int) bool {
		return adrs[i].ID > adrs[j].ID
	})

	return adrs, nil
}

// ADRByVersion 按版本号过滤 ADR。
func ADRByVersion(dir string, version Semver) ([]ADR, error) {
	all, err := ListADRs(dir)
	if err != nil {
		return nil, err
	}

	verStr := version.String()
	filtered := make([]ADR, 0)
	for _, adr := range all {
		if adr.Version == verStr {
			filtered = append(filtered, adr)
		}
	}
	return filtered, nil
}

// StatusLabel 返回 ADR 状态的中文标签。
func (a ADR) StatusLabel() string {
	switch a.Status {
	case ADRStatusProposed:
		return "提议中"
	case ADRStatusAccepted:
		return "已接受"
	case ADRStatusDeprecated:
		return "已废弃"
	case ADRStatusSuperseded:
		return "已替代"
	default:
		return a.Status
	}
}

// validateADR 验证 ADR 的完整性。
func validateADR(adr ADR) error {
	if strings.TrimSpace(adr.Title) == "" {
		return fmt.Errorf("ADR title is required")
	}
	if adr.Status != "" {
		valid := false
		for _, s := range ValidADRStatuses() {
			if adr.Status == s {
				valid = true
				break
			}
		}
		if !valid {
			return fmt.Errorf("invalid ADR status: %q (valid: %s)", adr.Status, strings.Join(ValidADRStatuses(), ", "))
		}
	}
	return nil
}

// sanitizeFilename 清理文件名中的非法字符。
func sanitizeFilename(title string) string {
	title = strings.ToLower(title)
	title = strings.ReplaceAll(title, " ", "-")
	// 只保留字母、数字、连字符
	result := strings.Builder{}
	for _, c := range title {
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-' {
			result.WriteRune(c)
		}
	}
	s := result.String()
	if s == "" {
		s = "untitled"
	}
	return s
}

// formatADRToMarkdown 将 ADR 序列化为 Markdown。
func formatADRToMarkdown(adr ADR) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("# %s: %s\n\n", adr.ID, adr.Title))
	b.WriteString(fmt.Sprintf("- Status: %s\n", adr.Status))
	if adr.Version != "" {
		b.WriteString(fmt.Sprintf("- Version: %s\n", adr.Version))
	}
	b.WriteString(fmt.Sprintf("- Date: %s\n", adr.Date.Format("2006-01-02")))
	if adr.SupersededBy != "" {
		b.WriteString(fmt.Sprintf("- Superseded By: %s\n", adr.SupersededBy))
	}
	b.WriteString("\n")

	if adr.Context != "" {
		b.WriteString("## Context\n\n")
		b.WriteString(adr.Context)
		b.WriteString("\n\n")
	}
	if adr.Decision != "" {
		b.WriteString("## Decision\n\n")
		b.WriteString(adr.Decision)
		b.WriteString("\n\n")
	}
	if adr.Consequences != "" {
		b.WriteString("## Consequences\n\n")
		b.WriteString(adr.Consequences)
		b.WriteString("\n\n")
	}

	return b.String()
}

// parseADRFromMarkdown 从 Markdown 解析 ADR。
func parseADRFromMarkdown(content string, path string) (ADR, error) {
	adr := ADR{FilePath: path}

	lines := strings.Split(content, "\n")
	section := ""
	sectionContent := strings.Builder{}

	for _, line := range lines {
		// 检测章节标题
		if strings.HasPrefix(line, "## ") {
			// 保存上一个章节
			saveADRSection(&adr, section, sectionContent.String())
			section = strings.TrimPrefix(line, "## ")
			sectionContent.Reset()
			continue
		}

		// 解析元数据行
		if section == "" && strings.HasPrefix(line, "- ") {
			kv := strings.TrimPrefix(line, "- ")
			parts := strings.SplitN(kv, ": ", 2)
			if len(parts) == 2 {
				switch strings.ToLower(parts[0]) {
				case "status":
					adr.Status = parts[1]
				case "version":
					adr.Version = parts[1]
				case "date":
					if t, err := time.Parse("2006-01-02", parts[1]); err == nil {
						adr.Date = t
					}
				case "superseded by":
					adr.SupersededBy = parts[1]
				}
			}
			continue
		}

		// 解析标题
		if section == "" && strings.HasPrefix(line, "# ") {
			titleLine := strings.TrimPrefix(line, "# ")
			// 解析 "ADR-0001: Title" 格式
			if idx := strings.Index(titleLine, ": "); idx > 0 {
				adr.ID = strings.TrimSpace(titleLine[:idx])
				adr.Title = strings.TrimSpace(titleLine[idx+2:])
			} else {
				adr.Title = titleLine
			}
			continue
		}

		sectionContent.WriteString(line)
		sectionContent.WriteString("\n")
	}

	// 保存最后一个章节
	saveADRSection(&adr, section, sectionContent.String())

	return adr, nil
}

// saveADRSection 将章节内容保存到 ADR 结构体。
func saveADRSection(adr *ADR, section string, content string) {
	content = strings.TrimSpace(content)
	if content == "" {
		return
	}

	switch strings.ToLower(section) {
	case "context":
		adr.Context = content
	case "decision":
		adr.Decision = content
	case "consequences":
		adr.Consequences = content
	}
}