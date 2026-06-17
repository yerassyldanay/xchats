package brain

import (
	"testing"

	"github.com/yerassyldanay/xchats/backend/internal/brain/domain"
)

func testSnapshot() *domain.Snapshot {
	return &domain.Snapshot{
		Config: domain.AssistantConfig{Persona: "p", Guardrails: "g"},
		Values: domain.NewValueBook(
			domain.Value{Token: "price.standard", Lang: "ru", Text: "19 900 ₸"},
		),
		Topics: []domain.Topic{{Slug: "pricing", Language: "ru", BodyMD: "{{price.standard}}"}},
		Assets: []domain.Asset{{Ref: "pricing_card", Kind: "image", URL: "/media/p.png"}},
	}
}

// Parity with the vendored brain_test.go: the post-processing pipeline injects
// prices, resolves+drops refs, strips the stage key, and flattens status.
func TestPostProcess_NormalPricingAnswer(t *testing.T) {
	raw := domain.RawDraft{
		ReplyText:       "Стандарт — {{price.standard}}/мес.",
		ReplyLanguage:   "ru",
		AssetRefs:       []string{"pricing_card", "hallucinated_ref"},
		ProfilePatch:    map[string]any{"interested_plan": "standard", "stage": "qualifying"},
		SuggestedStatus: &domain.StageWrapper{Stage: "qualifying"},
		Confidence:      0.82,
	}
	d := PostProcess(raw, testSnapshot(), nil)
	if d.ReplyText != "Стандарт — 19 900 ₸/мес." {
		t.Fatalf("prices not injected: %q", d.ReplyText)
	}
	if len(d.Media) != 1 || d.Media[0].Ref != "pricing_card" {
		t.Fatalf("media not resolved: %+v", d.Media)
	}
	if len(d.DroppedRefs) != 1 || d.DroppedRefs[0] != "hallucinated_ref" {
		t.Fatalf("hallucinated ref not dropped: %v", d.DroppedRefs)
	}
	if _, ok := d.ProfilePatch["stage"]; ok {
		t.Fatal("stage key must be stripped from profile_patch")
	}
	if d.ProfilePatch["interested_plan"] != "standard" {
		t.Fatalf("profile_patch lost field: %+v", d.ProfilePatch)
	}
	if d.SuggestedStatus != "qualifying" {
		t.Fatalf("status not flattened: %q", d.SuggestedStatus)
	}
	if d.Escalate {
		t.Fatal("should not escalate")
	}
}

func TestPostProcess_EscalateGateStops(t *testing.T) {
	raw := domain.RawDraft{
		ReplyText:        "Уточню у коллеги.",
		Escalate:         true,
		EscalationReason: "off-KB",
		AssetRefs:        []string{"pricing_card"},
	}
	d := PostProcess(raw, testSnapshot(), nil)
	if !d.Escalate {
		t.Fatal("expected escalate")
	}
	if len(d.Media) != 0 {
		t.Fatal("escalation must carry no media")
	}
}

func TestPostProcess_PriceRenderFailurePostsManualNote(t *testing.T) {
	raw := domain.RawDraft{
		ReplyText:     "{{price.enterprise}}", // unknown tariff
		ReplyLanguage: "ru",
		AssetRefs:     []string{"pricing_card"},
	}
	d := PostProcess(raw, testSnapshot(), nil)
	if !d.PricingError {
		t.Fatal("expected PricingError")
	}
	if d.ReplyText != pricingManualNote {
		t.Fatalf("expected manual-check note, got %q", d.ReplyText)
	}
	if len(d.Media) != 0 {
		t.Fatal("must not ship media with a pricing failure")
	}
}

// The embedded Demo Shop KB must be internally consistent: every {{token}} in a
// topic body resolves against the price book (catches a seed typo at test time).
func TestSeedSnapshot_TokensResolve(t *testing.T) {
	snap := SeedSnapshot()
	for _, topic := range snap.Topics {
		if _, err := snap.Values.Render(topic.BodyMD, topic.Language); err != nil {
			t.Fatalf("topic %q has an unresolved token: %v", topic.Slug, err)
		}
	}
}
