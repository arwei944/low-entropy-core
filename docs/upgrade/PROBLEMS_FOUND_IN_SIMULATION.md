# 1M LOC 模拟中发现的问题（基于三大升级实施后）

日期: 2026-06

## 模拟方法
- 基于 calculator 和 task_scheduler 示例扩展
- 实施了 Composer 模式、ExecutionStep 协议、Handoff
- 使用 go build/run 测试，代码审查

## 发现的主要问题

### 1. 核心实现问题（违反 4 单元或协议）
- **重复类型定义**：ExecutionStep 在 go-core/composer.go 和 observation.go 中重复声明。导致编译失败。
  - 原因：升级时没有统一到 observation.go
  - 影响：无法构建任何使用新代码的示例

- **步骤收集不完整**：
  - Branch/Parallel/WithRetry 等模式中，子 Composer.Run() 会清空自己的 Steps，但没有合并到父级。
  - 结果：观察协议中丢失子流程步骤，人类仪表盘看不到完整执行轨迹。
  - 示例：branch.Run() 后只看到顶层步骤。

- **Handoff 未遵守协议**：
  - NewHandoff 没有产生 Pattern: "Handoff" 的 ExecutionStep。
  - 没有调用 ObservationAdapter。
  - source Composer 未使用。
  - 违反 doc 03 要求“必须产生至少 4 条 ExecutionStep”。

- **缺少 Port 验证**：
  - 新模式和 Handoff 完全没有使用 Port。
  - 大量 interface{} 类型断言 (e.g. input.(Task)) ，运行时 panic 风险高。
  - 没有边界校验，违背 "Port is the second basic unit: contract / validation"。

### 2. 实现质量问题（低熵被破坏）
- 硬编码 trace ID："trace-1" 或 time based，但无父子关系正确传递。
- Parallel 不是真正的并发（for loop 顺序执行）。
- WithTimeout 完全假实现（忽略 duration）。
- 步骤中无 DurationMs 计算，无 Error 使用。
- 原子 (Atom) 仍是匿名 func，无类型安全。

### 3. 构建和运行环境问题
- **模块混乱**：
  - 根目录无 go.mod。
  - task_scheduler 无 go.mod，import "low-entropy-core/go-core" 无法解析。
  - calculator 有 replace，但因重复类型仍失败。
- 运行 task_scheduler 报 "package low-entropy-core/go-core is not in std"
- 备份文件仍包含旧代码，造成混淆。

### 4. 1M LOC 规模模拟问题（架构演化不足）
- **可观测性爆炸**：无采样、无聚合。1M LOC 项目中 Composer 步骤数会线性爆炸，仪表盘无法消费。
- **Handoff 状态问题**：仅 interface{}，无 schema 版本、无冲突解决、无回滚实现（虽然 doc 有计划）。
- **错误处理缺失**：模式中无 StepError 传播。真实系统错误会静默失败。
- **无资源约束**：无 Port 注入资源限制，规模大时无法防御。
- **并发和分布式**：Parallel 弱，Handoff 仅 in-proc，无真实网络 Adapter。
- **计算器集成不完整**：server.go 仍是 placeholder "result := 42.0"，未调用 types.go 的 RPN + 新模式。
- **人类监督不足**：虽有 /api/observation，但前端未更新为支持 trace 树；无全局视图。

### 5. 其他
- Atom/Port/Adapter 在新代码中角色弱化（主要是 Composer 膨胀）。
- 没有测试，类型断言到处。
- 原始 calculator 的内存、RPN 功能在更新中丢失或未集成。

## 建议修复（仍用 4 单元）
- 统一 ExecutionStep 到 observation.go，composer 引用。
- 在 Branch/Parallel 中合并 sub.Steps 到 parent。
- 在 Handoff 中添加步骤记录，使用 Pattern "Handoff"。
- 引入 Port 到模式入口做验证。
- 为 task_scheduler 添加 go.mod 和 replace。
- 添加步骤聚合逻辑到 ObservationAdapter。
- 扩展示例使用真实逻辑 + 模式。

这些问题显示，即使实施三大文档，当前实现离支持 1M LOC 恒定熵还有差距。需要继续迭代。
