package aiprompt

import (
	"strings"
	"testing"
)

// TestBuildCatalog_DeliveryZonesEmpty_UnaffectedByInvariants documents the
// "feature unused" default path: baseKB() has zero delivery zones, a flat
// ai_policies.delivery_cost/delivery_in_days, and a blank OutsideZonesNote —
// none of the zones-specific invariants apply, and delivery questions answer
// from the flat policy facts exactly as before this feature existed.
func TestBuildCatalog_DeliveryZonesEmpty_UnaffectedByInvariants(t *testing.T) {
	cat, err := BuildCatalog(baseKB())
	if err != nil {
		t.Fatalf("BuildCatalog: unexpected error with zero delivery zones: %v", err)
	}
	if f := cat.FactByToken("{{policy.main.delivery_cost}}"); f == nil {
		t.Error("expected the flat delivery_cost fact when ai_delivery_zones is empty")
	}
	if f := cat.FactByToken("{{policy.main.outside_zones_note}}"); f != nil {
		t.Error("blank outside_zones_note must not produce a fact entry")
	}
}

// TestBuildCatalog_DeliveryZoneFacts covers the shape of facts produced for
// an available zone (cost + days + availability) versus a deny zone
// (availability only — a blank cost/days never becomes a fact, matching
// addFact's general blank-value rule).
func TestBuildCatalog_DeliveryZoneFacts(t *testing.T) {
	cat, err := BuildCatalog(zonesKB())
	if err != nil {
		t.Fatalf("BuildCatalog: %v", err)
	}

	cost := cat.FactByToken("{{delivery.astana.delivery_cost}}")
	if cost == nil {
		t.Fatal("expected a delivery_cost fact for astana")
	}
	if cost.Kind != KindMoneyDisplay {
		t.Errorf("astana delivery_cost Kind = %v, want KindMoneyDisplay", cost.Kind)
	}
	if cost.ReasoningState != "" {
		t.Errorf("astana delivery_cost ReasoningState = %q, want empty (only the flag carries a state)", cost.ReasoningState)
	}

	days := cat.FactByToken("{{delivery.astana.delivery_in_days}}")
	if days == nil {
		t.Fatal("expected a delivery_in_days fact for astana")
	}

	available := cat.FactByToken("{{delivery.astana.delivery_available}}")
	if available == nil {
		t.Fatal("expected a delivery_available fact for astana")
	}
	if available.Kind != KindDeliveryFlag {
		t.Errorf("astana delivery_available Kind = %v, want KindDeliveryFlag", available.Kind)
	}
	if available.ReasoningState != "delivers" {
		t.Errorf("astana delivery_available ReasoningState = %q, want delivers", available.ReasoningState)
	}

	// Country-level fallback row produces its own facts too.
	if cat.FactByToken("{{delivery.kazakhstan.delivery_cost}}") == nil {
		t.Error("expected a delivery_cost fact for the kazakhstan fallback zone")
	}

	// The deny row produces ONLY the availability fact — cost/days are blank
	// by construction and addFact skips blank values, exactly like a product
	// with a blank price produces no price fact.
	denyAvailable := cat.FactByToken("{{delivery.baikonur.delivery_available}}")
	if denyAvailable == nil {
		t.Fatal("expected a delivery_available fact for baikonur")
	}
	if denyAvailable.ReasoningState != "no_delivery" {
		t.Errorf("baikonur delivery_available ReasoningState = %q, want no_delivery", denyAvailable.ReasoningState)
	}
	if cat.FactByToken("{{delivery.baikonur.delivery_cost}}") != nil {
		t.Error("a deny zone must not produce a delivery_cost fact")
	}
	if cat.FactByToken("{{delivery.baikonur.delivery_in_days}}") != nil {
		t.Error("a deny zone must not produce a delivery_in_days fact")
	}
}

// TestBuildCatalog_DeliveryZoneFailClosed covers every zones-specific
// fail-closed rule: a violation must fail the entire BuildCatalog call,
// mirroring TestBuildCatalog_MaterialFailClosed's discipline for materials.
func TestBuildCatalog_DeliveryZoneFailClosed(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*KB)
	}{
		{
			name: "duplicate zone ref",
			mutate: func(kb *KB) {
				kb.DeliveryZones = append(kb.DeliveryZones, DeliveryZone{
					Ref: "astana", Name: "Дубликат", ZoneLevel: "city",
					DeliveryAvailable: true, DeliveryCost: "1 ₸", DeliveryInDays: "1",
					SalesStatus: "active",
				})
			},
		},
		{
			name:   "invalid zone_level",
			mutate: func(kb *KB) { kb.DeliveryZones[0].ZoneLevel = "planet" },
		},
		{
			name: "invalid ref characters",
			mutate: func(kb *KB) {
				kb.DeliveryZones[1].Ref = "Astana City!"
			},
		},
		{
			name:   "available zone missing cost",
			mutate: func(kb *KB) { kb.DeliveryZones[1].DeliveryCost = "" },
		},
		{
			name:   "available zone missing days",
			mutate: func(kb *KB) { kb.DeliveryZones[1].DeliveryInDays = "" },
		},
		{
			name:   "unavailable zone has cost set",
			mutate: func(kb *KB) { kb.DeliveryZones[2].DeliveryCost = "1 000 ₸" },
		},
		{
			name:   "unavailable zone has days set",
			mutate: func(kb *KB) { kb.DeliveryZones[2].DeliveryInDays = "1" },
		},
		{
			name:   "unknown parent_ref",
			mutate: func(kb *KB) { kb.DeliveryZones[1].ParentRef = "narnia" },
		},
		{
			name: "cyclic parent_ref (two nodes)",
			mutate: func(kb *KB) {
				// kazakhstan (index 0) currently has no parent; astana (index 1)
				// points at kazakhstan. Point kazakhstan back at astana.
				kb.DeliveryZones[0].ParentRef = "astana"
			},
		},
		{
			name:   "self-referential parent_ref",
			mutate: func(kb *KB) { kb.DeliveryZones[0].ParentRef = "kazakhstan" },
		},
		{
			name: "zones present with flat delivery_cost set",
			mutate: func(kb *KB) {
				kb.Policies.DeliveryCost = "1 500 ₸"
			},
		},
		{
			name: "zones present with flat delivery_in_days set",
			mutate: func(kb *KB) {
				kb.Policies.DeliveryInDays = "1–3"
			},
		},
		{
			name:   "zones present with blank outside_zones_note",
			mutate: func(kb *KB) { kb.Policies.OutsideZonesNote = "" },
		},
		{
			name:   "zones present with nil Policies",
			mutate: func(kb *KB) { kb.Policies = nil },
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			kb := zonesKB()
			tc.mutate(kb)
			if _, err := BuildCatalog(kb); err == nil {
				t.Fatalf("BuildCatalog: expected an error for %q, got none", tc.name)
			}
		})
	}
}

// TestBuildCatalog_DeliveryZoneInactiveExcluded confirms an inactive zone
// produces no facts of its own, while remaining a structurally valid
// parent_ref target for its still-active children (inactive is "temporarily
// hidden from customers", not "deleted from the hierarchy").
func TestBuildCatalog_DeliveryZoneInactiveExcluded(t *testing.T) {
	kb := zonesKB()
	kb.DeliveryZones[1].SalesStatus = "inactive" // astana
	cat, err := BuildCatalog(kb)
	if err != nil {
		t.Fatalf("BuildCatalog: %v", err)
	}
	if cat.FactByToken("{{delivery.astana.delivery_cost}}") != nil {
		t.Error("inactive zone must not produce fact entries")
	}
	if cat.FactByToken("{{delivery.kazakhstan.delivery_cost}}") == nil {
		t.Error("sibling zones must be unaffected by an inactive zone")
	}
	if cat.FactByToken("{{delivery.baikonur.delivery_available}}") == nil {
		t.Error("sibling zones must be unaffected by an inactive zone")
	}
}

// TestBuildCatalog_DeliveryZoneInactiveParentStillValidatesStructure: an
// inactive zone must still satisfy parent_ref existence for its active
// children — deactivating a country row must not suddenly make every city
// beneath it fail to build.
func TestBuildCatalog_DeliveryZoneInactiveParentStillValidatesStructure(t *testing.T) {
	kb := zonesKB()
	kb.DeliveryZones[0].SalesStatus = "inactive" // kazakhstan, parent of astana/baikonur
	cat, err := BuildCatalog(kb)
	if err != nil {
		t.Fatalf("BuildCatalog: unexpected error when only a parent zone is inactive: %v", err)
	}
	if cat.FactByToken("{{delivery.kazakhstan.delivery_cost}}") != nil {
		t.Error("inactive parent zone must not itself produce fact entries")
	}
	if cat.FactByToken("{{delivery.astana.delivery_cost}}") == nil {
		t.Error("an active child zone must still build even when its parent is inactive")
	}
}

// TestResolveFact_DeliveryZone exercises the current-value resolution path
// for all three delivery fact columns, including the code-owned wording pair
// for delivery_available.
func TestResolveFact_DeliveryZone(t *testing.T) {
	kb := zonesKB()
	cat, err := BuildCatalog(kb)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		token string
		want  string
	}{
		{"{{delivery.astana.delivery_cost}}", "4 000 ₸"},
		{"{{delivery.astana.delivery_in_days}}", "1"},
		{"{{delivery.astana.delivery_available}}", "доставляем"},
		{"{{delivery.baikonur.delivery_available}}", "не доставляем"},
		{"{{delivery.kazakhstan.delivery_cost}}", "10 000 ₸"},
		{"{{policy.main.outside_zones_note}}", "В другие страны и регионы не доставляем."},
	}
	for _, tt := range tests {
		got, err := ResolveFact(tt.token, kb, cat)
		if err != nil {
			t.Errorf("ResolveFact(%s): unexpected error: %v", tt.token, err)
			continue
		}
		if got != tt.want {
			t.Errorf("ResolveFact(%s) = %q, want %q", tt.token, got, tt.want)
		}
	}
}

// TestResolveFact_DeliveryZoneStaleAfterDeactivation: a token that was valid
// when the catalog was built must fail to resolve once its zone becomes
// inactive before delivery — re-validation, not a rubber stamp, matching the
// product/tariff stale-placeholder discipline.
func TestResolveFact_DeliveryZoneStaleAfterDeactivation(t *testing.T) {
	kb := zonesKB()
	cat, err := BuildCatalog(kb)
	if err != nil {
		t.Fatal(err)
	}
	kb.DeliveryZones[1].SalesStatus = "inactive" // astana
	if _, err := ResolveFact("{{delivery.astana.delivery_cost}}", kb, cat); err == nil {
		t.Fatal("expected ResolveFact to fail-closed on a since-deactivated zone")
	}
}

func TestSubstituteFacts_DeliveryZoneWording(t *testing.T) {
	kb := zonesKB()
	cat, err := BuildCatalog(kb)
	if err != nil {
		t.Fatal(err)
	}
	out, err := SubstituteFacts("Доставка: {{delivery.astana.delivery_available}}, {{delivery.astana.delivery_cost}}.", kb, cat)
	if err != nil {
		t.Fatal(err)
	}
	if out != "Доставка: доставляем, 4 000 ₸." {
		t.Errorf("SubstituteFacts() = %q", out)
	}

	out2, err := SubstituteFacts("Доставка: {{delivery.baikonur.delivery_available}}.", kb, cat)
	if err != nil {
		t.Fatal(err)
	}
	if out2 != "Доставка: не доставляем." {
		t.Errorf("SubstituteFacts() = %q", out2)
	}
}

// TestValidateResponse_Valid_DeliveryZoneTokens confirms a reply using ONLY
// tokens for delivery availability and values — never the code-owned wording
// as connective prose — produces zero issues.
func TestValidateResponse_Valid_DeliveryZoneTokens(t *testing.T) {
	kb := zonesKB()
	cat, err := BuildCatalog(kb)
	if err != nil {
		t.Fatal(err)
	}
	text := "Астана: {{delivery.astana.delivery_available}}, {{delivery.astana.delivery_cost}}, " +
		"{{delivery.astana.delivery_in_days}} дня."
	_, issues := ValidateResponse(responseWithText(t, text), kb, cat)
	if len(issues) != 0 {
		t.Fatalf("unexpected issues for a token-only delivery reply: %+v", issues)
	}
}

// TestValidateResponse_RejectsModelAuthoredDeliveryWording: the model must
// never phrase delivery availability or a zone's exact value itself — only
// copy the token — exactly the same fail-closed rule already proven for
// prices and stock wording, applied to the new delivery vocabulary. This is
// only checkable when the wording/value in question UNIQUELY identifies one
// zone's fact: zonesKB's kazakhstan and astana zones both have
// delivery_available=true, so their shared "доставляем" wording cannot be
// attributed to either one specifically (same ambiguous-collision rule as
// TestValidateResponse_SharedWordingNeverFlaggedAsLiteralLeak, and the exact
// postmortem #7 failure mode) and must NOT be flagged; baikonur is the only
// no_delivery zone and astana's own price is unique, so those two stay
// checkable and must still be flagged.
func TestValidateResponse_RejectsModelAuthoredDeliveryWording(t *testing.T) {
	kb := zonesKB()
	cat, err := BuildCatalog(kb)
	if err != nil {
		t.Fatal(err)
	}

	_, issues := ValidateResponse(responseWithText(t, "Да, доставляем в Астану."), kb, cat)
	if containsCode(issues, "exact_value_literal") {
		t.Errorf("wording shared by two delivers=true zones (kazakhstan, astana) must not be flagged, got issues = %+v", issues)
	}

	for _, text := range []string{
		"К сожалению, в Байконур не доставляем.",
		"Стоимость доставки в Астану — 4 000 ₸.",
	} {
		_, issues := ValidateResponse(responseWithText(t, text), kb, cat)
		if !containsCode(issues, "exact_value_literal") {
			t.Errorf("text %q: issues = %+v, want exact_value_literal (uniquely identifies one zone's fact)", text, issues)
		}
	}
}

// TestRenderFacts_DeliveryZoneLabelIncludesRef confirms delivery facts join
// the product/tariff convention of prefixing the label with the natural ref
// (so multiple zones' facts are distinguishable in the FACTS block).
func TestRenderFacts_DeliveryZoneLabelIncludesRef(t *testing.T) {
	cat, err := BuildCatalog(zonesKB())
	if err != nil {
		t.Fatal(err)
	}
	rendered := renderFacts(cat.Facts)
	if !strings.Contains(rendered, "astana — стоимость доставки") {
		t.Errorf("expected a ref-prefixed label for astana's delivery_cost fact, got:\n%s", rendered)
	}
	if !strings.Contains(rendered, "baikonur — доступность доставки") {
		t.Errorf("expected a ref-prefixed label for baikonur's delivery_available fact, got:\n%s", rendered)
	}
	if !strings.Contains(rendered, "no_delivery") {
		t.Error("expected baikonur's reasoning state 'no_delivery' to appear in the FACTS render")
	}
	if !strings.Contains(rendered, "delivers") {
		t.Error("expected astana's reasoning state 'delivers' to appear in the FACTS render")
	}
}

// TestRenderDescriptions_DeliveryZoneNotes: only zones with non-blank Notes
// produce a DESCRIPTIONS line, matching every other entity's blank-prose-
// omitted rule.
func TestRenderDescriptions_DeliveryZoneNotes(t *testing.T) {
	kb := zonesKB()
	desc := renderDescriptions(kb.PromptInput())
	if !strings.Contains(desc, "Доставка — Байконур: Закрытый административный статус") {
		t.Errorf("expected baikonur's non-blank Notes to appear in DESCRIPTIONS, got:\n%s", desc)
	}
	if strings.Contains(desc, "Доставка — Астана:") {
		t.Error("astana has no Notes in the fixture; it must not produce a DESCRIPTIONS line")
	}
}

// TestBuildPrompt_DeliveryZones_EndToEnd exercises the full render path with
// zones present: facts reach the prompt, the closed-world refusal token is
// present, and every existing leak/slot gate still passes.
func TestBuildPrompt_DeliveryZones_EndToEnd(t *testing.T) {
	prompt, cat, err := BuildPrompt(basicFrame, zonesKB())
	if err != nil {
		t.Fatalf("BuildPrompt: %v", err)
	}
	if cat.FactByToken("{{delivery.astana.delivery_cost}}") == nil {
		t.Error("expected the astana delivery_cost fact in the built catalog")
	}
	if !strings.Contains(prompt, "{{delivery.astana.delivery_cost}}") {
		t.Error("expected the astana delivery_cost token to appear in the rendered prompt")
	}
	if !strings.Contains(prompt, "{{policy.main.outside_zones_note}}") {
		t.Error("expected the outside_zones_note token to appear in the rendered prompt")
	}
	if !strings.Contains(prompt, "Байконур") {
		t.Error("expected baikonur's zone name to appear via DESCRIPTIONS")
	}
	if strings.Contains(prompt, "%%") {
		t.Errorf("prompt still contains an unfilled slot marker:\n%s", prompt)
	}
}
