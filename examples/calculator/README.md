# 计算器示例（低熵架构验证）

## 位置
`examples/calculator/main.go`

## 严格遵守的 4 个单元

| 单元     | 本示例中的实现                  | 职责                     |
|----------|--------------------------------|--------------------------|
| **Atom** | `parseExpression`, `performCalculation` | 纯函数解析与计算         |
| **Port** | `CalculatorPort`               | 输入验证契约             |
| **Adapter** | `OutputAdapter`, `HistoryAdapter` | 输出打印 + 历史持久化 |
| **Composer** | `NewCalculatorPipeline()`     | Pipeline 编排整个流程    |

## 功能
- 支持基本四则运算：+ - * /
- 除零保护
- 格式验证
- 计算历史记录（使用 Adapter 持久化）
- 交互式 REPL

## 运行方式

```bash
cd /c/Users/Administrator/low-entropy-core
go run examples/calculator/main.go
```

输入示例：
- `5 + 3`
- `10 / 2`
- `history`
- `exit`

## 设计亮点
- 所有业务逻辑都在 Atom 中（纯函数）
- 验证逻辑强制通过 Port
- 所有副作用（打印、历史）集中在 Adapter
- 使用 Composer 进行受控编排
- 没有引入任何第 5 种抽象

这是对“低熵 + 4 单元”架构的中等规模验证。
