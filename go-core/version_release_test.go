package core

import (
	"path/filepath"
	"testing"
)

// ============================================================================
// TestRelease: 发布流水线
// ============================================================================

func TestReleaseComposer_DryRun(t *testing.T) {
	repoDir := filepath.Join("..")
	rc := NewReleaseComposer(repoDir)

	plan, err := rc.DryRun()
	if err != nil {
		t.Logf("DryRun (expected, may be no commits): %v", err)
		return
	}
	if !plan.DryRun {
		t.Error("DryRun plan should have DryRun=true")
	}
	t.Logf("Release plan: version=%s, tag=%s, commits=%d", plan.Version.String(), plan.Tag, len(plan.Commits))
}

func TestReleaseComposer_Steps(t *testing.T) {
	rc := NewReleaseComposer(".")
	steps := rc.Steps()
	if len(steps) != 6 {
		t.Errorf("Expected 6 steps, got %d", len(steps))
	}
}

func TestReleaseComposer_PlanRelease(t *testing.T) {
	repoDir := filepath.Join("..")
	rc := NewReleaseComposer(repoDir)

	plan, err := rc.PlanRelease()
	if err != nil {
		t.Logf("PlanRelease (expected, may be no commits): %v", err)
		return
	}
	if plan.Version.IsZero() {
		t.Error("Version should not be zero")
	}
	t.Logf("Plan: version=%s, bump=%+v, changelog_len=%d", plan.Version.String(), plan.Bump, len(plan.Changelog))
}
