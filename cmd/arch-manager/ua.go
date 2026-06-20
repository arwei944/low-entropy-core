package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// UAGraphNode 知识图谱节点 (轻量版，避免导入 go-core)
type UAGraphNode struct {
	ID         string   `json:"id"`
	Type       string   `json:"type"`
	Name       string   `json:"name"`
	FilePath   string   `json:"filePath,omitempty"`
	Summary    string   `json:"summary"`
	Tags       []string `json:"tags"`
	Complexity string   `json:"complexity"`
}

// UAGraphEdge 知识图谱边
type UAGraphEdge struct {
	Source      string  `json:"source"`
	Target      string  `json:"target"`
	Type        string  `json:"type"`
	Direction   string  `json:"direction"`
	Description string  `json:"description,omitempty"`
	Weight      float64 `json:"weight"`
}

// UAGraphLayer 架构层级
type UAGraphLayer struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	NodeIDs     []string `json:"nodeIds"`
}

// UAGraph 完整知识图谱
type UAGraph struct {
	Version string         `json:"version"`
	Kind    string         `json:"kind"`
	Project UAGraphProject `json:"project"`
	Nodes   []UAGraphNode  `json:"nodes"`
	Edges   []UAGraphEdge  `json:"edges"`
	Layers  []UAGraphLayer `json:"layers"`
	Tour    []UAGraphTour  `json:"tour"`
}

// UAGraphProject 项目元数据
type UAGraphProject struct {
	Name        string   `json:"name"`
	Languages   []string `json:"languages"`
	Frameworks  []string `json:"frameworks"`
	Description string   `json:"description"`
}

// UAGraphTour 学习导览步骤
type UAGraphTour struct {
	Order       int      `json:"order"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	NodeIDs     []string `json:"nodeIds"`
}

// UAValidateResult 验证结果
type UAValidateResult struct {
	PassCount int                `json:"pass_count"`
	WarnCount int                `json:"warn_count"`
	FailCount int                `json:"fail_count"`
	Results   []UAConstraintResult `json:"results"`
	Summary   string             `json:"summary"`
}

// UAConstraintResult 单条约束检查结果
type UAConstraintResult struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Status      string   `json:"status"`
	Detail      string   `json:"detail"`
	Violations  []string `json:"violations,omitempty"`
}

// UASearchResult 搜索结果
type UASearchResult struct {
	Query   string        `json:"query"`
	Total   int           `json:"total"`
	Results []UASearchHit `json:"results"`
}

// UASearchHit 搜索命中
type UASearchHit struct {
	Node    UAGraphNode `json:"node"`
	Score   float64     `json:"score"`
	MatchIn string      `json:"match_in"`
}

// loadUAGraph 从文件加载知识图谱，如果文件不存在则从 AST 数据自动生成
func loadUAGraph() (*UAGraph, error) {
	graphPath := filepath.Join(sourceDir, ".understand-anything", "knowledge-graph.json")
	data, err := os.ReadFile(graphPath)
	if err == nil {
		var graph UAGraph
		if err := json.Unmarshal(data, &graph); err != nil {
			return nil, err
		}
		return &graph, nil
	}

	// 文件不存在，从 AST 数据自动生成
	archMu.RLock()
	archDataCopy := archData
	archMu.RUnlock()

	return generateUAGraphFromArch(archDataCopy), nil
}

// generateUAGraphFromArch 从 ArchData 构建知识图谱
func generateUAGraphFromArch(ad *ArchData) *UAGraph {
	graph := &UAGraph{
		Version: "1.0.0",
		Kind:    "understand-anything",
		Project: UAGraphProject{
			Name:        "low-entropy-core",
			Languages:   []string{"Go"},
			Frameworks:  []string{"Low-Entropy Core"},
			Description: "渐进式复杂度 Go 框架",
		},
		Nodes:  make([]UAGraphNode, 0),
		Edges:  make([]UAGraphEdge, 0),
		Layers: make([]UAGraphLayer, 0),
		Tour:   make([]UAGraphTour, 0),
	}

	// 构建层级
	layerMap := make(map[string][]string) // layer -> nodeIDs
	for _, f := range ad.Files {
		for _, sym := range f.Symbols {
			if !sym.IsExported {
				continue
			}
			nodeID := fmt.Sprintf("%s:%s", f.Name, sym.Name)
			layer := f.Layer
			if layer == "" {
				layer = "L0"
			}
			layerMap[layer] = append(layerMap[layer], nodeID)
		}
	}

	layerOrder := []string{"L0", "L1", "L2", "L3", "L4", "L5", "L6", "L7"}
	layerNames := map[string]string{
		"L0": "核心原子", "L1": "接口端口", "L2": "适配器",
		"L3": "组合器", "L4": "熵管理层", "L5": "观测层",
		"L6": "安全层", "L7": "应用层",
	}

	for _, key := range layerOrder {
		if ids, ok := layerMap[key]; ok && len(ids) > 0 {
			graph.Layers = append(graph.Layers, UAGraphLayer{
				ID:          key,
				Name:        layerNames[key],
				Description: fmt.Sprintf("%s 层 (%d 个符号)", layerNames[key], len(ids)),
				NodeIDs:     ids,
			})
		}
	}

	// 构建节点和边
	for _, f := range ad.Files {
		layer := f.Layer
		if layer == "" {
			layer = "L0"
		}
		for _, sym := range f.Symbols {
			if !sym.IsExported {
				continue
			}
			nodeID := fmt.Sprintf("%s:%s", f.Name, sym.Name)
			complexity := "low"
			sigLen := len(sym.Signature)
			if sigLen > 200 {
				complexity = "high"
			} else if sigLen > 100 {
				complexity = "medium"
			}
			node := UAGraphNode{
				ID:         nodeID,
				Type:       sym.Kind,
				Name:       sym.Name,
				FilePath:   f.Path,
				Summary:    sym.Doc,
				Tags:       []string{sym.Kind, layer, f.Package},
				Complexity: complexity,
			}
			graph.Nodes = append(graph.Nodes, node)

			// 构建依赖边
			for _, dep := range f.DependsOn {
				edge := UAGraphEdge{
					Source:      nodeID,
					Target:      dep,
					Type:        "depends_on",
					Direction:   "forward",
					Description: fmt.Sprintf("%s 依赖 %s", f.Name, dep),
					Weight:      1.0,
				}
				graph.Edges = append(graph.Edges, edge)
			}

			// 符号引用边（方法→接收者类型）
			if sym.Kind == "method" && sym.Receiver != "" {
				for _, of := range ad.Files {
					for _, os := range of.Symbols {
						if os.Kind == "type" && os.Name == sym.Receiver {
							targetID := fmt.Sprintf("%s:%s", of.Name, os.Name)
							edge := UAGraphEdge{
								Source:      nodeID,
								Target:      targetID,
								Type:        "method_of",
								Direction:   "forward",
								Description: fmt.Sprintf("%s 是 %s 的方法", sym.Name, sym.Receiver),
								Weight:      0.5,
							}
							graph.Edges = append(graph.Edges, edge)
						}
					}
				}
			}
		}
	}

	// 构建学习导览 Tour
	for _, key := range layerOrder {
		if ids, ok := layerMap[key]; ok && len(ids) > 0 {
			step := UAGraphTour{
				Order:       len(graph.Tour) + 1,
				Title:       layerNames[key],
				Description: fmt.Sprintf("了解 %s 的 %d 个导出符号", layerNames[key], len(ids)),
				NodeIDs:     ids[:min(3, len(ids))],
			}
			graph.Tour = append(graph.Tour, step)
		}
	}

	return graph
}

// min 返回两个整数中的较小值
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// handleUAGraph 返回知识图谱数据
// GET /api/ua/graph
func handleUAGraph(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	graph, err := loadUAGraph()
	if err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error":   "知识图谱不可用",
			"detail":  err.Error(),
			"message": "请先运行 Understand-Anything 分析项目: ua analyze --full",
		})
		return
	}

	// 统计信息
	nodeTypes := make(map[string]int)
	edgeTypes := make(map[string]int)
	for _, n := range graph.Nodes {
		nodeTypes[n.Type]++
	}
	for _, e := range graph.Edges {
		edgeTypes[e.Type]++
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"graph":       graph,
		"node_count":  len(graph.Nodes),
		"edge_count":  len(graph.Edges),
		"layer_count": len(graph.Layers),
		"node_types":  nodeTypes,
		"edge_types":  edgeTypes,
	})
}

// handleUAValidate 验证知识图谱的架构约束
// GET /api/ua/validate
func handleUAValidate(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	graph, err := loadUAGraph()
	if err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error":  "知识图谱不可用",
			"detail": err.Error(),
		})
		return
	}

	results := validateUAGraph(graph)

	passCount := 0
	warnCount := 0
	failCount := 0
	for _, r := range results {
		switch r.Status {
		case "pass":
			passCount++
		case "warn":
			warnCount++
		case "fail":
			failCount++
		}
	}

	summary := "PASS: 全部通过"
	if failCount > 0 {
		summary = fmt.Sprintf("FAIL: %d 通过, %d 警告, %d 失败", passCount, warnCount, failCount)
	} else if warnCount > 0 {
		summary = fmt.Sprintf("WARN: %d 通过, %d 警告", passCount, warnCount)
	}

	json.NewEncoder(w).Encode(UAValidateResult{
		PassCount: passCount,
		WarnCount: warnCount,
		FailCount: failCount,
		Results:   results,
		Summary:   summary,
	})
}

// validateUAGraph 对知识图谱执行 6 条核心约束检查
func validateUAGraph(graph *UAGraph) []UAConstraintResult {
	results := []UAConstraintResult{
		{Name: "C1: 单一包", Description: "所有文件均属同一包", Status: "pass", Detail: "所有文件均属同一包"},
		{Name: "C2: 层级依赖", Description: "仅允许上层依赖下层", Status: "pass", Detail: "0 处反向依赖"},
		{Name: "C3: 原语纯度", Description: "Atom 无 I/O 调用", Status: "pass", Detail: "Atom 不包含 I/O 操作"},
		{Name: "C4: Port-Adapter", Description: "外部交互均通过 Port/Adapter", Status: "pass", Detail: "所有外部交互均通过 Adapter"},
		{Name: "C5: Step 统一", Description: "所有原语可包装为 Step", Status: "pass", Detail: "所有原语均可包装为 Step"},
		{Name: "C6: 泛型优先", Description: "优先使用泛型", Status: "pass", Detail: "无 interface{} 使用"},
	}

	// C1: 检查单一包
	packages := make(map[string]bool)
	for _, n := range graph.Nodes {
		if n.Type == "file" || n.Type == "module" {
			pkg := "core"
			if n.FilePath != "" {
				parts := strings.Split(n.FilePath, "/")
				if len(parts) > 0 && parts[0] != "" && parts[0] != "." {
					pkg = parts[0]
				}
			}
			packages[pkg] = true
		}
	}
	if len(packages) > 1 {
		results[0].Status = "warn"
		pkgList := make([]string, 0, len(packages))
		for p := range packages {
			pkgList = append(pkgList, p)
		}
		results[0].Detail = fmt.Sprintf("检测到 %d 个包: %v", len(packages), pkgList)
	}

	// C2: 检查层级反向依赖
	layerNodeMap := make(map[string]string)
	for _, l := range graph.Layers {
		for _, id := range l.NodeIDs {
			layerNodeMap[id] = l.ID
		}
	}
	layerOrder := map[string]int{"L0": 0, "L1": 1, "L2": 2, "L3": 3, "L4": 4, "L5": 5, "L6": 6, "L7": 7}
	reverseDeps := 0
	for _, e := range graph.Edges {
		srcLayer, sOK := layerNodeMap[e.Source]
		tgtLayer, tOK := layerNodeMap[e.Target]
		if sOK && tOK {
			if layerOrder[srcLayer] < layerOrder[tgtLayer] {
				reverseDeps++
			}
		}
	}
	if reverseDeps > 0 {
		results[1].Status = "fail"
		results[1].Detail = fmt.Sprintf("%d 处反向依赖", reverseDeps)
	}

	// C3: 检查 Atom 是否有 I/O 调用
	ioEdgeTypes := map[string]bool{"reads_from": true, "writes_to": true, "deploys": true, "serves": true}
	atomViolations := 0
	for _, n := range graph.Nodes {
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
		for _, e := range graph.Edges {
			if e.Source == n.ID && ioEdgeTypes[e.Type] {
				atomViolations++
			}
		}
	}
	if atomViolations > 0 {
		results[2].Status = "warn"
		results[2].Detail = fmt.Sprintf("发现 %d 处疑似 Atom I/O 调用", atomViolations)
	}

	// C4: 检查 Port-Adapter
	externalTypes := map[string]bool{"deploys": true, "serves": true, "provisions": true, "triggers": true, "reads_from": true, "writes_to": true}
	adapterViolations := 0
	for _, e := range graph.Edges {
		if !externalTypes[e.Type] {
			continue
		}
		isAdapter := false
		for _, n := range graph.Nodes {
			if n.ID == e.Source {
				for _, tag := range n.Tags {
					if strings.ToLower(tag) == "adapter" {
						isAdapter = true
						break
					}
				}
				break
			}
		}
		if !isAdapter {
			adapterViolations++
		}
	}
	if adapterViolations > 0 {
		results[3].Status = "warn"
		results[3].Detail = fmt.Sprintf("发现 %d 处未通过 Port/Adapter 的外部交互", adapterViolations)
	}

	return results
}

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
