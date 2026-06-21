package migrate

import (
	"encoding/json"
	"testing"
)

func TestUnifiedFile_JSONRoundTrip(t *testing.T) {
	original := &UnifiedFile{
		Path:     "test.go",
		Language: "go",
		Functions: []UnifiedFunction{
			{
				Name:      "Add",
				Signature: "func Add(a, b int) int",
				Parameters: []UnifiedParam{
					{Name: "a", Type: "int"},
					{Name: "b", Type: "int"},
				},
				ReturnTypes: []string{"int"},
				BodyNodes: []BodyNode{
					{Type: BodyNodeReturn, Text: "return a + b", Line: 2},
				},
				CallGraph:  []string{},
				File:       "test.go",
				Line:       1,
				IsExported: true,
			},
		},
		Imports: []UnifiedImport{
			{Path: "fmt", IsStdLib: true},
		},
		RawLines: 5,
	}

	data, err := original.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON failed: %v", err)
	}

	loaded, err := UnifiedFileFromJSON(data)
	if err != nil {
		t.Fatalf("FromJSON failed: %v", err)
	}

	if loaded.Path != original.Path {
		t.Errorf("Path mismatch: got %q, want %q", loaded.Path, original.Path)
	}
	if len(loaded.Functions) != 1 {
		t.Fatalf("Functions count: got %d, want 1", len(loaded.Functions))
	}
	if loaded.Functions[0].Name != "Add" {
		t.Errorf("Function name: got %q, want %q", loaded.Functions[0].Name, "Add")
	}
	if len(loaded.Functions[0].Parameters) != 2 {
		t.Errorf("Parameters count: got %d, want 2", len(loaded.Functions[0].Parameters))
	}
	if loaded.Functions[0].Parameters[0].Name != "a" {
		t.Errorf("First param name: got %q, want %q", loaded.Functions[0].Parameters[0].Name, "a")
	}
}

func TestBodyNodeType_Values(t *testing.T) {
	types := []BodyNodeType{
		BodyNodeIf, BodyNodeLoop, BodyNodeCall, BodyNodeAssign,
		BodyNodeReturn, BodyNodeDefer, BodyNodePanic, BodyNodeSend,
		BodyNodeSelect, BodyNodeGo, BodyNodeIO, BodyNodeOther,
	}
	if len(types) != 12 {
		t.Errorf("BodyNodeType count: got %d, want 12", len(types))
	}
}

func TestUnifiedFile_ToJSON_IsValidJSON(t *testing.T) {
	f := &UnifiedFile{Path: "empty.py", Language: "python"}
	data, err := f.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON failed: %v", err)
	}
	if !json.Valid(data) {
		t.Error("ToJSON output is not valid JSON")
	}
}

func TestUnifiedFile_Empty(t *testing.T) {
	f := &UnifiedFile{}
	if f.Path != "" {
		t.Errorf("Empty UnifiedFile should have empty Path")
	}
}
