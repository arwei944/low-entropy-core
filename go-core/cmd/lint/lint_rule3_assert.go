package main

import (
	"go/ast"
	"go/token"
	"strings"
)

// stdlibPackages lists packages that are allowed to have type assertions.
var stdlibPackages = map[string]bool{
	"go/ast": true, "go/parser": true, "go/token": true,
	"go/types": true, "go/build": true, "go/scanner": true,
	"errors": true, "context": true, "json": true, "sync": true,
	"http": true, "os": true, "io": true, "net": true,
	"reflect": true, "testing": true, "fmt": true,
	"time": true, "strings": true, "bytes": true,
	"crypto": true, "hash": true, "encoding": true,
	"container": true, "sort": true, "math": true,
	"path": true, "runtime": true, "unsafe": true,
}

// errorTypes lists error-handling types that are allowed in type assertions.
var errorTypes = map[string]bool{
	"StepError": true, "error": true, "HandoffRequest": true,
	"TraceContext": true, "QueuedTask": true,
}

// primitiveTypes lists Go primitive/builtin types that are always allowed.
var primitiveTypes = map[string]bool{
	"string": true, "bool": true, "int": true, "int8": true, "int16": true,
	"int32": true, "int64": true, "uint": true, "uint8": true, "uint16": true,
	"uint32": true, "uint64": true, "float32": true, "float64": true,
	"byte": true, "rune": true, "complex64": true, "complex128": true,
	"uintptr": true, "any": true,
}

// =========================================================================
// Rule 3 – No concrete type assertions (only any allowed)
// =========================================================================
//
// The codebase uses generics, so concrete type assertions like
//   input.(Calculation)
// are disallowed.  Only type-switch forms (assertion type == nil) and
// assertions to the empty interface any are permitted.
// Exceptions: standard library types and error-handling types are allowed.

func checkRule3_NoConcreteTypeAssertions(f *ast.File, fset *token.FileSet, path string) {
	ast.Inspect(f, func(n ast.Node) bool {
		ta, ok := n.(*ast.TypeAssertExpr)
		if !ok {
			return true
		}

		// nil type -> type switch guard; skip.
		if ta.Type == nil {
			return true
		}

		// any (empty interface) is always allowed.
		if isEmptyInterface(ta.Type) {
			return true
		}

		typeName := exprToString(ta.Type)

		// Allow generic type parameters (single uppercase letters like T, In, Out, R)
		if isGenericTypeParam(typeName) {
			return true
		}

		// Allow primitive types
		if primitiveTypes[typeName] {
			return true
		}

		// Allow error types (strip * prefix for pointer types like *StepError)
		baseName := strings.TrimPrefix(typeName, "*")
		if errorTypes[baseName] {
			return true
		}

		// Allow 4-primitive generic types (e.g., Atom[any, any], Port[any, any])
		// Strip generic parameters and check the base name
		if bracketIdx := strings.Index(baseName, "["); bracketIdx >= 0 {
			genericBase := baseName[:bracketIdx]
			if fourPrimitives[genericBase] {
				return true
			}
		}

		// Allow standard library types (e.g., *ast.GenDecl, *ast.FuncDecl)
		if isStdlibType(baseName) {
			return true
		}

		pos := fset.Position(ta.Pos())
		report("ERROR", path, pos.Line,
			"concrete type assertion: .(%s) – use generics instead",
			typeName)
		return true
	})
}

// isStdlibType checks if a type name looks like a standard library type.
func isStdlibType(name string) bool {
	// Common stdlib type prefixes
	stdlibPrefixes := []string{
		"ast.", "token.", "parser.", "types.", "scanner.",
		"json.", "http.", "os.", "io.", "net.", "sync.",
		"reflect.", "testing.", "fmt.", "time.", "strings.",
		"bytes.", "crypto.", "hash.", "encoding.", "math.",
		"context.", "errors.", "runtime.", "sort.", "path.",
		"container.", "unsafe.", "template.",
	}
	for _, prefix := range stdlibPrefixes {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	// Also check if it's a raw stdlib type name
	if stdlibPackages[name] {
		return true
	}
	return false
}

// isGenericTypeParam returns true if the type name is a Go generic type parameter
// (single uppercase letter potentially followed by lowercase letters, like T, In, Out, R, K, V).
func isGenericTypeParam(name string) bool {
	// Strip pointer prefix
	name = strings.TrimPrefix(name, "*")
	if len(name) == 0 || len(name) > 4 {
		return false
	}
	// Must start with uppercase letter
	if name[0] < 'A' || name[0] > 'Z' {
		return false
	}
	// Remaining characters must be lowercase letters
	for i := 1; i < len(name); i++ {
		if name[i] < 'a' || name[i] > 'z' {
			return false
		}
	}
	return true
}

// isEmptyInterface returns true when expr is exactly any (the empty
// interface type with no methods).
func isEmptyInterface(expr ast.Expr) bool {
	iface, ok := expr.(*ast.InterfaceType)
	if !ok {
		return false
	}
	return iface.Methods == nil || len(iface.Methods.List) == 0
}
