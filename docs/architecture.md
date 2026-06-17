# Low-Entropy Core 架构文档

> **版本**: v2.0 | **语言**: Go 1.21+ | **核心理念**: 四原语（4-Primitive）哲学

---

## 目录

1. [概述](#1-概述)
2. [四大原语](#2-四大原语)
3. [架构分层](#3-架构分层)
4. [快速开始](#4-快速开始)
5. [设计决策](#5-设计决策)
6. [API 参考](#6-api-参考)

---

## 1. 概述

### 1.1 什么是 Low-Entropy Core

Low-Entropy Core 是一个 Go 语言编写的 Agent 编排框架，其核心思想是：**整个系统只需要四种抽象，就能表达任意复杂的 Agent 工作流**。这四种抽象被称为"四原语"（Four Primitives）。

### 1.2 四原语设计哲学

传统 Agent 框架往往引入大量概念：Tool、Skill、Plugin、Middleware、Hook、Interceptor、Pipeline、Chain、Agent、Task、Step......每一种概念都有自己的一套接口和行为，导致系统熵值急剧增加——开发者需要理解数十种抽象才能开始工作。

Low-Entropy Core 的回答是：**只用四种抽象**。

```
Atom     → 纯函数（无副作用，确定性）
Port     → 契约验证（边界守卫，数据校验）
Adapter  → 副作用隔离（I/O、存储、网络）
Composer → 编排引擎（组合、调度、协调）
```

这四种原语覆盖了所有计算场景：
- 纯逻辑用 Atom
- 边界校验用 Port
- 外部交互用 Adapter
- 流程编排用 Composer

### 1.3 为什么只需要四种

每一种原语承担一个不可替代的职责：

| 原语 | 职责 | 是否允许副作用 | 是否允许非确定性 |
|------|------|:---:|:---:|
| Atom | 纯计算 | 否 | 否 |
| Port | 契约验证 | 否 | 否 |
| Adapter | 副作用 | 是 | 是 |
| Composer | 编排 | 否 | 否 |

这四种职责构成了一个完备的集合。任何需要第五种原语的场景，都可以通过组合现有四种来实现。这不是一个巧合——这是通过精心设计达到的"最小完备集"。

### 1.4 Step 接口：统一外观

所有原语都可以通过 `Step[In, Out]` 接口统一包装，使得 Composer 不需要知道自己在编排什么类型的原语：

```go
type Step[In, Out any] interface {
    Execute(ctx context.Context, input In) (Out, error)
    UnitType() string  // "Atom" | "Port" | "Adapter" | "Composer"
}
```

这带来了一个关键属性：**Composer 只看到 Step，不关心具体原语类型**。新增原语类型不需要修改 Composer 的代码。

---

## 2. 四大原语

### 2.1 Atom —— 纯函数原语

Atom 是四种原语中最简单、最纯粹的一种。它是一个无副作用的纯函数，相同输入永远产生相同输出。

```go
// Atom 类型定义
type Atom[In, Out any] func(In) Out
```

**使用示例：**

```go
// 定义一个 Atom：将输入数字翻倍
double := func(x int) int { return x * 2 }
var atom Atom[int, int] = double

// 直接调用
result := atom(21)  // result = 42

// 包装为 Step
step := AtomAsStep(atom)
output, err := step.Execute(ctx, 42)
```

**约束：**
- 禁止 I/O 操作（文件读写、网络请求、数据库访问）
- 禁止随机数生成（使用 crypto/rand 的生成器必须通过 Adapter 注入）
- 禁止修改共享可变状态
- 禁止读取全局变量

### 2.2 Port —— 契约验证原语

Port 是系统边界的第一道防线。它验证输入数据是否符合契约，并在数据进入纯函数核心之前拒绝不合规的数据。

```go
// Port 类型定义
type Port[In, Out any] interface {
    Validate(ctx context.Context, input In) (Out, error)
}
```

**使用示例：**

```go
// 定义一个 Port：验证非空字符串
nonEmptyPort := NewPort(func(ctx context.Context, input string) (string, error) {
    if strings.TrimSpace(input) == "" {
        return "", NewStepError("VALIDATION_FAILED", "input must not be empty", true)
    }
    return input, nil
})

// 包装为 Step
step := PortAsStep(nonEmptyPort)
result, err := step.Execute(ctx, "hello")
```

**Port 的核心职责：**
- 输入验证与规范化
- 输出转换与格式化
- 类型安全检查
- 权限校验（见安全层）
- 契约合规性检查（见 PortContract）

### 2.3 Adapter —— 副作用隔离原语

Adapter 是系统中唯一允许执行副作用的地方。所有 I/O、数据库读写、网络调用、日志输出、外部 API 请求都必须通过 Adapter 进行。

```go
// Adapter 类型定义
type Adapter[In, Out any] interface {
    Execute(ctx context.Context, input In) (Out, error)
}
```

**使用示例：**

```go
// 定义一个 Adapter：将数据写入文件
fileStore := NewAdapter(func(ctx context.Context, input Data) (StoreResult, error) {
    data, err := json.Marshal(input)
    if err != nil {
        return StoreResult{}, err
    }
    err = os.WriteFile("data.json", data, 0644)
    return StoreResult{Path: "data.json"}, err
})

// 包装为 Step
step := AdapterAsStep(fileStore)
result, err := step.Execute(ctx, myData)
```

**Adapter 的核心职责：**
- 持久化存储（数据库、文件系统）
- 外部 API 调用（HTTP、gRPC）
- 消息队列（发布/订阅）
- 日志输出
- 指标采集

### 2.4 Composer —— 编排引擎原语

Composer 是编排引擎，负责将多个 Step 组合成完整的执行流程。它是四种原语中唯一能"包含"其他原语的原语。

```go
// Composer 类型定义
type Composer[T any] interface {
    Run(ctx context.Context, input T) (T, []ExecutionStep, error)
}
```

Composer 支持多种编排模式：

| 模式 | 实现 | 说明 |
|------|------|------|
| Pipeline | `NewPipeline[T](obs, steps...)` | 线性顺序执行 |
| Branch | `NewBranch[T](condition, truePath, falsePath)` | 条件分支 |
| Parallel | `RunParallel[T](ctx, input, composers...)` | 并发执行 |
| Retry | `WithRetry[T](comp, config)` | 指数退避重试 |
| Timeout | `WithTimeout[T](comp, timeout)` | 超时控制 |
| CircuitBreaker | `NewCircuitBreaker[T](inner, threshold, cooldown)` | 熔断器 |
| Fallback | `NewFallback[T](primary, fallback)` | 降级回退 |
| Bulkhead | `NewBulkhead[T](inner, maxConcurrency)` | 资源隔离 |
| RateLimiter | `NewRateLimiter[T](inner, rate, capacity)` | 限流 |
| Saga | `SagaComposer` | 分布式事务补偿 |

**使用示例：**

```go
// 创建 Pipeline
pipeline := NewPipeline[string](obs,
    AtomAsStep(validateAtom),
    PortAsStep(myPort),
    AdapterAsStep(myAdapter),
)

// 运行
result, steps, err := pipeline.Run(ctx, "input data")

// 检查执行 trace
tree := BuildTraceTree(steps)
fmt.Printf("记录了 %d 个执行步骤\n", len(steps))
```

### 2.5 Step 接口：统一四原语

`Step[In, Out]` 接口是连接四种原语的"通用适配器"。每种原语都可以通过对应的 `AsStep` 函数转换为 Step：

```go
// 四种原语 → Step 的转换函数
func AtomAsStep[In, Out any](a Atom[In, Out]) Step[In, Out]
func PortAsStep[In, Out any](p Port[In, Out]) Step[In, Out]
func AdapterAsStep[In, Out any](a Adapter[In, Out]) Step[In, Out]
// Composer 本身实现了 Step[In, Out] 接口（通过 ComposerAsStep）
```

此外，`StepFunc` 提供了从函数直接创建 Step 的便捷方式：

```go
step := NewStepFunc[int, string]("Custom", func(ctx context.Context, input int) (string, error) {
    return fmt.Sprintf("结果是: %d", input * 2), nil
})
```

---

## 3. 架构分层

Low-Entropy Core 采用 11 层分层架构。每一层都有明确的职责边界，层与层之间通过接口解耦。

```
┌──────────────────────────────────────────────────────────────┐
│                      Layer 11: 横切关注点                      │
│  TenantIsolation │ SagaTransaction │ DegradationManager │    │
│  FastPath │ StreamProcessing                                  │
├──────────────────────────────────────────────────────────────┤
│               Layer 10: 幂等与事件溯源 (Idempotency & ES)       │
│  IdempotentPort │ EventStore │ EventBus │ Projection         │
├──────────────────────────────────────────────────────────────┤
│                    Layer 9: 守护层 (Guardian)                  │
│  EntropyWatcher │ TransparencyWatcher │ DriftDetector │      │
│  ArchitectureGuard │ DecisionEngine │ AlertAdapter           │
├──────────────────────────────────────────────────────────────┤
│              Layer 8: 架构注册与韧性 (Registry & Resilience)    │
│  ArchitectureRegistry │ PortContract │ CircuitBreaker │      │
│  Fallback │ Bulkhead │ RateLimiter │ EntropyMetrics          │
├──────────────────────────────────────────────────────────────┤
│                  Layer 7: 模式管理 (Schema)                    │
│  SchemaRegistry │ CompatibilityChecker │ MigrationChain      │
├──────────────────────────────────────────────────────────────┤
│                     Layer 6: 安全层 (Security)                 │
│  CapabilityToken │ AccessControlPort │ AuditTrail │          │
│  MerkleChain                                                 │
├──────────────────────────────────────────────────────────────┤
│              Layer 5: 调度器系统 (Scheduler)                   │
│  AgentPool │ TaskQueue │ MatchEngine │ SchedulerComposer     │
├──────────────────────────────────────────────────────────────┤
│                  Layer 4: Handoff 协议                        │
│  Snapshot │ Contract │ Transport │ Persistence │             │
│  Composer │ Rollback                                         │
├──────────────────────────────────────────────────────────────┤
│                     Layer 3: 观测层 (Observation)              │
│  ObservationPipeline │ StepStore │ ObservationAPI │          │
│  Aggregator │ Sampler                                         │
├──────────────────────────────────────────────────────────────┤
│              Layer 2: 配置与组装 (Configuration)               │
│  PipelineConfig │ PipelineBuilder │ HotReload                │
├──────────────────────────────────────────────────────────────┤
│              Layer 1: 四原语核心 (Four Primitives)             │
│  Atom │ Port │ Adapter │ Composer │ Step │ ExecutionStep    │
│  TraceID │ SpanID │ StepError │ ObservationAdapter           │
└──────────────────────────────────────────────────────────────┘
```

### 3.1 Layer 1: 四原语核心

**文件**: `types.go`, `atom.go`, `port.go`, `adapter.go`, `composer.go`, `step.go`, `observation.go`, `errors.go`

核心层定义了整个框架的基础类型和接口。这是唯一不依赖其他层的模块。

**核心类型：**

| 类型 | 定义 | 说明 |
|------|------|------|
| `Atom[In, Out]` | `func(In) Out` | 纯函数，无副作用 |
| `Port[In, Out]` | `interface{ Validate(ctx, In) (Out, error) }` | 契约验证网关 |
| `Adapter[In, Out]` | `interface{ Execute(ctx, In) (Out, error) }` | 副作用边界 |
| `Composer[T]` | `interface{ Run(ctx, T) (T, []ExecutionStep, error) }` | 编排引擎 |
| `Step[In, Out]` | `interface{ Execute(ctx, In) (Out, error); UnitType() string }` | 统一外观 |
| `ExecutionStep` | 结构体 | 执行记录原子 |
| `StepError` | 结构体 | 结构化错误 |
| `TraceID` / `SpanID` | 类型别名 | 分布式追踪标识 |

**关键设计：**
- `ExecutionStep` 是唯一的执行记录格式，全局只有一个定义
- `ObservationAdapter` 定义了观测数据接收接口，由 Pipeline 在每一步自动调用
- `BuildTraceTree` 将扁平步骤列表构建为层级 TraceTree

### 3.2 Layer 2: 配置与组装

**文件**: `config.go`, `config_builder.go`, `config_hotreload.go`

第 2 层提供了将 JSON 配置转换为可运行的 Composer 的完整机制。

**核心类型：**

| 类型 | 说明 |
|------|------|
| `PipelineConfig` | 管线配置的 JSON 表示 |
| `StepConfig` | 单个步骤的配置（Type, Name, Params） |
| `PipelineBuilder` | 从 PipelineConfig 构建 Composer 实例 |
| `AdapterResolver` | 根据名称和环境解析 Adapter 实例 |
| `MapAdapterResolver` | 基于 map 的 AdapterResolver 实现 |

**配置解析流程：**

```go
// 1. 解析 JSON 配置
config, err := ParseConfig(jsonBytes)

// 2. 创建 Adapter 解析器
resolver := NewMapAdapterResolver()
resolver.Register("store", "dev", inMemoryStore)

// 3. 构建 Pipeline
builder := NewPipelineBuilder(resolver, obs)
composer, err := builder.Build(config)

// 4. 运行
result, steps, err := composer.Run(ctx, input)
```

**热重载（HotReload）：** 配置变更时自动重建 Pipeline，无需重启服务。

### 3.3 Layer 3: 观测层

**文件**: `observation.go`, `observation_pipeline.go`, `observation_store.go`, `observation_api.go`, `observation_aggregator.go`, `observation_sampler.go`

观测层自动记录所有执行步骤，提供全链路可观测性。**观测是自动的，不是手动的**——开发者不需要在代码中插入日志语句，Composer 在每一步执行时自动记录 ExecutionStep。

**核心类型：**

| 类型 | 说明 |
|------|------|
| `ExecutionStep` | 执行记录原子（在 Layer 1 定义，Layer 3 消费） |
| `ObservationAdapter` | 观测数据接收接口 |
| `InMemoryObservationAdapter` | 内存实现的观测适配器 |
| `NoOpObservationAdapter` | 空操作观测适配器（测试用） |
| `TraceNode` | 追踪树节点 |
| `TraceTree` | 从扁平步骤列表构建的层级追踪树 |
| `StepStore` | 步骤存储接口 |
| `InMemoryStepStore` | 内存步骤存储 |
| `StepQuery` | 步骤查询条件 |
| `ObservationPipeline` | 观测管线（采样、聚合、存储） |
| `ObservationAggregator` | 步骤聚合器 |
| `ObservationSampler` | 采样器（减少海量数据） |

**TraceTree 结构：**

```go
// 将扁平步骤列表构建为层级树
tree := BuildTraceTree(steps)

// 遍历树
for _, root := range tree.Roots {
    fmt.Printf("Trace: %s, Depth: %d\n", root.Step.TraceID, root.Depth)
    for _, child := range root.Children {
        fmt.Printf("  Child: %s\n", child.Step.Unit)
    }
}
```

### 3.4 Layer 4: Handoff 协议

**文件**: `handoff.go`, `handoff_snapshot.go`, `handoff_contract.go`, `handoff_transport.go`, `handoff_persistence.go`, `handoff_composer.go`, `handoff_rollback.go`

Handoff 协议是多 Agent 协作的核心机制。它允许 Agent A 将状态"存入"架构，Agent B 从架构中"取出"状态继续执行，实现 Agent 间无耦合的接力。

**核心类型：**

| 类型 | 说明 |
|------|------|
| `HandoffRequest` | 接力请求信封（SourceID, TargetID, TaskType, Payload, Token） |
| `HandoffSnapshot` | 架构中存储的状态快照 |
| `HandoffResult` | 接力结果 |
| `HandoffComposer` | 完整的接力编排器 |
| `HandoffRollback` | 接力回滚机制 |
| `SnapshotAdapter[T]` | 类型化快照适配器 |
| `DefaultSnapshotAdapter` | 默认快照适配器 |
| `TransportFunc` | 传输函数（队列、数据库、RPC） |

**Handoff 流程：**

```
Agent A  ──deposit──▶  Architecture (Snapshot)  ──withdraw──▶  Agent B
         创建快照         持久化存储/传输                          恢复状态
```

```go
// 创建 Handoff
handoff := NewHandoff(sourceAgent, targetAgent, snapshotAdapter, transport)

// 执行接力
result, steps, err := handoff.Run(ctx, HandoffRequest{
    SourceID: "agent-1",
    TargetID: "agent-2",
    TaskType: "data-processing",
    Payload:  myData,
    Token:    "trace-123",
})
```

### 3.5 Layer 5: 调度器系统

**文件**: `scheduler_agent_pool.go`, `scheduler_queue.go`, `scheduler_match.go`, `scheduler_composer.go`

调度器系统负责将任务分配给合适的 Agent，支持队列管理、Agent 池、匹配引擎和完整的调度编排。

**核心类型：**

| 类型 | 说明 |
|------|------|
| `AgentPool` | Agent 池，管理可用 Agent 的生命周期 |
| `TaskQueue` | 任务队列，支持优先级、延迟、重试 |
| `MatchEngine` | 纯函数匹配引擎（Atom），将任务匹配到 Agent |
| `SchedulerComposer` | 完整调度编排器（Dequeue → Match → Dispatch） |
| `ScheduleResult` | 调度结果 |

**调度流程：**

```
TaskQueue ──Dequeue──▶  MatchEngine  ──Match──▶  HandoffComposer  ──Dispatch──▶  Agent
    ↑                                                                                │
    └────────────────────── Requeue (no match) ──────────────────────────────────────┘
```

```go
// 创建调度器
scheduler := NewSchedulerComposer(pool, queue, handoff, obs)

// 调度下一个任务
result, steps, err := scheduler.ScheduleNext(ctx, 5*time.Second)
if result.Dispatched {
    fmt.Printf("任务 %s 已分派给 Agent %s\n", result.TaskID, result.MatchedAgent)
}
```

### 3.6 Layer 6: 安全层

**文件**: `security_capability.go`, `security_access.go`, `security_audit.go`, `security_merkle.go`

安全层提供基于能力的访问控制、不可篡改的审计日志和 Merkle 树防篡改机制。

**核心类型：**

| 类型 | 文件 | 说明 |
|------|------|------|
| `CapabilityToken` | `security_capability.go` | HMAC-SHA256 签名的能力令牌 |
| `CapabilityPort` | `security_capability.go` | 令牌验证 Port |
| `AccessRequest` | `security_access.go` | 访问请求 |
| `AccessDecision` | `security_access.go` | 访问决策 |
| `AccessControlPort` | `security_access.go` | 访问控制 Port |
| `AccessPolicy` | `security_access.go` | 声明式访问策略 |
| `AuditEntry` | `security_audit.go` | 审计条目 |
| `AuditTrailAdapter` | `security_audit.go` | 审计日志 Adapter |
| `MerkleAuditChain` | `security_merkle.go` | Merkle 树审计链 |
| `MerkleProof` | `security_merkle.go` | Merkle 证明 |

**CapabilityToken 工作流：**

```go
// 1. 颁发令牌
token := NewCapabilityToken("agent-1", []string{"pipeline:read", "pipeline:write"})
token.Sign(secretKey)

// 2. 验证令牌
capPort := NewCapabilityPort(secretKey, "pipeline:read")
validated, err := capPort.Validate(ctx, *token)

// 3. 检查访问权限
acPort := NewAccessControlPort(secretKey)
decision, err := acPort.Validate(ctx, AccessRequest{
    AgentID:  "agent-1",
    Action:   "write",
    Resource: "pipeline",
    Token:    token,
})
```

**Merkle 审计链：** 每条审计记录通过 SHA-256 链接到前一条，形成防篡改链。`DetectTampering()` 方法可以检测任何被篡改的记录。

### 3.7 Layer 7: 模式管理

**文件**: `schema_registry.go`, `schema_compat.go`, `schema_migration.go`

模式管理层提供线程安全的模式注册、版本控制、兼容性检查和迁移链管理。

**核心类型：**

| 类型 | 说明 |
|------|------|
| `SchemaRegistry` | 线程安全的模式注册表（`sync.Map` 实现） |
| `SchemaChange` | 模式变更记录 |
| `CompatibilityChecker` | 兼容性检查器（向前/向后兼容） |
| `MigrationChain` | 模式迁移链 |

**SchemaRegistry 使用：**

```go
registry := NewSchemaRegistry()

// 注册模式
registry.Register("TaskRequest", "v1", taskSchemaV1)
registry.Register("TaskRequest", "v2", taskSchemaV2)

// 查询模式
schema, err := registry.Get("TaskRequest", "v2")

// 列出所有版本
versions := registry.ListVersions("TaskRequest")
// ["v1", "v2"]

// 列出所有类型
types := registry.ListTypes()
```

### 3.8 Layer 8: 架构注册与韧性

**文件**: `architecture_registry.go`, `port_contract.go`, `patterns_resilience.go`, `entropy_metrics.go`

第 8 层提供架构静态视图、端口契约自动描述和多种韧性模式（熔断器、降级、隔离、限流）。

**核心类型：**

| 类型 | 文件 | 说明 |
|------|------|------|
| `ArchitectureRegistry` | `architecture_registry.go` | 管线与契约的静态注册表 |
| `PipelineDescriptor` | `architecture_registry.go` | 管线描述符 |
| `PortContract` | `port_contract.go` | 端口契约（输入/输出类型、验证规则） |
| `ValidationRule` | `port_contract.go` | 验证规则 |
| `CircuitBreaker[T]` | `patterns_resilience.go` | 熔断器（Closed → Open → HalfOpen） |
| `Fallback[T]` | `patterns_resilience.go` | 降级回退 |
| `Bulkhead[T]` | `patterns_resilience.go` | 资源隔离（信号量） |
| `RateLimiter[T]` | `patterns_resilience.go` | 令牌桶限流 |
| `ResilienceChain[T]` | `patterns_resilience.go` | 韧性组合链 |
| `EntropySnapshot` | `entropy_metrics.go` | 系统熵快照 |
| `EntropyCollector` | `entropy_metrics.go` | 熵采集器 |

**PortContract 自动描述：**

```go
// 自动从 Port 实例提取契约
contract := DescribePortContract("MyPort", myPort, "my-pipeline")
// 自动注册到架构注册表
RegisterPortContract(registry, "MyPort", myPort, "my-pipeline")
```

**韧性链：**

```go
// 一键构建完整韧性链：RateLimiter → Bulkhead → CircuitBreaker → Fallback
resilient := ResilienceChain(myComposer, ResilienceConfig[MyType]{
    RateLimit:                100,
    RateLimitBurst:           200,
    BulkheadMax:              50,
    CircuitBreakerThreshold:  5,
    CircuitBreakerCooldown:   30 * time.Second,
    Fallback:                 fallbackComposer,
})
```

### 3.9 Layer 9: 守护层

**文件**: `guardian_entropy.go`, `guardian_transparency.go`, `guardian_drift.go`, `guardian_architecture.go`, `guardian_decision.go`, `guardian_alert.go`

守护层是系统的"免疫系统"。它包含四个独立的 Watcher 和一个中心化的 DecisionEngine，持续监控系统健康状态。

**四个 Watcher：**

| Watcher | 文件 | 接口 | 职责 |
|---------|------|------|------|
| `EntropyWatcher` | `guardian_entropy.go` | `Port[EntropySnapshot, EntropyAlert]` | 监控熵值，检测加速 |
| `TransparencyWatcher` | `guardian_transparency.go` | `Port[TransparencyInput, TransparencyAlert]` | 审计链完整性、观测覆盖率 |
| `DriftDetector` | `guardian_drift.go` | Atom（纯函数） | Agent 行为偏差检测 |
| `ArchitectureGuard` | `guardian_architecture.go` | `Port[ArchitectureInput, ArchitectureAlert]` | 架构合规性检查 |

**DecisionEngine 和 AlertAdapter：**

```
EntropyWatcher ──────────┐
TransparencyWatcher ─────┤
DriftDetector ───────────┼──▶ DecisionEngine ──▶ AlertAdapter ──▶ 日志/Webhook/Channel
ArchitectureGuard ───────┘         │
                                   │
                          Action: Allow / Warn / Block / Rollback
```

**决策优先级：**

| 优先级 | 条件 | 动作 |
|--------|------|------|
| 1 | Entropy=Red 或 ShouldQuarantine 或 Violations>=3 | Rollback |
| 2 | Entropy=Orange 或 DriftScore>0.7 | Block |
| 3 | Entropy=Yellow 或 DriftScore>0.3 或 !IsHealthy | Warn |
| 4 | 以上均不满足 | Allow |

```go
// 创建守护者管线
engine := NewDecisionEngine(obs)
input := GuardianInput{
    EntropyAlert: entropyAlert,
    TranspAlert:  transpAlert,
    DriftResult:  driftResult,
    ArchAlert:    archAlert,
}
decision, err := engine.Run(ctx, input)

switch decision.Action {
case ActionAllow:
    fmt.Println("系统正常")
case ActionWarn:
    fmt.Println("需要关注:", decision.Reason)
case ActionBlock:
    fmt.Println("已阻止:", decision.Reason)
case ActionRollback:
    fmt.Println("触发回滚:", decision.Reason)
}
```

### 3.10 Layer 10: 幂等与事件溯源

**文件**: `idempotent.go`, `eventstore.go`, `eventbus.go`, `projection.go`

第 10 层提供幂等执行保证和事件溯源（Event Sourcing）基础设施。

**核心类型：**

| 类型 | 文件 | 说明 |
|------|------|------|
| `IdempotentRequest[In]` | `idempotent.go` | 带幂等键的请求 |
| `IdempotentResult[Out]` | `idempotent.go` | 带缓存标记的结果 |
| `IdempotentStore` | `idempotent.go` | 幂等存储接口 |
| `InMemoryIdempotentStore` | `idempotent.go` | 内存幂等存储 |
| `IdempotentPort` | `idempotent.go` | 幂等性检查 Port |
| `EventEnvelope` | `eventstore.go` | 事件信封 |
| `EventStore` | `eventstore.go` | 不可变事件存储（Adapter） |
| `EventBus` | `eventbus.go` | 事件总线（Adapter） |
| `EventHandler` | `eventbus.go` | 事件处理器 |
| `Projection` | `projection.go` | 投影（Atom，纯函数） |
| `ProjectionHandler` | `projection.go` | 投影处理器 |

**幂等执行：**

```go
store := NewInMemoryIdempotentStore()
port := NewIdempotentPort(store, myComposer, 1*time.Hour)

// 第一次执行：真实执行
result, err := port.Validate(ctx, IdempotentRequest[MyInput]{
    Key:   "idem-key-001",
    Input: myInput,
})
// result.FromCache = false

// 第二次执行（相同 key）：从缓存返回
result, err = port.Validate(ctx, IdempotentRequest[MyInput]{
    Key:   "idem-key-001",
    Input: myInput,
})
// result.FromCache = true
```

**事件溯源 + 投影：**

```go
// 1. 追加事件
eventStore := NewEventStore()
result, err := eventStore.Execute(ctx, EventEnvelope{
    AggregateID: "task-123",
    EventType:   "TaskCreated",
    EventData:   eventData,
})

// 2. 发布事件
eventBus := NewEventBus()
eventBus.Subscribe("TaskCreated", handler, false)
eventBus.Execute(ctx, envelope)

// 3. 投影重建状态
projection := NewProjection(myHandler)
output, err := projection.Execute(ProjectionInput{
    AggregateID: "task-123",
    Events:      allEvents,
    FromVersion: 0,
})
```

### 3.11 Layer 11: 横切关注点

**文件**: `tenant.go`, `transaction.go`, `degradation.go`, `fastpath.go`, `stream.go`

第 11 层包含跨越多个层的横切关注点。

**核心类型：**

| 类型 | 文件 | 说明 |
|------|------|------|
| `TenantIsolationPort` | `tenant.go` | 多租户隔离 Port |
| `TenantRequest` | `tenant.go` | 租户请求包装 |
| `SagaComposer` | `transaction.go` | Saga 分布式事务编排器 |
| `SagaStep` | `transaction.go` | Saga 步骤（Execute + Compensate） |
| `DegradationManager` | `degradation.go` | 优雅降级管理器 |
| `DegradationMode` | `degradation.go` | 降级模式（none/non_critical/safe/emergency） |
| `FastPipeline[T]` | `fastpath.go` | 零分配快速路径 |
| `StreamMap` | `stream.go` | 流式 Map |
| `StreamFilter` | `stream.go` | 流式 Filter |
| `StreamReduce` | `stream.go` | 流式 Reduce |
| `Window` | `stream.go` | 窗口聚合 |

**Saga 事务：**

```go
saga := NewSagaComposer(obs)
saga.AddStep(SagaStep{
    Name:    "create_order",
    Execute:    createOrderStep,
    Compensate: cancelOrderStep,
})
saga.AddStep(SagaStep{
    Name:    "reserve_inventory",
    Execute:    reserveInventoryStep,
    Compensate: releaseInventoryStep,
})

result, err := saga.Run(ctx, input)
// 失败时自动按逆序执行补偿
```

**优雅降级：**

```go
dm := NewDegradationManager(obs)

// 根据降级模式决定是否处理请求
if dm.ShouldProcess("critical") {
    // 始终处理关键操作
}
if dm.ShouldProcess("non_critical") {
    // 在 safe 或 emergency 模式下跳过
}
```

**FastPath：**

```go
// 零分配快速路径——跳过所有观测记录
fast := NewFastPipeline[MyType]("hot-path")
    .AddStep(step1)
    .AddStep(step2)

result, err := fast.Run(ctx, input)
// 不产生 ExecutionStep，适合高频调用
```

---

## 4. 快速开始

下面是一个完整的 5 分钟示例，展示从定义 Atom 到检查 TraceTree 的完整流程。

### 步骤 1：定义 Atom

```go
package main

import (
    "context"
    "fmt"
    "strings"
    core "low-entropy-core/go-core"
)

// 定义一个纯函数 Atom：将输入字符串转为大写
toUpper := func(input string) string {
    return strings.ToUpper(input)
}

// 定义为 Atom 类型
var upperAtom core.Atom[string, string] = toUpper
```

### 步骤 2：定义 Port 和 Adapter

```go
// 定义 Port：验证输入非空
validatePort := core.NewPort(func(ctx context.Context, input string) (string, error) {
    if strings.TrimSpace(input) == "" {
        return "", core.NewStepError("EMPTY_INPUT", "输入不能为空", true)
    }
    return input, nil
})

// 定义 Adapter：模拟外部存储
saveAdapter := core.NewAdapter(func(ctx context.Context, input string) (string, error) {
    result := fmt.Sprintf("已保存: %s", input)
    // 这里可以替换为真实的数据库写入
    return result, nil
})
```

### 步骤 3：构建 Pipeline 并运行

```go
func main() {
    ctx := context.Background()

    // 创建观测适配器
    obs := &core.InMemoryObservationAdapter{}

    // 构建 Pipeline
    pipeline := core.NewPipeline[string](obs,
        core.PortAsStep(validatePort),   // 第 1 步：验证输入
        core.AtomAsStep(upperAtom),      // 第 2 步：转大写
        core.AdapterAsStep(saveAdapter), // 第 3 步：存储
    )

    // 运行
    result, steps, err := pipeline.Run(ctx, "hello world")
    if err != nil {
        fmt.Printf("执行失败: %v\n", err)
        return
    }

    fmt.Printf("最终结果: %s\n", result)
    fmt.Printf("执行了 %d 个步骤\n", len(steps))
}
```

### 步骤 4：检查 TraceTree

```go
    // 构建追踪树
    tree := obs.GetTraceTree()

    fmt.Println("\n=== 执行追踪树 ===")
    for _, root := range tree.Roots {
        fmt.Printf("Trace: %s\n", root.Step.TraceID)
        printNode(root, 0)
    }

    // 输出熵信息
    collector := core.NewEntropyCollector()
    snap := collector.CollectFromSteps(steps)
    fmt.Printf("\n熵得分: %.2f\n", snap.EntropyScore)
    fmt.Printf("错误率: %.2f%%\n", snap.ErrorRate*100)
}

func printNode(node *core.TraceNode, depth int) {
    indent := strings.Repeat("  ", depth)
    unit := node.Step.Unit
    action := node.Step.Action
    duration := node.Step.DurationMs
    if node.Step.Error != nil {
        fmt.Printf("%s[%s] %s - ERROR: %s (%dms)\n",
            indent, unit, action, node.Step.Error.Message, duration)
    } else {
        fmt.Printf("%s[%s] %s - OK (%dms)\n",
            indent, unit, action, duration)
    }
    for _, child := range node.Children {
        printNode(child, depth+1)
    }
}
```

**预期输出：**

```
最终结果: 已保存: HELLO WORLD
执行了 3 个步骤

=== 执行追踪树 ===
Trace: a1b2c3d4-...
  [Port] execute - OK (0ms)
  [Atom] execute - OK (0ms)
  [Adapter] execute - OK (1ms)

熵得分: 5.00
错误率: 0.00%
```

---

## 5. 设计决策

本节列出 5 个关键架构决策及其背后的理由。

### 5.1 四种原语，拒绝第五种

**决策：** 整个系统只用四种原语（Atom, Port, Adapter, Composer），不引入第五种抽象。

**理由：**

四种原语形成了一个完备的职责划分：
- **纯计算**（Atom）和**副作用**（Adapter）是正交的
- **边界验证**（Port）和**内部编排**（Composer）是正交的
- 任何新的抽象都可以映射到这四个类别之一

在 1.0 版本中，我们曾尝试引入 "Middleware"、"Interceptor"、"Hook"、"Plugin" 等概念。每一次引入都导致：
- 开发者需要学习新的接口
- 执行顺序变得难以预测
- 调试时需要理解更多代码路径

v2.0 的结论是：这四种原语已经足够。`WithRetry`、`WithTimeout`、`CircuitBreaker` 等"看起来像第五种原语"的模式，实际上都是 Composer 的装饰器——它们包装 Composer 并返回 Composer。

### 5.2 Step 接口作为通用适配器

**决策：** 所有原语都通过 `Step[In, Out]` 接口统一，Composer 只看到 Step，不关心具体原语类型。

**理由：**

这个决策带来了两个关键好处：

1. **可组合性**：任何原语都可以放入任何 Composer 中。Pipeline 可以组合 Atom、Port、Adapter，甚至嵌套的 Composer。

2. **可扩展性**：如果未来需要引入新的原语类型（例如 "Validator"），只需实现 `Step[In, Out]` 接口，无需修改 Composer 代码。

```go
// Composer 不需要知道它编排的是什么
func (p *Pipeline[T]) Run(ctx context.Context, input T) (T, []ExecutionStep, error) {
    for _, step := range p.steps {
        output, err := step.Execute(ctx, result)  // Step 接口
        // ...
    }
}
```

### 5.3 Composer 作为唯一的编排入口

**决策：** 所有编排逻辑（顺序、分支、并发、重试）都通过 Composer 表达，不存在独立的 "Orchestrator"、"FlowEngine" 或 "WorkflowManager"。

**理由：**

在 1.0 版本中，我们同时存在 `Pipeline`、`Supervisor`、`Orchestrator`、`FlowManager` 等多个编排概念。代码审查时经常出现的问题是："这段逻辑应该放在 Pipeline 还是 Orchestrator 里？"

v2.0 统一为 Composer：
- `Pipeline` 是 Composer 的一种实现
- `Branch` 是 Composer 的组合
- `Parallel` 是 Composer 的并发组合
- `WithRetry` 是 Composer 的装饰器
- `SagaComposer` 是 Composer 的扩展

**一个 Composer 可以嵌套另一个 Composer**，这是实现任意复杂度的关键。

### 5.4 观测是自动的，不是手动的

**决策：** 观测数据（ExecutionStep）由 Composer 在执行每一步时自动生成，开发者不需要手动调用日志函数。

**理由：**

传统框架中，开发者需要手动插入观测代码：

```go
// 传统方式：手动日志
log.Info("starting step", "step", "validate")
result, err := validate(input)
log.Info("step completed", "step", "validate", "duration", elapsed)
```

这种方式的问题：
1. 容易遗漏（开发者忘记打日志）
2. 格式不一致（不同开发者写的日志格式不同）
3. 无法统一查询（日志分散在各处）

Low-Entropy Core 的方式：

```go
// Pipeline.Run 内部自动生成 ExecutionStep
// 开发者零代码，观测全自动
```

`ExecutionStep` 是全局唯一的记录格式，包含 TraceID、SpanID、ParentID、Duration、Error 等完整信息，可以直接构建 TraceTree 用于可视化。

### 5.5 守护层作为独立的横切层

**决策：** 系统健康监控（熵、透明度、偏差、架构合规）不嵌入到任何业务层，而是作为独立的第 9 层（Guardian Layer）横切整个系统。

**理由：**

如果将监控逻辑分散到各层，会导致：
- 监控逻辑与业务逻辑耦合，难以独立测试
- 监控阈值分散在各处，难以统一调整
- 新增监控维度需要修改多处代码

守护层的设计将所有监控逻辑集中在一起：

```
Layer 9: Guardian Layer
├── EntropyWatcher       (监控系统熵)
├── TransparencyWatcher  (监控审计完整性)
├── DriftDetector        (监控 Agent 行为偏差)
├── ArchitectureGuard    (监控架构合规性)
├── DecisionEngine       (综合决策)
└── AlertAdapter         (告警分发)
```

每个 Watcher 是独立的 Port 或 Atom，可以独立测试、独立部署。DecisionEngine 只做决策路由，不包含业务逻辑。AlertAdapter 是唯一的副作用点（发送告警）。

---

## 6. API 参考

### 6.1 四原语核心 (Layer 1)

| 类型 | 签名 | 说明 |
|------|------|------|
| `Atom[In, Out]` | `func(In) Out` | 纯函数原语 |
| `Port[In, Out]` | `interface{ Validate(ctx, In) (Out, error) }` | 契约验证原语 |
| `Adapter[In, Out]` | `interface{ Execute(ctx, In) (Out, error) }` | 副作用隔离原语 |
| `Composer[T]` | `interface{ Run(ctx, T) (T, []ExecutionStep, error) }` | 编排引擎原语 |
| `Step[In, Out]` | `interface{ Execute(ctx, In) (Out, error); UnitType() string }` | 统一外观 |
| `StepFunc[In, Out]` | 结构体 | 函数适配器 |
| `ExecutionStep` | 结构体 | 执行记录原子 |
| `StepError` | 结构体 | 结构化错误 |
| `TraceID` | `type TraceID string` | 追踪标识 |
| `SpanID` | `type SpanID string` | 跨度标识 |
| `TraceContext` | 结构体 | 追踪上下文 |
| `ObservationAdapter` | `interface{ Record([]ExecutionStep) }` | 观测接收器 |
| `InMemoryObservationAdapter` | 结构体 | 内存观测实现 |
| `NoOpObservationAdapter` | 结构体 | 空操作观测实现 |
| `TraceNode` | 结构体 | 追踪树节点 |
| `TraceTree` | 结构体 | 层级追踪树 |
| `ErrorCategory` | `type ErrorCategory string` | 错误分类 |
| `ErrorCode` | 结构体 | 标准化错误码 |

**构造函数/辅助函数：**

| 函数 | 说明 |
|------|------|
| `NewStepFunc[In, Out](unitType, fn)` | 创建 StepFunc |
| `AtomAsStep[In, Out](a)` | Atom 转 Step |
| `PortAsStep[In, Out](p)` | Port 转 Step |
| `AdapterAsStep[In, Out](a)` | Adapter 转 Step |
| `NewPort[In, Out](fn)` | 创建 PortFunc |
| `NewAdapter[In, Out](fn)` | 创建 AdapterFunc |
| `NewPipeline[T](obs, steps...)` | 创建 Pipeline |
| `NewBranch[T](condition, truePath, falsePath)` | 创建分支 |
| `RunParallel[T](ctx, input, composers...)` | 并发执行 |
| `WithRetry[T](comp, config)` | 重试装饰器 |
| `WithTimeout[T](comp, timeout)` | 超时装饰器 |
| `Compose[T](obs, step)` | Step 转 Composer |
| `NewStepError(code, msg, recoverable)` | 创建 StepError |
| `NewTraceID()` | 生成追踪 ID |
| `NewSpanID()` | 生成跨度 ID |
| `BuildTraceTree(steps)` | 构建追踪树 |
| `WithTraceContext(ctx, tc)` | 注入追踪上下文 |
| `GetTraceContext(ctx)` | 提取追踪上下文 |

### 6.2 配置与组装 (Layer 2)

| 类型 | 说明 |
|------|------|
| `PipelineConfig` | 管线配置 |
| `StepConfig` | 步骤配置 |
| `PipelineBuilder` | 管线构建器 |
| `AdapterResolver` | 适配器解析器接口 |
| `MapAdapterResolver` | 基于 map 的解析器 |

**函数：**

| 函数 | 说明 |
|------|------|
| `ParseConfig(jsonBytes)` | 解析 JSON 配置 |
| `ValidateConfig(config)` | 验证配置 |
| `ParseAndValidateConfig(jsonBytes)` | 解析并验证 |
| `NewPipelineBuilder(resolver, obs)` | 创建构建器 |
| `NewMapAdapterResolver()` | 创建解析器 |

### 6.3 观测层 (Layer 3)

| 类型 | 说明 |
|------|------|
| `StepStore` | 步骤存储接口 |
| `InMemoryStepStore` | 内存步骤存储 |
| `StepQuery` | 步骤查询条件 |
| `ObservationPipeline` | 观测管线 |
| `ObservationAggregator` | 聚合器 |
| `ObservationSampler` | 采样器 |
| `ObservationAPI` | 观测 API |

### 6.4 Handoff 协议 (Layer 4)

| 类型 | 说明 |
|------|------|
| `HandoffRequest` | 接力请求 |
| `HandoffSnapshot` | 状态快照 |
| `HandoffResult` | 接力结果 |
| `HandoffComposer` | 接力编排器 |
| `HandoffRollback` | 接力回滚 |
| `SnapshotAdapter[T]` | 类型化快照适配器 |
| `DefaultSnapshotAdapter` | 默认快照适配器 |
| `TransportFunc` | 传输函数 |
| `HandoffContract` | 接力契约 |

**函数：**

| 函数 | 说明 |
|------|------|
| `NewHandoff(source, target, snap, transport)` | 创建接力 |
| `InProcTransport(snap)` | 进程内传输 |

### 6.5 调度器 (Layer 5)

| 类型 | 说明 |
|------|------|
| `AgentPool` | Agent 池 |
| `TaskQueue` | 任务队列 |
| `MatchEngine` | 匹配引擎 |
| `SchedulerComposer` | 调度编排器 |
| `ScheduleResult` | 调度结果 |

**函数：**

| 函数 | 说明 |
|------|------|
| `NewSchedulerComposer(pool, queue, handoff, obs)` | 创建调度器 |

### 6.6 安全层 (Layer 6)

| 类型 | 说明 |
|------|------|
| `CapabilityToken` | 能力令牌 |
| `CapabilityPort` | 令牌验证 Port |
| `AccessRequest` | 访问请求 |
| `AccessDecision` | 访问决策 |
| `AccessControlPort` | 访问控制 Port |
| `AccessPolicy` | 访问策略 |
| `AuditEntry` | 审计条目 |
| `AuditTrailAdapter` | 审计日志 Adapter |
| `MerkleAuditChain` | Merkle 审计链 |
| `MerkleProof` | Merkle 证明 |
| `MerkleNode` | Merkle 树节点 |

**函数：**

| 函数 | 说明 |
|------|------|
| `NewCapabilityToken(agentID, caps)` | 创建令牌 |
| `NewCapabilityPort(secretKey, requiredCap)` | 创建令牌验证 Port |
| `NewAccessControlPort(secretKey)` | 创建访问控制 Port |
| `NewAuditTrailAdapter()` | 创建审计日志 |
| `NewMerkleAuditChain()` | 创建 Merkle 链 |
| `VerifyProof(proof, rootHash)` | 验证 Merkle 证明 |
| `AuditSuccess(agentID, action, resource, id, details)` | 成功审计 |
| `AuditFailure(agentID, action, resource, id, details, err)` | 失败审计 |
| `AuditDenied(agentID, action, resource, id, reason)` | 拒绝审计 |

### 6.7 模式管理 (Layer 7)

| 类型 | 说明 |
|------|------|
| `SchemaRegistry` | 模式注册表 |
| `SchemaChange` | 模式变更 |
| `CompatibilityChecker` | 兼容性检查器 |
| `MigrationChain` | 迁移链 |

**函数：**

| 函数 | 说明 |
|------|------|
| `NewSchemaRegistry()` | 创建注册表 |

### 6.8 架构注册与韧性 (Layer 8)

| 类型 | 说明 |
|------|------|
| `ArchitectureRegistry` | 架构注册表 |
| `PipelineDescriptor` | 管线描述符 |
| `PortContract` | 端口契约 |
| `ValidationRule` | 验证规则 |
| `ContractComplianceResult` | 契约合规结果 |
| `CircuitBreaker[T]` | 熔断器 |
| `Fallback[T]` | 降级回退 |
| `Bulkhead[T]` | 资源隔离 |
| `RateLimiter[T]` | 限流器 |
| `ResilienceConfig[T]` | 韧性配置 |
| `EntropySnapshot` | 熵快照 |
| `EntropyCollector` | 熵采集器 |

**函数：**

| 函数 | 说明 |
|------|------|
| `NewCircuitBreaker[T](inner, threshold, cooldown)` | 创建熔断器 |
| `NewFallback[T](primary, fallback)` | 创建降级 |
| `NewBulkhead[T](inner, maxConcurrency)` | 创建隔离 |
| `NewRateLimiter[T](inner, rate, capacity)` | 创建限流 |
| `ResilienceChain[T](inner, config)` | 组合韧性链 |
| `DescribePortContract(name, port, pipelineName)` | 描述契约 |
| `RegisterPortContract(registry, name, port, pipelineName)` | 注册契约 |
| `CheckContractCompliance(contract, input)` | 检查合规 |
| `NewEntropyCollector()` | 创建熵采集器 |
| `EntropyAtom(store)` | 创建熵 Atom |

### 6.9 守护层 (Layer 9)

| 类型 | 说明 |
|------|------|
| `EntropyWatcher` | 熵监控 Port |
| `EntropyAlert` | 熵告警 |
| `EntropyLevel` | 熵级别（OK/Yellow/Orange/Red） |
| `TransparencyWatcher` | 透明度监控 Port |
| `TransparencyInput` | 透明度输入 |
| `TransparencyAlert` | 透明度告警 |
| `DriftDetector` | 偏差检测 Atom |
| `DriftInput` | 偏差输入 |
| `DriftOutput` | 偏差输出 |
| `DriftType` | 偏差类型 |
| `ArchitectureGuard` | 架构合规 Port |
| `ArchitectureInput` | 架构输入 |
| `ArchitectureAlert` | 架构告警 |
| `DecisionEngine` | 决策引擎 Composer |
| `GuardianInput` | 守护者输入 |
| `GuardianDecision` | 守护者决策 |
| `GuardianAction` | 守护者动作 |
| `AlertAdapter` | 告警适配器 |
| `AlertConfig` | 告警配置 |
| `AlertResult` | 告警结果 |
| `AlertChannel` | 告警通道 |

**函数：**

| 函数 | 说明 |
|------|------|
| `NewEntropyWatcher()` | 创建熵监控 |
| `NewTransparencyWatcher()` | 创建透明度监控 |
| `NewDriftDetector()` | 创建偏差检测 |
| `NewArchitectureGuard()` | 创建架构守卫 |
| `NewDecisionEngine(obs)` | 创建决策引擎 |
| `NewAlertAdapter(config)` | 创建告警适配器 |

### 6.10 幂等与事件溯源 (Layer 10)

| 类型 | 说明 |
|------|------|
| `IdempotentRequest[In]` | 幂等请求 |
| `IdempotentResult[Out]` | 幂等结果 |
| `IdempotentStore` | 幂等存储接口 |
| `InMemoryIdempotentStore` | 内存幂等存储 |
| `IdempotentPort` | 幂等 Port |
| `EventEnvelope` | 事件信封 |
| `EventStore` | 事件存储 Adapter |
| `AppendResult` | 追加结果 |
| `EventBus` | 事件总线 Adapter |
| `PublishResult` | 发布结果 |
| `EventHandler` | 事件处理器 |
| `Subscription` | 订阅 |
| `Projection` | 投影 Atom |
| `ProjectionHandler` | 投影处理器 |
| `ProjectionInput` | 投影输入 |
| `ProjectionOutput` | 投影输出 |

**函数：**

| 函数 | 说明 |
|------|------|
| `NewInMemoryIdempotentStore()` | 创建幂等存储 |
| `NewIdempotentPort(store, inner, ttl)` | 创建幂等 Port |
| `NewEventStore()` | 创建事件存储 |
| `NewEventBus()` | 创建事件总线 |
| `NewProjection(handler)` | 创建投影 |

### 6.11 横切关注点 (Layer 11)

| 类型 | 说明 |
|------|------|
| `TenantID` | 租户 ID |
| `TenantContext` | 租户上下文 |
| `TenantRequest` | 租户请求 |
| `TenantIsolationPort` | 租户隔离 Port |
| `SagaComposer` | Saga 事务编排器 |
| `SagaStep` | Saga 步骤 |
| `TransactionContext` | 事务上下文 |
| `DegradationManager` | 降级管理器 |
| `DegradationMode` | 降级模式 |
| `FastPipeline[T]` | 快速路径管线 |
| `Stream[T]` | 流式数据类型 |
| `StreamConfig` | 流式配置 |

**函数：**

| 函数 | 说明 |
|------|------|
| `NewTenantIsolationPort()` | 创建租户隔离 |
| `NewSagaComposer(obs)` | 创建 Saga 编排器 |
| `NewDegradationManager(obs)` | 创建降级管理器 |
| `NewFastPipeline[T](name)` | 创建快速管线 |
| `StreamMap[In, Out](input, fn)` | 流式 Map |
| `StreamFilter[T](input, predicate)` | 流式 Filter |
| `StreamReduce[T, R](input, initial, fn)` | 流式 Reduce |
| `Window[T](input, size)` | 窗口聚合 |

---

> **文档版本**: v2.0 | **最后更新**: 2026-06-18 | **维护者**: Low-Entropy Core Team