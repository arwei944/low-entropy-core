package core

import (
	"path/filepath"
	"testing"
)

// ============================================================================
// TestGit: Git 操作
// ============================================================================

func TestGitTags(t *testing.T) {
	repoDir := filepath.Join("..")
	tags, err := GitTags(repoDir)
	if err != nil {
		t.Skipf("Git not available or no tags: %v", err)
	}
	if len(tags) == 0 {
		t.Skip("No tags found")
	}
	t.Logf("Found %d tags: %v", len(tags), tags)
}

func TestGitLastTag(t *testing.T) {
	repoDir := filepath.Join("..")
	tag, err := GitLastTag(repoDir)
	if err != nil {
		t.Skipf("Git not available: %v", err)
	}
	t.Logf("Last tag: %q", tag)
}

func TestGitCurrentBranch(t *testing.T) {
	repoDir := filepath.Join("..")
	branch, err := GitCurrentBranch(repoDir)
	if err != nil {
		t.Skipf("Git not available: %v", err)
	}
	if branch == "" {
		t.Error("Branch should not be empty")
	}
	t.Logf("Current branch: %s", branch)
}

func TestGitIsClean(t *testing.T) {
	repoDir := filepath.Join("..")
	clean, err := GitIsClean(repoDir)
	if err != nil {
		t.Skipf("Git not available: %v", err)
	}
	t.Logf("Working tree clean: %v", clean)
}

func TestGitLog(t *testing.T) {
	repoDir := filepath.Join("..")
	log, err := GitLog(repoDir, GitLogOptions{MaxCount: 5, SkipMerges: true})
	if err != nil {
		t.Skipf("Git not available: %v", err)
	}
	if log == "" {
		t.Error("Git log should not be empty")
	}
	t.Logf("Git log (first 100 chars): %.100s", log)
}

func TestGitCommitsSinceTag(t *testing.T) {
	repoDir := filepath.Join("..")
	tag, _ := GitLastTag(repoDir)
	log, err := GitCommitsSinceTag(repoDir, tag)
	if err != nil {
		t.Skipf("Git not available: %v", err)
	}
	t.Logf("Commits since %s: %d lines", tag, len(log))
}
