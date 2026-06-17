# 升级文档：标准化 ExecutionStep + 通用观察协议

## 1. 背景与目标
当前 ExecutionStep 是 calculator 示例中的本地结构体，缺乏标准化。在 100 万行代码规模下，需要：
- 统一的结构化事件格式
- 支持分布式、聚合、采样
- 人类仪表盘可跨系统消费
- 低熵：不增加新原语

目标：定义 **ExecutionStep Protocol v1**，作为所有 4 单元系统必须遵守的观察契约。

## 2. 设计原则
- ExecutionStep 由 Composer 统一收集和输出
- 格式必须是纯数据（可序列化为 JSON）
- 支持分布式追踪（trace_id, span_id, parent_id）
- 必须包含足够信息让人类理解“发生了什么”，但不能泄露过多细节（低熵）
- 所有模式（见文档 01）必须产生标准步骤

## 3. 标准化 ExecutionStep 定义

```go
type ExecutionStep struct {
    Timestamp   time.Time `json:"timestamp"`
    TraceID     string    `json:"trace_id"`      // 全局追踪ID
    SpanID      string    `json:"span_id"`       // 当前步骤ID
    ParentID    string    `json:"parent_id"`     // 父步骤
    Unit        string    `json:"unit"`          // "Atom" | "Port" | "Adapter" | "Composer"
    Action      string    `json:"action"`        // 具体动作
    Details     string    `json:"details"`
    Pattern     string    `json:"pattern,omitempty"` // 如 "Branch", "Parallel", "Handoff"
    DurationMs  int64     `json:"duration_ms,omitempty"`
    Error       *StepError `json:"error,omitempty"`
    Metadata    map[string]interface{} `json:"metadata,omitempty"` // 低熵元数据
}

type StepError struct {
    Code    string `json:"code"`
    Message string `json:"message"`
    Recoverable bool `json:"recoverable"`
}
```

## 4. 通用观察协议（Observation Protocol）

### 4.1 收集方式
- Composer 在 Run 时统一收集步骤
- 每个原语调用后立即 append 步骤
- 支持流式输出（channel）或批量

### 4.2 传输方式（Adapter 负责）
- 推荐 Adapter 类型：
  - InMemoryAdapter（开发）
  - FileAdapter（持久化）
  - HTTPAdapter（推送到中央观察服务）
  - KafkaAdapter（大规模）

### 4.3 聚合与采样规则
- 采样率：默认 100%，大规模时可配置（通过 Port 注入）
- 聚合：相同 Pattern 的步骤可聚合计数
- 保留策略：最近 N 条 + 错误必留

### 4.4 仪表盘消费契约
- 所有系统必须暴露 `/observation/steps` 接口（或类似）
- 返回格式：`[]ExecutionStep`
- 支持查询参数：trace_id, unit, since, limit

## 5. 最小可执行任务分解

### 阶段 1: 核心结构定义
**任务 1.1**  
在 `go-core/` 新建 `observation.go`，定义标准 `ExecutionStep` 和 `StepError` 结构体。

**任务 1.2**  
更新 `go-core/composer.go` 的 `Run` 方法，使其返回 `[]ExecutionStep` 而非仅 interface{}。

**任务 1.3**  
修改 calculator 的 `types.go` 和 `server.go`，使用新标准结构体替换本地定义。

### 阶段 2: 协议扩展
**任务 2.1**  
为 ExecutionStep 添加 `TraceID`、`SpanID`、`ParentID` 生成逻辑（使用 uuid 或简单递增）。

**任务 2.2**  
在 Composer 中添加 `WithTraceContext` 方法，支持传入/生成 trace。

**任务 2.3**  
定义 `ObservationAdapter` 接口：
```go
type ObservationAdapter interface {
    Adapter
    Record(steps []ExecutionStep)
}
```

**任务 2.4**  
实现基础的 `InMemoryObservationAdapter` 和 `FileObservationAdapter`。

### 阶段 3: 仪表盘与 API
**任务 3.1**  
在 calculator 示例中，扩展前端实时日志，使用新字段（trace_id, duration, pattern）。

**任务 3.2**  
添加 HTTP handler `/api/observation/steps` 返回最近步骤（支持 query）。

**任务 3.3**  
更新监控面板，增加“全局视图”标签页（显示 trace 树）。

### 阶段 4: 分布式与采样
**任务 4.1**  
定义采样策略接口（通过 Port 注入）。

**任务 4.2**  
在 task_scheduler 示例中演示跨“服务”（用 goroutine 模拟）的 trace 传递。

**任务 4.3**  
添加步骤聚合示例（同一 Pattern 计数）。

**任务 4.4**  
创建 `docs/observation/` 目录，编写协议规范 + 示例 JSON。

### 阶段 5: 集成与验证
**任务 5.1**  
更新所有现有示例（calculator, task_scheduler），强制使用新协议。

**任务 5.2**  
在 README 中增加“观察协议”章节。

**任务 5.3**  
添加测试：验证步骤结构完整性 + trace 关联正确。

**任务 5.4**  
模拟 1M LOC 场景：生成 10000 条步骤，验证仪表盘渲染性能（前端节流）。

## 6. 实现约束
- 禁止在 Atom 中直接写日志，必须通过 Adapter
- Metadata 只允许少量关键字段（避免熵爆炸）
- 所有步骤必须有 TraceID

## 7. 风险与缓解
- 风险：步骤数量爆炸 → 缓解：采样 + 聚合 + 仅错误全量
- 风险：trace_id 冲突 → 缓解：使用高熵随机 + 时间前缀
- 风险：协议版本演进 → 缓解：ExecutionStep 增加 Version 字段

## 8. 验收标准
- 任意 Composer 执行后能输出符合 schema 的 []ExecutionStep
- 前端能正确渲染 trace 树和 pattern 标记
- 支持通过 Adapter 将步骤推送到外部系统

**完成此文档后，下一步**：执行任务 1.1（定义 observation.go）。
