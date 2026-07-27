package main

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestResolveExpectedExtractCalls(t *testing.T) {
	tests := []struct {
		name                            string
		numCases, numModels, numPrompts int
		want                            int
	}{
		{"one case, one model, default prompt", 1, 1, 1, 1},
		{"5 cases x 3 models x 2 prompts (v1/v2 comparison)", 5, 3, 2, 30},
		{"zero cases", 0, 3, 2, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolveExpectedExtractCalls(tt.numCases, tt.numModels, tt.numPrompts); got != tt.want {
				t.Errorf("resolveExpectedExtractCalls(%d,%d,%d) = %d, want %d", tt.numCases, tt.numModels, tt.numPrompts, got, tt.want)
			}
		})
	}
}

// TestCmdLaunch_RequiresAllInV1 guards the documented v1 scope (review amendment 4's
// launch manifest doesn't help if the command silently ran a partial, undeclared
// scope) — must fail BEFORE any pre-flight counting or manifest write, not partway
// through.
func TestCmdLaunch_RequiresAllInV1(t *testing.T) {
	if err := cmdLaunch(nil); err == nil {
		t.Fatal("want cmdLaunch to reject running with neither -all nor any scope flag")
	}
}

// TestPreflightScenarioCallCount_SkipsArchivedScenarios confirms launch's own -all
// pre-flight honors the same zero-new-spend archival guarantee as cmdRun's -all
// expansion (run.go) — an archived scenario contributes zero tests to the count and is
// never rendered, not merely hidden from the final total.
func TestPreflightScenarioCallCount_SkipsArchivedScenarios(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)

	writeCatalogFixtureScenario(t, root, "retired", ScenarioConfig{
		Data: "data.yaml", Tests: "tests.yaml", Archived: true, ArchivedReason: "superseded",
	}, Data{}, "tests:\n  - id: \"1\"\n    message: \"hi\"\n")

	modelsB, err := yaml.Marshal(&ModelsFile{Providers: []ModelProvider{{ID: "openrouter:test/model", Default: true}}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "models.yaml"), modelsB, 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := preflightScenarioCallCount("models.yaml")
	if err != nil {
		t.Fatalf("preflightScenarioCallCount: %v", err)
	}
	if got != 0 {
		t.Fatalf("want 0 billed calls (the only scenario is archived and must be skipped, never rendered), got %d", got)
	}
}
