# 升级文档：正式的 Agent Handoff 协议

## 1. 背景与目标
当前多智能体协作是隐式的（人类手动切换示例）。在 100 万行代码 + 多智能体自主开发场景下，需要**显式的、可验证的 Handoff 协议**。

目标：定义 Agent Handoff Protocol，使一个 Composer（或 Agent）能安全地将任务/状态交给另一个 Composer，所有过程通过 4 原语表达，且全程可被人类仪表盘观察。

## 2. 设计原则
- Handoff 本身是一个特殊的 Composer 流程
- 使用 Port 进行契约验证（Handoff 契约）
- 使用 Adapter 进行状态快照、传输、回滚
- 整个过程产生标准 ExecutionStep（Pattern: "Handoff"）
- 禁止隐式共享状态（必须显式通过 Handoff 传递）

## 3. 协议定义

### 3.1 核心概念
- **Source Agent**：当前持有任务的 Composer
- **Target Agent**：接收任务的 Composer
- **Handoff Contract**：Port 定义的契约（任务类型、必要字段、权限）
- **Snapshot**：Adapter 生成的状态快照
- **Handoff Token**：唯一标识一次交接（用于追踪）

### 3.2 协议流程（用 4 原语表达）
1. Source 创建 Handoff Request（Atom）
2. Source Port 验证请求 + 目标可用性（Port）
3. Source Adapter 生成 Snapshot + 序列化（Adapter）
4. Source Composer 调用 Handoff Composer：
   - 传输 Snapshot（通过 NetworkAdapter 或 InProcAdapter）
   - Target Port 验证 Snapshot（Port）
   - Target Adapter 恢复状态（Adapter）
   - Target Composer 接管（Composer）
5. 双方记录 ExecutionStep（Pattern: "Handoff"）
6. 源端可选择释放资源（Adapter）

### 3.3 Handoff 契约示例
```go
type HandoffContract struct {
    TaskType     string
    RequiredFields []string
    MaxPayloadSize int
    AllowedTargets []string
}
```

## 4. 最小可执行任务分解

### 阶段 1: 协议核心定义
**任务 1.1**  
在 `go-core/` 新建 `handoff.go`，定义：
- `HandoffRequest`
- `HandoffSnapshot`
- `HandoffContract` 接口
- `HandoffResult`

**任务 1.2**  
实现基础 `HandoffComposer` 工厂：
```go
func NewHandoff(source, target Composer, contract Port, transport Adapter) Composer
```

**任务 1.3**  
定义标准 Handoff ExecutionStep 模板（Unit: "Composer", Pattern: "Handoff", Details 包含 source/target）。

### 阶段 2: 状态与快照
**任务 2.1**  
实现 `SnapshotAdapter`：
- `CreateSnapshot(state interface{}) HandoffSnapshot`
- `RestoreSnapshot(snapshot HandoffSnapshot) interface{}`

**任务 2.2**  
在 task_scheduler 示例中，实现任务状态的 Snapshot（使用现有 Task struct）。

**任务 2.3**  
添加 Handoff 时的校验 Port（验证快照完整性 + 目标能力）。

### 阶段 3: 传输与安全
**任务 3.1**  
实现两种传输 Adapter：
- `InProcessHandoffAdapter`（同一进程模拟）
- `SimpleHTTPHandoffAdapter`（跨进程模拟）

**任务 3.2**  
在 Handoff 流程中加入超时和重试（复用文档 01 的模式）。

**任务 3.3**  
添加 Handoff Token 生成与验证（使用 Atom）。

### 阶段 4: 多智能体集成
**任务 4.1**  
扩展 task_scheduler，使其支持两个“Agent”：
- SchedulerAgent（负责分配）
- WorkerAgent（负责执行）
- 使用 Handoff 从 Scheduler 交给 Worker

**任务 4.2**  
在 Handoff 成功/失败时，双方都记录 ExecutionStep。

**任务 4.3**  
更新 calculator 示例，演示“计算任务”从一个“计算 Agent” Handoff 到“持久化 Agent”。

### 阶段 5: 观察与人类监督
**任务 5.1**  
确保所有 Handoff 步骤包含：
- source_agent_id
- target_agent_id
- snapshot_hash
- handoff_token

**任务 5.2**  
在前端仪表盘增加 “Agent Handoff” 专用视图（显示交接树）。

**任务 5.3**  
添加 `/api/handoff/history` 接口，返回历史交接记录。

### 阶段 6: 高级与演进
**任务 6.1**  
支持“回滚 Handoff”（使用 Snapshot 恢复源端）。

**任务 6.2**  
定义 Handoff 失败的标准化错误码。

**任务 6.3**  
编写 `docs/handoff/` 协议规范 + 序列图（ASCII 或 Mermaid）。

**任务 6.4**  
在 README 中加入“多智能体 Handoff 协议”章节。

**任务 6.5**  
模拟 1M LOC：设计一个包含 5 个 Agent 的工作流，使用 Handoff 链式传递任务，并验证全流程可观测。

## 5. 实现约束
- Handoff 必须是显式的 Composer 调用，不能是隐式函数调用
- 状态必须通过 Snapshot 传递，禁止全局变量
- 每次 Handoff 必须产生至少 4 条 ExecutionStep（请求、验证、传输、接管）

## 6. 风险与缓解
- 风险：状态快照过大 → 缓解：只传递必要 delta + 引用
- 风险：循环 Handoff → 缓解：Handoff Token + 最大深度检查（Port）
- 风险：协议不兼容 → 缓解：HandoffContract 版本控制

## 7. 验收标准
- 能用 HandoffComposer 在两个独立 Composer 间完成一次完整交接
- 交接全程在 ExecutionStep 中可追溯（trace 关联）
- 人类仪表盘能清晰看到 “Agent A → Agent B” 的交接记录
- 示例代码中演示了失败回滚场景

**完成此文档后，下一步**：执行任务 1.1（定义 handoff.go）。
