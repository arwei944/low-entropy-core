//go:build lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

package core

import (
	"context"
	"strings"
	"testing"
)

func TestStaticGuardPort_ManifestMismatch(t *testing.T) {
	g := newTestStaticGuard()

	sub := AgentCodeSubmission{
		AgentID: "agent-bad",
		TaskID:  "task-001",
		SourceCode: `package main

type myAtom2 struct{}
func (a *myAtom2) Run(ctx context.Context, input int) (int, error) {
	return input, nil
}

func main() {}`,
		Manifest: []PrimitiveManifest{
			{PrimitiveType: "Atom", Name: "myAtom1", Layer: "L1", InputType: "int", OutputType: "int"},
		},
		Attempt: 1,
	}

	result, err := g.Validate(context.Background(), sub)
	if err != nil {
		t.Fatalf("Validate() unexpected error: %v", err)
	}

	hasMismatch := false
	for _, v := range result.Violations {
		if v.Rule == "manifest_mismatch" {
			hasMismatch = true
		}
	}
	if !hasMismatch {
		t.Error("expected manifest_mismatch violation")
	}

	hasMissing := false
	for _, v := range result.Violations {
		if v.Rule == "manifest_missing" {
			hasMissing = true
		}
	}
	if !hasMissing {
		t.Error("expected manifest_missing violation")
	}
}

func TestStaticGuardPort_ComplexityWarnings(t *testing.T) {
	g := newTestStaticGuard()

	var sb strings.Builder
	sb.WriteString("package main\n\nimport \"context\"\n\n")
	sb.WriteString("type longAtom struct{}\n")
	sb.WriteString("func (a *longAtom) Run(ctx context.Context, input int) (int, error) {\n")
	sb.WriteString("\tx := input\n")
	for i := 0; i < 60; i++ {
		sb.WriteString("\tx = x + 1\n")
	}
	sb.WriteString("\treturn x, nil\n}\n\nfunc main() {}\n")

	sub := AgentCodeSubmission{
		AgentID:    "agent-test",
		TaskID:     "task-001",
		SourceCode: sb.String(),
		Manifest: []PrimitiveManifest{
			{PrimitiveType: "Atom", Name: "longAtom", Layer: "L1", InputType: "int", OutputType: "int"},
		},
		Attempt: 1,
	}

	result, err := g.Validate(context.Background(), sub)
	if err != nil {
		t.Fatalf("Validate() unexpected error: %v", err)
	}

	if result.HasErrors() {
		t.Errorf("complexity violations should be warnings, not errors: %v", result.Violations)
	}

	hasTooLong := false
	for _, v := range result.Violations {
		if v.Rule == "function_too_long" {
			hasTooLong = true
		}
	}
	if !hasTooLong {
		t.Error("expected function_too_long violation")
	}
}

func TestStaticGuardPort_ParseError(t *testing.T) {
	g := newTestStaticGuard()

	sub := AgentCodeSubmission{
		AgentID: "agent-bad",
		TaskID:  "task-001",
		SourceCode: `package main

func main() { // 缺少闭合括号
`,
		Manifest: []PrimitiveManifest{
			{PrimitiveType: "Atom", Name: "test", Layer: "L1", InputType: "int", OutputType: "int"},
		},
		Attempt: 1,
	}

	result, err := g.Validate(context.Background(), sub)
	if err != nil {
		t.Fatalf("Validate() unexpected error: %v", err)
	}

	if !result.HasErrors() {
		t.Error("expected parse_error for malformed code")
	}

	hasParseError := false
	for _, v := range result.Violations {
		if v.Rule == "parse_error" {
			hasParseError = true
		}
	}
	if !hasParseError {
		t.Error("expected parse_error violation")
	}
}

func TestStaticGuardPort_EmptySourceCode(t *testing.T) {
	g := newTestStaticGuard()

	sub := AgentCodeSubmission{
		AgentID:    "agent-bad",
		TaskID:     "task-001",
		SourceCode: "",
		Manifest: []PrimitiveManifest{
			{PrimitiveType: "Atom", Name: "test", Layer: "L1", InputType: "int", OutputType: "int"},
		},
		Attempt: 1,
	}

	result, err := g.Validate(context.Background(), sub)
	if err != nil {
		t.Fatalf("Validate() unexpected error: %v", err)
	}

	if !result.HasErrors() {
		t.Error("expected parse_error for empty code")
	}
}

func TestStaticGuardPort_AllStandardLibImports(t *testing.T) {
	g := newTestStaticGuard()

	sub := AgentCodeSubmission{
		AgentID: "agent-test",
		TaskID:  "task-001",
		SourceCode: `package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"go-core"
	"math"
	"strings"
	"sync"
	"time"
)

type myAtom struct{}
func (a *myAtom) Run(ctx context.Context, input int) (int, error) {
	return input, nil
}

func main() {}`,
		Manifest: []PrimitiveManifest{
			{PrimitiveType: "Atom", Name: "myAtom", Layer: "L1", InputType: "int", OutputType: "int"},
		},
		Attempt: 1,
	}

	result, err := g.Validate(context.Background(), sub)
	if err != nil {
		t.Fatalf("Validate() unexpected error: %v", err)
	}

	if result.HasErrors() {
		t.Errorf("standard library imports should be allowed, got: %v", result.Violations)
	}
}

func TestStaticGuardPort_ContextCancellation(t *testing.T) {
	g := newTestStaticGuard()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	sub := AgentCodeSubmission{
		AgentID: "agent-test",
		TaskID:  "task-001",
		SourceCode: `package main
func main() {}`,
		Manifest: []PrimitiveManifest{
			{PrimitiveType: "Atom", Name: "test", Layer: "L1", InputType: "int", OutputType: "int"},
		},
		Attempt: 1,
	}

	_, err := g.Validate(ctx, sub)
	if err == nil {
		t.Error("expected error for cancelled context")
	}
}

func TestStaticGuardPortAsStep(t *testing.T) {
	g := newTestStaticGuard()
	step := StaticGuardPortAsStep(g)

	sub := AgentCodeSubmission{
		AgentID: "agent-test",
		TaskID:  "task-001",
		SourceCode: `package main
type myAtom struct{}
func (a *myAtom) Run(ctx context.Context, input int) (int, error) { return input, nil }
func main() {}`,
		Manifest: []PrimitiveManifest{
			{PrimitiveType: "Atom", Name: "myAtom", Layer: "L1", InputType: "int", OutputType: "int"},
		},
		Attempt: 1,
	}

	result, err := step.Execute(context.Background(), sub)
	if err != nil {
		t.Fatalf("step.Execute() unexpected error: %v", err)
	}

	if result.HasErrors() {
		t.Errorf("expected no errors, got: %v", result.Violations)
	}
}
