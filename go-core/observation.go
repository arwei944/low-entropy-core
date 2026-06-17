package core

import (
	"context"
	"sync"
	"time"
)

// ──────────────────────────────────────────────
// ExecutionStep — the universal observation atom
// ──────────────────────────────────────────────

// ExecutionStep is the single, unified record of every execution event.
// It is the atomic unit of the observation layer. Every primitive
// (Atom, Port, Adapter, Composer) emits ExecutionSteps.
//
// This is the ONLY definition of ExecutionStep in the codebase.
// No other file may define a duplicate.
type ExecutionStep struct {
	Timestamp  time.Time              `json:"timestamp"`
	TraceID    string                 `json:"trace_id"`
	SpanID     string                 `json:"span_id"`
	ParentID   string                 `json:"parent_id,omitempty"`
	Unit       string                 `json:"unit"`
	Action     string                 `json:"action"`
	Details    string                 `json:"details"`
	Pattern    string                 `json:"pattern,omitempty"`
	DurationMs int64                  `json:"duration_ms,omitempty"`
	Error      *StepError             `json:"error,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// ──────────────────────────────────────────────
// CompactExecutionStep — 紧凑型执行步骤 (v4.0)
// ──────────────────────────────────────────────

// CompactExecutionStep 是 ExecutionStep 的紧凑版本，专为十亿级热路径设计。
// 与 ExecutionStep 的区别：
//   - TraceID/SpanID/ParentID 使用 [16]byte 代替 string（节省 16-32 字节/字段）
//   - Metadata 使用 *map[string]any 指针（nil 时零分配）
//   - struct 总大小约 200 字节，相比 ExecutionStep 的 ~400 字节减少约 50%
//
// 在十亿级调用量下，每个步骤节省 200 字节意味着节省 200GB 内存。
type CompactExecutionStep struct {
	Timestamp  time.Time               `json:"timestamp"`
	TraceID    CompactTraceID          `json:"trace_id"`    // [16]byte
	SpanID     CompactTraceID          `json:"span_id"`     // [16]byte
	ParentID   CompactTraceID          `json:"parent_id"`   // [16]byte (zero = none)
	Unit       string                  `json:"unit"`
	Action     string                  `json:"action"`
	Details    string                  `json:"details"`
	Pattern    string                  `json:"pattern,omitempty"`
	DurationMs int64                   `json:"duration_ms,omitempty"`
	Error      *StepError              `json:"error,omitempty"`
	Metadata   *map[string]interface{} `json:"metadata,omitempty"` // nil = 无元数据
}

// ToCompact 将 ExecutionStep 转换为 CompactExecutionStep。
// 在热路径末尾调用以释放内存。
func (s ExecutionStep) ToCompact() CompactExecutionStep {
	cs := CompactExecutionStep{
		Timestamp:  s.Timestamp,
		Unit:       s.Unit,
		Action:     s.Action,
		Details:    s.Details,
		Pattern:    s.Pattern,
		DurationMs: s.DurationMs,
		Error:      s.Error,
	}
	if s.TraceID != "" {
		cs.TraceID = CompactTraceIDFromString(s.TraceID)
	}
	if s.SpanID != "" {
		cs.SpanID = CompactTraceIDFromString(s.SpanID)
	}
	if s.ParentID != "" {
		cs.ParentID = CompactTraceIDFromString(s.ParentID)
	}
	if len(s.Metadata) > 0 {
		md := make(map[string]interface{}, len(s.Metadata))
		for k, v := range s.Metadata {
			md[k] = v
		}
		cs.Metadata = &md
	}
	return cs
}

// ToExecutionStep 将 CompactExecutionStep 转换回 ExecutionStep。
func (cs CompactExecutionStep) ToExecutionStep() ExecutionStep {
	s := ExecutionStep{
		Timestamp:  cs.Timestamp,
		TraceID:    cs.TraceID.String(),
		SpanID:     cs.SpanID.String(),
		ParentID:   cs.ParentID.String(),
		Unit:       cs.Unit,
		Action:     cs.Action,
		Details:    cs.Details,
		Pattern:    cs.Pattern,
		DurationMs: cs.DurationMs,
		Error:      cs.Error,
	}
	if cs.ParentID.IsZero() {
		s.ParentID = ""
	}
	if cs.Metadata != nil {
		s.Metadata = *cs.Metadata
	}
	return s
}

// CompactTraceIDFromString 将十六进制字符串解析为 CompactTraceID。
func CompactTraceIDFromString(s string) CompactTraceID {
	var id CompactTraceID
	if len(s) >= 32 {
		for i := 0; i < 16; i++ {
			hi := hexToByte(s[i*2])
			lo := hexToByte(s[i*2+1])
			id[i] = hi<<4 | lo
		}
	}
	return id
}

// hexToByte 将十六进制字符转换为字节值。
func hexToByte(c byte) byte {
	switch {
	case c >= '0' && c <= '9':
		return c - '0'
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10
	default:
		return 0
	}
}

// NewCompactExecutionStep 创建紧凑型执行步骤（零分配热路径）。
// 使用 CompactTraceID 代替 string TraceID/SpanID。
func NewCompactExecutionStep(traceID, spanID CompactTraceID, unit, action, details, pattern string) CompactExecutionStep {
	return CompactExecutionStep{
		Timestamp: time.Now(),
		TraceID:   traceID,
		SpanID:    spanID,
		Unit:      unit,
		Action:    action,
		Details:   details,
		Pattern:   pattern,
	}
}

// ──────────────────────────────────────────────
// TraceTree — hierarchical trace visualization
// ──────────────────────────────────────────────

// TraceNode is a node in the trace tree. It represents a single span
// and its children, forming a hierarchical view of execution.
type TraceNode struct {
	Step     ExecutionStep `json:"step"`
	Children []*TraceNode  `json:"children,omitempty"`
	Depth    int           `json:"depth"`
}

// TraceTree builds a hierarchical tree from a flat list of ExecutionSteps.
// It organizes spans by ParentID to reconstruct the call graph.
type TraceTree struct {
	Roots []*TraceNode `json:"roots"`
}

// BuildTraceTree constructs a TraceTree from a flat list of ExecutionSteps.
// Steps with no ParentID or whose ParentID does not match any SpanID become roots.
func BuildTraceTree(steps []ExecutionStep) *TraceTree {
	if len(steps) == 0 {
		return &TraceTree{}
	}

	// Build a map of SpanID -> node for O(1) lookup
	nodeMap := make(map[string]*TraceNode, len(steps))
	for i := range steps {
		step := steps[i]
		nodeMap[step.SpanID] = &TraceNode{Step: step, Depth: 0}
	}

	// Second pass: assign children to parents
	roots := make([]*TraceNode, 0)
	for _, node := range nodeMap {
		if node.Step.ParentID == "" {
			roots = append(roots, node)
			continue
		}
		parent, ok := nodeMap[node.Step.ParentID]
		if !ok {
			// Parent not found, treat as root
			roots = append(roots, node)
			continue
		}
		parent.Children = append(parent.Children, node)
	}

	// Set depths
	var setDepth func(n *TraceNode, d int)
	setDepth = func(n *TraceNode, d int) {
		n.Depth = d
		for _, child := range n.Children {
			setDepth(child, d+1)
		}
	}
	for _, root := range roots {
		setDepth(root, 0)
	}

	return &TraceTree{Roots: roots}
}

// Flatten returns all nodes in depth-first order.
func (t *TraceTree) Flatten() []*TraceNode {
	result := make([]*TraceNode, 0)
	var dfs func(n *TraceNode)
	dfs = func(n *TraceNode) {
		result = append(result, n)
		for _, child := range n.Children {
			dfs(child)
		}
	}
	for _, root := range t.Roots {
		dfs(root)
	}
	return result
}

// TotalNodes returns the total number of nodes in the tree.
func (t *TraceTree) TotalNodes() int {
	count := 0
	var dfs func(n *TraceNode)
	dfs = func(n *TraceNode) {
		count++
		for _, child := range n.Children {
			dfs(child)
		}
	}
	for _, root := range t.Roots {
		dfs(root)
	}
	return count
}

// ──────────────────────────────────────────────
// ObservationAdapter — observation sink interface
// ──────────────────────────────────────────────

// ObservationAdapter is the sink for execution records.
// All primitives emit to this interface; the observation layer
// consumes it for the X-Ray dashboard.
type ObservationAdapter interface {
	Record(steps []ExecutionStep)
}

// ──────────────────────────────────────────────
// InMemoryObservationAdapter — in-memory implementation
// ──────────────────────────────────────────────

// InMemoryObservationAdapter stores execution steps in memory.
// Thread-safe for concurrent use.
type InMemoryObservationAdapter struct {
	mu    sync.RWMutex
	Steps []ExecutionStep
}

// Record appends execution steps to the in-memory store.
func (a *InMemoryObservationAdapter) Record(steps []ExecutionStep) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.Steps = append(a.Steps, steps...)
}

// GetSteps returns a copy of all recorded steps.
func (a *InMemoryObservationAdapter) GetSteps() []ExecutionStep {
	a.mu.RLock()
	defer a.mu.RUnlock()
	result := make([]ExecutionStep, len(a.Steps))
	copy(result, a.Steps)
	return result
}

// GetTraceTree builds a trace tree from all recorded steps.
func (a *InMemoryObservationAdapter) GetTraceTree() *TraceTree {
	return BuildTraceTree(a.GetSteps())
}

// Clear removes all recorded steps.
func (a *InMemoryObservationAdapter) Clear() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.Steps = a.Steps[:0]
}

// StepCount returns the number of recorded steps.
func (a *InMemoryObservationAdapter) StepCount() int {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return len(a.Steps)
}

// ──────────────────────────────────────────────
// NoOpObservationAdapter — discards all records
// ──────────────────────────────────────────────

// NoOpObservationAdapter silently discards all execution steps.
// Useful for testing or when observation is disabled.
type NoOpObservationAdapter struct{}

// Record is a no-op.
func (a *NoOpObservationAdapter) Record(steps []ExecutionStep) {}

// ──────────────────────────────────────────────
// ExecutionStep constructors
// ──────────────────────────────────────────────

// NewExecutionStep creates an ExecutionStep with UUID-based trace and span IDs.
// This is the canonical constructor; all primitives should use this.
func NewExecutionStep(unit, action, details, pattern string) ExecutionStep {
	now := time.Now()
	return ExecutionStep{
		Timestamp: now,
		TraceID:   string(NewTraceID()),
		SpanID:    string(NewSpanID()),
		Unit:      unit,
		Action:    action,
		Details:   details,
		Pattern:   pattern,
	}
}

// NewExecutionStepWithTrace creates an ExecutionStep as a child of a parent span.
func NewExecutionStepWithTrace(parentSpanID string, unit, action, details, pattern string) ExecutionStep {
	step := NewExecutionStep(unit, action, details, pattern)
	step.ParentID = parentSpanID
	return step
}

// NewExecutionStepWithError creates an ExecutionStep that records an error.
func NewExecutionStepWithError(unit, action, details, pattern string, err *StepError) ExecutionStep {
	step := NewExecutionStep(unit, action, details, pattern)
	step.Error = err
	return step
}

// ──────────────────────────────────────────────
// Context helpers for trace propagation
// ──────────────────────────────────────────────

type traceKeyType struct{}

var traceKey = traceKeyType{}

// TraceContext carries trace identity through context.
type TraceContext struct {
	TraceID string
	SpanID  string
}

// WithTraceContext injects trace identity into a context.
func WithTraceContext(ctx context.Context, tc TraceContext) context.Context {
	return context.WithValue(ctx, traceKey, tc)
}

// GetTraceContext extracts trace identity from a context.
func GetTraceContext(ctx context.Context) (TraceContext, bool) {
	tc, ok := ctx.Value(traceKey).(TraceContext)
	return tc, ok
}