// Package core — HTTP 可观测性中间件 (v0.9.0)
//
// 为 HTTP 请求自动创建 Span、记录 Metrics、输出结构化访问日志。

package core

import (
	"context"
	"net/http"
	"time"
)

// HTTPMiddleware 为 HTTP Handler 添加可观测性埋点。
// 自动创建 Span、记录请求耗时、输出访问日志。
func HTTPMiddleware(handler http.Handler, provider *ObservabilityProvider) http.Handler {
	if provider == nil || provider.TracerProvider == nil {
		return handler
	}

	tracer := provider.TracerProvider.Tracer("http")
	logger := provider.Logger
	if logger == nil {
		logger = &noOpLogger{}
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// 创建 Span
		ctx, span := tracer.Start(r.Context(), r.Method+" "+r.URL.Path,
			WithSpanKind(1), // Server
			WithSpanAttributes(
				NewKeyValue("http.method", r.Method),
				NewKeyValue("http.url", r.URL.String()),
				NewKeyValue("http.user_agent", r.UserAgent()),
			),
		)
		defer span.End()

		// 包装 ResponseWriter 以捕获状态码
		wrapped := &responseWriterWrapper{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}

		// 执行请求
		handler.ServeHTTP(wrapped, r.WithContext(ctx))

		duration := time.Since(start)

		// 记录 Span 属性
		span.SetAttributes(
			NewKeyValue("http.status_code", wrapped.statusCode),
			NewKeyValue("http.duration_ms", duration.Milliseconds()),
		)

		if wrapped.statusCode >= 400 {
			span.SetStatus(StatusError, http.StatusText(wrapped.statusCode))
		} else {
			span.SetStatus(StatusOK, "")
		}

		// 输出访问日志
		logger.InfoContext(ctx, "http request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", wrapped.statusCode,
			"duration_ms", duration.Milliseconds(),
			"user_agent", r.UserAgent(),
		)
	})
}

// responseWriterWrapper 包装 http.ResponseWriter 以捕获状态码。
type responseWriterWrapper struct {
	http.ResponseWriter
	statusCode int
}

func (w *responseWriterWrapper) WriteHeader(statusCode int) {
	w.statusCode = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

// ============================================================================
// Metrics HTTP Handler
// ============================================================================

// MetricsHandler 返回 Prometheus /metrics 端点的 Handler。
// 需要用户注入 Prometheus MeterProvider 的 HTTP Handler。
// 使用方式:
//
//	mux.Handle("/metrics", core.MetricsHandler(promHandler))
func MetricsHandler(promHandler http.Handler) http.Handler {
	return promHandler
}

// ============================================================================
// 健康检查 HTTP Handler
// ============================================================================

// HealthResponse 健康检查响应。
type HealthResponse struct {
	Status    string            `json:"status"`
	Timestamp time.Time         `json:"timestamp"`
	Services  map[string]string `json:"services,omitempty"`
}

// HealthHandler 返回健康检查端点 Handler。
// checkers 为可选的后端健康检查器映射。
func HealthHandler(checkers map[string]func(context.Context) error) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := HealthResponse{
			Status:    "ok",
			Timestamp: time.Now(),
			Services:  make(map[string]string),
		}

		allHealthy := true
		for name, check := range checkers {
			if err := check(r.Context()); err != nil {
				resp.Services[name] = "unhealthy: " + err.Error()
				allHealthy = false
			} else {
				resp.Services[name] = "healthy"
			}
		}

		if !allHealthy {
			resp.Status = "degraded"
		}

		w.Header().Set("Content-Type", "application/json")
		if allHealthy {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
		// 使用简单的 JSON 输出（避免导入 encoding/json 的循环依赖风险）
		output := `{"status":"` + resp.Status + `","timestamp":"` + resp.Timestamp.Format(time.RFC3339) + `","services":{`
		first := true
		for name, status := range resp.Services {
			if !first {
				output += ","
			}
			output += `"` + name + `":"` + status + `"`
			first = false
		}
		output += `}}`
		w.Write([]byte(output))
	})
}