package migrate

import (
	"math"
	"testing"
)

// ---------------------------------------------------------------------------
// Mock classifier for generic tests
// ---------------------------------------------------------------------------

type mockClassifier struct {
	name     string
	pattern  CodePattern
	confidence float64
}

func (m *mockClassifier) Name() string { return m.name }

func (m *mockClassifier) Classify(fn *UnifiedFunction) PatternMatch {
	return PatternMatch{
		File:       fn.File,
		Line:       fn.Line,
		FuncName:   fn.Name,
		Pattern:    m.pattern,
		Confidence: m.confidence,
		Evidence:   []string{"mock"},
	}
}

// ---------------------------------------------------------------------------
// TestPatternMap_Stats
// ---------------------------------------------------------------------------

func TestPatternMap_Stats(t *testing.T) {
	pm := &PatternMap{
		Atoms:     make([]PatternMatch, 3),
		Ports:     make([]PatternMatch, 2),
		Adapters:  make([]PatternMatch, 1),
		Composers: make([]PatternMatch, 4),
		Unknowns:  make([]PatternMatch, 0),
	}

	stats := pm.Stats()

	if stats[PatternPureFunc] != 3 {
		t.Errorf("expected 3 atoms, got %d", stats[PatternPureFunc])
	}
	if stats[PatternValidator] != 2 {
		t.Errorf("expected 2 ports, got %d", stats[PatternValidator])
	}
	if stats[PatternIOCall] != 1 {
		t.Errorf("expected 1 adapter, got %d", stats[PatternIOCall])
	}
	if stats[PatternOrchestrator] != 4 {
		t.Errorf("expected 4 composers, got %d", stats[PatternOrchestrator])
	}
	if stats[PatternUnknown] != 0 {
		t.Errorf("expected 0 unknowns, got %d", stats[PatternUnknown])
	}
	if pm.Total() != 10 {
		t.Errorf("expected total 10, got %d", pm.Total())
	}
}

// ---------------------------------------------------------------------------
// TestPatternMap_UnknownRatio
// ---------------------------------------------------------------------------

func TestPatternMap_UnknownRatio(t *testing.T) {
	pm := &PatternMap{
		Atoms:     make([]PatternMatch, 9),
		Ports:     make([]PatternMatch, 0),
		Adapters:  make([]PatternMatch, 0),
		Composers: make([]PatternMatch, 0),
		Unknowns:  make([]PatternMatch, 1), // 1/10 = 10%
	}

	ratio := pm.UnknownRatio()
	expected := 0.1
	if math.Abs(ratio-expected) > 1e-9 {
		t.Errorf("expected unknown ratio %.2f, got %.2f", expected, ratio)
	}
}

// ---------------------------------------------------------------------------
// TestClassifyFunctions
// ---------------------------------------------------------------------------

func TestClassifyFunctions(t *testing.T) {
	functions := []UnifiedFunction{
		{Name: "fnA", File: "a.go", Line: 10},
		{Name: "fnB", File: "b.go", Line: 20},
		{Name: "fnC", File: "c.go", Line: 30},
	}

	classifiers := []PatternClassifier{
		&mockClassifier{name: "atom-mock", pattern: PatternPureFunc, confidence: 0.9},
		&mockClassifier{name: "port-mock", pattern: PatternValidator, confidence: 0.6},
	}

	pm := ClassifyFunctions(functions, classifiers)

	if pm.Total() != 3 {
		t.Errorf("expected total 3, got %d", pm.Total())
	}

	// All three functions should be classified as atom (confidence 0.9 > 0.6).
	if len(pm.Atoms) != 3 {
		t.Errorf("expected 3 atoms, got %d", len(pm.Atoms))
	}
	if len(pm.Ports) != 0 {
		t.Errorf("expected 0 ports, got %d", len(pm.Ports))
	}
}

// ---------------------------------------------------------------------------
// TestAtomClassifier_PureFunction
// ---------------------------------------------------------------------------

func TestAtomClassifier_PureFunction(t *testing.T) {
	fn := &UnifiedFunction{
		Name: "add",
		File: "math.go",
		Line: 5,
		Parameters: []UnifiedParam{
			{Name: "a", Type: "int"},
			{Name: "b", Type: "int"},
		},
		ReturnTypes: []string{"int"},
		BodyNodes: []BodyNode{
			{Type: BodyNodeAssign, Text: "c = a + b"},
			{Type: BodyNodeReturn, Text: "return c"},
		},
		CallGraph: []string{},
	}

	c := &AtomClassifier{}
	m := c.Classify(fn)

	if m.Pattern != PatternPureFunc {
		t.Errorf("expected pattern %s, got %s", PatternPureFunc, m.Pattern)
	}
	if m.Confidence < 0.8 {
		t.Errorf("expected confidence >= 0.8 for pure function, got %.2f", m.Confidence)
	}
}

// ---------------------------------------------------------------------------
// TestAtomClassifier_IOFunction
// ---------------------------------------------------------------------------

func TestAtomClassifier_IOFunction(t *testing.T) {
	fn := &UnifiedFunction{
		Name: "writeData",
		File: "io.go",
		Line: 10,
		Parameters: []UnifiedParam{
			{Name: "w", Type: "io.Writer"},
			{Name: "data", Type: "[]byte"},
		},
		ReturnTypes: []string{"error"},
		BodyNodes: []BodyNode{
			{Type: BodyNodeIO, Text: "w.Write(data)"},
		},
		CallGraph: []string{"Write"},
	}

	c := &AtomClassifier{}
	m := c.Classify(fn)

	if m.Confidence > 0.5 {
		t.Errorf("expected confidence <= 0.5 for IO function, got %.2f", m.Confidence)
	}
}

// ---------------------------------------------------------------------------
// TestPortClassifier_ValidateFunction
// ---------------------------------------------------------------------------

func TestPortClassifier_ValidateFunction(t *testing.T) {
	fn := &UnifiedFunction{
		Name: "validateEmail",
		File: "validator.go",
		Line: 15,
		Parameters: []UnifiedParam{
			{Name: "email", Type: "string"},
		},
		ReturnTypes: []string{"bool", "error"},
		BodyNodes: []BodyNode{
			{Type: BodyNodeIf, Text: "if email == ''"},
			{Type: BodyNodeReturn, Text: "return false, err"},
		},
		CallGraph: []string{},
	}

	c := &PortClassifier{}
	m := c.Classify(fn)

	if m.Pattern != PatternValidator {
		t.Errorf("expected pattern %s, got %s", PatternValidator, m.Pattern)
	}
}

// ---------------------------------------------------------------------------
// TestAdapterClassifier_SaveFunction
// ---------------------------------------------------------------------------

func TestAdapterClassifier_SaveFunction(t *testing.T) {
	fn := &UnifiedFunction{
		Name: "saveUser",
		File: "repo.go",
		Line: 20,
		Parameters: []UnifiedParam{
			{Name: "db", Type: "sql.DB"},
			{Name: "user", Type: "User"},
		},
		ReturnTypes: []string{"error"},
		BodyNodes: []BodyNode{
			{Type: BodyNodeIO, Text: "db.Exec(query, user.Name)"},
		},
		CallGraph: []string{"Exec"},
	}

	c := &AdapterClassifier{}
	m := c.Classify(fn)

	if m.Pattern != PatternIOCall {
		t.Errorf("expected pattern %s, got %s", PatternIOCall, m.Pattern)
	}
}

// ---------------------------------------------------------------------------
// TestComposerClassifier_Orchestrator
// ---------------------------------------------------------------------------

func TestComposerClassifier_Orchestrator(t *testing.T) {
	fn := &UnifiedFunction{
		Name: "processOrder",
		File: "order.go",
		Line: 30,
		Parameters: []UnifiedParam{
			{Name: "ctx", Type: "context.Context"},
			{Name: "order", Type: "Order"},
		},
		ReturnTypes: []string{"error"},
		BodyNodes: []BodyNode{
			{Type: BodyNodeIf, Text: "if order.Valid"},
			{Type: BodyNodeGo, Text: "go sendNotification(order)"},
			{Type: BodyNodeCall, Text: "reserveStock(order)"},
		},
		CallGraph: []string{"validateOrder", "reserveStock", "chargePayment", "sendNotification"},
	}

	c := &ComposerClassifier{}
	m := c.Classify(fn)

	if m.Pattern != PatternOrchestrator {
		t.Errorf("expected pattern %s, got %s", PatternOrchestrator, m.Pattern)
	}
}
