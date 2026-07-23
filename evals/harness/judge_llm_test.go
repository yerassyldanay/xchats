package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// setupJudgeLLMFixture builds a minimal scenario + run directory judge-llm can operate
// on: a live generated/resolved_tests.json (no snapshot — exercises the same fallback
// path judgeScenario itself uses for a standalone run) declaring one test with two
// llm_checks claims and one plain test with none, plus a judged.json with matching
// verdicts (both already ContractPass — the state a real `harness judge` run would leave
// them in).
func setupJudgeLLMFixture(t *testing.T) (scenarioDir, runDir, modelsPath string) {
	t.Helper()
	root := t.TempDir()

	scenarioDir = filepath.Join(root, "scenarios", "llm-fixture")
	if err := os.MkdirAll(filepath.Join(scenarioDir, "generated"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(scenarioDir, "scenario.yaml"), []byte("name: llm-fixture\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	resolved := ResolvedTests{Tests: []TestCase{
		{
			ID:      "d4. china",
			Message: "А в Китай доставляете?",
			LLMChecks: []LLMCheck{
				{Claim: "Ответ сообщает, что доставки нет", Expect: true},
				{Claim: "Ответ добавляет обещания сверх отказа", Expect: false},
			},
		},
		{ID: "plain", Message: "hi"}, // no llm_checks -> judge-llm must never touch its verdict
		{
			ID:      "contract-failed",
			Message: "hi3",
			// Declares llm_checks too, so the skip below is provably due to ContractPass
			// alone, not merely a missing TestCase lookup.
			LLMChecks: []LLMCheck{{Claim: "irrelevant — must never be asked", Expect: true}},
		},
	}}
	if err := writeJSON(filepath.Join(scenarioDir, "generated", "resolved_tests.json"), resolved); err != nil {
		t.Fatal(err)
	}

	runDir = filepath.Join(root, "runs", "2026-01-01_00-00-00-r1")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	judged := JudgedRun{Scenario: "llm-fixture", Verdicts: []Verdict{
		{
			TestID: "d4. china", Model: "openrouter:test/model", ContractPass: true,
			ModelBehaviorPass: true, Reason: "ok",
			InjectedText: "К сожалению, в Китай мы не доставляем.",
		},
		{
			TestID: "plain", Model: "openrouter:test/model", ContractPass: true,
			ModelBehaviorPass: true, Reason: "ok", InjectedText: "Здравствуйте!",
		},
		{
			TestID: "contract-failed", Model: "openrouter:test/model", ContractPass: false,
			Reason: "could not parse JSON output",
		},
	}}
	if err := writeJSON(filepath.Join(runDir, "llm-fixture.judged.json"), judged); err != nil {
		t.Fatal(err)
	}

	modelsPath = filepath.Join(root, "models.yaml")
	modelsYAML := "pricing_source: test\npricing_checked_at: \"2026-01-01\"\nproviders:\n  - id: openrouter:google/gemini-2.5-flash\n    input_per_mtok: 0.1\n    output_per_mtok: 0.4\n"
	if err := os.WriteFile(modelsPath, []byte(modelsYAML), 0o644); err != nil {
		t.Fatal(err)
	}
	return scenarioDir, runDir, modelsPath
}

func verdictJSONResponse(verdict bool) orResponse {
	resp := orResponse{}
	resp.Choices = []orChoice{{FinishReason: "stop"}}
	b, _ := json.Marshal(map[string]bool{"verdict": verdict})
	resp.Choices[0].Message.Content = string(b)
	resp.Usage = &orUsage{PromptTokens: 80, CompletionTokens: 5}
	return resp
}

func TestCmdJudgeLLM_EvaluatesDeclaredClaimsAndLeavesDeterministicFieldsAlone(t *testing.T) {
	t.Setenv("OPENROUTER_API_KEY", "test-key")
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		// First claim expects true ("no delivery" IS communicated); second expects false
		// ("no extra promises" — the model must answer "false" too, i.e. none added).
		verdict := calls == 1
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(verdictJSONResponse(verdict))
	}))
	defer srv.Close()

	scenarioDir, runDir, modelsPath := setupJudgeLLMFixture(t)
	args := []string{"-scenario", scenarioDir, "-run", runDir, "-models", modelsPath, "-base-url", srv.URL}
	if err := cmdJudgeLLM(args); err != nil {
		t.Fatalf("cmdJudgeLLM: %v", err)
	}
	if calls != 2 {
		t.Fatalf("want 2 API calls (one per declared claim), got %d", calls)
	}

	var judged JudgedRun
	if err := readJSON(filepath.Join(runDir, "llm-fixture.judged.json"), &judged); err != nil {
		t.Fatal(err)
	}

	byID := map[string]Verdict{}
	for _, v := range judged.Verdicts {
		byID[v.TestID] = v
	}

	china := byID["d4. china"]
	if !china.LLMCheckEvaluated {
		t.Fatal("want LLMCheckEvaluated=true for the row that declared llm_checks")
	}
	if china.LLMJudgePass == nil || !*china.LLMJudgePass {
		t.Fatalf("want LLMJudgePass=true (both claims matched Expect), got %+v", china.LLMJudgePass)
	}
	if china.LLMJudgeModel != defaultLLMJudgeModel {
		t.Errorf("LLMJudgeModel = %q, want %q", china.LLMJudgeModel, defaultLLMJudgeModel)
	}
	if len(china.LLMChecks) != 2 || !china.LLMChecks[0].Verdict || china.LLMChecks[1].Verdict {
		t.Fatalf("LLMChecks = %+v", china.LLMChecks)
	}
	if china.LLMJudgeCostUSD <= 0 {
		t.Error("want a positive cost estimate now that models.yaml has pricing for this model")
	}
	// The deterministic fields judge.go itself wrote must survive byte-identical —
	// judge-llm only ever appends LLM-prefixed fields.
	if !china.ContractPass || !china.ModelBehaviorPass || china.Reason != "ok" {
		t.Errorf("judge-llm must not touch deterministic fields, got ContractPass=%v ModelBehaviorPass=%v Reason=%q",
			china.ContractPass, china.ModelBehaviorPass, china.Reason)
	}

	plain := byID["plain"]
	if plain.LLMCheckEvaluated {
		t.Error("want the 'plain' row (no llm_checks declared) to be left untouched")
	}
}

// TestCmdJudgeLLM_SkipsContractFailedRows: a row whose contract already failed has no
// valid customer-facing text — judge-llm must never call the API for it, even if its
// TestCase happens to declare llm_checks (not the case in this fixture, but the row must
// still be skipped on ContractPass alone).
func TestCmdJudgeLLM_SkipsContractFailedRows(t *testing.T) {
	t.Setenv("OPENROUTER_API_KEY", "test-key")
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(verdictJSONResponse(true))
	}))
	defer srv.Close()

	scenarioDir, runDir, modelsPath := setupJudgeLLMFixture(t)
	args := []string{"-scenario", scenarioDir, "-run", runDir, "-models", modelsPath, "-base-url", srv.URL}
	if err := cmdJudgeLLM(args); err != nil {
		t.Fatalf("cmdJudgeLLM: %v", err)
	}
	var judged JudgedRun
	if err := readJSON(filepath.Join(runDir, "llm-fixture.judged.json"), &judged); err != nil {
		t.Fatal(err)
	}
	for _, v := range judged.Verdicts {
		if v.TestID == "contract-failed" && v.LLMCheckEvaluated {
			t.Error("want the contract-failed row to remain unevaluated")
		}
	}
	if calls != 2 {
		t.Fatalf("want exactly 2 calls (only d4. china's declared claims), got %d", calls)
	}
}

// TestCmdJudgeLLM_CacheHitSkipsSecondCall proves the whole point of caching: an unchanged
// row's claims are never re-billed on a second invocation against the same run.
func TestCmdJudgeLLM_CacheHitSkipsSecondCall(t *testing.T) {
	t.Setenv("OPENROUTER_API_KEY", "test-key")
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(verdictJSONResponse(calls%2 == 1))
	}))
	defer srv.Close()

	scenarioDir, runDir, modelsPath := setupJudgeLLMFixture(t)
	args := []string{"-scenario", scenarioDir, "-run", runDir, "-models", modelsPath, "-base-url", srv.URL}
	if err := cmdJudgeLLM(args); err != nil {
		t.Fatalf("first invocation: %v", err)
	}
	if calls != 2 {
		t.Fatalf("want 2 calls on the first invocation, got %d", calls)
	}
	if err := cmdJudgeLLM(args); err != nil {
		t.Fatalf("second invocation: %v", err)
	}
	if calls != 2 {
		t.Fatalf("want NO additional calls on the second invocation (cache hit), total = %d", calls)
	}

	// -force must bypass the cache and re-bill.
	args = append(args, "-force")
	if err := cmdJudgeLLM(args); err != nil {
		t.Fatalf("forced invocation: %v", err)
	}
	if calls != 4 {
		t.Fatalf("want -force to re-call for both claims, total calls = %d", calls)
	}
}

// TestCmdJudgeLLM_HTTPFailureMarksUnverified proves a judge-call failure never becomes a
// silent pass — fail-closed, matching every other fail-closed discipline in this harness.
func TestCmdJudgeLLM_HTTPFailureMarksUnverified(t *testing.T) {
	t.Setenv("OPENROUTER_API_KEY", "test-key")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"boom"}`))
	}))
	defer srv.Close()

	scenarioDir, runDir, modelsPath := setupJudgeLLMFixture(t)
	args := []string{"-scenario", scenarioDir, "-run", runDir, "-models", modelsPath, "-base-url", srv.URL}
	if err := cmdJudgeLLM(args); err != nil {
		t.Fatalf("cmdJudgeLLM: %v", err)
	}

	var judged JudgedRun
	if err := readJSON(filepath.Join(runDir, "llm-fixture.judged.json"), &judged); err != nil {
		t.Fatal(err)
	}
	for _, v := range judged.Verdicts {
		if v.TestID != "d4. china" {
			continue
		}
		for _, r := range v.LLMChecks {
			if !r.Unverified {
				t.Errorf("claim %q: want Unverified=true after an HTTP failure, got %+v", r.Claim, r)
			}
			if r.Pass {
				t.Errorf("claim %q: want Pass=false when unverified (never a silent pass)", r.Claim)
			}
		}
		if v.LLMJudgePass == nil || *v.LLMJudgePass {
			t.Errorf("want LLMJudgePass=false when every claim is unverified, got %+v", v.LLMJudgePass)
		}
	}
}

// TestCmdJudgeLLM_UnparseableResponseMarksUnverified: the judge model itself can return
// prose instead of the requested JSON object — this must fail closed exactly like an
// HTTP error, not silently default to some polarity.
func TestCmdJudgeLLM_UnparseableResponseMarksUnverified(t *testing.T) {
	t.Setenv("OPENROUTER_API_KEY", "test-key")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := orResponse{}
		resp.Choices = []orChoice{{FinishReason: "stop"}}
		resp.Choices[0].Message.Content = "Конечно, да, отвечаю без JSON."
		resp.Usage = &orUsage{PromptTokens: 80, CompletionTokens: 5}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	scenarioDir, runDir, modelsPath := setupJudgeLLMFixture(t)
	args := []string{"-scenario", scenarioDir, "-run", runDir, "-models", modelsPath, "-base-url", srv.URL}
	if err := cmdJudgeLLM(args); err != nil {
		t.Fatalf("cmdJudgeLLM: %v", err)
	}

	var judged JudgedRun
	if err := readJSON(filepath.Join(runDir, "llm-fixture.judged.json"), &judged); err != nil {
		t.Fatal(err)
	}
	for _, v := range judged.Verdicts {
		if v.TestID != "d4. china" {
			continue
		}
		for _, r := range v.LLMChecks {
			if !r.Unverified || r.Pass {
				t.Errorf("claim %q: want Unverified=true, Pass=false for a non-JSON response, got %+v", r.Claim, r)
			}
		}
	}
}

func TestParseLLMVerdict(t *testing.T) {
	tests := []struct {
		raw     string
		want    bool
		wantOK  bool
		comment string
	}{
		{`{"verdict": true}`, true, true, "exact"},
		{"```json\n{\"verdict\": false}\n```", false, true, "fenced"},
		{`{"verdict": "true"}`, false, false, "string, not bool -> rejected"},
		{"да, конечно", false, false, "plain prose -> rejected"},
		{`{}`, false, false, "missing key -> rejected"},
	}
	for _, tt := range tests {
		got, ok := parseLLMVerdict(tt.raw)
		if ok != tt.wantOK || (ok && got != tt.want) {
			t.Errorf("%s: parseLLMVerdict(%q) = (%v, %v), want (%v, %v)", tt.comment, tt.raw, got, ok, tt.want, tt.wantOK)
		}
	}
}

func TestLLMCheckCacheKey_StableAndSensitiveToEveryInput(t *testing.T) {
	base := llmCheckCacheKey("msg", "reply", "claim", "model-a")
	cases := map[string]string{
		"message changed": llmCheckCacheKey("msg2", "reply", "claim", "model-a"),
		"reply changed":   llmCheckCacheKey("msg", "reply2", "claim", "model-a"),
		"claim changed":   llmCheckCacheKey("msg", "reply", "claim2", "model-a"),
		"model changed":   llmCheckCacheKey("msg", "reply", "claim", "model-b"),
	}
	for name, key := range cases {
		if key == base {
			t.Errorf("%s: cache key must differ from the base key, both were %q", name, key)
		}
	}
	if llmCheckCacheKey("msg", "reply", "claim", "model-a") != base {
		t.Error("cache key must be stable for identical inputs")
	}
}
