package aiprompt

import (
	"encoding/json"
	"strings"
	"testing"
)

// virtualFactsKB extends baseKB with the full availability spectrum and a
// representative virtual fact of each scalar kind on a product, a tariff,
// and the tariff_info singleton — the shared fixture for this file's tests.
func virtualFactsKB() *KB {
	kb := baseKB()
	kb.Products[0].AdditionalFacts = []AdditionalFact{
		{Ref: "working_pressure", Value: json.Number("275"), Instruction: "Рабочее давление, бар. Формулируй нейтрально: «Рабочее давление, бар: …», без слова после числа."},
		{Ref: "has_wifi", Value: true, Instruction: "Поддерживает ли товар Wi-Fi."},
		{Ref: "model_code", Value: "DLM-500X", Instruction: "Точный код модели для гарантийных документов."},
	}
	kb.Products = append(kb.Products,
		Product{
			Ref: "smart-kettle", Name: "Умный чайник", Price: "19 900 ₸",
			AvailabilityStatus: "preorder", SalesStatus: "active",
			AvailabilityNote: "Отправляем партиями каждую неделю.",
		},
		Product{
			Ref: "custom-sofa", Name: "Диван на заказ", Price: "450 000 ₸",
			AvailabilityStatus: "on_demand", SalesStatus: "active",
		},
	)
	kb.Tariffs[0].AdditionalFacts = []AdditionalFact{
		{Ref: "limit_on_devices", Value: json.Number("5"), Instruction: "Максимальное количество устройств. Формулируй нейтрально: «Количество устройств: …», без склоняемой единицы после числа."},
	}
	kb.TariffInfo = &TariffInfo{
		AdditionalFacts: []AdditionalFact{
			{Ref: "trial_in_days", Value: json.Number("3"), Instruction: "Продолжительность общего пробного периода. Формулируй нейтрально: «Продолжительность пробного периода в днях: …»."},
		},
	}
	return kb
}

// --- Availability status coverage (required tests #2, #3, #4) -------------

func TestAvailability_AllFourStatusesBuildCatalogWithoutError(t *testing.T) {
	kb := virtualFactsKB()
	if _, err := BuildCatalog(kb); err != nil {
		t.Fatalf("BuildCatalog: %v", err)
	}
	statuses := map[string]bool{}
	for _, p := range kb.Products {
		statuses[p.AvailabilityStatus] = true
	}
	for _, want := range []string{"in_stock", "preorder", "on_demand", "unavailable"} {
		if !statuses[want] {
			t.Errorf("fixture is missing a product with availability_status %q", want)
		}
	}
}

// TestAvailability_InvalidStatusFailsBuildCatalogClosed guards the fail-closed
// contract productVisible's doc comment promises: an availability_status
// outside the four known values (reachable only via a manual DB edit or a
// future write path that skips enum validation, since the migration has no
// SQL CHECK) must fail the WHOLE BuildCatalog call, never default to fully
// visible.
func TestAvailability_InvalidStatusFailsBuildCatalogClosed(t *testing.T) {
	kb := virtualFactsKB()
	kb.Products[0].AvailabilityStatus = "discontinued" // not one of in_stock/preorder/on_demand/unavailable
	if _, err := BuildCatalog(kb); err == nil {
		t.Fatal("BuildCatalog must reject a product with an unrecognized availability_status, got nil error")
	}
}

func TestAvailability_PreorderAndOnDemandAreFullyVisible(t *testing.T) {
	kb := virtualFactsKB()
	cat, err := BuildCatalog(kb)
	if err != nil {
		t.Fatalf("BuildCatalog: %v", err)
	}
	prompt, err := RenderPrompt(FrameShopKBV6RU(), kb.PromptInput(), cat)
	if err != nil {
		t.Fatalf("RenderPrompt: %v", err)
	}
	for _, ref := range []string{"smart-kettle", "custom-sofa"} {
		if !strings.Contains(prompt, "product: "+ref) {
			t.Errorf("preorder/on_demand product %q must render its full block in PRODUCTS_AVAILABLE", ref)
		}
	}
	if !strings.Contains(prompt, "availability_status: предзаказ") {
		t.Error("expected the preorder product's availability_status to render as «предзаказ»")
	}
	if !strings.Contains(prompt, "availability_status: под заказ") {
		t.Error("expected the on_demand product's availability_status to render as «под заказ»")
	}
	if !strings.Contains(prompt, "Отправляем партиями каждую неделю.") {
		t.Error("expected the preorder product's availability_note prose to render")
	}
}

func TestAvailability_UnavailableProductCompletelySuppressed(t *testing.T) {
	kb := virtualFactsKB()
	// Give the unavailable product a virtual fact and media too, to prove
	// suppression is unconditional — not merely "no facts were staged".
	kb.Products[1].AdditionalFacts = []AdditionalFact{
		{Ref: "restock_hint", Value: "скоро", Instruction: "Подсказка о поступлении."},
	}
	kb.Products[1].FeaturedImage = "m-cm-featured"
	cat, err := BuildCatalog(kb)
	if err != nil {
		t.Fatalf("BuildCatalog: %v", err)
	}
	if f := cat.FactByToken("{{product.cookware-set.restock_hint}}"); f != nil {
		t.Error("unavailable product must emit no virtual fact token")
	}
	if m := cat.MediaByToken("products.cookware-set.featured_image"); m != nil {
		t.Error("unavailable product must emit no media token")
	}
	for _, a := range cat.Absent {
		if a.Ref == "cookware-set" {
			t.Error("unavailable product must not appear in Absent — it never gets a full block at all")
		}
	}

	prompt, err := RenderPrompt(FrameShopKBV6RU(), kb.PromptInput(), cat)
	if err != nil {
		t.Fatalf("RenderPrompt: %v", err)
	}
	if strings.Contains(prompt, "product: cookware-set") {
		t.Error("unavailable product must not get a full 'product: <ref>' block")
	}
	if !strings.Contains(prompt, "Набор посуды") {
		t.Error("unavailable product's NAME must still appear (name-only, in PRODUCTS_UNAVAILABLE)")
	}
	if strings.Contains(prompt, "скоро") {
		t.Error("unavailable product's virtual fact instruction/value must never render")
	}
}

// --- Token generation (required test #5) -----------------------------------

func TestVirtualTokens_ProductTariffTariffInfoGenerated(t *testing.T) {
	kb := virtualFactsKB()
	cat, err := BuildCatalog(kb)
	if err != nil {
		t.Fatalf("BuildCatalog: %v", err)
	}
	for _, tok := range []string{
		"{{product.coffee-machine.working_pressure}}",
		"{{product.coffee-machine.has_wifi}}",
		"{{product.coffee-machine.model_code}}",
		"{{tariff.basic.limit_on_devices}}",
		"{{tariff_info.main.trial_in_days}}",
	} {
		if cat.FactByToken(tok) == nil {
			t.Errorf("expected catalog to contain virtual fact token %s", tok)
		}
	}
}

func TestVirtualTokens_RenderedExactlyOnceInV6Frame(t *testing.T) {
	kb := virtualFactsKB()
	prompt, cat, err := BuildPrompt(FrameShopKBV6RU(), kb)
	if err != nil {
		t.Fatalf("BuildPrompt: %v", err)
	}
	if strings.Contains(prompt, "%%") {
		t.Errorf("prompt still contains an unfilled slot marker:\n%s", prompt)
	}
	for _, f := range cat.Facts {
		if n := strings.Count(prompt, f.Token); n != 1 {
			t.Errorf("fact token %s appears %d times, want exactly 1", f.Token, n)
		}
	}
	for _, m := range cat.Media {
		if n := strings.Count(prompt, m.Token); n != 1 {
			t.Errorf("media token %s appears %d times, want exactly 1", m.Token, n)
		}
	}
	if err := ValidatePrompt(prompt, cat); err != nil {
		t.Errorf("ValidatePrompt: %v", err)
	}
}

// --- Substitution by scalar type (required test #10) -----------------------

func TestResolveFactLang_NumberSubstitutesCanonicalDigits(t *testing.T) {
	kb := virtualFactsKB()
	cat, err := BuildCatalog(kb)
	if err != nil {
		t.Fatalf("BuildCatalog: %v", err)
	}
	got, err := ResolveFactLang("{{product.coffee-machine.working_pressure}}", kb, cat, "ru")
	if err != nil {
		t.Fatalf("ResolveFactLang: %v", err)
	}
	if got != "275" {
		t.Errorf("resolved = %q, want %q", got, "275")
	}
}

func TestResolveFactLang_BooleanSubstitutesLocalizedWording(t *testing.T) {
	kb := virtualFactsKB()
	cat, err := BuildCatalog(kb)
	if err != nil {
		t.Fatalf("BuildCatalog: %v", err)
	}
	ru, err := ResolveFactLang("{{product.coffee-machine.has_wifi}}", kb, cat, "ru")
	if err != nil {
		t.Fatalf("ResolveFactLang(ru): %v", err)
	}
	if ru != "да" {
		t.Errorf("ru resolved = %q, want %q", ru, "да")
	}
	kk, err := ResolveFactLang("{{product.coffee-machine.has_wifi}}", kb, cat, "kk")
	if err != nil {
		t.Fatalf("ResolveFactLang(kk): %v", err)
	}
	if kk != "иә" {
		t.Errorf("kk resolved = %q, want %q", kk, "иә")
	}
}

func TestResolveFactLang_StringSubstitutesVerbatim(t *testing.T) {
	kb := virtualFactsKB()
	cat, err := BuildCatalog(kb)
	if err != nil {
		t.Fatalf("BuildCatalog: %v", err)
	}
	got, err := ResolveFactLang("{{product.coffee-machine.model_code}}", kb, cat, "ru")
	if err != nil {
		t.Fatalf("ResolveFactLang: %v", err)
	}
	if got != "DLM-500X" {
		t.Errorf("resolved = %q, want %q", got, "DLM-500X")
	}
}

func TestResolveFactLang_TariffAndTariffInfoTokensResolve(t *testing.T) {
	kb := virtualFactsKB()
	cat, err := BuildCatalog(kb)
	if err != nil {
		t.Fatalf("BuildCatalog: %v", err)
	}
	if got, err := ResolveFactLang("{{tariff.basic.limit_on_devices}}", kb, cat, "ru"); err != nil || got != "5" {
		t.Errorf("tariff fact resolved = (%q, %v), want (5, nil)", got, err)
	}
	if got, err := ResolveFactLang("{{tariff_info.main.trial_in_days}}", kb, cat, "ru"); err != nil || got != "3" {
		t.Errorf("tariff_info fact resolved = (%q, %v), want (3, nil)", got, err)
	}
}

// --- Fail-closed substitution (required test #13) ---------------------------

func TestResolveFactLang_RemovedFactFailsClosed(t *testing.T) {
	kb := virtualFactsKB()
	cat, err := BuildCatalog(kb)
	if err != nil {
		t.Fatalf("BuildCatalog: %v", err)
	}
	token := "{{product.coffee-machine.working_pressure}}"
	if cat.FactByToken(token) == nil {
		t.Fatal("fixture setup: token must exist in the catalog before the fail-closed check")
	}
	// The fact is removed from the CURRENT KB (e.g. the seller deleted it)
	// after the catalog/prompt was already generated.
	kb.Products[0].AdditionalFacts = kb.Products[0].AdditionalFacts[1:] // drop working_pressure
	if _, err := ResolveFactLang(token, kb, cat, "ru"); err == nil {
		t.Error("want error resolving a removed fact against the CURRENT kb, got nil")
	}
	if _, err := SubstituteFactsLang(token, kb, cat, "ru"); err == nil {
		t.Error("want error substituting a removed fact, got nil")
	}
}

func TestResolveFactLang_TypeChangedFactFailsClosed(t *testing.T) {
	kb := virtualFactsKB()
	cat, err := BuildCatalog(kb)
	if err != nil {
		t.Fatalf("BuildCatalog: %v", err)
	}
	token := "{{product.coffee-machine.working_pressure}}"
	// The fact still exists under the same ref, but its scalar type changed
	// (number -> string) since the catalog was built — stale, must fail
	// closed rather than silently substituting the new string.
	for i := range kb.Products[0].AdditionalFacts {
		if kb.Products[0].AdditionalFacts[i].Ref == "working_pressure" {
			kb.Products[0].AdditionalFacts[i].Value = "250 бар примерно"
		}
	}
	if _, err := ResolveFactLang(token, kb, cat, "ru"); err == nil {
		t.Error("want error resolving a type-changed fact, got nil")
	}
}

func TestResolveFactLang_ClearedValueFailsClosed(t *testing.T) {
	kb := virtualFactsKB()
	cat, err := BuildCatalog(kb)
	if err != nil {
		t.Fatalf("BuildCatalog: %v", err)
	}
	token := "{{product.coffee-machine.model_code}}"
	for i := range kb.Products[0].AdditionalFacts {
		if kb.Products[0].AdditionalFacts[i].Ref == "model_code" {
			kb.Products[0].AdditionalFacts[i].Value = ""
		}
	}
	if _, err := ResolveFactLang(token, kb, cat, "ru"); err == nil {
		t.Error("want error resolving a now-empty fact, got nil")
	}
}

func TestResolveFactLang_ProductGoneUnavailableFailsClosedForVirtualFactToo(t *testing.T) {
	kb := virtualFactsKB()
	cat, err := BuildCatalog(kb)
	if err != nil {
		t.Fatalf("BuildCatalog: %v", err)
	}
	token := "{{product.coffee-machine.working_pressure}}"
	kb.Products[0].AvailabilityStatus = "unavailable"
	if _, err := ResolveFactLang(token, kb, cat, "ru"); err == nil {
		t.Error("want error: product went unavailable since the catalog was built, so its virtual fact must fail closed")
	}
	priceTok := "{{product.coffee-machine.price}}"
	if _, err := ResolveFactLang(priceTok, kb, cat, "ru"); err == nil {
		t.Error("want error: the concrete price fact must ALSO fail closed once the product is unavailable")
	}
}

// --- No exact-value leakage (required test #11) ------------------------------

func TestValidateResponse_VirtualTokenIsAcceptedButRawValueLeakIsFlagged(t *testing.T) {
	kb := virtualFactsKB()
	cat, err := BuildCatalog(kb)
	if err != nil {
		t.Fatalf("BuildCatalog: %v", err)
	}

	valid := `{"reply_text":"Рабочее давление, бар: {{product.coffee-machine.working_pressure}}.","reply_language":"ru","media_files_to_send":[],"escalate":false}`
	if _, issues := ValidateResponse(valid, kb, cat); len(issues) != 0 {
		t.Errorf("valid response using the token was rejected: %+v", issues)
	}

	leaked := `{"reply_text":"Рабочее давление 275 бар.","reply_language":"ru","media_files_to_send":[],"escalate":false}`
	_, issues := ValidateResponse(leaked, kb, cat)
	found := false
	for _, iss := range issues {
		if iss.Code == "exact_value_literal" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected exact_value_literal issue for a model-authored raw fact value, got %+v", issues)
	}
}

func TestBuildPrompt_NeverContainsRawVirtualFactValues(t *testing.T) {
	kb := virtualFactsKB()
	prompt, _, err := BuildPrompt(FrameShopKBV6RU(), kb)
	if err != nil {
		t.Fatalf("BuildPrompt: %v", err)
	}
	// The instruction text is allowed (and expected) in the prompt; the
	// hidden VALUES themselves must never appear as bare literals.
	for _, raw := range []string{"275", "DLM-500X"} {
		if strings.Contains(prompt, raw) {
			t.Errorf("rendered prompt leaks a hidden fact's raw value %q", raw)
		}
	}
}

// --- Malformed/duplicate/colliding facts fail BuildCatalog (required tests
// #6, #7, #8) — defense-in-depth re-validation at catalog build time -------

func TestBuildCatalog_RejectsCollidingRef(t *testing.T) {
	kb := baseKB()
	kb.Products[0].AdditionalFacts = []AdditionalFact{
		{Ref: "price", Value: json.Number("1"), Instruction: "i"},
	}
	if _, err := BuildCatalog(kb); err == nil {
		t.Fatal("want BuildCatalog error for a virtual ref colliding with the concrete price column, got nil")
	}
}

func TestBuildCatalog_RejectsDuplicateRef(t *testing.T) {
	kb := baseKB()
	kb.Products[0].AdditionalFacts = []AdditionalFact{
		{Ref: "limit_on_devices", Value: json.Number("1"), Instruction: "first"},
		{Ref: "limit_on_devices", Value: json.Number("2"), Instruction: "second"},
	}
	if _, err := BuildCatalog(kb); err == nil {
		t.Fatal("want BuildCatalog error for a duplicate virtual ref, got nil")
	}
}

func TestBuildCatalog_UnknownRefIsSimplyNotInCatalog(t *testing.T) {
	// "Unknown ref" at substitution time (a token naming a ref the current
	// entity does not carry) is covered by TestResolveFactLang_RemovedFactFailsClosed;
	// this test covers the build side: a well-formed fact list builds fine
	// and only its own refs appear as tokens, nothing invented.
	kb := baseKB()
	kb.Products[0].AdditionalFacts = []AdditionalFact{
		{Ref: "known_fact", Value: json.Number("1"), Instruction: "i"},
	}
	cat, err := BuildCatalog(kb)
	if err != nil {
		t.Fatalf("BuildCatalog: %v", err)
	}
	if cat.FactByToken("{{product.coffee-machine.unknown_fact}}") != nil {
		t.Error("catalog must never contain a token for a ref that was never declared")
	}
	if cat.FactByToken("{{product.coffee-machine.known_fact}}") == nil {
		t.Error("catalog must contain the token for the ref that WAS declared")
	}
}
