package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"xchats-evals-harness/internal/kbfixture"
	"xchats-evals-harness/internal/provenance"

	"github.com/yerassyldanay/xchats/backend/aiprompt"
)

// retryTimeout is the HTTP timeout for a single retry call — kimi-k2.5 has been
// observed taking ~124s on a heavy-reasoning call; describeImage's default 90s
// (openrouter.go) would cut a retry off before it ever finishes.
const retryTimeout = 180 * time.Second

// retryCandidateIndexes returns the indexes (into rows, order preserved) of every row
// this run should retry — unparseable JSON, JSON that fails the strict six-field
// contract schema (missing/wrong-typed field), or a media_files_to_send token outside
// validMedia — AND not already retried once. Delegates the actual determination to
// judgeOne against an empty TestCase{} (no Requires/Media/Escalate/Language
// expectations to evaluate — those judge MODEL BEHAVIOR on a specific test, a different
// question from "is the contract itself well-formed") so candidacy can never drift from
// what judgeScenario itself considers ParseOK/ContractFields/UnknownMedia. A row whose
// contract is well-formed but whose model behavior is simply wrong (bad requires, wrong
// escalate, forbidden phrase, ...) is deliberately NOT retried: a second roll of the
// same prompt cannot fix a genuine behavior problem, only a pipeline-shape one, and
// retrying it would spend money without addressing anything a retry can actually help
// with.
func retryCandidateIndexes(rows []PromptfooRow, validMedia map[string]bool) []int {
	var out []int
	for i, row := range rows {
		if row.Retries > 0 {
			continue
		}
		v := judgeOne(TestCase{}, row, map[string]string{}, validMedia, nil)
		if !v.ParseOK || !v.ContractFields || len(v.UnknownMedia) > 0 {
			out = append(out, i)
		}
	}
	return out
}

// retryCandidateIndexesSchemaKB is retryCandidateIndexes' counterpart for
// pipeline:schema_kb_v1 rows — delegates to judgeOneSchemaKB (aiprompt.ValidateResponse)
// instead, which additionally catches an unknown JSON property (additionalProperties:
// false) the legacy hand-rolled check in judgeOne has no equivalent for.
func retryCandidateIndexesSchemaKB(rows []PromptfooRow, kb *aiprompt.KB, cat *aiprompt.Catalog) []int {
	var out []int
	for i, row := range rows {
		if row.Retries > 0 {
			continue
		}
		v := judgeOneSchemaKB(TestCase{}, row, kb, cat, map[string]string{}, nil)
		if !v.ParseOK || !v.ContractFields || len(v.UnknownMedia) > 0 {
			out = append(out, i)
		}
	}
	return out
}

// correctiveFeedbackText returns Russian feedback appended to a retry's prompt when the
// original response's specific problem is one the model can act on — right now, only
// "you named a media_files_to_send token that doesn't exist in the catalog," naming
// exactly which tokens were invalid and instructing the model to copy tokens verbatim
// from the prompt's own MEDIA catalog rather than paraphrasing or inventing one. Empty
// for every other retry-candidate reason (unparseable JSON, a missing/wrong-typed
// field): there is no specific named problem to point at the way there is for an
// identifiable bad token, so the same repeated prompt is sent unchanged and left to
// self-correct on the second attempt.
func correctiveFeedbackText(raw string, validMedia map[string]bool) string {
	obj, ok := extractModelJSON(raw, nil)
	if !ok {
		return ""
	}
	rawMedia, _ := obj[mediaFilesField].([]any)
	var badTokens []string
	for _, e := range rawMedia {
		s, ok := e.(string)
		if !ok || validMedia[s] {
			continue
		}
		badTokens = append(badTokens, s)
	}
	if len(badTokens) == 0 {
		return ""
	}
	return fmt.Sprintf(
		"\n\nВАЖНО — исправь и попробуй снова: в предыдущем ответе поле %q содержало значения, "+
			"которых нет в каталоге MEDIA выше: %s. Скопируй нужный токен ТОЧНО из каталога — "+
			"не изменяй, не сокращай и не придумывай новый.",
		mediaFilesField, strings.Join(badTokens, ", "),
	)
}

// providerByIDLabel indexes a ModelsFile the same way judge.go's providerModelKey
// groups verdicts — by (id, label) together, never id alone, so a retry against a
// models.yaml carrying two labeled variants of the same underlying model (a
// reasoning-on/off pair) always resolves the ENTRY that produced the row being
// retried, not an arbitrary same-id sibling.
func providerByIDLabel(models *ModelsFile) map[string]ModelProvider {
	out := map[string]ModelProvider{}
	for _, p := range models.Providers {
		out[providerModelKey(p.ID, p.Label)] = p
	}
	return out
}

// gitHEAD is a tiny, best-effort git-SHA lookup local to package main — deliberately
// not reusing provenance's unexported gitCapture (that package's git helpers are
// private to its own Manifest-building code). A repair key still needs SOME notion of
// "which harness code produced this derivative," and best-effort (empty on any git
// failure) matches provenance's own philosophy: git provenance must never fail the
// command it's describing.
func gitHEAD() string {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "git", "rev-parse", "HEAD").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// copyDir recursively copies src's regular files into dst (creating directories as
// needed) — used to carry a parent run's snapshot directory into a derivative run
// unchanged, since the derivative judges against the SAME rendered prompt/catalog/tests
// the parent did (no re-render happens for a repair).
func copyDir(src, dst string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	for _, e := range entries {
		srcPath := filepath.Join(src, e.Name())
		dstPath := filepath.Join(dst, e.Name())
		if e.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
			continue
		}
		b, err := os.ReadFile(srcPath)
		if err != nil {
			return err
		}
		if err := provenance.AtomicWriteFile(dstPath, b, 0o644); err != nil {
			return err
		}
	}
	return nil
}

// findDerivativeByRepairKey scans every manifest.json under runsRoot for one already
// carrying this exact repair key — the idempotent-CREATION guard `cmdRetry` checks
// before minting a new run dir or spending anything, so re-invoking `harness retry`
// against an already-repaired (parent, retry-config, code-version) combination refuses
// instead of silently billing the retries again.
func findDerivativeByRepairKey(runsRoot, repairKey string) (runID string, found bool) {
	entries, err := os.ReadDir(runsRoot)
	if err != nil {
		return "", false
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		manifestPath := filepath.Join(runsRoot, e.Name(), "manifest.json")
		var m provenance.Manifest
		if err := readJSON(manifestPath, &m); err != nil {
			continue
		}
		if m.RepairKey != "" && m.RepairKey == repairKey {
			return e.Name(), true
		}
	}
	return "", false
}

// resultsEnvelope mirrors the outermost {"results":{"results":[...]}} shape of
// promptfoo's -o results.json, keeping the inner row array as raw JSON — see
// patchResultsFile's doc comment for why untouched rows must never round-trip through
// a typed Go struct.
type resultsEnvelope struct {
	Results struct {
		Results []json.RawMessage `json:"results"`
	} `json:"results"`
}

// patchResultsFile reads a results.json, retries every row in candidateIdx via
// client.chatText, and writes a NEW results.json (at dstPath) where:
//   - every row NOT in candidateIdx is byte-identical to the source (never decoded
//     into a typed struct and re-marshaled — that would reorder keys and could drop
//     fields this harness doesn't model, e.g. promptfoo's own gradingResult/vars/
//     metadata) — this is the losslessness the RawMessage design exists for;
//   - every retried row gets its Attempts/SelectedAttempt/Retries populated, and its
//     top-level response.output/response.finishReason/response.tokenUsage/tokenUsage/
//     latencyMs updated to the selected attempt (summed across attempts for
//     tokenUsage/latency — real spend, real wall time).
//
// Returns the count actually retried (for the caller's summary line) and any error —
// a single HTTP failure on one row does NOT abort the whole batch (see the
// per-row error handling below); it's recorded as a failed attempt on that row only.
func patchResultsFile(srcPath, dstPath string, candidateIdx []int, typedRows []PromptfooRow,
	client *orClient, providersByCandidate map[int]ModelProvider, parentModelsSHA, retryModelsSHA string, validMedia map[string]bool) (retried int, err error) {

	raw, err := os.ReadFile(srcPath)
	if err != nil {
		return 0, err
	}
	var env resultsEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return 0, fmt.Errorf("parse %s: %w", srcPath, err)
	}
	rows := env.Results.Results
	candidateSet := map[int]bool{}
	for _, i := range candidateIdx {
		candidateSet[i] = true
	}

	for i := range rows {
		if !candidateSet[i] {
			continue // byte-identical raw message, untouched
		}
		row := typedRows[i]
		provider, ok := providersByCandidate[i]
		if !ok {
			return retried, fmt.Errorf("retry row %d (%s / %s): no preflighted provider", i, row.Provider.ID, row.TestCase.Description)
		}
		patched, callErr := retryOneRow(client, provider, row, parentModelsSHA, retryModelsSHA, validMedia)
		if callErr != nil {
			return retried, fmt.Errorf("retry row %d (%s / %s): %w", i, row.Provider.ID, row.TestCase.Description, callErr)
		}
		newRow, err := mergeRowPatch(rows[i], patched)
		if err != nil {
			return retried, fmt.Errorf("patch row %d: %w", i, err)
		}
		rows[i] = newRow
		retried++
	}

	env.Results.Results = rows
	out, err := json.Marshal(env)
	if err != nil {
		return retried, err
	}
	return retried, provenance.AtomicWriteFile(dstPath, out, 0o644)
}

// rowPatch is the set of fields patchResultsFile actually overwrites on a retried row —
// kept as its own small struct (rather than mutating a full PromptfooRow and
// re-marshaling it, which would lose any field this harness doesn't model) so
// mergeRowPatch can apply it onto the row's ORIGINAL raw JSON via a targeted
// unmarshal-mutate-remarshal of only these keys.
type rowPatch struct {
	Retries         int             `json:"retries"`
	Attempts        []ResultAttempt `json:"attempts"`
	SelectedAttempt int             `json:"selected_attempt"`
	Response        struct {
		Output       string `json:"output"`
		FinishReason string `json:"finishReason"`
		TokenUsage   struct {
			Total      int `json:"total"`
			Prompt     int `json:"prompt"`
			Completion int `json:"completion"`
		} `json:"tokenUsage"`
	} `json:"response"`
	TokenUsage struct {
		Total      int `json:"total"`
		Prompt     int `json:"prompt"`
		Completion int `json:"completion"`
	} `json:"tokenUsage"`
	LatencyMs int `json:"latencyMs"`
}

// retryOneRow makes the actual retry call for one row and builds its rowPatch —
// preserving the ORIGINAL attempt (from the row as promptfoo/the parent recorded it)
// as attempt 0, appending the new attempt as attempt 1. On an HTTP-level failure, the
// original stays selected (SelectedAttempt: 0) and the failed attempt is recorded with
// its own Error — retries is still incremented to 1 so a second `harness retry`
// invocation is a no-op (idempotent row predicate), never a repeated spend.
func retryOneRow(client *orClient, provider ModelProvider, row PromptfooRow, parentModelsSHA, retryModelsSHA string, validMedia map[string]bool) (rowPatch, error) {
	original := ResultAttempt{
		Output:            row.Response.Output,
		FinishReason:      row.Response.FinishReason,
		ModelConfigSHA256: parentModelsSHA,
	}
	original.TokenUsage.Total = row.TokenUsage.Total
	original.TokenUsage.Prompt = row.TokenUsage.Prompt
	original.TokenUsage.Completion = row.TokenUsage.Completion
	original.LatencyMs = row.LatencyMs

	var p rowPatch
	p.Attempts = []ResultAttempt{original}

	promptToSend := row.Prompt.Raw + correctiveFeedbackText(row.Response.Output, validMedia)

	ctx, cancel := context.WithTimeout(context.Background(), retryTimeout)
	defer cancel()
	result, callErr := client.chatText(ctx, provider, promptToSend)

	if callErr != nil {
		failed := ResultAttempt{ModelConfigSHA256: retryModelsSHA, Error: callErr.Error()}
		p.Attempts = append(p.Attempts, failed)
		p.Retries = 1
		p.SelectedAttempt = 0
		p.Response.Output = original.Output
		p.Response.FinishReason = original.FinishReason
		p.Response.TokenUsage = original.TokenUsage
		p.TokenUsage.Total = original.TokenUsage.Total
		p.TokenUsage.Prompt = original.TokenUsage.Prompt
		p.TokenUsage.Completion = original.TokenUsage.Completion
		p.LatencyMs = original.LatencyMs
		return p, nil // HTTP failure on the RETRY call is not a fatal error for the batch
	}

	reasoningTokens := 0
	if result.Usage.CompletionTokensDetails != nil && result.Usage.CompletionTokensDetails.ReasoningTokens != nil {
		reasoningTokens = *result.Usage.CompletionTokensDetails.ReasoningTokens
	}
	retried := ResultAttempt{
		Output:             result.Raw,
		FinishReason:       result.FinishReason,
		NativeFinishReason: result.NativeFinishReason,
		ResponseID:         result.ResponseID,
		UpstreamProvider:   result.UpstreamProvider,
		ReasoningTokens:    reasoningTokens,
		ModelConfigSHA256:  retryModelsSHA,
	}
	retried.TokenUsage.Total = result.Usage.PromptTokens + result.Usage.CompletionTokens
	retried.TokenUsage.Prompt = result.Usage.PromptTokens
	retried.TokenUsage.Completion = result.Usage.CompletionTokens
	p.Attempts = append(p.Attempts, retried)
	p.Retries = 1
	p.SelectedAttempt = 1
	p.Response.Output = retried.Output
	p.Response.FinishReason = retried.FinishReason
	// Real spend, real wall time: SUM across every attempt, not just the selected one —
	// both attempts were real billed API calls.
	p.TokenUsage.Total = original.TokenUsage.Total + retried.TokenUsage.Total
	p.TokenUsage.Prompt = original.TokenUsage.Prompt + retried.TokenUsage.Prompt
	p.TokenUsage.Completion = original.TokenUsage.Completion + retried.TokenUsage.Completion
	p.Response.TokenUsage = p.TokenUsage
	p.LatencyMs = original.LatencyMs + retried.LatencyMs
	return p, nil
}

// mergeRowPatch applies a rowPatch onto a row's ORIGINAL raw JSON by decoding into a
// generic map, overwriting only the patch's keys, and re-marshaling — every OTHER key
// promptfoo wrote (gradingResult, namedScores, vars, metadata, id, promptId, ...) is
// preserved untouched, exactly as read, never modeled or dropped by this harness.
func mergeRowPatch(original json.RawMessage, patch rowPatch) (json.RawMessage, error) {
	var obj map[string]any
	if err := json.Unmarshal(original, &obj); err != nil {
		return nil, err
	}
	response, _ := obj["response"].(map[string]any)
	if response == nil {
		response = map[string]any{}
	}
	response["output"] = patch.Response.Output
	response["finishReason"] = patch.Response.FinishReason
	response["tokenUsage"] = patch.Response.TokenUsage
	obj["response"] = response
	obj["tokenUsage"] = patch.TokenUsage
	obj["latencyMs"] = patch.LatencyMs
	obj["retries"] = patch.Retries
	obj["attempts"] = patch.Attempts
	obj["selected_attempt"] = patch.SelectedAttempt
	return json.Marshal(obj)
}

// cmdRetry implements `harness retry` — Part 6 of the retry-mechanism plan: repair an
// EXISTING run's unparseable/empty rows by retrying them once each, as a NEW derivative
// run (the parent is never modified). See docs in main.go's usage() for the flag
// summary; the design rationale (why a derivative, why dual model-config snapshots, why
// the repair-key idempotency guard) lives in provenance.Manifest's doc comment.
func cmdRetry(args []string) error {
	fs := flag.NewFlagSet("retry", flag.ExitOnError)
	scenarioDir := fs.String("scenario", "", "path to the scenario directory (must match one the parent run judged)")
	parentRunDir := fs.String("run", "", "path to the PARENT run directory to repair")
	modelsPath := fs.String("models-file", "models.yaml", "path to models.yaml — the RETRY config used for retried calls (may differ from the parent's own snapshotted config)")
	expectRetryCalls := fs.Int("expect-retry-calls", 0, "if >0, hard-fail before spending anything unless the resolved retry-candidate row count matches exactly")
	forceNew := fs.Bool("force-new-derivative", false, "create a new derivative even if one with the identical (parent, retry-config, code-version) repair key already exists")
	baseURL := fs.String("base-url", "", "override the OpenRouter base URL (default: $EVAL_BASE_URL, else https://openrouter.ai/api/v1) — same convention as `harness extract`, primarily for pointing at a fake server in tests")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *scenarioDir == "" || *parentRunDir == "" {
		return fmt.Errorf("retry: -scenario and -run are both required")
	}
	apiKey := os.Getenv("OPENROUTER_API_KEY")
	if apiKey == "" {
		return fmt.Errorf("retry: OPENROUTER_API_KEY is not set (this makes real, billed model calls)")
	}
	base := *baseURL
	if base == "" {
		base = envOrDefault("EVAL_BASE_URL", "https://openrouter.ai/api/v1")
	}

	inputs, err := resolveScenarioRunInputs(*scenarioDir, *parentRunDir)
	if err != nil {
		return err
	}
	scenario := inputs.Scenario

	var parentManifest provenance.Manifest
	parentManifestPath := filepath.Join(*parentRunDir, "manifest.json")
	if err := readJSON(parentManifestPath, &parentManifest); err != nil {
		return fmt.Errorf("read parent manifest %s: %w", parentManifestPath, err)
	}

	parentResultsPath := filepath.Join(*parentRunDir, scenario.Name+".results.json")
	var typedResults PromptfooResults
	if err := readJSON(parentResultsPath, &typedResults); err != nil {
		return fmt.Errorf("read %s: %w", parentResultsPath, err)
	}
	rows := typedResults.Results.Results

	// Candidacy is judged against exactly what THIS parent run graded against — the
	// same snapshot-preference judgeScenario itself uses — not whatever the live
	// scenario looks like today.
	var kb *aiprompt.KB
	var cat *aiprompt.Catalog
	validMedia := map[string]bool{}
	var candidateIdx []int
	if scenario.Pipeline == "schema_kb_v1" {
		kb, err = kbfixture.Load(inputs.FixturePath)
		if err != nil {
			return fmt.Errorf("load fixture %s: %w", inputs.FixturePath, err)
		}
		if kb, err = kbfixture.ApplyLimits(kb, scenario.Limits); err != nil {
			return fmt.Errorf("scenario %q: %w", scenario.Name, err)
		}
		cat, err = aiprompt.BuildCatalog(kb)
		if err != nil {
			return fmt.Errorf("scenario %q: build catalog: %w", scenario.Name, err)
		}
		for _, m := range cat.Media {
			validMedia[m.Token] = true
		}
		candidateIdx = retryCandidateIndexesSchemaKB(rows, kb, cat)
	} else {
		var catalog Catalog
		if err := readJSON(filepath.Join(inputs.GeneratedDir, "catalog.json"), &catalog); err != nil {
			return fmt.Errorf("read catalog.json (did you run render first?): %w", err)
		}
		for _, tok := range catalog.MediaTokens {
			validMedia[tok] = true
		}
		candidateIdx = retryCandidateIndexes(rows, validMedia)
	}
	fmt.Printf("retry: %d retry-candidate row(s) in %s (parent %s)\n", len(candidateIdx), scenario.Name, filepath.Base(*parentRunDir))
	if *expectRetryCalls > 0 && len(candidateIdx) != *expectRetryCalls {
		return fmt.Errorf("retry: resolved %d candidate rows, -expect-retry-calls wanted %d — refusing to spend anything; adjust -run/-expect-retry-calls if this is intentional", len(candidateIdx), *expectRetryCalls)
	}
	if len(candidateIdx) == 0 {
		fmt.Println("retry: nothing to retry — every row already parses or was already retried")
		return nil
	}

	retryModels, err := loadModels(*modelsPath)
	if err != nil {
		return fmt.Errorf("load %s: %w", *modelsPath, err)
	}
	retryProviders := providerByIDLabel(retryModels)

	parentModelsPath := provenance.SnapshotModelsPath(*parentRunDir, *modelsPath)
	parentModelsSHA, err := provenance.SHA256File(parentModelsPath)
	if err != nil {
		return fmt.Errorf("hash parent models file %s: %w", parentModelsPath, err)
	}
	retryModelsSHA, err := provenance.SHA256File(*modelsPath)
	if err != nil {
		return fmt.Errorf("hash retry models file %s: %w", *modelsPath, err)
	}
	parentManifestSHA, err := provenance.SHA256File(parentManifestPath)
	if err != nil {
		return fmt.Errorf("hash parent manifest %s: %w", parentManifestPath, err)
	}
	parentResultsSHA, err := provenance.SHA256File(parentResultsPath)
	if err != nil {
		return fmt.Errorf("hash parent results %s: %w", parentResultsPath, err)
	}

	// Idempotent DERIVATIVE CREATION: a stable key over (parent run, retry config,
	// harness code) — if a derivative with this exact key already exists, refuse
	// before spending anything, rather than the mere row-level predicate (which alone
	// would let a second `harness retry` invocation mint a SECOND derivative and bill
	// the same 2 retries twice).
	repairKey := provenance.SHA256Bytes([]byte(parentManifest.RunID + "|" + retryModelsSHA + "|" + gitHEAD()))
	runsRoot := filepath.Dir(strings.TrimRight(*parentRunDir, string(filepath.Separator)))
	if existing, found := findDerivativeByRepairKey(runsRoot, repairKey); found && !*forceNew {
		return fmt.Errorf("retry: a derivative with this exact repair key already exists at %s/%s — refusing to spend again (pass -force-new-derivative to override)", runsRoot, existing)
	}

	// Every candidate row must resolve to a provider in the RETRY config — resolved
	// once, up front, so a misconfigured -models-file fails before any call, not
	// partway through the batch.
	providersByCandidate := make(map[int]ModelProvider, len(candidateIdx))
	for _, i := range candidateIdx {
		row := rows[i]
		p, ok := retryProviders[providerModelKey(row.Provider.ID, row.Provider.Label)]
		if !ok {
			return fmt.Errorf("retry: row %d's provider %s (label %q) has no matching entry in %s", i, row.Provider.ID, row.Provider.Label, *modelsPath)
		}
		providersByCandidate[i] = p
	}

	runID, runDir, err := provenance.NewStagedRunDir(runsRoot)
	if err != nil {
		return err
	}
	manifest := provenance.NewManifest(runDir, runID, "scenario", "retry", args)
	manifest.PromptfooVersion = provenance.PromptfooVersion
	manifest.ModelsPath = *modelsPath
	manifest.ModelsSHA256 = retryModelsSHA
	manifest.RepairedFrom = parentManifest.RunID
	manifest.RepairKey = repairKey
	manifest.ParentManifestSHA256 = parentManifestSHA
	manifest.ParentResultsSHA256 = parentResultsSHA
	manifest.ParentModelsSHA256 = parentModelsSHA
	manifest.RetryModelsSHA256 = retryModelsSHA

	// Carry the parent's snapshot forward unchanged — a repair judges against the SAME
	// rendered prompt/catalog/resolved_tests the parent did, never a fresh render.
	parentSnapDir := filepath.Join(*parentRunDir, "snapshots", scenario.Name)
	derivSnapDir := filepath.Join(runDir, "snapshots", scenario.Name)
	if err := copyDir(parentSnapDir, derivSnapDir); err != nil {
		return fmt.Errorf("copy parent snapshot %s: %w", parentSnapDir, err)
	}
	// Dual model-config provenance: both configs a derivative's rows were actually
	// produced under. Also snapshot a plain "models.yaml" (= the retry config) so
	// judgeScenario/reportRun's existing SnapshotModelsPath fallback lookup finds
	// something sensible without needing to know about the parent/retry split.
	if _, err := provenance.SnapshotFile(parentModelsPath, filepath.Join(runDir, "snapshots", "models.parent.yaml")); err != nil {
		return fmt.Errorf("snapshot parent models: %w", err)
	}
	if _, err := provenance.SnapshotFile(*modelsPath, filepath.Join(runDir, "snapshots", "models.retry.yaml")); err != nil {
		return fmt.Errorf("snapshot retry models: %w", err)
	}
	if _, err := provenance.SnapshotFile(*modelsPath, filepath.Join(runDir, "snapshots", "models.yaml")); err != nil {
		return fmt.Errorf("snapshot models.yaml: %w", err)
	}
	if err := provenance.WriteManifest(runDir, manifest); err != nil {
		return err
	}

	client := newORClientWithTimeout(base, apiKey, retryTimeout)
	dstResultsPath := filepath.Join(runDir, scenario.Name+".results.json")
	retried, err := patchResultsFile(
		parentResultsPath,
		dstResultsPath,
		candidateIdx,
		rows,
		client,
		providersByCandidate,
		parentModelsSHA,
		retryModelsSHA,
		validMedia,
	)
	if err != nil {
		return fmt.Errorf("retry: %w (partial results, if any, were not written)", err)
	}
	fmt.Printf("retry: retried %d row(s), wrote %s\n", retried, dstResultsPath)

	resultsSHA, err := provenance.SHA256File(dstResultsPath)
	if err != nil {
		return err
	}
	ref := provenance.ScenarioSnapshotRef{Scenario: scenario.Name}
	for _, parentRef := range parentManifest.Scenarios {
		if parentRef.Scenario == scenario.Name {
			ref = parentRef
			break
		}
	}
	ref.ResultsSHA256 = resultsSHA
	manifest.Scenarios = append(manifest.Scenarios, ref)
	if err := judgeScenario(*scenarioDir, runDir, *modelsPath); err != nil {
		return fmt.Errorf("judge %s: %w", scenario.Name, err)
	}
	manifest.Finish()
	if err := provenance.WriteManifest(runDir, manifest); err != nil {
		return err
	}
	publishedRunDir, err := provenance.PublishStagedRun(runsRoot, runID, runDir)
	if err != nil {
		return err
	}
	return reportRun(publishedRunDir, *modelsPath)
}
