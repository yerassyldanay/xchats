package aiprompt

import (
	"strings"
	"testing"
)

// TestFrameShopKBV4RU_RendersBaseKBWithExpectedContent renders the real
// production frame (byte-pinned in frame_test.go) against a realistic KB —
// every other prompt test in this package uses the synthetic basicFrame or
// v2Frame fixtures, never the frame production actually sends to the LLM.
// This checks the assembled prompt content end to end: in-stock/out-of-stock
// product blocks, topic prose, business facts, media references, the
// response schema, and — defense in depth — that no exact fact value or
// storage locator ever reaches the text.
func TestFrameShopKBV4RU_RendersBaseKBWithExpectedContent(t *testing.T) {
	kb := baseKB()
	prompt, cat, err := BuildPrompt(FrameShopKBV4RU(), kb)
	if err != nil {
		t.Fatalf("BuildPrompt: %v", err)
	}
	if cat == nil {
		t.Fatal("expected a non-nil catalog")
	}
	if strings.Contains(prompt, "%%") {
		t.Errorf("prompt still contains an unfilled slot marker:\n%s", prompt)
	}

	for _, want := range []string{
		"Кофемашина DeLonghi",                    // in-stock product name
		"{{product.coffee-machine.price}}",       // its price placeholder
		"products.coffee-machine.featured_image", // its featured-image ref
		"products.coffee-machine.gallery_images", // its gallery ref
		"- Набор посуды",                         // out-of-stock product, name only
		"Доставка",                               // topic title
		"Доставляем по всему Алматы",             // topic body prose
		"{{contact.main.phone}}",                 // business fact: phone placeholder
		"{{policy.main.delivery_cost}}",          // business fact: flat delivery cost
		"{{policy.main.delivery_in_days}}",       // business fact: flat delivery days
		`"media_files_to_send"`,                  // response schema
	} {
		if !strings.Contains(prompt, want) {
			t.Errorf("rendered prompt missing expected content %q", want)
		}
	}

	// Rule 1a ("known but unavailable — no facts, no media") must hold
	// structurally, not just as frame prose: the out-of-stock product's own
	// ref must never appear as a fact or media token anywhere in the text.
	if strings.Contains(prompt, "cookware-set") {
		t.Error("out-of-stock product must carry no fact or media token anywhere in the prompt")
	}

	// Exact fact values are substituted only after the model responds
	// (SubstituteFactsLang) — the rendered prompt must carry placeholders
	// only, never the underlying value.
	for _, leaked := range []string{
		kb.Products[0].Price, kb.Contacts.Phone, kb.Contacts.WorkingHours,
		kb.Policies.DeliveryCost, kb.Policies.DeliveryInDays,
	} {
		if strings.Contains(prompt, leaked) {
			t.Errorf("rendered prompt leaks exact fact value %q", leaked)
		}
	}

	if err := ValidateNoStorageLocatorLeak(prompt); err != nil {
		t.Errorf("ValidateNoStorageLocatorLeak on a clean real-shaped prompt: %v", err)
	}
}

// TestFrameShopKBV4RU_RendersZonesKBWithExpectedContent proves the delivery
// zones hierarchy and per-zone facts (rule 2) render correctly through the
// real frame, and that once zones exist the flat ai_policies delivery facts
// never appear alongside them.
func TestFrameShopKBV4RU_RendersZonesKBWithExpectedContent(t *testing.T) {
	kb := zonesKB()
	prompt, _, err := BuildPrompt(FrameShopKBV4RU(), kb)
	if err != nil {
		t.Fatalf("BuildPrompt: %v", err)
	}

	for _, want := range []string{
		"kazakhstan | Казахстан (остальные города) | zone_level: country | parent_ref: —",
		"astana | Астана | zone_level: city | parent_ref: kazakhstan",
		"baikonur | Байконур | zone_level: city | parent_ref: kazakhstan | Закрытый административный статус — доставка недоступна.",
		"{{delivery.astana.delivery_cost}}",
		"{{delivery.astana.delivery_in_days}}",
		"{{delivery.kazakhstan.delivery_available}}",
		"{{delivery.baikonur.delivery_available}}",
		"{{policy.main.outside_zones_note}}",
	} {
		if !strings.Contains(prompt, want) {
			t.Errorf("rendered prompt missing expected zones content %q", want)
		}
	}

	for _, absent := range []string{"{{policy.main.delivery_cost}}", "{{policy.main.delivery_in_days}}"} {
		if strings.Contains(prompt, absent) {
			t.Errorf("rendered prompt has flat delivery placeholder %q alongside zones", absent)
		}
	}

	if err := ValidateNoStorageLocatorLeak(prompt); err != nil {
		t.Errorf("ValidateNoStorageLocatorLeak on a clean zones prompt: %v", err)
	}
}

// TestFrameShopKBV4RU_LeakGateCatchesURLPastedIntoTopicProse proves the
// storage-locator leak gate is genuinely load-bearing defense in depth: a
// topic body is seller-authored trusted prose, rendered into the prompt
// verbatim, so nothing in BuildPrompt's own checks (ValidateNoMaterialLeak,
// ValidatePrompt) has any reason to reject an operator accidentally pasting a
// real URL into it. Only the explicit ValidateNoStorageLocatorLeak call that
// Engine.Generate runs after RenderPrompt catches it before the prompt ever
// reaches the LLM.
func TestFrameShopKBV4RU_LeakGateCatchesURLPastedIntoTopicProse(t *testing.T) {
	kb := baseKB()
	kb.Topics[0].BodyMD += " Подробнее: https://good-shop.example.com/promo"

	prompt, _, err := BuildPrompt(FrameShopKBV4RU(), kb)
	if err != nil {
		t.Fatalf("BuildPrompt: %v (a URL in trusted topic prose is not a material leak, so BuildPrompt itself must still succeed)", err)
	}
	if !strings.Contains(prompt, "https://good-shop.example.com/promo") {
		t.Fatal("test setup invalid: expected the pasted URL to actually reach the rendered prompt")
	}

	if err := ValidateNoStorageLocatorLeak(prompt); err == nil {
		t.Fatal("want an error: a URL accidentally pasted into topic prose must still be caught by the storage-locator leak gate before reaching the LLM")
	}
}
