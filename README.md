# 低熵核心 (Low-Entropy Core)

**由最基本的 4 个原语组成的极简后端架构**，目标是让复杂度和熵值在百万行甚至亿行代码规模下保持恒定。

人类作为**纯旁观监督者**，通过仪表盘观察系统运行，不参与任何代码编写。所有开发由多智能体完全自主完成，并支持无缝交接。

## 核心理念

- **严格 4 原语**：Atom（原子操作）、Port（端口/边界）、Adapter（适配器/副作用）、Composer（编排器）
- **零熵增长**：任何新功能都必须通过这 4 个原语实现，禁止引入第 5 种抽象
- **人类只观察**：所有状态通过结构化数据（ExecutionStep）实时暴露给仪表盘
- **多智能体自主**：智能体之间通过明确协议无缝交接，人类仅监督

## 4 个原语定义

| 原语       | 职责                     | 特点                  |
|------------|--------------------------|-----------------------|
| **Atom**   | 纯计算、解析、核心逻辑    | 纯函数，无副作用      |
| **Port**   | 边界验证、输入校验        | 防御性编程            |
| **Adapter**| 副作用、持久化、外部交互  | 唯一允许副作用的位置  |
| **Composer**| 编排 Pipeline，收集步骤   | 单一事实来源          |

## 示例：完整计算器

本仓库包含一个使用 4 原语实现的**完整计算器**示例，证明架构在真实场景下的可用性。

### 已实现功能（完整版）
- 括号、优先级、乘方 (^)
- 模运算 (%)
- 内存功能 (MC / MR / M+)
- 文件持久化历史 (HistoryAdapter 写入 history.txt)
- 连续计算（= 后结果自动作为下次左操作数）
- 可点击历史复用
- 实时 4 单元执行步骤日志（彩色区分）
- 低熵监控面板（累计计算、步数、历史条数）
- 完整按钮网格 + 键盘支持

运行方式：
```bash
cd examples/calculator
go run server.go types.go
# 然后访问 http://localhost:8081
```

## 如何使用本架构

1. 在 `go-core/` 中实现 4 个原语
2. 所有业务逻辑必须拆分到 Atom / Port / Adapter
3. Composer 负责组装并输出 ExecutionStep
4. 人类只通过网页仪表盘观察

## 设计原则

- YAGNI + KISS
- 没有最优解，只有最适合当前上下文的解
- 所有外部输入默认不可信
- 可观测性优先（没有被监控的，就没有发生）
- 数据为王（先明确数据量级、访问模式）

## 许可证

MIT License

## 贡献

本项目由多智能体自主开发，人类仅监督。欢迎通过 Issue 或 PR 提出对架构的改进建议（需严格遵守 4 原语原则）。

---

**项目目标**：为 1 亿行代码规模的后端系统提供一个熵值永不增长的底层基础。

## 架构升级路线图（100万行规模模拟）

为支撑大规模系统，我们已制定三大核心升级文档（严格遵守 4 原语）：

- [Composer 组合模式库](docs/upgrade/01-composer-patterns.md)
- [标准化 ExecutionStep + 通用观察协议](docs/upgrade/02-executionstep-protocol.md)
- [正式的 Agent Handoff 协议](docs/upgrade/03-agent-handoff-protocol.md)

每个文档都包含详细的**最小可执行任务分解**。

查看完整索引： [docs/upgrade/README.md](docs/upgrade/README.md)

## 三大升级已全部完成交付 (2026-06)

详见 docs/upgrade/DELIVERY.md 和 docs/upgrade/README.md

核心实现:
- Composer 模式库 (Branch, Parallel 等)
- 标准化 ExecutionStep + 观察协议
- Agent Handoff 协议

所有符合 4 单元低熵原则。
