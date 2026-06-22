package main

import (
	"fmt"
	"go/ast"
	"path/filepath"
	"sort"
	"strings"
)

// buildImportIndex 从所有文件构建「包路径 → 文件名」的反向索引。
//
// 输入: []arch.FileInfo — 每个文件的解析结果（含 Path / Name / Imports）
// 输出: pkgPathIndex   map[string][]string — 完整包路径 (如 "low-entropy-core/go-core/arch") → 文件名列表
//       pkgBaseIndex   map[string][]string — 包基础路径 (如 "go-core/arch") → 文件名列表
//
// 用于在 buildArchData 的跨文件依赖分析阶段，将文件 import 路径快速映射到实际文件名。
func buildImportIndex(files []FileInfo) (pkgPathIndex map[string][]string, pkgBaseIndex map[string][]string) {
	pkgPathIndex = make(map[string][]string)
	pkgBaseIndex = make(map[string][]string)

	for _, f := range files {
		// 从文件完整路径中提取「low-entropy-core/<...>/<pkg>」形式的包路径
		// f.Path 可能是 "d:/work/.../low-entropy-core/go-core/arch/types.go"
		// 也可能是 "low-entropy-core/go-core/arch/types.go"（相对路径）
		dir := filepath.ToSlash(filepath.Dir(f.Path))

		// 寻找 "low-entropy-core/" 作为模块根标记
		pkgPath := ""
		if idx := strings.Index(dir, "low-entropy-core/"); idx >= 0 {
			pkgPath = dir[idx:]
		} else {
			// 找不到模块标记，退化为使用目录末两段（兼容相对路径）
			parts := strings.Split(dir, "/")
			if len(parts) >= 2 {
				pkgPath = strings.Join(parts[len(parts)-2:], "/")
			} else {
				pkgPath = dir
			}
		}

		// 完整包路径索引："low-entropy-core/go-core/arch" → [types.go, ...]
		pkgPathIndex[pkgPath] = append(pkgPathIndex[pkgPath], f.Name)

		// 去 low-entropy-core 前缀的基础索引："go-core/arch" → [types.go, ...]
		if strings.HasPrefix(pkgPath, "low-entropy-core/") {
			base := strings.TrimPrefix(pkgPath, "low-entropy-core/")
			pkgBaseIndex[base] = append(pkgBaseIndex[base], f.Name)
		} else {
			pkgBaseIndex[pkgPath] = append(pkgBaseIndex[pkgPath], f.Name)
		}
	}

	// 去重 + 排序每个索引值（保证结果稳定）
	for k, names := range pkgPathIndex {
		pkgPathIndex[k] = dedupAndSort(names)
	}
	for k, names := range pkgBaseIndex {
		pkgBaseIndex[k] = dedupAndSort(names)
	}
	return pkgPathIndex, pkgBaseIndex
}

// dedupAndSort 去重并排序字符串切片（用于依赖列表/索引值稳定化）
func dedupAndSort(xs []string) []string {
	seen := make(map[string]bool, len(xs))
	out := make([]string, 0, len(xs))
	for _, x := range xs {
		if !seen[x] {
			seen[x] = true
			out = append(out, x)
		}
	}
	sort.Strings(out)
	return out
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
