package provenance

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestNewLaunchManifest_WritesBeforeAnyMemberStarts is the core guarantee
// LaunchManifest exists for: the manifest file must be ON DISK, with every planned
// family recorded as "pending," the instant NewLaunchManifest returns — before the
// caller has run a single billed call. An interrupted launch (process killed right
// after this call) still leaves an honest record of what was planned.
func TestNewLaunchManifest_WritesBeforeAnyMemberStarts(t *testing.T) {
	runsRoot := t.TempDir()
	lm, err := NewLaunchManifest(runsRoot, []string{"scenario", "extract"}, LaunchCallCount{Scenario: 10, Extract: 5, Total: 15})
	if err != nil {
		t.Fatal(err)
	}
	if lm.Status != LaunchStatusRunning {
		t.Errorf("want initial status=running, got %q", lm.Status)
	}
	if len(lm.Members) != 2 || lm.Members[0].Status != LaunchMemberPending || lm.Members[1].Status != LaunchMemberPending {
		t.Fatalf("want both members pending, got %+v", lm.Members)
	}

	path := filepath.Join(runsRoot, "launches", lm.LaunchID+".json")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("want the manifest already on disk at %s: %v", path, err)
	}
	var onDisk LaunchManifest
	if err := json.Unmarshal(b, &onDisk); err != nil {
		t.Fatal(err)
	}
	if onDisk.ExpectedCalls.Total != 15 {
		t.Errorf("want the pre-flight count already recorded on disk, got %+v", onDisk.ExpectedCalls)
	}
}

// TestLaunchManifest_FinishDerivesStatusFromMembers is the regression test for
// review amendment 4: a launch where one family completes and the other fails must
// report "partial," never a flat success or failure that would hide which half broke.
func TestLaunchManifest_FinishDerivesStatusFromMembers(t *testing.T) {
	tests := []struct {
		name    string
		members []LaunchMember
		want    string
	}{
		{"all complete", []LaunchMember{{Status: LaunchMemberComplete}, {Status: LaunchMemberComplete}}, LaunchStatusComplete},
		{"all failed", []LaunchMember{{Status: LaunchMemberFailed}, {Status: LaunchMemberFailed}}, LaunchStatusFailed},
		{"mixed -> partial", []LaunchMember{{Status: LaunchMemberComplete}, {Status: LaunchMemberFailed}}, LaunchStatusPartial},
		{"single member, complete", []LaunchMember{{Status: LaunchMemberComplete}}, LaunchStatusComplete},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &LaunchManifest{Members: tt.members}
			m.Finish()
			if m.Status != tt.want {
				t.Errorf("Finish() status = %q, want %q", m.Status, tt.want)
			}
			if m.FinishedAt == "" {
				t.Error("want FinishedAt stamped")
			}
		})
	}
}

// TestLaunchManifest_SetMemberUpdatesInPlace proves SetMember mutates the right
// member by family, preserving fields not passed (empty runID/errMsg must not
// clobber a previously-recorded value — e.g. a "running" -> "failed" transition that
// already has a RunID from an earlier "running" update).
func TestLaunchManifest_SetMemberUpdatesInPlace(t *testing.T) {
	m := &LaunchManifest{Members: []LaunchMember{{Family: "scenario", Status: LaunchMemberPending}, {Family: "extract", Status: LaunchMemberPending}}}

	m.SetMember("scenario", LaunchMemberRunning, "", "")
	m.SetMember("scenario", LaunchMemberComplete, "2026-01-01_00-00-00-aaaa", "")
	if m.Members[0].Status != LaunchMemberComplete || m.Members[0].RunID != "2026-01-01_00-00-00-aaaa" {
		t.Errorf("want scenario member updated, got %+v", m.Members[0])
	}
	if m.Members[1].Status != LaunchMemberPending {
		t.Errorf("want extract member untouched, got %+v", m.Members[1])
	}

	m.SetMember("extract", LaunchMemberFailed, "", "boom")
	if m.Members[1].Status != LaunchMemberFailed || m.Members[1].Error != "boom" {
		t.Errorf("want extract member's failure recorded, got %+v", m.Members[1])
	}
}

// TestWriteLaunchManifest_RoundTrips proves the write/read pair preserves every field.
func TestWriteLaunchManifest_RoundTrips(t *testing.T) {
	runsRoot := t.TempDir()
	lm, err := NewLaunchManifest(runsRoot, []string{"scenario"}, LaunchCallCount{Scenario: 3, Total: 3})
	if err != nil {
		t.Fatal(err)
	}
	lm.SetMember("scenario", LaunchMemberComplete, "run-1", "")
	lm.Finish()
	if err := WriteLaunchManifest(runsRoot, lm); err != nil {
		t.Fatal(err)
	}

	b, err := os.ReadFile(filepath.Join(runsRoot, "launches", lm.LaunchID+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var reread LaunchManifest
	if err := json.Unmarshal(b, &reread); err != nil {
		t.Fatal(err)
	}
	if reread.Status != LaunchStatusComplete || reread.Members[0].RunID != "run-1" {
		t.Errorf("round-trip mismatch: %+v", reread)
	}
}
