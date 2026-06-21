// Package arch — Architecture Analysis Shared Types (L1)
//
// 定义架构分析的核心数据结构。纯数据结构，无副作用。
// 为后续的 parser/analyzer/validator/renderer/generator 提供类型基础。
//
// 来源（整合自）:
//   - cmd/arch-manager/models.go    (Symbol, FileInfo, ArchData, LayerStat, Violation)
//   - cmd/arch-manager/primitives.go (PrimitiveInfo)
//
// 设计约束:
//   - 仅依赖标准库 time（L1 不允许外部依赖）
//   - 纯数据结构 + 少量方法（String/JSON tag）
//   - 文件 ≤ 300 行
package arch

import "time"

// ──────────────────────────────────────────────
// 符号 & 文件（Go 源码元数据）
// ──────────────────────────────────────────────

// Symbol 表示 Go 源文件中的一个导出符号。
// 覆盖 type/func/method/const/var/interface 六类声明。
type Symbol struct {
	Name       string   `json:"name"`
	Kind       string   `json:"kind"`              // "type" | "func" | "method" | "const" | "var" | "interface"
	Signature  string   `json:"signature"`         // 类型签名
	Receiver   string   `json:"receiver"`          // 方法接收者
	Doc        string   `json:"doc,omitempty"`     // 文档注释
	Fields     []string `json:"fields,omitempty"`  // struct 字段
	Methods    []string `json:"methods,omitempty"` // interface 方法
	Values     []string `json:"values,omitempty"`   // const/var 值
	IsExported bool     `json:"is_exported"`        // 是否大写开头
}

// FileInfo 表示一个 Go 源文件的解析结果。
// 这是架构分析的最小数据单元 — 所有后续分析都以此为基础。
type FileInfo struct {
	Path       string   `json:"path"`        // 完整路径
	Name       string   `json:"name"`        // 文件名（不含路径）
	Package    string   `json:"package"`     // package 声明
	Lines      int      `json:"lines"`       // 代码行数
	Imports    []string `json:"imports"`     // import 的包路径
	Symbols    []Symbol `json:"symbols"`     // 定义的符号
	Layer      string   `json:"layer"`       // 所属架构层（"L0"~"L7"）
	LayerName  string   `json:"layer_name"`  // 层的可读名称
	DependsOn  []string `json:"depends_on"`  // 依赖的其他文件（相对于本模块）
	DependedBy []string `json:"depended_by"` // 被哪些文件依赖
}

// ──────────────────────────────────────────────
// 架构快照 & 统计
// ──────────────────────────────────────────────

// ArchData 是某一时刻整个项目的架构快照。
// 由 Analyzer Atom 根据一组 FileInfo 计算得出。
type ArchData struct {
	GeneratedAt  time.Time           `json:"generated_at"`
	ProjectRoot  string              `json:"project_root"`
	TotalFiles   int                 `json:"total_files"`
	TotalLines   int                 `json:"total_lines"`
	TotalSymbols int                 `json:"total_symbols"`
	Files        []FileInfo          `json:"files"`
	Layers       []LayerStat         `json:"layers"`
	SymbolKinds  map[string]int      `json:"symbol_kinds"` // 按 Kind 聚合的符号数
	Primitives   []PrimitiveInfo     `json:"primitives,omitempty"`
}

// LayerStat 表示某一架构层级（L0~L7）的统计信息。
type LayerStat struct {
	Layer   string `json:"layer"`   // "L0" | "L1" | ... | "L7"
	Name    string `json:"name"`    // 可读名称
	Files   int    `json:"files"`   // 文件数
	Lines   int    `json:"lines"`   // 代码行数
	Symbols int    `json:"symbols"` // 符号数
	Color   string `json:"color"`   // 可视化颜色
}

// ──────────────────────────────────────────────
// 四原语识别信息
// ──────────────────────────────────────────────

// PrimitiveInfo 表示一个检测到的四原语接口断言。
// 由 Analyzer Atom 从 AST 中识别 `var _ Atom[...] = (*Type)(nil)` 模式产生。
type PrimitiveInfo struct {
	Name       string `json:"name"`        // 实现类型名（如 "ParseFileAtom"）
	Type       string `json:"type"`        // "Atom" | "Port" | "Adapter" | "Composer"
	File       string `json:"file"`        // 所在文件名
	Package    string `json:"package"`     // 所在包
	Line       int    `json:"line"`        // 行号
	Signature  string `json:"signature"`   // 完整断言签名
	IsExported bool   `json:"is_exported"` // 是否大写开头
}

// ──────────────────────────────────────────────
// 违规检测
// ──────────────────────────────────────────────

// ViolationSeverity 表示违规的严重程度。
type ViolationSeverity string

const (
	SeverityInfo  ViolationSeverity = "info"   // 信息级（建议改进）
	SeverityWarn  ViolationSeverity = "warn"   // 警告级（应修复）
	SeverityError ViolationSeverity = "error"  // 错误级（必须修复）
)

// ViolationType 表示违规的类型。
// 严格对齐 CLAUDE.md 中禁止事项的枚举。
type ViolationType string

const (
	// L1/L2/L3 依赖上层包
	ViolationReverseDependency ViolationType = "reverse_dependency"
	// 跨层跳跃（如 L3 直接依赖 L5）
	ViolationLayerJump ViolationType = "layer_jump"
	// 循环依赖
	ViolationCircularDependency ViolationType = "circular_dependency"
	// 业务逻辑未使用四原语模式（裸写 func）
	ViolationMissingPrimitive ViolationType = "missing_primitive"
	// 单个文件超过 300 行
	ViolationFileTooLong ViolationType = "file_too_long"
	// L0-L3 中引入外部第三方库
	ViolationThirdPartyInLowerLayer ViolationType = "third_party_in_lower_layer"
	// 直接 fmt.Println 而不经过 Observation Pipeline
	ViolationRawPrintln ViolationType = "raw_println"
)

// Violation 表示一条架构违规记录。
// 由 Validator Port 产生，作为分析流水线的输出之一。
type Violation struct {
	Type       ViolationType     `json:"type"`
	Severity   ViolationSeverity `json:"severity"`
	File       string            `json:"file"`
	Message    string            `json:"message"`    // 简短描述
	Detail     string            `json:"detail"`     // 详细说明（可含建议）
	Suggestion string            `json:"suggestion"` // 修复建议
}

// ──────────────────────────────────────────────
// 层级元信息
// ──────────────────────────────────────────────

// LayerInfo 描述单个架构层级的元信息。
// 用于 analyzer 中的层级分类与 violation 报告。
type LayerInfo struct {
	Layer string // "L0" ~ "L7"
	Name  string // 可读名称
	Color string // 可视化颜色
}

// GetLayerInfo 根据文件名获取层级信息。
// 文件名匹配规则来自 arch-manager 的 fileLayerMap。
func GetLayerInfo(filename string) LayerInfo {
	for pattern, info := range standardLayers {
		if matchLayer(filename, pattern) {
			return info
		}
	}
	return LayerInfo{"L7", "应用层", "#f59e0b"}
}

// standardLayers 是标准层级分类表。
var standardLayers = map[string]LayerInfo{
	"errors":                 {"L0", "错误处理", "#7f8ea3"},
	"perf_":                  {"L0", "性能基础设施", "#7f8ea3"},
	"fastpath":               {"L0", "快速路径", "#7f8ea3"},
	"atom":                   {"L1", "四原语定义", "#00d4aa"},
	"port.go":                {"L1", "四原语定义", "#00d4aa"},
	"port_":                  {"L1", "四原语定义", "#00d4aa"},
	"adapter.go":             {"L1", "四原语定义", "#00d4aa"},
	"composer":               {"L1", "四原语定义", "#00d4aa"},
	"step.go":                {"L1", "四原语定义", "#00d4aa"},
	"execution_step":         {"L1", "四原语定义", "#00d4aa"},
	"types":                  {"L1", "四原语定义", "#00d4aa"},
	"parser":                 {"L1", "四原语定义", "#00d4aa"},
	"analyzer":               {"L1", "四原语定义", "#00d4aa"},
	"validator":              {"L1", "四原语定义", "#00d4aa"},
	"generator":              {"L1", "四原语定义", "#00d4aa"},
	"renderer":               {"L1", "四原语定义", "#00d4aa"},
	"pipeline":               {"L1", "四原语定义", "#00d4aa"},
	"patterns_retry":         {"L2", "单机韧性", "#60a5fa"},
	"patterns_backoff":       {"L2", "单机韧性", "#60a5fa"},
	"patterns_circuit":       {"L2", "单机韧性", "#60a5fa"},
	"patterns_rate_limiter":  {"L2", "单机韧性", "#60a5fa"},
	"patterns_token_bucket":  {"L2", "单机韧性", "#60a5fa"},
	"patterns_fallback":      {"L2", "单机韧性", "#60a5fa"},
	"patterns_resilience":    {"L2", "单机韧性", "#60a5fa"},
	"degradation":            {"L2", "单机韧性", "#60a5fa"},
	"patterns_distributed":   {"L3", "分布式韧性", "#38bdf8"},
	"patterns_health_dist":   {"L3", "分布式韧性", "#38bdf8"},
	"patterns_degradation":   {"L3", "分布式韧性", "#38bdf8"},
	"perf_sharded":           {"L3", "分布式韧性", "#38bdf8"},
	"perf_traceid":           {"L3", "分布式韧性", "#38bdf8"},
	"perf_uuid":              {"L3", "分布式韧性", "#38bdf8"},
	"perf_tdigest":           {"L3", "分布式韧性", "#38bdf8"},
	"perf_anomaly":           {"L3", "分布式韧性", "#38bdf8"},
	"sharded":                {"L3", "分布式韧性", "#38bdf8"},
	"transaction":            {"L3", "分布式韧性", "#38bdf8"},
	"guardian":               {"L4", "Guardian 监督", "#ef4444"},
	"observation":            {"L5", "Observation 可观测", "#34d399"},
	"observability":          {"L5", "Observation 可观测", "#34d399"},
	"observer":               {"L5", "Observation 可观测", "#34d399"},
	"complexity_profile":     {"L5", "Observation 可观测", "#34d399"},
	"eventstore":             {"L6", "EventStore 事件溯源", "#f472b6"},
	"eventbus":               {"L6", "EventStore 事件溯源", "#f472b6"},
	"projection":             {"L6", "EventStore 事件溯源", "#f472b6"},
	"schema":                 {"L6", "EventStore 事件溯源", "#f472b6"},
	"idempotent":             {"L6", "EventStore 事件溯源", "#f472b6"},
	"tenant":                 {"L6", "EventStore 事件溯源", "#f472b6"},
	"config":                 {"L7", "应用层", "#f59e0b"},
	"handoff":                {"L7", "应用层", "#f59e0b"},
	"scheduler":              {"L7", "应用层", "#f59e0b"},
	"security":               {"L7", "应用层", "#f59e0b"},
	"architecture_registry":  {"L7", "应用层", "#f59e0b"},
	"entropy_metrics":        {"L7", "应用层", "#f59e0b"},
	"auto_detect":            {"L7", "应用层", "#f59e0b"},
	"migration":              {"L7", "应用层", "#f59e0b"},
	"remote_composer":        {"L7", "应用层", "#f59e0b"},
	"constraint":             {"L7", "应用层", "#f59e0b"},
	"tier_":                  {"L7", "应用层", "#f59e0b"},
	"storage":                {"L7", "应用层", "#f59e0b"},
	"understand":             {"L7", "应用层", "#f59e0b"},
	"agent_":                 {"L7", "应用层", "#f59e0b"},
	"version":                {"L7", "应用层", "#f59e0b"},
	"app.go":                 {"L7", "应用层", "#f59e0b"},
	"kg_":                    {"L7", "应用层", "#f59e0b"},
	"graph_":                 {"L7", "应用层", "#f59e0b"},
}

// matchLayer 检查文件名是否匹配某一层的模式（前缀匹配）。
func matchLayer(filename, pattern string) bool {
	if len(filename) < len(pattern) {
		return false
	}
	return filename[:len(pattern)] == pattern
}

// ──────────────────────────────────────────────
// 健康度评分
// ──────────────────────────────────────────────

// HealthScore 架构健康度综合评分（0.0 ~ 1.0）。
type HealthScore struct {
	Overall     float64            `json:"overall"`
	Grade       string             `json:"grade"` // "A" | "B" | "C" | "D"
	Factors     map[string]float64 `json:"factors"`
	Violations  int                `json:"violations"`
	Suggestions []string           `json:"suggestions,omitempty"`
}

// ComputeGrade 将 0~1 的总分映射到等级。
func ComputeGrade(overall float64) string {
	switch {
	case overall >= 0.90:
		return "A"
	case overall >= 0.75:
		return "B"
	case overall >= 0.60:
		return "C"
	default:
		return "D"
	}
}
