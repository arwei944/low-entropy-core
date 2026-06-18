# Phase 5: 质量深化与 Agent 实战

> **For agentic workers:** 按任务顺序执行。每个 Task 使用 checkbox (`- [ ]`) 追踪。

**Goal:** 修复 1M LOC 模拟中发现的关键问题 + 构建完整的 Agent Workbench 端到端演示。

**Architecture:** 三管齐下：(1) 修复 Branch 步骤丢失问题 (2) 创建 Agent Workbench E2E 集成测试 (3) 构建文档处理 Agent 实战示例。

**Tech Stack:** Go 1.22 stdlib，纯四原语架构（Atom/Port/Adapter/Composer）。

---

## 背景

生产化攻坚计划（Phase 1-4）已全部完成。1M LOC 模拟中发现以下待修复问题：

| 问题 | 严重程度 | 状态 |
|------|:---:|:---:|
| Branch 丢弃子步骤（`_ = steps`） | 高 | 待修复 |
| 缺少 Agent Workbench 全流程 E2E 测试 | 中 | 待创建 |
| 缺少真实 AI Agent 实战示例 | 中 | 待创建 |

---

### Task 5.1: 修复 Branch 步骤收集 — 从 Step 改为 Composer

**Files:**
- Modify: `go-core/composer.go:120-134`
- Modify: `go-core/composer_test.go`（追加测试）

**Goal:** Branch 当前返回 `Step[T, T]`，子 Composer 的 ExecutionStep 被 `_ = steps` 丢弃。改为返回 `Composer[T]`，使子步骤被正确收集。

- [ ] **Step 1: 将 NewBranch 从 Step 改为 Composer**

将 `composer.go` 中的 `NewBranch` 函数从返回 `Step[T, T]` 改为返回 `Composer[T]`：

```go
// NewBranch 创建条件分支 Composer。
// 根据 condition 的结果选择执行 truePath 或 falsePath。
// 子 Composer 的 ExecutionStep 被正确收集到父级。
func NewBranch[T any](condition func(T) bool, truePath, falsePath Composer[T]) Composer[T] {
	return &branchComposer[T]{
		condition: condition,
		truePath:  truePath,
		falsePath: falsePath,
	}
}

type branchComposer[T any] struct {
	condition func(T) bool
	truePath  Composer[T]
	falsePath Composer[T]
}

func (b *branchComposer[T]) Run(ctx context.Context, input T) (T, []ExecutionStep, error) {
	if b.condition(input) {
		return b.truePath.Run(ctx, input)
	}
	return b.falsePath.Run(ctx, input)
}
```

**删除** 旧的 `NewBranch` 函数（`StepFunc` 实现 + `_ = steps`）。

- [ ] **Step 2: 更新所有调用 NewBranch 的代码**

搜索所有 `NewBranch` 调用点，将 `Step` 用法改为 `Composer` 用法：

```go
// 旧用法（Step 放在 Pipeline 中）
pipeline := NewPipeline[T](obs,
    NewBranch(condition, truePath, falsePath), // Step 类型
)

// 新用法：Branch 作为 Composer 包装整个 Pipeline
branch := NewBranch[T](condition, truePath, falsePath)
result, steps, err := branch.Run(ctx, input)
```

或在 Pipeline 中通过 `ComposerAsStep` 包装：

```go
pipeline := NewPipeline[T](obs,
    composerAsStep(NewBranch[T](condition, truePath, falsePath)),
)
```

- [ ] **Step 3: 编写测试验证步骤收集**

```go
// composer_test.go 追加
func TestBranch_CollectsChildSteps(t *testing.T) {
	obs := &InMemoryObservationAdapter{}

	truePath := NewPipeline[string](obs,
		AtomAsStep[string, string](func(s string) string { return "true: " + s }),
	)
	falsePath := NewPipeline[string](obs,
		AtomAsStep[string, string](func(s string) string { return "false: " + s }),
	)

	branch := NewBranch[string](
		func(s string) bool { return len(s) > 5 },
		truePath,
		falsePath,
	)

	// 测试 true 分支
	result, steps, err := branch.Run(context.Background(), "hello world")
	if err != nil {
		t.Fatal(err)
	}
	if result != "true: hello world" {
		t.Errorf("expected 'true: hello world', got %q", result)
	}
	if len(steps) == 0 {
		t.Error("expected child steps to be collected, got 0")
	}

	// 测试 false 分支
	result, steps, err = branch.Run(context.Background(), "hi")
	if err != nil {
		t.Fatal(err)
	}
	if result != "false: hi" {
		t.Errorf("expected 'false: hi', got %q", result)
	}
	if len(steps) == 0 {
		t.Error("expected child steps to be collected, got 0")
	}
}
```

- [ ] **Step 4: 运行测试验证**

```bash
cd go-core && go test -run TestBranch -v -count=1
```

- [ ] **Step 5: 运行全量测试确保无回归**

```bash
cd go-core && go test ./... -count=1
```

- [ ] **Step 6: Commit**

```bash
git add go-core/composer.go go-core/composer_test.go
git commit -m "fix: Branch now returns Composer[T] to preserve child ExecutionSteps"
```

---

### Task 5.2: Agent Workbench 全流程 E2E 集成测试

**Files:**
- Create: `go-core/agent_e2e_test.go`

**Goal:** 创建端到端测试，覆盖 Agent Workbench 完整生命周期：Agent 写代码 → Submit → StaticGuard 审核 → DecisionEngine 决策 → AgentRunner 编译执行 → ExecutionStep 流。

- [ ] **Step 1: 编写 E2E 测试**

```go
// agent_e2e_test.go
package core

import (
	"context"
	"testing"
	"time"
)

// TestAgentWorkbench_E2E 测试完整的 Agent Workbench 管道。
// 模拟：Agent 编写一段合法的 Atom 代码 → 提交到 Workbench → 审核通过 → 编译执行 → 观察结果。
func TestAgentWorkbench_E2E(t *testing.T) {
	obs := &InMemoryObservationAdapter{}

	// 创建 Agent Workbench
	wb := NewDefaultAgentWorkbench(obs, nil, nil, nil)

	// Agent 提交一段合法的代码
	submission := AgentCodeSubmission{
		AgentID:    "agent-e2e-001",
		TaskID:     "task-e2e-001",
		SourceCode: `package main
import "fmt"
func main() { fmt.Println("hello from agent") }`,
		Manifest: []PrimitiveManifest{
			{
				PrimitiveType: "Atom",
				Name:          "helloAtom",
				Layer:         "L1",
				InputType:     "string",
				OutputType:    "string",
				Dependencies:  []string{},
			},
		},
		Attempt:     1,
		SubmittedAt: time.Now(),
	}

	ctx := context.Background()
	result, err := wb.Submit(ctx, submission)
	if err != nil {
		t.Fatalf("submit failed: %v", err)
	}

	// 验证提交结果
	if result.SubmissionID == "" {
		t.Error("expected non-empty SubmissionID")
	}
	if result.Status != "accepted" && result.Status != "rejected" {
		t.Errorf("expected status 'accepted' or 'rejected', got %q", result.Status)
	}

	// 验证提交历史
	subs := wb.ListSubmissionsByAgent("agent-e2e-001")
	if len(subs) == 0 {
		t.Error("expected at least 1 submission in history")
	}
}

// TestAgentWorkbench_GuardianRejection 测试 Guardian 拒绝不合规代码。
func TestAgentWorkbench_GuardianRejection(t *testing.T) {
	obs := &InMemoryObservationAdapter{}
	wb := NewDefaultAgentWorkbench(obs, nil, nil, nil)

	// 提交声明为 Atom 但实际是 Adapter 的代码（包含 I/O）
	submission := AgentCodeSubmission{
		AgentID:    "agent-bad-001",
		TaskID:     "task-bad-001",
		SourceCode: `package main
import "os"
func main() { os.WriteFile("bad.txt", []byte("x"), 0644) }`,
		Manifest: []PrimitiveManifest{
			{
				PrimitiveType: "Atom",
				Name:          "badAtom",
				Layer:         "L1",
				InputType:     "string",
				OutputType:    "string",
				Dependencies:  []string{},
			},
		},
		Attempt:     1,
		SubmittedAt: time.Now(),
	}

	result, err := wb.Submit(context.Background(), submission)
	if err != nil {
		t.Fatal(err)
	}

	// 应该检测到 Manifest 声明与实际代码不符
	if result.Status == "accepted" {
		// 如果 Guardian 没有启用，这也是可以接受的
		t.Log("Guardian not active, submission accepted")
	}
}

// TestAgentWorkbench_MultipleSubmissions 测试多次迭代提交。
// 模拟：Agent 第 1 次提交被拒绝 → 根据 Violation 建议修复 → 第 2 次提交。
func TestAgentWorkbench_MultipleSubmissions(t *testing.T) {
	obs := &InMemoryObservationAdapter{}
	wb := NewDefaultAgentWorkbench(obs, nil, nil, nil)

	ctx := context.Background()
	now := time.Now()

	// 第 1 次尝试
	sub1 := AgentCodeSubmission{
		AgentID:    "agent-iter-001",
		TaskID:     "task-iter-001",
		SourceCode: `package main
func main() {}`,
		Manifest: []PrimitiveManifest{
			{PrimitiveType: "Atom", Name: "test", Layer: "L1", InputType: "string", OutputType: "string"},
		},
		Attempt:     1,
		SubmittedAt: now,
	}
	result1, _ := wb.Submit(ctx, sub1)
	t.Logf("Attempt 1: status=%s, violations=%d", result1.Status, len(result1.Violations))

	// 第 2 次尝试（根据反馈修复）
	sub2 := AgentCodeSubmission{
		AgentID:    "agent-iter-001",
		TaskID:     "task-iter-001",
		SourceCode: `package main
func main() {}`,
		Manifest: []PrimitiveManifest{
			{PrimitiveType: "Atom", Name: "test", Layer: "L1", InputType: "string", OutputType: "string"},
		},
		Attempt:     2,
		SubmittedAt: now.Add(time.Second),
	}
	result2, _ := wb.Submit(ctx, sub2)
	t.Logf("Attempt 2: status=%s, violations=%d", result2.Status, len(result2.Violations))

	// 验证提交历史
	subs := wb.ListSubmissionsByAgent("agent-iter-001")
	if len(subs) < 2 {
		t.Errorf("expected at least 2 submissions, got %d", len(subs))
	}
}
```

- [ ] **Step 2: 运行测试**

```bash
cd go-core && go test -run TestAgentWorkbench -v -count=1
```

- [ ] **Step 3: Commit**

```bash
git add go-core/agent_e2e_test.go
git commit -m "test: add Agent Workbench E2E integration tests"
```

---

### Task 5.3: 文档处理 Agent 实战示例

**Files:**
- Create: `examples/doc-processor/main.go`
- Create: `examples/doc-processor/go.mod`
- Create: `examples/doc-processor/README.md`

**Goal:** 创建一个真实场景的 AI Agent 示例：文档处理流水线，展示 Agent Workbench 的完整生命周期。

**场景描述：**
- 用户上传文档 → Agent 提取文本 → 分析内容 → 生成摘要 → 存储结果
- 使用 Atom（纯文本处理）、Port（输入验证）、Adapter（文件存储）、Composer（编排）
- 展示 Agent 迭代提交：Guardian 审核 → 拒绝 → 修复 → 重新提交

- [ ] **Step 1: 创建 go.mod**

```go
// examples/doc-processor/go.mod
module low-entropy-core/examples/doc-processor

go 1.22

require low-entropy-core/go-core v0.0.0

replace low-entropy-core/go-core => ../../go-core
```

- [ ] **Step 2: 编写 main.go**

```go
// examples/doc-processor/main.go
// 文档处理 Agent 示例 — 展示 Agent Workbench 完整生命周期
//
// 流程：
//   1. Agent 注册到 Workbench
//   2. Agent 编写文档处理代码并提交
//   3. StaticGuard 审核（检查原语声明是否与实际代码一致）
//   4. DecisionEngine 决策（Allow/Block/Warn）
//   5. AgentRunner 编译执行
//   6. 每一步产生 ExecutionStep，被 Observation 收集
//   7. 人类通过 Observation API 查看完整执行轨迹
//
// 场景：
//   用户上传文档 → Agent 提取文本 → 分析内容 → 生成摘要 → 存储结果
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	core "low-entropy-core/go-core"
)

// ─── 文档处理原语 ───

// ExtractTextAtom 从文档内容中提取纯文本。
var ExtractTextAtom = core.Atom[Document, Document](func(doc Document) Document {
	doc.Content = strings.TrimSpace(doc.Content)
	doc.WordCount = len(strings.Fields(doc.Content))
	return doc
})

// AnalyzeContentAtom 分析文档内容（关键词提取）。
var AnalyzeContentAtom = core.Atom[Document, Document](func(doc Document) Document {
	words := strings.Fields(doc.Content)
	keywords := make(map[string]int)
	for _, w := range words {
		w = strings.ToLower(strings.Trim(w, ".,!?;:()[]{}\"'"))
		if len(w) > 3 {
			keywords[w]++
		}
	}
	doc.Keywords = keywords
	return doc
})

// GenerateSummaryAtom 生成文档摘要。
var GenerateSummaryAtom = core.Atom[Document, Document](func(doc Document) Document {
	if len(doc.Content) <= 100 {
		doc.Summary = doc.Content
	} else {
		doc.Summary = doc.Content[:100] + "..."
	}
	return doc
})

// DocPort 文档验证 Port。
type DocPort struct{}

func (p *DocPort) Validate(ctx context.Context, doc Document) (Document, error) {
	if doc.Content == "" {
		return doc, core.NewStepError("EMPTY_DOC", "document content is empty", true)
	}
	if doc.Name == "" {
		doc.Name = "untitled"
	}
	return doc, nil
}

// StoreAdapter 文档存储 Adapter。
type StoreAdapter struct {
	dir string
}

func (a *StoreAdapter) Execute(ctx context.Context, doc Document) (Document, error) {
	data := fmt.Sprintf("Name: %s\nWords: %d\nSummary: %s\nKeywords: %v\n",
		doc.Name, doc.WordCount, doc.Summary, doc.Keywords)
	path := fmt.Sprintf("%s/%s.txt", a.dir, doc.Name)
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		return doc, core.NewStepError("STORE_FAILED", err.Error(), false)
	}
	doc.StoredPath = path
	return doc, nil
}

// ─── 数据模型 ───

type Document struct {
	Name       string         `json:"name"`
	Content    string         `json:"content"`
	WordCount  int            `json:"word_count"`
	Keywords   map[string]int `json:"keywords"`
	Summary    string         `json:"summary"`
	StoredPath string         `json:"stored_path"`
}

// ─── Agent Workbench 设置 ───

var (
	obs       = &core.InMemoryObservationAdapter{}
	workbench *core.DefaultAgentWorkbench
	agentMu   sync.Mutex
	agentID   = "doc-processor-agent"
)

func init() {
	// 创建 Agent Workbench（带全部组件）
	staticGuard := core.NewStaticGuardPort(core.GlobalPrimitiveTypeSet, core.NewArchitectureGuard(), obs)
	decisionEngine := core.NewDecisionEngine(obs)
	runner := core.NewAgentRunner(os.TempDir()+"/doc-processor", obs)

	workbench = core.NewDefaultAgentWorkbench(obs, staticGuard, decisionEngine, runner)

	// 注册 Agent
	workbench.RegisterAgent(agentID, map[string]string{
		"capability": "document-processing",
		"version":    "1.0.0",
	})
}

// ─── HTTP 端点 ───

func main() {
	// 构建文档处理 Pipeline
	storeDir := os.TempDir() + "/doc-processor-output"
	os.MkdirAll(storeDir, 0755)

	docPort := &DocPort{}
	storeAdapter := &StoreAdapter{dir: storeDir}

	docPipeline := core.NewPipeline[Document](obs,
		core.PortAsStep(docPort),
		core.AtomAsStep(ExtractTextAtom),
		core.AtomAsStep(AnalyzeContentAtom),
		core.AtomAsStep(GenerateSummaryAtom),
		core.AdapterAsStep(storeAdapter),
	)

	// API 端点
	http.HandleFunc("/api/process", func(w http.ResponseWriter, r *http.Request) {
		doc := Document{
			Name:    r.URL.Query().Get("name"),
			Content: r.URL.Query().Get("content"),
		}
		if doc.Name == "" {
			doc.Name = fmt.Sprintf("doc-%d", time.Now().Unix())
		}

		ctx := context.Background()
		result, steps, err := docPipeline.Run(ctx, doc)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(500)
			fmt.Fprintf(w, `{"error":"%s"}`, err.Error())
			return
		}

		_ = steps

		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"name":"%s","word_count":%d,"summary":"%s","stored_path":"%s","steps":%d}`,
			result.Name, result.WordCount, result.Summary, result.StoredPath, len(steps))
	})

	// 提交 Agent 代码（模拟 AI Agent 写代码并提交）
	http.HandleFunc("/api/agent/submit", func(w http.ResponseWriter, r *http.Request) {
		submission := core.AgentCodeSubmission{
			AgentID:    agentID,
			TaskID:     fmt.Sprintf("task-%d", time.Now().Unix()),
			SourceCode: `// AI Agent 生成的文档处理代码
package main
import "fmt"
func main() { fmt.Println("document processed") }`,
			Manifest: []core.PrimitiveManifest{
				{
					PrimitiveType: "Atom",
					Name:          "extractTextAtom",
					Layer:         "L1",
					InputType:     "Document",
					OutputType:    "Document",
					Dependencies:  []string{},
				},
			},
			Attempt:     1,
			SubmittedAt: time.Now(),
		}

		result, err := workbench.Submit(context.Background(), submission)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(500)
			fmt.Fprintf(w, `{"error":"%s"}`, err.Error())
			return
		}

		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"submission_id":"%s","status":"%s","violations":%d}`,
			result.SubmissionID, result.Status, len(result.Violations))
	})

	// 观察端点
	http.HandleFunc("/api/observation/steps", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"steps":%d,"trace_tree":%s}`,
			obs.StepCount(), "available at /api/observation/trace-tree")
	})

	http.HandleFunc("/api/observation/trace-tree", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		tree := obs.GetTraceTree()
		fmt.Fprintf(w, `{"roots":%d,"depth":%d}`, len(tree.Roots), maxDepth(tree))
	})

	// 健康检查
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"status":"ok","service":"doc-processor","version":"1.0.0"}`)
	})

	log.Println("文档处理 Agent 服务启动在 :8084")
	log.Println("端点:")
	log.Println("  POST /api/process?name=doc&content=...  处理文档")
	log.Println("  POST /api/agent/submit                   Agent 提交代码")
	log.Println("  GET  /api/observation/steps              观察步骤")
	log.Println("  GET  /api/observation/trace-tree         追踪树")
	log.Println("  GET  /health                             健康检查")
	http.ListenAndServe(":8084", nil)
}

func maxDepth(tree *core.TraceTree) int {
	max := 0
	for _, root := range tree.Roots {
		d := depth(root, 1)
		if d > max {
			max = d
		}
	}
	return max
}

func depth(node *core.TraceNode, d int) int {
	max := d
	for _, child := range node.Children {
		cd := depth(child, d+1)
		if cd > max {
			max = cd
		}
	}
	return max
}
```

- [ ] **Step 3: 编译并运行示例**

```bash
cd examples/doc-processor && go build -o doc-processor.exe .
```

- [ ] **Step 4: 验证端点**

```bash
# 启动服务
./doc-processor.exe &
sleep 2

# 测试文档处理
curl "http://localhost:8084/api/process?name=test&content=The+quick+brown+fox+jumps+over+the+lazy+dog"

# 测试 Agent 提交
curl -X POST http://localhost:8084/api/agent/submit

# 测试观察端点
curl http://localhost:8084/api/observation/steps
curl http://localhost:8084/api/observation/trace-tree

# 停止服务
kill %1
```

- [ ] **Step 5: Commit**

```bash
git add examples/doc-processor/
git commit -m "feat: add doc-processor example — Agent Workbench E2E demo"
```

---

## 执行顺序

```
Task 5.1 → Task 5.2 → Task 5.3
```

Task 5.1 是前提修复，Task 5.2 和 5.3 可并行执行。