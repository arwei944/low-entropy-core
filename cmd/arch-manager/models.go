package main

import "time"

// ============================================================================
// 核心数据模型
// ============================================================================

// Symbol 表示一个导出符号（类型、函数、方法、常量、变量）
type Symbol struct {
	Name       string   `json:"name"`
	Kind       string   `json:"kind"` // "type", "func", "method", "const", "var", "interface"
	Signature  string   `json:"signature,omitempty"`
	Receiver   string   `json:"receiver,omitempty"` // 方法接收者
	Doc        string   `json:"doc,omitempty"`
	Fields     []string `json:"fields,omitempty"`     // struct 字段
	Methods    []string `json:"methods,omitempty"`    // interface 方法
	Values     []string `json:"values,omitempty"`     // const/var 值
	IsExported bool     `json:"is_exported"`
}

// FileInfo 表示一个 Go 源文件
type FileInfo struct {
	Path       string   `json:"path"`
	Name       string   `json:"name"`
	Package    string   `json:"package"`
	Lines      int      `json:"lines"`
	Imports    []string `json:"imports"`
	Symbols    []Symbol `json:"symbols"`
	Layer      string   `json:"layer"`      // L0-L7
	LayerName  string   `json:"layer_name"` // 层名称
	DependsOn  []string `json:"depends_on"` // 依赖的文件名列表
	DependedBy []string `json:"depended_by"`
}

// ArchData 是整个架构的数据快照
type ArchData struct {
	GeneratedAt  time.Time      `json:"generated_at"`
	TotalFiles   int            `json:"total_files"`
	TotalLines   int            `json:"total_lines"`
	TotalSymbols int            `json:"total_symbols"`
	Files        []FileInfo     `json:"files"`
	Layers       []LayerStat    `json:"layers"`
	SymbolKinds  map[string]int `json:"symbol_kinds"`
}

// LayerStat 是每层的统计
type LayerStat struct {
	Layer   string `json:"layer"`
	Name    string `json:"name"`
	Files   int    `json:"files"`
	Lines   int    `json:"lines"`
	Symbols int    `json:"symbols"`
	Color   string `json:"color"`
}

// EnhancedFileInfo 是带复杂度评分的文件信息
type EnhancedFileInfo struct {
	FileInfo
	ComplexityScore float64 `json:"complexity_score"`
}

// EnhancedArchData 是带复杂度评分的架构数据
type EnhancedArchData struct {
	*ArchData
	ComplexityScores map[string]float64 `json:"complexity_scores"`
	MaxLines         int                `json:"max_lines"`
	MaxSymbols       int                `json:"max_symbols"`
	MaxDeps          int                `json:"max_deps"`
}

// ============================================================================
// Agent 数据模型
// ============================================================================

// AgentStatus 表示 Agent 的当前运行状态
type AgentStatus string

const (
	AgentStatusIdle    AgentStatus = "idle"
	AgentStatusWorking AgentStatus = "working"
	AgentStatusError   AgentStatus = "error"
	AgentStatusOffline AgentStatus = "offline"
)

// Agent 表示 AgentPool 中一个已注册的 Agent 实例
type Agent struct {
	ID            string      `json:"id"`
	Status        AgentStatus `json:"status"`
	Capabilities  []string    `json:"capabilities"`
	Phase         string      `json:"phase"`
	LastHeartbeat time.Time   `json:"last_heartbeat"`
	CurrentTask   string      `json:"current_task,omitempty"`
}

// SubmissionResult 表示 Agent 的一次任务提交结果
type SubmissionResult struct {
	ID        string    `json:"id"`
	AgentID   string    `json:"agent_id"`
	Task      string    `json:"task"`
	Status    string    `json:"status"` // "success", "failed", "partial"
	Output    string    `json:"output,omitempty"`
	Error     string    `json:"error,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// AgentEvent 表示 Agent 状态变化事件（用于 SSE 推送）
type AgentEvent struct {
	Type      string    `json:"type"` // "register", "unregister", "status_change", "submission"
	AgentID   string    `json:"agent_id"`
	Timestamp time.Time `json:"timestamp"`
	Data      any       `json:"data,omitempty"`
}

// ============================================================================
// 健康评分模型
// ============================================================================

// HealthScore 架构健康度评分
type HealthScore struct {
	Overall     float64            `json:"overall"`
	Grade       string             `json:"grade"`
	Factors     map[string]float64 `json:"factors"`
	Suggestions []string           `json:"suggestions"`
}

// ============================================================================
// 违规检测模型
// ============================================================================

// Violation 架构违规记录
type Violation struct {
	Severity string `json:"severity"`
	Type     string `json:"type"`
	File     string `json:"file"`
	Message  string `json:"message"`
	Detail   string `json:"detail"`
}

// ============================================================================
// 引导层模型
// ============================================================================

// GuideData 引导层结构化数据
type GuideData struct {
	Primitives  []PrimitiveDef    `json:"primitives"`
	Layers      []LayerDepEdge    `json:"layer_deps"`
	Constraints []ConstraintCheck `json:"constraints"`
	Patterns    []PatternDef      `json:"patterns"`
	QuickStart  QuickStartInfo    `json:"quick_start"`
	Tour        *TourGuide        `json:"tour,omitempty"`
}

// TourGuide UA 学习导览
type TourGuide struct {
	Title       string          `json:"title"`
	Description string          `json:"description"`
	Steps       []TourGuideStep `json:"steps"`
	Available   bool            `json:"available"`
}

// TourGuideStep 学习导览步骤
type TourGuideStep struct {
	Order       int      `json:"order"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	NodeCount   int      `json:"node_count"`
	KeyConcepts []string `json:"key_concepts,omitempty"`
}

// PrimitiveDef 原语定义
type PrimitiveDef struct {
	Name        string `json:"name"`
	Signature   string `json:"signature"`
	Description string `json:"description"`
	Color       string `json:"color"`
	Example     string `json:"example"`
}

// LayerDepEdge 层级依赖边
type LayerDepEdge struct {
	From  string `json:"from"`
	To    string `json:"to"`
	Label string `json:"label"`
}

// ConstraintCheck 约束检查结果
type ConstraintCheck struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Status      string `json:"status"` // "pass", "fail", "warn"
	Detail      string `json:"detail"`
}

// PatternDef 模式定义
type PatternDef struct {
	Name        string `json:"name"`
	Code        string `json:"code"`
	Description string `json:"description"`
	UseCase     string `json:"use_case"`
	FullExample string `json:"full_example"`
}

// QuickStartInfo 快速入门信息
type QuickStartInfo struct {
	Code     string `json:"code"`
	DataFlow string `json:"data_flow"`
}

// ============================================================================
// 层级映射
// ============================================================================

// LayerInfo 层级信息
type LayerInfo struct {
	Layer string
	Name  string
	Color string
}

var fileLayerMap = map[string]LayerInfo{
	"perf_core.go":                {"L0", "性能基础设施", "#7f8ea3"},
	"perf_tdigest.go":             {"L0", "性能基础设施", "#7f8ea3"},
	"perf_sharded_observation.go": {"L0", "性能基础设施", "#7f8ea3"},
	"types.go":                    {"L0", "类型定义", "#7f8ea3"},
	"errors.go":                   {"L0", "错误处理", "#7f8ea3"},
	"fastpath.go":                 {"L0", "快速路径", "#7f8ea3"},
	"atom.go":                     {"L1", "四原语定义", "#00d4aa"},
	"port.go":                     {"L1", "四原语定义", "#00d4aa"},
	"adapter.go":                  {"L1", "四原语定义", "#00d4aa"},
	"composer.go":                 {"L1", "四原语定义", "#00d4aa"},
	"step.go":                     {"L1", "四原语定义", "#00d4aa"},
	"patterns_resilience.go":      {"L2", "单机韧性", "#60a5fa"},
	"degradation.go":              {"L2", "单机韧性", "#60a5fa"},
	"patterns_distributed.go":     {"L3", "分布式韧性", "#38bdf8"},
	"guardian_entropy.go":         {"L4", "Guardian 监督", "#ef4444"},
	"guardian_decision.go":        {"L4", "Guardian 监督", "#ef4444"},
	"guardian_dependency.go":      {"L4", "Guardian 监督", "#ef4444"},
	"guardian_transparency.go":    {"L4", "Guardian 监督", "#ef4444"},
	"observation.go":              {"L5", "Observation 可观测", "#34d399"},
	"observation_pipeline.go":     {"L5", "Observation 可观测", "#34d399"},
	"observation_store.go":        {"L5", "Observation 可观测", "#34d399"},
	"observation_aggregator.go":   {"L5", "Observation 可观测", "#34d399"},
	"observation_sampler.go":      {"L5", "Observation 可观测", "#34d399"},
	"observation_api.go":          {"L5", "Observation 可观测", "#34d399"},
	"eventstore.go":               {"L6", "EventStore 事件溯源", "#f472b6"},
	"eventstore_upgrade.go":       {"L6", "EventStore 事件溯源", "#f472b6"},
	"eventbus.go":                 {"L6", "EventStore 事件溯源", "#f472b6"},
	"projection.go":               {"L6", "EventStore 事件溯源", "#f472b6"},
	"idempotent.go":               {"L6", "EventStore 事件溯源", "#f472b6"},
	"tenant.go":                   {"L6", "EventStore 事件溯源", "#f472b6"},
	"transaction.go":              {"L6", "EventStore 事件溯源", "#f472b6"},
	"config.go":                   {"L7", "应用层", "#f59e0b"},
	"schema.go":                   {"L7", "应用层", "#f59e0b"},
	"handoff.go":                  {"L7", "应用层", "#f59e0b"},
	"handoff_persistence.go":      {"L7", "应用层", "#f59e0b"},
	"scheduler.go":                {"L7", "应用层", "#f59e0b"},
	"security.go":                 {"L7", "应用层", "#f59e0b"},
	"architecture_registry.go":    {"L7", "应用层", "#f59e0b"},
	"port_contract.go":            {"L7", "应用层", "#f59e0b"},
	"entropy_metrics.go":          {"L7", "应用层", "#f59e0b"},
}

// getLayerInfo 根据文件名获取层级信息
func getLayerInfo(filename string) LayerInfo {
	if info, ok := fileLayerMap[filename]; ok {
		return info
	}
	return LayerInfo{"L7", "应用层", "#f59e0b"}
}
