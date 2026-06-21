# Low-Entropy Core v0.12.0 开发计划

> 版本目标：修复测试环境、完成剩余文件拆分、增强 arch-manager 功能
> 生成时间：2026-06-21
> 前置版本：v0.11.0（C2 约束合规拆分 + 迁移面板 + 变动日志引擎）

---

## 总览

v0.11.0 完成了18个大文件的拆分（122 files changed, +17938/-10507），但仍有26个核心源文件超过300行，22个测试因 Windows 临时目录权限失败，arch-manager 的迁移面板和变动日志存在功能缺口。本计划将这三类问题拆分为最小任务单元，每个任务独立可验证。

**任务统计**：4个阶段、55个任务、预计产出 ~40 个新文件

---

## 阶段一：测试环境修复（T01~T04）

### 根因

`os.TempDir()` 在 Windows 上返回 `C:\Users\Administrator\AppData\Local\Temp\`，该目录无写入权限。影响22个测试，分布在4个测试文件中。

### T01 — 设置测试环境变量

- **目标**：在测试运行命令中添加 `$env:TMPDIR = "c:\temp"; $env:TMP = "c:\temp"`
- **影响范围**：所有使用 `t.TempDir()` 和 `os.TempDir()` 的测试
- **操作**：修改 `.trae/documents/file-split-plan.md` 中的测试命令模板，添加 TMPDIR/TMP 环境变量
- **验收**：运行 `go test ./go-core/... -count=1`，TempDir 相关的17个测试全部通过

### T02 — 修复 understand_test.go 手动路径拼接

- **目标**：将 `understand_test.go` 中5个测试的 `filepath.Join(os.TempDir(), ...)` 改为 `t.TempDir()`
- **文件**：`go-core/understand_test.go`
- **涉及测试**：
  - `TestUnderstandAdapter_LoadGraph`（行940）
  - `TestUnderstandAdapter_HasGraph`（行972）
  - `TestMigrationAdapter_CreateBaseline`（行1000）
  - `TestMigrationAdapter_SaveAndLoadBaseline`（行1030）
  - `TestNewMigrationStep`（行1081）
- **操作**：将 `os.TempDir()` 替换为 `t.TempDir()`，将 `os.MkdirAll` 替换为依赖 `t.TempDir()` 自动创建
- **验收**：5个测试全部通过

### T03 — 验证全量测试

- **目标**：确认所有22个测试修复后通过
- **操作**：运行 `go test ./go-core/... -count=1 -v 2>&1`
- **验收**：0 FAIL，migrate 包 38/38 通过

### T04 — 更新测试文档

- **目标**：在项目 README 或开发文档中记录 Windows 测试环境要求
- **操作**：记录 `TMPDIR=c:\temp` 环境变量要求
- **验收**：文档已更新

---

## 阶段二：Go 核心文件拆分（T05~T42）

### 拆分原则

- 同包拆分，不创建子包
- 每个新文件 ≤300 行
- 纯代码移动，不修改逻辑
- 每个文件只保留自己用到的 imports
- 每次拆分后 `go build ./go-core/` 验证

### T05 — scheduler.go（497行 → 4文件）

| 新文件 | 内容 | 预估行数 |
|--------|------|---------|
| `scheduler_agent_pool.go` | AgentStatus, AgentInfo, AgentPool, AgentPoolAdapter, AgentPoolOp | ~152 |
| `scheduler_queue.go` | QueuedTask, taskHeap, TaskQueue | ~125 |
| `scheduler_match.go` | MatchInput, MatchOutput, MatchEngine + hasAllCapabilities, findLongestIdle | ~71 |
| `scheduler_composer.go` | SchedulerComposer, ScheduleResult | ~125 |

### T06 — understand_migration.go（524行 → 3文件）

| 新文件 | 内容 | 预估行数 |
|--------|------|---------|
| `understand_migration_types.go` | MigrationBaseline, MigrationRequest, MigrationReport, GraphDiff, NodeDiff, EdgeDiff, DiffSummary, LayerDrift | ~81 |
| `understand_migration_diff.go` | DiffKnowledgeGraphs, DetectLayerDrifts + 8个未导出辅助函数 | ~247 |
| `understand_migration_adapter.go` | UnderstandMigrationAdapter, NewMigrationStep | ~166 |

### T07 — config.go（478行 → 3文件）

| 新文件 | 内容 | 预估行数 |
|--------|------|---------|
| `config_types.go` | PipelineConfig, StepConfig, AllowedStepTypes, isAllowedStepType | ~15 |
| `config_parse.go` | ParseConfig, ValidateConfig, ParseAndValidateConfig, AdapterResolver, MapAdapterResolver, PipelineBuilder, Build, composerAsStep | ~203 |
| `config_hotreload.go` | HotReload, LoadAppConfigFromFile, ApplyEnvOverrides, buildFromFile, computeFileHash, NewConfigChangeStep | ~226 |

### T08 — agent_submit.go（450行 → 3文件）

| 新文件 | 内容 | 预估行数 |
|--------|------|---------|
| `agent_types.go` | AgentCodeSubmission, PrimitiveManifest, SubmissionStatus, SubmissionResult, Violation, ValidPrimitiveTypes | ~116 |
| `agent_workbench.go` | AgentWorkbench interface, DefaultAgentWorkbench, Submit, SubmitAndRun, GetSubmission, ListSubmissionsByAgent | ~173 |
| `agent_registration.go` | validateBasic, RegisterAgent, AgentHeartbeat, DeregisterAgent, hasOnlyWarnings | ~129 |

### T09 — understand_supervisor.go（435行 → 2文件）

| 新文件 | 内容 | 预估行数 |
|--------|------|---------|
| `constraint_rules.go` | ConstraintStatus, ConstraintResult, ConstraintRule, ConstraintReport, DefaultConstraints + 6个check函数 + 辅助函数 | ~352 |
| `graph_supervisor.go` | GraphSupervisor, NewGraphSupervisor, ValidateAll, NewSupervisorStep | ~56 |

### T10 — guardian_transparency.go（401行 → 2文件）

| 新文件 | 内容 | 预估行数 |
|--------|------|---------|
| `guardian_transparency.go` | TransparencyWatcher + 所有方法 | ~164 |
| `guardian_drift.go` | DriftDetector + DriftInput, DriftOutput, DriftType + 辅助函数 | ~211 |

### T11 — guardian_decision.go（374行 → 2文件）

| 新文件 | 内容 | 预估行数 |
|--------|------|---------|
| `guardian_decision.go` | GuardianAction, GuardianInput, GuardianDecision, DecisionEngine + 所有方法 | ~213 |
| `guardian_alert.go` | AlertChannel, AlertResult, AlertConfig, AlertAdapter + 所有方法 | ~134 |

### T12 — observation.go（372行 → 3文件）

| 新文件 | 内容 | 预估行数 |
|--------|------|---------|
| `execution_step.go` | ExecutionStep, CompactExecutionStep + 转换方法 + 构造函数 | ~133 |
| `trace_tree.go` | TraceNode, TraceTree, BuildTraceTree + Flatten, TotalNodes | ~94 |
| `observation_adapter.go` | ObservationAdapter interface, InMemoryObservationAdapter, NoOpObservationAdapter, TraceContext | ~125 |

### T13 — schema.go（365行 → 3文件）

| 新文件 | 内容 | 预估行数 |
|--------|------|---------|
| `schema_registry.go` | SchemaRegistry + 所有方法 + buildSchemaKey | ~80 |
| `schema_migration.go` | MigrationFunc, MigrationChain + BFS + parseTransition | ~105 |
| `schema_compat.go` | CompatibilityChecker + diff类型 + collectFields, buildSummary | ~157 |

### T14 — understand_observer.go（366行 → 3文件）

| 新文件 | 内容 | 预估行数 |
|--------|------|---------|
| `observer_types.go` | StructureObservation, ComplexityStats, ChangeObservation, SearchQuery, SearchResult, SearchHit | ~48 |
| `graph_index.go` | GraphIndex + tokenize, containsInTags | ~163 |
| `graph_observer.go` | GraphObserver + Step包装函数 | ~105 |

### T15 — understand_types.go（315行 → 2文件）

| 新文件 | 内容 | 预估行数 |
|--------|------|---------|
| `kg_types.go` | 所有 struct + 21个节点类型常量 + 35个边类型常量 + ValidNodeTypes/ValidEdgeTypes | ~194 |
| `kg_parse.go` | ParseKnowledgeGraph, ParseKnowledgeGraphFile, ValidateKnowledgeGraph, CountByType, CountByEdgeType | ~103 |

### T16 — perf_tdigest.go（474行 → 3文件）

| 新文件 | 内容 | 预估行数 |
|--------|------|---------|
| `perf_tdigest.go` | TDigest + tdCentroid + 常量 + 所有方法（保留 build tag） | ~275 |
| `perf_drift.go` | DriftDirection, DistributionDriftResult, DistributionDriftDetector（保留 build tag） | ~90 |
| `perf_anomaly.go` | AnomalyLabelType, AnomalyAutoLabeler（保留 build tag） | ~110 |

### T17 — observability.go（345行 → 2文件）

| 新文件 | 内容 | 预估行数 |
|--------|------|---------|
| `observability_interfaces.go` | 所有接口定义 + ObservabilityProvider + option 类型 + 工厂函数 | ~250 |
| `observability_noop.go` | NewNoOpObservabilityProvider + 所有 noOp 实现 | ~73 |

### T18 — config_enhanced.go（318行 → 3文件）

| 新文件 | 内容 | 预估行数 |
|--------|------|---------|
| `config_loader.go` | ConfigLoader + LoadFromFile, Get, OnReload, reload | ~74 |
| `config_secrets.go` | SecretResolver, EnvSecretResolver, FileSecretResolver + resolveSecrets, resolveSecretRef | ~94 |
| `config_validation_watcher.go` | ValidateConfigEnhanced, MustValidateConfigEnhanced, ConfigWatcher | ~127 |

### T19 — migration_analyze.go（311行 → 2文件）

| 新文件 | 内容 | 预估行数 |
|--------|------|---------|
| `migration_analyze_types.go` | RiskLevel, ModuleStatus, ModuleMigration, MigrationRisk, RollbackStep, MigrationPlan | ~89 |
| `migration_analyze.go` | MigrateAnalyze + 辅助函数 + MigrationPlan 方法（覆盖原文件） | ~222 |

### T20 — cmd/arch-manager/version.go（397行 → 2文件）

| 新文件 | 内容 | 预估行数 |
|--------|------|---------|
| `version_handlers.go` | 所有 handleVersion* handler 函数 | ~200 |
| `version_data.go` | 所有 getBuiltin* 数据函数 + getString/getBool 工具函数 | ~190 |

### T21 — cmd/arch-manager/simulate.go（364行 → 2文件）

| 新文件 | 内容 | 预估行数 |
|--------|------|---------|
| `simulate.go` | SimulateResult + handleSimulate + parsePercent | ~150 |
| `entropy_observe.go` | EntropyMetrics, ObservedMetrics + handleEntropyCheck + handleObserveCheck | ~210 |

### T22 — cmd/arch-manager/parser.go（356行 → 2文件）

| 新文件 | 内容 | 预估行数 |
|--------|------|---------|
| `parser_file.go` | parseFile + resolveInternalDeps + collectCalledFunctions | ~130 |
| `parser_ast.go` | parseTypeSpec + buildFuncSignature + funcTypeToString + exprToString | ~230 |

### T23 — cmd/arch-manager/migrate_api.go（347行 → 2文件）

| 新文件 | 内容 | 预估行数 |
|--------|------|---------|
| `migrate_api.go` | migrateState, migrateSessionInfo + 会话/日志/状态查询 handlers | ~180 |
| `migrate_analyze.go` | handleMigrateAnalyze + handleMigrateValidate | ~170 |

### T24 — cmd/arch-manager/main.go（309行 → 2文件）

| 新文件 | 内容 | 预估行数 |
|--------|------|---------|
| `main.go` | 全局变量 + main 函数（参数解析+启动服务器） | ~80 |
| `routes.go` | handleAPI/handleFile/handleRefresh/handleHealth + registerRoutes 函数 | ~230 |

### T25 — go-core/security_jwt.go（355行 → 2文件）

| 新文件 | 内容 | 预估行数 |
|--------|------|---------|
| `security_jwt_types.go` | JWTClaims（含方法）, JWTToken, JWTHeader, ParseJWT + base64工具 | ~100 |
| `security_jwt_service.go` | JWTService, JWTConfig, NewJWTService + 所有 Service 方法 | ~170 |

### T26 — cmd/lec/main.go（322行 → 2文件）

| 新文件 | 内容 | 预估行数 |
|--------|------|---------|
| `main.go` | init + main + runCmd | ~100 |
| `scaffold.go` | TemplateData, templateFile + tierLabel/tierNum + collectTemplateFiles + readTemplate | ~160 |

### T27 — go-core/schema_test.go（456行 → 3文件）

| 新文件 | 内容 | 预估行数 |
|--------|------|---------|
| `schema_registry_test.go` | SchemaRegistry 相关6个测试 + 辅助类型 | ~190 |
| `schema_compat_test.go` | CompatibilityChecker 相关4个测试 | ~130 |
| `schema_migrate_test.go` | MigrationChain 相关5个测试 + 辅助类型 | ~140 |

### T28~T42 — 可选拆分（≤337行，内聚性高）

以下文件行数在300~337之间，内聚性好，可按需拆分：

| 任务 | 文件 | 行数 | 拆分方案 | 优先级 |
|------|------|------|---------|--------|
| T28 | `version_adr.go` | 312 | adr_markdown.go（序列化/反序列化） | 低 |
| T29 | `storage_fs.go` | 337 | storage_helpers.go（JSON辅助+Stat） | 低 |
| T30 | `eventstore_postgres.go` | 327 | eventstore_postgres_ddl.go（建表逻辑） | 低 |
| T31 | `observation_aggregator.go` | 307 | 无需拆分（仅超7行） | 无 |

### 编译验证检查点

- **T42**：`go build ./go-core/` + `go build ./go-core/migrate/` + `go build ./cmd/arch-manager/` + `go build ./go-core/cmd/lint/` + `go build ./cmd/lec/` 全部通过
- **T42**：`go test ./go-core/migrate/ -count=1` 38/38 通过
- **T42**：`go test ./go-core/ -count=1` 0 FAIL

---

## 阶段三：arch-manager 功能增强（T43~T50）

### T43 — 迁移执行 API

- **目标**：添加 `POST /api/migrate/execute` 和 `POST /api/migrate/rollback` 端点
- **文件**：新建 `cmd/arch-manager/migrate_execute.go`
- **内容**：
  - `handleMigrateExecute`：接收目标目录和迁移选项，调用 migrate 包执行迁移
  - `handleMigrateRollback`：接收会话ID，执行回滚
  - 迁移进度通过 `migrateEventBus` 推送 SSE 事件
- **预估行数**：~150
- **验收**：`curl -X POST /api/migrate/execute -d '{"dir":"..."}'` 返回迁移结果

### T44 — 迁移 SSE 事件补全

- **目标**：在迁移执行过程中推送进度事件
- **文件**：修改 `cmd/arch-manager/migrate_execute.go` 和 `cmd/arch-manager/migrate_sse.go`
- **新增事件类型**：
  - `migration_file_start`：开始处理单个文件
  - `migration_file_done`：单个文件处理完成
  - `migration_complete`：整个迁移完成
  - `migration_error`：迁移出错
- **验收**：执行迁移时 SSE 客户端收到完整进度流

### T45 — SSE 心跳机制

- **目标**：所有 SSE 端点每30秒发送 ping 事件
- **文件**：修改 `cmd/arch-manager/sse.go`
- **实现**：在 SSE handler 中添加 `time.Ticker(30s)`，发送 `{"type":"ping","ts":...}` 事件
- **涉及端点**：`/api/sse`, `/api/sse/dev-events`, `/api/sse/migrate`, `/api/sse/arch-changelog`, `/api/guardian/sse`
- **验收**：SSE 连接空闲时每30秒收到 ping 事件

### T46 — 变动日志写入 API

- **目标**：添加 `POST /api/arch-changelog` 手动写入端点
- **文件**：修改 `cmd/arch-manager/arch_changelog_api.go`
- **内容**：`handleArchChangelogCreate` 接收 category, severity, message, source 字段
- **验收**：`curl -X POST /api/arch-changelog -d '{"category":"manual","severity":"info","message":"test"}'` 返回创建的条目

### T47 — 变动日志导出 API

- **目标**：添加 `GET /api/arch-changelog/export` 导出端点
- **文件**：修改 `cmd/arch-manager/arch_changelog_api.go`
- **内容**：支持 json/md/csv 格式导出，参考 `handleMigrateLogsExport` 实现
- **验收**：`curl /api/arch-changelog/export?format=md` 返回 Markdown 格式日志

### T48 — 变动日志前端 SSE 订阅

- **目标**：`arch-manager-changelog.js` 连接 `/api/sse/arch-changelog` 实时推送
- **文件**：修改 `arch-manager-changelog.js`
- **实现**：在 `init()` 中添加 EventSource 连接，收到事件时自动追加到列表顶部
- **验收**：文件变动时 changelog 面板实时更新，无需手动刷新

### T49 — 变动日志分页控件

- **目标**：前端 changelog 面板添加分页导航
- **文件**：修改 `arch-manager-changelog.js`
- **实现**：每页50条，添加上一页/下一页按钮，传递 offset/limit 参数
- **验收**：超过50条日志时显示分页控件，翻页正确加载数据

### T50 — 迁移会话取消 API

- **目标**：添加 `DELETE /api/migrate/sessions/{id}` 取消迁移会话
- **文件**：修改 `cmd/arch-manager/migrate_api.go`
- **实现**：标记会话为 cancelled 状态，停止正在执行的迁移
- **验收**：`curl -X DELETE /api/migrate/sessions/{id}` 返回成功

---

## 阶段四：最终验证与发布（T51~T55）

### T51 — 全量编译验证

- **操作**：
  ```
  go build ./go-core/
  go build ./go-core/migrate/
  go build ./cmd/arch-manager/
  go build ./go-core/cmd/lint/
  go build ./cmd/lec/
  ```
- **验收**：5个模块全部编译通过，0 错误

### T52 — 全量测试

- **操作**：
  ```
  $env:TMPDIR = "c:\temp"; $env:TMP = "c:\temp"
  go test ./go-core/... -count=1
  go test ./go-core/migrate/ -count=1
  ```
- **验收**：0 FAIL

### T53 — 行数约束审计

- **操作**：运行行数检查脚本，确认所有核心源文件 ≤300 行
- **验收**：0 个核心源文件超过300行（HTML/CSS/测试文件除外）

### T54 — arch-manager 功能验证

- **操作**：启动 arch-manager，逐面板检查
- **验收**：
  - 28个面板全部正常渲染
  - 5个 SSE 连接全部建立（主事件流、开发进度、迁移、变动日志、Guardian）
  - fetchAll 26个 API 请求全部返回
  - 迁移面板：分析、验证、执行、回滚全部可用
  - 变动日志：查询、导出、SSE 实时推送全部可用
  - 分页控件正常工作

### T55 — 提交、打 Tag、发布 Release

- **操作**：
  1. `git add` 所有变更
  2. `git commit` 使用 conventional commit 格式
  3. `git tag -a v0.12.0`
  4. `git push origin master --tags`
  5. `gh release create v0.12.0` 附带完整变更说明
- **验收**：GitHub Release 页面可访问，tag 已推送

---

## 验收标准总表

| 维度 | 标准 | 验证方式 |
|------|------|---------|
| 编译 | 5个模块 `go build` 全部通过 | T51 |
| 测试 | `go test ./go-core/...` 0 FAIL | T52 |
| 行数 | 核心源文件全部 ≤300 行 | T53 |
| 迁移测试 | migrate 包 38/38 通过 | T52 |
| 面板渲染 | 28个面板全部正常 | T54 |
| SSE 连接 | 5个 SSE 端点全部建立 | T54 |
| API 完整 | 迁移 CRUD + 变动日志 CRUD + 导出 | T54 |
| 心跳 | SSE 空闲时每30秒 ping | T45 |
| 发布 | v0.12.0 tag + GitHub Release | T55 |

---

## 执行依赖图

```
阶段一（测试修复）          阶段二（文件拆分）         阶段三（功能增强）        阶段四（验证发布）
  T01 ──┐
  T02 ──┼── T03 ── T04       T05~T27（可并行）         T43~T50（依赖阶段二）     T51~T55（依赖全部）
        ┘
```

- 阶段一和阶段二可并行执行
- 阶段三依赖阶段二完成（需要新增的文件结构稳定）
- 阶段四依赖阶段一~三全部完成
