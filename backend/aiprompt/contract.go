package aiprompt

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// Response is the canonical customer-response JSON contract
// (DECISIONS.md §"Customer-response JSON contract"). There is exactly one
// media field, media_files_to_send; the legacy names asset_refs,
// attach_groups, and send are not aliases and must not appear anywhere.
type Response struct {
	ReplyText        string   `json:"reply_text"`
	ReplyLanguage    string   `json:"reply_language"`
	MediaFilesToSend []string `json:"media_files_to_send"`
	Escalate         bool     `json:"escalate"`
	EscalationReason string   `json:"escalation_reason"`
	Confidence       float64  `json:"confidence"`
}

// propertyDescriptions are part of the contract, not optional comments; they
// are rendered verbatim into the JSON Schema the model sees.
var propertyDescriptions = []struct{ Name, Type, Description string }{
	{"reply_text", "string", "Russian customer-facing reply. Exact business values must be represented by approved placeholders, never model-written literals."},
	{"reply_language", "string", "Language of reply_text; the only allowed v1 value is \"ru\"."},
	{"media_files_to_send", "array of strings", "Ordered semantic tokens copied exactly from the media catalog. An empty array means no media."},
	{"escalate", "boolean", "true when approved live knowledge is insufficient and human review is required."},
	{"escalation_reason", "string", "Internal Russian reason for escalation; empty when escalate is false and never shown to the customer."},
	{"confidence", "number", "Model confidence from 0 to 1; informational only and never a safety gate."},
}

// ResponseJSONSchema returns the strict JSON Schema for the contract
// (additionalProperties rejected, every property required and described).
func ResponseJSONSchema() map[string]any {
	props := map[string]any{}
	required := make([]string, 0, len(propertyDescriptions))
	for _, p := range propertyDescriptions {
		required = append(required, p.Name)
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
	Code   string // parse | unknown_property | missing_property | bad_language | bad_confidence | unknown_media_token | escalation_reason_mismatch
	Detail string
}

var jsonExtract = regexp.MustCompile(`(?s)\{.*\}`)

// ValidateResponse strictly parses raw model output against the contract and
// the catalog. Unknown JSON properties are rejected. Media tokens must be
// exact media-catalog tokens. Returns the parsed response (when parseable)
// plus every issue found.
func ValidateResponse(raw string, cat *Catalog) (*Response, []ContractIssue) {
	var issues []ContractIssue
	blob := jsonExtract.FindString(raw)
	if blob == "" {
		return nil, []ContractIssue{{Code: "parse", Detail: "no JSON object found in output"}}
	}

	// Pass 1: generic map to detect unknown/missing properties explicitly.
	var m map[string]json.RawMessage
	if err := json.Unmarshal([]byte(blob), &m); err != nil {
		return nil, []ContractIssue{{Code: "parse", Detail: err.Error()}}
	}
	known := map[string]bool{}
	for _, p := range propertyDescriptions {
		known[p.Name] = true
		if _, ok := m[p.Name]; !ok {
			issues = append(issues, ContractIssue{Code: "missing_property", Detail: p.Name})
		}
	}
	for k := range m {
		if !known[k] {
			issues = append(issues, ContractIssue{Code: "unknown_property", Detail: k})
		}
	}

	// Pass 2: typed strict decode.
	dec := json.NewDecoder(bytes.NewReader([]byte(blob)))
	dec.DisallowUnknownFields()
	var resp Response
	if err := dec.Decode(&resp); err != nil {
		issues = append(issues, ContractIssue{Code: "parse", Detail: err.Error()})
		return nil, issues
	}

	if resp.ReplyLanguage != "ru" {
		issues = append(issues, ContractIssue{Code: "bad_language", Detail: fmt.Sprintf("reply_language %q; only \"ru\" is allowed in v1", resp.ReplyLanguage)})
	}
	if resp.Confidence < 0 || resp.Confidence > 1 {
		issues = append(issues, ContractIssue{Code: "bad_confidence", Detail: fmt.Sprintf("%v", resp.Confidence)})
	}
	if !resp.Escalate && strings.TrimSpace(resp.EscalationReason) != "" {
		issues = append(issues, ContractIssue{Code: "escalation_reason_mismatch", Detail: "escalation_reason must be empty when escalate is false"})
	}
	if cat != nil {
		for _, tok := range resp.MediaFilesToSend {
			if cat.MediaByToken(tok) == nil {
				issues = append(issues, ContractIssue{Code: "unknown_media_token", Detail: tok})
			}
		}
	}
	return &resp, issues
}

var placeholderPattern = regexp.MustCompile(`\{\{[^{}]*\}\}`)

// SubstituteFacts replaces every {{…}} placeholder in text with its stored
// value; the in_stock flag substitutes the reviewed Russian wording, never the
// boolean literal. An unknown placeholder is a hard error — a made-up
// placeholder fails loudly, blocking the whole reply.
func SubstituteFacts(text string, cat *Catalog) (string, error) {
	var badTok string
	out := placeholderPattern.ReplaceAllStringFunc(text, func(tok string) string {
		f := cat.FactByToken(tok)
		if f == nil {
			if badTok == "" {
				badTok = tok
			}
			return tok
		}
		if f.Kind == KindStockFlag {
			return StockWordingRU[f.stockValue]
		}
		return f.Value
	})
	if badTok != "" {
		return "", fmt.Errorf("aiprompt: unknown placeholder %s", badTok)
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
	seen := map[string]bool{}
	var out []ResolvedMaterial
	for _, tok := range tokens {
		if seen[tok] {
			continue // deduplicate while preserving order
		}
		seen[tok] = true
		entry := cat.MediaByToken(tok)
		if entry == nil {
			return nil, fmt.Errorf("aiprompt: media token %q is not in the catalog", tok)
		}
		spec, err := mediaColumnSpec(entry.Table, entry.Column)
		if err != nil {
			return nil, err
		}
		for _, id := range cat.resolution[tok].MaterialIDs {
			if err := validateMaterialRef(kb, id, spec, entry.Table, entry.Ref); err != nil {
				return nil, err
			}
			out = append(out, ResolvedMaterial{Token: tok, Material: *kb.MaterialByID(id)})
		}
	}
	return out, nil
}
