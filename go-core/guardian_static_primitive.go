//go:build lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

package core

import (
	"fmt"
	"go/ast"
	"go/token"
)

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
