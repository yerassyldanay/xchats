package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func cmdReport(args []string) error {
	fs := flag.NewFlagSet("report", flag.ExitOnError)
	runDir := fs.String("run", "", "path to the run directory")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *runDir == "" {
		return fmt.Errorf("report: -run is required")
	}
	return reportRun(*runDir)
}

func reportRun(runDir string) error {
	judgedFiles, err := filepath.Glob(filepath.Join(runDir, "*.judged.json"))
	if err != nil {
		return err
	}
	if len(judgedFiles) == 0 {
		return fmt.Errorf("no *.judged.json files in %s (did you run judge first?)", runDir)
	}
	sort.Strings(judgedFiles)

	var runs []JudgedRun
	for _, f := range judgedFiles {
		var jr JudgedRun
		if err := readJSON(f, &jr); err != nil {
			return fmt.Errorf("read %s: %w", f, err)
		}
		runs = append(runs, jr)
	}

	summary := buildSummary(filepath.Base(runDir), runs)
	if err := os.WriteFile(filepath.Join(runDir, "SUMMARY.md"), []byte(summary), 0o644); err != nil {
		return err
	}
	contract := buildContractReport(runs)
	if err := os.WriteFile(filepath.Join(runDir, "CONTRACT.md"), []byte(contract), 0o644); err != nil {
		return err
	}
	if err := appendIndexLine(filepath.Dir(runDir), filepath.Base(runDir), runs); err != nil {
		return err
	}
	fmt.Printf("report written: %s/SUMMARY.md, %s/CONTRACT.md\n", runDir, runDir)
	return nil
}

type modelStats struct {
	model                                  string
	total, modelBehaviorPass, contractPass int
	cost                                   float64
	latencySum, tokensSum                  int
}

func buildSummary(runID string, runs []JudgedRun) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Run %s\n\n", runID)
	fmt.Fprintf(&b, "Generated %s. One table per scenario; a scenario's own README/PLAYGROUND.md\n", time.Now().Format("2006-01-02 15:04"))
	b.WriteString("explains what \"model-behavior\" vs \"contract\" pass rate means.\n\n")

	totalCost, totalTokens := 0.0, 0
	for _, run := range runs {
		for _, v := range run.Verdicts {
			totalCost += v.Cost
			totalTokens += v.Tokens
		}
	}
	costUnknown := totalCost == 0 && totalTokens > 0
	if costUnknown {
		b.WriteString("**Cost shows as n/a**: promptfoo has no pricing table for generic " +
			"`openrouter:` provider IDs, so it reports $0 regardless of real spend — this is " +
			"not \"free\", it's \"unmeasured\". Check OpenRouter's own dashboard for actual " +
			"cost. Tokens are real and come straight from the API response.\n\n")
	}

	var allFailures []Verdict
	for _, run := range runs {
		fmt.Fprintf(&b, "## %s\n\n", run.Scenario)

		byModel := map[string]*modelStats{}
		var order []string
		for _, v := range run.Verdicts {
			ms, ok := byModel[v.Model]
			if !ok {
				ms = &modelStats{model: v.Model}
				byModel[v.Model] = ms
				order = append(order, v.Model)
			}
			ms.total++
			if v.ModelBehaviorPass {
				ms.modelBehaviorPass++
			}
			if v.ContractPass {
				ms.contractPass++
			}
			ms.cost += v.Cost
			ms.latencySum += v.LatencyMs
			ms.tokensSum += v.Tokens
			if !v.ModelBehaviorPass || !v.ContractPass {
				allFailures = append(allFailures, v)
			}
		}
		sort.Strings(order)

		b.WriteString("| model | model-behavior pass | contract pass | cost | avg latency | avg tokens |\n")
		b.WriteString("|---|---|---|---|---|---|\n")
		for _, m := range order {
			ms := byModel[m]
			costStr := fmt.Sprintf("$%.4f", ms.cost)
			if costUnknown {
				costStr = "n/a"
			}
			fmt.Fprintf(&b, "| %s | %d/%d (%.0f%%) | %d/%d (%.0f%%) | %s | %dms | %d |\n",
				m, ms.modelBehaviorPass, ms.total, pct(ms.modelBehaviorPass, ms.total),
				ms.contractPass, ms.total, pct(ms.contractPass, ms.total),
				costStr, avg(ms.latencySum, ms.total), avg(ms.tokensSum, ms.total))
		}
		b.WriteString("\n")
	}

	if len(allFailures) > 0 {
		b.WriteString("## Failures (verbatim)\n\n")
		for _, v := range allFailures {
			fmt.Fprintf(&b, "**%s** | %s\n", v.TestID, v.Model)
			fmt.Fprintf(&b, "- message: %s\n", v.Message)
			fmt.Fprintf(&b, "- reason: %s\n", v.Reason)
			fmt.Fprintf(&b, "- raw output: `%s`\n\n", truncate(v.RawOutput, 300))
		}
	}
	return b.String()
}

// buildContractReport is the per-answer detail: what the model actually said, what the
// harness resolved it to, and every fail-closed/media check applied — the evidence behind
// SUMMARY.md's numbers.
func buildContractReport(runs []JudgedRun) string {
	var b strings.Builder
	b.WriteString("# Contract verdicts (per answer)\n\n")
	for _, run := range runs {
		fmt.Fprintf(&b, "## %s\n\n", run.Scenario)
		for _, v := range run.Verdicts {
			fmt.Fprintf(&b, "### %s — %s\n\n", v.TestID, v.Model)
			fmt.Fprintf(&b, "- parse ok: %v\n", v.ParseOK)
			fmt.Fprintf(&b, "- contract fields ok: %v\n", v.ContractFields)
			if len(v.UnknownTokens) > 0 {
				fmt.Fprintf(&b, "- **BLOCKED — unknown token(s):** %s\n", strings.Join(v.UnknownTokens, ", "))
			}
			if v.LeftoverBraces {
				b.WriteString("- **leftover `{{` after injection**\n")
			}
			if len(v.UnknownMedia) > 0 {
				fmt.Fprintf(&b, "- unknown media (dropped by the real product, not blocked — but still counted against model-behavior here): %s\n", strings.Join(v.UnknownMedia, ", "))
			}
			if len(v.InventedDigits) > 0 {
				fmt.Fprintf(&b, "- invented digits: %s\n", strings.Join(v.InventedDigits, ", "))
			}
			if len(v.UnitIssues) > 0 {
				fmt.Fprintf(&b, "- unit/currency issues: %s\n", strings.Join(v.UnitIssues, ", "))
			}
			if !v.MustNotContainPass {
				fmt.Fprintf(&b, "- **escalated but still committed to an invented answer** (forbidden phrase: %q)\n", v.ForbiddenPhrase)
			}
			fmt.Fprintf(&b, "- requires met: %v · media met: %v · escalate met: %v · language met: %v · no-invented-answer met: %v · units ok: %v\n",
				v.RequiresPass, v.MediaPass, v.EscalatePass, v.LanguagePass, v.MustNotContainPass, len(v.UnitIssues) == 0)
			if v.InjectedText != "" {
				fmt.Fprintf(&b, "- injected text: %s\n", v.InjectedText)
			}
			fmt.Fprintf(&b, "- contract pass: **%v** · model-behavior pass: **%v**\n\n", v.ContractPass, v.ModelBehaviorPass)
		}
	}
	return b.String()
}

func appendIndexLine(runsDir, runID string, runs []JudgedRun) error {
	indexPath := filepath.Join(runsDir, "INDEX.md")
	var scenarios []string
	total, modelBehaviorPass, contractPass := 0, 0, 0
	for _, run := range runs {
		scenarios = append(scenarios, run.Scenario)
		for _, v := range run.Verdicts {
			total++
			if v.ModelBehaviorPass {
				modelBehaviorPass++
			}
			if v.ContractPass {
				contractPass++
			}
		}
	}
	line := fmt.Sprintf("- `%s` — %s — model-behavior %d/%d (%.0f%%), contract %d/%d (%.0f%%)\n",
		runID, strings.Join(scenarios, ", "),
		modelBehaviorPass, total, pct(modelBehaviorPass, total),
		contractPass, total, pct(contractPass, total))

	existing, _ := os.ReadFile(indexPath)
	if len(existing) == 0 {
		existing = []byte("# Run index\n\nMost recent first.\n\n")
	}
	// Replace any existing line(s) for this exact run ID rather than appending
	// alongside them — `report` is safe to re-run after re-judging (e.g. once new
	// checks land), and INDEX.md should reflect the latest verdict for a run, not
	// accumulate one stale line per re-run.
	marker := fmt.Sprintf("- `%s`", runID)
	var kept []string
	for _, l := range strings.Split(string(existing), "\n") {
		if strings.HasPrefix(l, marker) {
			continue
		}
		kept = append(kept, l)
	}
	out := strings.TrimRight(strings.Join(kept, "\n"), "\n") + "\n" + line
	return os.WriteFile(indexPath, []byte(out), 0o644)
}

func pct(n, total int) float64 {
	if total == 0 {
		return 0
	}
	return 100 * float64(n) / float64(total)
}

func avg(sum, total int) int {
	if total == 0 {
		return 0
	}
	return sum / total
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
