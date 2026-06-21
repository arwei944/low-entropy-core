// Package core — Understand-Anything 观测数据模型 (v0.7.0)
package core

// StructureObservation 结构观测结果
type StructureObservation struct {
	NodeCount    int            `json:"node_count"`
	EdgeCount    int            `json:"edge_count"`
	LayerCount   int            `json:"layer_count"`
	GraphDensity float64        `json:"graph_density"`
	OrphanNodes  int            `json:"orphan_nodes"`
	NodeTypes    map[string]int `json:"node_types"`
	EdgeTypes    map[string]int `json:"edge_types"`
	LayerStats   map[string]int `json:"layer_stats"`
	Complexity   ComplexityStats `json:"complexity"`
}

// ComplexityStats 复杂度统计
type ComplexityStats struct {
	Simple   int `json:"simple"`
	Moderate int `json:"moderate"`
	Complex  int `json:"complex"`
}

// ChangeObservation 变更观测结果
type ChangeObservation struct {
	SnapshotVersion string     `json:"snapshot_version"`
	CurrentVersion  string     `json:"current_version"`
	Diff            *GraphDiff `json:"diff"`
	LayerDrifts     []LayerDrift `json:"layer_drifts"`
}

// SearchQuery 搜索查询
type SearchQuery struct {
	Keyword string `json:"keyword"`
	Limit   int    `json:"limit"`
}

// SearchResult 搜索结果
type SearchResult struct {
	Query    string          `json:"query"`
	Total    int             `json:"total"`
	Results  []SearchHit     `json:"results"`
}

// SearchHit 搜索命中项
type SearchHit struct {
	Node     GraphNode `json:"node"`
	Score    float64   `json:"score"`
	MatchIn  string    `json:"match_in"` // "name" | "summary" | "tags"
}
