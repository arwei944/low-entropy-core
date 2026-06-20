package main

import (
	"go/ast"
	"go/token"
)

// =========================================================================
// Rule 2 – No I/O operations in Atom / Port function bodies
// =========================================================================
//
// Identifies methods whose receiver is of type Atom or Port (value or
// pointer).  Walks the function body with ast.Inspect and flags any
// SelectorExpr call that matches a known I/O function.

func checkRule2_NoIOInAtomPort(f *ast.File, fset *token.FileSet, path string) {
	for _, decl := range f.Decls {
		funcDecl, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		if !hasAtomOrPortReceiver(funcDecl) {
			continue
		}
		if funcDecl.Body == nil {
			continue
		}

		receiverName := receiverTypeName(funcDecl)

		ast.Inspect(funcDecl.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			pkgIdent, ok := sel.X.(*ast.Ident)
			if !ok {
				return true
			}
			funcName := sel.Sel.Name

			if funcs, knownPkg := ioFuncsByPkg[pkgIdent.Name]; knownPkg {
				if funcs[funcName] {
					pos := fset.Position(call.Pos())
					report("ERROR", path, pos.Line,
						"I/O operation in %s receiver: %s.%s()", receiverName,
						pkgIdent.Name, funcName)
				}
			}
			return true
		})
	}
}

// hasAtomOrPortReceiver returns true when the function receiver type is
// Atom or Port (either value receiver or pointer receiver).
func hasAtomOrPortReceiver(fn *ast.FuncDecl) bool {
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return false
	}
	return isAtomOrPortType(fn.Recv.List[0].Type)
}

// isAtomOrPortType checks a type expression for Atom or Port (with/without *).
func isAtomOrPortType(expr ast.Expr) bool {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name == "Atom" || t.Name == "Port"
	case *ast.StarExpr:
		if ident, ok := t.X.(*ast.Ident); ok {
			return ident.Name == "Atom" || ident.Name == "Port"
		}
	}
	return false
}

// receiverTypeName returns a human-readable representation of the receiver type.
func receiverTypeName(fn *ast.FuncDecl) string {
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return "<none>"
	}
	return exprToString(fn.Recv.List[0].Type)
}
