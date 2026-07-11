package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"xchats-evals-harness/internal/provenance"
)

func cmdReport(args []string) error {
	fs := flag.NewFlagSet("report", flag.ExitOnError)
	runDir := fs.String("run", "", "path to the run directory")
	modelsPath := fs.String("models", "models.yaml", "path to models.yaml (for pricing provenance in the report header)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *runDir == "" {
		return fmt.Errorf("report: -run is required")
	}
	return reportRun(*runDir, *modelsPath)
}

func reportRun(runDir, modelsPath string) error {
	runs, _, err := loadJudgedRuns(runDir)
	if err != nil {
		return err
	}
	if len(runs) == 0 {
		return fmt.Errorf("no *.judged.json files in %s (did you run judge first?)", runDir)
	}

	// Prefer this run's own snapshotted models.yaml (pins the pricing disclaimer to
	// what was in force at run time) over the live file; falls back for legacy runs.
	resolvedModelsPath := provenance.SnapshotModelsPath(runDir, modelsPath)
	models, err := loadModels(resolvedModelsPath)
	if err != nil {
		return fmt.Errorf("load %s: %w", resolvedModelsPath, err)
	}

	summary := buildSummary(filepath.Base(runDir), runs, models)
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

	// Best-effort: a broken HTML viewer must never turn a successful report into a
	// failed command. Shared by cmdReport (standalone `harness report`) and cmdRun
	// (which calls reportRun directly), so both entry points get the viewer for free.
	writeRunHTMLBestEffort(runDir)

	return nil
}

type modelStats struct {
	model                                  string
	total, modelBehaviorPass, contractPass int
	costEstimate                           float64
	measuredN, borrowedN, unpricedN        int
	unknownPricingN                        int
	tokensInSum, tokensOutSum              int
	latencySum, tokensSum                  int
}

// formatCostCell turns one model's cost-basis tally into a single honest table cell —
// never a bare dollar figure. See CostBasis's doc comment in judge.go for what each basis
// means; this is where those meanings become human-readable.
func formatCostCell(ms *modelStats) string {
	priced := ms.measuredN + ms.borrowedN
	if priced == 0 {
		if ms.unknownPricingN == ms.total {
			return "unknown pricing"
		}
		return "unpriceable (cached, no split to borrow)"
	}
	var basisParts []string
	if ms.measuredN > 0 {
		basisParts = append(basisParts, fmt.Sprintf("%d measured", ms.measuredN))
	}
	if ms.borrowedN > 0 {
		basisParts = append(basisParts, fmt.Sprintf("%d cached-borrowed", ms.borrowedN))
	}
	if ms.unpricedN > 0 {
		basisParts = append(basisParts, fmt.Sprintf("%d unpriceable", ms.unpricedN))
	}
	if ms.unknownPricingN > 0 {
		basisParts = append(basisParts, fmt.Sprintf("%d unknown-pricing", ms.unknownPricingN))
	}
	return fmt.Sprintf("$%.4f est. (%s)", ms.costEstimate, strings.Join(basisParts, ", "))
}

func formatLatencyCell(avgMs int) string {
	if avgMs < 50 {
		return fmt.Sprintf("%dms (cached — not meaningful)", avgMs)
	}
	return fmt.Sprintf("%dms", avgMs)
}

func buildSummary(runID string, runs []JudgedRun, models *ModelsFile) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Run %s\n\n", runID)
	fmt.Fprintf(&b, "Generated %s. One table per scenario; a scenario's own README/PLAYGROUND.md\n", time.Now().Format("2006-01-02 15:04"))
	b.WriteString("explains what \"model-behavior\" vs \"contract\" pass rate means.\n\n")

	fmt.Fprintf(&b, "**Cost is an ESTIMATE, not real spend.** Computed from models.yaml's "+
		"hand-maintained prices (source: %s, checked %s) × token counts from the API "+
		"response. promptfoo has no pricing table of its own for generic `openrouter:` "+
		"provider IDs — this report fills that gap, at the accuracy of whoever last updated "+
		"models.yaml. Check OpenRouter's own dashboard for real spend. A cached row (repeat "+
		"run, same question) reports zero prompt/completion tokens from promptfoo, so its "+
		"cost is estimated by BORROWING the split from a fresh row in the same run for the "+
		"same (model, test) if one exists — otherwise it's marked unpriceable, not free. "+
		"Every model/cost cell below says which of these applied.\n\n",
		models.PricingSource, models.PricingCheckedAt)

	// scaleStats collects shop-scale-N scenarios' per-model stats as they're computed below,
	// so the scale-comparison table after the main loop doesn't recompute anything.
	scaleStats := map[string]map[string]*modelStats{}

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
			switch v.CostBasis {
			case CostBasisMeasured:
				ms.measuredN++
				ms.costEstimate += v.CostEstimateUSD
			case CostBasisCachedBorrowed:
				ms.borrowedN++
				ms.costEstimate += v.CostEstimateUSD
			case CostBasisCachedUnpriced:
				ms.unpricedN++
			case CostBasisUnknownPricing:
				ms.unknownPricingN++
			}
			ms.tokensInSum += v.TokensIn
			ms.tokensOutSum += v.TokensOut
			ms.latencySum += v.LatencyMs
			ms.tokensSum += v.Tokens
			if !v.ModelBehaviorPass || !v.ContractPass {
				allFailures = append(allFailures, v)
			}
		}
		sort.Strings(order)

		b.WriteString("| model | model-behavior pass | 95% CI (Wilson, pooled) | contract pass | est. cost | avg latency | avg tokens | prompt share |\n")
		b.WriteString("|---|---|---|---|---|---|---|---|\n")
		for _, m := range order {
			ms := byModel[m]
			promptShare := "n/a"
			if total := ms.tokensInSum + ms.tokensOutSum; total > 0 {
				promptShare = fmt.Sprintf("%.0f%%", 100*float64(ms.tokensInSum)/float64(total))
			}
			ciLo, ciHi := wilsonInterval(ms.modelBehaviorPass, ms.total)
			fmt.Fprintf(&b, "| %s | %d/%d (%.0f%%) | [%.0f%%, %.0f%%] | %d/%d (%.0f%%) | %s | %s | %d | %s |\n",
				m, ms.modelBehaviorPass, ms.total, pct(ms.modelBehaviorPass, ms.total),
				ciLo*100, ciHi*100,
				ms.contractPass, ms.total, pct(ms.contractPass, ms.total),
				formatCostCell(ms), formatLatencyCell(avg(ms.latencySum, ms.total)), avg(ms.tokensSum, ms.total),
				promptShare)
		}
		// The CI column is a POOLED interval over every output this (scenario, model)
		// row's total already counts — never a per-intent interval (a handful of
		// repetitions per individual customer intent is enough to notice a
		// systematically-broken intent, not enough for its own confidence interval; see
		// wilson.go). Rows are already one-per-model, never merged across models — a
		// specific prompt+model combination stays the experimental unit throughout, here
		// same as everywhere else in this report.
		b.WriteString("\n")

		if strings.HasPrefix(run.Scenario, "shop-scale-") {
			scaleStats[run.Scenario] = byModel
		}
	}

	if table := buildScaleComparison(scaleStats); table != "" {
		b.WriteString(table)
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

// buildScaleComparison reads directly off the SAME modelStats buildSummary already computed
// for each shop-scale-N scenario (no re-parsing of verdicts) and turns them into one
// size-series table per model — model-behavior pass % and avg prompt tokens at each size —
// so "does quality degrade as the catalog grows" and "what does that growth cost in tokens"
// are both answerable from one place instead of eyeballing N separate per-scenario tables.
func buildScaleComparison(scaleStats map[string]map[string]*modelStats) string {
	if len(scaleStats) < 2 {
		return ""
	}
	var scenarios []string
	for name := range scaleStats {
		scenarios = append(scenarios, name)
	}
	sort.Slice(scenarios, func(i, j int) bool {
		return scaleSizeOf(scenarios[i]) < scaleSizeOf(scenarios[j])
	})

	models := map[string]bool{}
	for _, byModel := range scaleStats {
		for m := range byModel {
			models[m] = true
		}
	}
	var modelOrder []string
	for m := range models {
		modelOrder = append(modelOrder, m)
	}
	sort.Strings(modelOrder)

	var b strings.Builder
	b.WriteString("## Scale comparison (shop-scale-N)\n\n")
	b.WriteString("Model-behavior pass % and avg total tokens per answer at each catalog size — the\n")
	b.WriteString("direct answer to \"does answer quality hold up as the product list grows\" and what\n")
	b.WriteString("that growth costs in tokens (avg tokens here is the raw API count, always\n")
	b.WriteString("available regardless of whether this model's cost is priced — unlike the est.\n")
	b.WriteString("cost column above).\n\n")
	b.WriteString("| model |")
	for _, s := range scenarios {
		fmt.Fprintf(&b, " %s (behavior / avg tokens) |", s)
	}
	b.WriteString("\n|---|")
	for range scenarios {
		b.WriteString("---|")
	}
	b.WriteString("\n")
	for _, m := range modelOrder {
		fmt.Fprintf(&b, "| %s |", m)
		for _, s := range scenarios {
			ms, ok := scaleStats[s][m]
			if !ok {
				b.WriteString(" n/a |")
				continue
			}
			fmt.Fprintf(&b, " %.0f%% / %d |", pct(ms.modelBehaviorPass, ms.total), avg(ms.tokensSum, ms.total))
		}
		b.WriteString("\n")
	}
	b.WriteString("\n")
	return b.String()
}

// scaleSizeOf pulls the trailing number off a "shop-scale-N" scenario name for sort order
// (plain string sort would put "shop-scale-10" before "shop-scale-20" only by luck of
// digit count matching — this makes it correct regardless of how many sizes exist).
func scaleSizeOf(scenario string) int {
	n := 0
	for _, r := range scenario {
		if r >= '0' && r <= '9' {
			n = n*10 + int(r-'0')
		}
	}
	return n
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
			if v.Truncated {
				fmt.Fprintf(&b, "- **TRUNCATED — finish_reason=%s** (response cut off before the model finished; contract fails regardless of what parsed)\n", v.FinishReason)
			}
			if v.ReasoningLeak {
				b.WriteString("- **REASONING LEAK — reply_text contains a <think>/<thinking> tag marker** (contract fails; a real customer would have received this)\n")
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
			fmt.Fprintf(&b, "- injection clean (no brace survived, whether blocked or not): %v\n", !v.LeftoverBraces)
			fmt.Fprintf(&b, "- cost basis: %s", v.CostBasis)
			if v.CostBasis == CostBasisMeasured || v.CostBasis == CostBasisCachedBorrowed {
				fmt.Fprintf(&b, " (%d in / %d out tokens, est. $%.6f)", v.TokensIn, v.TokensOut, v.CostEstimateUSD)
			}
			b.WriteString("\n")
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
