package core

import "testing"

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
