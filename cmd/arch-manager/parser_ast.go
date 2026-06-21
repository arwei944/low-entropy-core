package main

import (
	"fmt"
	"go/ast"
	"strings"
)

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
