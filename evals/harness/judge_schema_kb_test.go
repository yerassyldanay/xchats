package main

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"xchats-evals-harness/internal/kbfixture"

	"github.com/yerassyldanay/xchats/backend/aiprompt"
)

func loadSchemaKBJudgeContext(t *testing.T) (*aiprompt.KB, *aiprompt.Catalog) {
	t.Helper()
	dir := t.TempDir()
	fixturePath := filepath.Join(dir, "data-ru.yaml")
	mustWrite(t, fixturePath, testSchemaKBFixture)
	kb, err := kbfixture.Load(fixturePath)
	if err != nil {
		t.Fatal(err)
	}
	cat, err := aiprompt.BuildCatalog(kb)
	if err != nil {
		t.Fatal(err)
	}
	return kb, cat
}

func schemaKBResponse(t *testing.T, text string, media []string, escalationReason string) string {
	t.Helper()
	return schemaKBResponseLang(t, text, media, escalationReason, "ru")
}

// schemaKBResponseLang is schemaKBResponse's language-parameterized counterpart
// (2026-07 Kazakh customer-message testing).
func schemaKBResponseLang(t *testing.T, text string, media []string, escalationReason, lang string) string {
	t.Helper()
	b, err := json.Marshal(aiprompt.Response{
		ReplyText:        text,
		ReplyLanguage:    lang,
		MediaFilesToSend: media,
		Escalate:         escalationReason != "",
		EscalationReason: escalationReason,
		Confidence:       0.9,
	})
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func schemaKBPromptfooRow(output string) PromptfooRow {
	row := PromptfooRow{}
	row.Provider.ID = "openrouter:test/model"
	row.TestCase.Description = "schema test"
	row.Response.Output = output
	row.Response.FinishReason = "stop"
	return row
}

func judgeSchemaKBOutput(t *testing.T, kb *aiprompt.KB, cat *aiprompt.Catalog, output string) Verdict {
	t.Helper()
	return judgeOneSchemaKB(
		TestCase{},
		schemaKBPromptfooRow(output),
		kb,
		cat,
		aipromptTokenValues(cat),
		trustedDigitsFromKB(kb),
	)
}

func TestJudgeOneSchemaKB_ValidatesAndInjectsCurrentFact(t *testing.T) {
	kb, cat := loadSchemaKBJudgeContext(t)
	output := schemaKBResponse(t, "Цена — {{product.coffee-machine.price}}.", []string{}, "")
	v := judgeSchemaKBOutput(t, kb, cat, output)
	if !v.ContractPass {
		t.Fatalf("ContractPass=false: %s", v.Reason)
	}
	if v.InjectedText != "Цена — 129 900 ₸." {
		t.Fatalf("InjectedText = %q", v.InjectedText)
	}
	parseOK, contractPass := firstAttemptOutcome(schemaKBPromptfooRow(output), v, func(row PromptfooRow) Verdict {
		return judgeOneSchemaKB(TestCase{}, row, kb, cat, aipromptTokenValues(cat), trustedDigitsFromKB(kb))
	})
	if !parseOK || !contractPass {
		t.Fatalf("first attempt = (%v, %v), want valid contract", parseOK, contractPass)
	}
}

func TestJudgeOneSchemaKB_BlocksFactContractIssues(t *testing.T) {
	tests := []struct {
		name             string
		text             string
		escalationReason string
		mutate           func(*aiprompt.KB)
	}{
		{name: "unknown placeholder", text: "{{product.unknown.price}}"},
		{name: "malformed placeholder", text: "{product.coffee-machine.price}"},
		{name: "literal exact value", text: "Цена — 129 900 ₸."},
		{
			name: "stale placeholder",
			text: "{{product.coffee-machine.price}}",
			mutate: func(kb *aiprompt.KB) {
				kb.Products[0].Price = ""
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kb, cat := loadSchemaKBJudgeContext(t)
			if tt.mutate != nil {
				tt.mutate(kb)
			}
			output := schemaKBResponse(t, tt.text, []string{}, tt.escalationReason)
			v := judgeSchemaKBOutput(t, kb, cat, output)
			if !v.Blocked || v.ContractPass {
				t.Fatalf("Blocked=%v ContractPass=%v reason=%q", v.Blocked, v.ContractPass, v.Reason)
			}
			if candidates := retryCandidateIndexesSchemaKB([]PromptfooRow{schemaKBPromptfooRow(output)}, kb, cat); len(candidates) != 0 {
				t.Fatalf("fact behavior failure must not auto-retry, got candidates %v", candidates)
			}
		})
	}
}

// TestJudgeOneSchemaKB_PlaceholderInEscalationReasonDoesNotBlock: escalation_reason is
// diagnostic-only telemetry — a fact placeholder inside it is never substituted and
// must never block the draft (unlike the same placeholder inside reply_text).
func TestJudgeOneSchemaKB_PlaceholderInEscalationReasonDoesNotBlock(t *testing.T) {
	kb, cat := loadSchemaKBJudgeContext(t)
	output := schemaKBResponse(t, "Уточню.", []string{}, "Нужен {{product.coffee-machine.price}}")
	v := judgeSchemaKBOutput(t, kb, cat, output)
	if v.Blocked || !v.ContractPass {
		t.Fatalf("Blocked=%v ContractPass=%v reason=%q, want unblocked/contract-pass", v.Blocked, v.ContractPass, v.Reason)
	}
}

func TestJudgeOneSchemaKB_UnknownMediaFailsContractAndRetries(t *testing.T) {
	kb, cat := loadSchemaKBJudgeContext(t)
	output := schemaKBResponse(t, "Прикрепляю.", []string{"products.unknown.gallery_images"}, "")
	row := schemaKBPromptfooRow(output)
	v := judgeSchemaKBOutput(t, kb, cat, output)
	if v.ContractPass || len(v.UnknownMedia) != 1 {
		t.Fatalf("ContractPass=%v UnknownMedia=%v", v.ContractPass, v.UnknownMedia)
	}
	candidates := retryCandidateIndexesSchemaKB([]PromptfooRow{row}, kb, cat)
	if len(candidates) != 1 || candidates[0].Index != 0 || candidates[0].Reason != retryReasonMediaNotFound {
		t.Fatalf("retry candidates = %+v, want [{Index:0 Reason:media_not_found}]", candidates)
	}
}

// TestJudgeOneSchemaKB_ForbidTokens proves TestCase.ForbidTokens gates
// ModelBehaviorPass on the schema_kb_v1 path exactly as it does on judgeOne — the same
// mechanism a "this direction matches no delivery zone" case needs to prove the model
// didn't fall back to citing some OTHER zone's price.
func TestJudgeOneSchemaKB_ForbidTokens(t *testing.T) {
	kb, cat := loadSchemaKBJudgeContext(t)
	tc := TestCase{ID: "t", ForbidTokens: []string{"policy."}}
	tokenValue := aipromptTokenValues(cat)
	trustedDigits := trustedDigitsFromKB(kb)

	t.Run("citing a forbidden-prefix token fails behavior", func(t *testing.T) {
		output := schemaKBResponse(t, "Доставка — {{policy.main.delivery_cost}}.", []string{}, "")
		v := judgeOneSchemaKB(tc, schemaKBPromptfooRow(output), kb, cat, tokenValue, trustedDigits)
		if v.ForbidTokensPass || v.ModelBehaviorPass {
			t.Fatalf("ForbidTokensPass=%v ModelBehaviorPass=%v reason=%q", v.ForbidTokensPass, v.ModelBehaviorPass, v.Reason)
		}
		if len(v.ForbiddenTokensHit) != 1 || v.ForbiddenTokensHit[0] != "{{policy.main.delivery_cost}}" {
			t.Fatalf("ForbiddenTokensHit = %v", v.ForbiddenTokensHit)
		}
	})

	t.Run("citing a different token passes", func(t *testing.T) {
		output := schemaKBResponse(t, "Цена — {{product.coffee-machine.price}}.", []string{}, "")
		v := judgeOneSchemaKB(tc, schemaKBPromptfooRow(output), kb, cat, tokenValue, trustedDigits)
		if !v.ForbidTokensPass {
			t.Fatalf("want ForbidTokensPass=true, reason=%q", v.Reason)
		}
	})
}

// TestJudgeOneSchemaKB_ExtractsFinalAnswerFromCombinedReasoning mirrors
// TestJudgeOne_ExtractsFinalAnswerFromCombinedReasoning for the schema_kb_v1 path —
// aiprompt.ExtractFinalOutput must recover the final answer here too, and RawOutput
// must stay byte-for-byte the provider's original combined output.
func TestJudgeOneSchemaKB_ExtractsFinalAnswerFromCombinedReasoning(t *testing.T) {
	kb, cat := loadSchemaKBJudgeContext(t)
	final := schemaKBResponse(t, "Цена — {{product.coffee-machine.price}}.", []string{}, "")
	reasoning := "Thinking: checking the knowledge base for the coffee machine price before replying."
	raw := reasoning + "\n\n" + final

	v := judgeSchemaKBOutput(t, kb, cat, raw)
	if !v.ParseOK {
		t.Fatalf("ParseOK=false: %s", v.Reason)
	}
	if v.RawOutput != raw {
		t.Errorf("RawOutput = %q, want untouched original %q", v.RawOutput, raw)
	}
	if v.FinalOutput != final {
		t.Errorf("FinalOutput = %q, want %q", v.FinalOutput, final)
	}
	if !strings.Contains(v.NonFinalOutput, "Thinking:") {
		t.Errorf("NonFinalOutput = %q, want reasoning prose retained", v.NonFinalOutput)
	}
	if !v.ContractPass {
		t.Errorf("ContractPass=false: %s", v.Reason)
	}
	if v.InjectedText != "Цена — 129 900 ₸." {
		t.Errorf("InjectedText = %q", v.InjectedText)
	}
}

// TestJudgeOneSchemaKB_TruncatedFinalAnswerStaysAFailure: a final JSON object that
// never completes (reasoning consumed the budget) must remain a parse failure — never
// repaired or reconstructed.
func TestJudgeOneSchemaKB_TruncatedFinalAnswerStaysAFailure(t *testing.T) {
	kb, cat := loadSchemaKBJudgeContext(t)
	final := schemaKBResponse(t, "Цена — {{product.coffee-machine.price}}.", []string{}, "")
	raw := "Thinking: checking the price.\n\n" + final[:len(final)-10]

	v := judgeSchemaKBOutput(t, kb, cat, raw)
	if v.ParseOK {
		t.Fatal("ParseOK=true, want false — the final object was never complete")
	}
	if v.ContractPass {
		t.Fatal("ContractPass=true, want false")
	}
}

// testSchemaKBFixtureTwoProducts adds a second, media-less product (blender-x) alongside
// coffee-machine's own featured_image — the minimum needed to distinguish "wrong
// product's media" from "no media at all" in the media regression tests below.
const testSchemaKBFixtureTwoProducts = `
organization_id: "11111111-1111-1111-1111-111111111111"

ai_assistants:
  persona: "Ты — ассистент интернет-магазина «Demo Shop»."
  guardrails: "Никогда не выдумывай цены и факты."
  language_policy: "Отвечай только на русском языке."

ai_products:
  - ref: coffee-machine
    name: "Кофемашина DeLonghi"
    price: "129 900 ₸"
    description: "Автоматическая кофемашина для дома."
    availability_status: in_stock
    sales_status: active
    featured_image: "22222222-2222-2222-2222-222222222222"
  - ref: blender-x
    name: "Блендер X"
    price: "9 900 ₸"
    availability_status: in_stock
    sales_status: active

ai_policies:
  delivery_cost: "2 000 ₸"

kbd_materials:
  - id: "22222222-2222-2222-2222-222222222222"
    source_type: file
    source_ref: "coffee-front-sentinel.jpg"
    filename: "coffee-front-sentinel.jpg"
    mime_type: "image/jpeg"
    size_bytes: 248315
    storage_backend: fake
    storage_key: "fake://bucket/sentinel-2222"
    processing_status: built
    customer_visibility: visible
`

func loadSchemaKBJudgeContextTwoProducts(t *testing.T) (*aiprompt.KB, *aiprompt.Catalog) {
	t.Helper()
	dir := t.TempDir()
	fixturePath := filepath.Join(dir, "data-ru.yaml")
	mustWrite(t, fixturePath, testSchemaKBFixtureTwoProducts)
	kb, err := kbfixture.Load(fixturePath)
	if err != nil {
		t.Fatal(err)
	}
	cat, err := aiprompt.BuildCatalog(kb)
	if err != nil {
		t.Fatal(err)
	}
	return kb, cat
}

// TestJudgeOneSchemaKB_MediaTokenWrappedInBraces_UnknownMedia is media regression case
// 1 (2026-07 Phase 2 postmortem follow-up): a media reference the model wrapped in
// `{{ }}` — the fact-placeholder syntax, not media syntax — must not silently match its
// unwrapped catalog entry; it is simply a different string, so it fails catalog
// membership exactly like any other invented token.
func TestJudgeOneSchemaKB_MediaTokenWrappedInBraces_UnknownMedia(t *testing.T) {
	kb, cat := loadSchemaKBJudgeContext(t)
	output := schemaKBResponse(t, "Прикрепляю фото.", []string{"{{products.coffee-machine.featured_image}}"}, "")
	v := judgeSchemaKBOutput(t, kb, cat, output)
	if v.ContractPass {
		t.Fatalf("want ContractPass=false for a brace-wrapped media reference, got true (reason=%q)", v.Reason)
	}
	if len(v.UnknownMedia) != 1 {
		t.Fatalf("want exactly 1 UnknownMedia entry, got %v", v.UnknownMedia)
	}
}

// TestJudgeOneSchemaKB_MediaTokenInReplyText_Blocked is media regression case 2: a
// valid media token written as literal text inside reply_text (instead of listed in
// media_files_to_send) blocks the draft — aiprompt.ValidateResponse's
// media_token_in_reply_text issue is treated the same severity as an invented/stale
// fact placeholder.
func TestJudgeOneSchemaKB_MediaTokenInReplyText_Blocked(t *testing.T) {
	kb, cat := loadSchemaKBJudgeContext(t)
	output := schemaKBResponse(t, "Вот фото: products.coffee-machine.featured_image", []string{}, "")
	v := judgeSchemaKBOutput(t, kb, cat, output)
	if !v.Blocked || v.ContractPass {
		t.Fatalf("Blocked=%v ContractPass=%v reason=%q, want blocked/contract-fail", v.Blocked, v.ContractPass, v.Reason)
	}
}

// TestJudgeOneSchemaKB_InventedMediaOnMediaLessProduct_UnknownMedia is media
// regression case 3: a media kind the model invents for a product that has NO media
// columns populated at all must fail catalog membership — the same fail-closed rule as
// an entirely unknown product/table, just for a real product with an empty column.
func TestJudgeOneSchemaKB_InventedMediaOnMediaLessProduct_UnknownMedia(t *testing.T) {
	kb, cat := loadSchemaKBJudgeContextTwoProducts(t)
	output := schemaKBResponse(t, "Прикрепляю фото.", []string{"products.blender-x.featured_image"}, "")
	v := judgeSchemaKBOutput(t, kb, cat, output)
	if v.ContractPass {
		t.Fatalf("want ContractPass=false for an invented media token on a media-less product, got true (reason=%q)", v.Reason)
	}
	if len(v.UnknownMedia) != 1 {
		t.Fatalf("want exactly 1 UnknownMedia entry, got %v", v.UnknownMedia)
	}
}

// TestJudgeOneSchemaKB_WrongProductMedia_FailsForbidExpectation is media regression
// case 4: attaching another product's media is a VALID catalog token (not caught by
// UnknownMedia at all — this is the "correction" the plan calls out), so a test
// expecting no attachment for THIS product must reject it via media.forbid explicitly,
// not rely on catalog-membership checking to catch a wrong-but-real token.
func TestJudgeOneSchemaKB_WrongProductMedia_FailsForbidExpectation(t *testing.T) {
	kb, cat := loadSchemaKBJudgeContextTwoProducts(t)
	tc := TestCase{ID: "t", Media: &MediaExpect{Forbid: true}}
	output := schemaKBResponse(t, "Пришлите фото блендера X.", []string{"products.coffee-machine.featured_image"}, "")
	v := judgeOneSchemaKB(tc, schemaKBPromptfooRow(output), kb, cat, aipromptTokenValues(cat), trustedDigitsFromKB(kb))
	if len(v.UnknownMedia) != 0 {
		t.Fatalf("a real product's own valid token must NOT be UnknownMedia, got %v", v.UnknownMedia)
	}
	if v.MediaPass {
		t.Fatal("want MediaPass=false: a valid but WRONG product's media was attached where none was expected")
	}
}

// TestJudgeOneSchemaKB_MediaOmittedDespiteAvailable_FailsAnyOfExpectation is media
// regression case 5: the model has a valid, available media reference to attach but
// simply doesn't — MediaPass must fail even though nothing invalid was sent.
func TestJudgeOneSchemaKB_MediaOmittedDespiteAvailable_FailsAnyOfExpectation(t *testing.T) {
	kb, cat := loadSchemaKBJudgeContext(t)
	tc := TestCase{ID: "t", Media: &MediaExpect{AnyOf: []string{"products.coffee-machine.featured_image"}}}
	output := schemaKBResponse(t, "Кофемашина в наличии, но фото не приложу.", []string{}, "")
	v := judgeOneSchemaKB(tc, schemaKBPromptfooRow(output), kb, cat, aipromptTokenValues(cat), trustedDigitsFromKB(kb))
	if v.MediaPass {
		t.Fatal("want MediaPass=false: an available, requested media reference was omitted")
	}
}

// TestJudgeOneSchemaKB_MediaRequestedWhenFieldAbsent_HonestWithholdingPasses is media
// regression case 6: when the requested kind genuinely doesn't exist for this product
// (blender-x has no media columns at all), correctly attaching NOTHING must pass —
// proving the judge never punishes honest withholding just because a media field was
// asked about.
func TestJudgeOneSchemaKB_MediaRequestedWhenFieldAbsent_HonestWithholdingPasses(t *testing.T) {
	kb, cat := loadSchemaKBJudgeContextTwoProducts(t)
	tc := TestCase{ID: "t", Media: &MediaExpect{Forbid: true}}
	output := schemaKBResponse(t, "К сожалению, фото блендера X нет.", []string{}, "")
	v := judgeOneSchemaKB(tc, schemaKBPromptfooRow(output), kb, cat, aipromptTokenValues(cat), trustedDigitsFromKB(kb))
	if !v.MediaPass {
		t.Fatalf("want MediaPass=true: correctly attaching nothing for a media-less product, reason=%q", v.Reason)
	}
	if !v.ContractPass {
		t.Fatalf("want ContractPass=true, reason=%q", v.Reason)
	}
}

// testSchemaKBFixtureDeliveryZone is a minimal fixture carrying one delivery zone —
// used only by the Kazakh-substitution test below, which needs a boolean fact
// (delivery_available is the only one left since in_stock stopped being a fact
// token, 2026-07-26, registry.go) to prove language-aware wording substitution.
const testSchemaKBFixtureDeliveryZone = `
organization_id: "11111111-1111-1111-1111-111111111111"

ai_assistants:
  persona: "Ты — ассистент интернет-магазина «Demo Shop»."
  guardrails: "Никогда не выдумывай цены и факты."
  language_policy: "Отвечай только на русском языке."

ai_policies:
  outside_zones_note: "В другие города не доставляем."

ai_delivery_zones:
  - ref: almaty
    name: "Алматы"
    zone_level: city
    delivery_available: true
    delivery_cost: "1 500 ₸"
    delivery_in_days: "1-2"
    sales_status: active
`

func loadSchemaKBJudgeContextDeliveryZone(t *testing.T) (*aiprompt.KB, *aiprompt.Catalog) {
	t.Helper()
	dir := t.TempDir()
	fixturePath := filepath.Join(dir, "data-ru.yaml")
	mustWrite(t, fixturePath, testSchemaKBFixtureDeliveryZone)
	kb, err := kbfixture.Load(fixturePath)
	if err != nil {
		t.Fatal(err)
	}
	cat, err := aiprompt.BuildCatalog(kb)
	if err != nil {
		t.Fatal(err)
	}
	return kb, cat
}

// TestJudgeOneSchemaKB_KazakhResponse_LanguageAwareSubstitution confirms a response
// declaring reply_language "kk" is accepted (not rejected as bad_language), passes
// contract, and injects the KAZAKH delivery-availability wording — not the Russian
// one — into InjectedText. End-to-end proof (through the same judgeOneSchemaKB the
// real judge pipeline uses) that aiprompt.SubstituteFactsLang is wired correctly for
// the schema_kb_v1 judging path (2026-07 Kazakh customer-message testing).
func TestJudgeOneSchemaKB_KazakhResponse_LanguageAwareSubstitution(t *testing.T) {
	kb, cat := loadSchemaKBJudgeContextDeliveryZone(t)
	output := schemaKBResponseLang(t, "Иә, {{delivery.almaty.delivery_available}}.", []string{}, "", "kk")
	tc := TestCase{ID: "kk-test", Requires: [][]string{{"delivery.almaty.delivery_available"}}, Language: "kk"}
	v := judgeOneSchemaKB(tc, schemaKBPromptfooRow(output), kb, cat, aipromptTokenValues(cat), trustedDigitsFromKB(kb))
	if !v.ContractPass {
		t.Fatalf("ContractPass=false: %s", v.Reason)
	}
	if !strings.Contains(v.InjectedText, aiprompt.DeliveryWordingKK[true]) {
		t.Errorf("want Kazakh delivery wording %q injected, got %q", aiprompt.DeliveryWordingKK[true], v.InjectedText)
	}
	if strings.Contains(v.InjectedText, aiprompt.DeliveryWordingRU[true]) {
		t.Errorf("want Russian delivery wording NOT injected for a Kazakh reply, got %q", v.InjectedText)
	}
	if !v.RequiresPass {
		t.Errorf("RequiresPass=false, want true")
	}
}

func TestJudgeOneSchemaKB_RereadsMediaColumn(t *testing.T) {
	kb, cat := loadSchemaKBJudgeContext(t)
	kb.Products[0].FeaturedImage = ""
	output := schemaKBResponse(
		t,
		"Прикрепляю фото.",
		[]string{"products.coffee-machine.featured_image"},
		"",
	)
	v := judgeSchemaKBOutput(t, kb, cat, output)
	if v.MediaResolveOK || v.ContractPass {
		t.Fatalf("MediaResolveOK=%v ContractPass=%v reason=%q", v.MediaResolveOK, v.ContractPass, v.Reason)
	}
}

// schemaKBResponseEscalate builds a Response with explicit, independent control over
// Escalate and ReplyText — schemaKBResponse/schemaKBResponseLang always couple Escalate
// to "was an escalation_reason given," which the escalate/text-consistency check
// specifically needs to pull apart: a reply can deflect to a manager in its TEXT while
// its Escalate field says false.
func schemaKBResponseEscalate(t *testing.T, text string, escalate bool) string {
	t.Helper()
	b, err := json.Marshal(aiprompt.Response{
		ReplyText:        text,
		ReplyLanguage:    "ru",
		MediaFilesToSend: []string{},
		Escalate:         escalate,
		Confidence:       0.9,
	})
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// TestJudgeOneSchemaKB_EscalateTextConsistency mirrors judge_test.go's
// TestJudgeOne_EscalateTextConsistency for the schema_kb_v1 pipeline — same regexes
// (managerDeflectionPatterns, judge.go), same pinned pickup-topic negative control.
func TestJudgeOneSchemaKB_EscalateTextConsistency(t *testing.T) {
	kb, cat := loadSchemaKBJudgeContext(t)

	tests := []struct {
		name             string
		replyText        string
		escalate         bool
		wantBehaviorPass bool
	}{
		{
			name:             "canonical RU holding phrase with escalate=false contradicts itself",
			replyText:        "Сейчас у меня нет этой информации — передаю ваш вопрос менеджеру и вернусь с точным ответом.",
			escalate:         false,
			wantBehaviorPass: false,
		},
		{
			name:             "same text with escalate=true is consistent",
			replyText:        "Сейчас у меня нет этой информации — передаю ваш вопрос менеджеру и вернусь с точным ответом.",
			escalate:         true,
			wantBehaviorPass: true,
		},
		{
			name:             "pickup topic honestly mentions a manager without deflecting — must NOT trip",
			replyText:        "Самовывоз возможен со склада в Алматы по предварительной договорённости с менеджером.",
			escalate:         false,
			wantBehaviorPass: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := schemaKBResponseEscalate(t, tt.replyText, tt.escalate)
			v := judgeSchemaKBOutput(t, kb, cat, output)
			if !v.ContractPass {
				t.Fatalf("precondition failed: ContractPass=false, reason=%s", v.Reason)
			}
			if !v.EscalateTextConsistencyEvaluated {
				t.Fatalf("want EscalateTextConsistencyEvaluated=true, got false")
			}
			if v.EscalateTextConsistencyPass != tt.wantBehaviorPass {
				t.Errorf("EscalateTextConsistencyPass = %v, want %v (DeflectionPhrase=%q)", v.EscalateTextConsistencyPass, tt.wantBehaviorPass, v.DeflectionPhrase)
			}
			if v.ModelBehaviorPass != tt.wantBehaviorPass {
				t.Errorf("ModelBehaviorPass = %v, want %v; reason=%s", v.ModelBehaviorPass, tt.wantBehaviorPass, v.Reason)
			}
		})
	}
}
