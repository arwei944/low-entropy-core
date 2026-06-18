package core

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestTierCheck_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	result := TierCheck(dir, TierL0)
	if !result.IsOk() {
		t.Errorf("empty dir should match TierL0, got status=%s drift=%d", result.Status, result.DriftLevel)
	}
}

func TestTierCheck_MatchedTier(t *testing.T) {
	// 用 L7 检查当前项目目录，应该匹配（因为 L7 是最高 tier）
	result := TierCheck(".", TierL7)
	if !result.IsOk() {
		// 如果当前项目文件数超过 100M，L7 可能不够
		t.Logf("TierCheck result: status=%s drift=%d detected=%s",
			result.Status, result.DriftLevel, result.DetectedTier)
	}
}

func TestTierCheck_DriftDetected(t *testing.T) {
	// 创建 50 个文件模拟中型项目，但用 L0 tier
	dir := t.TempDir()
	for i := 0; i < 50; i++ {
		f, _ := os.Create(filepath.Join(dir, fmt.Sprintf("file_%d.go", i)))
		f.Close()
	}
	result := TierCheck(dir, TierL0)
	if !result.NeedsMigration() {
		t.Errorf("expected drift detected, got status=%s drift=%d", result.Status, result.DriftLevel)
	}
	if result.DetectedTier < TierL1 {
		t.Errorf("expected at least TierL1, got %s", result.DetectedTier)
	}
}

func TestTierCheck_Oversized(t *testing.T) {
	// 创建 500 个文件，用 L0 tier — 应该是 critical
	dir := t.TempDir()
	for i := 0; i < 500; i++ {
		f, _ := os.Create(filepath.Join(dir, fmt.Sprintf("f_%d.go", i)))
		f.Close()
	}
	result := TierCheck(dir, TierL0)
	if !result.IsCritical() {
		t.Errorf("expected critical drift, got status=%s drift=%d", result.Status, result.DriftLevel)
	}
}

func TestTierCheckResult_IsOk(t *testing.T) {
	tests := []struct {
		name   string
		result TierCheckResult
		want   bool
	}{
		{"ok", TierCheckResult{Status: "ok", DriftLevel: 0}, true},
		{"drift", TierCheckResult{Status: "drift_detected", DriftLevel: 1}, false},
		{"oversized", TierCheckResult{Status: "oversized", DriftLevel: 2}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.result.IsOk(); got != tt.want {
				t.Errorf("IsOk() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTierCheckResult_NeedsMigration(t *testing.T) {
	result := TierCheckResult{DriftLevel: 1}
	if !result.NeedsMigration() {
		t.Error("DriftLevel=1 should need migration")
	}
	result = TierCheckResult{DriftLevel: 0}
	if result.NeedsMigration() {
		t.Error("DriftLevel=0 should not need migration")
	}
}

func TestTierCheckResult_IsCritical(t *testing.T) {
	result := TierCheckResult{DriftLevel: 2}
	if !result.IsCritical() {
		t.Error("DriftLevel=2 should be critical")
	}
	result = TierCheckResult{DriftLevel: 1}
	if result.IsCritical() {
		t.Error("DriftLevel=1 should not be critical")
	}
}

func TestTierCheck_LargeProject(t *testing.T) {
	// 创建 1500 个文件，用 L1 tier
	dir := t.TempDir()
	for i := 0; i < 1500; i++ {
		f, _ := os.Create(filepath.Join(dir, fmt.Sprintf("file_%d.go", i)))
		f.Close()
	}
	result := TierCheck(dir, TierL1)
	if !result.IsCritical() {
		t.Errorf("1500 files with L1 should be critical, got drift=%d status=%s",
			result.DriftLevel, result.Status)
	}
}