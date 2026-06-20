# 动态架构仪表盘 — 实施计划

## 核心设计理念

**仪表盘是一个活体**：不是静态报表，而是一个持续呼吸、自我感知、可操控、可溯源的有机体。

- **可控**：每个模块/层/原语都可以手动干预（暂停、限流、覆盖阈值）
- **可视**：从宏观全景到微观细胞，每个层级都有对应的可视化
- **可溯源**：任何状态变化都可以追溯到具体的事件链和代码变更

---

## 第一核心：架构全景（Architecture Panorama）

> 解决"架构是什么、有什么"的问题

### 1.1 架构拓扑图（Architecture Topology）

**数据来源**：`/api/arch` → `ArchData`（已有）

**可视化**：
- **力导向图（Force-Directed Graph）**：节点 = 文件/模块，边 = 依赖关系
- **层级着色**：L0-L7 各层用不同颜色，一眼看出架构分层
- **交互**：
  - 点击节点 → 右侧面板展示文件详情（符号列表、依赖关系、代码预览）
  - 拖拽节点 → 调整布局
  - 悬停边 → 显示依赖方向和类型
  - 搜索框 → 快速定位文件/符号
- **问题标记**：违规节点红色高亮（循环依赖、层级反向依赖）

### 1.2 架构健康仪表（Health Gauge）

**数据来源**：`/api/health-score` → `HealthScore`（已有）

**可视化**：
- **五维雷达图**：层级平衡 / 文件粒度 / 符号密度 / 依赖深度 / 接口率
- **综合评分仪表盘**：0-100 分 + 字母等级（A+/A/B/C/D/F）
- **趋势线**：历史评分变化趋势（需新增存储）
- **建议列表**：点击建议项 → 自动跳转到对应问题区域

### 1.3 架构违规看板（Violation Board）

**数据来源**：`/api/violations` → `[]Violation`（已有）

**可视化**：
- **严重度分组**：Critical / Warning / Info 三列卡片
- **类型过滤**：循环依赖 / 层级反向 / 超大文件 / 空文件
- **点击展开**：显示违规详情 + 涉及文件列表 + 修复建议
- **溯源**：关联到 git commit（哪个提交引入了此违规）

### 1.4 四原语分布图（Primitive Distribution）

**数据来源**：AST 解析（需增强，识别 `var _ Atom[...] = ...` 等接口断言）

**可视化**：
- **旭日图（Sunburst）**：外圈 = 文件，中圈 = 原语类型（Atom/Port/Adapter/Composer），内圈 = 层级
- **统计卡片**：Atom 数量 / Port 数量 / Adapter 数量 / Composer 数量 / 总原语数
- **点击下钻**：点击某个原语 → 展示该原语的接口签名、实现类型、所在文件

### 1.5 层级统计矩阵（Layer Matrix）

**数据来源**：`/api/arch` → `ArchData.Layers`（已有）

**可视化**：
- **堆叠柱状图**：X轴 = 层级，Y轴 = 文件数/行数/符号数（可切换）
- **表格视图**：层级 | 文件数 | 行数 | 符号数 | 颜色标识
- **点击下钻**：点击某层 → 展示该层所有文件列表

---

## 第二核心：架构动态运行（Dynamic Runtime）

> 解决"架构正在怎么运行"的问题

### 2.1 实时心跳面板（Heartbeat Panel）

**生物学隐喻**：心跳 = TPS（每秒事务数）

**数据来源**：需新增 — Observation Pipeline 步数统计

**可视化**：
- **ECG 心电图样式**：实时滚动的 TPS 曲线，带脉搏动画
- **数值显示**：当前 TPS / 平均 TPS / 峰值 TPS
- **颜色编码**：绿色（正常）→ 黄色（偏高）→ 红色（过载）

### 2.2 熵值热力图（Entropy Heatmap）

**生物学隐喻**：体温 = 系统熵值

**数据来源**：
- 静态熵：`/api/entropy` → `[]EntropyMetrics`（已有）
- 动态熵：需新增 Guardian SSE 端点 → `MultiDimensionEntropySnapshot`

**可视化**：
- **模块热力图矩阵**：行 = 模块，列 = 时间窗口，颜色 = 熵值（绿→黄→红）
- **全局熵值仪表**：当前全局熵值 + 熵值等级（OK/Yellow/Orange/Red）
- **漂移检测标记**：检测到熵值加速上升的模块闪烁警告
- **点击下钻**：点击某模块 → 展示该模块的熵值历史曲线 + 触发因素

### 2.3 错误率血压计（Error Blood Pressure）

**生物学隐喻**：血压 = 错误率 × P99 延迟

**数据来源**：需新增 — Observation Pipeline 错误统计

**可视化**：
- **双指针仪表**：左指针 = 错误率，右指针 = P99 延迟
- **趋势线**：错误率和 P99 延迟的时间序列
- **错误列表**：最近的错误事件，点击 → 跳转到溯源面板

### 2.4 Trace 神经传导图（Neural Trace Graph）

**生物学隐喻**：神经系统 = 分布式追踪

**数据来源**：`/api/observation/trace-tree`（已有 API，需注册路由）

**可视化**：
- **树形调用图**：根节点 = 入口请求，子节点 = 内部调用
- **颜色编码**：绿色 = 正常，红色 = 错误，黄色 = 慢
- **耗时标注**：每条边标注耗时，粗细 = 耗时比例
- **点击节点**：展开详情（参数、返回值、错误信息）

### 2.5 数据流拓扑（Blood Circulation）

**生物学隐喻**：血液循环 = 数据流拓扑

**数据来源**：`/api/arch` 依赖关系 + Observation Pipeline 运行时调用

**可视化**：
- **动态有向图**：节点 = 模块，边 = 数据流向，边上动画 = 数据流动
- **流量标注**：边的粗细 = 调用频率，颜色 = 健康状态
- **瓶颈标记**：延迟最高的路径高亮

---

## 第三核心：控制面板（Control Plane）

> 解决"我要控制它"的问题

### 3.1 采样率控制（Sampling Control）

- **滑块**：调整 Observation Pipeline 的采样率（1% - 100%）
- **实时反馈**：调整后立即显示采样率变化对数据量的影响
- **预设模式**：开发模式（100%）/ 预发布模式（10%）/ 生产模式（1%）

### 3.2 阈值覆盖（Threshold Override）

- **阈值编辑器**：修改熵值告警阈值（Yellow/Orange/Red）
- **临时覆盖**：紧急情况下临时调整阈值，设置自动恢复时间
- **覆盖审计日志**：所有覆盖操作记录（谁、何时、为什么、恢复时间）

### 3.3 What-If 推演（What-If Analysis）

- **场景输入**：假设某个模块故障 / 负载翻倍 / 新增依赖
- **影响分析**：自动计算对全局熵值、健康评分、依赖图的影响
- **对比视图**：推演前 vs 推演后的架构状态对比

---

## 第四核心：溯源面板（Traceability Panel）

> 解决"为什么会这样"的问题

### 4.1 事件因果链（Event Causation Chain）

**数据来源**：EventStore（已有）+ Observation Pipeline（已有）

**可视化**：
- **DAG 有向无环图**：事件节点 + 因果边
- **时间线视图**：按时间排列的事件序列
- **点击事件**：展示事件详情（类型、时间、关联 trace_id、触发者）

### 4.2 时间旅行调试（Time Travel Debugging）

**数据来源**：Observation Pipeline 的历史 ExecutionStep（已有）

**可视化**：
- **时间轴滑块**：拖动到任意历史时间点
- **状态快照**：展示该时间点的架构状态、熵值、健康评分
- **回放功能**：从选定时间点开始回放事件序列

### 4.3 变更归因（Change Attribution）

**数据来源**：`/api/version/diff` + `/api/version/changelog`（已有）

**可视化**：
- **变更时间线**：按版本排列的架构变更历史
- **影响分析**：每次变更对健康评分、熵值的影响
- **归因标签**：自动标注变更类型（新增模块/修改依赖/重构/修复）

---

## 实施路线图

### Phase 1：后端 API 补全（Go 代码）

**目标**：将已有的但未暴露的数据通过 HTTP API 暴露出来

#### 1.1 注册 Observation API 路由
- **文件**：`cmd/arch-manager/main.go`
- **改动**：在 `main()` 中创建 `ObservationPipeline` + `ArchitectureRegistry`，调用 `observationAPI.RegisterHandlers(mux)`
- **依赖**：需 build tag `lecore_tier4+`

#### 1.2 新增 Guardian SSE 端点
- **文件**：`cmd/arch-manager/main.go`（新增 ~200 行）
- **新增端点**：
  - `GET /api/guardian/snapshot` — 返回 `MultiDimensionEntropySnapshot`
  - `GET /api/guardian/sse` — SSE 推送熵值快照（每 5 秒）
  - `GET /api/guardian/thresholds` — 获取当前阈值
  - `PUT /api/guardian/thresholds` — 覆盖阈值（含审计日志）
  - `GET /api/guardian/drift` — 获取漂移检测状态
  - `GET /api/guardian/history` — 获取熵值历史（最近 N 条）
- **实现**：创建 `MultiDimensionEntropyCollector`，定时采集，SSE 推送

#### 1.3 新增运行时统计端点
- **文件**：`cmd/arch-manager/main.go`（新增 ~100 行）
- **新增端点**：
  - `GET /api/runtime/tps` — 当前 TPS（基于 Observation Pipeline 步数）
  - `GET /api/runtime/errors` — 最近错误列表
  - `GET /api/runtime/latency` — P50/P95/P99 延迟
  - `GET /api/runtime/sampling-rate` — 当前采样率
  - `PUT /api/runtime/sampling-rate` — 修改采样率

#### 1.4 新增四原语识别
- **文件**：`cmd/arch-manager/main.go`（修改 AST 解析部分，新增 ~150 行）
- **改动**：在 AST 解析中识别四原语接口断言（`var _ Atom[...] = ...`）
- **新增端点**：`GET /api/primitives` — 返回所有识别到的四原语及其位置

#### 1.5 新增健康评分历史存储
- **文件**：`cmd/arch-manager/main.go`（新增 ~80 行）
- **改动**：每次计算健康评分时保存到内存环形缓冲区（最近 100 条）
- **新增端点**：`GET /api/health-score/history` — 返回历史评分

### Phase 2：前端仪表盘（HTML/CSS/JS）

**目标**：构建单文件 HTML 仪表盘，连接所有 API

#### 2.1 技术选型
- **图表库**：ECharts 5（已在 arch-manager.html 中使用）
- **图可视化**：ECharts Graph（力导向图）+ 自定义 SVG
- **实时通信**：SSE（EventSource）
- **样式**：暗色主题，macOS 设计风格（延续 arch-manager.html 风格）
- **字体**：JetBrains Mono（代码）+ Work Sans（UI）

#### 2.2 布局结构
```
┌─────────────────────────────────────────────────────┐
│ 顶部状态栏：全局健康评分 | 全局熵值 | TPS | 时间    │
├──────────┬──────────────────────────────┬────────────┤
│          │                              │            │
│ 左侧导航 │     主视图区域                │ 右侧详情  │
│          │                              │   面板     │
│ ▸ 架构全景│  [根据导航切换]              │            │
│   拓扑图  │                              │ 文件详情  │
│   健康仪表│                              │ 符号列表  │
│   违规看板│                              │ 依赖关系  │
│   原语分布│                              │ 代码预览  │
│   层级矩阵│                              │            │
│          │                              │ 事件详情  │
│ ▸ 动态运行│                              │ Trace树   │
│   心跳    │                              │            │
│   熵值热力│                              │            │
│   错误血压│                              │            │
│   Trace图 │                              │            │
│   数据流  │                              │            │
│          │                              │            │
│ ▸ 控制面板│                              │            │
│   采样率  │                              │            │
│   阈值    │                              │            │
│   What-If │                              │            │
│          │                              │            │
│ ▸ 溯源面板│                              │            │
│   因果链  │                              │            │
│   时间旅行│                              │            │
│   变更归因│                              │            │
│          │                              │            │
└──────────┴──────────────────────────────┴────────────┘
```

#### 2.3 文件结构
- **主文件**：`arch-manager.html`（重写，~3000-4000 行）
- **不拆分**：保持单文件 HTML，便于部署和分发
- **内嵌**：所有 CSS 和 JS 内嵌在 HTML 中

### Phase 3：集成与验证

#### 3.1 启动验证
- 启动 arch-manager（`go run ./cmd/arch-manager --watch`）
- 验证所有新增端点返回正确数据
- 验证 SSE 推送正常工作

#### 3.2 仪表盘验证
- 打开 arch-manager.html
- 验证架构全景五个子视图正常渲染
- 验证动态运行五个子视图实时更新
- 验证控制面板操作生效
- 验证溯源面板数据完整

---

## 关键设计决策

1. **单文件 HTML**：不引入构建工具（Webpack/Vite），保持零依赖部署
2. **SSE 而非 WebSocket**：单向推送足够，SSE 更简单且自动重连
3. **内存存储**：历史数据存内存环形缓冲区，不引入数据库依赖
4. **复用现有 API**：优先使用已有的 28 个端点，仅新增必要的端点
5. **渐进增强**：后端 API 先行，前端可以分步迭代
6. **不引入模拟场景**：实战项目不需要模拟数据，所有数据来自真实运行时

---

## 新增 API 端点汇总

| 端点 | 方法 | Phase | 说明 |
|------|------|-------|------|
| `/api/guardian/snapshot` | GET | 1.2 | 多维度熵值快照 |
| `/api/guardian/sse` | GET | 1.2 | 熵值 SSE 推送 |
| `/api/guardian/thresholds` | GET | 1.2 | 当前阈值 |
| `/api/guardian/thresholds` | PUT | 1.2 | 覆盖阈值 |
| `/api/guardian/drift` | GET | 1.2 | 漂移检测状态 |
| `/api/guardian/history` | GET | 1.2 | 熵值历史 |
| `/api/runtime/tps` | GET | 1.3 | 当前 TPS |
| `/api/runtime/errors` | GET | 1.3 | 最近错误 |
| `/api/runtime/latency` | GET | 1.3 | 延迟分位数 |
| `/api/runtime/sampling-rate` | GET | 1.3 | 采样率 |
| `/api/runtime/sampling-rate` | PUT | 1.3 | 修改采样率 |
| `/api/primitives` | GET | 1.4 | 四原语列表 |
| `/api/health-score/history` | GET | 1.5 | 健康评分历史 |

**已有但未注册的端点（Phase 1.1 注册）**：

| 端点 | 方法 | 说明 |
|------|------|------|
| `/api/observation/steps` | GET | 所有执行步骤 |
| `/api/observation/steps/query` | GET | 条件查询步骤 |
| `/api/observation/steps/errors` | GET | 仅错误步骤 |
| `/api/observation/trace/{id}` | GET | 按 trace_id 查步骤 |
| `/api/observation/trace-tree` | GET | 完整 trace 树 |
| `/api/observation/aggregates` | GET | 所有聚合指标 |
| `/api/observation/aggregates/query` | GET | 条件查询聚合 |
| `/api/observation/pipelines` | GET | Pipeline 列表 |
| `/api/observation/architecture` | GET | 架构总览 |
