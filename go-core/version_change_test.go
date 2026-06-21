package core

import "testing"

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
