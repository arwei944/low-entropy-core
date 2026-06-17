# 升级文档：Composer 组合模式库

## 1. 背景与目标
当前 Composer 仅支持线性 Pipeline。在 100 万行代码规模下，需要支持结构化编排（分支、并行、循环、错误恢复），但**必须严格使用现有 4 个原语**，禁止引入新抽象。

目标：在 4 原语基础上，建立可复用的 Composer 组合模式库，使复杂业务流程能以低熵方式表达。

## 2. 设计原则
- 所有模式必须通过 Atom、Port、Adapter、Composer 实现
- Composer 仍是唯一编排入口
- 模式以函数/结构体形式提供，不新增类型
- 所有模式必须产生标准 ExecutionStep（后续文档定义）
- 保持纯函数特性（Atom 内）

## 3. 核心组合模式（用 4 原语表达）

### 3.1 条件分支 (Branch)
- 使用 Atom 做条件判断
- 使用 Composer 选择不同 Pipeline

### 3.2 并行执行 (Parallel)
- 使用多个 Composer 并发（通过 Adapter 包装 goroutine）
- 使用 Adapter 汇聚结果

### 3.3 重试与超时 (Retry + Timeout)
- Retry 作为 Adapter 包装
- Timeout 使用 context（通过 Port 注入）

### 3.4 错误恢复 (Recover)
- 错误作为数据流动
- Recover Adapter 捕获并路由到恢复路径

### 3.5 循环 (Loop)
- 使用 Atom 判断循环条件
- Composer 内部递归调用（小心熵）

## 4. 最小可执行任务分解（单一任务）

### 阶段 1: 基础模式定义
**任务 1.1**  
在 `go-core/composer.go` 中添加 `Branch` 辅助函数：
```go
func Branch(condition Atom, truePath, falsePath Composer) Composer
```
要求：返回的 Composer 必须收集 ExecutionStep。

**任务 1.2**  
添加 `Parallel` 辅助：
```go
func Parallel(composers ...Composer) Composer
```
要求：使用 goroutine + Adapter 汇聚，输出合并步骤。

**任务 1.3**  
添加 `Retry` 包装：
```go
func WithRetry(c Composer, maxAttempts int, backoff Adapter) Composer
```

**任务 1.4**  
添加 `WithTimeout`：
```go
func WithTimeout(c Composer, duration time.Duration, timeoutAdapter Adapter) Composer
```

### 阶段 2: 错误与恢复
**任务 2.1**  
定义标准错误类型（在 Atom/Port 中流动，作为 interface{}）：
```go
type ExecutionError struct {
    Code    string
    Message string
    Recoverable bool
}
```

**任务 2.2**  
实现 `Recover` 模式：
```go
func Recover(c Composer, recoveryAdapter Adapter) Composer
```

**任务 2.3**  
更新 calculator 示例，使用新的 Branch 模式重构部分逻辑（例如金额计算中的条件）。

### 阶段 3: 循环与高级
**任务 3.1**  
实现 `While` 循环模式（使用 Atom 作为条件）。

**任务 3.2**  
在 task_scheduler 示例中，使用 Parallel + Retry 重构任务调度逻辑。

**任务 3.3**  
添加模式使用文档和单元测试（仅测试 4 单元组合）。

**任务 3.4**  
在 ExecutionStep 中增加 `Pattern` 字段，记录使用了哪个模式（如 "Branch", "Parallel"）。

### 阶段 4: 验证与集成
**任务 4.1**  
创建 `docs/patterns/` 目录，编写每个模式的使用示例和 ExecutionStep 输出示例。

**任务 4.2**  
更新 README，加入“Composer 组合模式库”章节。

**任务 4.3**  
在 1M LOC 模拟中，挑选一个模块（例如订单处理），用新模式重写流程，并记录熵指标（步骤数量、Adapter 调用次数）。

**任务 4.4**  
添加 GitHub Action 检查：确保没有使用除 4 原语外的其他控制流关键字（可选，使用静态分析）。

## 5. 实现约束
- 禁止在 Atom 里写 if/else 业务逻辑（条件必须是独立 Atom）
- 所有新模式必须输出 ExecutionStep，包含 Unit、Action、Details、Pattern
- 并发模式必须通过 Adapter 隔离副作用

## 6. 风险与缓解
- 风险：递归导致栈溢出 → 缓解：限制递归深度，使用尾递归风格或迭代
- 风险：并行导致步骤顺序不稳定 → 缓解：步骤带 timestamp 和 parent ID
- 风险：模式库膨胀 → 缓解：严格控制模式数量，先实现 5 个核心模式

## 7. 验收标准
- 能用 Branch/Parallel/Retry 表达一个完整业务流程
- 所有流程的 ExecutionStep 可被仪表盘完整渲染
- 代码行数增加但“模式复杂度”不增加（通过步骤计数衡量）

**完成此文档后，下一步**：开始执行任务 1.1。
