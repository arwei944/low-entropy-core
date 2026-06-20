# Low-Entropy Core — 架构速查

> 一份给 AI 代理快速理解架构的地图。详细约束见 `CLAUDE.md`。

---

## 架构全景（一张图）

```
L7 应用层 ─── cmd/arch-manager/, cmd/lecore/
         │
L6 EventStore ─── eventstore.go, eventstore_wal.go, snapshot.go
         │
L5 Observation ─── observation_pipeline.go, observation_aggregator.go
         │         observation_api.go, observation_sampler.go, observation_store.go
         │
L4 Guardian ─── guardian.go, guardian_entropy.go, guardian_threshold.go
         │      guardian_types.go, guardian_decision.go
         │
L3 分布式韧性 ─── patterns_distributed.go, fastpath.go, perf_core.go
         │      observation.go, risk.go, engine.go, wallet.go, scheduler.go
         │
L2 单机韧性 ─── circuit_breaker.go, retry.go, ratelimit.go, bulkhead.go
         │      backpressure.go, cache.go, dedup.go, idempotency.go, shed_load.go
         │
L1 四原语 ─── atom.go, port.go, adapter.go, composer.go
         │
L0 错误处理 ─── errors.go, errors_enhanced.go
```

---

## 核心概念

### 四原语（L1）
所有业务逻辑由 4 种泛型接口组合而成：

```
Atom[In, Out]    → 纯计算单元
Port[In, Out]    → 外部边界（I/O/网络/MQ）
Adapter[In, Out] → 协议转换
Composer[In, Out] → 编排器（组合多个原语）
```

### 韧性模式（L2-L3）
- **L2 单机**：熔断、重试、限流、舱壁隔离、背压、去重、幂等
- **L3 分布式**：快速路径、分片锁、UUID 批量生成、CompactTraceID

### 监督层（L4-L5）
- **Guardian**：多维度熵值监控、阈值告警、漂移检测
- **Observation**：ExecutionStep 记录、Pipeline 聚合、TraceTree

---

## 文件组织

```
low-entropy-core/
├── go-core/           ← 核心库（package core）
│   ├── atom.go, port.go, adapter.go, composer.go   ← L1 原语
│   ├── circuit_breaker.go, retry.go, ...           ← L2 韧性
│   ├── patterns_distributed.go, fastpath.go, ...   ← L3 分布式
│   ├── guardian*.go                                 ← L4 监督
│   ├── observation*.go                              ← L5 可观测
│   ├── eventstore*.go, snapshot.go                  ← L6 事件溯源
│   └── *_test.go                                    ← 测试
├── cmd/
│   ├── arch-manager/   ← 架构仪表盘
│   └── lecore/         ← 主程序入口
├── CLAUDE.md           ← AI 代理约束规则
├── ARCHITECTURE.md     ← 本文档
└── arch-manager.html   ← 仪表盘前端
```

---

## 关键规则速查

| 规则 | 说明 |
|------|------|
| 依赖方向 | 只能向下（L7 → L0），不能向上 |
| 新文件 | 必须确定属于 L0-L7 哪一层 |
| 新功能 | 必须是 Atom/Port/Adapter/Composer 之一 |
| 包名 | 核心代码统一 `package core` |
| 外部库 | L0-L3 禁止第三方依赖 |
| 验证 | 提交前 `curl localhost:8090/api/violations` 必须为空 |

---

## 仪表盘

```bash
# 启动
go build -tags lecore_tier4 -o arch-manager.exe ./cmd/arch-manager/
.\arch-manager.exe

# 访问
http://localhost:8090/arch-manager.html
```

仪表盘实时显示：健康评分、拓扑依赖图、文件层级树、违规检测、原语分布、熵值监控。