package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"xchats-evals-harness/internal/kbfixture"
	"xchats-evals-harness/internal/provenance"

	"github.com/yerassyldanay/xchats/backend/aiprompt"
)

// defaultLLMJudgeModel is the pinned judge model for the opt-in judge-llm command — see
// LLMCheck's doc comment (types.go) for why this check class exists at all. Pinned to one
// exact model id (not "whichever cheap model is around") so verdicts stay comparable
// across runs; override with -judge-model only for deliberate experimentation. Chosen
// over the even-cheaper flash-lite because this repo's own escalation-canary data showed
// flash-lite is the weakest judge of situational nuance among the three models on hand
// (see PROD-READINESS notes) — not a good pick for the examiner's seat.
const defaultLLMJudgeModel = "openrouter:google/gemini-2.5-flash"

// llmJudgeTimeout matches retry.go's retryTimeout rather than extract.go's 90s default —
// a one-claim yes/no judgment is a much smaller call than a full customer-response draft.
const llmJudgeTimeout = 60 * time.Second

// cmdJudgeLLM is the SEPARATE, opt-in semantic-judge pass over an already-`judge`d run:
// for every verdict whose TestCase declares llm_checks AND whose ContractPass is true (a
// failed contract has no valid customer-facing text to judge), it asks the pinned judge
// model one binary yes/no question per claim and writes the result back onto that
// Verdict. The plain `judge` command never reads or writes any LLMCheck field, so running
// or skipping judge-llm never changes ContractPass/ModelBehaviorPass/Reason for any row —
// this is purely an additional, independently reported dimension (TestCase.LLMChecks'
// doc comment explains why: no token or substring list can express "this paraphrased
// refusal actually communicates X").
//
// Idempotent and cheap to re-run: each claim's verdict is cached in judged.json keyed by
// a hash of (message, reply text, claim, judge model id) — an unchanged row is never
// re-billed unless -force is passed or -judge-model changes. Re-running the plain `judge`
// command afterward REBUILDS judged.json from scratch and drops any cached LLM verdicts
// (documented, not a bug: judge.go has no knowledge of this file at all) — cost to
// recompute is a fraction of a cent, so this is accepted rather than engineered around.
func cmdJudgeLLM(args []string) error {
	fs := flag.NewFlagSet("judge-llm", flag.ExitOnError)
	scenarioDir := fs.String("scenario", "", "path to the scenario directory (required)")
	runDir := fs.String("run", "", "path to the run directory containing <scenario>.judged.json (required; run `harness judge` first)")
	judgeModelID := fs.String("judge-model", defaultLLMJudgeModel, "pinned OpenRouter model id used to evaluate TestCase.llm_checks claims")
	modelsPath := fs.String("models", "models.yaml", "path to models.yaml (for the judge model's cost-estimate pricing only)")
	baseURL := fs.String("base-url", "", "override the OpenRouter base URL (default: $EVAL_BASE_URL, else https://openrouter.ai/api/v1)")
	envPath := fs.String("env", ".env", "path to a .env file to load (best-effort; missing file is fine)")
	force := fs.Bool("force", false, "re-judge every llm_checks claim even when a cached verdict already matches")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *scenarioDir == "" || *runDir == "" {
		return fmt.Errorf("judge-llm: -scenario and -run are both required")
	}

	loadDotEnv(*envPath)
	apiKey := os.Getenv("OPENROUTER_API_KEY")
	if apiKey == "" {
		return fmt.Errorf("judge-llm: OPENROUTER_API_KEY is not set (judge-llm makes real, billed model calls)")
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
	judgeModel := resolveJudgeModel(models, *judgeModelID)

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

	// Ground truth for the auto-generated stock-correctness dimension (StockCheckRef)
	// comes from the catalog, schema_kb_v1 only — a legacy fact_tables scenario has no
	// per-product InStock signal this command knows how to read, so kb stays nil and
	// every stockChecksFor call below is a no-op for it.
	var kb *aiprompt.KB
	if scenario.Pipeline == "schema_kb_v1" && inputs.FixturePath != "" {
		loaded, err := kbfixture.Load(inputs.FixturePath)
		if err != nil {
			return fmt.Errorf("load fixture %s: %w", inputs.FixturePath, err)
		}
		if loaded, err = kbfixture.ApplyLimits(loaded, scenario.Limits); err != nil {
			return fmt.Errorf("scenario %q: %w", scenario.Name, err)
		}
		kb = loaded
	}

	client := newORClientWithTimeout(base, apiKey, llmJudgeTimeout)
	ctx := context.Background()

	evaluated, contractFailedSkips, calls := 0, 0, 0
	stockEvaluated, stockCalls := 0, 0
	var totalCost, totalStockCost float64
	for i := range judged.Verdicts {
		v := &judged.Verdicts[i]
		tc, ok := testByID[v.TestID]
		if !ok {
			continue
		}
		hasLLMChecks := len(tc.LLMChecks) > 0
		hasStockCheck := kb != nil && tc.StockCheckRef != ""
		if !hasLLMChecks && !hasStockCheck {
			continue
		}
		if !v.ContractPass {
			// A failed contract has no valid customer-facing text — nothing to judge.
			contractFailedSkips++
			continue
		}

		if hasLLMChecks {
			cached := map[string]LLMCheckResult{}
			for _, r := range v.LLMChecks {
				cached[r.CacheKey] = r
			}

			results := make([]LLMCheckResult, 0, len(tc.LLMChecks))
			passAll := true
			var verdictCost float64
			for _, check := range tc.LLMChecks {
				key := llmCheckCacheKey(tc.Message, v.InjectedText, check.Claim, *judgeModelID)
				if !*force {
					if hit, ok := cached[key]; ok {
						results = append(results, hit)
						if !hit.Pass {
							passAll = false
						}
						continue
					}
				}
				result, cost := judgeLLMClaim(ctx, client, judgeModel, *judgeModelID, tc.Message, v.InjectedText, check)
				calls++
				verdictCost += cost
				results = append(results, result)
				if !result.Pass {
					passAll = false
				}
			}

			v.LLMChecks = results
			v.LLMCheckEvaluated = true
			v.LLMJudgeModel = *judgeModelID
			v.LLMJudgePass = &passAll
			v.LLMJudgeCostUSD = verdictCost
			totalCost += verdictCost
			evaluated++
		}

		if hasStockCheck {
			expected, productName, ok := stockGroundTruth(kb, tc.StockCheckRef)
			if !ok {
				// StockCheckRef names a ref this (possibly size-limited) catalog no
				// longer carries — nothing to judge against, not an error.
				continue
			}
			cachedStock := map[string]StockLLMCheckResult{}
			for _, r := range v.StockLLMChecks {
				cachedStock[r.CacheKey] = r
			}
			key := stockCheckCacheKey(tc.Message, v.InjectedText, tc.StockCheckRef, *judgeModelID)
			var result StockLLMCheckResult
			var cost float64
			if hit, ok := cachedStock[key]; ok && !*force {
				result = hit
			} else {
				result, cost = judgeStockClaim(ctx, client, judgeModel, *judgeModelID, tc.Message, v.InjectedText, tc.StockCheckRef, productName, expected)
				stockCalls++
				totalStockCost += cost
			}
			v.StockLLMChecks = []StockLLMCheckResult{result}
			v.StockLLMEvaluated = true
			v.LLMStockModel = *judgeModelID
			v.LLMStockPass = &result.Pass
			v.LLMStockCostUSD = cost
			stockEvaluated++
		}
	}

	if err := writeJSON(judgedPath, judged); err != nil {
		return err
	}
	fmt.Printf("judge-llm %s: %d verdict(s) evaluated, %d contract-failed skip(s), %d API call(s), est. cost $%.4f; stock: %d evaluated, %d API call(s), est. cost $%.4f -> %s\n",
		scenario.Name, evaluated, contractFailedSkips, calls, totalCost, stockEvaluated, stockCalls, totalStockCost, judgedPath)
	return nil
}

// resolveJudgeModel looks up id in models.yaml for its pricing (falling back to
// unknown_pricing if absent, same discipline as resolveVisionModels) and forces
// deterministic, cheap judging parameters regardless of what that entry configures for
// its OTHER role as a subject model elsewhere in this suite: temperature 0 (a yes/no
// classification, not a creative reply) and a small MaxTokens (the only valid output is
// one short JSON object).
func resolveJudgeModel(models *ModelsFile, id string) ModelProvider {
	judgeModel := ModelProvider{ID: id}
	for _, p := range models.Providers {
		if p.ID == id {
			judgeModel = p
			break
		}
	}
	judgeModel.ID = id
	judgeModel.Temperature = 0
	judgeModel.MaxTokens = 100
	return judgeModel
}

// llmCheckCacheKey identifies one (customer message, reply text, claim, judge model)
// tuple — a re-run with all four unchanged reuses the cached LLMCheckResult instead of
// re-billing. expect is deliberately NOT part of the key: it only affects the LOCAL pass
// comparison after the judge model answers, never what gets sent to the API.
func llmCheckCacheKey(message, replyText, claim, judgeModelID string) string {
	sum := sha256.Sum256([]byte(message + "\x00" + replyText + "\x00" + claim + "\x00" + judgeModelID))
	return hex.EncodeToString(sum[:])
}

// judgeLLMClaim asks the judge model ONE binary question about ONE reply and returns the
// judged result plus its estimated cost (0 for an unverified call — nothing measurable to
// price). Every failure mode (HTTP/network error, unparseable response, a response
// missing the "verdict" boolean) becomes Unverified=true with Pass left false — fail
// closed, never a silent pass standing in for a judge call that didn't actually happen.
func judgeLLMClaim(ctx context.Context, client *orClient, judgeModel ModelProvider, judgeModelID, message, replyText string, check LLMCheck) (LLMCheckResult, float64) {
	result := LLMCheckResult{
		Claim:    check.Claim,
		Expect:   check.Expect,
		CacheKey: llmCheckCacheKey(message, replyText, check.Claim, judgeModelID),
	}
	res, err := client.chatText(ctx, judgeModel, buildLLMJudgePrompt(message, replyText, check.Claim))
	if err != nil {
		result.Unverified = true
		return result, 0
	}
	verdict, ok := parseLLMVerdict(res.Raw)
	if !ok {
		result.Unverified = true
		return result, 0
	}
	result.Verdict = verdict
	result.Pass = verdict == check.Expect
	cost, _ := estimateCost(judgeModel, res.Usage)
	return result, cost
}

// buildLLMJudgePrompt is the one Russian judge prompt every claim uses — the customer
// message and reply text are shown verbatim (not paraphrased or summarized) so the judge
// sees exactly what a human reviewer would, and the instruction demands a single bare
// JSON object so parseLLMVerdict's plain parseModelJSON (fence-tolerant, but not prose-
// tolerant) can read it reliably.
func buildLLMJudgePrompt(message, replyText, claim string) string {
	return fmt.Sprintf(`Ты — независимый проверяющий качества ответов ассистента интернет-магазина. Тебе даны сообщение клиента, черновик ответа ассистента и одно утверждение об этом ответе. Определи, верно ли утверждение — да или нет.

Сообщение клиента:
%s

Черновик ответа:
%s

Утверждение: %s

Ответь СТРОГО одним JSON-объектом без какого-либо текста до или после него, ровно в этом виде: {"verdict": true} или {"verdict": false}.`, message, replyText, claim)
}

// parseLLMVerdict extracts the boolean "verdict" field from the judge model's raw
// output, tolerating markdown fences the same way every other model-output parse in this
// harness does (parseModelJSON). The second return is false for anything that isn't
// valid JSON, isn't an object, or whose "verdict" key isn't a JSON boolean.
func parseLLMVerdict(raw string) (bool, bool) {
	obj, ok := parseModelJSON(raw)
	if !ok {
		return false, false
	}
	v, ok := obj["verdict"].(bool)
	return v, ok
}

// stockGroundTruth looks up ref's current stock state directly from the KB (never
// hand-authored — see TestCase.StockCheckRef's doc comment) — "in_stock" or
// "out_of_stock", plus the product's display name for the judge prompt. ok is false
// when ref names no active product in THIS (possibly size-limited) catalog, e.g. a
// StockCheckRef the current -limits truncation no longer includes.
func stockGroundTruth(kb *aiprompt.KB, ref string) (expected, displayName string, ok bool) {
	for _, p := range kb.Products {
		if p.Ref != ref || p.SalesStatus != "active" {
			continue
		}
		if p.InStock {
			return "in_stock", p.Name, true
		}
		return "out_of_stock", p.Name, true
	}
	return "", "", false
}

// stockCheckCacheKey mirrors llmCheckCacheKey's doctrine for the stock dimension: a
// re-run with all four unchanged reuses the cached StockLLMCheckResult instead of
// re-billing. expected is deliberately NOT part of the key (same reasoning as
// llmCheckCacheKey's expect) — it only affects the LOCAL pass comparison after the
// judge classifies, never what gets sent to the API.
func stockCheckCacheKey(message, replyText, productRef, judgeModelID string) string {
	sum := sha256.Sum256([]byte(message + "\x00" + replyText + "\x00" + productRef + "\x00" + judgeModelID))
	return hex.EncodeToString(sum[:])
}

// judgeStockClaim asks the judge model to classify what the reply says about ONE
// product's stock status — a 4-way classification ("in_stock" | "out_of_stock" |
// "unclear" | "contradictory"), not a binary yes/no — and compares it against the
// catalog's ground truth. Every failure mode (HTTP/network error, unparseable
// response, a response whose "classification" isn't one of the four recognized
// values) becomes Unverified=true with Pass left false — fail closed, same doctrine as
// judgeLLMClaim.
func judgeStockClaim(ctx context.Context, client *orClient, judgeModel ModelProvider, judgeModelID, message, replyText, productRef, productName, expected string) (StockLLMCheckResult, float64) {
	result := StockLLMCheckResult{
		ProductRef:    productRef,
		ExpectedState: expected,
		CacheKey:      stockCheckCacheKey(message, replyText, productRef, judgeModelID),
	}
	res, err := client.chatText(ctx, judgeModel, buildStockJudgePrompt(message, replyText, productName))
	if err != nil {
		result.Unverified = true
		return result, 0
	}
	classification, ok := parseStockClassification(res.Raw)
	if !ok {
		result.Unverified = true
		return result, 0
	}
	result.Classification = classification
	result.Pass = classification == expected
	cost, _ := estimateCost(judgeModel, res.Usage)
	return result, cost
}

// buildStockJudgePrompt is the one Russian judge prompt every stock classification
// uses. Explicitly notes the reply may be Russian OR Kazakh (2026-07 Kazakh customer-
// message testing) — the judge model must read either. "unclear" — not a forced
// binary — is what makes an alternative-only reply (never actually addressing THIS
// product's own status) fail rather than accidentally pass: an alternative-only reply
// classifies "unclear", which then fails the ground-truth comparison, since "unclear"
// can never equal "in_stock"/"out_of_stock".
func buildStockJudgePrompt(message, replyText, productName string) string {
	return fmt.Sprintf(`Ты — независимый проверяющий качества ответов ассистента интернет-магазина. Тебе даны сообщение клиента и черновик ответа ассистента. Ответ может быть на русском ИЛИ на казахском языке — читай оба варианта одинаково внимательно.

Определи, что ЧЕРНОВИК ОТВЕТА сообщает о наличии товара «%s» — именно ЭТОГО товара, а не альтернативы или другого товара.

Классифицируй строго в одну из четырёх категорий:
- "in_stock" — ответ ясно утверждает или подразумевает, что этот товар есть в наличии.
- "out_of_stock" — ответ ясно утверждает или подразумевает, что этого товара сейчас нет в наличии.
- "unclear" — ответ не сообщает о наличии именно ЭТОГО товара (например, предлагает только альтернативу, задаёт встречный вопрос или эскалирует без ответа по существу).
- "contradictory" — ответ одновременно и утверждает, и отрицает наличие одного и того же товара.

Сообщение клиента:
%s

Черновик ответа:
%s

Ответь СТРОГО одним JSON-объектом без какого-либо текста до или после него, ровно в этом виде: {"classification": "in_stock"} — используй одно из четырёх значений: in_stock, out_of_stock, unclear, contradictory.`, productName, message, replyText)
}

var validStockClassifications = map[string]bool{
	"in_stock": true, "out_of_stock": true, "unclear": true, "contradictory": true,
}

// parseStockClassification extracts the "classification" field from the judge
// model's raw output (fence-tolerant, same as parseLLMVerdict). The second return is
// false for anything that isn't valid JSON, isn't an object, or whose value isn't one
// of the four recognized classifications — an unrecognized string is treated as
// unparseable, never silently coerced to "unclear".
func parseStockClassification(raw string) (string, bool) {
	obj, ok := parseModelJSON(raw)
	if !ok {
		return "", false
	}
	c, ok := obj["classification"].(string)
	if !ok || !validStockClassifications[c] {
		return "", false
	}
	return c, true
}
