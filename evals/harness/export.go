package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"xchats-evals-harness/internal/provenance"
)

// cmdExport implements `harness export [-run <dir> | -all]` — fatal-on-error
// regeneration of executions.json + index.html (per run) and runs.json + index.html
// (the launches list), unlike the best-effort auto-write writeRunHTML/writeRunsIndex
// already do at the end of run/extract/report/html. The one command a fresh clone
// needs before the eval comparison UI has anything to show: evals/runs/*.judged.json
// (and manifest.json, the *.md reports) are committed, but the derived JSON exports
// are gitignored (see evals/.gitignore) — regenerable, not a second source of truth.
func cmdExport(args []string) error {
	fs := flag.NewFlagSet("export", flag.ExitOnError)
	runDir := fs.String("run", "", "regenerate this one run dir's executions.json + index.html (default with neither flag: -all)")
	all := fs.Bool("all", false, "regenerate every run dir under runs/, plus runs/runs.json + runs/index.html")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *runDir != "" && *all {
		return fmt.Errorf("export: -run and -all are mutually exclusive")
	}

	if *runDir != "" {
		if err := writeRunHTML(*runDir); err != nil {
			return fmt.Errorf("export %s: %w", *runDir, err)
		}
		fmt.Printf("export: wrote %s/{executions.json,index.html}\n", *runDir)
		return nil
	}

	// -all (also the default when neither flag is passed).
	entries, err := os.ReadDir("runs")
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("export: runs/ does not exist yet — nothing to export")
			return nil
		}
		return err
	}
	n := 0
	for _, e := range entries {
		// "launches" holds LaunchManifest files, not run dirs — writeRunHTML would
		// fail loudly on it (no *.judged.json / extract_outputs there), which is
		// exactly the fatal-on-error behavior this command promises, but for the
		// wrong reason; skip it explicitly instead of relying on that accident.
		if !e.IsDir() || e.Name() == "launches" {
			continue
		}
		dir := filepath.Join("runs", e.Name())
		if err := writeRunHTML(dir); err != nil {
			return fmt.Errorf("export %s: %w", dir, err)
		}
		n++
	}
	if err := writeRunsIndex("runs"); err != nil {
		return fmt.Errorf("export: runs.json: %w", err)
	}
	fmt.Printf("export: regenerated %d run(s) + runs/runs.json\n", n)
	return nil
}

// Export schema versions — bump on any breaking shape change so a stale frontend
// build can detect a mismatch rather than silently misrender. Independent of each
// other: executions.json and runs.json can evolve on separate timelines.
const (
	ExecutionsFileSchemaVersion = 1
	RunsFileSchemaVersion       = 1
)

// ExecutionsFile is runs/<id>/executions.json — the eval comparison UI's per-run data
// source. A dedicated, explicitly-tagged export wrapping this run's []VExecution
// (itself already JSON-tagged and package-neutral — NOT one of html.go's
// html/template view models, e.g. runPageData/scenarioGroup, which carry no stability
// contract and are free to change shape purely for template-rendering convenience).
type ExecutionsFile struct {
	SchemaVersion int          `json:"schema_version"`
	RunID         string       `json:"run_id"`
	LaunchID      string       `json:"launch_id,omitempty"`
	GeneratedAt   string       `json:"generated_at"`
	Executions    []VExecution `json:"executions"`
}

// writeExecutionsJSON writes runDir/executions.json from execs. Called alongside the
// HTML viewer (writeRunHTML, best-effort — a broken export must never fail an
// otherwise-successful eval run) and by `harness export` (fatal-on-error, for a
// deterministic post-clone regen — see cmdExport).
func writeExecutionsJSON(runDir string, execs []VExecution) error {
	launchID := ""
	var m provenance.Manifest
	if err := readJSON(filepath.Join(runDir, "manifest.json"), &m); err == nil {
		launchID = m.LaunchID
	}
	file := ExecutionsFile{
		SchemaVersion: ExecutionsFileSchemaVersion,
		RunID:         filepath.Base(runDir),
		LaunchID:      launchID,
		GeneratedAt:   time.Now().UTC().Format(time.RFC3339),
		Executions:    execs,
	}
	b, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return err
	}
	return provenance.AtomicWriteFile(filepath.Join(runDir, "executions.json"), b, 0o644)
}

// RunSummary is one runs.json row — a dedicated, explicitly-tagged export shape,
// independent of runsIndexRow (index.go's html/template view model: no JSON tags,
// never meant to be serialized directly — see runSummaryFromIndexRow).
type RunSummary struct {
	RunID       string   `json:"run_id"`
	LaunchID    string   `json:"launch_id,omitempty"`
	HasManifest bool     `json:"has_manifest"`
	Family      string   `json:"family"`
	Models      []string `json:"models"`
	Prompts     []string `json:"prompts"`
	StartedAt   string   `json:"started_at,omitempty"`

	ScenarioTotal        int `json:"scenario_total"`
	ScenarioBehaviorPass int `json:"scenario_behavior_pass"`
	ScenarioContractPass int `json:"scenario_contract_pass"`
	ExtractTotal         int `json:"extract_total"`
	ExtractChecksPass    int `json:"extract_checks_pass"`

	HasIndexHTML bool   `json:"has_index_html"`
	LoadError    string `json:"load_error,omitempty"`
}

// RunsFile is runs/runs.json — the launches-list page's one data source.
type RunsFile struct {
	SchemaVersion int          `json:"schema_version"`
	GeneratedAt   string       `json:"generated_at"`
	Runs          []RunSummary `json:"runs"`
}

// runSummaryFromIndexRow copies runsIndexRow's already-computed fields into the
// dedicated export shape — index.go's buildRunsIndexRows remains the ONE place
// counting/aggregation happens; this is a pure field-for-field translation, never a
// second computation that could drift from it.
func runSummaryFromIndexRow(row runsIndexRow) RunSummary {
	return RunSummary{
		RunID:                row.RunID,
		LaunchID:             row.LaunchID,
		HasManifest:          row.HasManifest,
		Family:               row.Family,
		Models:               row.Models,
		Prompts:              row.Prompts,
		StartedAt:            row.StartedAt,
		ScenarioTotal:        row.ScenarioTotal,
		ScenarioBehaviorPass: row.ScenarioBehaviorPass,
		ScenarioContractPass: row.ScenarioContractPass,
		ExtractTotal:         row.ExtractTotal,
		ExtractChecksPass:    row.ExtractChecksPass,
		HasIndexHTML:         row.IndexHref != "",
		LoadError:            row.LoadError,
	}
}

// writeRunsJSON writes runsRoot/runs.json from rows (already computed by
// buildRunsIndexRows — see writeRunsIndex).
func writeRunsJSON(runsRoot string, rows []runsIndexRow) error {
	summaries := make([]RunSummary, len(rows))
	for i, row := range rows {
		summaries[i] = runSummaryFromIndexRow(row)
	}
	file := RunsFile{
		SchemaVersion: RunsFileSchemaVersion,
		GeneratedAt:   time.Now().UTC().Format(time.RFC3339),
		Runs:          summaries,
	}
	b, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return err
	}
	return provenance.AtomicWriteFile(filepath.Join(runsRoot, "runs.json"), b, 0o644)
}
