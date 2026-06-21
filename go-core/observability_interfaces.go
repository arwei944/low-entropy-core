// Package core — Observability 可观测性核心接口 (v0.9.0)
//
// 设计原则:
//   - 框架零依赖: 所有接口定义不导入任何外部库
//   - Provider 模式: 用户注入具体实现，框架只依赖接口
//   - 三信号统一: Tracing + Metrics + Logging 通过 Context 关联
//   - 零开销回退: 未注入 Provider 时，所有埋点跳过（NoOp 实现）
//
// 架构:
//   ObservabilityProvider
//   ├── TracerProvider  → Tracer → Span  (Tracing)
//   ├── MeterProvider   → Meter  → Counter/Histogram/Gauge  (Metrics)
//   └── LoggerProvider  → Logger  (Logging, 基于 slog)

package core

import (
	"context"
	"time"
)

// ============================================================================
// SECTION 1: Trace 追踪接口
// ============================================================================

// Span 表示一个执行单元（对应一个 Step 或 Pipeline）。
// 与 OpenTelemetry Span API 对齐，但框架不依赖 OTel。
type Span interface {
	// End 标记 Span 结束。必须在 defer 中调用。
	End()

	// SetAttributes 设置 Span 属性（键值对）。
	SetAttributes(attrs ...KeyValue)

	// RecordError 记录错误。
	RecordError(err error)

	// SetStatus 设置 Span 状态。
	SetStatus(code StatusCode, description string)

	// AddEvent 添加事件（带时间戳的日志）。
	AddEvent(name string, attrs ...KeyValue)

	// IsRecording 返回是否正在记录（用于避免不必要的计算）。
	IsRecording() bool
}

// Tracer 创建 Span 的工厂接口。
// 通过 Context 传播实现 Span 父子关系。
type Tracer interface {
	// Start 创建一个新的 Span。如果 ctx 包含父 Span，新 Span 自动成为子 Span。
	// 返回新的 context（包含新 Span）和 Span 对象。
	Start(ctx context.Context, name string, opts ...SpanOption) (context.Context, Span)
}

// TracerProvider 创建 Tracer 的工厂接口。
type TracerProvider interface {
	// Tracer 返回指定名称的 Tracer。
	Tracer(name string, opts ...TracerOption) Tracer

	// Shutdown 优雅关闭，刷新所有待发送的 Span。
	Shutdown(ctx context.Context) error
}

// ============================================================================
// SECTION 2: Metrics 指标接口
// ============================================================================

// Meter 创建指标对象的工厂接口。
// 与 OpenTelemetry Metrics API 对齐。
type Meter interface {
	// NewCounter 创建一个计数器（只增不减）。
	// 适用场景: 请求计数、错误计数、任务完成数。
	NewCounter(name, description string, opts ...MetricOption) (Counter, error)

	// NewHistogram 创建一个直方图（观测值分布）。
	// 适用场景: 请求延迟、响应大小、Step 耗时。
	NewHistogram(name, description string, buckets []float64, opts ...MetricOption) (Histogram, error)

	// NewGauge 创建一个仪表盘（可增可减）。
	// 适用场景: 当前连接数、队列长度、内存使用。
	NewGauge(name, description string, opts ...MetricOption) (Gauge, error)
}

// Counter 只增不减的计数器。
type Counter interface {
	// Add 增加计数。
	Add(ctx context.Context, value float64, attrs ...KeyValue)
}

// Histogram 观测值分布记录器。
type Histogram interface {
	// Record 记录一个观测值。
	Record(ctx context.Context, value float64, attrs ...KeyValue)
}

// Gauge 可增可减的仪表盘。
type Gauge interface {
	// Set 设置当前值。
	Set(ctx context.Context, value float64, attrs ...KeyValue)

	// Add 增加或减少值。
	Add(ctx context.Context, value float64, attrs ...KeyValue)
}

// MeterProvider 创建 Meter 的工厂接口。
type MeterProvider interface {
	// Meter 返回指定名称的 Meter。
	Meter(name string, opts ...MeterOption) Meter

	// Shutdown 优雅关闭。
	Shutdown(ctx context.Context) error
}

// ============================================================================
// SECTION 3: Logging 日志接口
// ============================================================================

// Logger 结构化日志接口。与 slog 对齐，但框架不依赖 slog。
// 通过 Context 关联 TraceID/SpanID。
type Logger interface {
	// DebugContext 记录 Debug 级别日志。
	DebugContext(ctx context.Context, msg string, args ...any)

	// InfoContext 记录 Info 级别日志。
	InfoContext(ctx context.Context, msg string, args ...any)

	// WarnContext 记录 Warn 级别日志。
	WarnContext(ctx context.Context, msg string, args ...any)

	// ErrorContext 记录 Error 级别日志。
	ErrorContext(ctx context.Context, msg string, args ...any)

	// With 创建带额外属性的子 Logger。
	With(args ...any) Logger
}

// LoggerProvider 创建 Logger 的工厂接口。
type LoggerProvider interface {
	// Logger 返回指定名称的 Logger。
	Logger(name string) Logger
}

// ============================================================================
// SECTION 4: 统一入口
// ============================================================================

// ObservabilityProvider 可观测性统一入口。
// 注入到框架后，Pipeline/Step 自动获得可观测性能力。
type ObservabilityProvider struct {
	TracerProvider TracerProvider
	MeterProvider  MeterProvider
	Logger         Logger

	// NowFunc 提供当前时间。Composer 层通过此函数获取时间，禁止直接调用 time.Now()。
	// 默认为 time.Now，可通过测试时注入 mock 实现注入。
	NowFunc func() time.Time
}

// NewObservabilityProvider 创建 ObservabilityProvider 并设置默认时间函数。
func NewObservabilityProvider() *ObservabilityProvider {
	return &ObservabilityProvider{
		NowFunc: time.Now,
	}
}

// Shutdown 关闭所有 Provider，刷新待发送数据。
func (p *ObservabilityProvider) Shutdown(ctx context.Context) error {
	var errs []error
	if p.TracerProvider != nil {
		if err := p.TracerProvider.Shutdown(ctx); err != nil {
			errs = append(errs, err)
		}
	}
	if p.MeterProvider != nil {
		if err := p.MeterProvider.Shutdown(ctx); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return errs[0]
	}
	return nil
}

// ============================================================================
// SECTION 5: 辅助类型
// ============================================================================

// StatusCode 表示 Span 状态码。
type StatusCode int

const (
	// StatusUnset 默认状态（未设置）。
	StatusUnset StatusCode = 0
	// StatusOK 成功。
	StatusOK StatusCode = 1
	// StatusError 错误。
	StatusError StatusCode = 2
)

// KeyValue 键值对，用于 Span 属性、Metric 标签、Log 字段。
type KeyValue struct {
	Key   string
	Value any
}

// NewKeyValue 创建 KeyValue 的便捷函数。
func NewKeyValue(key string, value any) KeyValue {
	return KeyValue{Key: key, Value: value}
}

// SpanOption 创建 Span 时的选项。
type SpanOption func(*spanConfig)

type spanConfig struct {
	kind       int
	attrs      []KeyValue
	startTime  time.Time
}

// WithSpanKind 设置 Span 类型。
// 0=Internal, 1=Server, 2=Client, 3=Producer, 4=Consumer
func WithSpanKind(kind int) SpanOption {
	return func(c *spanConfig) {
		c.kind = kind
	}
}

// WithSpanAttributes 设置 Span 属性。
func WithSpanAttributes(attrs ...KeyValue) SpanOption {
	return func(c *spanConfig) {
		c.attrs = append(c.attrs, attrs...)
	}
}

// WithStartTime 设置 Span 开始时间。
func WithStartTime(t time.Time) SpanOption {
	return func(c *spanConfig) {
		c.startTime = t
	}
}

// TracerOption 创建 Tracer 时的选项。
type TracerOption func(*tracerConfig)

type tracerConfig struct {
	schemaURL string
	version   string
}

// MeterOption 创建 Meter 时的选项。
type MeterOption func(*meterConfig)

type meterConfig struct {
	schemaURL string
	version   string
}

// MetricOption 创建指标时的选项。
type MetricOption func(*metricConfig)

type metricConfig struct {
	unit        string
	description string
}

// WithMetricUnit 设置指标单位。
func WithMetricUnit(unit string) MetricOption {
	return func(c *metricConfig) {
		c.unit = unit
	}
}

// WithMetricDescription 设置指标描述。
func WithMetricDescription(desc string) MetricOption {
	return func(c *metricConfig) {
		c.description = desc
	}
}
