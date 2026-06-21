package migrate

import (
	"strings"
)

// PortClassifier classifies validator/port functions — functions that validate,
// check, parse, normalize, sanitize, authenticate, or authorize data.
type PortClassifier struct{}

// Name returns the classifier name.
func (c *PortClassifier) Name() string {
	return "port"
}

// portKeywords lists name patterns that suggest a validator/port function.
var portKeywords = []string{
	"validate", "check", "ensure", "parse", "normalize",
	"sanitize", "authenticate", "authorize",
}

// Classify evaluates how likely a function is a validator/port function.
// Rules:
//   - Name contains validate/check/ensure/parse/normalize/sanitize/authenticate/authorize +0.3
//   - Contains conditional branches (if nodes) +0.2
//   - Returns error type +0.2
//   - Base 0.3
func (c *PortClassifier) Classify(fn *UnifiedFunction) PatternMatch {
	confidence := 0.3
	var evidence []string

	// Check name keywords.
	nameLower := strings.ToLower(fn.Name)
	for _, kw := range portKeywords {
		if strings.Contains(nameLower, kw) {
			confidence += 0.3
			evidence = append(evidence, "name contains '"+kw+"'")
			break
		}
	}

	// Check for conditional branches.
	hasIf := false
	for _, node := range fn.BodyNodes {
		if node.Type == BodyNodeIf {
			hasIf = true
			break
		}
	}
	if hasIf {
		confidence += 0.2
		evidence = append(evidence, "contains conditional branches")
	}

	// Check for error return type.
	for _, rt := range fn.ReturnTypes {
		if strings.Contains(rt, "error") {
			confidence += 0.2
			evidence = append(evidence, "returns error type")
			break
		}
	}

	return PatternMatch{
		File:       fn.File,
		Line:       fn.Line,
		FuncName:   fn.Name,
		Pattern:    PatternValidator,
		Confidence: minConfidence(confidence),
		Evidence:   evidence,
	}
}
