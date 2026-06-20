# 大文件拆分 — 零漂移最小任务单元开发文档

> **版本**: v1.0 | **日期**: 2026-06-21 | **目标**: 全部文件 ≤300 行，整体熵最低
> **核心约束**: C1(任意语言) C2(架构约束) C3(原子日志) C4(完整CLI)
> **熵值公式**: H = -Σ(fi/N)·log2(fi/N)，fi = 文件行数，N = 总行数。拆分越多文件越均匀，熵越高（越低熵越有序，但单文件过大会导致局部熵爆炸）

---

## 1. 现状分析

### 1.1 大文件清单（>300行，排除测试和文档）

| # | 文件 | 行数 | 模块 | 核心问题 |
|---|------|------|------|---------|
| F01 | `arch-manager.html` | 2970 | 前端 | 61个函数混合：CSS+HTML+JS+SSE+28面板渲染 |
| F02 | `scripts/lec.ps1` | 1658 | CLI | 16命令+4模板生成器+13辅助函数 |
| F03 | `cmd/lint/main.go` | 721 | lint | 4条规则+数据声明+辅助函数全在一个文件 |
| F04 | `go-core/guardian_entropy.go` | 771 | guardian | 6个独立子系统（EntropyWatcher/ModuleTracker/GrowthDetector/DriftDetector/AdaptiveThreshold/MultiDimension） |
| F05 | `go-core/composer.go` | 719 | core | 12个Composer变体（Pipeline/Branch/Parallel/Retry/Timeout/Map/Stream/FanOut/Debounce/Throttle） |
| F06 | `go-core/perf_sharded_observation.go` | 710 | perf | 分片观测相关 |
| F07 | `go-core/observation_aggregator.go` | 675 | observation | 3个聚合器（Batch/Incremental/Sharded） |
| F08 | `go-core/handoff.go` | 673 | handoff | 6个子系统（Snapshot/Contract/Legacy/NewHandoff/HandoffComposer/Rollback/Transport） |
| F09 | `go-core/guardian_static.go` | 665 | guardian | 1个Port+5个检查器+辅助函数 |
| F10 | `go-core/perf_core.go` | 645 | perf | 5个独立组件（ShardedLock/AtomicState/BatchedUUID/CompactTraceID/Pool集合） |
| F11 | `go-core/eventstore_upgrade.go` | 618 | eventstore | 5个独立子系统（ShardedStore/AutoSnapshot/Compaction/WorkerPool/ProjectionCheckpoint） |
| F12 | `go-core/guardian_dependency.go` | 614 | guardian | 5个独立子系统（DependencyGraph/DependencyGuard/SnapshotStore/ArchitectureGuard/typeSet） |
| F13 | `cmd/arch-manager/ua.go` | 566 | arch-manager | 10个类型+图谱生成+验证+搜索 |
| F14 | `go-core/patterns_distributed.go` | 554 | patterns | 4个独立子系统（CircuitBreaker/DegradationManager/RateLimiter/HealthChecker） |
| F15 | `go-core/migrate/parser_go.go` | 543 | migrate | 1个类型+20个方法/函数 |
| F16 | `go-core/patterns_resilience_enhanced.go` | 542 | patterns | 5个独立子系统（TokenBucket/EnhancedCircuit/Backoff/Retry/Middleware） |
| F17 | `go-core/security.go` | 538 | security | 4个独立子系统（Capability/AccessControl/AuditTrail/MerkleChain） |
| F18 | `go-core/patterns_resilience.go` | 537 | patterns | 6个独立子系统（CircuitBreaker/Fallback/Bulkhead/RateLimiter/ShardedRateLimiter/ResilienceChain） |

### 1.2 拆分原则

1. **单一职责**: 每个文件只包含一个逻辑子系统
2. **≤300行**: 拆分后每个文件不超过 300 行
3. **同包拆分**: Go 文件在同一 package 内拆分，不创建子包（避免循环依赖）
4. **最小变更**: 只移动代码，不修改逻辑
5. **测试不变**: 测试文件无需修改（同包内类型/函数可见性不变）
6. **前端模块化**: HTML 拆为多文件 JS 模块，通过 `<script src>` 引入

---

## 2. 拆分任务总览

### 按优先级分组

| 优先级 | 文件 | 拆分后文件数 | 任务数 |
|--------|------|------------|--------|
| **P0** | F01 arch-manager.html | 1 HTML + 7 JS + 1 CSS | 9 |
| **P0** | F02 lec.ps1 | 1 入口 + 5 模块 | 6 |
| **P1** | F04 guardian_entropy.go | 6 | 6 |
| **P1** | F05 composer.go | 6 | 6 |
| **P1** | F07 observation_aggregator.go | 3 | 3 |
| **P1** | F08 handoff.go | 5 | 5 |
| **P1** | F10 perf_core.go | 5 | 5 |
| **P2** | F03 lint/main.go | 5 | 5 |
| **P2** | F09 guardian_static.go | 4 | 4 |
| **P2** | F11 eventstore_upgrade.go | 5 | 5 |
| **P2** | F12 guardian_dependency.go | 5 | 5 |
| **P2** | F13 ua.go | 3 | 3 |
| **P2** | F14 patterns_distributed.go | 4 | 4 |
| **P2** | F15 parser_go.go | 3 | 3 |
| **P2** | F16 patterns_resilience_enhanced.go | 4 | 4 |
| **P2** | F17 security.go | 4 | 4 |
| **P2** | F18 patterns_resilience.go | 5 | 5 |

**总计**: 18 个大文件 → 85 个小文件，**91 个最小任务单元**

---

## 3. P0: arch-manager.html 拆分（2970行 → 9个文件）

### 现状结构

| 区域 | 行号 | 行数 |
|------|------|------|
| CSS | 8–572 | 565 |
| HTML body | 574–857 | 284 |
| JS: 全局变量+API+fetchAll+SSE | 859–1076 | 218 |
| JS: 工具函数+视图路由 | 1078–1230 | 153 |
| JS: 28个渲染函数 | 1232–2968 | 1737 |

### 拆分方案

#### T01: 提取 CSS 到独立文件

**新建**: `arch-manager.css`
**内容**: 第 8–572 行的全部 CSS（565 行）
**修改**: `arch-manager.html` 第 8–572 行替换为 `<link rel="stylesheet" href="arch-manager.css">`

#### T02: 提取全局状态和工具函数

**新建**: `arch-manager-core.js`
**内容**:
- 第 860–887 行：全局变量声明
- 第 888–890 行：`api(url)` 函数
- 第 895–919 行：`queryObsSteps`, `fetchObsErrors`, `fetchObsTrace`, `queryObsAggregates`
- 第 1081–1165 行：`updateTopBar`, `updateOverviewDash`, `updateClock`
- 第 1169–1185 行：`toggleSection`, `switchView`, `renderCurrentView`
- 第 1225–1230 行：`disposeCharts`
- 第 2574–2595 行：`showDetail`, `esc`, `toast`
**预估行数**: ~200 行

#### T03: 提取数据加载和 SSE

**新建**: `arch-manager-data.js`
**内容**:
- 第 921–977 行：`fetchAll()`（26 个 API 并发请求）
- 第 982–1076 行：`connectSSE()`（5 个 EventSource）
- 第 2651–2670 行：`init()` 入口函数
**预估行数**: ~150 行

#### T04: 提取全景面板渲染函数

**新建**: `arch-manager-panorama.js`
**内容**:
- `renderFileTreeView` (1233–1323, 91行)
- `showFileDetail` (1325–1356, 32行)
- `renderTopology` (1358–1445, 88行)
- `renderHealth` (1447–1556, 110行)
- `renderViolationsView` (1558–1588, 31行)
- `renderPrimitivesView` (1590–1621, 32行)
- `renderLayerMatrix` (1623–1653, 31行)
**预估行数**: ~420 行 → 需要再拆分为 `panorama-views.js` 和 `panorama-charts.js`

**修正**: 拆为 2 个文件:
- `arch-manager-panorama.js`: renderFileTreeView + showFileDetail + renderViolationsView + renderPrimitivesView + renderLayerMatrix (~220行)
- `arch-manager-charts.js`: renderTopology + renderHealth（含 radar/gauge/trend 图表）(~250行)

#### T05: 提取运行时面板渲染函数

**新建**: `arch-manager-runtime.js`
**内容**:
- `renderHeartbeatView` + `renderHeartbeat` (1655–1803, 149行)
- `renderEntropyHeatmapView` + `renderEntropyHeatmap` (1750–1809, 60行)
- `renderErrorBPView` (1811–1866, 56行)
- `renderNeuralTraceView` (1868–1919, 52行)
- `renderDataFlowView` (1921–1986, 66行)
**预估行数**: ~383 行 → 拆为 2 个文件

**修正**: 拆为 2 个文件:
- `arch-manager-runtime.js`: renderHeartbeatView + renderHeartbeat + renderEntropyHeatmapView + renderEntropyHeatmap (~200行)
- `arch-manager-runtime2.js`: renderErrorBPView + renderNeuralTraceView + renderDataFlowView (~180行)

#### T06: 提取可观测性面板渲染函数

**新建**: `arch-manager-observation.js`
**内容**:
- `renderObsSteps` (1988–2041, 54行)
- `renderObsTraceDetail` (2043–2082, 40行)
- `renderObsAggregates` (2084–2163, 80行)
- `renderObsPipelines` (2165–2192, 28行)
- `renderObsArch` (2194–2243, 50行)
- `renderDevProgress` (2598–2649, 52行)
**预估行数**: ~305 行 → 接近上限，保留

#### T07: 提取控制和溯源面板渲染函数

**新建**: `arch-manager-control.js`
**内容**:
- `renderSamplingView` + `applySampling` + `resetSampling` (2245–2299, 55行)
- `renderThresholdView` + `overrideThreshold` + `saveThresholds` (2301–2343, 43行)
- `renderWhatIfView` + `runWhatIf` (2345–2403, 59行)
- `renderCausationView` (2405–2469, 65行)
- `renderTimeTravelView` + `travelTime` (2471–2515, 45行)
- `renderAttributionView` (2517–2572, 56行)
**预估行数**: ~323 行 → 拆为 2 个文件

**修正**: 拆为 2 个文件:
- `arch-manager-control.js`: renderSamplingView + renderThresholdView + renderWhatIfView (~200行)
- `arch-manager-trace.js`: renderCausationView + renderTimeTravelView + renderAttributionView (~170行)

#### T08: 提取迁移面板渲染函数

**新建**: `arch-manager-migration.js`
**内容**:
- `renderMigStatus` + `triggerMigrateAnalyze` + `triggerMigrateValidate` (2675–2731, 57行)
- `renderMigPatternMap` (2733–2790, 58行)
- `renderMigGateChain` (2792–2837, 46行)
- `renderMigLog` + `fetchMigLogs` + `exportMigLogs` (2839–2888, 50行)
- `renderMigSessions` (2890–2908, 19行)
- `renderArchChangelog` + `fetchArchChangelog` (2910–2968, 59行)
**预估行数**: ~290 行

#### T09: 修改 arch-manager.html 为入口文件

**修改**: `arch-manager.html`
- 删除全部 CSS → `<link rel="stylesheet" href="arch-manager.css">`
- 删除全部 JS → 添加 `<script>` 标签引入 8 个 JS 文件
- 保留 HTML body（574–857 行，284 行）
- 最终行数: ~300 行

**引入顺序**（有依赖关系的必须按序）:
```html
<script src="arch-manager-core.js"></script>
<script src="arch-manager-data.js"></script>
<script src="arch-manager-panorama.js"></script>
<script src="arch-manager-charts.js"></script>
<script src="arch-manager-runtime.js"></script>
<script src="arch-manager-runtime2.js"></script>
<script src="arch-manager-observation.js"></script>
<script src="arch-manager-control.js"></script>
<script src="arch-manager-trace.js"></script>
<script src="arch-manager-migration.js"></script>
```

### 验收检查清单

- [ ] `arch-manager.html` ≤ 300 行
- [ ] `arch-manager.css` ≤ 600 行（CSS 单文件可接受稍大）
- [ ] 每个 JS 文件 ≤ 310 行
- [ ] 启动 arch-manager 后所有 28 个面板正常渲染
- [ ] SSE 连接正常（5 条）
- [ ] fetchAll 正常加载 26 个 API
- [ ] ECharts 图表正常初始化和销毁

---

## 4. P0: lec.ps1 拆分（1658行 → 6个文件）

### 现状结构

| 区域 | 行号 | 行数 |
|------|------|------|
| 头部+param+helper | 1–186 | 186 |
| Cmd-Version + Cmd-List | 192–221 | 30 |
| Cmd-Init | 227–447 | 221 |
| Generate-L0 | 453–531 | 79 |
| Generate-L1 | 533–673 | 141 |
| Generate-L3 | 675–939 | 265 |
| Cmd-Add + 4个Snippet | 945–1115 | 171 |
| Cmd-Check | 1121–1209 | 89 |
| Cmd-Upgrade | 1215–1340 | 126 |
| 迁移命令(8个) | 1346–1599 | 254 |
| Cmd-Help + Router | 1605–1658 | 54 |

### 拆分方案

#### T10: 创建入口文件

**新建**: `scripts/lec.ps1`（重写为入口点）
**内容**:
- 头部注释 + 版本号
- param() 参数定义（24–66 行）
- dot-source 引入所有模块
- Command Router switch
**预估行数**: ~100 行

```powershell
# lec.ps1 — 入口文件
$LEC_VERSION = "0.3.0"
param(
    [Parameter(Position=0)][string]$Command = "help",
    [Parameter(Position=1, ValueFromRemainingArguments)][string[]]$RestArgs = @(),
    [string]$Tier = "l0", [string]$Module = "", [string]$Target = "",
    [string]$Type = "", [string]$Name = "",
    [string]$Lang = "auto", [string]$Output = "text",
    [double]$Threshold = 0.4, [switch]$GateOnly,
    [switch]$Force, [switch]$DryRun, [switch]$Fix,
    [string]$Step = "last", [switch]$All,
    [string]$CorePath = "", [string]$Only = "all", [string]$Skip = "none",
    [switch]$Detailed
)
$PSScriptRoot = if ($PSScriptRoot) { $PSScriptRoot } else { Split-Path $MyInvocation.MyCommand.Path }
. "$PSScriptRoot\lec-helpers.ps1"
. "$PSScriptRoot\lec-init.ps1"
. "$PSScriptRoot\lec-templates.ps1"
. "$PSScriptRoot\lec-commands.ps1"
. "$PSScriptRoot\lec-migrate.ps1"

switch ($Command) {
    "init"      { Cmd-Init }
    "add"       { Cmd-Add }
    "check"     { Cmd-Check }
    "upgrade"   { Cmd-Upgrade }
    "list"      { Cmd-List }
    "version"   { Cmd-Version }
    "analyze"   { Cmd-Analyze }
    "pattern"   { Cmd-Pattern }
    "plan"      { Cmd-Plan }
    "migrate"   { Cmd-Migrate }
    "log"       { Cmd-Log }
    "validate"  { Cmd-Validate }
    "rollback"  { Cmd-Rollback }
    "shim"      { Cmd-Shim }
    "help"      { Cmd-Help }
    default     { Cmd-Help }
}
```

#### T11: 创建辅助函数模块

**新建**: `scripts/lec-helpers.ps1`
**内容**: 第 74–186 行全部 helper 函数（113 行）
- Write-Header, Write-Ok, Write-Err, Write-Warn, Write-Info, Write-File
- Get-TierLabel, Get-TierNum, Get-CoreTag, Get-CoreModule
- Get-ProjectTier, Get-GoFiles, Resolve-CorePath

#### T12: 创建 init 命令模块

**新建**: `scripts/lec-init.ps1`
**内容**: Cmd-Init 函数（227–447 行，221 行）
- Cmd-Init 完整函数体

#### T13: 创建模板生成模块

**新建**: `scripts/lec-templates.ps1`
**内容**:
- Generate-L0 (453–531, 79行)
- Generate-L1 (533–673, 141行)
- Generate-L3 (675–939, 265行)
- Generate-AtomSnippet (1023–1044, 22行)
- Generate-PortSnippet (1046–1069, 24行)
- Generate-AdapterSnippet (1071–1092, 22行)
- Generate-ComposerSnippet (1094–1115, 22行)
**预估行数**: ~575 行 → 拆为 2 个文件

**修正**: 拆为 2 个文件:
- `scripts/lec-templates.ps1`: Generate-L0 + Generate-L1 (~220行)
- `scripts/lec-templates2.ps1`: Generate-L3 + 4个Snippet (~310行) → Generate-L3 本身 265 行，加 4 个 snippet 90 行 = 355 行 → 需要再拆

**最终方案**: 拆为 3 个文件:
- `scripts/lec-templates.ps1`: Generate-L0 + Generate-L1 (~220行)
- `scripts/lec-templates-l3.ps1`: Generate-L3 (~265行)
- `scripts/lec-templates-snippets.ps1`: 4个 Generate-*Snippet (~90行)

#### T14: 创建项目命令模块

**新建**: `scripts/lec-commands.ps1`
**内容**:
- Cmd-Version (192–197, 6行)
- Cmd-List (203–221, 19行)
- Cmd-Add (945–1021, 77行)
- Cmd-Check (1121–1209, 89行)
- Cmd-Upgrade (1215–1340, 126行)
- Cmd-Help (1605–1635, 31行)
**预估行数**: ~350 行 → 拆为 2 个文件

**修正**: 拆为 2 个文件:
- `scripts/lec-commands.ps1`: Cmd-Add + Cmd-Check + Cmd-Upgrade (~295行)
- `scripts/lec-commands2.ps1`: Cmd-Version + Cmd-List + Cmd-Help (~60行)

#### T15: 创建迁移命令模块

**新建**: `scripts/lec-migrate.ps1`
**内容**: 第 1346–1599 行全部迁移命令（254 行）
- Cmd-Analyze, Cmd-Pattern, Cmd-Plan, Cmd-Migrate
- Cmd-Log, Cmd-Validate, Cmd-Rollback, Cmd-Shim

### 验收检查清单

- [ ] `lec.ps1` ≤ 100 行
- [ ] 每个子模块 ≤ 310 行
- [ ] `.\lec.ps1 help` 正常输出
- [ ] `.\lec.ps1 init -Tier l1 test-proj` 正常创建项目
- [ ] `.\lec.ps1 version` 输出 0.3.0
- [ ] 所有 16 个命令均可正常调用

---

## 5. P1: Go 核心文件拆分

### 5.1 F04: guardian_entropy.go（771行 → 6个文件）

| 新文件 | 内容 | 行数 |
|--------|------|------|
| `guardian_entropy_watcher.go` | EntropyWatcher + EntropyAlert + EntropyLevel (29–162) | ~134 |
| `guardian_entropy_module.go` | ModuleEntropyTracker + ModuleEntropyState (164–281) | ~118 |
| `guardian_entropy_growth.go` | PipelineStepGrowthDetector (283–392) | ~110 |
| `guardian_entropy_drift.go` | AgentBehaviorDriftDetector (394–529) | ~136 |
| `guardian_entropy_adaptive.go` | AdaptiveThresholdEngine (531–673) | ~143 |
| `guardian_entropy_multidim.go` | MultiDimensionEntropySnapshot + Collector (675–771) | ~97 |

**任务**: T16~T21（6个任务，每个任务 = 创建新文件 + 删除原文件中对应代码 + 编译验证）

### 5.2 F05: composer.go（719行 → 6个文件）

| 新文件 | 内容 | 行数 |
|--------|------|------|
| `composer.go` | Composer 接口 + Pipeline + Branch (18–224) | ~207 |
| `composer_parallel.go` | Parallel + WithRetry + WithTimeout (226–396) | ~171 |
| `composer_map.go` | Map + Compose (398–429) | ~32 → 合入 composer.go |
| `composer_stream.go` | Stream 全部函数 (431–610) | ~180 |
| `composer_fanout.go` | FanOut + Debounce + Throttle (612–719) | ~108 |

**修正**: 5 个文件:
- `composer.go`: 接口 + Pipeline + Branch + Map + Compose (18–429, ~240行)
- `composer_parallel.go`: Parallel + WithRetry + WithTimeout (226–396, ~171行)
- `composer_stream.go`: Stream 全部 (431–610, ~180行)
- `composer_fanout.go`: FanOut + Debounce + Throttle (612–719, ~108行)

**任务**: T22~T25（4个任务）

### 5.3 F07: observation_aggregator.go（675行 → 3个文件）

| 新文件 | 内容 | 行数 |
|--------|------|------|
| `observation_aggregator.go` | Aggregator（批量）(11–307) | ~297 |
| `observation_aggregator_incremental.go` | IncrementalAggregator (309–528) | ~220 |
| `observation_aggregator_sharded.go` | ShardedAggregator (530–675) | ~146 |

**任务**: T26~T28（3个任务）

### 5.4 F08: handoff.go（673行 → 5个文件）

| 新文件 | 内容 | 行数 |
|--------|------|------|
| `handoff_types.go` | DevSnapshot + Artifact + WorkItem + Decision + Contract + Legacy (22–197) | ~176 |
| `handoff_composer.go` | NewHandoff + HandoffComposer + Execute + ReceiveSnapshot (199–455) | ~257 |
| `handoff_rollback.go` | RollbackResult + RollbackHandoff + HandoffWithRollback (457–534) | ~78 |
| `handoff_transport.go` | HandoffTransport + InProc + HTTP + Adapters (536–673) | ~138 |

**任务**: T29~T32（4个任务）

### 5.5 F10: perf_core.go（645行 → 5个文件）

| 新文件 | 内容 | 行数 |
|--------|------|------|
| `perf_core.go` | 常量 + ShardedLock (27–140) | ~114 |
| `perf_atomic.go` | AtomicState (142–238) | ~97 |
| `perf_uuid.go` | BatchedUUIDGen (240–391) | ~152 |
| `perf_traceid.go` | CompactTraceID (393–464) | ~72 |
| `perf_pools.go` | sync.Pool 集合 (466–645) | ~180 |

**任务**: T33~T37（5个任务）

---

## 6. P2: Go 次优先级文件拆分

### 6.1 F03: cmd/lint/main.go（721行 → 5个文件）

| 新文件 | 内容 | 行数 |
|--------|------|------|
| `main.go` | main + checkFile + report (254–309) + 数据声明 (30–248) | ~274 |
| `lint_rule1_types.go` | checkRule1_NoNon4PrimitiveTypes (311–401) | ~91 |
| `lint_rule2_io.go` | checkRule2_NoIOInAtomPort (403–482) | ~80 |
| `lint_rule3_assert.go` | checkRule3_NoConcreteTypeAssertions (484–631) | ~148 |
| `lint_rule4_pipeline.go` | checkRule4_PipelineStepLimit (633–721) | ~89 |

**任务**: T38~T42（5个任务）

### 6.2 F09: guardian_static.go（665行 → 4个文件）

| 新文件 | 内容 | 行数 |
|--------|------|------|
| `guardian_static.go` | StaticReviewResult + StaticGuardPort + Validate + review (31–151) | ~121 |
| `guardian_static_primitive.go` | checkPrimitiveCompliance + checkFieldType + checkCallExpr (152–299) | ~148 |
| `guardian_static_layer.go` | checkExternalDeps + checkLayerCompliance (301–465) | ~165 |
| `guardian_static_complexity.go` | checkComplexity + checkManifestConsistency + AsStep (467–665) | ~199 |

**任务**: T43~T46（4个任务）

### 6.3 F11: eventstore_upgrade.go（618行 → 5个文件）

| 新文件 | 内容 | 行数 |
|--------|------|------|
| `eventstore_sharded.go` | ShardedEventStore (28–158) | ~131 |
| `eventstore_snapshot.go` | AutoSnapshotTrigger (164–284) | ~121 |
| `eventstore_compaction.go` | CompactionPolicy + EventCompactor (286–374) | ~89 |
| `eventstore_worker.go` | EventBusWorkerPool (376–440) | ~65 |
| `eventstore_projection.go` | ProjectionCheckpoint + Store + WithCheckpoint (442–618) | ~177 |

**任务**: T47~T51（5个任务）

### 6.4 F12: guardian_dependency.go（614行 → 5个文件）

| 新文件 | 内容 | 行数 |
|--------|------|------|
| `guardian_dependency_graph.go` | DependencyGraph + DependencyEdge (33–248) | ~216 |
| `guardian_dependency_guard.go` | DependencyGuard + DependencyViolation (254–327) | ~74 |
| `guardian_snapshot_store.go` | SnapshotSummaryStore + InMemory + Diff (329–425) | ~97 |
| `guardian_architecture.go` | ArchitectureGuard + Input + Alert (427–527) | ~101 |
| `guardian_typeset.go` | typeSet + GlobalPrimitiveTypeSet (529–614) | ~86 |

**任务**: T52~T56（5个任务）

### 6.5 F13: cmd/arch-manager/ua.go（566行 → 3个文件）

| 新文件 | 内容 | 行数 |
|--------|------|------|
| `ua.go` | 10个类型定义 + loadUAGraph + generateUAGraphFromArch (14–258) | ~245 |
| `ua_validate.go` | handleUAValidate + validateUAGraph (296–454) | ~159 |
| `ua_search.go` | handleUASearch + searchUAGraph (456–566) | ~111 |

**任务**: T57~T59（3个任务）

### 6.6 F14: patterns_distributed.go（554行 → 4个文件）

| 新文件 | 内容 | 行数 |
|--------|------|------|
| `patterns_circuit_global.go` | GlobalCircuitBreaker (24–183) | ~160 |
| `patterns_degradation_federated.go` | FederatedDegradationManager (184–348) | ~165 |
| `patterns_rate_limiter_dist.go` | DistributedRateLimiter (350–459) | ~110 |
| `patterns_health_dist.go` | DistHealthChecker (461–554) | ~94 |

**任务**: T60~T63（4个任务）

### 6.7 F15: migrate/parser_go.go（543行 → 3个文件）

| 新文件 | 内容 | 行数 |
|--------|------|------|
| `parser_go.go` | GoParserBackend 接口 + Parse + ParseDir + convertAST + convertFuncDecl (15–168) | ~154 |
| `parser_go_body.go` | extractBodyNodes + stmtToBodyNode + extractCallGraph + extractTypeDecls (170–313) | ~144 |
| `parser_go_helpers.go` | callName + isIOCall + isBuiltin + isStdLib + 全部辅助函数 (314–543) | ~230 |

**任务**: T64~T66（3个任务）

### 6.8 F16: patterns_resilience_enhanced.go（542行 → 4个文件）

| 新文件 | 内容 | 行数 |
|--------|------|------|
| `patterns_token_bucket.go` | TokenBucketRateLimiter (21–119) | ~99 |
| `patterns_circuit_enhanced.go` | EnhancedCircuitBreaker (121–355) | ~235 |
| `patterns_backoff.go` | BackoffStrategy + Exponential + Linear (357–459) | ~103 |
| `patterns_retry.go` | RetryWithBackoff + IsRetryable + RateLimitMiddleware (461–542) | ~82 |

**任务**: T67~T70（4个任务）

### 6.9 F17: security.go（538行 → 4个文件）

| 新文件 | 内容 | 行数 |
|--------|------|------|
| `security_capability.go` | CapabilityToken + CapabilityPort (25–98) | ~74 |
| `security_access.go` | AccessControl + AccessPolicy (100–194) | ~95 |
| `security_audit.go` | AuditTrailAdapter + 工厂函数 (196–306) | ~111 |
| `security_merkle.go` | MerkleAuditChain + VerifyProof (308–538) | ~231 |

**任务**: T71~T74（4个任务）

### 6.10 F18: patterns_resilience.go（537行 → 5个文件）

| 新文件 | 内容 | 行数 |
|--------|------|------|
| `patterns_circuit.go` | CircuitBreaker (16–141) | ~126 |
| `patterns_fallback_bulkhead.go` | Fallback + Bulkhead (143–214) | ~72 |
| `patterns_rate_limiter.go` | RateLimiter (216–320) | ~105 |
| `patterns_rate_limiter_sharded.go` | ShardedRateLimiter (322–468) | ~147 |
| `patterns_resilience_chain.go` | ResilienceChain + Config (470–537) | ~68 |

**任务**: T75~T79（5个任务）

---

## 7. 执行路径

### 阶段 1: P0 前端拆分（T01~T09）

| 步骤 | 任务 | 并行 |
|------|------|------|
| 1.1 | T01(提取CSS) + T02(提取core.js) + T03(提取data.js) | 是 |
| 1.2 | T04(panorama) + T05(runtime) + T06(observation) + T07(control) + T08(migration) | 是 |
| 1.3 | T09(修改HTML入口) | 否（依赖 1.1+1.2） |
| 1.4 | 编译验证：启动 arch-manager，检查所有面板 | 否 |

### 阶段 2: P0 CLI 拆分（T10~T15）

| 步骤 | 任务 | 并行 |
|------|------|------|
| 2.1 | T11(helpers) + T12(init) + T13(templates) | 是 |
| 2.2 | T14(commands) + T15(migrate) | 是 |
| 2.3 | T10(重写入口) | 否（依赖 2.1+2.2） |
| 2.4 | 验证：所有 16 个命令 | 否 |

### 阶段 3: P1 Go 核心拆分（T16~T37）

| 步骤 | 任务 | 并行 |
|------|------|------|
| 3.1 | T16~T21(guardian_entropy 6文件) + T22~T25(composer 4文件) | 是 |
| 3.2 | T26~T28(aggregator 3文件) + T29~T32(handoff 4文件) + T33~T37(perf_core 5文件) | 是 |
| 3.3 | `go build ./go-core/` + `go test ./go-core/...` | 否 |

### 阶段 4: P2 Go 次优先级拆分（T38~T79）

| 步骤 | 任务 | 并行 |
|------|------|------|
| 4.1 | T38~T42(lint 5文件) + T43~T46(guardian_static 4文件) + T47~T51(eventstore 5文件) | 是 |
| 4.2 | T52~T56(guardian_dep 5文件) + T57~T59(ua 3文件) + T60~T63(distributed 4文件) | 是 |
| 4.3 | T64~T66(parser_go 3文件) + T67~T70(resilience_enhanced 4文件) + T71~T74(security 4文件) + T75~T79(resilience 5文件) | 是 |
| 4.4 | `go build ./...` + `go test ./...` | 否 |

---

## 8. 每个任务的通用验收检查清单

### Go 文件拆分任务

- [ ] 新文件创建成功，package 声明正确
- [ ] 原文件中对应代码已删除
- [ ] 原文件保留 package 声明和必要的 import
- [ ] `go build` 编译通过（无 import 错误）
- [ ] `go test ./...` 全部通过
- [ ] 新文件行数 ≤ 310 行
- [ ] 原文件行数 ≤ 310 行（如仍有残留代码）或已删除

### 前端文件拆分任务

- [ ] 新文件创建成功
- [ ] 原文件中对应代码已删除
- [ ] `<script src>` 引入顺序正确
- [ ] 浏览器控制台无 JS 错误
- [ ] 所有面板正常渲染
- [ ] SSE 连接正常
- [ ] 新文件行数 ≤ 310 行

### CLI 文件拆分任务

- [ ] 新文件创建成功
- [ ] dot-source 引入路径正确
- [ ] PowerShell 语法检查通过
- [ ] 所有命令可正常调用
- [ ] 新文件行数 ≤ 310 行

---

## 9. 文件变更汇总

### 新增文件（85 个）

| 分类 | 数量 |
|------|------|
| arch-manager CSS/JS | 9 |
| lec.ps1 模块 | 6 |
| go-core 拆分 | 56 |
| cmd/arch-manager 拆分 | 2 |
| cmd/lint 拆分 | 4 |
| migrate 拆分 | 2 |
| **总计** | **79** |

### 修改文件（18 个）

| 文件 | 操作 |
|------|------|
| arch-manager.html | 删除 CSS/JS，添加 link/script 引入 |
| lec.ps1 | 重写为入口点 |
| guardian_entropy.go | 删除（拆为 6 个文件） |
| composer.go | 保留接口+Pipeline+Branch，删除其余 |
| observation_aggregator.go | 保留 Aggregator，删除其余 |
| handoff.go | 删除（拆为 4 个文件） |
| perf_core.go | 保留常量+ShardedLock，删除其余 |
| cmd/lint/main.go | 保留 main+数据声明，删除规则函数 |
| guardian_static.go | 保留 StaticGuardPort+review，删除检查器 |
| eventstore_upgrade.go | 删除（拆为 5 个文件） |
| guardian_dependency.go | 删除（拆为 5 个文件） |
| ua.go | 保留类型+生成，删除 handler |
| patterns_distributed.go | 删除（拆为 4 个文件） |
| parser_go.go | 保留接口+Parse+convertAST，删除其余 |
| patterns_resilience_enhanced.go | 删除（拆为 4 个文件） |
| security.go | 删除（拆为 4 个文件） |
| patterns_resilience.go | 删除（拆为 5 个文件） |
