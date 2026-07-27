package main

import (
	"fmt"
	"os"
	"path/filepath"

	"xchats-evals-harness/internal/provenance"
)

// scenarioRunInputs resolves the immutable inputs used to judge or retry one run.
// New schema runs carry their private fixture in the snapshot. Legacy runs retain an
// explicit, warned live-fixture fallback so old evidence remains readable.
type scenarioRunInputs struct {
	Scenario     *ScenarioConfig
	GeneratedDir string
	FixturePath  string
	SnapshotDir  string
}

func resolveScenarioRunInputs(scenarioDir, runDir string) (*scenarioRunInputs, error) {
	liveScenario, err := loadScenario(scenarioDir)
	if err != nil {
		return nil, fmt.Errorf("load scenario.yaml: %w", err)
	}
	resolved := &scenarioRunInputs{
		Scenario:     liveScenario,
		GeneratedDir: filepath.Join(scenarioDir, "generated"),
	}
	if liveScenario.Pipeline == "schema_kb_v1" {
		resolved.FixturePath = filepath.Join(scenarioDir, liveScenario.Data)
	}

	snapshotDir, ok := provenance.SnapshotDirFor(runDir, liveScenario.Name)
	if !ok {
		return resolved, nil
	}
	snapshotScenario, err := loadScenario(snapshotDir)
	if err != nil {
		return nil, fmt.Errorf("load snapshotted scenario.yaml: %w", err)
	}
	resolved.Scenario = snapshotScenario
	resolved.GeneratedDir = snapshotDir
	resolved.SnapshotDir = snapshotDir
	if snapshotScenario.Pipeline != "schema_kb_v1" {
		resolved.FixturePath = ""
		return resolved, nil
	}

	snapshotFixture := filepath.Join(snapshotDir, "kb-fixture.yaml")
	if info, err := os.Stat(snapshotFixture); err == nil && !info.IsDir() {
		resolved.FixturePath = snapshotFixture
		return resolved, nil
	}

	// Runs created before fixture snapshotting cannot be made retroactively immutable.
	// Keep them usable, but make the provenance downgrade visible every time.
	resolved.FixturePath = filepath.Join(scenarioDir, snapshotScenario.Data)
	fmt.Fprintf(
		os.Stderr,
		"warning: schema run %s predates kb-fixture.yaml snapshots; using live fixture %s\n",
		snapshotScenario.Name,
		resolved.FixturePath,
	)
	return resolved, nil
}
