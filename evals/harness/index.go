package main

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"sort"
	"time"

	"xchats-evals-harness/internal/provenance"
)

//go:embed templates/index.html.tmpl
var runsIndexTemplateFS embed.FS

var runsIndexTemplate = template.Must(template.New("index.html.tmpl").Funcs(template.FuncMap{
	"pct":    formatPct,
	"inList": inStringList,
}).ParseFS(runsIndexTemplateFS, "templates/index.html.tmpl"))

// inStringList is the runs-index template's membership test (Go templates have no
// built-in "is x in this slice" operator) — used to badge a row's Models entries
// against its own already-computed ArchivedModels subset.
func inStringList(s string, list []string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// runsIndexPageData is the html/template view model for runs/index.html — the
// cross-run landing page listing every run dir, newest first.
type runsIndexPageData struct {
	GeneratedAt string
	Rows        []runsIndexRow
}

// runsIndexRow summarizes one run dir. No cross-run comparison here on purpose (per
// the plan: comparison waits until these keys — family, model, prompt version — exist
// across enough real runs to design a meaningful comparison view around); this is
// discoverability and filtering, not diffing.
type runsIndexRow struct {
	RunID string
	// LaunchID groups this run with sibling runs from one `harness launch`
	// invocation (provenance.Manifest.LaunchID) — falls back to the run's own
	// RunID in buildRunsIndexRow when unset, so an unlaunched or legacy run is
	// simply a launch of one member, never a special case downstream.
	LaunchID    string
	HasManifest bool
	Family      string
	Models      []string
	Prompts     []string // "name@vN", extraction only
	StartedAt   string
	// FinishedAt is empty for an interrupted/still-running run (manifest.FinishedAt
	// is only stamped by Manifest.Finish(), called after the run's own work
	// completes) — never guessed from StartedAt, so a duration computed from the
	// pair can honestly report "unknown" instead of zero.
	FinishedAt string

	ScenarioTotal        int
	ScenarioBehaviorPass int
	ScenarioContractPass int
	ExtractTotal         int
	ExtractChecksPass    int

	// ArchivedScenarios/ArchivedModels are the subsets of this run's own scenario
	// names / models (below) that are currently archived — resolved at DISPLAY time
	// against the CURRENT scenarios/*/scenario.yaml + models.yaml, never baked into
	// the run itself (see ScenarioConfig.Archived's doc comment). A run entirely made
	// of archived scenarios/models is still listed (historical data untouched); the
	// index template badges it rather than hiding it.
	Scenarios         []string
	ArchivedScenarios []string
	ArchivedModels    []string

	IndexHref   string // "<id>/index.html", relative to runs/, if that run's own page exists
	ReportLinks []reportLink
	LoadError   string
}

// writeRunsIndex rebuilds runs/index.html wholesale by scanning runsRoot — cheap
// enough (small JSON files per run) to do unconditionally rather than track deltas.
func writeRunsIndex(runsRoot string) error {
	rows, err := buildRunsIndexRows(runsRoot)
	if err != nil {
		return err
	}
	data := runsIndexPageData{
		GeneratedAt: time.Now().UTC().Format("2006-01-02 15:04 MST"),
		Rows:        rows,
	}
	var buf bytes.Buffer
	if err := runsIndexTemplate.Execute(&buf, data); err != nil {
		return fmt.Errorf("render template: %w", err)
	}
	if err := provenance.AtomicWriteFile(filepath.Join(runsRoot, "index.html"), buf.Bytes(), 0o644); err != nil {
		return err
	}
	return writeRunsJSON(runsRoot, rows)
}

// writeRunsIndexBestEffort mirrors writeRunHTMLBestEffort's contract: the index is a
// convenience view, never a reason to fail an otherwise-successful eval run.
func writeRunsIndexBestEffort(runsRoot string) {
	if err := writeRunsIndex(runsRoot); err != nil {
		fmt.Fprintf(os.Stderr, "warning: runs index: %v\n", err)
	}
}

func buildRunsIndexRows(runsRoot string) ([]runsIndexRow, error) {
	entries, err := os.ReadDir(runsRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var rows []runsIndexRow
	for _, e := range entries {
		// These are support directories, not published runs. In particular,
		// .incomplete retains interrupted raw responses for debugging, while catalog
		// holds image assets used by catalog.json. Scanning either one used to create
		// bogus 0/0 rows.
		if !e.IsDir() || isRunSupportDir(e.Name()) {
			continue
		}
		row, err := buildRunsIndexRow(filepath.Join(runsRoot, e.Name()))
		if err != nil {
			// A single unreadable run dir must not silently vanish from the index
			// (no silent caps/omissions) — surface it as a degraded row instead of
			// skipping it or failing the whole index.
			row = runsIndexRow{RunID: e.Name(), LoadError: err.Error()}
		}
		// A directory with no judged scenario rows and no extraction outputs is not
		// an evaluation result. Do not publish a meaningless 0/0 entry.
		if row.LoadError == "" && row.ScenarioTotal == 0 && row.ExtractTotal == 0 {
			continue
		}
		rows = append(rows, row)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].RunID > rows[j].RunID }) // newest first
	return rows, nil
}

func isRunSupportDir(name string) bool {
	return name == "launches" || name == "catalog" || name == provenance.IncompleteRunsDirName
}

func buildRunsIndexRow(runDir string) (runsIndexRow, error) {
	id := filepath.Base(runDir)
	row := runsIndexRow{RunID: id, ReportLinks: reportLinksFor(runDir)}
	// reportLinksFor returns hrefs relative to runDir itself (correct for the per-run
	// page, which lives inside runDir); this index page lives one level up, in
	// runsRoot, so every href needs the run id prefixed back on.
	for i := range row.ReportLinks {
		row.ReportLinks[i].Href = id + "/" + row.ReportLinks[i].Href
	}

	var m provenance.Manifest
	if err := readJSON(filepath.Join(runDir, "manifest.json"), &m); err == nil {
		row.HasManifest = true
		row.Family = m.Family
		row.StartedAt = m.StartedAt
		row.FinishedAt = m.FinishedAt
		row.LaunchID = m.LaunchID
	}
	if row.LaunchID == "" {
		row.LaunchID = id // no -launch flag passed, or a legacy manifest — this run is its own singleton launch
	}

	if _, err := os.Stat(filepath.Join(runDir, "index.html")); err == nil {
		row.IndexHref = id + "/index.html"
	}

	execs, err := loadRunExecutions(runDir)
	if err != nil {
		return row, err
	}
	modelSet := map[string]bool{}
	promptSet := map[string]bool{}
	scenarioSet := map[string]bool{}
	for _, e := range execs {
		if e.Variant.Model != "" {
			modelSet[e.Variant.Model] = true
		}
		if e.Variant.Prompt.Name != "" {
			promptSet[e.Variant.Prompt.String()] = true
		}
		switch e.Family {
		case "scenario":
			row.ScenarioTotal++
			if e.Subject.Scenario != "" {
				scenarioSet[e.Subject.Scenario] = true
			}
			for _, r := range e.Rollups {
				switch {
				case r.Key == "model_behavior_pass" && r.Pass:
					row.ScenarioBehaviorPass++
				case r.Key == "contract_pass" && r.Pass:
					row.ScenarioContractPass++
				}
			}
		case "extract":
			row.ExtractTotal++
			for _, r := range e.Rollups {
				if r.Key == "all_checks_pass" && r.Pass {
					row.ExtractChecksPass++
				}
			}
		}
	}
	row.Models = sortedMapKeys(modelSet)
	row.Prompts = sortedMapKeys(promptSet)
	row.Scenarios = sortedMapKeys(scenarioSet)

	archivedScenarios := loadArchivedScenarios()
	for _, name := range row.Scenarios {
		if info, ok := archivedScenarios[name]; ok && info.Archived {
			row.ArchivedScenarios = append(row.ArchivedScenarios, name)
		}
	}
	archivedModels := loadArchivedModels()
	for _, model := range row.Models {
		if _, ok := archivedModels[orModelID(model)]; ok {
			row.ArchivedModels = append(row.ArchivedModels, model)
		}
	}

	if row.Family == "" {
		switch {
		case row.ScenarioTotal > 0 && row.ExtractTotal > 0:
			row.Family = "mixed"
		case row.ScenarioTotal > 0:
			row.Family = "scenario"
		case row.ExtractTotal > 0:
			row.Family = "extract"
		default:
			row.Family = "unknown"
		}
	}
	return row, nil
}

// sortedMapKeys returns a map's string keys, sorted — the value type is generic since
// every caller so far only ever needs the keys (a set's bool, a tally's int); adding a
// new value type never needs a new copy of this function.
func sortedMapKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
