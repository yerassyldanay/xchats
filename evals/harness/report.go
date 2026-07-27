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
	fmt.Printf("report written: %s/SUMMARY.md, %s/CONTRACT.md\n", runDir, runDir)

	// Best-effort: a broken HTML viewer must never turn a successful report into a
	// failed command. Shared by cmdReport (standalone `harness report`) and cmdRun
	// (which calls reportRun directly), so both entry points get the viewer for free.
	writeRunHTMLBestEffort(runDir)

	return nil
}

type modelStats struct {
	model string
	// The four SEPARATE result dimensions (never averaged together): parseOK is the
	// final-JSON extraction/parse result, contractPass the full operational contract,
	// modelBehaviorPass the deterministic code-based behavior checks, and the llm*
	// fields below the optional LLM-as-judge dimension (reported "not run" when
	// judge-llm never executed — never as 0%).
	total, parseOK, modelBehaviorPass, contractPass int
	costEstimate                                    float64
	measuredN, borrowedN, unpricedN        int
	unknownPricingN                        int
	tokensInSum, tokensOutSum              int
	latencySum, tokensSum                  int
	// retried/retryRecovered track retry.go's per-row Retries/RetryRecovered — see
	// formatRetryCell's doc comment for why these are reported as their OWN line rather
	// than folded into modelBehaviorPass/contractPass: a recovered retry must never look
	// identical to a clean first-attempt pass.
	retried, retryRecovered int
	// mediaNotFoundRetried/mediaNotFoundRecovered pool the SUBSET of retried/
	// retryRecovered above whose RetryReason is specifically "media_not_found" (2026-07
	// consolidation) — see formatMediaNotFoundRetryCell's doc comment for why this is
	// reported as its own line rather than folded into the general retry line.
	mediaNotFoundRetried, mediaNotFoundRecovered int
	// firstAttemptParseOK/firstAttemptContractPass pool Verdict.FirstAttemptParseOK/
	// FirstAttemptContractPass across EVERY row (whether or not it was ever retried —
	// unlike the old firstAttemptParseOK counter this replaces, which only counted rows
	// that never needed a retry at all). This is the true first-shot rate: for a row
	// that WAS retried, it reflects attempt 0's own outcome, not the final/selected one.
	firstAttemptParseOK, firstAttemptContractPass int
	// llmChecked/llmPass/llmUnverified/llmCost pool the OPTIONAL judge-llm dimension
	// (see formatLLMCheckCell's doc comment) — zero for every run judge-llm never
	// touched, same "absent means never run, not zero" discipline as retried above.
	llmChecked, llmPass, llmUnverified int
	llmCost                            float64
	// stockChecked/stockPass/stockUnverified/stockCost pool the auto-generated
	// stock-correctness dimension (2026-07 consolidation) — see
	// formatStockCheckCell's doc comment. stockClassCount tallies each
	// classification the judge actually returned (in_stock/out_of_stock/unclear/
	// contradictory), independent of whether it matched ground truth, so a report
	// can show the classification DISTRIBUTION, not just a pass/fail rate.
	stockChecked, stockPass, stockUnverified int
	stockCost                                float64
	stockClassCount                          map[string]int
}

// formatRetryCell reports retry.go's effect on this model's rows, or "" when this run
// never touched the retry path at all (every existing SUMMARY.md before this feature
// existed had no such row — an all-zero ms.retried must not print a misleading "0
// retried" line for a run that simply predates retries).
func formatRetryCell(ms *modelStats) string {
	if ms.retried == 0 {
		return ""
	}
	return fmt.Sprintf("retried %d, recovered %d", ms.retried, ms.retryRecovered)
}

// formatMediaNotFoundRetryCell reports the media-not-found-specific SUBSET of
// retry.go's effect (see formatRetryCell for the general line) — "" when this run
// never retried a row for that specific reason (opt-in: retry-media is off by default
// for cmdRun, see run.go's -retry-media flag; `harness retry` always retries whatever
// candidates it finds, labeled the same way). "recovered" here means first-shot failed
// and the SELECTED (post-retry) attempt succeeded — spelled out since a row can be
// retried without being recovered (the retry call may fail, or still not resolve).
func formatMediaNotFoundRetryCell(ms *modelStats) string {
	if ms.mediaNotFoundRetried == 0 {
		return ""
	}
	return fmt.Sprintf("%d retried for missing media, %d recovered (first-shot failed, selected succeeded after retry)",
		ms.mediaNotFoundRetried, ms.mediaNotFoundRecovered)
}

// formatFirstShotCell reports the true first-shot contract-pass rate — pooled from
// Verdict.FirstAttemptContractPass across every row, so a retried row's ORIGINAL
// attempt counts on its own terms, not the outcome after retry.go's correction.
// Printed next to (never instead of) the main table's post-retry "contract pass"
// column, per the plan's split-reporting requirement: strict-schema families may show a
// materially lower first-shot number than their final one, and that gap is signal (how
// often retry.go's correction was actually needed), not noise to hide.
// formatLLMCheckCell reports the OPTIONAL judge-llm dimension's effect on this model's
// rows, or "" when this run never ran judge-llm at all — same "absent, not zero" doctrine
// as formatRetryCell. Unverified rows are called out separately from the pass count
// because an unverified claim is neither a confirmed pass nor a confirmed fail (the judge
// call itself failed or was unparseable) — folding it silently into either would misreport
// what judge-llm actually established.
func formatLLMCheckCell(ms *modelStats) string {
	if ms.llmChecked == 0 {
		return ""
	}
	cell := fmt.Sprintf("%d/%d pass (%.0f%%)", ms.llmPass, ms.llmChecked, pct(ms.llmPass, ms.llmChecked))
	if ms.llmUnverified > 0 {
		cell += fmt.Sprintf(", %d unverified", ms.llmUnverified)
	}
	cell += fmt.Sprintf(", ~$%.4f", ms.llmCost)
	return cell
}

// formatStockCheckCell reports the auto-generated stock-correctness dimension's effect
// on this model's rows, or "" when this run never evaluated any StockCheckRef test at
// all — same "absent, not zero" doctrine as formatRetryCell/formatLLMCheckCell. Shows
// the classification distribution alongside the pass rate so a report can distinguish
// "the judge is unsure" (many unclear/contradictory) from "the judge is confident but
// wrong" (confident classifications that still miss ground truth).
func formatStockCheckCell(ms *modelStats) string {
	if ms.stockChecked == 0 {
		return ""
	}
	cell := fmt.Sprintf("%d/%d pass (%.0f%%)", ms.stockPass, ms.stockChecked, pct(ms.stockPass, ms.stockChecked))
	if ms.stockUnverified > 0 {
		cell += fmt.Sprintf(", %d unverified", ms.stockUnverified)
	}
	var classParts []string
	for _, class := range []string{"in_stock", "out_of_stock", "unclear", "contradictory"} {
		if n := ms.stockClassCount[class]; n > 0 {
			classParts = append(classParts, fmt.Sprintf("%s=%d", class, n))
		}
	}
	if len(classParts) > 0 {
		cell += " (" + strings.Join(classParts, ", ") + ")"
	}
	cell += fmt.Sprintf(", ~$%.4f", ms.stockCost)
	return cell
}

func formatFirstShotCell(ms *modelStats) string {
	return fmt.Sprintf("%d/%d (%.0f%%)", ms.firstAttemptContractPass, ms.total, pct(ms.firstAttemptContractPass, ms.total))
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
	b.WriteString("explains what \"model-behavior\" vs \"contract\" pass rate means.\n")
	b.WriteString("Four result types are reported separately and never averaged together:\n")
	b.WriteString("final-JSON parse/extraction, operational contract, deterministic code-based\n")
	b.WriteString("behavior checks (the \"model-behavior\" column — computed by harness code, not by\n")
	b.WriteString("an LLM), and the optional LLM-as-judge dimension (its own line per scenario;\n")
	b.WriteString("\"not run\" when judge-llm never executed).\n\n")

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
			if v.ParseOK {
				ms.parseOK++
			}
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
			if v.Retries > 0 {
				ms.retried++
				if v.RetryRecovered {
					ms.retryRecovered++
				}
				if v.RetryReason == string(retryReasonMediaNotFound) {
					ms.mediaNotFoundRetried++
					if v.RetryRecovered {
						ms.mediaNotFoundRecovered++
					}
				}
			}
			if v.FirstAttemptParseOK {
				ms.firstAttemptParseOK++
			}
			if v.FirstAttemptContractPass {
				ms.firstAttemptContractPass++
			}
			if v.LLMCheckEvaluated {
				ms.llmCost += v.LLMJudgeCostUSD
				for _, r := range v.LLMChecks {
					ms.llmChecked++
					switch {
					case r.Unverified:
						ms.llmUnverified++
					case r.Pass:
						ms.llmPass++
					}
				}
			}
			if v.StockLLMEvaluated {
				ms.stockCost += v.LLMStockCostUSD
				if ms.stockClassCount == nil {
					ms.stockClassCount = map[string]int{}
				}
				for _, r := range v.StockLLMChecks {
					ms.stockChecked++
					if r.Classification != "" {
						ms.stockClassCount[r.Classification]++
					}
					switch {
					case r.Unverified:
						ms.stockUnverified++
					case r.Pass:
						ms.stockPass++
					}
				}
			}
			if !v.ModelBehaviorPass || !v.ContractPass {
				allFailures = append(allFailures, v)
			}
		}
		sort.Strings(order)

		// Column semantics, kept deliberately separate (never averaged into one
		// number): "parse" is the final-JSON extraction/parse result alone;
		// "contract pass" the full operational contract; "model-behavior pass" the
		// DETERMINISTIC code-based checks (requires/media/escalate/language/...),
		// computed by this harness's own code — never an LLM judgment. The optional
		// LLM-as-judge dimension is reported in its own line below the table.
		b.WriteString("| model | parse | model-behavior pass (deterministic) | 95% CI (Wilson, pooled) | contract pass (final) | contract pass (first shot) | est. cost | avg latency | avg tokens | prompt share |\n")
		b.WriteString("|---|---|---|---|---|---|---|---|---|---|\n")
		for _, m := range order {
			ms := byModel[m]
			promptShare := "n/a"
			if total := ms.tokensInSum + ms.tokensOutSum; total > 0 {
				promptShare = fmt.Sprintf("%.0f%%", 100*float64(ms.tokensInSum)/float64(total))
			}
			ciLo, ciHi := wilsonInterval(ms.modelBehaviorPass, ms.total)
			fmt.Fprintf(&b, "| %s | %d/%d (%.0f%%) | %d/%d (%.0f%%) | [%.0f%%, %.0f%%] | %d/%d (%.0f%%) | %s | %s | %s | %d | %s |\n",
				m, ms.parseOK, ms.total, pct(ms.parseOK, ms.total),
				ms.modelBehaviorPass, ms.total, pct(ms.modelBehaviorPass, ms.total),
				ciLo*100, ciHi*100,
				ms.contractPass, ms.total, pct(ms.contractPass, ms.total),
				formatFirstShotCell(ms),
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

		// Retry line (only for models this run actually retried — see
		// formatRetryCell's doc comment): printed SEPARATELY from the table above, not
		// folded into model-behavior/contract pass, so a recovered retry never reads
		// identically to a clean first-attempt pass in the headline numbers.
		var retryLines []string
		for _, m := range order {
			if cell := formatRetryCell(byModel[m]); cell != "" {
				retryLines = append(retryLines, fmt.Sprintf("- %s: %s", m, cell))
			}
		}
		if len(retryLines) > 0 {
			b.WriteString("Retries (retry.go — see each row's `attempts` in .judged.json for the full history):\n\n")
			for _, line := range retryLines {
				b.WriteString(line)
				b.WriteString("\n")
			}
			b.WriteString("\n")
		}

		// Media-not-found retry line (opt-in — see run.go's -retry-media flag and
		// retry.go's retryReasonMediaNotFound): a SUBSET of the general retry line
		// above, printed separately so "how many retries were specifically about
		// missing media" is visible without cross-referencing judged.json by hand.
		var mediaRetryLines []string
		for _, m := range order {
			if cell := formatMediaNotFoundRetryCell(byModel[m]); cell != "" {
				mediaRetryLines = append(mediaRetryLines, fmt.Sprintf("- %s: %s", m, cell))
			}
		}
		if len(mediaRetryLines) > 0 {
			b.WriteString("Media-not-found retries (retry.go's media_not_found reason — see each row's `retry_reason` in .judged.json):\n\n")
			for _, line := range mediaRetryLines {
				b.WriteString(line)
				b.WriteString("\n")
			}
			b.WriteString("\n")
		}

		// LLM checks line (only for models this run actually ran judge-llm against — see
		// formatLLMCheckCell's doc comment): an OPTIONAL, separately-reported dimension,
		// never folded into model-behavior/contract pass above.
		var llmLines []string
		for _, m := range order {
			if cell := formatLLMCheckCell(byModel[m]); cell != "" {
				llmLines = append(llmLines, fmt.Sprintf("- %s: %s", m, cell))
			}
		}
		if len(llmLines) > 0 {
			b.WriteString("LLM checks (judge-llm — optional semantic claims, see each row's `llm_checks` in .judged.json):\n\n")
			for _, line := range llmLines {
				b.WriteString(line)
				b.WriteString("\n")
			}
			b.WriteString("\n")
		} else {
			// Explicit "not run", never an implied 0%: every number in the table above
			// is deterministic code-based checking; no LLM judged anything in this run.
			b.WriteString("LLM-as-judge (judge-llm): not run. All pass rates above are deterministic code-based checks.\n\n")
		}

		// LLM stock line (only for models this run actually evaluated a StockCheckRef
		// test for — see formatStockCheckCell's doc comment): the auto-generated
		// semantic stock-correctness dimension, kept SEPARATE from both the
		// deterministic table above and the hand-declared LLM checks line — never
		// folded into ContractPass/ModelBehaviorPass/LLMJudgePass.
		var stockLines []string
		for _, m := range order {
			if cell := formatStockCheckCell(byModel[m]); cell != "" {
				stockLines = append(stockLines, fmt.Sprintf("- %s: %s", m, cell))
			}
		}
		if len(stockLines) > 0 {
			b.WriteString("LLM stock check (judge-llm — auto-generated semantic classification, see each row's `stock_llm_checks` in .judged.json):\n\n")
			for _, line := range stockLines {
				b.WriteString(line)
				b.WriteString("\n")
			}
			b.WriteString("\n")
		} else {
			b.WriteString("LLM stock check (judge-llm): not run.\n\n")
		}

		if isScaleScenario(run.Scenario) {
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
	b.WriteString("## Scale comparison\n\n")
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

// scaleScenarioPrefixes are the scenario-name families whose per-model stats feed
// buildScaleComparison — the original shop-scale-N family and the shop-kb-v1 family's
// size variants (shop-kb-v1-30, shop-kb-v1-scale-60, shop-kb-v1-scale-100). Naming is
// data, not a hardcoded single prefix, so a later family can opt in by adding its own
// prefix here without another shape change.
var scaleScenarioPrefixes = []string{"shop-scale-", "shop-kb-v1-"}

func isScaleScenario(name string) bool {
	for _, p := range scaleScenarioPrefixes {
		if strings.HasPrefix(name, p) {
			return true
		}
	}
	return false
}

// scaleSizeOf pulls the TRAILING number off a scenario name for sort order (plain
// string sort would put "shop-scale-10" before "shop-scale-20" only by luck of digit
// count matching — this makes it correct regardless of how many sizes exist).
// Deliberately walks from the END and stops at the first non-digit: a naive
// left-to-right digit-concatenation over the WHOLE string (the previous
// implementation) corrupts the moment any earlier digit exists anywhere in the name —
// e.g. a "-v1-" version marker — by shifting it in ahead of the real size digits
// ("shop-kb-v1-scale-100" must parse as 100, not 1100).
func scaleSizeOf(scenario string) int {
	runes := []rune(scenario)
	end := len(runes)
	for end > 0 && runes[end-1] >= '0' && runes[end-1] <= '9' {
		end--
	}
	n := 0
	place := 1
	for i := len(runes) - 1; i >= end; i-- {
		n += int(runes[i]-'0') * place
		place *= 10
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
			if v.ExtractionMethod != "" {
				fmt.Fprintf(&b, "- final answer extraction: %s", v.ExtractionMethod)
				if v.NonFinalOutput != "" {
					b.WriteString(" (non-final reasoning/preamble text stored separately, not scored)")
				}
				b.WriteString("\n")
			}
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
			if v.MediaCountEvaluated && v.TooManyMedia {
				fmt.Fprintf(&b, "- **too many attachments:** %d entries (frame cap: 2)\n", v.MediaCount)
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
				// Generic wording — this check is no longer escalation-only (e.g. a test
				// forbidding "claim to attach a video that doesn't exist" has no escalate
				// expectation at all).
				fmt.Fprintf(&b, "- **reply_text contains a forbidden phrase:** %q\n", v.ForbiddenPhrase)
			}
			if v.EscalateTextConsistencyEvaluated && !v.EscalateTextConsistencyPass {
				fmt.Fprintf(&b, "- **reply deflects to a manager while escalate=false:** %q\n", v.DeflectionPhrase)
			}
			mediaCountCell := "n/a (verdict predates this check)"
			if v.MediaCountEvaluated {
				mediaCountCell = fmt.Sprintf("%v", !v.TooManyMedia)
			}
			escalateTextCell := "n/a (verdict predates this check)"
			if v.EscalateTextConsistencyEvaluated {
				escalateTextCell = fmt.Sprintf("%v", v.EscalateTextConsistencyPass)
			}
			fmt.Fprintf(&b, "- requires met: %v · media met: %v · escalate met: %v · escalate/text consistent: %s · language met: %v · no-invented-answer met: %v · units ok: %v · media count ok: %s\n",
				v.RequiresPass, v.MediaPass, v.EscalatePass, escalateTextCell, v.LanguagePass, v.MustNotContainPass, len(v.UnitIssues) == 0, mediaCountCell)
			if v.InjectedText != "" {
				fmt.Fprintf(&b, "- injected text: %s\n", v.InjectedText)
			}
			fmt.Fprintf(&b, "- injection clean (no brace survived, whether blocked or not): %v\n", !v.LeftoverBraces)
			fmt.Fprintf(&b, "- cost basis: %s", v.CostBasis)
			if v.CostBasis == CostBasisMeasured || v.CostBasis == CostBasisCachedBorrowed {
				fmt.Fprintf(&b, " (%d in / %d out tokens, est. $%.6f)", v.TokensIn, v.TokensOut, v.CostEstimateUSD)
			}
			b.WriteString("\n")
			llmCell := "not run"
			if v.LLMCheckEvaluated {
				llmCell = "fail"
				if v.LLMJudgePass != nil && *v.LLMJudgePass {
					llmCell = "pass"
				}
			}
			fmt.Fprintf(&b, "- contract pass: **%v** · model-behavior pass (deterministic): **%v** · llm judge: %s\n\n", v.ContractPass, v.ModelBehaviorPass, llmCell)
		}
	}
	return b.String()
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
