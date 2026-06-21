package migrate

import (
	"strings"
)

// AtomClassifier classifies pure functions (atoms) — functions with no I/O,
// no context parameters, and no external calls.
type AtomClassifier struct{}

// Name returns the classifier name.
func (c *AtomClassifier) Name() string {
	return "atom"
}

// Classify evaluates how likely a function is a pure/atomic function.
// Rules:
//   - No I/O BodyNode          +0.3
//   - No context/IO parameters +0.1
//   - No external calls        +0.1
//   - Base                     0.5
func (c *AtomClassifier) Classify(fn *UnifiedFunction) PatternMatch {
	confidence := 0.5
	var evidence []string

	// Check for absence of I/O body nodes.
	hasIO := false
	for _, node := range fn.BodyNodes {
		if node.Type == BodyNodeIO {
			hasIO = true
			break
		}
	}
	if !hasIO {
		confidence += 0.3
		evidence = append(evidence, "no I/O body nodes")
	}

	// Check for absence of context/IO parameters.
	hasContextOrIO := false
	for _, p := range fn.Parameters {
		lower := strings.ToLower(p.Type)
		if strings.Contains(lower, "context") ||
			strings.Contains(lower, "io.") ||
			strings.Contains(lower, "http") ||
			strings.Contains(lower, "sql") ||
			strings.Contains(lower, "database") ||
			strings.Contains(lower, "db.") ||
			strings.Contains(lower, "net.") ||
			strings.Contains(lower, "os.file") ||
			strings.Contains(lower, "writer") ||
			strings.Contains(lower, "reader") {
			hasContextOrIO = true
			break
		}
	}
	if !hasContextOrIO {
		confidence += 0.1
		evidence = append(evidence, "no context/IO parameters")
	}

	// Check for absence of external calls.
	if len(fn.CallGraph) == 0 {
		confidence += 0.1
		evidence = append(evidence, "no external calls")
	}

	return PatternMatch{
		File:       fn.File,
		Line:       fn.Line,
		FuncName:   fn.Name,
		Pattern:    PatternPureFunc,
		Confidence: minConfidence(confidence),
		Evidence:   evidence,
	}
}
