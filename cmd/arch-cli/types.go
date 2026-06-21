// Package main — arch-cli 本地类型定义
//
// 共享类型通过 go-core/arch 别名引入，本地特有类型直接定义。
package main

import (
	"time"

	"low-entropy-core/go-core/arch"
)

// ──────────────────────────────────────────────
// 共享类型别名（L1 go-core/arch）
// ──────────────────────────────────────────────

type Symbol = arch.Symbol
type FileInfo = arch.FileInfo
type ArchData = arch.ArchData
type LayerStat = arch.LayerStat

// EnhancedFileInfo 带复杂度评分
type EnhancedFileInfo struct {
	FileInfo
	ComplexityScore float64 `json:"complexity_score"`
}

// EnhancedArchData 带复杂度评分的架构数据
type EnhancedArchData struct {
	*ArchData
	ComplexityScores map[string]float64 `json:"complexity_scores"`
	MaxLines         int                `json:"max_lines"`
	MaxSymbols       int                `json:"max_symbols"`
	MaxDeps          int                `json:"max_deps"`
}

// ──────────────────────────────────────────────
// Agent 类型
// ──────────────────────────────────────────────

type AgentStatus string

const (
	AgentStatusIdle    AgentStatus = "idle"
	AgentStatusWorking AgentStatus = "working"
	AgentStatusError   AgentStatus = "error"
	AgentStatusOffline AgentStatus = "offline"
)

type Agent struct {
	ID            string      `json:"id"`
	Status        AgentStatus `json:"status"`
	Capabilities  []string    `json:"capabilities"`
	Phase         string      `json:"phase"`
	LastHeartbeat time.Time   `json:"last_heartbeat"`
	CurrentTask   string      `json:"current_task,omitempty"`
}

type SubmissionResult struct {
	ID        string    `json:"id"`
	AgentID   string    `json:"agent_id"`
	Task      string    `json:"task"`
	Status    string    `json:"status"`
	Output    string    `json:"output,omitempty"`
	Error     string    `json:"error,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

type AgentEvent struct {
	Type      string    `json:"type"`
	AgentID   string    `json:"agent_id"`
	Timestamp time.Time `json:"timestamp"`
	Data      any       `json:"data,omitempty"`
}

// ──────────────────────────────────────────────
// 健康评分
// ──────────────────────────────────────────────

type HealthScore struct {
	Overall     float64            `json:"overall"`
	Grade       string             `json:"grade"`
	Factors     map[string]float64 `json:"factors"`
	Suggestions []string           `json:"suggestions"`
}

// ──────────────────────────────────────────────
// 违规检测
// ──────────────────────────────────────────────

type Violation struct {
	Severity string `json:"severity"`
	Type     string `json:"type"`
	File     string `json:"file"`
	Message  string `json:"message"`
	Detail   string `json:"detail"`
}

// ──────────────────────────────────────────────
// 引导层
// ──────────────────────────────────────────────

type PrimitiveDef struct {
	Name        string `json:"name"`
	Signature   string `json:"signature"`
	Description string `json:"description"`
	Color       string `json:"color"`
	Example     string `json:"example"`
}

type LayerDepEdge struct {
	From  string `json:"from"`
	To    string `json:"to"`
	Label string `json:"label"`
}

type ConstraintCheck struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Status      string `json:"status"`
	Detail      string `json:"detail"`
}

type PatternDef struct {
	Name        string `json:"name"`
	Code        string `json:"code"`
	Description string `json:"description"`
	UseCase     string `json:"use_case"`
	FullExample string `json:"full_example"`
}

type QuickStartInfo struct {
	Code     string `json:"code"`
	DataFlow string `json:"data_flow"`
}

type GuideData struct {
	Primitives  []PrimitiveDef    `json:"primitives"`
	Layers      []LayerDepEdge    `json:"layer_deps"`
	Constraints []ConstraintCheck `json:"constraints"`
	Patterns    []PatternDef      `json:"patterns"`
	QuickStart  QuickStartInfo    `json:"quick_start"`
	Tour        *TourGuide        `json:"tour,omitempty"`
}

type TourGuide struct {
	Title       string          `json:"title"`
	Description string          `json:"description"`
	Steps       []TourGuideStep `json:"steps"`
	Available   bool            `json:"available"`
}

type TourGuideStep struct {
	Order       int      `json:"order"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	NodeCount   int      `json:"node_count"`
	KeyConcepts []string `json:"key_concepts,omitempty"`
}

// ──────────────────────────────────────────────
// 层级映射
// ──────────────────────────────────────────────

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

func getLayerInfo(filename string) LayerInfo {
	if info, ok := fileLayerMap[filename]; ok {
		return info
	}
	return LayerInfo{"L7", "应用层", "#f59e0b"}
}