package core

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// ============================================================================
// Benchmarks
// ============================================================================

func BenchmarkParseSemver(b *testing.B) {
	for i := 0; i < b.N; i++ {
		ParseSemver("v1.2.3-alpha.1+build.123")
	}
}

func BenchmarkParseCommit(b *testing.B) {
	raw := "feat(auth): add OAuth2 support\n\nDetailed description\n\nBREAKING CHANGE: old API removed"
	for i := 0; i < b.N; i++ {
		ParseCommit(raw)
	}
}

func BenchmarkGenerateChangelog(b *testing.B) {
	commits := []ConventionalCommit{
		{Type: "feat", Scope: "version", Description: "add version management"},
		{Type: "fix", Scope: "auth", Description: "fix login bug"},
		{Type: "docs", Description: "update readme"},
	}
	version := Semver{Major: 0, Minor: 8, Patch: 0}
	date := time.Now()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		GenerateChangelog(version, commits, date)
	}
}

func BenchmarkClassifyCommits(b *testing.B) {
	commits := []ConventionalCommit{
		{Type: "feat"}, {Type: "feat"}, {Type: "fix"},
		{Type: "docs"}, {Type: "refactor"}, {Type: "perf"},
		{Type: "test"}, {Type: "chore"},
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ClassifyCommits(commits)
	}
}

func BenchmarkInferNextVersion(b *testing.B) {
	commits := []ConventionalCommit{
		{Type: "feat", BreakingChange: false},
		{Type: "fix", BreakingChange: false},
	}
	current := Semver{Major: 0, Minor: 7, Patch: 0}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		InferNextVersion(commits, current)
	}
}

func BenchmarkCreateChange(b *testing.B) {
	dir := os.TempDir()
	changesDir := filepath.Join(dir, changeDirName)
	os.MkdirAll(changesDir, 0755)
	defer os.RemoveAll(changesDir)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		intent := ChangeIntent{Title: "Benchmark change", Type: "feat"}
		CreateChange(dir, intent)
	}
}

func BenchmarkCreateADR(b *testing.B) {
	dir := os.TempDir()
	adrDir := filepath.Join(dir, adrDirName)
	os.MkdirAll(adrDir, 0755)
	defer os.RemoveAll(adrDir)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		adr := ADR{Title: "Benchmark ADR", Status: ADRStatusAccepted, Version: "1.0.0"}
		CreateADR(dir, adr)
	}
}

func BenchmarkListADRs(b *testing.B) {
	dir := os.TempDir()
	for i := 0; i < 10; i++ {
		adr := ADR{Title: "ADR", Status: ADRStatusAccepted, Version: "1.0.0"}
		CreateADR(dir, adr)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ListADRs(dir)
	}
}
