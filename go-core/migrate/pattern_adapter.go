package migrate

import (
	"strings"
)

// AdapterClassifier classifies I/O adapter functions — functions that perform
// file, network, database, or other external I/O operations.
type AdapterClassifier struct{}

// Name returns the classifier name.
func (c *AdapterClassifier) Name() string {
	return "adapter"
}

// adapterKeywords lists name patterns that suggest an I/O adapter function.
var adapterKeywords = []string{
	"save", "load", "create", "update", "delete",
	"send", "fetch", "query", "insert", "connect",
	"write", "read", "close", "open", "execute",
	"publish", "subscribe",
}

// ioTypePatterns lists type patterns that suggest I/O parameters.
var ioTypePatterns = []string{
	"io.", "http", "sql", "database", "db.", "net.",
	"os.file", "writer", "reader", "client", "conn",
}

// Classify evaluates how likely a function is an I/O adapter function.
// Rules:
//   - Contains I/O BodyNode        +0.4
//   - Name contains I/O keywords    +0.2
//   - Parameters contain I/O types +0.1
//   - Base                         0.3
func (c *AdapterClassifier) Classify(fn *UnifiedFunction) PatternMatch {
	confidence := 0.3
	var evidence []string

	// Check for I/O body nodes.
	hasIO := false
	for _, node := range fn.BodyNodes {
		if node.Type == BodyNodeIO {
			hasIO = true
			break
		}
	}
	if hasIO {
		confidence += 0.4
		evidence = append(evidence, "contains I/O body nodes")
	}

	// Check name keywords.
	nameLower := strings.ToLower(fn.Name)
	for _, kw := range adapterKeywords {
		if strings.Contains(nameLower, kw) {
			confidence += 0.2
			evidence = append(evidence, "name contains '"+kw+"'")
			break
		}
	}

	// Check parameter types for I/O indicators.
	hasIOParam := false
	for _, p := range fn.Parameters {
		typeLower := strings.ToLower(p.Type)
		for _, iop := range ioTypePatterns {
			if strings.Contains(typeLower, iop) {
				hasIOParam = true
				break
			}
		}
		if hasIOParam {
			break
		}
	}
	if hasIOParam {
		confidence += 0.1
		evidence = append(evidence, "parameters contain I/O types")
	}

	return PatternMatch{
		File:       fn.File,
		Line:       fn.Line,
		FuncName:   fn.Name,
		Pattern:    PatternIOCall,
		Confidence: minConfidence(confidence),
		Evidence:   evidence,
	}
}
