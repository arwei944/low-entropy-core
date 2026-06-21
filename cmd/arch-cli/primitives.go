package main

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// PrimitiveInfo 四原语信息
type PrimitiveInfo struct {
	Name       string `json:"name"`
	Type       string `json:"type"`       // "Atom", "Port", "Adapter", "Composer"
	File       string `json:"file"`
	Package    string `json:"package"`
	Line       int    `json:"line"`
	Signature  string `json:"signature,omitempty"`
	IsExported bool   `json:"is_exported"`
}

// handlePrimitives 返回所有识别到的四原语
func handlePrimitives(w http.ResponseWriter, r *http.Request) {
	archMu.RLock()
	defer archMu.RUnlock()

	if archData == nil {
		http.Error(w, `{"error":"no data available"}`, http.StatusServiceUnavailable)
		return
	}

	primitives := detectPrimitives(archData)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(primitives)
}

// detectPrimitives 从架构数据中检测四原语接口断言
func detectPrimitives(data *ArchData) []PrimitiveInfo {
	var primitives []PrimitiveInfo

	for _, file := range data.Files {
		path := file.Path
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			continue
		}

		for _, decl := range f.Decls {
			genDecl, ok := decl.(*ast.GenDecl)
			if !ok || genDecl.Tok != token.VAR {
				continue
			}

			for _, spec := range genDecl.Specs {
				valueSpec, ok := spec.(*ast.ValueSpec)
				if !ok || len(valueSpec.Values) == 0 {
					continue
				}

				// 检查是否是接口断言: var _ Atom[...] = (*Type)(nil)
				for i, name := range valueSpec.Names {
					if name.Name != "_" {
						continue
					}
					if i >= len(valueSpec.Values) {
						continue
					}

					// 检查类型是否是四原语接口
					primType := extractPrimitiveType(valueSpec.Type)
					if primType == "" {
						continue
					}

					// 提取实现类型名
					implType := extractImplType(valueSpec.Values[i])
					if implType == "" {
						continue
					}

					pos := fset.Position(name.Pos())
					primitives = append(primitives, PrimitiveInfo{
						Name:       implType,
						Type:       primType,
						File:       filepath.Base(path),
						Package:    f.Name.Name,
						Line:       pos.Line,
						Signature:  fmt.Sprintf("var _ %s = (*%s)(nil)", typeExprToString(valueSpec.Type), implType),
						IsExported: len(implType) > 0 && implType[0] >= 'A' && implType[0] <= 'Z',
					})
				}
			}
		}
	}

	return primitives
}

// extractPrimitiveType 从类型表达式中提取四原语类型名
func extractPrimitiveType(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.IndexExpr:
		if ident, ok := e.X.(*ast.Ident); ok {
			return isPrimitive(ident.Name)
		}
	case *ast.IndexListExpr:
		if ident, ok := e.X.(*ast.Ident); ok {
			return isPrimitive(ident.Name)
		}
	case *ast.Ident:
		return isPrimitive(e.Name)
	}
	return ""
}

// isPrimitive 检查名称是否是四原语之一
func isPrimitive(name string) string {
	switch name {
	case "Atom", "Port", "Adapter", "Composer":
		return name
	}
	return ""
}

// extractImplType 从值表达式中提取实现类型名
func extractImplType(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.CallExpr:
		// (*Type)(nil)
		if len(e.Args) == 1 {
			if lit, ok := e.Args[0].(*ast.Ident); ok && lit.Name == "nil" {
				return extractTypeFromParen(e.Fun)
			}
		}
	case *ast.StarExpr:
		// &Type{}
		return typeExprToString(e.X)
	case *ast.UnaryExpr:
		if e.Op == token.AND {
			return typeExprToString(e.X)
		}
	}
	return ""
}

// extractTypeFromParen 从 (*Type) 中提取类型名
func extractTypeFromParen(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.ParenExpr:
		return extractTypeFromParen(e.X)
	case *ast.StarExpr:
		return typeExprToString(e.X)
	case *ast.Ident:
		return e.Name
	}
	return ""
}

// typeExprToString 将类型表达式转为字符串
func typeExprToString(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.StarExpr:
		return "*" + typeExprToString(e.X)
	case *ast.SelectorExpr:
		return typeExprToString(e.X) + "." + e.Sel.Name
	case *ast.IndexExpr:
		return typeExprToString(e.X) + "[" + typeExprToString(e.Index) + "]"
	case *ast.IndexListExpr:
		parts := make([]string, 0, len(e.Indices))
		for _, idx := range e.Indices {
			parts = append(parts, typeExprToString(idx))
		}
		return typeExprToString(e.X) + "[" + strings.Join(parts, ", ") + "]"
	case *ast.ArrayType:
		return "[]" + typeExprToString(e.Elt)
	case *ast.MapType:
		return "map[" + typeExprToString(e.Key) + "]" + typeExprToString(e.Value)
	case *ast.ChanType:
		return "chan " + typeExprToString(e.Value)
	case *ast.FuncType:
		return "func(...)"
	case *ast.InterfaceType:
		return "interface{}"
	case *ast.StructType:
		return "struct{...}"
	case *ast.ParenExpr:
		return "(" + typeExprToString(e.X) + ")"
	default:
		return fmt.Sprintf("<%T>", expr)
	}
}

// scanDirForPrimitives 扫描目录查找四原语（备用方法）
func scanDirForPrimitives(dir string) []PrimitiveInfo {
	var primitives []PrimitiveInfo

	filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		if strings.Contains(path, string(filepath.Separator)+"cmd"+string(filepath.Separator)) {
			return nil
		}

		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return nil
		}

		for _, decl := range f.Decls {
			genDecl, ok := decl.(*ast.GenDecl)
			if !ok || genDecl.Tok != token.VAR {
				continue
			}

			for _, spec := range genDecl.Specs {
				valueSpec, ok := spec.(*ast.ValueSpec)
				if !ok || len(valueSpec.Values) == 0 {
					continue
				}

				for i, name := range valueSpec.Names {
					if name.Name != "_" || i >= len(valueSpec.Values) {
						continue
					}

					primType := extractPrimitiveType(valueSpec.Type)
					if primType == "" {
						continue
					}

					implType := extractImplType(valueSpec.Values[i])
					if implType == "" {
						continue
					}

					pos := fset.Position(name.Pos())
					primitives = append(primitives, PrimitiveInfo{
						Name:       implType,
						Type:       primType,
						File:       filepath.Base(path),
						Package:    f.Name.Name,
						Line:       pos.Line,
						Signature:  fmt.Sprintf("var _ %s = (*%s)(nil)", typeExprToString(valueSpec.Type), implType),
						IsExported: len(implType) > 0 && implType[0] >= 'A' && implType[0] <= 'Z',
					})
				}
			}
		}
		return nil
	})

	return primitives
}
