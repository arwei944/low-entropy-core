//go:build lecore_tier3 || lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

package core

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMigrateAdopt_NilPlan(t *testing.T) {
	err := MigrateAdopt(nil)
	if err == nil {
		t.Error("nil plan should return error")
	}
}

func TestMigrateAdopt_L1toL2(t *testing.T) {
	plan := MigrateAnalyze(".", TierL1, TierL2)
	err := MigrateAdopt(plan)
	if err != nil {
		t.Fatalf("MigrateAdopt failed: %v", err)
	}

	// 所有模块应该标记为 done
	for _, m := range plan.Modules {
		if m.Status != ModuleDone {
			t.Errorf("module %s status should be done, got %s", m.ModuleName, m.Status)
		}
	}
}

func TestMigrateAdoptWithOutput(t *testing.T) {
	dir := t.TempDir()
	plan := MigrateAnalyze(".", TierL1, TierL2)

	err := MigrateAdoptWithOutput(plan, dir)
	if err != nil {
		t.Fatalf("MigrateAdoptWithOutput failed: %v", err)
	}

	// 检查是否有文件生成
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	if len(entries) == 0 && plan.ModuleCount() > 0 {
		t.Log("no bridge files generated (expected for modules without bridges)")
	}
}

func TestMigrateAdoptSingle(t *testing.T) {
	code, err := MigrateAdoptSingle(TierL1, TierL2, "eventstore")
	if err != nil {
		t.Fatalf("MigrateAdoptSingle failed: %v", err)
	}
	if code == "" {
		t.Error("bridge code should not be empty")
	}
	if len(code) < 100 {
		t.Errorf("bridge code too short: %d chars", len(code))
	}
}

func TestMigrateAdoptSingle_NoBridge(t *testing.T) {
	_, err := MigrateAdoptSingle(TierL1, TierL2, "nonexistent")
	if err == nil {
		t.Error("nonexistent module should return error")
	}
}

func TestGenerateTierBridge(t *testing.T) {
	code := GenerateTierBridge(TierL1, TierL2, "eventstore")
	if code == "" {
		t.Error("bridge code should not be empty for eventstore")
	}
	if code[:2] == "//" && code[:8] == "// No st" {
		t.Error("should not return 'no standard bridge' for eventstore")
	}
}

func TestGenerateTierBridge_UnknownModule(t *testing.T) {
	code := GenerateTierBridge(TierL1, TierL2, "unknown_module")
	if code == "" {
		t.Error("should return a message, not empty")
	}
}

func TestListAvailableBridges(t *testing.T) {
	bridges := ListAvailableBridges(TierL1, TierL2)
	if len(bridges) == 0 {
		t.Error("should have bridges from L1 to L2")
	}
	for _, b := range bridges {
		if b.AdapterCode == "" {
			t.Errorf("bridge %s has empty adapter code", b.Name)
		}
	}
}

func TestTierBridge_Fields(t *testing.T) {
	bridges := ListAvailableBridges(TierL1, TierL3)
	for _, b := range bridges {
		if b.Name == "" {
			t.Error("bridge name should not be empty")
		}
		if b.Description == "" {
			t.Errorf("bridge %s description should not be empty", b.Name)
		}
	}
}

func TestMigrateAdoptWithOutput_NonexistentDir(t *testing.T) {
	plan := MigrateAnalyze(".", TierL1, TierL2)
	dir := filepath.Join(t.TempDir(), "bridges", "sub")

	err := MigrateAdoptWithOutput(plan, dir)
	if err != nil {
		t.Fatalf("MigrateAdoptWithOutput should create dirs: %v", err)
	}
}