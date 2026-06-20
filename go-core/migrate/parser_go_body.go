package migrate

import (
	"go/ast"
	"go/token"
	"sort"
)

// extractBodyNodes converts an ast.BlockStmt into a slice of BodyNode.
func (g *GoParserBackend) extractBodyNodes(fset *token.FileSet, block *ast.BlockStmt) []BodyNode {
	if block == nil {
		return nil
	}
	var nodes []BodyNode
	for _, stmt := range block.List {
		nodes = append(nodes, g.stmtToBodyNode(fset, stmt))
	}
	return nodes
}

// stmtToBodyNode classifies a single statement into a BodyNode.
func (g *GoParserBackend) stmtToBodyNode(fset *token.FileSet, stmt ast.Stmt) BodyNode {
	line := fset.Position(stmt.Pos()).Line
	text := nodeText(stmt)

	switch s := stmt.(type) {
	case *ast.IfStmt:
		return BodyNode{Type: BodyNodeIf, Text: text, Line: line, Children: g.extractBodyNodes(fset, s.Body)}
	case *ast.ForStmt:
		return BodyNode{Type: BodyNodeLoop, Text: text, Line: line, Children: g.extractBodyNodes(fset, s.Body)}
	case *ast.RangeStmt:
		return BodyNode{Type: BodyNodeLoop, Text: text, Line: line, Children: g.extractBodyNodes(fset, s.Body)}
	case *ast.ExprStmt:
		if call, ok := s.X.(*ast.CallExpr); ok {
			if isIOCall(call) {
				return BodyNode{Type: BodyNodeIO, Text: text, Line: line}
			}
			return BodyNode{Type: BodyNodeCall, Text: text, Line: line}
		}
		return BodyNode{Type: BodyNodeOther, Text: text, Line: line}
	case *ast.AssignStmt:
		// Check if RHS has an I/O call.
		if len(s.Rhs) == 1 {
			if call, ok := s.Rhs[0].(*ast.CallExpr); ok {
				if isIOCall(call) {
					return BodyNode{Type: BodyNodeIO, Text: text, Line: line}
				}
			}
		}
		return BodyNode{Type: BodyNodeAssign, Text: text, Line: line}
	case *ast.ReturnStmt:
		return BodyNode{Type: BodyNodeReturn, Text: text, Line: line}
	case *ast.DeferStmt:
		return BodyNode{Type: BodyNodeDefer, Text: text, Line: line}
	case *ast.GoStmt:
		return BodyNode{Type: BodyNodeGo, Text: text, Line: line}
	case *ast.SelectStmt:
		return BodyNode{Type: BodyNodeSelect, Text: text, Line: line}
	case *ast.SendStmt:
		return BodyNode{Type: BodyNodeSend, Text: text, Line: line}
	case *ast.BlockStmt:
		return BodyNode{Type: BodyNodeOther, Text: text, Line: line, Children: g.extractBodyNodes(fset, s)}
	case *ast.SwitchStmt:
		return BodyNode{Type: BodyNodeOther, Text: text, Line: line}
	case *ast.CaseClause:
		return BodyNode{Type: BodyNodeOther, Text: text, Line: line}
	case *ast.DeclStmt:
		return BodyNode{Type: BodyNodeOther, Text: text, Line: line}
	default:
		return BodyNode{Type: BodyNodeOther, Text: text, Line: line}
	}
}

// extractCallGraph extracts all function calls from a function body,
// filtering out built-in functions.
func (g *GoParserBackend) extractCallGraph(body *ast.BlockStmt) []string {
	if body == nil {
		return nil
	}
	var calls []string
	seen := make(map[string]bool)

	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		name := callName(call)
		if name == "" || isBuiltin(name) {
			return true
		}
		if !seen[name] {
			seen[name] = true
			calls = append(calls, name)
		}
		return true
	})

	sort.Strings(calls)
	return calls
}

// extractTypeDecls extracts struct and interface declarations from a GenDecl.
func (g *GoParserBackend) extractTypeDecls(decl *ast.GenDecl, uf *UnifiedFile, filePath string, fset *token.FileSet) {
	if decl.Tok != token.TYPE {
		return
	}
	for _, spec := range decl.Specs {
		ts, ok := spec.(*ast.TypeSpec)
		if !ok {
			continue
		}
		line := fset.Position(ts.Pos()).Line

		switch t := ts.Type.(type) {
		case *ast.StructType:
			uc := UnifiedClass{
				Name: ts.Name.Name,
				File: filePath,
				Line: line,
			}
			for _, field := range t.Fields.List {
				typ := g.exprString(field.Type)
				if len(field.Names) == 0 {
					uc.Fields = append(uc.Fields, typ)
				} else {
					for _, name := range field.Names {
						uc.Fields = append(uc.Fields, name.Name+" "+typ)
					}
				}
			}
			uf.Classes = append(uf.Classes, uc)

		case *ast.InterfaceType:
			ui := UnifiedInterface{
				Name: ts.Name.Name,
				File: filePath,
				Line: line,
			}
			for _, method := range t.Methods.List {
				if len(method.Names) > 0 {
					ui.Methods = append(ui.Methods, method.Names[0].Name)
				}
			}
			uf.Interfaces = append(uf.Interfaces, ui)
		}
	}
}
