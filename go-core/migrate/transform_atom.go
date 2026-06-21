package migrate

import "fmt"

// AtomTransformer transforms functions into pure/atomic functions (atoms).
// It implements the CodeTransformer interface and only handles the
// PatternPureFunc target pattern.
type AtomTransformer struct{}

// Name returns the unique identifier for this transformer.
func (t *AtomTransformer) Name() string {
	return "atom-transformer"
}

// Transform converts the given function into a pure atomic function.
// It verifies that the target is PatternPureFunc, generates placeholder
// transformed code, and records a MigrationLogEntry for the signature change.
func (t *AtomTransformer) Transform(fn *UnifiedFunction, target CodePattern) (*TransformResult, error) {
	if target != PatternPureFunc {
		return nil, fmt.Errorf("atom-transformer: unsupported target pattern %q, expected %q", target, PatternPureFunc)
	}

	originalCode := fn.Signature
	if originalCode == "" {
		originalCode = fmt.Sprintf("func %s(...)", fn.Name)
	}

	transformedCode := fmt.Sprintf(
		"// atom: pure function\nfunc %s(%s) %s {\n\t// transformed by atom-transformer\n\treturn nil\n}",
		fn.Name,
		formatParams(fn.Parameters),
		formatReturnTypes(fn.ReturnTypes),
	)

	diff := fmt.Sprintf("--- original\n+++ transformed (atom)\n- %s\n+ %s", originalCode, transformedCode)

	logEntry := MigrationLogEntry{
		Phase:      "transform",
		ActionType: "signature_change",
		FilePath:   fn.File,
		LineStart:  fn.Line,
		LineEnd:    fn.Line,
		Original:   originalCode,
		Transformed: transformedCode,
		Diff:       diff,
		Metadata: map[string]string{
			"transformer": t.Name(),
			"target":       string(target),
			"func_name":    fn.Name,
		},
	}

	return &TransformResult{
		OriginalCode:    originalCode,
		TransformedCode: transformedCode,
		Diff:            diff,
		LogEntries:      []MigrationLogEntry{logEntry},
	}, nil
}
