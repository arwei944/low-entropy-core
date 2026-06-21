package main

import (
	"time"

	core "low-entropy-core/go-core"
)

// currentVersion 返回当前版本号（从 go.mod 或默认值）
func currentVersion() string { return "0.10.0" }

// listVersions 返回所有已知版本号
func listVersions() []core.VersionInfo {
	now := time.Now().Format(time.RFC3339)
	return []core.VersionInfo{
		{Version: "0.10.0", Timestamp: now, Files: 108, Lines: 28629, Symbols: 1628, ChangelogLen: 0},
		{Version: "0.9.0", Timestamp: now, Files: 14, Lines: 3840, Symbols: 0, ChangelogLen: 0},
		{Version: "0.8.0", Timestamp: now, Files: 12, Lines: 3200, Symbols: 0, ChangelogLen: 0},
		{Version: "0.7.0", Timestamp: now, Files: 10, Lines: 2800, Symbols: 0, ChangelogLen: 0},
		{Version: "0.6.0", Timestamp: now, Files: 8, Lines: 2400, Symbols: 0, ChangelogLen: 0},
		{Version: "0.5.0", Timestamp: now, Files: 6, Lines: 2000, Symbols: 0, ChangelogLen: 0},
	}
}

func getBuiltinChangelog(version string) []map[string]interface{} {
	all := []map[string]interface{}{
		{"type": "feat", "scope": "core", "message": "四原语架构：Atom/Port/Adapter/Composer 统一计算模型"},
		{"type": "feat", "scope": "core", "message": "渐进复杂度模型：8 级 Build Tag (L0-L7) 按需编译"},
		{"type": "feat", "scope": "composer", "message": "Pipeline/Branch/Parallel/Retry/Timeout/Stream/FanOut 编排模式"},
		{"type": "feat", "scope": "resilience", "message": "熔断器、限流器、退避重试、超时控制"},
		{"type": "feat", "scope": "guardian", "message": "Guardian 熵管理层：EntropyWatcher、DecisionEngine、HealthChecker"},
		{"type": "feat", "scope": "observation", "message": "Observation 观测层：ExecutionStep 追踪、Pipeline 指标采集"},
		{"type": "feat", "scope": "eventstore", "message": "EventStore 事件溯源：投影、升级、EventBus 发布订阅"},
		{"type": "feat", "scope": "observability", "message": "ObservabilityProvider 接口：Tracer/Span/Meter/Logger 统一抽象"},
		{"type": "feat", "scope": "security", "message": "安全模块：JWT 认证、RBAC 权限、API Key 管理"},
		{"type": "feat", "scope": "storage", "message": "数据库适配器：PostgreSQL (pgx) + Redis，Build Tag 隔离"},
		{"type": "feat", "scope": "config", "message": "增强配置：JSON 加载、密钥解析、热重载"},
		{"type": "feat", "scope": "errors", "message": "RichError：分类、堆栈追踪、HTTP/gRPC 状态码映射"},
		{"type": "refactor", "scope": "core", "message": "go.mod 零外部依赖：仅 require go 1.22"},
		{"type": "fix", "scope": "resilience", "message": "修复滑动窗口熔断器 HalfOpen 状态转换边界条件"},
	}
	if version == "0.8.0" {
		return all[:8]
	}
	return all
}

func getBuiltinADRs() []core.ADR {
	now := time.Now()
	return []core.ADR{
		{
			ID: "ADR-0001", Title: "Provider 模式实现可观测性",
			Status: core.ADRStatusAccepted, Version: "0.9.0", Date: now,
			Context: "框架需要统一的可观测性接口，但 L0 内核不能依赖任何具体的日志/追踪库。",
			Decision: "采用 Provider 模式：定义 ObservabilityProvider 接口，默认 NoOp 实现零开销。通过依赖注入，用户在应用层注入具体实现。",
			Consequences: "优点：L0 内核零依赖，Provider 可替换。缺点：接口抽象层增加了少量代码。",
		},
		{
			ID: "ADR-0002", Title: "Build Tag 隔离外部数据库依赖",
			Status: core.ADRStatusAccepted, Version: "0.9.0", Date: now,
			Context: "PostgreSQL 和 Redis 适配器需要引入外部依赖（pgx、go-redis），这会破坏 go.mod 的零依赖承诺。",
			Decision: "使用独立 Build Tag（lecore_pgx、lecore_redis）隔离数据库适配器。用户需显式 go build -tags lecore_pgx 启用。",
			Consequences: "优点：go.mod 保持零 require，按需编译。缺点：用户需了解 Build Tag 机制，IDE 支持可能不完整。",
		},
		{
			ID: "ADR-0003", Title: "四原语作为唯一计算模型",
			Status: core.ADRStatusAccepted, Version: "0.5.0", Date: now,
			Context: "需要一种统一的抽象来表达所有业务逻辑，避免框架中出现多种计算范式。",
			Decision: "所有计算必须通过 Atom（纯函数）、Port（验证）、Adapter（副作用）、Composer（编排）四种原语实现。",
			Consequences: "优点：强制边界隔离，易于测试和推理。缺点：学习曲线较陡，简单操作也需要包装成原语。",
		},
		{
			ID: "ADR-0004", Title: "渐进复杂度模型（8 级 Build Tag）",
			Status: core.ADRStatusAccepted, Version: "0.5.0", Date: now,
			Context: "不同项目对框架复杂度需求不同，Prototype 项目不应编译企业级功能。",
			Decision: "采用 8 级 Build Tag（L0-L7）实现渐进复杂度，每级引入特定功能，按需编译。",
			Consequences: "优点：Prototype 项目编译体积小、启动快。缺点：层级划分需要持续维护，跨层级调用受限。",
		},
	}
}

func getBuiltinCommitAnalyze() map[string]interface{} {
	return map[string]interface{}{
		"commits": []map[string]string{
			{"type": "feat", "scope": "observability", "description": "ObservabilityProvider 接口：Tracer/Span/Meter/Logger 统一抽象"},
			{"type": "feat", "scope": "security", "description": "安全模块：JWT 认证、RBAC 权限、API Key 管理"},
			{"type": "feat", "scope": "storage", "description": "数据库适配器：PostgreSQL + Redis，Build Tag 隔离"},
			{"type": "feat", "scope": "config", "description": "增强配置：JSON 加载、密钥解析、热重载"},
			{"type": "feat", "scope": "errors", "description": "RichError：分类、堆栈追踪、HTTP/gRPC 状态码映射"},
			{"type": "feat", "scope": "resilience", "description": "增强韧性：滑动窗口熔断器、令牌桶限流器"},
			{"type": "fix", "scope": "resilience", "description": "修复熔断器 HalfOpen 状态转换边界条件"},
			{"type": "refactor", "scope": "core", "description": "go.mod 零外部依赖：仅 require go 1.22"},
		},
		"total": 8,
		"classification": map[string]int{
			"feat": 6, "fix": 1, "refactor": 1,
		},
		"bump":         "minor",
		"current":      currentVersion(),
		"next_version": "0.10.0",
	}
}

func getBuiltinArchChanges() []core.ChangeIntent {
	now := time.Now()
	return []core.ChangeIntent{
		{ID: "CHG-001", Title: "为 v0.9.0 增强模块添加 Build Tag 隔离", Type: "refactor", Scope: "core", Description: "将 security_*.go、observability*.go、errors_enhanced.go 等纳入 Build Tag 体系", Breaking: false, CreatedAt: now},
		{ID: "CHG-002", Title: "消除 patterns_resilience 功能重复", Type: "refactor", Scope: "resilience", Description: "合并 patterns_resilience_enhanced.go 到 patterns_resilience.go", Breaking: true, Migration: "迁移到泛型 RateLimiter[T] 和 CircuitBreaker[T]", CreatedAt: now},
		{ID: "CHG-003", Title: "统一版本号管理", Type: "fix", Scope: "config", Description: "将 go.mod、DefaultAppConfig().Version 和架构管理器快照版本号对齐", Breaking: false, CreatedAt: now},
		{ID: "CHG-004", Title: "拆分 AppConfig 概念泄漏", Type: "refactor", Scope: "config", Description: "将 PostgresDSN、RedisAddr、JWTSecret 等字段移入对应 Build Tag 文件", Breaking: true, Migration: "使用 Extensions map 替代直接字段访问", CreatedAt: now},
	}
}

// getString 从 map 中安全获取字符串值
func getString(m map[string]interface{}, key string, defaultVal string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return defaultVal
}

// getBool 从 map 中安全获取布尔值
func getBool(m map[string]interface{}, key string, defaultVal bool) bool {
	if v, ok := m[key]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return defaultVal
}
