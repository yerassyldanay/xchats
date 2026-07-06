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
		MediaGroups: []string{"product.coffee-machine.images"},
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
