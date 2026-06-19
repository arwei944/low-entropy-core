// Package core — Understand-Anything 数据桥接层 (v0.7.0)
//
// T-01: Go 类型定义 — 直接映射 UA 的 KnowledgeGraph JSON Schema
// T-02: ParseKnowledgeGraph — 纯 Atom 函数，确定性解析
//
// 约束遵循:
//   C1: 单一包 — 所有代码属 package core
//   C3: 原语纯度 — ParseKnowledgeGraph 是纯函数
//   C6: 泛型优先 — 无 interface{} 使用

package core

import (
	"encoding/json"
	"fmt"
)

// ============================================================================
// T-01: KnowledgeGraph 核心类型 (21 节点类型 + 35 边类型)
// ============================================================================

// KnowledgeGraph 是 UA 输出的完整知识图谱
type KnowledgeGraph struct {
	Version string        `json:"version"`
	Kind    string        `json:"kind,omitempty"` // "codebase" | "knowledge"
	Project ProjectMeta   `json:"project"`
	Nodes   []GraphNode   `json:"nodes"`
	Edges   []GraphEdge   `json:"edges"`
	Layers  []GraphLayer  `json:"layers"`
	Tour    []TourStep    `json:"tour"`
}

// ProjectMeta 项目元数据
type ProjectMeta struct {
	Name          string   `json:"name"`
	Languages     []string `json:"languages"`
	Frameworks    []string `json:"frameworks"`
	Description   string   `json:"description"`
	AnalyzedAt    string   `json:"analyzedAt"`
	GitCommitHash string   `json:"gitCommitHash"`
}

// GraphNode 图谱节点 (21 种类型)
type GraphNode struct {
	ID            string       `json:"id"`
	Type          string       `json:"type"`
	Name          string       `json:"name"`
	FilePath      string       `json:"filePath,omitempty"`
	LineRange     [2]int       `json:"lineRange,omitempty"`
	Summary       string       `json:"summary"`
	Tags          []string     `json:"tags"`
	Complexity    string       `json:"complexity"` // "simple" | "moderate" | "complex"
	LanguageNotes string       `json:"languageNotes,omitempty"`
	DomainMeta    *DomainMeta  `json:"domainMeta,omitempty"`
	KnowledgeMeta *KnowledgeMeta `json:"knowledgeMeta,omitempty"`
}

// DomainMeta 业务领域元数据
type DomainMeta struct {
	Entities               []string `json:"entities,omitempty"`
	BusinessRules          []string `json:"businessRules,omitempty"`
	CrossDomainInteractions []string `json:"crossDomainInteractions,omitempty"`
	EntryPoint             string   `json:"entryPoint,omitempty"`
	EntryType              string   `json:"entryType,omitempty"` // "http" | "cli" | "event" | "cron" | "manual"
}

// KnowledgeMeta 知识库元数据
type KnowledgeMeta struct {
	Wikilinks []string `json:"wikilinks,omitempty"`
	Backlinks []string `json:"backlinks,omitempty"`
	Category  string   `json:"category,omitempty"`
	Content   string   `json:"content,omitempty"`
}

// GraphEdge 图谱边 (35 种类型)
type GraphEdge struct {
	Source      string  `json:"source"`
	Target      string  `json:"target"`
	Type        string  `json:"type"`
	Direction   string  `json:"direction"` // "forward" | "backward" | "bidirectional"
	Description string  `json:"description,omitempty"`
	Weight      float64 `json:"weight"` // 0-1
}

// GraphLayer 架构层级分组
type GraphLayer struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	NodeIDs     []string `json:"nodeIds"`
}

// TourStep 学习导览步骤
type TourStep struct {
	Order          int      `json:"order"`
	Title          string   `json:"title"`
	Description    string   `json:"description"`
	NodeIDs        []string `json:"nodeIds"`
	LanguageLesson string   `json:"languageLesson,omitempty"`
}

// ============================================================================
// 节点类型常量 (21 种)
// ============================================================================
const (
	NodeTypeFile      = "file"
	NodeTypeFunction  = "function"
	NodeTypeClass     = "class"
	NodeTypeModule    = "module"
	NodeTypeConcept   = "concept"
	NodeTypeConfig    = "config"
	NodeTypeDocument  = "document"
	NodeTypeService   = "service"
	NodeTypeTable     = "table"
	NodeTypeEndpoint  = "endpoint"
	NodeTypePipeline  = "pipeline"
	NodeTypeSchema    = "schema"
	NodeTypeResource  = "resource"
	NodeTypeDomain    = "domain"
	NodeTypeFlow      = "flow"
	NodeTypeStep      = "step"
	NodeTypeArticle   = "article"
	NodeTypeEntity    = "entity"
	NodeTypeTopic     = "topic"
	NodeTypeClaim     = "claim"
	NodeTypeSource    = "source"
)

// ValidNodeTypes 返回所有有效的节点类型
func ValidNodeTypes() []string {
	return []string{
		NodeTypeFile, NodeTypeFunction, NodeTypeClass, NodeTypeModule, NodeTypeConcept,
		NodeTypeConfig, NodeTypeDocument, NodeTypeService, NodeTypeTable, NodeTypeEndpoint,
		NodeTypePipeline, NodeTypeSchema, NodeTypeResource,
		NodeTypeDomain, NodeTypeFlow, NodeTypeStep,
		NodeTypeArticle, NodeTypeEntity, NodeTypeTopic, NodeTypeClaim, NodeTypeSource,
	}
}

// ============================================================================
// 边类型常量 (35 种，按 8 大类分组)
// ============================================================================
const (
	// Structural (5)
	EdgeTypeImports    = "imports"
	EdgeTypeExports    = "exports"
	EdgeTypeContains   = "contains"
	EdgeTypeInherits   = "inherits"
	EdgeTypeImplements = "implements"

	// Behavioral (4)
	EdgeTypeCalls       = "calls"
	EdgeTypeSubscribes  = "subscribes"
	EdgeTypePublishes   = "publishes"
	EdgeTypeMiddleware  = "middleware"

	// Data flow (4)
	EdgeTypeReadsFrom  = "reads_from"
	EdgeTypeWritesTo   = "writes_to"
	EdgeTypeTransforms = "transforms"
	EdgeTypeValidates  = "validates"

	// Dependencies (3)
	EdgeTypeDependsOn  = "depends_on"
	EdgeTypeTestedBy   = "tested_by"
	EdgeTypeConfigures = "configures"

	// Semantic (2)
	EdgeTypeRelated   = "related"
	EdgeTypeSimilarTo = "similar_to"

	// Infrastructure (4)
	EdgeTypeDeploys    = "deploys"
	EdgeTypeServes     = "serves"
	EdgeTypeProvisions = "provisions"
	EdgeTypeTriggers   = "triggers"

	// Schema/Data (4)
	EdgeTypeMigrates      = "migrates"
	EdgeTypeDocuments     = "documents"
	EdgeTypeRoutes        = "routes"
	EdgeTypeDefinesSchema = "defines_schema"

	// Domain (3)
	EdgeTypeContainsFlow = "contains_flow"
	EdgeTypeFlowStep     = "flow_step"
	EdgeTypeCrossDomain  = "cross_domain"

	// Knowledge (6)
	EdgeTypeCites             = "cites"
	EdgeTypeContradicts       = "contradicts"
	EdgeTypeBuildsOn          = "builds_on"
	EdgeTypeExemplifies       = "exemplifies"
	EdgeTypeCategorizedUnder  = "categorized_under"
	EdgeTypeAuthoredBy        = "authored_by"
)

// ValidEdgeTypes 返回所有有效的边类型
func ValidEdgeTypes() []string {
	return []string{
		EdgeTypeImports, EdgeTypeExports, EdgeTypeContains, EdgeTypeInherits, EdgeTypeImplements,
		EdgeTypeCalls, EdgeTypeSubscribes, EdgeTypePublishes, EdgeTypeMiddleware,
		EdgeTypeReadsFrom, EdgeTypeWritesTo, EdgeTypeTransforms, EdgeTypeValidates,
		EdgeTypeDependsOn, EdgeTypeTestedBy, EdgeTypeConfigures,
		EdgeTypeRelated, EdgeTypeSimilarTo,
		EdgeTypeDeploys, EdgeTypeServes, EdgeTypeProvisions, EdgeTypeTriggers,
		EdgeTypeMigrates, EdgeTypeDocuments, EdgeTypeRoutes, EdgeTypeDefinesSchema,
		EdgeTypeContainsFlow, EdgeTypeFlowStep, EdgeTypeCrossDomain,
		EdgeTypeCites, EdgeTypeContradicts, EdgeTypeBuildsOn, EdgeTypeExemplifies, EdgeTypeCategorizedUnder, EdgeTypeAuthoredBy,
	}
}

// ============================================================================
// T-02: ParseKnowledgeGraph — 纯 Atom 函数
// ============================================================================

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