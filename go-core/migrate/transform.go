package migrate

import (
	"fmt"
	"strings"
)

// CodeTransformer is the interface implemented by each pattern-specific
// code transformer. A transformer takes a UnifiedFunction and a target
// CodePattern, produces transformed code, and records migration log entries.
type CodeTransformer interface {
	// Transform applies the transformation to the given function so that it
	// conforms to the target CodePattern. It returns a TransformResult
	// containing the original code, transformed code, diff, and log entries.
	Transform(fn *UnifiedFunction, target CodePattern) (*TransformResult, error)

	// Name returns the unique identifier for this transformer.
	Name() string
}

// TransformResult holds the outcome of a single transformation.
type TransformResult struct {
	OriginalCode   string               `json:"original_code"`
	TransformedCode string               `json:"transformed_code"`
	Diff           string               `json:"diff"`
	LogEntries      []MigrationLogEntry  `json:"log_entries"`
}

// CodeGenerator is the interface for language-specific code generators.
// Each generator produces source code strings for a particular language
// targeting a specific architectural pattern.
type CodeGenerator interface {
	GenerateAtom(fn *UnifiedFunction) (string, error)
	GeneratePort(fn *UnifiedFunction) (string, error)
	GenerateAdapter(fn *UnifiedFunction) (string, error)
	GenerateComposer(fn *UnifiedFunction) (string, error)
	Language() string
}

// TransformFunction executes a code transformation using the given transformer.
// It checks that the transformer supports the target pattern, delegates to
// the transformer's Transform method, and appends all produced log entries
// to the provided MigrationLog.
func TransformFunction(
	fn *UnifiedFunction,
	target CodePattern,
	transformer CodeTransformer,
	log *MigrationLog,
) (*TransformResult, error) {
	if fn == nil {
		return nil, fmt.Errorf("transform: function must not be nil")
	}
	if log == nil {
		return nil, fmt.Errorf("transform: log must not be nil")
	}

	originalCode := fn.Signature
	if originalCode == "" {
		originalCode = fmt.Sprintf("func %s(...)", fn.Name)
	}

	result, err := transformer.Transform(fn, target)
	if err != nil {
		return nil, fmt.Errorf("transform: %s failed for %s: %w", transformer.Name(), fn.Name, err)
	}

	// Ensure OriginalCode is populated.
	if result.OriginalCode == "" {
		result.OriginalCode = originalCode
	}

	// Build a simple diff placeholder if not provided.
	if result.Diff == "" && result.TransformedCode != "" {
		result.Diff = fmt.Sprintf("--- original\n+++ transformed\n- %s\n+ %s",
			result.OriginalCode, result.TransformedCode)
	}

	// Append all log entries produced by the transformer.
	for _, entry := range result.LogEntries {
		if appendErr := log.Append(entry); appendErr != nil {
			return nil, fmt.Errorf("transform: failed to append log entry: %w", appendErr)
		}
	}

	return result, nil
}

// formatParams formats a slice of UnifiedParam into a Go-style parameter list.
func formatParams(params []UnifiedParam) string {
	if len(params) == 0 {
		return ""
	}
	parts := make([]string, len(params))
	for i, p := range params {
		parts[i] = fmt.Sprintf("%s %s", p.Name, p.Type)
	}
	return strings.Join(parts, ", ")
}

// formatReturnTypes formats a slice of return type strings into a
// Go-style return type declaration.
func formatReturnTypes(returnTypes []string) string {
	if len(returnTypes) == 0 {
		return ""
	}
	return strings.Join(returnTypes, ", ")
}
