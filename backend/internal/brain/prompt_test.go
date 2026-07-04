package brain

import (
	"strings"
	"testing"

	"github.com/yerassyldanay/xchats/backend/internal/brain/domain"
)

func testSnapshot() *domain.Snapshot {
	tariffs := []domain.Tariff{
		{Ref: "standard", Lang: "ru", Name: "Стандарт", Price: "19 900 ₸"},
	}
	return &domain.Snapshot{
		Config:  domain.AssistantConfig{Persona: "p", Guardrails: "g"},
		Tariffs: tariffs,
		Facts:   domain.NewFactBook(tariffs, nil, nil),
		Topics:  []domain.Topic{{Slug: "pricing", Language: "ru", BodyMD: "Наш стандартный тариф покрывает основные нужды."}},
		Assets:  []domain.Asset{{Ref: "pricing_card", Kind: "image", URL: "/media/p.png"}},
	}
}

// BuildSystem must advertise the Facts lane [F] block so the model has a token to
// emit — bodies are pure prose now, so this catalog is the only place the tokens live.
func TestBuildSystem_IncludesFactsBlock(t *testing.T) {
	sys := BuildSystem(testSnapshot())
	if !strings.Contains(sys, "FACTS — when the customer asks") {
		t.Fatalf("system prompt missing [F] FACTS block:\n%s", sys)
	}
	if !strings.Contains(sys, "{{tariff.standard.price}}") {
		t.Fatalf("FACTS block missing the tariff price token:\n%s", sys)
	}
}

// Parity with the vendored brain_test.go: the post-processing pipeline injects
// prices, resolves+drops refs, strips the stage key, and flattens status.
func TestPostProcess_NormalPricingAnswer(t *testing.T) {
	raw := domain.RawDraft{
		ReplyText:       "Стандарт — {{tariff.standard.price}}/мес.",
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
		ReplyText:     "{{tariff.enterprise.price}}", // unknown tariff
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

// The embedded Demo Shop KB must be internally consistent: topic bodies are PURE
// PROSE (no fact tokens — 14 D3), and every fact advertised in the [F] catalog
// resolves against the FactBook (catches a seed typo at test time).
func TestSeedSnapshot_PureProseAndFactsResolve(t *testing.T) {
	snap := SeedSnapshot()
	for _, topic := range snap.Topics {
		if strings.Contains(topic.BodyMD, "{{") {
			t.Fatalf("topic %q body must be pure prose, found a token: %q", topic.Slug, topic.BodyMD)
		}
	}
	facts := snap.Facts.List()
	if len(facts) == 0 {
		t.Fatal("seed must advertise at least one fact in the [F] catalog")
	}
	for _, f := range facts {
		if _, err := snap.Facts.Render(f.Token, "ru"); err != nil {
			t.Fatalf("advertised fact %q does not resolve: %v", f.Token, err)
		}
	}
}
