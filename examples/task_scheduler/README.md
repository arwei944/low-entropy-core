# 任务调度系统示例（中等规模验证）

## 位置
examples/task_scheduler/main.go

## 设计说明
严格只使用 4 个基本单元：
- Atom：状态转换 (transitionState)、资源分配 (allocateResource)
- Port：任务验证契约 (TaskPort)
- Adapter：持久化 (PersistenceAdapter)、日志 (LogAdapter)
- Composer：Pipeline 编排

## 运行方法（在用户真实机器上）

cd /c/Users/Administrator/low-entropy-core

go run examples/task_scheduler/main.go

## 预期输出
=== 任务调度系统示例（纯 4 单元实现） ===
[Log] 开始调度任务
[Atom] 任务 task-042 状态 -> running
[Atom] 为 task-042 分配资源
[Adapter] 持久化任务 task-042 状态: completed
[Log] 调度完成

最终任务状态: {ID:task-042 Status:completed Data:map[resource:cpu-1]}

## 注意
- 使用了相对 import "./go-core" 以便直接从项目根运行。
- 如果报 module 错误，在你的机器上可能需要先进入 go-core 目录测试，或在根目录初始化简单 go.mod。
