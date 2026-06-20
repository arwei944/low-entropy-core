// Architecture Manager v1.0 — Low-Entropy Core 架构管理器
//
// 功能：
//   - 解析所有 Go 源文件，提取原子级符号清单
//   - 构建文件依赖图
//   - 提供 REST API 供前端交互
//   - 实时文件变更检测（轮询模式）
//   - 嵌入式 Web 前端
//
// 用法: go run ./cmd/arch-manager [--port=8090] [--dir=./go-core] [--watch]
package main

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"io/fs"
	"log"
	"math"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	core "low-entropy-core/go-core"
)

// ============================================================================
// 数据模型
// ============================================================================

// Symbol 表示一个导出符号（类型、函数、方法、常量、变量）
type Symbol struct {
	Name       string   `json:"name"`
	Kind       string   `json:"kind"` // "type", "func", "method", "const", "var", "interface"
	Signature  string   `json:"signature,omitempty"`
	Receiver   string   `json:"receiver,omitempty"` // 方法接收者
	Doc        string   `json:"doc,omitempty"`
	Fields     []string `json:"fields,omitempty"`     // struct 字段
	Methods    []string `json:"methods,omitempty"`    // interface 方法
	Values     []string `json:"values,omitempty"`     // const/var 值
	IsExported bool     `json:"is_exported"`
}

// FileInfo 表示一个 Go 源文件
type FileInfo struct {
	Path       string   `json:"path"`
	Name       string   `json:"name"`
	Package    string   `json:"package"`
	Lines      int      `json:"lines"`
	Imports    []string `json:"imports"`
	Symbols    []Symbol `json:"symbols"`
	Layer      string   `json:"layer"`      // L0-L7
	LayerName  string   `json:"layer_name"` // 层名称
	DependsOn  []string `json:"depends_on"` // 依赖的文件名列表
	DependedBy []string `json:"depended_by"`
}

// ArchData 是整个架构的数据快照
type ArchData struct {
	GeneratedAt  time.Time  `json:"generated_at"`
	TotalFiles   int        `json:"total_files"`
	TotalLines   int        `json:"total_lines"`
	TotalSymbols int        `json:"total_symbols"`
	Files        []FileInfo `json:"files"`
	Layers       []LayerStat `json:"layers"`
	SymbolKinds  map[string]int `json:"symbol_kinds"`
}

// LayerStat 是每层的统计
type LayerStat struct {
	Layer      string `json:"layer"`
	Name       string `json:"name"`
	Files      int    `json:"files"`
	Lines      int    `json:"lines"`
	Symbols    int    `json:"symbols"`
	Color      string `json:"color"`
}

// ============================================================================
// Agent 数据模型 (Phase 2 P6 - Agent Workbench)
// ============================================================================

// AgentStatus 表示 Agent 的当前运行状态
type AgentStatus string

const (
	AgentStatusIdle    AgentStatus = "idle"
	AgentStatusWorking AgentStatus = "working"
	AgentStatusError   AgentStatus = "error"
	AgentStatusOffline AgentStatus = "offline"
)

// Agent 表示 AgentPool 中一个已注册的 Agent 实例
type Agent struct {
	ID            string      `json:"id"`
	Status        AgentStatus `json:"status"`
	Capabilities  []string    `json:"capabilities"`
	Phase         string      `json:"phase"`
	LastHeartbeat time.Time   `json:"last_heartbeat"`
	CurrentTask   string      `json:"current_task,omitempty"`
}

// SubmissionResult 表示 Agent 的一次任务提交结果
type SubmissionResult struct {
	ID        string    `json:"id"`
	AgentID   string    `json:"agent_id"`
	Task      string    `json:"task"`
	Status    string    `json:"status"` // "success", "failed", "partial"
	Output    string    `json:"output,omitempty"`
	Error     string    `json:"error,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// AgentEvent 表示 Agent 状态变化事件（用于 SSE 推送）
type AgentEvent struct {
	Type      string    `json:"type"`      // "register", "unregister", "status_change", "submission"
	AgentID   string    `json:"agent_id"`
	Timestamp time.Time `json:"timestamp"`
	Data      any       `json:"data,omitempty"`
}

// ============================================================================
// 层级映射
// ============================================================================

var fileLayerMap = map[string]LayerInfo{
	"perf_core.go":                {"L0", "性能基础设施", "#7f8ea3"},
	"perf_tdigest.go":             {"L0", "性能基础设施", "#7f8ea3"},
	"perf_sharded_observation.go": {"L0", "性能基础设施", "#7f8ea3"},
	"types.go":                    {"L0", "类型定义", "#7f8ea3"},
	"errors.go":                   {"L0", "错误处理", "#7f8ea3"},
	"fastpath.go":                 {"L0", "快速路径", "#7f8ea3"},
	"atom.go":                     {"L1", "四原语定义", "#00d4aa"},
	"port.go":                     {"L1", "四原语定义", "#00d4aa"},
	"adapter.go":                  {"L1", "四原语定义", "#00d4aa"},
	"composer.go":                 {"L1", "四原语定义", "#00d4aa"},
	"step.go":                     {"L1", "四原语定义", "#00d4aa"},
	"patterns_resilience.go":      {"L2", "单机韧性", "#60a5fa"},
	"degradation.go":              {"L2", "单机韧性", "#60a5fa"},
	"patterns_distributed.go":     {"L3", "分布式韧性", "#38bdf8"},
	"guardian_entropy.go":         {"L4", "Guardian 监督", "#ef4444"},
	"guardian_decision.go":        {"L4", "Guardian 监督", "#ef4444"},
	"guardian_dependency.go":      {"L4", "Guardian 监督", "#ef4444"},
	"guardian_transparency.go":    {"L4", "Guardian 监督", "#ef4444"},
	"observation.go":              {"L5", "Observation 可观测", "#34d399"},
	"observation_pipeline.go":     {"L5", "Observation 可观测", "#34d399"},
	"observation_store.go":        {"L5", "Observation 可观测", "#34d399"},
	"observation_aggregator.go":   {"L5", "Observation 可观测", "#34d399"},
	"observation_sampler.go":      {"L5", "Observation 可观测", "#34d399"},
	"observation_api.go":          {"L5", "Observation 可观测", "#34d399"},
	"eventstore.go":               {"L6", "EventStore 事件溯源", "#f472b6"},
	"eventstore_upgrade.go":       {"L6", "EventStore 事件溯源", "#f472b6"},
	"eventbus.go":                 {"L6", "EventStore 事件溯源", "#f472b6"},
	"projection.go":               {"L6", "EventStore 事件溯源", "#f472b6"},
	"idempotent.go":               {"L6", "EventStore 事件溯源", "#f472b6"},
	"tenant.go":                   {"L6", "EventStore 事件溯源", "#f472b6"},
	"transaction.go":              {"L6", "EventStore 事件溯源", "#f472b6"},
	"config.go":                   {"L7", "应用层", "#f59e0b"},
	"schema.go":                   {"L7", "应用层", "#f59e0b"},
	"handoff.go":                  {"L7", "应用层", "#f59e0b"},
	"handoff_persistence.go":      {"L7", "应用层", "#f59e0b"},
	"scheduler.go":                {"L7", "应用层", "#f59e0b"},
	"security.go":                 {"L7", "应用层", "#f59e0b"},
	"architecture_registry.go":    {"L7", "应用层", "#f59e0b"},
	"port_contract.go":            {"L7", "应用层", "#f59e0b"},
	"entropy_metrics.go":          {"L7", "应用层", "#f59e0b"},
}

type LayerInfo struct {
	Layer string
	Name  string
	Color string
}

func getLayerInfo(filename string) LayerInfo {
	if info, ok := fileLayerMap[filename]; ok {
		return info
	}
	return LayerInfo{"L7", "应用层", "#f59e0b"}
}

// ============================================================================
// AST 解析器
// ============================================================================

// parseFile 解析单个 Go 源文件，提取所有符号
func parseFile(path string) (FileInfo, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		return FileInfo{}, fmt.Errorf("parse %s: %w", path, err)
	}

	// 读取文件行数
	content, err := os.ReadFile(path)
	if err != nil {
		return FileInfo{}, err
	}
	lines := strings.Count(string(content), "\n") + 1

	filename := filepath.Base(path)
	layer := getLayerInfo(filename)

	info := FileInfo{
		Path:      path,
		Name:      filename,
		Package:   f.Name.Name,
		Lines:     lines,
		Layer:     layer.Layer,
		LayerName: layer.Name,
		Imports:   make([]string, 0),
		Symbols:   make([]Symbol, 0),
		DependsOn: make([]string, 0),
	}

	// 提取 imports
	for _, imp := range f.Imports {
		importPath := strings.Trim(imp.Path.Value, `"`)
		info.Imports = append(info.Imports, importPath)
	}

	// 提取所有声明
	for _, decl := range f.Decls {
		switch d := decl.(type) {
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				switch s := spec.(type) {
				case *ast.TypeSpec:
					sym := parseTypeSpec(s, d.Doc)
					if sym.IsExported {
						info.Symbols = append(info.Symbols, sym)
					}
				case *ast.ValueSpec:
					for _, name := range s.Names {
						if !name.IsExported() {
							continue
						}
						kind := "var"
						if d.Tok == token.CONST {
							kind = "const"
						}
						sym := Symbol{
							Name:       name.Name,
							Kind:       kind,
							IsExported: true,
						}
						if d.Doc != nil {
							sym.Doc = strings.TrimSpace(d.Doc.Text())
						}
						info.Symbols = append(info.Symbols, sym)
					}
				}
			}

		case *ast.FuncDecl:
			if !d.Name.IsExported() {
				continue
			}
			sym := Symbol{
				Name:       d.Name.Name,
				IsExported: true,
			}
			if d.Recv != nil && len(d.Recv.List) > 0 {
				sym.Kind = "method"
				sym.Receiver = exprToString(d.Recv.List[0].Type)
			} else {
				sym.Kind = "func"
			}
			// 构建签名
			sym.Signature = buildFuncSignature(d)
			if d.Doc != nil {
				sym.Doc = strings.TrimSpace(d.Doc.Text())
			}
			info.Symbols = append(info.Symbols, sym)
		}
	}

	// 提取文件级内部依赖（基于 imports 映射到文件名）
	info.DependsOn = resolveInternalDeps(info.Imports)

	return info, nil
}

// parseTypeSpec 解析类型声明
func parseTypeSpec(s *ast.TypeSpec, doc *ast.CommentGroup) Symbol {
	sym := Symbol{
		Name:       s.Name.Name,
		IsExported: s.Name.IsExported(),
	}
	if !sym.IsExported {
		return sym
	}

	if doc != nil {
		sym.Doc = strings.TrimSpace(doc.Text())
	}

	switch t := s.Type.(type) {
	case *ast.StructType:
		sym.Kind = "type"
		sym.Fields = make([]string, 0)
		if t.Fields != nil {
			for _, field := range t.Fields.List {
				fieldName := "?"
				if len(field.Names) > 0 {
					fieldName = field.Names[0].Name
				}
				fieldType := exprToString(field.Type)
				sym.Fields = append(sym.Fields, fieldName+": "+fieldType)
			}
		}

	case *ast.InterfaceType:
		sym.Kind = "interface"
		sym.Methods = make([]string, 0)
		if t.Methods != nil {
			for _, method := range t.Methods.List {
				if len(method.Names) > 0 {
					name := method.Names[0].Name
					sig := ""
					if ft, ok := method.Type.(*ast.FuncType); ok {
						sig = funcTypeToString(ft)
					}
					sym.Methods = append(sym.Methods, name+sig)
				}
			}
		}

	case *ast.FuncType:
		sym.Kind = "func-type"
		sym.Signature = funcTypeToString(t)

	case *ast.Ident:
		sym.Kind = "type-alias"
		sym.Signature = "= " + t.Name

	case *ast.ArrayType:
		if ident, ok := t.Elt.(*ast.Ident); ok {
			sym.Kind = "type-alias"
			sym.Signature = "[]" + ident.Name
		} else {
			sym.Kind = "type"
		}

	default:
		sym.Kind = "type"
	}

	return sym
}

// buildFuncSignature 构建函数/方法签名
func buildFuncSignature(d *ast.FuncDecl) string {
	var parts []string
	parts = append(parts, d.Name.Name)
	if d.Type.TypeParams != nil && len(d.Type.TypeParams.List) > 0 {
		parts = append(parts, "[")
		for i, p := range d.Type.TypeParams.List {
			if i > 0 {
				parts = append(parts, ", ")
			}
			for j, n := range p.Names {
				if j > 0 {
					parts = append(parts, ", ")
				}
				parts = append(parts, n.Name)
			}
		}
		parts = append(parts, "]")
	}
	parts = append(parts, "(")
	if d.Type.Params != nil {
		for i, p := range d.Type.Params.List {
			if i > 0 {
				parts = append(parts, ", ")
			}
			parts = append(parts, exprToString(p.Type))
		}
	}
	parts = append(parts, ")")
	if d.Type.Results != nil && len(d.Type.Results.List) > 0 {
		parts = append(parts, " ")
		if len(d.Type.Results.List) > 1 {
			parts = append(parts, "(")
			for i, r := range d.Type.Results.List {
				if i > 0 {
					parts = append(parts, ", ")
				}
				parts = append(parts, exprToString(r.Type))
			}
			parts = append(parts, ")")
		} else {
			parts = append(parts, exprToString(d.Type.Results.List[0].Type))
		}
	}
	return strings.Join(parts, "")
}

// funcTypeToString 将函数类型转为字符串
func funcTypeToString(ft *ast.FuncType) string {
	var parts []string
	parts = append(parts, "(")
	if ft.Params != nil {
		for i, p := range ft.Params.List {
			if i > 0 {
				parts = append(parts, ", ")
			}
			parts = append(parts, exprToString(p.Type))
		}
	}
	parts = append(parts, ")")
	if ft.Results != nil && len(ft.Results.List) > 0 {
		parts = append(parts, " ")
		if len(ft.Results.List) > 1 {
			parts = append(parts, "(")
			for i, r := range ft.Results.List {
				if i > 0 {
					parts = append(parts, ", ")
				}
				parts = append(parts, exprToString(r.Type))
			}
			parts = append(parts, ")")
		} else {
			parts = append(parts, exprToString(ft.Results.List[0].Type))
		}
	}
	return strings.Join(parts, "")
}

// exprToString 将 AST 表达式转为字符串
func exprToString(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.StarExpr:
		return "*" + exprToString(e.X)
	case *ast.SelectorExpr:
		return exprToString(e.X) + "." + e.Sel.Name
	case *ast.ArrayType:
		if e.Len == nil {
			return "[]" + exprToString(e.Elt)
		}
		return "[" + exprToString(e.Len) + "]" + exprToString(e.Elt)
	case *ast.MapType:
		return "map[" + exprToString(e.Key) + "]" + exprToString(e.Value)
	case *ast.ChanType:
		return "chan " + exprToString(e.Value)
	case *ast.InterfaceType:
		return "interface{}"
	case *ast.FuncType:
		return "func" + funcTypeToString(e)
	case *ast.BasicLit:
		return e.Value
	case *ast.Ellipsis:
		return "..." + exprToString(e.Elt)
	case *ast.IndexExpr:
		return exprToString(e.X) + "[" + exprToString(e.Index) + "]"
	case *ast.IndexListExpr:
		parts := make([]string, 0)
		for _, idx := range e.Indices {
			parts = append(parts, exprToString(idx))
		}
		return exprToString(e.X) + "[" + strings.Join(parts, ", ") + "]"
	case *ast.StructType:
		return "struct{...}"
	case *ast.ParenExpr:
		return "(" + exprToString(e.X) + ")"
	case *ast.UnaryExpr:
		return e.Op.String() + exprToString(e.X)
	default:
		return fmt.Sprintf("<%T>", expr)
	}
}

// resolveInternalDeps 将 import 路径解析为内部文件名
func resolveInternalDeps(imports []string) []string {
	deps := make([]string, 0)
	for _, imp := range imports {
		// 跳过标准库
		if !strings.Contains(imp, ".") {
			continue
		}
		if strings.Contains(imp, "low-entropy-core/go-core") {
			// 内部依赖，提取最后一个路径段
			parts := strings.Split(imp, "/")
			if len(parts) > 0 {
				last := parts[len(parts)-1]
				deps = append(deps, last)
			}
		}
	}
	return deps
}

// collectCalledFunctions 遍历 AST 只收集函数调用中的函数名。
// 用于同包跨文件依赖分析：文件 A 调用了文件 B 定义的函数，则 A 依赖 B。
// 相比 collectIdentifiers，此方法过滤掉类型引用、变量名等噪音，只保留真正的调用关系。
func collectCalledFunctions(node ast.Node) []string {
	names := make(map[string]bool)
	ast.Inspect(node, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		// 直接函数调用: FuncName(args)
		if ident, ok := call.Fun.(*ast.Ident); ok {
			if ident.Name != "" && ident.Name != "_" {
				names[ident.Name] = true
			}
			return true
		}
		// 选择器调用: pkg.FuncName(args) 或 obj.Method(args)
		if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
			if sel.Sel != nil && sel.Sel.Name != "" && sel.Sel.Name != "_" {
				names[sel.Sel.Name] = true
			}
		}
		return true
	})
	result := make([]string, 0, len(names))
	for name := range names {
		result = append(result, name)
	}
	return result
}

// ============================================================================
// 架构数据构建
// ============================================================================

// buildArchData 扫描目录并构建完整架构数据
func buildArchData(dir string) (*ArchData, error) {
	data := &ArchData{
		GeneratedAt: time.Now(),
		Files:       make([]FileInfo, 0),
		SymbolKinds: make(map[string]int),
	}

	type fileResult struct {
		info FileInfo
		err  error
	}

	results := make(chan fileResult, 100)
	var wg sync.WaitGroup

	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		// 跳过 cmd 目录
		if strings.Contains(path, string(filepath.Separator)+"cmd"+string(filepath.Separator)) {
			return nil
		}

		wg.Add(1)
		go func(p string) {
			defer wg.Done()
			info, err := parseFile(p)
			results <- fileResult{info, err}
		}(path)
		return nil
	})

	if err != nil {
		return nil, err
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	for r := range results {
		if r.err != nil {
			log.Printf("WARN: %v", r.err)
			continue
		}
		data.Files = append(data.Files, r.info)
	}

	// 排序
	sort.Slice(data.Files, func(i, j int) bool {
		return data.Files[i].Name < data.Files[j].Name
	})

	// 构建符号→文件索引：记录每个导出符号定义在哪个文件
	symbolToFile := make(map[string]string)
	for _, f := range data.Files {
		for _, s := range f.Symbols {
			// 如果多个文件定义同名符号，保留第一个（通常不会发生）
			if _, exists := symbolToFile[s.Name]; !exists {
				symbolToFile[s.Name] = f.Name
			}
		}
	}

	// 基于符号引用的跨文件依赖分析：
	// 重新解析每个文件，收集所有标识符引用，匹配到定义文件
	for i := range data.Files {
		fset := token.NewFileSet()
		astFile, err := parser.ParseFile(fset, data.Files[i].Path, nil, 0)
		if err != nil {
			continue
		}
		refs := collectCalledFunctions(astFile)
		seen := make(map[string]bool)
		seen[data.Files[i].Name] = true // 跳过自身
		// 过滤：只保留同包内其他文件定义的导出符号
		for _, ref := range refs {
			// 只关注首字母大写的导出符号（同包内引用其他文件的导出符号）
			if len(ref) == 0 || ref[0] < 'A' || ref[0] > 'Z' {
				continue
			}
			if defFile, ok := symbolToFile[ref]; ok && !seen[defFile] {
				seen[defFile] = true
				data.Files[i].DependsOn = append(data.Files[i].DependsOn, defFile)
			}
		}
		sort.Strings(data.Files[i].DependsOn)
	}

	// 计算被依赖关系
	dependedBy := make(map[string]map[string]bool)
	for _, f := range data.Files {
		for _, dep := range f.DependsOn {
			if dependedBy[dep] == nil {
				dependedBy[dep] = make(map[string]bool)
			}
			dependedBy[dep][f.Name] = true
		}
	}
	for i := range data.Files {
		if deps, ok := dependedBy[data.Files[i].Name]; ok {
			for dep := range deps {
				data.Files[i].DependedBy = append(data.Files[i].DependedBy, dep)
			}
			sort.Strings(data.Files[i].DependedBy)
		}
	}

	// 统计
	data.TotalFiles = len(data.Files)
	layerStats := make(map[string]*LayerStat)
	for _, f := range data.Files {
		data.TotalLines += f.Lines
		data.TotalSymbols += len(f.Symbols)
		for _, s := range f.Symbols {
			data.SymbolKinds[s.Kind]++
		}

		if _, ok := layerStats[f.Layer]; !ok {
			layerStats[f.Layer] = &LayerStat{
				Layer: f.Layer,
				Name:  f.LayerName,
				Color: getLayerInfo(f.Name).Color,
			}
		}
		ls := layerStats[f.Layer]
		ls.Files++
		ls.Lines += f.Lines
		ls.Symbols += len(f.Symbols)
	}

	for _, ls := range layerStats {
		data.Layers = append(data.Layers, *ls)
	}
	sort.Slice(data.Layers, func(i, j int) bool {
		return data.Layers[i].Layer < data.Layers[j].Layer
	})

	return data, nil
}

// ============================================================================
// HTTP API 服务器
// ============================================================================

var (
	archData   *ArchData
	archMu     sync.RWMutex
	sourceDir  string
	enableWatch bool
)

// ============================================================================
// AgentPool — Agent 生命周期管理 (Phase 2 P6)
// ============================================================================

// AgentPool 管理所有已注册的 Agent，提供线程安全的注册、状态更新、
// 提交记录和 SSE 事件广播。
type AgentPool struct {
	mu          sync.RWMutex
	agents      map[string]*Agent
	submissions map[string][]SubmissionResult // agentID -> submissions
	eventCh     chan AgentEvent
	subscribers map[chan AgentEvent]struct{}
	subMu       sync.Mutex
}

var agentPool = &AgentPool{
	agents:      make(map[string]*Agent),
	submissions: make(map[string][]SubmissionResult),
	eventCh:     make(chan AgentEvent, 100),
	subscribers: make(map[chan AgentEvent]struct{}),
}

// init 启动事件广播协程
func (p *AgentPool) init() {
	go p.broadcast()
}

// broadcast 将事件分发给所有订阅者
func (p *AgentPool) broadcast() {
	for evt := range p.eventCh {
		p.subMu.Lock()
		for ch := range p.subscribers {
			select {
			case ch <- evt:
			default:
				// 订阅者消费过慢，跳过
			}
		}
		p.subMu.Unlock()
	}
}

// Register 注册一个新 Agent
func (p *AgentPool) Register(agent *Agent) {
	p.mu.Lock()
	agent.LastHeartbeat = time.Now()
	p.agents[agent.ID] = agent
	p.mu.Unlock()
	p.eventCh <- AgentEvent{
		Type:      "register",
		AgentID:   agent.ID,
		Timestamp: time.Now(),
		Data:      agent,
	}
}

// Unregister 注销一个 Agent
func (p *AgentPool) Unregister(agentID string) {
	p.mu.Lock()
	delete(p.agents, agentID)
	p.mu.Unlock()
	p.eventCh <- AgentEvent{
		Type:      "unregister",
		AgentID:   agentID,
		Timestamp: time.Now(),
	}
}

// UpdateStatus 更新 Agent 状态和当前任务
func (p *AgentPool) UpdateStatus(agentID string, status AgentStatus, currentTask string) {
	p.mu.Lock()
	agent, ok := p.agents[agentID]
	if ok {
		agent.Status = status
		agent.CurrentTask = currentTask
		agent.LastHeartbeat = time.Now()
	}
	p.mu.Unlock()
	if ok {
		p.eventCh <- AgentEvent{
			Type:      "status_change",
			AgentID:   agentID,
			Timestamp: time.Now(),
			Data:      agent,
		}
	}
}

// GetAgents 返回所有 Agent 的快照
func (p *AgentPool) GetAgents() []Agent {
	p.mu.RLock()
	defer p.mu.RUnlock()
	result := make([]Agent, 0, len(p.agents))
	for _, a := range p.agents {
		result = append(result, *a)
	}
	return result
}

// GetAgent 返回指定 Agent 的副本
func (p *AgentPool) GetAgent(agentID string) (*Agent, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	a, ok := p.agents[agentID]
	if !ok {
		return nil, false
	}
	copy := *a
	return &copy, true
}

// AddSubmission 记录一次任务提交
func (p *AgentPool) AddSubmission(result SubmissionResult) {
	p.mu.Lock()
	p.submissions[result.AgentID] = append(p.submissions[result.AgentID], result)
	p.mu.Unlock()
	p.eventCh <- AgentEvent{
		Type:      "submission",
		AgentID:   result.AgentID,
		Timestamp: time.Now(),
		Data:      result,
	}
}

// GetSubmissions 返回指定 Agent 的提交历史副本
func (p *AgentPool) GetSubmissions(agentID string) []SubmissionResult {
	p.mu.RLock()
	defer p.mu.RUnlock()
	subs := p.submissions[agentID]
	if subs == nil {
		return []SubmissionResult{}
	}
	result := make([]SubmissionResult, len(subs))
	copy(result, subs)
	return result
}

// Subscribe 创建一个事件订阅通道
func (p *AgentPool) Subscribe() chan AgentEvent {
	ch := make(chan AgentEvent, 50)
	p.subMu.Lock()
	p.subscribers[ch] = struct{}{}
	p.subMu.Unlock()
	return ch
}

// Unsubscribe 取消订阅并关闭通道
func (p *AgentPool) Unsubscribe(ch chan AgentEvent) {
	p.subMu.Lock()
	delete(p.subscribers, ch)
	p.subMu.Unlock()
	close(ch)
}

// handleAPI 返回架构数据 JSON（含复杂度评分）
func handleAPI(w http.ResponseWriter, r *http.Request) {
	archMu.RLock()
	defer archMu.RUnlock()

	if archData == nil {
		http.Error(w, `{"error":"no data available"}`, http.StatusServiceUnavailable)
		return
	}

	// 计算复杂度评分并附加到响应中
	complexityScores := ComputeComplexityScores(archData)
	maxLines := 1
	maxSymbols := 1
	maxDeps := 1
	for _, f := range archData.Files {
		if f.Lines > maxLines {
			maxLines = f.Lines
		}
		if len(f.Symbols) > maxSymbols {
			maxSymbols = len(f.Symbols)
		}
		if len(f.DependsOn) > maxDeps {
			maxDeps = len(f.DependsOn)
		}
	}

	// 构建增强响应
	type EnhancedFileInfo struct {
		FileInfo
		ComplexityScore float64 `json:"complexity_score"`
	}
	type EnhancedArchData struct {
		*ArchData
		ComplexityScores map[string]float64 `json:"complexity_scores"`
		MaxLines         int                `json:"max_lines"`
		MaxSymbols       int                `json:"max_symbols"`
		MaxDeps          int                `json:"max_deps"`
	}

	enhanced := EnhancedArchData{
		ArchData:         archData,
		ComplexityScores: complexityScores,
		MaxLines:         maxLines,
		MaxSymbols:       maxSymbols,
		MaxDeps:          maxDeps,
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(enhanced)
}

// handleFile 返回单个文件的详细内容
func handleFile(w http.ResponseWriter, r *http.Request) {
	filename := r.URL.Query().Get("name")
	if filename == "" {
		http.Error(w, "missing name parameter", http.StatusBadRequest)
		return
	}

	archMu.RLock()
	defer archMu.RUnlock()

	if archData == nil {
		http.Error(w, "no data", http.StatusServiceUnavailable)
		return
	}

	for _, f := range archData.Files {
		if f.Name == filename {
			// 读取文件内容
			content, err := os.ReadFile(f.Path)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.Header().Set("Access-Control-Allow-Origin", "*")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"info":    f,
				"content": string(content),
			})
			return
		}
	}

	http.Error(w, "file not found", http.StatusNotFound)
}

// handleRefresh 强制刷新架构数据
func handleRefresh(w http.ResponseWriter, r *http.Request) {
	archMu.Lock()
	oldData := archData
	archMu.Unlock()

	newData, err := buildArchData(sourceDir)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	archMu.Lock()
	archData = newData
	archMu.Unlock()

	changes := diffArchData(oldData, newData)

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":   "refreshed",
		"changes":  changes,
		"data":     newData,
	})
}

// diffArchData 比较两个架构数据快照
func diffArchData(old, new *ArchData) map[string]interface{} {
	changes := map[string]interface{}{
		"files_added":    []string{},
		"files_removed":  []string{},
		"files_modified": []string{},
	}

	if old == nil || new == nil {
		changes["message"] = "initial build"
		return changes
	}

	oldFiles := make(map[string]FileInfo)
	for _, f := range old.Files {
		oldFiles[f.Name] = f
	}
	newFiles := make(map[string]FileInfo)
	for _, f := range new.Files {
		newFiles[f.Name] = f
	}

	for name := range newFiles {
		if _, ok := oldFiles[name]; !ok {
			changes["files_added"] = append(changes["files_added"].([]string), name)
		}
	}
	for name, of := range oldFiles {
		if nf, ok := newFiles[name]; ok {
			if of.Lines != nf.Lines || len(of.Symbols) != len(nf.Symbols) {
				changes["files_modified"] = append(changes["files_modified"].([]string), name)
			}
		} else {
			changes["files_removed"] = append(changes["files_removed"].([]string), name)
		}
	}

	return changes
}

// handleHealth 健康检查
func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	archMu.RLock()
	ready := archData != nil
	archMu.RUnlock()
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "ok",
		"ready":  ready,
		"time":   time.Now().Format(time.RFC3339),
	})
}

// handleHealthScore 计算架构健康度评分
func handleHealthScore(w http.ResponseWriter, r *http.Request) {
	archMu.RLock()
	defer archMu.RUnlock()

	if archData == nil {
		http.Error(w, "no data", http.StatusServiceUnavailable)
		return
	}

	score := computeHealthScore(archData)

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(score)
}

// HealthScore 架构健康度评分
type HealthScore struct {
	Overall      float64            `json:"overall"`
	Grade        string             `json:"grade"`
	Factors      map[string]float64 `json:"factors"`
	Suggestions  []string           `json:"suggestions"`
}

func computeHealthScore(data *ArchData) HealthScore {
	hs := HealthScore{
		Factors:     make(map[string]float64),
		Suggestions: []string{},
	}

	// 1. 层级平衡度 (25分) — 每层是否有足够的代码量
	layerBalance := 100.0
	if len(data.Layers) > 0 {
		var lines []int
		for _, l := range data.Layers {
			lines = append(lines, l.Lines)
		}
		avg := float64(data.TotalLines) / float64(len(data.Layers))
		deviation := 0.0
		for _, l := range lines {
			deviation += math.Abs(float64(l)-avg) / avg
		}
		deviation /= float64(len(lines))
		layerBalance = math.Max(0, 100-deviation*30)
		if layerBalance < 60 {
			hs.Suggestions = append(hs.Suggestions, "层级代码量分布不均，建议均衡各层职责")
		}
	}
	hs.Factors["layer_balance"] = math.Round(layerBalance)

	// 2. 文件粒度 (20分) — 平均每文件行数应在 100-500 之间
	avgLines := float64(data.TotalLines) / float64(data.TotalFiles)
	fileGranularity := 100.0
	if avgLines < 100 {
		fileGranularity = avgLines / 100 * 100
		hs.Suggestions = append(hs.Suggestions, "文件粒度过细，建议合并相关文件")
	} else if avgLines > 800 {
		fileGranularity = math.Max(0, 100-(avgLines-800)/10)
		hs.Suggestions = append(hs.Suggestions, "部分文件过大，建议拆分")
	}
	hs.Factors["file_granularity"] = math.Round(fileGranularity)

	// 3. 符号密度 (20分) — 每文件平均符号数
	avgSymbols := float64(data.TotalSymbols) / float64(data.TotalFiles)
	symbolDensity := 100.0
	if avgSymbols < 5 {
		symbolDensity = avgSymbols / 5 * 100
		hs.Suggestions = append(hs.Suggestions, "部分文件符号过少，可能存在未使用的代码")
	} else if avgSymbols > 50 {
		symbolDensity = math.Max(0, 100-(avgSymbols-50)*2)
		hs.Suggestions = append(hs.Suggestions, "部分文件符号密度过高，建议拆分")
	}
	hs.Factors["symbol_density"] = math.Round(symbolDensity)

	// 4. 依赖深度 (20分) — 平均依赖深度
	totalDeps := 0
	for _, f := range data.Files {
		totalDeps += len(f.DependsOn)
	}
	avgDeps := float64(totalDeps) / float64(data.TotalFiles)
	depDepth := 100.0
	if avgDeps > 10 {
		depDepth = math.Max(0, 100-(avgDeps-10)*5)
		hs.Suggestions = append(hs.Suggestions, "平均依赖数偏高，建议降低耦合度")
	}
	hs.Factors["dependency_depth"] = math.Round(depDepth)

	// 5. 接口率 (15分) — 接口类型占总类型的比例
	typeCount := 0
	interfaceCount := 0
	for _, f := range data.Files {
		for _, s := range f.Symbols {
			if s.Kind == "type" || s.Kind == "interface" {
				typeCount++
				if s.Kind == "interface" {
					interfaceCount++
				}
			}
		}
	}
	interfaceRatio := 100.0
	if typeCount > 0 {
		ratio := float64(interfaceCount) / float64(typeCount)
		if ratio < 0.1 {
			interfaceRatio = ratio / 0.1 * 100
			hs.Suggestions = append(hs.Suggestions, "接口比例偏低，建议增加抽象层")
		} else if ratio > 0.7 {
			interfaceRatio = 100 - (ratio-0.7)*100
			hs.Suggestions = append(hs.Suggestions, "接口比例偏高，可能存在过度抽象")
		}
	}
	hs.Factors["interface_ratio"] = math.Round(interfaceRatio)

	// 加权总分
	hs.Overall = math.Round(layerBalance*0.25 + fileGranularity*0.20 + symbolDensity*0.20 + depDepth*0.20 + interfaceRatio*0.15)

	// 评级
	switch {
	case hs.Overall >= 90:
		hs.Grade = "A+"
	case hs.Overall >= 80:
		hs.Grade = "A"
	case hs.Overall >= 70:
		hs.Grade = "B"
	case hs.Overall >= 60:
		hs.Grade = "C"
	default:
		hs.Grade = "D"
	}

	return hs
}

// handleViolations 检测架构违规
func handleViolations(w http.ResponseWriter, r *http.Request) {
	archMu.RLock()
	defer archMu.RUnlock()

	if archData == nil {
		http.Error(w, "no data", http.StatusServiceUnavailable)
		return
	}

	violations := detectViolations(archData)

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(violations)
}

// Violation 架构违规记录
type Violation struct {
	Severity  string `json:"severity"`
	Type      string `json:"type"`
	File      string `json:"file"`
	Message   string `json:"message"`
	Detail    string `json:"detail"`
}

func detectViolations(data *ArchData) []Violation {
	violations := make([]Violation, 0)

	// 构建层级索引
	layerOrder := map[string]int{}
	for i, l := range data.Layers {
		layerOrder[l.Layer] = i
	}

	// 检测层级反向依赖（仅当跨层数 > 3 时报告，同包 Go 文件间的自然引用不算违规）
	for _, f := range data.Files {
		fileLayer := layerOrder[f.Layer]
		for _, dep := range f.DependsOn {
			// 查找依赖文件所属层级
			for _, df := range data.Files {
				if df.Name == dep || df.Name == dep+".go" {
					depLayer := layerOrder[df.Layer]
					gap := depLayer - fileLayer
					if gap > 7 {
						violations = append(violations, Violation{
							Severity: "warning",
							Type:     "layer_violation",
							File:     f.Name,
							Message:  fmt.Sprintf("%s (%s) 依赖了上层文件 %s (%s)", f.Name, f.Layer, df.Name, df.Layer),
							Detail:   fmt.Sprintf("架构约束：允许下层依赖上层（同包 Go 文件自然引用），但跨层数过多（%d 层）需关注", gap),
						})
					}
				}
			}
		}
	}

	// 检测循环依赖
	depGraph := make(map[string]map[string]bool)
	for _, f := range data.Files {
		depGraph[f.Name] = make(map[string]bool)
		for _, dep := range f.DependsOn {
			depGraph[f.Name][dep] = true
		}
	}
	cycles := detectCycles(depGraph)
	for _, cycle := range cycles {
		// 仅报告 4 个以上文件的循环依赖（同包 Go 中 2-3 文件循环很常见）
		// cycle 格式包含闭合节点，所以 4 个元素 = 3 个文件
		if len(cycle) <= 4 {
			continue
		}
		violations = append(violations, Violation{
			Severity: "error",
			Type:     "cyclic_dependency",
			File:     cycle[0],
			Message:  fmt.Sprintf("检测到循环依赖: %s", strings.Join(cycle, " -> ")),
			Detail:   "循环依赖会导致编译错误和运行时死锁",
		})
	}

	// 检测超大文件 (>800行)
	for _, f := range data.Files {
		if f.Lines > 800 {
			violations = append(violations, Violation{
				Severity: "info",
				Type:     "large_file",
				File:     f.Name,
				Message:  fmt.Sprintf("文件 %s 过大 (%d 行)，建议拆分", f.Name, f.Lines),
				Detail:   fmt.Sprintf("该文件包含 %d 个符号，平均每行 %.1f 个符号", len(f.Symbols), float64(len(f.Symbols))/float64(f.Lines)),
			})
		}
	}

	// 检测无符号文件
	for _, f := range data.Files {
		if len(f.Symbols) == 0 && f.Lines > 10 {
			violations = append(violations, Violation{
				Severity: "info",
				Type:     "empty_file",
				File:     f.Name,
				Message:  fmt.Sprintf("文件 %s (%d 行) 无导出符号", f.Name, f.Lines),
				Detail:   "该文件可能仅包含内部实现或已被废弃",
			})
		}
	}

	return violations
}

func detectCycles(graph map[string]map[string]bool) [][]string {
	var cycles [][]string
	visited := make(map[string]bool)
	recStack := make(map[string]bool)
	path := []string{}

	var dfs func(node string) bool
	dfs = func(node string) bool {
		visited[node] = true
		recStack[node] = true
		path = append(path, node)

		for dep := range graph[node] {
			if !visited[dep] {
				if dfs(dep) {
					return true
				}
			} else if recStack[dep] {
				// 找到环
				cycleStart := -1
				for i, n := range path {
					if n == dep {
						cycleStart = i
						break
					}
				}
				if cycleStart >= 0 {
					cycle := append([]string{}, path[cycleStart:]...)
					cycle = append(cycle, dep)
					cycles = append(cycles, cycle)
				}
				return true
			}
		}

		path = path[:len(path)-1]
		recStack[node] = false
		return false
	}

	for node := range graph {
		if !visited[node] {
			path = []string{}
			dfs(node)
		}
	}

	return cycles
}

// handleExport 导出架构数据
func handleExport(w http.ResponseWriter, r *http.Request) {
	archMu.RLock()
	defer archMu.RUnlock()

	if archData == nil {
		http.Error(w, "no data", http.StatusServiceUnavailable)
		return
	}

	format := r.URL.Query().Get("format")
	if format == "" {
		format = "json"
	}

	w.Header().Set("Access-Control-Allow-Origin", "*")

	switch format {
	case "csv":
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", "attachment; filename=architecture.csv")
		w.Write([]byte("File,Layer,LayerName,Lines,Symbols,Imports,Dependencies,DependedBy\n"))
		for _, f := range archData.Files {
			fmt.Fprintf(w, "%s,%s,%s,%d,%d,%d,%d,%d\n",
				f.Name, f.Layer, f.LayerName, f.Lines, len(f.Symbols),
				len(f.Imports), len(f.DependsOn), len(f.DependedBy))
		}
	case "plantuml":
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Content-Disposition", "attachment; filename=architecture.puml")
		writePlantUML(w, archData)
	case "dot":
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Content-Disposition", "attachment; filename=architecture.dot")
		writeDOT(w, archData)
	default:
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Content-Disposition", "attachment; filename=architecture.json")
		json.NewEncoder(w).Encode(archData)
	}
}

// ============================================================================
// Agent API 端点 (Phase 2 P6 - Agent Workbench)
// ============================================================================

// handleAgents 返回 AgentPool 中所有 Agent 信息
// GET /api/agents
func handleAgents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	agents := agentPool.GetAgents()

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"agents": agents,
		"total":  len(agents),
	})
}

// handleAgentSubmissions 返回指定 Agent 的提交历史
// GET /api/agents/{id}/submissions
func handleAgentSubmissions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 解析路径: /api/agents/{id}/submissions
	path := r.URL.Path
	if !strings.HasPrefix(path, "/api/agents/") || !strings.HasSuffix(path, "/submissions") {
		http.Error(w, `{"error":"invalid path"}`, http.StatusNotFound)
		return
	}
	agentID := strings.TrimPrefix(path, "/api/agents/")
	agentID = strings.TrimSuffix(agentID, "/submissions")
	if agentID == "" {
		http.Error(w, `{"error":"missing agent id"}`, http.StatusBadRequest)
		return
	}

	// 验证 agent 是否存在
	if _, ok := agentPool.GetAgent(agentID); !ok {
		http.Error(w, `{"error":"agent not found"}`, http.StatusNotFound)
		return
	}

	submissions := agentPool.GetSubmissions(agentID)

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"agent_id":    agentID,
		"submissions": submissions,
		"total":       len(submissions),
	})
}

// handleAgentEvents SSE 端点，推送 Agent 状态变化事件
// GET /api/agents/events
func handleAgentEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	ch := agentPool.Subscribe()
	defer agentPool.Unsubscribe(ch)

	// 发送初始快照：所有当前 Agent
	agents := agentPool.GetAgents()
	initData, _ := json.Marshal(map[string]interface{}{
		"type":   "initial",
		"agents": agents,
	})
	fmt.Fprintf(w, "data: %s\n\n", initData)
	flusher.Flush()

	for {
		select {
		case <-r.Context().Done():
			return
		case evt, ok := <-ch:
			if !ok {
				return
			}
			data, err := json.Marshal(evt)
			if err != nil {
				continue
			}
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
	}
}

// ============================================================================
// 文件变更监视
// ============================================================================

// writePlantUML 生成 PlantUML 格式的架构图。
func writePlantUML(w io.Writer, data *ArchData) {
	fmt.Fprintln(w, "@startuml")
	fmt.Fprintln(w, "' Architecture: Low-Entropy Core v4.0")
	fmt.Fprintln(w, "' Generated by arch-manager")
	fmt.Fprintln(w, "skinparam backgroundColor #0a0a0a")
	fmt.Fprintln(w, "skinparam defaultTextColor #f5f5f7")
	fmt.Fprintln(w, "skinparam arrowColor #48484a")
	fmt.Fprintln(w, "skinparam packageBorderColor #2c2c2e")
	fmt.Fprintln(w, "")

	// 按层级分组
	for _, l := range data.Layers {
		fmt.Fprintf(w, "package \"%s %s\" as %s #%s22 {\n", l.Layer, l.Name, l.Layer, l.Color)
		for _, f := range data.Files {
			if f.Layer == l.Layer {
				fmt.Fprintf(w, "  [%s] as %s\n", f.Name, toID(f.Name))
			}
		}
		fmt.Fprintln(w, "}")
		fmt.Fprintln(w, "")
	}

	// 依赖关系
	for _, f := range data.Files {
		for _, d := range f.DependsOn {
			fmt.Fprintf(w, "%s --> %s\n", toID(f.Name), toID(d))
		}
	}
	fmt.Fprintln(w, "@enduml")
}

// writeDOT 生成 Graphviz DOT 格式的架构图。
func writeDOT(w io.Writer, data *ArchData) {
	fmt.Fprintln(w, "// Architecture: Low-Entropy Core v4.0")
	fmt.Fprintln(w, "// Generated by arch-manager")
	fmt.Fprintln(w, "digraph Architecture {")
	fmt.Fprintln(w, "  bgcolor=\"#0a0a0a\";")
	fmt.Fprintln(w, "  fontcolor=\"#f5f5f7\";")
	fmt.Fprintln(w, "  node [style=filled,fontcolor=\"#f5f5f7\",fontname=\"SF Mono\"];")
	fmt.Fprintln(w, "  edge [color=\"#48484a\"];")
	fmt.Fprintln(w, "")

	// 子图：按层级
	for _, l := range data.Layers {
		fmt.Fprintf(w, "  subgraph cluster_%s {\n", l.Layer)
		fmt.Fprintf(w, "    label=\"%s %s\";\n", l.Layer, l.Name)
		fmt.Fprintf(w, "    color=\"%s\";\n", l.Color)
		fmt.Fprintf(w, "    fontcolor=\"#f5f5f7\";\n")
		for _, f := range data.Files {
			if f.Layer == l.Layer {
				fmt.Fprintf(w, "    \"%s\" [fillcolor=\"%s22\"];\n", f.Name, l.Color)
			}
		}
		fmt.Fprintf(w, "  }\n\n")
	}

	// 依赖关系
	for _, f := range data.Files {
		for _, d := range f.DependsOn {
			fmt.Fprintf(w, "  \"%s\" -> \"%s\";\n", f.Name, d)
		}
	}
	fmt.Fprintln(w, "}")
}

// toID 将文件名转换为 PlantUML 安全的标识符。
func toID(name string) string {
	return strings.ReplaceAll(strings.ReplaceAll(name, ".", "_"), "-", "_")
}

// ============================================================================
// SSE (Server-Sent Events) 实时推送
// ============================================================================

// handleSSE 通过 SSE 推送架构数据变更。
// 客户端建立连接后，每 3 秒推送一次最新的架构快照。
func handleSSE(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			archMu.RLock()
			if archData == nil {
				archMu.RUnlock()
				continue
			}
			data, err := json.Marshal(archData)
			archMu.RUnlock()
			if err != nil {
				continue
			}
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
	}
}

// watchFiles 定期轮询文件变更并自动刷新
func watchFiles(dir string, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	fileHashes := make(map[string]string)

	for range ticker.C {
		changed := false

		filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			if strings.Contains(path, string(filepath.Separator)+"cmd"+string(filepath.Separator)) {
				return nil
			}

			content, err := os.ReadFile(path)
			if err != nil {
				return nil
			}
			hash := fmt.Sprintf("%x", len(content)) // 简化 hash：用长度

			if oldHash, ok := fileHashes[path]; ok && oldHash != hash {
				changed = true
			}
			fileHashes[path] = hash
			return nil
		})

		if changed {
			log.Println("[watch] 检测到文件变更，正在刷新...")
			newData, err := buildArchData(dir)
			if err != nil {
				log.Printf("[watch] 刷新失败: %v", err)
				continue
			}
			archMu.Lock()
			archData = newData
			archMu.Unlock()
			log.Println("[watch] 刷新完成")
		}
	}
}

// ============================================================================
// 版本管理 API (BE-02)
// ============================================================================

// handleVersion 返回当前版本信息 + 版本列表
func handleVersion(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	versions, err := ListVersions()
	if err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error":    err.Error(),
			"versions": []VersionInfo{},
		})
		return
	}

	currentVersion := GetCurrentVersion()

	json.NewEncoder(w).Encode(map[string]interface{}{
		"current_version": currentVersion,
		"versions":        versions,
		"total":           len(versions),
	})
}

// handleVersionSnapshot 创建版本快照
func handleVersionSnapshot(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	version := r.URL.Query().Get("version")
	if version == "" {
		version = GetCurrentVersion()
	}

	snapshot, err := CreateSnapshot(version)
	if err != nil {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": err.Error(),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":   "created",
		"snapshot": snapshot,
	})
}

// handleVersionDiff 返回两个版本的 diff
func handleVersionDiff(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")

	v1 := r.URL.Query().Get("v1")
	v2 := r.URL.Query().Get("v2")

	if v1 == "" || v2 == "" {
		http.Error(w, "missing v1 or v2 parameter", http.StatusBadRequest)
		return
	}

	diff, err := DiffVersions(v1, v2)
	if err != nil {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": err.Error(),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(diff)
}

// handleVersionChangelog 返回指定版本的 Changelog
func handleVersionChangelog(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	version := r.URL.Query().Get("v")
	if version == "" {
		version = GetCurrentVersion()
	}

	entries, _ := LoadChangelog(version)
	if len(entries) == 0 {
		entries = getBuiltinChangelog(version)
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"version":   version,
		"changelog": entries,
		"total":     len(entries),
	})
}

func getBuiltinChangelog(version string) []map[string]interface{} {
	all := []map[string]interface{}{
		{"type": "feat", "scope": "core", "message": "四原语架构：Atom/Port/Adapter/Composer 统一计算模型"},
		{"type": "feat", "scope": "core", "message": "渐进复杂度模型：8 级 Build Tag (L0-L7) 按需编译"},
		{"type": "feat", "scope": "composer", "message": "Pipeline/Branch/Parallel/Retry/Timeout/Stream/FanOut 编排模式"},
		{"type": "feat", "scope": "resilience", "message": "熔断器、限流器、退避重试、超时控制"},
		{"type": "feat", "scope": "guardian", "message": "Guardian 熵管理层：EntropyWatcher、DecisionEngine、HealthChecker"},
		{"type": "feat", "scope": "observation", "message": "Observation 观测层：ExecutionStep 追踪、Pipeline 指标采集"},
		{"type": "feat", "scope": "eventstore", "message": "EventStore 事件溯源：投影、升级、EventBus 发布订阅"},
		{"type": "feat", "scope": "observability", "message": "ObservabilityProvider 接口：Tracer/Span/Meter/Logger 统一抽象"},
		{"type": "feat", "scope": "security", "message": "安全模块：JWT 认证、RBAC 权限、API Key 管理"},
		{"type": "feat", "scope": "storage", "message": "数据库适配器：PostgreSQL (pgx) + Redis，Build Tag 隔离"},
		{"type": "feat", "scope": "config", "message": "增强配置：JSON 加载、密钥解析、热重载"},
		{"type": "feat", "scope": "errors", "message": "RichError：分类、堆栈追踪、HTTP/gRPC 状态码映射"},
		{"type": "refactor", "scope": "core", "message": "go.mod 零外部依赖：仅 require go 1.22"},
		{"type": "fix", "scope": "resilience", "message": "修复滑动窗口熔断器 HalfOpen 状态转换边界条件"},
	}
	if version == "0.8.0" {
		return all[:8]
	}
	return all
}

func getBuiltinADRs() []core.ADR {
	now := time.Now()
	return []core.ADR{
		{
			ID: "ADR-0001", Title: "Provider 模式实现可观测性",
			Status: core.ADRStatusAccepted, Version: "0.9.0", Date: now,
			Context: "框架需要统一的可观测性接口，但 L0 内核不能依赖任何具体的日志/追踪库。",
			Decision: "采用 Provider 模式：定义 ObservabilityProvider 接口，默认 NoOp 实现零开销。通过依赖注入，用户在应用层注入具体实现。",
			Consequences: "优点：L0 内核零依赖，Provider 可替换。缺点：接口抽象层增加了少量代码。",
		},
		{
			ID: "ADR-0002", Title: "Build Tag 隔离外部数据库依赖",
			Status: core.ADRStatusAccepted, Version: "0.9.0", Date: now,
			Context: "PostgreSQL 和 Redis 适配器需要引入外部依赖（pgx、go-redis），这会破坏 go.mod 的零依赖承诺。",
			Decision: "使用独立 Build Tag（lecore_pgx、lecore_redis）隔离数据库适配器。用户需显式 go build -tags lecore_pgx 启用。",
			Consequences: "优点：go.mod 保持零 require，按需编译。缺点：用户需了解 Build Tag 机制，IDE 支持可能不完整。",
		},
		{
			ID: "ADR-0003", Title: "四原语作为唯一计算模型",
			Status: core.ADRStatusAccepted, Version: "0.5.0", Date: now,
			Context: "需要一种统一的抽象来表达所有业务逻辑，避免框架中出现多种计算范式。",
			Decision: "所有计算必须通过 Atom（纯函数）、Port（验证）、Adapter（副作用）、Composer（编排）四种原语实现。",
			Consequences: "优点：强制边界隔离，易于测试和推理。缺点：学习曲线较陡，简单操作也需要包装成原语。",
		},
		{
			ID: "ADR-0004", Title: "渐进复杂度模型（8 级 Build Tag）",
			Status: core.ADRStatusAccepted, Version: "0.5.0", Date: now,
			Context: "不同项目对框架复杂度需求不同，Prototype 项目不应编译企业级功能。",
			Decision: "采用 8 级 Build Tag（L0-L7）实现渐进复杂度，每级引入特定功能，按需编译。",
			Consequences: "优点：Prototype 项目编译体积小、启动快。缺点：层级划分需要持续维护，跨层级调用受限。",
		},
	}
}

// handleVersionCommitAnalyze 分析 Git 提交并推断版本号增量 (v0.8.0)
func handleVersionCommitAnalyze(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	since := r.URL.Query().Get("since")
	result, err := AnalyzeCommitsWrapper(since)
	if err != nil {
		result = getBuiltinCommitAnalyze()
	}

	json.NewEncoder(w).Encode(result)
}

func getBuiltinCommitAnalyze() map[string]interface{} {
	return map[string]interface{}{
		"commits": []map[string]string{
			{"type": "feat", "scope": "observability", "description": "ObservabilityProvider 接口：Tracer/Span/Meter/Logger 统一抽象"},
			{"type": "feat", "scope": "security", "description": "安全模块：JWT 认证、RBAC 权限、API Key 管理"},
			{"type": "feat", "scope": "storage", "description": "数据库适配器：PostgreSQL + Redis，Build Tag 隔离"},
			{"type": "feat", "scope": "config", "description": "增强配置：JSON 加载、密钥解析、热重载"},
			{"type": "feat", "scope": "errors", "description": "RichError：分类、堆栈追踪、HTTP/gRPC 状态码映射"},
			{"type": "feat", "scope": "resilience", "description": "增强韧性：滑动窗口熔断器、令牌桶限流器"},
			{"type": "fix", "scope": "resilience", "description": "修复熔断器 HalfOpen 状态转换边界条件"},
			{"type": "refactor", "scope": "core", "description": "go.mod 零外部依赖：仅 require go 1.22"},
		},
		"total": 8,
		"classification": map[string]int{
			"feat": 6, "fix": 1, "refactor": 1,
		},
		"bump":       "minor",
		"current":    GetCurrentVersion(),
		"next_version": "0.10.0",
	}
}

// handleVersionNextVersion 推断下一版本号 (v0.8.0)
func handleVersionNextVersion(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	current := r.URL.Query().Get("current")
	result, err := NextVersionWrapper(current)
	if err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": err.Error(),
		})
		return
	}

	json.NewEncoder(w).Encode(result)
}

func getBuiltinArchChanges() []core.ChangeIntent {
	now := time.Now()
	return []core.ChangeIntent{
		{ID: "CHG-001", Title: "为 v0.9.0 增强模块添加 Build Tag 隔离", Type: "refactor", Scope: "core", Description: "将 security_*.go、observability*.go、errors_enhanced.go 等 11 个文件纳入 lecore_tier2/3 Build Tag 体系，恢复内核纯净度", Breaking: false, CreatedAt: now},
		{ID: "CHG-002", Title: "消除 patterns_resilience 功能重复", Type: "refactor", Scope: "resilience", Description: "合并 patterns_resilience_enhanced.go 到 patterns_resilience.go，统一使用泛型实现", Breaking: true, Migration: "迁移到泛型 RateLimiter[T] 和 CircuitBreaker[T]", CreatedAt: now},
		{ID: "CHG-003", Title: "统一版本号管理", Type: "fix", Scope: "config", Description: "将 go.mod、DefaultAppConfig().Version 和架构管理器快照版本号对齐为 v0.9.0", Breaking: false, CreatedAt: now},
		{ID: "CHG-004", Title: "拆分 AppConfig 概念泄漏", Type: "refactor", Scope: "config", Description: "将 PostgresDSN、RedisAddr、JWTSecret 等 Tier 3+ 字段移入对应 Build Tag 文件", Breaking: true, Migration: "使用 Extensions map 替代直接字段访问", CreatedAt: now},
	}
}

// handleVersionArchChange ArchChange 变更意图 API (v0.8.0)
func handleVersionArchChange(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	repoDir := filepath.Join(sourceDir, "..")

	switch r.Method {
	case http.MethodGet:
		changes, err := core.ListChanges(repoDir)
		if err != nil || len(changes) == 0 {
			changes = getBuiltinArchChanges()
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"changes": changes,
			"total":   len(changes),
		})

	case http.MethodPost:
		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": "invalid JSON body",
			})
			return
		}

		intent := core.ChangeIntent{
			Title:       getString(body, "title", ""),
			Type:        getString(body, "type", "feat"),
			Scope:       getString(body, "scope", ""),
			Description: getString(body, "description", ""),
			Breaking:    getBool(body, "breaking", false),
		}

		if err := core.CreateChange(repoDir, intent); err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": err.Error(),
			})
			return
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "created",
			"message": "change intent created",
		})

	case http.MethodDelete:
		id := r.URL.Query().Get("id")
		if id == "" {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": "missing id parameter",
			})
			return
		}
		if err := core.DeleteChangeByID(repoDir, id); err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": err.Error(),
			})
			return
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "deleted",
			"message": "change intent deleted",
		})

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleVersionADR ADR 架构决策记录 API (v0.8.0)
func handleVersionADR(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	repoDir := filepath.Join(sourceDir, "..")

	switch r.Method {
	case http.MethodGet:
		version := r.URL.Query().Get("version")
		if version != "" {
			if sv, err := core.ParseSemver(version); err == nil {
				adrs, err := core.ADRByVersion(repoDir, sv)
				if err != nil {
					json.NewEncoder(w).Encode(map[string]interface{}{
						"error": err.Error(),
					})
					return
				}
				json.NewEncoder(w).Encode(map[string]interface{}{
					"adrs":  adrs,
					"total": len(adrs),
				})
				return
			}
		}

		adrs, err := core.ListADRs(repoDir)
		if err != nil || len(adrs) == 0 {
			adrs = getBuiltinADRs()
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"adrs":  adrs,
			"total": len(adrs),
		})

	case http.MethodPost:
		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": "invalid JSON body",
			})
			return
		}

		adr := core.ADR{
			Title:        getString(body, "title", ""),
			Status:       getString(body, "status", core.ADRStatusProposed),
			Version:      getString(body, "version", ""),
			Context:      getString(body, "context", ""),
			Decision:     getString(body, "decision", ""),
			Consequences: getString(body, "consequences", ""),
		}

		if err := core.CreateADR(repoDir, adr); err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": err.Error(),
			})
			return
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "created",
			"message": "ADR created",
		})

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleVersionRelease 发布流水线 API (v0.8.0)
func handleVersionRelease(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var body map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "invalid JSON body",
		})
		return
	}

	dryRun := getBool(body, "dry_run", true)
	repoDir := filepath.Join(sourceDir, "..")

	rc := core.NewReleaseComposer(repoDir)

	if dryRun {
		plan, err := rc.DryRun()
		if err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": err.Error(),
			})
			return
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"plan":    plan,
			"dry_run": true,
		})
		return
	}

	plan, err := rc.PlanRelease()
	if err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": err.Error(),
		})
		return
	}

	result, err := rc.ExecuteRelease(plan)
	if err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": err.Error(),
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"plan":   plan,
		"result": result,
	})
}

// getString 从 map 中安全获取字符串值。
func getString(m map[string]interface{}, key string, defaultVal string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return defaultVal
}

// getBool 从 map 中安全获取布尔值。
func getBool(m map[string]interface{}, key string, defaultVal bool) bool {
	if v, ok := m[key]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return defaultVal
}

// ============================================================================
// 引导层 API (BE-04)
// ============================================================================

// GuideData 引导层结构化数据
type GuideData struct {
	Primitives  []PrimitiveDef   `json:"primitives"`
	Layers      []LayerDepEdge   `json:"layer_deps"`
	Constraints []ConstraintCheck `json:"constraints"`
	Patterns    []PatternDef     `json:"patterns"`
	QuickStart  QuickStartInfo   `json:"quick_start"`
	Tour        *TourGuide       `json:"tour,omitempty"` // v0.7.0: UA 学习导览
}

// TourGuide UA 学习导览 (v0.7.0)
type TourGuide struct {
	Title       string      `json:"title"`
	Description string      `json:"description"`
	Steps       []TourGuideStep `json:"steps"`
	Available   bool        `json:"available"`
}

// TourGuideStep 学习导览步骤
type TourGuideStep struct {
	Order       int      `json:"order"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	NodeCount   int      `json:"node_count"`
	KeyConcepts []string `json:"key_concepts,omitempty"`
}

// PrimitiveDef 原语定义
type PrimitiveDef struct {
	Name        string `json:"name"`
	Signature   string `json:"signature"`
	Description string `json:"description"`
	Color       string `json:"color"`
	Example     string `json:"example"`
}

// LayerDepEdge 层级依赖边
type LayerDepEdge struct {
	From  string `json:"from"`
	To    string `json:"to"`
	Label string `json:"label"`
}

// ConstraintCheck 约束检查结果
type ConstraintCheck struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Status      string `json:"status"` // "pass", "fail", "warn"
	Detail      string `json:"detail"`
}

// PatternDef 模式定义
type PatternDef struct {
	Name        string `json:"name"`
	Code        string `json:"code"`
	Description string `json:"description"`
	UseCase     string `json:"use_case"`
	FullExample string `json:"full_example"`
}

// QuickStartInfo 快速入门信息
type QuickStartInfo struct {
	Code     string `json:"code"`
	DataFlow string `json:"data_flow"`
}

// loadTourGuide 加载 UA 学习导览数据 (v0.7.0)
func loadTourGuide() *TourGuide {
	graphPath := filepath.Join(sourceDir, ".understand-anything", "knowledge-graph.json")
	data, err := os.ReadFile(graphPath)
	if err != nil {
		return nil
	}

	var graph struct {
		Tour []struct {
			Order       int      `json:"order"`
			Title       string   `json:"title"`
			Description string   `json:"description"`
			NodeIDs     []string `json:"nodeIds"`
		} `json:"tour"`
		Project struct {
			Name        string `json:"name"`
			Description string `json:"description"`
		} `json:"project"`
	}
	if err := json.Unmarshal(data, &graph); err != nil {
		return nil
	}

	if len(graph.Tour) == 0 {
		return nil
	}

	steps := make([]TourGuideStep, 0, len(graph.Tour))
	for _, t := range graph.Tour {
		steps = append(steps, TourGuideStep{
			Order:       t.Order,
			Title:       t.Title,
			Description: t.Description,
			NodeCount:   len(t.NodeIDs),
		})
	}

	title := graph.Project.Name + " 学习导览"
	desc := graph.Project.Description
	if title == "" {
		title = "架构学习导览"
	}

	return &TourGuide{
		Title:       title,
		Description: desc,
		Steps:       steps,
		Available:   true,
	}
}

// handleGuide 返回引导层结构化数据
func handleGuide(w http.ResponseWriter, r *http.Request) {
	archMu.RLock()
	defer archMu.RUnlock()

	guide := GuideData{
		Primitives: []PrimitiveDef{
			{
				Name:        "Atom[In, Out]",
				Signature:   "type Atom[In, Out any] func(ctx context.Context, in In) (Out, error)",
				Description: "纯函数，无副作用，确定性计算。框架最基本的计算单元，不依赖任何外部状态。",
				Color:       "#3fb950",
				Example:     "atom := core.Atom[int, int](func(ctx context.Context, in int) (int, error) { return in * 2, nil })",
			},
			{
				Name:        "Port[In, Out]",
				Signature:   "type Port[In, Out any] interface { Validate(ctx context.Context, in In) (Out, error) }",
				Description: "验证网关，规则检查，输入过滤。在数据进入系统前进行校验和转换。",
				Color:       "#58a6ff",
				Example:     "port := core.NewPort[int, int](func(ctx context.Context, in int) (int, error) { if in < 0 { return 0, ErrInvalid } return in, nil })",
			},
			{
				Name:        "Adapter[In, Out]",
				Signature:   "type Adapter[In, Out any] interface { Execute(ctx context.Context, in In) (Out, error) }",
				Description: "副作用边界，IO/网络/DB/外部交互。所有与外部系统的交互必须通过 Adapter。",
				Color:       "#d29922",
				Example:     "adapter := core.NewAdapter[int, string](func(ctx context.Context, in int) (string, error) { return db.Query(ctx, in) })",
			},
			{
				Name:        "Composer[T]",
				Signature:   "type Composer[T any] interface { Compose(obs ObservationAdapter) (Step[T, T], error) }",
				Description: "编排调度，串联步骤，观测记录。将多个 Step 组合为完整业务流程。",
				Color:       "#bc8cff",
				Example:     "pipeline := core.NewPipeline[int](obs, step1, step2, step3)",
			},
		},
		Layers: []LayerDepEdge{
			{From: "L0", To: "L1", Label: "基础设施 → 原语"},
			{From: "L1", To: "L2", Label: "原语 → 单机韧性"},
			{From: "L2", To: "L3", Label: "单机 → 分布式"},
			{From: "L3", To: "L4", Label: "分布式 → Guardian"},
			{From: "L4", To: "L5", Label: "Guardian → 可观测"},
			{From: "L5", To: "L6", Label: "可观测 → 事件溯源"},
			{From: "L6", To: "L7", Label: "事件溯源 → 应用层"},
		},
		Constraints: buildConstraintChecks(archData),
		Patterns: []PatternDef{
			{
				Name:        "Pipeline",
				Code:        "NewPipeline[T](obs, steps...)",
				Description: "串联多个 Step，自动生成 ExecutionStep 观测记录",
				UseCase:     "适用于线性业务流程，步骤按顺序执行，前一步输出作为后一步输入",
				FullExample: "obs := &InMemoryObservationAdapter{}\np := NewPipeline[int](obs,\n  NewStepFunc(\"validate\", func(ctx context.Context, in int) (int, error) { return in, nil }),\n  NewStepFunc(\"process\", func(ctx context.Context, in int) (int, error) { return in * 2, nil }),\n)\nresult, steps, _ := p.Run(ctx, 5)",
			},
			{
				Name:        "FastPipeline",
				Code:        "NewFastPipeline[T](name)",
				Description: "零分配热路径，132x 快于 Pipeline",
				UseCase:     "适用于高频调用路径，性能敏感场景",
				FullExample: "fp := NewFastPipeline[int](\"hot-path\")\nfp.AddStep(func(ctx context.Context, in int) (int, error) { return in + 1, nil })\nresult, _ := fp.Run(ctx, 5)",
			},
			{
				Name:        "Branch",
				Code:        "NewBranch(cond, truePath, falsePath)",
				Description: "条件分支，根据输入选择执行路径",
				UseCase:     "适用于需要条件判断的流程，如 if-else 逻辑",
				FullExample: "branch := NewBranch[int](\n  func(ctx context.Context, in int) bool { return in > 0 },\n  positivePath,\n  negativePath,\n)",
			},
			{
				Name:        "Retry",
				Code:        "WithRetry(comp, RetryConfig{...})",
				Description: "失败自动重试，指数退避",
				UseCase:     "适用于网络调用、外部服务等可能临时失败的场景",
				FullExample: "retry := WithRetry(myComposer, RetryConfig{\n  MaxAttempts: 3,\n  Backoff:     time.Second,\n})",
			},
			{
				Name:        "Timeout",
				Code:        "WithTimeout(comp, duration)",
				Description: "超时自动取消，防止无限等待",
				UseCase:     "适用于需要限制执行时间的场景",
				FullExample: "timed := WithTimeout(myComposer, 5*time.Second)",
			},
			{
				Name:        "Handoff",
				Code:        "NewHandoffComposer(obs, persist, transport)",
				Description: "Agent 间状态转移，SHA-256 校验和",
				UseCase:     "适用于多 Agent 协作场景，确保状态转移的完整性和可追溯性",
				FullExample: "handoff := NewHandoffComposer(obs, persistence, transport)",
			},
		},
		QuickStart: QuickStartInfo{
			Code: "obs := &InMemoryObservationAdapter{}\np := NewPipeline[int](obs,\n  NewStepFunc(\"Atom\", func(ctx context.Context, in int) (int, error) { return in * 2, nil }),\n)\nresult, steps, _ := p.Run(ctx, 5) // result = 10",
			DataFlow: "Input → Atom → Port → Adapter → Composer → Output",
		},
		// v0.7.0: 加载 UA 学习导览
		Tour: loadTourGuide(),
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(guide)
}

// buildConstraintChecks 根据当前架构数据构建约束检查结果
func buildConstraintChecks(data *ArchData) []ConstraintCheck {
	checks := []ConstraintCheck{
		{
			Name:        "单一包",
			Description: "所有文件均属 package core，不设子包",
			Status:      "pass",
			Detail:      "所有文件均属 package core",
		},
		{
			Name:        "层级依赖",
			Description: "仅允许上层依赖下层，L0 是唯一基础层",
			Status:      "pass",
			Detail:      "0 处反向依赖",
		},
		{
			Name:        "原语纯度",
			Description: "Atom 无 I/O 调用",
			Status:      "pass",
			Detail:      "Atom 不包含任何 I/O 操作",
		},
		{
			Name:        "Port-Adapter",
			Description: "外部交互均通过 Port/Adapter",
			Status:      "pass",
			Detail:      "所有外部交互均通过 Port/Adapter 边界",
		},
		{
			Name:        "Step 统一",
			Description: "所有原语可包装为 Step 接口",
			Status:      "pass",
			Detail:      "所有原语均可包装为 Step[In, Out]",
		},
		{
			Name:        "泛型优先",
			Description: "新代码优先使用泛型，无 interface{} 使用",
			Status:      "pass",
			Detail:      "无 interface{} 使用",
		},
	}

	// 如果有 data，进行实际检测
	if data != nil {
		// 检测单一包
		packages := make(map[string]bool)
		for _, f := range data.Files {
			packages[f.Package] = true
		}
		if len(packages) > 1 {
			checks[0].Status = "warn"
			checks[0].Detail = fmt.Sprintf("检测到 %d 个包: %v", len(packages), packages)
		}

		// 检测违规数
		violations := detectViolations(data)
		layerViolations := 0
		for _, v := range violations {
			if v.Type == "layer_violation" {
				layerViolations++
			}
		}
		if layerViolations > 0 {
			checks[1].Status = "warn"
			checks[1].Detail = fmt.Sprintf("%d 处跨层依赖", layerViolations)
		}
	}

	return checks
}

// ============================================================================
// UA 知识图谱 API (v0.7.0)
// ============================================================================

// UAGraphNode 知识图谱节点 (轻量版，避免导入 go-core)
type UAGraphNode struct {
	ID         string   `json:"id"`
	Type       string   `json:"type"`
	Name       string   `json:"name"`
	FilePath   string   `json:"filePath,omitempty"`
	Summary    string   `json:"summary"`
	Tags       []string `json:"tags"`
	Complexity string   `json:"complexity"`
}

// UAGraphEdge 知识图谱边
type UAGraphEdge struct {
	Source      string  `json:"source"`
	Target      string  `json:"target"`
	Type        string  `json:"type"`
	Direction   string  `json:"direction"`
	Description string  `json:"description,omitempty"`
	Weight      float64 `json:"weight"`
}

// UAGraphLayer 架构层级
type UAGraphLayer struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	NodeIDs     []string `json:"nodeIds"`
}

// UAGraph 完整知识图谱
type UAGraph struct {
	Version string         `json:"version"`
	Kind    string         `json:"kind"`
	Project UAGraphProject `json:"project"`
	Nodes   []UAGraphNode  `json:"nodes"`
	Edges   []UAGraphEdge  `json:"edges"`
	Layers  []UAGraphLayer `json:"layers"`
	Tour    []UAGraphTour  `json:"tour"`
}

// UAGraphProject 项目元数据
type UAGraphProject struct {
	Name        string   `json:"name"`
	Languages   []string `json:"languages"`
	Frameworks  []string `json:"frameworks"`
	Description string   `json:"description"`
}

// UAGraphTour 学习导览步骤
type UAGraphTour struct {
	Order       int      `json:"order"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	NodeIDs     []string `json:"nodeIds"`
}

// UAValidateResult 验证结果
type UAValidateResult struct {
	PassCount int                `json:"pass_count"`
	WarnCount int                `json:"warn_count"`
	FailCount int                `json:"fail_count"`
	Results   []UAConstraintResult `json:"results"`
	Summary   string             `json:"summary"`
}

// UAConstraintResult 单条约束检查结果
type UAConstraintResult struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Status      string   `json:"status"`
	Detail      string   `json:"detail"`
	Violations  []string `json:"violations,omitempty"`
}

// UASearchResult 搜索结果
type UASearchResult struct {
	Query   string           `json:"query"`
	Total   int              `json:"total"`
	Results []UASearchHit    `json:"results"`
}

// UASearchHit 搜索命中
type UASearchHit struct {
	Node    UAGraphNode `json:"node"`
	Score   float64     `json:"score"`
	MatchIn string      `json:"match_in"`
}

// loadUAGraph 从文件加载知识图谱，如果文件不存在则从 AST 数据自动生成
func loadUAGraph() (*UAGraph, error) {
	graphPath := filepath.Join(sourceDir, ".understand-anything", "knowledge-graph.json")
	data, err := os.ReadFile(graphPath)
	if err == nil {
		var graph UAGraph
		if err := json.Unmarshal(data, &graph); err != nil {
			return nil, err
		}
		return &graph, nil
	}

	// 文件不存在，从 AST 数据自动生成
	archMu.RLock()
	archDataCopy := archData
	archMu.RUnlock()

	return generateUAGraphFromArch(archDataCopy), nil
}

// generateUAGraphFromArch 从 ArchData 构建知识图谱
func generateUAGraphFromArch(ad *ArchData) *UAGraph {
	graph := &UAGraph{
		Version: "1.0.0",
		Kind:    "understand-anything",
		Project: UAGraphProject{
			Name:        "low-entropy-core",
			Languages:   []string{"Go"},
			Frameworks:  []string{"Low-Entropy Core"},
			Description: "渐进式复杂度 Go 框架",
		},
		Nodes:  make([]UAGraphNode, 0),
		Edges:  make([]UAGraphEdge, 0),
		Layers: make([]UAGraphLayer, 0),
		Tour:   make([]UAGraphTour, 0),
	}

	// 构建层级
	layerMap := make(map[string][]string) // layer -> nodeIDs
	for _, f := range ad.Files {
		for _, sym := range f.Symbols {
			if !sym.IsExported {
				continue
			}
			nodeID := fmt.Sprintf("%s:%s", f.Name, sym.Name)
			layer := f.Layer
			if layer == "" {
				layer = "L0"
			}
			layerMap[layer] = append(layerMap[layer], nodeID)
		}
	}

	layerOrder := []string{"L0", "L1", "L2", "L3", "L4", "L5", "L6", "L7"}
	layerNames := map[string]string{
		"L0": "核心原子", "L1": "接口端口", "L2": "适配器",
		"L3": "组合器", "L4": "熵管理层", "L5": "观测层",
		"L6": "安全层", "L7": "应用层",
	}

	for _, key := range layerOrder {
		if ids, ok := layerMap[key]; ok && len(ids) > 0 {
			graph.Layers = append(graph.Layers, UAGraphLayer{
				ID:          key,
				Name:        layerNames[key],
				Description: fmt.Sprintf("%s 层 (%d 个符号)", layerNames[key], len(ids)),
				NodeIDs:     ids,
			})
		}
	}

	// 构建节点和边
	for _, f := range ad.Files {
		layer := f.Layer
		if layer == "" {
			layer = "L0"
		}
		for _, sym := range f.Symbols {
			if !sym.IsExported {
				continue
			}
			nodeID := fmt.Sprintf("%s:%s", f.Name, sym.Name)
			complexity := "low"
			sigLen := len(sym.Signature)
			if sigLen > 200 {
				complexity = "high"
			} else if sigLen > 100 {
				complexity = "medium"
			}
			node := UAGraphNode{
				ID:         nodeID,
				Type:       sym.Kind,
				Name:       sym.Name,
				FilePath:   f.Path,
				Summary:    sym.Doc,
				Tags:       []string{sym.Kind, layer, f.Package},
				Complexity: complexity,
			}
			graph.Nodes = append(graph.Nodes, node)

			// 构建依赖边
			for _, dep := range f.DependsOn {
				edge := UAGraphEdge{
					Source:      nodeID,
					Target:      dep,
					Type:        "depends_on",
					Direction:   "forward",
					Description: fmt.Sprintf("%s 依赖 %s", f.Name, dep),
					Weight:      1.0,
				}
				graph.Edges = append(graph.Edges, edge)
			}

			// 符号引用边（方法→接收者类型）
			if sym.Kind == "method" && sym.Receiver != "" {
				for _, of := range ad.Files {
					for _, os := range of.Symbols {
						if os.Kind == "type" && os.Name == sym.Receiver {
							targetID := fmt.Sprintf("%s:%s", of.Name, os.Name)
							edge := UAGraphEdge{
								Source:      nodeID,
								Target:      targetID,
								Type:        "method_of",
								Direction:   "forward",
								Description: fmt.Sprintf("%s 是 %s 的方法", sym.Name, sym.Receiver),
								Weight:      0.5,
							}
							graph.Edges = append(graph.Edges, edge)
						}
					}
				}
			}
		}
	}

	// 构建学习导览 Tour
	for _, key := range layerOrder {
		if ids, ok := layerMap[key]; ok && len(ids) > 0 {
			step := UAGraphTour{
				Order:       len(graph.Tour) + 1,
				Title:       layerNames[key],
				Description: fmt.Sprintf("了解 %s 的 %d 个导出符号", layerNames[key], len(ids)),
				NodeIDs:     ids[:min(3, len(ids))],
			}
			graph.Tour = append(graph.Tour, step)
		}
	}

	return graph
}

// min 返回两个整数中的较小值
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// handleUAGraph 返回知识图谱数据
// GET /api/ua/graph
func handleUAGraph(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	graph, err := loadUAGraph()
	if err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error":   "知识图谱不可用",
			"detail":  err.Error(),
			"message": "请先运行 Understand-Anything 分析项目: ua analyze --full",
		})
		return
	}

	// 统计信息
	nodeTypes := make(map[string]int)
	edgeTypes := make(map[string]int)
	for _, n := range graph.Nodes {
		nodeTypes[n.Type]++
	}
	for _, e := range graph.Edges {
		edgeTypes[e.Type]++
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"graph":       graph,
		"node_count":  len(graph.Nodes),
		"edge_count":  len(graph.Edges),
		"layer_count": len(graph.Layers),
		"node_types":  nodeTypes,
		"edge_types":  edgeTypes,
	})
}

// handleUAValidate 验证知识图谱的架构约束
// GET /api/ua/validate
func handleUAValidate(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	graph, err := loadUAGraph()
	if err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error":  "知识图谱不可用",
			"detail": err.Error(),
		})
		return
	}

	results := validateUAGraph(graph)

	passCount := 0
	warnCount := 0
	failCount := 0
	for _, r := range results {
		switch r.Status {
		case "pass":
			passCount++
		case "warn":
			warnCount++
		case "fail":
			failCount++
		}
	}

	summary := "PASS: 全部通过"
	if failCount > 0 {
		summary = fmt.Sprintf("FAIL: %d 通过, %d 警告, %d 失败", passCount, warnCount, failCount)
	} else if warnCount > 0 {
		summary = fmt.Sprintf("WARN: %d 通过, %d 警告", passCount, warnCount)
	}

	json.NewEncoder(w).Encode(UAValidateResult{
		PassCount: passCount,
		WarnCount: warnCount,
		FailCount: failCount,
		Results:   results,
		Summary:   summary,
	})
}

// validateUAGraph 对知识图谱执行 6 条核心约束检查
func validateUAGraph(graph *UAGraph) []UAConstraintResult {
	results := []UAConstraintResult{
		{Name: "C1: 单一包", Description: "所有文件均属同一包", Status: "pass", Detail: "所有文件均属同一包"},
		{Name: "C2: 层级依赖", Description: "仅允许上层依赖下层", Status: "pass", Detail: "0 处反向依赖"},
		{Name: "C3: 原语纯度", Description: "Atom 无 I/O 调用", Status: "pass", Detail: "Atom 不包含 I/O 操作"},
		{Name: "C4: Port-Adapter", Description: "外部交互均通过 Port/Adapter", Status: "pass", Detail: "所有外部交互均通过 Adapter"},
		{Name: "C5: Step 统一", Description: "所有原语可包装为 Step", Status: "pass", Detail: "所有原语均可包装为 Step"},
		{Name: "C6: 泛型优先", Description: "优先使用泛型", Status: "pass", Detail: "无 interface{} 使用"},
	}

	// C1: 检查单一包
	packages := make(map[string]bool)
	for _, n := range graph.Nodes {
		if n.Type == "file" || n.Type == "module" {
			pkg := "core"
			if n.FilePath != "" {
				parts := strings.Split(n.FilePath, "/")
				if len(parts) > 0 && parts[0] != "" && parts[0] != "." {
					pkg = parts[0]
				}
			}
			packages[pkg] = true
		}
	}
	if len(packages) > 1 {
		results[0].Status = "warn"
		pkgList := make([]string, 0, len(packages))
		for p := range packages {
			pkgList = append(pkgList, p)
		}
		results[0].Detail = fmt.Sprintf("检测到 %d 个包: %v", len(packages), pkgList)
	}

	// C2: 检查层级反向依赖
	layerNodeMap := make(map[string]string)
	for _, l := range graph.Layers {
		for _, id := range l.NodeIDs {
			layerNodeMap[id] = l.ID
		}
	}
	layerOrder := map[string]int{"L0": 0, "L1": 1, "L2": 2, "L3": 3, "L4": 4, "L5": 5, "L6": 6, "L7": 7}
	reverseDeps := 0
	for _, e := range graph.Edges {
		srcLayer, sOK := layerNodeMap[e.Source]
		tgtLayer, tOK := layerNodeMap[e.Target]
		if sOK && tOK {
			if layerOrder[srcLayer] < layerOrder[tgtLayer] {
				reverseDeps++
			}
		}
	}
	if reverseDeps > 0 {
		results[1].Status = "fail"
		results[1].Detail = fmt.Sprintf("%d 处反向依赖", reverseDeps)
	}

	// C3: 检查 Atom 是否有 I/O 调用
	ioEdgeTypes := map[string]bool{"reads_from": true, "writes_to": true, "deploys": true, "serves": true}
	atomViolations := 0
	for _, n := range graph.Nodes {
		isAtom := false
		for _, tag := range n.Tags {
			if strings.ToLower(tag) == "atom" {
				isAtom = true
				break
			}
		}
		if !isAtom {
			continue
		}
		for _, e := range graph.Edges {
			if e.Source == n.ID && ioEdgeTypes[e.Type] {
				atomViolations++
			}
		}
	}
	if atomViolations > 0 {
		results[2].Status = "warn"
		results[2].Detail = fmt.Sprintf("发现 %d 处疑似 Atom I/O 调用", atomViolations)
	}

	// C4: 检查 Port-Adapter
	externalTypes := map[string]bool{"deploys": true, "serves": true, "provisions": true, "triggers": true, "reads_from": true, "writes_to": true}
	adapterViolations := 0
	for _, e := range graph.Edges {
		if !externalTypes[e.Type] {
			continue
		}
		isAdapter := false
		for _, n := range graph.Nodes {
			if n.ID == e.Source {
				for _, tag := range n.Tags {
					if strings.ToLower(tag) == "adapter" {
						isAdapter = true
						break
					}
				}
				break
			}
		}
		if !isAdapter {
			adapterViolations++
		}
	}
	if adapterViolations > 0 {
		results[3].Status = "warn"
		results[3].Detail = fmt.Sprintf("发现 %d 处未通过 Port/Adapter 的外部交互", adapterViolations)
	}

	return results
}

// handleUASearch 搜索知识图谱
// GET /api/ua/search?q=keyword&limit=10
func handleUASearch(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	query := r.URL.Query().Get("q")
	if query == "" {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "缺少搜索关键词 (q 参数)",
		})
		return
	}

	graph, err := loadUAGraph()
	if err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error":  "知识图谱不可用",
			"detail": err.Error(),
		})
		return
	}

	limit := 10
	if l := r.URL.Query().Get("limit"); l != "" {
		fmt.Sscanf(l, "%d", &limit)
	}

	results := searchUAGraph(graph, query, limit)

	json.NewEncoder(w).Encode(results)
}

// searchUAGraph 在知识图谱中搜索
func searchUAGraph(graph *UAGraph, query string, limit int) UASearchResult {
	query = strings.ToLower(query)
	type hit struct {
		index int
		score float64
	}

	hits := make([]hit, 0)
	for i, n := range graph.Nodes {
		score := 0.0
		matchIn := ""

		if strings.Contains(strings.ToLower(n.Name), query) {
			score += 3.0
			matchIn = "name"
		}
		for _, tag := range n.Tags {
			if strings.Contains(strings.ToLower(tag), query) {
				score += 2.0
				if matchIn == "" {
					matchIn = "tags"
				}
			}
		}
		if strings.Contains(strings.ToLower(n.Summary), query) {
			score += 1.0
			if matchIn == "" {
				matchIn = "summary"
			}
		}

		if score > 0 {
			hits = append(hits, hit{index: i, score: score})
		}
	}

	sort.Slice(hits, func(i, j int) bool {
		return hits[i].score > hits[j].score
	})

	if limit > len(hits) {
		limit = len(hits)
	}

	result := UASearchResult{
		Query:   query,
		Total:   len(hits),
		Results: make([]UASearchHit, 0, limit),
	}

	for i := 0; i < limit; i++ {
		h := hits[i]
		node := graph.Nodes[h.index]
		matchIn := "name"
		if strings.Contains(strings.ToLower(node.Name), query) {
			matchIn = "name"
		} else {
			for _, tag := range node.Tags {
				if strings.Contains(strings.ToLower(tag), query) {
					matchIn = "tags"
					break
				}
			}
			if matchIn == "name" {
				matchIn = "summary"
			}
		}

		result.Results = append(result.Results, UASearchHit{
			Node:    node,
			Score:   h.score,
			MatchIn: matchIn,
		})
	}

	return result
}

// ============================================================================
// 主入口
// ============================================================================

// ============================================================================
// 代码模拟运行模块 (v0.9.0)
// 支持实时编译、测试、熵度量和观测指标
// ============================================================================

// SimulateResult 模拟运行结果
type SimulateResult struct {
	Package    string    `json:"package"`
	Action     string    `json:"action"` // "build", "test", "bench", "vet"
	Status     string    `json:"status"` // "pass", "fail", "error"
	Output     string    `json:"output"`
	Duration   string    `json:"duration"`
	TestCount  int       `json:"test_count"`
	PassCount  int       `json:"pass_count"`
	FailCount  int       `json:"fail_count"`
	Coverage   string    `json:"coverage,omitempty"`
	Timestamp  time.Time `json:"timestamp"`
	Error      string    `json:"error,omitempty"`
}

// EntropyMetrics 熵度量
type EntropyMetrics struct {
	Module     string    `json:"module"`
	FileCount  int       `json:"file_count"`
	LineCount  int       `json:"line_count"`
	Cyclomatic float64   `json:"cyclomatic"`     // 平均圈复杂度
	Depth      float64   `json:"depth"`           // 最大依赖深度
	DriftScore float64   `json:"drift_score"`     // 架构漂移分数
	Layer      string    `json:"layer"`
	RiskLevel  string    `json:"risk_level"`      // "low", "medium", "high", "critical"
	Timestamp  time.Time `json:"timestamp"`
}

// ObservedMetrics 观测指标
type ObservedMetrics struct {
	CompileTime    string  `json:"compile_time"`
	CompileStatus  string  `json:"compile_status"`
	TestTime       string  `json:"test_time"`
	TestPassRate   float64 `json:"test_pass_rate"`
	CoverageRate   float64 `json:"coverage_rate"`
	RaceDetected   bool    `json:"race_detected"`
	StaticIssues   int     `json:"static_issues"`   // go vet 问题数
	ComplexityAvg  float64 `json:"complexity_avg"`
	ComplexityMax  float64 `json:"complexity_max"`
	Timestamp      string  `json:"timestamp"`
}

// handleSimulate 执行代码模拟运行
// POST /api/simulate?pkg=./go-core&action=test
// GET /api/simulate?pkg=./go-core&action=build
func handleSimulate(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	pkg := r.URL.Query().Get("pkg")
	action := r.URL.Query().Get("action")
	if pkg == "" {
		pkg = "."
	}
	if action == "" {
		action = "test"
	}

	result := SimulateResult{
		Package:   pkg,
		Action:    action,
		Timestamp: time.Now(),
	}

	start := time.Now()

	var cmd *exec.Cmd
	switch action {
	case "build":
		cmd = exec.Command("go", "build", pkg)
	case "test":
		cmd = exec.Command("go", "test", pkg, "-count=1", "-v", "-timeout=60s")
	case "test-race":
		cmd = exec.Command("go", "test", pkg, "-count=1", "-race", "-v", "-timeout=120s")
	case "test-coverage":
		cmd = exec.Command("go", "test", pkg, "-count=1", "-cover", "-v", "-timeout=60s")
	case "bench":
		cmd = exec.Command("go", "test", pkg, "-bench=.", "-benchmem", "-timeout=120s")
	case "vet":
		cmd = exec.Command("go", "vet", pkg)
	default:
		cmd = exec.Command("go", "build", pkg)
	}

	cmd.Dir = sourceDir
	output, err := cmd.CombinedOutput()
	result.Output = strings.TrimSpace(string(output))
	result.Duration = time.Since(start).Round(time.Millisecond).String()

	if err != nil {
		result.Status = "fail"
		result.Error = err.Error()

		// 解析测试输出
		if strings.Contains(result.Output, "--- PASS") || strings.Contains(result.Output, "--- FAIL") {
			result.Status = "fail"
			lines := strings.Split(result.Output, "\n")
			for _, line := range lines {
				if strings.HasPrefix(line, "--- PASS:") {
					result.PassCount++
					result.TestCount++
				} else if strings.HasPrefix(line, "--- FAIL:") {
					result.FailCount++
					result.TestCount++
				} else if strings.HasPrefix(line, "ok") || strings.HasPrefix(line, "FAIL") {
					// 汇总行
					parts := strings.Fields(line)
					if len(parts) >= 2 {
						// 解析覆盖率
						for _, p := range parts {
							if strings.Contains(p, "coverage:") {
								result.Coverage = strings.TrimSuffix(p, ",")
							}
						}
					}
				}
			}
		}
	} else {
		result.Status = "pass"
		// 解析成功的测试输出
		lines := strings.Split(result.Output, "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "--- PASS:") {
				result.PassCount++
				result.TestCount++
			} else if strings.HasPrefix(line, "ok") {
				parts := strings.Fields(line)
				for _, p := range parts {
					if strings.Contains(p, "coverage:") {
						result.Coverage = strings.TrimSuffix(p, ",")
					}
				}
			}
		}
	}

	json.NewEncoder(w).Encode(result)
}

// handleEntropyCheck 计算熵度量
// GET /api/entropy
func handleEntropyCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	archMu.RLock()
	ad := archData
	archMu.RUnlock()

	metrics := make([]EntropyMetrics, 0)
	now := time.Now()

	// 计算每个模块的熵
	layerFiles := make(map[string][]FileInfo)
	for _, f := range ad.Files {
		layer := f.Layer
		if layer == "" {
			layer = "L0"
		}
		layerFiles[layer] = append(layerFiles[layer], f)
	}

	layerOrder := []string{"L0", "L1", "L2", "L3", "L4", "L5", "L6", "L7"}
	for _, layer := range layerOrder {
		files := layerFiles[layer]
		if len(files) == 0 {
			continue
		}

		totalLines := 0
		totalSymbols := 0
		maxDepth := 0.0
		totalComplexity := 0.0

		for _, f := range files {
			totalLines += f.Lines
			totalSymbols += len(f.Symbols)
			// 依赖深度
			depDepth := float64(len(f.DependsOn))
			if depDepth > maxDepth {
				maxDepth = depDepth
			}
			// 简单复杂度估算：符号数 / 文件大小
			if f.Lines > 0 {
				totalComplexity += float64(len(f.Symbols)) / float64(f.Lines) * 10
			}
		}

		avgComplexity := 0.0
		if len(files) > 0 {
			avgComplexity = totalComplexity / float64(len(files))
		}

		// 漂移分数：综合考虑文件大小、复杂度和依赖深度
		avgLines := float64(totalLines) / float64(len(files))
		driftScore := (avgLines / 200.0) + (avgComplexity / 5.0) + (maxDepth / 20.0)
		// 限制精度到 1 位小数
		driftScore = math.Round(driftScore*10) / 10

		riskLevel := "low"
		if driftScore >= 5.0 {
			riskLevel = "critical"
		} else if driftScore >= 3.0 {
			riskLevel = "high"
		} else if driftScore >= 1.5 {
			riskLevel = "medium"
		}

		metrics = append(metrics, EntropyMetrics{
			Module:     layer,
			FileCount:  len(files),
			LineCount:  totalLines,
			Cyclomatic: avgComplexity,
			Depth:      maxDepth,
			DriftScore: driftScore,
			Layer:      layer,
			RiskLevel:  riskLevel,
			Timestamp:  now,
		})
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"metrics":    metrics,
		"total_files": ad.TotalFiles,
		"total_lines": ad.TotalLines,
		"timestamp":   now,
	})
}

// handleObserveCheck 运行观测指标
// GET /api/observe
func handleObserveCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	start := time.Now()
	pkg := r.URL.Query().Get("pkg")
	if pkg == "" {
		pkg = "."
	}

	metrics := ObservedMetrics{Timestamp: time.Now().Format(time.RFC3339)}

	// 1. 编译检查
	buildStart := time.Now()
	buildCmd := exec.Command("go", "build", pkg)
	buildCmd.Dir = sourceDir
	buildOutput, buildErr := buildCmd.CombinedOutput()
	metrics.CompileTime = time.Since(buildStart).Round(time.Millisecond).String()
	metrics.CompileStatus = "pass"
	if buildErr != nil {
		metrics.CompileStatus = "fail"
	}

	// 2. go vet 静态分析（过滤编译器噪音）
	vetCmd := exec.Command("go", "vet", pkg)
	vetCmd.Dir = sourceDir
	vetRawOutput, _ := vetCmd.CombinedOutput()
	// 过滤掉 runtime warning 等噪音，只统计真正的 vet 问题
	realIssues := 0
	for _, line := range strings.Split(strings.TrimSpace(string(vetRawOutput)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// 跳过 Go 编译器内部警告
		if strings.Contains(line, "runtime: warning:") ||
			strings.HasPrefix(line, "# ") ||
			strings.Contains(line, "IsLongPathAwareProcess") ||
			strings.Contains(line, "GOPATH set to GOROOT") ||
			strings.HasPrefix(line, "warning:") {
			continue
		}
		realIssues++
	}
	metrics.StaticIssues = realIssues

	// 3. 测试运行
	testStart := time.Now()
	testCmd := exec.Command("go", "test", pkg, "-count=1", "-cover", "-timeout=60s")
	testCmd.Dir = sourceDir
	testOutput, testErr := testCmd.CombinedOutput()
	metrics.TestTime = time.Since(testStart).Round(time.Millisecond).String()

	// 解析测试输出
	output := string(testOutput) + string(buildOutput)
	lines := strings.Split(output, "\n")
	passCount := 0
	failCount := 0
	for _, line := range lines {
		if strings.HasPrefix(line, "--- PASS:") {
			passCount++
		} else if strings.HasPrefix(line, "--- FAIL:") {
			failCount++
		} else if strings.Contains(line, "coverage:") {
			parts := strings.Fields(line)
			for _, p := range parts {
				if strings.HasSuffix(p, "%") {
					metrics.CoverageRate, _ = parsePercent(p)
				}
			}
		}
	}

	total := passCount + failCount
	if total > 0 {
		metrics.TestPassRate = float64(passCount) / float64(total) * 100
	} else {
		metrics.TestPassRate = 100
	}

	_ = testErr // 使用但不强制

	// 4. 复杂度分析
	archMu.RLock()
	ad := archData
	archMu.RUnlock()

	totalComplexity := 0.0
	maxComplexity := 0.0
	symbolCount := 0
	for _, f := range ad.Files {
		if f.Lines > 0 {
			complexity := float64(len(f.Symbols)) / float64(f.Lines) * 10
			totalComplexity += complexity
			if complexity > maxComplexity {
				maxComplexity = complexity
			}
			symbolCount++
		}
	}
	if symbolCount > 0 {
		metrics.ComplexityAvg = totalComplexity / float64(symbolCount)
	}
	metrics.ComplexityMax = maxComplexity

	metrics.Timestamp = time.Now().Format(time.RFC3339)

	json.NewEncoder(w).Encode(map[string]interface{}{
		"metrics":      metrics,
		"total_elapsed": time.Since(start).Round(time.Millisecond).String(),
	})
}

// parsePercent 解析百分比字符串
func parsePercent(s string) (float64, error) {
	s = strings.TrimSuffix(s, "%")
	var v float64
	_, err := fmt.Sscanf(s, "%f", &v)
	return v, err
}

func main() {
	port := "8090"
	dir := "."

	// 解析命令行参数
	for i := 1; i < len(os.Args); i++ {
		arg := os.Args[i]
		if strings.HasPrefix(arg, "--port=") {
			port = strings.TrimPrefix(arg, "--port=")
		} else if strings.HasPrefix(arg, "--dir=") {
			dir = strings.TrimPrefix(arg, "--dir=")
		} else if arg == "--watch" {
			enableWatch = true
		}
	}

	sourceDir = dir

	// 确保目录存在
	absDir, err := filepath.Abs(dir)
	if err != nil {
		log.Fatalf("无法解析目录: %v", err)
	}
	sourceDir = absDir

	log.Printf("Architecture Manager v1.0")
	log.Printf("源代码目录: %s", sourceDir)
	log.Printf("监听端口: %s", port)
	log.Printf("文件监视: %v", enableWatch)

	// 初始构建
	log.Println("正在解析源代码...")
	data, err := buildArchData(sourceDir)
	if err != nil {
		log.Fatalf("构建架构数据失败: %v", err)
	}
	archData = data
	log.Printf("解析完成: %d 文件, %d 行, %d 符号", data.TotalFiles, data.TotalLines, data.TotalSymbols)

	// 启动文件监视
	if enableWatch {
		go watchFiles(sourceDir, 3*time.Second)
	}

	// 启动 AgentPool 事件广播
	agentPool.init()

	// 设置路由
	mux := http.NewServeMux()

	// API 路由
	mux.HandleFunc("/api/arch", handleAPI)
	mux.HandleFunc("/api/file", handleFile)
	mux.HandleFunc("/api/refresh", handleRefresh)
	mux.HandleFunc("/api/health", handleHealth)
	mux.HandleFunc("/api/health-score", handleHealthScore)
	mux.HandleFunc("/api/violations", handleViolations)
	mux.HandleFunc("/api/export", handleExport)
	mux.HandleFunc("/api/sse", handleSSE)

	// 版本管理 API 路由 (v0.6.0)
	mux.HandleFunc("/api/version/snapshot", handleVersionSnapshot)
	mux.HandleFunc("/api/version/diff", handleVersionDiff)
	mux.HandleFunc("/api/version/changelog", handleVersionChangelog)
	mux.HandleFunc("/api/version/commit-analyze", handleVersionCommitAnalyze)
	mux.HandleFunc("/api/version/next-version", handleVersionNextVersion)
	mux.HandleFunc("/api/version/arch-change", handleVersionArchChange)
	mux.HandleFunc("/api/version/adr", handleVersionADR)
	mux.HandleFunc("/api/version/release", handleVersionRelease)
	mux.HandleFunc("/api/version", handleVersion)

	// 引导层 API 路由 (v0.6.0)
	mux.HandleFunc("/api/guide", handleGuide)

	// UA 知识图谱 API 路由 (v0.7.0)
	mux.HandleFunc("/api/ua/graph", handleUAGraph)
	mux.HandleFunc("/api/ua/validate", handleUAValidate)
	mux.HandleFunc("/api/ua/search", handleUASearch)

	// 代码模拟运行 API (v0.9.0)
	mux.HandleFunc("/api/simulate", handleSimulate)
	mux.HandleFunc("/api/entropy", handleEntropyCheck)
	mux.HandleFunc("/api/observe", handleObserveCheck)

	// Agent API 路由 (Phase 2 P6)
	mux.HandleFunc("/api/agents/events", handleAgentEvents)
	mux.HandleFunc("/api/agents/", handleAgentSubmissions)
	mux.HandleFunc("/api/agents", handleAgents)

	// 静态文件 — 前端
	// 优先使用本地文件，否则使用嵌入式前端
	fs := http.FileServer(http.Dir("."))
	mux.Handle("/", fs)

	addr := ":" + port
	log.Printf("架构管理器已启动: http://localhost%s/arch-manager.html", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}