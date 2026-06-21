package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
)

// handleUASearch 搜索知识图谱
// GET /api/ua/search?q=keyword&limit=10
func handleUASearch(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	query := r.URL.Query().Get("q")
	if query == "" {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "缺少搜索关键词 (q 参数)",
		})
		return
	}

	graph, err := loadUAGraph()
	if err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error":  "知识图谱不可用",
			"detail": err.Error(),
		})
		return
	}

	limit := 10
	if l := r.URL.Query().Get("limit"); l != "" {
		fmt.Sscanf(l, "%d", &limit)
	}

	results := searchUAGraph(graph, query, limit)

	json.NewEncoder(w).Encode(results)
}

// searchUAGraph 在知识图谱中搜索
func searchUAGraph(graph *UAGraph, query string, limit int) UASearchResult {
	query = strings.ToLower(query)
	type hit struct {
		index int
		score float64
	}

	hits := make([]hit, 0)
	for i, n := range graph.Nodes {
		score := 0.0
		matchIn := ""

		if strings.Contains(strings.ToLower(n.Name), query) {
			score += 3.0
			matchIn = "name"
		}
		for _, tag := range n.Tags {
			if strings.Contains(strings.ToLower(tag), query) {
				score += 2.0
				if matchIn == "" {
					matchIn = "tags"
				}
			}
		}
		if strings.Contains(strings.ToLower(n.Summary), query) {
			score += 1.0
			if matchIn == "" {
				matchIn = "summary"
			}
		}

		if score > 0 {
			hits = append(hits, hit{index: i, score: score})
		}
	}

	sort.Slice(hits, func(i, j int) bool {
		return hits[i].score > hits[j].score
	})

	if limit > len(hits) {
		limit = len(hits)
	}

	result := UASearchResult{
		Query:   query,
		Total:   len(hits),
		Results: make([]UASearchHit, 0, limit),
	}

	for i := 0; i < limit; i++ {
		h := hits[i]
		node := graph.Nodes[h.index]
		matchIn := "name"
		if strings.Contains(strings.ToLower(node.Name), query) {
			matchIn = "name"
		} else {
			for _, tag := range node.Tags {
				if strings.Contains(strings.ToLower(tag), query) {
					matchIn = "tags"
					break
				}
			}
			if matchIn == "name" {
				matchIn = "summary"
			}
		}

		result.Results = append(result.Results, UASearchHit{
			Node:    node,
			Score:   h.score,
			MatchIn: matchIn,
		})
	}

	return result
}
