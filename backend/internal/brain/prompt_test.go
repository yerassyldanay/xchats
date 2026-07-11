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
		Facts:   domain.NewFactBook(tariffs, nil, nil, nil),
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

// TestPostProcess_EscalateReplacesInventedClaimWithHoldingReply is the safety test
// for the exact failure mode observed in eval canaries: a model correctly sets
// escalate=true but ALSO writes a confident, invented claim in reply_text (e.g.
// asserting the business does not deliver somewhere — a claim the knowledge base
// never makes). The prompt telling the model not to do this is a quality measure;
// this test proves the pipeline enforces it regardless of what the model wrote.
func TestPostProcess_EscalateReplacesInventedClaimWithHoldingReply(t *testing.T) {
	raw := domain.RawDraft{
		ReplyText:        "К сожалению, мы не доставляем в Астану. Уточню у коллеги.",
		ReplyLanguage:    "ru",
		Escalate:         true,
		EscalationReason: "off-KB city coverage",
		AssetRefs:        []string{"pricing_card"},
	}
	d := PostProcess(raw, testSnapshot(), nil)
	if strings.Contains(d.ReplyText, "не доставляем") {
		t.Fatalf("model's invented claim leaked into the customer-facing draft: %q", d.ReplyText)
	}
	if d.ReplyText != HoldingReply {
		t.Fatalf("ReplyText = %q, want the trusted ru holding reply %q", d.ReplyText, HoldingReply)
	}
	if d.EscalationReason != "off-KB city coverage" {
		t.Fatalf("EscalationReason must survive unchanged, got %q", d.EscalationReason)
	}
	if len(d.Media) != 0 {
		t.Fatal("escalation must carry no media")
	}
}

// TestPostProcess_EscalateKazakhGetsKazakhHoldingReply proves the fallback is
// language-aware: a Kazakh-declaring escalation must not silently fall back to the
// Russian holding reply (that would itself be the kind of language failure the
// eval's Phase 2.1 language bake-off exists to catch, just relocated to the
// escalation path instead of the normal-answer path).
func TestPostProcess_EscalateKazakhGetsKazakhHoldingReply(t *testing.T) {
	raw := domain.RawDraft{
		ReplyText:        "Өкінішке орай, біз Астанаға жеткізбейміз.",
		ReplyLanguage:    "kk",
		Escalate:         true,
		EscalationReason: "off-KB city coverage, kk",
	}
	d := PostProcess(raw, testSnapshot(), nil)
	if d.ReplyText != holdingReplyByLanguage["kk"] {
		t.Fatalf("ReplyText = %q, want the trusted kk holding reply %q", d.ReplyText, holdingReplyByLanguage["kk"])
	}
	if strings.Contains(d.ReplyText, "жеткізбейміз") {
		t.Fatalf("model's invented Kazakh claim leaked into the customer-facing draft: %q", d.ReplyText)
	}
}

// TestPostProcess_EscalateUnknownLanguageDefaultsToRussian mirrors the same
// empty/unrecognized -> "ru" default PostProcess already applies to
// raw.ReplyLanguage on the normal-answer path (see the fact-injection step above).
func TestPostProcess_EscalateUnknownLanguageDefaultsToRussian(t *testing.T) {
	for _, lang := range []string{"", "en", "mixed"} {
		raw := domain.RawDraft{ReplyText: "anything", ReplyLanguage: lang, Escalate: true}
		d := PostProcess(raw, testSnapshot(), nil)
		if d.ReplyText != HoldingReply {
			t.Errorf("lang=%q: ReplyText = %q, want the ru default %q", lang, d.ReplyText, HoldingReply)
		}
	}
}

// TestPostProcess_EscalatePreservesConfidence proves the fallback only replaces
// ReplyText — Confidence (and, per the earlier tests, EscalationReason) still come
// from the model, since only the customer-facing text is a hallucination risk.
func TestPostProcess_EscalatePreservesConfidence(t *testing.T) {
	raw := domain.RawDraft{ReplyText: "x", Escalate: true, Confidence: 0.42}
	d := PostProcess(raw, testSnapshot(), nil)
	if d.Confidence != 0.42 {
		t.Fatalf("Confidence = %v, want 0.42 (must pass through from the model unchanged)", d.Confidence)
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
