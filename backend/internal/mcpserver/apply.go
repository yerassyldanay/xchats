package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/internal/kbstore"
)

// UpsertTools is the closed set of tool names ParseUpsertCall accepts — the
// seven typed kb_*_upsert tools, in Tools()' own declared order. kb_delete
// is deliberately ABSENT: a second caller of this seam (internal/kbimport,
// staging draft entries from internet-sourced content) must never be able
// to construct a delete call — there is structurally no code path from
// parsed web/document content to a delete marker, not merely a convention
// this list happens to follow (plan/mcp.md §10).
var UpsertTools = []string{
	toolKBTopicUpsert, toolKBProductUpsert, toolKBTariffUpsert,
	toolKBContactsUpsert, toolKBPoliciesUpsert, toolKBDeliveryZoneUpsert,
	toolKBAssistantUpsert,
}

// UpsertCall is one parsed, not-yet-applied typed-upsert call — the output
// of ParseUpsertCall. A caller resolves every media-field handle in args to
// a real material_id BEFORE calling ParseUpsertCall (internal/kbimport's
// handles.go does this by walking kbstore.MediaFieldKinds()); UpsertCall
// itself carries no handle-resolution logic — args is expected to already
// look exactly like an ordinary MCP kb_*_upsert call's arguments.
type UpsertCall struct {
	// Tool is the tool name this call was parsed from — one of UpsertTools.
	Tool string
	// Key is the record's natural ref/slug exactly as supplied in args ("" —
	// the empty string — when the caller omitted it, meaning "create with a
	// key derived from the title/name"; the real key is known only once
	// Apply returns it in UpsertResult.Key). Always kbstore.NaturalKeyMain
	// for the three singleton tools (assistant/contacts/policies), which
	// take no ref/slug argument at all.
	Key string

	apply func(ctx context.Context, kb *kbstore.Store, orgID, userID uuid.UUID, expectedVersion *int64, prov kbstore.MCPProvenance) (kbstore.UpsertResult, error)
}

// Apply runs this call's kbstore MCPUpsertX method — the exact same write
// path an ordinary MCP tool call goes through (natural-key duplicate
// detection, media-reference validation, gate checks, provenance
// recording), so a draft entry a second caller stages is indistinguishable
// from one an MCP-connected model created by hand. One contract, two
// callers, cannot drift.
//
// prov is silently NOT forwarded for kb_assistant_upsert:
// kbstore.Store.MCPUpsertAssistant takes no provenance parameter at all
// (mcp_write.go) — a persona/guardrails edit has never had a provenance
// concept, so there is nothing to record it into.
func (c UpsertCall) Apply(ctx context.Context, kb *kbstore.Store, orgID, userID uuid.UUID, expectedVersion *int64, prov kbstore.MCPProvenance) (kbstore.UpsertResult, error) {
	return c.apply(ctx, kb, orgID, userID, expectedVersion, prov)
}

// ParseUpsertCall parses one {tool, args} pair — the exact {name, arguments}
// shape dispatchToolsCall itself receives off the wire — into an UpsertCall.
// It reuses the SAME per-field parser (parseTopicChanges, parseProductChanges,
// ...) and the SAME validation (rejectUnknownFields, optString/optMaterialID/
// optMaterialIDs/optBool, enum checks deferred to Apply just like the live
// MCP path) every ordinary handleXUpsert call already goes through — see
// each parser's own doc comment. tool must be one of UpsertTools; anything
// else is an error.
func ParseUpsertCall(tool string, args map[string]json.RawMessage) (UpsertCall, error) {
	changes, err := rawObject(args["changes"])
	if err != nil {
		return UpsertCall{}, fmt.Errorf("changes: %w", err)
	}
	switch tool {
	case toolKBAssistantUpsert:
		ch, err := parseAssistantChanges(changes)
		if err != nil {
			return UpsertCall{}, err
		}
		return UpsertCall{
			Tool: tool, Key: kbstore.NaturalKeyMain,
			apply: func(ctx context.Context, kb *kbstore.Store, orgID, userID uuid.UUID, expectedVersion *int64, _ kbstore.MCPProvenance) (kbstore.UpsertResult, error) {
				return kb.MCPUpsertAssistant(ctx, orgID, userID, ch, expectedVersion)
			},
		}, nil

	case toolKBTopicUpsert:
		ch, err := parseTopicChanges(changes)
		if err != nil {
			return UpsertCall{}, err
		}
		slug := stringField(args, "slug")
		return UpsertCall{
			Tool: tool, Key: slug,
			apply: func(ctx context.Context, kb *kbstore.Store, orgID, userID uuid.UUID, expectedVersion *int64, prov kbstore.MCPProvenance) (kbstore.UpsertResult, error) {
				return kb.MCPUpsertTopic(ctx, orgID, userID, slug, ch, expectedVersion, prov)
			},
		}, nil

	case toolKBProductUpsert:
		ch, err := parseProductChanges(changes)
		if err != nil {
			return UpsertCall{}, err
		}
		ref := stringField(args, "ref")
		return UpsertCall{
			Tool: tool, Key: ref,
			apply: func(ctx context.Context, kb *kbstore.Store, orgID, userID uuid.UUID, expectedVersion *int64, prov kbstore.MCPProvenance) (kbstore.UpsertResult, error) {
				return kb.MCPUpsertProduct(ctx, orgID, userID, ref, ch, expectedVersion, prov)
			},
		}, nil

	case toolKBTariffUpsert:
		ch, err := parseTariffChanges(changes)
		if err != nil {
			return UpsertCall{}, err
		}
		ref := stringField(args, "ref")
		return UpsertCall{
			Tool: tool, Key: ref,
			apply: func(ctx context.Context, kb *kbstore.Store, orgID, userID uuid.UUID, expectedVersion *int64, prov kbstore.MCPProvenance) (kbstore.UpsertResult, error) {
				return kb.MCPUpsertTariff(ctx, orgID, userID, ref, ch, expectedVersion, prov)
			},
		}, nil

	case toolKBContactsUpsert:
		ch, err := parseContactsChanges(changes)
		if err != nil {
			return UpsertCall{}, err
		}
		return UpsertCall{
			Tool: tool, Key: kbstore.NaturalKeyMain,
			apply: func(ctx context.Context, kb *kbstore.Store, orgID, userID uuid.UUID, expectedVersion *int64, prov kbstore.MCPProvenance) (kbstore.UpsertResult, error) {
				return kb.MCPUpsertContacts(ctx, orgID, userID, ch, expectedVersion, prov)
			},
		}, nil

	case toolKBPoliciesUpsert:
		ch, err := parsePoliciesChanges(changes)
		if err != nil {
			return UpsertCall{}, err
		}
		return UpsertCall{
			Tool: tool, Key: kbstore.NaturalKeyMain,
			apply: func(ctx context.Context, kb *kbstore.Store, orgID, userID uuid.UUID, expectedVersion *int64, prov kbstore.MCPProvenance) (kbstore.UpsertResult, error) {
				return kb.MCPUpsertPolicies(ctx, orgID, userID, ch, expectedVersion, prov)
			},
		}, nil

	case toolKBDeliveryZoneUpsert:
		ch, err := parseDeliveryZoneChanges(changes)
		if err != nil {
			return UpsertCall{}, err
		}
		ref := stringField(args, "ref")
		return UpsertCall{
			Tool: tool, Key: ref,
			apply: func(ctx context.Context, kb *kbstore.Store, orgID, userID uuid.UUID, expectedVersion *int64, prov kbstore.MCPProvenance) (kbstore.UpsertResult, error) {
				return kb.MCPUpsertDeliveryZone(ctx, orgID, userID, ref, ch, expectedVersion, prov)
			},
		}, nil

	default:
		return UpsertCall{}, fmt.Errorf("mcpserver: %q is not an upsert tool", tool)
	}
}

// UpsertToolSchemas returns the {tool name: inputSchema} map for names (a
// subset of UpsertTools, in practice) — the exact InputSchema each tool's
// own Tools() entry declares, so a synthesis prompt (internal/kbimport)
// can quote a tool's real JSON Schema without reaching into tools.go's
// unexported per-tool constructors directly. A name Tools() does not
// recognize is silently absent from the result, not an error: a caller
// building a prompt from a caller-chosen subset wants "every schema that
// exists among what I asked for," not a hard failure over one bad name.
func UpsertToolSchemas(names []string) map[string]map[string]any {
	all := Tools()
	byName := make(map[string]Tool, len(all))
	for _, t := range all {
		byName[t.Name] = t
	}
	out := make(map[string]map[string]any, len(names))
	for _, n := range names {
		if t, ok := byName[n]; ok {
			out[n] = t.InputSchema
		}
	}
	return out
}
