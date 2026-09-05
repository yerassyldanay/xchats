package aiprompt

import (
	"strings"
	"testing"
)

// --- ValidateResponse (v6) must stay byte-for-byte unaffected -------------

// TestValidateResponse_KBGapIsUnknownProperty pins v6's existing behavior:
// ValidateResponseV7 is a new, separate function specifically so
// ValidateResponse never has to learn about kb_gap. A kb_gap property in v6
// model output is exactly as unrecognized as any other stray property.
func TestValidateResponse_KBGapIsUnknownProperty(t *testing.T) {
	cat := validCatalog(t)
	raw := `{
  "reply_text": "ok",
  "reply_language": "ru",
  "media_files_to_send": [],
  "escalate": false,
  "kb_gap": {"reason_code": "missing_field"}
}`
	resp, issues := ValidateResponse(raw, baseKB(), cat)
	if !containsCode(issues, "unknown_property") {
		t.Fatalf("expected unknown_property issue for kb_gap on the v6 path, got %+v", issues)
	}
	if issueDetail(issues, "unknown_property") != "kb_gap" {
		t.Fatalf("expected unknown_property detail 'kb_gap', got %+v", issues)
	}
	if resp != nil && resp.KBGap != nil {
		t.Fatalf("ValidateResponse (v6) must never populate KBGap, got %+v", resp.KBGap)
	}
}

// --- ValidateResponseV7: operational strictness unchanged -----------------

func TestValidateResponseV7_ValidWithoutKBGap(t *testing.T) {
	cat := validCatalog(t)
	resp, issues := ValidateResponseV7(validResponseJSON, baseKB(), cat)
	if len(issues) != 0 {
		t.Fatalf("unexpected issues: %+v", issues)
	}
	if resp == nil {
		t.Fatal("expected a parsed response")
	}
	if resp.KBGap != nil {
		t.Fatalf("no kb_gap in input must leave KBGap nil, got %+v", resp.KBGap)
	}
}

func TestValidateResponseV7_MissingOperationalFieldStillAnIssue(t *testing.T) {
	cat := validCatalog(t)
	raw := `{
  "reply_text": "ok",
  "reply_language": "ru",
  "media_files_to_send": [],
  "kb_gap": {"reason_code": "other"}
}` // escalate omitted
	_, issues := ValidateResponseV7(raw, baseKB(), cat)
	if !containsCode(issues, "missing_property") {
		t.Fatalf("expected missing_property issue, got %+v", issues)
	}
}

func TestValidateResponseV7_UnknownPropertyStillCaught(t *testing.T) {
	cat := validCatalog(t)
	raw := `{
  "reply_text": "ok",
  "reply_language": "ru",
  "media_files_to_send": [],
  "escalate": false,
  "not_a_real_property": true
}`
	_, issues := ValidateResponseV7(raw, baseKB(), cat)
	if !containsCode(issues, "unknown_property") {
		t.Fatalf("expected unknown_property issue, got %+v", issues)
	}
}

// --- kb_gap: valid, trusted diagnostic --------------------------------

func TestValidateResponseV7_ValidKBGapMissingField(t *testing.T) {
	cat := validCatalog(t)
	raw := `{
  "reply_text": "Секунду, уточню.",
  "reply_language": "ru",
  "media_files_to_send": [],
  "escalate": true,
  "escalation_reason": "нет цены",
  "kb_gap": {
    "reason_code": "missing_field",
    "target_entity_type": "product",
    "target_entity_ref": "coffee-machine",
    "missing_fields": ["warranty_terms"]
  }
}`
	resp, issues := ValidateResponseV7(raw, baseKB(), cat)
	if len(issues) != 0 {
		t.Fatalf("unexpected issues: %+v", issues)
	}
	if resp.KBGap == nil {
		t.Fatal("expected a populated KBGap")
	}
	if resp.KBGap.ReasonCode != KBGapReasonMissingField {
		t.Errorf("ReasonCode = %q, want %q", resp.KBGap.ReasonCode, KBGapReasonMissingField)
	}
	if resp.KBGap.TargetEntityType != KBGapEntityProduct || resp.KBGap.TargetEntityRef != "coffee-machine" {
		t.Errorf("target = %q/%q, want product/coffee-machine", resp.KBGap.TargetEntityType, resp.KBGap.TargetEntityRef)
	}
	if len(resp.KBGap.MissingFields) != 1 || resp.KBGap.MissingFields[0] != "warranty_terms" {
		t.Errorf("MissingFields = %v, want [warranty_terms]", resp.KBGap.MissingFields)
	}
	if resp.KBGap.Source != KBGapSourceModel {
		t.Errorf("Source = %q, want %q", resp.KBGap.Source, KBGapSourceModel)
	}
}

// --- kb_gap: every sanitization case must NEVER become a ContractIssue ---

func TestValidateResponseV7_InvalidKBGapNeverInvalidatesReply(t *testing.T) {
	cat := validCatalog(t)
	baseFields := `
  "reply_text": "ok",
  "reply_language": "ru",
  "media_files_to_send": [],
  "escalate": true,`

	cases := map[string]string{
		"unknown reason code":                 `{` + baseFields + `"kb_gap": {"reason_code": "not_a_real_code"}}`,
		"engine_error from model is rejected": `{` + baseFields + `"kb_gap": {"reason_code": "engine_error"}}`,
		"kb_gap is a string, not object":      `{` + baseFields + `"kb_gap": "missing_field"}`,
		"kb_gap is null":                      `{` + baseFields + `"kb_gap": null}`,
		"kb_gap is empty object":              `{` + baseFields + `"kb_gap": {}}`,
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			resp, issues := ValidateResponseV7(raw, baseKB(), cat)
			if len(issues) != 0 {
				t.Fatalf("%s: malformed kb_gap must never produce a ContractIssue, got %+v", name, issues)
			}
			if resp == nil {
				t.Fatalf("%s: expected a parsed response", name)
			}
			if resp.KBGap != nil {
				t.Errorf("%s: expected KBGap dropped entirely, got %+v", name, resp.KBGap)
			}
		})
	}
}

// TestValidateResponseV7_InventedTargetDroppedReasonCodeKept covers "never
// accept invented refs as authoritative": the ref/type themselves must never
// be trusted, but the reason code alone is still useful aggregate telemetry
// ("a missing_field escalation happened") even when we cannot trust which
// entity the model meant — so sanitizeKBGap clears only the target, not the
// whole diagnostic.
func TestValidateResponseV7_InventedTargetDroppedReasonCodeKept(t *testing.T) {
	cat := validCatalog(t)
	cases := map[string]string{
		"invented product ref": `{"reply_text":"ok","reply_language":"ru","media_files_to_send":[],"escalate":true,` +
			`"kb_gap": {"reason_code": "missing_field", "target_entity_type": "product", "target_entity_ref": "invented-ref", "missing_fields": ["price"]}}`,
		"invented entity type": `{"reply_text":"ok","reply_language":"ru","media_files_to_send":[],"escalate":true,` +
			`"kb_gap": {"reason_code": "missing_field", "target_entity_type": "spaceship", "target_entity_ref": "coffee-machine"}}`,
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			resp, issues := ValidateResponseV7(raw, baseKB(), cat)
			if len(issues) != 0 {
				t.Fatalf("%s: unexpected issues: %+v", name, issues)
			}
			if resp.KBGap == nil || resp.KBGap.ReasonCode != KBGapReasonMissingField {
				t.Fatalf("%s: expected reason_code kept, got %+v", name, resp.KBGap)
			}
			if resp.KBGap.TargetEntityType != "" || resp.KBGap.TargetEntityRef != "" || len(resp.KBGap.MissingFields) != 0 {
				t.Errorf("%s: expected the untrusted target and its fields cleared, got %+v", name, resp.KBGap)
			}
		})
	}
}

func TestValidateResponseV7_InventedFieldNameDroppedNotWholeGap(t *testing.T) {
	cat := validCatalog(t)
	raw := `{
  "reply_text": "ok",
  "reply_language": "ru",
  "media_files_to_send": [],
  "escalate": true,
  "kb_gap": {
    "reason_code": "missing_field",
    "target_entity_type": "product",
    "target_entity_ref": "coffee-machine",
    "missing_fields": ["price", "working_pressure_bar"]
  }
}`
	resp, issues := ValidateResponseV7(raw, baseKB(), cat)
	if len(issues) != 0 {
		t.Fatalf("unexpected issues: %+v", issues)
	}
	if resp.KBGap == nil {
		t.Fatal("expected KBGap to survive with the valid field kept")
	}
	if len(resp.KBGap.MissingFields) != 1 || resp.KBGap.MissingFields[0] != "price" {
		t.Errorf("MissingFields = %v, want only [price] (working_pressure_bar is not a real column)", resp.KBGap.MissingFields)
	}
}

func TestValidateResponseV7_ReasonCodeAloneKeptWithoutTarget(t *testing.T) {
	cat := validCatalog(t)
	raw := `{
  "reply_text": "ok",
  "reply_language": "ru",
  "media_files_to_send": [],
  "escalate": true,
  "kb_gap": {"reason_code": "unsupported_request"}
}`
	resp, issues := ValidateResponseV7(raw, baseKB(), cat)
	if len(issues) != 0 {
		t.Fatalf("unexpected issues: %+v", issues)
	}
	if resp.KBGap == nil || resp.KBGap.ReasonCode != KBGapReasonUnsupportedRequest {
		t.Fatalf("expected a bare unsupported_request diagnostic, got %+v", resp.KBGap)
	}
	if resp.KBGap.TargetEntityType != "" || resp.KBGap.TargetEntityRef != "" {
		t.Errorf("expected no target, got %+v", resp.KBGap)
	}
}

func TestValidateResponseV7_TariffInfoAndTopicAllowlistsAreClosed(t *testing.T) {
	cat := validCatalog(t)
	kb := baseKB()
	kb.TariffInfo = &TariffInfo{}
	raw := `{
  "reply_text": "ok",
  "reply_language": "ru",
  "media_files_to_send": [],
  "escalate": true,
  "kb_gap": {
    "reason_code": "missing_field",
    "target_entity_type": "tariff_info",
    "target_entity_ref": "main",
    "missing_fields": ["trial_period_days"]
  }
}`
	resp, issues := ValidateResponseV7(raw, kb, cat)
	if len(issues) != 0 {
		t.Fatalf("unexpected issues: %+v", issues)
	}
	if resp.KBGap == nil || resp.KBGap.TargetEntityType != KBGapEntityTariffInfo {
		t.Fatalf("expected tariff_info target to survive (it is a real singleton), got %+v", resp.KBGap)
	}
	if len(resp.KBGap.MissingFields) != 0 {
		t.Errorf("tariff_info has no fixed fields — expected missing_fields dropped, got %v", resp.KBGap.MissingFields)
	}
}

// --- Schema shape ----------------------------------------------------------

func TestResponseJSONSchemaV7_AddsKBGapWithoutMutatingV6(t *testing.T) {
	v6 := ResponseJSONSchema()
	v7 := ResponseJSONSchemaV7()

	v6Props := v6["properties"].(map[string]any)
	if _, ok := v6Props["kb_gap"]; ok {
		t.Fatal("ResponseJSONSchema (v6) must never gain a kb_gap property")
	}
	v7Props := v7["properties"].(map[string]any)
	if _, ok := v7Props["kb_gap"]; !ok {
		t.Fatal("ResponseJSONSchemaV7 must expose kb_gap")
	}

	v6Required := v6["required"].([]string)
	v7Required := v7["required"].([]string)
	if len(v6Required) != len(v7Required) {
		t.Fatalf("kb_gap must stay optional: required lists differ: v6=%v v7=%v", v6Required, v7Required)
	}
	for _, name := range v7Required {
		if name == "kb_gap" {
			t.Fatal("kb_gap must not be a required property")
		}
	}
}

// TestBuildPromptV7_RendersKBGapSchemaEndToEnd closes the loop between the
// v7 frame text (frame.go) and RenderPromptV7 (prompt.go): the full rendered
// prompt a v7-configured Engine.Generate would actually send must mention
// kb_gap exactly once (from the substituted schema), while the same KB
// rendered through the unchanged v6 path must never mention it at all.
func TestBuildPromptV7_RendersKBGapSchemaEndToEnd(t *testing.T) {
	kb := baseKB()

	v7, _, err := BuildPromptV7(FrameShopKBV7RU(), kb)
	if err != nil {
		t.Fatalf("BuildPromptV7: %v", err)
	}
	if !strings.Contains(v7, "kb_gap") {
		t.Fatal("v7 rendered prompt must mention kb_gap (frame rule 9 + schema property)")
	}

	v6, _, err := BuildPrompt(FrameShopKBV6RU(), kb)
	if err != nil {
		t.Fatalf("BuildPrompt (v6): %v", err)
	}
	if strings.Contains(v6, "kb_gap") {
		t.Fatal("v6 rendered prompt must never mention kb_gap")
	}
}

func TestRenderResponseSchemaV7_IsValidJSONAndMentionsKBGap(t *testing.T) {
	rendered := RenderResponseSchemaV7()
	if !strings.Contains(rendered, `"kb_gap"`) {
		t.Fatal("rendered v7 schema must mention kb_gap")
	}
	// RenderResponseSchema (v6) must stay byte-identical to before — no
	// kb_gap leaking into the frozen v4/v5/v6 prompt path.
	if strings.Contains(RenderResponseSchema(), `"kb_gap"`) {
		t.Fatal("RenderResponseSchema (v6) must never mention kb_gap")
	}
}
