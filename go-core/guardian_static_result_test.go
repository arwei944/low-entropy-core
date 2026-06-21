//go:build lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

package core

import "testing"

func TestStaticReviewResult_HasErrors(t *testing.T) {
	tests := []struct {
		name     string
		result   StaticReviewResult
		expected bool
	}{
		{"no violations", StaticReviewResult{Violations: nil}, false},
		{"only warnings", StaticReviewResult{Violations: []Violation{
			{Rule: "test", Severity: "warn", Detail: "test"},
		}}, false},
		{"has errors", StaticReviewResult{Violations: []Violation{
			{Rule: "test", Severity: "error", Detail: "test"},
		}}, true},
		{"mixed", StaticReviewResult{Violations: []Violation{
			{Rule: "warn1", Severity: "warn", Detail: "test"},
			{Rule: "err1", Severity: "error", Detail: "test"},
		}}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.result.HasErrors(); got != tt.expected {
				t.Errorf("HasErrors() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestStaticReviewResult_ErrorCount(t *testing.T) {
	tests := []struct {
		name     string
		result   StaticReviewResult
		expected int
	}{
		{"no violations", StaticReviewResult{Violations: nil}, 0},
		{"only warnings", StaticReviewResult{Violations: []Violation{
			{Rule: "test", Severity: "warn", Detail: "test"},
		}}, 0},
		{"one error", StaticReviewResult{Violations: []Violation{
			{Rule: "test", Severity: "error", Detail: "test"},
		}}, 1},
		{"mixed", StaticReviewResult{Violations: []Violation{
			{Rule: "warn1", Severity: "warn", Detail: "test"},
			{Rule: "err1", Severity: "error", Detail: "test"},
			{Rule: "err2", Severity: "error", Detail: "test"},
		}}, 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.result.ErrorCount(); got != tt.expected {
				t.Errorf("ErrorCount() = %d, want %d", got, tt.expected)
			}
		})
	}
}

func TestAllowedImportPaths(t *testing.T) {
	allowed := []string{
		"go-core", "fmt", "context", "sync", "time", "strings",
		"crypto/sha256", "encoding/json", "database/sql",
		"net/http", "os", "math", "errors",
	}
	for _, path := range allowed {
		if !isAllowedImport(path) {
			t.Errorf("expected %s to be allowed", path)
		}
	}

	disallowed := []string{
		"github.com/gin-gonic/gin",
		"github.com/gorilla/mux",
		"google.golang.org/grpc",
		"gopkg.in/yaml.v3",
	}
	for _, path := range disallowed {
		if isAllowedImport(path) {
			t.Errorf("expected %s to be disallowed", path)
		}
	}
}

func TestLayerOrder(t *testing.T) {
	tests := []struct {
		layer string
		want  int
	}{
		{"L1", 1}, {"L2", 2}, {"L3", 3}, {"L4", 4},
		{"L5", 5}, {"L6", 6}, {"L7", 7},
		{"unknown", 0},
	}
	for _, tt := range tests {
		if got := layerOrder(tt.layer); got != tt.want {
			t.Errorf("layerOrder(%s) = %d, want %d", tt.layer, got, tt.want)
		}
	}
}

func TestFindManifest(t *testing.T) {
	manifest := []PrimitiveManifest{
		{Name: "atom1", PrimitiveType: "Atom", Layer: "L1"},
		{Name: "port1", PrimitiveType: "Port", Layer: "L1"},
		{Name: "adapter1", PrimitiveType: "Adapter", Layer: "L7"},
	}

	if found := findManifest(manifest, "atom1"); found == nil {
		t.Error("should find atom1")
	}
	if found := findManifest(manifest, "nonexistent"); found != nil {
		t.Error("should not find nonexistent")
	}
	if found := findManifest(manifest, "port1"); found.PrimitiveType != "Port" {
		t.Errorf("expected Port, got %s", found.PrimitiveType)
	}
}
