# Low-Entropy Core — AI Agent 开发约束

> **TRAE / Claude / Cursor 等 AI 代理在修改此项目时必须遵循以下规则。**

---

## 1. 架构层级（强制）

项目采用 **8 层架构**，每层有明确的职责边界。新增代码必须先确定属于哪一层：

| 层级 | 名称 | 职责 | 依赖方向 |
|------|------|------|----------|
| **L0** | 错误处理 | `errors.go`, `errors_enhanced.go` | 无依赖（基础层） |
| **L1** | 四原语定义 | `atom.go`, `port.go`, `adapter.go`, `composer.go` | L0 |
| **L2** | 单机韧性 | `circuit_breaker.go`, `retry.go`, `ratelimit.go`, `bulkhead.go` | L0, L1 |
| **L3** | 分布式韧性 | `patterns_distributed.go`, `fastpath.go`, `perf_core.go`, `observation.go` | L0-L2 |
| **L4** | Guardian 监督 | `guardian.go`, `guardian_types.go`, `guardian_threshold.go`, `guardian_entropy.go` | L0-L3 |
| **L5** | Observation 可观测 | `observation_aggregator.go`, `observation_api.go`, `observation_pipeline.go` | L0-L4 |
| **L6** | EventStore 事件溯源 | `eventstore.go`, `eventstore_wal.go`, `snapshot.go` | L0-L5 |
| **L7** | 应用层 | `cmd/` 下的入口文件 | L0-L6 |

**约束规则**：
- **禁止反向依赖**：低层代码不得 import 高层包
- **禁止跨层跳跃**：L3 不能直接依赖 L5
- **禁止循环依赖**：任何两个文件之间不得形成 import 环

---

## 2. 四原语模式（L1 核心）

所有业务逻辑必须使用以下四种原语实现，**禁止裸写 func**：

| 原语 | 接口 | 用途 |
|------|------|------|
| **Atom** | `Atom[Input, Output]` | 最小计算单元，纯函数 |
| **Port** | `Port[Input, Output]` | 外部边界（I/O、网络、消息队列） |
| **Adapter** | `Adapter[Input, Output]` | 协议转换/格式适配 |
| **Composer** | `Composer[Input, Output]` | 编排多个 Atom/Port/Adapter |

**检查方式**：`arch-manager.exe` 启动后访问 `http://localhost:8090/api/primitives` 查看当前原语实现。

---

## 3. 代码规范

- **包名**：所有核心代码在 `package core` 中，入口文件在 `package main`
- **文件命名**：`snake_case.go`，按功能模块命名（如 `guardian_entropy.go`）
- **导出规则**：所有公开类型/函数必须大写开头
- **测试文件**：`*_test.go`，必须与源文件同目录
- **Build Tags**：桩文件使用 `//go:build !lecore_tier4` 排除

---

## 4. 开发前检查清单

在提交任何代码变更前，AI 代理必须：

1. **确认层级归属**：新代码属于 L0-L7 哪一层？
2. **确认原语类型**：是 Atom / Port / Adapter / Composer 中的哪一种？
3. **检查依赖方向**：import 的包是否都来自同层或更低层？
4. **运行架构检查**：启动 `arch-manager.exe` 后访问 `/api/violations`，确保违规数为 0
5. **验证原语注册**：访问 `/api/primitives`，确认新原语已被识别

---

## 5. 禁止事项

- 禁止在业务代码中直接使用 `fmt.Println`（应通过 Observation Pipeline）
- 禁止绕过 Guardian 阈值直接修改状态
- 禁止在 L0-L3 层中 import 外部第三方库（除标准库外）
- 禁止创建新的 `.go` 文件而不定义所属层级
- 禁止在同一个 commit 中跨层修改（L3 和 L5 的修改应分开提交）

---

## 6. 仪表盘验证

每次代码变更后，访问 `http://localhost:8090/arch-manager.html` 确认：

- 健康评分 ≥ 80（A 级或以上）
- 违规数 = 0
- 拓扑图无孤立节点
- 文件树中新增文件已正确归属层级

---

## 7. 快速参考

```bash
# 编译并启动仪表盘
cd c:\Users\Administrator\low-entropy-core
go build -tags lecore_tier4 -o arch-manager.exe ./cmd/arch-manager/
.\arch-manager.exe

# 检查违规
curl http://localhost:8090/api/violations

# 检查架构数据
curl http://localhost:8090/api/arch | jq .layers
```