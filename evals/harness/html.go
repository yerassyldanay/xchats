package main

import (
	"bytes"
	"embed"
	"flag"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"strings"
	"time"

	"xchats-evals-harness/internal/provenance"
)

//go:embed templates/run.html.tmpl
var runHTMLTemplateFS embed.FS

var runHTMLTemplate = template.Must(template.New("run.html.tmpl").Funcs(template.FuncMap{
	"costLabel": formatVCost,
	"pct":       formatPct,
	"upper":     strings.ToUpper,
}).ParseFS(runHTMLTemplateFS, "templates/run.html.tmpl"))

// cmdHTML implements `harness html -run <dir>` — also called automatically (and
// best-effort: a viewer failure warns, never fails the run) at the end of `run`,
// `extract`, and `report`.
func cmdHTML(args []string) error {
	fs := flag.NewFlagSet("html", flag.ExitOnError)
	runDir := fs.String("run", "", "path to the run directory")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *runDir == "" {
		return fmt.Errorf("html: -run is required")
	}
	if err := writeRunHTML(*runDir); err != nil {
		return err
	}
	fmt.Printf("wrote %s\n", filepath.Join(*runDir, "index.html"))
	return nil
}

// writeRunHTMLBestEffort is what run/extract/report call: a broken viewer must never
// turn a successful eval run into a failed command.
func writeRunHTMLBestEffort(runDir string) {
	if err := writeRunHTML(runDir); err != nil {
		fmt.Fprintf(os.Stderr, "warning: html viewer: %v\n", err)
	}
}

func writeRunHTML(runDir string) error {
	data, err := buildRunPage(runDir)
	if err != nil {
		return err
	}
	if len(data.ScenarioGroups) == 0 && len(data.ExtractGroups) == 0 {
		return fmt.Errorf("no evaluated executions in %s; refusing to publish an empty 0/0 report", runDir)
	}
	var buf bytes.Buffer
	if err := runHTMLTemplate.Execute(&buf, data); err != nil {
		return fmt.Errorf("render template: %w", err)
	}
	if err := provenance.AtomicWriteFile(filepath.Join(runDir, "index.html"), buf.Bytes(), 0o644); err != nil {
		return err
	}

	// executions.json is the eval comparison UI's per-run data source — reuses the
	// SAME loadRunExecutions this page's own data came from (buildRunPage already
	// called it once; re-deriving here keeps writeRunHTML's signature simple, and run
	// dirs are small enough that a second glob+read is not worth avoiding).
	execs, err := loadRunExecutions(runDir)
	if err != nil {
		return err
	}
	if err := writeExecutionsJSON(runDir, execs); err != nil {
		return err
	}

	// The cross-run index (step 6) is regenerated wholesale alongside every per-run
	// page — best-effort, same as the per-run page itself must never fail an eval run.
	writeRunsIndexBestEffort(filepath.Dir(runDir))

	return nil
}

// runPageData is the html/template view model for one run's index.html — built
// entirely from loadRunExecutions (step 4) plus whatever provenance files (step 2)
// happen to exist. Works unmodified for legacy runs (HasManifest=false, images show
// as "not captured") and for runs with full provenance.
type runPageData struct {
	RunID          string
	GeneratedAt    string
	HasManifest    bool
	Manifest       provenance.Manifest
	ReportLinks    []reportLink
	ScenarioGroups []scenarioGroup
	ExtractGroups  []extractGroup
}

type reportLink struct {
	Name string
	Href string
}

type scenarioGroup struct {
	Scenario string
	Stats    []scenarioModelStat
	Verdicts []VExecution
}

type scenarioModelStat struct {
	Model        string
	BehaviorPass int
	ContractPass int
	Total        int
	CostLabel    string
}

type extractGroup struct {
	CaseID       string
	InputRelPath string
	InputExists  bool
	Variants     []VExecution
}

func buildRunPage(runDir string) (runPageData, error) {
	execs, err := loadRunExecutions(runDir)
	if err != nil {
		return runPageData{}, err
	}

	data := runPageData{
		RunID:          filepath.Base(runDir),
		GeneratedAt:    time.Now().UTC().Format("2006-01-02 15:04 MST"),
		ReportLinks:    reportLinksFor(runDir),
		ScenarioGroups: groupScenarios(execs),
		ExtractGroups:  groupExtraction(execs, runDir),
	}
	var m provenance.Manifest
	if err := readJSON(filepath.Join(runDir, "manifest.json"), &m); err == nil {
		data.HasManifest = true
		data.Manifest = m
	}
	return data, nil
}

// reportLinksFor lists whichever of the pre-existing Markdown/JSON reports actually
// exist in this run dir, so the HTML page can link to them without assuming a family.
func reportLinksFor(runDir string) []reportLink {
	candidates := []string{"SUMMARY.md", "CONTRACT.md", "EXTRACT.md", "manifest.json", "worktree.diff"}
	var out []reportLink
	for _, c := range candidates {
		if _, err := os.Stat(filepath.Join(runDir, c)); err == nil {
			out = append(out, reportLink{Name: c, Href: c})
		}
	}
	return out
}

// groupExtraction groups extraction executions by case, in first-appearance order,
// and checks for the ACTUAL captured input file on disk — never trusting
// Subject.InputRef alone, since that path is synthesized for every extraction result
// regardless of whether step 2's input-capture ever ran for this (legacy) run.
func groupExtraction(execs []VExecution, runDir string) []extractGroup {
	var order []string
	byCase := map[string]*extractGroup{}
	for _, e := range execs {
		if e.Family != "extract" {
			continue
		}
		id := e.Subject.CaseID
		g, ok := byCase[id]
		if !ok {
			relPath := filepath.Join("inputs", id+".jpg")
			_, statErr := os.Stat(filepath.Join(runDir, relPath))
			g = &extractGroup{CaseID: id, InputRelPath: relPath, InputExists: statErr == nil}
			byCase[id] = g
			order = append(order, id)
		}
		g.Variants = append(g.Variants, e)
	}
	out := make([]extractGroup, 0, len(order))
	for _, id := range order {
		out = append(out, *byCase[id])
	}
	return out
}

// groupScenarios groups scenario executions by scenario name, in first-appearance
// order, and computes the per-model summary table shown above the per-verdict detail.
func groupScenarios(execs []VExecution) []scenarioGroup {
	var order []string
	byScenario := map[string]*scenarioGroup{}
	for _, e := range execs {
		if e.Family != "scenario" {
			continue
		}
		name := e.Subject.Scenario
		g, ok := byScenario[name]
		if !ok {
			g = &scenarioGroup{Scenario: name}
			byScenario[name] = g
			order = append(order, name)
		}
		g.Verdicts = append(g.Verdicts, e)
	}
	out := make([]scenarioGroup, 0, len(order))
	for _, name := range order {
		g := byScenario[name]
		g.Stats = aggregateScenarioModelStats(g.Verdicts)
		out = append(out, *g)
	}
	return out
}

func aggregateScenarioModelStats(verdicts []VExecution) []scenarioModelStat {
	type acc struct {
		total, behaviorPass, contractPass int
		costSum                           float64
		costPricedCount                   int
		anyUnpriced                       bool
	}
	var order []string
	byModel := map[string]*acc{}
	for _, v := range verdicts {
		m := v.Variant.Model
		a, ok := byModel[m]
		if !ok {
			a = &acc{}
			byModel[m] = a
			order = append(order, m)
		}
		a.total++
		for _, r := range v.Rollups {
			switch r.Key {
			case "model_behavior_pass":
				if r.Pass {
					a.behaviorPass++
				}
			case "contract_pass":
				if r.Pass {
					a.contractPass++
				}
			}
		}
		switch v.Cost.Basis {
		case CostBasisMeasured, CostBasisCachedBorrowed:
			a.costSum += v.Cost.EstimateUSD
			a.costPricedCount++
		default:
			a.anyUnpriced = true
		}
	}
	out := make([]scenarioModelStat, 0, len(order))
	for _, m := range order {
		a := byModel[m]
		stat := scenarioModelStat{Model: m, BehaviorPass: a.behaviorPass, ContractPass: a.contractPass, Total: a.total}
		switch {
		case a.costPricedCount == 0:
			stat.CostLabel = "no estimate"
		case a.anyUnpriced:
			stat.CostLabel = fmt.Sprintf("~$%.5f avg (%d/%d priced)", a.costSum/float64(a.costPricedCount), a.costPricedCount, a.total)
		default:
			stat.CostLabel = fmt.Sprintf("~$%.5f avg", a.costSum/float64(a.costPricedCount))
		}
		out = append(out, stat)
	}
	return out
}

// formatVCost mirrors judge.go/report.go's existing cost-basis discipline: a number is
// only ever shown alongside what it's based on, and "no basis" never reads as "$0".
func formatVCost(c VCost) string {
	switch c.Basis {
	case CostBasisMeasured, CostBasisCachedBorrowed:
		return fmt.Sprintf("$%.5f", c.EstimateUSD)
	case CostBasisCachedUnpriced:
		return "unpriceable (cached, no split to borrow)"
	case CostBasisUnknownPricing:
		return "unknown pricing"
	default:
		return "n/a"
	}
}

func formatPct(n, total int) string {
	if total == 0 {
		return "n/a"
	}
	return fmt.Sprintf("%.0f%% (%d/%d)", float64(n)/float64(total)*100, n, total)
}
