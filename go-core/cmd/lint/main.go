// Static analysis CLI tool (TASK-6.1) that recursively scans Go source files
// using go/ast and go/parser. Checks 4 architectural rules for the low-entropy-core
// codebase based on the 4-primitive design: Atom, Port, Adapter, Composer, Step, Pipeline.
//
// Rules:
//   1. No non-4-primitive interface type definitions (ERROR)
//   2. No I/O operations in Atom/Port function bodies (ERROR)
//   3. No concrete type assertions beyond any (ERROR)
//   4. Pipeline steps must be <= 20 (WARNING)
//
// Usage: lint <directory>
// Exit codes: 0 = clean, 1 = violations found or usage error.

package main

import (
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

// ---------------------------------------------------------------------------
// main
// ---------------------------------------------------------------------------

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: lint <directory>\n")
		os.Exit(1)
	}

	dir := os.Args[1]

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(path, ".go") &&
			!strings.HasSuffix(path, "_test.go") &&
			!strings.Contains(path, string(filepath.Separator)+"cmd"+string(filepath.Separator)+"lint") {
			checkFile(path)
		}
		return nil
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error walking directory: %v\n", err)
		os.Exit(1)
	}

	if violations > 0 {
		os.Exit(1)
	}
}

// ---------------------------------------------------------------------------
// checkFile parses a single .go file and runs all four rules against the AST.
// ---------------------------------------------------------------------------

func checkFile(path string) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		fmt.Fprintf(os.Stderr, "WARNING: parse error in %s: %v\n", path, err)
		return
	}

	checkRule1_NoNon4PrimitiveTypes(f, fset, path)
	checkRule2_NoIOInAtomPort(f, fset, path)
	checkRule3_NoConcreteTypeAssertions(f, fset, path)
	checkRule4_PipelineStepLimit(f, fset, path)
}

// ---------------------------------------------------------------------------
// Helper: emit a violation to stderr and bump the global counter.
// ---------------------------------------------------------------------------

func report(level string, file string, line int, format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(os.Stderr, "%s: %s:%d: %s\n", level, file, line, msg)
	violations++
}
