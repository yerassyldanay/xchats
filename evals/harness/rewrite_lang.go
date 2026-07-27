package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"xchats-evals-harness/internal/evaltext"
	"xchats-evals-harness/internal/kbfixture"
	"xchats-evals-harness/internal/provenance"

	"github.com/yerassyldanay/xchats/backend/aiprompt"
)

// defaultRewriteModel is the pinned cheap model for the opt-in rewrite-lang command
// (2026-07-27 design note): a translation-only task with the source template and target
// language both handed to it explicitly, not a judgment call, so the cheapest capable
// model is the right default — unlike defaultLLMJudgeModel (judge_llm.go), which needs
// situational nuance a flash-lite-class model was found weak at.
const defaultRewriteModel = "openrouter:google/gemini-2.5-flash-lite"

// rewriteTimeout matches llmJudgeTimeout (judge_llm.go): a single rewrite call is a
// similarly small request, not a full customer-response draft.
const rewriteTimeout = 60 * time.Second

// cmdRewriteLang is the SEPARATE, opt-in language safety net over an already-`judge`d
// run (2026-07-27 design note): for every ContractPass row, it compares the TARGET
// language (detectLang on the customer's own message + history — the same pre-call
// router langdetect.go already provides) against the OUTPUT language (classifyText on
// the row's already-injected, customer-facing text — never the raw `{{...}}`-laden
// template, which reads as neither language to a statistical classifier). On a match —
// the overwhelmingly common case — nothing else happens: zero API calls, zero cost.
//
// On a mismatch (including "too short/unclear to classify," treated the same as a
// confirmed disagreement — see rewriteOneVerdict), ONE rewrite call asks a cheap model
// to translate ONLY the reply_text string, with every `{{table.ref.field}}` placeholder
// preserved byte-for-byte; every OTHER field (escalate, media_files_to_send,
// escalation_reason, confidence) is copied verbatim from the original response in Go
// code, never touched by the rewrite model at all — facts, prices, and the escalation
// flag can never be corrupted by translation. The rewritten template is then re-judged
// from scratch (judgeOne/judgeOneSchemaKB against a synthetic row) exactly like any
// other model output: a rewrite that damaged a placeholder fails ContractPass and is
// discarded, the ORIGINAL verdict (mismatch and all) is left completely untouched, and
// LanguageRewriteFailReason records why. A rewrite that passes replaces every graded
// field on the verdict (see LanguageRewriteApplied's doc comment on Verdict) — at most
// ONE rewrite attempt per row, never a retry loop.
//
// Idempotent: a row already evaluated (LanguageRewriteEvaluated) is skipped on a later
// run unless -force is passed — mirrors judge-llm's re-run discipline, minus the
// per-claim cache (a rewrite is a per-ROW decision, not a set of independent claims).
// Plain `judge` has no knowledge of any field this command writes — re-running it
// rebuilds judged.json from scratch and drops every LanguageRewrite* field, exactly
// like it already does for judge-llm's fields (documented there, not a bug).
func cmdRewriteLang(args []string) error {
	fs := flag.NewFlagSet("rewrite-lang", flag.ExitOnError)
	scenarioDir := fs.String("scenario", "", "path to the scenario directory (required)")
	runDir := fs.String("run", "", "path to the run directory containing <scenario>.judged.json (required; run `harness judge` first)")
	rewriteModelID := fs.String("rewrite-model", defaultRewriteModel, "cheap OpenRouter model id used to rewrite a mismatched-language reply_text")
	modelsPath := fs.String("models", "models.yaml", "path to models.yaml (for the rewrite model's cost-estimate pricing only)")
	baseURL := fs.String("base-url", "", "override the OpenRouter base URL (default: $EVAL_BASE_URL, else https://openrouter.ai/api/v1)")
	envPath := fs.String("env", ".env", "path to a .env file to load (best-effort; missing file is fine)")
	force := fs.Bool("force", false, "re-check every row's language even if rewrite-lang already evaluated it")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *scenarioDir == "" || *runDir == "" {
		return fmt.Errorf("rewrite-lang: -scenario and -run are both required")
	}

	loadDotEnv(*envPath)
	apiKey := os.Getenv("OPENROUTER_API_KEY")
	if apiKey == "" {
		return fmt.Errorf("rewrite-lang: OPENROUTER_API_KEY is not set (rewrite-lang makes real, billed model calls on a language mismatch)")
	}
	base := *baseURL
	if base == "" {
		base = envOrDefault("EVAL_BASE_URL", "https://openrouter.ai/api/v1")
	}

	inputs, err := resolveScenarioRunInputs(*scenarioDir, *runDir)
	if err != nil {
		return err
	}
	scenario := inputs.Scenario

	resolvedModelsPath := *modelsPath
	if inputs.SnapshotDir != "" {
		resolvedModelsPath = provenance.SnapshotModelsPath(*runDir, *modelsPath)
	}
	models, err := loadModels(resolvedModelsPath)
	if err != nil {
		return fmt.Errorf("load %s: %w", resolvedModelsPath, err)
	}
	rewriteModel := resolveRewriteModel(models, *rewriteModelID)

	var resolved ResolvedTests
	if err := readJSON(filepath.Join(inputs.GeneratedDir, "resolved_tests.json"), &resolved); err != nil {
		return fmt.Errorf("read resolved_tests.json: %w", err)
	}
	testByID := map[string]TestCase{}
	for _, t := range resolved.Tests {
		testByID[t.ID] = t
	}

	judgedPath := filepath.Join(*runDir, scenario.Name+".judged.json")
	var judged JudgedRun
	if err := readJSON(judgedPath, &judged); err != nil {
		return fmt.Errorf("read %s (run `harness judge` first): %w", judgedPath, err)
	}

	// judgeFn re-judges a synthetic row exactly as judgeScenario would for this
	// scenario's pipeline — resolved once, reused for every mismatched row.
	var judgeFn func(row PromptfooRow, tc TestCase) Verdict
	if scenario.Pipeline == "schema_kb_v1" {
		if inputs.FixturePath == "" {
			return fmt.Errorf("rewrite-lang: scenario %q is schema_kb_v1 but has no resolvable fixture path", scenario.Name)
		}
		kb, err := kbfixture.Load(inputs.FixturePath)
		if err != nil {
			return fmt.Errorf("load fixture %s: %w", inputs.FixturePath, err)
		}
		if kb, err = kbfixture.ApplyLimits(kb, scenario.Limits); err != nil {
			return fmt.Errorf("scenario %q: %w", scenario.Name, err)
		}
		cat, err := aiprompt.BuildCatalog(kb)
		if err != nil {
			return fmt.Errorf("build catalog: %w", err)
		}
		tokenValue := aipromptTokenValues(cat)
		trustedDigits := trustedDigitsFromKB(kb)
		judgeFn = func(row PromptfooRow, tc TestCase) Verdict {
			return judgeOneSchemaKB(tc, row, kb, cat, tokenValue, trustedDigits)
		}
	} else {
		var catalog Catalog
		if err := readJSON(filepath.Join(inputs.GeneratedDir, "catalog.json"), &catalog); err != nil {
			return fmt.Errorf("read catalog.json (did you run render first?): %w", err)
		}
		tokenValue := map[string]string{}
		for _, f := range catalog.Tokens {
			tokenValue[f.Token] = f.Value
		}
		validMedia := map[string]bool{}
		for _, tok := range catalog.MediaTokens {
			validMedia[tok] = true
		}
		judgeFn = func(row PromptfooRow, tc TestCase) Verdict {
			return judgeOne(tc, row, tokenValue, validMedia, catalog.TrustedDigits)
		}
	}

	client := newORClientWithTimeout(base, apiKey, rewriteTimeout)
	ctx := context.Background()

	checked, mismatches, applied, failed := 0, 0, 0, 0
	var totalCost float64
	for i := range judged.Verdicts {
		v := judged.Verdicts[i]
		tc, ok := testByID[v.TestID]
		if !ok {
			continue
		}
		if v.LanguageRewriteEvaluated && !*force {
			continue
		}
		if !v.ContractPass {
			// A failed contract has no valid customer-facing text to check or rewrite —
			// same gate judge-llm applies for the identical reason.
			continue
		}
		checked++

		result := rewriteOneVerdict(ctx, client, rewriteModel, *rewriteModelID, tc, v, func(row PromptfooRow) Verdict {
			return judgeFn(row, tc)
		})
		judged.Verdicts[i] = result

		if result.PreRewriteLanguage != "" {
			mismatches++
			totalCost += result.LanguageRewriteCostUSD
			if result.LanguageRewriteApplied {
				applied++
			} else {
				failed++
			}
		}
	}

	if err := writeJSON(judgedPath, judged); err != nil {
		return err
	}
	fmt.Printf("rewrite-lang %s: %d row(s) checked, %d mismatch(es) found, %d rewrite(s) applied, %d discarded, est. cost $%.4f -> %s\n",
		scenario.Name, checked, mismatches, applied, failed, totalCost, judgedPath)
	return nil
}

// resolveRewriteModel mirrors resolveJudgeModel (judge_llm.go) — looks up id in
// models.yaml for its pricing, falling back to unknown_pricing if absent — but forces a
// larger MaxTokens: judge_llm's 100-token cap is sized for a one-word yes/no
// classification, while a rewritten reply_text can legitimately run to the frame's own
// ~120-word cap, and Cyrillic text tokenizes less efficiently per character than the
// English judge_llm.go was originally sized against.
func resolveRewriteModel(models *ModelsFile, id string) ModelProvider {
	model := ModelProvider{ID: id}
	for _, p := range models.Providers {
		if p.ID == id {
			model = p
			break
		}
	}
	model.ID = id
	model.Temperature = 0
	model.MaxTokens = 500
	return model
}

// rewriteOneVerdict is rewrite-lang's per-row decision, factored out of cmdRewriteLang
// so the mismatch-detection/rewrite/re-judge/merge logic is written exactly once and
// shared by both pipelines (judgeFn is the only thing that differs between them). v is
// passed by value — every return path returns a COMPLETE replacement Verdict, never a
// mutation of the caller's copy, so "did this row change" is always a plain value
// comparison, not an aliasing question.
func rewriteOneVerdict(ctx context.Context, client *orClient, rewriteModel ModelProvider, rewriteModelID string, tc TestCase, v Verdict, judgeFn func(PromptfooRow) Verdict) Verdict {
	v.LanguageRewriteEvaluated = true
	target := detectLang(tc.Message, tc.History)
	v.TargetLanguage = target

	if v.InjectedText == "" {
		// A ContractPass row with no injected text is a degenerate case (e.g. an empty
		// reply_text) — nothing to classify or rewrite; leave it as a clean "checked,
		// no mismatch" row rather than guessing.
		return v
	}

	output, textOK := classifyText(v.InjectedText)
	if textOK && output == target {
		return v // happy path: already consistent, zero cost
	}

	// A mismatch was found — either a confirmed disagreement, or the text was too
	// short/unclear to classify at all (textOK false). Both are treated as "don't trust
	// this without a second look," per the 2026-07-27 design note's own framing of a
	// classifier false alarm as a reason to fire the safety net, not to skip it.
	// "unclear" is a deliberate, non-empty sentinel — distinct from PreRewriteLanguage's
	// zero value, which means "no mismatch was ever found," not "found one classified as
	// unclear."
	preLang := output
	if !textOK {
		preLang = "unclear"
	}
	v.PreRewriteLanguage = preLang

	obj, ok := parseModelJSON(judgedOutput(v))
	if !ok {
		v.LanguageRewriteFailReason = "could not parse the original response to extract reply_text for rewriting"
		return v
	}
	rawReplyText, ok := obj["reply_text"].(string)
	if !ok {
		v.LanguageRewriteFailReason = "original response has no string reply_text to rewrite"
		return v
	}

	rewritten, cost, finishReason, err := rewriteReplyText(ctx, client, rewriteModel, rawReplyText, target)
	v.LanguageRewriteModel = rewriteModelID
	if err != nil {
		v.LanguageRewriteFailReason = "rewrite call failed: " + err.Error()
		return v
	}
	v.LanguageRewriteCostUSD = cost

	// Splice the translated string back into a COPY of the original object — every
	// other key (escalate, media_files_to_send, escalation_reason, confidence) is
	// whatever the original model wrote, byte-for-byte; the rewrite model never sees
	// them and cannot have altered them.
	obj["reply_text"] = rewritten
	obj["reply_language"] = target
	rewrittenJSON, err := json.Marshal(obj)
	if err != nil {
		v.LanguageRewriteFailReason = "could not re-encode the rewritten response: " + err.Error()
		return v
	}

	row := PromptfooRow{}
	row.Provider.ID = v.Model
	row.Response.Output = string(rewrittenJSON)
	row.Response.FinishReason = finishReason

	fresh := judgeFn(row)
	if !fresh.ContractPass {
		// Expected failure mode: the rewrite model altered, dropped, or malformed a
		// {{placeholder}} — caught here exactly like it would be for any model's own
		// mistake. Discard the rewrite; the ORIGINAL verdict (mismatch and all) is
		// returned completely unchanged below.
		v.LanguageRewriteFailReason = "rewritten template failed the contract re-judge: " + fresh.Reason
		return v
	}

	original := v
	merged := fresh
	// Everything below describes the ORIGINAL provider call, not the rewrite — preserve
	// it rather than let the synthetic re-judge zero it out (see
	// Verdict.LanguageRewriteApplied's doc comment for the full list and why).
	merged.RawOutput = original.RawOutput
	merged.Cost = original.Cost
	merged.LatencyMs = original.LatencyMs
	merged.Tokens = original.Tokens
	merged.TokensIn = original.TokensIn
	merged.TokensOut = original.TokensOut
	merged.CostEstimateUSD = original.CostEstimateUSD
	merged.CostBasis = original.CostBasis
	merged.Retries = original.Retries
	merged.RetryRecovered = original.RetryRecovered
	merged.RetryReason = original.RetryReason
	merged.FirstAttemptParseOK = original.FirstAttemptParseOK
	merged.FirstAttemptContractPass = original.FirstAttemptContractPass
	merged.Reasoning = original.Reasoning

	merged.LanguageRewriteEvaluated = true
	merged.TargetLanguage = target
	merged.PreRewriteLanguage = original.PreRewriteLanguage
	merged.PreRewriteInjectedText = original.InjectedText
	merged.LanguageRewriteApplied = true
	merged.LanguageRewriteModel = rewriteModelID
	merged.LanguageRewriteCostUSD = cost

	return merged
}

// rewriteReplyText sends ONE rewrite call and returns the cleaned translated text, its
// estimated cost, and the call's own finish_reason (threaded through to the synthetic
// re-judge row so a truncated rewrite fails ContractPass exactly like a truncated draft
// would). An empty response after cleaning is treated as a failure — never silently
// substituted in as an empty reply_text.
func rewriteReplyText(ctx context.Context, client *orClient, model ModelProvider, replyText, targetLang string) (rewritten string, cost float64, finishReason string, err error) {
	res, err := client.chatText(ctx, model, buildRewritePrompt(replyText, targetLang))
	if err != nil {
		return "", 0, "", err
	}
	cleaned := cleanRewriteResponse(res.Raw)
	if cleaned == "" {
		return "", 0, res.FinishReason, fmt.Errorf("empty rewrite response")
	}
	cost, _ = estimateCost(model, res.Usage)
	return cleaned, cost, res.FinishReason, nil
}

// buildRewritePrompt is the one rewrite instruction every call uses — Russian prompt
// text regardless of source/target language (matching buildLLMJudgePrompt's own
// convention), demanding the placeholder-preservation guarantee explicitly rather than
// leaving it implicit.
func buildRewritePrompt(replyText, targetLanguage string) string {
	langName := "русский"
	if targetLanguage == "kk" {
		langName = "казахский"
	}
	return fmt.Sprintf(`Перепиши следующий текст ответа ассистента интернет-магазина клиенту ПОЛНОСТЬЮ на %s язык, сохранив исходный смысл и тон. Текст может содержать плейсхолдеры вида {{table.ref.field}} (двойные фигурные скобки, внутри — слова через точки) — каждый такой плейсхолдер скопируй В ТОЧНОСТИ, побуквенно, на том же месте по смыслу предложения; ничего в нём не меняй, не переводи, не удаляй и не добавляй новых плейсхолдеров. Не пиши ничего, кроме самого переписанного текста целиком — без кавычек, без markdown, без пояснений до или после.

Текст:
%s`, langName, replyText)
}

// cleanRewriteResponse strips markdown fences (same tolerance as parseModelJSON, via
// evaltext.StripFences, even though this response isn't JSON — a model wrapping plain
// text in a code block is the same habit either way) and a single matching pair of
// surrounding quote marks a model adds despite the prompt's explicit instruction not
// to — confirmed necessary against real judge-llm-adjacent runs, where models routinely
// quote-wrap a "just the text" instruction.
func cleanRewriteResponse(raw string) string {
	cleaned := strings.TrimSpace(evaltext.StripFences(raw))
	if len(cleaned) < 2 {
		return cleaned
	}
	pairs := [][2]rune{{'"', '"'}, {'«', '»'}, {'“', '”'}}
	runes := []rune(cleaned)
	first, last := runes[0], runes[len(runes)-1]
	for _, p := range pairs {
		if first == p[0] && last == p[1] {
			return strings.TrimSpace(string(runes[1 : len(runes)-1]))
		}
	}
	return cleaned
}
