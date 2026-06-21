package migrate

import "testing"

// ---------------------------------------------------------------------------
// GateAllClassified
// ---------------------------------------------------------------------------

func TestGateAllClassified_AllClassified(t *testing.T) {
	gate := GateAllClassified{}
	ctx := &MigrationContext{
		PatternMap: &PatternMap{
			Atoms: []PatternMatch{
				{File: "a.go", FuncName: "Foo", Pattern: PatternPureFunc},
			},
			Unknowns: nil,
		},
	}

	decision := gate.Evaluate(ctx)
	if !decision.Pass {
		t.Fatalf("expected Pass=true, got Pass=%v, blocked_rules=%v", decision.Pass, decision.BlockedRules)
	}
	if decision.GateID != "G2" {
		t.Errorf("expected GateID=G2, got %s", decision.GateID)
	}
}

func TestGateAllClassified_HasUnknown(t *testing.T) {
	gate := GateAllClassified{}
	ctx := &MigrationContext{
		PatternMap: &PatternMap{
			Atoms: []PatternMatch{
				{File: "a.go", FuncName: "Foo", Pattern: PatternPureFunc},
			},
			Unknowns: []PatternMatch{
				{File: "foo.go", FuncName: "Bar", Pattern: PatternUnknown},
				{File: "bar.go", FuncName: "Baz", Pattern: PatternUnknown},
			},
		},
	}

	decision := gate.Evaluate(ctx)
	if decision.Pass {
		t.Fatal("expected Pass=false for unknown files, got Pass=true")
	}
	if decision.GateID != "G2" {
		t.Errorf("expected GateID=G2, got %s", decision.GateID)
	}
	if len(decision.BlockedRules) == 0 {
		t.Error("expected blocked rules to be non-empty")
	}
}

// ---------------------------------------------------------------------------
// GateUnknownRatio
// ---------------------------------------------------------------------------

func TestGateUnknownRatio_WithinThreshold(t *testing.T) {
	gate := GateUnknownRatio{}
	// Build a PatternMap with 90 atoms + 10 unknowns = 100 total => ratio = 0.1 (<= 0.1)
	atoms := make([]PatternMatch, 90)
	for i := range atoms {
		atoms[i] = PatternMatch{File: "a.go", FuncName: "Fn", Pattern: PatternPureFunc}
	}
	unknowns := make([]PatternMatch, 10)
	for i := range unknowns {
		unknowns[i] = PatternMatch{File: "u.go", FuncName: "Unk", Pattern: PatternUnknown}
	}
	ctx := &MigrationContext{
		PatternMap: &PatternMap{
			Atoms:    atoms,
			Unknowns: unknowns,
		},
	}

	decision := gate.Evaluate(ctx)
	if !decision.Pass {
		t.Fatalf("expected Pass=true for 10%% unknown ratio, got Pass=%v, blocked_rules=%v",
			decision.Pass, decision.BlockedRules)
	}
	if decision.GateID != "G3" {
		t.Errorf("expected GateID=G3, got %s", decision.GateID)
	}
}

func TestGateUnknownRatio_ExceedsThreshold(t *testing.T) {
	gate := GateUnknownRatio{}
	// Build a PatternMap with 89 atoms + 11 unknowns = 100 total => ratio = 0.11 (> 0.1)
	atoms := make([]PatternMatch, 89)
	for i := range atoms {
		atoms[i] = PatternMatch{File: "a.go", FuncName: "Fn", Pattern: PatternPureFunc}
	}
	unknowns := make([]PatternMatch, 11)
	for i := range unknowns {
		unknowns[i] = PatternMatch{File: "u.go", FuncName: "Unk", Pattern: PatternUnknown}
	}
	ctx := &MigrationContext{
		PatternMap: &PatternMap{
			Atoms:    atoms,
			Unknowns: unknowns,
		},
	}

	decision := gate.Evaluate(ctx)
	if decision.Pass {
		t.Fatal("expected Pass=false for >10%% unknown ratio, got Pass=true")
	}
	if decision.GateID != "G3" {
		t.Errorf("expected GateID=G3, got %s", decision.GateID)
	}
	if len(decision.BlockedRules) == 0 {
		t.Error("expected blocked rules to be non-empty")
	}
}

// ---------------------------------------------------------------------------
// GateChain
// ---------------------------------------------------------------------------

func TestGateChain_FirstBlock(t *testing.T) {
	// Create a chain where the first gate fails.
	// GateParseCoverage will fail because ctx.Files is empty.
	chain := NewGateChain(
		GateParseCoverage{},
		GateAllClassified{},
		GateUnknownRatio{},
	)

	ctx := &MigrationContext{
		Files: []*UnifiedFile{},
		PatternMap: &PatternMap{
			Atoms: []PatternMatch{
				{File: "a.go", FuncName: "Foo", Pattern: PatternPureFunc},
			},
		},
	}

	decision := chain.Evaluate(ctx)
	if decision.Pass {
		t.Fatal("expected chain to fail at first gate, got Pass=true")
	}
	if decision.GateID != "G1" {
		t.Errorf("expected failure at G1, got GateID=%s", decision.GateID)
	}
}

// ---------------------------------------------------------------------------
// GateParseCoverage
// ---------------------------------------------------------------------------

func TestGateParseCoverage_NoFiles(t *testing.T) {
	gate := GateParseCoverage{}
	ctx := &MigrationContext{
		Files: []*UnifiedFile{},
	}

	decision := gate.Evaluate(ctx)
	if decision.Pass {
		t.Fatal("expected Pass=false when no files, got Pass=true")
	}
	if decision.GateID != "G1" {
		t.Errorf("expected GateID=G1, got %s", decision.GateID)
	}
	if len(decision.BlockedRules) == 0 {
		t.Error("expected blocked rules to be non-empty")
	}
}

// ---------------------------------------------------------------------------
// DefaultGateChain
// ---------------------------------------------------------------------------

func TestDefaultGateChain_Created(t *testing.T) {
	chain := DefaultGateChain()
	if chain == nil {
		t.Fatal("DefaultGateChain returned nil")
	}
	if len(chain.gates) != 6 {
		t.Errorf("expected 6 gates, got %d", len(chain.gates))
	}

	expectedIDs := []string{"G1", "G2", "G3", "G4", "G5", "G6"}
	for i, expected := range expectedIDs {
		if chain.gates[i].ID() != expected {
			t.Errorf("gate[%d]: expected ID=%s, got %s", i, expected, chain.gates[i].ID())
		}
	}
}
