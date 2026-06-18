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
//
// 使用 4 原语：
//   - Atom: 纯文本处理（提取、分析、摘要）
//   - Port: 输入验证契约
//   - Adapter: 文件存储
//   - Composer: Pipeline 编排
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

	core "go-core"
)

// ─── 数据模型 ───

// Document 表示一个待处理的文档。
type Document struct {
	Name       string         `json:"name"`
	Content    string         `json:"content"`
	WordCount  int            `json:"word_count"`
	Keywords   map[string]int `json:"keywords"`
	Summary    string         `json:"summary"`
	StoredPath string         `json:"stored_path"`
}

// ─── Atom: 纯文本处理 ───

// extractText 从文档内容中提取纯文本，统计词数。
func extractText(doc Document) Document {
	doc.Content = strings.TrimSpace(doc.Content)
	doc.WordCount = len(strings.Fields(doc.Content))
	return doc
}

// analyzeContent 分析文档内容，提取关键词。
func analyzeContent(doc Document) Document {
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
}

// generateSummary 生成文档摘要。
func generateSummary(doc Document) Document {
	if len(doc.Content) <= 100 {
		doc.Summary = doc.Content
	} else {
		doc.Summary = doc.Content[:100] + "..."
	}
	return doc
}

// ─── Port: 输入验证 ───

// docPort 验证文档输入。
type docPort struct{}

func (p *docPort) Validate(ctx context.Context, doc Document) (Document, error) {
	if doc.Content == "" {
		return doc, fmt.Errorf("document content is empty")
	}
	if doc.Name == "" {
		doc.Name = "untitled"
	}
	return doc, nil
}

// ─── Adapter: 文件存储 ───

// storeAdapter 将文档持久化到文件系统。
type storeAdapter struct {
	dir string
}

func (a *storeAdapter) Execute(ctx context.Context, doc Document) (Document, error) {
	data := fmt.Sprintf("Name: %s\nWords: %d\nSummary: %s\nKeywords: %v\n",
		doc.Name, doc.WordCount, doc.Summary, doc.Keywords)
	path := fmt.Sprintf("%s/%s.txt", a.dir, doc.Name)
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		return doc, fmt.Errorf("store failed: %w", err)
	}
	doc.StoredPath = path
	return doc, nil
}

// ─── Agent Workbench 设置 ───

var (
	obs       = &core.InMemoryObservationAdapter{}
	workbench *core.DefaultAgentWorkbench
	agentMu   sync.Mutex
	agentID   = "doc-processor-agent"
)

func init() {
	// 创建 Agent Workbench（带完整组件）
	staticGuard := core.NewStaticGuardPort(core.GlobalPrimitiveTypeSet, core.NewArchitectureGuard(), obs)
	decisionEngine := core.NewDecisionEngine(obs)
	runner := core.NewAgentRunner(os.TempDir()+"/doc-processor", obs)

	workbench = core.NewDefaultAgentWorkbench(obs, staticGuard, decisionEngine, runner)
}

// ─── HTTP 端点 ───

func main() {
	storeDir := os.TempDir() + "/doc-processor-output"
	os.MkdirAll(storeDir, 0755)

	// 构建文档处理 Pipeline
	docPipeline := core.NewPipeline[Document](obs,
		core.PortAsStep(&docPort{}),
		core.AtomAsStep(core.Atom[Document, Document](extractText)),
		core.AtomAsStep(core.Atom[Document, Document](analyzeContent)),
		core.AtomAsStep(core.Atom[Document, Document](generateSummary)),
		core.AdapterAsStep(&storeAdapter{dir: storeDir}),
	)

	// 文档处理端点
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

		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"name":"%s","word_count":%d,"summary":"%s","stored_path":"%s","steps":%d}`,
			result.Name, result.WordCount, result.Summary, result.StoredPath, len(steps))
	})

	// Agent 提交端点（模拟 AI Agent 写代码并提交）
	http.HandleFunc("/api/agent/submit", func(w http.ResponseWriter, r *http.Request) {
		agentMu.Lock()
		defer agentMu.Unlock()

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
		tree := obs.GetTraceTree()
		fmt.Fprintf(w, `{"steps":%d,"trace_roots":%d}`,
			obs.StepCount(), len(tree.Roots))
	})

	// 健康检查
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"status":"ok","service":"doc-processor","version":"1.0.0"}`)
	})

	log.Println("文档处理 Agent 服务启动在 :8084")
	log.Println("端点:")
	log.Println("  GET  /api/process?name=doc&content=...  处理文档")
	log.Println("  POST /api/agent/submit                   Agent 提交代码")
	log.Println("  GET  /api/observation/steps              观察步骤")
	log.Println("  GET  /health                             健康检查")
	log.Fatal(http.ListenAndServe(":8084", nil))
}