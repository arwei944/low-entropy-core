// Package core — 通用版本管理模块 (v0.8.0)
//
// version_semver_test.go: SemVer 解析与比较

package core

import "testing"

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
