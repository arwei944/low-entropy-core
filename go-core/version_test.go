// Package core — 通用版本管理模块 (v0.8.0)
//
// version_test.go: 全面测试

package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ============================================================================
// TestSemver: SemVer 解析与比较
// ============================================================================

func TestParseSemver_Basic(t *testing.T) {
	tests := []struct {
		input    string
		expected Semver
	}{
		{"0.7.0", Semver{Major: 0, Minor: 7, Patch: 0}},
		{"v0.7.0", Semver{Major: 0, Minor: 7, Patch: 0}},
		{"1.0.0", Semver{Major: 1, Minor: 0, Patch: 0}},
		{"v1.2.3", Semver{Major: 1, Minor: 2, Patch: 3}},
		{"0.7.0-alpha.1", Semver{Major: 0, Minor: 7, Patch: 0, Prerelease: "alpha.1"}},
		{"v1.0.0-beta.2+build.123", Semver{Major: 1, Minor: 0, Patch: 0, Prerelease: "beta.2", Build: "build.123"}},
		{"2.0.0-rc.1", Semver{Major: 2, Minor: 0, Patch: 0, Prerelease: "rc.1"}},
	}

	for _, tt := range tests {
		result, err := ParseSemver(tt.input)
		if err != nil {
			t.Errorf("ParseSemver(%q) error: %v", tt.input, err)
			continue
		}
		if result != tt.expected {
			t.Errorf("ParseSemver(%q) = %+v, want %+v", tt.input, result, tt.expected)
		}
	}
}

func TestParseSemver_Invalid(t *testing.T) {
	invalid := []string{"", "abc", "1.0", "v1", "1.0.0.0", "not-a-version"}
	for _, v := range invalid {
		_, err := ParseSemver(v)
		if err == nil {
			t.Errorf("ParseSemver(%q) should fail, but didn't", v)
		}
	}
}

func TestSemver_String(t *testing.T) {
	tests := []struct {
		sv   Semver
		want string
	}{
		{Semver{Major: 0, Minor: 7, Patch: 0}, "0.7.0"},
		{Semver{Major: 1, Minor: 2, Patch: 3, Prerelease: "alpha.1"}, "1.2.3-alpha.1"},
		{Semver{Major: 1, Minor: 0, Patch: 0, Build: "20260619"}, "1.0.0+20260619"},
	}
	for _, tt := range tests {
		got := tt.sv.String()
		if got != tt.want {
			t.Errorf("Semver.String() = %q, want %q", got, tt.want)
		}
	}
}

func TestSemver_TagName(t *testing.T) {
	sv := Semver{Major: 0, Minor: 8, Patch: 0}
	if got := sv.TagName(); got != "v0.8.0" {
		t.Errorf("TagName() = %q, want %q", got, "v0.8.0")
	}
}

func TestSemver_Compare(t *testing.T) {
	tests := []struct {
		a, b   Semver
		result int
	}{
		{Semver{Major: 1, Minor: 0, Patch: 0}, Semver{Major: 0, Minor: 9, Patch: 0}, 1},
		{Semver{Major: 0, Minor: 7, Patch: 0}, Semver{Major: 0, Minor: 8, Patch: 0}, -1},
		{Semver{Major: 0, Minor: 7, Patch: 0}, Semver{Major: 0, Minor: 7, Patch: 0}, 0},
		{Semver{Major: 0, Minor: 7, Patch: 1}, Semver{Major: 0, Minor: 7, Patch: 0}, 1},
		{Semver{Major: 1, Minor: 0, Patch: 0, Prerelease: "alpha.1"}, Semver{Major: 1, Minor: 0, Patch: 0}, -1},
		{Semver{Major: 1, Minor: 0, Patch: 0, Prerelease: "alpha.2"}, Semver{Major: 1, Minor: 0, Patch: 0, Prerelease: "alpha.1"}, 1},
		{Semver{Major: 1, Minor: 0, Patch: 0, Prerelease: "beta.1"}, Semver{Major: 1, Minor: 0, Patch: 0, Prerelease: "alpha.2"}, 1},
	}
	for _, tt := range tests {
		got := tt.a.Compare(tt.b)
		if got != tt.result {
			t.Errorf("%v.Compare(%v) = %d, want %d", tt.a, tt.b, got, tt.result)
		}
	}
}

func TestSemver_Bump(t *testing.T) {
	sv := Semver{Major: 0, Minor: 7, Patch: 0}

	if got := sv.BumpMajor(); got.String() != "1.0.0" {
		t.Errorf("BumpMajor() = %s, want 1.0.0", got.String())
	}
	if got := sv.BumpMinor(); got.String() != "0.8.0" {
		t.Errorf("BumpMinor() = %s, want 0.8.0", got.String())
	}
	if got := sv.BumpPatch(); got.String() != "0.7.1" {
		t.Errorf("BumpPatch() = %s, want 0.7.1", got.String())
	}
}

func TestSemver_BumpPrerelease(t *testing.T) {
	sv := Semver{Major: 0, Minor: 7, Patch: 0}
	got := sv.BumpPrerelease()
	if got.Prerelease != "alpha.1" {
		t.Errorf("BumpPrerelease() prerelease = %s, want alpha.1", got.Prerelease)
	}

	sv2 := Semver{Major: 0, Minor: 7, Patch: 0, Prerelease: "alpha.1"}
	got2 := sv2.BumpPrerelease()
	if got2.Prerelease != "alpha.2" {
		t.Errorf("BumpPrerelease() prerelease = %s, want alpha.2", got2.Prerelease)
	}
}

func TestIsValidSemver(t *testing.T) {
	if !IsValidSemver("0.7.0") {
		t.Error("0.7.0 should be valid")
	}
	if IsValidSemver("invalid") {
		t.Error("'invalid' should not be valid")
	}
}

func TestSortSemvers(t *testing.T) {
	versions := []Semver{
		{Major: 0, Minor: 5, Patch: 0},
		{Major: 0, Minor: 7, Patch: 0},
		{Major: 0, Minor: 6, Patch: 0},
	}
	SortSemvers(versions)
	if versions[0].String() != "0.7.0" {
		t.Errorf("First should be 0.7.0, got %s", versions[0].String())
	}
	if versions[2].String() != "0.5.0" {
		t.Errorf("Last should be 0.5.0, got %s", versions[2].String())
	}
}

func TestInferBump(t *testing.T) {
	commits := []ConventionalCommit{
		{Type: "feat", BreakingChange: false},
	}
	bump := InferBump(commits)
	if !bump.Minor || bump.Major || bump.Patch {
		t.Errorf("feat commit should infer MINOR bump, got %+v", bump)
	}

	commits2 := []ConventionalCommit{
		{Type: "feat", BreakingChange: true},
	}
	bump2 := InferBump(commits2)
	if !bump2.Major {
		t.Errorf("breaking change should infer MAJOR bump, got %+v", bump2)
	}

	commits3 := []ConventionalCommit{
		{Type: "fix", BreakingChange: false},
	}
	bump3 := InferBump(commits3)
	if !bump3.Patch || bump3.Minor || bump3.Major {
		t.Errorf("fix commit should infer PATCH bump, got %+v", bump3)
	}
}

func TestInferNextVersion(t *testing.T) {
	current := Semver{Major: 0, Minor: 7, Patch: 0}

	commits := []ConventionalCommit{{Type: "feat", BreakingChange: false}}
	next := InferNextVersion(commits, current)
	if next.String() != "0.8.0" {
		t.Errorf("feat: expected 0.8.0, got %s", next.String())
	}

	commits2 := []ConventionalCommit{{Type: "fix", BreakingChange: false}}
	next2 := InferNextVersion(commits2, current)
	if next2.String() != "0.7.1" {
		t.Errorf("fix: expected 0.7.1, got %s", next2.String())
	}

	commits3 := []ConventionalCommit{{Type: "feat", BreakingChange: true}}
	next3 := InferNextVersion(commits3, current)
	if next3.String() != "1.0.0" {
		t.Errorf("breaking: expected 1.0.0, got %s", next3.String())
	}
}

// ============================================================================
// TestCommit: Conventional Commits 解析
// ============================================================================

func TestParseCommit_Basic(t *testing.T) {
	tests := []struct {
		raw          string
		wantType     string
		wantScope    string
		wantBreaking bool
	}{
		{"feat: add login", "feat", "", false},
		{"feat(auth): add login", "feat", "auth", false},
		{"fix: resolve bug", "fix", "", false},
		{"fix(api): resolve null pointer", "fix", "api", false},
		{"docs: update readme", "docs", "", false},
		{"refactor: cleanup code", "refactor", "", false},
		{"feat!: remove old API", "feat", "", true},
		{"feat(api)!: breaking change", "feat", "api", true},
	}

	for _, tt := range tests {
		c, err := ParseCommit(tt.raw)
		if err != nil {
			t.Errorf("ParseCommit(%q) error: %v", tt.raw, err)
			continue
		}
		if c.Type != tt.wantType {
			t.Errorf("ParseCommit(%q).Type = %q, want %q", tt.raw, c.Type, tt.wantType)
		}
		if c.Scope != tt.wantScope {
			t.Errorf("ParseCommit(%q).Scope = %q, want %q", tt.raw, c.Scope, tt.wantScope)
		}
		if c.BreakingChange != tt.wantBreaking {
			t.Errorf("ParseCommit(%q).BreakingChange = %v, want %v", tt.raw, c.BreakingChange, tt.wantBreaking)
		}
	}
}

func TestParseCommit_BreakingFooter(t *testing.T) {
	raw := "feat: add new API\n\nBREAKING CHANGE: old API removed"
	c, err := ParseCommit(raw)
	if err != nil {
		t.Fatalf("ParseCommit error: %v", err)
	}
	if !c.BreakingChange {
		t.Error("Expected BreakingChange=true for BREAKING CHANGE footer")
	}
}

func TestParseCommit_MultiLine(t *testing.T) {
	raw := "feat(auth): add OAuth2 support\n\nThis adds OAuth2 authentication.\n\nCloses: #123"
	c, err := ParseCommit(raw)
	if err != nil {
		t.Fatalf("ParseCommit error: %v", err)
	}
	if c.Type != "feat" {
		t.Errorf("Type = %q, want feat", c.Type)
	}
	if c.Scope != "auth" {
		t.Errorf("Scope = %q, want auth", c.Scope)
	}
	if c.Body == "" {
		t.Error("Body should not be empty")
	}
}

func TestParseCommit_Invalid(t *testing.T) {
	invalid := []string{"", "not a commit", "random text"}
	for _, raw := range invalid {
		_, err := ParseCommit(raw)
		if err == nil {
			t.Errorf("ParseCommit(%q) should fail", raw)
		}
	}
}

func TestParseCommitsFromLog(t *testing.T) {
	log := `abc1234 feat: add login
def5678 fix: resolve bug
a1b2c3d docs: update readme`
	commits, err := ParseCommitsFromLog(log)
	if err != nil {
		t.Fatalf("ParseCommitsFromLog error: %v", err)
	}
	if len(commits) != 3 {
		t.Errorf("Expected 3 commits, got %d", len(commits))
	}
}

func TestParseCommitsFromLog_Empty(t *testing.T) {
	commits, err := ParseCommitsFromLog("")
	if err != nil {
		t.Fatalf("ParseCommitsFromLog error: %v", err)
	}
	if len(commits) != 0 {
		t.Errorf("Expected 0 commits, got %d", len(commits))
	}
}

func TestCommit_Helpers(t *testing.T) {
	c := ConventionalCommit{Type: "feat", BreakingChange: true}
	if !c.IsFeat() {
		t.Error("IsFeat should be true")
	}
	if !c.IsBreaking() {
		t.Error("IsBreaking should be true")
	}
	if c.IsFix() {
		t.Error("IsFix should be false")
	}
}

func TestClassifyCommits(t *testing.T) {
	commits := []ConventionalCommit{
		{Type: "feat"},
		{Type: "feat"},
		{Type: "fix"},
		{Type: "docs"},
	}
	class := ClassifyCommits(commits)
	if class.Total != 4 {
		t.Errorf("Total = %d, want 4", class.Total)
	}
	if class.Feat != 2 {
		t.Errorf("Feat = %d, want 2", class.Feat)
	}
	if class.Fix != 1 {
		t.Errorf("Fix = %d, want 1", class.Fix)
	}
}

func TestIsValidType(t *testing.T) {
	if !IsValidType("feat") {
		t.Error("feat should be valid")
	}
	if IsValidType("invalid") {
		t.Error("invalid should not be valid")
	}
}

func TestValidTypes(t *testing.T) {
	types := ValidTypes()
	if len(types) < 10 {
		t.Errorf("Expected at least 10 valid types, got %d", len(types))
	}
}

// ============================================================================
// TestChange: ArchChange 变更意图文件系统
// ============================================================================

func TestChange_CreateAndRead(t *testing.T) {
	dir := t.TempDir()

	intent := ChangeIntent{
		Title:       "Add version management",
		Type:        "feat",
		Scope:       "version",
		Description: "Implements universal version management.",
		Files:       []string{"go-core/version_types.go"},
		Breaking:    false,
	}

	if err := CreateChange(dir, intent); err != nil {
		t.Fatalf("CreateChange error: %v", err)
	}

	changes, err := ListChanges(dir)
	if err != nil {
		t.Fatalf("ListChanges error: %v", err)
	}

	if len(changes) != 1 {
		t.Fatalf("Expected 1 change, got %d", len(changes))
	}

	got := changes[0]
	if got.Title != intent.Title {
		t.Errorf("Title = %q, want %q", got.Title, intent.Title)
	}
	if got.Type != intent.Type {
		t.Errorf("Type = %q, want %q", got.Type, intent.Type)
	}
	if len(got.Files) != 1 || got.Files[0] != intent.Files[0] {
		t.Errorf("Files = %v, want %v", got.Files, intent.Files)
	}
}

func TestChange_Validate(t *testing.T) {
	err := ValidateChange(ChangeIntent{Title: "", Type: "feat"})
	if err == nil {
		t.Error("Should fail for empty title")
	}

	err = ValidateChange(ChangeIntent{Title: "test", Type: "invalid"})
	if err == nil {
		t.Error("Should fail for invalid type")
	}

	err = ValidateChange(ChangeIntent{Title: "test", Type: "feat"})
	if err != nil {
		t.Errorf("Should succeed: %v", err)
	}
}

func TestChange_MergeAndDelete(t *testing.T) {
	dir := t.TempDir()

	CreateChange(dir, ChangeIntent{Title: "Change 1", Type: "feat"})
	CreateChange(dir, ChangeIntent{Title: "Change 2", Type: "fix"})

	ac, err := MergeChanges(dir)
	if err != nil {
		t.Fatalf("MergeChanges error: %v", err)
	}
	if len(ac.Intents) != 2 {
		t.Errorf("Expected 2 intents, got %d", len(ac.Intents))
	}

	changes, _ := ListChanges(dir)
	for _, c := range changes {
		err := DeleteChangeByID(dir, c.ID)
		if err != nil {
			t.Errorf("DeleteChangeByID error: %v", err)
		}
	}

	remaining, _ := ListChanges(dir)
	if len(remaining) != 0 {
		t.Errorf("Expected 0 changes, got %d", len(remaining))
	}
}

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

// ============================================================================
// TestADR: 架构决策记录
// ============================================================================

func TestADR_CreateAndRead(t *testing.T) {
	dir := t.TempDir()

	adr := ADR{
		Title:        "Use Semantic Versioning",
		Status:       ADRStatusAccepted,
		Version:      "0.8.0",
		Context:      "The project needs versioning.",
		Decision:     "Adopt SemVer 2.0.0.",
		Consequences: "Clear version semantics.",
	}

	if err := CreateADR(dir, adr); err != nil {
		t.Fatalf("CreateADR error: %v", err)
	}

	adrs, err := ListADRs(dir)
	if err != nil {
		t.Fatalf("ListADRs error: %v", err)
	}

	if len(adrs) != 1 {
		t.Fatalf("Expected 1 ADR, got %d", len(adrs))
	}

	got := adrs[0]
	if got.Title != adr.Title {
		t.Errorf("Title = %q, want %q", got.Title, adr.Title)
	}
	if got.Status != adr.Status {
		t.Errorf("Status = %q, want %q", got.Status, adr.Status)
	}
	if got.Context != adr.Context {
		t.Errorf("Context = %q, want %q", got.Context, adr.Context)
	}
}

func TestADR_ByVersion(t *testing.T) {
	dir := t.TempDir()

	CreateADR(dir, ADR{Title: "ADR 1", Status: ADRStatusAccepted, Version: "0.7.0"})
	CreateADR(dir, ADR{Title: "ADR 2", Status: ADRStatusAccepted, Version: "0.8.0"})
	CreateADR(dir, ADR{Title: "ADR 3", Status: ADRStatusAccepted, Version: "0.8.0"})

	adrs, err := ADRByVersion(dir, Semver{Major: 0, Minor: 8, Patch: 0})
	if err != nil {
		t.Fatalf("ADRByVersion error: %v", err)
	}
	if len(adrs) != 2 {
		t.Errorf("Expected 2 ADRs for v0.8.0, got %d", len(adrs))
	}
}

func TestADR_StatusLabel(t *testing.T) {
	adr := ADR{Status: ADRStatusAccepted}
	if adr.StatusLabel() == "" {
		t.Error("StatusLabel should not be empty")
	}
}

func TestNextADRID(t *testing.T) {
	dir := t.TempDir()
	id, err := NextADRID(dir)
	if err != nil {
		t.Fatalf("NextADRID error: %v", err)
	}
	if id != "ADR-0001" {
		t.Errorf("First ADR ID should be ADR-0001, got %s", id)
	}
}

func TestADR_Validate(t *testing.T) {
	dir := t.TempDir()
	err := CreateADR(dir, ADR{Title: "", Status: ADRStatusAccepted})
	if err == nil {
		t.Error("Should fail for empty title")
	}
}

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