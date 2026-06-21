//go:build lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

package core

import (
	"context"
	"testing"
)

func newTestStaticGuard() *StaticGuardPort {
	return NewStaticGuardPort(nil, nil, nil)
}

func TestStaticGuardPort_CompliantCode(t *testing.T) {
	g := newTestStaticGuard()

	sub := AgentCodeSubmission{
		AgentID: "agent-test",
		TaskID:  "task-001",
		SourceCode: `package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"go-core"
)

type validateRegisterPort struct{}
func (p *validateRegisterPort) Run(ctx context.Context, req RegisterReq) (RegisterReq, error) {
	if len(req.Username) < 3 {
		return req, core.NewStepError("INVALID", "too short", false)
	}
	return req, nil
}

type hashPasswordAtom struct{}
func (a *hashPasswordAtom) Run(ctx context.Context, req RegisterReq) (User, error) {
	h := sha256.Sum256([]byte(req.Password))
	return User{Username: req.Username, PasswordHash: hex.EncodeToString(h[:])}, nil
}

type saveUserAdapter struct{
	connString string
}
func (a *saveUserAdapter) Execute(ctx context.Context, user User) error {
	_ = a.connString
	return nil
}

type RegisterReq struct {
	Username string
	Password string
}

type User struct {
	Username     string
	PasswordHash string
}

func main() {}`,
		Manifest: []PrimitiveManifest{
			{PrimitiveType: "Port",    Name: "validateRegisterPort", Layer: "L1", InputType: "RegisterReq", OutputType: "RegisterReq"},
			{PrimitiveType: "Atom",    Name: "hashPasswordAtom",     Layer: "L1", InputType: "RegisterReq", OutputType: "User"},
			{PrimitiveType: "Adapter", Name: "saveUserAdapter",      Layer: "L7", InputType: "User",        OutputType: "error"},
		},
		Attempt: 1,
	}

	result, err := g.Validate(context.Background(), sub)
	if err != nil {
		t.Fatalf("Validate() unexpected error: %v", err)
	}

	if result.HasErrors() {
		t.Errorf("expected no errors but got: %v", result.Violations)
	}
}

func TestStaticGuardPort_ExternalDependency(t *testing.T) {
	g := newTestStaticGuard()

	sub := AgentCodeSubmission{
		AgentID: "agent-bad",
		TaskID:  "task-001",
		SourceCode: `package main

import (
	"go-core"
	"github.com/gin-gonic/gin"
)

type myHandler struct{}
func (h *myHandler) Run(ctx context.Context, input int) (int, error) {
	return input, nil
}

func main() {}`,
		Manifest: []PrimitiveManifest{
			{PrimitiveType: "Atom", Name: "myHandler", Layer: "L1", InputType: "int", OutputType: "int"},
		},
		Attempt: 1,
	}

	result, err := g.Validate(context.Background(), sub)
	if err != nil {
		t.Fatalf("Validate() unexpected error: %v", err)
	}

	if !result.HasErrors() {
		t.Error("expected errors for external dependency, got none")
	}

	hasExternalDepViolation := false
	for _, v := range result.Violations {
		if v.Rule == "external_dependency" {
			hasExternalDepViolation = true
			if v.Suggestion == "" {
				t.Error("external_dependency violation should have suggestion")
			}
		}
	}
	if !hasExternalDepViolation {
		t.Error("expected external_dependency violation")
	}
}

func TestStaticGuardPort_InvalidLayer(t *testing.T) {
	g := newTestStaticGuard()

	sub := AgentCodeSubmission{
		AgentID: "agent-bad",
		TaskID:  "task-001",
		SourceCode: `package main

type myAtom struct{}
func (a *myAtom) Run(ctx context.Context, input int) (int, error) {
	return input, nil
}

func main() {}`,
		Manifest: []PrimitiveManifest{
			{PrimitiveType: "Atom", Name: "myAtom", Layer: "L99", InputType: "int", OutputType: "int"},
		},
		Attempt: 1,
	}

	result, err := g.Validate(context.Background(), sub)
	if err != nil {
		t.Fatalf("Validate() unexpected error: %v", err)
	}

	if !result.HasErrors() {
		t.Error("expected errors for invalid layer, got none")
	}

	hasInvalidLayerViolation := false
	for _, v := range result.Violations {
		if v.Rule == "invalid_layer" {
			hasInvalidLayerViolation = true
		}
	}
	if !hasInvalidLayerViolation {
		t.Error("expected invalid_layer violation")
	}
}

func TestStaticGuardPort_LayerMismatch(t *testing.T) {
	g := newTestStaticGuard()

	sub := AgentCodeSubmission{
		AgentID: "agent-bad",
		TaskID:  "task-001",
		SourceCode: `package main

type myAdapter struct{}
func (a *myAdapter) Execute(ctx context.Context, input int) (int, error) {
	return input, nil
}

func main() {}`,
		Manifest: []PrimitiveManifest{
			{PrimitiveType: "Adapter", Name: "myAdapter", Layer: "L1", InputType: "int", OutputType: "int"},
		},
		Attempt: 1,
	}

	result, err := g.Validate(context.Background(), sub)
	if err != nil {
		t.Fatalf("Validate() unexpected error: %v", err)
	}

	if !result.HasErrors() {
		t.Error("expected errors for layer mismatch, got none")
	}

	hasLayerMismatchViolation := false
	for _, v := range result.Violations {
		if v.Rule == "layer_mismatch" {
			hasLayerMismatchViolation = true
			if v.Suggestion == "" {
				t.Error("layer_mismatch violation should have suggestion")
			}
		}
	}
	if !hasLayerMismatchViolation {
		t.Error("expected layer_mismatch violation")
	}
}

func TestStaticGuardPort_PortInCorrectLayer(t *testing.T) {
	g := newTestStaticGuard()

	sub := AgentCodeSubmission{
		AgentID: "agent-test",
		TaskID:  "task-001",
		SourceCode: `package main

type myPort struct{}
func (p *myPort) Run(ctx context.Context, input int) (int, error) {
	return input, nil
}

func main() {}`,
		Manifest: []PrimitiveManifest{
			{PrimitiveType: "Port", Name: "myPort", Layer: "L1", InputType: "int", OutputType: "int"},
		},
		Attempt: 1,
	}

	result, err := g.Validate(context.Background(), sub)
	if err != nil {
		t.Fatalf("Validate() unexpected error: %v", err)
	}

	if result.HasErrors() {
		t.Errorf("expected no errors for Port in L1, got: %v", result.Violations)
	}

	sub2 := AgentCodeSubmission{
		AgentID: "agent-bad",
		TaskID:  "task-002",
		SourceCode: `package main

type myPort struct{}
func (p *myPort) Run(ctx context.Context, input int) (int, error) {
	return input, nil
}

func main() {}`,
		Manifest: []PrimitiveManifest{
			{PrimitiveType: "Port", Name: "myPort", Layer: "L7", InputType: "int", OutputType: "int"},
		},
		Attempt: 1,
	}

	result2, err := g.Validate(context.Background(), sub2)
	if err != nil {
		t.Fatalf("Validate() unexpected error: %v", err)
	}

	if !result2.HasErrors() {
		t.Error("expected errors for Port in L7, got none")
	}
}
