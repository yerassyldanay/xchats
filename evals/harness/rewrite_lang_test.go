package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// priceTestModel builds a ModelProvider with real pricing set (unlike a bare
// ModelProvider{ID: ...}, which estimateCost correctly reports as unknown_pricing/$0) —
// for tests asserting LanguageRewriteCostUSD is actually computed, not just present.
func priceTestModel(id string) ModelProvider {
	in, out := 0.02, 0.08
	return ModelProvider{ID: id, InputPerMTok: &in, OutputPerMTok: &out}
}

func rewriteJSONResponse(text string) orResponse {
	resp := orResponse{}
	resp.Choices = []orChoice{{FinishReason: "stop"}}
	resp.Choices[0].Message.Content = text
	resp.Usage = &orUsage{PromptTokens: 40, CompletionTokens: 20}
	return resp
}

// TestRewriteOneVerdict_AlreadyConsistent_NoAPICallMadeAtAll is the zero-cost-happy-path
// guarantee: a server that fails the test if hit at all proves the language check alone
// (no HTTP call) is enough when the reply already reads as the target language.
func TestRewriteOneVerdict_AlreadyConsistent_NoAPICallMadeAtAll(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("rewrite call must never happen when the reply already matches the target language")
	}))
	defer srv.Close()
	client := newORClient(srv.URL, "test-key")
	rewriteModel := ModelProvider{ID: defaultRewriteModel}

	tc := TestCase{ID: "t1", Message: "Сколько стоит кофемашина?"}
	v := Verdict{
		TestID: "t1", Model: "openrouter:test/model", ContractPass: true, ModelBehaviorPass: true,
		InjectedText: "Кофемашина стоит 129 900 ₸. Хотите оформить заказ?",
	}
	got := rewriteOneVerdict(context.Background(), client, rewriteModel, defaultRewriteModel, tc, v, func(row PromptfooRow) Verdict {
		t.Fatal("judgeFn must never be called on the happy path")
		return Verdict{}
	})

	if !got.LanguageRewriteEvaluated {
		t.Error("want LanguageRewriteEvaluated=true")
	}
	if got.TargetLanguage != "ru" {
		t.Errorf("TargetLanguage = %q, want ru", got.TargetLanguage)
	}
	if got.PreRewriteLanguage != "" {
		t.Errorf("want PreRewriteLanguage empty (no mismatch found), got %q", got.PreRewriteLanguage)
	}
	if got.LanguageRewriteApplied {
		t.Error("want LanguageRewriteApplied=false — nothing to apply")
	}
	if got.InjectedText != v.InjectedText {
		t.Errorf("InjectedText changed on the happy path: got %q, want unchanged %q", got.InjectedText, v.InjectedText)
	}
}

// TestRewriteOneVerdict_MismatchRewriteSucceeds_MergesFreshGradingKeepsOriginalBookkeeping
// is the core proof of the merge contract described on Verdict.LanguageRewriteApplied:
// once the rewrite passes its re-judge, every GRADED field comes from the fresh verdict,
// but call-bookkeeping fields (RawOutput, Cost, LatencyMs, Tokens, Retries, Reasoning)
// stay exactly as the ORIGINAL draft call reported them — the rewrite has its own
// separate cost (LanguageRewriteCostUSD), not a second entry in Cost.
func TestRewriteOneVerdict_MismatchRewriteSucceeds_MergesFreshGradingKeepsOriginalBookkeeping(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(rewriteJSONResponse("Кофемашинаның бағасы {{product.coffee-machine.price}} құрайды."))
	}))
	defer srv.Close()
	client := newORClient(srv.URL, "test-key")
	rewriteModel := priceTestModel(defaultRewriteModel)

	tc := TestCase{ID: "kka2", Message: "Кофемашина DeLonghi канша турады?"} // shared-alphabet kk
	original := Verdict{
		TestID: "kka2", Model: "openrouter:test/model", ContractPass: true, ModelBehaviorPass: true,
		RawOutput:    `{"reply_text":"Кофемашина стоит {{product.coffee-machine.price}}.","reply_language":"ru","media_files_to_send":[],"escalate":false}`,
		FinalOutput:  `{"reply_text":"Кофемашина стоит {{product.coffee-machine.price}}.","reply_language":"ru","media_files_to_send":[],"escalate":false}`,
		InjectedText: "Кофемашина стоит 129 900 ₸.", // Russian — a mismatch for a Kazakh question
		Cost:         0.002, LatencyMs: 1500, Tokens: 300, TokensIn: 280, TokensOut: 20,
		CostEstimateUSD: 0.002, CostBasis: "measured_split", Retries: 1, RetryRecovered: true,
		Reasoning: "some captured reasoning",
	}

	var gotRow PromptfooRow
	fakeJudge := func(row PromptfooRow) Verdict {
		gotRow = row
		return Verdict{
			TestID: tc.ID, Model: row.Provider.ID, ContractPass: true, ModelBehaviorPass: true, Reason: "ok",
			InjectedText: "Кофемашинаның бағасы 129 900 ₸ құрайды.",
			// Deliberately zero-valued Cost/LatencyMs/Tokens/Retries/Reasoning — a synthetic
			// re-judge row carries none of the original call's bookkeeping, proving the
			// merge step is what restores them, not judgeFn itself.
		}
	}

	got := rewriteOneVerdict(context.Background(), client, rewriteModel, defaultRewriteModel, tc, original, fakeJudge)

	if gotRow.Provider.ID != original.Model {
		t.Errorf("synthetic row Provider.ID = %q, want %q", gotRow.Provider.ID, original.Model)
	}
	var sentObj map[string]any
	if err := json.Unmarshal([]byte(gotRow.Response.Output), &sentObj); err != nil {
		t.Fatalf("synthetic row Response.Output is not valid JSON: %v (%s)", err, gotRow.Response.Output)
	}
	if sentObj["reply_text"] != "Кофемашинаның бағасы {{product.coffee-machine.price}} құрайды." {
		t.Errorf("rewritten reply_text = %v", sentObj["reply_text"])
	}
	if sentObj["reply_language"] != "kk" {
		t.Errorf("rewritten reply_language = %v, want kk", sentObj["reply_language"])
	}
	if sentObj["escalate"] != false {
		t.Errorf("escalate field must be copied verbatim from the original, got %v", sentObj["escalate"])
	}

	if !got.LanguageRewriteApplied {
		t.Fatalf("want LanguageRewriteApplied=true, got fail reason %q", got.LanguageRewriteFailReason)
	}
	if got.TargetLanguage != "kk" {
		t.Errorf("TargetLanguage = %q, want kk", got.TargetLanguage)
	}
	if got.PreRewriteLanguage != "ru" {
		t.Errorf("PreRewriteLanguage = %q, want ru", got.PreRewriteLanguage)
	}
	if got.PreRewriteInjectedText != original.InjectedText {
		t.Errorf("PreRewriteInjectedText = %q, want the discarded original %q", got.PreRewriteInjectedText, original.InjectedText)
	}
	if got.InjectedText != "Кофемашинаның бағасы 129 900 ₸ құрайды." {
		t.Errorf("InjectedText = %q, want the fresh (rewritten) injected text", got.InjectedText)
	}
	if got.LanguageRewriteModel != defaultRewriteModel {
		t.Errorf("LanguageRewriteModel = %q", got.LanguageRewriteModel)
	}
	if got.LanguageRewriteCostUSD <= 0 {
		t.Errorf("want LanguageRewriteCostUSD > 0, got %v", got.LanguageRewriteCostUSD)
	}
	// Bookkeeping preserved from the ORIGINAL draft call, not zeroed out by the fresh re-judge.
	if got.RawOutput != original.RawOutput {
		t.Errorf("RawOutput must stay the immutable original: got %q", got.RawOutput)
	}
	if got.Cost != original.Cost || got.LatencyMs != original.LatencyMs || got.Tokens != original.Tokens {
		t.Errorf("want Cost/LatencyMs/Tokens preserved from original, got Cost=%v LatencyMs=%v Tokens=%v", got.Cost, got.LatencyMs, got.Tokens)
	}
	if got.Retries != original.Retries || !got.RetryRecovered {
		t.Errorf("want Retries/RetryRecovered preserved from original, got Retries=%v RetryRecovered=%v", got.Retries, got.RetryRecovered)
	}
	if got.Reasoning != original.Reasoning {
		t.Errorf("want Reasoning preserved from original, got %q", got.Reasoning)
	}
}

// TestRewriteOneVerdict_RewriteCallFails_OriginalVerdictUntouched proves a network/API
// failure discards cleanly: the verdict is returned byte-for-byte as it was, plus only
// the bookkeeping fields that record the attempt happened and why it didn't help.
func TestRewriteOneVerdict_RewriteCallFails_OriginalVerdictUntouched(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":{"message":"boom"}}`))
	}))
	defer srv.Close()
	client := newORClient(srv.URL, "test-key")
	rewriteModel := ModelProvider{ID: defaultRewriteModel}

	tc := TestCase{ID: "t1", Message: "Кофемашина DeLonghi канша турады?"}
	original := Verdict{
		TestID: "t1", Model: "openrouter:test/model", ContractPass: true, ModelBehaviorPass: true,
		FinalOutput:  `{"reply_text":"Стоит 129 900 тенге.","reply_language":"ru","media_files_to_send":[],"escalate":false}`,
		InjectedText: "Стоит 129 900 тенге.",
	}

	got := rewriteOneVerdict(context.Background(), client, rewriteModel, defaultRewriteModel, tc, original, func(row PromptfooRow) Verdict {
		t.Fatal("judgeFn must never be called when the rewrite call itself fails")
		return Verdict{}
	})

	if got.LanguageRewriteApplied {
		t.Error("want LanguageRewriteApplied=false")
	}
	if got.LanguageRewriteFailReason == "" {
		t.Error("want a non-empty LanguageRewriteFailReason")
	}
	if got.InjectedText != original.InjectedText || got.ModelBehaviorPass != original.ModelBehaviorPass {
		t.Errorf("want the original grading completely untouched, got InjectedText=%q ModelBehaviorPass=%v", got.InjectedText, got.ModelBehaviorPass)
	}
}

// TestRewriteOneVerdict_RewriteBreaksContract_DiscardedNotSubstituted is the safety
// property that matters most: a rewrite that corrupts a placeholder (or otherwise fails
// its re-judge) must NEVER be substituted in — the original verdict, mismatch and all,
// is what survives.
func TestRewriteOneVerdict_RewriteBreaksContract_DiscardedNotSubstituted(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(rewriteJSONResponse("Бағасы {{product.coffee-machine.pric}} құрайды.")) // corrupted token
	}))
	defer srv.Close()
	client := newORClient(srv.URL, "test-key")
	rewriteModel := ModelProvider{ID: defaultRewriteModel}

	tc := TestCase{ID: "t1", Message: "Кофемашина DeLonghi канша турады?"}
	original := Verdict{
		TestID: "t1", Model: "openrouter:test/model", ContractPass: true, ModelBehaviorPass: true,
		FinalOutput:  `{"reply_text":"Стоит {{product.coffee-machine.price}}.","reply_language":"ru","media_files_to_send":[],"escalate":false}`,
		InjectedText: "Стоит 129 900 ₸.",
	}

	got := rewriteOneVerdict(context.Background(), client, rewriteModel, defaultRewriteModel, tc, original, func(row PromptfooRow) Verdict {
		return Verdict{TestID: tc.ID, ContractPass: false, Reason: "unknown fact placeholder, draft would be BLOCKED: {{product.coffee-machine.pric}}"}
	})

	if got.LanguageRewriteApplied {
		t.Error("want LanguageRewriteApplied=false — the rewrite corrupted a placeholder")
	}
	if got.LanguageRewriteFailReason == "" {
		t.Error("want a non-empty LanguageRewriteFailReason naming the contract failure")
	}
	if got.InjectedText != original.InjectedText {
		t.Errorf("want InjectedText unchanged from the original, got %q", got.InjectedText)
	}
}

// setupRewriteLangFixture mirrors setupJudgeLLMFixture (judge_llm_test.go) closely — a
// minimal legacy-pipeline scenario + run directory, one row with a Kazakh question
// answered in Russian (a real mismatch) and one already-consistent row (must trigger no
// API call at all).
func setupRewriteLangFixture(t *testing.T) (scenarioDir, runDir, modelsPath string) {
	t.Helper()
	root := t.TempDir()

	scenarioDir = filepath.Join(root, "scenarios", "rewrite-fixture")
	if err := os.MkdirAll(filepath.Join(scenarioDir, "generated"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(scenarioDir, "scenario.yaml"), []byte("name: rewrite-fixture\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	resolved := ResolvedTests{Tests: []TestCase{
		{ID: "kk-mismatch", Message: "Кофемашина DeLonghi канша турады?"},
		{ID: "already-consistent", Message: "Сколько стоит кофемашина?"},
		{ID: "contract-failed", Message: "hi3"},
	}}
	if err := writeJSON(filepath.Join(scenarioDir, "generated", "resolved_tests.json"), resolved); err != nil {
		t.Fatal(err)
	}
	catalog := Catalog{Tokens: []CatalogFact{{Token: "{{product.coffee-machine.price}}", Value: "129 900 ₸"}}}
	if err := writeJSON(filepath.Join(scenarioDir, "generated", "catalog.json"), catalog); err != nil {
		t.Fatal(err)
	}

	runDir = filepath.Join(root, "runs", "2026-01-01_00-00-00-r1")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	judged := JudgedRun{Scenario: "rewrite-fixture", Verdicts: []Verdict{
		{
			TestID: "kk-mismatch", Model: "openrouter:test/model", ContractPass: true, ModelBehaviorPass: true,
			FinalOutput:  `{"reply_text":"Стоит {{product.coffee-machine.price}}.","reply_language":"ru","media_files_to_send":[],"escalate":false}`,
			InjectedText: "Стоит 129 900 ₸.",
		},
		{
			TestID: "already-consistent", Model: "openrouter:test/model", ContractPass: true, ModelBehaviorPass: true,
			FinalOutput:  `{"reply_text":"Кофемашина стоит {{product.coffee-machine.price}}. Хотите оформить заказ?","reply_language":"ru","media_files_to_send":[],"escalate":false}`,
			InjectedText: "Кофемашина стоит 129 900 ₸. Хотите оформить заказ?",
		},
		{TestID: "contract-failed", Model: "openrouter:test/model", ContractPass: false, Reason: "could not parse JSON output"},
	}}
	if err := writeJSON(filepath.Join(runDir, "rewrite-fixture.judged.json"), judged); err != nil {
		t.Fatal(err)
	}

	modelsPath = filepath.Join(root, "models.yaml")
	modelsYAML := "pricing_source: test\npricing_checked_at: \"2026-01-01\"\nproviders:\n  - id: " + defaultRewriteModel + "\n    input_per_mtok: 0.02\n    output_per_mtok: 0.08\n"
	if err := os.WriteFile(modelsPath, []byte(modelsYAML), 0o644); err != nil {
		t.Fatal(err)
	}
	return scenarioDir, runDir, modelsPath
}

// TestCmdRewriteLang_EndToEnd_OnlyCallsOnTheMismatchedRow is the end-to-end proof over
// the real command: exactly one row needed a rewrite call (the other ContractPass row
// already matched its target language; the third row's ContractPass=false skips it
// entirely), and the resulting judged.json reflects the merge.
func TestCmdRewriteLang_EndToEnd_OnlyCallsOnTheMismatchedRow(t *testing.T) {
	t.Setenv("OPENROUTER_API_KEY", "test-key")
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(rewriteJSONResponse("Бағасы {{product.coffee-machine.price}} құрайды."))
	}))
	defer srv.Close()

	scenarioDir, runDir, modelsPath := setupRewriteLangFixture(t)
	args := []string{"-scenario", scenarioDir, "-run", runDir, "-models", modelsPath, "-base-url", srv.URL}
	if err := cmdRewriteLang(args); err != nil {
		t.Fatalf("cmdRewriteLang: %v", err)
	}
	if calls != 1 {
		t.Fatalf("want exactly 1 API call (only kk-mismatch needed one), got %d", calls)
	}

	var judged JudgedRun
	if err := readJSON(filepath.Join(runDir, "rewrite-fixture.judged.json"), &judged); err != nil {
		t.Fatal(err)
	}
	byID := map[string]Verdict{}
	for _, v := range judged.Verdicts {
		byID[v.TestID] = v
	}

	mismatch := byID["kk-mismatch"]
	if !mismatch.LanguageRewriteApplied {
		t.Fatalf("want kk-mismatch rewritten, got fail reason %q", mismatch.LanguageRewriteFailReason)
	}
	if mismatch.InjectedText != "Бағасы 129 900 ₸ құрайды." {
		t.Errorf("kk-mismatch InjectedText = %q", mismatch.InjectedText)
	}

	consistent := byID["already-consistent"]
	if !consistent.LanguageRewriteEvaluated || consistent.LanguageRewriteApplied || consistent.PreRewriteLanguage != "" {
		t.Errorf("want already-consistent checked but untouched, got %+v", consistent)
	}

	failed := byID["contract-failed"]
	if failed.LanguageRewriteEvaluated {
		t.Error("want contract-failed row never checked at all (ContractPass=false gate)")
	}

	// Re-run without -force: the already-evaluated rows must not trigger a second call.
	calls = 0
	if err := cmdRewriteLang(args); err != nil {
		t.Fatalf("cmdRewriteLang (second run): %v", err)
	}
	if calls != 0 {
		t.Errorf("want 0 API calls on a re-run without -force, got %d", calls)
	}
}
