package kbimport

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/internal/kbstore"
)

// manifestIndex maps a request-scoped handle ("upload.1", "evidence.2") to
// its real kbd_materials.id — the backend sidecar map plan/mcp.md
// describes for MCP's own handle resolution, built fresh per synthesis run
// (prompt.go) and consumed here before a call ever reaches
// mcpserver.ParseUpsertCall.
type manifestIndex map[string]uuid.UUID

// resolveHandlesInArgs walks changes inside args (a parsed model call's
// {"changes": {...}} object), replacing every recognized media field's
// upload.N handle(s) with the real material_id string(s) they map to, and
// re-encodes the result back into args in place. Returns a non-empty
// dropReason — never an error — because an unresolvable handle is a
// per-call content problem (drop this call, keep processing the rest), not
// a system failure.
func resolveHandlesInArgs(args map[string]json.RawMessage, idx manifestIndex) (dropReason string) {
	changesRaw, ok := args["changes"]
	if !ok {
		return "" // ParseUpsertCall itself rejects a missing changes object
	}
	var changes map[string]json.RawMessage
	if err := json.Unmarshal(changesRaw, &changes); err != nil {
		return "changes is not a JSON object"
	}
	for field, raw := range changes {
		if kbstore.MediaFieldKind(field) == "" {
			continue // not a media field — nothing to resolve
		}
		resolved, reason := resolveMediaValue(raw, idx)
		if reason != "" {
			return fmt.Sprintf("%s: %s", field, reason)
		}
		changes[field] = resolved
	}
	out, err := json.Marshal(changes)
	if err != nil {
		return "internal error re-encoding changes"
	}
	args["changes"] = out
	return ""
}

// resolveMediaValue resolves one media field's raw JSON value — either a
// single handle string, an array of handle strings, or JSON null (a
// legitimate explicit clear, left untouched).
func resolveMediaValue(raw json.RawMessage, idx manifestIndex) (json.RawMessage, string) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || string(trimmed) == "null" {
		return raw, ""
	}
	if trimmed[0] == '[' {
		var handles []string
		if err := json.Unmarshal(raw, &handles); err != nil {
			return nil, "malformed media array"
		}
		ids := make([]string, 0, len(handles))
		for _, h := range handles {
			id, reason := resolveOneHandle(h, idx)
			if reason != "" {
				return nil, reason
			}
			ids = append(ids, id)
		}
		out, err := json.Marshal(ids)
		if err != nil {
			return nil, "internal error encoding resolved handles"
		}
		return out, ""
	}
	var handle string
	if err := json.Unmarshal(raw, &handle); err != nil {
		return nil, "malformed media value"
	}
	id, reason := resolveOneHandle(handle, idx)
	if reason != "" {
		return nil, reason
	}
	out, err := json.Marshal(id)
	if err != nil {
		return nil, "internal error encoding resolved handle"
	}
	return out, ""
}

// resolveOneHandle resolves a single handle string to a material_id. An
// evidence.* handle, an unrecognized handle, or ANYTHING else not present
// in idx — including a bare UUID the model wrote directly, which
// ParseUpsertCall's own optMaterialID/optMaterialIDs would otherwise parse
// without complaint — is rejected. This is what makes a hallucinated or
// foreign-org id structurally impossible to place in a draft media column,
// not merely disallowed by convention.
func resolveOneHandle(handle string, idx manifestIndex) (id string, dropReason string) {
	if strings.HasPrefix(handle, "evidence.") {
		return "", fmt.Sprintf("evidence handle %q cannot be used in a media field", handle)
	}
	if resolved, ok := idx[handle]; ok {
		return resolved.String(), ""
	}
	return "", fmt.Sprintf("unknown or unmapped handle %q", handle)
}
