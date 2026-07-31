package mcpserver

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/internal/kbstore"
)

// unmarshalParams decodes a JSON-RPC request's params into a typed struct,
// treating an absent/empty params as an empty object rather than an error
// (several MCP methods, e.g. tools/list, are valid with no params at all).
func unmarshalParams(raw json.RawMessage, dst any) error {
	if len(raw) == 0 {
		return nil
	}
	if err := json.Unmarshal(raw, dst); err != nil {
		return fmt.Errorf("invalid params: %w", err)
	}
	return nil
}

// rawObject decodes a JSON object into a map of still-raw member values, so
// callers can tell "key absent" (omitted → unchanged) from "key present with
// JSON null" (explicit clear) from "key present with a value" — the
// three-way distinction plan/mcp.md's common write behavior requires
// ("Omitted fields remain unchanged. Explicit null clears a nullable
// field.") that a plain struct-tagged json.Unmarshal cannot make.
func rawObject(raw json.RawMessage) (map[string]json.RawMessage, error) {
	if len(raw) == 0 {
		return map[string]json.RawMessage{}, nil
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("expected a JSON object: %w", err)
	}
	return m, nil
}

// rejectUnknownFields enforces the additionalProperties:false every tool's
// `changes` schema already declares (tools.go's changesObject) — without
// this, a typo'd field name (e.g. "titel" instead of "title") is silently
// dropped by every optX accessor (none of them read a key they don't know
// about) rather than surfaced to the caller, so the model has no way to
// learn its argument was ignored.
func rejectUnknownFields(m map[string]json.RawMessage, allowed ...string) error {
	known := make(map[string]bool, len(allowed))
	for _, f := range allowed {
		known[f] = true
	}
	for k := range m {
		if !known[k] {
			return fmt.Errorf("unknown field %q (allowed: %s)", k, strings.Join(allowed, ", "))
		}
	}
	return nil
}

func isJSONNull(raw json.RawMessage) bool { return string(raw) == "null" }

// optString: key absent → nil (unchanged); present as null or "" → pointer
// to "" (this schema's live tables use empty-string, not SQL NULL, as the
// "no value" sentinel almost everywhere — see kbstore's DraftTariff etc.);
// present as a string → that value.
func optString(m map[string]json.RawMessage, key string) (*string, error) {
	raw, ok := m[key]
	if !ok {
		return nil, nil
	}
	if isJSONNull(raw) {
		empty := ""
		return &empty, nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, fmt.Errorf("%s: expected a string: %w", key, err)
	}
	return &s, nil
}

// optBool: key absent → nil (unchanged); present → that boolean. These
// booleans (in_stock, delivery_available) are NOT NULL columns with no
// "clear" concept, so an explicit null is a caller error, not a clear.
func optBool(m map[string]json.RawMessage, key string) (*bool, error) {
	raw, ok := m[key]
	if !ok {
		return nil, nil
	}
	if isJSONNull(raw) {
		return nil, fmt.Errorf("%s: cannot be null", key)
	}
	var b bool
	if err := json.Unmarshal(raw, &b); err != nil {
		return nil, fmt.Errorf("%s: expected a boolean: %w", key, err)
	}
	return &b, nil
}

// optInt: key absent → nil (unchanged); present → that integer.
func optInt(m map[string]json.RawMessage, key string) (*int, error) {
	raw, ok := m[key]
	if !ok {
		return nil, nil
	}
	if isJSONNull(raw) {
		return nil, fmt.Errorf("%s: cannot be null", key)
	}
	var n int
	if err := json.Unmarshal(raw, &n); err != nil {
		return nil, fmt.Errorf("%s: expected an integer: %w", key, err)
	}
	return &n, nil
}

// optMaterialID: key absent → nil (unchanged); null → a genuine clear (this
// IS a nullable DB column — featured_image and friends); a string → parsed
// as the material_id UUID a prior kb_media_upload returned.
func optMaterialID(m map[string]json.RawMessage, key string) (**uuid.UUID, error) {
	raw, ok := m[key]
	if !ok {
		return nil, nil
	}
	if isJSONNull(raw) {
		var nilUUID *uuid.UUID
		return &nilUUID, nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, fmt.Errorf("%s: expected a material_id string: %w", key, err)
	}
	id, err := uuid.Parse(s)
	if err != nil {
		return nil, fmt.Errorf("%s: invalid material_id %q: %w", key, s, err)
	}
	idPtr := &id
	return &idPtr, nil
}

// optMaterialIDs: key absent → nil (unchanged); null or [] → replace with an
// empty array; a string array → parsed material_id list, in order.
func optMaterialIDs(m map[string]json.RawMessage, key string) (*[]uuid.UUID, error) {
	raw, ok := m[key]
	if !ok {
		return nil, nil
	}
	if isJSONNull(raw) {
		empty := []uuid.UUID{}
		return &empty, nil
	}
	var strs []string
	if err := json.Unmarshal(raw, &strs); err != nil {
		return nil, fmt.Errorf("%s: expected an array of material_id strings: %w", key, err)
	}
	ids := make([]uuid.UUID, 0, len(strs))
	for _, s := range strs {
		id, err := uuid.Parse(s)
		if err != nil {
			return nil, fmt.Errorf("%s: invalid material_id %q: %w", key, s, err)
		}
		ids = append(ids, id)
	}
	return &ids, nil
}

// optExpectedVersion reads the common expected_draft_version?  top-level
// argument (plan/mcp.md "Common write behavior") shared by every upsert/
// delete tool.
func optExpectedVersion(m map[string]json.RawMessage) (*int64, error) {
	raw, ok := m["expected_draft_version"]
	if !ok || isJSONNull(raw) {
		return nil, nil
	}
	var n int64
	if err := json.Unmarshal(raw, &n); err != nil {
		return nil, fmt.Errorf("expected_draft_version: expected an integer: %w", err)
	}
	return &n, nil
}

// parseProvenance parses the common provenance?  { source_url?,
// material_ids? } argument (plan/mcp.md "Common write behavior") into the
// typed value kbstore.recordProvenance uses to create/tag a kbd_materials
// audit-trail association — never a field on the draft entry itself (Task
// 16 / plan/DECISIONS.md's "business columns only"). Malformed provenance
// (a non-object, or a material_ids entry that isn't a UUID) is a caller
// error returned to the model as invalid params, not silently dropped into
// a default value the way this used to swallow it.
func parseProvenance(m map[string]json.RawMessage) (kbstore.MCPProvenance, error) {
	raw, ok := m["provenance"]
	if !ok || isJSONNull(raw) {
		return kbstore.MCPProvenance{}, nil
	}
	var p struct {
		SourceURL   string   `json:"source_url"`
		MaterialIDs []string `json:"material_ids"`
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		return kbstore.MCPProvenance{}, fmt.Errorf("provenance: expected an object: %w", err)
	}
	ids := make([]uuid.UUID, 0, len(p.MaterialIDs))
	for _, s := range p.MaterialIDs {
		id, err := uuid.Parse(s)
		if err != nil {
			return kbstore.MCPProvenance{}, fmt.Errorf("provenance.material_ids: invalid material_id %q: %w", s, err)
		}
		ids = append(ids, id)
	}
	return kbstore.MCPProvenance{SourceURL: p.SourceURL, MaterialIDs: ids}, nil
}

// stringField reads a plain required/optional top-level string argument
// (ref, slug, type, key, query, source, ...) — never tri-state, since these
// identify WHICH record a tool call targets rather than patch a field on it.
func stringField(m map[string]json.RawMessage, key string) string {
	raw, ok := m[key]
	if !ok || isJSONNull(raw) {
		return ""
	}
	var s string
	_ = json.Unmarshal(raw, &s)
	return s
}

// stringSliceField reads a plain top-level string-array argument (`types`).
func stringSliceField(m map[string]json.RawMessage, key string) []string {
	raw, ok := m[key]
	if !ok || isJSONNull(raw) {
		return nil
	}
	var out []string
	_ = json.Unmarshal(raw, &out)
	return out
}

// intField reads a plain top-level integer argument (`limit`), defaulting to
// 0 (the kbstore layer applies its own default/max clamp).
func intField(m map[string]json.RawMessage, key string) int {
	raw, ok := m[key]
	if !ok || isJSONNull(raw) {
		return 0
	}
	var n int
	_ = json.Unmarshal(raw, &n)
	return n
}
