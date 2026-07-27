package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"xchats-evals-harness/internal/provenance"
)

func TestRetryCandidateIndexes(t *testing.T) {
	rows := []PromptfooRow{
		{}, // index 0: unparseable (empty output) -> candidate, contract_shape
		{}, // index 1: valid JSON -> not a candidate
		{}, // index 2: unparseable but already retried once -> NOT a candidate (idempotent)
		{}, // index 3: unparseable, never retried -> candidate, contract_shape
	}
	rows[0].Response.Output = ""
	rows[1].Response.Output = `{"reply_text":"ok","reply_language":"ru","media_files_to_send":[],"escalate":false,"escalation_reason":"","confidence":0.9}`
	rows[2].Response.Output = "Thinking: still broken"
	rows[2].Retries = 1
	rows[3].Response.Output = "not json at all"

	got := retryCandidateIndexes(rows, map[string]bool{})
	want := []retryCandidate{{Index: 0, Reason: retryReasonContractShape}, {Index: 3, Reason: retryReasonContractShape}}
	if len(got) != len(want) {
		t.Fatalf("want candidates %+v, got %+v", want, got)
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("want candidates %+v, got %+v", want, got)
			break
		}
	}
}

// TestRetryCandidateIndexes_MediaNotFoundReason confirms an otherwise-well-formed row
// naming an out-of-catalog media token is labeled media_not_found, not contract_shape —
// the two reasons must never collapse into one.
func TestRetryCandidateIndexes_MediaNotFoundReason(t *testing.T) {
	row := PromptfooRow{}
	row.Response.Output = `{"reply_text":"ok","reply_language":"ru","media_files_to_send":["totally.invented.token"],"escalate":false,"escalation_reason":"","confidence":0.9}`
	got := retryCandidateIndexes([]PromptfooRow{row}, map[string]bool{})
	if len(got) != 1 || got[0].Reason != retryReasonMediaNotFound {
		t.Fatalf("want a single media_not_found candidate, got %+v", got)
	}
}

// TestRetryCandidateIndexes_RunTwiceIsANoOp is the idempotency guarantee at the row
// level: once a row has Retries>0, it must never be selected again, regardless of
// whether its (possibly still-failed) output parses.
func TestRetryCandidateIndexes_RunTwiceIsANoOp(t *testing.T) {
	rows := []PromptfooRow{{}}
	rows[0].Response.Output = ""
	rows[0].Retries = 1 // already retried once, and the retry itself still failed
	got := retryCandidateIndexes(rows, map[string]bool{})
	if len(got) != 0 {
		t.Fatalf("want no candidates (already retried once), got %v", got)
	}
}

func TestMergeRowPatch_PreservesUnknownFields(t *testing.T) {
	original := json.RawMessage(`{
		"id": "row-1",
		"promptId": "p1",
		"testCase": {"description": "1. price question, Russian"},
		"provider": {"id": "openrouter:minimax/minimax-m2.5", "label": ""},
		"response": {"output": "broken", "cached": false, "finishReason": "length"},
		"tokenUsage": {"total": 500, "prompt": 400, "completion": 100},
		"latencyMs": 12000,
		"gradingResult": {"pass": true, "score": 1, "reason": "unmodeled by this harness"},
		"namedScores": {"custom": 1},
		"vars": {"message": "Сколько стоит?"},
		"metadata": {"foo": "bar"},
		"success": true,
		"failureReason": null
	}`)

	patch := rowPatch{Retries: 1, SelectedAttempt: 1}
	patch.Response.Output = `{"reply_text":"fixed","reply_language":"ru","media_files_to_send":[],"escalate":false}`
	patch.Response.FinishReason = "stop"
	patch.TokenUsage.Total = 900
	patch.TokenUsage.Prompt = 400
	patch.TokenUsage.Completion = 500
	patch.Response.TokenUsage = patch.TokenUsage // mergeRowPatch copies this verbatim, same as retryOneRow does
	patch.LatencyMs = 20000
	patch.Attempts = []ResultAttempt{{Output: "broken", FinishReason: "length"}, {Output: patch.Response.Output, FinishReason: "stop"}}

	merged, err := mergeRowPatch(original, patch)
	if err != nil {
		t.Fatal(err)
	}

	var obj map[string]any
	if err := json.Unmarshal(merged, &obj); err != nil {
		t.Fatal(err)
	}

	// Untouched, unmodeled fields must survive exactly.
	if obj["id"] != "row-1" {
		t.Errorf("want id preserved, got %v", obj["id"])
	}
	if obj["promptId"] != "p1" {
		t.Errorf("want promptId preserved, got %v", obj["promptId"])
	}
	grading, _ := obj["gradingResult"].(map[string]any)
	if grading["reason"] != "unmodeled by this harness" {
		t.Errorf("want gradingResult preserved untouched, got %v", obj["gradingResult"])
	}
	if obj["namedScores"] == nil {
		t.Error("want namedScores preserved")
	}
	vars, _ := obj["vars"].(map[string]any)
	if vars["message"] != "Сколько стоит?" {
		t.Errorf("want vars preserved, got %v", obj["vars"])
	}
	if obj["success"] != true {
		t.Errorf("want success preserved, got %v", obj["success"])
	}

	// Patched fields must reflect the new values.
	response, _ := obj["response"].(map[string]any)
	if response["output"] != patch.Response.Output {
		t.Errorf("want response.output patched, got %v", response["output"])
	}
	if response["finishReason"] != "stop" {
		t.Errorf("want response.finishReason patched to stop, got %v", response["finishReason"])
	}
	respTU, _ := response["tokenUsage"].(map[string]any)
	if respTU["total"] != float64(900) {
		t.Errorf("want response.tokenUsage.total patched to 900 (summed across attempts), got %v", respTU["total"])
	}
	topTU, _ := obj["tokenUsage"].(map[string]any)
	if topTU["total"] != float64(900) {
		t.Errorf("want TOP-LEVEL tokenUsage.total ALSO patched to 900 (both copies must agree), got %v", topTU["total"])
	}
	if obj["retries"] != float64(1) {
		t.Errorf("want retries=1, got %v", obj["retries"])
	}
	attempts, _ := obj["attempts"].([]any)
	if len(attempts) != 2 {
		t.Fatalf("want 2 preserved attempts (original + retry), got %d", len(attempts))
	}
}

func TestRetryOneRow_SuccessAppendsAttemptAndSelectsIt(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := orResponse{ID: "gen-test-1", Provider: "TestProvider"}
		resp.Choices = []orChoice{{FinishReason: "stop", NativeFinishReason: "stop"}}
		resp.Choices[0].Message.Content = `{"reply_text":"fixed on retry","reply_language":"ru","media_files_to_send":[],"escalate":false}`
		reasoningTokens := 42
		resp.Usage = &orUsage{
			PromptTokens: 100, CompletionTokens: 60,
			CompletionTokensDetails: &orCompletionTokensDetails{ReasoningTokens: &reasoningTokens},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	client := newORClientWithTimeout(srv.URL, "test-key", 5*time.Second)
	provider := ModelProvider{ID: "openrouter:minimax/minimax-m2.5", Temperature: 0.3, MaxTokens: 2000}
	row := PromptfooRow{}
	row.Response.Output = "Thinking: never finished"
	row.Response.FinishReason = "length"
	row.TokenUsage.Total, row.TokenUsage.Prompt, row.TokenUsage.Completion = 500, 400, 100
	row.LatencyMs = 30000
	row.Prompt.Raw = "the exact rendered prompt"

	patch, err := retryOneRow(client, provider, row, "parent-sha", "retry-sha", map[string]bool{}, nil, nil, retryReasonContractShape)
	if err != nil {
		t.Fatal(err)
	}
	if patch.Retries != 1 {
		t.Errorf("want Retries=1, got %d", patch.Retries)
	}
	if patch.RetryReason != string(retryReasonContractShape) {
		t.Errorf("want RetryReason=%q, got %q", retryReasonContractShape, patch.RetryReason)
	}
	if patch.Attempts[1].RetryReason != string(retryReasonContractShape) {
		t.Errorf("want the retry attempt's own RetryReason=%q, got %q", retryReasonContractShape, patch.Attempts[1].RetryReason)
	}
	if patch.SelectedAttempt != 1 {
		t.Errorf("want SelectedAttempt=1 (the retry), got %d", patch.SelectedAttempt)
	}
	if len(patch.Attempts) != 2 {
		t.Fatalf("want 2 attempts (original + retry), got %d", len(patch.Attempts))
	}
	if patch.Attempts[0].Output != row.Response.Output || patch.Attempts[0].ModelConfigSHA256 != "parent-sha" {
		t.Errorf("want attempt 0 to be the ORIGINAL attempt under the parent config, got %+v", patch.Attempts[0])
	}
	if patch.Attempts[1].ModelConfigSHA256 != "retry-sha" || patch.Attempts[1].ReasoningTokens != 42 {
		t.Errorf("want attempt 1 to be the retry under the retry config with reasoning tokens captured, got %+v", patch.Attempts[1])
	}
	if patch.Response.Output != `{"reply_text":"fixed on retry","reply_language":"ru","media_files_to_send":[],"escalate":false}` {
		t.Errorf("want response.output = the retry's content, got %q", patch.Response.Output)
	}
	// Real spend, real wall time: summed across BOTH attempts, not just the selected one.
	if patch.TokenUsage.Total != 500+160 {
		t.Errorf("want summed tokenUsage.total = 660, got %d", patch.TokenUsage.Total)
	}
}

func TestRetryOneRow_HTTPFailureKeepsOriginalSelected(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"upstream exploded"}`))
	}))
	defer srv.Close()

	client := newORClientWithTimeout(srv.URL, "test-key", 5*time.Second)
	provider := ModelProvider{ID: "openrouter:minimax/minimax-m2.5", Temperature: 0.3, MaxTokens: 2000}
	row := PromptfooRow{}
	row.Response.Output = "Thinking: never finished"
	row.Response.FinishReason = "length"
	row.LatencyMs = 30000
	row.Prompt.Raw = "the exact rendered prompt"

	patch, err := retryOneRow(client, provider, row, "parent-sha", "retry-sha", map[string]bool{}, nil, nil, retryReasonMediaNotFound)
	// An HTTP failure on the retry call itself is NOT a fatal error for the batch —
	// it's recorded as a failed attempt, original stays selected.
	if err != nil {
		t.Fatalf("want no error (HTTP failures are recorded, not propagated), got %v", err)
	}
	if patch.SelectedAttempt != 0 {
		t.Errorf("want SelectedAttempt=0 (original kept) on an HTTP failure, got %d", patch.SelectedAttempt)
	}
	if patch.Response.Output != row.Response.Output {
		t.Errorf("want response.output unchanged (still the original broken output), got %q", patch.Response.Output)
	}
	if patch.Retries != 1 {
		t.Errorf("want Retries=1 even on failure — a second `harness retry` invocation must not spend again on this row, got %d", patch.Retries)
	}
	if len(patch.Attempts) != 2 || patch.Attempts[1].Error == "" {
		t.Fatalf("want 2 attempts with the second carrying a non-empty Error, got %+v", patch.Attempts)
	}
	// RetryReason is still recorded even though the retry call itself failed — WHY a
	// retry was attempted is a fact about candidacy, independent of the outcome.
	if patch.RetryReason != string(retryReasonMediaNotFound) || patch.Attempts[1].RetryReason != string(retryReasonMediaNotFound) {
		t.Errorf("want RetryReason recorded on both patch and the failed attempt, got patch=%q attempt=%q", patch.RetryReason, patch.Attempts[1].RetryReason)
	}
}

// TestPatchResultsFile_UntouchedRowsByteIdentical is the losslessness guarantee: a row
// NOT selected for retry must never be decoded into a typed struct and re-marshaled —
// that would reorder keys and could silently drop fields this harness doesn't model.
func TestPatchResultsFile_UntouchedRowsByteIdentical(t *testing.T) {
	dir := t.TempDir()
	srcPath := dir + "/src.results.json"
	dstPath := dir + "/dst.results.json"

	untouchedRow := `{"id":"row-untouched","zzz_last_field":"should survive in this exact position","response":{"output":"{\"reply_text\":\"ok\",\"reply_language\":\"ru\",\"media_files_to_send\":[],\"escalate\":false,\"escalation_reason\":\"\",\"confidence\":0.9}","cached":false,"finishReason":"stop"},"tokenUsage":{"total":100,"prompt":80,"completion":20},"testCase":{"description":"ok test"},"provider":{"id":"openrouter:minimax/minimax-m2.5","label":""}}`
	brokenRow := `{"id":"row-broken","response":{"output":"Thinking: broken","cached":false,"finishReason":"length"},"tokenUsage":{"total":500,"prompt":400,"completion":100},"testCase":{"description":"broken test"},"provider":{"id":"openrouter:minimax/minimax-m2.5","label":""},"prompt":{"raw":"the prompt"}}`

	srcJSON := `{"results":{"results":[` + untouchedRow + `,` + brokenRow + `]}}`
	if err := os.WriteFile(srcPath, []byte(srcJSON), 0o644); err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := orResponse{}
		resp.Choices = []orChoice{{FinishReason: "stop"}}
		resp.Choices[0].Message.Content = `{"reply_text":"fixed","reply_language":"ru","media_files_to_send":[],"escalate":false}`
		resp.Usage = &orUsage{PromptTokens: 400, CompletionTokens: 50}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()
	client := newORClientWithTimeout(srv.URL, "test-key", 5*time.Second)
	provider := ModelProvider{ID: "openrouter:minimax/minimax-m2.5", Temperature: 0.3, MaxTokens: 2000}

	var typedResults PromptfooResults
	if err := readJSON(srcPath, &typedResults); err != nil {
		t.Fatal(err)
	}
	rows := typedResults.Results.Results
	candidates := retryCandidateIndexes(rows, map[string]bool{})
	if len(candidates) != 1 || candidates[0].Index != 1 {
		t.Fatalf("want exactly row 1 (the broken one) as a candidate, got %+v", candidates)
	}

	retried, err := patchResultsFile(
		srcPath,
		dstPath,
		candidates,
		rows,
		client,
		map[int]ModelProvider{1: provider},
		"parent-sha",
		"retry-sha",
		map[string]bool{},
		nil,
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	if retried != 1 {
		t.Fatalf("want 1 row retried, got %d", retried)
	}

	var dstEnv resultsEnvelope
	if err := readJSON(dstPath, &dstEnv); err != nil {
		t.Fatal(err)
	}
	if len(dstEnv.Results.Results) != 2 {
		t.Fatalf("want 2 rows in output, got %d", len(dstEnv.Results.Results))
	}
	// Row 0 (untouched) must be BYTE-IDENTICAL to the source — proves it was never
	// decoded/re-marshaled through a typed struct.
	if string(dstEnv.Results.Results[0]) != untouchedRow {
		t.Errorf("want untouched row byte-identical to source, got:\n%s\nwant:\n%s", dstEnv.Results.Results[0], untouchedRow)
	}

	// Row 1 (retried) must be patched.
	var patchedRow map[string]any
	if err := json.Unmarshal(dstEnv.Results.Results[1], &patchedRow); err != nil {
		t.Fatal(err)
	}
	response, _ := patchedRow["response"].(map[string]any)
	if response["output"] != `{"reply_text":"fixed","reply_language":"ru","media_files_to_send":[],"escalate":false}` {
		t.Errorf("want retried row's response.output patched, got %v", response["output"])
	}
	if patchedRow["retries"] != float64(1) {
		t.Errorf("want retried row's retries=1, got %v", patchedRow["retries"])
	}
}

func TestPatchResultsFile_UsesEachCandidatesProvider(t *testing.T) {
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "src.results.json")
	dstPath := filepath.Join(dir, "dst.results.json")
	rowA := `{"response":{"output":"broken a","finishReason":"length"},"testCase":{"description":"a"},"provider":{"id":"openrouter:vendor/model-a","label":"fast"},"prompt":{"raw":"prompt a"}}`
	rowB := `{"response":{"output":"broken b","finishReason":"length"},"testCase":{"description":"b"},"provider":{"id":"openrouter:vendor/model-b","label":"accurate"},"prompt":{"raw":"prompt b"}}`
	if err := os.WriteFile(srcPath, []byte(`{"results":{"results":[`+rowA+`,`+rowB+`]}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	modelsSeen := []string{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req orRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode request: %v", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		modelsSeen = append(modelsSeen, req.Model)
		resp := orResponse{}
		resp.Choices = []orChoice{{FinishReason: "stop"}}
		resp.Choices[0].Message.Content = `{"reply_text":"fixed","reply_language":"ru","media_files_to_send":[],"escalate":false,"escalation_reason":"","confidence":0.9}`
		resp.Usage = &orUsage{PromptTokens: 10, CompletionTokens: 5}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	var typedResults PromptfooResults
	if err := readJSON(srcPath, &typedResults); err != nil {
		t.Fatal(err)
	}
	providers := map[int]ModelProvider{
		0: {ID: "openrouter:vendor/model-a", Label: "fast", MaxTokens: 100},
		1: {ID: "openrouter:vendor/model-b", Label: "accurate", MaxTokens: 200},
	}
	retried, err := patchResultsFile(
		srcPath,
		dstPath,
		[]retryCandidate{{Index: 0, Reason: retryReasonContractShape}, {Index: 1, Reason: retryReasonContractShape}},
		typedResults.Results.Results,
		newORClientWithTimeout(srv.URL, "test-key", 5*time.Second),
		providers,
		"parent-sha",
		"retry-sha",
		map[string]bool{},
		nil,
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	if retried != 2 {
		t.Fatalf("retried = %d, want 2", retried)
	}
	want := []string{"vendor/model-a", "vendor/model-b"}
	if len(modelsSeen) != len(want) {
		t.Fatalf("models seen = %v, want %v", modelsSeen, want)
	}
	for i := range want {
		if modelsSeen[i] != want[i] {
			t.Fatalf("models seen = %v, want %v", modelsSeen, want)
		}
	}
}

// TestRetryMediaNotFoundInPlace_RetriesOnlyMediaNotFoundCandidatesInPlace is
// cmdRun's `-retry-media` opt-in path exercised directly (bypassing cmdRun/promptfoo
// entirely, the same way patchResultsFile/retryOneRow are tested at their own level):
// a row naming an out-of-catalog media token is retried and overwritten IN PLACE in
// the same results.json path, labeled retry_reason=media_not_found; an already-fine
// row is left completely untouched.
func TestRetryMediaNotFoundInPlace_RetriesOnlyMediaNotFoundCandidatesInPlace(t *testing.T) {
	root := t.TempDir()
	sd := filepath.Join(root, "scenarios", "media-fixture")
	genDir := filepath.Join(sd, "generated")
	if err := os.MkdirAll(genDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sd, "scenario.yaml"), []byte("name: media-fixture\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Empty catalog (no media tokens at all) -> any media_files_to_send entry counts
	// as out-of-catalog, i.e. a media_not_found candidate.
	if err := writeJSON(filepath.Join(genDir, "catalog.json"), Catalog{}); err != nil {
		t.Fatal(err)
	}

	runDir := filepath.Join(root, "runs", "2026-01-01_00-00-00-r1")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	mediaNotFoundRow := PromptfooRow{}
	mediaNotFoundRow.Provider.ID = "openrouter:test/model"
	mediaNotFoundRow.TestCase.Description = "media-not-found test"
	mediaNotFoundRow.Prompt.Raw = "the exact rendered prompt"
	mediaNotFoundRow.Response.Output = `{"reply_text":"вот фото","reply_language":"ru","media_files_to_send":["totally.invented.token"],"escalate":false,"escalation_reason":"","confidence":0.9}`

	okRow := PromptfooRow{}
	okRow.Provider.ID = "openrouter:test/model"
	okRow.TestCase.Description = "ok test"
	okRow.Response.Output = `{"reply_text":"ok","reply_language":"ru","media_files_to_send":[],"escalate":false,"escalation_reason":"","confidence":0.9}`

	results := PromptfooResults{}
	results.Results.Results = []PromptfooRow{mediaNotFoundRow, okRow}
	resultsPath := filepath.Join(runDir, "media-fixture.results.json")
	if err := writeJSON(resultsPath, results); err != nil {
		t.Fatal(err)
	}

	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		resp := orResponse{}
		resp.Choices = []orChoice{{FinishReason: "stop"}}
		resp.Choices[0].Message.Content = `{"reply_text":"fixed","reply_language":"ru","media_files_to_send":[],"escalate":false}`
		resp.Usage = &orUsage{PromptTokens: 50, CompletionTokens: 20}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	scenario := &ScenarioConfig{Name: "media-fixture"}
	models := &ModelsFile{Providers: []ModelProvider{{ID: "openrouter:test/model"}}}

	retried, err := retryMediaNotFoundInPlace(sd, runDir, scenario, models, "models-sha", srv.URL, "test-key")
	if err != nil {
		t.Fatalf("retryMediaNotFoundInPlace: %v", err)
	}
	if retried != 1 {
		t.Fatalf("want exactly 1 row retried (the media_not_found one), got %d", retried)
	}
	if calls != 1 {
		t.Fatalf("want exactly 1 HTTP call, got %d", calls)
	}

	// The SAME results.json path must now reflect the retry, in place.
	var patched PromptfooResults
	if err := readJSON(resultsPath, &patched); err != nil {
		t.Fatal(err)
	}
	if len(patched.Results.Results) != 2 {
		t.Fatalf("want 2 rows still present, got %d", len(patched.Results.Results))
	}
	retriedRow := patched.Results.Results[0]
	if retriedRow.Retries != 1 || retriedRow.RetryReason != string(retryReasonMediaNotFound) {
		t.Errorf("want row 0 retried with reason=media_not_found, got Retries=%d RetryReason=%q", retriedRow.Retries, retriedRow.RetryReason)
	}
	if len(retriedRow.Attempts) != 2 {
		t.Fatalf("want 2 attempts recorded (original + retry), got %d", len(retriedRow.Attempts))
	}
	okRowAfter := patched.Results.Results[1]
	if okRowAfter.Retries != 0 {
		t.Errorf("want the already-fine row completely untouched, got Retries=%d", okRowAfter.Retries)
	}
}

// TestRetryMediaNotFoundInPlace_NothingToRetryIsANoOp confirms a scenario with no
// media_not_found candidates makes zero HTTP calls and reports 0 retried.
func TestRetryMediaNotFoundInPlace_NothingToRetryIsANoOp(t *testing.T) {
	root := t.TempDir()
	sd := filepath.Join(root, "scenarios", "clean-fixture")
	genDir := filepath.Join(sd, "generated")
	if err := os.MkdirAll(genDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sd, "scenario.yaml"), []byte("name: clean-fixture\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writeJSON(filepath.Join(genDir, "catalog.json"), Catalog{}); err != nil {
		t.Fatal(err)
	}
	runDir := filepath.Join(root, "runs", "2026-01-01_00-00-00-r1")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	okRow := PromptfooRow{}
	okRow.Provider.ID = "openrouter:test/model"
	okRow.Response.Output = `{"reply_text":"ok","reply_language":"ru","media_files_to_send":[],"escalate":false,"escalation_reason":"","confidence":0.9}`
	results := PromptfooResults{}
	results.Results.Results = []PromptfooRow{okRow}
	if err := writeJSON(filepath.Join(runDir, "clean-fixture.results.json"), results); err != nil {
		t.Fatal(err)
	}

	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
	}))
	defer srv.Close()

	scenario := &ScenarioConfig{Name: "clean-fixture"}
	models := &ModelsFile{Providers: []ModelProvider{{ID: "openrouter:test/model"}}}
	retried, err := retryMediaNotFoundInPlace(sd, runDir, scenario, models, "models-sha", srv.URL, "test-key")
	if err != nil {
		t.Fatalf("retryMediaNotFoundInPlace: %v", err)
	}
	if retried != 0 || calls != 0 {
		t.Fatalf("want 0 retried and 0 calls when nothing is a candidate, got retried=%d calls=%d", retried, calls)
	}
}

// setupRetryFixture builds a minimal but complete parent run (scenario dir + a run dir
// with manifest/snapshot/results.json) under a fresh runsRoot, ready for cmdRetry.
// Returns scenarioDir, parentRunDir, retryModelsPath (a temp models.yaml distinct from
// the parent's own snapshotted one, matching a real repair's "config may differ"
// scenario) and the fake OpenRouter server's URL.
func setupRetryFixture(t *testing.T, srv *httptest.Server) (scenarioDir, parentRunDir, retryModelsPath string) {
	t.Helper()
	root := t.TempDir()
	runsRoot := filepath.Join(root, "runs")
	if err := os.MkdirAll(runsRoot, 0o755); err != nil {
		t.Fatal(err)
	}

	scenarioDir = filepath.Join(root, "scenarios", "fixture-scenario")
	if err := os.MkdirAll(scenarioDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(scenarioDir, "scenario.yaml"), []byte("name: fixture-scenario\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	parentRunDir = filepath.Join(runsRoot, "2026-01-01_00-00-00-par1")
	if err := os.MkdirAll(parentRunDir, 0o755); err != nil {
		t.Fatal(err)
	}
	snapDir := filepath.Join(parentRunDir, "snapshots", "fixture-scenario")
	if err := os.MkdirAll(snapDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(snapDir, "scenario.yaml"), []byte("name: fixture-scenario\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cat := Catalog{}
	if err := writeJSON(filepath.Join(snapDir, "catalog.json"), cat); err != nil {
		t.Fatal(err)
	}
	resolved := ResolvedTests{Tests: []TestCase{
		{ID: "t1", Message: "hi"},
		{ID: "t2", Message: "hi2"},
	}}
	if err := writeJSON(filepath.Join(snapDir, "resolved_tests.json"), resolved); err != nil {
		t.Fatal(err)
	}

	parentModelsYAML := "pricing_source: test\npricing_checked_at: \"2026-01-01\"\nproviders:\n  - id: openrouter:test/model\n    temperature: 0.3\n    max_tokens: 500\n"
	if err := os.WriteFile(filepath.Join(parentRunDir, "snapshots", "models.yaml"), []byte(parentModelsYAML), 0o644); err != nil {
		t.Fatal(err)
	}

	results := PromptfooResults{}
	okRow := PromptfooRow{}
	okRow.Provider.ID = "openrouter:test/model"
	okRow.TestCase.Description = "t1"
	okRow.Response.Output = `{"reply_text":"ok","reply_language":"ru","media_files_to_send":[],"escalate":false,"escalation_reason":"","confidence":0.9}`
	brokenRow := PromptfooRow{}
	brokenRow.Provider.ID = "openrouter:test/model"
	brokenRow.TestCase.Description = "t2"
	brokenRow.Response.Output = "Thinking: never finished"
	brokenRow.Response.FinishReason = "length"
	brokenRow.Prompt.Raw = "the exact rendered prompt for t2"
	results.Results.Results = []PromptfooRow{okRow, brokenRow}
	if err := writeJSON(filepath.Join(parentRunDir, "fixture-scenario.results.json"), results); err != nil {
		t.Fatal(err)
	}

	parentManifest := provenance.NewManifest(parentRunDir, "2026-01-01_00-00-00-par1", "scenario", "run", nil)
	parentManifest.ModelsPath = "models.yaml"
	parentManifest.Scenarios = []provenance.ScenarioSnapshotRef{{
		Scenario:      "fixture-scenario",
		FixtureSHA256: "parent-fixture-sha",
		PromptSHA256:  "parent-prompt-sha",
	}}
	if sha, err := provenance.SHA256File(filepath.Join(parentRunDir, "snapshots", "models.yaml")); err == nil {
		parentManifest.ModelsSHA256 = sha
	}
	if err := provenance.WriteManifest(parentRunDir, parentManifest); err != nil {
		t.Fatal(err)
	}

	retryModelsPath = filepath.Join(root, "models.retry.yaml")
	retryModelsYAML := "pricing_source: test\npricing_checked_at: \"2026-01-01\"\nproviders:\n  - id: openrouter:test/model\n    temperature: 0.3\n    max_tokens: 2000\n"
	if err := os.WriteFile(retryModelsPath, []byte(retryModelsYAML), 0o644); err != nil {
		t.Fatal(err)
	}
	return scenarioDir, parentRunDir, retryModelsPath
}

// TestCmdRetry_ArchivedScenario_HardErrorsBeforeAnyCall confirms `harness retry` never
// bills a call against an archived scenario — an explicit request naming a retired
// scenario is a mistake worth failing loudly on, the same doctrine as cmdRun's own
// -scenario path (run.go), not the -all silent skip (a retry always names one
// scenario). resolveScenarioRunInputs prefers a run's OWN snapshotted scenario.yaml
// over the live one when present, so both copies are archived here to exercise the
// actual resolution path this fixture takes.
func TestCmdRetry_ArchivedScenario_HardErrorsBeforeAnyCall(t *testing.T) {
	t.Setenv("OPENROUTER_API_KEY", "test-key")
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
	}))
	defer srv.Close()

	scenarioDir, parentRunDir, retryModelsPath := setupRetryFixture(t, srv)
	archivedYAML := []byte("name: fixture-scenario\narchived: true\narchived_reason: superseded\n")
	if err := os.WriteFile(filepath.Join(scenarioDir, "scenario.yaml"), archivedYAML, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(parentRunDir, "snapshots", "fixture-scenario", "scenario.yaml"), archivedYAML, 0o644); err != nil {
		t.Fatal(err)
	}

	err := cmdRetry([]string{
		"-scenario", scenarioDir,
		"-run", parentRunDir,
		"-models-file", retryModelsPath,
		"-base-url", srv.URL,
	})
	if err == nil {
		t.Fatal("want an error for an archived scenario, got nil")
	}
	if !strings.Contains(err.Error(), "archived") {
		t.Errorf("want the error to mention archival, got: %v", err)
	}
	if calls != 0 {
		t.Fatalf("want zero HTTP calls (refused before any spend), got %d", calls)
	}
}

// TestCmdRetry_EndToEnd_RepairsAndIsIdempotentOnSecondInvocation is the integration
// proof for Part 6: a real `harness retry` invocation creates a derivative run with the
// broken row fixed (and the clean row byte-identical to the parent), then a SECOND
// invocation against the exact same parent+retry-config refuses instead of spending
// again — the idempotent-CREATION guarantee, not just the row-level predicate.
func TestCmdRetry_EndToEnd_RepairsAndIsIdempotentOnSecondInvocation(t *testing.T) {
	t.Setenv("OPENROUTER_API_KEY", "test-key")
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		resp := orResponse{}
		resp.Choices = []orChoice{{FinishReason: "stop"}}
		resp.Choices[0].Message.Content = `{"reply_text":"fixed t2","reply_language":"ru","media_files_to_send":[],"escalate":false}`
		resp.Usage = &orUsage{PromptTokens: 50, CompletionTokens: 20}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	scenarioDir, parentRunDir, retryModelsPath := setupRetryFixture(t, srv)
	runsRoot := filepath.Dir(parentRunDir)

	args := []string{
		"-scenario", scenarioDir,
		"-run", parentRunDir,
		"-models-file", retryModelsPath,
		"-expect-retry-calls", "1",
		"-base-url", srv.URL,
	}
	if err := cmdRetry(args); err != nil {
		t.Fatalf("first cmdRetry invocation: %v", err)
	}
	if calls != 1 {
		t.Fatalf("want exactly 1 HTTP call on the first invocation, got %d", calls)
	}

	// Exactly one NEW derivative dir must exist alongside the parent.
	entries, err := os.ReadDir(runsRoot)
	if err != nil {
		t.Fatal(err)
	}
	var derivativeID string
	for _, e := range entries {
		if e.IsDir() && e.Name() != "2026-01-01_00-00-00-par1" {
			derivativeID = e.Name()
		}
	}
	if derivativeID == "" {
		t.Fatal("want a new derivative run directory to have been created")
	}
	derivDir := filepath.Join(runsRoot, derivativeID)

	var derivManifest provenance.Manifest
	if err := readJSON(filepath.Join(derivDir, "manifest.json"), &derivManifest); err != nil {
		t.Fatal(err)
	}
	if derivManifest.RepairedFrom != "2026-01-01_00-00-00-par1" {
		t.Errorf("want manifest.repaired_from to name the parent, got %q", derivManifest.RepairedFrom)
	}
	if derivManifest.RepairKey == "" {
		t.Error("want a non-empty repair key recorded")
	}
	if len(derivManifest.Scenarios) != 1 ||
		derivManifest.Scenarios[0].FixtureSHA256 != "parent-fixture-sha" ||
		derivManifest.Scenarios[0].PromptSHA256 != "parent-prompt-sha" ||
		derivManifest.Scenarios[0].ResultsSHA256 == "" {
		t.Fatalf("derivative scenario provenance was not copied and updated: %+v", derivManifest.Scenarios)
	}
	for _, name := range []string{"models.parent.yaml", "models.retry.yaml", "models.yaml"} {
		if _, err := os.Stat(filepath.Join(derivDir, "snapshots", name)); err != nil {
			t.Errorf("want snapshots/%s to exist in the derivative: %v", name, err)
		}
	}

	var jr JudgedRun
	if err := readJSON(filepath.Join(derivDir, "fixture-scenario.judged.json"), &jr); err != nil {
		t.Fatal(err)
	}
	if len(jr.Verdicts) != 2 {
		t.Fatalf("want 2 verdicts in the repaired report, got %d", len(jr.Verdicts))
	}
	for _, v := range jr.Verdicts {
		if v.TestID == "t2" {
			if !v.ParseOK {
				t.Errorf("want t2 to now parse after the retry, reason=%q", v.Reason)
			}
			if v.Retries != 1 {
				t.Errorf("want t2's verdict to carry retries=1, got %d", v.Retries)
			}
			if !v.RetryRecovered {
				t.Error("want t2 to be flagged RetryRecovered")
			}
		}
	}

	// SECOND invocation against the exact same parent + retry config: must refuse,
	// zero additional HTTP calls — the idempotent-CREATION guarantee.
	err = cmdRetry(args)
	if err == nil {
		t.Fatal("want the second invocation to refuse (a derivative with this repair key already exists)")
	}
	if calls != 1 {
		t.Errorf("want NO additional HTTP calls on the refused second invocation, total calls = %d", calls)
	}
}
