# Agent Workbench 端到端演示

基于 Low-Entropy Core 四大原语（Atom / Port / Adapter / Composer）的 Agent 工作台完整生命周期演示。

## 概述

本示例模拟了一个多 Agent 协作场景：Agent A（计算专家）编写代码并通过 StaticGuardPort 静态审核 → AgentRunner 编译执行 → 通过 HandoffComposer 将 DevSnapshot 传递给 Agent B（展示专家）→ Agent B 渲染结果。同时演示了 Guardian 违规拦截与修复流程。

## 架构

```
AgentCodeSubmission {源码 + Manifest}
        │
        v
DefaultAgentWorkbench (Composer)
  ├── Submit() ──► StaticGuardPort (Port) ──► DecisionEngine (Composer)
  └── SubmitAndRun() ──► Submit + AgentRunner (Adapter) 编译执行
        │
        v
HandoffComposer ──► Agent A ──► DevSnapshot ──► Agent B 接收并渲染
```

### 四大原语

| 原语 | 角色 | 演示中的实例 |
|------|------|-------------|
| **Atom** | 纯函数，无副作用，确定性转换 | Manifest 中声明 `computeResult` |
| **Port** | 合约验证网关，在系统边界校验 | `StaticGuardPort`（5 项静态检查） |
| **Adapter** | 副作用边界，I/O/持久化 | `AgentRunner`、`InMemoryObservationAdapter`、`InProcHandoffTransport` |
| **Composer** | 编排引擎，组合其他原语 | `DefaultAgentWorkbench`、`HandoffComposer`、`SchedulerComposer`、`DecisionEngine` |

## 运行

```bash
cd examples/agent-demo
go run main.go
```

**前置条件：**
- Go 1.22+
- 本地 `go-core` 模块可用（通过 `go.mod` 中的 `replace` 指令指向 `../../go-core`）
- 系统安装 Go 工具链（AgentRunner 内部调用 `go build` 编译 Agent 提交的代码）

## 7 步流程

### 步骤 1：初始化基础设施
创建 AgentPool、TaskQueue、SchedulerComposer、StaticGuardPort、DecisionEngine、AgentRunner、DefaultAgentWorkbench。

### 步骤 2：注册 Agent
注册 Agent A（`compute` 能力）和 Agent B（`render` 能力），发送心跳确认在线。

### 步骤 3：Agent A 编写合规代码并提交
Agent A 编写计算 `3+5*2` 的 Go 代码，附带 Manifest 声明，通过 `SubmitAndRun` 完成审核→编译→执行全流程。

### 步骤 4：ExecutionStep 时间线
打印完整的执行步骤追踪链，包含每一步的 Unit、Action、Details、Duration。

### 步骤 5：Handoff 接力
Agent A 构建 DevSnapshot（含产物、决策、约束），通过 HandoffComposer 传递给 Agent B，使用 SHA256 校验和保证完整性。

### 步骤 6：Agent B 渲染
Agent B 通过校验和取回 DevSnapshot，验证完整性后渲染结果。

### 步骤 7：Guardian 介入
- **7a**：Agent A 提交违规代码（直接使用 `http.Client` 未通过 Adapter 包装）→ StaticGuardPort 检测到 `primitive_compliance` 违规 → 返回 Violations 列表
- **7b**：Agent A 根据修复建议将 `http.Client` 包装为 Adapter 原语，调整 Manifest 类型为 `Adapter`、层级为 `L7`，重新提交 → 审核通过 → 编译执行成功

## 关键技术点

### Manifest 承诺机制
Agent 提交代码时必须附带 `PrimitiveManifest`，声明每个原语的类型、名称、层级、输入输出类型。StaticGuardPort 会对比 Manifest 与 AST 代码的一致性。

### StaticGuardPort 5 项检查
1. **原语合规**：代码中使用的类型必须在 `GlobalPrimitiveTypeSet` 白名单中
2. **外部依赖**：import 限定在 go-core + 标准库白名单
3. **层级合规**：Port L1-L3，Adapter L5-L7，跨层距离 >3 层警告
4. **复杂度**：函数行数 ≤50、嵌套深度 ≤3、圈复杂度 ≤10
5. **Manifest 一致性**：声明与代码定义双向匹配

### Handoff 多 Agent 接力协议
- Agent A 构建 `DevSnapshot` 并计算 SHA256 校验和
- `HandoffComposer` 建立 `HandoffContract`（5 分钟过期）
- Agent B 通过校验和取回快照并验证完整性

### 统一观察层
所有组件通过 `InMemoryObservationAdapter` 记录 `ExecutionStep`，形成完整 Trace Tree，支持全链路追踪。

## 期望输出

```
======================================================================
  Phase 2 P5 端到端演示：Agent Workbench 完整生命周期
  AgentCodeSubmission → StaticGuardPort → AgentRunner → Handoff
======================================================================

[1/7] 初始化基础设施：AgentPool + TaskQueue + SchedulerComposer
[2/7] 注册 Agent A（计算专家）和 Agent B（展示专家）
[3/7] Agent A 编写合规代码（计算 3+5*2）并提交
[4/7] ExecutionStep 时间线（完整审核→编译→执行追踪）
[5/7] Handoff: Agent A → Agent B（DevSnapshot 传递）
[6/7] Agent B 接收 DevSnapshot 并渲染结果
[7/7] Guardian 介入演示：违规代码 → Block → 修复 → 重新提交

======================================================================
  演示完成：Phase 2 P5 Agent Workbench 端到端流程
======================================================================
```

## 相关文件

- `main.go` — 主演示程序
- `go.mod` — 模块定义，通过 `replace` 指向 `../../go-core`
- `../../go-core/` — Low-Entropy Core 框架核心库