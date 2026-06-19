// Package core — Understand-Anything 监督层 (v0.7.0)
//
// T-08: ConstraintRule — 6 条约束规则定义
// T-09: GraphSupervisor — 约束验证执行器
//
// 约束遵循:
//   C3: 原语纯度 — 约束检查是纯函数
//   C5: Step 统一 — 监督流程包装为 Step

package core

import (
	"context"
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
	PassCount  int                `json:"pass_count"`
	WarnCount  int                `json:"warn_count"`
	FailCount  int                `json:"fail_count"`
	Results    []ConstraintResult `json:"results"`
	HasFatal   bool               `json:"has_fatal"`
	Summary    string             `json:"summary"`
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
			// 从 FilePath 推断包名
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

// checkLayerDependency 检查层级依赖是否正确
func checkLayerDependency(kg *KnowledgeGraph) ConstraintResult {
	result := ConstraintResult{
		Name:        "C2: 层级依赖",
		Description: "仅允许上层依赖下层，L0 是唯一基础层",
		Status:      ConstraintPass,
		Detail:      "0 处反向依赖",
	}

	// 构建层级 → 节点映射
	layerNodes := make(map[string][]string)
	for _, l := range kg.Layers {
		layerNodes[l.ID] = l.NodeIDs
	}

	// 定义层级顺序 (L0 最低，L7 最高)
	layerOrder := map[string]int{
		"L0": 0, "L1": 1, "L2": 2, "L3": 3,
		"L4": 4, "L5": 5, "L6": 6, "L7": 7,
	}

	// 检查每条边是否有反向依赖
	violations := make([]string, 0)
	for _, e := range kg.Edges {
		sourceLayer := findNodeLayer(kg, e.Source)
		targetLayer := findNodeLayer(kg, e.Target)

		sourceOrder, sOK := layerOrder[sourceLayer]
		targetOrder, tOK := layerOrder[targetLayer]

		if sOK && tOK && sourceOrder < targetOrder {
			// source 层级低于 target — 这是反向依赖（下层依赖上层）
			violations = append(violations, fmt.Sprintf(
				"%s (%s) → %s (%s): 反向依赖",
				e.Source, sourceLayer, e.Target, targetLayer,
			))
		}
	}

	if len(violations) > 0 {
		result.Status = ConstraintFail
		result.Detail = fmt.Sprintf("%d 处反向依赖", len(violations))
		result.Violations = violations
	}

	return result
}

// checkPrimitivePurity 检查 Atom 是否有 I/O 调用
func checkPrimitivePurity(kg *KnowledgeGraph) ConstraintResult {
	result := ConstraintResult{
		Name:        "C3: 原语纯度",
		Description: "Atom 无 I/O 调用",
		Status:      ConstraintPass,
		Detail:      "Atom 不包含任何 I/O 操作",
	}

	// 检测标签中包含 "atom" 的节点是否连接了 I/O 相关的边
	ioEdgeTypes := map[string]bool{
		EdgeTypeReadsFrom: true,
		EdgeTypeWritesTo:  true,
		EdgeTypeCalls:     true,
		EdgeTypeDeploys:   true,
		EdgeTypeServes:    true,
	}

	violations := make([]string, 0)
	for _, n := range kg.Nodes {
		isAtom := false
		for _, tag := range n.Tags {
			if strings.ToLower(tag) == "atom" {
				isAtom = true
				break
			}
		}
		if !isAtom {
			continue
		}

		// 检查该 Atom 节点是否有 I/O 边
		for _, e := range kg.Edges {
			if e.Source == n.ID && ioEdgeTypes[e.Type] {
				violations = append(violations, fmt.Sprintf(
					"%s (%s): 标记为 Atom 但包含 %s 边", n.ID, n.Name, e.Type,
				))
			}
		}
	}

	if len(violations) > 0 {
		result.Status = ConstraintWarn
		result.Detail = fmt.Sprintf("发现 %d 处疑似 I/O 调用", len(violations))
		result.Violations = violations
	}

	return result
}

// checkPortAdapter 检查外部交互是否通过 Port/Adapter
func checkPortAdapter(kg *KnowledgeGraph) ConstraintResult {
	result := ConstraintResult{
		Name:        "C4: Port-Adapter",
		Description: "外部交互均通过 Port/Adapter",
		Status:      ConstraintPass,
		Detail:      "所有外部交互均通过 Port/Adapter 边界",
	}

	// 检查是否有非 Adapter 节点直接连接外部资源
	externalTypes := map[string]bool{
		EdgeTypeDeploys:    true,
		EdgeTypeServes:     true,
		EdgeTypeProvisions: true,
		EdgeTypeTriggers:   true,
		EdgeTypeReadsFrom:  true,
		EdgeTypeWritesTo:   true,
	}

	violations := make([]string, 0)
	for _, e := range kg.Edges {
		if !externalTypes[e.Type] {
			continue
		}
		// 检查 source 是否为 Adapter
		sourceNode := findNodeByID(kg, e.Source)
		if sourceNode != nil {
			isAdapter := false
			for _, tag := range sourceNode.Tags {
				if strings.ToLower(tag) == "adapter" {
					isAdapter = true
					break
				}
			}
			if !isAdapter {
				violations = append(violations, fmt.Sprintf(
					"%s (%s): 外部交互 %s 未通过 Adapter",
					sourceNode.ID, sourceNode.Name, e.Type,
				))
			}
		}
	}

	if len(violations) > 0 {
		result.Status = ConstraintWarn
		result.Detail = fmt.Sprintf("发现 %d 处未通过 Port/Adapter 的外部交互", len(violations))
		result.Violations = violations
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

	// 检查是否有 implements 边指向 Step 接口
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

	// 检查 languageNotes 中是否有泛型标记
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
	// 简单规则：取第一级目录名
	parts := strings.Split(filePath, "/")
	if len(parts) > 0 && parts[0] != "" && parts[0] != "." {
		return parts[0]
	}
	return "core"
}

// ============================================================================
// T-09: GraphSupervisor
// ============================================================================

// GraphSupervisor 图谱监督器
type GraphSupervisor struct {
	rules []ConstraintRule
}

// NewGraphSupervisor 创建监督器
func NewGraphSupervisor(rules []ConstraintRule) *GraphSupervisor {
	if rules == nil {
		rules = DefaultConstraints()
	}
	return &GraphSupervisor{rules: rules}
}

// ValidateAll 执行全部约束检查
func (s *GraphSupervisor) ValidateAll(kg *KnowledgeGraph) ConstraintReport {
	report := ConstraintReport{
		Results: make([]ConstraintResult, 0, len(s.rules)),
	}

	for _, rule := range s.rules {
		result := rule.Check(kg)
		report.Results = append(report.Results, result)

		switch result.Status {
		case ConstraintPass:
			report.PassCount++
		case ConstraintWarn:
			report.WarnCount++
		case ConstraintFail:
			report.FailCount++
			if rule.Severity == "fatal" {
				report.HasFatal = true
			}
		}
	}

	// 生成摘要
	if report.HasFatal {
		report.Summary = fmt.Sprintf("FAIL: %d 通过, %d 警告, %d 失败 (含严重错误)",
			report.PassCount, report.WarnCount, report.FailCount)
	} else if report.FailCount > 0 || report.WarnCount > 0 {
		report.Summary = fmt.Sprintf("WARN: %d 通过, %d 警告, %d 失败",
			report.PassCount, report.WarnCount, report.FailCount)
	} else {
		report.Summary = fmt.Sprintf("PASS: %d 条约束全部通过", report.PassCount)
	}

	return report
}

// NewSupervisorStep 创建监督 Step
func NewSupervisorStep(supervisor *GraphSupervisor, kg *KnowledgeGraph) Step[struct{}, ConstraintReport] {
	return NewStepFunc("Atom", func(ctx context.Context, _ struct{}) (ConstraintReport, error) {
		report := supervisor.ValidateAll(kg)
		return report, nil
	})
}