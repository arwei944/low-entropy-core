package main

import (
	"go/ast"
	"go/token"
)

// =========================================================================
// Rule 4 – Pipeline / Composer steps must be <= 20 (WARNING)
// =========================================================================
//
// Scans composite literals (CompositeLit) whose type is Pipeline or []Step.
// If the literal has more than 20 elements, a WARNING is emitted.

func checkRule4_PipelineStepLimit(f *ast.File, fset *token.FileSet, path string) {
	ast.Inspect(f, func(n ast.Node) bool {
		compLit, ok := n.(*ast.CompositeLit)
		if !ok {
			return true
		}

		if !isPipelineOrStepSliceType(compLit.Type) {
			return true
		}

		stepCount := len(compLit.Elts)
		if stepCount > 20 {
			pos := fset.Position(compLit.Pos())
			report("WARNING", path, pos.Line,
				"Pipeline/Composer has %d steps (limit is 20)", stepCount)
		}
		return true
	})
}

// isPipelineOrStepSliceType returns true when the type expression is
// "Pipeline" or "[]Step" (the two ways steps can be collected).
func isPipelineOrStepSliceType(expr ast.Expr) bool {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name == "Pipeline"
	case *ast.SelectorExpr:
		// e.g. pkg.Pipeline
		return t.Sel.Name == "Pipeline"
	case *ast.ArrayType:
		if t.Len != nil {
			return false // fixed-size array [N]Step – unusual but possible
		}
		if ident, ok := t.Elt.(*ast.Ident); ok {
			return ident.Name == "Step"
		}
	}
	return false
}
