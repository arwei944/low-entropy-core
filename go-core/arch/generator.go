// Package arch — Code Generator Composer (L1)
//
// 代码生成编排器。调用 Renderer（Adapter）+ 组合模板 → 输出完整项目。
// Composer：输入 GenConfig → 输出项目文件。
//
// 来源:
//   cmd/arch-dev/arch_init.go + arch_new.go + arch_add.go
//
// 设计约束: 文件 ≤ 300 行
package arch

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// GenConfig 项目生成配置。
type GenConfig struct {
	Name    string
	Package string
	Tier    string // "microservice" | "monolith" | "lib"
	Root    string // 输出根目录
}

// GenResult 一次生成操作的结果。
type GenResult struct {
	CreatedAt time.Time `json:"created_at"`
	Files     []string  `json:"files"`
	Root      string    `json:"root"`
}

// Generator 代码生成 Composer 接口。
type Generator interface {
	InitProject(cfg GenConfig) (*GenResult, error)
	NewModule(cfg GenConfig) (*GenResult, error)
	AddFeature(cfg GenConfig, featureName string) (*GenResult, error)
}

// defaultGenerator 默认代码生成器。
type defaultGenerator struct {
	renderer *EmbedRenderer
}

// NewGenerator 创建默认生成器。
func NewGenerator() (Generator, error) {
	r, err := NewRenderer()
	if err != nil {
		return nil, err
	}
	return &defaultGenerator{renderer: r}, nil
}

// InitProject 初始化一个新的项目（完整脚手架）。
func (g *defaultGenerator) InitProject(cfg GenConfig) (*GenResult, error) {
	result := &GenResult{
		CreatedAt: time.Now(),
		Root:      cfg.Root,
		Files:     make([]string, 0),
	}

	// 1. 创建目录
	if err := os.MkdirAll(filepath.Join(cfg.Root, cfg.Package), 0o755); err != nil {
		return nil, fmt.Errorf("创建目录失败: %w", err)
	}

	// 2. 按模板生成核心文件
	templateData := map[string]interface{}{
		"Package": cfg.Package,
		"Name":    cfg.Name,
		"Input":   "string",
		"Output":  "string",
	}

	// 生成四种原语文件
	files := []struct {
		template string
		output   string
	}{
		{"l1_atom", filepath.Join(cfg.Root, cfg.Package, "atom.go")},
		{"l1_port", filepath.Join(cfg.Root, cfg.Package, "port.go")},
		{"l1_adapter", filepath.Join(cfg.Root, cfg.Package, "adapter.go")},
		{"l1_composer", filepath.Join(cfg.Root, cfg.Package, "composer.go")},
	}

	for _, f := range files {
		if err := g.renderer.RenderToFile(f.template, f.output, templateData); err != nil {
			return nil, fmt.Errorf("生成 %s 失败: %w", f.output, err)
		}
		result.Files = append(result.Files, f.output)
	}

	return result, nil
}

// NewModule 创建新模块（仅生成核心代码文件）。
func (g *defaultGenerator) NewModule(cfg GenConfig) (*GenResult, error) {
	return g.InitProject(cfg)
}

// AddFeature 添加新功能（生成单独的文件）。
func (g *defaultGenerator) AddFeature(cfg GenConfig, featureName string) (*GenResult, error) {
	result := &GenResult{
		CreatedAt: time.Now(),
		Root:      cfg.Root,
		Files:     make([]string, 0),
	}

	templateData := map[string]interface{}{
		"Package": cfg.Package,
		"Name":    capitalize(featureName),
		"Input":   "string",
		"Output":  "string",
	}

	// 为新功能生成一个 Atom 文件
	output := filepath.Join(cfg.Root, cfg.Package, featureName+".go")
	if err := g.renderer.RenderToFile("l1_atom", output, templateData); err != nil {
		return nil, fmt.Errorf("生成失败: %w", err)
	}
	result.Files = append(result.Files, output)
	return result, nil
}

// capitalize 首字母大写。
func capitalize(s string) string {
	if len(s) == 0 {
		return s
	}
	return string(s[0]-32) + s[1:]
}
