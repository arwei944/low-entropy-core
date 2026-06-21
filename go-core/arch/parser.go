// Package arch — Parser Atom (L1)
//
// 解析 Go 源文件，提取符号/行数/import/依赖等元数据。
// 纯函数：输入文件路径 → 输出 FileInfo。
//
// 来源:
//   cmd/arch-manager/parser_file.go
//   cmd/arch-manager/parser_ast.go
//
// 设计约束:
//   文件 ≤ 300 行
//   仅依赖标准库 + go-core/arch/types.go
package arch

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

// ──────────────────────────────────────────────
// 公开入口
// ──────────────────────────────────────────────

// ParseFile 解析单个 Go 源文件并返回 FileInfo。
// 输入: 文件路径
// 输出: FileInfo（包含包名/行数/imports/符号/层级/依赖）
func ParseFile(path string) (*FileInfo, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}

	// 行数
	content, readErr := os.ReadFile(path)
	lines := 0
	if readErr == nil {
		lines = strings.Count(string(content), "\n") + 1
	}

	filename := filepath.Base(path)
	layer := GetLayerInfo(filename)

	info := &FileInfo{
		Path:      path,
		Name:      filename,
		Package:   f.Name.Name,
		Lines:     lines,
		Layer:     layer.Layer,
		LayerName: layer.Name,
		Imports:   extractImports(f),
		Symbols:   extractSymbols(f),
	}
	info.DependsOn = resolveInternalDeps(info.Imports)

	return info, nil
}

// ParseDirectory 递归解析目录下的所有 .go 文件。
// 跳过 _test.go（测试文件不参与架构分析）。
// 跳过 vendor/ 和 .git/ 等第三方目录。
func ParseDirectory(root string) ([]FileInfo, error) {
	var files []FileInfo

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // 跳过无法访问的项
		}
		if info.IsDir() {
			// 跳过特殊目录
			name := info.Name()
			if name == "vendor" || name == ".git" || name == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		f, parseErr := ParseFile(path)
		if parseErr != nil {
			return nil // 跳过无法解析的文件
		}
		files = append(files, *f)
		return nil
	})

	return files, err
}

// ──────────────────────────────────────────────
// imports 提取
// ──────────────────────────────────────────────

func extractImports(f *ast.File) []string {
	imports := make([]string, 0, len(f.Imports))
	for _, imp := range f.Imports {
		importPath := strings.Trim(imp.Path.Value, `"`)
		imports = append(imports, importPath)
	}
	return imports
}

// resolveInternalDeps 将 import 路径解析为内部文件名（用于构建依赖图）。
// 对 low-entropy-core/go-core 包提取最后一个路径片段作为依赖名。
func resolveInternalDeps(imports []string) []string {
	deps := make([]string, 0)
	for _, imp := range imports {
		// 纯标准库不含点
		if !strings.Contains(imp, ".") && !strings.Contains(imp, "/") {
			continue
		}
		if strings.Contains(imp, "low-entropy-core") {
			parts := strings.Split(imp, "/")
			if len(parts) > 0 {
				deps = append(deps, parts[len(parts)-1])
			}
		}
	}
	return deps
}

// ──────────────────────────────────────────────
// 符号提取（类型/函数/方法/常量/变量）
// ──────────────────────────────────────────────

func extractSymbols(f *ast.File) []Symbol {
	symbols := make([]Symbol, 0)

	for _, decl := range f.Decls {
		switch d := decl.(type) {
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				switch s := spec.(type) {
				case *ast.TypeSpec:
					sym := parseTypeSpec(s, d.Doc)
					if sym.IsExported {
						symbols = append(symbols, sym)
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
						symbols = append(symbols, Symbol{
							Name:       name.Name,
							Kind:       kind,
							IsExported: true,
						})
					}
				}
			}

		case *ast.FuncDecl:
			if !d.Name.IsExported() {
				continue
			}
			sym := Symbol{Name: d.Name.Name, IsExported: true}
			if d.Recv != nil && len(d.Recv.List) > 0 {
				sym.Kind = "method"
				sym.Receiver = exprToString(d.Recv.List[0].Type)
			} else {
				sym.Kind = "func"
			}
			sym.Signature = buildFuncSignature(d)
			symbols = append(symbols, sym)
		}
	}

	return symbols
}

func parseTypeSpec(s *ast.TypeSpec, doc *ast.CommentGroup) Symbol {
	sym := Symbol{Name: s.Name.Name, IsExported: s.Name.IsExported()}
	if !sym.IsExported {
		return sym
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
				sym.Fields = append(sym.Fields, fieldName+": "+exprToString(field.Type))
			}
		}
	case *ast.InterfaceType:
		sym.Kind = "interface"
		sym.Methods = make([]string, 0)
		if t.Methods != nil {
			for _, method := range t.Methods.List {
				if len(method.Names) > 0 {
					sig := ""
					if ft, ok := method.Type.(*ast.FuncType); ok {
						sig = funcTypeToString(ft)
					}
					sym.Methods = append(sym.Methods, method.Names[0].Name+sig)
				}
			}
		}
	case *ast.FuncType:
		sym.Kind = "func-type"
		sym.Signature = funcTypeToString(t)
	case *ast.Ident:
		sym.Kind = "type-alias"
		sym.Signature = "= " + t.Name
	default:
		sym.Kind = "type"
	}
	return sym
}

// ──────────────────────────────────────────────
// 签名与表达式字符串化
// ──────────────────────────────────────────────

func buildFuncSignature(d *ast.FuncDecl) string {
	parts := make([]string, 0, 16)
	parts = append(parts, d.Name.Name)
	parts = append(parts, funcTypeToString(d.Type))
	return strings.Join(parts, "")
}

func funcTypeToString(ft *ast.FuncType) string {
	parts := make([]string, 0, 8)
	// 类型参数 (泛型)
	if ft.TypeParams != nil && len(ft.TypeParams.List) > 0 {
		names := make([]string, 0)
		for _, p := range ft.TypeParams.List {
			for _, n := range p.Names {
				names = append(names, n.Name)
			}
		}
		parts = append(parts, "["+strings.Join(names, ", ")+"]")
	}
	// 参数
	paramStrs := make([]string, 0)
	if ft.Params != nil {
		for _, p := range ft.Params.List {
			paramStrs = append(paramStrs, exprToString(p.Type))
		}
	}
	parts = append(parts, "("+strings.Join(paramStrs, ", ")+")")
	// 返回值
	if ft.Results != nil && len(ft.Results.List) > 0 {
		retStrs := make([]string, 0, len(ft.Results.List))
		for _, r := range ft.Results.List {
			retStrs = append(retStrs, exprToString(r.Type))
		}
		if len(retStrs) > 1 {
			parts = append(parts, " ("+strings.Join(retStrs, ", ")+")")
		} else {
			parts = append(parts, " "+retStrs[0])
		}
	}
	return strings.Join(parts, "")
}

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
	case *ast.StructType:
		return "struct{...}"
	case *ast.ParenExpr:
		return "(" + exprToString(e.X) + ")"
	default:
		return fmt.Sprintf("<%T>", expr)
	}
}
