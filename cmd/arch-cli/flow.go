package main

import (
	"encoding/json"
	"net/http"

	arch "low-entropy-core/go-core/arch"
)

// FlowEdge 表示两个层之间的数据流边
type FlowEdge struct {
	From       string `json:"from"`
	FromName   string `json:"from_name"`
	To         string `json:"to"`
	ToName     string `json:"to_name"`
	Weight     int    `json:"weight"`
	FileCount  int    `json:"file_count"`
	Primitives string `json:"primitives"`
}

// FlowNode 表示数据流节点
type FlowNode struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Layer       string `json:"layer"`
	FileCount   int    `json:"file_count"`
	LineCount   int    `json:"line_count"`
	SymbolCount int    `json:"symbol_count"`
	Primitive   string `json:"primitive,omitempty"`
}

// FlowResponse 数据流响应
type FlowResponse struct {
	Nodes       []FlowNode    `json:"nodes"`
	Edges       []FlowEdge    `json:"edges"`
	TotalEdges  int           `json:"total_edges"`
	TotalNodes  int           `json:"total_nodes"`
	LayerFlow   []LayerFlow   `json:"layer_flow"`
	TopPaths    []FlowPath    `json:"top_paths"`
}

// LayerFlow 层级间流量统计
type LayerFlow struct {
	FromLayer string `json:"from_layer"`
	ToLayer   string `json:"to_layer"`
	Count     int    `json:"count"`
}

// FlowPath 数据流路径
type FlowPath struct {
	Path   []string `json:"path"`
	Weight int      `json:"weight"`
}

func handleFlow(w http.ResponseWriter, r *http.Request) {
	archMu.RLock()
	defer archMu.RUnlock()

	if archData == nil {
		http.Error(w, `{"error":"no data available"}`, http.StatusServiceUnavailable)
		return
	}

	resp := buildFlowResponse(archData)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func buildFlowResponse(data *arch.ArchData) FlowResponse {
	// 1. 构建文件级节点
	nodeMap := make(map[string]FlowNode)
	for i := range data.Files {
		file := &data.Files[i]
		primType := inferPrimitiveByName(file.Name)
		if primType == "" {
			primType = inferPrimitiveByPath(file.Path)
		}
		nodeMap[file.Name] = FlowNode{
			ID:          file.Name,
			Name:        file.Name,
			Layer:       file.Layer,
			FileCount:   1,
			LineCount:   file.Lines,
			SymbolCount: len(file.Symbols),
			Primitive:   primType,
		}
	}

	// 2. 构建边
	edgeMap := make(map[string]*FlowEdge)
	layerFlowMap := make(map[string]int)
	for i := range data.Files {
		file := &data.Files[i]
		for _, dep := range file.DependsOn {
			key := file.Name + "->" + dep
			if _, ok := edgeMap[key]; !ok {
				edgeMap[key] = &FlowEdge{
					From:     file.Name,
					FromName: file.Layer,
					To:       dep,
					Weight:   1,
				}
			}
			edgeMap[key].Weight++

			// 层间流量
			depLayer := "L7"
			for j := range data.Files {
				if data.Files[j].Name == dep {
					depLayer = data.Files[j].Layer
					break
				}
			}
			layerKey := file.Layer + "->" + depLayer
			layerFlowMap[layerKey]++
		}
	}

	// 3. 转成切片
	var nodes []FlowNode
	for _, n := range nodeMap {
		nodes = append(nodes, n)
	}

	var edges []FlowEdge
	for _, e := range edgeMap {
		edges = append(edges, *e)
	}

	// 4. 构建层间流量
	var layerFlows []LayerFlow
	for k, v := range layerFlowMap {
		parts := splitLayerKey(k)
		layerFlows = append(layerFlows, LayerFlow{
			FromLayer: parts[0],
			ToLayer:   parts[1],
			Count:     v,
		})
	}

	// 5. 构建顶层路径（按权重排序）
	var topPaths []FlowPath
	for i := range data.Files {
		file := &data.Files[i]
		if len(file.DependsOn) == 0 {
			continue
		}
		path := append([]string{file.Name}, file.DependsOn...)
		weight := 0
		for _, dep := range file.DependsOn {
			for j := range data.Files {
				if data.Files[j].Name == dep {
					weight += len(data.Files[j].Symbols)
				}
			}
		}
		topPaths = append(topPaths, FlowPath{
			Path:   path[:minInt(len(path), 6)],
			Weight: weight,
		})
	}

	// 排序并限制
	sortByWeight(topPaths)
	if len(topPaths) > 20 {
		topPaths = topPaths[:20]
	}

	return FlowResponse{
		Nodes:      nodes,
		Edges:      edges,
		TotalEdges: len(edges),
		TotalNodes: len(nodes),
		LayerFlow:  layerFlows,
		TopPaths:   topPaths,
	}
}

func splitLayerKey(k string) []string {
	parts := make([]string, 2)
	for i, c := range k {
		if c == '-' && i+2 < len(k) && k[i+1] == '>' {
			parts[0] = k[:i]
			parts[1] = k[i+2:]
			return parts
		}
	}
	parts[0] = k
	parts[1] = "unknown"
	return parts
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func sortByWeight(paths []FlowPath) {
	for i := 1; i < len(paths); i++ {
		for j := 0; j < len(paths)-i; j++ {
			if paths[j].Weight < paths[j+1].Weight {
				paths[j], paths[j+1] = paths[j+1], paths[j]
			}
		}
	}
}
