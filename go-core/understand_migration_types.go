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
	"time"
)

// ============================================================================
// T-04: 迁移数据模型
// ============================================================================

// MigrationBaseline 迁移基线快照
type MigrationBaseline struct {
	Version    string            `json:"version"`
	Timestamp  time.Time         `json:"timestamp"`
	FileCount  int               `json:"file_count"`
	NodeCount  int               `json:"node_count"`
	EdgeCount  int               `json:"edge_count"`
	LayerStats map[string]int    `json:"layer_stats"`  // 层级 → 节点数
	NodeTypes  map[string]int    `json:"node_types"`   // 节点类型 → 数量
	EdgeTypes  map[string]int    `json:"edge_types"`   // 边类型 → 数量
	FileHashes map[string]string `json:"file_hashes"`  // 文件路径 → SHA-256
}

// MigrationRequest 迁移请求
type MigrationRequest struct {
	TargetVersion string `json:"target_version"`
	SourceVersion string `json:"source_version"`
	Description   string `json:"description"`
	DryRun        bool   `json:"dry_run"`
}

// MigrationReport 迁移报告
type MigrationReport struct {
	SourceVersion  string              `json:"source_version"`
	TargetVersion  string              `json:"target_version"`
	Timestamp      time.Time           `json:"timestamp"`
	Success        bool                `json:"success"`
	Diff           *GraphDiff          `json:"diff,omitempty"`
	LayerDrifts    []LayerDrift        `json:"layer_drifts"`
	Warnings       []string            `json:"warnings"`
	Errors         []string            `json:"errors"`
	Recommendation string              `json:"recommendation"`
}

// ============================================================================
// T-05: GraphDiff — 图谱差异对比
// ============================================================================

// GraphDiff 两个 KnowledgeGraph 之间的差异
type GraphDiff struct {
	NodesAdded    []GraphNode   `json:"nodes_added"`
	NodesRemoved  []GraphNode   `json:"nodes_removed"`
	NodesModified []NodeDiff    `json:"nodes_modified"`
	EdgesAdded    []GraphEdge   `json:"edges_added"`
	EdgesRemoved  []GraphEdge   `json:"edges_removed"`
	EdgesModified []EdgeDiff    `json:"edges_modified"`
	Summary       DiffSummary   `json:"summary"`
}

// NodeDiff 节点修改详情
type NodeDiff struct {
	Before    GraphNode `json:"before"`
	After     GraphNode `json:"after"`
	Changes   []string  `json:"changes"` // 变更字段列表
}

// EdgeDiff 边修改详情
type EdgeDiff struct {
	Before  GraphEdge `json:"before"`
	After   GraphEdge `json:"after"`
	Changes []string  `json:"changes"`
}

// DiffSummary 差异摘要
type DiffSummary struct {
	NodesAdded    int `json:"nodes_added"`
	NodesRemoved  int `json:"nodes_removed"`
	NodesModified int `json:"nodes_modified"`
	EdgesAdded    int `json:"edges_added"`
	EdgesRemoved  int `json:"edges_removed"`
	EdgesModified int `json:"edges_modified"`
	TotalChanges  int `json:"total_changes"`
}

// LayerDrift 层级漂移
type LayerDrift struct {
	Layer       string `json:"layer"`
	NodesBefore int    `json:"nodes_before"`
	NodesAfter  int    `json:"nodes_after"`
	Delta       int    `json:"delta"`
	Severity    string `json:"severity"` // "info" | "warn" | "critical"
}
