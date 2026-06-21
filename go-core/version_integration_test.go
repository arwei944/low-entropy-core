package core

import (
	"path/filepath"
	"testing"
	"time"
)

// ============================================================================
// TestIntegration: 集成测试
// ============================================================================

func TestIntegration_FullWorkflow(t *testing.T) {
	repoDir := filepath.Join("..")

	log, err := GitCommitsSinceTag(repoDir, "")
	if err != nil {
		t.Skipf("Git not available: %v", err)
	}
	if log == "" {
		t.Skip("No commits found")
	}

	commits, err := ParseCommitsFromLog(log)
	if err != nil {
		t.Fatalf("ParseCommitsFromLog error: %v", err)
	}
	t.Logf("Parsed %d commits", len(commits))

	class := ClassifyCommits(commits)
	t.Logf("Classification: feat=%d, fix=%d, breaking=%d, total=%d", class.Feat, class.Fix, class.Breaking, class.Total)

	bump := InferBump(commits)
	t.Logf("Version bump: %+v", bump)

	current := Semver{Major: 0, Minor: 7, Patch: 0}
	lastTag, _ := GitLastTag(repoDir)
	if lastTag != "" {
		if parsed, err := ParseSemver(lastTag); err == nil {
			current = parsed
		}
	}

	next := InferNextVersion(commits, current)
	t.Logf("Next version: %s → %s", current.String(), next.String())

	changelog := GenerateChangelog(next, commits, time.Now())
	t.Logf("Changelog length: %d chars", len(changelog))
}

func TestIntegration_ChangeWorkflow(t *testing.T) {
	dir := t.TempDir()

	intents := []ChangeIntent{
		{Title: "Add version module", Type: "feat", Scope: "version", Breaking: false},
		{Title: "Fix auth bug", Type: "fix", Scope: "auth", Breaking: false},
	}

	for _, intent := range intents {
		if err := CreateChange(dir, intent); err != nil {
			t.Fatalf("CreateChange error: %v", err)
		}
	}

	changes, err := ListChanges(dir)
	if err != nil {
		t.Fatalf("ListChanges error: %v", err)
	}
	if len(changes) != 2 {
		t.Errorf("Expected 2 changes, got %d", len(changes))
	}

	ac, _ := MergeChanges(dir)
	if ac.Status != "merged" {
		t.Errorf("Status should be merged, got %s", ac.Status)
	}
}

func TestIntegration_ADRWorkflow(t *testing.T) {
	dir := t.TempDir()

	CreateADR(dir, ADR{Title: "Use SemVer", Status: ADRStatusAccepted, Version: "0.8.0"})
	CreateADR(dir, ADR{Title: "Use Conventional Commits", Status: ADRStatusProposed, Version: "0.8.0"})

	adrs, _ := ListADRs(dir)
	if len(adrs) != 2 {
		t.Errorf("Expected 2 ADRs, got %d", len(adrs))
	}

	filtered, _ := ADRByVersion(dir, Semver{Major: 0, Minor: 8, Patch: 0})
	if len(filtered) != 2 {
		t.Errorf("Expected 2 ADRs for v0.8.0, got %d", len(filtered))
	}
}
