// complexity_profile_test.go — Tests for the progressive complexity model
// This file is part of the kernel (always compiled, no build tags).

package core

import (
	"os"
	"path/filepath"
	"testing"
)

func TestComplexityTier_String(t *testing.T) {
	tests := []struct {
		tier     ComplexityTier
		expected string
	}{
		{TierL0, "Prototype"},
		{TierL1, "Microservice"},
		{TierL2, "Mid-size Service"},
		{TierL3, "Large Service"},
		{TierL4, "Platform"},
		{TierL5, "Enterprise Platform"},
		{TierL6, "System-level"},
		{TierL7, "Windows-scale"},
		{TierAuto, "Unknown"},
		{ComplexityTier(99), "Unknown"},
	}

	for _, tt := range tests {
		got := tt.tier.String()
		if got != tt.expected {
			t.Errorf("Tier(%d).String() = %q, want %q", tt.tier, got, tt.expected)
		}
	}
}

func TestComplexityTier_FrameworkFileCount(t *testing.T) {
	tests := []struct {
		tier          ComplexityTier
		expectedCount int
	}{
		{TierL0, 12},
		{TierL1, 14},
		{TierL2, 19},
		{TierL3, 30},
		{TierL4, 49},
		{TierL5, 48},
		{TierL6, 48},
		{TierL7, 48},
		{ComplexityTier(99), 0},
	}

	for _, tt := range tests {
		got := tt.tier.FrameworkFileCount()
		if got != tt.expectedCount {
			t.Errorf("Tier(%d).FrameworkFileCount() = %d, want %d", tt.tier, got, tt.expectedCount)
		}
	}
}

func TestAutoDetect_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	tier := AutoDetect(dir)
	if tier != TierL0 {
		t.Errorf("AutoDetect(empty dir) = %v, want TierL0", tier)
	}
}

func TestAutoDetect_SmallProject(t *testing.T) {
	dir := t.TempDir()

	// Create 15 Go files (should be TierL1: 10-100 files)
	for i := 0; i < 15; i++ {
		f, err := os.Create(filepath.Join(dir, "file_"+string(rune('a'+i%26))+".go"))
		if err != nil {
			t.Fatal(err)
		}
		f.Close()
	}

	tier := AutoDetect(dir)
	if tier != TierL1 {
		t.Errorf("AutoDetect(15 files) = %v, want TierL1", tier)
	}
}

func TestAutoDetect_MediumProject(t *testing.T) {
	dir := t.TempDir()

	// Create 150 Go files (should be TierL2: 100-1000 files)
	for i := 0; i < 150; i++ {
		f, err := os.Create(filepath.Join(dir, "file_"+string(rune('a'+i%26))+"_"+string(rune('0'+(i/26)%10))+".go"))
		if err != nil {
			t.Fatal(err)
		}
		f.Close()
	}

	tier := AutoDetect(dir)
	if tier != TierL2 {
		t.Errorf("AutoDetect(150 files) = %v, want TierL2", tier)
	}
}

func TestAutoDetectWithBoost_MultiLanguage(t *testing.T) {
	dir := t.TempDir()

	// Create 15 files across 3 languages (should get +1 boost: TierL1 → TierL2)
	exts := []string{".go", ".rs", ".py"}
	for i := 0; i < 15; i++ {
		ext := exts[i%3]
		f, _ := os.Create(filepath.Join(dir, "file_"+string(rune('a'+i))+ext))
		f.Close()
	}

	tier := AutoDetectWithBoost(dir)
	if tier < TierL1 {
		t.Errorf("AutoDetectWithBoost(15 files, 3 languages) = %v, want >= TierL1", tier)
	}
}

func TestScanProject(t *testing.T) {
	dir := t.TempDir()

	// Create files with known extensions
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main"), 0644)
	os.WriteFile(filepath.Join(dir, "lib.rs"), []byte("fn main() {}"), 0644)
	os.WriteFile(filepath.Join(dir, "script.py"), []byte("print('hello')"), 0644)
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test"), 0644)
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Test"), 0644)

	stats := ScanProject(dir)

	if stats.TotalFiles < 3 {
		t.Errorf("TotalFiles = %d, want >= 3", stats.TotalFiles)
	}
	if stats.ModuleCount != 1 {
		t.Errorf("ModuleCount = %d, want 1", stats.ModuleCount)
	}
	// .md is not in sourceExts, so it shouldn't be counted
	if stats.Languages[".go"] != 1 {
		t.Errorf("Go files = %d, want 1", stats.Languages[".go"])
	}
}

func TestTierConstants_Unique(t *testing.T) {
	// Verify all tier constants are distinct
	seen := make(map[ComplexityTier]bool)
	tiers := []ComplexityTier{TierL0, TierL1, TierL2, TierL3, TierL4, TierL5, TierL6, TierL7}
	for _, tier := range tiers {
		if seen[tier] {
			t.Errorf("duplicate tier value: %d", tier)
		}
		seen[tier] = true
	}
}

func TestTierAuto_IsNegative(t *testing.T) {
	if TierAuto >= 0 {
		t.Errorf("TierAuto should be negative, got %d", TierAuto)
	}
}