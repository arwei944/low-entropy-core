package migrate

import "fmt"

// AdapterTransformer transforms functions into I/O adapter functions.
// It implements the CodeTransformer interface and only handles the
// PatternIOCall target pattern.
type AdapterTransformer struct{}

// Name returns the unique identifier for this transformer.
func (t *AdapterTransformer) Name() string {
	return "adapter-transformer"
}

// Transform converts the given function into an I/O adapter function.
// It verifies that the target is PatternIOCall, generates placeholder
// transformed code, and records a MigrationLogEntry for the signature change.
func (t *AdapterTransformer) Transform(fn *UnifiedFunction, target CodePattern) (*TransformResult, error) {
	if target != PatternIOCall {
		return nil, fmt.Errorf("adapter-transformer: unsupported target pattern %q, expected %q", target, PatternIOCall)
	}

	originalCode := fn.Signature
	if originalCode == "" {
		originalCode = fmt.Sprintf("func %s(...)", fn.Name)
	}

	transformedCode := fmt.Sprintf(
		"// adapter: I/O function\nfunc %s(%s) %s {\n\t// transformed by adapter-transformer\n\tresult, err := ioHandler(%s)\n\tif err != nil {\n\t\treturn nil, err\n\t}\n\treturn result, nil\n}",
		fn.Name,
		formatParams(fn.Parameters),
		formatReturnTypes(fn.ReturnTypes),
		fn.Name,
	)

	diff := fmt.Sprintf("--- original\n+++ transformed (adapter)\n- %s\n+ %s", originalCode, transformedCode)

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
