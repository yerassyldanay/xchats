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
	{"reply_text", "string", "Customer-facing reply (Russian, or Kazakh when the customer wrote Kazakh). Exact business values must be represented by approved placeholders, never model-written literals.", false},
	{"reply_language", "string", "Language of reply_text: \"ru\" or \"kk\".", false},
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
			schema["enum"] = []string{"ru", "kk"}
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

	if resp.ReplyLanguage != "ru" && resp.ReplyLanguage != "kk" {
		issues = append(issues, ContractIssue{Code: "bad_language", Detail: fmt.Sprintf("reply_language %q; only \"ru\" or \"kk\" is allowed", resp.ReplyLanguage)})
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

	// Media references belong ONLY in media_files_to_send, never as literal text
	// inside reply_text (the v2 canonical-block frame states this explicitly as an
	// omission-semantics rule) — a valid token written into customer-facing prose is
	// still a leak of an internal reference string, exactly the same class of mistake
	// as a model-authored exact value (see the literal-leak check below), just for
	// media instead of facts.
	for _, m := range cat.Media {
		if strings.Contains(resp.ReplyText, m.Token) {
			issues = append(issues, ContractIssue{
				Code:   "media_token_in_reply_text",
				Detail: "reply_text contains media reference " + m.Token + " as literal text",
			})
		}
	}

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
		if _, err := ResolveFactLang(tok, kb, cat, resp.ReplyLanguage); err != nil {
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

	// Literal-value leak detection only fires for a value that UNIQUELY identifies one
	// fact across the whole catalog. A resolved value shared by two or more distinct
	// fact tokens (the fixed, small-vocabulary delivery-availability wording is
	// identical for every zone in that state, e.g. every delivering zone resolves to
	// the same "доставляем") cannot be attributed to any specific fact if it appears
	// in the reply — the model may simply have described that shared state honestly in
	// its own words, not leaked one entity's specific business value. Confirmed false
	// positive (originally observed on in_stock wording, before it was removed as a
	// fact token entirely — see registry.go): evals/SHOP_KB_V1_30_POSTMORTEM.md #7
	// ("в наличии" collided across unrelated products once the catalog held enough of
	// them). Checking only unique
	// values is deliberately conservative — it can miss a genuine coincidental
	// collision (two products priced identically) — but under-flagging a rare
	// coincidence is the safer tradeoff against a confirmed, repeated failure mode.
	type resolvedFact struct{ token, value string }
	var resolved []resolvedFact
	valueTokens := map[string][]string{} // normalized value -> every distinct token producing it
	for _, fact := range cat.Facts {
		value, err := ResolveFactLang(fact.Token, kb, cat, resp.ReplyLanguage)
		if err != nil {
			continue // an unrelated stale fact does not invalidate this response
		}
		normalizedValue := normalizeLiteral(value)
		if normalizedValue == "" {
			continue
		}
		resolved = append(resolved, resolvedFact{token: fact.Token, value: value})
		valueTokens[normalizedValue] = append(valueTokens[normalizedValue], fact.Token)
	}
	checked := map[string]bool{}
	for _, rf := range resolved {
		normalizedValue := normalizeLiteral(rf.value)
		if checked[normalizedValue] {
			continue
		}
		checked[normalizedValue] = true
		if len(valueTokens[normalizedValue]) > 1 {
			continue // shared/non-distinguishing value — cannot attribute a leak to one fact
		}
		if containsLiteral(withoutPlaceholders, rf.value) {
			issues = append(issues, ContractIssue{
				Code:   "exact_value_literal",
				Detail: "reply_text contains a model-authored exact value represented by " + rf.token,
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
// column, using the native-reviewed Russian wording for delivery-availability
// booleans. The returned value is final customer wording and is never model-facing.
// ResolveFactLang is the language-aware counterpart (2026-07 Kazakh customer-message
// testing) — every other exact value (price, a number, a complete string) is
// language-neutral, so only the categorical wording lookup differs between the two.
func ResolveFact(token string, kb *KB, cat *Catalog) (string, error) {
	return ResolveFactLang(token, kb, cat, "ru")
}

// ResolveFactLang is ResolveFact's language-aware counterpart: lang selects the
// delivery-availability wording table ("kk" selects the DRAFT Kazakh table, anything
// else selects the native-reviewed Russian one — see deliveryWording in registry.go).
// Every other resolved value is unaffected by lang.
func ResolveFactLang(token string, kb *KB, cat *Catalog, lang string) (string, error) {
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
	// A virtual fact's Column is a seller-authored ref, not a code-owned
	// registry column — factColumnSpec's closed lookup does not (and must
	// not) know about it; resolveVirtualFact (via currentFactValue) is its
	// own sanity check (a stale/tampered Kind fails there instead).
	if !isVirtualKind(fact.Kind) {
		spec, err := factColumnSpec(fact.Table, fact.Column)
		if err != nil {
			return "", err
		}
		if fact.Kind != spec.Kind {
			return "", fmt.Errorf("aiprompt: fact token %q has value kind %q, want %q", token, fact.Kind, spec.Kind)
		}
	}
	return currentFactValue(kb, fact, lang)
}

func currentFactValue(kb *KB, fact *FactEntry, lang string) (string, error) {
	var value string
	switch fact.Table {
	case "product":
		product := currentProduct(kb, fact.Ref)
		if product == nil {
			return "", fmt.Errorf("aiprompt: fact token %q no longer has an active, available product row", fact.Token)
		}
		if isVirtualKind(fact.Kind) {
			return resolveVirtualFact(product.AdditionalFacts, fact, lang)
		}
		switch fact.Column {
		case "price":
			value = product.Price
		}
	case "tariff":
		tariff := currentTariff(kb, fact.Ref)
		if tariff == nil {
			return "", fmt.Errorf("aiprompt: fact token %q no longer has an active tariff row", fact.Token)
		}
		if isVirtualKind(fact.Kind) {
			return resolveVirtualFact(tariff.AdditionalFacts, fact, lang)
		}
		switch fact.Column {
		case "price":
			value = tariff.Price
		case "fee":
			value = tariff.Fee
		}
	case "tariff_info":
		if fact.Ref != SingletonRef || kb.TariffInfo == nil {
			return "", fmt.Errorf("aiprompt: fact token %q no longer has a tariff_info row", fact.Token)
		}
		return resolveVirtualFact(kb.TariffInfo.AdditionalFacts, fact, lang)
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
			return deliveryWording(lang)[zone.DeliveryAvailable], nil
		}
	}
	if strings.TrimSpace(value) == "" {
		return "", fmt.Errorf("aiprompt: fact token %q now resolves to an empty value", fact.Token)
	}
	return value, nil
}

// currentProduct re-reads the CURRENT product row for ref, applying the
// same visibility rule BuildCatalog used to decide whether to emit a token
// for it in the first place (productVisible, catalog.go): active AND not
// unavailable. Substitution must apply the identical rule the catalog
// applied, not just "still active" — a product that went unavailable
// between catalog build and substitution must fail closed exactly as one
// that was already unavailable at build time never got a token at all.
func currentProduct(kb *KB, ref string) *Product {
	for i := range kb.Products {
		if kb.Products[i].Ref == ref && productVisible(&kb.Products[i]) {
			return &kb.Products[i]
		}
	}
	return nil
}

// resolveVirtualFact re-reads the CURRENT approved value of a virtual fact
// (facts.go's AdditionalFact) by ref within facts — the owning entity's
// live AdditionalFacts list at the moment of substitution, never the
// catalog's own (deliberately value-free) FactEntry. Missing, malformed,
// or type-changed ("stale": a fact edited to a different JSON scalar type
// since the prompt was generated) facts fail closed, matching every other
// ResolveFactLang error path.
func resolveVirtualFact(facts []AdditionalFact, fact *FactEntry, lang string) (string, error) {
	for _, f := range facts {
		if f.Ref != fact.Column {
			continue
		}
		text, kind, ok := valueKindAndText(f.Value)
		if !ok {
			return "", fmt.Errorf("aiprompt: fact token %q now has a malformed value", fact.Token)
		}
		if kind != fact.Kind {
			return "", fmt.Errorf("aiprompt: fact token %q changed type since the prompt was generated", fact.Token)
		}
		if kind == KindVirtualBoolean {
			b, _ := f.Value.(bool)
			return factBoolWording(lang)[b], nil
		}
		if strings.TrimSpace(text) == "" {
			return "", fmt.Errorf("aiprompt: fact token %q now resolves to an empty value", fact.Token)
		}
		return text, nil
	}
	return "", fmt.Errorf("aiprompt: fact token %q no longer has a matching additional fact", fact.Token)
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

// SubstituteFacts replaces every approved placeholder with the latest current value,
// using the native-reviewed Russian wording for stock/delivery-availability booleans.
// Unknown, malformed, inactive, or now-empty facts fail the complete substitution.
// SubstituteFactsLang is the language-aware counterpart (2026-07 Kazakh customer-
// message testing) — see ResolveFactLang.
func SubstituteFacts(text string, kb *KB, cat *Catalog) (string, error) {
	return SubstituteFactsLang(text, kb, cat, "ru")
}

// SubstituteFactsLang is SubstituteFacts' language-aware counterpart: lang selects
// which wording table a stock/delivery-availability placeholder resolves through
// (see ResolveFactLang); every other value is unaffected.
func SubstituteFactsLang(text string, kb *KB, cat *Catalog, lang string) (string, error) {
	var resolveErr error
	out := placeholderPattern.ReplaceAllStringFunc(text, func(tok string) string {
		if resolveErr != nil {
			return tok
		}
		value, err := ResolveFactLang(tok, kb, cat, lang)
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
//
// Deduplicates by both TOKEN and resolved MATERIAL ID: two distinct tokens
// can resolve to the same file (most commonly featured_image's fallback —
// see resolvedFeaturedImage — landing on the same material as that type's
// primary image array's first entry), and sending the identical bytes twice
// in one reply is never correct, so the second token contributes nothing
// further once its material has already been included under an earlier one.
func ResolveSend(tokens []string, kb *KB, cat *Catalog) ([]ResolvedMaterial, error) {
	if kb == nil || cat == nil {
		return nil, fmt.Errorf("aiprompt: current KB and request catalog are required")
	}
	seenToken := map[string]bool{}
	seenMaterial := map[string]bool{}
	out := []ResolvedMaterial{}
	for _, tok := range tokens {
		if seenToken[tok] {
			continue // deduplicate while preserving order
		}
		seenToken[tok] = true
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
			if seenMaterial[id] {
				continue
			}
			seenMaterial[id] = true
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
