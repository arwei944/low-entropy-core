package migrate

import "fmt"

// ComposerTransformer transforms functions into orchestrator/composer functions.
// It implements the CodeTransformer interface and only handles the
// PatternOrchestrator target pattern.
type ComposerTransformer struct{}

// Name returns the unique identifier for this transformer.
func (t *ComposerTransformer) Name() string {
	return "composer-transformer"
}

// Transform converts the given function into an orchestrator/composer function.
// It verifies that the target is PatternOrchestrator, generates placeholder
// transformed code, and records a MigrationLogEntry for the signature change.
func (t *ComposerTransformer) Transform(fn *UnifiedFunction, target CodePattern) (*TransformResult, error) {
	if target != PatternOrchestrator {
		return nil, fmt.Errorf("composer-transformer: unsupported target pattern %q, expected %q", target, PatternOrchestrator)
	}

	originalCode := fn.Signature
	if originalCode == "" {
		originalCode = fmt.Sprintf("func %s(...)", fn.Name)
	}

	transformedCode := fmt.Sprintf(
		"// composer: orchestrator function\nfunc %s(%s) %s {\n\t// transformed by composer-transformer\n\t// orchestrate sub-calls\n\tfor _, step := range steps {\n\t\tif err := step.Execute(ctx); err != nil {\n\t\t\treturn err\n\t\t}\n\t}\n\treturn nil\n}",
		fn.Name,
		formatParams(fn.Parameters),
		formatReturnTypes(fn.ReturnTypes),
	)

	diff := fmt.Sprintf("--- original\n+++ transformed (composer)\n- %s\n+ %s", originalCode, transformedCode)

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
