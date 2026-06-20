# Low-Entropy Core 全链路动态架构仪表盘 — 设计与实施计划

## 核心思考：什么是一个"每一个环节都可控、可视、可溯源"的仪表盘？

### 现状诊断

当前的 `v4-dashboard` 是**静态报告页**——数据写死在 HTML/JS 中，打开后永远不会变化。它回答的是"系统做了什么"，而不是"系统正在做什么"。

一个真正的动态仪表盘必须满足三个维度：

- **可控**：不只是看，还能操作——调整阈值、暂停监控、手动 Override 决策、模拟故障场景
- **可视**：从细胞级（单个函数调用）到器官级（模块健康度）到有机体级（全局熵值），每个层次都有对应的可视化
- **可溯源**：看到任何一个异常指标，都能一路追溯到具体的请求、代码行、决策原因

### 生物学隐喻 → 全链路架构映射

| 生物学 | 架构对应 | 仪表盘表现 | 数据来源 | 可控？ | 可溯源？ |
|--------|----------|-----------|---------|--------|---------|
| **心跳** | TPS / 请求吞吐量 | 实时脉搏曲线 | Observation Pipeline | 调整采样率 | → Trace 详情 |
| **体温** | 系统熵值 | 温度计仪表 | EntropyCollector | 调整阈值 | → 熵值公式分解 |
| **血压** | 错误率 × P99 延迟 | 双轴仪表 | Aggregator | 调整告警规则 | → 具体错误请求 |
| **细胞代谢** | 函数调用频次/耗时 | 热力图 | StepStore Query | 暂停/恢复模块监控 | → ExecutionStep 详情 |
| **细胞凋亡** | 错误/超时/熔断 | 红色闪烁节点 | ErrorAlwaysSampler | 手动触发熔断 | → 错误堆栈 |
| **器官** | Guardian/Observation/EventStore | 器官健康度卡片 | HealthCheck | 暂停/恢复模块 | → 模块内部函数 |
| **免疫系统** | Guardian DecisionEngine | 实时决策事件流 | AlertAdapter | Override 决策 | → 决策规则链 |
| **神经系统** | Trace Tree | 神经元放电动画 | TraceTree API | 调整 Trace 采样 | → 每个 Span 详情 |
| **血液循环** | 模块间数据流 | 粒子流动拓扑图 | 依赖图 + TPS | 模拟流量变化 | → 具体数据包 |
| **DNA** | ADR + 代码结构 | 基因组图谱 | arch-manager API | 创建 ADR | → 具体代码行 |

---

## 实施计划

### Phase 1: 后端 Control Plane（控制面）

**目标**：让架构管理器不仅能"看"，还能"控"

#### 1.1 新增 Control Plane REST 端点

在 `cmd/arch-manager/main.go` 中新增：

| 端点 | 方法 | 功能 | 参数 |
|------|------|------|------|
| `/api/control/sampling` | PATCH | 动态调整采样率 | `{module, rate, duration, reason}` |
| `/api/control/thresholds` | PATCH | 调整熵值告警阈值 | `{yellow, orange, red, mode}` |
| `/api/control/modules/{id}/pause` | POST | 暂停模块监控 | `{reason, duration}` |
| `/api/control/modules/{id}/resume` | POST | 恢复模块监控 | `{reason}` |
| `/api/control/override` | POST | Override Guardian 决策 | `{target_action, reason, expires_at, scope}` |
| `/api/control/override` | DELETE | 撤销 Override | `{override_id}` |
| `/api/control/simulate` | POST | 运行 What-If 模拟 | `{scenario, params}` |
| `/api/control/status` | GET | 获取所有控制状态 | — |

#### 1.2 新增 SSE 实时推送端点

| 端点 | 推送频率 | 数据内容 |
|------|---------|---------|
| `/api/sse/metrics` | 1秒 | TPS、熵值、错误率、P50/P99 延迟、goroutine 数、内存 |
| `/api/sse/heartbeat` | 5秒 | 各模块健康状态（心跳信号） |
| `/api/sse/immune` | 事件驱动 | Guardian 决策事件（含 Override 标记） |
| `/api/sse/traces` | 事件驱动 | 最新 Trace 树（最近 10 条，含 Span 详情） |
| `/api/sse/cells` | 2秒 | 函数级调用热度矩阵（热力图数据） |
| `/api/sse/annotations` | 事件驱动 | 控制操作标注（采样率变更、Override、暂停等） |

#### 1.3 新增 REST 查询端点

| 端点 | 功能 |
|------|------|
| `/api/dashboard/overview` | 全局概览（熵值、TPS、健康度、模块数、运行时间） |
| `/api/dashboard/modules` | 各模块健康度详情（含暂停状态、Override 状态） |
| `/api/dashboard/cells` | 细胞级数据（最近 N 个 ExecutionStep，可按模块过滤） |
| `/api/dashboard/immune-history` | Guardian 决策历史（含 Override 审计日志） |
| `/api/dashboard/topology` | 模块依赖拓扑图数据（含实时 TPS 和健康状态） |
| `/api/dashboard/entropy-trend` | 熵值时间序列（最近 1 小时，含多维度分解） |
| `/api/dashboard/trace/{id}` | 单条 Trace 完整详情（含所有 Span 的耗时、状态、错误） |
| `/api/dashboard/causation` | 事件因果链数据（Guardian 决策 → 触发的模块 → 影响的 Trace） |

#### 1.4 数据源架构：真实数据优先，模拟仅作开发辅助

**生产模式**：所有数据来自 go-core 运行时的真实 Observation Pipeline + Guardian DecisionEngine
- Observation Pipeline 持续产生 `ExecutionStep` → Buffer → Sampler → Aggregator → Store
- Guardian DecisionEngine 持续产生决策事件 → AlertAdapter
- EntropyCollector 持续计算熵值快照
- SSE 端点直接订阅这些真实数据流

**开发模式**（`--dev` 标志）：当 go-core 未运行时，arch-manager 内置模拟数据引擎提供演示数据
- 仅在 `--dev` 模式下激活，生产环境完全禁用
- 模拟引擎生成与真实数据结构完全一致的数据，确保前端代码无需区分数据来源
- 用于前端开发和调试，不用于任何生产决策

**What-If 场景推演**（生产可用）：
- 不是模拟数据，而是基于当前真实数据做**预测推演**
- 输入：假设参数（如"熵值升到 80"、"错误率升到 5%"）
- 输出：Guardian DecisionEngine 的预测决策 + 影响范围分析
- 实现方式：调用 `DecisionEngine.evaluate()` 的纯函数，传入假设参数，不产生副作用
- 所有推演操作记录审计日志，标注 `[SIMULATION]`

### Phase 2: 前端仪表盘实现

**文件**: `v4-dashboard/v4-dashboard.html`（重写）+ `v4-dashboard/assets/charts.js`（重写）

#### 2.1 页面整体布局

```
┌──────────────────────────────────────────────────────────────────┐
│  Header: Low-Entropy Core · 状态灯(绿/黄/红) · 运行时间 · 控制  │
│  [变量栏: 模块▼ | 时间窗口▼ | TraceID搜索...]                   │
├────────┬───────────────────────────────────────────────────────┤
│        │  ┌─────────────────┐ ┌─────────────────────────────┐   │
│  模块   │  │  心跳曲线 (TPS)  │ │  熵值温度计 + 多维分解      │   │
│  健康   │  │  实时折线 60s    │ │  全局/模块/Pipeline/Agent   │   │
│  度列表  │  └─────────────────┘ └─────────────────────────────┘   │
│  (器官  │  ┌─────────────────┐ ┌─────────────────────────────┐   │
│  级)    │  │  延迟分位数       │ │  错误率 + 资源监控          │   │
│  每行   │  │  P50/P95/P99    │ │  内存/goroutine/channel     │   │
│  可点击  │  └─────────────────┘ └─────────────────────────────┘   │
│  下钻   │  ┌────────────────────────────────────────────────┐   │
│  可暂停  │  │  免疫系统事件流 (Guardian 决策 + Override)      │   │
│  可恢复  │  │  实时滚动，每条可点击溯源，可 Override            │   │
│        │  └────────────────────────────────────────────────┘   │
│        │  ┌───────────────────┬────────────────────────────┐   │
│        │  │  细胞热力图       │  神经系统 Trace Tree          │   │
│        │  │  函数调用热度矩阵  │  可展开/折叠，点击溯源        │   │
│        │  │  悬停显示详情     │  神经元放电动画               │   │
│        │  └───────────────────┴────────────────────────────┘   │
│        │  ┌────────────────────────────────────────────────┐   │
│        │  │  血液循环拓扑图 (力导向图 + 粒子流动动画)         │   │
│        │  │  节点=模块，边=调用，粒子=实时数据流              │   │
│        │  └────────────────────────────────────────────────┘   │
├────────┴───────────────────────────────────────────────────────┤
│  Footer: 数据源状态 · SSE 连接状态 · 最后刷新 · Annotations    │
└──────────────────────────────────────────────────────────────────┘
```

#### 2.2 控制面板（右侧抽屉）

点击 Header 的"控制"按钮，从右侧滑出控制面板：

```
┌────────────────────────────────────────────┐
│  控制面板                          [✕ 关闭] │
├────────────────────────────────────────────┤
│                                            │
│  ▼ 采样控制                                │
│    全局采样率: ═══●═══════ 50%             │
│    有效期: [30分钟 ▼]  原因: [调试中]       │
│    [应用]                                   │
│                                            │
│  ▼ 阈值控制                                │
│    Yellow: ═══●═══════ 20                  │
│    Orange: ══════●═════ 50                  │
│    Red:    ══════════●══ 80                 │
│    模式: [自动适应 ▼]                       │
│    [应用]                                   │
│                                            │
│  ▼ Guardian Override                        │
│    当前活跃 Override: 0                     │
│    [新建 Override]                           │
│    ┌──────────────────────────────────┐    │
│    │ 目标动作: [Block ▼]               │    │
│    │ 原因: [排查异常中]                  │    │
│    │ 有效期: [30分钟 ▼]                 │    │
│    │ 范围: [全局 ▼]                      │    │
│    │ [确认 Override]                     │    │
│    └──────────────────────────────────┘    │
│                                            │
│  ▼ 场景模拟 (What-If)                      │
│    场景: [熵值突增 ▼]                       │
│    熵值: ═══════●═══════ 75/100            │
│    错误率: ═══●══════════ 3%               │
│    P99: ═══════●═══════ 200ms              │
│    [▶ 运行模拟]                            │
│    ┌──────────────────────────────────┐    │
│    │ 预测结果:                          │    │
│    │ Guardian 决策: ROLLBACK            │    │
│    │ 触发规则: entropy Red + drift>0.7  │    │
│    │ 影响范围: 3 个 Pipeline 被阻断      │    │
│    └──────────────────────────────────┘    │
│                                            │
└────────────────────────────────────────────┘
```

#### 2.3 溯源面板（点击下钻时出现）

从任何异常指标点击"溯源"，右侧滑入溯源面板，展示完整因果链：

```
┌────────────────────────────────────────────┐
│  溯源: 告警 > Guardian > evaluate()        │
│  面包屑: [告警] > [Guardian] > [evaluate]   │
├────────────────────────────────────────────┤
│                                            │
│  事件因果链 (DAG)                           │
│  ┌──────────┐     ┌──────────┐            │
│  │ 熵值突增  │────▶│ Guardian  │            │
│  │ 14:32:01 │     │ WARN     │            │
│  └──────────┘     └────┬─────┘            │
│                        │                  │
│                  ┌─────▼──────┐            │
│                  │ evaluate() │            │
│                  │ 耗时 120ms │            │
│                  └─────┬──────┘            │
│                        │                  │
│              ┌─────────┼─────────┐        │
│              ▼         ▼         ▼        │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐   │
│  │entropyChk│ │driftDet │ │alertDisp │   │
│  │  45ms   │ │  60ms   │ │  15ms   │   │
│  └──────────┘ └──────────┘ └──────────┘   │
│                                            │
│  关联 Trace:                                │
│  TraceID: abc123...  [查看完整 Trace →]     │
│                                            │
│  时间线:                                    │
│  14:32:01 熵值突增 (entropy=65)            │
│  14:32:02 drift检测 (score=0.45)           │
│  14:32:03 Guardian WARN                    │
│  14:32:03 告警分发 → Webhook               │
│                                            │
│  [Override 此决策]  [查看原始数据]           │
└────────────────────────────────────────────┘
```

#### 2.4 时间旅行面板（底部抽屉）

点击 Footer 的"时间旅行"按钮，从底部滑出时间轴控件：

```
┌──────────────────────────────────────────────────────────┐
│  时间旅行                                    [✕ 关闭]     │
│  ◀◀  ◀  ⏸  ▶  ▶▶   [====●==================] 14:32:05    │
│  速度: [1x] [2x] [5x] [10x]   步长: [1s] [10s] [1m]       │
├──────────────────────┬───────────────────────────────────┤
│  系统快照 @14:32:05   │  事件流                             │
│  熵值: 45.2           │  14:31:58 entropy Yellow ▲          │
│  TPS:  1,234          │  14:32:01 drift detected (0.45)     │
│  错误率: 0.8%         │  14:32:03 guardian WARN              │
│  Guardian: ⚠          │  14:32:05 ● 当前位置                 │
│  Observer: ✅         │                                      │
│  EventStore: ✅       │  Trace Waterfall:                    │
│  Goroutine: 42        │  ═══ Trace#abc123 ═══ 150ms          │
│  Memory: 128MB        │  ├─ evaluate() ──────── 120ms        │
│                      │  │  ├─ entropyCheck() 45ms            │
│                      │  │  └─ driftDetect()  60ms            │
│                      │  └─ alertDispatch() ── 25ms           │
└──────────────────────┴───────────────────────────────────┘
```

### Phase 3: 动画与交互细节

#### 3.1 实时数据推送（SSE 连接管理）

```javascript
// 多路 SSE 连接，统一管理
const connections = {
    metrics:    new EventSource('/api/sse/metrics'),
    heartbeat:  new EventSource('/api/sse/heartbeat'),
    immune:     new EventSource('/api/sse/immune'),
    traces:     new EventSource('/api/sse/traces'),
    cells:      new EventSource('/api/sse/cells'),
    annotations: new EventSource('/api/sse/annotations'),
};

// 连接状态指示器
connections.metrics.onopen = () => updateConnectionStatus('metrics', 'connected');
connections.metrics.onerror = () => updateConnectionStatus('metrics', 'reconnecting');
```

#### 3.2 关键动画清单

| 动画 | 触发条件 | 实现方式 |
|------|---------|---------|
| 心跳脉冲 | 每秒新数据到达 | ECharts `markPoint` + CSS `pulse` 动画 |
| 温度计指针 | 熵值变化 | CSS `transition: transform 0.5s` |
| 免疫响应 | Block/Rollback 事件 | 对应模块卡片 `box-shadow` 红色脉冲 |
| 细胞呼吸 | 每 2 秒热力图更新 | ECharts `visualMap` 渐变过渡 |
| 血液流动 | 持续运行 | ECharts Graph `lines` 系列 + `effect` |
| 神经放电 | 新 Trace 到达 | DOM 节点依次添加 `glow` class |
| Override 标记 | Override 激活 | 事件卡片金色边框 + `OVERRIDDEN` 标签 |
| 暂停模块 | 模块暂停 | 卡片灰度 + 暂停图标 + 倒计时 |
| 注释标记 | 控制操作 | 时间轴上垂直虚线 + 标签 |
| 溯源高亮 | 点击下钻 | 因果链路径高亮 + 非相关节点淡出 |

#### 3.3 Annotations（时间线标注）

所有控制操作自动在时间轴上添加标注：
- 采样率变更 → 蓝色虚线 + "采样率调整为 50%"
- Override → 金色虚线 + "Override: Guardian → Block"
- 模块暂停 → 灰色虚线 + "Guardian 监控已暂停"
- 场景模拟 → 紫色虚线 + "模拟: 熵值突增场景"
- Guardian 决策 → 红/黄/绿色虚线 + 决策类型

### Phase 4: 集成与验证

**开发模式验证**（`--dev`）：
1. 启动 arch-manager --dev，确认模拟数据引擎激活
2. 打开仪表盘，确认 6 路 SSE 连接全部建立，数据正常流动
3. 验证心跳曲线每秒更新，熵值温度计实时变化
4. 验证免疫事件流出现 Warn/Block 事件
5. 验证细胞热力图亮度随数据变化
6. 验证拓扑图粒子持续流动

**生产模式验证**（go-core 运行中）：
7. 启动 go-core 示例服务 + arch-manager，确认数据来自真实 Observation Pipeline
8. 发送真实请求，验证 TPS 曲线反映实际吞吐量
9. 触发真实 Guardian 决策，验证免疫事件流正确显示
10. 验证 Trace 树展示真实的 ExecutionStep 调用链

**控制面验证**（两种模式均需）：
11. 验证控制面板：调整采样率 → 观察热力图数据密度变化
12. 验证 Override：Override Guardian 决策 → 观察事件流标记变化
13. 验证 What-If 推演：输入假设参数 → 显示预测决策结果（标注 [SIMULATION]）
14. 验证溯源：从告警点击溯源 → 展示完整因果链
15. 验证时间旅行：拖动时间轴 → 系统快照和事件流联动

---

## 技术选型

| 组件 | 选型 | 理由 |
|------|------|------|
| 前端 | 原生 HTML + CSS + JS | 与 arch-manager 一致，零外部依赖 |
| 图表 | ECharts 5.x | 项目已广泛使用，支持实时更新 + 力导向图 + 热力图 |
| 实时通信 | SSE | arch-manager 已有 SSE 基础设施，轻量高效 |
| 后端 | Go (arch-manager) | 复用现有 28 个 API 端点架构 |
| 模拟数据 | Go math/rand | 简单高效，无需外部依赖 |

---

## 文件变更清单

| 文件 | 操作 | 说明 |
|------|------|------|
| `cmd/arch-manager/main.go` | 修改 | 新增 8 个 Control API + 6 个 SSE 端点 + 8 个 Dashboard API + 模拟引擎 |
| `v4-dashboard/v4-dashboard.html` | 重写 | 全新动态仪表盘：6 视图区域 + 控制面板 + 溯源面板 + 时间旅行面板 |
| `v4-dashboard/assets/charts.js` | 重写 | ECharts 实时图表 + SSE 连接管理 + 动画控制 + 面板交互逻辑 |

---

## 验证标准

1. **可控**：通过控制面板调整采样率/阈值/Override/模拟场景，仪表盘实时响应
2. **可视**：6 个视图区域全部有实时数据驱动，无静态写死数据
3. **可溯源**：从任何异常指标，3 次点击内到达具体 ExecutionStep 或 Trace 详情
4. **实时性**：TPS/熵值 1 秒刷新，模块心跳 5 秒刷新，免疫事件即时推送
5. **流畅度**：所有动画在 Chrome 中保持 60fps
