// Package core — Understand-Anything 数据桥接层类型 (v0.7.0)
package core

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
