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
	resp, issues := ValidateResponse(validResponseJSON, baseKB(), cat)
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

func TestValidateResponse_MarkdownFencedJSON(t *testing.T) {
	cat := validCatalog(t)
	for name, raw := range map[string]string{
		"json fence": "```json\n" + validResponseJSON + "\n```",
		"bare fence": "```\n" + validResponseJSON + "\n```",
		"padded":     "  ```json  \n" + validResponseJSON + "\n```  \n",
	} {
		resp, issues := ValidateResponse(raw, baseKB(), cat)
		if len(issues) != 0 {
			t.Errorf("%s: unexpected issues: %+v", name, issues)
		}
		if resp == nil {
			t.Errorf("%s: expected a parsed response", name)
		}
	}
}

func TestValidateResponse_FenceOnlyOutput(t *testing.T) {
	cat := validCatalog(t)
	resp, issues := ValidateResponse("```json\n```", baseKB(), cat)
	if resp != nil {
		t.Fatal("fence with no body must not parse")
	}
	if !containsCode(issues, "parse") {
		t.Fatalf("expected parse issue, got %+v", issues)
	}
}

func TestValidateResponse_MissingOperationalField(t *testing.T) {
	cat := validCatalog(t)
	raw := `{
  "reply_text": "ok",
  "reply_language": "ru",
  "media_files_to_send": [],
  "escalation_reason": "",
  "confidence": 0.9
}` // escalate omitted
	_, issues := ValidateResponse(raw, baseKB(), cat)
	if !containsCode(issues, "missing_property") {
		t.Fatalf("expected missing_property issue, got %+v", issues)
	}
	if issueDetail(issues, "missing_property") != "escalate" {
		t.Fatalf("expected missing property 'escalate', got issues %+v", issues)
	}
}

// Diagnostic fields (confidence, escalation_reason) are optional telemetry: their
// absence must never produce an issue.
func TestValidateResponse_MissingDiagnosticFieldsOK(t *testing.T) {
	cat := validCatalog(t)
	for name, raw := range map[string]string{
		"confidence omitted": `{
  "reply_text": "ok",
  "reply_language": "ru",
  "media_files_to_send": [],
  "escalate": false,
  "escalation_reason": ""
}`,
		"escalation_reason omitted": `{
  "reply_text": "ok",
  "reply_language": "ru",
  "media_files_to_send": [],
  "escalate": false,
  "confidence": 0.9
}`,
		"both diagnostics omitted": `{
  "reply_text": "ok",
  "reply_language": "ru",
  "media_files_to_send": [],
  "escalate": false
}`,
	} {
		t.Run(name, func(t *testing.T) {
			resp, issues := ValidateResponse(raw, baseKB(), cat)
			if resp == nil || len(issues) != 0 {
				t.Fatalf("ValidateResponse() = (%+v, %+v), want valid with no issues", resp, issues)
			}
		})
	}
}

func TestValidateResponse_WrongTypeOperational(t *testing.T) {
	cat := validCatalog(t)
	raw := `{
  "reply_text": "ok",
  "reply_language": "ru",
  "media_files_to_send": [],
  "escalate": "false",
  "escalation_reason": "",
  "confidence": 0.9
}`
	resp, issues := ValidateResponse(raw, baseKB(), cat)
	if resp != nil {
		t.Fatalf("expected nil response on operational type error, got %+v", resp)
	}
	if !containsCode(issues, "wrong_property_type") || issueDetail(issues, "wrong_property_type") != "escalate" {
		t.Fatalf("expected wrong_property_type for escalate, got %+v", issues)
	}
}

// A wrong-typed DIAGNOSTIC value must not reject an otherwise valid operational
// response — the malformed value stays visible in the caller's preserved raw output.
func TestValidateResponse_WrongTypeDiagnosticTolerated(t *testing.T) {
	cat := validCatalog(t)
	for name, raw := range map[string]string{
		"confidence string": `{
  "reply_text": "ok",
  "reply_language": "ru",
  "media_files_to_send": [],
  "escalate": false,
  "escalation_reason": "",
  "confidence": "high"
}`,
		"escalation_reason number": `{
  "reply_text": "ok",
  "reply_language": "ru",
  "media_files_to_send": [],
  "escalate": false,
  "escalation_reason": 7,
  "confidence": 0.9
}`,
	} {
		t.Run(name, func(t *testing.T) {
			resp, issues := ValidateResponse(raw, baseKB(), cat)
			if resp == nil || len(issues) != 0 {
				t.Fatalf("ValidateResponse() = (%+v, %+v), want valid with no issues", resp, issues)
			}
		})
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
			_, issues := ValidateResponse(raw, baseKB(), cat)
			if !containsCode(issues, "unknown_property") {
				t.Fatalf("expected unknown_property issue for alias %q, got %+v", alias, issues)
			}
			if issueDetail(issues, "unknown_property") != alias {
				t.Fatalf("expected unknown_property detail %q, got issues %+v", alias, issues)
			}
		})
	}
}

// TestValidateResponse_BadLanguage: "kk" is now a valid reply_language (2026-07
// Kazakh customer-message testing) — only something outside {"ru","kk"} is rejected.
func TestValidateResponse_BadLanguage(t *testing.T) {
	cat := validCatalog(t)
	for _, lang := range []string{"en", "kz", ""} {
		raw := `{
  "reply_text": "ok",
  "reply_language": "` + lang + `",
  "media_files_to_send": [],
  "escalate": false,
  "escalation_reason": "",
  "confidence": 0.5
}`
		_, issues := ValidateResponse(raw, baseKB(), cat)
		if !containsCode(issues, "bad_language") {
			t.Errorf("lang %q: expected bad_language issue, got %+v", lang, issues)
		}
	}
}

// TestValidateResponse_KazakhLanguageAccepted confirms "kk" is accepted with no
// bad_language issue, and that delivery-availability wording resolves through the
// Kazakh table when the response declares it.
func TestValidateResponse_KazakhLanguageAccepted(t *testing.T) {
	kb := zonesKB() // has an active astana zone with DeliveryAvailable: true
	cat, err := BuildCatalog(kb)
	if err != nil {
		t.Fatal(err)
	}
	raw := `{
  "reply_text": "Иә, {{delivery.astana.delivery_available}}.",
  "reply_language": "kk",
  "media_files_to_send": [],
  "escalate": false,
  "escalation_reason": "",
  "confidence": 0.9
}`
	resp, issues := ValidateResponse(raw, kb, cat)
	if containsCode(issues, "bad_language") {
		t.Fatalf("want \"kk\" accepted with no bad_language issue, got %+v", issues)
	}
	if resp == nil {
		t.Fatal("want a parsed response")
	}
	out, err := SubstituteFactsLang(resp.ReplyText, kb, cat, resp.ReplyLanguage)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, DeliveryWordingKK[true]) {
		t.Errorf("want Kazakh delivery wording %q substituted, got %q", DeliveryWordingKK[true], out)
	}
	if strings.Contains(out, DeliveryWordingRU[true]) {
		t.Errorf("want Russian delivery wording NOT substituted for a Kazakh reply, got %q", out)
	}
}

// Out-of-range confidence is diagnostic noise, never a failure.
func TestValidateResponse_OutOfRangeConfidenceTolerated(t *testing.T) {
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
		resp, issues := ValidateResponse(raw, baseKB(), cat)
		if resp == nil || len(issues) != 0 {
			t.Errorf("confidence %v: ValidateResponse() = (%+v, %+v), want valid with no issues", conf, resp, issues)
		}
	}
}

func jsonNum(f float64) string {
	b, _ := json.Marshal(f)
	return string(b)
}

// A nonempty escalation_reason with escalate=false is a diagnostic observation, not a
// contract failure — the field is telemetry, never customer-facing.
func TestValidateResponse_EscalationReasonWithoutEscalateTolerated(t *testing.T) {
	cat := validCatalog(t)
	raw := `{
  "reply_text": "ok",
  "reply_language": "ru",
  "media_files_to_send": [],
  "escalate": false,
  "escalation_reason": "клиент спрашивает о том, чего нет в базе",
  "confidence": 0.5
}`
	resp, issues := ValidateResponse(raw, baseKB(), cat)
	if resp == nil || len(issues) != 0 {
		t.Fatalf("ValidateResponse() = (%+v, %+v), want valid with no issues", resp, issues)
	}
	if resp.EscalationReason == "" {
		t.Fatal("escalation_reason should still be decoded for logging")
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
	_, issues := ValidateResponse(raw, baseKB(), cat)
	if !containsCode(issues, "unknown_media_token") {
		t.Fatalf("expected unknown_media_token issue, got %+v", issues)
	}
	if issueDetail(issues, "unknown_media_token") != "products.coffee-machine.demo_videos" {
		t.Fatalf("unexpected detail: %+v", issues)
	}
}

// TestValidateResponse_MediaTokenInReplyText_Blocked is media regression case 2
// (2026-07 Phase 2/3 postmortem follow-up): a valid media token written as literal
// text inside reply_text — instead of listed in media_files_to_send — is a leak of an
// internal reference string, the same severity class as a model-authored exact value,
// even though the token itself is spelled correctly and does exist in the catalog.
func TestValidateResponse_MediaTokenInReplyText_Blocked(t *testing.T) {
	cat := validCatalog(t)
	raw := `{
  "reply_text": "Вот фото: products.coffee-machine.featured_image",
  "reply_language": "ru",
  "media_files_to_send": [],
  "escalate": false,
  "escalation_reason": "",
  "confidence": 0.5
}`
	_, issues := ValidateResponse(raw, baseKB(), cat)
	if !containsCode(issues, "media_token_in_reply_text") {
		t.Fatalf("want media_token_in_reply_text issue, got %+v", issues)
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
	resp, issues := ValidateResponse(raw, baseKB(), nil)
	if resp == nil {
		t.Fatal("expected a parsed response even with nil catalog")
	}
	if containsCode(issues, "unknown_media_token") {
		t.Fatalf("nil catalog must skip the media-token check, got %+v", issues)
	}
}

// TestSubstituteFacts_DeliveryAvailableWording covers the boolean-fact ->
// reviewed-wording substitution mechanism (previously covered by
// TestSubstituteFacts_InStockWording, removed 2026-07-26 along with in_stock as a
// fact token — delivery_available is now the only substituted boolean).
func TestSubstituteFacts_DeliveryAvailableWording(t *testing.T) {
	kb := zonesKB()
	cat, err := BuildCatalog(kb)
	if err != nil {
		t.Fatalf("BuildCatalog: %v", err)
	}
	out, err := SubstituteFacts("Доставка: {{delivery.astana.delivery_available}}.", kb, cat)
	if err != nil {
		t.Fatalf("SubstituteFacts: %v", err)
	}
	if !strings.Contains(out, "доставляем") || strings.Contains(out, "не доставляем") {
		t.Errorf("delivery_available=true should render «доставляем», got %q", out)
	}

	out2, err := SubstituteFacts("Доставка: {{delivery.baikonur.delivery_available}}.", kb, cat)
	if err != nil {
		t.Fatalf("SubstituteFacts: %v", err)
	}
	if !strings.Contains(out2, "не доставляем") {
		t.Errorf("delivery_available=false should render «не доставляем», got %q", out2)
	}
}

func TestSubstituteFacts_UnknownPlaceholder(t *testing.T) {
	cat := validCatalog(t)
	_, err := SubstituteFacts("Цена: {{product.does-not-exist.price}}.", baseKB(), cat)
	if err == nil {
		t.Fatal("expected an error for an unknown placeholder")
	}
	if !strings.Contains(err.Error(), "not in the request catalog") {
		t.Errorf("error message should name the problem, got %q", err.Error())
	}
}

func responseWithText(t *testing.T, text string) string {
	t.Helper()
	b, err := json.Marshal(Response{
		ReplyText:        text,
		ReplyLanguage:    "ru",
		MediaFilesToSend: []string{},
		Escalate:         false,
		EscalationReason: "",
		Confidence:       0.9,
	})
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestValidateResponse_RejectsAnythingExceptOneJSONObject(t *testing.T) {
	kb := baseKB()
	cat, err := BuildCatalog(kb)
	if err != nil {
		t.Fatal(err)
	}
	valid := responseWithText(t, "Готово.")
	tests := []struct {
		name string
		raw  string
	}{
		{name: "top-level null", raw: "null"},
		{name: "leading prose", raw: "Ответ: " + valid},
		{name: "trailing prose", raw: valid + " готово"},
		// A bare markdown fence around the object is NOT here: it is stripped as
		// transport noise (see TestValidateResponse_MarkdownFencedJSON). Prose
		// outside a fence still fails.
		{name: "prose before fence", raw: "Ответ:\n```json\n" + valid + "\n```"},
		{name: "second object", raw: valid + " {}"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, issues := ValidateResponse(tt.raw, kb, cat)
			if resp != nil || !containsCode(issues, "parse") {
				t.Fatalf("ValidateResponse() = (%+v, %+v), want parse failure", resp, issues)
			}
		})
	}
}

func TestValidateResponse_RejectsNullOperationalProperties(t *testing.T) {
	kb := baseKB()
	cat, err := BuildCatalog(kb)
	if err != nil {
		t.Fatal(err)
	}
	properties := []string{
		"reply_text",
		"reply_language",
		"media_files_to_send",
		"escalate",
	}
	for _, property := range properties {
		t.Run(property, func(t *testing.T) {
			var object map[string]any
			if err := json.Unmarshal([]byte(validResponseJSON), &object); err != nil {
				t.Fatal(err)
			}
			object[property] = nil
			raw, err := json.Marshal(object)
			if err != nil {
				t.Fatal(err)
			}
			resp, issues := ValidateResponse(string(raw), kb, cat)
			if resp != nil || !containsCode(issues, "wrong_property_type") {
				t.Fatalf("ValidateResponse() = (%+v, %+v), want wrong_property_type", resp, issues)
			}
			if detail := issueDetail(issues, "wrong_property_type"); detail != property {
				t.Fatalf("wrong_property_type detail = %q, want %q", detail, property)
			}
		})
	}
	// Null DIAGNOSTIC properties are tolerated — best-effort decode, never a gate.
	for _, property := range []string{"escalation_reason", "confidence"} {
		t.Run(property+" null tolerated", func(t *testing.T) {
			var object map[string]any
			if err := json.Unmarshal([]byte(validResponseJSON), &object); err != nil {
				t.Fatal(err)
			}
			object[property] = nil
			raw, err := json.Marshal(object)
			if err != nil {
				t.Fatal(err)
			}
			resp, issues := ValidateResponse(string(raw), kb, cat)
			if resp == nil || len(issues) != 0 {
				t.Fatalf("ValidateResponse() = (%+v, %+v), want valid with no issues", resp, issues)
			}
		})
	}
}

func TestValidateResponse_FactPlaceholderFailures(t *testing.T) {
	tests := []struct {
		name   string
		text   string
		mutate func(*KB)
		code   string
	}{
		{
			name: "unknown placeholder",
			text: "Цена {{product.unknown.price}}.",
			code: "unknown_fact_placeholder",
		},
		{
			name: "malformed placeholder",
			text: "Цена {product.coffee-machine.price}.",
			code: "malformed_fact_placeholder",
		},
		{
			name: "stale inactive row",
			text: "Цена {{product.coffee-machine.price}}.",
			mutate: func(kb *KB) {
				kb.Products[0].SalesStatus = "inactive"
			},
			code: "stale_fact_placeholder",
		},
		{
			name: "stale empty value",
			text: "Цена {{product.coffee-machine.price}}.",
			mutate: func(kb *KB) {
				kb.Products[0].Price = " "
			},
			code: "stale_fact_placeholder",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kb := baseKB()
			cat, err := BuildCatalog(kb)
			if err != nil {
				t.Fatal(err)
			}
			if tt.mutate != nil {
				tt.mutate(kb)
			}
			_, issues := ValidateResponse(responseWithText(t, tt.text), kb, cat)
			if !containsCode(issues, tt.code) {
				t.Fatalf("issues = %+v, want %s", issues, tt.code)
			}
		})
	}
}

// Placeholders or braces inside escalation_reason are tolerated (diagnostic-only
// field) and are NEVER substituted — the value is preserved exactly as written.
func TestValidateResponse_PlaceholderInEscalationReasonTolerated(t *testing.T) {
	kb := baseKB()
	cat, err := BuildCatalog(kb)
	if err != nil {
		t.Fatal(err)
	}
	raw := `{
  "reply_text": "Уточню цену.",
  "reply_language": "ru",
  "media_files_to_send": [],
  "escalate": true,
  "escalation_reason": "Нужен {{product.coffee-machine.price}}",
  "confidence": 0.5
}`
	resp, issues := ValidateResponse(raw, kb, cat)
	if resp == nil || len(issues) != 0 {
		t.Fatalf("ValidateResponse() = (%+v, %+v), want valid with no issues", resp, issues)
	}
	if resp.EscalationReason != "Нужен {{product.coffee-machine.price}}" {
		t.Fatalf("escalation_reason must be preserved verbatim (never substituted), got %q", resp.EscalationReason)
	}
}

// Availability wording ("в наличии"/"нет в наличии") is deliberately NOT covered
// here — since in_stock stopped being a fact token (2026-07-26, registry.go), writing
// that phrase in the model's own words is the required, honest behavior (frame-ru.txt
// rules 1a/3), not a literal-value leak.
func TestValidateResponse_RejectsModelAuthoredExactValues(t *testing.T) {
	kb := baseKB()
	cat, err := BuildCatalog(kb)
	if err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{
		"Цена кофемашины — 129 900 ₸.",
	} {
		_, issues := ValidateResponse(responseWithText(t, text), kb, cat)
		if !containsCode(issues, "exact_value_literal") {
			t.Errorf("text %q: issues = %+v, want exact_value_literal", text, issues)
		}
	}
}

// TestValidateResponse_SharedWordingNeverFlaggedAsLiteralLeak is the negative-control
// regression test for evals/SHOP_KB_V1_30_POSTMORTEM.md #7: once a catalog holds
// several products in the SAME stock state, their identical wording ("в наличии") no
// longer uniquely identifies any one of them, so a reply describing that shared state
// in its own words must NOT be flagged — the check can never attribute the phrase to a
// specific fact when several facts produce it. A genuinely unique value (this
// product's own price) in the SAME reply must still be flagged, proving the fix didn't
// just disable the check wholesale.
func TestValidateResponse_SharedWordingNeverFlaggedAsLiteralLeak(t *testing.T) {
	kb := baseKB()
	kb.Products = append(kb.Products,
		Product{Ref: "blender-a", Name: "Блендер A", Price: "9 900 ₸", InStock: true, SalesStatus: "active"},
		Product{Ref: "blender-b", Name: "Блендер B", Price: "10 900 ₸", InStock: true, SalesStatus: "active"},
	)
	cat, err := BuildCatalog(kb)
	if err != nil {
		t.Fatal(err)
	}

	_, issues := ValidateResponse(responseWithText(t, "Кофемашина в наличии."), kb, cat)
	if containsCode(issues, "exact_value_literal") {
		t.Errorf("shared stock wording across 3 in-stock products must not be flagged, got issues = %+v", issues)
	}

	_, issues = ValidateResponse(responseWithText(t, "Цена кофемашины — 129 900 ₸."), kb, cat)
	if !containsCode(issues, "exact_value_literal") {
		t.Errorf("a genuinely unique price literal must still be flagged even in a larger catalog, got issues = %+v", issues)
	}
}

func TestSubstituteFacts_UsesLatestApprovedValue(t *testing.T) {
	kb := baseKB()
	cat, err := BuildCatalog(kb)
	if err != nil {
		t.Fatal(err)
	}
	kb.Products[0].Price = "130 500 ₸"
	out, err := SubstituteFacts("Цена: {{product.coffee-machine.price}}.", kb, cat)
	if err != nil {
		t.Fatal(err)
	}
	if out != "Цена: 130 500 ₸." {
		t.Fatalf("SubstituteFacts() = %q, want latest approved value", out)
	}
}

func TestSubstituteFacts_ExactValueVerbatim(t *testing.T) {
	cat := validCatalog(t)
	out, err := SubstituteFacts("Цена: {{product.coffee-machine.price}}.", baseKB(), cat)
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

func TestResolveSend_RereadsCurrentMediaColumn(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*KB)
		wantID  string
		wantErr bool
	}{
		{
			name: "cleared column is stale",
			mutate: func(kb *KB) {
				kb.Products[0].FeaturedImage = ""
			},
			wantErr: true,
		},
		{
			name: "inactive owner is stale",
			mutate: func(kb *KB) {
				kb.Products[0].SalesStatus = "inactive"
			},
			wantErr: true,
		},
		{
			name: "changed approved column resolves latest material",
			mutate: func(kb *KB) {
				kb.Products[0].FeaturedImage = "m-new-featured"
				kb.Materials = append(kb.Materials, testMaterial("m-new-featured"))
			},
			wantID: "m-new-featured",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kb := baseKB()
			cat, err := BuildCatalog(kb)
			if err != nil {
				t.Fatal(err)
			}
			tt.mutate(kb)
			got, err := ResolveSend([]string{"products.coffee-machine.featured_image"}, kb, cat)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ResolveSend() = %+v, want error", got)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != 1 || got[0].Material.ID != tt.wantID {
				t.Fatalf("ResolveSend() = %+v, want material %q", got, tt.wantID)
			}
		})
	}
}
