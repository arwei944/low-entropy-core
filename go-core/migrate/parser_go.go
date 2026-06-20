package migrate

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"sort"
	"strings"
	"unicode"
)

// GoParserBackend implements ParserBackend for Go source files.
type GoParserBackend struct{}

func init() {
	RegisterParser("go", &GoParserBackend{})
}

// Language returns the language identifier.
func (g *GoParserBackend) Language() string {
	return "go"
}

// SupportedExtensions returns the file extensions handled by this parser.
func (g *GoParserBackend) SupportedExtensions() []string {
	return []string{".go"}
}

// Parse parses a single Go source file and returns a UnifiedFile.
func (g *GoParserBackend) Parse(file string) (*UnifiedFile, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, file, nil, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("go parser error: %w", err)
	}
	return g.convertAST(fset, f, file), nil
}

// ParseDir parses all Go files in a directory and returns a slice of UnifiedFile.
func (g *GoParserBackend) ParseDir(dir string) ([]*UnifiedFile, error) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, dir, nil, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("go parser dir error: %w", err)
	}

	var files []*UnifiedFile
	for _, pkg := range pkgs {
		for filename, f := range pkg.Files {
			files = append(files, g.convertAST(fset, f, filename))
		}
	}
	sort.Slice(files, func(i, j int) bool {
		return files[i].Path < files[j].Path
	})
	return files, nil
}

// convertAST converts an ast.File into a UnifiedFile.
func (g *GoParserBackend) convertAST(fset *token.FileSet, f *ast.File, filePath string) *UnifiedFile {
	uf := &UnifiedFile{
		Path:     filePath,
		Language: "go",
	}

	// Extract imports.
	for _, imp := range f.Imports {
		path := strings.Trim(imp.Path.Value, `"`)
		uf.Imports = append(uf.Imports, UnifiedImport{
			Path:     path,
			Alias:    aliasFromImportSpec(imp),
			IsStdLib: isStdLib(path),
		})
	}

	// Extract functions and methods.
	for _, decl := range f.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			uf.Functions = append(uf.Functions, g.convertFuncDecl(fset, d, filePath))
		case *ast.GenDecl:
			g.extractTypeDecls(d, uf, filePath, fset)
		}
	}

	// Extract comments.
	if f.Comments != nil {
		for _, cg := range f.Comments {
			for _, c := range cg.List {
				uf.Comments = append(uf.Comments, c.Text)
			}
		}
	}

	// Count raw lines.
	tf := fset.File(f.Pos())
	if tf != nil {
		uf.RawLines = tf.LineCount()
	}

	return uf
}

// convertFuncDecl converts an ast.FuncDecl into a UnifiedFunction.
func (g *GoParserBackend) convertFuncDecl(fset *token.FileSet, d *ast.FuncDecl, filePath string) UnifiedFunction {
	uf := UnifiedFunction{
		Name: d.Name.Name,
		File: filePath,
		Line: fset.Position(d.Pos()).Line,
	}

	// Determine if exported.
	if d.Name != nil && len(d.Name.Name) > 0 {
		uf.IsExported = unicode.IsUpper(rune(d.Name.Name[0]))
	}

	// Build signature.
	var sig strings.Builder
	sig.WriteString("func ")
	if d.Recv != nil {
		sig.WriteString(g.recvString(d.Recv))
		sig.WriteString(" ")
	}
	sig.WriteString(d.Name.Name)
	sig.WriteString(g.fieldListString(d.Type.Params))
	if d.Type.Results != nil {
		sig.WriteString(" ")
		sig.WriteString(g.fieldListString(d.Type.Results))
	}
	uf.Signature = sig.String()

	// Extract parameters.
	if d.Type.Params != nil {
		for _, field := range d.Type.Params.List {
			typ := g.exprString(field.Type)
			for _, name := range field.Names {
				uf.Parameters = append(uf.Parameters, UnifiedParam{
					Name: name.Name,
					Type: typ,
				})
			}
		}
	}

	// Extract return types.
	if d.Type.Results != nil {
		for _, field := range d.Type.Results.List {
			typ := g.exprString(field.Type)
			if len(field.Names) == 0 {
				uf.ReturnTypes = append(uf.ReturnTypes, typ)
			} else {
				for range field.Names {
					uf.ReturnTypes = append(uf.ReturnTypes, typ)
				}
			}
		}
	}

	// Extract body nodes and call graph.
	if d.Body != nil {
		uf.BodyNodes = g.extractBodyNodes(fset, d.Body)
		uf.CallGraph = g.extractCallGraph(d.Body)
	}

	return uf
}
