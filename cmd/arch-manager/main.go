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
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
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

// handleAPI 返回架构数据 JSON
func handleAPI(w http.ResponseWriter, r *http.Request) {
	archMu.RLock()
	defer archMu.RUnlock()

	if archData == nil {
		http.Error(w, `{"error":"no data available"}`, http.StatusServiceUnavailable)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(archData)
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
// 主入口
// ============================================================================

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