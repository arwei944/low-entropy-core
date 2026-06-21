// Package core — Understand-Anything 集成测试 (v0.7.0)
//
// T-07: 迁移层单元测试
// T-10: 监督层单元测试
// T-13: 观测层单元测试
// T-17: 全链路集成测试
// T-18: 性能基准测试

package core

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// ============================================================================
// 测试辅助函数
// ============================================================================

// makeTestGraph 创建用于测试的 KnowledgeGraph
func makeTestGraph(nodeCount int) *KnowledgeGraph {
	nodes := make([]GraphNode, 0, nodeCount)
	for i := 0; i < nodeCount; i++ {
		nodeTypes := []string{NodeTypeFile, NodeTypeFunction, NodeTypeClass, NodeTypeModule, NodeTypeService}
		complexities := []string{"simple", "moderate", "complex"}
		nodes = append(nodes, GraphNode{
			ID:         fmt.Sprintf("node-%d", i),
			Type:       nodeTypes[i%len(nodeTypes)],
			Name:       fmt.Sprintf("Node%d", i),
			FilePath:   fmt.Sprintf("layer/%s/node%d.go", nodeTypes[i%len(nodeTypes)], i),
			Summary:    fmt.Sprintf("Summary for node %d", i),
			Tags:       []string{nodeTypes[i%len(nodeTypes)], fmt.Sprintf("tag-%d", i%5)},
			Complexity: complexities[i%len(complexities)],
		})
	}

	edges := make([]GraphEdge, 0, nodeCount*2)
	for i := 0; i < nodeCount*2 && i+1 < nodeCount; i++ {
		edgeTypes := []string{EdgeTypeCalls, EdgeTypeDependsOn, EdgeTypeContains, EdgeTypeImports, EdgeTypeRelated}
		edges = append(edges, GraphEdge{
			Source:      fmt.Sprintf("node-%d", i%nodeCount),
			Target:      fmt.Sprintf("node-%d", (i+1)%nodeCount),
			Type:        edgeTypes[i%len(edgeTypes)],
			Direction:   "forward",
			Description: fmt.Sprintf("Edge %d", i),
			Weight:      0.5,
		})
	}

	layers := []GraphLayer{
		{ID: "L0", Name: "Base", NodeIDs: make([]string, 0)},
		{ID: "L1", Name: "Primitives", NodeIDs: make([]string, 0)},
		{ID: "L4", Name: "Supervisor", NodeIDs: make([]string, 0)},
	}
	for i := range nodes {
		layers[i%len(layers)].NodeIDs = append(layers[i%len(layers)].NodeIDs, nodes[i].ID)
	}

	return &KnowledgeGraph{
		Version: "1.0.0",
		Kind:    "codebase",
		Project: ProjectMeta{
			Name:        "test-project",
			Languages:   []string{"go"},
			Description: "Test project",
			AnalyzedAt:  time.Now().Format(time.RFC3339),
		},
		Nodes:  nodes,
		Edges:  edges,
		Layers: layers,
	}
}

// makeAtomGraph 创建标记了 Atom 标签的测试图谱
func makeAtomGraph() *KnowledgeGraph {
	kg := makeTestGraph(10)
	// 标记部分节点为 Atom
	kg.Nodes[0].Tags = append(kg.Nodes[0].Tags, "atom")
	kg.Nodes[1].Tags = append(kg.Nodes[1].Tags, "atom")
	kg.Nodes[2].Tags = append(kg.Nodes[2].Tags, "adapter")
	return kg
}

// ============================================================================
// T-07: 迁移层单元测试
// ============================================================================

// TestParseKnowledgeGraph 测试 JSON 解析
func TestParseKnowledgeGraph(t *testing.T) {
	kg := makeTestGraph(5)
	data, err := json.Marshal(kg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	parsed, err := ParseKnowledgeGraph(data)
	if err != nil {
		t.Fatalf("ParseKnowledgeGraph: %v", err)
	}

	if parsed.Version != kg.Version {
		t.Errorf("version: got %q, want %q", parsed.Version, kg.Version)
	}
	if parsed.Project.Name != kg.Project.Name {
		t.Errorf("project name: got %q, want %q", parsed.Project.Name, kg.Project.Name)
	}
	if len(parsed.Nodes) != len(kg.Nodes) {
		t.Errorf("nodes: got %d, want %d", len(parsed.Nodes), len(kg.Nodes))
	}
	if len(parsed.Edges) != len(kg.Edges) {
		t.Errorf("edges: got %d, want %d", len(parsed.Edges), len(kg.Edges))
	}
}

// TestParseKnowledgeGraph_Empty 测试空输入
func TestParseKnowledgeGraph_Empty(t *testing.T) {
	_, err := ParseKnowledgeGraph([]byte{})
	if err == nil {
		t.Error("expected error for empty input")
	}
}

// TestParseKnowledgeGraph_Invalid 测试无效 JSON
func TestParseKnowledgeGraph_Invalid(t *testing.T) {
	_, err := ParseKnowledgeGraph([]byte("{invalid"))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

// TestParseKnowledgeGraph_MissingVersion 测试缺少版本
func TestParseKnowledgeGraph_MissingVersion(t *testing.T) {
	_, err := ParseKnowledgeGraph([]byte(`{"project": {"name": "test"}}`))
	if err == nil {
		t.Error("expected error for missing version")
	}
}

// TestValidateKnowledgeGraph 测试图谱验证
func TestValidateKnowledgeGraph(t *testing.T) {
	kg := makeTestGraph(5)
	warnings := ValidateKnowledgeGraph(kg)
	// 每个节点都应该被至少一条边引用，所以不应该有孤立节点
	if len(warnings) > 0 {
		for _, w := range warnings {
			t.Logf("warning: %s", w)
		}
	}
}

// TestValidateKnowledgeGraph_Nil 测试 nil 图谱
func TestValidateKnowledgeGraph_Nil(t *testing.T) {
	warnings := ValidateKnowledgeGraph(nil)
	if len(warnings) == 0 {
		t.Error("expected warnings for nil graph")
	}
}

// TestValidateKnowledgeGraph_BadEdge 测试无效边引用
func TestValidateKnowledgeGraph_BadEdge(t *testing.T) {
	kg := makeTestGraph(3)
	kg.Edges = append(kg.Edges, GraphEdge{
		Source: "nonexistent-1",
		Target: "nonexistent-2",
		Type:   EdgeTypeCalls,
	})
	warnings := ValidateKnowledgeGraph(kg)
	found := false
	for _, w := range warnings {
		if contains(w, "not found in nodes") {
			found = true
		}
	}
	if !found {
		t.Error("expected warning about nonexistent node reference")
	}
}

// TestValidateKnowledgeGraph_BadEdgeType 测试无效边类型
func TestValidateKnowledgeGraph_BadEdgeType(t *testing.T) {
	kg := makeTestGraph(3)
	kg.Edges = append(kg.Edges, GraphEdge{
		Source: kg.Nodes[0].ID,
		Target: kg.Nodes[1].ID,
		Type:   "invalid_edge_type",
	})
	warnings := ValidateKnowledgeGraph(kg)
	found := false
	for _, w := range warnings {
		if contains(w, "unknown type") {
			found = true
		}
	}
	if !found {
		t.Error("expected warning about unknown edge type")
	}
}

// TestCountByType 测试节点类型统计
func TestCountByType(t *testing.T) {
	kg := makeTestGraph(10)
	counts := CountByType(kg)
	total := 0
	for _, c := range counts {
		total += c
	}
	if total != len(kg.Nodes) {
		t.Errorf("total count: got %d, want %d", total, len(kg.Nodes))
	}
}

// TestCountByEdgeType 测试边类型统计
func TestCountByEdgeType(t *testing.T) {
	kg := makeTestGraph(5)
	counts := CountByEdgeType(kg)
	total := 0
	for _, c := range counts {
		total += c
	}
	if total != len(kg.Edges) {
		t.Errorf("total edge count: got %d, want %d", total, len(kg.Edges))
	}
}

// TestDiffKnowledgeGraphs 测试图谱差异对比
func TestDiffKnowledgeGraphs(t *testing.T) {
	before := makeTestGraph(5)
	after := makeTestGraph(5)

	// 修改一个节点（保留在 after 中）
	after.Nodes[1].Name = "ModifiedNode"
	after.Nodes[1].Complexity = "complex"

	// 添加一个节点
	after.Nodes = append(after.Nodes, GraphNode{
		ID:   "node-new",
		Type: NodeTypeFunction,
		Name: "NewNode",
	})

	// 删除一个节点（移除 node-0）
	after.Nodes = append(after.Nodes[:0], after.Nodes[1:]...)

	diff := DiffKnowledgeGraphs(before, after)

	if diff.Summary.NodesAdded == 0 {
		t.Error("expected nodes_added > 0")
	}
	if diff.Summary.NodesRemoved == 0 {
		t.Error("expected nodes_removed > 0")
	}
	if diff.Summary.NodesModified == 0 {
		t.Error("expected nodes_modified > 0")
	}
	if diff.Summary.TotalChanges == 0 {
		t.Error("expected total_changes > 0")
	}
}

// TestDiffKnowledgeGraphs_Nil 测试 nil 输入
func TestDiffKnowledgeGraphs_Nil(t *testing.T) {
	diff := DiffKnowledgeGraphs(nil, nil)
	if diff.Summary.TotalChanges != 0 {
		t.Error("expected 0 changes for nil inputs")
	}

	kg := makeTestGraph(3)
	diff = DiffKnowledgeGraphs(nil, kg)
	if diff.Summary.TotalChanges != 0 {
		t.Error("expected 0 changes for nil before")
	}
}

// TestDiffKnowledgeGraphs_Deterministic 测试确定性输出
func TestDiffKnowledgeGraphs_Deterministic(t *testing.T) {
	before := makeTestGraph(5)
	after := makeTestGraph(5)
	after.Nodes[0].Name = "Changed"

	diff1 := DiffKnowledgeGraphs(before, after)
	diff2 := DiffKnowledgeGraphs(before, after)

	if diff1.Summary != diff2.Summary {
		t.Error("DiffKnowledgeGraphs should be deterministic")
	}
}

// TestDetectLayerDrifts 测试层级漂移检测
func TestDetectLayerDrifts(t *testing.T) {
	before := makeTestGraph(10)
	after := makeTestGraph(10)

	// 修改层级分布
	after.Nodes = append(after.Nodes, GraphNode{
		ID:       "node-extra",
		Type:     NodeTypeFunction,
		Name:     "ExtraNode",
		FilePath: "layer/service/extra.go",
	})

	drifts := DetectLayerDrifts(before, after)
	if len(drifts) == 0 {
		t.Error("expected layer drifts")
	}
}

// ============================================================================
// T-10: 监督层单元测试
// ============================================================================

// TestDefaultConstraints 测试默认约束规则
func TestDefaultConstraints(t *testing.T) {
	rules := DefaultConstraints()
	if len(rules) != 6 {
		t.Errorf("expected 6 default constraints, got %d", len(rules))
	}

	expectedNames := []string{
		"C1: 单一包", "C2: 层级依赖", "C3: 原语纯度",
		"C4: Port-Adapter", "C5: Step 统一", "C6: 泛型优先",
	}
	for i, rule := range rules {
		if rule.Name != expectedNames[i] {
			t.Errorf("rule[%d]: got %q, want %q", i, rule.Name, expectedNames[i])
		}
		if rule.Check == nil {
			t.Errorf("rule[%d]: check function is nil", i)
		}
	}
}

// TestSupervisor_ValidateAll 测试全量约束验证
func TestSupervisor_ValidateAll(t *testing.T) {
	kg := makeTestGraph(10)
	supervisor := NewGraphSupervisor(nil)
	report := supervisor.ValidateAll(kg)

	if len(report.Results) != 6 {
		t.Errorf("expected 6 results, got %d", len(report.Results))
	}
	if report.PassCount+report.WarnCount+report.FailCount != 6 {
		t.Error("pass + warn + fail should equal total results")
	}
	if report.Summary == "" {
		t.Error("summary should not be empty")
	}
}

// TestSupervisor_ValidateAll_Empty 测试空图谱
func TestSupervisor_ValidateAll_Empty(t *testing.T) {
	kg := &KnowledgeGraph{
		Version: "1.0",
		Project: ProjectMeta{Name: "empty"},
		Nodes:   []GraphNode{},
		Edges:   []GraphEdge{},
		Layers:  []GraphLayer{},
	}
	supervisor := NewGraphSupervisor(nil)
	report := supervisor.ValidateAll(kg)

	if len(report.Results) != 6 {
		t.Errorf("expected 6 results, got %d", len(report.Results))
	}
	// 空图谱应该全部通过
	if report.PassCount != 6 {
		t.Errorf("expected 6 passes for empty graph, got %d", report.PassCount)
	}
}

// TestSupervisor_C1_SinglePackage 测试单一包约束
func TestSupervisor_C1_SinglePackage(t *testing.T) {
	kg := makeTestGraph(5)
	// 模拟多包场景
	kg.Nodes[0].FilePath = "pkg1/file.go"
	kg.Nodes[1].FilePath = "pkg2/file.go"
	kg.Nodes[0].Type = NodeTypeFile
	kg.Nodes[1].Type = NodeTypeFile

	rules := DefaultConstraints()
	result := rules[0].Check(kg)

	if result.Status != ConstraintWarn {
		t.Errorf("expected warn for multi-package, got %s", result.Status)
	}
}

// TestSupervisor_C2_LayerDependency 测试层级依赖约束
func TestSupervisor_C2_LayerDependency(t *testing.T) {
	kg := makeTestGraph(5)
	// 添加反向依赖边：L0 → L4
	kg.Edges = append(kg.Edges, GraphEdge{
		Source: kg.Layers[0].NodeIDs[0], // L0
		Target: kg.Layers[2].NodeIDs[0], // L4
		Type:   EdgeTypeCalls,
	})

	rules := DefaultConstraints()
	result := rules[1].Check(kg)

	if result.Status != ConstraintFail {
		t.Errorf("expected fail for reverse dependency, got %s", result.Status)
	}
}

// TestSupervisor_C3_PrimitivePurity 测试原语纯度约束
func TestSupervisor_C3_PrimitivePurity(t *testing.T) {
	kg := makeAtomGraph()
	// 给 Atom 节点添加 I/O 边
	kg.Edges = append(kg.Edges, GraphEdge{
		Source: kg.Nodes[0].ID, // 标记为 atom
		Target: kg.Nodes[3].ID,
		Type:   EdgeTypeReadsFrom, // I/O 操作
	})
	kg.Edges = append(kg.Edges, GraphEdge{
		Source: kg.Nodes[1].ID, // 标记为 atom
		Target: kg.Nodes[4].ID,
		Type:   EdgeTypeWritesTo,
	})

	rules := DefaultConstraints()
	result := rules[2].Check(kg)

	if result.Status != ConstraintWarn {
		t.Errorf("expected warn for atom with I/O, got %s", result.Status)
	}
}

// TestSupervisor_C4_PortAdapter 测试 Port-Adapter 约束
func TestSupervisor_C4_PortAdapter(t *testing.T) {
	kg := makeTestGraph(5)
	// 非 Adapter 节点直接调用外部资源
	kg.Nodes[3].Tags = []string{"function"} // 非 adapter
	kg.Edges = append(kg.Edges, GraphEdge{
		Source: kg.Nodes[3].ID,
		Target: kg.Nodes[4].ID,
		Type:   EdgeTypeDeploys,
	})

	rules := DefaultConstraints()
	result := rules[3].Check(kg)

	if result.Status != ConstraintWarn {
		t.Errorf("expected warn for non-adapter external interaction, got %s", result.Status)
	}
}

// TestSupervisor_C5_StepUnification 测试 Step 统一约束
func TestSupervisor_C5_StepUnification(t *testing.T) {
	kg := makeTestGraph(5)
	rules := DefaultConstraints()
	result := rules[4].Check(kg)

	if result.Status != ConstraintPass {
		t.Errorf("expected pass for step unification, got %s", result.Status)
	}
}

// TestSupervisor_C6_GenericsFirst 测试泛型优先约束
func TestSupervisor_C6_GenericsFirst(t *testing.T) {
	kg := makeTestGraph(5)
	kg.Nodes[0].LanguageNotes = "generics: uses type parameters"
	kg.Nodes[1].LanguageNotes = "generics: Step[In, Out]"

	rules := DefaultConstraints()
	result := rules[5].Check(kg)

	if result.Status != ConstraintPass {
		t.Errorf("expected pass, got %s", result.Status)
	}
}

// TestNewSupervisorStep 测试监督 Step 包装
func TestNewSupervisorStep(t *testing.T) {
	kg := makeTestGraph(5)
	supervisor := NewGraphSupervisor(nil)
	step := NewSupervisorStep(supervisor, kg)

	ctx := context.Background()
	report, err := step.Execute(ctx, struct{}{})
	if err != nil {
		t.Fatalf("supervisor step: %v", err)
	}
	if report.PassCount == 0 {
		t.Error("expected some passes")
	}
}

// ============================================================================
// T-13: 观测层单元测试
// ============================================================================

// TestGraphObserver_ObserveStructure 测试结构观测
func TestGraphObserver_ObserveStructure(t *testing.T) {
	kg := makeTestGraph(10)
	observer := NewGraphObserver(kg)
	obs := observer.ObserveStructure()

	if obs.NodeCount != len(kg.Nodes) {
		t.Errorf("node_count: got %d, want %d", obs.NodeCount, len(kg.Nodes))
	}
	if obs.EdgeCount != len(kg.Edges) {
		t.Errorf("edge_count: got %d, want %d", obs.EdgeCount, len(kg.Edges))
	}
	if obs.LayerCount != len(kg.Layers) {
		t.Errorf("layer_count: got %d, want %d", obs.LayerCount, len(kg.Layers))
	}
	if obs.GraphDensity <= 0 {
		t.Error("graph_density should be > 0")
	}
	if obs.Complexity.Simple+obs.Complexity.Moderate+obs.Complexity.Complex != len(kg.Nodes) {
		t.Error("complexity stats should sum to node count")
	}
}

// TestGraphObserver_Nil 测试 nil 图谱
func TestGraphObserver_Nil(t *testing.T) {
	observer := NewGraphObserver(nil)
	obs := observer.ObserveStructure()
	if obs.NodeCount != 0 {
		t.Error("expected 0 nodes for nil graph")
	}
}

// TestGraphObserver_ObserveChanges 测试变更观测
func TestGraphObserver_ObserveChanges(t *testing.T) {
	before := makeTestGraph(5)
	after := makeTestGraph(5)
	after.Nodes[0].Name = "Modified"
	after.Nodes = append(after.Nodes, GraphNode{
		ID:   "node-new",
		Type: NodeTypeFunction,
		Name: "NewNode",
	})

	observer := NewGraphObserver(after)
	change := observer.ObserveChanges(before)

	if change.Diff == nil {
		t.Error("expected diff in change observation")
	}
	if change.Diff.Summary.NodesAdded == 0 {
		t.Error("expected nodes added")
	}
}

// TestGraphObserver_ObserveChanges_Nil 测试 nil 变更观测
func TestGraphObserver_ObserveChanges_Nil(t *testing.T) {
	observer := NewGraphObserver(nil)
	change := observer.ObserveChanges(nil)
	if change.Diff != nil {
		t.Error("expected nil diff for nil graphs")
	}
}

// TestGraphObserver_SearchGraph 测试语义搜索
func TestGraphObserver_SearchGraph(t *testing.T) {
	kg := makeTestGraph(20)
	// 确保有可搜索的节点
	kg.Nodes[5].Name = "SearchTarget"
	kg.Nodes[5].Summary = "This is a very specific search target for testing"
	kg.Nodes[5].Tags = []string{"searchable", "test"}

	observer := NewGraphObserver(kg)

	result := observer.SearchGraph(SearchQuery{Keyword: "SearchTarget", Limit: 5})
	if result.Total == 0 {
		t.Error("expected search results for 'SearchTarget'")
	}
	if len(result.Results) == 0 {
		t.Error("expected at least 1 result")
	}
	if result.Results[0].Score <= 0 {
		t.Error("expected positive score")
	}
}

// TestGraphObserver_SearchGraph_TagMatch 测试标签搜索
func TestGraphObserver_SearchGraph_TagMatch(t *testing.T) {
	kg := makeTestGraph(10)
	kg.Nodes[3].Tags = []string{"unique-tag-for-testing"}
	observer := NewGraphObserver(kg)

	result := observer.SearchGraph(SearchQuery{Keyword: "unique-tag-for-testing", Limit: 5})
	if result.Total == 0 {
		t.Error("expected search results for tag match")
	}
}

// TestGraphObserver_SearchGraph_NoMatch 测试无匹配
func TestGraphObserver_SearchGraph_NoMatch(t *testing.T) {
	kg := makeTestGraph(5)
	observer := NewGraphObserver(kg)

	// 使用纯数字关键词，不会匹配任何 name/tag/summary 中的 token
	result := observer.SearchGraph(SearchQuery{Keyword: "99999999", Limit: 5})
	if result.Total != 0 {
		t.Errorf("expected 0 results, got %d", result.Total)
	}
}

// TestGraphObserver_SetGraph 测试更新图谱
func TestGraphObserver_SetGraph(t *testing.T) {
	kg1 := makeTestGraph(5)
	observer := NewGraphObserver(kg1)

	kg2 := makeTestGraph(10)
	observer.SetGraph(kg2)

	obs := observer.ObserveStructure()
	if obs.NodeCount != 10 {
		t.Errorf("expected 10 nodes after SetGraph, got %d", obs.NodeCount)
	}
}

// TestGraphIndex_Search 测试倒排索引搜索
func TestGraphIndex_Search(t *testing.T) {
	kg := makeTestGraph(20)
	index := NewGraphIndex(kg)

	result := index.Search(SearchQuery{Keyword: "Node", Limit: 10})
	if result.Total == 0 {
		t.Error("expected search results")
	}
	if len(result.Results) > 10 {
		t.Errorf("expected at most 10 results, got %d", len(result.Results))
	}
}

// TestGraphIndex_DefaultLimit 测试默认 limit
func TestGraphIndex_DefaultLimit(t *testing.T) {
	kg := makeTestGraph(20)
	index := NewGraphIndex(kg)

	result := index.Search(SearchQuery{Keyword: "Node", Limit: 0})
	if len(result.Results) > 10 {
		t.Errorf("expected default limit 10, got %d", len(result.Results))
	}
}

// TestTokenize 测试分词
func TestTokenize(t *testing.T) {
	tokens := tokenize("Hello World Test")
	if len(tokens) == 0 {
		t.Error("expected tokens for 'Hello World Test'")
	}

	// 测试中文
	tokens = tokenize("搜索测试")
	if len(tokens) == 0 {
		t.Error("expected tokens for Chinese text")
	}
}

// TestNewObserverStructureStep 测试结构观测 Step
func TestNewObserverStructureStep(t *testing.T) {
	kg := makeTestGraph(5)
	observer := NewGraphObserver(kg)
	step := NewObserverStructureStep(observer)

	ctx := context.Background()
	obs, err := step.Execute(ctx, StructureObservationInput{})
	if err != nil {
		t.Fatalf("observer structure step: %v", err)
	}
	if obs.NodeCount != 5 {
		t.Errorf("expected 5 nodes, got %d", obs.NodeCount)
	}
}

// TestNewObserverSearchStep 测试搜索 Step
func TestNewObserverSearchStep(t *testing.T) {
	kg := makeTestGraph(10)
	kg.Nodes[0].Name = "UniqueSearchTarget"
	observer := NewGraphObserver(kg)
	step := NewObserverSearchStep(observer)

	ctx := context.Background()
	result, err := step.Execute(ctx, SearchQuery{Keyword: "UniqueSearchTarget", Limit: 5})
	if err != nil {
		t.Fatalf("observer search step: %v", err)
	}
	if result.Total == 0 {
		t.Error("expected search results")
	}
}

// ============================================================================
// T-17: 全链路集成测试
// ============================================================================

// TestFullChain_ParseToSupervise 测试解析→监督全链路
func TestFullChain_ParseToSupervise(t *testing.T) {
	// 1. 创建测试图谱
	kg := makeTestGraph(20)

	// 2. 序列化
	data, err := json.Marshal(kg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	// 3. 反序列化
	parsed, err := ParseKnowledgeGraph(data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	// 4. 验证
	warnings := ValidateKnowledgeGraph(parsed)
	if len(warnings) > 0 {
		t.Logf("validation warnings: %v", warnings)
	}

	// 5. 监督
	supervisor := NewGraphSupervisor(nil)
	report := supervisor.ValidateAll(parsed)
	if len(report.Results) != 6 {
		t.Errorf("expected 6 constraint results, got %d", len(report.Results))
	}

	// 6. 观测
	observer := NewGraphObserver(parsed)
	obs := observer.ObserveStructure()
	if obs.NodeCount != 20 {
		t.Errorf("expected 20 nodes, got %d", obs.NodeCount)
	}
}

// TestFullChain_MigrationFlow 测试迁移流程
func TestFullChain_MigrationFlow(t *testing.T) {
	before := makeTestGraph(10)
	after := makeTestGraph(10)

	// 模拟变更
	after.Nodes[0].Name = "Modified"
	after.Nodes = append(after.Nodes, GraphNode{
		ID:   "new-node",
		Type: NodeTypeFunction,
		Name: "NewFunction",
	})

	// 1. Diff
	diff := DiffKnowledgeGraphs(before, after)
	if diff.Summary.TotalChanges == 0 {
		t.Error("expected changes in diff")
	}

	// 2. Layer drift
	drifts := DetectLayerDrifts(before, after)
	if len(drifts) == 0 {
		t.Error("expected layer drifts")
	}

	// 3. 监督 after
	supervisor := NewGraphSupervisor(nil)
	report := supervisor.ValidateAll(after)
	if len(report.Results) != 6 {
		t.Errorf("expected 6 constraint results, got %d", len(report.Results))
	}

	// 4. 观测变更
	observer := NewGraphObserver(after)
	change := observer.ObserveChanges(before)
	if change.Diff == nil {
		t.Error("expected diff in change observation")
	}
}

// TestFullChain_SearchAndObserve 测试搜索→观测链路
func TestFullChain_SearchAndObserve(t *testing.T) {
	kg := makeTestGraph(30)
	kg.Nodes[10].Name = "TargetComponent"
	kg.Nodes[10].Summary = "Critical component for the system"
	kg.Nodes[10].Tags = []string{"critical", "core"}

	observer := NewGraphObserver(kg)

	// 1. 结构观测
	obs := observer.ObserveStructure()
	if obs.NodeCount != 30 {
		t.Errorf("expected 30 nodes, got %d", obs.NodeCount)
	}

	// 2. 搜索
	result := observer.SearchGraph(SearchQuery{Keyword: "TargetComponent", Limit: 5})
	if result.Total == 0 {
		t.Error("expected search results")
	}

	// 3. 通过搜索结果进行约束检查
	found := result.Results[0]
	if found.Node.Name != "TargetComponent" {
		t.Errorf("expected TargetComponent, got %s", found.Node.Name)
	}
}

// ============================================================================
// T-18: 性能基准测试
// ============================================================================

// BenchmarkParseKnowledgeGraph 基准测试：解析
func BenchmarkParseKnowledgeGraph(b *testing.B) {
	kg := makeTestGraph(100)
	data, _ := json.Marshal(kg)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ParseKnowledgeGraph(data)
	}
}

// BenchmarkValidateKnowledgeGraph 基准测试：验证
func BenchmarkValidateKnowledgeGraph(b *testing.B) {
	kg := makeTestGraph(500)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ValidateKnowledgeGraph(kg)
	}
}

// BenchmarkDiffKnowledgeGraphs 基准测试：差异对比
func BenchmarkDiffKnowledgeGraphs(b *testing.B) {
	before := makeTestGraph(500)
	after := makeTestGraph(500)
	after.Nodes[0].Name = "Modified"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		DiffKnowledgeGraphs(before, after)
	}
}

// BenchmarkSupervisor_ValidateAll 基准测试：约束验证
func BenchmarkSupervisor_ValidateAll(b *testing.B) {
	kg := makeTestGraph(500)
	supervisor := NewGraphSupervisor(nil)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		supervisor.ValidateAll(kg)
	}
}

// BenchmarkObserver_ObserveStructure 基准测试：结构观测
func BenchmarkObserver_ObserveStructure(b *testing.B) {
	kg := makeTestGraph(500)
	observer := NewGraphObserver(kg)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		observer.ObserveStructure()
	}
}

// BenchmarkGraphIndex_Search 基准测试：搜索
func BenchmarkGraphIndex_Search(b *testing.B) {
	kg := makeTestGraph(500)
	index := NewGraphIndex(kg)
	query := SearchQuery{Keyword: "Node", Limit: 10}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		index.Search(query)
	}
}

// BenchmarkGraphIndex_Build 基准测试：索引构建
func BenchmarkGraphIndex_Build(b *testing.B) {
	kg := makeTestGraph(1000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		NewGraphIndex(kg)
	}
}

// BenchmarkCountByType 基准测试：类型统计
func BenchmarkCountByType(b *testing.B) {
	kg := makeTestGraph(1000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		CountByType(kg)
	}
}

// ============================================================================
// 辅助函数
// ============================================================================

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// ============================================================================
// 集成测试：UnderstandAdapter 与文件系统
// ============================================================================

// TestUnderstandAdapter_Config 测试适配器配置
func TestUnderstandAdapter_Config(t *testing.T) {
	config := DefaultUnderstandConfig("test-project")
	if config.ProjectRoot != "test-project" {
		t.Errorf("expected ProjectRoot 'test-project', got %q", config.ProjectRoot)
	}
	if config.Timeout == 0 {
		t.Error("expected non-zero timeout")
	}
}

// TestUnderstandAdapter_LoadGraph 测试从文件加载图谱
func TestUnderstandAdapter_LoadGraph(t *testing.T) {
	// 创建临时目录和图谱文件
	tmpDir := t.TempDir()

	outputDir := filepath.Join(tmpDir, ".understand-anything")
	os.MkdirAll(outputDir, 0755)

	kg := makeTestGraph(5)
	data, _ := json.Marshal(kg)
	os.WriteFile(filepath.Join(outputDir, "knowledge-graph.json"), data, 0644)

	config := UnderstandConfig{
		ProjectRoot: tmpDir,
		OutputDir:   outputDir,
		Timeout:     10 * time.Second,
	}
	adapter := NewUnderstandAdapter(config)

	loaded, err := adapter.LoadGraph(context.Background())
	if err != nil {
		t.Fatalf("LoadGraph: %v", err)
	}
	if loaded.Version != kg.Version {
		t.Errorf("version mismatch: got %q, want %q", loaded.Version, kg.Version)
	}
}

// TestUnderstandAdapter_HasGraph 测试图谱文件存在性检查
func TestUnderstandAdapter_HasGraph(t *testing.T) {
	tmpDir := t.TempDir()

	outputDir := filepath.Join(tmpDir, ".understand-anything")
	os.MkdirAll(outputDir, 0755)

	config := UnderstandConfig{
		ProjectRoot: tmpDir,
		OutputDir:   outputDir,
	}
	adapter := NewUnderstandAdapter(config)

	// 初始应无图谱
	if adapter.HasGraph() {
		t.Error("expected HasGraph=false for empty directory")
	}

	// 创建图谱文件
	kg := makeTestGraph(3)
	data, _ := json.Marshal(kg)
	os.WriteFile(filepath.Join(outputDir, "knowledge-graph.json"), data, 0644)

	if !adapter.HasGraph() {
		t.Error("expected HasGraph=true after creating graph file")
	}
}

// ============================================================================
// 集成测试：MigrationAdapter
// ============================================================================

// TestMigrationAdapter_CreateBaseline 测试创建基线
func TestMigrationAdapter_CreateBaseline(t *testing.T) {
	tmpDir := t.TempDir()

	outputDir := filepath.Join(tmpDir, ".understand-anything")
	os.MkdirAll(outputDir, 0755)

	kg := makeTestGraph(10)
	data, _ := json.Marshal(kg)
	os.WriteFile(filepath.Join(outputDir, "knowledge-graph.json"), data, 0644)

	uaAdapter := NewUnderstandAdapter(UnderstandConfig{
		ProjectRoot: tmpDir,
		OutputDir:   outputDir,
	})
	migrationAdapter := NewUnderstandMigrationAdapter(uaAdapter)

	baseline, err := migrationAdapter.CreateBaseline(context.Background(), "v1.0.0")
	if err != nil {
		t.Fatalf("CreateBaseline: %v", err)
	}
	if baseline.Version != "v1.0.0" {
		t.Errorf("version: got %q, want 'v1.0.0'", baseline.Version)
	}
	if baseline.NodeCount != len(kg.Nodes) {
		t.Errorf("node_count: got %d, want %d", baseline.NodeCount, len(kg.Nodes))
	}
}

// TestMigrationAdapter_SaveAndLoadBaseline 测试基线持久化
func TestMigrationAdapter_SaveAndLoadBaseline(t *testing.T) {
	tmpDir := t.TempDir()

	outputDir := filepath.Join(tmpDir, ".understand-anything")
	os.MkdirAll(outputDir, 0755)

	kg := makeTestGraph(5)
	data, _ := json.Marshal(kg)
	os.WriteFile(filepath.Join(outputDir, "knowledge-graph.json"), data, 0644)

	uaAdapter := NewUnderstandAdapter(UnderstandConfig{
		ProjectRoot: tmpDir,
		OutputDir:   outputDir,
	})
	migrationAdapter := NewUnderstandMigrationAdapter(uaAdapter)

	_, err := migrationAdapter.CreateBaseline(context.Background(), "v1.0.0")
	if err != nil {
		t.Fatalf("CreateBaseline: %v", err)
	}

	err = migrationAdapter.SaveBaseline("v1.0.0")
	if err != nil {
		t.Fatalf("SaveBaseline: %v", err)
	}

	// 验证文件存在
	baselinePath := filepath.Join(outputDir, "baselines", "v1.0.0.json")
	if _, err := os.Stat(baselinePath); os.IsNotExist(err) {
		t.Error("baseline file not created")
	}

	// 创建新的 migration adapter 并加载基线
	migrationAdapter2 := NewUnderstandMigrationAdapter(uaAdapter)
	report, err := migrationAdapter2.CompareWithBaseline(context.Background(), "v1.0.0")
	if err != nil {
		t.Fatalf("CompareWithBaseline: %v", err)
	}
	if !report.Success {
		t.Error("expected successful comparison")
	}
}

// TestNewMigrationStep 测试迁移 Step 包装
func TestNewMigrationStep(t *testing.T) {
	tmpDir := t.TempDir()

	outputDir := filepath.Join(tmpDir, ".understand-anything")
	os.MkdirAll(outputDir, 0755)

	kg := makeTestGraph(5)
	data, _ := json.Marshal(kg)
	os.WriteFile(filepath.Join(outputDir, "knowledge-graph.json"), data, 0644)

	uaAdapter := NewUnderstandAdapter(UnderstandConfig{
		ProjectRoot: tmpDir,
		OutputDir:   outputDir,
	})
	migrationAdapter := NewUnderstandMigrationAdapter(uaAdapter)
	step := NewMigrationStep(migrationAdapter)

	ctx := context.Background()
	report, err := step.Execute(ctx, MigrationRequest{
		SourceVersion: "v1.0.0",
		TargetVersion: "v1.1.0",
		Description:   "Test migration",
	})
	if err != nil {
		t.Fatalf("MigrationStep: %v", err)
	}
	if !report.Success {
		t.Error("expected successful migration")
	}
}

// ============================================================================
// 边界条件测试
// ============================================================================

// TestEdgeCases_EmptyNodes 测试空节点列表
func TestEdgeCases_EmptyNodes(t *testing.T) {
	kg := &KnowledgeGraph{
		Version: "1.0",
		Project: ProjectMeta{Name: "empty"},
		Nodes:   []GraphNode{},
		Edges:   []GraphEdge{},
	}

	// 所有操作应正确处理空图谱
	_ = ValidateKnowledgeGraph(kg)
	_ = CountByType(kg)
	_ = CountByEdgeType(kg)

	observer := NewGraphObserver(kg)
	obs := observer.ObserveStructure()
	if obs.NodeCount != 0 {
		t.Error("expected 0 nodes")
	}

	supervisor := NewGraphSupervisor(nil)
	report := supervisor.ValidateAll(kg)
	if report.PassCount != 6 {
		t.Errorf("expected 6 passes, got %d", report.PassCount)
	}
}

// TestEdgeCases_LargeIDs 测试长 ID
func TestEdgeCases_LargeIDs(t *testing.T) {
	longID := "very-long-node-id-that-exceeds-normal-length-" + makeString(100)
	kg := &KnowledgeGraph{
		Version: "1.0",
		Project: ProjectMeta{Name: "long-id"},
		Nodes: []GraphNode{
			{ID: longID, Type: NodeTypeFunction, Name: "LongIDNode"},
			{ID: "node-2", Type: NodeTypeFile, Name: "File2"},
		},
		Edges: []GraphEdge{
			{Source: longID, Target: "node-2", Type: EdgeTypeCalls, Weight: 0.5},
		},
	}

	warnings := ValidateKnowledgeGraph(kg)
	if len(warnings) > 0 {
		t.Logf("warnings: %v", warnings)
	}
}

// TestEdgeCases_DuplicateNodes 测试重复节点 ID
func TestEdgeCases_DuplicateNodes(t *testing.T) {
	kg := &KnowledgeGraph{
		Version: "1.0",
		Project: ProjectMeta{Name: "duplicate"},
		Nodes: []GraphNode{
			{ID: "dup-1", Type: NodeTypeFunction, Name: "Func1"},
			{ID: "dup-1", Type: NodeTypeClass, Name: "Class1"},
		},
		Edges: []GraphEdge{
			{Source: "dup-1", Target: "dup-1", Type: EdgeTypeCalls, Weight: 0.5},
		},
	}

	// DiffKnowledgeGraphs 应该能处理重复 ID
	diff := DiffKnowledgeGraphs(kg, kg)
	if diff.Summary.TotalChanges != 0 {
		t.Error("expected 0 changes for identical graphs")
	}
}

// ============================================================================
// 确定性与并发安全测试
// ============================================================================

// TestDeterministic_ParseKnowledgeGraph 测试解析确定性
func TestDeterministic_ParseKnowledgeGraph(t *testing.T) {
	kg := makeTestGraph(10)
	data, _ := json.Marshal(kg)

	for i := 0; i < 10; i++ {
		parsed, err := ParseKnowledgeGraph(data)
		if err != nil {
			t.Fatalf("iteration %d: %v", i, err)
		}
		if len(parsed.Nodes) != len(kg.Nodes) {
			t.Errorf("iteration %d: node count mismatch", i)
		}
	}
}

// TestDeterministic_DiffKnowledgeGraphs 测试差异确定性
func TestDeterministic_DiffKnowledgeGraphs(t *testing.T) {
	before := makeTestGraph(20)
	after := makeTestGraph(20)
	after.Nodes[5].Name = "Changed"

	var firstSummary DiffSummary
	for i := 0; i < 10; i++ {
		diff := DiffKnowledgeGraphs(before, after)
		if i == 0 {
			firstSummary = diff.Summary
		} else if diff.Summary != firstSummary {
			t.Errorf("iteration %d: non-deterministic diff summary", i)
		}
	}
}

// TestDeterministic_ValidateAll 测试监督确定性
func TestDeterministic_ValidateAll(t *testing.T) {
	kg := makeTestGraph(10)
	supervisor := NewGraphSupervisor(nil)

	var firstSummary string
	for i := 0; i < 10; i++ {
		report := supervisor.ValidateAll(kg)
		if i == 0 {
			firstSummary = report.Summary
		} else if report.Summary != firstSummary {
			t.Errorf("iteration %d: non-deterministic supervisor summary", i)
		}
	}
}

func makeString(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = 'x'
	}
	return string(b)
}