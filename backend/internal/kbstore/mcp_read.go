package kbstore

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/google/uuid"
)

// deleteKindFor maps an MCP-facing KB type name to the internal
// DraftDelete.Kind vocabulary (draft.go), which predates and differs from it
// for the two singletons: "contacts"/"policies" (plural, matching plan/
// mcp.md's own type names and the ai_contacts/ai_policies table names) vs
// the blob's "contact"/"policy" (singular, the pre-existing Playground
// convention). Every other type name is already identical in both
// vocabularies.
func deleteKindFor(kbType string) string {
	switch kbType {
	case KBTypeContacts:
		return "contact"
	case KBTypePolicies:
		return "policy"
	default:
		return kbType // tariff_info's MCP name already matches the blob's DraftDelete.Kind
	}
}

// typeSet builds a membership set from a `types` filter; an empty filter
// means "every type".
func typeSet(types []string) map[string]bool {
	if len(types) == 0 {
		return nil // nil means "no filter" to typeWanted below
	}
	m := make(map[string]bool, len(types))
	for _, t := range types {
		m[t] = true
	}
	return m
}

func typeWanted(want map[string]bool, t string) bool {
	return want == nil || want[t]
}

// IdentityIndex is kb_summary's backbone (plan/mcp.md §5) and the server-side
// duplicate-detection backstop every MCP upsert re-runs (§4): one compact
// entry per natural key, across the requested types, live ∪ draft, with
// state pre-derived. types nil/empty means every type; source (""|"live"|
// "draft"|"both") filters which entries are returned by presence, not which
// data is fetched — ExistsInLive/ExistsInDraft are always computed either
// way, since kb_summary's whole point is showing both dimensions even when
// scoped to one; query, when non-empty, keeps only entries whose (possibly
// draft-shadowed) Title case-insensitively contains it, the same
// normalizeTitle+Contains match kb_read's flattenRecords uses.
func (s *Store) IdentityIndex(ctx context.Context, orgID uuid.UUID, types []string, source, query string) ([]Identity, error) {
	return s.identityIndex(ctx, s.db, orgID, types, source, query)
}

// identityIndex is IdentityIndex's db-parameterized core. Every MCPUpsert*
// duplicate check calls this (not IdentityIndex) from inside
// writeDraftBlobVersioned's mutate closure, passing the closure's own tx —
// reusing that already-locked connection instead of letting LiveView/
// DraftOnly/readDraftBlob each check out a SEPARATE one from the pool. Under
// N concurrent writers hitting the SAME org, that second acquisition can
// starve: writer A holds the row lock (via tx) and blocks on a free
// connection for this read, while every other writer is itself blocked on
// A's lock while holding a connection of its own — a pool-exhaustion
// deadlock with no timeout, not just a slow path (caught by
// TestMCPUpsert_ConcurrentWritesSerializeWithoutLostUpdates hanging forever
// before this fix). The duplicate-check backstop always passes
// source="both", query="" — it must see every candidate regardless of
// whatever kb_summary filters the model itself used to get here.
func (s *Store) identityIndex(ctx context.Context, db dbtx, orgID uuid.UUID, types []string, source, query string) ([]Identity, error) {
	want := typeSet(types)
	live, err := s.liveView(ctx, db, orgID)
	if err != nil {
		return nil, err
	}
	draft, err := s.draftOnly(ctx, db, orgID)
	if err != nil {
		return nil, err
	}
	blob, _, _, err := s.readDraftBlob(ctx, db, orgID)
	if err != nil {
		return nil, err
	}
	deleted := map[string]bool{}
	for _, d := range blob.Deletes {
		deleted[d.Kind+":"+d.Key] = true
	}

	byKey := map[string]*Identity{}
	order := []string{} // preserves first-seen (live-then-draft, in query order) for deterministic output
	get := func(typ, key string) *Identity {
		k := typ + ":" + key
		e, ok := byKey[k]
		if !ok {
			e = &Identity{Type: typ, Key: key}
			byKey[k] = e
			order = append(order, k)
		}
		if deleted[deleteKindFor(typ)+":"+key] {
			e.ToDelete = true
		}
		return e
	}

	if typeWanted(want, KBTypeAssistant) {
		e := get(KBTypeAssistant, NaturalKeyMain)
		e.Title = "Ассистент"
		// Nothing auto-seeds ai_assistants (kbstore/seed_demo.go is opt-in only,
		// never called from serve's boot path) — a fresh org, or any org whose
		// assistant config has only ever been staged and never approved, has no
		// live row yet. Was hardcoded true under the old assumption that 0008's
		// migration always seeded one; that pair nets to zero today.
		var existsLive bool
		if err := db.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM ai_assistants WHERE organization_id = $1)`, orgID).
			Scan(&existsLive); err != nil {
			return nil, err
		}
		e.ExistsInLive = existsLive
		e.ExistsInDraft = draft.Config.Draft
	}
	if typeWanted(want, KBTypeTopic) {
		for _, t := range live.Topics {
			e := get(KBTypeTopic, t.Slug)
			e.Title, e.ExistsInLive = t.Title, true
		}
		for _, t := range draft.Topics {
			e := get(KBTypeTopic, t.Slug)
			e.Title, e.ExistsInDraft = t.Title, true
		}
	}
	if typeWanted(want, KBTypeProduct) {
		for _, p := range live.Products {
			e := get(KBTypeProduct, p.Ref)
			e.Title, e.ExistsInLive = p.Name, true
		}
		for _, p := range draft.Products {
			e := get(KBTypeProduct, p.Ref)
			e.Title, e.ExistsInDraft = p.Name, true
		}
	}
	if typeWanted(want, KBTypeTariff) {
		for _, t := range live.Tariffs {
			e := get(KBTypeTariff, t.Ref)
			e.Title, e.ExistsInLive = t.Name, true
		}
		for _, t := range draft.Tariffs {
			e := get(KBTypeTariff, t.Ref)
			e.Title, e.ExistsInDraft = t.Name, true
		}
	}
	if typeWanted(want, KBTypeContacts) {
		for range live.Contacts {
			e := get(KBTypeContacts, NaturalKeyMain)
			e.Title, e.ExistsInLive = "Контакты", true
		}
		for range draft.Contacts {
			e := get(KBTypeContacts, NaturalKeyMain)
			e.Title, e.ExistsInDraft = "Контакты", true
		}
	}
	if typeWanted(want, KBTypePolicies) {
		for range live.Policies {
			e := get(KBTypePolicies, NaturalKeyMain)
			e.Title, e.ExistsInLive = "Политики", true
		}
		for range draft.Policies {
			e := get(KBTypePolicies, NaturalKeyMain)
			e.Title, e.ExistsInDraft = "Политики", true
		}
	}
	if typeWanted(want, KBTypeTariffInfo) {
		for range live.TariffInfo {
			e := get(KBTypeTariffInfo, NaturalKeyMain)
			e.Title, e.ExistsInLive = "Тарифная информация", true
		}
		for range draft.TariffInfo {
			e := get(KBTypeTariffInfo, NaturalKeyMain)
			e.Title, e.ExistsInDraft = "Тарифная информация", true
		}
	}
	if typeWanted(want, KBTypeDeliveryZone) {
		for _, z := range live.Zones {
			e := get(KBTypeDeliveryZone, z.Ref)
			e.Title, e.ExistsInLive = z.Name, true
		}
		for _, z := range draft.Zones {
			e := get(KBTypeDeliveryZone, z.Ref)
			e.Title, e.ExistsInDraft = z.Name, true
		}
	}

	qnorm := normalizeTitle(query)
	out := make([]Identity, 0, len(order))
	for _, k := range order {
		e := *byKey[k]
		if source == "live" && !e.ExistsInLive {
			continue
		}
		if source == "draft" && !e.ExistsInDraft {
			continue
		}
		if qnorm != "" && !strings.Contains(normalizeTitle(e.Title), qnorm) {
			continue
		}
		out = append(out, e)
	}
	return out, nil
}

// DuplicateVerdict is the outcome of the natural-key + normalized-title
// backstop every MCP upsert runs (plan/mcp.md §4), independent of whatever
// kb_summary/kb_read calls the model already made.
type DuplicateVerdict struct {
	// ExactKeyMatch is true when key already identifies a live or draft row
	// — this is always an UPDATE, never a duplicate question.
	ExactKeyMatch bool
	// Conflict is set when a DIFFERENT key has the exact same normalized
	// title — the caller must return ErrDuplicateConflict.
	Conflict *Identity
	// Candidates is set when one or more different keys have a SIMILAR (not
	// exact) title — the caller must return ErrAmbiguousMatch.
	Candidates []Identity
}

// checkDuplicate runs the plan/mcp.md §4 decision sequence for a create (key
// is "" or not found) or an explicit-new-key create: exact normalized title
// under another key → conflict; similar titles under other keys →
// candidates; otherwise clear to create. It is a pure decision over an
// already-fetched identity index, callable without a fresh query when the
// caller (an MCPUpsert* method) already has one from the same
// writeDraftBlob transaction.
func checkDuplicate(kbType, key, title string, index []Identity) DuplicateVerdict {
	norm := normalizeTitle(title)
	if norm == "" {
		return DuplicateVerdict{}
	}
	var candidates []Identity
	for _, id := range index {
		if id.Type != kbType || id.Key == key {
			continue
		}
		other := normalizeTitle(id.Title)
		if other == norm {
			dup := id
			return DuplicateVerdict{Conflict: &dup}
		}
		if similarTitles(norm, other) {
			candidates = append(candidates, id)
		}
	}
	return DuplicateVerdict{Candidates: candidates}
}

// ---------------------------------------------------------------------------
// kb_read
// ---------------------------------------------------------------------------

// KBRecord is one typed, sourced, complete record — kb_read's result shape.
// Source is always explicit ("live" or "draft"); plan/mcp.md §5 is explicit
// that source=both must show both origins side by side, never a merged
// "effective" row.
type KBRecord struct {
	Type   string `json:"type"`
	Source string `json:"source"`
	Data   any    `json:"data"`
}

// ReadPage is kb_read's paginated result.
type ReadPage struct {
	Items      []KBRecord `json:"items"`
	NextCursor string     `json:"next_cursor,omitempty"`
}

// MediaIDsIn collects every material id a page's records reference, in first-
// seen order and de-duplicated. Pure — no database access — so enriching a
// kb_read result costs exactly one extra query for the whole page, and none
// at all for a page that references no media.
//
// The types here are the *Row structs, NOT the Draft* blob structs — those
// are two separate families, and flattenRecords puts DraftView's *Row VALUES
// (not pointers) into KBRecord.Data. Getting that wrong silently collects
// nothing at all, which is why TestMediaIDsIn_CoversEveryMediaColumn drives
// this through the real read path rather than through hand-built structs.
// Every media column is listed, featured_image included — it is displayable
// even though it is not an attach target.
func MediaIDsIn(page ReadPage) []uuid.UUID {
	var out []uuid.UUID
	seen := map[uuid.UUID]bool{}
	add := func(ids ...*uuid.UUID) {
		for _, id := range ids {
			if id == nil || *id == uuid.Nil || seen[*id] {
				continue
			}
			seen[*id] = true
			out = append(out, *id)
		}
	}
	addAll := func(groups ...[]uuid.UUID) {
		for _, ids := range groups {
			for i := range ids {
				add(&ids[i])
			}
		}
	}
	for _, rec := range page.Items {
		switch d := rec.Data.(type) {
		case TopicRow:
			add(d.FeaturedImage)
			addAll(d.IllustrationImages, d.ExplainerVideos, d.ReferenceDocuments)
		case ProductRow:
			add(d.FeaturedImage)
			addAll(d.GalleryImages, d.DemoVideos, d.CertificateDocuments, d.GuaranteeDocuments)
		case TariffRow:
			add(d.FeaturedImage)
			addAll(d.PricingImages, d.ExplainerVideos, d.TermsDocuments)
		case ContactRow:
			add(d.ContactCardImage, d.LocationMapImage)
			addAll(d.CompanyLegalDocuments)
		case PolicyRow:
			addAll(d.CommercePolicyDocuments)
		}
	}
	return out
}

const (
	defaultReadLimit = 50
	maxReadLimit     = 100
)

// ReadRecords implements kb_read (plan/mcp.md §5): complete records for the
// model or widget, filtered by type/source/exact-key/text-query, paginated.
// Pagination is a plain offset cursor (an opaque decimal string) — the KB is
// documented as small by design (plan/DECISIONS.md's whole-KB-in-prompt
// premise), so real keyset pagination would be complexity with no present
// payoff; if that premise ever stops holding, this is the seam to revisit.
func (s *Store) ReadRecords(ctx context.Context, orgID uuid.UUID, types []string, source, key, query string, limit int, cursor string) (ReadPage, error) {
	if limit <= 0 {
		limit = defaultReadLimit
	}
	if limit > maxReadLimit {
		limit = maxReadLimit
	}
	offset, err := parseCursor(cursor)
	if err != nil {
		return ReadPage{}, fmt.Errorf("kbstore: invalid cursor: %w", err)
	}
	want := typeSet(types)
	qnorm := normalizeTitle(query)

	var all []KBRecord
	if source == "" || source == "live" || source == "both" {
		live, err := s.LiveView(ctx, orgID)
		if err != nil {
			return ReadPage{}, err
		}
		all = append(all, flattenRecords(live, "live", want, key, qnorm)...)
	}
	if source == "draft" || source == "both" {
		draft, err := s.DraftOnly(ctx, orgID)
		if err != nil {
			return ReadPage{}, err
		}
		all = append(all, flattenRecords(draft, "draft", want, key, qnorm)...)
	}

	if offset > len(all) {
		offset = len(all)
	}
	end := offset + limit
	if end > len(all) {
		end = len(all)
	}
	page := ReadPage{Items: append([]KBRecord{}, all[offset:end]...)}
	if end < len(all) {
		page.NextCursor = strconv.Itoa(end)
	}
	return page, nil
}

func parseCursor(cursor string) (int, error) {
	if cursor == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(cursor)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("cursor must be a non-negative integer")
	}
	return n, nil
}

// flattenRecords projects one DraftView (a live-only or draft-only
// projection — never the merged Draft() view) into KBRecord entries,
// filtered by type/key/normalized-query-substring-of-title.
func flattenRecords(v *DraftView, source string, want map[string]bool, key, qnorm string) []KBRecord {
	var out []KBRecord
	match := func(title string) bool {
		return qnorm == "" || strings.Contains(normalizeTitle(title), qnorm)
	}
	if typeWanted(want, KBTypeAssistant) && (key == "" || key == NaturalKeyMain) {
		out = append(out, KBRecord{Type: KBTypeAssistant, Source: source, Data: v.Config})
	}
	if typeWanted(want, KBTypeTopic) {
		for _, t := range v.Topics {
			if (key == "" || key == t.Slug) && match(t.Title) {
				out = append(out, KBRecord{Type: KBTypeTopic, Source: source, Data: t})
			}
		}
	}
	if typeWanted(want, KBTypeProduct) {
		for _, p := range v.Products {
			if (key == "" || key == p.Ref) && match(p.Name) {
				out = append(out, KBRecord{Type: KBTypeProduct, Source: source, Data: p})
			}
		}
	}
	if typeWanted(want, KBTypeTariff) {
		for _, t := range v.Tariffs {
			if (key == "" || key == t.Ref) && match(t.Name) {
				out = append(out, KBRecord{Type: KBTypeTariff, Source: source, Data: t})
			}
		}
	}
	if typeWanted(want, KBTypeContacts) && (key == "" || key == NaturalKeyMain) {
		for _, c := range v.Contacts {
			out = append(out, KBRecord{Type: KBTypeContacts, Source: source, Data: c})
		}
	}
	if typeWanted(want, KBTypePolicies) && (key == "" || key == NaturalKeyMain) {
		for _, p := range v.Policies {
			out = append(out, KBRecord{Type: KBTypePolicies, Source: source, Data: p})
		}
	}
	if typeWanted(want, KBTypeTariffInfo) && (key == "" || key == NaturalKeyMain) {
		for _, ti := range v.TariffInfo {
			out = append(out, KBRecord{Type: KBTypeTariffInfo, Source: source, Data: ti})
		}
	}
	if typeWanted(want, KBTypeDeliveryZone) {
		for _, z := range v.Zones {
			if (key == "" || key == z.Ref) && match(z.Name) {
				out = append(out, KBRecord{Type: KBTypeDeliveryZone, Source: source, Data: z})
			}
		}
	}
	return out
}
