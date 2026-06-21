package migrate

import "math"

// CodePattern represents a classification category for code patterns.
type CodePattern string

const (
	PatternPureFunc      CodePattern = "atom"
	PatternValidator     CodePattern = "port"
	PatternIOCall        CodePattern = "adapter"
	PatternOrchestrator  CodePattern = "composer"
	PatternUnknown       CodePattern = "unknown"
)

// PatternMatch holds the result of classifying a single function.
type PatternMatch struct {
	File       string       `json:"file"`
	Line       int          `json:"line"`
	FuncName   string       `json:"func_name"`
	Pattern    CodePattern  `json:"pattern"`
	Confidence float64      `json:"confidence"`
	Evidence   []string     `json:"evidence"`
}

// PatternClassifier is the interface implemented by each pattern classifier.
type PatternClassifier interface {
	Classify(fn *UnifiedFunction) PatternMatch
	Name() string
}

// PatternMap collects classified functions grouped by pattern.
type PatternMap struct {
	Atoms     []PatternMatch `json:"atoms"`
	Ports     []PatternMatch `json:"ports"`
	Adapters  []PatternMatch `json:"adapters"`
	Composers []PatternMatch `json:"composers"`
	Unknowns  []PatternMatch `json:"unknowns"`
}

// Stats returns a count map for each pattern.
func (pm *PatternMap) Stats() map[CodePattern]int {
	return map[CodePattern]int{
		PatternPureFunc:     len(pm.Atoms),
		PatternValidator:   len(pm.Ports),
		PatternIOCall:       len(pm.Adapters),
		PatternOrchestrator: len(pm.Composers),
		PatternUnknown:      len(pm.Unknowns),
	}
}

// Total returns the total number of classified functions.
func (pm *PatternMap) Total() int {
	return len(pm.Atoms) + len(pm.Ports) + len(pm.Adapters) +
		len(pm.Composers) + len(pm.Unknowns)
}

// UnknownRatio returns the ratio of unknown-classified functions (0.0 - 1.0).
func (pm *PatternMap) UnknownRatio() float64 {
	total := pm.Total()
	if total == 0 {
		return 0.0
	}
	return float64(len(pm.Unknowns)) / float64(total)
}

// minConfidence truncates a confidence value to [0, 1].
func minConfidence(v float64) float64 {
	return math.Min(v, 1.0)
}

// ClassifyFunctions runs all classifiers on every function and picks the
// best (highest confidence) match for each function. If no classifier
// exceeds 0.5 confidence the function is placed in Unknowns.
func ClassifyFunctions(functions []UnifiedFunction, classifiers []PatternClassifier) *PatternMap {
	pm := &PatternMap{}

	for i := range functions {
		fn := &functions[i]
		best := PatternMatch{
			File:       fn.File,
			Line:       fn.Line,
			FuncName:   fn.Name,
			Pattern:    PatternUnknown,
			Confidence: 0,
			Evidence:   nil,
		}

		for _, c := range classifiers {
			m := c.Classify(fn)
			if m.Confidence > best.Confidence {
				best = m
			}
		}

		switch best.Pattern {
		case PatternPureFunc:
			pm.Atoms = append(pm.Atoms, best)
		case PatternValidator:
			pm.Ports = append(pm.Ports, best)
		case PatternIOCall:
			pm.Adapters = append(pm.Adapters, best)
		case PatternOrchestrator:
			pm.Composers = append(pm.Composers, best)
		default:
			pm.Unknowns = append(pm.Unknowns, best)
		}
	}

	return pm
}
