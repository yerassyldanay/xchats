package main

import (
	"fmt"
	"path/filepath"
	"strings"

	"xchats-evals-harness/internal/evaltext"
	"xchats-evals-harness/internal/kbfixture"

	"github.com/yerassyldanay/xchats/backend/aiprompt"
)

// judgeSchemaKBRows grades every result row for a pipeline:schema_kb_v1 scenario. It
// rebuilds the catalog fresh from the fixture (see judgeScenario's call-site comment for
// why) and delegates each row to judgeOneSchemaKB, sharing every helper judgeOne itself
// uses (control-char/reasoning-leak/unit/digit checks, language checks, must-contain
// checks, outcome checks) — only contract-shape parsing and media-token validation
// differ, via aiprompt instead of judge.go's own hand-rolled map checks.
func judgeSchemaKBRows(scenarioDir string, scenario *ScenarioConfig, results PromptfooResults, testByID map[string]TestCase, priceByModel map[string]ModelProvider, freshSplit map[string]tokenSplit) ([]Verdict, error) {
	fixturePath := filepath.Join(scenarioDir, scenario.Data)
	kb, err := kbfixture.Load(fixturePath)
	if err != nil {
		return nil, fmt.Errorf("scenario %q: load fixture %s: %w", scenario.Name, fixturePath, err)
	}
	if kb, err = kbfixture.ApplyLimits(kb, scenario.Limits); err != nil {
		return nil, fmt.Errorf("scenario %q: %w", scenario.Name, err)
	}
	cat, err := aiprompt.BuildCatalog(kb)
	if err != nil {
		return nil, fmt.Errorf("scenario %q: build catalog: %w", scenario.Name, err)
	}

	tokenValue := aipromptTokenValues(cat)
	trustedDigits := trustedDigitsFromKB(kb)

	var verdicts []Verdict
	for _, row := range results.Results.Results {
		tc, ok := testByID[row.TestCase.Description]
		if !ok {
			continue
		}
		v := judgeOneSchemaKB(tc, row, kb, cat, tokenValue, trustedDigits)
		applyCostEstimate(&v, row, priceByModel, freshSplit)
		verdicts = append(verdicts, v)
	}
	return verdicts, nil
}

// aipromptTokenValues projects an aiprompt.Catalog's facts into the flat token->value
// map requiresSatisfied/outcomeSatisfied need for their membership checks (they only
// ever ask "is this token known", never read the value) — deliberately NOT used for
// customer-facing text injection, which must go through aiprompt.SubstituteFacts instead
// so the in_stock flag gets the reviewed Russian wording rather than its raw catalog
// display value ("да"/"нет", meant for the model's own reasoning, not a customer reply).
func aipromptTokenValues(cat *aiprompt.Catalog) map[string]string {
	m := make(map[string]string, len(cat.Facts))
	for _, f := range cat.Facts {
		m[f.Token] = f.Value
	}
	return m
}

// trustedDigitsFromKB extracts every digit run from exactly the trusted-prose fields
// aiprompt/prompt.go's renderDescriptions puts in the prompt's DESCRIPTIONS block — the
// schema_kb_v1 analogue of legacy judge.go's Catalog.TrustedDigits (itself sourced from
// FactRow.Description). A model correctly quoting a description's own digits (e.g. "1.7
// л") is not inventing a number.
func trustedDigitsFromKB(kb *aiprompt.KB) []string {
	seen := map[string]bool{}
	scan := func(s string) {
		for _, d := range digitRunRE.FindAllString(s, -1) {
			seen[d] = true
		}
	}
	for _, p := range kb.Products {
		scan(p.Description)
	}
	for _, t := range kb.Tariffs {
		scan(t.Summary)
		scan(t.LimitText)
		scan(t.Advantages)
		scan(t.Disadvantages)
	}
	if kb.Contacts != nil {
		scan(kb.Contacts.Address)
		scan(kb.Contacts.LegalInformation)
		scan(kb.Contacts.CallbackTime)
	}
	if kb.Policies != nil {
		scan(kb.Policies.Prepayment)
		scan(kb.Policies.Installment)
		scan(kb.Policies.Warranty)
	}
	out := make([]string, 0, len(seen))
	for d := range seen {
		out = append(out, d)
	}
	return out
}

// issueSummary renders a []aiprompt.ContractIssue as one human-readable string for
// Verdict.Reason.
func issueSummary(issues []aiprompt.ContractIssue) string {
	parts := make([]string, 0, len(issues))
	for _, is := range issues {
		if is.Detail != "" {
			parts = append(parts, is.Code+": "+is.Detail)
		} else {
			parts = append(parts, is.Code)
		}
	}
	return strings.Join(parts, "; ")
}

// judgeOneSchemaKB is judgeOne's counterpart for the shop-kb-v1 family: strict contract
// parsing via aiprompt.ValidateResponse (unknown properties, wrong types, bad
// reply_language/confidence, and unknown media tokens are ALL rejected by the same
// strict decode judge.go's own hand-rolled map checks only partially covered), fact
// substitution via aiprompt.SubstituteFacts (gets in_stock's Russian wording right), and
// a media_files_to_send re-resolution check via aiprompt.ResolveSend that the legacy
// pipeline has no equivalent of (it has no kbd_materials to resolve against). Every
// other check — reasoning leak, control chars, invented digits, unit issues, requires,
// escalate match, language, must-contain(-any), outcomes — reuses judgeOne's exact
// helpers, so the two pipelines can never silently diverge on what those mean.
func judgeOneSchemaKB(tc TestCase, row PromptfooRow, kb *aiprompt.KB, cat *aiprompt.Catalog, tokenValue map[string]string, trustedDigits []string) Verdict {
	v := Verdict{
		TestID:       tc.ID,
		Model:        providerModelKey(row.Provider.ID, row.Provider.Label),
		Message:      tc.Message,
		RawOutput:    row.Response.Output,
		Cost:         row.Cost,
		LatencyMs:    row.LatencyMs,
		Tokens:       row.TokenUsage.Total,
		FinishReason: row.Response.FinishReason,
		Truncated:    isTruncatedFinish(row.Response.FinishReason),
		Reasoning:    row.Response.Reasoning,
		Retries:      row.Retries,
	}

	cleaned := evaltext.StripFences(row.Response.Output)
	_, syntaxOK := parseModelJSON(row.Response.Output)
	v.ParseOK = syntaxOK
	if !syntaxOK {
		v.Reason = "could not parse JSON output"
		if v.Truncated {
			v.Reason += " (" + truncationNote(v.FinishReason) + ")"
		}
		return v
	}

	resp, issues := aiprompt.ValidateResponse(cleaned, cat)
	v.RetryRecovered = v.Retries > 0 && resp != nil
	if resp == nil {
		// Valid JSON, but it failed the strict six-field contract schema (wrong type,
		// missing/unknown property) badly enough that aiprompt couldn't even decode a
		// Response struct — a deliberately stricter outcome than the legacy pipeline's
		// lenient per-field map checks. This IS the intended tradeoff (see the plan's own
		// note: strict schema may lower first-shot contract rates — that is signal, not
		// breakage; retry.go's schema-invalid candidacy exists to recover from exactly
		// this).
		v.Reason = "response failed the strict contract schema: " + issueSummary(issues)
		if v.Truncated {
			v.Reason += " (" + truncationNote(v.FinishReason) + ")"
		}
		return v
	}

	replyText := resp.ReplyText
	v.ReasoningLeak = evaltext.HasReasoningMarkers(replyText)
	if v.ReasoningLeak {
		v.appendReason("reasoning/thinking content leaked into reply_text")
	}
	if bad, ok := firstBadControlChar(replyText); ok {
		v.ControlChars = true
		v.ControlCharsIssue = fmt.Sprintf("reply_text contains control character %U", bad)
		v.appendReason(v.ControlCharsIssue)
	}

	contractFieldsOK := true
	var unknownMediaFromCatalog []string
	for _, is := range issues {
		switch is.Code {
		case "missing_property", "unknown_property":
			contractFieldsOK = false
			v.appendReason(fmt.Sprintf("contract shape issue (%s): %s", is.Code, is.Detail))
		case "bad_language":
			contractFieldsOK = false
			v.appendReason("bad reply_language: " + is.Detail)
		case "bad_confidence":
			contractFieldsOK = false
			v.appendReason("confidence out of range: " + is.Detail)
		case "unknown_media_token":
			unknownMediaFromCatalog = append(unknownMediaFromCatalog, is.Detail)
		}
	}
	v.ContractFields = contractFieldsOK

	v.EscalationReasonOK = true
	for _, is := range issues {
		if is.Code == "escalation_reason_mismatch" {
			v.EscalationReasonOK = false
			v.appendReason("escalation_reason must be empty when escalate is false")
		}
	}

	// Fail-closed token substitution: scan first so EVERY unknown token is reported (not
	// just the first — aiprompt.SubstituteFacts itself fails fast on the first one), then
	// substitute via aiprompt.SubstituteFacts once every token is confirmed known, so
	// in_stock gets the reviewed Russian wording rather than its raw "да"/"нет" display
	// value.
	spans := tokenSpanRE.FindAllString(replyText, -1)
	reasonSpans := tokenSpanRE.FindAllString(resp.EscalationReason, -1)
	var unknownTokens []string
	for _, tok := range spans {
		if cat.FactByToken(tok) == nil {
			unknownTokens = append(unknownTokens, tok)
		}
	}
	for _, tok := range reasonSpans {
		if cat.FactByToken(tok) == nil {
			unknownTokens = append(unknownTokens, tok)
		}
	}
	v.UnknownTokens = unknownTokens
	v.Blocked = len(unknownTokens) > 0
	injected := replyText
	if v.Blocked {
		v.appendReason("unknown token(s), draft would be BLOCKED: " + strings.Join(v.UnknownTokens, ", "))
	} else if !v.ReasoningLeak {
		out, err := aiprompt.SubstituteFacts(replyText, cat)
		if err != nil {
			// Should be unreachable — every span was just confirmed known above — but
			// fail safe rather than silently ship un-substituted text if aiprompt's
			// placeholder regex and judge.go's tokenSpanRE ever drift apart.
			v.Blocked = true
			v.appendReason("fact substitution failed unexpectedly: " + err.Error())
		} else {
			injected = out
			v.InjectedText = injected
		}
	}
	if strings.ContainsAny(injected, "{}") {
		v.LeftoverBraces = true
		v.appendReason("leftover brace survived injection")
	}
	if v.Truncated {
		v.appendReason(truncationNote(v.FinishReason))
	}

	// media_files_to_send re-resolution: every token that passed aiprompt.ValidateResponse's
	// catalog-membership check (i.e. not already in unknownMediaFromCatalog) must ALSO
	// still resolve through kbd_materials right now — catching "valid name, stale
	// material" the membership check alone cannot see.
	v.MediaResolveOK = true
	if len(unknownMediaFromCatalog) == 0 && len(resp.MediaFilesToSend) > 0 {
		if _, err := aiprompt.ResolveSend(resp.MediaFilesToSend, kb, cat); err != nil {
			v.MediaResolveOK = false
			v.MediaResolveIssue = err.Error()
			v.appendReason("media_files_to_send token did not resolve: " + err.Error())
		}
	}

	v.ContractPass = v.ParseOK && resp != nil && v.ContractFields && v.EscalationReasonOK &&
		!v.Blocked && !v.LeftoverBraces && !v.Truncated && !v.ReasoningLeak && !v.ControlChars && v.MediaResolveOK

	// Model-behavior checks — identical helpers to judgeOne.
	stripped := tokenSpanRE.ReplaceAllString(replyText, "")
	stripped = listMarkerRE.ReplaceAllString(stripped, "")
	stripped = inlineListMarkerRE.ReplaceAllString(stripped, "$1$2")
	v.InventedDigits = filterInventedDigits(digitRunRE.FindAllString(stripped, -1), tc.Message, trustedDigits)

	v.RequiresPass = requiresSatisfied(tc.Requires, replyText, tokenValue)

	v.MediaPass = true
	if tc.Media != nil {
		v.MediaPass, v.MediaIssue = checkMediaExpect(tc.Media, resp.MediaFilesToSend)
	}

	v.MediaCountEvaluated = true
	v.MediaCount = len(resp.MediaFilesToSend)
	v.TooManyMedia = v.MediaCount > maxMediaGroups

	v.EscalatePass = true
	if tc.Escalate != nil {
		v.EscalatePass = resp.Escalate == *tc.Escalate
	}

	v.LanguagePass = true
	v.LanguageTextOK = true
	v.LanguageFieldOK = true
	if tc.Language == "kk" || tc.Language == "ru" {
		checkText := v.InjectedText
		if checkText == "" {
			checkText = replyText
		}
		textOK := languageTextOK(tc.Language, checkText)
		fieldOK := languageFieldOK(tc.Language, resp.ReplyLanguage)
		v.LanguageAliasUsed = fieldOK && resp.ReplyLanguage != tc.Language
		v.LanguageTextOK = textOK
		v.LanguageFieldOK = fieldOK
		v.LanguagePass = textOK && fieldOK
		switch {
		case !textOK && tc.Language == "kk":
			v.LanguageIssue = "reply does not look like Kazakh (too few Kazakh-specific letters)"
		case !textOK && tc.Language == "ru":
			v.LanguageIssue = "reply looks like Kazakh but a Russian reply was expected"
		case !fieldOK:
			v.LanguageIssue = fmt.Sprintf("reply_language field is %q, expected %q", resp.ReplyLanguage, tc.Language)
		}
	}
	v.UnitIssues = findUnitIssues(v.InjectedText)

	v.MustNotContainPass = true
	if len(tc.MustNotContain) > 0 {
		if phrase, hit := firstForbidden(tc.MustNotContain, injected); hit {
			v.MustNotContainPass = false
			v.ForbiddenPhrase = phrase
		}
	}

	v.MustContainAnyPass = true
	if len(tc.MustContainAny) > 0 {
		v.MustContainAnyExpected = tc.MustContainAny
		v.MustContainAnyPass = anyContained(tc.MustContainAny, injected)
	}

	v.OutcomesDeclared = len(tc.Outcomes) > 0
	v.OutcomesPass = true
	if v.OutcomesDeclared {
		for _, oc := range tc.Outcomes {
			v.OutcomeLabels = append(v.OutcomeLabels, oc.Label)
		}
		checkText := v.InjectedText
		if checkText == "" {
			checkText = replyText
		}
		in := outcomeInputs{
			replyText:    replyText,
			injected:     injected,
			checkText:    checkText,
			replyLang:    resp.ReplyLanguage,
			escalate:     resp.Escalate,
			mediaEntries: resp.MediaFilesToSend,
			tokenValue:   tokenValue,
		}
		v.OutcomesPass = false
		for _, oc := range tc.Outcomes {
			if outcomeSatisfied(oc, in) {
				v.OutcomesPass = true
				v.OutcomeMatched = oc.Label
				break
			}
		}
	}

	v.UnknownMedia = unknownMediaFromCatalog

	v.ModelBehaviorPass = v.RequiresPass && v.MediaPass && v.EscalatePass && v.LanguagePass &&
		v.MustNotContainPass && v.MustContainAnyPass && v.OutcomesPass && len(v.InventedDigits) == 0 &&
		len(v.UnitIssues) == 0 && len(v.UnknownMedia) == 0 && !v.TooManyMedia

	if v.Reason == "" && !v.ModelBehaviorPass {
		v.Reason = firstFailureReason(v)
	}
	if v.Reason == "" {
		v.Reason = "ok"
	}
	return v
}
