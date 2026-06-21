package core

import (
	"time"
)

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
	Metadata   map[string]any `json:"metadata,omitempty"`
}

// CompactExecutionStep is ExecutionStep 的紧凑版本，专为十亿级热路径设计。
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
	Metadata   *map[string]any `json:"metadata,omitempty"` // nil = 无元数据
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
		md := make(map[string]any, len(s.Metadata))
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
