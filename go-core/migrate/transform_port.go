package migrate

import "fmt"

// PortTransformer transforms functions into validator/port functions.
// It implements the CodeTransformer interface and only handles the
// PatternValidator target pattern.
type PortTransformer struct{}

// Name returns the unique identifier for this transformer.
func (t *PortTransformer) Name() string {
	return "port-transformer"
}

// Transform converts the given function into a validator/port function.
// It verifies that the target is PatternValidator, generates placeholder
// transformed code, and records a MigrationLogEntry for the signature change.
func (t *PortTransformer) Transform(fn *UnifiedFunction, target CodePattern) (*TransformResult, error) {
	if target != PatternValidator {
		return nil, fmt.Errorf("port-transformer: unsupported target pattern %q, expected %q", target, PatternValidator)
	}

	originalCode := fn.Signature
	if originalCode == "" {
		originalCode = fmt.Sprintf("func %s(...)", fn.Name)
	}

	transformedCode := fmt.Sprintf(
		"// port: validator function\nfunc %s(%s) %s {\n\t// transformed by port-transformer\n\tif err := validate(%s); err != nil {\n\t\treturn err\n\t}\n\treturn nil\n}",
		fn.Name,
		formatParams(fn.Parameters),
		formatReturnTypes(fn.ReturnTypes),
		fn.Name,
	)

	diff := fmt.Sprintf("--- original\n+++ transformed (port)\n- %s\n+ %s", originalCode, transformedCode)

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
