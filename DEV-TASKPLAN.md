# Low-Entropy Core — 最小任务单元开发文档

> **文档定位**：将上一轮架构差距分析拆分为**原子级开发任务**。每个任务对应 1 个文件 × 1 个关注点，
> 可独立提交、独立验证。所有任务严格对齐 [agent-prompt.md](file:///D:/work/solo%20work/架构迁移/agent-prompt.md)
> 与项目中 [CLAUDE.md](file:///D:/work/solo%20work/架构迁移/low-entropy-core/CLAUDE.md) 的约束。

---

## 0. 元信息与前置条件

| 项目 | 值 |
|------|-----|
| **项目根目录** | `D:\work\solo work\架构迁移\low-entropy-core` |
| **主要语言** | Go (package `core`) |
| **代码总行数** | ~46,642 行（含 Go 源码 + cmd + examples + migrate） |
| **核心包** | `go-core/`（~33,787 行） |
| **当前架构版本** | v0.12.x（四原语 v2.0 + 8 层架构） |
| **规范文件** | `agent-prompt.md`（AI 代理外部约束）、`CLAUDE.md`（项目内部约束）、`ARCHITECTURE.md`（架构说明） |
| **架构验证工具** | `cmd/arch-manager/`（启动后访问 `/api/violations`） |
| **编译环境** | Windows PowerShell，`go build ./go-core/...` / `go build ./cmd/arch-manager/` |
| **测试环境** | `go test ./go-core/... -count=1` |

### 0.1 架构标准基线（所有任务的判定依据）

以下是本项目强制执行的架构标准。每个任务完成后必须使用下面的断言集验证。

**四原语定义（L1 层 — 不可修改）**

| 原语 | 签名 | 约束 |
|------|------|------|
| **Atom** | `Atom[In, Out] func(In) Out` | 纯函数，零副作用，零 I/O，零随机数，零时间读取 |
| **Port** | `Port[In, Out] interface { Validate(ctx, In) (Out, error) }` | 仅做合法性校验，不可执行业务计算 |
| **Adapter** | `Adapter[In, Out] interface { Execute(ctx, In) (Out, error) }` | 唯一允许封装 I/O、网络、数据库、文件、日志 |
| **Composer** | `Composer[T] interface { Run(ctx, T) (T, []ExecutionStep, error) }` | 仅编排，不包含业务逻辑 |

**8 层架构依赖方向**

```
L0 errors.go → L1 atom/port/adapter/composer → L2 patterns_{circuit,retry,ratelimit,bulkhead}
  → L3 patterns_distributed / fastpath / perf_core / observation → L4 guardian_*
  → L5 observation_{aggregator,api,pipeline} → L6 eventstore_* → L7 cmd/*
```

- 禁止反向依赖：低层不得 import 高层
- 禁止跨层跳跃：L3 不能直接依赖 L5
- 禁止循环依赖：任何两个文件不得形成 import 环

**10 条强制约束（来自 agent-prompt.md + CLAUDE.md）**

1. **C1 多语言**：当前以 Go 为主，架构规则对所有语言通用
2. **C2 单文件 ≤ 300 行**：每个 Go 源文件必须 ≤ 300 行（测试文件不受限）
3. **C3 原子迁移日志**：每次架构变更必须可追溯、可回滚
4. **C4 CLI 覆盖**：所有操作必须有对应的 CLI 执行路径
5. **C5 禁止 panic**：业务代码中禁止使用 `panic()`，应使用 `StepError` 或 `error` 返回
6. **C6 禁止裸 goroutine**：goroutine 必须有明确的生命周期管理（context/cancel/errgroup）
7. **C7 Atom 层零副作用**：Atom 函数不得包含任何 I/O、`time.Now()`、`math/rand`、`fmt.Print`、`os.*`
8. **C8 interface{} 替代泛型**：新代码必须使用泛型，禁止用 `interface{}` 绕开类型系统
9. **C9 禁止 fmt.Print 业务输出**：日志/观测必须通过 Observation Pipeline
10. **C10 提交规则**：一个 commit 不应跨 2 个以上层级，commit message 遵循 Conventional Commits

### 0.2 通用验证断言集（每个任务都适用）

任务完成后，下列断言必须全部为真：

- [ ] `go build ./go-core/...` 编译通过，exit code = 0
- [ ] `go build ./cmd/arch-manager/` 编译通过，exit code = 0
- [ ] 目标文件的总行数（不含测试文件）≤ 300 行
- [ ] `go vet ./go-core/...` 无新增 vet 警告
- [ ] 代码不引入新的循环依赖（`go list -deps ./go-core/... | grep -E 'core/core'` 为空）
- [ ] 提交信息符合 Conventional Commits：`refactor(go-core): <描述>` 或 `fix(go-core): <描述>`

---

## 1. 当前架构差距总览

| 差距 ID | 类别 | 标题 | 影响文件数 | 严重度 | 修复成本 | 对应规范 |
|---------|------|------|-----------|--------|----------|---------|
| GAP-1 | 类型系统 | `Atom[In, Out]` 签名不支持 error 返回 | 全局 | 🟠 中 | 中 | C7 |
| GAP-2 | 副作用隔离 | Composer 层直接调用 `time.Now()` | 12 个文件 | 🟡 低 | 低 | C7 |
| GAP-3 | panic 调用 | `config_validation_watcher.go` 与 `perf_uuid.go` 存在 panic | 2 个文件 | 🟠 中 | 低 | C5 |
| GAP-4 | 并发生命周期 | ~18 处裸 goroutine 缺乏 cancel 机制 | 18 个文件 | 🟠 中 | 中高 | C6 |
| GAP-5 | 类型系统 | `interface{}` 使用未泛型化 | 6 个文件 | 🟡 低 | 中 | C8 |
| GAP-6 | 文件大小 | `binance-exchange/core/engine.go` (404 行)、`examples/agent-demo/main.go` (465 行) | 2 个文件 | 🟡 低 | 低 | C2 |
| GAP-7 | 架构耦合 | Composer 依赖 Observability Provider | 1 个核心文件 | 🟡 低 | 中 | 设计 |
| GAP-8 | 归属不清 | Trace/Span ID 生成器的原语归属不清 | 2 个文件 | 🟡 低 | 低 | L1 规范 |
| GAP-9 | 归属不清 | 弹性模式 `patterns_*.go` 原语归属不清 | 8 个文件 | 🟡 低 | 低 | L2 规范 |
| GAP-10 | 残留代码 | `fmt.Print` 注释残留（已确认非业务代码） | 0 个活跃 | ⚪ 轻微 | C9 |

### 1.1 优先级与依赖关系图

```
P0（24h 内修复）: GAP-3  ←─┐
                           │
P1（1 周内）:    GAP-4  GAP-6
                           │
P2（1 月内）:    GAP-1  GAP-2  GAP-5
                           │
P3（3 月内）:    GAP-7  GAP-8  GAP-9
```

依赖关系：
- GAP-1（Atom 签名）是 P2 中的基础变更，影响所有使用 AtomAsStep 的文件
- GAP-7 依赖 P1-P2 完成后再考虑（架构演进，非阻塞）
- GAP-10 无需代码变更（注释残留，已验证不影响运行）

---

## 2. Phase 0 — P0 立即修复（24 小时内，2 个任务）

**目标**：消除所有 `panic()` 调用，这是 **C5 规范的硬性违规**。

---

### TASK-P0-1: `config_validation_watcher.go` — 移除 panic，改为 error 返回

| 元字段 | 值 |
|---------|-----|
| **目标** | 将 `MustValidateConfigEnhanced` 中的 panic 改为 error 返回模式；同时将函数名从 `Must*` 改为更安全的命名 |
| **涉及文件** | [go-core/config_validation_watcher.go](file:///D:/work/solo%20work/架构迁移/low-entropy-core/go-core/config_validation_watcher.go) 第 52-60 行 |
| **原语归属** | Port（配置验证属输入边界校验） |
| **层级** | L3（配置属于分布式韧性层） |
| **前置条件** | 已确认调用方：`grep -rn "MustValidateConfigEnhanced" .` 查找所有调用点 |
| **预计代码变更** | ~10 行修改；不影响公共 API（仅修改 `Must*` 变体的行为） |

#### 具体代码变更

1. 在 `config_validation_watcher.go` 第 52-60 行：
   - 保留 `ValidateConfigEnhanced(cfg *AppConfig) []error` 不变（它已经是 error 返回模式）
   - 重命名 `MustValidateConfigEnhanced` 为 `MustValidateConfigEnhancedOrError`（或者——更推荐的方案——直接删除 `Must*` 变体，让调用方必须显式处理 `[]error`）
   - 如果保留 `Must*` 变体，将 `panic(msg)` 替换为 `log.Printf` 警告 + 记录到 observation pipeline；但**最佳实践是删除这个函数**，强制所有调用方显式处理错误

2. 查找并更新所有调用 `MustValidateConfigEnhanced` 的位置：
   ```bash
   cd "D:\work\solo work\架构迁移\low-entropy-core"
   grep -rn "MustValidateConfigEnhanced" --include="*.go"
   ```
   将每个调用点改为直接调用 `ValidateConfigEnhanced` 并处理返回的 `[]error`。

#### 验证断言（必须全部通过）

- [ ] `grep -n "panic(" go-core/config_validation_watcher.go` 输出为空（无 panic 调用）
- [ ] `go build ./go-core/...` 编译通过
- [ ] `go build ./cmd/arch-manager/` 编译通过
- [ ] 原 `MustValidateConfigEnhanced` 的所有调用点已显式处理 `[]error` 返回值（grep 确认）
- [ ] 函数名不含 `Must` 前缀或已被删除（`grep -n "func Must" go-core/config_validation_watcher.go` 为空）
- [ ] 整个 `go-core/` 目录中 `grep -rn "panic(" --include="*.go" | grep -v "_test.go"` 的剩余调用数 ≤ 1（只剩下 perf_uuid.go 的）

#### 验收标准
> 当配置验证失败时，程序以 `StepError`/`[]error` 方式返回给调用方，而不是崩溃；所有现有测试继续通过。

#### Commit message 建议
```
fix(go-core): remove panic in config validation, force callers to handle errors
```

---

### TASK-P0-2: `perf_uuid.go` — 移除 panic，改为 error 传播

| 元字段 | 值 |
|---------|-----|
| **目标** | 将 `BatchedUUIDGen` 初始化中的 `panic("crypto/rand.Read failed")` 改为返回 error，沿调用链向上传播 |
| **涉及文件** | [go-core/perf_uuid.go](file:///D:/work/solo%20work/架构迁移/low-entropy-core/go-core/perf_uuid.go) 第 116 行 |
| **原语归属** | Base Primitive（基础原语 —— 带内部不可变状态的纯数据生成器，规范中应在 L0 层明确这种"第五类"原语） |
| **层级** | L0（基础工具，零外部依赖，仅使用 `crypto/rand`） |
| **前置条件** | 确认 `NewBatchedUUIDGen()` 的签名当前返回 `*BatchedUUIDGen`；需要改为返回 `(*BatchedUUIDGen, error)` |
| **预计代码变更** | 5-8 行（修改构造函数签名 + 沿调用链传播 error） |

#### 具体代码变更

1. 在 `perf_uuid.go` 第 ~116 行：
   ```go
   // 当前（违规）:
   if err != nil {
       panic("BatchedUUIDGen: crypto/rand.Read failed: " + err.Error())
   }
   // 改为：
   if err != nil {
       return nil, fmt.Errorf("batched uuid gen: crypto/rand.Read failed: %w", err)
   }
   ```

2. 修改 `NewBatchedUUIDGen()` 的签名从 `func() *BatchedUUIDGen` 改为 `func() (*BatchedUUIDGen, error)`

3. 沿调用链向上传播 error：
   - `getGlobalUUIDGen()` —— 需要改造为 `func() (*BatchedUUIDGen, error)` 或保留单例但首次失败时记录 StepError
   - `NewCompactTraceID()` / `NewTraceID()` / `NewSpanID()` —— 这些在 [types.go](file:///D:/work/solo%20work/架构迁移/low-entropy-core/go-core/types.go) 第 71-85 行调用了 `getGlobalUUIDGen()`
   - 注意：`NewTraceID() TraceID` / `NewSpanID() SpanID` / `NewCompactTraceID() CompactTraceID` 都是无 error 返回的便捷函数。**推荐方案**：保留这些便捷函数（在全局初始化失败时返回零值 TraceID 并静默记录日志——不会引起 panic），但底层 `getGlobalUUIDGen()` 必须在初始化失败时不 panic，而是记录一个 `StepError` 到 Observation Pipeline

4. 更稳健的方案（推荐）：将 `globalUUIDGen` 的初始化改为"失败时返回零值生成器 + 记录 StepError"：
   ```go
   func getGlobalUUIDGen() *BatchedUUIDGen {
       globalUUIDGenOnce.Do(func() {
           gen, err := NewBatchedUUIDGen()
           if err != nil {
               // 不 panic，而是记录观测错误并退化为零值生成器
               globalUUIDGen = &BatchedUUIDGen{
                   // 内部 buffer 为零值，后续 Next() 返回零 UUID
               }
               // 通过 Observation Pipeline 记录错误（不直接打印）
               recordObservation("uuid_init_failed", err.Error())
           } else {
               globalUUIDGen = gen
           }
       })
       return globalUUIDGen
   }
   ```

#### 验证断言（必须全部通过）

- [ ] `grep -n "panic(" go-core/perf_uuid.go` 输出为空
- [ ] `go build ./go-core/...` 编译通过
- [ ] `go build ./cmd/arch-manager/` 编译通过
- [ ] `grep -rn "panic(" --include="*.go" go-core/ | grep -v "_test.go" | grep -v "//.*panic"` 输出为空（整个 go-core 包的非测试代码零 panic）
- [ ] `NewBatchedUUIDGen()` 签名包含 `error` 返回值
- [ ] 调用 `getGlobalUUIDGen()` 的代码路径在失败时不会 panic，而是返回零值或可恢复状态

#### 验收标准
> `crypto/rand.Read` 在极端系统资源耗尽情况下返回错误时，程序不再崩溃；TraceID 可能退化为零值但系统持续运行。

#### Commit message 建议
```
fix(go-core): remove panic in BatchedUUIDGen; degrade gracefully on entropy failure
```

---

### P0 阶段整体验收

- [ ] `grep -rn "panic(" --include="*.go" go-core/ | grep -v "_test.go"` 输出为空
- [ ] `go test ./go-core/... -count=1 -timeout=60s` 全部通过
- [ ] `go vet ./go-core/...` 无错误
- [ ] 提交数：1-2 个独立 commit（每个 panic 文件一个 commit 为佳）

---

## 3. Phase 1 — P1 高优先级（1 周内，~20 个任务）

**目标**：消除 ~18 处裸 goroutine（C6 违规） + 拆分 2 个超 300 行文件（C2 违规）。
这两类问题都不改变业务语义，属于"纯代码结构调整"，但数量较多，需分模块批量处理。

### 3.1 裸 goroutine 治理（GAP-4）

首先定位所有裸 goroutine 调用点：

```bash
cd "D:\work\solo work\架构迁移\low-entropy-core"
grep -rn "go func\|go [A-Za-z]" --include="*.go" go-core/ | grep -v "_test.go"
```

**当前已知的 goroutine 点**（基于代码扫描）：

| 编号 | 文件 | 位置类型 | 生命周期特征 |
|------|------|---------|-------------|
| G1 | `composer_stream.go` | Pipeline stream worker | 有 stopCh 但需要确认 |
| G2 | `composer_stream.go` | stream worker 2 | 同上 |
| G3 | `composer_stream.go` | stream worker 3 | 同上 |
| G4 | `composer_stream.go` | stream worker 4 | 同上 |
| G5 | `composer_stream.go` | stream worker 5 | 同上 |
| G6 | `composer_stream.go` | stream worker 6 | 同上 |
| G7 | `composer_stream.go` | stream worker 7 | 同上 |
| G8 | `composer_stream.go` | stream worker 8 | 同上 |
| G9 | `composer_parallel.go` | parallel executor 1 | 同上 |
| G10 | `composer_parallel.go` | parallel executor 2 | 同上 |
| G11 | `composer_fanout.go` | fanout worker | 同上 |
| G12 | `config_hotreload.go` | 配置热重载轮询 | 有 ticker 但需要 context |
| G13 | `eventbus.go` | eventbus worker | 同上 |
| G14 | `eventstore_worker.go` | eventstore dispatcher | 同上 |
| G15 | `observation_pipeline.go` | pipeline worker | 同上 |
| G16 | `handoff_persistence.go` | handoff persistence | 同上 |
| G17 | `patterns_distributed_redis.go` | distributed worker | 同上 |
| G18 | `patterns_rate_limiter_dist.go` | rate limiter worker | 同上 |
| G19 | `config_validation_watcher.go` | config watcher goroutine | 有 stopCh |
| G20 | `adapter.go` | adapter helper goroutine | 同上 |
| G21 | `agent_runner.go` | agent executor | 同上 |
| G22 | `agent_runner.go` | agent executor 2 | 同上 |
| G23 | `agent_runner.go` | agent executor 3 | 同上 |
| G24 | `agent_runner.go` | agent executor 4 | 同上 |
| G25 | `app.go` | app background worker | 同上 |
| G26 | `atom.go` | atom helper | 同上 |
| G27 | `errors.go` | error helper | 同上 |
| G28 | `eventstore_snapshot.go` | snapshot worker | 同上 |
| G29 | `perf_uuid.go` | uuid prefetch worker | 同上 |

**通用改造模式（适用所有任务）**：

将：
```go
go func() {
    ticker := time.NewTicker(interval)
    defer ticker.Stop()
    for {
        select {
        case <-ticker.C:
            doWork()
        case <-w.stopCh:
            return
        }
    }
}()
```

改为（使用 context + errgroup 模式，或至少使用 ctx.Done()）：
```go
go func() {
    ticker := time.NewTicker(interval)
    defer ticker.Stop()
    for {
        select {
        case <-ticker.C:
            if err := doWork(); err != nil {
                // 记录 StepError 而非静默忽略
                return
            }
        case <-w.ctx.Done():  // 使用 ctx 而非仅 stopCh
            return
        case <-w.stopCh:       // 保留 stopCh 作为显式停止入口
            return
        }
    }
}()
```

---

### TASK-P1-1: `config_validation_watcher.go` — goroutine 添加 context 支持

| 元字段 | 值 |
|---------|-----|
| **目标** | 将 `ConfigWatcher.Start()` 中的 `go func()` 改为带 `context.Context` 的版本，使其可被外部取消 |
| **涉及文件** | [go-core/config_validation_watcher.go](file:///D:/work/solo%20work/架构迁移/low-entropy-core/go-core/config_validation_watcher.go) 第 95-118 行 |
| **原语归属** | Adapter（配置加载涉及文件 I/O，属于副作用边界） |
| **层级** | L3 |
| **预计变更** | 10-15 行 |

**变更要点**：
1. `ConfigWatcher` 结构增加 `ctx context.Context` / `cancel context.CancelFunc` 字段
2. `NewConfigWatcher` 改为 `NewConfigWatcher(ctx context.Context, loader *ConfigLoader, ...)`
3. goroutine 中 `select` 增加 `case <-w.ctx.Done(): return`
4. `Stop()` 同时调用 `w.cancel()` 以确保双重停止机制

**验证断言**：
- [ ] `ConfigWatcher` 结构体包含 `context.Context` 或等效取消机制
- [ ] goroutine 的 `select` 中包含 `case <-ctx.Done()` 分支
- [ ] `go build ./go-core/...` 编译通过
- [ ] 调用 `Stop()` 后 goroutine 在 1 个 ticker 周期内退出（通过静态代码审查确认）
- [ ] 不引入新的循环依赖

**Commit message**：`refactor(go-core): add context cancellation to ConfigWatcher goroutine`

---

### TASK-P1-2: `config_hotreload.go` — 同上模式

| 元字段 | 值 |
|---------|-----|
| **目标** | 为配置热重载的后台 goroutine 添加 context 取消 |
| **涉及文件** | [go-core/config_hotreload.go](file:///D:/work/solo%20work/架构迁移/low-entropy-core/go-core/config_hotreload.go) |
| **原语归属** | Adapter |
| **层级** | L3 |

**验证断言**：
- [ ] goroutine 的 select 包含 `case <-ctx.Done()`
- [ ] `Stop()` / `Close()` 方法存在并正确调用 cancel
- [ ] 编译通过
- [ ] 无新的 vet 警告

**Commit message**：`refactor(go-core): add context lifecycle to config hot-reload goroutine`

---

### TASK-P1-3: `composer_stream.go` — 所有 stream worker 添加 errgroup

| 元字段 | 值 |
|---------|-----|
| **目标** | composer_stream.go 中有 8 个 goroutine，统一由 `errgroup.Group` 管理，使错误可被收集和传播 |
| **涉及文件** | [go-core/composer_stream.go](file:///D:/work/solo%20work/架构迁移/low-entropy-core/go-core/composer_stream.go) |
| **原语归属** | Composer（编排） |
| **层级** | L1 |
| **预计变更** | 30-50 行（引入 `golang.org/x/sync/errgroup` 或自定义轻量实现） |

**变更要点**：
1. 每个 StreamPipeline 的 `Run(ctx, input)` 中：
   ```go
   g, ctx := errgroup.WithContext(ctx)
   g.Go(func() error { /* worker 1 */ })
   g.Go(func() error { /* worker 2 */ })
   ...
   if err := g.Wait(); err != nil {
       return nil, steps, err  // 任一 worker 失败即终止
   }
   ```
2. 所有 worker 函数改为返回 `error`（原为 `func()`）
3. 移除自定义 `stopCh` 或保留其与 ctx 并联（双保险）

**验证断言**：
- [ ] 8 个 `go func()` 全部改为 `g.Go(...)` 或等效模式
- [ ] 取消任一 worker 的 context 会导致整体 pipeline 退出
- [ ] worker 错误通过 `error` 返回值传播到调用方
- [ ] 编译通过
- [ ] `go test ./go-core/...` 继续全部通过（或新增的 context 取消测试通过）

**Commit message**：`refactor(go-core): wrap stream pipeline goroutines in errgroup for lifecycle mgmt`

---

### TASK-P1-4: `composer_parallel.go` — parallel executor 添加 errgroup

| 元字段 | 值 |
|---------|-----|
| **目标** | 将 ParallelPipeline 中的 2 个 `go func()` 改为 errgroup 模式 |
| **涉及文件** | [go-core/composer_parallel.go](file:///D:/work/solo%20work/架构迁移/low-entropy-core/go-core/composer_parallel.go) |
| **原语归属** | Composer |
| **层级** | L1 |

**Commit message**：`refactor(go-core): wrap parallel pipeline goroutines in errgroup`

---

### TASK-P1-5: `composer_fanout.go` — fanout executor 添加 errgroup

| 元字段 | 值 |
|---------|-----|
| **目标** | 将 FanoutPipeline 中的 1 个 `go func()` 改为 errgroup 模式 |
| **涉及文件** | [go-core/composer_fanout.go](file:///D:/work/solo%20work/架构迁移/low-entropy-core/go-core/composer_fanout.go) |
| **原语归属** | Composer |
| **层级** | L1 |

**Commit message**：`refactor(go-core): wrap fanout pipeline goroutines in errgroup`

---

### TASK-P1-6: `eventbus.go` — eventbus worker goroutine 生命周期

| 元字段 | 值 |
|---------|-----|
| **目标** | EventBus 的后台投递 worker 添加 context 取消机制 |
| **涉及文件** | [go-core/eventbus.go](file:///D:/work/solo%20work/架构迁移/low-entropy-core/go-core/eventbus.go) |
| **原语归属** | Adapter（事件投递属于副作用） |
| **层级** | L6 |
| **前置条件** | 需先阅读 `eventbus_persistent.go` 确认整体 EventBus 接口设计 |

**Commit message**：`refactor(go-core): add context lifecycle to eventbus dispatch worker`

---

### TASK-P1-7: `eventstore_worker.go` — eventstore dispatcher 生命周期

| 元字段 | 值 |
|---------|-----|
| **目标** | EventStore 的后台 dispatcher goroutine 添加 context 取消 |
| **涉及文件** | [go-core/eventstore_worker.go](file:///D:/work/solo%20work/架构迁移/low-entropy-core/go-core/eventstore_worker.go) |
| **原语归属** | Adapter（事件存储属于副作用） |
| **层级** | L6 |

**Commit message**：`refactor(go-core): add context lifecycle to eventstore dispatcher`

---

### TASK-P1-8: `eventstore_snapshot.go` — snapshot worker 生命周期

| 元字段 | 值 |
|---------|-----|
| **目标** | EventStore snapshot 的后台 worker 添加 context 取消 |
| **涉及文件** | [go-core/eventstore_snapshot.go](file:///D:/work/solo%20work/架构迁移/low-entropy-core/go-core/eventstore_snapshot.go) |
| **原语归属** | Adapter |
| **层级** | L6 |

**Commit message**：`refactor(go-core): add context lifecycle to eventstore snapshot worker`

---

### TASK-P1-9: `observation_pipeline.go` — observation pipeline worker

| 元字段 | 值 |
|---------|-----|
| **目标** | Observation Pipeline 的后台聚合 worker 添加 context 取消 |
| **涉及文件** | [go-core/observation_pipeline.go](file:///D:/work/solo%20work/架构迁移/low-entropy-core/go-core/observation_pipeline.go) |
| **原语归属** | Adapter（可观测性 I/O） |
| **层级** | L5 |

**Commit message**：`refactor(go-core): add context lifecycle to observation pipeline worker`

---

### TASK-P1-10: `handoff_persistence.go` — handoff persistence goroutine

| 元字段 | 值 |
|---------|-----|
| **目标** | Handoff persistence 的后台存储 goroutine 添加 context 取消 |
| **涉及文件** | [go-core/handoff_persistence.go](file:///D:/work/solo%20work/架构迁移/low-entropy-core/go-core/handoff_persistence.go) |
| **原语归属** | Adapter |
| **层级** | L5（handoff 属于可观测性配套） |

**Commit message**：`refactor(go-core): add context lifecycle to handoff persistence worker`

---

### TASK-P1-11: `patterns_distributed_redis.go` — distributed pattern worker

| 元字段 | 值 |
|---------|-----|
| **目标** | 分布式弹性模式的后台 worker 添加 context 取消 |
| **涉及文件** | [go-core/patterns_distributed_redis.go](file:///D:/work/solo%20work/架构迁移/low-entropy-core/go-core/patterns_distributed_redis.go) |
| **原语归属** | Composer 子类（弹性编排） |
| **层级** | L3 |

**Commit message**：`refactor(go-core): add context lifecycle to distributed redis pattern worker`

---

### TASK-P1-12: `patterns_rate_limiter_dist.go` — rate limiter worker

| 元字段 | 值 |
|---------|-----|
| **目标** | 分布式限流器的后台 token bucket 刷新 goroutine 添加 context 取消 |
| **涉及文件** | [go-core/patterns_rate_limiter_dist.go](file:///D:/work/solo%20work/架构迁移/low-entropy-core/go-core/patterns_rate_limiter_dist.go) |
| **原语归属** | Composer 子类 |
| **层级** | L3 |

**Commit message**：`refactor(go-core): add context lifecycle to distributed rate-limiter worker`

---

### TASK-P1-13: `agent_runner.go` — agent runner goroutines（4 个）

| 元字段 | 值 |
|---------|-----|
| **目标** | Agent runner 中的 4 个 `go func()` 添加 context 取消与错误传播 |
| **涉及文件** | [go-core/agent_runner.go](file:///D:/work/solo%20work/架构迁移/low-entropy-core/go-core/agent_runner.go) |
| **原语归属** | Composer（Agent 执行是一种编排） |
| **层级** | L7 应用层 |

**变更要点**：
- 每个 `go func()` → `g.Go(func() error { ... })`
- `AgentRunner` 结构增加 `errgroup *errgroup.Group` 字段
- `Stop()` 方法应等待 `g.Wait()` 确保所有 goroutine 退出

**Commit message**：`refactor(go-core): wrap agent runner goroutines in errgroup with context cancellation`

---

### TASK-P1-14: `app.go` — app background worker

| 元字段 | 值 |
|---------|-----|
| **目标** | App 的后台启动 goroutine 添加 context 取消 |
| **涉及文件** | [go-core/app.go](file:///D:/work/solo%20work/架构迁移/low-entropy-core/go-core/app.go) |
| **原语归属** | Composer（应用启动编排） |
| **层级** | L7 |

**Commit message**：`refactor(go-core): add context lifecycle to app background worker`

---

### TASK-P1-15: `atom.go` — atom helper goroutine

| 元字段 | 值 |
|---------|-----|
| **目标** | atom.go 中存在的 helper goroutine（可能用于并发原子操作组合）添加生命周期管理 |
| **涉及文件** | [go-core/atom.go](file:///D:/work/solo%20work/架构迁移/low-entropy-core/go-core/atom.go) |
| **原语归属** | Atom（纯函数层 —— 如果此 goroutine 是纯计算组合则保留，否则需审查） |
| **层级** | L1 |
| **注意**：此文件中的 goroutine 需特别审查 —— L1 层理论上不应有自己的后台 goroutine；若存在则可能是违规设计，应被移到 L2/L3 层 |

**前置审查问题**（执行此任务前必须回答）：
- [ ] `atom.go` 中的 goroutine 是纯计算还是包含 I/O？
- [ ] 能否将其改为无 goroutine 的纯函数组合？
- [ ] 如果是纯计算并发，使用 errgroup 是否足够？

**Commit message**：`refactor(go-core): add lifecycle mgmt to atom.go goroutine OR move to composer layer`

---

### TASK-P1-16: `adapter.go` — adapter helper goroutine

| 元字段 | 值 |
|---------|-----|
| **目标** | adapter.go 中的 goroutine 添加 context 取消 |
| **涉及文件** | [go-core/adapter.go](file:///D:/work/solo%20work/架构迁移/low-entropy-core/go-core/adapter.go) |
| **原语归属** | Adapter |
| **层级** | L1 |

**Commit message**：`refactor(go-core): add context lifecycle to adapter helper goroutine`

---

### TASK-P1-17: `errors.go` — error helper goroutine

| 元字段 | 值 |
|---------|-----|
| **目标** | errors.go 中（如有）的错误处理 goroutine 添加生命周期管理 |
| **涉及文件** | [go-core/errors.go](file:///D:/work/solo%20work/架构迁移/low-entropy-core/go-core/errors.go) |
| **原语归属** | L0 基础工具（错误处理不应有 goroutine；存在时需审查设计） |
| **层级** | L0 |
| **注意**：如确认此处的 goroutine 是错误异步收集/聚合，则应移到 L5 Observation 层；L0 层应保持纯函数 |

**Commit message**：`refactor(go-core): review and fix goroutine in errors.go (L0 layer)`

---

### TASK-P1-18: `perf_uuid.go` — uuid prefetch worker

| 元字段 | 值 |
|---------|-----|
| **目标** | UUID 预生成 worker 的 goroutine 添加 context 取消 |
| **涉及文件** | [go-core/perf_uuid.go](file:///D:/work/solo%20work/架构迁移/low-entropy-core/go-core/perf_uuid.go) |
| **原语归属** | Base Primitive（L0） |
| **层级** | L0 |

**注意**：此任务与 **TASK-P0-2** 相关联（同一文件），可以合并提交以减少 commit 数。

**Commit message**：`refactor(go-core): add context lifecycle to UUID prefetch worker`

---

### 3.2 P1 阶段 — 文件拆分（GAP-6）

### TASK-P1-19: `binance-exchange/core/engine.go` — 拆分为 Atom + State + Adapter 三层

| 元字段 | 值 |
|---------|-----|
| **目标** | 将 404 行的 `engine.go` 按职责拆分为 3 个文件，每个 ≤ 300 行 |
| **涉及文件** | [binance-exchange/core/engine.go](file:///D:/work/solo%20work/架构迁移/low-entropy-core/binance-exchange/core/engine.go) |
| **原语归属** | 跨 Atom（订单匹配算法）+ State（订单簿）+ Adapter（交易所交互） |
| **层级** | L7 examples |
| **预计变更**：新增 2 个文件 + 保留原文件（精简到 ≤ 200 行） |

**拆分方案**：

1. **`engine_atom.go`** — 纯计算原子
   - 订单匹配算法（`matchOrders(book OrderBook, order Order) []Trade`）
   - 价格计算
   - 撮合引擎的纯函数部分
   - 零 I/O，零外部依赖

2. **`engine_state.go`** — 状态管理
   - `type OrderBook struct`
   - `type Order struct`
   - `type Trade struct`
   - 添加/取消订单的状态变更操作（纯内存操作，无 I/O）
   - 可用 `sync.Mutex` 保护并发修改

3. **`engine_adapter.go`** — 交易所交互
   - WebSocket 连接
   - REST API 调用
   - 行情订阅 / 订单推送
   - 所有 I/O 集中在此
   - 使用 `context.Context` 管理生命周期

4. **`engine.go`**（精简后） — 仅保留核心编排
   - `type Engine struct`（组合 state + adapter）
   - `func (e *Engine) Start(ctx context.Context) error`
   - `func (e *Engine) Stop() error`
   - 对外暴露的统一接口

**拆分原则**：纯代码移动，不改变行为（agent-prompt.md C2 约束）

**验证断言**：
- [ ] 4 个文件（engine.go + engine_atom.go + engine_state.go + engine_adapter.go）每个均 ≤ 300 行
- [ ] `engine_atom.go` 中不包含 `net/http`、`os.`、`time.Now`、`fmt.Print` 等副作用调用
- [ ] `engine_adapter.go` 集中了所有 I/O 调用（grep 验证）
- [ ] `go build ./binance-exchange/...` 编译通过
- [ ] 现有测试 `go test ./binance-exchange/...` 继续通过

**Commit message**：`refactor(binance-exchange): split engine.go (404L) into atom/state/adapter tri-layer`

---

### TASK-P1-20: `examples/agent-demo/main.go` — 拆分

| 元字段 | 值 |
|---------|-----|
| **目标** | 将 465 行的 agent-demo main.go 按职责拆分，使每个文件 ≤ 300 行 |
| **涉及文件** | [examples/agent-demo/main.go](file:///D:/work/solo%20work/架构迁移/low-entropy-core/examples/agent-demo/main.go) |
| **原语归属** | 跨原语（example 项目包含完整使用模式） |
| **层级** | L7 examples |

**拆分方案**（待代码审查后确定）：

1. **`main.go`**（精简到 ≤ 80 行） — 仅 `main()` 函数，负责初始化 + 启动 + 等待信号
2. **`agent_demo_types.go`** — 数据类型定义
3. **`agent_demo_handlers.go`** — 业务处理函数（按原语分类：Atom 纯计算 / Port 验证 / Adapter I/O 分别标记注释）
4. **`agent_demo_config.go`** — 配置加载

**验证断言**：
- [ ] 每个文件 ≤ 300 行
- [ ] `go build ./examples/agent-demo/` 编译通过
- [ ] 可执行 `go run ./examples/agent-demo/` 启动成功

**Commit message**：`refactor(agent-demo): split 465L main.go into focused files`

---

### P1 阶段整体验收

- [ ] `grep -rn "go func\|go [A-Za-z]" --include="*.go" go-core/ | grep -v "_test.go" | wc -l` 输出 = 0（或仅剩已管理的 goroutine，且每个都有对应的 context 取消路径）
- [ ] 所有 goroutine 的函数签名要么返回 `error`，要么在函数体内有 `case <-ctx.Done()` 分支
- [ ] 2 个超 300 行文件已被拆分，`find go-core binance-exchange examples -name "*.go" -exec wc -l {} \; | sort -n | tail -5` 最大值 ≤ 300
- [ ] `go test ./go-core/... -count=1` 全部通过
- [ ] `go vet ./go-core/...` 无错误
- [ ] 提交数：约 18-20 个独立 commit（按"一个文件 / 一个关注点 / 一个 commit"原则）

---

## 4. Phase 2 — P2 中优先级（1 个月内，8 个任务）

**目标**：处理类型系统级问题（GAP-1, GAP-2, GAP-5）。这些任务需要改变公开 API，风险较大，应在 P0/P1 完成后、确保测试基线上限后执行。

### 4.1 GAP-1: Atom 类型扩展为 `func(In) (Out, error)`

### TASK-P2-1: 定义 `AtomWithError` 类型（不破坏现有 Atom）

| 元字段 | 值 |
|---------|-----|
| **目标** | 不直接修改 `Atom[In, Out]`（避免破坏所有调用点），而是新增 `AtomWithError[In, Out]` 作为可选增强，并提供 `AtomAsStep` 的双版本 |
| **涉及文件** | [go-core/types.go](file:///D:/work/solo%20work/架构迁移/low-entropy-core/go-core/types.go) + [go-core/step.go](file:///D:/work/solo%20work/架构迁移/low-entropy-core/go-core/step.go) |
| **原语归属** | Atom 定义（L1 层类型系统） |
| **层级** | L1 |
| **风险**：中等（影响所有使用 `Atom[In, Out]` 的调用点 —— 如果不改动原签名则风险为低） |
| **前置条件**：P0 全部完成，测试基线确认 |

**方案 A（增量式，推荐）**：

在 `types.go` 末尾新增：
```go
// AtomWithError 是 Atom 的增强版本，允许纯函数表达"计算失败"。
// 失败不是"副作用"——它是确定性的计算结果（如除零、输入超出范围等）。
// 真正的 I/O 失败仍应通过 Adapter 层处理。
type AtomWithError[In, Out any] func(In) (Out, error)
```

在 `step.go` 新增：
```go
// AtomWithErrorAsStep 将 AtomWithError 包装为 Step，错误通过 StepError 传递。
func AtomWithErrorAsStep[In, Out any](a AtomWithError[In, Out]) Step[In, Out] {
    return StepFunc[In, Out]{
        execute: func(ctx context.Context, input In) (Out, error) {
            out, err := a(input)
            if err != nil {
                var zero Out
                return zero, &StepError{
                    Code:     "ATOM_COMPUTE_FAILED",
                    Message:  err.Error(),
                    Category: CategoryRecoverable,
                }
            }
            return out, nil
        },
        unitType: "Atom",
    }
}
```

**方案 B（激进式，不推荐）**：直接修改 `Atom[In, Out]` 签名为 `func(In) (Out, error)`，影响所有调用点。

**验证断言（方案 A）**：
- [ ] `types.go` 中存在 `AtomWithError` 类型定义
- [ ] `step.go` 中存在 `AtomWithErrorAsStep` 包装器
- [ ] `Atom` 原签名不变，所有现有调用点继续有效
- [ ] `AtomWithErrorAsStep` 对错误输入返回非 nil `*StepError`
- [ ] 编译通过，测试通过

**Commit message**：`feat(go-core): add AtomWithError[In, Out] for pure functions that can fail`

---

### TASK-P2-2: 在现有代码中逐步用 AtomWithError 替换"裸 func 返回 error"

| 元字段 | 值 |
|---------|-----|
| **目标** | 在所有被识别为"本质上是纯计算但当前返回 error"的函数中，使用 `AtomWithError` 明确标记其原语归属 |
| **涉及文件** | `go-core/config_parse.go`、`go-core/agent_types.go`、`go-core/entropy_metrics.go` |
| **前置条件**：TASK-P2-1 已完成 |
| **预计变更**：每个文件 5-10 行（类型声明替换） |

**验证断言**：
- [ ] 至少 3 个计算型函数被显式声明为 `AtomWithError`
- [ ] 调用点通过 `AtomWithErrorAsStep` 包装为 Step
- [ ] 编译通过

**Commit message**：`refactor(go-core): mark pure compute functions as AtomWithError for explicit primitive classification`

---

### 4.2 GAP-2: Composer 层的 `time.Now()` 依赖治理

### TASK-P2-3: `composer.go` — time.Now() 改为通过 Observation 层注入

| 元字段 | 值 |
|---------|-----|
| **目标** | Pipeline 执行的耗时测量不直接调用 `time.Now()`，而是通过可观测性层统一管理 |
| **涉及文件** | [go-core/composer.go](file:///D:/work/solo%20work/架构迁移/low-entropy-core/go-core/composer.go) — 检查 time.Now 调用点 |
| **原语归属** | Composer（纯编排） |
| **层级** | L1 |
| **前置条件**：理解当前 Pipeline 的 `obs ObservationAdapter` 和 `obsProv *ObservabilityProvider` 的设计 |

**方案**：
1. 在 `ObservationAdapter` 接口中增加 `Now() time.Time` 方法（或等效的"时序来源"抽象）
2. 默认实现 `NoOpObservationAdapter.Now()` 返回 `time.Now()`（保持行为不变，但架构上解耦）
3. `Pipeline.Run()` 中将 `start := time.Now()` 改为 `start := p.obs.Now()`
4. 同理处理 `composer_fanout.go` 和 `composer_parallel.go`

**验证断言**：
- [ ] `composer.go` 中不再有直接 `time.Now()` 调用（grep 验证）
- [ ] `obs.Now()` 或等效调用替代了直接时间读取
- [ ] Pipeline 耗时测量行为不变（可通过 benchmark/测试验证）
- [ ] 编译通过，测试通过

**Commit message**：`refactor(go-core): inject time source through ObservationAdapter instead of direct time.Now in Composer`

---

### TASK-P2-4: `composer_fanout.go` + `composer_parallel.go` + `handoff_composer.go` — 同上模式

| 元字段 | 值 |
|---------|-----|
| **目标** | 将 fanout/parallel/handoff composer 中的 time.Now() 统一改为通过 Observation 层注入 |
| **涉及文件** | [go-core/composer_fanout.go](file:///D:/work/solo%20work/架构迁移/low-entropy-core/go-core/composer_fanout.go)、[go-core/composer_parallel.go](file:///D:/work/solo%20work/架构迁移/low-entropy-core/go-core/composer_parallel.go)、[go-core/handoff_composer.go](file:///D:/work/solo%20work/架构迁移/low-entropy-core/go-core/handoff_composer.go) |
| **层级** | L1 / L5 |
| **前置条件**：TASK-P2-3 完成（ObservationAdapter 已扩展 Now 方法） |

**验证断言**：
- [ ] 3 个文件中均无直接 `time.Now()` 调用
- [ ] 全部改为通过 `obs.Now()` 或等效注入
- [ ] 编译通过，测试通过

**Commit message**：`refactor(go-core): remove direct time.Now from fanout/parallel/handoff composers`

---

### 4.3 GAP-5: interface{} 泛型化

### TASK-P2-5: `config_parse.go` — 用泛型替代 map[string]interface{}

| 元字段 | 值 |
|---------|-----|
| **目标** | 配置解析中使用的 `map[string]interface{}` / `[]interface{}` 改为强类型结构体或泛型集合 |
| **涉及文件** | [go-core/config_parse.go](file:///D:/work/solo%20work/架构迁移/low-entropy-core/go-core/config_parse.go) |
| **层级** | L3 |
| **预计变更**：5-15 行（取决于当前 interface{} 的具体用法） |

**验证断言**：
- [ ] `grep -n "interface{}" go-core/config_parse.go` 输出为空
- [ ] 配置项使用具体类型（如 `map[string]ConfigValue` 或强类型结构体）
- [ ] 编译通过

**Commit message**：`refactor(go-core): replace interface{} with strongly-typed config structures`

---

### TASK-P2-6: `handoff_types.go` — 泛型化 handoff 数据类型

| 元字段 | 值 |
|---------|-----|
| **目标** | handoff 层中使用的 `interface{}` 改为泛型参数 |
| **涉及文件** | [go-core/handoff_types.go](file:///D:/work/solo%20work/架构迁移/low-entropy-core/go-core/handoff_types.go) |
| **层级** | L5 |

**Commit message**：`refactor(go-core): replace interface{} with generics in handoff type system`

---

### TASK-P2-7: `idempotent.go` — 幂等性数据结构泛型化

| 元字段 | 值 |
|---------|-----|
| **目标** | 幂等性存储中使用的 `interface{}` 改为泛型 |
| **涉及文件** | [go-core/idempotent.go](file:///D:/work/solo%20work/架构迁移/low-entropy-core/go-core/idempotent.go) |
| **层级** | L3 |

**Commit message**：`refactor(go-core): replace interface{} with generics in idempotent store`

---

### TASK-P2-8: `agent_runner.go` / `schema_registry.go` / `schema_migration.go` — 泛型化剩余 interface{}

| 元字段 | 值 |
|---------|-----|
| **目标** | 剩余 3 个文件中的 `interface{}` 改为强类型 |
| **涉及文件** | [go-core/agent_runner.go](file:///D:/work/solo%20work/架构迁移/low-entropy-core/go-core/agent_runner.go)、[go-core/schema_registry.go](file:///D:/work/solo%20work/架构迁移/low-entropy-core/go-core/schema_registry.go)、[go-core/schema_migration.go](file:///D:/work/solo%20work/架构迁移/low-entropy-core/go-core/schema_migration.go) |
| **层级** | L7 / L4 |

**Commit message**：`refactor(go-core): replace remaining interface{} usages in agent/schema layers with generics`

---

### P2 阶段整体验收

- [ ] `grep -rn "interface{}" --include="*.go" go-core/ | grep -v "_test.go" | wc -l` = 0（或仅保留不可避免的 JSON 反序列化临时变量）
- [ ] `go-core/` 中非测试代码中 `time.Now()` 仅出现在 Adapter 层（`storage_*`、`eventstore_*`、`observation_*`、`security_*`、`handoff_*`、`patterns_*` 的实际 I/O 路径）
- [ ] `AtomWithError` 类型已存在且至少有 3 个使用点
- [ ] `go test ./go-core/... -count=1` 全部通过
- [ ] `go vet ./go-core/...` 无错误

---

## 5. Phase 3 — P3 架构演进（3 个月，3 个任务）

**目标**：解决架构设计层面的耦合与归属不清问题。非阻塞性，不影响功能，但提升架构的长期可维护性。

### TASK-P3-1: GAP-7 — Composer 中的 Observability Provider 改为装饰器模式

| 元字段 | 值 |
|---------|-----|
| **目标** | 当前 `Pipeline[T]` 直接持有 `*ObservabilityProvider`，将 Composer 与可观测性基础设施耦合。改为使用 Step 装饰器模式，在 Pipeline 构建时以"装饰"方式注入可观测性 |
| **涉及文件** | [go-core/composer.go](file:///D:/work/solo%20work/架构迁移/low-entropy-core/go-core/composer.go)（Pipeline 结构体定义） + 新增 `go-core/observation_decorator.go` |
| **原语归属** | Composer 层架构调整 |
| **层级** | L1 → L5 解耦 |
| **前置条件**：P2 全部完成 |

**设计思路**：

1. 新增装饰器类型 `ObservedStep[T, U]`：
   ```go
   type ObservedStep[T, U any] struct {
       inner Step[T, U]
       prov  *ObservabilityProvider
       name  string
   }
   func (s ObservedStep[T, U]) Execute(ctx context.Context, input T) (U, error) {
       // 启动 span，记录 metrics，调用 inner.Execute
       out, err := s.inner.Execute(ctx, input)
       // 结束 span，记录耗时
       return out, err
   }
   func (s ObservedStep[T, U]) UnitType() string { return s.inner.UnitType() }
   ```

2. 修改 `Pipeline[T]`：移除 `obsProv *ObservabilityProvider` 字段，改为构建时使用 `ObservedStep` 装饰每个 step

3. 提供构建器函数 `NewObservedPipeline[T any](prov *ObservabilityProvider, steps ...Step[T, T]) *Pipeline[T]`

4. 原 `NewPipeline` 保持不变（无观测开销）

**验证断言**：
- [ ] `Pipeline[T]` 结构体不再直接持有 `*ObservabilityProvider`
- [ ] 可观测性通过 `ObservedStep` 装饰器注入（代码审查确认）
- [ ] `NewPipeline` 和 `NewObservedPipeline` 两个构建路径均可用
- [ ] 性能基准测试显示无观测路径的 overhead ≤ 1%（相对前版本）
- [ ] 测试全部通过

**Commit message**：`refactor(go-core): decouple ObservabilityProvider from Pipeline core using Step decorator pattern`

---

### TASK-P3-2: GAP-8 — 明确 Trace/Span ID 生成器的原语归属与注入策略

| 元字段 | 值 |
|---------|-----|
| **目标** | 将 `getGlobalUUIDGen()` 的单例模式改为 context 注入式，明确 TraceID 生成属于"可观测性基础原语"而非业务代码 |
| **涉及文件** | [go-core/types.go](file:///D:/work/solo%20work/架构迁移/low-entropy-core/go-core/types.go) + [go-core/perf_traceid.go](file:///D:/work/solo%20work/架构迁移/low-entropy-core/go-core/perf_traceid.go) + [go-core/perf_uuid.go](file:///D:/work/solo%20work/架构迁移/low-entropy-core/go-core/perf_uuid.go) |
| **原语归属**：Base Primitive / TraceSource |
| **层级**：L0 → L5 |

**设计思路**：

1. 在 `context.Context` 中存储 `TraceID` / `SpanID`（当前已部分实现——确认 `composer.go` 中的 trace/spand ID 如何传递）
2. 新增 `TraceIDFromCtx(ctx) TraceID` 函数，从 context 中读取 TraceID
3. 保留便捷函数 `NewTraceID()` 但限制其使用——只在 ctx 无 TraceID 时作为 fallback 使用
4. 明确文档化：TraceID 属于"可观测性基础设施"，不参与业务四原语分类，但在 L1 层定义其类型以便全局可用

**验证断言**：
- [ ] 代码中 `NewTraceID()` / `NewCompactTraceID()` 的调用被审查，确认仅在 ctx 缺失 TraceID 时使用
- [ ] 存在 `TraceIDFromCtx(ctx context.Context) TraceID` 函数
- [ ] 主要执行路径（Pipeline.Run、Adapter.Execute、Port.Validate）优先从 ctx 获取 TraceID
- [ ] 单例 `getGlobalUUIDGen` 仍可存在，但其调用被限制在"TraceID 生成"这一条路径上

**Commit message**：`refactor(go-core): clarify Trace/Span ID as observability primitive; prefer context injection over global generation`

---

### TASK-P3-3: GAP-9 — 明确弹性模式 `patterns_*.go` 的原语归属

| 元字段 | 值 |
|---------|-----|
| **目标** | 在 CLAUDE.md / agent-prompt.md 中明确定义"弹性模式"的架构位置，并在每个 `patterns_*.go` 文件头的注释中标注其原语归属 |
| **涉及文件** | 8 个 patterns 文件 + [CLAUDE.md](file:///D:/work/solo%20work/架构迁移/low-entropy-core/CLAUDE.md) |
| **原语归属**：Composer 子类（弹性编排层） |
| **层级**：L2（单机韧性）→ L3（分布式韧性） |

**分类建议**：

| 文件 | 层级 | 原语归属 | 说明 |
|------|------|---------|------|
| `patterns_circuit.go` | L2 | Composer（断路器模式） | 编排：在失败率达到阈值时开路 |
| `patterns_circuit_enhanced.go` | L2 | Composer | 增强版断路器 |
| `patterns_circuit_global.go` | L3 | Composer | 全局共享状态断路器 |
| `patterns_retry.go` | L2 | Composer | 重试编排 |
| `patterns_backoff.go` | L2 | Atom（纯计算退避策略） | 退避算法是纯函数，无 I/O |
| `patterns_rate_limiter.go` | L2 | Composer | 令牌桶限流编排 |
| `patterns_rate_limiter_sharded.go` | L3 | Composer | 分片式限流 |
| `patterns_rate_limiter_dist.go` | L3 | Adapter + Composer | 分布式限流（涉及 Redis） |
| `patterns_fallback_bulkhead.go` | L2 | Composer | 舱壁/回退编排 |
| `patterns_resilience_chain.go` | L2 | Composer | 弹性组合链 |
| `patterns_resilience_test.go` | L2 | 测试 | 测试文件 |
| `patterns_degradation_federated.go` | L3 | Composer | 分级降级编排 |
| `patterns_health_dist.go` | L3 | Composer | 健康检查分布式编排 |
| `patterns_fanout_test.go` | L2 | 测试 | 测试文件 |
| `patterns_token_bucket.go` | L2 | Atom（纯计算） | token bucket 纯算法实现 |

**验证断言**：
- [ ] 每个 `patterns_*.go` 文件顶部有注释声明其"层级"和"原语归属"
- [ ] CLAUDE.md 的 L2/L3 架构描述中明确了弹性模式的定位
- [ ] `patterns_backoff.go` / `patterns_token_bucket.go` 中不含 I/O 调用（grep 验证）
- [ ] 涉及 Redis/网络的 `patterns_distributed_redis.go` / `patterns_rate_limiter_dist.go` 被明确标记为 L3 Adapter + Composer 混合

**Commit message**：`docs(go-core): clarify primitive classification and layer归属 for patterns_*.go resilience modules`

---

### P3 阶段整体验收

- [ ] `composer.go` 中的 `Pipeline[T]` 不直接持有可观测性基础设施
- [ ] TraceID 主要通过 context 传递，全局生成仅作为 fallback
- [ ] 所有 `patterns_*.go` 文件头明确注释了层级与原语归属
- [ ] CLAUDE.md 更新了 L2/L3 的弹性模式分类描述
- [ ] `go test ./go-core/... -count=1` 全部通过

---

## 6. 提交前架构验证清单（每次提交前强制执行）

### 6.1 静态检查

- [ ] **行数检查**：`go-core/` 中所有非测试 `.go` 文件 ≤ 300 行
  ```bash
  find go-core -name "*.go" -not -name "*_test.go" -exec wc -l {} + | sort -n | tail -3
  ```
- [ ] **panic 零容忍**：
  ```bash
  grep -rn "panic(" --include="*.go" go-core/ | grep -v "_test.go"
  ```
  期望输出 = 空
- [ ] **interface{} 零容忍（新代码）**：
  ```bash
  grep -rn "interface{}" --include="*.go" go-core/ | grep -v "_test.go"
  ```
  仅允许在 JSON 反序列化等标准库接口处使用
- [ ] **fmt.Print 零容忍（业务路径）**：
  ```bash
  grep -rn "fmt.Print" --include="*.go" go-core/ | grep -v "_test.go" | grep -v "^.*\/\/.*fmt.Print"
  ```
- [ ] **循环依赖零容忍**：
  ```bash
  # 若有 go list，使用:
  go list -deps ./go-core/... | grep -E "core/core"
  ```

### 6.2 编译与测试

- [ ] `go build ./go-core/...` exit code = 0
- [ ] `go build ./cmd/arch-manager/` exit code = 0
- [ ] `go vet ./go-core/...` exit code = 0
- [ ] `go test ./go-core/... -count=1 -timeout=120s` 全部通过
- [ ] `go test ./go-core/migrate/... -count=1` 全部通过（架构迁移测试集）

### 6.3 仪表盘验证（可选，适用于较大变更）

- [ ] 启动 `arch-manager`：`go build -tags lecore_tier4 -o arch-manager.exe ./cmd/arch-manager/ ; .\arch-manager.exe`
- [ ] 访问 `http://localhost:8090/api/violations` 返回违规数 = 0
- [ ] 访问 `http://localhost:8090/api/primitives` 返回四原语的正确分类
- [ ] 访问 `http://localhost:8090/api/` 返回健康评分 ≥ 80
- [ ] 关闭 `arch-manager`（Ctrl+C 或任务管理器）

### 6.4 Commit message 格式

必须遵循 Conventional Commits（agent-prompt.md §5.4）：

```
<type>(<scope>): <description>

[optional body with references to GAP-ID / TASK-ID]
```

合法 type：
- `fix` — 修复 bug / 消除违规
- `refactor` — 代码重构 / 文件拆分 / 不改变行为
- `test` — 新增或修改测试
- `feat` — 新功能（架构迁移中极少使用）
- `docs` — 文档更新
- `chore` — 构建/工具链

合法 scope：`go-core`、`arch-manager`、`migrate`、`binance-exchange`、`examples`

示例：
```
fix(go-core): remove panic in BatchedUUIDGen initialization

Refs TASK-P0-2, GAP-3
Gracefully degrade when crypto/rand.Read fails; no process crash.
```

---

## 7. 附录 A — 检查命令速查表

| 目标 | 命令（PowerShell） |
|------|-------------------|
| 列出所有 panic 调用 | `grep -rn "panic(" --include="*.go" go-core/ | grep -v "_test.go"` |
| 列出所有裸 goroutine | `grep -rn "go func\|go [A-Za-z]" --include="*.go" go-core/ | grep -v "_test.go"` |
| 列出所有 interface{} | `grep -rn "interface{}" --include="*.go" go-core/ | grep -v "_test.go"` |
| 列出所有 fmt.Print | `grep -rn "fmt.Print" --include="*.go" go-core/ | grep -v "_test.go"` |
| 列出所有 time.Now | `grep -rn "time.Now\|time.Since\|time.After" --include="*.go" go-core/ | grep -v "_test.go"` |
| 按行数排序的 Go 文件 | `Get-ChildItem -Path go-core -Filter "*.go" | Where-Object { $_.Name -notmatch "_test\.go$" } | ForEach-Object { "$((Get-Content $_ | Measure-Object -Line).Lines) $_" } | Sort-Object { [int]($_ -split ' ')[0] } -Descending | Select-Object -First 10` |
| 构建 go-core | `cd "D:\work\solo work\架构迁移\low-entropy-core" ; go build ./go-core/...` |
| 构建 arch-manager | `go build -tags lecore_tier4 -o arch-manager.exe ./cmd/arch-manager/` |
| 运行所有测试 | `go test ./go-core/... -count=1 -timeout=120s` |
| 运行架构迁移测试 | `go test ./go-core/migrate/... -count=1` |

---

## 8. 附录 B — 任务进度追踪模板

### 8.1 总览表（Master Tracking Sheet）

| 阶段 | GAP-ID | TASK-ID | 文件 | 优先级 | 预计工时 | 状态 | 提交哈希 | 完成日期 | 验证人 |
|------|--------|---------|------|--------|---------|------|---------|---------|-------|
| P0 | GAP-3 | TASK-P0-1 | config_validation_watcher.go | 立即 | 1h | ☐ | | | |
| P0 | GAP-3 | TASK-P0-2 | perf_uuid.go | 立即 | 1h | ☐ | | | |
| P1 | GAP-4 | TASK-P1-1 | config_validation_watcher.go | 高 | 1h | ☐ | | | |
| P1 | GAP-4 | TASK-P1-2 | config_hotreload.go | 高 | 1h | ☐ | | | |
| P1 | GAP-4 | TASK-P1-3 | composer_stream.go | 高 | 2h | ☐ | | | |
| P1 | GAP-4 | TASK-P1-4 | composer_parallel.go | 高 | 1h | ☐ | | | |
| P1 | GAP-4 | TASK-P1-5 | composer_fanout.go | 高 | 1h | ☐ | | | |
| P1 | GAP-4 | TASK-P1-6 | eventbus.go | 高 | 1h | ☐ | | | |
| P1 | GAP-4 | TASK-P1-7 | eventstore_worker.go | 高 | 1h | ☐ | | | |
| P1 | GAP-4 | TASK-P1-8 | eventstore_snapshot.go | 高 | 1h | ☐ | | | |
| P1 | GAP-4 | TASK-P1-9 | observation_pipeline.go | 高 | 1h | ☐ | | | |
| P1 | GAP-4 | TASK-P1-10 | handoff_persistence.go | 高 | 1h | ☐ | | | |
| P1 | GAP-4 | TASK-P1-11 | patterns_distributed_redis.go | 高 | 1h | ☐ | | | |
| P1 | GAP-4 | TASK-P1-12 | patterns_rate_limiter_dist.go | 高 | 1h | ☐ | | | |
| P1 | GAP-4 | TASK-P1-13 | agent_runner.go | 高 | 2h | ☐ | | | |
| P1 | GAP-4 | TASK-P1-14 | app.go | 高 | 1h | ☐ | | | |
| P1 | GAP-4 | TASK-P1-15 | atom.go | 高 | 1h | ☐ | | | |
| P1 | GAP-4 | TASK-P1-16 | adapter.go | 高 | 1h | ☐ | | | |
| P1 | GAP-4 | TASK-P1-17 | errors.go | 高 | 1h | ☐ | | | |
| P1 | GAP-4 | TASK-P1-18 | perf_uuid.go | 高 | 1h | ☐ | | | |
| P1 | GAP-6 | TASK-P1-19 | binance-exchange/core/engine.go | 高 | 2h | ☐ | | | |
| P1 | GAP-6 | TASK-P1-20 | examples/agent-demo/main.go | 高 | 2h | ☐ | | | |
| P2 | GAP-1 | TASK-P2-1 | types.go + step.go | 中 | 1h | ☐ | | | |
| P2 | GAP-1 | TASK-P2-2 | config_parse.go + agent_types.go + entropy_metrics.go | 中 | 2h | ☐ | | | |
| P2 | GAP-2 | TASK-P2-3 | composer.go | 中 | 1h | ☐ | | | |
| P2 | GAP-2 | TASK-P2-4 | composer_fanout.go + composer_parallel.go + handoff_composer.go | 中 | 2h | ☐ | | | |
| P2 | GAP-5 | TASK-P2-5 | config_parse.go | 中 | 1h | ☐ | | | |
| P2 | GAP-5 | TASK-P2-6 | handoff_types.go | 中 | 1h | ☐ | | | |
| P2 | GAP-5 | TASK-P2-7 | idempotent.go | 中 | 1h | ☐ | | | |
| P2 | GAP-5 | TASK-P2-8 | agent_runner.go + schema_registry.go + schema_migration.go | 中 | 2h | ☐ | | | |
| P3 | GAP-7 | TASK-P3-1 | composer.go + observation_decorator.go | 低 | 3h | ☐ | | | |
| P3 | GAP-8 | TASK-P3-2 | types.go + perf_traceid.go + perf_uuid.go | 低 | 2h | ☐ | | | |
| P3 | GAP-9 | TASK-P3-3 | patterns_*.go (8 个) + CLAUDE.md | 低 | 2h | ☐ | | | |

**状态标记**：
- `☐` 未开始
- `◐` 进行中（需要注明正在哪个子步骤）
- `☑` 已完成（已通过第 6 节的所有静态/编译验证）

### 8.2 单任务完成清单（每个 TASK 完成后逐项打勾）

```
[TASK-ID] 完成清单
────────────────────────────
[ ] 代码变更已实现（遵循原子化原则：1 个关注点 = 1 个文件修改）
[ ] grep 命令验证（panic / interface{} / time.Now / fmt.Print / goroutine）
[ ] go build ./go-core/...
[ ] go build ./cmd/arch-manager/
[ ] go vet ./go-core/...
[ ] go test ./go-core/... -count=1 -timeout=120s
[ ] go test ./go-core/migrate/... -count=1
[ ] 文件行数检查（所有非测试文件 ≤ 300 行）
[ ] Commit message 符合 Conventional Commits 格式
[ ] 提交前运行 arch-manager（可选，大变更时），/api/violations 返回 0
[ ] 关联的 GAP-ID 已在 commit message 中标注
[ ] 任务状态由"进行中"更新为"已完成"
```

### 8.3 阶段里程碑验收 Checklist

**P0 完成**（panic 零容忍）
```
[ ] grep -rn "panic(" --include="*.go" go-core/ | grep -v "_test.go"  输出为空
[ ] 2 个任务均标记 ☑
[ ] go test ./go-core/... -count=1  全部通过
```

**P1 完成**（goroutine 生命周期 + 文件拆分）
```
[ ] 所有 goroutine 有 context 取消路径（通过 grep + 代码审查确认）
[ ] 2 个超 300 行文件已拆分且每个子文件 ≤ 300 行
[ ] 20 个任务均标记 ☑
[ ] go test ./go-core/... -count=1  全部通过
[ ] go vet ./go-core/...  无错误
```

**P2 完成**（类型系统演进）
```
[ ] AtomWithError 类型已定义且至少 3 个使用点
[ ] time.Now 在 Composer 层通过 Observation 注入（grep 确认）
[ ] interface{} 零新增（或仅剩不可避免的标准库使用点）
[ ] 8 个任务均标记 ☑
[ ] go test ./go-core/... -count=1  全部通过
```

**P3 完成**（架构演进）
```
[ ] Composer 不直接持有 ObservabilityProvider
[ ] TraceID 注入策略已明确（ctx 优先）
[ ] patterns_*.go 文件头均有层级/原语注释
[ ] CLAUDE.md 已更新弹性模式分类
[ ] 3 个任务均标记 ☑
[ ] go test ./go-core/... -count=1  全部通过
```

---

## 9. 附录 C — 风险与回滚策略

### 9.1 风险评估矩阵

| 风险项 | 触发条件 | 影响范围 | 概率 | 严重度 | 缓解措施 | 回滚方式 |
|--------|---------|---------|------|-------|---------|---------|
| 变更 `Atom[In, Out]` 签名导致大规模编译错误 | TASK-P2-1 中选择方案 B | 全库所有使用 Atom 的代码 | 低（推荐方案 A） | 高 | 优先方案 A（增量），若需方案 B 则先做影响分析 | `git revert` 单个 commit |
| goroutine refactor 引入数据竞争（race condition） | errgroup 替换后状态共享逻辑错误 | 并发执行路径 | 中 | 中 | 变更后运行 `go test -race` | `git revert` 对应 commit |
| 文件拆分导致循环依赖 | 拆分文件时 import 方向错误 | binance-exchange / agent-demo | 低 | 中 | 拆分前做依赖分析，使用 `go list -deps` | 恢复原文件 |
| Observation 注入导致性能回归 | `obs.Now()` 开销高于直接 `time.Now()` | Pipeline 执行性能 | 低 | 低 | 变更前后对比 benchmark | 回滚单个变更 commit |
| Commit 跨层级破坏 C10 规则 | 一次 commit 包含 P0 + P1 混合内容 | git 历史可读性 | 低 | 低 | 使用交互式 `git add -p` 精确选择变更 | `git reset --soft HEAD~1` 重新提交 |

### 9.2 推荐提交策略

```
策略：一个 commit = 一个 TASK（≈ 一个文件 × 一个关注点）
理由：
  1. 最小化每个 commit 的变更半径 → 最小化回滚影响
  2. 精确的 blame 历史 → 便于未来问题定位
  3. 符合 agent-prompt.md C10 规则

异常处理：
  - 若多个文件的变更相互依赖（如 types.go + step.go 新增类型+包装器），
    可在一个 commit 中完成，但需在 message 中注明。
  - TASK-P0-2 与 TASK-P1-18 均涉及 perf_uuid.go，可合并为 1 个 commit
    以减少噪音。
```

### 9.3 回滚命令速查

```bash
# 回滚单个 commit（推荐）
git revert <commit-hash>

# 查看最近 N 个 commit 的摘要
git log --oneline -20

# 查看特定 commit 的变更内容（影响分析）
git show --stat <commit-hash>

# 软重置（重新组织提交，不丢失代码）
git reset --soft HEAD~1

# 分支保护：所有架构变更应在 feature/arch-<gap-id> 分支完成后再合入 main
git checkout -b feature/arch-gap3-panic-fix
# ... 完成任务 ...
git checkout main
git merge --no-ff feature/arch-gap3-panic-fix
```

---

## 10. 附录 D — 架构标准基线速查卡

```
┌─────────────────────────────────────────────────────────────────┐
│               LOW-ENTROPY CORE — ARCHITECTURE CHEAT SHEET        │
├─────────────────────────────────────────────────────────────────┤
│  四原语 L1 层：                                                  │
│    Atom     : func(In) Out            [纯计算, 零依赖, 零I/O]   │
│    Port     : Validate(ctx, In) (Out, error)  [仅输入验证]       │
│    Adapter  : Execute(ctx, In) (Out, error)   [唯一允许I/O]      │
│    Composer : Run(ctx, T) (T, Steps, error)  [仅编排, 无业务]   │
├─────────────────────────────────────────────────────────────────┤
│  8 层架构依赖方向：                                              │
│    L0 → L1 → L2 → L3 → L4 → L5 → L6 → L7                        │
│    (禁止反向依赖 / 禁止跨层跳跃 / 禁止循环依赖)                  │
├─────────────────────────────────────────────────────────────────┤
│  10 条约束强制执行：                                             │
│    C1 多语言架构通用    C2 文件≤300行       C3 原子迁移可追溯     │
│    C4 CLI覆盖           C5 禁止panic        C6 禁止裸goroutine   │
│    C7 Atom零副作用      C8 禁止interface{}   C9 禁止fmt.Print     │
│    C10 提交符合Conventional Commits                             │
├─────────────────────────────────────────────────────────────────┤
│  验证命令速查（每次提交前运行）：                                │
│    ✗  panic:   grep -rn "panic(" --include="*.go" go-core/       │
│    ✗  goroutine: grep -rn "go func\|go [A-Za-z]" go-core/        │
│    ✗  interface{}: grep -rn "interface{}" go-core/               │
│    ✗  fmt.Print: grep -rn "fmt.Print" go-core/                   │
│    ✗  time.Now:  grep -rn "time.Now" go-core/ (Composer层检查)   │
│    ✓  go build ./go-core/...                                     │
│    ✓  go build ./cmd/arch-manager/                               │
│    ✓  go vet ./go-core/...                                       │
│    ✓  go test ./go-core/... -count=1 -timeout=120s               │
│    ✓  find go-core -name "*.go" ! -name "*_test.go" -exec wc -l  │
└─────────────────────────────────────────────────────────────────┘
```

---

## 11. 元信息

| 字段 | 值 |
|------|-----|
| **文档标题** | Low-Entropy Core — 最小任务单元开发文档 |
| **项目版本** | v0.12.x |
| **文档版本** | v1.0 |
| **创建日期** | 2025-07-01 |
| **最后更新** | 2025-07-01 |
| **对应规范** | agent-prompt.md §4 (四原语) + §5 (核心约束) |
| **关联文档** | CLAUDE.md（项目内约束）、ARCHITECTURE.md（架构说明） |
| **适用范围** | `go-core/`、`cmd/arch-manager/`、`binance-exchange/`、`examples/agent-demo/` |
| **总任务数** | 33（P0: 2, P1: 20, P2: 8, P3: 3） |
| **预计总工时** | ≈ 39 小时（P0: 2h, P1: 22h, P2: 11h, P3: 4h） |
| **预计完成周期** | ≈ 3 个月（P0: 1 天, P1: 1 周, P2: 1 月, P3: 1.5 月） |
| **验证工具** | `cmd/arch-manager/`（`/api/violations`、`/api/primitives`、`/api/`） |
| **负责人** | 架构迁移工作组 |

---

## 12. 变更日志

| 版本 | 日期 | 变更内容 | 变更人 |
|------|------|---------|-------|
| v1.0 | 2025-07-01 | 初始版本：基于架构差距分析（GAP-1 ~ GAP-10），拆分为 4 阶段 33 任务的原子化开发计划 | 架构迁移工作组 |

---

*文档结束（EOF） — 所有架构变更需对照本文件执行并通过第 6 节验证清单*
