// Package core — Understand-Anything 迁移层 (v0.7.0)
//
// T-04: UnderstandMigrationAdapter — 迁移前/后分析
// T-05: DiffKnowledgeGraphs — 纯 Atom 函数，图谱差异对比
// T-06: NewMigrationStep — Step 包装
//
// 约束遵循:
//   C3: 原语纯度 — DiffKnowledgeGraphs 是纯函数
//   C4: Port-Adapter — 文件 I/O 在 Adapter 中
//   C5: Step 统一 — 迁移流程包装为 Step[MigrationRequest, MigrationReport]

package core

import (
	"path/filepath"
	"sort"
)

// DiffKnowledgeGraphs 对比两个 KnowledgeGraph 的差异。
// 纯 Atom 函数：相同输入保证相同输出，无副作用。
func DiffKnowledgeGraphs(before, after *KnowledgeGraph) *GraphDiff {
	if before == nil || after == nil {
		return &GraphDiff{Summary: DiffSummary{}}
	}

	diff := &GraphDiff{
		NodesAdded:    []GraphNode{},
		NodesRemoved:  []GraphNode{},
		NodesModified: []NodeDiff{},
		EdgesAdded:    []GraphEdge{},
		EdgesRemoved:  []GraphEdge{},
		EdgesModified: []EdgeDiff{},
	}

	// 构建节点索引
	beforeNodes := make(map[string]GraphNode, len(before.Nodes))
	for _, n := range before.Nodes {
		beforeNodes[n.ID] = n
	}
	afterNodes := make(map[string]GraphNode, len(after.Nodes))
	for _, n := range after.Nodes {
		afterNodes[n.ID] = n
	}

	// 检测新增和修改的节点
	for id, afterNode := range afterNodes {
		if beforeNode, ok := beforeNodes[id]; ok {
			changes := diffNode(beforeNode, afterNode)
			if len(changes) > 0 {
				diff.NodesModified = append(diff.NodesModified, NodeDiff{
					Before:  beforeNode,
					After:   afterNode,
					Changes: changes,
				})
			}
		} else {
			diff.NodesAdded = append(diff.NodesAdded, afterNode)
		}
	}

	// 检测删除的节点
	for id, beforeNode := range beforeNodes {
		if _, ok := afterNodes[id]; !ok {
			diff.NodesRemoved = append(diff.NodesRemoved, beforeNode)
		}
	}

	// 构建边索引（使用 source+target+type 作为复合键）
	edgeKey := func(e GraphEdge) string {
		return e.Source + "|" + e.Target + "|" + e.Type
	}

	beforeEdges := make(map[string]GraphEdge, len(before.Edges))
	for _, e := range before.Edges {
		beforeEdges[edgeKey(e)] = e
	}
	afterEdges := make(map[string]GraphEdge, len(after.Edges))
	for _, e := range after.Edges {
		afterEdges[edgeKey(e)] = e
	}

	// 检测新增和修改的边
	for key, afterEdge := range afterEdges {
		if beforeEdge, ok := beforeEdges[key]; ok {
			changes := diffEdge(beforeEdge, afterEdge)
			if len(changes) > 0 {
				diff.EdgesModified = append(diff.EdgesModified, EdgeDiff{
					Before:  beforeEdge,
					After:   afterEdge,
					Changes: changes,
				})
			}
		} else {
			diff.EdgesAdded = append(diff.EdgesAdded, afterEdge)
		}
	}

	// 检测删除的边
	for key, beforeEdge := range beforeEdges {
		if _, ok := afterEdges[key]; !ok {
			diff.EdgesRemoved = append(diff.EdgesRemoved, beforeEdge)
		}
	}

	// 排序以保证确定性输出
	sortNodesAdded(diff.NodesAdded)
	sortNodesRemoved(diff.NodesRemoved)
	sortEdgesAdded(diff.EdgesAdded)
	sortEdgesRemoved(diff.EdgesRemoved)

	// 摘要
	diff.Summary = DiffSummary{
		NodesAdded:    len(diff.NodesAdded),
		NodesRemoved:  len(diff.NodesRemoved),
		NodesModified: len(diff.NodesModified),
		EdgesAdded:    len(diff.EdgesAdded),
		EdgesRemoved:  len(diff.EdgesRemoved),
		EdgesModified: len(diff.EdgesModified),
	}
	diff.Summary.TotalChanges = diff.Summary.NodesAdded + diff.Summary.NodesRemoved +
		diff.Summary.NodesModified + diff.Summary.EdgesAdded +
		diff.Summary.EdgesRemoved + diff.Summary.EdgesModified

	return diff
}

// diffNode 对比两个节点，返回变更字段列表
func diffNode(before, after GraphNode) []string {
	var changes []string
	if before.Name != after.Name {
		changes = append(changes, "name")
	}
	if before.Summary != after.Summary {
		changes = append(changes, "summary")
	}
	if before.Complexity != after.Complexity {
		changes = append(changes, "complexity")
	}
	if before.FilePath != after.FilePath {
		changes = append(changes, "filePath")
	}
	if !equalStringSlices(before.Tags, after.Tags) {
		changes = append(changes, "tags")
	}
	return changes
}

// diffEdge 对比两条边，返回变更字段列表
func diffEdge(before, after GraphEdge) []string {
	var changes []string
	if before.Weight != after.Weight {
		changes = append(changes, "weight")
	}
	if before.Direction != after.Direction {
		changes = append(changes, "direction")
	}
	if before.Description != after.Description {
		changes = append(changes, "description")
	}
	return changes
}

// equalStringSlices 比较两个字符串切片
func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// 排序辅助函数
func sortNodesAdded(nodes []GraphNode) {
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].ID < nodes[j].ID })
}
func sortNodesRemoved(nodes []GraphNode) {
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].ID < nodes[j].ID })
}
func sortEdgesAdded(edges []GraphEdge) {
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].Source != edges[j].Source {
			return edges[i].Source < edges[j].Source
		}
		return edges[i].Target < edges[j].Target
	})
}
func sortEdgesRemoved(edges []GraphEdge) {
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].Source != edges[j].Source {
			return edges[i].Source < edges[j].Source
		}
		return edges[i].Target < edges[j].Target
	})
}

// ============================================================================
// 层级漂移检测
// ============================================================================

// DetectLayerDrifts 检测两个图谱之间的层级漂移。
// 纯 Atom 函数。
func DetectLayerDrifts(before, after *KnowledgeGraph) []LayerDrift {
	if before == nil || after == nil {
		return nil
	}

	beforeByLayer := nodesByLayer(before)
	afterByLayer := nodesByLayer(after)

	allLayers := make(map[string]bool)
	for l := range beforeByLayer { allLayers[l] = true }
	for l := range afterByLayer { allLayers[l] = true }

	var drifts []LayerDrift
	for layer := range allLayers {
		b := beforeByLayer[layer]
		a := afterByLayer[layer]
		delta := a - b
		severity := "info"
		if delta > 5 || delta < -5 {
			severity = "warn"
		}
		if delta > 20 || delta < -20 {
			severity = "critical"
		}
		drifts = append(drifts, LayerDrift{
			Layer:       layer,
			NodesBefore: b,
			NodesAfter:  a,
			Delta:       delta,
			Severity:    severity,
		})
	}

	sort.Slice(drifts, func(i, j int) bool {
		return drifts[i].Layer < drifts[j].Layer
	})
	return drifts
}

// nodesByLayer 按层级分组节点
func nodesByLayer(kg *KnowledgeGraph) map[string]int {
	result := make(map[string]int)
	for _, n := range kg.Nodes {
		layer := extractLayer(n)
		result[layer]++
	}
	return result
}

// extractLayer 从节点提取层级信息
func extractLayer(n GraphNode) string {
	// 优先从 FilePath 推断层级
	if n.FilePath != "" {
		dir := filepath.Dir(n.FilePath)
		if dir != "." {
			return dir
		}
	}
	return "unknown"
}
