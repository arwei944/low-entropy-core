# 迁移面板 + 架构变动日志引擎 — 零漂移开发文档

> **版本**: v1.0 | **日期**: 2026-06-21 | **约束**: C1~C4 四大核心约束
> **零漂移保证**: 每个任务包含精确文件路径、类型签名、代码模板、测试用例、验收检查清单

---

## 目录

- [1. 需求总览](#1-需求总览)
- [2. 现状分析](#2-现状分析)
- [3. 目标架构](#3-目标架构)
- [4. R1: 迁移面板（10个任务）](#4-r1-迁移面板)
  - [R1-T01: 迁移引擎 REST API](#r1-t01-迁移引擎-rest-api)
  - [R1-T02: 迁移引擎 SSE 事件流](#r1-t02-迁移引擎-sse-事件流)
  - [R1-T03: 路由注册到 main.go](#r1-t03-路由注册到-main.go)
  - [R1-T04: 前端侧边栏新增迁移分组](#r1-t04-前端侧边栏新增迁移分组)
  - [R1-T05: 引擎状态面板](#r1-t05-引擎状态面板)
  - [R1-T06: 模式分类面板](#r1-t06-模式分类面板)
  - [R1-T07: 约束门链面板](#r1-t07-约束门链面板)
  - [R1-T08: 迁移日志面板](#r1-t08-迁移日志面板)
  - [R1-T09: 会话历史面板](#r1-t09-会话历史面板)
  - [R1-T10: fetchAll 集成](#r1-t10-fetchall-集成)
- [5. R2: 架构变动日志引擎（5个任务）](#5-r2-架构变动日志引擎)
  - [R2-T01: 变动日志持久化存储](#r2-t01-变动日志持久化存储)
  - [R2-T02: 集成到 watchFiles](#r2-t02-集成到-watchfiles)
  - [R2-T03: 变动日志 REST API + SSE](#r2-t03-变动日志-rest-api--sse)
  - [R2-T04: 前端侧边栏新增面板项](#r2-t04-前端侧边栏新增面板项)
  - [R2-T05: 架构变动日志面板](#r2-t05-架构变动日志面板)
- [6. 任务依赖与执行路径](#6-任务依赖与执行路径)
- [7. 文件变更汇总](#7-文件变更汇总)

---

## 1. 需求总览

| 编号 | 需求 | 涉及层 | 新增文件 | 修改文件 |
|------|------|--------|---------|---------|
| R1 | 架构管理器增加迁移面板 | 后端 2 + 前端 1 | `migrate_api.go`, `migrate_sse.go` | `main.go`, `arch-manager.html` |
| R2 | 可观察层增加日志引擎 | 后端 2 + 前端 1 | `arch_changelog.go`, `arch_changelog_api.go` | `sse.go`, `main.go`, `arch-manager.html` |

---

## 2. 现状分析

### 2.1 前端现状（arch-manager.html）

- 纯 HTML + 原生 JS + ECharts 5.4.3，单文件暗色主题
- 侧边栏 5 个分组 22 个面板：架构全景(6)、动态运行(5)、可观测性(5)、控制面板(3)、溯源面板(3)
- 面板切换：`switchView(view)` → `renderCurrentView()` → `renderXxxView(container)`
- SSE 连接 3 条：`/api/sse`（3s 推送架构数据）、`/api/sse/dev-events`（开发事件）、`/api/guardian/sse`
- REST：`api(url)` 封装 fetch，`fetchAll()` 并行加载 21 个端点

### 2.2 后端现状（cmd/arch-manager/）

- `package main`，19 个 Go 文件
- `main.go`：路由注册 + `handleAPI`/`handleFile`/`handleRefresh`
- `sse.go`：`devEventBus`（subscribe/unsubscribe/publish）、`handleSSE`、`handleDevSSE`、`watchFiles`
- `models.go`：`ArchData`/`FileInfo`/`Symbol`/`Violation` 等全部数据模型
- 迁移引擎 `go-core/migrate/`（16 个文件）**无 HTTP API**

### 2.3 迁移引擎现状（go-core/migrate/）

- `parser_backend.go`：`ParserBackend` 接口 + `RegisterParser`/`GetParser`
- `parser_go.go`：`GoParserBackend`（`Parse`/`ParseDir`）
- `pattern.go`：`CodePattern` 枚举 + `PatternClassifier` + `ClassifyFunctions`
- `constraint_gate.go`：`GateChain` + `DefaultGateChain()`（G1~G6）
- `log_entry.go`：`MigrationLogEntry`（16 字段）+ `MigrationLog`
- `log_store.go`：`FileLogStore`（JSON Lines 持久化）
- `validate_enhanced.go`：`EnhancedValidator` + `ValidationReport`

### 2.4 可观察层现状（go-core/）

- `observation.go`：`ExecutionStep`/`TraceTree`/`ObservationAdapter`
- `observation_api.go`：`ObservationAPI.RegisterHandlers(mux)` — 9 个端点
- `observability.go`：`Span`/`Tracer`/`Meter`/`Logger`

---

## 3. 目标架构

```
arch-manager.html (前端)
  ├── 侧边栏
  │   ├── 架构全景 (6 面板)     [现有]
  │   ├── 动态运行 (5 面板)     [现有]
  │   ├── 可观测性 (6 面板)     [现有5 + 新增1: 架构变动日志]
  │   ├── 控制面板 (3 面板)     [现有]
  │   ├── 溯源面板 (3 面板)     [现有]
  │   └── 迁移引擎 (5 面板)     [新增: 引擎状态/模式分类/约束门链/迁移日志/会话历史]
  │
  └── SSE 连接
      ├── /api/sse               [现有]
      ├── /api/sse/dev-events    [现有]
      ├── /api/guardian/sse      [现有]
      ├── /api/sse/migrate       [新增: 迁移引擎事件]
      └── /api/sse/arch-changelog [新增: 架构变动事件]

cmd/arch-manager/ (后端)
  ├── main.go                   [修改: +15 行路由注册]
  ├── sse.go                    [修改: +30 行 changelog 记录]
  ├── migrate_api.go            [新建: 7 个 REST API]
  ├── migrate_sse.go            [新建: SSE 事件流]
  ├── arch_changelog.go         [新建: 变动日志存储]
  └── arch_changelog_api.go     [新建: 3 个 REST API + SSE]
```

---

## 4. R1: 迁移面板

### R1-T01: 迁移引擎 REST API

**文件**: `cmd/arch-manager/migrate_api.go` (新建)
**Package**: `main`
**依赖**: `go-core/migrate`

#### 精确类型签名

```go
package main

import (
    "encoding/json"
    "fmt"
    "net/http"
    "strconv"
    "sync"
    "time"

    "low-entropy-core/go-core/migrate"
)

// migrateState 迁移引擎全局状态
type migrateState struct {
    mu            sync.RWMutex
    sessions      []migrateSessionInfo
    activeSession *migrateSessionInfo
    logStore      *migrate.FileLogStore
}

// migrateSessionInfo 迁移会话摘要
type migrateSessionInfo struct {
    SessionID    string            `json:"session_id"`
    StartedAt    time.Time         `json:"started_at"`
    Status       string            `json:"status"`
    FileCount    int               `json:"file_count"`
    EntryCount   int               `json:"entry_count"`
    Language     string            `json:"language"`
    TargetTier   string            `json:"target_tier"`
    PatternStats map[string]int    `json:"pattern_stats"`
}

var migState *migrateState

func initMigrateState(baseDir string) {
    migState = &migrateState{
        logStore: migrate.NewFileLogStore(baseDir),
    }
}
```

#### API 端点清单（7 个）

| 端点 | 方法 | 功能 | 请求体/参数 |
|------|------|------|------------|
| `/api/migrate/analyze` | POST | 分析目录，返回 PatternMap | `{"dir":"./demo-project","language":"go"}` |
| `/api/migrate/validate` | POST | 运行约束门链 + 增强验证 | `{"dir":"./demo-project","language":"go"}` |
| `/api/migrate/sessions` | GET | 列出所有迁移会话 | - |
| `/api/migrate/sessions/{id}` | GET | 获取单个会话详情 | URL path |
| `/api/migrate/logs` | GET | 查询迁移日志 | `?phase=&action=&file=&limit=100` |
| `/api/migrate/logs/export` | GET | 导出日志 | `?format=json\|md` |
| `/api/migrate/status` | GET | 引擎全局状态摘要 | - |

#### Handler 模板（analyze 为例）

```go
func handleMigrateAnalyze(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
        return
    }
    var req struct {
        Dir      string `json:"dir"`
        Language string `json:"language"`
    }
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "invalid request body", http.StatusBadRequest)
        return
    }
    if req.Dir == "" {
        http.Error(w, "missing dir parameter", http.StatusBadRequest)
        return
    }

    // 语言检测
    if req.Language == "" || req.Language == "auto" {
        detector := migrate.NewLanguageDetector()
        req.Language = detector.Detect(req.Dir)
    }

    backend, err := migrate.GetParser(req.Language)
    if err != nil {
        http.Error(w, "unsupported language: "+req.Language, http.StatusBadRequest)
        return
    }

    files, err := backend.ParseDir(req.Dir)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    var allFuncs []migrate.UnifiedFunction
    for _, f := range files {
        allFuncs = append(allFuncs, f.Functions...)
    }

    classifiers := []migrate.PatternClassifier{
        &migrate.AtomClassifier{},
        &migrate.PortClassifier{},
        &migrate.AdapterClassifier{},
        &migrate.ComposerClassifier{},
    }
    patternMap := migrate.ClassifyFunctions(allFuncs, classifiers)

    sessionID := fmt.Sprintf("mig-%d", time.Now().UnixNano())
    session := migrateSessionInfo{
        SessionID:    sessionID,
        StartedAt:    time.Now(),
        Status:       "completed",
        FileCount:    len(files),
        Language:     req.Language,
        PatternStats: patternMap.Stats(),
    }

    migState.mu.Lock()
    migState.sessions = append(migState.sessions, session)
    migState.mu.Unlock()

    // 推送 SSE 事件
    migEventBus.publish(MigrateEvent{
        Type: "analyze_done", Timestamp: time.Now().Format(time.RFC3339),
        SessionID: sessionID, Message: fmt.Sprintf("分析完成: %d 文件", len(files)),
        Data: map[string]interface{}{"pattern_map": patternMap, "stats": patternMap.Stats()},
    })

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]interface{}{
        "session_id":  sessionID,
        "language":    req.Language,
        "file_count":  len(files),
        "pattern_map": patternMap,
        "stats":       patternMap.Stats(),
    })
}
```

#### 测试用例

```go
// cmd/arch-manager/migrate_api_test.go
func TestHandleMigrateAnalyze(t *testing.T) {
    // POST /api/migrate/analyze {"dir":"c:\\temp\\go-parser-test"}
    // 断言: status 200, pattern_map 非 nil, session_id 非空
    // GET /api/migrate/sessions 断言: 包含刚创建的 session
}

func TestHandleMigrateValidate(t *testing.T) {
    // 先 analyze，再 validate
    // 断言: 返回 GateDecision + ValidationReport
}

func TestHandleMigrateLogs(t *testing.T) {
    // GET /api/migrate/logs?phase=parse
    // 断言: 返回条目 phase 全为 "parse"
}

func TestHandleMigrateLogsExport(t *testing.T) {
    // GET /api/migrate/logs/export?format=json → 合法 JSON
    // GET /api/migrate/logs/export?format=md → Markdown 表格
}
```

#### 验收检查清单

- [ ] `migrate_api.go` 编译通过，无 import 错误
- [ ] 7 个端点全部可访问，返回正确 Content-Type
- [ ] `POST /api/migrate/analyze` 对 `./demo-project` 返回 PatternMap
- [ ] `GET /api/migrate/sessions` 返回历史会话列表
- [ ] `GET /api/migrate/logs?phase=transform` 正确过滤
- [ ] `GET /api/migrate/logs/export?format=md` 返回 Markdown 表格
- [ ] 非法输入返回 400，不支持方法返回 405

---

### R1-T02: 迁移引擎 SSE 事件流

**文件**: `cmd/arch-manager/migrate_sse.go` (新建)
**Package**: `main`

#### 精确类型签名

```go
// MigrateEvent 迁移引擎 SSE 事件
type MigrateEvent struct {
    Type      string      `json:"type"`       // analyze_start/analyze_progress/analyze_done/validate_start/validate_done/log_append/session_sealed
    Timestamp string      `json:"timestamp"`
    SessionID string      `json:"session_id,omitempty"`
    Message   string      `json:"message,omitempty"`
    Data      interface{} `json:"data,omitempty"`
}

// migrateEventBus 迁移引擎事件广播总线
type migrateEventBus struct {
    mu          sync.RWMutex
    subscribers map[chan MigrateEvent]bool
}

var migEventBus = &migrateEventBus{
    subscribers: make(map[chan MigrateEvent]bool),
}

func (b *migrateEventBus) subscribe() chan MigrateEvent
func (b *migrateEventBus) unsubscribe(ch chan MigrateEvent)
func (b *migrateEventBus) publish(evt MigrateEvent)
```

#### SSE Handler

```go
// GET /api/sse/migrate — 迁移引擎实时事件流
func handleMigrateSSE(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "text/event-stream")
    w.Header().Set("Cache-Control", "no-cache")
    w.Header().Set("Connection", "keep-alive")
    w.Header().Set("Access-Control-Allow-Origin", "*")

    flusher, ok := w.(http.Flusher)
    if !ok {
        http.Error(w, "streaming not supported", http.StatusInternalServerError)
        return
    }

    ch := migEventBus.subscribe()
    defer migEventBus.unsubscribe(ch)

    data, _ := json.Marshal(MigrateEvent{
        Type: "connected", Timestamp: time.Now().Format(time.RFC3339),
        Message: "迁移引擎事件流已连接",
    })
    fmt.Fprintf(w, "data: %s\n\n", data)
    flusher.Flush()

    for {
        select {
        case <-r.Context().Done():
            return
        case evt, ok := <-ch:
            if !ok { return }
            data, err := json.Marshal(evt)
            if err != nil { continue }
            fmt.Fprintf(w, "data: %s\n\n", data)
            flusher.Flush()
        }
    }
}
```

#### 验收检查清单

- [ ] `migrate_sse.go` 编译通过
- [ ] SSE 连接建立后立即收到 `connected` 事件
- [ ] `analyze_done` 事件携带 PatternMap 数据
- [ ] `validate_done` 事件携带 ValidationReport 数据
- [ ] 客户端断开后服务端正确清理资源

---

### R1-T03: 路由注册到 main.go

**文件**: `cmd/arch-manager/main.go` (修改)
**修改位置**: `startServer()` 路由注册区，Observation 路由之后、Guardian 路由之前

#### 精确插入代码

```go
// 迁移引擎 API (v0.11.0)
initMigrateState(filepath.Join(sourceDir, ".migration-logs"))
mux.HandleFunc("/api/migrate/analyze", handleMigrateAnalyze)
mux.HandleFunc("/api/migrate/validate", handleMigrateValidate)
mux.HandleFunc("/api/migrate/sessions", handleMigrateSessions)
mux.HandleFunc("/api/migrate/sessions/", handleMigrateSessionDetail)
mux.HandleFunc("/api/migrate/logs", handleMigrateLogs)
mux.HandleFunc("/api/migrate/logs/export", handleMigrateLogsExport)
mux.HandleFunc("/api/migrate/status", handleMigrateStatus)
mux.HandleFunc("/api/sse/migrate", handleMigrateSSE)
```

#### 验收检查清单

- [ ] 编译通过
- [ ] 8 个路由全部可访问
- [ ] 不影响现有路由

---

### R1-T04: 前端侧边栏新增迁移分组

**文件**: `arch-manager.html` (修改)
**修改位置**: `</aside>` 之前，"溯源面板"分组之后（约第 802 行）

#### HTML 模板

```html
<!-- 迁移引擎 -->
<div class="sidebar-section">
  <div class="sidebar-section-title" onclick="toggleSection('migration')">
    <svg class="chevron open" id="chev-migration" width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"><polyline points="9 18 15 12 9 6"/></svg>
    <span>迁移引擎</span>
  </div>
  <div class="sidebar-section-files" id="sec-migration" style="max-height:400px">
    <div class="sidebar-item" onclick="switchView('migStatus')" data-view="migStatus">
      <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"/><polyline points="12 6 12 12 16 14"/></svg>
      引擎状态
    </div>
    <div class="sidebar-item" onclick="switchView('migPatternMap')" data-view="migPatternMap">
      <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polygon points="12 2 2 7 12 12 22 7 12 2"/><polyline points="2 17 12 22 22 17"/><polyline points="2 12 12 17 22 12"/></svg>
      模式分类
    </div>
    <div class="sidebar-item" onclick="switchView('migGateChain')" data-view="migGateChain">
      <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="3" y="3" width="18" height="18" rx="2"/><line x1="3" y1="9" x2="21" y2="9"/><line x1="9" y1="21" x2="9" y2="9"/></svg>
      约束门链
    </div>
    <div class="sidebar-item" onclick="switchView('migLog')" data-view="migLog">
      <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><polyline points="14 2 14 8 20 8"/><line x1="16" y1="13" x2="8" y2="13"/><line x1="16" y1="17" x2="8" y2="17"/></svg>
      迁移日志
    </div>
    <div class="sidebar-item" onclick="switchView('migSessions')" data-view="migSessions">
      <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M22 11.08V12a10 10 0 1 1-5.93-9.14"/><polyline points="22 4 12 14.01 9 11.01"/></svg>
      会话历史
    </div>
  </div>
</div>
```

#### 同步修改清单

1. **全局变量区**新增:
```javascript
let migStatus = null, migPatternMap = null, migGateChain = null, migLogs = [], migSessions = [];
let sseMigrate = null;
```

2. **`collapsed` 对象**新增 `migration: false`

3. **`renderCurrentView()` switch** 新增 5 个 case:
```javascript
case 'migStatus': renderMigStatus(container); break;
case 'migPatternMap': renderMigPatternMap(container); break;
case 'migGateChain': renderMigGateChain(container); break;
case 'migLog': renderMigLog(container); break;
case 'migSessions': renderMigSessions(container); break;
```

4. **`connectSSE()`** 新增迁移 SSE 连接（在现有 SSE 之后）:
```javascript
if (sseMigrate) sseMigrate.close();
try {
    sseMigrate = new EventSource('/api/sse/migrate');
    sseMigrate.onmessage = (ev) => {
      try {
        const e = JSON.parse(ev.data);
        if (e.type === 'analyze_done' && e.data) migPatternMap = e.data.pattern_map;
        if (e.type === 'validate_done' && e.data) migGateChain = e.data;
        if (e.type === 'log_append' && e.data) migLogs.unshift(e.data);
        if (migLogs.length > 500) migLogs.length = 500;
        if (currentView === 'migStatus' || currentView === 'migLog') renderCurrentView();
      } catch (ex) {}
    };
    sseMigrate.onerror = () => { if (sseMigrate) { sseMigrate.close(); sseMigrate = null; } };
} catch (e) {}
```

#### 验收检查清单

- [ ] 侧边栏显示"迁移引擎"分组，含 5 个面板项
- [ ] 点击每个面板项可切换视图
- [ ] 分组可折叠/展开
- [ ] 不影响现有 5 个分组

---

### R1-T05: 引擎状态面板

**文件**: `arch-manager.html` (修改)
**函数**: `function renderMigStatus(container)`

#### 面板内容

- 4 个 stat-card: 活跃会话数、总日志条目、历史会话、引擎状态
- 操作栏: 目录输入框 + "分析目录"按钮 + "运行验证"按钮
- 最近事件列表（SSE 实时推送）

#### 辅助函数

```javascript
async function triggerMigrateAnalyze() {
    const dir = document.getElementById('migDirInput').value;
    try {
        const result = await api('/api/migrate/analyze', {
            method: 'POST', headers: {'Content-Type': 'application/json'},
            body: JSON.stringify({ dir: dir })
        });
        migPatternMap = result.pattern_map;
        toast('分析完成: ' + result.file_count + ' 文件', 'ok');
        renderCurrentView();
    } catch (e) { toast('分析失败: ' + e.message, 'err'); }
}

async function triggerMigrateValidate() {
    const dir = document.getElementById('migDirInput').value;
    try {
        const result = await api('/api/migrate/validate', {
            method: 'POST', headers: {'Content-Type': 'application/json'},
            body: JSON.stringify({ dir: dir })
        });
        migGateChain = result;
        toast('验证完成: ' + (result.pass ? '全部通过' : '存在阻断'), result.pass ? 'ok' : 'err');
        renderCurrentView();
    } catch (e) { toast('验证失败: ' + e.message, 'err'); }
}
```

#### 验收检查清单

- [ ] 面板显示 4 个统计卡片
- [ ] "分析目录"按钮触发 POST 并更新视图
- [ ] "运行验证"按钮触发 POST 并更新视图
- [ ] SSE 实时推送事件自动更新面板

---

### R1-T06: 模式分类面板

**文件**: `arch-manager.html` (修改)
**函数**: `function renderMigPatternMap(container)`

#### 面板内容

- ECharts 饼图: Atom/Port/Adapter/Composer/Unknown 五色分布
- ECharts 柱状图: 各模式置信度分布
- 表格: 每个函数的函数名、文件、模式、置信度

#### 验收检查清单

- [ ] 饼图正确显示 5 种模式分布
- [ ] 柱状图显示置信度分布
- [ ] 表格显示所有函数分类详情
- [ ] 无数据时显示空状态提示
- [ ] ECharts 实例在视图切换时正确销毁

---

### R1-T07: 约束门链面板

**文件**: `arch-manager.html` (修改)
**函数**: `function renderMigGateChain(container)`

#### 面板内容

- 流水线图: G1→G2→G3→G4→G5→G6，Pass 绿/Fail 红
- Issues 表格: Severity + Message

#### 验收检查清单

- [ ] 6 个门节点以流水线形式展示
- [ ] Pass/Fail 正确着色
- [ ] Issues 按严重度着色
- [ ] 全部通过时显示整体 PASS

---

### R1-T08: 迁移日志面板

**文件**: `arch-manager.html` (修改)
**函数**: `function renderMigLog(container)`

#### 面板内容

- 查询栏: Phase 下拉过滤 + 查询按钮 + 导出 JSON/MD 按钮
- 日志表格: SeqNo、Phase（着色）、Action、File、Line
- SSE 实时追加

#### 辅助函数

```javascript
async function fetchMigLogs() {
    const phase = document.getElementById('migLogPhase').value;
    let url = '/api/migrate/logs';
    if (phase) url += '?phase=' + encodeURIComponent(phase);
    try { migLogs = await api(url); toast('查询完成: ' + migLogs.length + ' 条', 'ok'); renderCurrentView(); }
    catch(e) { toast('查询失败: ' + e.message, 'err'); }
}

async function exportMigLogs(format) {
    try {
        const blob = await fetch('/api/migrate/logs/export?format=' + format).then(r => r.blob());
        const a = document.createElement('a');
        a.href = URL.createObjectURL(blob);
        a.download = 'migration-log.' + (format === 'json' ? 'json' : 'md');
        a.click();
        toast('导出成功', 'ok');
    } catch(e) { toast('导出失败: ' + e.message, 'err'); }
}
```

#### 验收检查清单

- [ ] 日志表格正确显示 SeqNo/Phase/Action/File/Line
- [ ] Phase 过滤功能正常
- [ ] 导出 JSON 下载合法文件
- [ ] 导出 MD 下载合法 Markdown 表格
- [ ] SSE log_append 事件自动追加

---

### R1-T09: 会话历史面板

**文件**: `arch-manager.html` (修改)
**函数**: `function renderMigSessions(container)`

#### 面板内容

- 会话表格: SessionID、语言、文件数、状态（着色）、开始时间

#### 验收检查清单

- [ ] 表格正确显示所有会话
- [ ] 状态按颜色区分（completed=绿, failed=红, running=橙）
- [ ] 空状态显示友好提示

---

### R1-T10: fetchAll 集成

**文件**: `arch-manager.html` (修改)
**修改位置**: `fetchAll()` 函数，`Promise.all` 数组追加 3 个调用

#### 精确插入代码

```javascript
// 在 Promise.all 的解构数组末尾追加:
const [..., mstatus, msessions, mlogs] = await Promise.all([
    // ... 现有调用 ...
    api('/api/migrate/status').catch(() => null),
    api('/api/migrate/sessions').catch(() => []),
    api('/api/migrate/logs?limit=100').catch(() => []),
]);
migStatus = mstatus;
migSessions = Array.isArray(msessions) ? msessions : [];
migLogs = Array.isArray(mlogs) ? mlogs : [];
```

#### 验收检查清单

- [ ] 页面初始化时自动加载迁移数据
- [ ] 迁移 API 不可用时页面不报错
- [ ] 不影响现有 fetchAll 加载速度

---

## 5. R2: 架构变动日志引擎

### R2-T01: 变动日志持久化存储

**文件**: `cmd/arch-manager/arch_changelog.go` (新建)
**Package**: `main`

#### 精确类型签名

```go
// ArchChangeEntry 架构变动日志条目（不可变）
type ArchChangeEntry struct {
    ID        string    `json:"id"`
    SeqNo     int64     `json:"seq_no"`
    Timestamp time.Time `json:"timestamp"`
    Category  string    `json:"category"`  // file_add|file_modify|file_delete|symbol_add|symbol_remove|violation_add|violation_resolve|health_change|layer_change
    Severity  string    `json:"severity"`  // info|warning|critical
    File      string    `json:"file,omitempty"`
    Detail    string    `json:"detail"`
    Before    string    `json:"before,omitempty"`
    After     string    `json:"after,omitempty"`
    Source    string    `json:"source"`    // watch|manual_refresh|guardian|migration
}

// ArchChangeFilter 查询过滤条件
type ArchChangeFilter struct {
    Category string    `json:"category"`
    Severity string    `json:"severity"`
    File     string    `json:"file"`
    Source   string    `json:"source"`
    Since    time.Time `json:"since"`
    Limit    int       `json:"limit"`
    Offset   int       `json:"offset"`
}

// ArchChangelogStore 架构变动日志存储
type ArchChangelogStore struct {
    mu      sync.RWMutex
    baseDir string
    seqNo   int64
    entries []ArchChangeEntry
    maxMem  int
}

func NewArchChangelogStore(baseDir string) *ArchChangelogStore
func (s *ArchChangelogStore) Append(entry ArchChangeEntry) error
func (s *ArchChangelogStore) Query(filter ArchChangeFilter) []ArchChangeEntry
func (s *ArchChangelogStore) Stats() map[string]interface{}
```

#### 持久化格式

- 按日期分片的 JSON Lines 文件: `.arch-changelog/2026-06-21.jsonl`
- 每行一个 JSON 对象（ArchChangeEntry）
- 内存缓存最近 1000 条

#### 测试用例

```go
func TestArchChangelogStore_Append(t *testing.T) {
    // Append → 断言 SeqNo=1, ID 非空, 磁盘文件存在
}

func TestArchChangelogStore_Query(t *testing.T) {
    // 追加 3 条不同 category → Query({Category:"file_add"}) → 只返回 1 条
}

func TestArchChangelogStore_Stats(t *testing.T) {
    // Stats() 返回 total + by_category + by_severity
}
```

#### 验收检查清单

- [ ] `arch_changelog.go` 编译通过
- [ ] Append 后 SeqNo 自增不可重复
- [ ] 持久化文件按日期分片
- [ ] Query 按 category/severity/file/source 正确过滤
- [ ] 内存缓存不超过 maxMem

---

### R2-T02: 集成到 watchFiles

**文件**: `cmd/arch-manager/sse.go` (修改)
**修改位置**: `watchFiles()` 函数内

#### 精确插入位置

在 `eventBus.publish(DevEvent{Type:"file_changed"...})` 之后追加 changelog 记录：

```go
// 文件新增
changelogStore.Append(ArchChangeEntry{
    Category: "file_add", Severity: "info", File: f,
    Detail: fmt.Sprintf("新文件创建: %s", f), Source: "watch",
})

// 文件修改
changelogStore.Append(ArchChangeEntry{
    Category: "file_modify", Severity: "info", File: f,
    Detail: fmt.Sprintf("文件修改: %s", f), Source: "watch",
})

// 文件删除
changelogStore.Append(ArchChangeEntry{
    Category: "file_delete", Severity: "warning", File: f,
    Detail: fmt.Sprintf("文件删除: %s", f), Source: "watch",
})

// 违规检测后
for _, v := range violations {
    changelogStore.Append(ArchChangeEntry{
        Category: "violation_add", Severity: v.Severity,
        File: v.File, Detail: v.Message, Source: "watch",
    })
}
```

#### 全局初始化（main.go）

```go
changelogStore = NewArchChangelogStore(filepath.Join(sourceDir, ".arch-changelog"))
```

#### 验收检查清单

- [ ] 文件创建/修改/删除均记录到 changelog
- [ ] 违规检测事件记录到 changelog
- [ ] Source 字段正确区分 "watch" / "manual_refresh"
- [ ] 不影响现有 watchFiles 性能

---

### R2-T03: 变动日志 REST API + SSE

**文件**: `cmd/arch-manager/arch_changelog_api.go` (新建)
**Package**: `main`

#### API 端点清单（3 个 + 1 SSE）

| 端点 | 方法 | 功能 |
|------|------|------|
| `/api/arch-changelog` | GET | 查询变动日志（category/severity/file/source/since/limit/offset） |
| `/api/arch-changelog/stats` | GET | 变动统计摘要（total + by_category + by_severity + by_source） |
| `/api/sse/arch-changelog` | SSE | 实时推送变动事件 |

#### Handler 模板

```go
// GET /api/arch-changelog
func handleArchChangelog(w http.ResponseWriter, r *http.Request) {
    q := r.URL.Query()
    filter := ArchChangeFilter{
        Category: q.Get("category"), Severity: q.Get("severity"),
        File: q.Get("file"), Source: q.Get("source"), Limit: 100,
    }
    if v := q.Get("limit"); v != "" { if n, err := strconv.Atoi(v); err == nil && n > 0 { filter.Limit = n } }
    entries := changelogStore.Query(filter)
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(entries)
}

// GET /api/arch-changelog/stats
func handleArchChangelogStats(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(changelogStore.Stats())
}
```

#### SSE Handler

复用 `devEventBus` 模式，`changelogStore.Append()` 中同步 publish 到 `changelogEventBus`。

#### 路由注册（main.go）

```go
mux.HandleFunc("/api/arch-changelog", handleArchChangelog)
mux.HandleFunc("/api/arch-changelog/stats", handleArchChangelogStats)
mux.HandleFunc("/api/sse/arch-changelog", handleArchChangelogSSE)
```

#### 验收检查清单

- [ ] `GET /api/arch-changelog` 返回变动日志列表
- [ ] `GET /api/arch-changelog?category=file_add` 正确过滤
- [ ] `GET /api/arch-changelog/stats` 返回统计摘要
- [ ] `GET /api/sse/arch-changelog` 实时推送变动事件

---

### R2-T04: 前端侧边栏新增面板项

**文件**: `arch-manager.html` (修改)
**修改位置**: "可观测性"分组的 `devProgress` 之后

#### HTML 模板

```html
<div class="sidebar-item" onclick="switchView('archChangelog')" data-view="archChangelog">
  <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><polyline points="14 2 14 8 20 8"/><line x1="16" y1="13" x2="8" y2="13"/><line x1="16" y1="17" x2="8" y2="17"/></svg>
  架构变动日志 <span id="changelogBadge" style="display:none;background:var(--orange);color:#fff;font-size:10px;padding:1px 6px;border-radius:8px;margin-left:auto">0</span>
</div>
```

#### 同步修改

1. 全局变量: `let archChangelog = [], archChangelogStats = null; let sseChangelog = null;`
2. `renderCurrentView()` switch: `case 'archChangelog': renderArchChangelog(container); break;`
3. `connectSSE()` 新增 SSE 连接 + Badge 更新
4. `fetchAll()` 新增 2 个 API 调用

#### 验收检查清单

- [ ] 侧边栏"可观测性"分组新增"架构变动日志"面板项
- [ ] Badge 显示变动条目数
- [ ] SSE 实时推送自动更新

---

### R2-T05: 架构变动日志面板

**文件**: `arch-manager.html` (修改)
**函数**: `function renderArchChangelog(container)`

#### 面板内容

- 4 个统计卡片: 总变动、文件变更、违规事件、日志条目
- 查询栏: Category 下拉 + Severity 下拉 + 查询按钮
- ECharts 散点时间线: 按时间分布的变动事件
- 变动日志表格: 时间、类别（着色）、级别（着色）、文件、详情

#### 验收检查清单

- [ ] 4 个统计卡片正确显示
- [ ] Category/Severity 过滤功能正常
- [ ] 时间线图正确渲染
- [ ] 日志表格按类别/级别着色
- [ ] SSE 实时推送自动追加并更新 Badge
- [ ] 空状态显示友好提示

---

## 6. 任务依赖与执行路径

```
R1-T01 (migrate_api.go) ──┬──→ R1-T03 (main.go 路由)
R1-T02 (migrate_sse.go) ──┘         │
                                    ▼
R1-T04 (sidebar) ──→ R1-T05~T09 (5 个渲染函数)
                │
                └──→ R1-T10 (fetchAll 集成)

R2-T01 (arch_changelog.go) ──→ R2-T02 (watchFiles 集成)
                                     │
                            R2-T03 (API + SSE) ──→ R2-T04 (sidebar)
                                                      │
                                             R2-T05 (渲染函数)
```

**建议执行路径**（可并行组）:

| 阶段 | 任务 | 说明 |
|------|------|------|
| P1 | R1-T01 + R1-T02 + R2-T01 | 后端 3 个文件并行创建 |
| P2 | R1-T03 + R2-T02 | 集成到 main.go + sse.go |
| P3 | R2-T03 | 变动日志 API |
| P4 | R1-T04 + R2-T04 | 前端 sidebar 修改 |
| P5 | R1-T05~T09 + R2-T05 | 前端 6 个渲染函数 |
| P6 | R1-T10 | fetchAll 集成 |

---

## 7. 文件变更汇总

### 新增文件（4 个）

| 文件 | 行数估算 | 职责 |
|------|---------|------|
| `cmd/arch-manager/migrate_api.go` | ~300 | 迁移引擎 7 个 REST API handler |
| `cmd/arch-manager/migrate_sse.go` | ~120 | 迁移引擎 SSE 事件流 + EventBus |
| `cmd/arch-manager/arch_changelog.go` | ~200 | 架构变动日志持久化存储 |
| `cmd/arch-manager/arch_changelog_api.go` | ~150 | 变动日志 3 个 REST API + SSE |

### 修改文件（3 个）

| 文件 | 修改量 | 修改内容 |
|------|--------|---------|
| `cmd/arch-manager/main.go` | +20 行 | 路由注册 + changelogStore 初始化 |
| `cmd/arch-manager/sse.go` | +30 行 | watchFiles 中插入 changelog 记录 |
| `arch-manager.html` | +500 行 | sidebar 分组 + 6 个渲染函数 + SSE + fetchAll |
