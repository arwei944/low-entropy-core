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
