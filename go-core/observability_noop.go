// Package core — Observability NoOp 空操作实现 (v0.9.0)
package core

import "context"

// NewNoOpObservabilityProvider 返回一个空操作的 Provider。
// 当用户未注入可观测性时，框架使用此默认值，所有埋点零开销。
func NewNoOpObservabilityProvider() *ObservabilityProvider {
	return &ObservabilityProvider{
		TracerProvider: &noOpTracerProvider{},
		MeterProvider:  &noOpMeterProvider{},
		Logger:         &noOpLogger{},
	}
}

type noOpSpan struct{}

func (s *noOpSpan) End()                                             {}
func (s *noOpSpan) SetAttributes(attrs ...KeyValue)                  {}
func (s *noOpSpan) RecordError(err error)                            {}
func (s *noOpSpan) SetStatus(code StatusCode, desc string)           {}
func (s *noOpSpan) AddEvent(name string, attrs ...KeyValue)          {}
func (s *noOpSpan) IsRecording() bool                                { return false }

type noOpTracer struct{}

func (t *noOpTracer) Start(ctx context.Context, name string, opts ...SpanOption) (context.Context, Span) {
	return ctx, &noOpSpan{}
}

type noOpTracerProvider struct{}

func (p *noOpTracerProvider) Tracer(name string, opts ...TracerOption) Tracer {
	return &noOpTracer{}
}
func (p *noOpTracerProvider) Shutdown(ctx context.Context) error { return nil }

type noOpCounter struct{}

func (c *noOpCounter) Add(ctx context.Context, value float64, attrs ...KeyValue) {}

type noOpHistogram struct{}

func (h *noOpHistogram) Record(ctx context.Context, value float64, attrs ...KeyValue) {}

type noOpGauge struct{}

func (g *noOpGauge) Set(ctx context.Context, value float64, attrs ...KeyValue)  {}
func (g *noOpGauge) Add(ctx context.Context, value float64, attrs ...KeyValue)  {}

type noOpMeter struct{}

func (m *noOpMeter) NewCounter(name, desc string, opts ...MetricOption) (Counter, error) {
	return &noOpCounter{}, nil
}
func (m *noOpMeter) NewHistogram(name, desc string, buckets []float64, opts ...MetricOption) (Histogram, error) {
	return &noOpHistogram{}, nil
}
func (m *noOpMeter) NewGauge(name, desc string, opts ...MetricOption) (Gauge, error) {
	return &noOpGauge{}, nil
}

type noOpMeterProvider struct{}

func (p *noOpMeterProvider) Meter(name string, opts ...MeterOption) Meter {
	return &noOpMeter{}
}
func (p *noOpMeterProvider) Shutdown(ctx context.Context) error { return nil }

type noOpLogger struct{}

func (l *noOpLogger) DebugContext(ctx context.Context, msg string, args ...any) {}
func (l *noOpLogger) InfoContext(ctx context.Context, msg string, args ...any)  {}
func (l *noOpLogger) WarnContext(ctx context.Context, msg string, args ...any)  {}
func (l *noOpLogger) ErrorContext(ctx context.Context, msg string, args ...any) {}
func (l *noOpLogger) With(args ...any) Logger                                   { return l }
