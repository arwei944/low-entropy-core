package main

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"net/http"
	"path/filepath"
	"strings"

	arch "low-entropy-core/go-core/arch"
)

// handlePrimitives 返回所有识别到的四原语
func handlePrimitives(w http.ResponseWriter, r *http.Request) {
	archMu.RLock()
	defer archMu.RUnlock()

	if archData == nil {
		http.Error(w, `{"error":"no data available"}`, http.StatusServiceUnavailable)
		return
	}

	primitives := detectPrimitives(archData)
	byType := map[string]int{}
	byLayer := map[string]int{}
	for _, p := range primitives {
		byType[p.Type]++
		if p.Layer != "" {
			byLayer[p.Layer]++
		}
	}
	resp := arch.PrimitiveResponse{
		Total:      len(primitives),
		ByType:     byType,
		ByLayer:    byLayer,
		Items:      primitives,
		DetectedIn: fmt.Sprintf("%d files", len(archData.Files)),
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// detectPrimitives 从架构数据中检测四原语（三种方式）
func detectPrimitives(data *arch.ArchData) []arch.PrimitiveInfo {
	var primitives []arch.PrimitiveInfo
	seen := map[string]bool{}

	for _, file := range data.Files {
		// 1. 接口断言检测 (type 1: interface_assert)
		astPrimitives := detectByAST(file)
		for _, p := range astPrimitives {
			key := p.File + ":" + p.Name + ":" + p.Type
			if seen[key] {
				continue
			}
			seen[key] = true
			p.Layer = file.Layer
			p.LayerName = file.LayerName
			primitives = append(primitives, p)
		}

		// 2. 命名启发式检测
		for _, sym := range file.Symbols {
			primType := inferPrimitiveByName(sym.Name)
			if primType == "" {
				continue
			}
			key := file.Name + ":" + sym.Name + ":" + primType + ":n"
			if seen[key] {
				continue
			}
			seen[key] = true
			desc := sym.Doc
			if desc == "" {
				desc = primType + ": " + sym.Name
			}
			sig := sym.Signature
			if sig == "" {
				sig = fmt.Sprintf("type %s %s", sym.Name, sym.Kind)
			}
			primitives = append(primitives, arch.PrimitiveInfo{
				Name:          sym.Name,
				Type:          primType,
				File:          file.Name,
				FilePath:      file.Path,
				Package:       file.Package,
				Line:          0,
				Signature:     sig,
				IsExported:    sym.IsExported,
				Layer:         file.Layer,
				LayerName:     file.LayerName,
				Description:   strings.TrimSpace(desc),
				DetectionMode: "naming",
			})
		}

		// 3. 路径推断 (type 2: path_based)
		primType := inferPrimitiveByPath(file.Path)
		if primType != "" {
			for _, sym := range file.Symbols {
				if !sym.IsExported {
					continue
				}
				key := file.Name + ":" + sym.Name + ":" + primType + ":p"
				if seen[key] {
					continue
				}
				seen[key] = true
				desc := sym.Doc
				if desc == "" {
					desc = primType + ": " + sym.Name
				}
				sig := sym.Signature
				if sig == "" {
					sig = fmt.Sprintf("%s %s", sym.Name, sym.Kind)
				}
				primitives = append(primitives, arch.PrimitiveInfo{
					Name:          sym.Name,
					Type:          primType,
					File:          file.Name,
					FilePath:      file.Path,
					Package:       file.Package,
					Line:          0,
					Signature:     sig,
					IsExported:    sym.IsExported,
					Layer:         file.Layer,
					LayerName:     file.LayerName,
					Description:   strings.TrimSpace(desc),
					DetectionMode: "path_based",
				})
			}
		}
	}
	return primitives
}

// detectByAST 扫描单个文件的 AST，提取接口断言
func detectByAST(file arch.FileInfo) []arch.PrimitiveInfo {
	var primitives []arch.PrimitiveInfo
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, file.Path, nil, 0)
	if err != nil {
		return primitives
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
				primitives = append(primitives, arch.PrimitiveInfo{
					Name:          implType,
					Type:          primType,
					File:          file.Name,
					FilePath:      file.Path,
					Package:       f.Name.Name,
					Line:          pos.Line,
					Signature:     fmt.Sprintf("var _ %s = (*%s)(nil)", typeExprToString(valueSpec.Type), implType),
					IsExported:    len(implType) > 0 && implType[0] >= 'A' && implType[0] <= 'Z',
					DetectionMode: "interface_assert",
				})
			}
		}
	}
	return primitives
}

// inferPrimitiveByName 通过符号名称前缀或后缀推断四原语类型
// 支持: Atom* / *Atom / Port* / *Port / Adapter* / *Adapter / Composer* / *Composer
func inferPrimitiveByName(name string) string {
	if len(name) < 4 {
		return ""
	}
	for _, kw := range []string{"Atom", "Port", "Adapter", "Composer"} {
		if strings.HasPrefix(name, kw) || strings.HasSuffix(name, kw) {
			return kw
		}
	}
	return ""
}

// inferPrimitiveByPath 根据目录名或文件名推断类型
// 目录: atoms/ports/adapters/composers 目录
// 文件: atom_*.go / adapter_*.go / port_*.go / composer_*.go
func inferPrimitiveByPath(path string) string {
	base := filepath.Base(path)
	lower := strings.ToLower(base)
	switch {
	case strings.HasPrefix(lower, "atom"):
		return "Atom"
	case strings.HasPrefix(lower, "port"):
		return "Port"
	case strings.HasPrefix(lower, "adapter"):
		return "Adapter"
	case strings.HasPrefix(lower, "composer"):
		return "Composer"
	}
	dir := filepath.Dir(path)
	segments := strings.Split(dir, string(filepath.Separator))
	for i := len(segments) - 1; i >= 0; i-- {
		seg := strings.ToLower(segments[i])
		switch seg {
		case "atom", "atoms":
			return "Atom"
		case "port", "ports":
			return "Port"
		case "adapter", "adapters":
			return "Adapter"
		case "composer", "composers":
			return "Composer"
		}
	}
	return ""
}

// extractPrimitiveType 从类型表达式中提取四原语类型名
func extractPrimitiveType(expr ast.Expr) string {
	var identName string
	switch e := expr.(type) {
	case *ast.IndexExpr:
		if ident, ok := e.X.(*ast.Ident); ok {
			identName = ident.Name
		}
	case *ast.IndexListExpr:
		if ident, ok := e.X.(*ast.Ident); ok {
			identName = ident.Name
		}
	case *ast.Ident:
		identName = e.Name
	}
	switch identName {
	case "Atom", "Port", "Adapter", "Composer":
		return identName
	}
	return ""
}

// extractImplType 从值表达式中提取实现类型名
func extractImplType(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.CallExpr:
		if len(e.Args) == 1 {
			if lit, ok := e.Args[0].(*ast.Ident); ok && lit.Name == "nil" {
				return extractTypeFromParen(e.Fun)
			}
		}
	case *ast.StarExpr:
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
