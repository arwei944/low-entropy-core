//go:build lecore_tier2 || lecore_tier3 || lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

// Package core — 配置解析 + 构建器 + 热重载 (v4.0)
//
// 合并自: config.go + config_builder.go + config_hotreload.go
//
// 包含:
//   - PipelineConfig / StepConfig: 配置类型定义
//   - ParseConfig / ValidateConfig / ParseAndValidateConfig: 配置解析与校验
//   - AdapterResolver / MapAdapterResolver: 适配器解析
//   - PipelineBuilder: 从配置构建 Pipeline
//   - HotReload: 配置热重载 (SHA256 轮询检测)
package core

// AllowedStepTypes defines the valid step type values for pipeline configuration.
var AllowedStepTypes = []string{"atom", "port", "adapter", "composer"}

// PipelineConfig defines the configuration for a pipeline.
type PipelineConfig struct {
	ID    string       `json:"id"`
	Name  string       `json:"name"`
	Steps []StepConfig `json:"steps"`
}

// StepConfig defines the configuration for a single step in a pipeline.
type StepConfig struct {
	Type   string         `json:"type"`
	Name   string         `json:"name"`
	Params map[string]any `json:"params"`
}
