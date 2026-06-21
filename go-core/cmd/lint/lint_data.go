// Static analysis CLI tool (TASK-6.1) — lint data declarations
//
// This file contains the data tables and helper functions used by the
// lint tool's rule-checking logic. Extracted from main.go to keep it
// under 300 lines.
//
// Rules:
//   1. No non-4-primitive interface type definitions (ERROR)
//   2. No I/O operations in Atom/Port function bodies (ERROR)
//   3. No concrete type assertions beyond any (ERROR)
//   4. Pipeline steps must be <= 20 (WARNING)

package main

import (
	"fmt"
	"go/ast"
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
		return "any"
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
