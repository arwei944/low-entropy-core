// Package core — ParseKnowledgeGraph 纯 Atom 函数 (v0.7.0)
package core

import (
	"encoding/json"
	"fmt"
)

// ParseKnowledgeGraph 将 JSON 字节解析为 KnowledgeGraph。
// 纯函数：相同输入保证相同输出，无副作用。
func ParseKnowledgeGraph(data []byte) (*KnowledgeGraph, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("understand: empty input")
	}

	var kg KnowledgeGraph
	if err := json.Unmarshal(data, &kg); err != nil {
		return nil, fmt.Errorf("understand: failed to parse knowledge graph: %w", err)
	}

	// 基础验证
	if kg.Version == "" {
		return nil, fmt.Errorf("understand: missing version field")
	}
	if kg.Project.Name == "" {
		return nil, fmt.Errorf("understand: missing project name")
	}

	return &kg, nil
}

// ParseKnowledgeGraphFile 从文件路径解析 KnowledgeGraph。
// 注意：此函数包含文件 I/O，属于副作用操作，应通过 Adapter 调用。
func ParseKnowledgeGraphFile(path string) (*KnowledgeGraph, error) {
	// 此函数供 Adapter 内部使用，标记为副作用边界
	return nil, fmt.Errorf("understand: use UnderstandAdapter for file I/O")
}

// ValidateKnowledgeGraph 对已解析的图谱进行轻量验证。
// 纯函数：检查节点引用完整性、边类型有效性。
func ValidateKnowledgeGraph(kg *KnowledgeGraph) []string {
	var warnings []string

	if kg == nil {
		return []string{"nil knowledge graph"}
	}

	// 构建节点 ID 集合
	nodeIDs := make(map[string]bool, len(kg.Nodes))
	for _, n := range kg.Nodes {
		if n.ID == "" {
			warnings = append(warnings, "node with empty ID")
			continue
		}
		nodeIDs[n.ID] = true
	}

	// 验证边引用的节点存在
	validEdgeTypes := make(map[string]bool)
	for _, t := range ValidEdgeTypes() {
		validEdgeTypes[t] = true
	}

	for i, e := range kg.Edges {
		if !nodeIDs[e.Source] {
			warnings = append(warnings, fmt.Sprintf("edge[%d]: source %q not found in nodes", i, e.Source))
		}
		if !nodeIDs[e.Target] {
			warnings = append(warnings, fmt.Sprintf("edge[%d]: target %q not found in nodes", i, e.Target))
		}
		if !validEdgeTypes[e.Type] {
			warnings = append(warnings, fmt.Sprintf("edge[%d]: unknown type %q", i, e.Type))
		}
	}

	// 检测孤立节点
	referenced := make(map[string]bool)
	for _, e := range kg.Edges {
		referenced[e.Source] = true
		referenced[e.Target] = true
	}
	for _, n := range kg.Nodes {
		if !referenced[n.ID] && n.Type != NodeTypeFile {
			warnings = append(warnings, fmt.Sprintf("orphan node: %s (%s)", n.ID, n.Name))
		}
	}

	return warnings
}

// CountByType 按节点类型统计数量
func CountByType(kg *KnowledgeGraph) map[string]int {
	counts := make(map[string]int)
	for _, n := range kg.Nodes {
		counts[n.Type]++
	}
	return counts
}

// CountByEdgeType 按边类型统计数量
func CountByEdgeType(kg *KnowledgeGraph) map[string]int {
	counts := make(map[string]int)
	for _, e := range kg.Edges {
		counts[e.Type]++
	}
	return counts
}
