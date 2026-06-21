package core

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestUnderstandAdapter_Config(t *testing.T) {
	config := DefaultUnderstandConfig("test-project")
	if config.ProjectRoot != "test-project" {
		t.Errorf("expected ProjectRoot 'test-project', got %q", config.ProjectRoot)
	}
	if config.Timeout == 0 {
		t.Error("expected non-zero timeout")
	}
}

func TestUnderstandAdapter_LoadGraph(t *testing.T) {
	tmpDir := t.TempDir()

	outputDir := filepath.Join(tmpDir, ".understand-anything")
	os.MkdirAll(outputDir, 0755)

	kg := makeTestGraph(5)
	data, _ := json.Marshal(kg)
	os.WriteFile(filepath.Join(outputDir, "knowledge-graph.json"), data, 0644)

	config := UnderstandConfig{
		ProjectRoot: tmpDir,
		OutputDir:   outputDir,
		Timeout:     10 * time.Second,
	}
	adapter := NewUnderstandAdapter(config)

	loaded, err := adapter.LoadGraph(context.Background())
	if err != nil {
		t.Fatalf("LoadGraph: %v", err)
	}
	if loaded.Version != kg.Version {
		t.Errorf("version mismatch: got %q, want %q", loaded.Version, kg.Version)
	}
}

func TestUnderstandAdapter_HasGraph(t *testing.T) {
	tmpDir := t.TempDir()

	outputDir := filepath.Join(tmpDir, ".understand-anything")
	os.MkdirAll(outputDir, 0755)

	config := UnderstandConfig{
		ProjectRoot: tmpDir,
		OutputDir:   outputDir,
	}
	adapter := NewUnderstandAdapter(config)

	if adapter.HasGraph() {
		t.Error("expected HasGraph=false for empty directory")
	}

	kg := makeTestGraph(3)
	data, _ := json.Marshal(kg)
	os.WriteFile(filepath.Join(outputDir, "knowledge-graph.json"), data, 0644)

	if !adapter.HasGraph() {
		t.Error("expected HasGraph=true after creating graph file")
	}
}

func TestMigrationAdapter_CreateBaseline(t *testing.T) {
	tmpDir := t.TempDir()

	outputDir := filepath.Join(tmpDir, ".understand-anything")
	os.MkdirAll(outputDir, 0755)

	kg := makeTestGraph(10)
	data, _ := json.Marshal(kg)
	os.WriteFile(filepath.Join(outputDir, "knowledge-graph.json"), data, 0644)

	uaAdapter := NewUnderstandAdapter(UnderstandConfig{
		ProjectRoot: tmpDir,
		OutputDir:   outputDir,
	})
	migrationAdapter := NewUnderstandMigrationAdapter(uaAdapter)

	baseline, err := migrationAdapter.CreateBaseline(context.Background(), "v1.0.0")
	if err != nil {
		t.Fatalf("CreateBaseline: %v", err)
	}
	if baseline.Version != "v1.0.0" {
		t.Errorf("version: got %q, want 'v1.0.0'", baseline.Version)
	}
	if baseline.NodeCount != len(kg.Nodes) {
		t.Errorf("node_count: got %d, want %d", baseline.NodeCount, len(kg.Nodes))
	}
}

func TestMigrationAdapter_SaveAndLoadBaseline(t *testing.T) {
	tmpDir := t.TempDir()

	outputDir := filepath.Join(tmpDir, ".understand-anything")
	os.MkdirAll(outputDir, 0755)

	kg := makeTestGraph(5)
	data, _ := json.Marshal(kg)
	os.WriteFile(filepath.Join(outputDir, "knowledge-graph.json"), data, 0644)

	uaAdapter := NewUnderstandAdapter(UnderstandConfig{
		ProjectRoot: tmpDir,
		OutputDir:   outputDir,
	})
	migrationAdapter := NewUnderstandMigrationAdapter(uaAdapter)

	_, err := migrationAdapter.CreateBaseline(context.Background(), "v1.0.0")
	if err != nil {
		t.Fatalf("CreateBaseline: %v", err)
	}

	err = migrationAdapter.SaveBaseline("v1.0.0")
	if err != nil {
		t.Fatalf("SaveBaseline: %v", err)
	}

	baselinePath := filepath.Join(outputDir, "baselines", "v1.0.0.json")
	if _, err := os.Stat(baselinePath); os.IsNotExist(err) {
		t.Error("baseline file not created")
	}

	migrationAdapter2 := NewUnderstandMigrationAdapter(uaAdapter)
	report, err := migrationAdapter2.CompareWithBaseline(context.Background(), "v1.0.0")
	if err != nil {
		t.Fatalf("CompareWithBaseline: %v", err)
	}
	if !report.Success {
		t.Error("expected successful comparison")
	}
}

func TestNewMigrationStep(t *testing.T) {
	tmpDir := t.TempDir()

	outputDir := filepath.Join(tmpDir, ".understand-anything")
	os.MkdirAll(outputDir, 0755)

	kg := makeTestGraph(5)
	data, _ := json.Marshal(kg)
	os.WriteFile(filepath.Join(outputDir, "knowledge-graph.json"), data, 0644)

	uaAdapter := NewUnderstandAdapter(UnderstandConfig{
		ProjectRoot: tmpDir,
		OutputDir:   outputDir,
	})
	migrationAdapter := NewUnderstandMigrationAdapter(uaAdapter)
	step := NewMigrationStep(migrationAdapter)

	ctx := context.Background()
	report, err := step.Execute(ctx, MigrationRequest{
		SourceVersion: "v1.0.0",
		TargetVersion: "v1.1.0",
		Description:   "Test migration",
	})
	if err != nil {
		t.Fatalf("MigrationStep: %v", err)
	}
	if !report.Success {
		t.Error("expected successful migration")
	}
}
