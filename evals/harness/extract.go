package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"xchats-evals-harness/internal/provenance"
)

// cmdExtract runs the pass-1 extraction eval: for each case in cases.yaml, downscale its
// image and send it to every selected model, grade the JSON answer against the case's
// requirements, and write a report. This is Eval 1 (file -> extracted information) — the
// deliberately separate, smaller half of evaluating the playground design; Eval 2
// (extracted information -> ai_* draft schema) is a later, separate command that
// consumes this command's -record'd fixtures instead of calling vision models again.
// cmdExtract returns the fresh run's id on success (empty on any error) — same
// contract as cmdRun, so cmdLaunch can record either family's member run id uniformly.
func cmdExtract(args []string) (runID string, err error) {
	fs := flag.NewFlagSet("extract", flag.ExitOnError)
	casesPath := fs.String("cases", "extract/cases.yaml", "path to the extraction cases file")
	caseID := fs.String("case", "", "run only this case id (default: every case in cases.yaml)")
	all := fs.Bool("all", false, "run every case (default when neither -case nor -all is given)")
	modelsFlag := fs.String("models", "", "comma-separated model ids to run (default: $EVAL_VISION_MODELS, else every provider in models.yaml)")
	modelsPath := fs.String("models-file", "models.yaml", "path to models.yaml")
	promptsFlag := fs.String("prompt", "extract@v1", "comma-separated prompt specs to run, <name>@v<N> (e.g. extract@v1,extract@v2)")
	promptsDir := fs.String("prompts-dir", "prompts", "directory containing <name>/v<N>.txt prompt files")
	record := fs.Bool("record", false, "write the first fully-passing output per case to extract/fixtures/<case>.json")
	baseURL := fs.String("base-url", "", "override the OpenRouter base URL (default: $EVAL_BASE_URL, else https://openrouter.ai/api/v1)")
	envPath := fs.String("env", ".env", "path to a .env file to load (best-effort; missing file is fine)")
	maxTokens := fs.Int("max-tokens", extractMaxTokens, "completion token budget per call (overrides models.yaml's max_tokens, which is tuned for short chat replies, not full-text transcription)")
	launchID := fs.String("launch", "", "group this run under an existing launch id (see `harness launch`) — leave empty for a standalone run, which is then its own singleton launch")
	if err := fs.Parse(args); err != nil {
		return "", err
	}
	_ = all // accepted for symmetry with -case; running with neither already means "every case"

	loadDotEnv(*envPath)

	apiKey := os.Getenv("OPENROUTER_API_KEY")
	if apiKey == "" {
		return "", fmt.Errorf("extract: OPENROUTER_API_KEY is not set (copy evals/.env.example to evals/.env and fill it in, or export it)")
	}
	base := *baseURL
	if base == "" {
		base = envOrDefault("EVAL_BASE_URL", "https://openrouter.ai/api/v1")
	}

	cf, err := loadExtractCases(*casesPath)
	if err != nil {
		return "", err
	}
	cases := cf.Cases
	if *caseID != "" {
		cases = filterCases(cases, *caseID)
		if len(cases) == 0 {
			return "", fmt.Errorf("extract: no case %q in %s", *caseID, *casesPath)
		}
	}
	if len(cases) == 0 {
		return "", fmt.Errorf("extract: %s has no cases", *casesPath)
	}

	models, err := resolveVisionModels(*modelsFlag, *modelsPath)
	if err != nil {
		return "", err
	}
	if len(models) == 0 {
		return "", fmt.Errorf("extract: no models to run (pass -models or set EVAL_VISION_MODELS)")
	}
	// models.yaml's max_tokens is tuned for Layer-1's short customer-chat replies;
	// full-text transcription needs much more headroom, or the model gets cut off
	// mid-JSON (observed: gpt-4o-mini/gemini truncating mid-sentence at exactly 500
	// completion tokens). Override for every provider, uniformly.
	for i := range models {
		models[i].MaxTokens = *maxTokens
	}

	prompts, err := provenance.LoadPromptSpecs(*promptsDir, splitCSV(*promptsFlag))
	if err != nil {
		return "", err
	}
	if len(prompts) == 0 {
		return "", fmt.Errorf("extract: no prompts to run (pass -prompt <name>@v<N>[,...])")
	}

	client := newORClient(base, apiKey)
	casesDir := filepath.Dir(*casesPath)

	var runDir string
	runID, runDir, err = provenance.NewRunDir("runs")
	if err != nil {
		return "", err
	}
	outputsDir := filepath.Join(runDir, "extract_outputs")
	if err := os.MkdirAll(outputsDir, 0o755); err != nil {
		return runID, err
	}

	manifest := provenance.NewManifest(runDir, runID, "extract", "extract", args)
	manifest.ModelsPath = *modelsPath
	manifest.LaunchID = *launchID
	if sha, err := provenance.SnapshotFile(*modelsPath, filepath.Join(runDir, "snapshots", "models.yaml")); err != nil {
		return runID, fmt.Errorf("snapshot %s: %w", *modelsPath, err)
	} else {
		manifest.ModelsSHA256 = sha
	}
	manifest.ExtractCasesPath = *casesPath
	if sha, err := provenance.SnapshotFile(*casesPath, filepath.Join(runDir, "snapshots", "extract_cases.yaml")); err != nil {
		return runID, fmt.Errorf("snapshot %s: %w", *casesPath, err)
	} else {
		manifest.ExtractCasesSHA256 = sha
	}
	if err := provenance.WriteManifest(runDir, manifest); err != nil {
		return runID, err
	}

	var results []extractRunResult
	for _, c := range cases {
		imgPath := filepath.Join(casesDir, c.File)
		imgData, mimetype, preprocessorName, err := preprocess(c.Kind, imgPath)
		if err != nil {
			return runID, fmt.Errorf("case %s: %w", c.ID, err)
		}

		originalSHA, err := provenance.SHA256File(imgPath)
		if err != nil {
			return runID, fmt.Errorf("case %s: hash original input: %w", c.ID, err)
		}
		inputDst := filepath.Join(runDir, "inputs", c.ID+".jpg")
		if err := os.MkdirAll(filepath.Dir(inputDst), 0o755); err != nil {
			return runID, err
		}
		if err := provenance.AtomicWriteFile(inputDst, imgData, 0o644); err != nil {
			return runID, fmt.Errorf("case %s: save processed input: %w", c.ID, err)
		}
		manifest.Inputs = append(manifest.Inputs, provenance.InputSnapshotRef{
			CaseID:          c.ID,
			OriginalSHA256:  originalSHA,
			ProcessedSHA256: provenance.SHA256Bytes(imgData),
		})
		if err := provenance.WriteManifest(runDir, manifest); err != nil {
			return runID, err
		}

		recorded := false
		for _, m := range models {
			for _, p := range prompts {
				fmt.Printf("extract: case=%s model=%s prompt=%s ... ", c.ID, orModelID(m.ID), p.Ref)
				res := runOneExtraction(context.Background(), client, c, m, p, preprocessorName, imgData, mimetype)
				results = append(results, res)

				outPath := filepath.Join(outputsDir, extractOutputFilename(c.ID, m, p.Ref))
				if werr := writeJSON(outPath, res); werr != nil {
					return runID, fmt.Errorf("write %s: %w", outPath, werr)
				}

				switch {
				case res.Error != "":
					fmt.Printf("ERROR: %s\n", res.Error)
				case res.ParseError != "":
					fmt.Printf("PARSE FAILED: %s\n", res.ParseError)
				default:
					fmt.Printf("%s\n", passFailSummary(res.Checks))
				}

				if *record && !recorded && res.Parsed != nil && allChecksPass(res.Checks) {
					fixturePath := filepath.Join(casesDir, "fixtures", c.ID+".json")
					if err := os.MkdirAll(filepath.Dir(fixturePath), 0o755); err != nil {
						return runID, err
					}
					if err := writeJSON(fixturePath, res.Parsed); err != nil {
						return runID, err
					}
					provPath := filepath.Join(casesDir, "fixtures", c.ID+".provenance.json")
					prov := fixtureProvenance{
						Model:                orModelID(m.ID),
						Prompt:               p.Ref,
						RunID:                runID,
						ProcessedInputSHA256: provenance.SHA256Bytes(imgData),
					}
					if err := writeJSON(provPath, prov); err != nil {
						return runID, err
					}
					fmt.Printf("  recorded fixture: %s (model=%s, prompt=%s)\n", fixturePath, orModelID(m.ID), p.Ref)
					recorded = true
				}
			}
		}
	}

	reportPath := filepath.Join(runDir, "EXTRACT.md")
	if err := writeExtractReport(reportPath, results); err != nil {
		return runID, err
	}
	fmt.Printf("\nwrote %s\n", reportPath)

	manifest.Finish()
	if err := provenance.WriteManifest(runDir, manifest); err != nil {
		return runID, err
	}

	// Best-effort: a broken HTML viewer must never turn a successful extraction run
	// into a failed command.
	writeRunHTMLBestEffort(runDir)

	return runID, appendExtractIndex(runDir, results)
}

// parseFailureRetries bounds retries for a parse failure specifically (never for a
// genuine HTTP/network error, which describeImage already surfaces as res.Error).
// Observed repeatedly in practice: some model routes (gemini-2.5-flash via OpenRouter
// most often) occasionally return an abruptly truncated response — cut off mid-word,
// far under the token budget, usually with usage reported as all-zero — that clears up
// on a plain retry. That's an infrastructure blip, not the model's answer, and
// reporting it as a hard failure would misrepresent the model being evaluated.
const parseFailureRetries = 2

func runOneExtraction(ctx context.Context, client *orClient, c ExtractCase, m ModelProvider, p provenance.LoadedPrompt, preprocessorName string, imgData []byte, mimetype string) extractRunResult {
	var res extractRunResult
	for attempt := 0; attempt <= parseFailureRetries; attempt++ {
		// providerModelKey (judge.go), not the bare id: two provider entries can share
		// an id with different Label values (e.g. a reasoning-on/off pair), and every
		// downstream grouping here (buildExtractAggregate, the HTML viewer) keys purely
		// off this Model string — an unlabeled key would silently merge them.
		res = extractRunResult{CaseID: c.ID, Model: providerModelKey(orModelID(m.ID), m.Label), Prompt: p.Ref, Preprocessor: preprocessorName}
		raw, reasoning, usage, err := client.describeImage(ctx, m, p.Content, imgData, mimetype)
		if err != nil {
			res.Error = err.Error()
			return res // a real HTTP/network error is not retried here
		}
		res.Raw = raw
		res.Reasoning = reasoning
		res.Usage = usage
		res.Cost, res.CostBasis = estimateCost(m, usage)

		parsed, perr := parseExtractionJSON(raw)
		if perr != nil {
			res.ParseError = perr.Error()
			if attempt < parseFailureRetries {
				fmt.Printf("[parse failed, retrying %d/%d] ", attempt+1, parseFailureRetries)
				continue // retry: likely a truncated-response blip, not the model's real answer
			}
			return res // out of retries — report the failure honestly
		}
		res.Parsed = &parsed
		res.Checks = runExtractChecks(c, parsed)
		return res
	}
	return res
}

func passFailSummary(checks []CheckResult) string {
	pass, total := 0, len(checks)
	for _, c := range checks {
		if c.Pass {
			pass++
		}
	}
	verdict := "FAIL"
	if pass == total {
		verdict = "PASS"
	}
	return fmt.Sprintf("%s (%d/%d checks)", verdict, pass, total)
}

func loadExtractCases(path string) (*ExtractCasesFile, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var cf ExtractCasesFile
	if err := yaml.Unmarshal(b, &cf); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	for i, c := range cf.Cases {
		if c.ID == "" {
			return nil, fmt.Errorf("%s: case at index %d has no id", path, i)
		}
		if c.File == "" {
			return nil, fmt.Errorf("%s: case %q has no file", path, c.ID)
		}
	}
	return &cf, nil
}

func filterCases(cases []ExtractCase, id string) []ExtractCase {
	var out []ExtractCase
	for _, c := range cases {
		if c.ID == id {
			out = append(out, c)
		}
	}
	return out
}

func sanitizeModelID(id string) string {
	id = orModelID(id)
	return strings.NewReplacer("/", "_", ":", "_").Replace(id)
}

// extractOutputFilename names one (case, model, prompt) result file. The prompt
// segment is what makes results from two prompt versions of the same case+model
// coexist in one run instead of the second silently overwriting the first.
func extractOutputFilename(caseID string, m ModelProvider, prompt provenance.PromptRef) string {
	return fmt.Sprintf("%s__%s__%s-v%d.json", caseID, sanitizeModelID(m.ID), prompt.Name, prompt.Version)
}

// fixtureProvenance is the sidecar written next to a -record'd fixture
// (fixtures/<case>.provenance.json) — the fixture itself (fixtures/<case>.json) keeps
// its existing shape (Eval 2's input contract) so nothing downstream needs to change;
// this file answers "which model and prompt produced this fixture" for whoever reads
// it later.
type fixtureProvenance struct {
	Model                string               `json:"model"`
	Prompt               provenance.PromptRef `json:"prompt"`
	RunID                string               `json:"run_id"`
	ProcessedInputSHA256 string               `json:"processed_input_sha256"`
}

func splitCSV(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// writeExtractReport writes EXTRACT.md: one table per case, one row per model, every
// named check as its own pass/fail column plus cost/tokens — the same "read this first"
// role SUMMARY.md plays for the Layer-1 prompt evals.
func writeExtractReport(path string, results []extractRunResult) error {
	byCase := map[string][]extractRunResult{}
	var caseOrder []string
	for _, r := range results {
		if _, seen := byCase[r.CaseID]; !seen {
			caseOrder = append(caseOrder, r.CaseID)
		}
		byCase[r.CaseID] = append(byCase[r.CaseID], r)
	}
	sort.Strings(caseOrder)

	var b strings.Builder
	fmt.Fprintf(&b, "# Extraction eval — %s\n\n", time.Now().Format("2006-01-02 15:04"))
	fmt.Fprintf(&b, "Eval 1: file -> extracted information. Every check is deterministic (no LLM judge). Raw per-(case,model,prompt) outputs are saved alongside this file in `extract_outputs/`.\n\n")

	fmt.Fprintf(&b, "%s\n", buildExtractAggregate(results))

	for _, caseID := range caseOrder {
		runs := byCase[caseID]
		fmt.Fprintf(&b, "## %s\n\n", caseID)
		fmt.Fprintf(&b, "| Model | Prompt | Result | Cost | Details |\n|---|---|---|---|---|\n")
		for _, r := range runs {
			cost := formatCost(r.Cost, r.CostBasis)
			prompt := r.Prompt.String()
			switch {
			case r.Error != "":
				fmt.Fprintf(&b, "| %s | %s | ERROR | — | %s |\n", r.Model, prompt, escapeMD(r.Error))
			case r.ParseError != "":
				fmt.Fprintf(&b, "| %s | %s | PARSE FAILED | %s | %s |\n", r.Model, prompt, cost, escapeMD(r.ParseError))
			default:
				fmt.Fprintf(&b, "| %s | %s | %s | %s | %s |\n", r.Model, prompt, passFailSummary(r.Checks), cost, checkDetails(r.Checks))
			}
		}
		b.WriteString("\n")

		for _, r := range runs {
			if r.Error != "" || r.ParseError != "" {
				continue
			}
			var failing []CheckResult
			for _, c := range r.Checks {
				if !c.Pass {
					failing = append(failing, c)
				}
			}
			if len(failing) == 0 {
				continue
			}
			// Model AND prompt in the heading — two prompt versions of the same model
			// on the same case would otherwise print two identically-labeled failure
			// blocks with no way to tell which raw output belongs to which prompt.
			fmt.Fprintf(&b, "**%s / %s failures:**\n\n", r.Model, r.Prompt.String())
			for _, c := range failing {
				fmt.Fprintf(&b, "- `%s`: %s\n", c.Name, c.Detail)
			}
			fmt.Fprintf(&b, "\nraw extracted_text:\n```\n%s\n```\n\n", r.Parsed.ExtractedText)
		}
	}

	return os.WriteFile(path, []byte(b.String()), 0o644)
}

// extractAggregateRow is one (prompt, model) group's rolled-up numbers — Phase 0.3 of the
// language/extraction plan: "full pass" alone collapses several independent questions
// (did structured fields classify right? was visible text transcribed completely? did the
// model avoid inventing numbers? what did it cost?) into one cliff-edge metric. Reporting
// them separately is what makes an extract@v1,extract@v2 comparison actually attributable.
type extractAggregateRow struct {
	Prompt, Model                   string
	Total                           int
	FullPass                        int
	FieldPass, FieldTotal           int
	TextContainsPass, TextContainsN int
	NoInventedPass, NoInventedN     int
	RequiredPass, RequiredN         int
	PricedCostSum                   float64
	PricedCostN                     int
	AvgTokens                       int
	Errors, ParseFailures           int
}

// buildExtractAggregate groups results by (prompt, model) — in first-appearance order —
// and renders the rolled-up table described on extractAggregateRow.
func buildExtractAggregate(results []extractRunResult) string {
	var order []string
	byGroup := map[string]*extractAggregateRow{}
	for _, r := range results {
		key := r.Prompt.String() + "|" + r.Model
		row, ok := byGroup[key]
		if !ok {
			row = &extractAggregateRow{Prompt: r.Prompt.String(), Model: r.Model}
			byGroup[key] = row
			order = append(order, key)
		}
		row.Total++
		switch {
		case r.Error != "":
			row.Errors++
			continue
		case r.ParseError != "":
			row.ParseFailures++
			continue
		}
		if allChecksPass(r.Checks) {
			row.FullPass++
		}
		tokens := 0
		for _, c := range r.Checks {
			switch {
			case strings.HasPrefix(c.Name, "field:"):
				row.FieldTotal++
				if c.Pass {
					row.FieldPass++
				}
			case c.Name == "text_contains_all":
				row.TextContainsN++
				if c.Pass {
					row.TextContainsPass++
				}
			case c.Name == "no_invented_numbers":
				row.NoInventedN++
				if c.Pass {
					row.NoInventedPass++
				}
			case c.Name == "required_numbers":
				row.RequiredN++
				if c.Pass {
					row.RequiredPass++
				}
			}
		}
		tokens = r.Usage.PromptTokens + r.Usage.CompletionTokens
		row.AvgTokens += tokens
		if r.CostBasis == CostBasisMeasured {
			row.PricedCostSum += r.Cost
			row.PricedCostN++
		}
	}

	var b strings.Builder
	b.WriteString("## Aggregate by (prompt, model)\n\n")
	b.WriteString("| Prompt | Model | Full pass | Field checks | text_contains_all | no_invented_numbers | required_numbers | Avg cost | Avg tokens |\n")
	b.WriteString("|---|---|---|---|---|---|---|---|---|\n")
	for _, key := range order {
		row := byGroup[key]
		graded := row.Total - row.Errors - row.ParseFailures
		avgTokens := 0
		if graded > 0 {
			avgTokens = row.AvgTokens / graded
		}
		avgCost := "no estimate"
		if row.PricedCostN > 0 {
			avgCost = fmt.Sprintf("$%.5f avg (%d/%d priced)", row.PricedCostSum/float64(row.PricedCostN), row.PricedCostN, graded)
		}
		errNote := ""
		if row.Errors+row.ParseFailures > 0 {
			errNote = fmt.Sprintf(" (%d error/parse-fail)", row.Errors+row.ParseFailures)
		}
		fmt.Fprintf(&b, "| %s | %s | %s%s | %s | %s | %s | %s | %s | %d |\n",
			row.Prompt, row.Model,
			pctFraction(row.FullPass, graded), errNote,
			pctFraction(row.FieldPass, row.FieldTotal),
			pctFraction(row.TextContainsPass, row.TextContainsN),
			pctFraction(row.NoInventedPass, row.NoInventedN),
			pctFraction(row.RequiredPass, row.RequiredN),
			avgCost, avgTokens)
	}
	return b.String()
}

func pctFraction(pass, total int) string {
	if total == 0 {
		return "n/a"
	}
	return fmt.Sprintf("%d/%d (%.0f%%)", pass, total, float64(pass)/float64(total)*100)
}

func checkDetails(checks []CheckResult) string {
	var failing []string
	for _, c := range checks {
		if !c.Pass {
			failing = append(failing, c.Name)
		}
	}
	if len(failing) == 0 {
		return "all checks passed"
	}
	return "failed: " + strings.Join(failing, ", ")
}

func formatCost(cost float64, basis string) string {
	if basis == CostBasisUnknownPricing {
		return "unknown_pricing"
	}
	return fmt.Sprintf("$%.5f", cost)
}

func escapeMD(s string) string {
	return strings.ReplaceAll(s, "|", "\\|")
}

// appendExtractIndex appends one line to runs/INDEX.md, matching the existing Layer-1
// convention of keeping every run discoverable from a single file.
func appendExtractIndex(runDir string, results []extractRunResult) error {
	pass, total := 0, 0
	for _, r := range results {
		if r.Error != "" || r.ParseError != "" {
			total++
			continue
		}
		total++
		if allChecksPass(r.Checks) {
			pass++
		}
	}
	line := fmt.Sprintf("- %s — extract: %d/%d attempts fully passed\n", filepath.Base(runDir), pass, total)
	f, err := os.OpenFile(filepath.Join("runs", "INDEX.md"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(line)
	return err
}
