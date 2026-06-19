// Package core — HealthChecker 健康检查接口 (v0.9.0)

package core

import (
	"context"
	"time"
)

// HealthChecker 后端健康检查接口。
// 所有 I/O 后端（Storage、EventStore、Database）都应实现此接口。
type HealthChecker interface {
	// HealthCheck 检查后端连接健康状态。
	// 返回 nil 表示健康，否则返回错误描述。
	HealthCheck(ctx context.Context) error
}

// HealthStatus 返回健康检查结果。
type HealthStatus struct {
	Healthy   bool              `json:"healthy"`
	Timestamp time.Time         `json:"timestamp"`
	Services  map[string]string `json:"services"`
}

// CheckAll 检查所有后端并返回汇总状态。
// services 为 name -> HealthChecker 的映射。
func CheckAll(ctx context.Context, services map[string]HealthChecker) HealthStatus {
	status := HealthStatus{
		Healthy:   true,
		Timestamp: time.Now(),
		Services:  make(map[string]string, len(services)),
	}

	for name, checker := range services {
		if err := checker.HealthCheck(ctx); err != nil {
			status.Services[name] = "unhealthy: " + err.Error()
			status.Healthy = false
		} else {
			status.Services[name] = "healthy"
		}
	}

	return status
}

// StartPeriodicHealthCheck 启动周期性健康检查。
// 每次检查失败时调用 onUnhealthy 回调。
// ctx 取消时停止检查。
func StartPeriodicHealthCheck(ctx context.Context, checker HealthChecker, interval time.Duration, onUnhealthy func(error)) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := checker.HealthCheck(ctx); err != nil && onUnhealthy != nil {
				onUnhealthy(err)
			}
		}
	}
}