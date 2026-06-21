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
- **文件行数约束（强制）**：每个 `.go` 文件（含源文件和测试文件）的实际代码行数 **不得超过 300 行**
  - 超限时必须按功能/按层级拆分，拆分后的每个文件仍需 ≤ 300 行
  - 行数统计以 `go build` 实际读取的代码行为准（不含空行/注释不计，但建议保守估算）
  - 测试文件同样适用：一个 `*_test.go` 超过 300 行需按测试类型（如 `*_types_test.go`、`*_pipeline_test.go` 等）拆分

---

## 4. 开发文档与验收标准（强制）

**任何代码提交前，必须先编写文档。禁止先写代码后补文档。**

### 4.1 最小任务单元拆分

开发任务必须先拆分为**最小可交付单元**，每个任务单元满足以下条件：

- **任务标题**：一句清晰描述该任务的目标
- **任务描述**：包括目标、背景、输入、输出
- **层级归属**：L0-L7 中哪一层或哪些层
- **原语类型**：涉及的 Atom/Port/Adapter/Composer
- **依赖项**：该任务依赖的其他任务
- **验收标准**：明确的、可执行的通过条件

**任务单元示例：**

```
任务单元1：新增 L1 Atom 实现 XXX
  ├── 目标：实现 XXX 纯函数
  ├── 输入：...
  ├── 输出：...
  ├── 层级：L1
  ├── 原语：Atom
  ├── 依赖：无
  └── 验收标准：
        1. 纯函数无副作用
        2. go test ./... 通过
        3. 文件 ≤ 300 行
        4. 架构违规数 = 0

任务单元2：编写 XXX 的单元测试
  └── 验收标准：
        1. TestXXX 覆盖主要分支
        2. 测试函数命名明确
        3. 测试文件 ≤ 300 行
```

### 4.2 严格验收标准（AC — Acceptance Criteria）

每个任务单元**必须包含以下验收标准**，缺少以下任何一项的任务不允许提交代码：

| 验收项 | 说明 | 判定方式 |
|---|---|---|
| **1. 功能正确** | 代码实现了任务描述的功能 | `go test ./...` 全部通过 |
| **2. 层级合规** | 文件归属正确的 L0-L7 层 | arch-manager `/api/violations` = 0 |
| **3. 依赖方向合规** | 无反向/跨层/循环依赖 | arch-manager 拓扑图检查 |
| **4. 四原语模式** | 使用 Atom/Port/Adapter/Composer | 代码审查 + arch-manager `/api/primitives` |
| **5. 文件行数合规** | 所有新增/修改文件 ≤ 300 行 | 行数统计脚本 |
| **6. 可观测性** | 关键流程有 Observation 接入 | 检查 Observation Pipeline |
| **7. 测试覆盖** | 核心逻辑有对应的 `*_test.go` | `go test -cover` |
| **8. 编译通过** | 整个项目 `go build` 无错误 | `go build ./...` |
| **9. 无 fmt.Println** | L0-L6 禁止裸写 print | 代码审查 |
| **10. 文档完整性** | 开发文档完整记录了设计决策 | 文档审查 |

**验收标准必须是可判定的、可自动化执行的**，不得使用模糊描述（如"代码质量好"、"性能优秀"）。

### 4.3 文档结构（每个任务单元的文档模板）

```markdown
## 任务：[任务名称]

### 目标
一句话描述

### 背景
为什么需要这个任务

### 输入输出
- 输入：...
- 输出：...

### 架构设计
- 层级归属：Lx
- 原语类型：Atom / Port / Adapter / Composer
- 依赖关系：... → ...

### 实现步骤
1. ...
2. ...

### 验收标准（严格 AC）
1. [ ] 功能正确：...
2. [ ] 层级合规：...
3. [ ] 文件行数 ≤ 300 行
4. [ ] go test ./... 全部通过
5. [ ] arch-manager 违规数 = 0

### 风险与注意事项
- ...
```

---

## 5. 开发前检查清单

在提交任何代码变更前，AI 代理必须：

1. **编写开发文档**：拆分最小任务单元，明确每个单元的验收标准
2. **确认层级归属**：新代码属于 L0-L7 哪一层？
3. **确认原语类型**：是 Atom / Port / Adapter / Composer 中的哪一种？
4. **检查依赖方向**：import 的包是否都来自同层或更低层？
5. **检查文件行数**：所有新增或修改的 `.go` 文件必须 ≤ 300 行，测试文件同样适用。超限时必须按功能拆分
6. **运行架构检查**：启动 `arch-manager.exe` 后访问 `/api/violations`，确保违规数为 0
7. **验证原语注册**：访问 `/api/primitives`，确认新原语已被识别
8. **验证验收标准**：对照 4.2 节的 10 项验收标准全部满足

---

## 6. 禁止事项

- 禁止在业务代码中直接使用 `fmt.Println`（应通过 Observation Pipeline）
- 禁止绕过 Guardian 阈值直接修改状态
- 禁止在 L0-L3 层中 import 外部第三方库（除标准库外）
- 禁止创建新的 `.go` 文件而不定义所属层级
- 禁止在同一个 commit 中跨层修改（L3 和 L5 的修改应分开提交）
- **禁止创建或保留超过 300 行的 `.go` 文件**：源文件和测试文件均受此约束。超出时必须按功能/按层级拆分为多个小文件（如 `core_types_test.go`、`core_pipeline_test.go`）。拆分后的每个文件仍需单独校验 ≤ 300 行
- **禁止先写代码后补文档**：任何开发任务必须先拆分最小任务单元并编写验收标准，然后才能开始写代码。缺少文档和验收标准的代码不得提交
- **禁止模糊的验收标准**：不得使用"代码质量好"、"性能优秀"等不可判定的描述，所有验收标准必须可通过自动化工具或明确步骤验证

---

## 7. 仪表盘验证

每次代码变更后，访问 `http://localhost:8090/arch-manager.html` 确认：

- 健康评分 ≥ 80（A 级或以上）
- 违规数 = 0
- 拓扑图无孤立节点
- 文件树中新增文件已正确归属层级
- **所有文件行数 ≤ 300 行**：超大文件必须按功能拆分后才能提交
- **开发文档完备**：每个任务单元都有明确的文档和验收标准

---

## 8. 快速参考

```bash
# 编译并启动仪表盘
cd c:\Users\Administrator\low-entropy-core
go build -tags lecore_tier4 -o arch-manager.exe ./cmd/arch-manager/
.\arch-manager.exe

# 检查违规
curl http://localhost:8090/api/violations

# 检查架构数据
curl http://localhost:8090/api/arch | jq .layers

# 检查文件行数（超过 300 行需拆分）
# Windows PowerShell
Get-ChildItem -Recurse -Filter *.go | ForEach-Object {
    $lines = (Get-Content $_.FullName | Measure-Object -Line).Lines
    if ($lines -gt 300) { Write-Host "$($_.FullName): $lines 行（超限）" }
}

# 或者更简洁：
Get-ChildItem -Recurse -Filter *.go |
  Select-Object @{Name='Lines';Expression={(Get-Content $_.FullName | Measure-Object -Line).Lines}},FullName |
  Sort-Object Lines -Descending | Select-Object -First 10
```