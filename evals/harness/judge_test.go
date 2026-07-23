package main

import (
	"strings"
	"testing"
)

func TestJudgeOne_DeterministicChecks(t *testing.T) {
	trueValue := true
	catalog := &Catalog{
		Tokens: []CatalogFact{
			{Token: "{{product.coffee-machine.price}}", Value: "129 900 ₸"},
			{Token: "{{policy.main.delivery_cost}}", Value: "1 500 ₸"},
			{Token: "{{policy.main.return_period}}", Value: "14 дней"},
		},
		MediaTokens:   []string{"product.coffee-machine.images", "product.cookware-set.images"},
		TrustedDigits: []string{"1", "7"}, // as if a row's Description mentioned "1.7 л"
	}
	tokenValue := map[string]string{
		"{{product.coffee-machine.price}}": "129 900 ₸",
		"{{policy.main.delivery_cost}}":    "1 500 ₸",
		"{{policy.main.return_period}}":    "14 дней",
	}
	validMedia := map[string]bool{
		"product.coffee-machine.images": true,
		"product.cookware-set.images":   true,
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
				Media:    &MediaExpect{AnyOf: []string{"product.coffee-machine.images"}},
			},
			output:           `{"reply_text":"Цена {{product.coffee-machine.price}}.","reply_language":"ru","media_files_to_send":["product.coffee-machine.images"],"escalate":false,"escalation_reason":"","confidence":0.9}`,
			wantContractPass: true,
			wantBehaviorPass: true,
			wantReason:       "ok",
		},
		{
			name: "wrong typed fields fail contract",
			testCase: TestCase{
				ID: "bad-contract",
			},
			output:           `{"reply_text":"ok","reply_language":7,"media_files_to_send":[],"escalate":"true"}`,
			wantContractPass: false,
			wantBehaviorPass: true,
			wantReason:       "missing or wrong-typed contract field (need string reply_text, string reply_language, bool escalate, array media_files_to_send of strings)",
		},
		{
			name: "unknown media fails behavior",
			testCase: TestCase{
				ID: "unknown-media",
			},
			output:           `{"reply_text":"Вот фото.","reply_language":"ru","media_files_to_send":["product.fake.images"],"escalate":false,"escalation_reason":"","confidence":0.9}`,
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
			output:           `{"reply_text":"К сожалению, мы не доставляем в Астану. Уточню у коллеги.","reply_language":"ru","media_files_to_send":[],"escalate":true,"escalation_reason":"","confidence":0.9}`,
			wantContractPass: true,
			wantBehaviorPass: false,
			wantReason:       `reply_text contains forbidden phrase: "не доставляем в астан"`,
		},
		{
			// The regression test for the injection-hole fix: "в течение 14 дней" is not
			// written by the model at all — it only exists because
			// {{policy.main.return_period}} injects to "14 дней". Scanning replyText (the
			// pre-injection bug) would have missed this entirely.
			name: "must not contain catches a phrase that only materializes after token injection",
			testCase: TestCase{
				ID:             "refund-injected",
				Escalate:       &trueValue,
				MustNotContain: []string{"в течение 14 дней"},
			},
			output:           `{"reply_text":"Передам коллеге. Возврат обычно возможен в течение {{policy.main.return_period}}.","reply_language":"ru","media_files_to_send":[],"escalate":true,"escalation_reason":"","confidence":0.9}`,
			wantContractPass: true,
			wantBehaviorPass: false,
			wantReason:       `reply_text contains forbidden phrase: "в течение 14 дней"`,
		},
		{
			name: "forbid_tokens exact match fails behavior",
			testCase: TestCase{
				ID:           "forbid-exact",
				ForbidTokens: []string{"policy.main.delivery_cost"},
			},
			output:           `{"reply_text":"Стоимость доставки {{policy.main.delivery_cost}}.","reply_language":"ru","media_files_to_send":[],"escalate":false,"escalation_reason":"","confidence":0.9}`,
			wantContractPass: true,
			wantBehaviorPass: false,
			wantReason:       "reply_text cites a forbidden fact token: {{policy.main.delivery_cost}}",
		},
		{
			name: "forbid_tokens dotted-prefix match fails behavior",
			testCase: TestCase{
				ID:           "forbid-prefix",
				ForbidTokens: []string{"policy."},
			},
			output:           `{"reply_text":"Срок возврата {{policy.main.return_period}}.","reply_language":"ru","media_files_to_send":[],"escalate":false,"escalation_reason":"","confidence":0.9}`,
			wantContractPass: true,
			wantBehaviorPass: false,
			wantReason:       "reply_text cites a forbidden fact token: {{policy.main.return_period}}",
		},
		{
			name: "forbid_tokens passes when a different token is used",
			testCase: TestCase{
				ID:           "forbid-ok",
				Requires:     [][]string{{"product.coffee-machine.price"}},
				ForbidTokens: []string{"policy."},
			},
			output:           `{"reply_text":"Цена {{product.coffee-machine.price}}.","reply_language":"ru","media_files_to_send":[],"escalate":false,"escalation_reason":"","confidence":0.9}`,
			wantContractPass: true,
			wantBehaviorPass: true,
			wantReason:       "ok",
		},
		{
			name: "media forbid passes when the reply attaches nothing",
			testCase: TestCase{
				ID:    "greeting-no-media",
				Media: &MediaExpect{Forbid: true},
			},
			output:           `{"reply_text":"Здравствуйте! Чем помочь?","reply_language":"ru","media_files_to_send":[],"escalate":false,"escalation_reason":"","confidence":0.9}`,
			wantContractPass: true,
			wantBehaviorPass: true,
			wantReason:       "ok",
		},
		{
			name: "media forbid fails when the reply attaches something anyway",
			testCase: TestCase{
				ID:    "greeting-with-media",
				Media: &MediaExpect{Forbid: true},
			},
			output:           `{"reply_text":"Здравствуйте! Вот фото.","reply_language":"ru","media_files_to_send":["product.coffee-machine.images"],"escalate":false,"escalation_reason":"","confidence":0.9}`,
			wantContractPass: true,
			wantBehaviorPass: false,
			wantReason:       "attached media, but this test forbids any attachment",
		},
		{
			name: "exclusive media passes when only the expected group is attached",
			testCase: TestCase{
				ID:    "cookware-photos-exclusive",
				Media: &MediaExpect{AnyOf: []string{"product.cookware-set.images"}, Exclusive: true},
			},
			output:           `{"reply_text":"Вот фото набора посуды.","reply_language":"ru","media_files_to_send":["product.cookware-set.images"],"escalate":false,"escalation_reason":"","confidence":0.9}`,
			wantContractPass: true,
			wantBehaviorPass: true,
			wantReason:       "ok",
		},
		{
			// The extra group is itself VALID (in validGroups) and normally would pass
			// mediaExpectationMet's "at least one of these" check — Exclusive additionally
			// requires nothing else be attached, so a real, catalog-known but unrequested
			// group must still fail here, distinct from the UnknownMedia check.
			name: "exclusive media fails when a valid but unrequested group is attached alongside the expected one",
			testCase: TestCase{
				ID:    "cookware-photos-exclusive-plus-extra",
				Media: &MediaExpect{AnyOf: []string{"product.cookware-set.images"}, Exclusive: true},
			},
			output:           `{"reply_text":"Вот фото.","reply_language":"ru","media_files_to_send":["product.cookware-set.images","product.coffee-machine.images"],"escalate":false,"escalation_reason":"","confidence":0.9}`,
			wantContractPass: true,
			wantBehaviorPass: false,
			wantReason:       "attached media outside the expected set: product.coffee-machine.images",
		},
		{
			name:             "media count within the media_files_to_send cap (2) passes",
			testCase:         TestCase{ID: "media-count-groups-ok"},
			output:           `{"reply_text":"Вот фото.","reply_language":"ru","media_files_to_send":["product.coffee-machine.images","product.cookware-set.images"],"escalate":false,"escalation_reason":"","confidence":0.9}`,
			wantContractPass: true,
			wantBehaviorPass: true,
			wantReason:       "ok",
		},
		{
			name:             "media count over the media_files_to_send cap (2) fails",
			testCase:         TestCase{ID: "media-count-groups-over"},
			output:           `{"reply_text":"Вот фото.","reply_language":"ru","media_files_to_send":["product.coffee-machine.images","product.cookware-set.images","product.coffee-machine.images"],"escalate":false,"escalation_reason":"","confidence":0.9}`,
			wantContractPass: true,
			wantBehaviorPass: false,
			wantReason:       "attached 3 media entries — over the frame's cap",
		},
		{
			// Duplicates count toward the cap — the SAME ref repeated 4 times is still 4
			// entries, not "1 distinct entry."
			name:             "duplicated refs count toward the cap (same entry repeated is not 1 distinct entry)",
			testCase:         TestCase{ID: "media-count-duplicates"},
			output:           `{"reply_text":"Вот фото.","reply_language":"ru","media_files_to_send":["product.coffee-machine.images","product.coffee-machine.images","product.coffee-machine.images","product.coffee-machine.images"],"escalate":false,"escalation_reason":"","confidence":0.9}`,
			wantContractPass: true,
			wantBehaviorPass: false,
			wantReason:       "attached 4 media entries — over the frame's cap",
		},
		{
			// A non-string element fails ContractFields on its own — isolated here with
			// only 2 total entries (at the cap, not over it) so this case proves ONLY the
			// type-check, without TooManyMedia also contributing to the failure.
			name:             "non-string media element fails ContractFields",
			testCase:         TestCase{ID: "media-malformed-element"},
			output:           `{"reply_text":"Вот фото.","reply_language":"ru","media_files_to_send":["product.coffee-machine.images",7],"escalate":false,"escalation_reason":"","confidence":0.9}`,
			wantContractPass: false,
			wantBehaviorPass: true,
			wantReason:       "missing or wrong-typed contract field (need string reply_text, string reply_language, bool escalate, array media_files_to_send of strings)",
		},
		{
			// Proves MediaCount counts the RAW array, not mediaEntries' string-filtered
			// view: 2 valid refs + 1 malformed element is 3 raw entries — over the
			// media_files_to_send cap of 2 — even though mediaEntries would only report 2
			// (silently dropping the malformed one), which would have wrongly read as
			// within the cap.
			name:             "malformed element still counts toward the media cap, not silently dropped",
			testCase:         TestCase{ID: "media-malformed-element-over-cap"},
			output:           `{"reply_text":"Вот фото.","reply_language":"ru","media_files_to_send":["product.coffee-machine.images","product.cookware-set.images",7],"escalate":false,"escalation_reason":"","confidence":0.9}`,
			wantContractPass: false,
			wantBehaviorPass: false,
			wantReason:       "missing or wrong-typed contract field (need string reply_text, string reply_language, bool escalate, array media_files_to_send of strings)",
		},
		{
			name: "duplicated currency fails behavior",
			testCase: TestCase{
				ID:       "delivery-cost",
				Requires: [][]string{{"policy.main.delivery_cost"}},
			},
			output:           `{"reply_text":"Доставка стоит {{policy.main.delivery_cost}} ₸.","reply_language":"ru","media_files_to_send":[],"escalate":false,"escalation_reason":"","confidence":0.9}`,
			wantContractPass: true,
			wantBehaviorPass: false,
			wantReason:       "unit/currency issue after injection: duplicated tenge symbol",
		},
		{
			name:             "single-digit invented number fails behavior",
			testCase:         TestCase{ID: "single-digit-invented"},
			output:           `{"reply_text":"Осталось 5 штук.","reply_language":"ru","media_files_to_send":[],"escalate":false,"escalation_reason":"","confidence":0.9}`,
			wantContractPass: true,
			wantBehaviorPass: false,
			wantReason:       "invented digits outside any token: 5",
		},
		{
			name:             "digit from a trusted product description is not invented",
			testCase:         TestCase{ID: "description-digit-ok"},
			output:           `{"reply_text":"Это чайник на 1.7 л.","reply_language":"ru","media_files_to_send":[],"escalate":false,"escalation_reason":"","confidence":0.9}`,
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
			output:           `{"reply_text":"К сожалению, iPhone 15 Pro у нас нет в наличии.","reply_language":"ru","media_files_to_send":[],"escalate":true,"escalation_reason":"","confidence":0.9}`,
			wantContractPass: true,
			wantBehaviorPass: true,
			wantReason:       "ok",
		},
		{
			name:             "numbered list markers (dot style) are not invented digits",
			testCase:         TestCase{ID: "numbered-list-ok"},
			output:           `{"reply_text":"Как оформить:\n1. Напишите адрес.\n2. Подтвердите заказ.","reply_language":"ru","media_files_to_send":[],"escalate":false,"escalation_reason":"","confidence":0.9}`,
			wantContractPass: true,
			wantBehaviorPass: true,
			wantReason:       "ok",
		},
		{
			name:             "numbered list markers (paren style) are not invented digits",
			testCase:         TestCase{ID: "numbered-list-paren-ok"},
			output:           `{"reply_text":"Нужно:\n1) Подтвердить\n2) Указать адрес\n3) Мы пришлём счёт","reply_language":"ru","media_files_to_send":[],"escalate":false,"escalation_reason":"","confidence":0.9}`,
			wantContractPass: true,
			wantBehaviorPass: true,
			wantReason:       "ok",
		},
		{
			name:             "mangled single-brace token fails contract",
			testCase:         TestCase{ID: "mangled-token"},
			output:           `{"reply_text":"Цена {product.coffee-machine.price}.","reply_language":"ru","media_files_to_send":[],"escalate":false,"escalation_reason":"","confidence":0.9}`,
			wantContractPass: false,
			wantBehaviorPass: true,
			wantReason:       "leftover brace survived injection",
		},
		{
			// escalation_reason is diagnostic-only: an unknown token inside it is never
			// scanned and never blocks the draft (unlike the same token in reply_text).
			name:             "unknown token in escalation_reason does not block the draft",
			testCase:         TestCase{ID: "escalation-reason-unknown-token"},
			output:           `{"reply_text":"Хорошо.","reply_language":"ru","media_files_to_send":[],"escalate":true,"escalation_reason":"Нужно уточнить {{product.unknown.field}}","confidence":0.5}`,
			wantContractPass: true,
			wantBehaviorPass: true,
			wantReason:       "ok",
		},
		{
			name: "reply_language field mismatch fails language check even if text looks right",
			testCase: TestCase{
				ID:       "kk-field-mismatch",
				Language: "kk",
			},
			output:           `{"reply_text":"Бұл сөйлем қазақша және қазақ әріптері бар: ә ғ қ.","reply_language":"ru","media_files_to_send":[],"escalate":false,"escalation_reason":"","confidence":0.9}`,
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
			output:           `{"reply_text":"Бұл сөйлем қазақша ә ғ қ.","reply_language":"ru","media_files_to_send":[],"escalate":false,"escalation_reason":"","confidence":0.9}`,
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
				tokenValue,
				validMedia,
				catalog.TrustedDigits,
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
	catalog := &Catalog{}

	t.Run("text looks Kazakh, field wrongly says ru", func(t *testing.T) {
		row := PromptfooRow{}
		row.Provider.ID = "test-model"
		row.Response.Output = `{"reply_text":"Бұл сөйлем қазақша және қазақ әріптері бар: ә ғ қ.","reply_language":"ru","media_files_to_send":[],"escalate":false,"escalation_reason":"","confidence":0.9}`
		got := judgeOne(TestCase{ID: "t", Language: "kk"}, row, map[string]string{}, map[string]bool{}, catalog.TrustedDigits)
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
		row.Response.Output = `{"reply_text":"Здравствуйте, чем могу помочь?","reply_language":"kk","media_files_to_send":[],"escalate":false,"escalation_reason":"","confidence":0.9}`
		got := judgeOne(TestCase{ID: "t", Language: "kk"}, row, map[string]string{}, map[string]bool{}, catalog.TrustedDigits)
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
		row.Response.Output = `{"reply_text":"Жеткізу қанша тұрады?","reply_language":"kk","media_files_to_send":[],"escalate":false,"escalation_reason":"","confidence":0.9}`
		got := judgeOne(TestCase{ID: "t", Language: "kk"}, row, map[string]string{}, map[string]bool{}, catalog.TrustedDigits)
		if !got.LanguageTextOK || !got.LanguageFieldOK || !got.LanguagePass {
			t.Errorf("want all three true, got TextOK=%v FieldOK=%v Pass=%v", got.LanguageTextOK, got.LanguageFieldOK, got.LanguagePass)
		}
	})
}

// TestLooksKazakh_DottedIIsKazakhSpecific locks і (U+0456) into kazakhOnlyLetters. The
// motivating false fail (run 2026-07-19_02-50-37-ef6e, lang-canary-v2, test 2, deepseek)
// was a fluent Kazakh sentence whose ONLY Kazakh-specific letter is і — every other
// letter is shared with the Russian alphabet, so before the і addition looksKazakh
// counted zero and judged it "does not look like Kazakh". A future revert of і from the
// set fails here loudly.
func TestLooksKazakh_DottedIIsKazakhSpecific(t *testing.T) {
	if !strings.ContainsRune(kazakhOnlyLetters, 'і') || !strings.ContainsRune(kazakhOnlyLetters, 'І') {
		t.Fatal("kazakhOnlyLetters must contain і and І (U+0456/U+0406) — see the const's doc comment before removing them")
	}

	tests := []struct {
		name string
		text string
		want bool
	}{
		{
			// The exact reply the old set falsely failed: і appears six times, no other
			// Kazakh-only letter anywhere (injected fact values are Russian on purpose).
			name: "observed false fail: Kazakh distinguished by і alone",
			text: "Кофемашина DeLonghi 129 900 ₸. Ол В наличии. Сізге онымен бірге келетін суреттерді жіберейін бе?",
			want: true,
		},
		{
			name: "plain Russian, no Kazakh-specific letters",
			text: "Обычный русский ответ без казахских букв.",
			want: false,
		},
		{
			// Threshold stays >=2: one lone і (a borrowed word, a typo) is not enough.
			name: "single і stays below the two-letter threshold",
			text: "білу",
			want: false,
		},
		{
			name: "uppercase І counts the same as lowercase",
			text: "Іні ІРІ",
			want: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := looksKazakh(tc.text); got != tc.want {
				t.Errorf("looksKazakh(%q) = %v, want %v", tc.text, got, tc.want)
			}
		})
	}

	t.Run("judgeOne passes LanguageTextOK for an і-only Kazakh reply", func(t *testing.T) {
		catalog := &Catalog{}
		row := PromptfooRow{}
		row.Provider.ID = "test-model"
		row.Response.Output = `{"reply_text":"Сізге онымен бірге келетін суреттерді жіберейін бе?","reply_language":"kk","media_files_to_send":[],"escalate":false,"escalation_reason":"","confidence":0.9}`
		got := judgeOne(TestCase{ID: "t", Language: "kk"}, row, map[string]string{}, map[string]bool{}, catalog.TrustedDigits)
		if !got.LanguageTextOK {
			t.Error("want LanguageTextOK=true — this exact sentence was the observed і-blind-spot false fail")
		}
		if !got.LanguagePass {
			t.Errorf("want LanguagePass=true, got TextOK=%v FieldOK=%v", got.LanguageTextOK, got.LanguageFieldOK)
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

// TestJudgeOne_MediaCountEvaluatedMirrorsParseOK pins the exact invariant the
// MediaCountEvaluated marker depends on: for CODE PRODUCED BY THIS judgeOne, the field is
// true whenever ParseOK is true (there is no code path between the parse succeeding and
// MediaCountEvaluated being set), and false on the one early-return path (unparseable
// JSON). This is what makes MediaCountEvaluated meaningful as a "judged by pre-upgrade
// code" marker for OLD verdicts (viewmodel.go/judge_snapshot tests) — it only works
// because a NEW verdict never has ParseOK=true with MediaCountEvaluated=false.
func TestJudgeOne_MediaCountEvaluatedMirrorsParseOK(t *testing.T) {
	catalog := &Catalog{}

	row := PromptfooRow{}
	row.Provider.ID = "test-model"
	row.Response.Output = `{"reply_text":"ok","reply_language":"ru","media_files_to_send":[],"escalate":false,"escalation_reason":"","confidence":0.9}`
	v := judgeOne(TestCase{ID: "t"}, row, map[string]string{}, map[string]bool{}, catalog.TrustedDigits)
	if !v.ParseOK || !v.MediaCountEvaluated {
		t.Errorf("want ParseOK=true and MediaCountEvaluated=true together, got ParseOK=%v MediaCountEvaluated=%v", v.ParseOK, v.MediaCountEvaluated)
	}

	badRow := PromptfooRow{}
	badRow.Provider.ID = "test-model"
	badRow.Response.Output = "not json"
	bad := judgeOne(TestCase{ID: "t"}, badRow, map[string]string{}, map[string]bool{}, catalog.TrustedDigits)
	if bad.ParseOK || bad.MediaCountEvaluated {
		t.Errorf("want ParseOK=false and MediaCountEvaluated=false together on the early-return path, got ParseOK=%v MediaCountEvaluated=%v", bad.ParseOK, bad.MediaCountEvaluated)
	}
}

// TestJudgeOne_MediaCountCap_Boundary pins the media_files_to_send attachment cap
// (maxMediaGroups, 2) at its exact boundary — every scenario shares this one cap now
// (see judge.go's TooManyMedia comment; the historical separate asset_refs cap of 3 no
// longer exists now that every scenario returns media_files_to_send).
func TestJudgeOne_MediaCountCap_Boundary(t *testing.T) {
	catalog := &Catalog{}
	validMedia := map[string]bool{"photo-1": true, "photo-2": true, "photo-3": true}

	row := PromptfooRow{}
	row.Provider.ID = "test-model"
	row.Response.Output = `{"reply_text":"Вот фото.","reply_language":"ru","media_files_to_send":["photo-1","photo-2"],"escalate":false,"escalation_reason":"","confidence":0.9}`
	within := judgeOne(TestCase{ID: "t"}, row, map[string]string{}, validMedia, catalog.TrustedDigits)
	if within.TooManyMedia || !within.ModelBehaviorPass {
		t.Errorf("want 2 media_files_to_send (at the cap) to pass, got TooManyMedia=%v ModelBehaviorPass=%v reason=%q", within.TooManyMedia, within.ModelBehaviorPass, within.Reason)
	}

	overRow := PromptfooRow{}
	overRow.Provider.ID = "test-model"
	overRow.Response.Output = `{"reply_text":"Вот фото.","reply_language":"ru","media_files_to_send":["photo-1","photo-2","photo-3"],"escalate":false,"escalation_reason":"","confidence":0.9}`
	over := judgeOne(TestCase{ID: "t"}, overRow, map[string]string{}, validMedia, catalog.TrustedDigits)
	if !over.TooManyMedia || over.ModelBehaviorPass {
		t.Errorf("want 3 media_files_to_send (over the cap) to fail, got TooManyMedia=%v ModelBehaviorPass=%v", over.TooManyMedia, over.ModelBehaviorPass)
	}
	if over.Reason != "attached 3 media entries — over the frame's cap" {
		t.Errorf("got Reason=%q", over.Reason)
	}
}

// TestJudgeOne_TruncatedFinishReasonFailsContract pins the one-line-addition premise:
// a response that otherwise parses and satisfies every other check must still fail
// ContractPass when finish_reason=length — a truncated response is a pipeline-safety
// issue, the same category as an unknown token or a leftover brace, regardless of what
// happened to parse successfully.
func TestJudgeOne_TruncatedFinishReasonFailsContract(t *testing.T) {
	catalog := &Catalog{}
	row := PromptfooRow{}
	row.Provider.ID = "test-model"
	row.Response.Output = `{"reply_text":"ok","reply_language":"ru","media_files_to_send":[],"escalate":false,"escalation_reason":"","confidence":0.9}`
	row.Response.FinishReason = "length"

	v := judgeOne(TestCase{ID: "t"}, row, map[string]string{}, map[string]bool{}, catalog.TrustedDigits)
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
	catalog := &Catalog{}
	row := PromptfooRow{}
	row.Provider.ID = "test-model"
	row.Response.Output = `{"reply_text":"ok","reply_language":"ru","media_files_to_send":[],"escalate":false,"escalation_reason":"","confidence":0.9}`

	v := judgeOne(TestCase{ID: "t"}, row, map[string]string{}, map[string]bool{}, catalog.TrustedDigits)
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
	catalog := &Catalog{}
	row := PromptfooRow{}
	row.Provider.ID = "test-model"
	row.Response.Output = `{"reply_text":"Кофемашина сто` // cut off mid-string, invalid JSON
	row.Response.FinishReason = "length"

	v := judgeOne(TestCase{ID: "t"}, row, map[string]string{}, map[string]bool{}, catalog.TrustedDigits)
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
	catalog := &Catalog{Tokens: []CatalogFact{
		{Token: "{{product.coffee-machine.price}}", Value: "129 900 ₸"},
	}}
	tokenValue := map[string]string{"{{product.coffee-machine.price}}": "129 900 ₸"}

	row := PromptfooRow{}
	row.Provider.ID = "test-model"
	row.Response.Output = `{"reply_text":"<think>the customer wants the price, I should state it</think>Цена {{product.coffee-machine.price}}.","reply_language":"ru","media_files_to_send":[],"escalate":false,"escalation_reason":"","confidence":0.9}`

	v := judgeOne(TestCase{ID: "t"}, row, tokenValue, map[string]bool{}, catalog.TrustedDigits)
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
	catalog := &Catalog{}
	row := PromptfooRow{}
	row.Provider.ID = "test-model"
	row.Response.Output = `{"reply_text":"<think>let me check the unknown fact</think>{{product.unknown.field}}","reply_language":"ru","media_files_to_send":[],"escalate":false,"escalation_reason":"","confidence":0.9}`

	v := judgeOne(TestCase{ID: "t"}, row, map[string]string{}, map[string]bool{}, catalog.TrustedDigits)
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
	catalog := &Catalog{}
	row := PromptfooRow{}
	row.Provider.ID = "test-model"
	row.Response.Output = `{"reply_text":"<think>internal chain of thought, never meant for a customer</think>ok","reply_language":"ru","media_files_to_send":[],"escalate":false,"escalation_reason":"","confidence":0.9}`
	v := judgeOne(TestCase{ID: "t"}, row, map[string]string{}, map[string]bool{}, catalog.TrustedDigits)

	report := buildContractReport([]JudgedRun{{Scenario: "fixture", Verdicts: []Verdict{v}}})
	if strings.Contains(report, "injected text:") {
		t.Errorf("want no 'injected text:' line at all for a reasoning-leaking verdict (InjectedText must stay empty), got:\n%s", report)
	}
	if !strings.Contains(report, "REASONING LEAK") {
		t.Error("want the report to flag the reasoning leak explicitly")
	}
}

// TestJudgeOne_ReasonAccumulatesAcrossEveryCombinationNotJustBlocked is the regression
// test for the gap left after the Blocked+ReasoningLeak fix above: that fix only taught
// the Blocked branch to append onto a non-empty Reason, so a ContractFields failure for
// a reason UNRELATED to reply_text (e.g. escalate wrong-typed) occurring alongside a
// reasoning leak in reply_text itself still silently dropped the leak from Reason —
// invisible from SUMMARY.md's "Failures (verbatim)" section, which prints Reason and
// nothing else (CONTRACT.md/the HTML viewer read v.ReasoningLeak directly as its own
// boolean and were never affected by this gap). appendReason (judge.go) fixes this
// generally — every fact-setting site now accumulates — not just for Blocked.
func TestJudgeOne_ReasonAccumulatesAcrossEveryCombinationNotJustBlocked(t *testing.T) {
	catalog := &Catalog{}
	row := PromptfooRow{}
	row.Provider.ID = "test-model"
	// escalate is a STRING, not a bool -> ContractFields fails for a reason having
	// nothing to do with reply_text, which is itself present, valid, AND leaking.
	row.Response.Output = `{"reply_text":"<think>internal</think>ok","reply_language":"ru","media_files_to_send":[],"escalate":"false"}`

	v := judgeOne(TestCase{ID: "t"}, row, map[string]string{}, map[string]bool{}, catalog.TrustedDigits)
	if v.ContractFields {
		t.Fatal("precondition failed: want ContractFields=false (escalate is a string, not bool)")
	}
	if !v.ReasoningLeak {
		t.Fatal("precondition failed: want ReasoningLeak=true")
	}
	if !strings.Contains(v.Reason, "missing or wrong-typed contract field") {
		t.Errorf("want Reason to still mention the contract-fields failure, got %q", v.Reason)
	}
	if !strings.Contains(v.Reason, "reasoning") {
		t.Errorf("want Reason to ALSO mention the reasoning leak, not just the contract-fields failure, got %q", v.Reason)
	}
}

// TestJudgeOne_KzAliasForKazakhFieldCheck covers the real c780 failure: minimax-m2.5
// wrote proper Kazakh text but declared reply_language "kz" (the ISO COUNTRY code,
// Kazakhstan) instead of "kk" (the ISO LANGUAGE code) — normalizeLangCode aliases the
// field check only; a model that writes "kz" but replies in RUSSIAN must still fail on
// the text heuristic.
func TestJudgeOne_KzAliasForKazakhFieldCheck(t *testing.T) {
	catalog := &Catalog{}

	t.Run("kz accepted when text is genuinely Kazakh", func(t *testing.T) {
		row := PromptfooRow{}
		row.Provider.ID = "test-model"
		row.Response.Output = `{"reply_text":"Кофемашина DeLonghi құны — 129 900 ₸. Ол қоймада бар.","reply_language":"kz","media_files_to_send":[],"escalate":false,"escalation_reason":"","confidence":0.9}`
		got := judgeOne(TestCase{ID: "t", Language: "kk"}, row, map[string]string{}, map[string]bool{}, catalog.TrustedDigits)
		if !got.LanguageFieldOK {
			t.Error("want LanguageFieldOK=true (kz aliases to kk)")
		}
		if !got.LanguagePass {
			t.Errorf("want LanguagePass=true, got issue %q", got.LanguageIssue)
		}
		if !got.LanguageAliasUsed {
			t.Error("want LanguageAliasUsed=true — the alias must stay a VISIBLE signal, not a silent pass")
		}
	})

	t.Run("kz does not launder a Russian reply", func(t *testing.T) {
		row := PromptfooRow{}
		row.Provider.ID = "test-model"
		row.Response.Output = `{"reply_text":"Кофемашина стоит 129900 тенге.","reply_language":"kz","media_files_to_send":[],"escalate":false,"escalation_reason":"","confidence":0.9}`
		got := judgeOne(TestCase{ID: "t", Language: "kk"}, row, map[string]string{}, map[string]bool{}, catalog.TrustedDigits)
		// The FIELD alias still applies (kz -> kk, so LanguageFieldOK is true) — that's
		// independent and expected. What must NOT happen is the overall LanguagePass
		// going green: the TEXT is Russian, so the combined gate must still fail.
		if got.LanguagePass {
			t.Error("want LanguagePass=false — the text is Russian, the kz field alias must not launder that")
		}
		if got.LanguageTextOK {
			t.Error("want LanguageTextOK=false — no Kazakh-specific letters in this reply")
		}
	})

	t.Run("plain ru vs kk still fails, no alias involved", func(t *testing.T) {
		row := PromptfooRow{}
		row.Provider.ID = "test-model"
		row.Response.Output = `{"reply_text":"Жеткізу қанша тұрады?","reply_language":"ru","media_files_to_send":[],"escalate":false,"escalation_reason":"","confidence":0.9}`
		got := judgeOne(TestCase{ID: "t", Language: "kk"}, row, map[string]string{}, map[string]bool{}, catalog.TrustedDigits)
		if got.LanguageFieldOK {
			t.Error("want LanguageFieldOK=false (ru != kk, no alias applies)")
		}
		if got.LanguageAliasUsed {
			t.Error("want LanguageAliasUsed=false — no alias was involved in this failure")
		}
	})
}

// TestJudgeOne_InlineListMarkersDoNotCountAsInventedDigits is the regression test for
// the real minimax-m2.5 case-11 failure: "Для оформления заказа: 1) укажите адрес
// доставки; 2) подтвердите заказ" — numbered-list markers after ":"/";" on ONE
// continuous line, which the line-start-only listMarkerRE didn't reach.
func TestJudgeOne_InlineListMarkersDoNotCountAsInventedDigits(t *testing.T) {
	catalog := &Catalog{}

	t.Run("real case-11 string passes", func(t *testing.T) {
		row := PromptfooRow{}
		row.Provider.ID = "test-model"
		// No literal price digits here on purpose — this test isolates the list-marker
		// fix; a literal, non-tokenized price number would ALSO be a genuine invented
		// digit, unrelated to what's being tested (covered by other existing tests).
		row.Response.Output = `{"reply_text":"Здравствуйте! Отлично, рады, что решили взять кофемашину DeLonghi — она в наличии. Для оформления заказа: 1) укажите адрес доставки; 2) подтвердите заказ — мы пришлём счёт.","reply_language":"ru","media_files_to_send":[],"escalate":false,"escalation_reason":"","confidence":0.9}`
		got := judgeOne(TestCase{ID: "t"}, row, map[string]string{}, map[string]bool{}, catalog.TrustedDigits)
		if len(got.InventedDigits) != 0 {
			t.Errorf("want no invented digits (list markers after ':'/';' are legitimate), got %v", got.InventedDigits)
		}
	})

	t.Run("inline digit+paren NOT preceded by a delimiter still flags", func(t *testing.T) {
		row := PromptfooRow{}
		row.Provider.ID = "test-model"
		row.Response.Output = `{"reply_text":"Гарантийный талон код 7)х указан на коробке.","reply_language":"ru","media_files_to_send":[],"escalate":false,"escalation_reason":"","confidence":0.9}`
		got := judgeOne(TestCase{ID: "t"}, row, map[string]string{}, map[string]bool{}, catalog.TrustedDigits)
		if len(got.InventedDigits) == 0 {
			t.Error("want the bare digit+')' in running prose (no preceding ':'/';') to still be flagged as a possible invented number")
		}
	})
}

// TestJudgeOne_ControlCharsFailsContract is the regression test for the real
// minimax-m2.5 case-12 output containing a literal backspace character
// ("...сейчас.\x08r\n\nУход за ней...") — a garbled byte a real product must never
// forward to a customer, independent of whether the JSON otherwise parses cleanly.
func TestJudgeOne_ControlCharsFailsContract(t *testing.T) {
	catalog := &Catalog{}

	t.Run("backspace fails contract", func(t *testing.T) {
		row := PromptfooRow{}
		row.Provider.ID = "test-model"
		// \\b here (not \b) is deliberate: this is the WIRE JSON text, and JSON requires
		// control characters to be ESCAPED inside a string (a raw unescaped 0x08 byte is
		// invalid JSON and would fail to parse at all, never reaching the control-char
		// check). "\\b" in this Go source produces the two literal characters backslash+b
		// in the string, i.e. JSON's own \b escape — decodes to rune 0x08 once
		// json.Unmarshal parses it, exactly like the real minimax-m2.5 response did.
		row.Response.Output = "{\"reply_text\":\"Всё готово.\\bУход простой.\",\"reply_language\":\"ru\",\"media_files_to_send\":[],\"escalate\":false,\"escalation_reason\":\"\",\"confidence\":0.9}"
		got := judgeOne(TestCase{ID: "t"}, row, map[string]string{}, map[string]bool{}, catalog.TrustedDigits)
		if !got.ControlChars {
			t.Error("want ControlChars=true")
		}
		if got.ContractPass {
			t.Error("want ContractPass=false — a control character must hard-fail the contract like ReasoningLeak/Truncated do")
		}
	})

	t.Run("normal CRLF is not flagged", func(t *testing.T) {
		row := PromptfooRow{}
		row.Provider.ID = "test-model"
		row.Response.Output = "{\"reply_text\":\"Строка один.\r\nСтрока два.\",\"reply_language\":\"ru\",\"media_files_to_send\":[],\"escalate\":false,\"escalation_reason\":\"\",\"confidence\":0.9}"
		got := judgeOne(TestCase{ID: "t"}, row, map[string]string{}, map[string]bool{}, catalog.TrustedDigits)
		if got.ControlChars {
			t.Error("want ControlChars=false — \\r\\n is legitimate formatting, not a garbled byte")
		}
	})
}

// TestJudgeOne_MustContainAny covers the positive-evidence check added for test 4's
// redesign (an availability yes/no question): at least one expected phrase must be
// present in the injected text, and — the reason this exists alongside
// must_not_contain rather than instead of it — a reply with the WRONG polarity phrase
// must still fail even though it says nothing forbidden by an empty/absent blocklist.
func TestJudgeOne_MustContainAny(t *testing.T) {
	catalog := &Catalog{}

	t.Run("expected phrase present -> pass", func(t *testing.T) {
		row := PromptfooRow{}
		row.Provider.ID = "test-model"
		row.Response.Output = `{"reply_text":"Да, кофемашина в наличии.","reply_language":"ru","media_files_to_send":[],"escalate":false,"escalation_reason":"","confidence":0.9}`
		tc := TestCase{ID: "t", MustContainAny: []string{"в наличии"}}
		got := judgeOne(tc, row, map[string]string{}, map[string]bool{}, catalog.TrustedDigits)
		if !got.MustContainAnyPass {
			t.Error("want MustContainAnyPass=true")
		}
		if !got.ModelBehaviorPass {
			t.Errorf("want ModelBehaviorPass=true, got reason=%q", got.Reason)
		}
	})

	t.Run("none of the expected phrases present -> fail, names the full expected set", func(t *testing.T) {
		row := PromptfooRow{}
		row.Provider.ID = "test-model"
		row.Response.Output = `{"reply_text":"Уточню у коллеги и вернусь с ответом.","reply_language":"ru","media_files_to_send":[],"escalate":true,"escalation_reason":"","confidence":0.9}`
		tc := TestCase{ID: "t", MustContainAny: []string{"в наличии"}}
		got := judgeOne(tc, row, map[string]string{}, map[string]bool{}, catalog.TrustedDigits)
		if got.MustContainAnyPass {
			t.Error("want MustContainAnyPass=false")
		}
		if got.ModelBehaviorPass {
			t.Error("want ModelBehaviorPass=false")
		}
		if !strings.Contains(got.Reason, "в наличии") {
			t.Errorf("want Reason to name the expected phrase(s), got %q", got.Reason)
		}
	})

	t.Run("no expectation declared -> vacuously true, same convention as must_not_contain", func(t *testing.T) {
		row := PromptfooRow{}
		row.Provider.ID = "test-model"
		row.Response.Output = `{"reply_text":"anything","reply_language":"ru","media_files_to_send":[],"escalate":false,"escalation_reason":"","confidence":0.9}`
		got := judgeOne(TestCase{ID: "t"}, row, map[string]string{}, map[string]bool{}, catalog.TrustedDigits)
		if !got.MustContainAnyPass {
			t.Error("want MustContainAnyPass=true when the test declares no must_contain_any at all")
		}
	})
}

// TestJudgeOne_Outcomes_ForbidTokens proves OutcomeCase.ForbidTokens gates a block the
// same way TestCase.ForbidTokens gates the top level — shaped after the real "generic
// delivery-cost question, zones present" case: a clarifying question (no zone token at
// all) and a per-zone breakdown (a zone token, but never the wrong one) are both
// acceptable; a reply that names a zone's price while pretending to still ask which city
// is neither.
func TestJudgeOne_Outcomes_ForbidTokens(t *testing.T) {
	escalateFalse := false
	catalog := &Catalog{}
	tokenValue := map[string]string{
		"{{delivery.astana.delivery_cost}}":     "4 000 ₸",
		"{{delivery.kazakhstan.delivery_cost}}": "10 000 ₸",
	}
	tc := TestCase{
		ID: "t",
		Outcomes: []OutcomeCase{
			{
				Label:        "asks which city, without citing any zone's price",
				Escalate:     &escalateFalse,
				ForbidTokens: []string{"delivery."},
			},
			{
				Label:    "answers with a zone's price directly",
				Requires: [][]string{{"delivery.astana.delivery_cost", "delivery.kazakhstan.delivery_cost"}},
				Escalate: &escalateFalse,
			},
		},
	}
	run := func(output string) Verdict {
		row := PromptfooRow{}
		row.Provider.ID = "test-model"
		row.Response.Output = output
		return judgeOne(tc, row, tokenValue, map[string]bool{}, catalog.TrustedDigits)
	}

	t.Run("clarifying question with no zone token satisfies the first block", func(t *testing.T) {
		got := run(`{"reply_text":"Подскажите, в какой город нужна доставка?","reply_language":"ru","media_files_to_send":[],"escalate":false,"escalation_reason":"","confidence":0.9}`)
		if !got.OutcomesPass || got.OutcomeMatched != tc.Outcomes[0].Label {
			t.Fatalf("want the clarifying block to match, got pass=%v matched=%q reason=%q", got.OutcomesPass, got.OutcomeMatched, got.Reason)
		}
	})

	t.Run("a zone price satisfies the second block", func(t *testing.T) {
		got := run(`{"reply_text":"По Казахстану {{delivery.kazakhstan.delivery_cost}}.","reply_language":"ru","media_files_to_send":[],"escalate":false,"escalation_reason":"","confidence":0.9}`)
		if !got.OutcomesPass || got.OutcomeMatched != tc.Outcomes[1].Label {
			t.Fatalf("want the price block to match, got pass=%v matched=%q reason=%q", got.OutcomesPass, got.OutcomeMatched, got.Reason)
		}
	})

	t.Run("citing a zone price while also escalating satisfies neither block", func(t *testing.T) {
		// Block 1 (forbid_tokens) rejects this for citing delivery.kazakhstan.delivery_cost;
		// block 2 (Requires the same token) rejects it separately for escalate=true — a
		// reply that fails each block for a DIFFERENT declared reason, not the ambiguous
		// "answers plus asks" shape (which is a legitimate match for block 2, by design —
		// see kb-delivery-ru.yaml's outcome comment for that call).
		got := run(`{"reply_text":"По Казахстану {{delivery.kazakhstan.delivery_cost}}.","reply_language":"ru","media_files_to_send":[],"escalate":true,"escalation_reason":"уточню детали","confidence":0.9}`)
		if got.OutcomesPass {
			t.Fatalf("want neither block to match, got pass=%v matched=%q", got.OutcomesPass, got.OutcomeMatched)
		}
		if got.ModelBehaviorPass {
			t.Error("want ModelBehaviorPass=false when no outcome block is satisfied")
		}
	})
}

// TestJudgeOne_Outcomes proves the alternative-outcome gate (TestCase.Outcomes): an OR
// over the declared blocks, AND-ed into ModelBehaviorPass, each block evaluated with the
// SAME helpers as the top-level checks. Shaped after the motivating xph2 case — an
// ambiguous pronoun where "answer for the last-named tariff" and "ask which tariff is
// meant" are both defensible, while a confident answer for the WRONG tariff (satisfying
// neither block) is exactly as wrong as it was before this knob existed.
func TestJudgeOne_Outcomes(t *testing.T) {
	escalateFalse := false
	catalog := &Catalog{}
	tokenValue := map[string]string{
		"{{tariff.business.payment_limit_monthly}}": "10 000 000 ₸",
		"{{tariff.start.payment_limit_monthly}}":    "1 000 000 ₸",
	}
	tc := TestCase{
		ID: "t",
		Outcomes: []OutcomeCase{
			{
				Label:    "states the Business tariff's monthly limit via its token",
				Requires: [][]string{{"tariff.business.payment_limit_monthly"}},
				Escalate: &escalateFalse,
			},
			{
				Label:          "asks which tariff the customer means, without inventing an answer",
				Escalate:       &escalateFalse,
				MustContainAny: []string{"какого тарифа", "какой тариф", "уточните"},
			},
		},
	}
	run := func(output string) Verdict {
		row := PromptfooRow{}
		row.Provider.ID = "test-model"
		row.Response.Output = output
		return judgeOne(tc, row, tokenValue, map[string]bool{}, catalog.TrustedDigits)
	}

	t.Run("first block satisfied -> pass, matched label is the first block's", func(t *testing.T) {
		got := run(`{"reply_text":"Лимит по тарифу «Бизнес» — {{tariff.business.payment_limit_monthly}}.","reply_language":"ru","media_files_to_send":[],"escalate":false,"escalation_reason":"","confidence":0.9}`)
		if !got.OutcomesDeclared || !got.OutcomesPass {
			t.Fatalf("want OutcomesDeclared+OutcomesPass, got declared=%v pass=%v reason=%q", got.OutcomesDeclared, got.OutcomesPass, got.Reason)
		}
		if got.OutcomeMatched != tc.Outcomes[0].Label {
			t.Errorf("want OutcomeMatched=%q, got %q", tc.Outcomes[0].Label, got.OutcomeMatched)
		}
		if !got.ModelBehaviorPass {
			t.Errorf("want ModelBehaviorPass=true, got reason=%q", got.Reason)
		}
	})

	t.Run("second block satisfied (clarifying question) -> pass via the alternative", func(t *testing.T) {
		got := run(`{"reply_text":"Уточните, пожалуйста, для какого тарифа вас интересует лимит?","reply_language":"ru","media_files_to_send":[],"escalate":false,"escalation_reason":"","confidence":0.9}`)
		if !got.OutcomesPass {
			t.Fatalf("want OutcomesPass=true via the clarify block, got reason=%q", got.Reason)
		}
		if got.OutcomeMatched != tc.Outcomes[1].Label {
			t.Errorf("want OutcomeMatched=%q (first PASSING block, declaration order), got %q", tc.Outcomes[1].Label, got.OutcomeMatched)
		}
	})

	t.Run("neither block satisfied (confident wrong-tariff answer) -> fail, reason names every alternative", func(t *testing.T) {
		got := run(`{"reply_text":"Лимит платежей — {{tariff.start.payment_limit_monthly}} в месяц.","reply_language":"ru","media_files_to_send":[],"escalate":false,"escalation_reason":"","confidence":0.9}`)
		if got.OutcomesPass {
			t.Fatal("want OutcomesPass=false — the wrong tariff's token satisfies neither block")
		}
		if got.OutcomeMatched != "" {
			t.Errorf("want OutcomeMatched empty on a miss, got %q", got.OutcomeMatched)
		}
		if got.ModelBehaviorPass {
			t.Error("want ModelBehaviorPass=false")
		}
		wantReason := "none of the acceptable outcomes was satisfied: " + tc.Outcomes[0].Label + " | " + tc.Outcomes[1].Label
		if got.Reason != wantReason {
			t.Errorf("want Reason=%q, got %q", wantReason, got.Reason)
		}
	})

	t.Run("universal checks stay AND-ed alongside a passing outcome", func(t *testing.T) {
		// Same passing clarify reply, but the test ALSO declares a top-level language
		// expectation the reply violates — a matched outcome must not paper over it.
		kkTC := tc
		kkTC.Language = "kk"
		row := PromptfooRow{}
		row.Provider.ID = "test-model"
		row.Response.Output = `{"reply_text":"Уточните, пожалуйста, для какого тарифа вас интересует лимит?","reply_language":"ru","media_files_to_send":[],"escalate":false,"escalation_reason":"","confidence":0.9}`
		got := judgeOne(kkTC, row, tokenValue, map[string]bool{}, catalog.TrustedDigits)
		if !got.OutcomesPass {
			t.Fatalf("want the outcome block itself to still pass, got reason=%q", got.Reason)
		}
		if got.ModelBehaviorPass {
			t.Error("want ModelBehaviorPass=false — the universal language check failed and outcomes must not override it")
		}
	})

	t.Run("no outcomes declared -> vacuously true, not marked declared", func(t *testing.T) {
		row := PromptfooRow{}
		row.Provider.ID = "test-model"
		row.Response.Output = `{"reply_text":"anything","reply_language":"ru","media_files_to_send":[],"escalate":false,"escalation_reason":"","confidence":0.9}`
		got := judgeOne(TestCase{ID: "t"}, row, map[string]string{}, map[string]bool{}, catalog.TrustedDigits)
		if got.OutcomesDeclared {
			t.Error("want OutcomesDeclared=false when the test declares no outcomes")
		}
		if !got.OutcomesPass {
			t.Error("want OutcomesPass vacuously true, same convention as MustNotContainPass")
		}
	})
}

// TestJudgeOne_ExtractsFinalAnswerFromCombinedReasoning proves judgeOne recovers the
// final customer-response JSON when a provider returns reasoning text combined with
// the answer in one string — the exact case that made gemini-3.5-flash/kimi-k2.6
// unparseable before extraction existed. FinalOutput/NonFinalOutput/ExtractionMethod
// are populated and RawOutput stays the untouched original.
func TestJudgeOne_ExtractsFinalAnswerFromCombinedReasoning(t *testing.T) {
	final := `{"reply_text":"Кофемашина в наличии.","reply_language":"ru","media_files_to_send":[],"escalate":false,"escalation_reason":"","confidence":0.9}`
	reasoning := "Thinking: I need to check the knowledge base for the coffee machine's stock status before replying to the customer."
	raw := reasoning + "\n\n" + final

	row := PromptfooRow{}
	row.Provider.ID = "test-model"
	row.Response.Output = raw

	got := judgeOne(TestCase{ID: "t"}, row, map[string]string{}, map[string]bool{}, nil)

	if !got.ParseOK {
		t.Fatalf("ParseOK = false, want true; reason=%s", got.Reason)
	}
	if got.RawOutput != raw {
		t.Errorf("RawOutput = %q, want the untouched original %q", got.RawOutput, raw)
	}
	if got.FinalOutput != final {
		t.Errorf("FinalOutput = %q, want %q", got.FinalOutput, final)
	}
	if !strings.Contains(got.NonFinalOutput, "Thinking:") {
		t.Errorf("NonFinalOutput = %q, want it to retain the reasoning prose", got.NonFinalOutput)
	}
	if got.ExtractionMethod == "" {
		t.Error("ExtractionMethod must be recorded when extraction ran")
	}
}

// TestJudgeOne_TruncatedReasoningWithNoFinalAnswerStaysAFailure: reasoning consumed
// the whole completion budget and the model never produced a complete final JSON
// object — this must remain a parse failure, never repaired/reconstructed.
func TestJudgeOne_TruncatedReasoningWithNoFinalAnswerStaysAFailure(t *testing.T) {
	row := PromptfooRow{}
	row.Provider.ID = "test-model"
	row.Response.Output = `Thinking: let me carefully check the price of the coffee machine in the knowledge base before I answer, considering all the delivery`
	row.Response.FinishReason = "length"

	got := judgeOne(TestCase{ID: "t"}, row, map[string]string{}, map[string]bool{}, nil)
	if got.ParseOK {
		t.Fatal("ParseOK = true, want false — no complete final answer existed")
	}
	if got.ContractPass {
		t.Fatal("ContractPass = true, want false")
	}
	if got.FinalOutput != "" {
		t.Errorf("FinalOutput = %q, want empty on a failed extraction", got.FinalOutput)
	}
}

// TestJudgeOne_RawOutputNeverRewrittenByJudging: RawOutput must always equal exactly
// what the provider returned, regardless of how judging extracted/scored it — the
// immutable audit record every re-judge relies on.
func TestJudgeOne_RawOutputNeverRewrittenByJudging(t *testing.T) {
	tests := []string{
		`{"reply_text":"ok","reply_language":"ru","media_files_to_send":[],"escalate":false,"escalation_reason":"","confidence":0.9}`,
		"Thinking: ...\n\n" + `{"reply_text":"ok","reply_language":"ru","media_files_to_send":[],"escalate":false}`,
		"not json at all, unparseable",
		"",
	}
	for _, raw := range tests {
		row := PromptfooRow{}
		row.Provider.ID = "test-model"
		row.Response.Output = raw
		got := judgeOne(TestCase{ID: "t"}, row, map[string]string{}, map[string]bool{}, nil)
		if got.RawOutput != raw {
			t.Errorf("RawOutput = %q, want untouched original %q", got.RawOutput, raw)
		}
	}
}
