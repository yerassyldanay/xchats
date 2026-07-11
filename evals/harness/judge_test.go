package main

import (
	"strings"
	"testing"
)

func TestJudgeOne_DeterministicChecks(t *testing.T) {
	trueValue := true
	catalog := &Catalog{
		Contract: "attach_groups",
		Tokens: []CatalogFact{
			{Token: "{{product.coffee-machine.price}}", Value: "129 900 ₸"},
			{Token: "{{policy.main.delivery_cost}}", Value: "1 500 ₸"},
		},
		MediaGroups:   []string{"product.coffee-machine.images"},
		TrustedDigits: []string{"1", "7"}, // as if a row's Description mentioned "1.7 л"
	}
	tokenValue := map[string]string{
		"{{product.coffee-machine.price}}": "129 900 ₸",
		"{{policy.main.delivery_cost}}":    "1 500 ₸",
	}
	validGroups := map[string]bool{
		"product.coffee-machine.images": true,
	}

	tests := []struct {
		name             string
		testCase         TestCase
		output           string
		wantContractPass bool
		wantBehaviorPass bool
		wantReason       string
	}{
		{
			name: "valid token and media pass",
			testCase: TestCase{
				ID:       "price",
				Requires: [][]string{{"product.coffee-machine.price"}},
				Media:    &MediaExpect{AnyOfGroups: []string{"product.coffee-machine.images"}},
			},
			output:           `{"reply_text":"Цена {{product.coffee-machine.price}}.","reply_language":"ru","attach_groups":["product.coffee-machine.images"],"escalate":false}`,
			wantContractPass: true,
			wantBehaviorPass: true,
			wantReason:       "ok",
		},
		{
			name: "wrong typed fields fail contract",
			testCase: TestCase{
				ID: "bad-contract",
			},
			output:           `{"reply_text":"ok","reply_language":7,"attach_groups":[],"escalate":"true"}`,
			wantContractPass: false,
			wantBehaviorPass: true,
			wantReason:       "missing or wrong-typed contract field (need string reply_text, string reply_language, bool escalate, array attach_groups)",
		},
		{
			name: "unknown media fails behavior",
			testCase: TestCase{
				ID: "unknown-media",
			},
			output:           `{"reply_text":"Вот фото.","reply_language":"ru","attach_groups":["product.fake.images"],"escalate":false}`,
			wantContractPass: true,
			wantBehaviorPass: false,
			wantReason:       "attached media not in the catalog: product.fake.images",
		},
		{
			name: "must not contain catches invented escalation answer",
			testCase: TestCase{
				ID:             "astana",
				Escalate:       &trueValue,
				MustNotContain: []string{"не доставляем в астан"},
			},
			output:           `{"reply_text":"К сожалению, мы не доставляем в Астану. Уточню у коллеги.","reply_language":"ru","attach_groups":[],"escalate":true}`,
			wantContractPass: true,
			wantBehaviorPass: false,
			wantReason:       `escalated, but reply_text still commits to an invented answer ("не доставляем в астан")`,
		},
		{
			name: "duplicated currency fails behavior",
			testCase: TestCase{
				ID:       "delivery-cost",
				Requires: [][]string{{"policy.main.delivery_cost"}},
			},
			output:           `{"reply_text":"Доставка стоит {{policy.main.delivery_cost}} ₸.","reply_language":"ru","attach_groups":[],"escalate":false}`,
			wantContractPass: true,
			wantBehaviorPass: false,
			wantReason:       "unit/currency issue after injection: duplicated tenge symbol",
		},
		{
			name:             "single-digit invented number fails behavior",
			testCase:         TestCase{ID: "single-digit-invented"},
			output:           `{"reply_text":"Осталось 5 штук.","reply_language":"ru","attach_groups":[],"escalate":false}`,
			wantContractPass: true,
			wantBehaviorPass: false,
			wantReason:       "invented digits outside any token: 5",
		},
		{
			name:             "digit from a trusted product description is not invented",
			testCase:         TestCase{ID: "description-digit-ok"},
			output:           `{"reply_text":"Это чайник на 1.7 л.","reply_language":"ru","attach_groups":[],"escalate":false}`,
			wantContractPass: true,
			wantBehaviorPass: true,
			wantReason:       "ok",
		},
		{
			name: "digit echoed back from the customer's own message is not invented",
			testCase: TestCase{
				ID:      "echoed-digit-ok",
				Message: "У вас есть iPhone 15 Pro?",
			},
			output:           `{"reply_text":"К сожалению, iPhone 15 Pro у нас нет в наличии.","reply_language":"ru","attach_groups":[],"escalate":true}`,
			wantContractPass: true,
			wantBehaviorPass: true,
			wantReason:       "ok",
		},
		{
			name:             "numbered list markers (dot style) are not invented digits",
			testCase:         TestCase{ID: "numbered-list-ok"},
			output:           `{"reply_text":"Как оформить:\n1. Напишите адрес.\n2. Подтвердите заказ.","reply_language":"ru","attach_groups":[],"escalate":false}`,
			wantContractPass: true,
			wantBehaviorPass: true,
			wantReason:       "ok",
		},
		{
			name:             "numbered list markers (paren style) are not invented digits",
			testCase:         TestCase{ID: "numbered-list-paren-ok"},
			output:           `{"reply_text":"Нужно:\n1) Подтвердить\n2) Указать адрес\n3) Мы пришлём счёт","reply_language":"ru","attach_groups":[],"escalate":false}`,
			wantContractPass: true,
			wantBehaviorPass: true,
			wantReason:       "ok",
		},
		{
			name:             "mangled single-brace token fails contract",
			testCase:         TestCase{ID: "mangled-token"},
			output:           `{"reply_text":"Цена {product.coffee-machine.price}.","reply_language":"ru","attach_groups":[],"escalate":false}`,
			wantContractPass: false,
			wantBehaviorPass: true,
			wantReason:       "leftover brace survived injection",
		},
		{
			name:             "unknown token in escalation_reason blocks the draft",
			testCase:         TestCase{ID: "escalation-reason-unknown-token"},
			output:           `{"reply_text":"Хорошо.","reply_language":"ru","attach_groups":[],"escalate":true,"escalation_reason":"Нужно уточнить {{product.unknown.field}}"}`,
			wantContractPass: false,
			wantBehaviorPass: true,
			wantReason:       "unknown token(s), draft would be BLOCKED: {{product.unknown.field}}",
		},
		{
			name: "reply_language field mismatch fails language check even if text looks right",
			testCase: TestCase{
				ID:       "kk-field-mismatch",
				Language: "kk",
			},
			output:           `{"reply_text":"Бұл сөйлем қазақша және қазақ әріптері бар: ә ғ қ.","reply_language":"ru","attach_groups":[],"escalate":false}`,
			wantContractPass: true,
			wantBehaviorPass: false,
			wantReason:       `reply_language field is "ru", expected "kk"`,
		},
		{
			name: "Kazakh-looking text fails when Russian was expected",
			testCase: TestCase{
				ID:       "ru-text-is-kazakh",
				Language: "ru",
			},
			output:           `{"reply_text":"Бұл сөйлем қазақша ә ғ қ.","reply_language":"ru","attach_groups":[],"escalate":false}`,
			wantContractPass: true,
			wantBehaviorPass: false,
			wantReason:       "reply looks like Kazakh but a Russian reply was expected",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			row := PromptfooRow{}
			row.Provider.ID = "test-model"
			row.Response.Output = tt.output

			got := judgeOne(
				tt.testCase,
				row,
				catalog,
				tokenValue,
				nil,
				validGroups,
			)
			if got.ContractPass != tt.wantContractPass {
				t.Fatalf("ContractPass = %v, want %v; reason=%s", got.ContractPass, tt.wantContractPass, got.Reason)
			}
			if got.ModelBehaviorPass != tt.wantBehaviorPass {
				t.Fatalf("ModelBehaviorPass = %v, want %v; reason=%s", got.ModelBehaviorPass, tt.wantBehaviorPass, got.Reason)
			}
			if got.Reason != tt.wantReason {
				t.Fatalf("Reason = %q, want %q", got.Reason, tt.wantReason)
			}
		})
	}
}

// TestJudgeOne_LanguageTextOKAndFieldOKAreIndependentSignals pins the exact failure mode
// the plan's Phase 0.4 flagged: LanguagePass alone can't distinguish "the text didn't read
// as Kazakh" from "the model declared the wrong reply_language field" — both are real,
// distinct ways a language check can fail, and a Kazakh canary run needs to tell them
// apart when someone is manually inspecting results, not just see one combined boolean.
func TestJudgeOne_LanguageTextOKAndFieldOKAreIndependentSignals(t *testing.T) {
	catalog := &Catalog{Contract: "attach_groups"}

	t.Run("text looks Kazakh, field wrongly says ru", func(t *testing.T) {
		row := PromptfooRow{}
		row.Provider.ID = "test-model"
		row.Response.Output = `{"reply_text":"Бұл сөйлем қазақша және қазақ әріптері бар: ә ғ қ.","reply_language":"ru","attach_groups":[],"escalate":false}`
		got := judgeOne(TestCase{ID: "t", Language: "kk"}, row, catalog, map[string]string{}, nil, map[string]bool{})
		if !got.LanguageTextOK {
			t.Error("want LanguageTextOK=true (the text does read as Kazakh)")
		}
		if got.LanguageFieldOK {
			t.Error("want LanguageFieldOK=false (declared reply_language is ru, not kk)")
		}
		if got.LanguagePass {
			t.Error("want LanguagePass=false (combined check must still fail)")
		}
	})

	t.Run("field correctly says kk, text does not look Kazakh", func(t *testing.T) {
		row := PromptfooRow{}
		row.Provider.ID = "test-model"
		row.Response.Output = `{"reply_text":"Здравствуйте, чем могу помочь?","reply_language":"kk","attach_groups":[],"escalate":false}`
		got := judgeOne(TestCase{ID: "t", Language: "kk"}, row, catalog, map[string]string{}, nil, map[string]bool{})
		if got.LanguageTextOK {
			t.Error("want LanguageTextOK=false (the text is plain Russian, no Kazakh-specific letters)")
		}
		if !got.LanguageFieldOK {
			t.Error("want LanguageFieldOK=true (declared reply_language correctly says kk)")
		}
		if got.LanguagePass {
			t.Error("want LanguagePass=false (combined check must still fail — this is the exact 'model lies about its own language field' case)")
		}
	})

	t.Run("both correct", func(t *testing.T) {
		row := PromptfooRow{}
		row.Provider.ID = "test-model"
		row.Response.Output = `{"reply_text":"Жеткізу қанша тұрады?","reply_language":"kk","attach_groups":[],"escalate":false}`
		got := judgeOne(TestCase{ID: "t", Language: "kk"}, row, catalog, map[string]string{}, nil, map[string]bool{})
		if !got.LanguageTextOK || !got.LanguageFieldOK || !got.LanguagePass {
			t.Errorf("want all three true, got TextOK=%v FieldOK=%v Pass=%v", got.LanguageTextOK, got.LanguageFieldOK, got.LanguagePass)
		}
	})
}

func TestRenderFieldUsage(t *testing.T) {
	tests := []struct {
		name string
		in   FieldSpec
		want string
	}{
		{
			name: "money display",
			in:   FieldSpec{ValueKind: "money_display"},
			want: "value already includes currency; do not add ₸/тенге",
		},
		{
			name: "localized number",
			in:   FieldSpec{ValueKind: "number_range", UnitRU: "дня", UnitKK: "күн"},
			want: "number only; add unit in reply language (ru: дня; kk: күн)",
		},
		{
			name: "unknown kind",
			in:   FieldSpec{},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := renderFieldUsage(tt.in); got != tt.want {
				t.Fatalf("renderFieldUsage() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestIsTruncatedFinish(t *testing.T) {
	tests := []struct {
		reason string
		want   bool
	}{
		{"length", true},
		{"stop", false},
		{"content_filter", false}, // a real but different failure mode — not claimed here
		{"tool_calls", false},
		{"", false}, // "not reported" must never be treated as truncation
	}
	for _, tt := range tests {
		if got := isTruncatedFinish(tt.reason); got != tt.want {
			t.Errorf("isTruncatedFinish(%q) = %v, want %v", tt.reason, got, tt.want)
		}
	}
}

func TestProviderModelKey(t *testing.T) {
	if got := providerModelKey("openrouter:google/gemini-2.5-flash", ""); got != "openrouter:google/gemini-2.5-flash" {
		t.Errorf("empty label should leave the id unchanged, got %q", got)
	}
	if got := providerModelKey("openrouter:google/gemini-2.5-flash", "reasoning-on"); got != "openrouter:google/gemini-2.5-flash [reasoning-on]" {
		t.Errorf("got %q", got)
	}
	offKey := providerModelKey("openrouter:google/gemini-2.5-flash", "reasoning-off")
	onKey := providerModelKey("openrouter:google/gemini-2.5-flash", "reasoning-on")
	if offKey == onKey {
		t.Fatalf("two different labels on the same id must produce different keys, both got %q", offKey)
	}
}

// TestJudgeOne_TruncatedFinishReasonFailsContract pins the one-line-addition premise:
// a response that otherwise parses and satisfies every other check must still fail
// ContractPass when finish_reason=length — a truncated response is a pipeline-safety
// issue, the same category as an unknown token or a leftover brace, regardless of what
// happened to parse successfully.
func TestJudgeOne_TruncatedFinishReasonFailsContract(t *testing.T) {
	catalog := &Catalog{Contract: "attach_groups"}
	row := PromptfooRow{}
	row.Provider.ID = "test-model"
	row.Response.Output = `{"reply_text":"ok","reply_language":"ru","attach_groups":[],"escalate":false}`
	row.Response.FinishReason = "length"

	v := judgeOne(TestCase{ID: "t"}, row, catalog, map[string]string{}, nil, map[string]bool{})
	if !v.Truncated {
		t.Error("want Truncated=true")
	}
	if v.ContractPass {
		t.Error("want ContractPass=false when finish_reason=length, regardless of otherwise-valid output")
	}
	if !strings.Contains(v.Reason, "truncated") {
		t.Errorf("want Reason to mention truncation, got %q", v.Reason)
	}
}

// TestJudgeOne_EmptyFinishReasonDoesNotFailContract guards backward compatibility: the
// many existing PromptfooRow{} fixtures built before FinishReason existed leave it at
// its zero value (""), and that must keep behaving exactly as before — never treated as
// truncation.
func TestJudgeOne_EmptyFinishReasonDoesNotFailContract(t *testing.T) {
	catalog := &Catalog{Contract: "attach_groups"}
	row := PromptfooRow{}
	row.Provider.ID = "test-model"
	row.Response.Output = `{"reply_text":"ok","reply_language":"ru","attach_groups":[],"escalate":false}`

	v := judgeOne(TestCase{ID: "t"}, row, catalog, map[string]string{}, nil, map[string]bool{})
	if v.Truncated {
		t.Error("want Truncated=false for an empty (unreported) finish_reason")
	}
	if !v.ContractPass {
		t.Errorf("want ContractPass=true, got false with reason %q", v.Reason)
	}
}

// TestJudgeOne_TruncatedAndUnparseableCombinesReasons proves the common real-world case
// (truncation is often the CAUSE of a parse failure) still surfaces the truncation fact,
// not just a generic "could not parse" message.
func TestJudgeOne_TruncatedAndUnparseableCombinesReasons(t *testing.T) {
	catalog := &Catalog{Contract: "attach_groups"}
	row := PromptfooRow{}
	row.Provider.ID = "test-model"
	row.Response.Output = `{"reply_text":"Кофемашина сто` // cut off mid-string, invalid JSON
	row.Response.FinishReason = "length"

	v := judgeOne(TestCase{ID: "t"}, row, catalog, map[string]string{}, nil, map[string]bool{})
	if v.ParseOK {
		t.Fatal("want ParseOK=false for genuinely truncated JSON")
	}
	if !v.Truncated {
		t.Error("want Truncated=true")
	}
	if v.ContractPass {
		t.Error("want ContractPass=false")
	}
	if !strings.Contains(v.Reason, "could not parse") || !strings.Contains(v.Reason, "truncated") {
		t.Errorf("want Reason to mention both parse failure and truncation, got %q", v.Reason)
	}
}

// TestJudgeOne_ReasoningLeakInReplyTextFailsContractAndSuppressesInjectedText is the
// concrete guarantee behind "reasoning/thinking payloads never leak into reply_text or
// into any report meant to show customer-facing output": a model that embeds a <think>
// block INSIDE an otherwise-valid reply_text string (a real, observed OpenRouter failure
// mode, independent of whether reasoning was even requested) must fail ContractPass, AND
// InjectedText — the one field documented and rendered everywhere as "the actual
// customer-facing text" (CONTRACT.md, the HTML viewer) — must stay empty, exactly like a
// Blocked response, even though every token in the leaked text would otherwise have
// resolved cleanly.
func TestJudgeOne_ReasoningLeakInReplyTextFailsContractAndSuppressesInjectedText(t *testing.T) {
	catalog := &Catalog{Contract: "attach_groups", Tokens: []CatalogFact{
		{Token: "{{product.coffee-machine.price}}", Value: "129 900 ₸"},
	}}
	tokenValue := map[string]string{"{{product.coffee-machine.price}}": "129 900 ₸"}

	row := PromptfooRow{}
	row.Provider.ID = "test-model"
	row.Response.Output = `{"reply_text":"<think>the customer wants the price, I should state it</think>Цена {{product.coffee-machine.price}}.","reply_language":"ru","attach_groups":[],"escalate":false}`

	v := judgeOne(TestCase{ID: "t"}, row, catalog, tokenValue, nil, map[string]bool{})
	if !v.ParseOK {
		t.Fatal("want ParseOK=true — the JSON itself is well-formed, only its content leaks")
	}
	if !v.ReasoningLeak {
		t.Error("want ReasoningLeak=true")
	}
	if v.ContractPass {
		t.Error("want ContractPass=false when reply_text contains a reasoning marker")
	}
	if v.InjectedText != "" {
		t.Errorf("want InjectedText suppressed (empty) on a reasoning leak, got %q — this is the actual leak into a customer-facing field", v.InjectedText)
	}
	if !strings.Contains(v.Reason, "reasoning") {
		t.Errorf("want Reason to mention the reasoning leak, got %q", v.Reason)
	}
}

// TestJudgeOne_ReasonMentionsBothBlockedAndReasoningLeak is the regression test for a
// bug review caught: Blocked's reason-set unconditionally overwrites v.Reason (existing,
// pre-diff behavior — it already did this to ContractFields' message), so a verdict that
// is BOTH blocked (unknown token) AND leaking reasoning used to silently lose the leak
// fact from the one human-readable Reason string, even though v.ReasoningLeak itself
// stayed correctly true and ContractPass still correctly failed either way.
func TestJudgeOne_ReasonMentionsBothBlockedAndReasoningLeak(t *testing.T) {
	catalog := &Catalog{Contract: "attach_groups"}
	row := PromptfooRow{}
	row.Provider.ID = "test-model"
	row.Response.Output = `{"reply_text":"<think>let me check the unknown fact</think>{{product.unknown.field}}","reply_language":"ru","attach_groups":[],"escalate":false}`

	v := judgeOne(TestCase{ID: "t"}, row, catalog, map[string]string{}, nil, map[string]bool{})
	if !v.Blocked {
		t.Fatal("precondition failed: want Blocked=true (unresolvable token)")
	}
	if !v.ReasoningLeak {
		t.Fatal("precondition failed: want ReasoningLeak=true")
	}
	if !strings.Contains(v.Reason, "BLOCKED") {
		t.Errorf("want Reason to still mention the block, got %q", v.Reason)
	}
	if !strings.Contains(v.Reason, "reasoning") {
		t.Errorf("want Reason to ALSO mention the reasoning leak, not just the block, got %q", v.Reason)
	}
}

// TestApplyCostEstimate_CachedRowNeverBorrowsAcrossLabels is the regression test for the
// collision the Label plumbing exists to prevent: a cached row for one labeled variant
// (e.g. reasoning-off) must never borrow a fresh token split recorded under a DIFFERENT
// label sharing the same underlying provider ID (e.g. reasoning-on) — their token
// profiles are expected to differ substantially (reasoning burns many more completion
// tokens), so borrowing across labels would silently corrupt the cost estimate.
func TestApplyCostEstimate_CachedRowNeverBorrowsAcrossLabels(t *testing.T) {
	price := 1.0
	priceByModel := map[string]ModelProvider{
		"test/model [reasoning-off]": {ID: "test/model", Label: "reasoning-off", InputPerMTok: &price, OutputPerMTok: &price},
		"test/model [reasoning-on]":  {ID: "test/model", Label: "reasoning-on", InputPerMTok: &price, OutputPerMTok: &price},
	}
	freshSplit := map[string]tokenSplit{
		"test/model [reasoning-on]|t1": {in: 100, out: 900},
	}

	cachedOffRow := PromptfooRow{}
	cachedOffRow.Provider.ID = "test/model"
	cachedOffRow.Provider.Label = "reasoning-off"
	cachedOffRow.TestCase.Description = "t1"
	cachedOffRow.Response.Cached = true

	var v Verdict
	applyCostEstimate(&v, cachedOffRow, priceByModel, freshSplit)

	if v.CostBasis != CostBasisCachedUnpriced {
		t.Fatalf("want cached_replay_unpriceable (no reasoning-off fresh split exists to borrow — the reasoning-on split must NOT be used), got %s (tokens in=%d out=%d)", v.CostBasis, v.TokensIn, v.TokensOut)
	}
}

// TestBuildContractReport_NeverEchoesReasoningLeakIntoInjectedTextLine proves the report
// layer's side of the same guarantee: even when a verdict's RawOutput carries a leaked
// <think> block, buildContractReport (CONTRACT.md) never prints it as injected/
// customer-facing text — it only ever prints v.InjectedText, which judgeOne already
// leaves empty on a reasoning leak (see the judgeOne test above).
func TestBuildContractReport_NeverEchoesReasoningLeakIntoInjectedTextLine(t *testing.T) {
	catalog := &Catalog{Contract: "attach_groups"}
	row := PromptfooRow{}
	row.Provider.ID = "test-model"
	row.Response.Output = `{"reply_text":"<think>internal chain of thought, never meant for a customer</think>ok","reply_language":"ru","attach_groups":[],"escalate":false}`
	v := judgeOne(TestCase{ID: "t"}, row, catalog, map[string]string{}, nil, map[string]bool{})

	report := buildContractReport([]JudgedRun{{Scenario: "fixture", Verdicts: []Verdict{v}}})
	if strings.Contains(report, "injected text:") {
		t.Errorf("want no 'injected text:' line at all for a reasoning-leaking verdict (InjectedText must stay empty), got:\n%s", report)
	}
	if !strings.Contains(report, "REASONING LEAK") {
		t.Error("want the report to flag the reasoning leak explicitly")
	}
}
