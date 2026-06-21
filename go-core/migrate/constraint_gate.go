package migrate

import "fmt"

// GateDecision represents the result of a constraint gate evaluation.
type GateDecision struct {
	Pass         bool     `json:"pass"`
	GateID       string   `json:"gate_id"`
	BlockedRules []string `json:"blocked_rules"`
	Warnings     []string `json:"warnings"`
}

// ConstraintGate is the interface for all migration constraint gates.
type ConstraintGate interface {
	ID() string
	Evaluate(ctx *MigrationContext) GateDecision
}

// MigrationContext carries all state needed for gate evaluation.
type MigrationContext struct {
	Files      []*UnifiedFile `json:"files"`
	PatternMap *PatternMap    `json:"pattern_map"`
	Log        *MigrationLog  `json:"log"`
	Phase      string         `json:"phase"`
	TargetTier string         `json:"target_tier"`
	ProjectDir string         `json:"project_dir"`
	Language   string         `json:"language"`
}

// ---------------------------------------------------------------------------
// GateChain
// ---------------------------------------------------------------------------

// GateChain evaluates a sequence of ConstraintGate instances in order.
// It short-circuits on the first failing gate.
type GateChain struct {
	gates []ConstraintGate
}

// Evaluate runs each gate in sequence. If any gate fails, it returns that
// GateDecision immediately without evaluating subsequent gates.
func (gc *GateChain) Evaluate(ctx *MigrationContext) GateDecision {
	for _, g := range gc.gates {
		decision := g.Evaluate(ctx)
		if !decision.Pass {
			return decision
		}
	}
	return GateDecision{Pass: true}
}

// NewGateChain creates a GateChain from the provided gates.
func NewGateChain(gates ...ConstraintGate) *GateChain {
	return &GateChain{gates: gates}
}

// ---------------------------------------------------------------------------
// Built-in Gates
// ---------------------------------------------------------------------------

// GateParseCoverage (G1) checks that at least one file was parsed.
type GateParseCoverage struct{}

func (g GateParseCoverage) ID() string { return "G1" }

func (g GateParseCoverage) Evaluate(ctx *MigrationContext) GateDecision {
	if len(ctx.Files) > 0 {
		return GateDecision{Pass: true, GateID: g.ID()}
	}
	return GateDecision{
		Pass:         false,
		GateID:       g.ID(),
		BlockedRules: []string{"no files parsed"},
	}
}

// GateAllClassified (G2) checks that all files have been classified
// (i.e., no unknowns in the PatternMap).
type GateAllClassified struct{}

func (g GateAllClassified) ID() string { return "G2" }

func (g GateAllClassified) Evaluate(ctx *MigrationContext) GateDecision {
	if ctx.PatternMap == nil {
		return GateDecision{
			Pass:         false,
			GateID:       g.ID(),
			BlockedRules: []string{"pattern map is nil"},
		}
	}
	if len(ctx.PatternMap.Unknowns) == 0 {
		return GateDecision{Pass: true, GateID: g.ID()}
	}
	// Collect unknown file names as warnings.
	warnings := make([]string, 0, len(ctx.PatternMap.Unknowns))
	for _, u := range ctx.PatternMap.Unknowns {
		warnings = append(warnings, u.File+":"+u.FuncName)
	}
	return GateDecision{
		Pass:         false,
		GateID:       g.ID(),
		BlockedRules: []string{"unclassified functions present"},
		Warnings:     warnings,
	}
}

// GateUnknownRatio (G3) checks that the ratio of unknown functions does not
// exceed 10%.
type GateUnknownRatio struct{}

func (g GateUnknownRatio) ID() string { return "G3" }

func (g GateUnknownRatio) Evaluate(ctx *MigrationContext) GateDecision {
	if ctx.PatternMap == nil {
		return GateDecision{
			Pass:         false,
			GateID:       g.ID(),
			BlockedRules: []string{"pattern map is nil"},
		}
	}
	ratio := ctx.PatternMap.UnknownRatio()
	if ratio <= 0.1 {
		return GateDecision{Pass: true, GateID: g.ID()}
	}
	return GateDecision{
		Pass:         false,
		GateID:       g.ID(),
		BlockedRules: []string{fmt.Sprintf("unknown ratio %.2f exceeds 0.1", ratio)},
	}
}

// GateAtomicLog (G4) checks that the migration log exists and has entries.
type GateAtomicLog struct{}

func (g GateAtomicLog) ID() string { return "G4" }

func (g GateAtomicLog) Evaluate(ctx *MigrationContext) GateDecision {
	if ctx.Log == nil {
		return GateDecision{
			Pass:         false,
			GateID:       g.ID(),
			BlockedRules: []string{"migration log is nil"},
		}
	}
	if len(ctx.Log.Entries) > 0 {
		return GateDecision{Pass: true, GateID: g.ID()}
	}
	return GateDecision{
		Pass:         false,
		GateID:       g.ID(),
		BlockedRules: []string{"migration log has no entries"},
	}
}

// GateCompileCheck (G5) is a placeholder gate that always passes.
// It will be implemented with actual compilation checks in a future iteration.
type GateCompileCheck struct{}

func (g GateCompileCheck) ID() string { return "G5" }

func (g GateCompileCheck) Evaluate(ctx *MigrationContext) GateDecision {
	return GateDecision{Pass: true, GateID: g.ID()}
}

// GateLogIntegrity (G6) verifies the integrity of the migration log
// (SeqNo continuity and Checksum consistency).
type GateLogIntegrity struct{}

func (g GateLogIntegrity) ID() string { return "G6" }

func (g GateLogIntegrity) Evaluate(ctx *MigrationContext) GateDecision {
	if ctx.Log == nil {
		return GateDecision{
			Pass:         false,
			GateID:       g.ID(),
			BlockedRules: []string{"migration log is nil"},
		}
	}
	if err := ctx.Log.VerifyIntegrity(); err != nil {
		return GateDecision{
			Pass:         false,
			GateID:       g.ID(),
			BlockedRules: []string{err.Error()},
		}
	}
	return GateDecision{Pass: true, GateID: g.ID()}
}

// DefaultGateChain returns a GateChain with all six built-in gates in order:
// G1 -> G2 -> G3 -> G4 -> G5 -> G6.
func DefaultGateChain() *GateChain {
	return NewGateChain(
		GateParseCoverage{},
		GateAllClassified{},
		GateUnknownRatio{},
		GateAtomicLog{},
		GateCompileCheck{},
		GateLogIntegrity{},
	)
}
