//go:build lecore_tier1 || lecore_tier2 || lecore_tier3 || lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

package core

import (
	"testing"
)

func TestDegradationManager_DefaultMode(t *testing.T) {
	obs := &InMemoryObservationAdapter{}
	dm := NewDegradationManager(obs)

	if dm.CurrentMode() != DegradationNone {
		t.Errorf("expected none, got %s", dm.CurrentMode())
	}
}

func TestDegradationManager_Degrade(t *testing.T) {
	obs := &InMemoryObservationAdapter{}
	dm := NewDegradationManager(obs)

	dm.Degrade(DegradationNonCritical)
	if dm.CurrentMode() != DegradationNonCritical {
		t.Errorf("expected non_critical, got %s", dm.CurrentMode())
	}
}

func TestDegradationManager_Recover(t *testing.T) {
	obs := &InMemoryObservationAdapter{}
	dm := NewDegradationManager(obs)

	dm.Degrade(DegradationEmergency)
	dm.Recover()
	if dm.CurrentMode() != DegradationNone {
		t.Errorf("expected none after recover, got %s", dm.CurrentMode())
	}
}

func TestDegradationManager_ShouldProcessNormal(t *testing.T) {
	obs := &InMemoryObservationAdapter{}
	dm := NewDegradationManager(obs)

	// None mode: everything allowed
	if !dm.ShouldProcess("critical") {
		t.Error("critical should be allowed in none mode")
	}
	if !dm.ShouldProcess("high") {
		t.Error("high should be allowed in none mode")
	}
	if !dm.ShouldProcess("normal") {
		t.Error("normal should be allowed in none mode")
	}
	if !dm.ShouldProcess("low") {
		t.Error("low should be allowed in none mode")
	}
}

func TestDegradationManager_ShouldProcessEmergency(t *testing.T) {
	obs := &InMemoryObservationAdapter{}
	dm := NewDegradationManager(obs)

	dm.Degrade(DegradationEmergency)

	if !dm.ShouldProcess("critical") {
		t.Error("critical should be allowed in emergency")
	}
	if dm.ShouldProcess("high") {
		t.Error("high should be blocked in emergency")
	}
	if dm.ShouldProcess("normal") {
		t.Error("normal should be blocked in emergency")
	}
	if dm.ShouldProcess("low") {
		t.Error("low should be blocked in emergency")
	}
}

func TestDegradationManager_ShouldProcessSafe(t *testing.T) {
	obs := &InMemoryObservationAdapter{}
	dm := NewDegradationManager(obs)

	dm.Degrade(DegradationSafe)

	if !dm.ShouldProcess("critical") {
		t.Error("critical should be allowed in safe mode")
	}
	if !dm.ShouldProcess("high") {
		t.Error("high should be allowed in safe mode")
	}
	if dm.ShouldProcess("normal") {
		t.Error("normal should be blocked in safe mode")
	}
	if dm.ShouldProcess("low") {
		t.Error("low should be blocked in safe mode")
	}
}

func TestDegradationManager_ShouldProcessNonCritical(t *testing.T) {
	obs := &InMemoryObservationAdapter{}
	dm := NewDegradationManager(obs)

	dm.Degrade(DegradationNonCritical)

	if !dm.ShouldProcess("critical") {
		t.Error("critical should be allowed in non_critical")
	}
	if !dm.ShouldProcess("high") {
		t.Error("high should be allowed in non_critical")
	}
	if !dm.ShouldProcess("normal") {
		t.Error("normal should be allowed in non_critical")
	}
	if dm.ShouldProcess("low") {
		t.Error("low should be blocked in non_critical")
	}
}

func TestDegradationManager_NilObs(t *testing.T) {
	dm := NewDegradationManager(nil)
	if dm.CurrentMode() != DegradationNone {
		t.Errorf("expected none, got %s", dm.CurrentMode())
	}
	// Should still work with nil obs
	dm.Degrade(DegradationSafe)
	if dm.CurrentMode() != DegradationSafe {
		t.Errorf("expected safe, got %s", dm.CurrentMode())
	}
}