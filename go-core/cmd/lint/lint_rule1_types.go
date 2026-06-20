package main

import (
	"go/ast"
	"go/token"
)

// =========================================================================
// Rule 1 – No non-4-primitive interface type definitions
// =========================================================================
//
// Walks every top-level GenDecl with token TYPE and flags any interface
// whose name is not in the set {Atom, Port, Adapter, Composer, Step, Pipeline}.
// Additionally, for struct type definitions, it inspects each field's type:
// if a field references a named type that is itself an interface defined
// elsewhere in the file and that interface is not a 4-primitive, it is flagged
// as "referencing a non-4-primitive abstraction".

func checkRule1_NoNon4PrimitiveTypes(f *ast.File, fset *token.FileSet, path string) {
	// First pass: collect all locally-defined interface names so we can
	// distinguish interface-typed fields from concrete-typed fields.
	localInterfaces := map[string]bool{}
	for _, decl := range f.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.TYPE {
			continue
		}
		for _, spec := range genDecl.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			if _, isInterface := typeSpec.Type.(*ast.InterfaceType); isInterface {
				localInterfaces[typeSpec.Name.Name] = true
			}
		}
	}

	// Second pass: inspect each type definition.
	for _, decl := range f.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.TYPE {
			continue
		}
		for _, spec := range genDecl.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			name := typeSpec.Name.Name

			// --- Case A: the type IS an interface ---
			if _, isInterface := typeSpec.Type.(*ast.InterfaceType); isInterface {
				if !fourPrimitives[name] {
					pos := fset.Position(typeSpec.Pos())
					report("ERROR", path, pos.Line,
						"non-4-primitive interface type definition: %s", name)
				}
				continue
			}

			// --- Case B: struct type referencing a non-4-primitive interface ---
			structType, isStruct := typeSpec.Type.(*ast.StructType)
			if !isStruct {
				continue
			}
			if structType.Fields == nil {
				continue
			}
			for _, field := range structType.Fields.List {
				fieldTypeName := resolveTypeName(field.Type)
				if fieldTypeName == "" {
					continue
				}
				// If the field type is a locally-defined interface that is NOT
				// a 4-primitive, flag it.
				if localInterfaces[fieldTypeName] && !fourPrimitives[fieldTypeName] {
					pos := fset.Position(field.Pos())
					report("ERROR", path, pos.Line,
						"type %s references non-4-primitive abstraction %s in field", name, fieldTypeName)
				}
			}
		}
	}
}

// resolveTypeName extracts the base identifier of a type expression.
// Examples: "User" -> "User", "*User" -> "User", "pkg.User" -> "" (external).
func resolveTypeName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return resolveTypeName(t.X)
	default:
		return ""
	}
}
