// Package core — Slog 结构化日志适配器 (v0.9.0)
//
// 将 Go 标准库 log/slog 适配为框架 Logger 接口。
// 建立 ErrorCategory 与 slog.Level 的映射关系。
// 自动从 Context 提取 TraceID/SpanID 注入到日志属性。

package core

import (
	"context"
	"log/slog"
	"os"
)

// ============================================================================
// SlogLogger — slog.Logger 的框架适配器
// ============================================================================

// SlogLogger 将 *slog.Logger 包装为框架 Logger 接口。
type SlogLogger struct {
	logger *slog.Logger
}

// NewSlogLogger 创建 SlogLogger。
// 默认使用 JSON 格式输出到 stdout，Level 为 Info。
func NewSlogLogger(name string) *SlogLogger {
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	return &SlogLogger{
		logger: slog.New(handler).With("logger", name),
	}
}

// NewSlogLoggerWithHandler 使用自定义 Handler 创建 SlogLogger。
func NewSlogLoggerWithHandler(handler slog.Handler, name string) *SlogLogger {
	return &SlogLogger{
		logger: slog.New(handler).With("logger", name),
	}
}

// NewTextSlogLogger 创建 Text 格式输出（适合开发环境）。
func NewTextSlogLogger(name string) *SlogLogger {
	handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})
	return &SlogLogger{
		logger: slog.New(handler).With("logger", name),
	}
}

// DebugContext 记录 Debug 级别日志。
func (l *SlogLogger) DebugContext(ctx context.Context, msg string, args ...any) {
	l.log(ctx, slog.LevelDebug, msg, args...)
}

// InfoContext 记录 Info 级别日志。
func (l *SlogLogger) InfoContext(ctx context.Context, msg string, args ...any) {
	l.log(ctx, slog.LevelInfo, msg, args...)
}

// WarnContext 记录 Warn 级别日志。
func (l *SlogLogger) WarnContext(ctx context.Context, msg string, args ...any) {
	l.log(ctx, slog.LevelWarn, msg, args...)
}

// ErrorContext 记录 Error 级别日志。
func (l *SlogLogger) ErrorContext(ctx context.Context, msg string, args ...any) {
	l.log(ctx, slog.LevelError, msg, args...)
}

// With 创建带额外属性的子 Logger。
func (l *SlogLogger) With(args ...any) Logger {
	return &SlogLogger{
		logger: l.logger.With(args...),
	}
}

// log 内部日志方法，从 Context 提取 TraceID/SpanID。
func (l *SlogLogger) log(ctx context.Context, level slog.Level, msg string, args ...any) {
	if !l.logger.Enabled(ctx, level) {
		return
	}

	// 从 Context 提取 TraceID 和 SpanID
	finalArgs := make([]any, 0, len(args)+4)
	if traceID := TraceIDFromContext(ctx); traceID != "" {
		finalArgs = append(finalArgs, "trace_id", traceID)
	}
	if spanID := SpanIDFromContext(ctx); spanID != "" {
		finalArgs = append(finalArgs, "span_id", spanID)
	}
	finalArgs = append(finalArgs, args...)

	l.logger.Log(ctx, level, msg, finalArgs...)
}

// ============================================================================
// ErrorCategory → slog.Level 映射
// ============================================================================

// ErrorCategoryToSlogLevel 将框架 ErrorCategory 映射为 slog.Level。
func ErrorCategoryToSlogLevel(cat ErrorCategory) slog.Level {
	switch cat {
	case CatDebug:
		return slog.LevelDebug
	case CatInfo, CatSuccess:
		return slog.LevelInfo
	case CatWarning:
		return slog.LevelWarn
	case CatError:
		return slog.LevelError
	case CatFatal:
		return slog.LevelError + 4 // 比 Error 高 4 级
	default:
		return slog.LevelInfo
	}
}

// SlogLevelToErrorCategory 将 slog.Level 映射为框架 ErrorCategory。
func SlogLevelToErrorCategory(level slog.Level) ErrorCategory {
	switch {
	case level < slog.LevelDebug:
		return CatDebug
	case level < slog.LevelInfo:
		return CatDebug
	case level < slog.LevelWarn:
		return CatInfo
	case level < slog.LevelError:
		return CatWarning
	default:
		return CatError
	}
}

// ============================================================================
// Context 键值 — TraceID / SpanID 传播
// ============================================================================

type contextKey string

const (
	ctxKeyTraceID contextKey = "lc_trace_id"
	ctxKeySpanID  contextKey = "lc_span_id"
)

// ContextWithTraceID 将 TraceID 注入 Context。
func ContextWithTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, ctxKeyTraceID, traceID)
}

// TraceIDFromContext 从 Context 提取 TraceID。
func TraceIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(ctxKeyTraceID).(string); ok {
		return v
	}
	return ""
}

// ContextWithSpanID 将 SpanID 注入 Context。
func ContextWithSpanID(ctx context.Context, spanID string) context.Context {
	return context.WithValue(ctx, ctxKeySpanID, spanID)
}

// SpanIDFromContext 从 Context 提取 SpanID。
func SpanIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(ctxKeySpanID).(string); ok {
		return v
	}
	return ""
}