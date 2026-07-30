// mcp_types.go, mcp_read.go, mcp_write.go, and mcp_media.go are the surface
// internal/mcpserver's 11 model-facing KB tools call into (plan/mcp.md §5).
// They add MCP-specific semantics — partial patches, natural-key duplicate
// detection, optimistic concurrency, media-reference validation — on top of
// the existing draft/live primitives (draft.go, live.go, zones.go); no
// existing Playground/live-editor behavior is reused or bypassed, only
// composed. The one safety boundary plan/mcp.md §1 states stays true here:
// every MCP write lands in kbd_draft, never in a live ai_* table directly.
package kbstore

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// KB type names as the MCP surface names them (plan/mcp.md §2's "KB type"
// column) — distinct from the draft blob's own internal section names
// (kept as-is for minimal diff against the existing Playground code) and
// from the live table names. This is the ONLY vocabulary internal/mcpserver
// and the KB Manager widget speak.
const (
	KBTypeAssistant    = "assistant"
	KBTypeTopic        = "topic"
	KBTypeProduct      = "product"
	KBTypeTariff       = "tariff"
	KBTypeContacts     = "contacts"
	KBTypePolicies     = "policies"
	KBTypeDeliveryZone = "delivery_zone"
)

// AllKBTypes is the closed, ordered vocabulary — used to validate a
// requested `types` filter and to drive an "every type" scan.
var AllKBTypes = []string{
	KBTypeAssistant, KBTypeTopic, KBTypeProduct, KBTypeTariff,
	KBTypeContacts, KBTypePolicies, KBTypeDeliveryZone,
}

// NaturalKeyMain is the fixed stable key for every singleton KB type
// (assistant, contacts, policies — plan/mcp.md §2's "Stable key" column).
// It is a presentation-layer constant only: none of these three tables has
// an actual key column any more (migration 0012 dropped the legacy
// `slug` columns once they became true one-row-per-org singletons), so this
// does not need to, and deliberately does not, match the dormant
// internal/brain/domain package's own ContactSlug ("support")/PolicySlug
// ("main") constants used by the legacy fact-token renderer.
const NaturalKeyMain = "main"

// Identity is one compact entry of the identity index kb_summary returns and
// every MCP upsert's server-side duplicate check runs against (plan/mcp.md
// §4). It intentionally carries no database id or full record content.
type Identity struct {
	Type          string `json:"type"`
	Key           string `json:"key"`
	Title         string `json:"title"`
	ExistsInLive  bool   `json:"exists_in_live"`
	ExistsInDraft bool   `json:"exists_in_draft"`
	ToDelete      bool   `json:"-"` // folded into State(); not its own wire field
}

// State derives the widget's four-state badge (plan/mcp.md §6 "All"):
// published | new | changed | to_delete. A pending delete always wins the
// label even though DeleteTopic/DeleteTariff/... also clear any shadowing
// draft entry (so ExistsInDraft is normally false on a to_delete row too) —
// this still needs to be checked first rather than falling out of
// ExistsInLive/ExistsInDraft alone, which cannot otherwise distinguish
// "published, untouched" from "published, marked for deletion".
func (id Identity) State() string {
	switch {
	case id.ToDelete:
		return "to_delete"
	case id.ExistsInLive && id.ExistsInDraft:
		return "changed"
	case id.ExistsInDraft:
		return "new"
	default:
		return "published"
	}
}

// --- errors -----------------------------------------------------------

// ErrRequiredFieldMissing means a create-only required field (in_stock,
// pricing_type, delivery_available, or the title/name needed to derive a
// key) was not supplied.
type ErrRequiredFieldMissing struct{ Field string }

func (e *ErrRequiredFieldMissing) Error() string {
	return fmt.Sprintf("kbstore: %s is required to create this record", e.Field)
}

// ErrDuplicateConflict means an exact normalized-title match exists under a
// DIFFERENT natural key (plan/mcp.md §4 step 2): the caller must call
// kb_read/kb_summary and retry with that key instead of creating a
// duplicate.
type ErrDuplicateConflict struct {
	Type          string
	ExistingKey   string
	ExistingTitle string
}

func (e *ErrDuplicateConflict) Error() string {
	return fmt.Sprintf("kbstore: a %s named %q already exists with key %q — update it by key instead of creating a duplicate",
		e.Type, e.ExistingTitle, e.ExistingKey)
}

// ErrAmbiguousMatch means one or more SIMILAR (not exact) titles exist under
// other keys (plan/mcp.md §4 step 3): the caller must ask the user to choose
// rather than silently creating a possible duplicate.
type ErrAmbiguousMatch struct {
	Type       string
	Candidates []Identity
}

func (e *ErrAmbiguousMatch) Error() string {
	keys := make([]string, 0, len(e.Candidates))
	for _, c := range e.Candidates {
		keys = append(keys, fmt.Sprintf("%s (%q)", c.Key, c.Title))
	}
	return fmt.Sprintf("kbstore: %d similar %s(s) already exist — ask the user which one before creating: %s",
		len(e.Candidates), e.Type, strings.Join(keys, ", "))
}

// ErrMediaReference wraps every way a supplied material_id can fail
// validation (plan/mcp.md §9: "same-organization media references").
type ErrMediaReference struct {
	MaterialID uuid.UUID
	Field      string
	Reason     string
}

func (e *ErrMediaReference) Error() string {
	return fmt.Sprintf("kbstore: media reference %s for %s: %s", e.MaterialID, e.Field, e.Reason)
}

// normalizeTitle folds a title/name to the comparable form the exact-match
// duplicate check uses: lowercase, whitespace-collapsed, trimmed. This is a
// deliberately simple fold (no Unicode case-folding beyond strings.ToLower,
// no punctuation stripping) — good enough to catch "Бизнес" vs " бизнес ",
// not intended as a general text-similarity engine.
func normalizeTitle(s string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(s))), " ")
}

// similarTitles reports whether two ALREADY-normalized titles are "similar
// but not identical" — plan/mcp.md §4 step 3's fuzzy match. Word-set (token)
// Jaccard overlap: cheap, symmetric, and good enough for seller-authored
// product/tariff/topic names ("Business" vs "Business Pro"), without pulling
// in an edit-distance dependency for what is fundamentally a UX nicety (the
// backend errs toward asking the user rather than guessing either way).
func similarTitles(a, b string) bool {
	if a == "" || b == "" || a == b {
		return false // exact/blank handled by the caller before this
	}
	wa := strings.Fields(a)
	wb := strings.Fields(b)
	setA := make(map[string]bool, len(wa))
	for _, w := range wa {
		setA[w] = true
	}
	setB := make(map[string]bool, len(wb))
	for _, w := range wb {
		setB[w] = true
	}
	shared := 0
	for w := range setA {
		if setB[w] {
			shared++
		}
	}
	union := len(setA) + len(setB) - shared
	if union == 0 {
		return false
	}
	return float64(shared)/float64(union) >= 0.5
}
