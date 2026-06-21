package migrate

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// testTempDir creates a temporary directory.
func testTempDir(t *testing.T) string {
	t.Helper()
	base := "c:\\temp\\go-parser-test"
	if err := os.MkdirAll(base, 0755); err != nil {
		t.Fatalf("failed to create temp base dir: %v", err)
	}
	dir, err := os.MkdirTemp(base, t.Name()+"_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return dir
}

// TestGoParserBackend_Parse creates a temporary Go file with Add and main functions,
// parses it, and verifies Functions, Imports, and IsExported.
func TestGoParserBackend_Parse(t *testing.T) {
	tmpDir := testTempDir(t)
	goFile := filepath.Join(tmpDir, "calc.go")
	content := `package calc

import (
	"fmt"
	"strings"
)

// Add returns the sum of two integers.
func Add(a, b int) int {
	return a + b
}

func main() {
	result := Add(1, 2)
	fmt.Println(result)
}
`
	if err := os.WriteFile(goFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	backend := &GoParserBackend{}
	uf, err := backend.Parse(goFile)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// Verify basic fields.
	if uf.Language != "go" {
		t.Errorf("Language: got %q, want %q", uf.Language, "go")
	}
	if uf.Path != goFile {
		t.Errorf("Path: got %q, want %q", uf.Path, goFile)
	}

	// Verify functions.
	if len(uf.Functions) != 2 {
		t.Fatalf("Functions count: got %d, want 2 (got: %v)", len(uf.Functions), funcNames(uf.Functions))
	}

	// Find Add function.
	var addFn *UnifiedFunction
	for i := range uf.Functions {
		if uf.Functions[i].Name == "Add" {
			addFn = &uf.Functions[i]
			break
		}
	}
	if addFn == nil {
		t.Fatal("Add function not found")
	}

	// Verify Add is exported.
	if !addFn.IsExported {
		t.Error("Add should be exported")
	}

	// Verify Add parameters.
	if len(addFn.Parameters) != 2 {
		t.Errorf("Add parameters count: got %d, want 2", len(addFn.Parameters))
	} else {
		if addFn.Parameters[0].Name != "a" || addFn.Parameters[0].Type != "int" {
			t.Errorf("Add param[0]: got %v, want {a int}", addFn.Parameters[0])
		}
		if addFn.Parameters[1].Name != "b" || addFn.Parameters[1].Type != "int" {
			t.Errorf("Add param[1]: got %v, want {b int}", addFn.Parameters[1])
		}
	}

	// Verify Add return types.
	if len(addFn.ReturnTypes) != 1 || addFn.ReturnTypes[0] != "int" {
		t.Errorf("Add return types: got %v, want [int]", addFn.ReturnTypes)
	}

	// Verify Add has a return body node.
	hasReturn := false
	for _, bn := range addFn.BodyNodes {
		if bn.Type == BodyNodeReturn {
			hasReturn = true
			break
		}
	}
	if !hasReturn {
		t.Error("Add should have a return body node")
	}

	// Verify Add call graph (should be empty since Add has no calls).
	if len(addFn.CallGraph) != 0 {
		t.Errorf("Add call graph: got %v, want empty", addFn.CallGraph)
	}

	// Find main function.
	var mainFn *UnifiedFunction
	for i := range uf.Functions {
		if uf.Functions[i].Name == "main" {
			mainFn = &uf.Functions[i]
			break
		}
	}
	if mainFn == nil {
		t.Fatal("main function not found")
	}

	// Verify main is not exported.
	if mainFn.IsExported {
		t.Error("main should not be exported")
	}

	// Verify main call graph contains Add and fmt.Println.
	sort.Strings(mainFn.CallGraph)
	wantCalls := []string{"Add", "Println"}
	if len(mainFn.CallGraph) != len(wantCalls) {
		t.Errorf("main call graph length: got %d, want %d (got: %v)", len(mainFn.CallGraph), len(wantCalls), mainFn.CallGraph)
	} else {
		for i, w := range wantCalls {
			if mainFn.CallGraph[i] != w {
				t.Errorf("main call graph[%d]: got %q, want %q", i, mainFn.CallGraph[i], w)
			}
		}
	}

	// Verify imports.
	if len(uf.Imports) != 2 {
		t.Fatalf("Imports count: got %d, want 2", len(uf.Imports))
	}
	importPaths := make(map[string]bool)
	for _, imp := range uf.Imports {
		importPaths[imp.Path] = true
	}
	if !importPaths["fmt"] {
		t.Error("missing import fmt")
	}
	if !importPaths["strings"] {
		t.Error("missing import strings")
	}

	// Verify stdlib detection.
	for _, imp := range uf.Imports {
		if imp.Path == "fmt" && !imp.IsStdLib {
			t.Error("fmt should be detected as stdlib")
		}
		if imp.Path == "strings" && !imp.IsStdLib {
			t.Error("strings should be detected as stdlib")
		}
	}

	// Verify comments.
	foundComment := false
	for _, c := range uf.Comments {
		if strings.Contains(c, "Add returns the sum") {
			foundComment = true
			break
		}
	}
	if !foundComment {
		t.Error("expected comment about Add not found")
	}
}

// TestGoParserBackend_ParseDir creates a temporary directory with 2 Go files,
// parses the directory, and verifies the file count.
func TestGoParserBackend_ParseDir(t *testing.T) {
	tmpDir := testTempDir(t)

	// File 1: util.go
	utilContent := `package mypkg

import "fmt"

func Hello(name string) string {
	return fmt.Sprintf("Hello, %s!", name)
}
`
	if err := os.WriteFile(filepath.Join(tmpDir, "util.go"), []byte(utilContent), 0644); err != nil {
		t.Fatalf("failed to write util.go: %v", err)
	}

	// File 2: calc.go
	calcContent := `package mypkg

func Double(x int) int {
	return x * 2
}
`
	if err := os.WriteFile(filepath.Join(tmpDir, "calc.go"), []byte(calcContent), 0644); err != nil {
		t.Fatalf("failed to write calc.go: %v", err)
	}

	backend := &GoParserBackend{}
	files, err := backend.ParseDir(tmpDir)
	if err != nil {
		t.Fatalf("ParseDir failed: %v", err)
	}

	if len(files) != 2 {
		t.Fatalf("ParseDir file count: got %d, want 2", len(files))
	}

	// Verify each file has correct language.
	for _, f := range files {
		if f.Language != "go" {
			t.Errorf("file %s language: got %q, want %q", f.Path, f.Language, "go")
		}
	}

	// Collect all function names across files.
	allFuncs := make(map[string]bool)
	for _, f := range files {
		for _, fn := range f.Functions {
			allFuncs[fn.Name] = true
		}
	}
	if !allFuncs["Hello"] {
		t.Error("Hello function not found in parsed files")
	}
	if !allFuncs["Double"] {
		t.Error("Double function not found in parsed files")
	}
}

// TestGoParserBackend_SupportedExtensions verifies the parser returns [".go"].
func TestGoParserBackend_SupportedExtensions(t *testing.T) {
	backend := &GoParserBackend{}
	exts := backend.SupportedExtensions()
	if len(exts) != 1 {
		t.Fatalf("SupportedExtensions count: got %d, want 1", len(exts))
	}
	if exts[0] != ".go" {
		t.Errorf("SupportedExtensions[0]: got %q, want %q", exts[0], ".go")
	}
}

// TestGoParserBackend_Registered verifies GetParser("go") returns the correct instance.
func TestGoParserBackend_Registered(t *testing.T) {
	// Re-register in case another test cleared the registry.
	RegisterParser("go", &GoParserBackend{})
	p, err := GetParser("go")
	if err != nil {
		t.Fatalf("GetParser(\"go\") failed: %v", err)
	}
	if _, ok := p.(*GoParserBackend); !ok {
		t.Errorf("GetParser(\"go\"): got %T, want *GoParserBackend", p)
	}
	if p.Language() != "go" {
		t.Errorf("Language(): got %q, want %q", p.Language(), "go")
	}
}

// funcNames is a test helper that extracts function names.
func funcNames(fns []UnifiedFunction) []string {
	names := make([]string, len(fns))
	for i, f := range fns {
		names[i] = f.Name
	}
	return names
}
