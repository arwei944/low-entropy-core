// Package core — Understand-Anything 观测层 (v0.7.0)
//
// T-11: GraphObserver — 结构观测 + 变更观测 + 语义搜索
// T-12: GraphIndex — 内存倒排索引
//
// 约束遵循:
//   C3: 原语纯度 — 搜索和统计是纯函数
//   C5: Step 统一 — 观测流程包装为 Step

package core

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

// ============================================================================
// 观测数据模型
// ============================================================================

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

// ============================================================================
// T-11: GraphObserver
// ============================================================================

// GraphObserver 图谱观测器
type GraphObserver struct {
	kg    *KnowledgeGraph
	index *GraphIndex
}

// NewGraphObserver 创建观测器
func NewGraphObserver(kg *KnowledgeGraph) *GraphObserver {
	o := &GraphObserver{kg: kg}
	if kg != nil {
		o.index = NewGraphIndex(kg)
	}
	return o
}

// SetGraph 更新图谱
func (o *GraphObserver) SetGraph(kg *KnowledgeGraph) {
	o.kg = kg
	o.index = NewGraphIndex(kg)
}

// ObserveStructure 观测结构信息
func (o *GraphObserver) ObserveStructure() *StructureObservation {
	if o.kg == nil {
		return &StructureObservation{}
	}

	obs := &StructureObservation{
		NodeCount:  len(o.kg.Nodes),
		EdgeCount:  len(o.kg.Edges),
		LayerCount: len(o.kg.Layers),
		NodeTypes:  CountByType(o.kg),
		EdgeTypes:  CountByEdgeType(o.kg),
		LayerStats: make(map[string]int),
	}

	// 图密度 = 2 * |E| / (|V| * (|V| - 1))
	if obs.NodeCount > 1 {
		obs.GraphDensity = float64(2*obs.EdgeCount) / float64(obs.NodeCount*(obs.NodeCount-1))
	}

	// 层级统计
	for _, n := range o.kg.Nodes {
		layer := extractLayer(n)
		obs.LayerStats[layer]++
	}

	// 复杂性统计
	for _, n := range o.kg.Nodes {
		switch n.Complexity {
		case "simple":
			obs.Complexity.Simple++
		case "moderate":
			obs.Complexity.Moderate++
		case "complex":
			obs.Complexity.Complex++
		}
	}

	// 孤立节点检测
	referenced := make(map[string]bool)
	for _, e := range o.kg.Edges {
		referenced[e.Source] = true
		referenced[e.Target] = true
	}
	for _, n := range o.kg.Nodes {
		if !referenced[n.ID] && n.Type != NodeTypeFile {
			obs.OrphanNodes++
		}
	}

	return obs
}

// ObserveChanges 观测变更（对比当前图谱与历史快照）
func (o *GraphObserver) ObserveChanges(previous *KnowledgeGraph) *ChangeObservation {
	if o.kg == nil || previous == nil {
		return &ChangeObservation{}
	}

	diff := DiffKnowledgeGraphs(previous, o.kg)
	drifts := DetectLayerDrifts(previous, o.kg)

	return &ChangeObservation{
		SnapshotVersion: previous.Project.GitCommitHash,
		CurrentVersion:  o.kg.Project.GitCommitHash,
		Diff:            diff,
		LayerDrifts:     drifts,
	}
}

// SearchGraph 语义搜索
func (o *GraphObserver) SearchGraph(query SearchQuery) *SearchResult {
	if o.index == nil {
		return &SearchResult{Query: query.Keyword}
	}
	return o.index.Search(query)
}

// ============================================================================
// T-12: GraphIndex — 内存倒排索引
// ============================================================================

// GraphIndex 图谱倒排索引
type GraphIndex struct {
	nodes []GraphNode
	// 倒排索引: token → node indices
	byName    map[string][]int
	byTag     map[string][]int
	bySummary map[string][]int
}

// NewGraphIndex 构建索引
func NewGraphIndex(kg *KnowledgeGraph) *GraphIndex {
	idx := &GraphIndex{
		nodes:     kg.Nodes,
		byName:    make(map[string][]int),
		byTag:     make(map[string][]int),
		bySummary: make(map[string][]int),
	}

	for i, n := range kg.Nodes {
		// 索引 name
		tokens := tokenize(n.Name)
		for _, t := range tokens {
			idx.byName[t] = append(idx.byName[t], i)
		}

		// 索引 tags
		for _, tag := range n.Tags {
			for _, t := range tokenize(tag) {
				idx.byTag[t] = append(idx.byTag[t], i)
			}
		}

		// 索引 summary
		for _, t := range tokenize(n.Summary) {
			idx.bySummary[t] = append(idx.bySummary[t], i)
		}
	}

	return idx
}

// Search 执行搜索
func (idx *GraphIndex) Search(query SearchQuery) *SearchResult {
	if query.Limit <= 0 {
		query.Limit = 10
	}

	keyword := strings.ToLower(query.Keyword)
	tokens := tokenize(keyword)

	// 评分: name 匹配权重 3, tag 2, summary 1
	scores := make(map[int]float64)

	for _, token := range tokens {
		// name 匹配
		for _, i := range idx.byName[token] {
			scores[i] += 3.0
		}
		// tag 匹配
		for _, i := range idx.byTag[token] {
			scores[i] += 2.0
		}
		// summary 匹配
		for _, i := range idx.bySummary[token] {
			scores[i] += 1.0
		}
	}

	// 排序
	type scoredNode struct {
		index int
		score float64
	}
	var ranked []scoredNode
	for i, s := range scores {
		ranked = append(ranked, scoredNode{index: i, score: s})
	}
	sort.Slice(ranked, func(i, j int) bool {
		return ranked[i].score > ranked[j].score
	})

	// 截取 Top N
	limit := query.Limit
	if len(ranked) < limit {
		limit = len(ranked)
	}

	result := &SearchResult{
		Query:   query.Keyword,
		Total:   len(ranked),
		Results: make([]SearchHit, 0, limit),
	}

	for i := 0; i < limit; i++ {
		r := ranked[i]
		node := idx.nodes[r.index]

		// 判断匹配位置
		matchIn := "name"
		nodeName := strings.ToLower(node.Name)
		if strings.Contains(nodeName, keyword) {
			matchIn = "name"
		} else if containsInTags(node.Tags, keyword) {
			matchIn = "tags"
		} else {
			matchIn = "summary"
		}

		result.Results = append(result.Results, SearchHit{
			Node:    node,
			Score:   r.score,
			MatchIn: matchIn,
		})
	}

	return result
}

// tokenize 分词：转小写，按非字母数字分割
func tokenize(s string) []string {
	s = strings.ToLower(s)
	var tokens []string
	seen := make(map[string]bool)

	// 按空格和常见分隔符分割
	words := strings.FieldsFunc(s, func(r rune) bool {
		return !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || (r >= 0x4e00 && r <= 0x9fff))
	})

	for _, w := range words {
		// 跳过太短的词
		if len(w) < 2 {
			continue
		}
		if !seen[w] {
			tokens = append(tokens, w)
			seen[w] = true
		}
		// 也加入 n-gram 子串（用于中文单字匹配）
		for i := 0; i < len(w)-1; i++ {
			sub := w[i : i+2]
			if !seen[sub] {
				tokens = append(tokens, sub)
				seen[sub] = true
			}
		}
	}

	return tokens
}

// containsInTags 检查 tags 中是否包含关键词
func containsInTags(tags []string, keyword string) bool {
	for _, t := range tags {
		if strings.Contains(strings.ToLower(t), keyword) {
			return true
		}
	}
	return false
}

// ============================================================================
// 观测 Step 包装
// ============================================================================

// StructureObservationInput 结构观测输入
type StructureObservationInput struct{}

// NewObserverStructureStep 创建结构观测 Step
func NewObserverStructureStep(observer *GraphObserver) Step[StructureObservationInput, *StructureObservation] {
	return NewStepFunc("Atom", func(ctx context.Context, _ StructureObservationInput) (*StructureObservation, error) {
		obs := observer.ObserveStructure()
		if obs == nil {
			return nil, fmt.Errorf("observe: no graph data")
		}
		return obs, nil
	})
}

// NewObserverSearchStep 创建搜索 Step
func NewObserverSearchStep(observer *GraphObserver) Step[SearchQuery, *SearchResult] {
	return NewStepFunc("Atom", func(ctx context.Context, query SearchQuery) (*SearchResult, error) {
		result := observer.SearchGraph(query)
		if result == nil {
			return nil, fmt.Errorf("search: no index")
		}
		return result, nil
	})
}