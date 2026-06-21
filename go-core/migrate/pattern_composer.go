package migrate

// ComposerClassifier classifies orchestrator functions — functions that
// coordinate multiple sub-calls, conditional branching, and parallel patterns.
type ComposerClassifier struct{}

// Name returns the classifier name.
func (c *ComposerClassifier) Name() string {
	return "composer"
}

// Classify evaluates how likely a function is an orchestrator/composer function.
// Rules:
//   - Calls >= 3 functions       +0.3
//   - Contains conditional branches +0.2
//   - Contains parallel patterns (go/select) +0.2
//   - Base                        0.3
func (c *ComposerClassifier) Classify(fn *UnifiedFunction) PatternMatch {
	confidence := 0.3
	var evidence []string

	// Check call graph size.
	if len(fn.CallGraph) >= 3 {
		confidence += 0.3
		evidence = append(evidence, "calls >= 3 functions")
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

	// Check for parallel patterns (go/select).
	hasParallel := false
	for _, node := range fn.BodyNodes {
		if node.Type == BodyNodeGo || node.Type == BodyNodeSelect {
			hasParallel = true
			break
		}
	}
	if hasParallel {
		confidence += 0.2
		evidence = append(evidence, "contains parallel patterns (go/select)")
	}

	return PatternMatch{
		File:       fn.File,
		Line:       fn.Line,
		FuncName:   fn.Name,
		Pattern:    PatternOrchestrator,
		Confidence: minConfidence(confidence),
		Evidence:   evidence,
	}
}
