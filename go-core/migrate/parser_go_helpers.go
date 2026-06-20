package migrate

import (
	"fmt"
	"go/ast"
	"os"
	"strings"
)

// --- helper functions ---

// callName extracts the function name from a CallExpr.
func callName(expr *ast.CallExpr) string {
	switch fun := expr.Fun.(type) {
	case *ast.Ident:
		return fun.Name
	case *ast.SelectorExpr:
		return fun.Sel.Name
	default:
		return ""
	}
}

// isIOCall checks whether a CallExpr is an I/O operation from known I/O packages.
func isIOCall(expr *ast.CallExpr) bool {
	sel, ok := expr.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	ident, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}
	pkg := ident.Name
	switch pkg {
	case "os", "fmt", "io", "log", "net", "http", "database", "sql":
		return true
	}
	// Handle "database/sql" style: X is "sql" from "database/sql" import.
	if pkg == "sql" {
		return true
	}
	// Handle "net/http" style: X is "http" from "net/http" import.
	if pkg == "http" {
		return true
	}
	return false
}

// isBuiltin returns true if the name is a Go built-in function.
func isBuiltin(name string) bool {
	switch name {
	case "append", "len", "cap", "make", "new", "close",
		"delete", "panic", "recover", "print", "println",
		"copy", "complex", "real", "imag":
		return true
	}
	return false
}

// isStdLib returns true if the import path belongs to the Go standard library.
func isStdLib(path string) bool {
	// Standard library paths do not contain a dot (no domain).
	// They also don't start with a dot or slash.
	if path == "" {
		return false
	}
	parts := strings.Split(path, "/")
	if strings.Contains(parts[0], ".") {
		return false
	}
	return true
}

// aliasFromImportSpec extracts the alias from an ImportSpec.
func aliasFromImportSpec(imp *ast.ImportSpec) string {
	if imp.Name == nil {
		return ""
	}
	return imp.Name.Name
}

// recvString formats a receiver list as a string.
func (g *GoParserBackend) recvString(recv *ast.FieldList) string {
	if recv == nil || len(recv.List) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("(")
	for i, field := range recv.List {
		if i > 0 {
			sb.WriteString(", ")
		}
		if len(field.Names) > 0 {
			sb.WriteString(field.Names[0].Name)
			sb.WriteString(" ")
		}
		sb.WriteString(g.exprString(field.Type))
	}
	sb.WriteString(")")
	return sb.String()
}

// fieldListString formats a FieldList (params or results) as a string.
func (g *GoParserBackend) fieldListString(fl *ast.FieldList) string {
	if fl == nil {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("(")
	for i, field := range fl.List {
		if i > 0 {
			sb.WriteString(", ")
		}
		if len(field.Names) > 0 {
			for j, name := range field.Names {
				if j > 0 {
					sb.WriteString(", ")
				}
				sb.WriteString(name.Name)
			}
			sb.WriteString(" ")
		}
		sb.WriteString(g.exprString(field.Type))
	}
	sb.WriteString(")")
	return sb.String()
}

// exprString returns a string representation of an ast.Expr.
func (g *GoParserBackend) exprString(expr ast.Expr) string {
	if expr == nil {
		return ""
	}
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.StarExpr:
		return "*" + g.exprString(e.X)
	case *ast.SelectorExpr:
		return g.exprString(e.X) + "." + e.Sel.Name
	case *ast.ArrayType:
		return "[]" + g.exprString(e.Elt)
	case *ast.MapType:
		return "map[" + g.exprString(e.Key) + "]" + g.exprString(e.Value)
	case *ast.ChanType:
		switch e.Dir {
		case ast.SEND:
			return "chan<- " + g.exprString(e.Value)
		case ast.RECV:
			return "<-chan " + g.exprString(e.Value)
		default:
			return "chan " + g.exprString(e.Value)
		}
	case *ast.FuncType:
		return "func" + g.fieldListString(e.Params) + g.fieldListString(e.Results)
	case *ast.InterfaceType:
		return "interface{}"
	case *ast.StructType:
		return "struct{}"
	case *ast.Ellipsis:
		return "..." + g.exprString(e.Elt)
	case *ast.ParenExpr:
		return "(" + g.exprString(e.X) + ")"
	default:
		return fmt.Sprintf("%T", expr)
	}
}

// nodeText returns a short text representation of a statement for BodyNode.Text.
func nodeText(stmt ast.Stmt) string {
	switch s := stmt.(type) {
	case *ast.IfStmt:
		return "if ..."
	case *ast.ForStmt:
		return "for ..."
	case *ast.RangeStmt:
		return "range ..."
	case *ast.ExprStmt:
		return fmt.Sprintf("%s", s.X)
	case *ast.AssignStmt:
		var lhs []string
		for _, x := range s.Lhs {
			lhs = append(lhs, fmt.Sprintf("%s", x))
		}
		var rhs []string
		for _, x := range s.Rhs {
			rhs = append(rhs, fmt.Sprintf("%s", x))
		}
		tok := s.Tok.String()
		return strings.Join(lhs, ", ") + " " + tok + " " + strings.Join(rhs, ", ")
	case *ast.ReturnStmt:
		if len(s.Results) == 0 {
			return "return"
		}
		var results []string
		for _, r := range s.Results {
			results = append(results, fmt.Sprintf("%s", r))
		}
		return "return " + strings.Join(results, ", ")
	case *ast.DeferStmt:
		return "defer " + fmt.Sprintf("%v", s.Call)
	case *ast.GoStmt:
		return "go " + fmt.Sprintf("%v", s.Call)
	case *ast.SelectStmt:
		return "select ..."
	case *ast.SendStmt:
		return fmt.Sprintf("%s <- %s", s.Chan, s.Value)
	case *ast.BlockStmt:
		return "{ ... }"
	case *ast.SwitchStmt:
		return "switch ..."
	case *ast.DeclStmt:
		return "decl ..."
	default:
		return fmt.Sprintf("%T", stmt)
	}
}

// countGoFiles counts the number of .go files in a directory.
func countGoFiles(dir string) (int, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".go") {
			count++
		}
	}
	return count, nil
}

// ParseDirCount parses a directory and returns the number of files parsed.
func (g *GoParserBackend) ParseDirCount(dir string) (int, error) {
	files, err := g.ParseDir(dir)
	if err != nil {
		return 0, err
	}
	return len(files), nil
}
