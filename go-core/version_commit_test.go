package core

import "testing"

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
