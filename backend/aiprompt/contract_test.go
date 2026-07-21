package aiprompt

import (
	"encoding/json"
	"strings"
	"testing"
)

func validCatalog(t *testing.T) *Catalog {
	t.Helper()
	cat, err := BuildCatalog(baseKB())
	if err != nil {
		t.Fatalf("BuildCatalog: %v", err)
	}
	return cat
}

const validResponseJSON = `{
  "reply_text": "Кофемашина стоит {{product.coffee-machine.price}}.",
  "reply_language": "ru",
  "media_files_to_send": ["products.coffee-machine.featured_image"],
  "escalate": false,
  "escalation_reason": "",
  "confidence": 0.9
}`

func TestValidateResponse_Valid(t *testing.T) {
	cat := validCatalog(t)
	resp, issues := ValidateResponse(validResponseJSON, cat)
	if len(issues) != 0 {
		t.Fatalf("unexpected issues: %+v", issues)
	}
	if resp == nil {
		t.Fatal("expected a parsed response")
	}
	if resp.ReplyLanguage != "ru" {
		t.Errorf("ReplyLanguage = %q, want ru", resp.ReplyLanguage)
	}
}

func TestValidateResponse_MissingField(t *testing.T) {
	cat := validCatalog(t)
	raw := `{
  "reply_text": "ok",
  "reply_language": "ru",
  "media_files_to_send": [],
  "escalate": false,
  "escalation_reason": ""
}` // confidence omitted
	_, issues := ValidateResponse(raw, cat)
	if !containsCode(issues, "missing_property") {
		t.Fatalf("expected missing_property issue, got %+v", issues)
	}
	if issueDetail(issues, "missing_property") != "confidence" {
		t.Fatalf("expected missing property 'confidence', got issues %+v", issues)
	}
}

func TestValidateResponse_WrongType(t *testing.T) {
	cat := validCatalog(t)
	raw := `{
  "reply_text": "ok",
  "reply_language": "ru",
  "media_files_to_send": [],
  "escalate": false,
  "escalation_reason": "",
  "confidence": "high"
}`
	resp, issues := ValidateResponse(raw, cat)
	if resp != nil {
		t.Fatalf("expected nil response on type error, got %+v", resp)
	}
	if !containsCode(issues, "parse") {
		t.Fatalf("expected a parse issue for wrong-typed confidence, got %+v", issues)
	}
}

func TestValidateResponse_UnknownProperty(t *testing.T) {
	cat := validCatalog(t)
	// Old alias fields must not be accepted anywhere near the contract.
	for _, alias := range []string{"asset_refs", "attach_groups", "send"} {
		t.Run(alias, func(t *testing.T) {
			raw := `{
  "reply_text": "ok",
  "reply_language": "ru",
  "media_files_to_send": [],
  "escalate": false,
  "escalation_reason": "",
  "confidence": 0.5,
  "` + alias + `": []
}`
			_, issues := ValidateResponse(raw, cat)
			if !containsCode(issues, "unknown_property") {
				t.Fatalf("expected unknown_property issue for alias %q, got %+v", alias, issues)
			}
			if issueDetail(issues, "unknown_property") != alias {
				t.Fatalf("expected unknown_property detail %q, got issues %+v", alias, issues)
			}
		})
	}
}

func TestValidateResponse_BadLanguage(t *testing.T) {
	cat := validCatalog(t)
	for _, lang := range []string{"kk", "en", ""} {
		raw := `{
  "reply_text": "ok",
  "reply_language": "` + lang + `",
  "media_files_to_send": [],
  "escalate": false,
  "escalation_reason": "",
  "confidence": 0.5
}`
		_, issues := ValidateResponse(raw, cat)
		if !containsCode(issues, "bad_language") {
			t.Errorf("lang %q: expected bad_language issue, got %+v", lang, issues)
		}
	}
}

func TestValidateResponse_BadConfidence(t *testing.T) {
	cat := validCatalog(t)
	for _, conf := range []float64{-0.01, 1.01, 2.0} {
		raw := `{
  "reply_text": "ok",
  "reply_language": "ru",
  "media_files_to_send": [],
  "escalate": false,
  "escalation_reason": "",
  "confidence": ` + jsonNum(conf) + `
}`
		_, issues := ValidateResponse(raw, cat)
		if !containsCode(issues, "bad_confidence") {
			t.Errorf("confidence %v: expected bad_confidence issue, got %+v", conf, issues)
		}
	}
}

func jsonNum(f float64) string {
	b, _ := json.Marshal(f)
	return string(b)
}

func TestValidateResponse_EscalationReasonMismatch(t *testing.T) {
	cat := validCatalog(t)
	raw := `{
  "reply_text": "ok",
  "reply_language": "ru",
  "media_files_to_send": [],
  "escalate": false,
  "escalation_reason": "клиент спрашивает о том, чего нет в базе",
  "confidence": 0.5
}`
	_, issues := ValidateResponse(raw, cat)
	if !containsCode(issues, "escalation_reason_mismatch") {
		t.Fatalf("expected escalation_reason_mismatch issue, got %+v", issues)
	}
}

func TestValidateResponse_InventedMediaToken(t *testing.T) {
	cat := validCatalog(t)
	raw := `{
  "reply_text": "ok",
  "reply_language": "ru",
  "media_files_to_send": ["products.coffee-machine.demo_videos"],
  "escalate": false,
  "escalation_reason": "",
  "confidence": 0.5
}`
	// demo_videos was never populated on coffee-machine in baseKB, so it must
	// not exist in the catalog.
	_, issues := ValidateResponse(raw, cat)
	if !containsCode(issues, "unknown_media_token") {
		t.Fatalf("expected unknown_media_token issue, got %+v", issues)
	}
	if issueDetail(issues, "unknown_media_token") != "products.coffee-machine.demo_videos" {
		t.Fatalf("unexpected detail: %+v", issues)
	}
}

func TestValidateResponse_NilCatalogSkipsMediaCheck(t *testing.T) {
	raw := `{
  "reply_text": "ok",
  "reply_language": "ru",
  "media_files_to_send": ["totally.invented.token"],
  "escalate": false,
  "escalation_reason": "",
  "confidence": 0.5
}`
	resp, issues := ValidateResponse(raw, nil)
	if resp == nil {
		t.Fatal("expected a parsed response even with nil catalog")
	}
	if containsCode(issues, "unknown_media_token") {
		t.Fatalf("nil catalog must skip the media-token check, got %+v", issues)
	}
}

func TestSubstituteFacts_InStockWording(t *testing.T) {
	cat := validCatalog(t)
	out, err := SubstituteFacts("Наличие: {{product.coffee-machine.in_stock}}.", cat)
	if err != nil {
		t.Fatalf("SubstituteFacts: %v", err)
	}
	if !strings.Contains(out, "в наличии") || strings.Contains(out, "нет в наличии") {
		t.Errorf("in_stock=true should render «в наличии», got %q", out)
	}

	out2, err := SubstituteFacts("Наличие: {{product.cookware-set.in_stock}}.", cat)
	if err != nil {
		t.Fatalf("SubstituteFacts: %v", err)
	}
	if !strings.Contains(out2, "нет в наличии") {
		t.Errorf("in_stock=false should render «нет в наличии», got %q", out2)
	}
}

func TestSubstituteFacts_UnknownPlaceholder(t *testing.T) {
	cat := validCatalog(t)
	_, err := SubstituteFacts("Цена: {{product.does-not-exist.price}}.", cat)
	if err == nil {
		t.Fatal("expected an error for an unknown placeholder")
	}
	if !strings.Contains(err.Error(), "unknown placeholder") {
		t.Errorf("error message should name the problem, got %q", err.Error())
	}
}

func TestSubstituteFacts_ExactValueVerbatim(t *testing.T) {
	cat := validCatalog(t)
	out, err := SubstituteFacts("Цена: {{product.coffee-machine.price}}.", cat)
	if err != nil {
		t.Fatalf("SubstituteFacts: %v", err)
	}
	if out != "Цена: 129 900 ₸." {
		t.Errorf("got %q", out)
	}
}

func TestResolveSend_OrderingAndGroupResolution(t *testing.T) {
	kb := baseKB()
	cat := validCatalog(t)
	// Request media in an order different from catalog build order.
	got, err := ResolveSend([]string{
		"products.coffee-machine.gallery_images",
		"products.coffee-machine.featured_image",
	}, kb, cat)
	if err != nil {
		t.Fatalf("ResolveSend: %v", err)
	}
	// gallery_images (2 materials) then featured_image (1 material) = 3 total,
	// in the order the tokens were requested.
	if len(got) != 3 {
		t.Fatalf("got %d resolved materials, want 3: %+v", len(got), got)
	}
	wantMaterial := []string{"m-cm-gallery-1", "m-cm-gallery-2", "m-cm-featured"}
	wantToken := []string{
		"products.coffee-machine.gallery_images",
		"products.coffee-machine.gallery_images",
		"products.coffee-machine.featured_image",
	}
	for i := range wantMaterial {
		if got[i].Material.ID != wantMaterial[i] {
			t.Errorf("resolved[%d].Material.ID = %q, want %q", i, got[i].Material.ID, wantMaterial[i])
		}
		if got[i].Token != wantToken[i] {
			t.Errorf("resolved[%d].Token = %q, want %q", i, got[i].Token, wantToken[i])
		}
	}
}

func TestResolveSend_Deduplicates(t *testing.T) {
	kb := baseKB()
	cat := validCatalog(t)
	got, err := ResolveSend([]string{
		"products.coffee-machine.featured_image",
		"products.coffee-machine.featured_image",
	}, kb, cat)
	if err != nil {
		t.Fatalf("ResolveSend: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected the repeated token to resolve once, got %d: %+v", len(got), got)
	}
}

func TestResolveSend_UnknownToken(t *testing.T) {
	kb := baseKB()
	cat := validCatalog(t)
	_, err := ResolveSend([]string{"products.coffee-machine.demo_videos"}, kb, cat)
	if err == nil {
		t.Fatal("expected an error for a token absent from the catalog")
	}
	if !strings.Contains(err.Error(), "not in the catalog") {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestResolveSend_StaleTokenFails: a token that was valid when the catalog
// was built must still fail to resolve if the underlying material becomes
// invalid before delivery (re-validation, not a rubber stamp).
func TestResolveSend_StaleTokenFails(t *testing.T) {
	kb := baseKB()
	cat := validCatalog(t)
	setMaterial(kb, "m-cm-featured", func(m *Material) { m.CustomerVisibility = "invisible" })
	_, err := ResolveSend([]string{"products.coffee-machine.featured_image"}, kb, cat)
	if err == nil {
		t.Fatal("expected ResolveSend to fail-closed on a since-invalidated material")
	}
}

// TestResolveSend_PartialGroupFailureBlocksWholeSend: if one material inside
// a group becomes invalid, the whole group send fails — no partial delivery.
func TestResolveSend_PartialGroupFailureBlocksWholeSend(t *testing.T) {
	kb := baseKB()
	cat := validCatalog(t)
	setMaterial(kb, "m-cm-gallery-2", func(m *Material) { m.SizeBytes = 0 })
	_, err := ResolveSend([]string{"products.coffee-machine.gallery_images"}, kb, cat)
	if err == nil {
		t.Fatal("expected the whole gallery_images group to fail when one member is invalid")
	}
}
