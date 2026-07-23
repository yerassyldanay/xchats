package aiprompt

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Response is the canonical customer-response JSON contract
// (DECISIONS.md §"Customer-response JSON contract"). There is exactly one
// media field, media_files_to_send; the legacy names asset_refs,
// attach_groups, and send are not aliases and must not appear anywhere.
//
// The four OPERATIONAL fields (reply_text, reply_language, media_files_to_send,
// escalate) are the contract: they decide what the customer sees and whether a
// human takes over, and they are validated strictly. escalation_reason and
// confidence are DIAGNOSTIC-only telemetry — optional, best-effort decoded,
// never shown to the customer, never substituted into, and never a reason to
// reject an otherwise valid operational response (a malformed diagnostic value
// stays visible in the preserved raw output).
type Response struct {
	ReplyText        string   `json:"reply_text"`
	ReplyLanguage    string   `json:"reply_language"`
	MediaFilesToSend []string `json:"media_files_to_send"`
	Escalate         bool     `json:"escalate"`
	// EscalationReason and Confidence are diagnostics: zero values mean
	// "absent or unparseable", which is fine — logging only, never a gate.
	EscalationReason string  `json:"escalation_reason"`
	Confidence       float64 `json:"confidence"`
}

// propertyDescriptions are part of the contract, not optional comments; they
// are rendered verbatim into the JSON Schema the model sees. Diagnostic
// properties may still be requested from the model (useful for debugging) but
// are optional and never affect whether a response is accepted.
var propertyDescriptions = []struct {
	Name, Type, Description string
	Diagnostic              bool
}{
	{"reply_text", "string", "Russian customer-facing reply. Exact business values must be represented by approved placeholders, never model-written literals.", false},
	{"reply_language", "string", "Language of reply_text; the only allowed v1 value is \"ru\".", false},
	{"media_files_to_send", "array of strings", "Ordered semantic tokens copied exactly from the media catalog. An empty array means no media.", false},
	{"escalate", "boolean", "true when approved live knowledge is insufficient and human review is required.", false},
	{"escalation_reason", "string", "Optional internal Russian reason for escalation; diagnostic only, never shown to the customer.", true},
	{"confidence", "number", "Optional model confidence from 0 to 1; diagnostic only and never a safety gate.", true},
}

// ResponseJSONSchema returns the JSON Schema for the contract
// (additionalProperties rejected, every property described). Only the four
// operational properties are required; the diagnostic properties
// (escalation_reason, confidence) may still be returned for debugging.
func ResponseJSONSchema() map[string]any {
	props := map[string]any{}
	required := make([]string, 0, len(propertyDescriptions))
	for _, p := range propertyDescriptions {
		if !p.Diagnostic {
			required = append(required, p.Name)
		}
		schema := map[string]any{"description": p.Description}
		switch p.Name {
		case "media_files_to_send":
			schema["type"] = "array"
			schema["items"] = map[string]any{"type": "string"}
		case "escalate":
			schema["type"] = "boolean"
		case "confidence":
			schema["type"] = "number"
			schema["minimum"] = 0
			schema["maximum"] = 1
		case "reply_language":
			schema["type"] = "string"
			schema["enum"] = []string{"ru"}
		default:
			schema["type"] = "string"
		}
		props[p.Name] = schema
	}
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             required,
		"properties":           props,
	}
}

// RenderResponseSchema renders the schema as indented JSON for the
// %%RESPONSE_SCHEMA%% prompt slot.
func RenderResponseSchema() string {
	b, err := json.MarshalIndent(ResponseJSONSchema(), "", "  ")
	if err != nil {
		panic(err) // static value; cannot fail
	}
	return string(b)
}

// ContractIssue is one validation failure; an empty slice means valid.
type ContractIssue struct {
	// Code is a stable machine-readable category. Shape codes are parse,
	// validation_context, unknown_property, missing_property, and
	// wrong_property_type; remaining codes describe semantic contract failures.
	Code   string
	Detail string
}

var placeholderPattern = regexp.MustCompile(`\{\{[^{}]*\}\}`)

// Markdown code fences around the JSON object are transport noise, not contract
// content: most models emit ```json fences even when the prompt says "strictly
// JSON", and both prod and the eval harness must grade the same object the model
// produced. Same semantics as the harness's evaltext.StripFences (which cannot be
// imported here: dependency direction is harness -> backend).
var (
	fenceOpenPattern  = regexp.MustCompile("^\\s*```[a-zA-Z]*\\s*")
	fenceClosePattern = regexp.MustCompile("\\s*```\\s*$")
)

func stripMarkdownFences(raw string) string {
	cleaned := fenceOpenPattern.ReplaceAllString(raw, "")
	return fenceClosePattern.ReplaceAllString(cleaned, "")
}

// ValidateResponse strictly parses one JSON object and validates every semantic token
// against both the catalog supplied to the model and the current approved KB. Exact
// business values must remain placeholders until code-side substitution. A markdown
// code fence around the object is tolerated; everything inside remains strict.
//
// Strictness applies to the OPERATIONAL fields only. The diagnostic fields
// (escalation_reason, confidence) are decoded best-effort and never produce an
// issue: absent, out-of-range, mistyped, or placeholder-bearing diagnostic
// values must not make an otherwise valid customer reply unusable — the
// caller's preserved raw output is the audit trail for malformed diagnostics.
func ValidateResponse(raw string, kb *KB, cat *Catalog) (*Response, []ContractIssue) {
	issues := []ContractIssue{}
	blob := strings.TrimSpace(stripMarkdownFences(raw))
	if blob == "" {
		return nil, []ContractIssue{{Code: "parse", Detail: "empty model output"}}
	}

	// Generic map decode: detects unknown/missing properties explicitly and (like any
	// single json.Unmarshal of the whole blob) rejects trailing content after the object.
	var m map[string]json.RawMessage
	if err := json.Unmarshal([]byte(blob), &m); err != nil {
		return nil, []ContractIssue{{Code: "parse", Detail: err.Error()}}
	}
	if m == nil {
		return nil, []ContractIssue{{Code: "parse", Detail: "top-level JSON value must be an object"}}
	}
	known := map[string]bool{}
	wrongType := false
	for _, p := range propertyDescriptions {
		known[p.Name] = true
		if p.Diagnostic {
			continue // optional, best-effort — never missing_property/wrong_property_type
		}
		rawProperty, ok := m[p.Name]
		if !ok {
			issues = append(issues, ContractIssue{Code: "missing_property", Detail: p.Name})
			continue
		}
		if !responsePropertyTypeOK(p.Name, rawProperty) {
			wrongType = true
			issues = append(issues, ContractIssue{Code: "wrong_property_type", Detail: p.Name})
		}
	}
	for k := range m {
		if !known[k] {
			issues = append(issues, ContractIssue{Code: "unknown_property", Detail: k})
		}
	}

	resp := decodeResponse(m)
	if wrongType {
		return nil, issues
	}

	if resp.ReplyLanguage != "ru" {
		issues = append(issues, ContractIssue{Code: "bad_language", Detail: fmt.Sprintf("reply_language %q; only \"ru\" is allowed in v1", resp.ReplyLanguage)})
	}
	if cat != nil {
		for _, tok := range resp.MediaFilesToSend {
			if cat.MediaByToken(tok) == nil {
				issues = append(issues, ContractIssue{Code: "unknown_media_token", Detail: tok})
			}
		}
	}
	if kb == nil || cat == nil {
		issues = append(issues, ContractIssue{
			Code:   "validation_context",
			Detail: "current KB and request catalog are required",
		})
		return resp, issues
	}
	issues = append(issues, validateFactContract(*resp, kb, cat)...)
	return resp, issues
}

// decodeResponse builds a Response from the already-parsed property map. The
// operational fields' types were verified by the caller (responsePropertyTypeOK),
// so their unmarshals cannot fail; the diagnostic fields are decoded best-effort —
// a mistyped escalation_reason or confidence simply leaves the zero value, and the
// original bytes stay available in the caller's preserved raw output.
func decodeResponse(m map[string]json.RawMessage) *Response {
	resp := &Response{}
	targets := map[string]any{
		"reply_text":          &resp.ReplyText,
		"reply_language":      &resp.ReplyLanguage,
		"media_files_to_send": &resp.MediaFilesToSend,
		"escalate":            &resp.Escalate,
		"escalation_reason":   &resp.EscalationReason,
		"confidence":          &resp.Confidence,
	}
	for name, target := range targets {
		if rawProperty, ok := m[name]; ok {
			_ = json.Unmarshal(rawProperty, target)
		}
	}
	return resp
}

func responsePropertyTypeOK(name string, raw json.RawMessage) bool {
	var value any
	if err := json.Unmarshal(raw, &value); err != nil || value == nil {
		return false
	}
	switch name {
	case "media_files_to_send":
		items, ok := value.([]any)
		if !ok {
			return false
		}
		for _, item := range items {
			if _, ok := item.(string); !ok {
				return false
			}
		}
		return true
	case "escalate":
		_, ok := value.(bool)
		return ok
	case "confidence":
		_, ok := value.(float64)
		return ok
	default:
		_, ok := value.(string)
		return ok
	}
}

func validateFactContract(resp Response, kb *KB, cat *Catalog) []ContractIssue {
	issues := []ContractIssue{}
	seenTokens := map[string]bool{}
	for _, tok := range placeholderPattern.FindAllString(resp.ReplyText, -1) {
		if seenTokens[tok] {
			continue
		}
		seenTokens[tok] = true
		if cat.FactByToken(tok) == nil {
			issues = append(issues, ContractIssue{Code: "unknown_fact_placeholder", Detail: tok})
			continue
		}
		if _, err := ResolveFact(tok, kb, cat); err != nil {
			issues = append(issues, ContractIssue{Code: "stale_fact_placeholder", Detail: err.Error()})
		}
	}

	withoutPlaceholders := placeholderPattern.ReplaceAllString(resp.ReplyText, "")
	if strings.ContainsAny(withoutPlaceholders, "{}") {
		issues = append(issues, ContractIssue{
			Code:   "malformed_fact_placeholder",
			Detail: "reply_text contains unmatched or malformed braces",
		})
	}
	// escalation_reason is diagnostic-only: placeholders or braces inside it are never
	// an issue, are never substituted, and the field is never shown to the customer.

	seenValues := map[string]bool{}
	for _, fact := range cat.Facts {
		value, err := ResolveFact(fact.Token, kb, cat)
		if err != nil {
			continue // an unrelated stale fact does not invalidate this response
		}
		normalizedValue := normalizeLiteral(value)
		if normalizedValue == "" || seenValues[normalizedValue] {
			continue
		}
		seenValues[normalizedValue] = true
		if containsLiteral(withoutPlaceholders, value) {
			issues = append(issues, ContractIssue{
				Code:   "exact_value_literal",
				Detail: "reply_text contains a model-authored exact value represented by " + fact.Token,
			})
		}
	}
	return issues
}

func normalizeLiteral(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(s), " "))
}

func containsLiteral(text, value string) bool {
	haystack := normalizeLiteral(text)
	needle := normalizeLiteral(value)
	if needle == "" {
		return false
	}
	for offset := 0; offset <= len(haystack)-len(needle); {
		relative := strings.Index(haystack[offset:], needle)
		if relative < 0 {
			return false
		}
		start := offset + relative
		end := start + len(needle)
		if literalBoundariesOK(haystack, needle, start, end) {
			return true
		}
		offset = start + 1
	}
	return false
}

func literalBoundariesOK(haystack, needle string, start, end int) bool {
	first, _ := utf8.DecodeRuneInString(needle)
	if isWordRune(first) && start > 0 {
		before, _ := utf8.DecodeLastRuneInString(haystack[:start])
		if isWordRune(before) {
			return false
		}
	}
	last, _ := utf8.DecodeLastRuneInString(needle)
	if isWordRune(last) && end < len(haystack) {
		after, _ := utf8.DecodeRuneInString(haystack[end:])
		if isWordRune(after) {
			return false
		}
	}
	return true
}

func isWordRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsNumber(r)
}

// ResolveFact resolves one request-catalog token against the current approved row and
// column. The returned value is final customer wording and is never model-facing.
func ResolveFact(token string, kb *KB, cat *Catalog) (string, error) {
	if kb == nil || cat == nil {
		return "", fmt.Errorf("aiprompt: current KB and request catalog are required")
	}
	fact := cat.FactByToken(token)
	if fact == nil {
		return "", fmt.Errorf("aiprompt: fact token %q is not in the request catalog", token)
	}
	canonical := "{{" + fact.Table + "." + fact.Ref + "." + fact.Column + "}}"
	if fact.Token != canonical {
		return "", fmt.Errorf("aiprompt: fact token %q has inconsistent catalog metadata", token)
	}
	spec, err := factColumnSpec(fact.Table, fact.Column)
	if err != nil {
		return "", err
	}
	if fact.Kind != spec.Kind {
		return "", fmt.Errorf("aiprompt: fact token %q has value kind %q, want %q", token, fact.Kind, spec.Kind)
	}
	return currentFactValue(kb, fact)
}

func currentFactValue(kb *KB, fact *FactEntry) (string, error) {
	var value string
	switch fact.Table {
	case "product":
		product := currentProduct(kb, fact.Ref)
		if product == nil {
			return "", fmt.Errorf("aiprompt: fact token %q no longer has an active product row", fact.Token)
		}
		switch fact.Column {
		case "price":
			value = product.Price
		case "in_stock":
			return StockWordingRU[product.InStock], nil
		}
	case "tariff":
		tariff := currentTariff(kb, fact.Ref)
		if tariff == nil {
			return "", fmt.Errorf("aiprompt: fact token %q no longer has an active tariff row", fact.Token)
		}
		switch fact.Column {
		case "price":
			value = tariff.Price
		case "fee":
			value = tariff.Fee
		}
	case "contact":
		if fact.Ref != SingletonRef || kb.Contacts == nil {
			return "", fmt.Errorf("aiprompt: fact token %q no longer has a contacts row", fact.Token)
		}
		values := map[string]string{
			"phone": kb.Contacts.Phone, "whatsapp": kb.Contacts.WhatsApp,
			"email": kb.Contacts.Email, "website": kb.Contacts.Website,
			"instagram": kb.Contacts.Instagram, "working_hours": kb.Contacts.WorkingHours,
		}
		value = values[fact.Column]
	case "policy":
		if fact.Ref != SingletonRef || kb.Policies == nil {
			return "", fmt.Errorf("aiprompt: fact token %q no longer has a policies row", fact.Token)
		}
		values := map[string]string{
			"delivery_cost":         kb.Policies.DeliveryCost,
			"delivery_in_days":      kb.Policies.DeliveryInDays,
			"free_delivery_from":    kb.Policies.FreeDeliveryFrom,
			"min_order":             kb.Policies.MinOrder,
			"return_period_in_days": kb.Policies.ReturnPeriodInDays,
			"outside_zones_note":    kb.Policies.OutsideZonesNote,
		}
		value = values[fact.Column]
	case "delivery":
		zone := currentDeliveryZone(kb, fact.Ref)
		if zone == nil {
			return "", fmt.Errorf("aiprompt: fact token %q no longer has an active delivery zone row", fact.Token)
		}
		switch fact.Column {
		case "delivery_cost":
			value = zone.DeliveryCost
		case "delivery_in_days":
			value = zone.DeliveryInDays
		case "delivery_available":
			return DeliveryWordingRU[zone.DeliveryAvailable], nil
		}
	}
	if strings.TrimSpace(value) == "" {
		return "", fmt.Errorf("aiprompt: fact token %q now resolves to an empty value", fact.Token)
	}
	return value, nil
}

func currentProduct(kb *KB, ref string) *Product {
	for i := range kb.Products {
		if kb.Products[i].Ref == ref && active(kb.Products[i].SalesStatus) {
			return &kb.Products[i]
		}
	}
	return nil
}

func currentTariff(kb *KB, ref string) *Tariff {
	for i := range kb.Tariffs {
		if kb.Tariffs[i].Ref == ref && active(kb.Tariffs[i].SalesStatus) {
			return &kb.Tariffs[i]
		}
	}
	return nil
}

func currentDeliveryZone(kb *KB, ref string) *DeliveryZone {
	for i := range kb.DeliveryZones {
		if kb.DeliveryZones[i].Ref == ref && active(kb.DeliveryZones[i].SalesStatus) {
			return &kb.DeliveryZones[i]
		}
	}
	return nil
}

// SubstituteFacts replaces every approved placeholder with the latest current value.
// Unknown, malformed, inactive, or now-empty facts fail the complete substitution.
func SubstituteFacts(text string, kb *KB, cat *Catalog) (string, error) {
	var resolveErr error
	out := placeholderPattern.ReplaceAllStringFunc(text, func(tok string) string {
		if resolveErr != nil {
			return tok
		}
		value, err := ResolveFact(tok, kb, cat)
		if err != nil {
			resolveErr = err
			return tok
		}
		return value
	})
	if resolveErr != nil {
		return "", resolveErr
	}
	if strings.ContainsAny(out, "{}") {
		return "", fmt.Errorf("aiprompt: malformed fact placeholder in reply_text")
	}
	return out, nil
}

// ResolvedMaterial is one material a validated media token would send.
type ResolvedMaterial struct {
	Token    string
	Material Material
}

// ResolveSend is the (fake-or-real) storage-boundary resolver: it turns
// media_files_to_send tokens into the ordered list of materials that would be
// delivered, re-validating each reference fail-closed. Evals use it to assert
// that exactly the right material records would have been sent — without any
// file bytes existing anywhere.
func ResolveSend(tokens []string, kb *KB, cat *Catalog) ([]ResolvedMaterial, error) {
	if kb == nil || cat == nil {
		return nil, fmt.Errorf("aiprompt: current KB and request catalog are required")
	}
	seen := map[string]bool{}
	out := []ResolvedMaterial{}
	for _, tok := range tokens {
		if seen[tok] {
			continue // deduplicate while preserving order
		}
		seen[tok] = true
		entry := cat.MediaByToken(tok)
		if entry == nil {
			return nil, fmt.Errorf("aiprompt: media token %q is not in the catalog", tok)
		}
		canonical := entry.Table + "." + entry.Ref + "." + entry.Column
		if entry.Token != canonical {
			return nil, fmt.Errorf("aiprompt: media token %q has inconsistent catalog metadata", tok)
		}
		spec, err := mediaColumnSpec(entry.Table, entry.Column)
		if err != nil {
			return nil, err
		}
		ids, err := currentMediaIDs(kb, entry)
		if err != nil {
			return nil, err
		}
		if spec.Singular && len(ids) > 1 {
			return nil, fmt.Errorf("aiprompt: singular media token %q now holds %d references", tok, len(ids))
		}
		for _, id := range ids {
			if err := validateMaterialRef(kb, id, spec, entry.Table, entry.Ref); err != nil {
				return nil, err
			}
			out = append(out, ResolvedMaterial{Token: tok, Material: *kb.MaterialByID(id)})
		}
	}
	return out, nil
}

func currentMediaIDs(kb *KB, entry *MediaEntry) ([]string, error) {
	ids := []string{}
	switch entry.Table {
	case "topics":
		for i := range kb.Topics {
			if kb.Topics[i].Slug == entry.Ref {
				ids = topicMedia(&kb.Topics[i])[entry.Column]
				break
			}
		}
	case "products":
		if product := currentProduct(kb, entry.Ref); product != nil {
			ids = productMedia(product)[entry.Column]
		}
	case "tariffs":
		if tariff := currentTariff(kb, entry.Ref); tariff != nil {
			ids = tariffMedia(tariff)[entry.Column]
		}
	case "contacts":
		if entry.Ref == SingletonRef && kb.Contacts != nil {
			ids = contactsMedia(kb.Contacts)[entry.Column]
		}
	case "policies":
		if entry.Ref == SingletonRef && kb.Policies != nil {
			ids = policiesMedia(kb.Policies)[entry.Column]
		}
	}
	if len(ids) == 0 {
		return nil, fmt.Errorf("aiprompt: media token %q no longer has an approved non-empty row and column", entry.Token)
	}
	return ids, nil
}
