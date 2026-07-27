package provenance

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const launchSchemaVersion = 1

// LaunchCallCount is the combined pre-flight billed-call estimate `harness launch`
// prints and gates -expect-calls against BEFORE any member run starts — split by
// family so a mismatch is diagnosable (which family's count is off), plus the total.
type LaunchCallCount struct {
	Scenario int `json:"scenario"`
	Extract  int `json:"extract"`
	Total    int `json:"total"`
}

// LaunchMember is one family's run within a launch. Status starts "pending" (written
// before ANY billed call, so an interrupted launch still leaves an honest record of
// what was planned) and moves to "running" -> "complete" or "failed". RunID is empty
// until that family's run dir actually exists — a member can fail before ever minting
// one (e.g. the family's own render/pre-flight step errors).
type LaunchMember struct {
	Family string `json:"family"` // "scenario" | "extract"
	RunID  string `json:"run_id,omitempty"`
	Status string `json:"status"` // "pending" | "running" | "complete" | "failed"
	Error  string `json:"error,omitempty"`
}

const (
	LaunchMemberPending  = "pending"
	LaunchMemberRunning  = "running"
	LaunchMemberComplete = "complete"
	LaunchMemberFailed   = "failed"
)

// LaunchManifest is runs/launches/<id>.json — written BEFORE any billed call so a
// human or the eval viewer can tell "what was this launch supposed to run" apart from
// "what actually finished," even if the process died halfway through. Unlike a run's
// own manifest.json, this is the ONE thing that must exist before the first API call:
// a launch that fails before minting any member run dir still leaves this record.
//
// Committed to git, like manifest.json — it is provenance (what was planned and what
// happened), not a derived/regenerable artifact.
type LaunchManifest struct {
	SchemaVersion   int              `json:"schema_version"`
	LaunchID        string           `json:"launch_id"`
	Status          string           `json:"status"` // "running" | "complete" | "failed" | "partial"
	PlannedFamilies []string         `json:"planned_families"`
	ExpectedCalls   LaunchCallCount  `json:"expected_calls"`
	Members         []LaunchMember   `json:"members"`
	StartedAt       string           `json:"started_at"`
	FinishedAt      string           `json:"finished_at,omitempty"`
}

const (
	LaunchStatusRunning = "running"
	LaunchStatusPartial = "partial"
	LaunchStatusFailed  = "failed"
	LaunchStatusComplete = "complete"
)

// NewLaunchManifest mints a fresh, unique launch id under runsRoot/launches/ (same
// exclusive-create collision-avoidance discipline as NewRunDir — claim the id by
// atomically creating its manifest file, not by checking-then-creating) and writes the
// initial manifest immediately, status "running", every member "pending". Callers
// mutate the returned manifest's Members as each family's run starts/finishes and call
// WriteLaunchManifest again — the same read-mutate-rewrite pattern run.go/extract.go
// already use for their own per-run Manifest.
func NewLaunchManifest(runsRoot string, plannedFamilies []string, expected LaunchCallCount) (*LaunchManifest, error) {
	launchesDir := filepath.Join(runsRoot, "launches")
	if err := os.MkdirAll(launchesDir, 0o755); err != nil {
		return nil, err
	}
	members := make([]LaunchMember, len(plannedFamilies))
	for i, f := range plannedFamilies {
		members[i] = LaunchMember{Family: f, Status: LaunchMemberPending}
	}

	const maxAttempts = 10
	for attempt := 0; attempt < maxAttempts; attempt++ {
		id := time.Now().Format("2006-01-02_15-04-05") + "-" + randSuffix(4)
		path := filepath.Join(launchesDir, id+".json")
		f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if err == nil {
			f.Close()
			m := &LaunchManifest{
				SchemaVersion:   launchSchemaVersion,
				LaunchID:        id,
				Status:          LaunchStatusRunning,
				PlannedFamilies: plannedFamilies,
				ExpectedCalls:   expected,
				Members:         members,
				StartedAt:       time.Now().UTC().Format(time.RFC3339),
			}
			if err := WriteLaunchManifest(runsRoot, m); err != nil {
				return nil, err
			}
			return m, nil
		}
		if !os.IsExist(err) {
			return nil, err
		}
	}
	return nil, fmt.Errorf("NewLaunchManifest: could not claim a unique launch id under %s after %d attempts", launchesDir, maxAttempts)
}

// WriteLaunchManifest atomically (re)writes runsRoot/launches/<id>.json — safe to call
// repeatedly as member status changes, same contract as WriteManifest.
func WriteLaunchManifest(runsRoot string, m *LaunchManifest) error {
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return AtomicWriteFile(filepath.Join(runsRoot, "launches", m.LaunchID+".json"), b, 0o644)
}

// Finish derives Status from the final member states and stamps FinishedAt — call
// once every member has reached a terminal state (complete/failed), then
// WriteLaunchManifest once more. All-complete -> "complete"; all-failed -> "failed";
// a mix -> "partial", so the UI can show "2 of 3 families ran" honestly instead of
// reporting a launch as a flat success or failure it wasn't.
func (m *LaunchManifest) Finish() {
	complete, failed := 0, 0
	for _, mem := range m.Members {
		switch mem.Status {
		case LaunchMemberComplete:
			complete++
		case LaunchMemberFailed:
			failed++
		}
	}
	switch {
	case failed == 0 && complete == len(m.Members):
		m.Status = LaunchStatusComplete
	case complete == 0:
		m.Status = LaunchStatusFailed
	default:
		m.Status = LaunchStatusPartial
	}
	m.FinishedAt = time.Now().UTC().Format(time.RFC3339)
}

// SetMember updates the named family's member entry in place (status + optional
// run id/error) — the one mutation launch.go's cmdLaunch needs between each
// WriteLaunchManifest call. No-op if family isn't a planned member (defensive; should
// never happen since Members is seeded from PlannedFamilies at construction).
func (m *LaunchManifest) SetMember(family, status, runID, errMsg string) {
	for i := range m.Members {
		if m.Members[i].Family != family {
			continue
		}
		m.Members[i].Status = status
		if runID != "" {
			m.Members[i].RunID = runID
		}
		if errMsg != "" {
			m.Members[i].Error = errMsg
		}
		return
	}
}
