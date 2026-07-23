package main

import (
	"strings"
	"testing"
)

func TestResolveExpectedCalls(t *testing.T) {
	tests := []struct {
		name                           string
		totalTests, numModels, repeats int
		want                           int
	}{
		{"no repeats (default)", 20, 4, 1, 80},
		{"screening: 3 uncached repetitions", 6, 4, 3, 72},
		{"finalist: 15 intents x 5 repetitions, one model", 15, 1, 5, 75},
		{"zero tests", 0, 4, 3, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolveExpectedCalls(tt.totalTests, tt.numModels, tt.repeats); got != tt.want {
				t.Errorf("resolveExpectedCalls(%d,%d,%d) = %d, want %d", tt.totalTests, tt.numModels, tt.repeats, got, tt.want)
			}
		})
	}
}

func TestValidateResolvedRunSize(t *testing.T) {
	tests := []struct {
		name                   string
		totalTests, totalCalls int
		wantErr                bool
	}{
		{"normal run", 31, 93, false},
		{"zero tests", 0, 0, true},
		{"zero calls", 31, 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateResolvedRunSize(tt.totalTests, tt.totalCalls)
			if tt.wantErr && err == nil {
				t.Fatal("want error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("want no error, got %v", err)
			}
		})
	}
}

// TestCmdRun_ExplicitArchivedScenario_HardErrors confirms a deliberate -scenario
// request against an archived scenario fails loudly BEFORE any billed call, unlike
// -all's silent skip — see ScenarioConfig.Archived's doc comment for why the two paths
// differ.
func TestCmdRun_ExplicitArchivedScenario_HardErrors(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	t.Setenv("OPENROUTER_API_KEY", "fake-key-for-test")

	writeCatalogFixtureScenario(t, root, "retired", ScenarioConfig{
		Data: "data.yaml", Tests: "tests.yaml", Archived: true, ArchivedReason: "superseded",
	}, Data{}, "tests:\n  - id: \"1\"\n    message: \"hi\"\n")

	_, err := cmdRun([]string{"-scenario", "scenarios/retired"})
	if err == nil {
		t.Fatal("want an error for an explicit -scenario request against an archived scenario, got nil")
	}
	if !strings.Contains(err.Error(), "archived") {
		t.Errorf("want the error to mention archival, got: %v", err)
	}
}

// TestCmdRun_AllSkipsArchivedScenarios confirms -all silently omits an archived
// scenario from what it resolves — proven here by the "zero tests" refusal that
// results when the only scenario present is archived, exactly like an empty
// scenarios/ dir would.
func TestCmdRun_AllSkipsArchivedScenarios(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	t.Setenv("OPENROUTER_API_KEY", "fake-key-for-test")

	writeCatalogFixtureScenario(t, root, "retired", ScenarioConfig{
		Data: "data.yaml", Tests: "tests.yaml", Archived: true, ArchivedReason: "superseded",
	}, Data{}, "tests:\n  - id: \"1\"\n    message: \"hi\"\n")

	_, err := cmdRun([]string{"-all"})
	if err == nil {
		t.Fatal("want an error: the only scenario is archived and must be skipped, resolving zero scenarios")
	}
	// scenarioDirs ends up empty (the archived scenario was skipped, not counted), which
	// hits the same "nothing resolved" refusal an empty scenarios/ dir would — proof
	// the skip happened before any render/count, not merely that the final tally was 0.
	if !strings.Contains(err.Error(), "pass -scenario") {
		t.Errorf("want the empty-selection refusal (proving the archived scenario was skipped, not rendered), got: %v", err)
	}
}

func TestValidateRepeats(t *testing.T) {
	tests := []struct {
		name    string
		repeats int
		noCache bool
		wantErr bool
	}{
		{"default repeats=1, cache allowed", 1, false, false},
		{"repeats=1 with -no-cache is also fine", 1, true, false},
		{"repeats>1 WITHOUT -no-cache is rejected", 5, false, true},
		{"repeats>1 WITH -no-cache is fine", 5, true, false},
		{"repeats=0 is rejected outright", 0, true, true},
		{"negative repeats is rejected outright", -1, true, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateRepeats(tt.repeats, tt.noCache)
			if tt.wantErr && err == nil {
				t.Errorf("validateRepeats(%d, noCache=%v): want an error, got nil", tt.repeats, tt.noCache)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("validateRepeats(%d, noCache=%v): want no error, got %v", tt.repeats, tt.noCache, err)
			}
		})
	}
}
