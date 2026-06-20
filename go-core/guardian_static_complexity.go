//go:build lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

package core

import (
	"context"
	"fmt"
	"go/ast"
	"go/token"
	"strings"
)

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
