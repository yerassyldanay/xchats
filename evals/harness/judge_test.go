package main

import "testing"

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
