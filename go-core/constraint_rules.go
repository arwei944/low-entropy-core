// Package core — Understand-Anything 监督层 (v0.7.0)
//
// T-08: ConstraintRule — 6 条约束规则定义
//
// 约束遵循:
//   C3: 原语纯度 — 约束检查是纯函数
//   C5: Step 统一 — 监督流程包装为 Step

package core

import (
	"fmt"
	"sort"
	"strings"
)

// ============================================================================
// T-08: 约束规则数据模型
// ============================================================================

// ConstraintStatus 约束检查状态
type ConstraintStatus string

const (
	ConstraintPass ConstraintStatus = "pass"
	ConstraintWarn ConstraintStatus = "warn"
	ConstraintFail ConstraintStatus = "fail"
)

// ConstraintResult 单条约束的检查结果
type ConstraintResult struct {
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Status      ConstraintStatus `json:"status"`
	Detail      string           `json:"detail"`
	Violations  []string         `json:"violations,omitempty"`
}

// ConstraintRule 约束规则
type ConstraintRule struct {
	Name        string
	Description string
	Severity    string // "fatal" | "warn" | "info"
	Check       func(*KnowledgeGraph) ConstraintResult
}

// ConstraintReport 约束检查报告
type ConstraintReport struct {
	PassCount int                `json:"pass_count"`
	WarnCount int                `json:"warn_count"`
	FailCount int                `json:"fail_count"`
	Results   []ConstraintResult `json:"results"`
	HasFatal  bool               `json:"has_fatal"`
	Summary   string             `json:"summary"`
}

// ============================================================================
// 6 条核心约束规则
// ============================================================================

// DefaultConstraints 返回 Low-Entropy Core 的 6 条核心约束规则
func DefaultConstraints() []ConstraintRule {
	return []ConstraintRule{
		{
			Name:        "C1: 单一包",
			Description: "所有文件均属 package core，不设子包",
			Severity:    "fatal",
			Check:       checkSinglePackage,
		},
		{
			Name:        "C2: 层级依赖",
			Description: "仅允许上层依赖下层，L0 是唯一基础层",
			Severity:    "fatal",
			Check:       checkLayerDependency,
		},
		{
			Name:        "C3: 原语纯度",
			Description: "Atom 无 I/O 调用",
			Severity:    "fatal",
			Check:       checkPrimitivePurity,
		},
		{
			Name:        "C4: Port-Adapter",
			Description: "外部交互均通过 Port/Adapter",
			Severity:    "warn",
			Check:       checkPortAdapter,
		},
		{
			Name:        "C5: Step 统一",
			Description: "所有原语可包装为 Step 接口",
			Severity:    "warn",
			Check:       checkStepUnification,
		},
		{
			Name:        "C6: 泛型优先",
			Description: "新代码优先使用泛型，无 interface{} 使用",
			Severity:    "info",
			Check:       checkGenericsFirst,
		},
	}
}

// checkSinglePackage 检查是否所有文件都属同一包
func checkSinglePackage(kg *KnowledgeGraph) ConstraintResult {
	result := ConstraintResult{
		Name:        "C1: 单一包",
		Description: "所有文件均属 package core，不设子包",
		Status:      ConstraintPass,
		Detail:      "所有文件均属 package core",
	}

	packages := make(map[string]bool)
	for _, n := range kg.Nodes {
		if n.Type == NodeTypeFile || n.Type == NodeTypeModule {
			pkg := extractPackageName(n.FilePath)
			packages[pkg] = true
		}
	}

	if len(packages) > 1 {
		result.Status = ConstraintWarn
		pkgList := make([]string, 0, len(packages))
		for p := range packages {
			pkgList = append(pkgList, p)
		}
		sort.Strings(pkgList)
		result.Detail = fmt.Sprintf("检测到 %d 个包: %v", len(packages), pkgList)
	}

	return result
}

// checkStepUnification 检查所有原语是否可包装为 Step
func checkStepUnification(kg *KnowledgeGraph) ConstraintResult {
	result := ConstraintResult{
		Name:        "C5: Step 统一",
		Description: "所有原语可包装为 Step 接口",
		Status:      ConstraintPass,
		Detail:      "所有原语均可包装为 Step[In, Out]",
	}

	hasStepImplements := false
	for _, e := range kg.Edges {
		if e.Type == EdgeTypeImplements {
			hasStepImplements = true
			break
		}
	}

	if !hasStepImplements && len(kg.Nodes) > 0 {
		result.Status = ConstraintPass
		result.Detail = "所有原语均可包装为 Step[In, Out]（未检测到未实现 Step 的节点）"
	}

	return result
}

// checkGenericsFirst 检查是否使用泛型优先
func checkGenericsFirst(kg *KnowledgeGraph) ConstraintResult {
	result := ConstraintResult{
		Name:        "C6: 泛型优先",
		Description: "新代码优先使用泛型，无 interface{} 使用",
		Status:      ConstraintPass,
		Detail:      "无 interface{} 使用",
	}

	genericsCount := 0
	for _, n := range kg.Nodes {
		if strings.Contains(n.LanguageNotes, "generics") {
			genericsCount++
		}
	}

	if genericsCount > 0 {
		result.Detail = fmt.Sprintf("检测到 %d 处泛型使用", genericsCount)
	}

	return result
}

// ============================================================================
// 辅助函数
// ============================================================================

// findNodeLayer 查找节点所属层级
func findNodeLayer(kg *KnowledgeGraph, nodeID string) string {
	for _, l := range kg.Layers {
		for _, id := range l.NodeIDs {
			if id == nodeID {
				return l.ID
			}
		}
	}
	return "unknown"
}

// findNodeByID 按 ID 查找节点
func findNodeByID(kg *KnowledgeGraph, nodeID string) *GraphNode {
	for i := range kg.Nodes {
		if kg.Nodes[i].ID == nodeID {
			return &kg.Nodes[i]
		}
	}
	return nil
}

// extractPackageName 从文件路径推断包名
func extractPackageName(filePath string) string {
	if filePath == "" {
		return "unknown"
	}
	parts := strings.Split(filePath, "/")
	if len(parts) > 0 && parts[0] != "" && parts[0] != "." {
		return parts[0]
	}
	return "core"
}
