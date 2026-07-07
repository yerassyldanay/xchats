package main

import "testing"

func TestValidateCatalog(t *testing.T) {
	if err := validateCatalog(&Catalog{Tokens: []CatalogFact{{Token: "{{a.b.c}}", Value: "129 900 ₸"}}}); err != nil {
		t.Fatalf("brace-free value should pass, got: %v", err)
	}
	err := validateCatalog(&Catalog{Tokens: []CatalogFact{{Token: "{{a.b.c}}", Value: "129{900 ₸"}}})
	if err == nil {
		t.Fatal("value containing a brace character should fail validateCatalog")
	}
}

func TestBuildCatalog_TrustedDigits(t *testing.T) {
	data := &Data{
		FactTables: []FactTable{
			{
				Table: "product",
				Fields: []FieldSpec{
					{Name: "price", ValueKind: "money_display"},
				},
				Rows: []FactRow{
					{
						Ref:         "kettle-tefal",
						DisplayName: "Чайник Tefal",
						Description: "электрочайник, 1.7 л, быстрое закипание.",
						Values:      map[string]string{"price": "12 900 ₸"},
					},
				},
			},
		},
	}
	cat := buildCatalog(data, "attach_groups")
	want := map[string]bool{"1": true, "7": true}
	got := map[string]bool{}
	for _, d := range cat.TrustedDigits {
		got[d] = true
	}
	for d := range want {
		if !got[d] {
			t.Fatalf("expected TrustedDigits to contain %q (from row Description), got %v", d, cat.TrustedDigits)
		}
	}
}

func TestValidatePrompt(t *testing.T) {
	cat := &Catalog{Tokens: []CatalogFact{{Token: "{{product.coffee-machine.price}}", Value: "129 900 ₸"}}}

	valid := "FACTS:\n{{product.coffee-machine.price}} | цена | 129 900 ₸\nКлиент пишет: {{message}}\nИстория: {{history}}\n"
	if err := validatePrompt(valid, cat); err != nil {
		t.Fatalf("prompt using only catalog tokens + promptfoo vars should pass, got: %v", err)
	}

	unknownToken := "Цена: {{product.unknown.field}}\nКлиент пишет: {{message}}\n"
	if err := validatePrompt(unknownToken, cat); err == nil {
		t.Fatal("prompt referencing a token not in the catalog should fail validatePrompt")
	}

	unfilledSlot := "%%FACTS%%\nКлиент пишет: {{message}}\n"
	if err := validatePrompt(unfilledSlot, cat); err == nil {
		t.Fatal("prompt with a leftover %%SLOT%% should fail validatePrompt")
	}
}
