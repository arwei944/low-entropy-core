// Package core — GraphIndex 内存倒排索引 (v0.7.0)
package core

import (
	"sort"
	"strings"
)

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
