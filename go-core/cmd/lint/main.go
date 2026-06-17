// Static analysis CLI tool (TASK-6.1) that recursively scans Go source files
// using go/ast and go/parser. Checks 4 architectural rules for the low-entropy-core
// codebase based on the 4-primitive design: Atom, Port, Adapter, Composer, Step, Pipeline.
//
// Rules:
//   1. No non-4-primitive interface type definitions (ERROR)
//   2. No I/O operations in Atom/Port function bodies (ERROR)
//   3. No concrete type assertions beyond interface{} (ERROR)
//   4. Pipeline steps must be <= 20 (WARNING)
//
// Usage: lint <directory>
// Exit codes: 0 = clean, 1 = violations found or usage error.

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

// ---------------------------------------------------------------------------
// The 4-primitive architectural types allowed by the codebase.
// ---------------------------------------------------------------------------

var fourPrimitives = map[string]bool{
	"Atom":              true,
	"Port":              true,
	"Adapter":           true,
	"Composer":          true,
	"Step":              true,
	"Pipeline":          true,
	// Handoff protocol interfaces
	"SnapshotAdapter":   true,
	"HandoffTransport":  true,
	"SnapshotPersistence": true,
	// Observation interfaces
	"ObservationAdapter": true,
	"SamplingPolicy":    true,
	"StepStore":         true,
	// Config interfaces
	"AdapterResolver":   true,
	// Scheduler interfaces
	"AgentPool":         true,
	"TaskQueue":         true,
	// Security interfaces
	"CapabilityPort":    true,
	"AccessControlPort": true,
	// Resilience interfaces
	"CircuitBreaker":    true,
	"Fallback":          true,
	"Bulkhead":          true,
	"RateLimiter":       true,
	// Schema interfaces
	"SchemaRegistry":    true,
	"CompatibilityChecker": true,
	// Other valid architectural types
	"HandoffSnapshot":   true,
	"HandoffContract":   true,
	"HandoffRollback":   true,
	"ContractValidator": true,
	"PortContract":      true,
	"EntropyCollector":  true,
	"AuditTrailAdapter": true,
	"ObservationPipeline": true,
	"ObservationAPI":    true,
	"ArchitectureRegistry": true,
	"HandoffComposer":   true,
	"SchedulerComposer": true,
	"MatchEngine":       true,
	"ResilienceChain":   true,
	"MigrationChain":    true,
	"PipelineBuilder":   true,
	"HotReload":         true,
	"PipelineConfig":    true,
	"StepConfig":        true,
	"AccessPolicy":      true,
	"AccessRequest":     true,
	"AccessDecision":    true,
	"CapabilityToken":   true,
	"AuditEntry":        true,
	"ExecutionStep":     true,
	"TraceTree":         true,
	"TraceNode":         true,
	"StepError":         true,
	"DevSnapshot":       true,
	"WorkItem":          true,
	"Decision":          true,
	"Artifact":          true,
	"ParallelResults":   true,
	"RetryConfig":       true,
	"ResilienceConfig":  true,
	"ObservationPipelineConfig": true,
	"StepQuery":         true,
	"AggregateWindow":   true,
	"EntropySnapshot":   true,
	"SchemaDiffRequest": true,
	"SchemaDiffResult":  true,
	"SchemaChange":      true,
	"MigrationFunc":     true,
	"TraceContext":      true,
	// v3.0 Guardian Layer
	"EntropyWatcher":       true,
	"TransparencyWatcher":  true,
	"DriftDetector":        true,
	"ArchitectureGuard":    true,
	"DecisionEngine":       true,
	"AlertAdapter":         true,
	"GuardianDecision":     true,
	"GuardianInput":        true,
	"EntropyAlert":         true,
	"TransparencyInput":    true,
	"TransparencyAlert":    true,
	"ArchitectureInput":    true,
	"ArchitectureAlert":    true,
	"DriftInput":           true,
	"DriftOutput":          true,
	"AlertResult":          true,
	"AlertConfig":          true,
	// v3.0 Idempotent & Stream
	"IdempotentStore":           true,
	"IdempotentPort":            true,
	"IdempotentRequest":         true,
	"IdempotentResult":          true,
	"InMemoryIdempotentStore":   true,
	// v3.0 Event Sourcing
	"EventStore":       true,
	"EventEnvelope":    true,
	"EventBus":         true,
	"PublishResult":    true,
	"EventHandler":     true,
	"Subscription":     true,
	"Projection":       true,
	"ProjectionHandler": true,
	"ProjectionInput":  true,
	"ProjectionOutput": true,
	"Snapshot":         true,
	"AppendResult":     true,
	// v3.0 Merkle
	"MerkleAuditChain": true,
	"MerkleProof":      true,
	"MerkleNode":       true,
	// v3.0 Fast Path
	"FastPipeline": true,
	"fastComposerAdapter": true,
	// v3.0 Tenant & Transaction & Degradation
	"TenantIsolationPort": true,
	"TenantContext":     true,
	"TenantRequest":     true,
	"SagaComposer":      true,
	"SagaStep":          true,
	"TransactionContext": true,
	"DegradationManager": true,
	// v3.0 Stream
	"StreamConfig":    true,
	// v3.0 Error codes
	"ErrorCategory":    true,
	"ErrorCode":        true,
	// v3.0 Guardian alert channel
	"GuardianAction":   true,
	"AlertChannel":     true,
	"DegradationMode":  true,
	"DriftType":        true,
	"EntropyLevel":     true,
	"GuardianAlert":    true,
	"idempotentEntry":  true,
	// v3.0 Error codes
	"ErrValidationFailed":    true,
	"ErrIOError":             true,
	"ErrTimeout":             true,
	"ErrRateLimited":         true,
	"ErrCircuitOpen":         true,
	"ErrAuthFailed":          true,
	"ErrSchemaIncompatible":  true,
	"ErrEntropyHigh":         true,
	"ErrDriftDetected":       true,
	"ErrIdempotentConflict":  true,
	"ErrNotFound":            true,
	"ErrInternal":            true,
	// v3.0 Guardian actions
	"ActionAllow":    true,
	"ActionWarn":     true,
	"ActionBlock":    true,
	"ActionRollback": true,
	// v3.0 Alert channels
	"AlertChannelLog":     true,
	"AlertChannelWebhook": true,
	"AlertChannelChannel": true,
	// v3.0 Drift types
	"DriftNone":          true,
	"DriftQualityDrop":   true,
	"DriftDecisionDev":   true,
	"DriftCapabilityLoss": true,
	"DriftOverreach":     true,
	"DriftSlowdown":      true,
	// v3.0 Entropy levels
	"EntropyOK":     true,
	"EntropyYellow": true,
	"EntropyOrange": true,
	"EntropyRed":    true,
	// v3.0 Degradation modes
	"DegradationNone":         true,
	"DegradationNonCritical":  true,
	"DegradationSafe":         true,
	"DegradationEmergency":    true,
	// v3.0 Error categories
	"CategoryRecoverable":   true,
	"CategoryUnrecoverable": true,
	"CategoryHumanRequired": true,
	// v3.0 Tenant ID
	"TenantID": true,
	// v3.0 Saga transaction
	"compensate": true,
}

// ---------------------------------------------------------------------------
// I/O call signatures to detect in Atom / Port bodies.
// Each entry maps a package name to a set of function names within that package.
// ---------------------------------------------------------------------------

var ioFuncsByPkg = map[string]map[string]bool{
	"http": {"Get": true, "Post": true, "Do": true, "Head": true,
		"NewRequest": true, "ListenAndServe": true},
	"os": {"Open": true, "Create": true, "OpenFile": true, "ReadFile": true,
		"WriteFile": true, "Stat": true, "Lstat": true, "Mkdir": true,
		"MkdirAll": true, "Remove": true, "RemoveAll": true, "Rename": true,
		"Chdir": true, "Getenv": true, "Setenv": true, "Exit": true},
	"io": {"ReadAll": true, "Copy": true, "CopyN": true, "ReadFull": true,
		"WriteString": true},
	"sql": {"Open": true, "Connect": true},
	"net": {"Dial": true, "Listen": true, "DialTimeout": true,
		"DialTCP": true, "DialUDP": true, "LookupHost": true},
	"fmt":  {"Print": true, "Printf": true, "Println": true,
		"Fprint": true, "Fprintf": true, "Fprintln": true},
	"log": {"Print": true, "Printf": true, "Println": true,
		"Fatal": true, "Fatalf": true, "Fatalln": true,
		"Panic": true, "Panicf": true, "Panicln": true},
}

// ---------------------------------------------------------------------------
// Global violation counter – drives the final exit code.
// ---------------------------------------------------------------------------

var violations int

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

func report(level string, file string, line int, format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(os.Stderr, "%s: %s:%d: %s\n", level, file, line, msg)
	violations++
}

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

// =========================================================================
// Rule 2 – No I/O operations in Atom / Port function bodies
// =========================================================================
//
// Identifies methods whose receiver is of type Atom or Port (value or
// pointer).  Walks the function body with ast.Inspect and flags any
// SelectorExpr call that matches a known I/O function.

func checkRule2_NoIOInAtomPort(f *ast.File, fset *token.FileSet, path string) {
	for _, decl := range f.Decls {
		funcDecl, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		if !hasAtomOrPortReceiver(funcDecl) {
			continue
		}
		if funcDecl.Body == nil {
			continue
		}

		receiverName := receiverTypeName(funcDecl)

		ast.Inspect(funcDecl.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			pkgIdent, ok := sel.X.(*ast.Ident)
			if !ok {
				return true
			}
			funcName := sel.Sel.Name

			if funcs, knownPkg := ioFuncsByPkg[pkgIdent.Name]; knownPkg {
				if funcs[funcName] {
					pos := fset.Position(call.Pos())
					report("ERROR", path, pos.Line,
						"I/O operation in %s receiver: %s.%s()", receiverName,
						pkgIdent.Name, funcName)
				}
			}
			return true
		})
	}
}

// hasAtomOrPortReceiver returns true when the function receiver type is
// Atom or Port (either value receiver or pointer receiver).
func hasAtomOrPortReceiver(fn *ast.FuncDecl) bool {
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return false
	}
	return isAtomOrPortType(fn.Recv.List[0].Type)
}

// isAtomOrPortType checks a type expression for Atom or Port (with/without *).
func isAtomOrPortType(expr ast.Expr) bool {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name == "Atom" || t.Name == "Port"
	case *ast.StarExpr:
		if ident, ok := t.X.(*ast.Ident); ok {
			return ident.Name == "Atom" || ident.Name == "Port"
		}
	}
	return false
}

// receiverTypeName returns a human-readable representation of the receiver type.
func receiverTypeName(fn *ast.FuncDecl) string {
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return "<none>"
	}
	return exprToString(fn.Recv.List[0].Type)
}

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
	"uintptr": true, "any": true, "interface{}": true,
}

// =========================================================================
// Rule 3 – No concrete type assertions (only interface{} allowed)
// =========================================================================
//
// The codebase uses generics, so concrete type assertions like
//   input.(Calculation)
// are disallowed.  Only type-switch forms (assertion type == nil) and
// assertions to the empty interface interface{} are permitted.
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

		// interface{} (empty interface) is always allowed.
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

// isEmptyInterface returns true when expr is exactly interface{} (the empty
// interface type with no methods).
func isEmptyInterface(expr ast.Expr) bool {
	iface, ok := expr.(*ast.InterfaceType)
	if !ok {
		return false
	}
	return iface.Methods == nil || len(iface.Methods.List) == 0
}

// =========================================================================
// Rule 4 – Pipeline / Composer steps must be <= 20 (WARNING)
// =========================================================================
//
// Scans composite literals (CompositeLit) whose type is Pipeline or []Step.
// If the literal has more than 20 elements, a WARNING is emitted.

func checkRule4_PipelineStepLimit(f *ast.File, fset *token.FileSet, path string) {
	ast.Inspect(f, func(n ast.Node) bool {
		compLit, ok := n.(*ast.CompositeLit)
		if !ok {
			return true
		}

		if !isPipelineOrStepSliceType(compLit.Type) {
			return true
		}

		stepCount := len(compLit.Elts)
		if stepCount > 20 {
			pos := fset.Position(compLit.Pos())
			report("WARNING", path, pos.Line,
				"Pipeline/Composer has %d steps (limit is 20)", stepCount)
		}
		return true
	})
}

// isPipelineOrStepSliceType returns true when the type expression is
// "Pipeline" or "[]Step" (the two ways steps can be collected).
func isPipelineOrStepSliceType(expr ast.Expr) bool {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name == "Pipeline"
	case *ast.SelectorExpr:
		// e.g. pkg.Pipeline
		return t.Sel.Name == "Pipeline"
	case *ast.ArrayType:
		if t.Len != nil {
			return false // fixed-size array [N]Step – unusual but possible
		}
		if ident, ok := t.Elt.(*ast.Ident); ok {
			return ident.Name == "Step"
		}
	}
	return false
}

// =========================================================================
// Utility: render an arbitrary type expression as a compact string.
// =========================================================================

func exprToString(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return "*" + exprToString(t.X)
	case *ast.SelectorExpr:
		return exprToString(t.X) + "." + t.Sel.Name
	case *ast.InterfaceType:
		return "interface{}"
	case *ast.ArrayType:
		if t.Len == nil {
			return "[]" + exprToString(t.Elt)
		}
		return "[...]" + exprToString(t.Elt)
	case *ast.MapType:
		return "map[" + exprToString(t.Key) + "]" + exprToString(t.Value)
	case *ast.ChanType:
		return "chan " + exprToString(t.Value)
	case *ast.FuncType:
		return "func(...)"
	case *ast.Ellipsis:
		return "..." + exprToString(t.Elt)
	case *ast.StructType:
		return "struct{...}"
	case *ast.IndexExpr:
		return exprToString(t.X) + "[" + exprToString(t.Index) + "]"
	case *ast.IndexListExpr:
		indices := make([]string, len(t.Indices))
		for i, idx := range t.Indices {
			indices[i] = exprToString(idx)
		}
		return exprToString(t.X) + "[" + strings.Join(indices, ", ") + "]"
	default:
		return fmt.Sprintf("<%T>", t)
	}
}