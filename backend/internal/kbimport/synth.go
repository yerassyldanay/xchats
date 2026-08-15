package kbimport

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/aiprompt"
	"github.com/yerassyldanay/xchats/backend/internal/kbstore"
	"github.com/yerassyldanay/xchats/backend/internal/mcpserver"
	"github.com/yerassyldanay/xchats/backend/llm"
)

const (
	synthTemperature = 0.1
	synthMaxTokens   = 4000
)

// modelCall is one entry of the model's "calls" array — see prompt.go's
// outputContractDescription for the exact contract this mirrors.
type modelCall struct {
	Tool string                     `json:"tool"`
	Ref  string                     `json:"ref"`
	Slug string                     `json:"slug"`
	Args map[string]json.RawMessage `json:"-"`
}

// UnmarshalJSON accepts the model's call shape while ALSO capturing every
// top-level field (tool/ref/slug/changes/expected_draft_version/provenance)
// into Args, so ParseUpsertCall receives the exact {ref/slug, changes}
// argument shape it expects regardless of which optional fields a given
// tool uses — this avoids re-deriving a per-tool args builder here that
// could drift from apply.go's own per-tool switch.
func (c *modelCall) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if tool, ok := raw["tool"]; ok {
		_ = json.Unmarshal(tool, &c.Tool)
	}
	args, ok := raw["args"]
	if !ok {
		c.Args = map[string]json.RawMessage{}
		return nil
	}
	var argsMap map[string]json.RawMessage
	if err := json.Unmarshal(args, &argsMap); err != nil {
		return fmt.Errorf("args is not an object")
	}
	c.Args = argsMap
	return nil
}

// modelOutput is pass 2's expected top-level JSON shape.
type modelOutput struct {
	Calls    []modelCall `json:"calls"`
	Notes    string      `json:"notes"`
	Unmapped []string    `json:"unmapped"`
}

func hasCallsField(m map[string]json.RawMessage) bool {
	_, ok := m["calls"]
	return ok
}

// findPrimary returns the run's primary material, or nil.
func findPrimary(materials []kbstore.ImportMaterial) *kbstore.ImportMaterial {
	for i := range materials {
		if materials[i].Params.Primary {
			return &materials[i]
		}
	}
	return nil
}

// allTerminal reports whether every material in a run has left pass 1
// (queued/extracting are the only non-terminal states this pipeline has).
func allTerminal(materials []kbstore.ImportMaterial) bool {
	for _, m := range materials {
		if m.ProcessingStatus == kbstore.ImportStatusQueued || m.ProcessingStatus == "extracting" {
			return false
		}
	}
	return true
}

// maybeSynthesize checks whether runID is ready for pass 2 (every material
// terminal, synthesis not yet claimed) and, if so, attempts the CAS and —
// only if this call actually won it — runs pass 2 inline. Safe to call
// redundantly (once per sibling material finishing pass 1, from possibly
// different worker goroutines): BeginImportSynthesis's CAS guarantees at
// most one caller ever proceeds.
func (s *Service) maybeSynthesize(ctx context.Context, orgID, runID uuid.UUID) {
	materials, err := s.deps.KB.ImportRunMaterials(ctx, orgID, runID)
	if err != nil {
		s.deps.Log.Error("kbimport: read run materials", "run_id", runID, "err", err)
		return
	}
	primary := findPrimary(materials)
	if primary == nil || primary.Params.Synthesis != nil || !allTerminal(materials) {
		return
	}
	_, claimed, err := s.deps.KB.BeginImportSynthesis(ctx, primary.ID)
	if err != nil {
		s.deps.Log.Error("kbimport: begin synthesis", "run_id", runID, "err", err)
		return
	}
	if !claimed {
		return
	}
	s.runSynthesis(ctx, orgID, runID, primary.ID, primary.Params.UserID)
}

// runSynthesis is pass 2: build the prompt, call the model, parse (one
// re-ask on a parse failure), validate+apply every call (one further
// re-ask if zero calls survive validation), then record the outcome.
func (s *Service) runSynthesis(ctx context.Context, orgID, runID, primaryID, userID uuid.UUID) {
	sctx, cancel := context.WithTimeout(ctx, s.cfg.SynthTimeout)
	defer cancel()
	defer s.notify()

	fail := func(reason string) {
		if err := s.deps.KB.FinishImportSynthesis(ctx, primaryID, kbstore.SynthesisState{Status: kbstore.SynthesisFailed, Notes: reason}); err != nil {
			s.deps.Log.Error("kbimport: record failed synthesis", "run_id", runID, "err", err)
		}
	}
	needsHuman := func(reason string) {
		if err := s.deps.KB.FinishImportSynthesis(ctx, primaryID, kbstore.SynthesisState{Status: kbstore.SynthesisNeedsHuman, Notes: reason}); err != nil {
			s.deps.Log.Error("kbimport: record needs_human synthesis", "run_id", runID, "err", err)
		}
	}

	materials, err := s.deps.KB.ImportRunMaterials(sctx, orgID, runID)
	if err != nil {
		s.deps.Log.Error("kbimport: read run materials for synthesis", "run_id", runID, "err", err)
		fail("внутренняя ошибка при чтении материалов")
		return
	}
	primary := findPrimary(materials)
	if primary == nil {
		return
	}

	prompt, idx, evidenceIDs, err := s.buildSynthesisPrompt(sctx, orgID, materials, primary.Params)
	if err != nil {
		s.deps.Log.Error("kbimport: build synthesis prompt", "run_id", runID, "err", err)
		fail("не удалось подготовить запрос к модели")
		return
	}

	modelRef, err := s.resolveSynthModel()
	if err != nil {
		needsHuman("модель для синтеза не настроена")
		return
	}
	client, err := s.deps.LLM.Client(modelRef)
	if err != nil {
		needsHuman("не удалось получить клиент модели: " + err.Error())
		return
	}

	usage := kbstore.TokenUsage{}
	out, callUsage, callErr := s.callModel(sctx, client, modelRef.Model, prompt)
	usage = addUsage(usage, callUsage)
	output, ok := parseModelOutput(out)
	if callErr == nil && !ok {
		// One re-ask with a corrective appendix — plan spec's parse-failure
		// retry budget.
		out2, usage2, callErr2 := s.callModel(sctx, client, modelRef.Model, prompt+correctiveAppendixParse)
		usage = addUsage(usage, usage2)
		if callErr2 == nil {
			output, ok = parseModelOutput(out2)
		}
	}
	if callErr != nil {
		needsHuman("вызов модели завершился ошибкой: " + callErr.Error())
		return
	}
	if !ok {
		fail("не удалось разобрать ответ модели как ожидаемый JSON после повторной попытки")
		return
	}

	applied, dropped := s.applyCalls(sctx, orgID, userID, primary.Params.TargetType, idx, output.Calls, evidenceIDs)
	if len(applied) == 0 && len(output.Calls) > 0 {
		// Zero survivors, but the model DID try — one more re-ask with the
		// specific validation failures so it can correct them.
		out2, usage2, callErr2 := s.callModel(sctx, client, modelRef.Model, prompt+correctiveAppendixForDropped(dropped))
		usage = addUsage(usage, usage2)
		if callErr2 == nil {
			if output2, ok2 := parseModelOutput(out2); ok2 {
				applied, dropped = s.applyCalls(sctx, orgID, userID, primary.Params.TargetType, idx, output2.Calls, evidenceIDs)
				if output2.Notes != "" {
					output.Notes = output2.Notes
				}
			}
		}
	}

	if len(applied) == 0 {
		notes := output.Notes
		if notes == "" {
			notes = "модель не предложила ни одной применимой записи"
		}
		if err := s.deps.KB.FinishImportSynthesis(ctx, primaryID, kbstore.SynthesisState{
			Status: kbstore.SynthesisFailed, Notes: notes, Dropped: dropped, Usage: usage,
		}); err != nil {
			s.deps.Log.Error("kbimport: record failed synthesis", "run_id", runID, "err", err)
		}
		return
	}

	if err := s.deps.KB.MarkImportBuilt(ctx, evidenceIDs); err != nil {
		s.deps.Log.Error("kbimport: mark import built", "run_id", runID, "err", err)
	}
	if err := s.deps.KB.FinishImportSynthesis(ctx, primaryID, kbstore.SynthesisState{
		Status: kbstore.SynthesisBuilt, Notes: output.Notes, Applied: applied, Dropped: dropped, Usage: usage,
	}); err != nil {
		s.deps.Log.Error("kbimport: record built synthesis", "run_id", runID, "err", err)
	}
}

// callModel makes one text-only, low-temperature model call — pass 2's
// single request shape (plan spec: "one text-only llm.ChatRequest,
// Russian, temp 0.1, 4000 max tokens").
func (s *Service) callModel(ctx context.Context, client llm.ChatClient, model, prompt string) (string, kbstore.TokenUsage, error) {
	resp, err := client.Complete(ctx, llm.ChatRequest{
		Model:       model,
		Messages:    []llm.Message{{Role: "user", Content: prompt}},
		Temperature: synthTemperature,
		MaxTokens:   synthMaxTokens,
	})
	if err != nil {
		return "", kbstore.TokenUsage{}, err
	}
	return resp.Text, kbstore.TokenUsage{PromptTokens: resp.PromptTokens, CompletionTokens: resp.CompletionTokens}, nil
}

func addUsage(a, b kbstore.TokenUsage) kbstore.TokenUsage {
	return kbstore.TokenUsage{PromptTokens: a.PromptTokens + b.PromptTokens, CompletionTokens: a.CompletionTokens + b.CompletionTokens}
}

// parseModelOutput locates the model's JSON object via
// aiprompt.ExtractJSONObject (the contract-agnostic core shared with the
// customer-response contract's own ExtractFinalOutput) and decodes it.
func parseModelOutput(raw string) (modelOutput, bool) {
	extracted, ok := aiprompt.ExtractJSONObject(raw, hasCallsField)
	if !ok {
		return modelOutput{}, false
	}
	var out modelOutput
	if err := json.Unmarshal([]byte(extracted.Final), &out); err != nil {
		return modelOutput{}, false
	}
	return out, true
}

// applyRank sorts contacts/policies last — plan spec's "apply singletons
// last for stable ordering."
func applyRank(tool string) int {
	switch tool {
	case "kb_contacts_upsert", "kb_policies_upsert":
		return 1
	default:
		return 0
	}
}

// applyCalls validates and applies every model call, fail-closed:
// drop-and-continue per call, never aborting the batch. Mirrors
// plan/playground.md's "keep valid entries, drop broken ones, report what
// was dropped."
func (s *Service) applyCalls(ctx context.Context, orgID, userID uuid.UUID, targetType string, idx manifestIndex, calls []modelCall, evidenceIDs []uuid.UUID) (applied []kbstore.AppliedUpsert, dropped []kbstore.DroppedUpsert) {
	if len(calls) > s.cfg.MaxCallsPerRun {
		calls = calls[:s.cfg.MaxCallsPerRun]
	}
	sort.SliceStable(calls, func(i, j int) bool { return applyRank(calls[i].Tool) < applyRank(calls[j].Tool) })

	prov := kbstore.MCPProvenance{MaterialIDs: evidenceIDs}

	for _, c := range calls {
		if !isContentUpsertTool(c.Tool) {
			dropped = append(dropped, kbstore.DroppedUpsert{Tool: c.Tool, Reason: "not an allowed tool for this import"})
			continue
		}
		if allowed := allowedToolsForTarget(targetType); !containsString(allowed, c.Tool) {
			dropped = append(dropped, kbstore.DroppedUpsert{Tool: c.Tool, Reason: "does not match the selected target type"})
			continue
		}

		args := c.Args
		if args == nil {
			args = map[string]json.RawMessage{}
		}
		if c.Ref != "" {
			if b, err := json.Marshal(c.Ref); err == nil {
				args["ref"] = b
			}
		}
		if c.Slug != "" {
			if b, err := json.Marshal(c.Slug); err == nil {
				args["slug"] = b
			}
		}

		if reason := resolveHandlesInArgs(args, idx); reason != "" {
			dropped = append(dropped, kbstore.DroppedUpsert{Tool: c.Tool, Reason: reason})
			continue
		}

		call, err := mcpserver.ParseUpsertCall(c.Tool, args)
		if err != nil {
			dropped = append(dropped, kbstore.DroppedUpsert{Tool: c.Tool, Reason: err.Error()})
			continue
		}
		res, err := call.Apply(ctx, s.deps.KB, orgID, userID, nil, prov)
		if err != nil {
			dropped = append(dropped, kbstore.DroppedUpsert{Tool: c.Tool, Reason: err.Error()})
			continue
		}
		applied = append(applied, kbstore.AppliedUpsert{Tool: c.Tool, Type: res.Type, Key: res.Key, Created: res.Created})
	}
	return applied, dropped
}

func isContentUpsertTool(tool string) bool { return containsString(contentUpsertTools, tool) }

func containsString(list []string, v string) bool {
	for _, s := range list {
		if s == v {
			return true
		}
	}
	return false
}

// correctiveAppendixForDropped builds the second re-ask's appendix, naming
// exactly why every call was dropped so the model can address the SAME
// mistakes instead of repeating them blind.
func correctiveAppendixForDropped(dropped []kbstore.DroppedUpsert) string {
	if len(dropped) == 0 {
		return "\n\nТвой предыдущий ответ не содержал ни одного вызова инструмента. Проанализируй материалы ещё раз и верни хотя бы один вызов, если в материалах есть подходящие данные, либо явно укажи в \"unmapped\", почему это невозможно."
	}
	s := "\n\nНи один из вызовов инструментов в твоём предыдущем ответе не прошёл проверку. Причины:\n"
	for _, d := range dropped {
		s += fmt.Sprintf("- %s: %s\n", d.Tool, d.Reason)
	}
	s += "Ответь ЗАНОВО, исправив эти проблемы (СТРОГО тем же форматом JSON)."
	return s
}
