// Package core — Understand-Anything 监督层 (v0.7.0)
//
// T-08: ConstraintRule — 复杂约束检查实现
//
// 本文件包含需要遍历图结构进行复杂检查的约束规则：
//   - C2: 层级依赖
//   - C3: 原语纯度
//   - C4: Port-Adapter

package core

import (
	"fmt"
	"strings"
)

// checkLayerDependency 检查层级依赖是否正确
func checkLayerDependency(kg *KnowledgeGraph) ConstraintResult {
	result := ConstraintResult{
		Name:        "C2: 层级依赖",
		Description: "仅允许上层依赖下层，L0 是唯一基础层",
		Status:      ConstraintPass,
		Detail:      "0 处反向依赖",
	}

	layerOrder := map[string]int{
		"L0": 0, "L1": 1, "L2": 2, "L3": 3,
		"L4": 4, "L5": 5, "L6": 6, "L7": 7,
	}

	violations := make([]string, 0)
	for _, e := range kg.Edges {
		sourceLayer := findNodeLayer(kg, e.Source)
		targetLayer := findNodeLayer(kg, e.Target)

		sourceOrder, sOK := layerOrder[sourceLayer]
		targetOrder, tOK := layerOrder[targetLayer]

		if sOK && tOK && sourceOrder < targetOrder {
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
