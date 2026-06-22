// Package main — 架构违规检测规则实现
//
// 共 15 类规则：R1 large_file ~ R15 high_dependency_centrality
// 每条规则返回 arch.Violation 列表，由 violations.go 汇总。
package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	arch "low-entropy-core/go-core/arch"
)

const (
	thresholdLargeFile         = 300
	thresholdEmptyFile         = 10
	thresholdFunctionLines     = 80
	thresholdSymbolsPerFile    = 15
	thresholdGlobalVars        = 3
	thresholdLayerGap          = 2
	thresholdDependedBy        = 20
	thresholdCyclomatic        = 10
	thresholdHardcodedHintLen  = 6
)

// ──────────────────────────────────────────────
// 汇总
// ──────────────────────────────────────────────

func summarizeViolations(items []arch.Violation) arch.ViolationResponse {
	var errCount, warnCount, infoCount int
	byRule := map[string]int{}
	bySeverity := map[string]int{}
	for _, v := range items {
		byRule[v.RuleID]++
		switch v.Severity {
		case arch.SeverityError:
			errCount++
		case arch.SeverityWarning:
			warnCount++
		default:
			infoCount++
		}
	}
	bySeverity["error"] = errCount
	bySeverity["warning"] = warnCount
	bySeverity["info"] = infoCount

	sort.SliceStable(items, func(i, j int) bool {
		order := func(s arch.ViolationSeverity) int {
			switch s {
			case arch.SeverityError:
				return 0
			case arch.SeverityWarning:
				return 1
			default:
				return 2
			}
		}
		if order(items[i].Severity) != order(items[j].Severity) {
			return order(items[i].Severity) < order(items[j].Severity)
		}
		return items[i].File < items[j].File
	})

	return arch.ViolationResponse{
		Total:        len(items),
		ErrorCount:   errCount,
		WarningCount: warnCount,
		InfoCount:    infoCount,
		ByRule:       byRule,
		BySeverity:   bySeverity,
		Items:        items,
	}
}

// ──────────────────────────────────────────────
// R1 large_file
// ──────────────────────────────────────────────

func checkLargeFile(data *arch.ArchData) []arch.Violation {
	var out []arch.Violation
	for _, f := range data.Files {
		if f.Lines > thresholdLargeFile {
			out = append(out, arch.Violation{
				Type:       arch.ViolationFileTooLong,
				Severity:   arch.SeverityWarning,
				RuleID:     "R1",
				File:       f.Name,
				FilePath:   f.Path,
				Message:    fmt.Sprintf("文件 %s 代码行数过多（%d 行）", f.Name, f.Lines),
				Detail:     fmt.Sprintf("文件共 %d 行，包含 %d 个符号，超出 300 行阈值", f.Lines, len(f.Symbols)),
				Consequence: "文件过大时难以阅读、修改与 Code Review，容易形成神类/神文件，不利于后续迁移",
				Suggestion: "按职责拆分为多个文件，优先将独立模块、接口与实现分离",
				Value:      f.Lines,
				Threshold:  thresholdLargeFile,
			})
		}
	}
	return out
}

// ──────────────────────────────────────────────
// R2 empty_file
// ──────────────────────────────────────────────

func checkEmptyFile(data *arch.ArchData) []arch.Violation {
	var out []arch.Violation
	for _, f := range data.Files {
		if f.Lines <= thresholdEmptyFile || len(f.Symbols) != 0 {
			continue
		}
		if f.Name == "main.go" || strings.Contains(f.Name, "_fallback") {
			continue
		}
		if f.Lines < 30 && strings.Contains(f.Name, "_") {
			continue
		}
		out = append(out, arch.Violation{
			Type:       arch.ViolationType("empty_file"),
			Severity:   arch.SeverityInfo,
			RuleID:     "R2",
			File:       f.Name,
			FilePath:   f.Path,
			Message:    fmt.Sprintf("文件 %s 未定义导出符号（%d 行）", f.Name, f.Lines),
			Detail:     "该文件可能仅包含内部实现或已被废弃",
			Consequence: "长期存在会提升代码维护复杂度，影响新人的理解成本",
			Suggestion: "确认其职责，必要时并入其它文件或删除",
			Value:      f.Lines,
			Threshold:  thresholdEmptyFile,
		})
	}
	return out
}

// ──────────────────────────────────────────────
// R3 cyclic_dependency
// ──────────────────────────────────────────────

func checkCyclicDependency(data *arch.ArchData) []arch.Violation {
	var out []arch.Violation

	graph := make(map[string]map[string]bool, len(data.Files))
	for _, f := range data.Files {
		graph[f.Name] = map[string]bool{}
		for _, dep := range f.DependsOn {
			graph[f.Name][dep] = true
		}
	}

	cycles := detectCycles(graph)
	seen := map[string]bool{}
	for _, cycle := range cycles {
		key := strings.Join(cycle, "|")
		if seen[key] {
			continue
		}
		seen[key] = true

		if len(cycle) < 2 {
			continue
		}

		pkgSet := map[string]string{}
		allSame := true
		var first string
		for _, n := range cycle {
			pkg := packageOfFile(data, n)
			if first == "" {
				first = pkg
			}
			if pkg != first {
				allSame = false
			}
			pkgSet[n] = pkg
		}
		if allSame {
			continue
		}

		out = append(out, arch.Violation{
			Type:       arch.ViolationCircularDependency,
			Severity:   arch.SeverityError,
			RuleID:     "R3",
			File:       cycle[0],
			Message:    fmt.Sprintf("检测到跨包循环依赖：%s", strings.Join(cycle, " → ")),
			Detail:     fmt.Sprintf("共涉及 %d 个文件，可能引发编译失败或初始化顺序问题", len(cycle)),
			Consequence: "跨包循环依赖会导致无法构建，以及运行时的死循环初始化",
			Suggestion: "将共同依赖下沉到更底层包，或引入中介包以打破循环",
			Value:      len(cycle),
		})
	}
	return out
}

func packageOfFile(data *arch.ArchData, name string) string {
	for _, f := range data.Files {
		if f.Name == name || f.Name == name+".go" {
			return f.Package
		}
	}
	return ""
}

// detectCycles 使用 DFS + 三色标记，查找所有简单环。
func detectCycles(graph map[string]map[string]bool) [][]string {
	var cycles [][]string
	visited := map[string]bool{}
	recStack := map[string]bool{}
	path := []string{}

	var dfs func(string)
	dfs = func(node string) {
		visited[node] = true
		recStack[node] = true
		path = append(path, node)

		// 对邻居排序以获得稳定输出
		neighbors := make([]string, 0, len(graph[node]))
		for n := range graph[node] {
			neighbors = append(neighbors, n)
		}
		sort.Strings(neighbors)

		for _, dep := range neighbors {
			if _, exists := graph[dep]; exists && !visited[dep] {
				dfs(dep)
			} else if recStack[dep] {
				startIdx := -1
				for i, n := range path {
					if n == dep {
						startIdx = i
						break
					}
				}
				if startIdx >= 0 {
					cycle := append([]string{}, path[startIdx:]...)
					cycle = append(cycle, dep)
					cycles = append(cycles, cycle)
				}
			}
		}

		path = path[:len(path)-1]
		recStack[node] = false
	}

	// 按名字稳定遍历
	nodes := make([]string, 0, len(graph))
	for n := range graph {
		nodes = append(nodes, n)
	}
	sort.Strings(nodes)
	for _, n := range nodes {
		if !visited[n] {
			dfs(n)
		}
	}
	return cycles
}

// ──────────────────────────────────────────────
// R4 layer_violation
// ──────────────────────────────────────────────

func checkLayerViolation(data *arch.ArchData) []arch.Violation {
	var out []arch.Violation

	layerIndex := map[string]int{}
	for i, l := range data.Layers {
		layerIndex[l.Layer] = i
	}

	fileLayerByName := map[string]string{}
	for _, f := range data.Files {
		fileLayerByName[f.Name] = f.Layer
	}

	for _, f := range data.Files {
		from, ok := layerIndex[f.Layer]
		if !ok {
			continue
		}
		for _, dep := range f.DependsOn {
			targetLayer, ok := fileLayerByName[dep]
			if !ok {
				continue
			}
			to, ok := layerIndex[targetLayer]
			if !ok {
				continue
			}
			gap := to - from
			if gap > thresholdLayerGap {
				out = append(out, arch.Violation{
					Type:       arch.ViolationReverseDependency,
					Severity:   arch.SeverityWarning,
					RuleID:     "R4",
					File:       f.Name,
					FilePath:   f.Path,
					Message:    fmt.Sprintf("%s (%s) 直接依赖上层文件 %s (%s)，跨层数过多", f.Name, f.Layer, dep, targetLayer),
					Detail:     fmt.Sprintf("跨 %d 层依赖（阈值 %d），违反分层架构原则", gap, thresholdLayerGap),
					Consequence: "跨层跳跃会破坏分层约束，后续重构/替换成本极高",
					Suggestion: "通过下层的 Port/Adapter 间接调用，或下沉通用逻辑",
					Value:      gap,
					Threshold:  thresholdLayerGap,
				})
			}
		}
	}
	return out
}

// ──────────────────────────────────────────────
// R5 no_primitive
// ──────────────────────────────────────────────

var primitiveFileHints = []string{"atom", "port", "adapter", "composer", "step", "pipeline"}

func checkNoPrimitive(data *arch.ArchData) []arch.Violation {
	var out []arch.Violation

	// 已识别的四原语文件
	prims := map[string]bool{}
	for _, p := range data.Primitives {
		prims[p.File] = true
	}

	for _, f := range data.Files {
		if f.Name == "main.go" || strings.HasSuffix(f.Name, "_test.go") {
			continue
		}
		if prims[f.Name] {
			continue
		}
		if f.Layer == "L0" {
			continue
		}
		if len(f.Symbols) == 0 {
			continue
		}
		lowerName := strings.ToLower(f.Name)
		for _, hint := range primitiveFileHints {
			if strings.Contains(lowerName, hint) {
				out = append(out, arch.Violation{
					Type:       arch.ViolationMissingPrimitive,
					Severity:   arch.SeverityWarning,
					RuleID:     "R5",
					File:       f.Name,
					FilePath:   f.Path,
					Message:    fmt.Sprintf("文件 %s 命名暗示四原语，但未显式实现", f.Name),
					Detail:     "该文件存在导出符号，建议以 Atom/Port/Adapter/Composer 形式声明职责",
					Consequence: "隐式四原语难以被架构分析工具识别，长期积累会破坏可治理性",
					Suggestion: "使用四原语接口断言或显式命名（如 var _ arch.Port = MyPort{}）",
					Value:      len(f.Symbols),
				})
				break
			}
		}
	}
	return out
}

// ──────────────────────────────────────────────
// R6 large_function
// ──────────────────────────────────────────────

func checkLargeFunction(data *arch.ArchData) []arch.Violation {
	var out []arch.Violation
	for _, f := range data.Files {
		fset, file, err := parseGoFile(f.Path)
		if err != nil || file == nil {
			continue
		}
		ast.Inspect(file, func(n ast.Node) bool {
			fn, ok := n.(*ast.FuncDecl)
			if !ok {
				return true
			}
			lines := nodeLineSpan(fset, fn)
			if lines > thresholdFunctionLines {
				name := fnName(fn)
				out = append(out, arch.Violation{
					Type:       arch.ViolationType("large_function"),
					Severity:   arch.SeverityWarning,
					RuleID:     "R6",
					File:       f.Name,
					FilePath:   f.Path,
					Line:       fset.Position(fn.Pos()).Line,
					Message:    fmt.Sprintf("函数 %s 过长（%d 行）", name, lines),
					Detail:     fmt.Sprintf("该函数体跨 %d 行，超过阈值 %d", lines, thresholdFunctionLines),
					Consequence: "超长函数难以理解与测试，潜在分支风险高",
					Suggestion: "按语义拆分为多个子函数，并通过命名表达意图",
					Value:      lines,
					Threshold:  thresholdFunctionLines,
				})
			}
			return true
		})
	}
	return out
}

// ──────────────────────────────────────────────
// R7 raw_print
// ──────────────────────────────────────────────

func checkRawPrint(data *arch.ArchData) []arch.Violation {
	var out []arch.Violation
	for _, f := range data.Files {
		if strings.Contains(f.Name, "main.go") || strings.Contains(f.Name, "cli") {
			continue
		}
		fset, file, err := parseGoFile(f.Path)
		if err != nil || file == nil {
			continue
		}
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			name, pkg := selectorSplit(call.Fun)
			if pkg == "fmt" && (name == "Println" || name == "Print" || name == "Printf") {
				out = append(out, arch.Violation{
					Type:       arch.ViolationRawPrintln,
					Severity:   arch.SeverityWarning,
					RuleID:     "R7",
					File:       f.Name,
					FilePath:   f.Path,
					Line:       fset.Position(call.Pos()).Line,
					Message:    fmt.Sprintf("文件 %s 中存在直接 fmt.%s 调用", f.Name, name),
					Detail:     "业务代码应通过 Observation Pipeline 输出日志/事件，而非裸打印",
					Consequence: "裸打印难以被日志系统统一采集，且会污染标准输出",
					Suggestion: "替换为 Observation/Logger 接口或对应四原语适配器",
				})
			}
			return true
		})
	}
	return out
}

// ──────────────────────────────────────────────
// R8 hardcoded_config
// ──────────────────────────────────────────────

var hardcodedKeywords = []string{"timeout", "port", "path", "url", "key", "secret", "token"}

func checkHardcodedConfig(data *arch.ArchData) []arch.Violation {
	var out []arch.Violation
	for _, f := range data.Files {
		if strings.HasSuffix(f.Name, "_test.go") {
			continue
		}
		fset, file, err := parseGoFile(f.Path)
		if err != nil || file == nil {
			continue
		}
		ast.Inspect(file, func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			raw := strings.Trim(lit.Value, "\"`")
			if len(raw) <= thresholdHardcodedHintLen {
				return true
			}
			lower := strings.ToLower(raw)
			hit := ""
			for _, kw := range hardcodedKeywords {
				if strings.Contains(lower, kw) {
					hit = kw
					break
				}
			}
			if hit == "" {
				// 数字型端口 / 毫秒时间
				if strings.HasPrefix(raw, ":") {
					if _, err := strconv.Atoi(strings.TrimPrefix(raw, ":")); err == nil {
						hit = "port"
					}
				}
				if hit == "" && looksLikeNumberDuration(raw) {
					hit = "timeout"
				}
			}
			if hit == "" {
				return true
			}
			out = append(out, arch.Violation{
				Type:       arch.ViolationType("hardcoded_config"),
				Severity:   arch.SeverityInfo,
				RuleID:     "R8",
				File:       f.Name,
				FilePath:   f.Path,
				Line:       fset.Position(lit.Pos()).Line,
				Message:    fmt.Sprintf("存在潜在硬编码 %s：%s", hit, truncate(raw, 80)),
				Detail:     "配置项应从配置文件或环境变量读取",
				Consequence: "硬编码降低环境适配能力，易在生产环境引发故障",
				Suggestion: "通过 config/flags 统一管理配置值",
			})
			return true
		})
	}
	return out
}

func looksLikeNumberDuration(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	i := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	if i == 0 {
		return false
	}
	rest := strings.ToLower(strings.TrimSpace(s[i:]))
	for _, suffix := range []string{"s", "ms", "sec", "min", "h", "hour", "second"} {
		if rest == suffix {
			return true
		}
	}
	return false
}

// ──────────────────────────────────────────────
// R9 unchecked_error
// ──────────────────────────────────────────────

func checkUncheckedError(data *arch.ArchData) []arch.Violation {
	var out []arch.Violation
	for _, f := range data.Files {
		if strings.HasSuffix(f.Name, "_test.go") {
			continue
		}
		fset, file, err := parseGoFile(f.Path)
		if err != nil || file == nil {
			continue
		}
		ast.Inspect(file, func(n ast.Node) bool {
			asgn, ok := n.(*ast.AssignStmt)
			if !ok {
				return true
			}
			// 仅检测赋值右侧为单 CallExpr，且左值全部为 _ 的模式
			if len(asgn.Rhs) != 1 {
				return true
			}
			call, ok := asgn.Rhs[0].(*ast.CallExpr)
			if !ok {
				return true
			}
			if len(asgn.Lhs) < 2 {
				return true
			}
			allBlanks := true
			for _, lhs := range asgn.Lhs {
				if id, ok := lhs.(*ast.Ident); !ok || id.Name != "_" {
					allBlanks = false
					break
				}
			}
			if !allBlanks {
				return true
			}
			out = append(out, arch.Violation{
				Type:       arch.ViolationType("unchecked_error"),
				Severity:   arch.SeverityWarning,
				RuleID:     "R9",
				File:       f.Name,
				FilePath:   f.Path,
				Line:       fset.Position(asgn.Pos()).Line,
				Message:    fmt.Sprintf("忽略错误返回值：%s", truncate(exprStr(call), 80)),
				Detail:     "左值全部为 _，可能有意忽略错误",
				Consequence: "潜在的静默失败会导致难以排查的故障",
				Suggestion: "至少通过 errcheck / 显式变量判断，或使用 Observation 采集",
			})
			return true
		})
	}
	return out
}

// ──────────────────────────────────────────────
// R10 too_many_symbols
// ──────────────────────────────────────────────

func checkTooManySymbols(data *arch.ArchData) []arch.Violation {
	var out []arch.Violation
	for _, f := range data.Files {
		if len(f.Symbols) > thresholdSymbolsPerFile {
			out = append(out, arch.Violation{
				Type:       arch.ViolationType("too_many_symbols"),
				Severity:   arch.SeverityWarning,
				RuleID:     "R10",
				File:       f.Name,
				FilePath:   f.Path,
				Message:    fmt.Sprintf("文件 %s 定义了 %d 个符号，超出阈值", f.Name, len(f.Symbols)),
				Detail:     "单个文件符号过多，职责可能过大",
				Consequence: "文件内耦合度高，修改影响范围难以评估",
				Suggestion: "按领域拆分文件，使每个文件聚焦单一职责",
				Value:      len(f.Symbols),
				Threshold:  thresholdSymbolsPerFile,
			})
		}
	}
	return out
}

// ──────────────────────────────────────────────
// R11 global_state
// ──────────────────────────────────────────────

func checkGlobalState(data *arch.ArchData) []arch.Violation {
	var out []arch.Violation
	for _, f := range data.Files {
		if strings.HasSuffix(f.Name, "_test.go") {
			continue
		}
		fset, file, err := parseGoFile(f.Path)
		if err != nil || file == nil {
			continue
		}
		globals := 0
		var firstLine int
		for _, decl := range file.Decls {
			gd, ok := decl.(*ast.GenDecl)
			if !ok || gd.Tok != token.VAR {
				continue
			}
			for _, spec := range gd.Specs {
				vs, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for _, name := range vs.Names {
					if name.Name == "_" {
						continue
					}
					// 排除 interface 断言（仅类型无值的 var 不算状态）
					if vs.Values == nil {
						continue
					}
					globals++
					if firstLine == 0 {
						firstLine = fset.Position(name.Pos()).Line
					}
				}
			}
		}
		if globals > thresholdGlobalVars {
			out = append(out, arch.Violation{
				Type:       arch.ViolationType("global_state"),
				Severity:   arch.SeverityWarning,
				RuleID:     "R11",
				File:       f.Name,
				FilePath:   f.Path,
				Line:       firstLine,
				Message:    fmt.Sprintf("文件 %s 定义了 %d 个包级 var（> %d）", f.Name, globals, thresholdGlobalVars),
				Detail:     "过多全局状态可能导致并发与初始化问题",
				Consequence: "隐藏的可变全局状态会破坏模块独立性，难以测试",
				Suggestion: "将状态封装为依赖注入对象或使用单例工厂",
				Value:      globals,
				Threshold:  thresholdGlobalVars,
			})
		}
	}
	return out
}

// ──────────────────────────────────────────────
// R12 low_test_coverage
// ──────────────────────────────────────────────

func checkLowTestCoverage(data *arch.ArchData) []arch.Violation {
	var out []arch.Violation
	testsByDir := map[string]bool{}
	sourceByDir := map[string][]string{}
	for _, f := range data.Files {
		dir := filepath.Dir(f.Path)
		if strings.HasSuffix(f.Name, "_test.go") {
			testsByDir[dir] = true
			continue
		}
		if f.Name == "main.go" {
			continue
		}
		sourceByDir[dir] = append(sourceByDir[dir], f.Name)
	}
	for dir, names := range sourceByDir {
		if testsByDir[dir] {
			continue
		}
		for _, n := range names {
			out = append(out, arch.Violation{
				Type:       arch.ViolationType("low_test_coverage"),
				Severity:   arch.SeverityInfo,
				RuleID:     "R12",
				File:       n,
				FilePath:   filepath.Join(dir, n),
				Message:    fmt.Sprintf("目录 %s 下无 _test.go 文件", dir),
				Detail:     "业务文件未配套测试文件",
				Consequence: "缺乏测试保障会使后续重构和维护存在风险",
				Suggestion: "为关键业务路径增加单元测试或基于四原语的集成测试",
			})
		}
	}
	return out
}

// ──────────────────────────────────────────────
// R13 missing_doc
// ──────────────────────────────────────────────

func checkMissingDoc(data *arch.ArchData) []arch.Violation {
	var out []arch.Violation
	for _, f := range data.Files {
		if strings.HasSuffix(f.Name, "_test.go") {
			continue
		}
		for _, s := range f.Symbols {
			if !s.IsExported {
				continue
			}
			if strings.TrimSpace(s.Doc) != "" {
				continue
			}
			out = append(out, arch.Violation{
				Type:       arch.ViolationType("missing_doc"),
				Severity:   arch.SeverityInfo,
				RuleID:     "R13",
				File:       f.Name,
				FilePath:   f.Path,
				Message:    fmt.Sprintf("导出符号 %s 缺少文档注释", s.Name),
				Detail:     fmt.Sprintf("类型：%s；签名：%s", s.Kind, truncate(s.Signature, 80)),
				Consequence: "外部调用方难以理解正确用法与边界条件",
				Suggestion: "补充一句描述用途、输入输出与边界条件的注释",
			})
		}
	}
	return out
}

// ──────────────────────────────────────────────
// R14 high_cyclomatic
// ──────────────────────────────────────────────

func checkHighCyclomatic(data *arch.ArchData) []arch.Violation {
	var out []arch.Violation
	for _, f := range data.Files {
		fset, file, err := parseGoFile(f.Path)
		if err != nil || file == nil {
			continue
		}
		ast.Inspect(file, func(n ast.Node) bool {
			fn, ok := n.(*ast.FuncDecl)
			if !ok {
				return true
			}
			complexity := 1
			ast.Inspect(fn.Body, func(sub ast.Node) bool {
				switch sub.(type) {
				case *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt,
					*ast.SwitchStmt, *ast.TypeSwitchStmt, *ast.SelectStmt,
					*ast.CaseClause, *ast.CommClause:
					complexity++
				case *ast.BinaryExpr:
					be := sub.(*ast.BinaryExpr)
					if be.Op == token.LAND || be.Op == token.LOR {
						complexity++
					}
				}
				return true
			})
			if complexity > thresholdCyclomatic {
				out = append(out, arch.Violation{
					Type:       arch.ViolationType("high_cyclomatic"),
					Severity:   arch.SeverityWarning,
					RuleID:     "R14",
					File:       f.Name,
					FilePath:   f.Path,
					Line:       fset.Position(fn.Pos()).Line,
					Message:    fmt.Sprintf("函数 %s 圈复杂度 %d 过高", fnName(fn), complexity),
					Detail:     "包含过多分支/循环/开关语句，认知负担大",
					Consequence: "高圈复杂度函数测试路径爆炸，易留缺陷",
					Suggestion: "抽取语义清晰的子函数，合并重复分支逻辑",
					Value:      complexity,
					Threshold:  thresholdCyclomatic,
				})
			}
			return true
		})
	}
	return out
}

// ──────────────────────────────────────────────
// R15 high_dependency_centrality
// ──────────────────────────────────────────────

func checkHighDependencyCentrality(data *arch.ArchData) []arch.Violation {
	var out []arch.Violation
	for _, f := range data.Files {
		count := len(f.DependedBy)
		if count > thresholdDependedBy {
			out = append(out, arch.Violation{
				Type:       arch.ViolationType("high_dependency_centrality"),
				Severity:   arch.SeverityWarning,
				RuleID:     "R15",
				File:       f.Name,
				FilePath:   f.Path,
				Message:    fmt.Sprintf("文件 %s 被 %d 个文件依赖，可能承担过重职责", f.Name, count),
				Detail:     "中心度高的文件修改影响面极大，是架构关键节点",
				Consequence: "此类文件一旦出现故障会放大影响，单点风险高",
				Suggestion: "将通用能力下沉为共享四原语，或按职责拆分",
				Value:      count,
				Threshold:  thresholdDependedBy,
			})
		}
	}
	return out
}

// ──────────────────────────────────────────────
// AST 辅助
// ──────────────────────────────────────────────

func parseGoFile(path string) (*token.FileSet, *ast.File, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, nil, err
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		return nil, nil, err
	}
	return fset, file, nil
}

func nodeLineSpan(fset *token.FileSet, n ast.Node) int {
	start := fset.Position(n.Pos()).Line
	end := fset.Position(n.End()).Line
	if end < start {
		return 1
	}
	return end - start + 1
}

func fnName(fn *ast.FuncDecl) string {
	if fn.Recv != nil && len(fn.Recv.List) > 0 {
		recv := exprStr(fn.Recv.List[0].Type)
		return fmt.Sprintf("(%s).%s", recv, fn.Name.Name)
	}
	return fn.Name.Name
}

func selectorSplit(n ast.Node) (name, pkg string) {
	sel, ok := n.(*ast.SelectorExpr)
	if !ok {
		if id, ok := n.(*ast.Ident); ok {
			return id.Name, ""
		}
		return "", ""
	}
	if id, ok := sel.X.(*ast.Ident); ok {
		return sel.Sel.Name, id.Name
	}
	return sel.Sel.Name, ""
}

func exprStr(n ast.Node) string {
	if n == nil {
		return ""
	}
	switch v := n.(type) {
	case *ast.CallExpr:
		return fmt.Sprintf("%s(...)", exprStr(v.Fun))
	case *ast.SelectorExpr:
		return fmt.Sprintf("%s.%s", exprStr(v.X), v.Sel.Name)
	case *ast.Ident:
		return v.Name
	case *ast.StarExpr:
		return "*" + exprStr(v.X)
	}
	return "<expr>"
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
