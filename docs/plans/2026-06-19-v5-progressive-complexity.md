# v5.0 渐进复杂度模型 — 最小任务单元开发计划

## 目标

将 Low-Entropy Core 从 v4.0（单一尺寸 48 文件）重构为 v5.0（8 级渐进复杂度），通过 Go Build Tags 实现零成本特性门控。

## 核心原则

1. **最小内核**：L0 仅 6 个文件，零外部依赖，始终编译
2. **零成本特性门控**：未激活模块不进入二进制
3. **渐进式披露**：8 个层级，每层是上层的严格超集

## 层级定义

| 层级 | 名称 | 文件规模 | 开发者 | 框架代码量 | Tag |
|------|------|----------|--------|-----------|-----|
| L0 | 原型 | <10 | 1 | ~400 行 | (无 tag，始终编译) |
| L1 | 微服务 | 10-100 | 1-3 | ~800 行 | `lecore_tier1` |
| L2 | 中型服务 | 100-1K | 3-10 | ~2500 行 | `lecore_tier2` |
| L3 | 大型服务 | 1K-10K | 10-50 | ~6000 行 | `lecore_tier3` |
| L4 | 平台 | 10K-100K | 50-200 | ~12000 行 | `lecore_tier4` |
| L5 | 企业平台 | 100K-1M | 200-1K | ~20000 行 | `lecore_tier5` |
| L6 | 系统级 | 1M-10M | 1K-4K | ~35000 行 | `lecore_tier6` |
| L7 | Windows 级 | 10M-50M+ | 4K+ | ~50000 行 | `lecore_tier7` |

---

## 任务分解

### Task 1: 内核解耦 —— 消除内核文件的外部依赖

**问题**：当前内核文件存在两个跨模块依赖，需要修复后才能提取为纯内核。

| 子任务 | 文件 | 问题 | 修复 |
|--------|------|------|------|
| 1.1 | `adapter.go` | `SnapshotAdapter[T]` 引用 `handoff.go` 的 `HandoffSnapshot` | 将 `SnapshotAdapter` 移至 `handoff_persistence.go` |
| 1.2 | `observation.go` | 使用 `perf_core.go` 的 `CompactTraceID` | 将 `CompactTraceID` 移至 `observation.go` |
| 1.3 | 验证 | 全量测试 | 确保重构后所有 353 测试通过 |

**文件变更**：~3 个文件修改

---

### Task 2: 添加 Build Tags 门控

**目标**：为每个非内核文件添加 `//go:build` 标签，控制其编译层级。

**层级映射**：

| 文件 | 最低层级 | 原因 |
|------|----------|------|
| `fastpath.go` | L1 | 零分配热路径，依赖 ObservationAdapter |
| `entropy_metrics.go` | L1 | 基础熵值追踪 |
| `degradation.go` | L1 | 优雅降级 |
| `eventstore.go` | L2 | 内存事件溯源 |
| `eventbus.go` | L2 | 进程内事件总线 |
| `config.go` | L2 | 配置管理 |
| `patterns_resilience.go` | L2 | 熔断器 |
| `port_contract.go` | L2 | Port 合约 |
| `architecture_registry.go` | L2 | 架构注册表 |
| `eventstore_persistent.go` | L3 | 持久化事件存储 |
| `eventbus_persistent.go` | L3 | 持久化事件总线 |
| `storage_fs.go` | L3 | 文件存储 |
| `security.go` | L3 | 安全 HMAC |
| `transaction.go` | L3 | Saga 事务 |
| `guardian_static.go` | L3 | Go AST 静态分析 |
| `agent_submit.go` | L3 | Agent 代码提交 |
| `agent_runner.go` | L3 | Agent 编译执行 |
| `scheduler.go` | L3 | Agent 调度 |
| `handoff.go` | L3 | Agent 交接 |
| `handoff_persistence.go` | L3 | 快照持久化 |
| `schema.go` | L3 | Schema 注册表 |
| `guardian_decision.go` | L4 | 决策引擎 |
| `guardian_entropy.go` | L4 | 熵值监控 |
| `guardian_dependency.go` | L4 | 依赖图分析 |
| `guardian_transparency.go` | L4 | 透明度监控 |
| `observation_pipeline.go` | L4 | 观测管线 |
| `observation_sampler.go` | L4 | 采样 |
| `observation_aggregator.go` | L4 | 聚合 |
| `observation_store.go` | L4 | 观测存储 |
| `observation_api.go` | L4 | 观测 API |
| `perf_core.go` | L4 | 性能基础设施 |
| `perf_tdigest.go` | L4 | TDigest |
| `perf_sharded_observation.go` | L4 | 分片观测 |
| `projection.go` | L4 | 事件投射 |
| `idempotent.go` | L4 | 幂等 |
| `tenant.go` | L4 | 多租户 |
| `patterns_distributed.go` | L5 | 分布式韧性 |
| `eventstore_upgrade.go` | L5 | 事件存储升级 |
| `remote_composer.go` | L5 | 远程编排 |
| `app.go` | L5 | 应用启动器 |

**实现**：每个文件头部添加 `//go:build lecore_tierN || lecore_tier(N+1) || ... || lecore_tier7`

**文件变更**：~40 个文件添加 1 行

---

### Task 3: 实现 ComplexityTier 类型 + AppConfig

**新文件**：`complexity_profile.go`

**内容**：
- `ComplexityTier` 枚举类型（L0-L7 + AutoDetect）
- `AppConfig` 结构体（Tier 字段）
- `NewApp(cfg AppConfig)` 工厂函数
- Tier 常量定义

**文件变更**：+1 新文件

---

### Task 4: 实现 AutoDetect 项目扫描器

**新文件**：`auto_detect.go`

**内容**：
- `AutoDetect(root string) ComplexityTier` — 扫描项目文件数/语言/模块数
- `ProjectStats` 结构体
- `scanProject(root string) ProjectStats` — 内部实现

**文件变更**：+1 新文件

---

### Task 5: 适配测试

**问题**：测试文件在 L0 编译时需要引用 L3+ 的模块，需要将测试也按 tier 分层。

**策略**：
- `core_test.go`（内核测试）→ L0，始终编译
- 各模块测试加对应 tier 的 build tag
- 新增 `tier_test.go` — 验证每个 tier 能正确编译

**文件变更**：~5 个测试文件修改/新增

---

### Task 6: 各 tier 最小示例

**示例项目**：
- `examples/tier-l0-prototype/` — 仅内核，10 行代码
- `examples/tier-l1-microservice/` — 内核 + 观测 + 降级
- `examples/tier-l3-large-service/` — 内核 + Guardian + EventStore + Security

**文件变更**：+3 个示例目录

---

## 执行顺序

```
Task 1 (内核解耦) → Task 2 (Build Tags) → Task 3 (ComplexityTier) → Task 4 (AutoDetect) → Task 5 (测试) → Task 6 (示例)
```

每个 Task 完成后运行全量测试验证。