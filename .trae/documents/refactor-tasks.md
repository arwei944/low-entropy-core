# 架构管理器重构 — 最小任务单元开发文档

## 项目信息
- **分支**: `refactor-dashboard`
- **目标**: 将 `cmd/arch-manager/main.go` (3392行) 拆分为多个文件，每个函数 ≤ 50 行
- **约束**: 功能完备、零行为变更、严格遵循核心架构约束

---

## 任务总览

```
Phase A: 文件拆分（无行为变更）
  A1. 提取模型定义 → models.go
  A2. 提取 AST 解析器 → parser.go
  A3. 提取架构数据构建 → builder.go
  A4. 提取 Agent 管理 → agent.go
  A5. 提取健康评分 → health.go
  A6. 提取违规检测 → violations.go
  A7. 提取导出格式 → export.go
  A8. 提取版本管理 → version.go
  A9. 提取引导层 → guide.go
  A10. 提取 UA 知识图谱 → ua.go
  A11. 提取 SSE/监视 → sse.go
  A12. 提取模拟运行 → simulate.go
  A13. 精简 main.go → 仅保留 main() 和路由注册

Phase B: 新增仪表盘 API（计划中的 Phase 1）
  B1. 注册 Observation API 路由
  B2. 新增 Guardian 端点
  B3. 新增运行时统计端点
  B4. 新增四原语识别
  B5. 新增健康评分历史

Phase C: 前端重写
  C1. 重写 arch-manager.html
```

---

## Phase A: 文件拆分（零行为变更）

### A1. 创建 models.go — 数据模型

**输入**: main.go 第 35-188 行（数据模型 + 层级映射）
**输出**: `cmd/arch-manager/models.go`
**函数清单**:
- `type Symbol struct` — 已有，直接迁移
- `type FileInfo struct` — 已有，直接迁移
- `type ArchData struct` — 已有，直接迁移
- `type LayerStat struct` — 已有，直接迁移
- `type AgentStatus string` + const — 已有，直接迁移
- `type Agent struct` — 已有，直接迁移
- `type SubmissionResult struct` — 已有，直接迁移
- `type AgentEvent struct` — 已有，直接迁移
- `type LayerInfo struct` — 已有，直接迁移
- `var fileLayerMap` — 已有，直接迁移
- `func getLayerInfo(filename string) LayerInfo` — 已有，直接迁移
- `type HealthScore struct` — 从第 1040 行迁移
- `type Violation struct` — 从第 1174 行迁移
- `type GuideData struct` + 子类型 — 从第 2083 行迁移
- `type EnhancedArchData struct` — 从 handleAPI 中提取

**验证**: 编译通过，无新增/删除符号

---

### A2. 创建 parser.go — AST 解析器

**输入**: main.go 第 190-533 行
**输出**: `cmd/arch-manager/parser.go`
**函数清单**:
- `func parseFile(path string) (FileInfo, error)` — 已有，≤ 100行需拆分
  - 拆分为: `parseFile` → 调用 `extractImports`, `extractSymbols`, `resolveInternalDeps`
- `func parseTypeSpec(s *ast.TypeSpec, doc *ast.CommentGroup) Symbol` — 已有，≤ 60行可接受
- `func buildFuncSignature(d *ast.FuncDecl) string` — 已有，≤ 50行
- `func funcTypeToString(ft *ast.FuncType) string` — 已有，≤ 40行
- `func exprToString(expr ast.Expr) string` — 已有，≤ 45行
- `func resolveInternalDeps(imports []string) []string` — 已有，≤ 20行
- `func collectCalledFunctions(node ast.Node) []string` — 已有，≤ 30行

**新增函数** (拆分 parseFile):
- `func extractImports(f *ast.File) []string` — 提取 imports
- `func extractSymbols(f *ast.File) []Symbol` — 提取所有符号
- `func extractFileDeps(f *ast.File, symbolToFile map[string]string) []string` — 提取文件依赖

**验证**: `go build ./cmd/arch-manager` 通过

---

### A3. 创建 builder.go — 架构数据构建

**输入**: main.go 第 535-684 行
**输出**: `cmd/arch-manager/builder.go`
**函数清单**:
- `func buildArchData(dir string) (*ArchData, error)` — 已有，~145行需拆分
  - 拆分为: `buildArchData` → 调用 `scanFiles`, `buildSymbolIndex`, `resolveDeps`, `computeStats`
- `func diffArchData(old, new *ArchData) map[string]interface{}` — 已有，≤ 40行

**新增函数** (拆分 buildArchData):
- `func scanFiles(dir string) ([]FileInfo, error)` — 扫描目录，并发解析文件
- `func buildSymbolIndex(files []FileInfo) map[string]string` — 构建符号→文件索引
- `func resolveCrossFileDeps(files []FileInfo, symbolToFile map[string]string)` — 解析跨文件依赖
- `func computeLayerStats(files []FileInfo) []LayerStat` — 计算层级统计
- `func computeDependedBy(files []FileInfo) map[string][]string` — 计算被依赖关系

**验证**: `go test` (如有) 或手动验证 `/api/arch` 返回一致

---

### A4. 创建 agent.go — Agent 生命周期管理

**输入**: main.go 第 697-850 行
**输出**: `cmd/arch-manager/agent.go`
**函数清单**:
- `type AgentPool struct` — 已有，直接迁移
- `var agentPool` — 已有，直接迁移
- `func (p *AgentPool) init()` — 已有，≤ 5行
- `func (p *AgentPool) broadcast()` — 已有，≤ 15行
- `func (p *AgentPool) Register(agent *Agent)` — 已有，≤ 15行
- `func (p *AgentPool) Unregister(agentID string)` — 已有，≤ 10行
- `func (p *AgentPool) UpdateStatus(agentID string, status AgentStatus, currentTask string)` — 已有，≤ 20行
- `func (p *AgentPool) GetAgents() []Agent` — 已有，≤ 10行
- `func (p *AgentPool) GetAgent(agentID string) (*Agent, bool)` — 已有，≤ 10行
- `func (p *AgentPool) AddSubmission(result SubmissionResult)` — 已有，≤ 15行
- `func (p *AgentPool) GetSubmissions(agentID string) []SubmissionResult` — 已有，≤ 15行
- `func (p *AgentPool) Subscribe() chan AgentEvent` — 已有，≤ 10行
- `func (p *AgentPool) Unsubscribe(ch chan AgentEvent)` — 已有，≤ 10行

**验证**: Agent SSE 端点正常工作

---

### A5. 创建 health.go — 健康评分

**输入**: main.go 第 1023-1155 行
**输出**: `cmd/arch-manager/health.go`
**函数清单**:
- `func handleHealthScore(w http.ResponseWriter, r *http.Request)` — 已有，≤ 20行
- `func computeHealthScore(data *ArchData) HealthScore` — 已有，~107行需拆分
  - 拆分为: `computeHealthScore` → 调用 5 个独立评分函数

**新增函数**:
- `func scoreLayerBalance(data *ArchData) (float64, string)` — 层级平衡度评分
- `func scoreFileGranularity(data *ArchData) (float64, string)` — 文件粒度评分
- `func scoreSymbolDensity(data *ArchData) (float64, string)` — 符号密度评分
- `func scoreDependencyDepth(data *ArchData) (float64, string)` — 依赖深度评分
- `func scoreInterfaceRatio(data *ArchData) (float64, string)` — 接口率评分
- `func gradeFromScore(score float64) string` — 评分转等级

**验证**: `/api/health-score` 返回与重构前完全一致

---

### A6. 创建 violations.go — 违规检测

**输入**: main.go 第 1157-1316 行
**输出**: `cmd/arch-manager/violations.go`
**函数清单**:
- `func handleViolations(w http.ResponseWriter, r *http.Request)` — 已有，≤ 20行
- `func detectViolations(data *ArchData) []Violation` — 已有，~83行需拆分
  - 拆分为: `detectViolations` → 调用 4 个独立检测函数
- `func detectCycles(graph map[string]map[string]bool) [][]string` — 已有，≤ 50行

**新增函数**:
- `func detectLayerViolations(data *ArchData, layerOrder map[string]int) []Violation` — 层级反向依赖检测
- `func detectCyclicDeps(data *ArchData) []Violation` — 循环依赖检测
- `func detectLargeFiles(data *ArchData) []Violation` — 超大文件检测
- `func detectEmptyFiles(data *ArchData) []Violation` — 无符号文件检测

**验证**: `/api/violations` 返回与重构前完全一致

---

### A7. 创建 export.go — 导出格式

**输入**: main.go 第 1318-1542 行
**输出**: `cmd/arch-manager/export.go`
**函数清单**:
- `func handleExport(w http.ResponseWriter, r *http.Request)` — 已有，≤ 45行
- `func writePlantUML(w io.Writer, data *ArchData)` — 已有，~30行
- `func writeDOT(w io.Writer, data *ArchData)` — 已有，~35行
- `func toID(name string) string` — 已有，≤ 5行

**验证**: `/api/export?format=plantuml` 和 `format=dot` 输出一致

---

### A8. 创建 version.go — 版本管理

**输入**: main.go 第 1632-2057 行
**输出**: `cmd/arch-manager/version.go`
**函数清单**:
- `func handleVersion(w http.ResponseWriter, r *http.Request)` — 已有，≤ 25行
- `func handleVersionSnapshot(w http.ResponseWriter, r *http.Request)` — 已有，≤ 30行
- `func handleVersionDiff(w http.ResponseWriter, r *http.Request)` — 已有，≤ 25行
- `func handleVersionChangelog(w http.ResponseWriter, r *http.Request)` — 已有，≤ 25行
- `func getBuiltinChangelog(version string) []map[string]interface{}` — 已有，≤ 25行
- `func getBuiltinADRs() []core.ADR` — 已有，≤ 35行
- `func handleVersionCommitAnalyze(w http.ResponseWriter, r *http.Request)` — 已有，≤ 15行
- `func getBuiltinCommitAnalyze() map[string]interface{}` — 已有，≤ 25行
- `func handleVersionNextVersion(w http.ResponseWriter, r *http.Request)` — 已有，≤ 20行
- `func getBuiltinArchChanges() []core.ChangeIntent` — 已有，≤ 15行
- `func handleVersionArchChange(w http.ResponseWriter, r *http.Request)` — 已有，~70行需拆分
- `func handleVersionADR(w http.ResponseWriter, r *http.Request)` — 已有，~65行需拆分
- `func handleVersionRelease(w http.ResponseWriter, r *http.Request)` — 已有，~55行需拆分

**验证**: 所有版本端点正常工作

---

### A9. 创建 guide.go — 引导层

**输入**: main.go 第 2079-2300+ 行
**输出**: `cmd/arch-manager/guide.go`
**函数清单**:
- `func handleGuide(w http.ResponseWriter, r *http.Request)` — 已有，~100行需拆分
  - 拆分为: `handleGuide` → 调用 `buildGuideData`
- `func loadTourGuide() *TourGuide` — 已有，≤ 50行
- `func buildConstraintChecks(data *ArchData) []ConstraintCheck` — 需要查找定义

**新增函数**:
- `func buildGuideData(data *ArchData) GuideData` — 构建引导层数据
- `func buildPrimitives() []PrimitiveDef` — 构建原语定义
- `func buildLayerEdges() []LayerDepEdge` — 构建层级依赖边
- `func buildPatterns() []PatternDef` — 构建模式定义

**验证**: `/api/guide` 返回与重构前一致

---

### A10. 创建 ua.go — UA 知识图谱

**输入**: 需要查找 handleUAGraph, handleUAValidate, handleUASearch 的定义位置
**输出**: `cmd/arch-manager/ua.go`
**函数清单**: 待查找

**验证**: `/api/ua/graph`, `/api/ua/validate`, `/api/ua/search` 正常工作

---

### A11. 创建 sse.go — SSE 和文件监视

**输入**: main.go 第 1544-1630 行
**输出**: `cmd/arch-manager/sse.go`
**函数清单**:
- `func handleSSE(w http.ResponseWriter, r *http.Request)` — 已有，≤ 40行
- `func watchFiles(dir string, interval time.Duration)` — 已有，~45行

**验证**: SSE 连接正常推送，文件变更检测正常

---

### A12. 创建 simulate.go — 代码模拟运行

**输入**: 需要查找 handleSimulate, handleEntropyCheck, handleObserveCheck 的定义位置
**输出**: `cmd/arch-manager/simulate.go`
**函数清单**: 待查找

**验证**: `/api/simulate`, `/api/entropy`, `/api/observe` 正常工作

---

### A13. 精简 main.go

**输入**: 剩余代码
**输出**: `cmd/arch-manager/main.go` (目标 ≤ 200 行)
**保留内容**:
- package main
- imports
- `var archData, archMu, sourceDir, enableWatch`
- `func main()` — 仅参数解析 + 初始化 + 路由注册 + 启动服务器
- `func setCORS(w http.ResponseWriter)` — 辅助函数（如需要）

**删除/迁移内容**: 所有 handler 函数、所有模型定义、所有辅助函数

**验证**: `go build ./cmd/arch-manager` 通过，服务启动正常

---

## Phase B: 新增仪表盘 API

### B1. 注册 Observation API 路由

**文件**: `cmd/arch-manager/observation.go` (新建)
**任务**:
1. 创建 `ObservationPipeline` 实例
2. 创建 `ArchitectureRegistry` 实例
3. 创建 `ObservationAPI` 实例
4. 调用 `api.RegisterHandlers(mux)`
5. 启动 Pipeline

**依赖**: build tag `lecore_tier4+`
**验证**: `/api/observation/steps` 返回空数组（无数据时）

---

### B2. 新增 Guardian 端点

**文件**: `cmd/arch-manager/guardian.go` (新建)
**端点清单**:
- `GET /api/guardian/snapshot`
- `GET /api/guardian/sse`
- `GET /api/guardian/thresholds`
- `PUT /api/guardian/thresholds`
- `GET /api/guardian/drift`
- `GET /api/guardian/history`

**验证**: 每个端点返回正确结构

---

### B3. 新增运行时统计端点

**文件**: `cmd/arch-manager/runtime.go` (新建)
**端点清单**:
- `GET /api/runtime/tps`
- `GET /api/runtime/errors`
- `GET /api/runtime/latency`
- `GET /api/runtime/sampling-rate`
- `PUT /api/runtime/sampling-rate`

**验证**: 采样率修改后 ObservationPipeline 行为改变

---

### B4. 新增四原语识别

**文件**: 修改 `parser.go`
**任务**:
1. 在 AST 解析中识别 `var _ Atom[...] = ...` 等接口断言
2. 新增 `GET /api/primitives` 端点

**验证**: 返回币安交易所等项目的四原语列表

---

### B5. 新增健康评分历史

**文件**: 修改 `health.go`
**任务**:
1. 添加内存环形缓冲区（最近 100 条）
2. 新增 `GET /api/health-score/history`

**验证**: 多次调用 `/api/health-score` 后历史端点有数据

---

## Phase C: 前端重写

### C1. 重写 arch-manager.html

**文件**: `arch-manager.html`
**布局**: 三栏式（左导航 | 主视图 | 右详情）
**视图清单**:
- 架构全景: 拓扑图、健康仪表、违规看板、原语分布、层级矩阵
- 动态运行: 心跳、熵值热力图、错误血压计、Trace图、数据流
- 控制面板: 采样率、阈值、What-If
- 溯源面板: 因果链、时间旅行、变更归因

**验证**: 所有视图正常渲染，SSE 实时更新

---

## 任务执行顺序

```
A1 → A2 → A3 → A4 → A5 → A6 → A7 → A8 → A9 → A10 → A11 → A12 → A13
（每个任务完成后编译验证，确保零行为变更）

→ B1 → B2 → B3 → B4 → B5
（每个端点完成后 curl 验证）

→ C1
（启动服务后浏览器验证）
```

---

## 验证矩阵

| 端点 | 重构前 | 重构后 | 新增 |
|------|--------|--------|------|
| /api/arch | ✓ | ✓ | |
| /api/file | ✓ | ✓ | |
| /api/refresh | ✓ | ✓ | |
| /api/health | ✓ | ✓ | |
| /api/health-score | ✓ | ✓ | |
| /api/health-score/history | | | ✓ |
| /api/violations | ✓ | ✓ | |
| /api/export | ✓ | ✓ | |
| /api/sse | ✓ | ✓ | |
| /api/version/* | ✓ | ✓ | |
| /api/guide | ✓ | ✓ | |
| /api/ua/* | ✓ | ✓ | |
| /api/simulate | ✓ | ✓ | |
| /api/entropy | ✓ | ✓ | |
| /api/observe | ✓ | ✓ | |
| /api/agents/* | ✓ | ✓ | |
| /api/observation/* | | | ✓ |
| /api/guardian/* | | | ✓ |
| /api/runtime/* | | | ✓ |
| /api/primitives | | | ✓ |
