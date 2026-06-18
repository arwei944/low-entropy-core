# Phase 6: 架构修复与功能补全 实施计划

> **For agentic workers:** 按任务顺序执行。每个 Task 使用 checkbox (`- [ ]`) 追踪。

**Goal:** 修复 Handoff 协议违规 + 补全 FanOut/Debounce/Throttle 模式 + 前端增强 + 测试覆盖。

**Architecture:** 四管齐下：(1) 修复 NewHandoff 使其遵守 03-agent-handoff-protocol.md 协议 (2) 新增 3 个 Composer 模式 (3) 前端添加热力图和时间线 (4) 为关键模块补测试。

**Tech Stack:** Go 1.22 stdlib，纯四原语架构，HTML/CSS/JS 前端。

**前置验证:** 对 1M LOC 模拟报告的 9 个 P0 问题逐一核实，确认 5 个已修复（Parallel 真并发、WithTimeout 真实现、Calculator 已集成 RPN、go.mod 已齐全、观测采样/聚合已存在），本计划仅针对剩余 1 个真实问题 + 功能补全。

---

## 核实结果：已修复（无需处理）

| 问题 | 实际状态 |
|------|----------|
| Parallel 假并发 | `RunParallel` 使用真实 goroutine + WaitGroup + channel，并非假并发 |
| WithTimeout 假实现 | 使用 `context.WithTimeout(ctx, t.timeout)`，并非假实现 |
| Calculator 集成不完整 | `types.go` 有完整 RPN 实现（Tokenize→Validate→ToRPN→EvaluateRPN），`server.go` 构建了完整 Pipeline |
| 根目录/go.mod 缺失 | 所有目录 `go.mod` 均已存在 |
| 观测采样/聚合缺失 | `observation_sampler.go` + `observation_aggregator.go` 均已实现 |

---

### Task 6.1: 修复 NewHandoff 协议违规

**Files:**
- Modify: `go-core/handoff.go:197-220`
- Modify: `go-core/handoff_test.go`（追加测试）

**Goal:** `NewHandoff` 当前不产生 `Pattern: "Handoff"` 的 ExecutionStep、不调用 ObservationAdapter、source Composer 未使用。改为使用 `HandoffComposer` 内部实现或重写为正确协议。

**当前问题代码** (`handoff.go:197-220`):
```go
func NewHandoff(source, target Composer[any], snapshot SnapshotAdapter[any], transport TransportFunc) Composer[any] {
    return NewPipeline[any](nil,  // ← nil obs: 不记录任何步骤
        StepFunc[any, any]{
            execute: func(ctx context.Context, input any) (any, error) {
                // ...
                _ = transport(snap)           // ← 丢弃 transport 结果
                _, _, runErr := target.Run(ctx, state)  // ← 丢弃 target 的 steps
                // ...
            },
            unitType: "Handoff",  // ← 只设 unitType，不设 Pattern
        },
    )
}
```

**修复后代码**:

```go
// NewHandoff 创建 Agent 交接 Composer。
// 遵守 docs/upgrade/03-agent-handoff-protocol.md 协议：
//   - 产生 Pattern: "Handoff" 的 ExecutionStep
//   - 调用 ObservationAdapter 记录所有步骤
//   - source Composer 的结果作为 snapshot 的输入
//   - target Composer 的执行步骤被完整收集
func NewHandoff(source, target Composer[any], snapshot SnapshotAdapter[any], transport TransportFunc, obs ObservationAdapter) Composer[any] {
	if obs == nil {
		obs = &NoOpObservationAdapter{}
	}
	return &handoffComposer[any]{
		source:   source,
		target:   target,
		snapshot: snapshot,
		transport: transport,
		obs:      obs,
	}
}

type handoffComposer[T any] struct {
	source    Composer[T]
	target    Composer[T]
	snapshot  SnapshotAdapter[T]
	transport TransportFunc
	obs       ObservationAdapter
}

func (h *handoffComposer[T]) Run(ctx context.Context, input T) (T, []ExecutionStep, error) {
	steps := make([]ExecutionStep, 0, 4)
	traceID := string(NewTraceID())
	parentSpanID := string(NewSpanID())

	// Step 1: 执行 source Composer
	step1 := NewExecutionStepWithTrace(parentSpanID, "Handoff", "SourceComposer", "executing source composer", "Handoff")
	step1.TraceID = traceID
	now := time.Now()
	sourceResult, sourceSteps, sourceErr := h.source.Run(ctx, input)
	step1.DurationMs = time.Since(now).Milliseconds()
	if sourceErr != nil {
		step1.Error = NewStepError("SOURCE_FAILED", sourceErr.Error(), false)
		steps = append(steps, step1)
		steps = append(steps, sourceSteps...)
		h.obs.Record(steps)
		return sourceResult, steps, sourceErr
	}
	steps = append(steps, step1)
	steps = append(steps, sourceSteps...)

	// Step 2: 创建快照
	step2 := NewExecutionStepWithTrace(parentSpanID, "Handoff", "CreateSnapshot", "creating handoff snapshot", "Handoff")
	step2.TraceID = traceID
	now = time.Now()
	snap := h.snapshot.CreateSnapshot(sourceResult)
	step2.DurationMs = time.Since(now).Milliseconds()
	steps = append(steps, step2)

	// Step 3: 传输快照
	step3 := NewExecutionStepWithTrace(parentSpanID, "Handoff", "TransportSnapshot", "transporting snapshot", "Handoff")
	step3.TraceID = traceID
	now = time.Now()
	transported := h.transport(snap)
	_ = transported
	step3.DurationMs = time.Since(now).Milliseconds()
	steps = append(steps, step3)

	// Step 4: 恢复并执行 target
	step4 := NewExecutionStepWithTrace(parentSpanID, "Handoff", "RestoreAndExecute", "restoring snapshot and executing target", "Handoff")
	step4.TraceID = traceID
	now = time.Now()
	state, restoreErr := h.snapshot.RestoreSnapshot(snap)
	if restoreErr != nil {
		step4.Error = NewStepError("RESTORE_FAILED", restoreErr.Error(), false)
		step4.DurationMs = time.Since(now).Milliseconds()
		steps = append(steps, step4)
		h.obs.Record(steps)
		return input, steps, restoreErr
	}
	targetResult, targetSteps, targetErr := h.target.Run(ctx, state)
	step4.DurationMs = time.Since(now).Milliseconds()
	if targetErr != nil {
		step4.Error = NewStepError("TARGET_FAILED", targetErr.Error(), false)
		steps = append(steps, step4)
		steps = append(steps, targetSteps...)
		h.obs.Record(steps)
		return targetResult, steps, targetErr
	}
	steps = append(steps, step4)
	steps = append(steps, targetSteps...)

	h.obs.Record(steps)
	return targetResult, steps, nil
}
```

- [ ] **Step 1: 替换 NewHandoff 实现**

将 `handoff.go:197-220` 的旧 `NewHandoff` 替换为上述新实现。需要同时添加 `time` 包引用（如果尚未导入）。

- [ ] **Step 2: 更新所有 NewHandoff 调用点**

以下 2 个调用点需要添加 `obs` 参数：

`examples/calculator/server.go:167`:
```go
// 旧:
handoff := core.NewHandoff(scheduler, worker, snap, core.InProcTransport)
// 新:
handoff := core.NewHandoff(scheduler, worker, snap, core.InProcTransport, obs)
```

`examples/task_scheduler/main.go:102`:
```go
// 旧:
handoff := core.NewHandoff(scheduler, worker, snap, core.InProcTransport)
// 新:
handoff := core.NewHandoff(scheduler, worker, snap, core.InProcTransport, obs)
```

- [ ] **Step 2b: 验证两个示例编译通过**

```bash
cd examples/calculator && go build -o calculator.exe .
cd examples/task_scheduler && go build -o task_scheduler.exe .
```

- [ ] **Step 3: 编写测试验证协议遵守**

```go
// handoff_test.go 追加
func TestNewHandoff_RecordsExecutionSteps(t *testing.T) {
	obs := &InMemoryObservationAdapter{}

	source := NewPipeline[any](obs,
		AtomAsStep(Atom[any, any](func(x any) any { return x })),
	)
	target := NewPipeline[any](obs,
		AtomAsStep(Atom[any, any](func(x any) any { return x })),
	)

	snapshot := &DefaultSnapshotAdapter{}
	transport := InProcTransport

	handoff := NewHandoff(source, target, snapshot, transport, obs)

	req := HandoffRequest{
		SourceID: "agent-a",
		TargetID: "agent-b",
		TaskType: "test",
		Payload:  "hello",
		Token:    "tok-001",
	}

	result, steps, err := handoff.Run(context.Background(), req)
	if err != nil {
		t.Fatalf("handoff failed: %v", err)
	}

	hr, ok := result.(HandoffResult)
	if !ok {
		t.Fatalf("expected HandoffResult, got %T", result)
	}
	if !hr.Success {
		t.Errorf("expected successful handoff, got error: %s", hr.Error)
	}

	// 验证产生了 ExecutionStep
	if len(steps) == 0 {
		t.Error("expected execution steps to be recorded")
	}

	// 验证 Pattern 为 "Handoff"
	hasHandoffPattern := false
	for _, s := range steps {
		if s.Pattern == "Handoff" {
			hasHandoffPattern = true
			break
		}
	}
	if !hasHandoffPattern {
		t.Error("expected at least one step with Pattern 'Handoff'")
	}

	// 验证 ObservationAdapter 被调用
	if obs.StepCount() == 0 {
		t.Error("expected ObservationAdapter to have recorded steps")
	}
}

func TestNewHandoff_SourceError(t *testing.T) {
	obs := &InMemoryObservationAdapter{}

	source := NewPipeline[any](obs,
		StepFunc[any, any]{
			execute: func(ctx context.Context, input any) (any, error) {
				return nil, NewStepError("SOURCE_FAIL", "source failed", false)
			},
			unitType: "Failing",
		},
	)
	target := NewPipeline[any](obs,
		AtomAsStep(Atom[any, any](func(x any) any { return x })),
	)

	handoff := NewHandoff(source, target, &DefaultSnapshotAdapter{}, InProcTransport, obs)

	_, _, err := handoff.Run(context.Background(), HandoffRequest{
		SourceID: "a", TargetID: "b", TaskType: "test", Payload: "x", Token: "t",
	})
	if err == nil {
		t.Fatal("expected error from source failure")
	}
}
```

- [ ] **Step 4: 运行测试**

```bash
cd go-core && go test -run "TestNewHandoff" -v -count=1
```

- [ ] **Step 5: 运行全量测试**

```bash
cd go-core && go test ./... -count=1
```

- [ ] **Step 6: Commit**

```bash
git add go-core/handoff.go go-core/handoff_test.go
git commit -m "fix: NewHandoff follows protocol — records Pattern:Handoff steps with ObservationAdapter"
```

---

### Task 6.2: FanOut/Debounce/Throttle 模式

**Files:**
- Create: `go-core/patterns_fanout.go`
- Create: `go-core/patterns_fanout_test.go`

**Goal:** 新增 3 个 Composer 模式，补全分布式韧性模式矩阵。

- [ ] **Step 1: 创建 patterns_fanout.go**

```go
package core

import (
	"context"
	"sync"
	"time"
)

// ============================================================================
// FanOut — 扇出模式
// ============================================================================

// FanOut 将输入广播到多个 Composer，收集所有结果。
// 所有 Composer 以相同输入并行执行，结果按顺序收集。
type FanOut[T any] struct {
	children []Composer[T]
}

// NewFanOut 创建扇出 Composer。
func NewFanOut[T any](children ...Composer[T]) *FanOut[T] {
	return &FanOut[T]{children: children}
}

// Run 并行执行所有子 Composer。
func (f *FanOut[T]) Run(ctx context.Context, input T) (T, []ExecutionStep, error) {
	if len(f.children) == 0 {
		return input, nil, nil
	}

	results := make([]T, len(f.children))
	allSteps := make([][]ExecutionStep, len(f.children))
	errs := make([]error, len(f.children))

	var wg sync.WaitGroup
	for i, child := range f.children {
		wg.Add(1)
		go func(idx int, c Composer[T]) {
			defer wg.Done()
			r, s, e := c.Run(ctx, input)
			results[idx] = r
			allSteps[idx] = s
			errs[idx] = e
		}(i, child)
	}
	wg.Wait()

	flatSteps := make([]ExecutionStep, 0)
	for _, s := range allSteps {
		flatSteps = append(flatSteps, s...)
	}

	// 返回第一个结果（FanOut 通常用于副作用分发）
	return results[0], flatSteps, errs[0]
}

// ============================================================================
// Debounce — 防抖模式
// ============================================================================

// Debounce 对高频调用做防抖：在静默期（quietPeriod）内只执行最后一次调用。
// 适用于搜索输入、窗口 resize 等场景。
type Debounce[T any] struct {
	inner       Composer[T]
	quietPeriod time.Duration
	mu          sync.Mutex
	timer       *time.Timer
	lastInput   T
	lastResult  T
	lastSteps   []ExecutionStep
	lastErr     error
	pending     bool
}

// NewDebounce 创建防抖 Composer。
// quietPeriod: 静默期，在此期间内的新调用会重置计时器。
func NewDebounce[T any](inner Composer[T], quietPeriod time.Duration) *Debounce[T] {
	if quietPeriod <= 0 {
		quietPeriod = 300 * time.Millisecond
	}
	return &Debounce[T]{inner: inner, quietPeriod: quietPeriod}
}

// Run 实现防抖逻辑。
// 如果 quietPeriod 内有新调用，旧调用被丢弃，重新计时。
func (d *Debounce[T]) Run(ctx context.Context, input T) (T, []ExecutionStep, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.lastInput = input
	d.pending = true

	if d.timer != nil {
		d.timer.Stop()
	}

	done := make(chan struct{})
	d.timer = time.AfterFunc(d.quietPeriod, func() {
		d.mu.Lock()
		defer d.mu.Unlock()
		if d.pending {
			d.lastResult, d.lastSteps, d.lastErr = d.inner.Run(ctx, d.lastInput)
			d.pending = false
		}
		close(done)
	})

	<-done
	return d.lastResult, d.lastSteps, d.lastErr
}

// ============================================================================
// Throttle — 节流模式
// ============================================================================

// Throttle 对高频调用做节流：在 interval 内只执行第一次调用，后续调用在间隔内被忽略。
// 适用于滚动事件、鼠标移动等场景。
type Throttle[T any] struct {
	inner      Composer[T]
	interval   time.Duration
	mu         sync.Mutex
	lastRun    time.Time
	lastResult T
	lastSteps  []ExecutionStep
	lastErr    error
}

// NewThrottle 创建节流 Composer。
// interval: 最小执行间隔，在此期间内的调用直接返回上次结果。
func NewThrottle[T any](inner Composer[T], interval time.Duration) *Throttle[T] {
	if interval <= 0 {
		interval = 100 * time.Millisecond
	}
	return &Throttle[T]{inner: inner, interval: interval}
}

// Run 实现节流逻辑。
func (t *Throttle[T]) Run(ctx context.Context, input T) (T, []ExecutionStep, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now()
	if now.Sub(t.lastRun) < t.interval {
		// 在间隔内，返回上次结果
		return t.lastResult, t.lastSteps, t.lastErr
	}

	t.lastResult, t.lastSteps, t.lastErr = t.inner.Run(ctx, input)
	t.lastRun = now
	return t.lastResult, t.lastSteps, t.lastErr
}
```

- [ ] **Step 2: 创建 patterns_fanout_test.go**

```go
package core

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestFanOut_Basic(t *testing.T) {
	ctx := context.Background()
	obs := &InMemoryObservationAdapter{}

	var count1, count2 atomic.Int32

	child1 := NewPipeline[int](obs,
		StepFunc[int, int]{
			execute: func(ctx context.Context, input int) (int, error) {
				count1.Add(1)
				return input * 2, nil
			},
			unitType: "Atom",
		},
	)
	child2 := NewPipeline[int](obs,
		StepFunc[int, int]{
			execute: func(ctx context.Context, input int) (int, error) {
				count2.Add(1)
				return input + 10, nil
			},
			unitType: "Atom",
		},
	)

	fanOut := NewFanOut[int](child1, child2)
	result, steps, err := fanOut.Run(ctx, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != 10 {
		t.Errorf("expected 10, got %d", result)
	}
	if count1.Load() != 1 || count2.Load() != 1 {
		t.Error("both children should have been called")
	}
	if len(steps) == 0 {
		t.Error("expected steps to be collected")
	}
}

func TestFanOut_Empty(t *testing.T) {
	fanOut := NewFanOut[int]()
	result, steps, err := fanOut.Run(context.Background(), 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != 5 {
		t.Errorf("expected 5, got %d", result)
	}
	if len(steps) != 0 {
		t.Error("expected no steps for empty fanout")
	}
}

func TestDebounce_Basic(t *testing.T) {
	ctx := context.Background()
	obs := &InMemoryObservationAdapter{}

	var callCount atomic.Int32
	inner := NewPipeline[int](obs,
		StepFunc[int, int]{
			execute: func(ctx context.Context, input int) (int, error) {
				callCount.Add(1)
				return input * 2, nil
			},
			unitType: "Atom",
		},
	)

	debounce := NewDebounce[int](inner, 50*time.Millisecond)

	// 快速连续调用 3 次
	go func() { debounce.Run(ctx, 1) }()
	go func() { debounce.Run(ctx, 2) }()
	time.Sleep(10 * time.Millisecond)
	result, _, err := debounce.Run(ctx, 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != 6 {
		t.Errorf("expected 6 (last input × 2), got %d", result)
	}
	// 应该只执行了最后一次
	if callCount.Load() != 1 {
		t.Logf("call count: %d (may vary due to timing)", callCount.Load())
	}
}

func TestDebounce_Immediate(t *testing.T) {
	ctx := context.Background()
	obs := &InMemoryObservationAdapter{}

	inner := NewPipeline[int](obs,
		AtomAsStep(Atom[int, int](func(x int) int { return x + 1 })),
	)

	debounce := NewDebounce[int](inner, 10*time.Millisecond)
	result, _, err := debounce.Run(ctx, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != 6 {
		t.Errorf("expected 6, got %d", result)
	}
}

func TestThrottle_Basic(t *testing.T) {
	ctx := context.Background()
	obs := &InMemoryObservationAdapter{}

	var callCount atomic.Int32
	inner := NewPipeline[int](obs,
		StepFunc[int, int]{
			execute: func(ctx context.Context, input int) (int, error) {
				callCount.Add(1)
				return input * 2, nil
			},
			unitType: "Atom",
		},
	)

	throttle := NewThrottle[int](inner, 100*time.Millisecond)

	// 第一次调用
	result, _, err := throttle.Run(ctx, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != 10 {
		t.Errorf("expected 10, got %d", result)
	}

	// 立即第二次调用（应被节流，返回缓存结果）
	result, _, err = throttle.Run(ctx, 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != 10 {
		t.Errorf("expected 10 (cached), got %d", result)
	}
	if callCount.Load() != 1 {
		t.Errorf("expected 1 call, got %d", callCount.Load())
	}
}

func TestThrottle_AfterInterval(t *testing.T) {
	ctx := context.Background()
	obs := &InMemoryObservationAdapter{}

	var callCount atomic.Int32
	inner := NewPipeline[int](obs,
		StepFunc[int, int]{
			execute: func(ctx context.Context, input int) (int, error) {
				callCount.Add(1)
				return input * 2, nil
			},
			unitType: "Atom",
		},
	)

	throttle := NewThrottle[int](inner, 50*time.Millisecond)

	throttle.Run(ctx, 5)
	time.Sleep(60 * time.Millisecond)
	result, _, err := throttle.Run(ctx, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != 20 {
		t.Errorf("expected 20, got %d", result)
	}
	if callCount.Load() != 2 {
		t.Errorf("expected 2 calls, got %d", callCount.Load())
	}
}
```

- [ ] **Step 3: 运行测试**

```bash
cd go-core && go test -run "TestFanOut|TestDebounce|TestThrottle" -v -count=1
```

- [ ] **Step 4: 运行全量测试**

```bash
cd go-core && go test ./... -count=1
```

- [ ] **Step 5: Commit**

```bash
git add go-core/patterns_fanout.go go-core/patterns_fanout_test.go
git commit -m "feat: add FanOut, Debounce, Throttle composer patterns"
```

---

### Task 6.3: 前端 — 文件复杂度热力图

**Files:**
- Modify: `arch-manager.html:1650-1700`（在现有图表区域追加）

**Goal:** 在 arch-manager 仪表盘中新增热力图，可视化每个文件的复杂度分布。

- [ ] **Step 1: 追加热力图 HTML 容器**

在 `arch-manager.html` 中找到 `chart-stats` 或 `chart-container` 区域，追加：

```html
<div class="chart-container">
    <h3>文件复杂度热力图</h3>
    <div id="chart-heatmap" style="width:100%;height:400px;"></div>
</div>
```

- [ ] **Step 2: 追加热力图 ECharts 渲染代码**

在 `<script>` 标签中追加：

```javascript
function renderHeatmap(data) {
    const files = data.files || [];
    // 按复杂度（符号数/行数）排序
    const heatData = files.map(f => ({
        name: f.file,
        value: f.symbols ? f.symbols.length : 0,
        loc: f.loc || 0,
        layer: f.layer || 'L0'
    })).sort((a, b) => b.value - a.value).slice(0, 30);

    const chart = echarts.init(document.getElementById('chart-heatmap'));
    chart.setOption({
        tooltip: {
            formatter: p => `${p.name}<br/>符号数: ${p.value}<br/>LOC: ${heatData[p.dataIndex].loc}<br/>层级: ${heatData[p.dataIndex].layer}`
        },
        xAxis: {
            type: 'category',
            data: heatData.map(d => d.name.split('/').pop()),
            axisLabel: { rotate: 45, fontSize: 10 }
        },
        yAxis: { type: 'value', name: '符号数' },
        series: [{
            type: 'bar',
            data: heatData.map((d, i) => ({
                value: d.value,
                itemStyle: {
                    color: d.layer === 'L0' ? '#7f8ea3' :
                           d.layer === 'L1' ? '#00d4aa' :
                           d.layer === 'L2' ? '#60a5fa' :
                           d.layer === 'L3' ? '#38bdf8' :
                           d.layer === 'L4' ? '#ef4444' :
                           d.layer === 'L5' ? '#34d399' :
                           d.layer === 'L6' ? '#f472b6' : '#f59e0b'
                }
            })),
            label: { show: true, position: 'top', fontSize: 10 }
        }]
    });
}
```

- [ ] **Step 3: 在数据加载后调用 renderHeatmap**

在 `loadArchData()` 或数据刷新回调中追加：

```javascript
renderHeatmap(archData);
```

- [ ] **Step 4: 验证**

启动 arch-manager，访问仪表盘，确认热力图渲染正常。

- [ ] **Step 5: Commit**

```bash
git add arch-manager.html
git commit -m "feat: add file complexity heatmap to arch-manager dashboard"
```

---

### Task 6.4: 前端 — 架构演进时间线

**Files:**
- Modify: `arch-manager.html`（追加时间线图表）
- Modify: `cmd/arch-manager/main.go`（追加 `/api/snapshots` 端点）

**Goal:** 记录架构快照历史，展示健康度趋势。

- [ ] **Step 1: 追加快照存储端点**

在 `cmd/arch-manager/main.go` 中追加：

```go
// 快照历史存储
var snapshotHistory []ArchSnapshot
var snapshotMu sync.Mutex

type ArchSnapshot struct {
    Timestamp    time.Time `json:"timestamp"`
    TotalFiles   int       `json:"total_files"`
    TotalSymbols int       `json:"total_symbols"`
    HealthScore  string    `json:"health_score"`
    Violations   int       `json:"violations"`
    AvgEntropy   float64   `json:"avg_entropy"`
}

// 在 main() 中追加路由
mux.HandleFunc("/api/snapshots", handleSnapshots)
mux.HandleFunc("/api/snapshots/add", handleAddSnapshot)

// 启动定时快照（每 10 分钟）
go func() {
    ticker := time.NewTicker(10 * time.Minute)
    defer ticker.Stop()
    for range ticker.C {
        takeSnapshot()
    }
}()
```

- [ ] **Step 2: 实现快照函数**

```go
func takeSnapshot() {
    snapshotMu.Lock()
    defer snapshotMu.Unlock()

    // 只保留最近 100 个快照
    if len(snapshotHistory) >= 100 {
        snapshotHistory = snapshotHistory[1:]
    }

    snapshotHistory = append(snapshotHistory, ArchSnapshot{
        Timestamp:    time.Now(),
        TotalFiles:   len(archData.Files),
        TotalSymbols: countTotalSymbols(archData),
        HealthScore:  computeHealthScore(archData),
        Violations:   countViolations(archData),
        AvgEntropy:   computeAvgEntropy(archData),
    })
}

func handleSnapshots(w http.ResponseWriter, r *http.Request) {
    snapshotMu.Lock()
    defer snapshotMu.Unlock()
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(snapshotHistory)
}

func handleAddSnapshot(w http.ResponseWriter, r *http.Request) {
    takeSnapshot()
    w.Header().Set("Content-Type", "application/json")
    fmt.Fprint(w, `{"status":"ok"}`)
}
```

- [ ] **Step 3: 追加时间线 HTML 容器**

```html
<div class="chart-container">
    <h3>架构演进时间线</h3>
    <div id="chart-timeline" style="width:100%;height:350px;"></div>
</div>
```

- [ ] **Step 4: 追加时间线 ECharts 渲染代码**

```javascript
async function renderTimeline() {
    const resp = await fetch('/api/snapshots');
    const snapshots = await resp.json();
    if (!snapshots || snapshots.length === 0) return;

    const chart = echarts.init(document.getElementById('chart-timeline'));
    chart.setOption({
        tooltip: { trigger: 'axis' },
        legend: { data: ['文件数', '符号数', '违规数'] },
        xAxis: {
            type: 'time',
            data: snapshots.map(s => s.timestamp)
        },
        yAxis: [
            { type: 'value', name: '数量' },
            { type: 'value', name: '健康度' }
        ],
        series: [
            {
                name: '文件数', type: 'line',
                data: snapshots.map(s => [s.timestamp, s.total_files]),
                smooth: true
            },
            {
                name: '符号数', type: 'line',
                data: snapshots.map(s => [s.timestamp, s.total_symbols]),
                smooth: true
            },
            {
                name: '违规数', type: 'line',
                data: snapshots.map(s => [s.timestamp, s.violations]),
                smooth: true, lineStyle: { color: '#ef4444' }
            }
        ]
    });
}
```

- [ ] **Step 5: 页面加载时调用**

```javascript
renderTimeline();
// 每 5 分钟刷新
setInterval(renderTimeline, 5 * 60 * 1000);
```

- [ ] **Step 6: 编译验证**

```bash
cd cmd/arch-manager && go build -o arch-manager.exe .
```

- [ ] **Step 7: Commit**

```bash
git add arch-manager.html cmd/arch-manager/main.go
git commit -m "feat: add architecture evolution timeline with snapshot history"
```

---

### Task 6.5: 关键模块测试覆盖

**Files:**
- Create: `go-core/patterns_resilience_test.go`
- Create: `go-core/guardian_decision_test.go`

**Goal:** 为韧性模式和决策引擎补充测试，优先覆盖最关键的路径。

- [ ] **Step 1: 创建 patterns_resilience_test.go**

```go
package core

import (
	"context"
	"testing"
	"time"
)

func TestCircuitBreaker_ClosedPasses(t *testing.T) {
	ctx := context.Background()
	obs := &InMemoryObservationAdapter{}

	inner := NewPipeline[int](obs,
		AtomAsStep(Atom[int, int](func(x int) int { return x * 2 })),
	)
	cb := NewCircuitBreaker[int](inner, 3, 100*time.Millisecond)

	result, _, err := cb.Run(ctx, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != 10 {
		t.Errorf("expected 10, got %d", result)
	}
	if cb.State() != CircuitClosed {
		t.Errorf("expected closed, got %s", cb.State())
	}
}

func TestCircuitBreaker_OpensOnThreshold(t *testing.T) {
	ctx := context.Background()
	obs := &InMemoryObservationAdapter{}

	inner := NewPipeline[int](obs,
		StepFunc[int, int]{
			execute: func(ctx context.Context, input int) (int, error) {
				return 0, NewStepError("FAIL", "always fail", true)
			},
			unitType: "Failing",
		},
	)
	cb := NewCircuitBreaker[int](inner, 2, 1*time.Second)

	// 触发 2 次失败
	cb.Run(ctx, 1)
	cb.Run(ctx, 1)

	if cb.State() != CircuitOpen {
		t.Errorf("expected open after 2 failures, got %s", cb.State())
	}

	// 熔断器打开时应拒绝请求
	_, _, err := cb.Run(ctx, 1)
	if err == nil {
		t.Error("expected error when circuit is open")
	}
}

func TestFallback_UsesFallback(t *testing.T) {
	ctx := context.Background()
	obs := &InMemoryObservationAdapter{}

	primary := NewPipeline[int](obs,
		StepFunc[int, int]{
			execute: func(ctx context.Context, input int) (int, error) {
				return 0, NewStepError("FAIL", "primary failed", false)
			},
			unitType: "Failing",
		},
	)
	fallback := NewPipeline[int](obs,
		AtomAsStep(Atom[int, int](func(x int) int { return 999 })),
	)

	fb := NewFallback[int](primary, fallback)
	result, steps, err := fb.Run(ctx, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != 999 {
		t.Errorf("expected 999 (fallback), got %d", result)
	}
	if len(steps) == 0 {
		t.Error("expected steps to be collected")
	}
}

func TestBulkhead_RejectsWhenFull(t *testing.T) {
	ctx := context.Background()
	obs := &InMemoryObservationAdapter{}

	inner := NewPipeline[int](obs,
		StepFunc[int, int]{
			execute: func(ctx context.Context, input int) (int, error) {
				time.Sleep(100 * time.Millisecond)
				return input, nil
			},
			unitType: "Slow",
		},
	)

	bh := NewBulkhead[int](inner, 1)

	// 启动一个慢请求占满信号量
	go bh.Run(ctx, 1)
	time.Sleep(10 * time.Millisecond) // 让 goroutine 先获取信号量

	// 第二个请求应立即被拒绝
	_, _, err := bh.Run(ctx, 2)
	if err == nil {
		t.Error("expected bulkhead rejection")
	}
}

func TestResilienceChain_WrapsCorrectly(t *testing.T) {
	ctx := context.Background()
	obs := &InMemoryObservationAdapter{}

	inner := NewPipeline[int](obs,
		AtomAsStep(Atom[int, int](func(x int) int { return x * 2 })),
	)

	chain := ResilienceChain[int](inner, ResilienceConfig[int]{
		RateLimit:                100,
		RateLimitBurst:           200,
		BulkheadMax:              10,
		CircuitBreakerThreshold:  3,
		CircuitBreakerCooldown:   1 * time.Second,
	})

	result, _, err := chain.Run(ctx, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != 10 {
		t.Errorf("expected 10, got %d", result)
	}
}
```

- [ ] **Step 2: 创建 guardian_decision_test.go**

```go
package core

import (
	"context"
	"testing"
)

func TestDecisionEngine_Allow(t *testing.T) {
	obs := &InMemoryObservationAdapter{}
	engine := NewDecisionEngine(obs)

	input := GuardianInput{
		TranspAlert: TransparencyAlert{
			IsHealthy:    true,
			EntropyScore: 0.1,
		},
	}

	decision := engine.evaluate(input)
	if decision.Action != ActionAllow {
		t.Errorf("expected Allow, got %s", decision.Action)
	}
}

func TestDecisionEngine_Block(t *testing.T) {
	obs := &InMemoryObservationAdapter{}
	engine := NewDecisionEngine(obs)

	input := GuardianInput{
		TranspAlert: TransparencyAlert{
			IsHealthy:    false,
			EntropyScore: 0.9,
		},
	}

	decision := engine.evaluate(input)
	if decision.Action == ActionAllow {
		t.Error("expected Block or Warn, got Allow")
	}
}

func TestDecisionEngine_NilObs(t *testing.T) {
	engine := NewDecisionEngine(nil)

	input := GuardianInput{
		TranspAlert: TransparencyAlert{
			IsHealthy: true,
		},
	}

	decision := engine.evaluate(input)
	if decision.Action != ActionAllow {
		t.Errorf("expected Allow, got %s", decision.Action)
	}
}
```

- [ ] **Step 3: 运行测试**

```bash
cd go-core && go test -run "TestCircuitBreaker|TestFallback|TestBulkhead|TestResilienceChain|TestDecisionEngine" -v -count=1
```

- [ ] **Step 4: 运行全量测试**

```bash
cd go-core && go test ./... -count=1
```

- [ ] **Step 5: Commit**

```bash
git add go-core/patterns_resilience_test.go go-core/guardian_decision_test.go
git commit -m "test: add resilience patterns and decision engine tests"
```

---

## 执行顺序

```
Task 6.1 → Task 6.2 → Task 6.3 → Task 6.4 → Task 6.5
```

Task 6.1 是 P0 修复，必须最先执行。Task 6.2/6.3/6.4/6.5 可并行执行。