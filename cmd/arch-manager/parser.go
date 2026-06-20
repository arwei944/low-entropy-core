package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

// ============================================================================
// AST 解析器
// ============================================================================

// parseFile 解析单个 Go 源文件，提取所有符号
func parseFile(path string) (FileInfo, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		return FileInfo{}, fmt.Errorf("parse %s: %w", path, err)
	}

	// 读取文件行数
	content, err := os.ReadFile(path)
	if err != nil {
		return FileInfo{}, err
	}
	lines := strings.Count(string(content), "\n") + 1

	filename := filepath.Base(path)
	layer := getLayerInfo(filename)

	info := FileInfo{
		Path:      path,
		Name:      filename,
		Package:   f.Name.Name,
		Lines:     lines,
		Layer:     layer.Layer,
		LayerName: layer.Name,
		Imports:   make([]string, 0),
		Symbols:   make([]Symbol, 0),
		DependsOn: make([]string, 0),
	}

	// 提取 imports
	for _, imp := range f.Imports {
		importPath := strings.Trim(imp.Path.Value, `"`)
		info.Imports = append(info.Imports, importPath)
	}

	// 提取所有声明
	for _, decl := range f.Decls {
		switch d := decl.(type) {
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				switch s := spec.(type) {
				case *ast.TypeSpec:
					sym := parseTypeSpec(s, d.Doc)
					if sym.IsExported {
						info.Symbols = append(info.Symbols, sym)
					}
				case *ast.ValueSpec:
					for _, name := range s.Names {
						if !name.IsExported() {
							continue
						}
						kind := "var"
						if d.Tok == token.CONST {
							kind = "const"
						}
						sym := Symbol{
							Name:       name.Name,
							Kind:       kind,
							IsExported: true,
						}
						if d.Doc != nil {
							sym.Doc = strings.TrimSpace(d.Doc.Text())
						}
						info.Symbols = append(info.Symbols, sym)
					}
				}
			}

		case *ast.FuncDecl:
			if !d.Name.IsExported() {
				continue
			}
			sym := Symbol{
				Name:       d.Name.Name,
				IsExported: true,
			}
			if d.Recv != nil && len(d.Recv.List) > 0 {
				sym.Kind = "method"
				sym.Receiver = exprToString(d.Recv.List[0].Type)
			} else {
				sym.Kind = "func"
			}
			// 构建签名
			sym.Signature = buildFuncSignature(d)
			if d.Doc != nil {
				sym.Doc = strings.TrimSpace(d.Doc.Text())
			}
			info.Symbols = append(info.Symbols, sym)
		}
	}

	// 提取文件级内部依赖（基于 imports 映射到文件名）
	info.DependsOn = resolveInternalDeps(info.Imports)

	return info, nil
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

// resolveInternalDeps 将 import 路径解析为内部文件名
func resolveInternalDeps(imports []string) []string {
	deps := make([]string, 0)
	for _, imp := range imports {
		// 跳过标准库
		if !strings.Contains(imp, ".") {
			continue
		}
		if strings.Contains(imp, "low-entropy-core/go-core") {
			// 内部依赖，提取最后一个路径段
			parts := strings.Split(imp, "/")
			if len(parts) > 0 {
				last := parts[len(parts)-1]
				deps = append(deps, last)
			}
		}
	}
	return deps
}

// collectCalledFunctions 遍历 AST 只收集函数调用中的函数名。
// 用于同包跨文件依赖分析：文件 A 调用了文件 B 定义的函数，则 A 依赖 B。
// 相比 collectIdentifiers，此方法过滤掉类型引用、变量名等噪音，只保留真正的调用关系。
func collectCalledFunctions(node ast.Node) []string {
	names := make(map[string]bool)
	ast.Inspect(node, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		// 直接函数调用: FuncName(args)
		if ident, ok := call.Fun.(*ast.Ident); ok {
			if ident.Name != "" && ident.Name != "_" {
				names[ident.Name] = true
			}
			return true
		}
		// 选择器调用: pkg.FuncName(args) 或 obj.Method(args)
		if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
			if sel.Sel != nil && sel.Sel.Name != "" && sel.Sel.Name != "_" {
				names[sel.Sel.Name] = true
			}
		}
		return true
	})
	result := make([]string, 0, len(names))
	for name := range names {
		result = append(result, name)
	}
	return result
}
