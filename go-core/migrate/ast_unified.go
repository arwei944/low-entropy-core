package migrate

import "encoding/json"

// UnifiedFile represents a language-agnostic AST for a source file.
type UnifiedFile struct {
	Path       string            `json:"path"`
	Language   string            `json:"language"` // "go" | "python" | "java" | "typescript" | "rust"
	Functions  []UnifiedFunction `json:"functions"`
	Classes    []UnifiedClass    `json:"classes"`
	Interfaces []UnifiedInterface `json:"interfaces"`
	Imports    []UnifiedImport   `json:"imports"`
	Comments   []string          `json:"comments"`
	RawLines   int               `json:"raw_lines"`
}

// UnifiedFunction represents a function or method.
type UnifiedFunction struct {
	Name        string         `json:"name"`
	Signature   string         `json:"signature"`
	Parameters  []UnifiedParam `json:"parameters"`
	ReturnTypes []string       `json:"return_types"`
	BodyNodes   []BodyNode     `json:"body_nodes"`
	Annotations []string       `json:"annotations"`
	CallGraph   []string       `json:"call_graph"` // names of called functions
	File        string         `json:"file"`
	Line        int            `json:"line"`
	IsExported  bool           `json:"is_exported"`
}

// UnifiedParam represents a function parameter.
type UnifiedParam struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// UnifiedClass represents a class or struct.
type UnifiedClass struct {
	Name       string   `json:"name"`
	Methods    []string `json:"methods"`
	Fields     []string `json:"fields"`
	Implements []string `json:"implements"`
	File       string   `json:"file"`
	Line       int      `json:"line"`
}

// UnifiedInterface represents an interface.
type UnifiedInterface struct {
	Name    string   `json:"name"`
	Methods []string `json:"methods"`
	File    string   `json:"file"`
	Line    int      `json:"line"`
}

// UnifiedImport represents an import declaration.
type UnifiedImport struct {
	Path     string `json:"path"`
	Alias    string `json:"alias"`
	IsStdLib bool   `json:"is_stdlib"`
}

// BodyNodeType enumerates AST body node types.
type BodyNodeType string

const (
	BodyNodeIf     BodyNodeType = "if"
	BodyNodeLoop   BodyNodeType = "loop"
	BodyNodeCall   BodyNodeType = "call"
	BodyNodeAssign BodyNodeType = "assign"
	BodyNodeReturn BodyNodeType = "return"
	BodyNodeDefer  BodyNodeType = "defer"
	BodyNodePanic  BodyNodeType = "panic"
	BodyNodeSend   BodyNodeType = "send"   // channel send
	BodyNodeSelect BodyNodeType = "select" // channel select
	BodyNodeGo     BodyNodeType = "go"     // goroutine
	BodyNodeIO     BodyNodeType = "io"     // I/O operation (file/network/db)
	BodyNodeOther  BodyNodeType = "other"
)

// BodyNode represents a single AST node inside a function body.
type BodyNode struct {
	Type     BodyNodeType `json:"type"`
	Text     string       `json:"text"`
	Children []BodyNode   `json:"children"`
	Line     int          `json:"line"`
}

// ToJSON serializes the UnifiedFile to indented JSON.
func (f *UnifiedFile) ToJSON() ([]byte, error) {
	return json.MarshalIndent(f, "", "  ")
}

// UnifiedFileFromJSON deserializes JSON into a UnifiedFile.
func UnifiedFileFromJSON(data []byte) (*UnifiedFile, error) {
	var f UnifiedFile
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, err
	}
	return &f, nil
}
