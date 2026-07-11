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
)

// cmdExtract runs the pass-1 extraction eval: for each case in cases.yaml, downscale its
// image and send it to every selected model, grade the JSON answer against the case's
// requirements, and write a report. This is Eval 1 (file -> extracted information) — the
// deliberately separate, smaller half of evaluating the playground design; Eval 2
// (extracted information -> ai_* draft schema) is a later, separate command that
// consumes this command's -record'd fixtures instead of calling vision models again.
func cmdExtract(args []string) error {
	fs := flag.NewFlagSet("extract", flag.ExitOnError)
	casesPath := fs.String("cases", "extract/cases.yaml", "path to the extraction cases file")
	caseID := fs.String("case", "", "run only this case id (default: every case in cases.yaml)")
	all := fs.Bool("all", false, "run every case (default when neither -case nor -all is given)")
	modelsFlag := fs.String("models", "", "comma-separated model ids to run (default: $EVAL_VISION_MODELS, else every provider in models.yaml)")
	modelsPath := fs.String("models-file", "models.yaml", "path to models.yaml")
	record := fs.Bool("record", false, "write the first fully-passing output per case to extract/fixtures/<case>.json")
	baseURL := fs.String("base-url", "", "override the OpenRouter base URL (default: $EVAL_BASE_URL, else https://openrouter.ai/api/v1)")
	envPath := fs.String("env", ".env", "path to a .env file to load (best-effort; missing file is fine)")
	maxTokens := fs.Int("max-tokens", extractMaxTokens, "completion token budget per call (overrides models.yaml's max_tokens, which is tuned for short chat replies, not full-text transcription)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	_ = all // accepted for symmetry with -case; running with neither already means "every case"

	loadDotEnv(*envPath)

	apiKey := os.Getenv("OPENROUTER_API_KEY")
	if apiKey == "" {
		return fmt.Errorf("extract: OPENROUTER_API_KEY is not set (copy evals/.env.example to evals/.env and fill it in, or export it)")
	}
	base := *baseURL
	if base == "" {
		base = envOrDefault("EVAL_BASE_URL", "https://openrouter.ai/api/v1")
	}

	cf, err := loadExtractCases(*casesPath)
	if err != nil {
		return err
	}
	cases := cf.Cases
	if *caseID != "" {
		cases = filterCases(cases, *caseID)
		if len(cases) == 0 {
			return fmt.Errorf("extract: no case %q in %s", *caseID, *casesPath)
		}
	}
	if len(cases) == 0 {
		return fmt.Errorf("extract: %s has no cases", *casesPath)
	}

	models, err := resolveVisionModels(*modelsFlag, *modelsPath)
	if err != nil {
		return err
	}
	if len(models) == 0 {
		return fmt.Errorf("extract: no models to run (pass -models or set EVAL_VISION_MODELS)")
	}
	// models.yaml's max_tokens is tuned for Layer-1's short customer-chat replies;
	// full-text transcription needs much more headroom, or the model gets cut off
	// mid-JSON (observed: gpt-4o-mini/gemini truncating mid-sentence at exactly 500
	// completion tokens). Override for every provider, uniformly.
	for i := range models {
		models[i].MaxTokens = *maxTokens
	}

	client := newORClient(base, apiKey)
	casesDir := filepath.Dir(*casesPath)

	runID := time.Now().Format("2006-01-02_15-04-05")
	runDir := filepath.Join("runs", runID)
	outputsDir := filepath.Join(runDir, "extract_outputs")
	if err := os.MkdirAll(outputsDir, 0o755); err != nil {
		return err
	}

	var results []extractRunResult
	for _, c := range cases {
		imgPath := filepath.Join(casesDir, c.File)
		imgData, mimetype, err := loadAndScaleImage(imgPath)
		if err != nil {
			return fmt.Errorf("case %s: %w", c.ID, err)
		}

		recorded := false
		for _, m := range models {
			fmt.Printf("extract: case=%s model=%s ... ", c.ID, orModelID(m.ID))
			res := runOneExtraction(context.Background(), client, c, m, imgData, mimetype)
			results = append(results, res)

			outPath := filepath.Join(outputsDir, fmt.Sprintf("%s__%s.json", c.ID, sanitizeModelID(m.ID)))
			if werr := writeJSON(outPath, res); werr != nil {
				return fmt.Errorf("write %s: %w", outPath, werr)
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
					return err
				}
				if err := writeJSON(fixturePath, res.Parsed); err != nil {
					return err
				}
				fmt.Printf("  recorded fixture: %s (model=%s)\n", fixturePath, orModelID(m.ID))
				recorded = true
			}
		}
	}

	reportPath := filepath.Join(runDir, "EXTRACT.md")
	if err := writeExtractReport(reportPath, results); err != nil {
		return err
	}
	fmt.Printf("\nwrote %s\n", reportPath)
	return appendExtractIndex(runDir, results)
}

// parseFailureRetries bounds retries for a parse failure specifically (never for a
// genuine HTTP/network error, which describeImage already surfaces as res.Error).
// Observed repeatedly in practice: some model routes (gemini-2.5-flash via OpenRouter
// most often) occasionally return an abruptly truncated response — cut off mid-word,
// far under the token budget, usually with usage reported as all-zero — that clears up
// on a plain retry. That's an infrastructure blip, not the model's answer, and
// reporting it as a hard failure would misrepresent the model being evaluated.
const parseFailureRetries = 2

func runOneExtraction(ctx context.Context, client *orClient, c ExtractCase, m ModelProvider, imgData []byte, mimetype string) extractRunResult {
	var res extractRunResult
	for attempt := 0; attempt <= parseFailureRetries; attempt++ {
		res = extractRunResult{CaseID: c.ID, Model: orModelID(m.ID)}
		raw, usage, err := client.describeImage(ctx, m, extractSystemPrompt, imgData, mimetype)
		if err != nil {
			res.Error = err.Error()
			return res // a real HTTP/network error is not retried here
		}
		res.Raw = raw
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
	fmt.Fprintf(&b, "Eval 1: file -> extracted information. Every check is deterministic (no LLM judge). Raw per-(case,model) outputs are saved alongside this file in `extract_outputs/`.\n\n")

	for _, caseID := range caseOrder {
		runs := byCase[caseID]
		fmt.Fprintf(&b, "## %s\n\n", caseID)
		fmt.Fprintf(&b, "| Model | Result | Cost | Details |\n|---|---|---|---|\n")
		for _, r := range runs {
			cost := formatCost(r.Cost, r.CostBasis)
			switch {
			case r.Error != "":
				fmt.Fprintf(&b, "| %s | ERROR | — | %s |\n", r.Model, escapeMD(r.Error))
			case r.ParseError != "":
				fmt.Fprintf(&b, "| %s | PARSE FAILED | %s | %s |\n", r.Model, cost, escapeMD(r.ParseError))
			default:
				fmt.Fprintf(&b, "| %s | %s | %s | %s |\n", r.Model, passFailSummary(r.Checks), cost, checkDetails(r.Checks))
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
			fmt.Fprintf(&b, "**%s failures:**\n\n", r.Model)
			for _, c := range failing {
				fmt.Fprintf(&b, "- `%s`: %s\n", c.Name, c.Detail)
			}
			fmt.Fprintf(&b, "\nraw extracted_text:\n```\n%s\n```\n\n", r.Parsed.ExtractedText)
		}
	}

	return os.WriteFile(path, []byte(b.String()), 0o644)
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
