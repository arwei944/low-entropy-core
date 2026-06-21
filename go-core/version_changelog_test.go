package core

import (
	"strings"
	"testing"
	"time"
)

// ============================================================================
// TestChangelog: Changelog 生成
// ============================================================================

func TestGenerateChangelog(t *testing.T) {
	commits := []ConventionalCommit{
		{Type: "feat", Scope: "version", Description: "add version management module"},
		{Type: "feat", Scope: "api", Description: "add commit analyze endpoint"},
		{Type: "fix", Scope: "auth", Description: "fix login bug"},
		{Type: "docs", Description: "update changelog"},
	}

	version := Semver{Major: 0, Minor: 8, Patch: 0}
	date := time.Date(2026, 6, 19, 0, 0, 0, 0, time.UTC)

	changelog := GenerateChangelog(version, commits, date)

	if changelog == "" {
		t.Error("Changelog should not be empty")
	}
	if !strings.Contains(changelog, "## [0.8.0]") {
		t.Error("Changelog should contain version header")
	}
	if !strings.Contains(changelog, "### Added") {
		t.Error("Changelog should contain Added section")
	}
	if !strings.Contains(changelog, "### Fixed") {
		t.Error("Changelog should contain Fixed section")
	}
}

func TestGenerateChangelog_Empty(t *testing.T) {
	version := Semver{Major: 0, Minor: 8, Patch: 0}
	date := time.Now()
	changelog := GenerateChangelog(version, []ConventionalCommit{}, date)
	if !strings.Contains(changelog, "## [0.8.0]") {
		t.Error("Changelog should still have version header")
	}
}

func TestGroupByType(t *testing.T) {
	commits := []ConventionalCommit{
		{Type: "feat"},
		{Type: "feat"},
		{Type: "fix"},
	}
	grouped := GroupByType(commits)
	if len(grouped["Added"]) != 2 {
		t.Errorf("Expected 2 Added, got %d", len(grouped["Added"]))
	}
	if len(grouped["Fixed"]) != 1 {
		t.Errorf("Expected 1 Fixed, got %d", len(grouped["Fixed"]))
	}
}

func TestPrependChangelog(t *testing.T) {
	existing := "# Changelog\n\n## [0.7.0] - 2026-06-19\n\n### Added\n- UA integration"
	newEntry := "## [0.8.0] - 2026-06-19\n\n### Added\n- Version management"

	result := PrependChangelog(existing, newEntry)
	if !strings.Contains(result, "0.8.0") {
		t.Error("Result should contain 0.8.0")
	}
	if !strings.Contains(result, "0.7.0") {
		t.Error("Result should still contain 0.7.0")
	}
}
