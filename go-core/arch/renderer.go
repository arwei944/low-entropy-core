// Package arch — Template Renderer Adapter (L1)
//
// 渲染架构脚手架模板。统一使用 embed.FS + text/template。
// Adapter：输入模板路径 + 数据 → 输出渲染后的字符串。
//
// 来源:
//   cmd/lec/scaffold.go    (embed.FS + text/template 引擎)
//   cmd/arch-dev/arch_scaffold.go  (字符串替换模板 → 迁移为标准模板)
//
// 设计约束: 文件 ≤ 300 行
package arch

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

//go:embed templates/*.tmpl
var templateFS embed.FS

// TemplateRenderer 模板渲染接口。
type TemplateRenderer interface {
	Render(templatePath string, data interface{}) (string, error)
	RenderToFile(templatePath, outputPath string, data interface{}) error
	ListTemplates() []string
}

// EmbedRenderer 基于 embed.FS 的默认实现。
type EmbedRenderer struct {
	tmpl *template.Template
}

// NewRenderer 创建默认渲染器，预加载 templates/ 目录下的所有模板。
func NewRenderer() (*EmbedRenderer, error) {
	t, err := template.ParseFS(templateFS, "templates/*.tmpl")
	if err != nil {
		return nil, fmt.Errorf("加载模板失败: %w", err)
	}
	return &EmbedRenderer{tmpl: t}, nil
}

// Render 渲染指定模板并返回字符串。
func (r *EmbedRenderer) Render(templatePath string, data interface{}) (string, error) {
	if r.tmpl == nil {
		return "", fmt.Errorf("渲染器未初始化")
	}

	name := filepath.Base(templatePath)
	if !strings.HasSuffix(name, ".tmpl") {
		name = name + ".tmpl"
	}

	var b strings.Builder
	if err := r.tmpl.ExecuteTemplate(&b, name, data); err != nil {
		return "", fmt.Errorf("渲染模板 %s 失败: %w", name, err)
	}
	return b.String(), nil
}

// RenderToFile 渲染模板并写入文件。
func (r *EmbedRenderer) RenderToFile(templatePath, outputPath string, data interface{}) error {
	content, err := r.Render(templatePath, data)
	if err != nil {
		return err
	}

	dir := filepath.Dir(outputPath)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("创建目录失败: %w", err)
		}
	}
	return os.WriteFile(outputPath, []byte(content), 0o644)
}

// ListTemplates 列出所有可用模板名。
func (r *EmbedRenderer) ListTemplates() []string {
	if r.tmpl == nil {
		return nil
	}
	tmpls := r.tmpl.Templates()
	names := make([]string, 0, len(tmpls))
	for _, t := range tmpls {
		if name := t.Name(); name != "" {
			names = append(names, name)
		}
	}
	return names
}
