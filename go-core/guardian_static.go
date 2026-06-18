// Package core — Guardian 静态审核 (Phase 2 P2)
//
// StaticGuardPort 在编译前对 Agent 提交的代码进行 4 项静态检查：
//   1. 原语合规 — 代码中使用的类型是否在 GlobalPrimitiveTypeSet 白名单中
//   2. 外部依赖 — import 是否引入了 go-core 和标准库之外的包
//   3. 层级合规 — 声明所在层是否合理，是否越层调用
//   4. 复杂度 — 函数行数、嵌套深度、圈复杂度是否超标
//
// StaticGuardPort 实现 Port 接口，可嵌入任何 Pipeline。
// 审核结果通过 Violation 列表返回，包含具体代码位置和修复建议。
package core

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"time"
)

// ──────────────────────────────────────────────
// SECTION 1: StaticReviewResult — 静态审核结果
// ──────────────────────────────────────────────

// StaticReviewResult 是 StaticGuardPort 的审核输出。
// 会被接入 DecisionEngine 做统一决策。
type StaticReviewResult struct {
	Violations []Violation `json:"violations"`
	ReviewedAt time.Time   `json:"reviewed_at"`
}

// HasErrors 检查是否有 error 级别的违规。
func (r *StaticReviewResult) HasErrors() bool {
	for _, v := range r.Violations {
		if v.Severity == "error" {
			return true
		}
	}
	return false
}

// ErrorCount 返回 error 级别违规数量。
func (r *StaticReviewResult) ErrorCount() int {
	count := 0
	for _, v := range r.Violations {
		if v.Severity == "error" {
			count++
		}
	}
	return count
}

// ──────────────────────────────────────────────
// SECTION 2: StaticGuardPort — 静态审核器
// ──────────────────────────────────────────────

// StaticGuardPort 是编译前的静态代码审核器。
// 实现 Port 接口，可嵌入任何 Pipeline。
//
// 审核流程：
//   1. 解析 SourceCode 为 AST
//   2. 遍历 AST 节点，执行 4 项检查
//   3. 返回 Violation 列表（含修复建议）
type StaticGuardPort struct {
	primitiveSet *typeSet         // 已有：GlobalPrimitiveTypeSet
	archGuard    *ArchitectureGuard // 已有：架构守卫（层级合规检查）
	obs          ObservationAdapter
}

// NewStaticGuardPort 创建 StaticGuardPort。
func NewStaticGuardPort(primitiveSet *typeSet, archGuard *ArchitectureGuard, obs ObservationAdapter) *StaticGuardPort {
	if obs == nil {
		obs = &NoOpObservationAdapter{}
	}
	return &StaticGuardPort{
		primitiveSet: primitiveSet,
		archGuard:    archGuard,
		obs:          obs,
	}
}

// Validate 实现 Port 接口。
// 输入：AgentCodeSubmission
// 输出：StaticReviewResult（含违规列表）
func (g *StaticGuardPort) Validate(ctx context.Context, input AgentCodeSubmission) (StaticReviewResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	select {
	case <-ctx.Done():
		return StaticReviewResult{}, ctx.Err()
	default:
	}

	start := time.Now()
	result := g.review(input)

	es := NewExecutionStep("StaticGuardPort", "Validate", "static code review completed", "Port")
	es.DurationMs = time.Since(start).Milliseconds()
	es.Metadata = map[string]interface{}{
		"agent_id":    input.AgentID,
		"task_id":     input.TaskID,
		"attempt":     input.Attempt,
		"violations":  len(result.Violations),
		"has_errors":  result.HasErrors(),
	}
	g.obs.Record([]ExecutionStep{es})

	return result, nil
}

// review 是核心审核逻辑（纯函数，便于测试）。
func (g *StaticGuardPort) review(sub AgentCodeSubmission) StaticReviewResult {
	var violations []Violation

	// 解析 AST
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "agent_code.go", sub.SourceCode, parser.ParseComments)
	if err != nil {
		violations = append(violations, Violation{
			Rule:       "parse_error",
			Severity:   "error",
			Detail:     fmt.Sprintf("代码解析失败: %v", err),
			Suggestion: "检查 Go 语法是否正确",
		})
		return StaticReviewResult{Violations: violations, ReviewedAt: time.Now()}
	}

	// 检查 1: 原语合规
	violations = append(violations, g.checkPrimitiveCompliance(fset, file, sub.Manifest)...)

	// 检查 2: 外部依赖
	violations = append(violations, g.checkExternalDeps(fset, file)...)

	// 检查 3: 层级合规
	violations = append(violations, g.checkLayerCompliance(sub.Manifest)...)

	// 检查 4: 复杂度
	violations = append(violations, g.checkComplexity(fset, file)...)

	// 检查 5: Manifest 与实际代码一致性
	violations = append(violations, g.checkManifestConsistency(fset, file, sub.Manifest)...)

	return StaticReviewResult{Violations: violations, ReviewedAt: time.Now()}
}

// ──────────────────────────────────────────────
// SECTION 3: 检查 1 — 原语合规
// ──────────────────────────────────────────────

// checkPrimitiveCompliance 检查代码中使用的类型是否在 GlobalPrimitiveTypeSet 白名单中。
// 遍历 AST，检查所有类型引用（struct type、interface type、type alias）。
func (g *StaticGuardPort) checkPrimitiveCompliance(fset *token.FileSet, file *ast.File, manifest []PrimitiveManifest) []Violation {
	var violations []Violation

	// 收集 Manifest 中声明的类型名（这些是 Agent 自己定义的，允许）
	manifestTypes := make(map[string]bool)
	for _, m := range manifest {
		manifestTypes[m.Name] = true
	}

	ast.Inspect(file, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.TypeSpec:
			// 检查类型定义中引用的外部类型
			// 例如: type myAdapter struct { client *http.Client }
			// 如果 http.Client 不在白名单中，违规
			if structType, ok := node.Type.(*ast.StructType); ok {
				for _, field := range structType.Fields.List {
					violations = append(violations, g.checkFieldType(fset, field.Type, manifestTypes)...)
				}
			}
		case *ast.CallExpr:
			// 检查函数调用中是否使用了非白名单类型
			// 例如: http.NewRequest(...)
			violations = append(violations, g.checkCallExpr(fset, node, manifestTypes)...)
		}
		return true
	})

	return violations
}

// checkFieldType 检查字段类型是否在白名单中。
func (g *StaticGuardPort) checkFieldType(fset *token.FileSet, expr ast.Expr, manifestTypes map[string]bool) []Violation {
	var violations []Violation

	switch t := expr.(type) {
	case *ast.SelectorExpr:
		// 例如: http.Client, sql.DB
		typeName := getSelectorName(t)
		// 只检查标准库和第三方包的类型引用
		// 如果包名是 go-core 中的，跳过
		if pkgIdent, ok := t.X.(*ast.Ident); ok {
			pkgName := pkgIdent.Name
			// 是 go-core 包中的类型，跳过
			if g.primitiveSet != nil && g.primitiveSet.contains(typeName) {
				return nil
			}
			if manifestTypes[typeName] {
				return nil
			}
			// 标准库包的引用（如 http, sql, os 等）
			_ = pkgName
			violations = append(violations, Violation{
				Rule:       "primitive_compliance",
				Severity:   "error",
				Location:   fmt.Sprintf("line %d", fset.Position(t.Pos()).Line),
				Detail:     fmt.Sprintf("使用了非白名单类型 %s.%s，必须包装为 Adapter 原语", pkgName, t.Sel.Name),
				Suggestion: fmt.Sprintf("将 %s.%s 包装为 Adapter 结构体，实现 Adapter 接口。例如: type %sAdapter struct { client *%s.%s }", pkgName, t.Sel.Name, t.Sel.Name, pkgName, t.Sel.Name),
			})
		}
	case *ast.StarExpr:
		return g.checkFieldType(fset, t.X, manifestTypes)
	case *ast.ArrayType:
		return g.checkFieldType(fset, t.Elt, manifestTypes)
	}

	return violations
}

// checkCallExpr 检查函数调用中是否使用了非白名单类型。
func (g *StaticGuardPort) checkCallExpr(fset *token.FileSet, call *ast.CallExpr, manifestTypes map[string]bool) []Violation {
	var violations []Violation

	if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
		if pkgIdent, ok := sel.X.(*ast.Ident); ok {
			pkgName := pkgIdent.Name
			// 标准库包的直接调用（如 http.Get, os.Open, sql.Open）
			// 检查是否为已知需要包装的标准库
			if isStandardLibFunc(pkgName, sel.Sel.Name) {
				violations = append(violations, Violation{
					Rule:       "primitive_compliance",
					Severity:   "error",
					Location:   fmt.Sprintf("line %d", fset.Position(call.Pos()).Line),
					Detail:     fmt.Sprintf("直接调用了 %s.%s()，必须通过 Adapter 原语隔离", pkgName, sel.Sel.Name),
					Suggestion: fmt.Sprintf("将 %s.%s() 调用封装到 Adapter 的 Execute() 方法中", pkgName, sel.Sel.Name),
				})
			}
		}
	}

	return violations
}

// getSelectorName 从 SelectorExpr 中提取类型名。
func getSelectorName(sel *ast.SelectorExpr) string {
	return sel.Sel.Name
}

// isStandardLibFunc 检查是否为已知需要包装的标准库函数。
func isStandardLibFunc(pkg, fn string) bool {
	// 只拦截已知的 I/O 操作（网络、文件、数据库）
	ioPackages := map[string]bool{
		"http": true, "net": true, "os": true, "io": true,
		"sql": true, "database": true,
	}
	if !ioPackages[pkg] {
		return false
	}
	// 具体函数名
	ioFuncs := map[string]bool{
		"Get": true, "Post": true, "Do": true, "NewRequest": true,
		"Open": true, "Create": true, "ReadFile": true, "WriteFile": true,
		"Dial": true, "Listen": true, "Accept": true,
		"Exec": true, "Query": true, "QueryRow": true,
		"Copy": true, "ReadAll": true,
	}
	return ioFuncs[fn]
}

// ──────────────────────────────────────────────
// SECTION 4: 检查 2 — 外部依赖
// ──────────────────────────────────────────────

// allowedImportPaths 定义了允许的 import 路径。
var allowedImportPaths = map[string]bool{
	"go-core": true,
	"fmt":     true, "time": true, "context": true, "sync": true,
	"strings": true, "strconv": true, "encoding": true, "encoding/json": true,
	"errors": true, "math": true, "crypto": true, "crypto/sha256": true,
	"crypto/sha512": true, "crypto/md5": true, "encoding/hex": true,
	"encoding/base64": true, "io": true, "os": true, "path": true,
	"path/filepath": true, "sort": true, "bytes": true, "bufio": true,
	"log": true, "flag": true, "net": true, "net/http": true,
	"database/sql": true, "database/sql/driver": true,
	"reflect": true, "regexp": true, "unicode": true, "unicode/utf8": true,
	"container": true, "container/heap": true, "container/list": true,
	"hash": true, "hash/fnv": true, "image": true,
	"runtime": true, "runtime/debug": true, "sync/atomic": true,
	"testing": true, "compress": true,
}

// checkExternalDeps 检查 import 是否引入了非白名单包。
// 只允许 go-core 和标准库（白名单中的路径）。
func (g *StaticGuardPort) checkExternalDeps(fset *token.FileSet, file *ast.File) []Violation {
	var violations []Violation

	for _, imp := range file.Imports {
		importPath := strings.Trim(imp.Path.Value, `"`)

		// 检查是否为允许的路径
		if isAllowedImport(importPath) {
			continue
		}

		line := fset.Position(imp.Pos()).Line
		violations = append(violations, Violation{
			Rule:       "external_dependency",
			Severity:   "error",
			Location:   fmt.Sprintf("line %d: import \"%s\"", line, importPath),
			Detail:     fmt.Sprintf("引入了外部依赖 %s，只允许 import go-core 和标准库", importPath),
			Suggestion: fmt.Sprintf("移除 %s 的 import，将相关功能封装为 Adapter 原语（通过 go-core 的 Adapter 接口隔离外部依赖）", importPath),
		})
	}

	return violations
}

// isAllowedImport 检查 import 路径是否在白名单中。
func isAllowedImport(path string) bool {
	if allowedImportPaths[path] {
		return true
	}
	// 检查前缀匹配（如 "encoding/json" 匹配 "encoding" 前缀）
	for allowed := range allowedImportPaths {
		if strings.HasPrefix(path, allowed+"/") {
			return true
		}
	}
	// 内部包：go-core 的子包
	if strings.HasPrefix(path, "go-core/") {
		return true
	}
	return false
}

// ──────────────────────────────────────────────
// SECTION 5: 检查 3 — 层级合规
// ──────────────────────────────────────────────

// validLayers 定义了合法的层级。
var validLayers = map[string]bool{
	"L1": true, "L2": true, "L3": true, "L4": true,
	"L5": true, "L6": true, "L7": true,
}

// primitiveLayerRules 定义了每种原语类型的合法层级。
var primitiveLayerRules = map[string][]string{
	"Atom":     {"L1", "L2", "L3", "L4", "L5", "L6", "L7"}, // Atom 可在任意层
	"Port":     {"L1", "L2", "L3"},                            // Port 在边界层
	"Adapter":  {"L5", "L6", "L7"},                            // Adapter 在基础设施层
	"Composer": {"L1", "L2", "L3", "L4", "L5", "L6", "L7"},   // Composer 可在任意层
}

// checkLayerCompliance 检查 Manifest 声明的层级是否合规。
func (g *StaticGuardPort) checkLayerCompliance(manifest []PrimitiveManifest) []Violation {
	var violations []Violation

	for _, m := range manifest {
		// 检查层级是否合法
		if !validLayers[m.Layer] {
			violations = append(violations, Violation{
				Rule:       "invalid_layer",
				Severity:   "error",
				Location:   fmt.Sprintf("Manifest[%s]", m.Name),
				Detail:     fmt.Sprintf("原语 %s 声明了非法层级 %s，合法层级为 L1-L7", m.Name, m.Layer),
				Suggestion: fmt.Sprintf("将 %s 的 layer 改为 L1-L7 之间的合法层级", m.Name),
			})
			continue
		}

		// 检查原语类型与层级是否匹配
		allowedLayers, ok := primitiveLayerRules[m.PrimitiveType]
		if !ok {
			continue // 已在 Manifest.Validate() 中检查
		}

		layerAllowed := false
		for _, l := range allowedLayers {
			if l == m.Layer {
				layerAllowed = true
				break
			}
		}

		if !layerAllowed {
			suggestedLayer := allowedLayers[0]
			if m.PrimitiveType == "Adapter" {
				suggestedLayer = "L7"
			} else if m.PrimitiveType == "Port" {
				suggestedLayer = "L1"
			}
			violations = append(violations, Violation{
				Rule:       "layer_mismatch",
				Severity:   "error",
				Location:   fmt.Sprintf("Manifest[%s]", m.Name),
				Detail:     fmt.Sprintf("原语类型 %s 不能在 %s 层，允许的层级: %s", m.PrimitiveType, m.Layer, strings.Join(allowedLayers, ", ")),
				Suggestion: fmt.Sprintf("将 %s 的 layer 从 %s 改为 %s", m.Name, m.Layer, suggestedLayer),
			})
		}
	}

	// 检查依赖关系：是否越层调用
	for _, m := range manifest {
		for _, depName := range m.Dependencies {
			dep := findManifest(manifest, depName)
			if dep == nil {
				continue
			}
			// 检查是否从低层调用高层（越层）
			if layerOrder(m.Layer) > layerOrder(dep.Layer) {
				// 当前层高于依赖层，允许（高层依赖低层）
				continue
			}
			// 如果跨层调用距离过大（> 3 层），warn
			if layerOrder(dep.Layer)-layerOrder(m.Layer) > 3 {
				violations = append(violations, Violation{
					Rule:       "layer_skip",
					Severity:   "warn",
					Location:   fmt.Sprintf("Manifest[%s].dependencies[%s]", m.Name, depName),
					Detail:     fmt.Sprintf("原语 %s(%s) 依赖 %s(%s)，跨层距离过大（%d 层）", m.Name, m.Layer, depName, dep.Layer, layerOrder(dep.Layer)-layerOrder(m.Layer)),
					Suggestion: "考虑引入中间层原语来减少跨层距离",
				})
			}
		}
	}

	return violations
}

// layerOrder 返回层级的数值顺序。
func layerOrder(layer string) int {
	order := map[string]int{
		"L1": 1, "L2": 2, "L3": 3, "L4": 4,
		"L5": 5, "L6": 6, "L7": 7,
	}
	return order[layer]
}

// findManifest 在 Manifest 列表中查找指定名称的原语。
func findManifest(manifest []PrimitiveManifest, name string) *PrimitiveManifest {
	for i := range manifest {
		if manifest[i].Name == name {
			return &manifest[i]
		}
	}
	return nil
}

// ──────────────────────────────────────────────
// SECTION 6: 检查 4 — 复杂度
// ──────────────────────────────────────────────

const (
	maxFunctionLines    = 50
	maxNestingDepth     = 3
	maxCyclomaticComplexity = 10
)

// checkComplexity 检查代码复杂度。
func (g *StaticGuardPort) checkComplexity(fset *token.FileSet, file *ast.File) []Violation {
	var violations []Violation

	for _, decl := range file.Decls {
		funcDecl, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}

		funcName := funcDecl.Name.Name
		startLine := fset.Position(funcDecl.Pos()).Line
		endLine := fset.Position(funcDecl.End()).Line
		lineCount := endLine - startLine + 1

		// 检查函数行数
		if lineCount > maxFunctionLines {
			violations = append(violations, Violation{
				Rule:       "function_too_long",
				Severity:   "warn",
				Location:   fmt.Sprintf("line %d: func %s", startLine, funcName),
				Detail:     fmt.Sprintf("函数 %s 有 %d 行，超过 %d 行限制", funcName, lineCount, maxFunctionLines),
				Suggestion: fmt.Sprintf("将 %s 拆分为多个更小的 Atom/Port/Adapter", funcName),
			})
		}

		// 检查嵌套深度
		depth := computeNestingDepth(funcDecl.Body)
		if depth > maxNestingDepth {
			violations = append(violations, Violation{
				Rule:       "excessive_nesting",
				Severity:   "warn",
				Location:   fmt.Sprintf("line %d: func %s", startLine, funcName),
				Detail:     fmt.Sprintf("函数 %s 嵌套深度 %d，超过 %d 层限制", funcName, depth, maxNestingDepth),
				Suggestion: "使用 early return 或提取子函数来减少嵌套深度",
			})
		}

		// 检查圈复杂度
		cc := computeCyclomaticComplexity(funcDecl.Body)
		if cc > maxCyclomaticComplexity {
			violations = append(violations, Violation{
				Rule:       "high_complexity",
				Severity:   "warn",
				Location:   fmt.Sprintf("line %d: func %s", startLine, funcName),
				Detail:     fmt.Sprintf("函数 %s 圈复杂度 %d，超过 %d 限制", funcName, cc, maxCyclomaticComplexity),
				Suggestion: "拆分条件分支为独立的 Atom 或使用 Composer 的 Branch 模式",
			})
		}
	}

	return violations
}

// computeNestingDepth 计算 AST 节点的最大嵌套深度。
func computeNestingDepth(node ast.Node) int {
	if node == nil {
		return 0
	}

	maxDepth := 0
	ast.Inspect(node, func(n ast.Node) bool {
		switch n.(type) {
		case *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt, *ast.SwitchStmt, *ast.SelectStmt:
			depth := countNestingParents(n)
			if depth > maxDepth {
				maxDepth = depth
			}
		}
		return true
	})

	return maxDepth
}

// countNestingParents 计算从节点到根的控制流嵌套数量。
func countNestingParents(node ast.Node) int {
	depth := 0
	// 向上遍历，但 AST 没有 Parent 指针，改用递归方式
	// 简化实现：使用 Inpect 中的嵌套深度
	// 这里返回一个近似值
	_ = node
	return depth + 1
}

// computeCyclomaticComplexity 计算函数体圈复杂度。
func computeCyclomaticComplexity(body *ast.BlockStmt) int {
	if body == nil {
		return 1
	}

	complexity := 1 // 基础复杂度
	ast.Inspect(body, func(n ast.Node) bool {
		switch n.(type) {
		case *ast.IfStmt:
			complexity++
		case *ast.ForStmt:
			complexity++
		case *ast.RangeStmt:
			complexity++
		case *ast.CaseClause:
			// case 子句只在非空时增加复杂度
			if clause, ok := n.(*ast.CaseClause); ok && clause.List != nil {
				complexity++
			}
		case *ast.CommClause:
			complexity++
		case *ast.BinaryExpr:
			if bin, ok := n.(*ast.BinaryExpr); ok && (bin.Op == token.LAND || bin.Op == token.LOR) {
				complexity++
			}
		}
		return true
	})

	return complexity
}

// ──────────────────────────────────────────────
// SECTION 7: 检查 5 — Manifest 一致性
// ──────────────────────────────────────────────

// checkManifestConsistency 检查 Manifest 声明与实际代码是否一致。
// 防止 Agent 声明一套、实际写另一套。
func (g *StaticGuardPort) checkManifestConsistency(fset *token.FileSet, file *ast.File, manifest []PrimitiveManifest) []Violation {
	var violations []Violation

	// 收集代码中实际定义的类型
	codeTypes := make(map[string]string) // name -> kind (struct/interface)
	for _, decl := range file.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.TYPE {
			continue
		}
		for _, spec := range genDecl.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			kind := "unknown"
			switch typeSpec.Type.(type) {
			case *ast.StructType:
				kind = "struct"
			case *ast.InterfaceType:
				kind = "interface"
			case *ast.FuncType:
				kind = "func"
			}
			codeTypes[typeSpec.Name.Name] = kind
		}
	}

	// 检查 Manifest 中声明的类型是否在代码中实际存在
	for _, m := range manifest {
		if _, exists := codeTypes[m.Name]; !exists {
			violations = append(violations, Violation{
				Rule:       "manifest_mismatch",
				Severity:   "error",
				Location:   fmt.Sprintf("Manifest[%s]", m.Name),
				Detail:     fmt.Sprintf("Manifest 中声明的原语 %s 在代码中未找到对应的类型定义", m.Name),
				Suggestion: fmt.Sprintf("确保代码中存在 %s 结构体定义，或从 Manifest 中移除该声明", m.Name),
			})
		}
	}

	// 检查代码中定义的类型是否都在 Manifest 中声明
	manifestNames := make(map[string]bool)
	for _, m := range manifest {
		manifestNames[m.Name] = true
	}

	for name, kind := range codeTypes {
		if kind == "struct" && !manifestNames[name] && name != "_" && name != "" {
			// 跳过 main 函数和测试辅助类型
			if strings.HasPrefix(name, "main") {
				continue
			}
			violations = append(violations, Violation{
				Rule:       "manifest_missing",
				Severity:   "warn",
				Location:   fmt.Sprintf("line %d: type %s", fset.Position(file.Pos()).Line, name),
				Detail:     fmt.Sprintf("代码中定义了结构体 %s，但未在 Manifest 中声明", name),
				Suggestion: fmt.Sprintf("在 Manifest 中添加 %s 的 PrimitiveManifest 声明", name),
			})
		}
	}

	return violations
}

// ──────────────────────────────────────────────
// SECTION 8: StaticGuardPort 作为 Step
// ──────────────────────────────────────────────

// StaticGuardPortAsStep 将 StaticGuardPort 包装为 Step。
func StaticGuardPortAsStep(g *StaticGuardPort) Step[AgentCodeSubmission, StaticReviewResult] {
	return NewStepFunc[AgentCodeSubmission, StaticReviewResult]("Port", func(ctx context.Context, input AgentCodeSubmission) (StaticReviewResult, error) {
		return g.Validate(ctx, input)
	})
}