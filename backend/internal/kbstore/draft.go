package kbstore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/yerassyldanay/xchats/backend/internal/brain/domain"
)

// ---------------------------------------------------------------------------
// The draft blob (kbd_draft) — the WHOLE pending KB as one jsonb document, one
// row per org (15 Decision 3). The playground reads/writes this blob; the brain
// NEVER touches it — it reads only the live ai_ tables (kbstore.go · LoadLive).
// Facts are typed entities (tariffs/products/contacts) with concrete columns —
// there is no generic value store (15 Decision 6).
// ---------------------------------------------------------------------------

// DraftConfigPatch carries pending config overrides — a nil field means that
// field has no pending edit (the live value shows through in the merged view).
type DraftConfigPatch struct {
	Persona        *string `json:"persona,omitempty"`
	Mission        *string `json:"mission,omitempty"`
	Guardrails     *string `json:"guardrails,omitempty"`
	LanguagePolicy *string `json:"language_policy,omitempty"`
	ReplyMaxWords  *int    `json:"reply_max_words,omitempty"`
}

// hasPending reports whether any field of the config patch is set — config
// has no natural key, so "is there a pending edit at all" (rather than a
// per-field check) is what an entity-scoped approve of kind "config" targets.
func (c DraftConfigPatch) hasPending() bool {
	return c.Persona != nil || c.Mission != nil || c.Guardrails != nil || c.LanguagePolicy != nil || c.ReplyMaxWords != nil
}

// Draft* are pending blob entries — the same fields as their live row.
// Business columns only, no database id, organization id, timestamp, or
// authoring metadata (plan/DECISIONS.md's kbd_draft shape: "Entries contain
// business columns only") — an audit/authoring trail (plan/mcp.md's
// provenance{source_url?, material_ids?}) is recorded as a kbd_materials
// association instead (kbstore.recordProvenance, mcp_media.go), never as a
// field here. Media fields mirror the canonical ai_* columns (plan/
// DECISIONS.md "Concrete media-column naming"): a nullable singular
// reference is *uuid.UUID (nil = none), a plural reference is []uuid.UUID
// (never nil — always at least an empty slice, matching the live column's
// NOT NULL DEFAULT '{}').
type DraftTopic struct {
	Slug               string      `json:"slug"`
	Title              string      `json:"title"`
	BodyMD             string      `json:"body_md"`
	FeaturedImage      *uuid.UUID  `json:"featured_image"`
	IllustrationImages []uuid.UUID `json:"illustration_images"`
	ExplainerVideos    []uuid.UUID `json:"explainer_videos"`
	ReferenceDocuments []uuid.UUID `json:"reference_documents"`
}

type DraftTariff struct {
	Ref             string      `json:"ref"`
	Name            string      `json:"name"`
	Price           string      `json:"price"`
	LimitText       string      `json:"limit_text"`
	Fee             string      `json:"fee"`
	Summary         string      `json:"summary"`
	PricingType     string      `json:"pricing_type"`
	Advantages      string      `json:"advantages"`
	Disadvantages   string      `json:"disadvantages"`
	SalesStatus     string      `json:"sales_status"`
	FeaturedImage   *uuid.UUID  `json:"featured_image"`
	PricingImages   []uuid.UUID `json:"pricing_images"`
	ExplainerVideos []uuid.UUID `json:"explainer_videos"`
	TermsDocuments  []uuid.UUID `json:"terms_documents"`
}

type DraftProduct struct {
	Ref                  string      `json:"ref"`
	Name                 string      `json:"name"`
	Price                string      `json:"price"`
	Description          string      `json:"description"`
	Category             string      `json:"category"`
	InStock              bool        `json:"in_stock"`
	SalesStatus          string      `json:"sales_status"`
	FeaturedImage        *uuid.UUID  `json:"featured_image"`
	GalleryImages        []uuid.UUID `json:"gallery_images"`
	DemoVideos           []uuid.UUID `json:"demo_videos"`
	CertificateDocuments []uuid.UUID `json:"certificate_documents"`
	GuaranteeDocuments   []uuid.UUID `json:"guarantee_documents"`
}

// DraftContact is the org's single pending support-contact entry — a true
// singleton (no lang dimension; V1 is Russian-only, plan/DECISIONS.md).
type DraftContact struct {
	WhatsApp              string      `json:"whatsapp"`
	Email                 string      `json:"email"`
	Address               string      `json:"address"`
	LegalInformation      string      `json:"legal_information"`
	CallbackTime          string      `json:"callback_time"`
	WorkingHours          string      `json:"working_hours"`
	Phone                 string      `json:"phone"`
	Website               string      `json:"website"`
	Instagram             string      `json:"instagram"`
	ContactCardImage      *uuid.UUID  `json:"contact_card_image"`
	LocationMapImage      *uuid.UUID  `json:"location_map_image"`
	CompanyLegalDocuments []uuid.UUID `json:"company_legal_documents"`
}

// DraftPolicy is a pending ai_policies entry — a structural clone of
// DraftContact (singleton slug 'main'). OutsideZonesNote is also used as the
// live-write path's read-modify-write scratch value (live.go ·
// currentLivePolicy) even though nothing on the Playground/draft side sets
// it yet (draft milestone later) — so it always round-trips as "" for a
// Playground-authored entry, same as before this field existed.
type DraftPolicy struct {
	DeliveryCost            string      `json:"delivery_cost"`
	DeliveryInDays          string      `json:"delivery_in_days"`
	FreeDeliveryFrom        string      `json:"free_delivery_from"`
	MinOrder                string      `json:"min_order"`
	Prepayment              string      `json:"prepayment"`
	Installment             string      `json:"installment"`
	ReturnPeriodInDays      string      `json:"return_period_in_days"`
	Warranty                string      `json:"warranty"`
	OutsideZonesNote        string      `json:"outside_zones_note"`
	CommercePolicyDocuments []uuid.UUID `json:"commerce_policy_documents"`
}

// DraftDeliveryZone is a pending ai_delivery_zones entry — no media columns
// exist on this table in v1 (plan/DECISIONS.md).
type DraftDeliveryZone struct {
	Ref               string `json:"ref"`
	Name              string `json:"name"`
	ZoneLevel         string `json:"zone_level"`
	ParentRef         string `json:"parent_ref"`
	DeliveryAvailable bool   `json:"delivery_available"`
	DeliveryCost      string `json:"delivery_cost"`
	DeliveryInDays    string `json:"delivery_in_days"`
	Notes             string `json:"notes"`
	SalesStatus       string `json:"sales_status"`
}

// DraftDelete marks a live entity for removal at approve. Key is the entity's
// natural key: topic slug, tariff/product/zone ref; contact/policy carry no
// key (true singletons — Kind alone identifies the one row).
type DraftDelete struct {
	Kind string `json:"kind"` // 'topic'|'tariff'|'product'|'contact'|'policy'|'delivery_zone'
	Key  string `json:"key"`
}

// DraftBlob is the whole pending KB — the exact shape of kbd_draft.draft.
type DraftBlob struct {
	Config        DraftConfigPatch    `json:"config"`
	Topics        []DraftTopic        `json:"topics"`
	Tariffs       []DraftTariff       `json:"tariffs"`
	Products      []DraftProduct      `json:"products"`
	Contacts      []DraftContact      `json:"contacts"`
	Policies      []DraftPolicy       `json:"policies"`
	DeliveryZones []DraftDeliveryZone `json:"delivery_zones"`
	Deletes       []DraftDelete       `json:"deletes"`
}

func (b *DraftBlob) upsertTopic(t DraftTopic) {
	for i := range b.Topics {
		if b.Topics[i].Slug == t.Slug {
			b.Topics[i] = t
			return
		}
	}
	b.Topics = append(b.Topics, t)
}

func (b *DraftBlob) removeTopic(slug string) {
	out := b.Topics[:0]
	for _, t := range b.Topics {
		if t.Slug != slug {
			out = append(out, t)
		}
	}
	b.Topics = out
}

func (b *DraftBlob) upsertTariff(t DraftTariff) {
	for i := range b.Tariffs {
		if b.Tariffs[i].Ref == t.Ref {
			b.Tariffs[i] = t
			return
		}
	}
	b.Tariffs = append(b.Tariffs, t)
}

func (b *DraftBlob) removeTariff(ref string) {
	out := b.Tariffs[:0]
	for _, t := range b.Tariffs {
		if t.Ref != ref {
			out = append(out, t)
		}
	}
	b.Tariffs = out
}

func (b *DraftBlob) upsertProduct(p DraftProduct) {
	for i := range b.Products {
		if b.Products[i].Ref == p.Ref {
			b.Products[i] = p
			return
		}
	}
	b.Products = append(b.Products, p)
}

func (b *DraftBlob) removeProduct(ref string) {
	out := b.Products[:0]
	for _, p := range b.Products {
		if p.Ref != ref {
			out = append(out, p)
		}
	}
	b.Products = out
}

// upsertContact replaces the org's single pending contact entry (true
// singleton — no lang key).
func (b *DraftBlob) upsertContact(c DraftContact) {
	b.Contacts = []DraftContact{c}
}

func (b *DraftBlob) removeContact() {
	b.Contacts = nil
}

// upsertPolicy / removePolicy — exact clone of upsertContact/removeContact
// (ai_policies is a singleton table like ai_contacts).
func (b *DraftBlob) upsertPolicy(p DraftPolicy) {
	b.Policies = []DraftPolicy{p}
}

func (b *DraftBlob) removePolicy() {
	b.Policies = nil
}

func (b *DraftBlob) upsertZone(z DraftDeliveryZone) {
	for i := range b.DeliveryZones {
		if b.DeliveryZones[i].Ref == z.Ref {
			b.DeliveryZones[i] = z
			return
		}
	}
	b.DeliveryZones = append(b.DeliveryZones, z)
}

func (b *DraftBlob) removeZone(ref string) {
	out := b.DeliveryZones[:0]
	for _, z := range b.DeliveryZones {
		if z.Ref != ref {
			out = append(out, z)
		}
	}
	b.DeliveryZones = out
}

func (b *DraftBlob) addDelete(kind, key string) {
	for _, d := range b.Deletes {
		if d.Kind == kind && d.Key == key {
			return
		}
	}
	b.Deletes = append(b.Deletes, DraftDelete{Kind: kind, Key: key})
}

func (b *DraftBlob) removeDelete(kind, key string) {
	out := b.Deletes[:0]
	for _, d := range b.Deletes {
		if !(d.Kind == kind && d.Key == key) {
			out = append(out, d)
		}
	}
	b.Deletes = out
}

// removeDeleteMatching is removeDelete's CancelChange-facing counterpart: it
// drops any delete marker for (singular, key) via deleteMatches — the same
// singleton-key-is-ignored comparison selectApproved uses — rather than an
// exact (kind, key) pair, so a marker MCPDelete wrote under NaturalKeyMain
// and one CancelChange is asked to drop under domain.ContactSlug/PolicySlug
// are recognized as the SAME marker.
func (b *DraftBlob) removeDeleteMatching(singular, key string) {
	out := b.Deletes[:0]
	for _, d := range b.Deletes {
		if !deleteMatches(d, singular, key) {
			out = append(out, d)
		}
	}
	b.Deletes = out
}

// ---------------------------------------------------------------------------
// Blob read + read-modify-write (optimistic concurrency via base_version)
// ---------------------------------------------------------------------------

// readDraftBlob is a side-effect-free read of the org's draft blob — an empty
// blob, version 0, zero time if no row exists yet (the row is created lazily on
// the first write). db lets a caller already inside writeDraftBlobVersioned's
// transaction (tx) reuse that SAME connection instead of checking out another
// one from the pool — see identityIndex's doc comment for why that matters.
func (s *Store) readDraftBlob(ctx context.Context, db dbtx, orgID uuid.UUID) (DraftBlob, int64, time.Time, error) {
	var raw []byte
	var ver int64
	var updatedAt time.Time
	err := db.QueryRow(ctx, `SELECT draft, base_version, updated_at FROM xchats.kbd_draft WHERE organization_id = $1`, orgID).
		Scan(&raw, &ver, &updatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return DraftBlob{}, 0, time.Time{}, nil
	}
	if err != nil {
		return DraftBlob{}, 0, time.Time{}, err
	}
	blob := DraftBlob{}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &blob); err != nil {
			return DraftBlob{}, 0, time.Time{}, err
		}
	}
	return blob, ver, updatedAt, nil
}

// DraftBaseVersion returns the org's current blob version — the optimistic-
// concurrency token clients echo via If-Match (doc 9 · kbd_draft.base_version).
// 0 if no draft activity has happened yet.
func (s *Store) DraftBaseVersion(ctx context.Context, orgID uuid.UUID) (int64, error) {
	_, ver, _, err := s.readDraftBlob(ctx, s.pool, orgID)
	return ver, err
}

// writeDraftBlob runs mutate over the org's draft blob inside a row-locked
// transaction (SELECT ... FOR UPDATE), so concurrent writers serialize instead
// of racing, then persists the result and bumps base_version. This is the ONLY
// way the blob is written for every pre-existing (non-MCP) caller. mutate
// receives the transaction itself (as a dbtx) so a closure that needs to read
// something else (currentTopic and friends, IdentityIndex) can run that read
// on the SAME already-locked connection — see identityIndex's doc comment for
// why reaching for s.pool instead, mid-transaction, is a deadlock risk under
// concurrent writers.
func (s *Store) writeDraftBlob(ctx context.Context, orgID uuid.UUID, userID uuid.UUID, mutate func(dbtx, *DraftBlob) error) error {
	_, err := s.writeDraftBlobVersioned(ctx, orgID, nil, userID, mutate)
	return err
}

// writeDraftBlobVersioned is writeDraftBlob's superset for the MCP write
// path: when expectedVersion is non-nil, the write is rejected with ErrStale
// unless the blob's CURRENT base_version (read under the same row lock)
// matches exactly (plan/mcp.md's `expected_draft_version?` optimistic
// concurrency — an MCP-only concern; writeDraftBlob's nil-expectedVersion
// callers never conflict on version, unchanged from before this existed).
// Returns the resulting base_version either way, so a caller can report it
// back (kb_summary's draft_version, an upsert result's new version). userID
// is recorded as kbd_draft.updated_by (DECISIONS.md: "Last operator who
// changed the draft") — uuid.Nil for a caller with no attributable human
// (there is none today; every write path, MCP or the legacy editor, has an
// authenticated actor by the time it reaches here).
//
// Built on lockDraftBlob/persistDraftBlob — the same lock-read/marshal-write
// primitives CancelChange uses with its own ordering (CancelChange must
// decide whether a stale expectedVersion even matters BEFORE the version
// check runs, which this straight-line check-then-mutate order cannot
// express — see CancelChange's doc comment).
func (s *Store) writeDraftBlobVersioned(ctx context.Context, orgID uuid.UUID, expectedVersion *int64, userID uuid.UUID, mutate func(dbtx, *DraftBlob) error) (int64, error) {
	tx, blob, currentVersion, err := s.lockDraftBlob(ctx, orgID)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	if expectedVersion != nil && *expectedVersion != currentVersion {
		return 0, ErrStale
	}

	if err := mutate(tx, &blob); err != nil {
		return 0, err
	}

	return persistDraftBlob(ctx, tx, orgID, userID, blob)
}

// lockDraftBlob begins a transaction, takes the org's advisory lock, and
// reads the current blob + base_version under FOR UPDATE — the shared
// preamble every draft-blob writer needs. The caller owns the returned
// transaction on success (commit or rollback it); on error the transaction
// is already rolled back and the returned tx is nil.
//
// The advisory lock, keyed on the org, serializes EVERY writer to this org's
// draft — including the very first one, before any kbd_draft row exists for
// "SELECT ... FOR UPDATE" below to lock at all. Without this: two concurrent
// first-writers both see "no row, version 0" (pgx.ErrNoRows locks nothing),
// both mutate an independently-empty blob, and whichever's INSERT ... ON
// CONFLICT in persistDraftBlob runs second silently overwrites the first's
// already-committed content — a lost update the row lock alone cannot
// prevent, because there is no row yet to lock against. draftLockSeed
// namespaces this codebase's one advisory-lock use; a future unrelated
// advisory lock added elsewhere should pick a different seed to avoid
// needlessly serializing against this one. The _xact_ variant releases
// automatically at commit/rollback, so it can never leak on an early return
// or a panic.
func (s *Store) lockDraftBlob(ctx context.Context, orgID uuid.UUID) (pgx.Tx, DraftBlob, int64, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, DraftBlob{}, 0, err
	}
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, $2))`, orgID.String(), draftLockSeed); err != nil {
		_ = tx.Rollback(ctx)
		return nil, DraftBlob{}, 0, err
	}

	var raw []byte
	var currentVersion int64
	err = tx.QueryRow(ctx, `SELECT draft, base_version FROM xchats.kbd_draft WHERE organization_id = $1 FOR UPDATE`, orgID).
		Scan(&raw, &currentVersion)
	blob := DraftBlob{}
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		// no row yet — version 0; the advisory lock above already rules out
		// a concurrent first-writer racing this exact branch.
	case err != nil:
		_ = tx.Rollback(ctx)
		return nil, DraftBlob{}, 0, err
	case len(raw) > 0:
		if err := json.Unmarshal(raw, &blob); err != nil {
			_ = tx.Rollback(ctx)
			return nil, DraftBlob{}, 0, err
		}
	}
	return tx, blob, currentVersion, nil
}

// persistDraftBlob marshals blob, writes it as the org's new kbd_draft row
// (bumping base_version), and commits tx — the write half of every draft
// mutation, shared by writeDraftBlobVersioned and CancelChange.
func persistDraftBlob(ctx context.Context, tx pgx.Tx, orgID, userID uuid.UUID, blob DraftBlob) (int64, error) {
	out, err := json.Marshal(blob)
	if err != nil {
		return 0, err
	}
	// RETURNING the row's ACTUAL stored base_version, rather than computing
	// currentVersion+1 locally, so the reported version can never disagree
	// with what is really persisted — belt-and-suspenders alongside
	// lockDraftBlob's advisory lock, which already makes the version this
	// increments from trustworthy by the time we get here.
	var newVersion int64
	if err := tx.QueryRow(ctx, `
		INSERT INTO xchats.kbd_draft (organization_id, draft, base_version, updated_at, updated_by)
		VALUES ($1, $2::jsonb, 1, now(), $3)
		ON CONFLICT (organization_id) DO UPDATE SET
			draft = EXCLUDED.draft, base_version = xchats.kbd_draft.base_version + 1, updated_at = now(), updated_by = EXCLUDED.updated_by
		RETURNING base_version`,
		orgID, string(out), nullIfNilUUID(userID)).Scan(&newVersion); err != nil {
		return 0, err
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return newVersion, nil
}

// draftLockSeed is the hashtextextended seed for writeDraftBlobVersioned's
// advisory lock — an arbitrary constant distinguishing this lock's key space
// from any other advisory lock this codebase might add later.
const draftLockSeed int64 = 0x6b626472616674 // "kbdraft" in hex, just a memorable distinct constant

// nullIfNilUUID converts the zero uuid.UUID (no attributable actor) to SQL
// NULL — kbd_draft.updated_by is a nullable FK (ON DELETE SET NULL), so a
// literal all-zero UUID would be a foreign-key violation instead of "no
// actor known".
func nullIfNilUUID(id uuid.UUID) *uuid.UUID {
	if id == uuid.Nil {
		return nil
	}
	return &id
}

// ClearDraft discards every pending edit ("Отменить изменения") — a plain reset
// of the blob to empty. Live rows are untouched.
func (s *Store) ClearDraft(ctx context.Context, orgID uuid.UUID, actor uuid.UUID) error {
	return s.writeDraftBlob(ctx, orgID, actor, func(db dbtx, b *DraftBlob) error {
		*b = DraftBlob{}
		return nil
	})
}

// ---------------------------------------------------------------------------
// CancelChange — «Отменить изменение» on a Черновик card: drop ONE pending
// change without touching live. Genuinely idempotent (15's own bar for any
// mutation reachable by a client retry): a repeat call performs NO write,
// does NOT advance base_version, and reports Changed:false.
// ---------------------------------------------------------------------------

// CancelResult reports whether a CancelChange call actually changed
// anything. A cancel whose outcome already holds performs no write and
// leaves BaseVersion exactly as it was — repeating a cancellation is free,
// so a client retry after a lost response can never inflate base_version
// and stale every other open tab.
type CancelResult struct {
	Changed     bool
	BaseVersion int64
}

// CancelChange drops ONE pending change addressed by the same (kind, key)
// pair a card's Publish (ApproveVersioned/ApproveSelector) uses: the staged
// blob entry (an addition or an update) AND any delete marker for it (a
// staged removal), returning that entity to exactly whatever live says.
// kind == "config" takes a FIELD name as key
// (persona|mission|guardrails|language_policy|reply_max_words) and clears
// just that pointer; key == NaturalKeyMain clears the whole pending config
// patch ("Отменить все изменения ассистента"). ErrUnknownKind for any other
// kind, or an unrecognized config field name. Live tables are never
// touched, so no KB-cache invalidation.
//
// Ordering — why this cannot be built on writeDraftBlobVersioned's
// straight-line check-then-mutate order:
//  1. Read the blob under the advisory lock (lockDraftBlob).
//  2. Determine whether the target is still present.
//  3. Target absent -> return {Changed: false, BaseVersion: current} with NO
//     write, regardless of expectedVersion. A stale If-Match is not an error
//     here: 409 exists to stop one writer clobbering another's payload, and
//     cancellation is convergent — there is no payload to clobber and no
//     lost update to prevent. The caller's response carries the current
//     version and the fresh change set, so a stale client resyncs anyway.
//  4. Target present and expectedVersion stale -> ErrStale -> 409. A real
//     write is about to happen, so the conflict is genuine and is surfaced.
//  5. Target present, version matches -> remove, persist, return
//     {Changed: true, BaseVersion: new}.
//
// Cancelling an unknown key never lazily creates an empty kbd_draft row —
// step 3 returns before any write, and lockDraftBlob already tolerates
// pgx.ErrNoRows.
func (s *Store) CancelChange(ctx context.Context, orgID, actor uuid.UUID, kind, key string, expectedVersion *int64) (CancelResult, error) {
	if kind == "config" {
		if key != NaturalKeyMain && !isConfigField(key) {
			return CancelResult{}, ErrUnknownKind
		}
		return s.cancelWithinLock(ctx, orgID, actor, expectedVersion,
			func(b DraftBlob) bool { return configFieldPending(b.Config, key) },
			func(b *DraftBlob) {
				if key == NaturalKeyMain {
					b.Config = DraftConfigPatch{}
				} else {
					clearConfigField(&b.Config, key)
				}
			})
	}
	singular, ok := SingularDeleteKind(kind)
	if !ok {
		return CancelResult{}, ErrUnknownKind
	}
	return s.cancelWithinLock(ctx, orgID, actor, expectedVersion,
		func(b DraftBlob) bool { return entityChangePresent(b, singular, key) },
		func(b *DraftBlob) {
			switch singular {
			case "topic":
				b.removeTopic(key)
			case "tariff":
				b.removeTariff(key)
			case "product":
				b.removeProduct(key)
			case "delivery_zone":
				b.removeZone(key)
			case "contact":
				b.removeContact()
			case "policy":
				b.removePolicy()
			}
			b.removeDeleteMatching(singular, key)
		})
}

// cancelWithinLock implements CancelChange's 5-step ordering (see its doc
// comment), parameterized by present (step 2) and apply (step 5's mutation)
// so the config and entity branches above share one transaction and one set
// of concurrency rules instead of reimplementing them twice.
func (s *Store) cancelWithinLock(
	ctx context.Context, orgID, actor uuid.UUID, expectedVersion *int64,
	present func(DraftBlob) bool, apply func(*DraftBlob),
) (CancelResult, error) {
	tx, blob, currentVersion, err := s.lockDraftBlob(ctx, orgID)
	if err != nil {
		return CancelResult{}, err
	}
	defer tx.Rollback(ctx)

	if !present(blob) {
		if err := tx.Commit(ctx); err != nil {
			return CancelResult{}, err
		}
		return CancelResult{Changed: false, BaseVersion: currentVersion}, nil
	}
	if expectedVersion != nil && *expectedVersion != currentVersion {
		return CancelResult{}, ErrStale
	}

	apply(&blob)

	newVersion, err := persistDraftBlob(ctx, tx, orgID, actor, blob)
	if err != nil {
		return CancelResult{}, err
	}
	return CancelResult{Changed: true, BaseVersion: newVersion}, nil
}

// entityChangePresent reports whether the blob has a pending upsert entry OR
// a delete marker for (singular, key) — CancelChange's step 2 for every kind
// except config.
func entityChangePresent(blob DraftBlob, singular, key string) bool {
	for _, d := range blob.Deletes {
		if deleteMatches(d, singular, key) {
			return true
		}
	}
	switch singular {
	case "topic":
		for _, t := range blob.Topics {
			if t.Slug == key {
				return true
			}
		}
	case "tariff":
		for _, t := range blob.Tariffs {
			if t.Ref == key {
				return true
			}
		}
	case "product":
		for _, p := range blob.Products {
			if p.Ref == key {
				return true
			}
		}
	case "delivery_zone":
		for _, z := range blob.DeliveryZones {
			if z.Ref == key {
				return true
			}
		}
	case "contact":
		return len(blob.Contacts) > 0
	case "policy":
		return len(blob.Policies) > 0
	}
	return false
}

// isConfigField reports whether key names one of DraftConfigPatch's fields —
// CancelChange("config", key)'s per-field vocabulary.
func isConfigField(key string) bool {
	switch key {
	case "persona", "mission", "guardrails", "language_policy", "reply_max_words":
		return true
	}
	return false
}

// configFieldPending reports whether the given config key has a pending
// edit: NaturalKeyMain asks "any field at all" (hasPending), a field name
// asks about just that pointer.
func configFieldPending(c DraftConfigPatch, key string) bool {
	if key == NaturalKeyMain {
		return c.hasPending()
	}
	switch key {
	case "persona":
		return c.Persona != nil
	case "mission":
		return c.Mission != nil
	case "guardrails":
		return c.Guardrails != nil
	case "language_policy":
		return c.LanguagePolicy != nil
	case "reply_max_words":
		return c.ReplyMaxWords != nil
	}
	return false
}

// clearConfigField nils out one DraftConfigPatch field by name — the
// per-field twin of a whole `*c = DraftConfigPatch{}` reset. Caller
// (CancelChange) has already validated key via isConfigField.
func clearConfigField(c *DraftConfigPatch, key string) {
	switch key {
	case "persona":
		c.Persona = nil
	case "mission":
		c.Mission = nil
	case "guardrails":
		c.Guardrails = nil
	case "language_policy":
		c.LanguagePolicy = nil
	case "reply_max_words":
		c.ReplyMaxWords = nil
	}
}

// ---------------------------------------------------------------------------
// The merged DraftView — live rows overlaid by pending blob entries. Every
// entity is flagged Draft: true|false; there is no more open/no-draft state —
// the view always exists (possibly with nothing pending).
// ---------------------------------------------------------------------------

type DraftConfig struct {
	OrganizationID uuid.UUID `json:"organization_id"`
	Persona        string    `json:"persona"`
	Mission        string    `json:"mission"`
	Guardrails     string    `json:"guardrails"`
	LanguagePolicy string    `json:"language_policy"`
	ReplyMaxWords  int       `json:"reply_max_words"`
	Draft          bool      `json:"draft"`
	BaseVersion    int64     `json:"base_version"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// TopicRow / TariffRow / ProductRow are editor-facing KB rows. ID is the
// entity's natural key (slug / ref / ref) — blob entries carry no DB row id.
// UpdatedAt is the live row's own timestamp for a live entity, or the whole
// draft blob's timestamp for a pending one. ContactRow/PolicyRow (below) are
// true singletons — one per org, ID a fixed constant.
type TopicRow struct {
	ID                 string      `json:"id"`
	Slug               string      `json:"slug"`
	Title              string      `json:"title"`
	BodyMD             string      `json:"body_md"`
	FeaturedImage      *uuid.UUID  `json:"featured_image"`
	IllustrationImages []uuid.UUID `json:"illustration_images"`
	ExplainerVideos    []uuid.UUID `json:"explainer_videos"`
	ReferenceDocuments []uuid.UUID `json:"reference_documents"`
	Draft              bool        `json:"draft"`
	UpdatedAt          time.Time   `json:"updated_at"`
}

type TariffRow struct {
	ID              string      `json:"id"`
	Ref             string      `json:"ref"`
	Name            string      `json:"name"`
	Price           string      `json:"price"`
	LimitText       string      `json:"limit_text"`
	Fee             string      `json:"fee"`
	Summary         string      `json:"summary"`
	PricingType     string      `json:"pricing_type"`
	Advantages      string      `json:"advantages"`
	Disadvantages   string      `json:"disadvantages"`
	SalesStatus     string      `json:"sales_status"`
	FeaturedImage   *uuid.UUID  `json:"featured_image"`
	PricingImages   []uuid.UUID `json:"pricing_images"`
	ExplainerVideos []uuid.UUID `json:"explainer_videos"`
	TermsDocuments  []uuid.UUID `json:"terms_documents"`
	Draft           bool        `json:"draft"`
	UpdatedAt       time.Time   `json:"updated_at"`
}

type ProductRow struct {
	ID                   string      `json:"id"`
	Ref                  string      `json:"ref"`
	Name                 string      `json:"name"`
	Price                string      `json:"price"`
	Description          string      `json:"description"`
	Category             string      `json:"category"`
	InStock              bool        `json:"in_stock"`
	SalesStatus          string      `json:"sales_status"`
	FeaturedImage        *uuid.UUID  `json:"featured_image"`
	GalleryImages        []uuid.UUID `json:"gallery_images"`
	DemoVideos           []uuid.UUID `json:"demo_videos"`
	CertificateDocuments []uuid.UUID `json:"certificate_documents"`
	GuaranteeDocuments   []uuid.UUID `json:"guarantee_documents"`
	Draft                bool        `json:"draft"`
	UpdatedAt            time.Time   `json:"updated_at"`
}

type ContactRow struct {
	ID                    string      `json:"id"`
	Slug                  string      `json:"slug"`
	WhatsApp              string      `json:"whatsapp"`
	Email                 string      `json:"email"`
	Address               string      `json:"address"`
	LegalInformation      string      `json:"legal_information"`
	CallbackTime          string      `json:"callback_time"`
	WorkingHours          string      `json:"working_hours"`
	Phone                 string      `json:"phone"`
	Website               string      `json:"website"`
	Instagram             string      `json:"instagram"`
	ContactCardImage      *uuid.UUID  `json:"contact_card_image"`
	LocationMapImage      *uuid.UUID  `json:"location_map_image"`
	CompanyLegalDocuments []uuid.UUID `json:"company_legal_documents"`
	Draft                 bool        `json:"draft"`
	UpdatedAt             time.Time   `json:"updated_at"`
}

// PolicyRow is the editor-facing ai_policies row — a structural clone of
// ContactRow (ID/Slug the singleton domain.PolicySlug).
type PolicyRow struct {
	ID                      string      `json:"id"`
	Slug                    string      `json:"slug"`
	DeliveryCost            string      `json:"delivery_cost"`
	DeliveryInDays          string      `json:"delivery_in_days"`
	FreeDeliveryFrom        string      `json:"free_delivery_from"`
	MinOrder                string      `json:"min_order"`
	Prepayment              string      `json:"prepayment"`
	Installment             string      `json:"installment"`
	ReturnPeriodInDays      string      `json:"return_period_in_days"`
	Warranty                string      `json:"warranty"`
	OutsideZonesNote        string      `json:"outside_zones_note"`
	CommercePolicyDocuments []uuid.UUID `json:"commerce_policy_documents"`
	Draft                   bool        `json:"draft"`
	UpdatedAt               time.Time   `json:"updated_at"`
}

// DraftView is the whole working KB for the editor + builder: live rows merged
// with pending blob entries.
type DraftView struct {
	Config   DraftConfig  `json:"config"`
	Topics   []TopicRow   `json:"topics"`
	Tariffs  []TariffRow  `json:"tariffs"`
	Products []ProductRow `json:"products"`
	Contacts []ContactRow `json:"contacts"`
	Policies []PolicyRow  `json:"policies"`
	// Zones is live ai_delivery_zones rows overlaid by pending
	// blob.DeliveryZones entries — same live+blob merge as every other
	// entity kind (see mergedView's zones section).
	Zones     []ZoneRow  `json:"zones"`
	Materials []Material `json:"materials"`
	Requests  []Request  `json:"requests"`
}

// Draft assembles the merged working view: live rows, overlaid by pending blob
// entries, with entities under a Deletes[] marker suppressed. Side-effect-free.
func (s *Store) Draft(ctx context.Context, orgID uuid.UUID) (*DraftView, error) {
	blob, ver, updatedAt, err := s.readDraftBlob(ctx, s.pool, orgID)
	if err != nil {
		return nil, err
	}
	v, err := s.mergedView(ctx, s.pool, orgID, blob, ver, updatedAt)
	if err != nil {
		return nil, err
	}
	if v.Materials, err = s.listMaterials(ctx, orgID); err != nil {
		return nil, err
	}
	if v.Requests, err = s.listRequests(ctx, orgID); err != nil {
		return nil, err
	}
	// Guarantee non-nil slices (see mergedView) for the two collections it does
	// not itself query.
	if v.Materials == nil {
		v.Materials = []Material{}
	}
	if v.Requests == nil {
		v.Requests = []Request{}
	}
	return v, nil
}

// LiveView returns the live KB only — no blob overlay, so every row is
// Draft:false, and no materials/requests (playground-only concepts). Used by
// the /kb/* live editor so its reads and writes never see, or touch, pending
// Playground work — the two flows stay fully separate (see plan "Playground
// redesign").
func (s *Store) LiveView(ctx context.Context, orgID uuid.UUID) (*DraftView, error) {
	return s.liveView(ctx, s.pool, orgID)
}

// liveView is LiveView's db-parameterized core — see identityIndex's doc
// comment for why a caller already inside writeDraftBlobVersioned's
// transaction must pass tx here instead of letting this reach for s.pool.
func (s *Store) liveView(ctx context.Context, db dbtx, orgID uuid.UUID) (*DraftView, error) {
	v, err := s.mergedView(ctx, db, orgID, DraftBlob{}, 0, time.Time{})
	if err != nil {
		return nil, err
	}
	v.Materials = []Material{}
	v.Requests = []Request{}
	return v, nil
}

// DraftOnly returns exactly what is pending in kbd_draft — no live overlay —
// with every row's origin explicit (Draft: true) and no merging with a live
// counterpart. This is what kb_read(source=both) needs that Draft() cannot
// give it: Draft() shadows a live row with its pending edit into ONE row, but
// plan/mcp.md §5 requires "live and draft origins remain explicit... there is
// no effective source." Every draft entry is already a complete row by the
// time it is written (the common-write-behavior contract every MCP upsert
// follows), so presenting it standalone needs no live read at all.
func (s *Store) DraftOnly(ctx context.Context, orgID uuid.UUID) (*DraftView, error) {
	return s.draftOnly(ctx, s.pool, orgID)
}

// draftOnly is DraftOnly's db-parameterized core (see liveView).
func (s *Store) draftOnly(ctx context.Context, db dbtx, orgID uuid.UUID) (*DraftView, error) {
	blob, ver, updatedAt, err := s.readDraftBlob(ctx, db, orgID)
	if err != nil {
		return nil, err
	}
	return draftRowsFromBlob(orgID, blob, ver, updatedAt), nil
}

// draftRowsFromBlob is draftOnly's pure core: exactly what the given blob has
// staged, with every row's origin explicit (Draft: true) and a staged
// deletion suppressed for every kind — no database access, no error return,
// since everything it needs is already in blob. Split out so DraftChanges
// (the Черновик review payload) and draftOnly (kb_read(source=draft), the KB
// Manager widget) can never disagree about what "pending" means — both call
// this SAME function over the SAME blob read.
func draftRowsFromBlob(orgID uuid.UUID, blob DraftBlob, ver int64, updatedAt time.Time) *DraftView {
	v := &DraftView{Config: DraftConfig{OrganizationID: orgID, BaseVersion: ver, UpdatedAt: updatedAt}}
	deleted := map[string]bool{}
	for _, d := range blob.Deletes {
		deleted[d.Kind+":"+d.Key] = true
	}
	if p := blob.Config.Persona; p != nil {
		v.Config.Persona, v.Config.Draft = *p, true
	}
	if p := blob.Config.Mission; p != nil {
		v.Config.Mission, v.Config.Draft = *p, true
	}
	if p := blob.Config.Guardrails; p != nil {
		v.Config.Guardrails, v.Config.Draft = *p, true
	}
	if p := blob.Config.LanguagePolicy; p != nil {
		v.Config.LanguagePolicy, v.Config.Draft = *p, true
	}
	if p := blob.Config.ReplyMaxWords; p != nil {
		v.Config.ReplyMaxWords, v.Config.Draft = *p, true
	}
	for _, t := range blob.Topics {
		if deleted["topic:"+t.Slug] {
			continue
		}
		v.Topics = append(v.Topics, TopicRow{ID: t.Slug, Slug: t.Slug, Title: t.Title, BodyMD: t.BodyMD,
			FeaturedImage: t.FeaturedImage, IllustrationImages: t.IllustrationImages,
			ExplainerVideos:    t.ExplainerVideos,
			ReferenceDocuments: t.ReferenceDocuments,
			Draft:              true, UpdatedAt: updatedAt})
	}
	for _, t := range blob.Tariffs {
		if deleted["tariff:"+t.Ref] {
			continue
		}
		v.Tariffs = append(v.Tariffs, TariffRow{ID: t.Ref, Ref: t.Ref, Name: t.Name, Price: t.Price,
			LimitText: t.LimitText, Fee: t.Fee, Summary: t.Summary, PricingType: t.PricingType,
			Advantages: t.Advantages, Disadvantages: t.Disadvantages, SalesStatus: t.SalesStatus,
			FeaturedImage: t.FeaturedImage, PricingImages: t.PricingImages, ExplainerVideos: t.ExplainerVideos,
			TermsDocuments: t.TermsDocuments,
			Draft:          true, UpdatedAt: updatedAt})
	}
	for _, p := range blob.Products {
		if deleted["product:"+p.Ref] {
			continue
		}
		v.Products = append(v.Products, ProductRow{ID: p.Ref, Ref: p.Ref, Name: p.Name, Price: p.Price,
			Description: p.Description, Category: p.Category, InStock: p.InStock, SalesStatus: p.SalesStatus,
			FeaturedImage: p.FeaturedImage, GalleryImages: p.GalleryImages, DemoVideos: p.DemoVideos,
			CertificateDocuments: p.CertificateDocuments, GuaranteeDocuments: p.GuaranteeDocuments,
			Draft: true, UpdatedAt: updatedAt})
	}
	if len(blob.Contacts) > 0 && !deletedSingleton(blob, "contact") {
		c := blob.Contacts[0]
		v.Contacts = append(v.Contacts, ContactRow{ID: domain.ContactSlug, Slug: domain.ContactSlug,
			WhatsApp: c.WhatsApp, Email: c.Email, Address: c.Address, LegalInformation: c.LegalInformation,
			CallbackTime: c.CallbackTime, WorkingHours: c.WorkingHours, Phone: c.Phone, Website: c.Website,
			Instagram: c.Instagram, ContactCardImage: c.ContactCardImage, LocationMapImage: c.LocationMapImage,
			CompanyLegalDocuments: c.CompanyLegalDocuments,
			Draft:                 true, UpdatedAt: updatedAt})
	}
	if len(blob.Policies) > 0 && !deletedSingleton(blob, "policy") {
		p := blob.Policies[0]
		v.Policies = append(v.Policies, PolicyRow{ID: domain.PolicySlug, Slug: domain.PolicySlug,
			DeliveryCost: p.DeliveryCost, DeliveryInDays: p.DeliveryInDays, FreeDeliveryFrom: p.FreeDeliveryFrom,
			MinOrder: p.MinOrder, Prepayment: p.Prepayment, Installment: p.Installment,
			ReturnPeriodInDays: p.ReturnPeriodInDays, Warranty: p.Warranty, OutsideZonesNote: p.OutsideZonesNote,
			CommercePolicyDocuments: p.CommercePolicyDocuments,
			Draft:                   true, UpdatedAt: updatedAt})
	}
	for _, z := range blob.DeliveryZones {
		if deleted["delivery_zone:"+z.Ref] {
			continue
		}
		v.Zones = append(v.Zones, ZoneRow{ID: z.Ref, Ref: z.Ref, Name: z.Name, ZoneLevel: z.ZoneLevel,
			ParentRef: z.ParentRef, DeliveryAvailable: z.DeliveryAvailable, DeliveryCost: z.DeliveryCost,
			DeliveryInDays: z.DeliveryInDays, Notes: z.Notes, SalesStatus: orDefault(z.SalesStatus, "active"),
			Draft: true, UpdatedAt: updatedAt})
	}
	v.Materials = []Material{}
	v.Requests = []Request{}
	if v.Topics == nil {
		v.Topics = []TopicRow{}
	}
	if v.Tariffs == nil {
		v.Tariffs = []TariffRow{}
	}
	if v.Products == nil {
		v.Products = []ProductRow{}
	}
	if v.Contacts == nil {
		v.Contacts = []ContactRow{}
	}
	if v.Policies == nil {
		v.Policies = []PolicyRow{}
	}
	if v.Zones == nil {
		v.Zones = []ZoneRow{}
	}
	return v
}

// ---------------------------------------------------------------------------
// DraftChanges — the Черновик review payload. Unlike Draft()'s merged view,
// this carries ONLY what kbd_draft has staged, plus explicit deletion
// entries, so an unchanged published row can never appear in it. The
// published counterpart a reviewer diffs a pending row against comes from
// LiveView/GET /kb, never from this payload.
// ---------------------------------------------------------------------------

// DraftChangeSet is the review payload behind GET /playground/draft.
// Config is nil when there is no pending config edit at all (as opposed to
// DraftView.Config, which always carries a full, possibly-live-only value).
type DraftChangeSet struct {
	BaseVersion int64               `json:"base_version"`
	UpdatedAt   time.Time           `json:"updated_at"`
	Config      *DraftConfigPatch   `json:"config"`
	Topics      []TopicRow          `json:"topics"`
	Tariffs     []TariffRow         `json:"tariffs"`
	Products    []ProductRow        `json:"products"`
	Contacts    []ContactRow        `json:"contacts"`
	Policies    []PolicyRow         `json:"policies"`
	Zones       []ZoneRow           `json:"zones"`
	Deletes     []DraftChangeDelete `json:"deletes"`
}

// DraftChangeDelete is one staged removal, addressed in the SAME
// HTTP-facing plural vocabulary POST /playground/draft/approve/:kind/:id and
// DELETE /playground/draft/changes/:kind/:key use — never the blob's own
// singular DraftDelete.Kind (bridged via PluralChangeKind).
type DraftChangeDelete struct {
	Kind string `json:"kind"` // topics|tariffs|products|contacts|policies|delivery_zones
	Key  string `json:"key"`
}

// DraftChanges returns the org's pending change set: one blob read, no live
// query, built from the SAME draftRowsFromBlob draftOnly uses — so this
// payload and kb_read(source=draft) can never disagree about what "pending"
// means.
func (s *Store) DraftChanges(ctx context.Context, orgID uuid.UUID) (*DraftChangeSet, error) {
	blob, ver, updatedAt, err := s.readDraftBlob(ctx, s.pool, orgID)
	if err != nil {
		return nil, err
	}
	rows := draftRowsFromBlob(orgID, blob, ver, updatedAt)
	out := &DraftChangeSet{
		BaseVersion: ver,
		UpdatedAt:   updatedAt,
		Topics:      rows.Topics,
		Tariffs:     rows.Tariffs,
		Products:    rows.Products,
		Contacts:    rows.Contacts,
		Policies:    rows.Policies,
		Zones:       rows.Zones,
		Deletes:     make([]DraftChangeDelete, 0, len(blob.Deletes)),
	}
	if blob.Config.hasPending() {
		cfg := blob.Config
		out.Config = &cfg
	}
	for _, d := range blob.Deletes {
		out.Deletes = append(out.Deletes, DraftChangeDelete{Kind: PluralChangeKind(d.Kind), Key: d.Key})
	}
	return out, nil
}

// mergedView loads live rows and overlays the given blob's pending entries —
// shared by Draft (a real blob) and LiveView (an always-empty blob, so every
// overlay loop below is a no-op and every row stays Draft:false). It fills
// everything except Materials/Requests, which only Draft queries (LiveView has
// no playground concept of them).
func (s *Store) mergedView(ctx context.Context, db dbtx, orgID uuid.UUID, blob DraftBlob, ver int64, updatedAt time.Time) (*DraftView, error) {
	v := &DraftView{Config: DraftConfig{OrganizationID: orgID, ReplyMaxWords: 120, BaseVersion: ver, UpdatedAt: updatedAt}}
	err := db.QueryRow(ctx, `SELECT persona, mission, guardrails, language_policy, reply_max_words
		FROM xchats.ai_assistants WHERE organization_id = $1`, orgID).
		Scan(&v.Config.Persona, &v.Config.Mission, &v.Config.Guardrails, &v.Config.LanguagePolicy, &v.Config.ReplyMaxWords)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	if p := blob.Config.Persona; p != nil {
		v.Config.Persona, v.Config.Draft = *p, true
	}
	if p := blob.Config.Mission; p != nil {
		v.Config.Mission, v.Config.Draft = *p, true
	}
	if p := blob.Config.Guardrails; p != nil {
		v.Config.Guardrails, v.Config.Draft = *p, true
	}
	if p := blob.Config.LanguagePolicy; p != nil {
		v.Config.LanguagePolicy, v.Config.Draft = *p, true
	}
	if p := blob.Config.ReplyMaxWords; p != nil {
		v.Config.ReplyMaxWords, v.Config.Draft = *p, true
	}

	deleted := map[string]bool{}
	for _, d := range blob.Deletes {
		deleted[d.Kind+":"+d.Key] = true
	}

	// topics
	topicIdx := map[string]int{}
	trows, err := db.Query(ctx, `SELECT slug, title, body_md, featured_image, illustration_images,
		explainer_videos, reference_documents, updated_at
		FROM xchats.ai_topics WHERE organization_id = $1 ORDER BY created_at`, orgID)
	if err != nil {
		return nil, err
	}
	for trows.Next() {
		var t TopicRow
		if err := trows.Scan(&t.Slug, &t.Title, &t.BodyMD, &t.FeaturedImage, &t.IllustrationImages,
			&t.ExplainerVideos, &t.ReferenceDocuments, &t.UpdatedAt); err != nil {
			trows.Close()
			return nil, err
		}
		t.ID = t.Slug
		v.Topics = append(v.Topics, t)
		topicIdx[t.Slug] = len(v.Topics) - 1
	}
	trows.Close()
	if err := trows.Err(); err != nil {
		return nil, err
	}
	for _, bt := range blob.Topics {
		row := TopicRow{ID: bt.Slug, Slug: bt.Slug, Title: bt.Title, BodyMD: bt.BodyMD,
			FeaturedImage: bt.FeaturedImage, IllustrationImages: bt.IllustrationImages,
			ExplainerVideos:    bt.ExplainerVideos,
			ReferenceDocuments: bt.ReferenceDocuments,
			Draft:              true, UpdatedAt: updatedAt}
		if i, ok := topicIdx[bt.Slug]; ok {
			v.Topics[i] = row
		} else {
			v.Topics = append(v.Topics, row)
			topicIdx[bt.Slug] = len(v.Topics) - 1
		}
	}
	v.Topics = filterTopics(v.Topics, deleted)

	// tariffs
	tariffIdx := map[string]int{}
	trrows, err := db.Query(ctx, `SELECT ref, name, price, limit_text, fee, summary, pricing_type, advantages,
		disadvantages, sales_status, featured_image, pricing_images, explainer_videos, terms_documents, updated_at
		FROM xchats.ai_tariffs WHERE organization_id = $1 ORDER BY created_at`, orgID)
	if err != nil {
		return nil, err
	}
	for trrows.Next() {
		var t TariffRow
		if err := trrows.Scan(&t.Ref, &t.Name, &t.Price, &t.LimitText, &t.Fee, &t.Summary, &t.PricingType, &t.Advantages,
			&t.Disadvantages, &t.SalesStatus, &t.FeaturedImage, &t.PricingImages, &t.ExplainerVideos, &t.TermsDocuments,
			&t.UpdatedAt); err != nil {
			trrows.Close()
			return nil, err
		}
		t.ID = t.Ref
		v.Tariffs = append(v.Tariffs, t)
		tariffIdx[t.Ref] = len(v.Tariffs) - 1
	}
	trrows.Close()
	if err := trrows.Err(); err != nil {
		return nil, err
	}
	for _, bt := range blob.Tariffs {
		row := TariffRow{ID: bt.Ref, Ref: bt.Ref, Name: bt.Name, Price: bt.Price, LimitText: bt.LimitText,
			Fee: bt.Fee, Summary: bt.Summary, PricingType: bt.PricingType, Advantages: bt.Advantages,
			Disadvantages: bt.Disadvantages, SalesStatus: bt.SalesStatus, FeaturedImage: bt.FeaturedImage,
			PricingImages: bt.PricingImages, ExplainerVideos: bt.ExplainerVideos, TermsDocuments: bt.TermsDocuments,
			Draft: true, UpdatedAt: updatedAt}
		if i, ok := tariffIdx[bt.Ref]; ok {
			v.Tariffs[i] = row
		} else {
			v.Tariffs = append(v.Tariffs, row)
			tariffIdx[bt.Ref] = len(v.Tariffs) - 1
		}
	}
	kt := v.Tariffs[:0]
	for _, t := range v.Tariffs {
		if !deleted["tariff:"+t.Ref] {
			kt = append(kt, t)
		}
	}
	v.Tariffs = kt

	// products (availability is a dead legacy column — not selected)
	productIdx := map[string]int{}
	prows, err := db.Query(ctx, `SELECT ref, name, price, description, category, in_stock, sales_status,
		featured_image, gallery_images, demo_videos, certificate_documents, guarantee_documents, updated_at
		FROM xchats.ai_products WHERE organization_id = $1 ORDER BY created_at`, orgID)
	if err != nil {
		return nil, err
	}
	for prows.Next() {
		var p ProductRow
		if err := prows.Scan(&p.Ref, &p.Name, &p.Price, &p.Description, &p.Category, &p.InStock, &p.SalesStatus,
			&p.FeaturedImage, &p.GalleryImages, &p.DemoVideos, &p.CertificateDocuments,
			&p.GuaranteeDocuments, &p.UpdatedAt); err != nil {
			prows.Close()
			return nil, err
		}
		p.ID = p.Ref
		v.Products = append(v.Products, p)
		productIdx[p.Ref] = len(v.Products) - 1
	}
	prows.Close()
	if err := prows.Err(); err != nil {
		return nil, err
	}
	for _, bp := range blob.Products {
		row := ProductRow{ID: bp.Ref, Ref: bp.Ref, Name: bp.Name, Price: bp.Price,
			Description: bp.Description, Category: bp.Category, InStock: bp.InStock, SalesStatus: bp.SalesStatus,
			FeaturedImage: bp.FeaturedImage, GalleryImages: bp.GalleryImages, DemoVideos: bp.DemoVideos,
			CertificateDocuments: bp.CertificateDocuments, GuaranteeDocuments: bp.GuaranteeDocuments,
			Draft: true, UpdatedAt: updatedAt}
		if i, ok := productIdx[bp.Ref]; ok {
			v.Products[i] = row
		} else {
			v.Products = append(v.Products, row)
			productIdx[bp.Ref] = len(v.Products) - 1
		}
	}
	kp := v.Products[:0]
	for _, p := range v.Products {
		if !deleted["product:"+p.Ref] {
			kp = append(kp, p)
		}
	}
	v.Products = kp

	// contacts — a true singleton: at most one live row, at most one pending
	// blob entry, both keyed by nothing but the org.
	crows, err := db.Query(ctx, `SELECT whatsapp, email, address, legal_information, callback_time,
		working_hours, phone, website, instagram, contact_card_image, location_map_image,
		company_legal_documents, updated_at
		FROM xchats.ai_contacts WHERE organization_id = $1 ORDER BY created_at`, orgID)
	if err != nil {
		return nil, err
	}
	for crows.Next() {
		var c ContactRow
		var legalInfo *string
		if err := crows.Scan(&c.WhatsApp, &c.Email, &c.Address, &legalInfo, &c.CallbackTime,
			&c.WorkingHours, &c.Phone, &c.Website, &c.Instagram, &c.ContactCardImage, &c.LocationMapImage,
			&c.CompanyLegalDocuments, &c.UpdatedAt); err != nil {
			crows.Close()
			return nil, err
		}
		c.LegalInformation = strOrEmpty(legalInfo)
		c.ID, c.Slug = domain.ContactSlug, domain.ContactSlug
		v.Contacts = append(v.Contacts, c)
	}
	crows.Close()
	if err := crows.Err(); err != nil {
		return nil, err
	}
	if len(blob.Contacts) > 0 {
		bc := blob.Contacts[0]
		row := ContactRow{ID: domain.ContactSlug, Slug: domain.ContactSlug, WhatsApp: bc.WhatsApp, Email: bc.Email,
			Address: bc.Address, LegalInformation: bc.LegalInformation, CallbackTime: bc.CallbackTime,
			WorkingHours: bc.WorkingHours, Phone: bc.Phone, Website: bc.Website, Instagram: bc.Instagram,
			ContactCardImage: bc.ContactCardImage, LocationMapImage: bc.LocationMapImage,
			CompanyLegalDocuments: bc.CompanyLegalDocuments,
			Draft:                 true, UpdatedAt: updatedAt}
		if len(v.Contacts) > 0 {
			v.Contacts[0] = row
		} else {
			v.Contacts = append(v.Contacts, row)
		}
	}
	if deletedSingleton(blob, "contact") {
		v.Contacts = nil
	}

	// policies — an exact clone of the contacts section above (singleton
	// table, slug domain.PolicySlug).
	polrows, err := db.Query(ctx, `SELECT delivery_cost, delivery_in_days, free_delivery_from, min_order,
		prepayment, installment, return_period_in_days, warranty, outside_zones_note,
		commerce_policy_documents, updated_at
		FROM xchats.ai_policies WHERE organization_id = $1 ORDER BY created_at`, orgID)
	if err != nil {
		return nil, err
	}
	for polrows.Next() {
		var p PolicyRow
		var deliveryInDays, returnPeriodInDays *string
		if err := polrows.Scan(&p.DeliveryCost, &deliveryInDays, &p.FreeDeliveryFrom, &p.MinOrder,
			&p.Prepayment, &p.Installment, &returnPeriodInDays, &p.Warranty, &p.OutsideZonesNote,
			&p.CommercePolicyDocuments, &p.UpdatedAt); err != nil {
			polrows.Close()
			return nil, err
		}
		p.DeliveryInDays = strOrEmpty(deliveryInDays)
		p.ReturnPeriodInDays = strOrEmpty(returnPeriodInDays)
		p.ID, p.Slug = domain.PolicySlug, domain.PolicySlug
		v.Policies = append(v.Policies, p)
	}
	polrows.Close()
	if err := polrows.Err(); err != nil {
		return nil, err
	}
	if len(blob.Policies) > 0 {
		bp := blob.Policies[0]
		row := PolicyRow{ID: domain.PolicySlug, Slug: domain.PolicySlug, DeliveryCost: bp.DeliveryCost,
			DeliveryInDays: bp.DeliveryInDays, FreeDeliveryFrom: bp.FreeDeliveryFrom, MinOrder: bp.MinOrder,
			Prepayment: bp.Prepayment, Installment: bp.Installment, ReturnPeriodInDays: bp.ReturnPeriodInDays, Warranty: bp.Warranty,
			OutsideZonesNote: bp.OutsideZonesNote, CommercePolicyDocuments: bp.CommercePolicyDocuments,
			Draft: true, UpdatedAt: updatedAt}
		if len(v.Policies) > 0 {
			v.Policies[0] = row
		} else {
			v.Policies = append(v.Policies, row)
		}
	}
	if deletedSingleton(blob, "policy") {
		v.Policies = nil
	}

	// zones — live rows overlaid by pending blob.DeliveryZones entries, same
	// pattern as tariffs/products above (no media columns in v1).
	zoneIdx := map[string]int{}
	liveZones, err := loadZoneRows(ctx, db, orgID)
	if err != nil {
		return nil, err
	}
	for _, z := range liveZones {
		v.Zones = append(v.Zones, z)
		zoneIdx[z.Ref] = len(v.Zones) - 1
	}
	for _, bz := range blob.DeliveryZones {
		row := ZoneRow{ID: bz.Ref, Ref: bz.Ref, Name: bz.Name, ZoneLevel: bz.ZoneLevel, ParentRef: bz.ParentRef,
			DeliveryAvailable: bz.DeliveryAvailable, DeliveryCost: bz.DeliveryCost, DeliveryInDays: bz.DeliveryInDays,
			Notes: bz.Notes, SalesStatus: orDefault(bz.SalesStatus, "active"),
			Draft: true, UpdatedAt: updatedAt}
		if i, ok := zoneIdx[bz.Ref]; ok {
			v.Zones[i] = row
		} else {
			v.Zones = append(v.Zones, row)
			zoneIdx[bz.Ref] = len(v.Zones) - 1
		}
	}
	kz := v.Zones[:0]
	for _, z := range v.Zones {
		if !deleted["delivery_zone:"+z.Ref] {
			kz = append(kz, z)
		}
	}
	v.Zones = kz

	// Guarantee non-nil slices: every collection must serialize as a JSON array
	// ([]), never null. A nil slice (empty table + empty blob) marshals to null,
	// and the client reads d.<coll>.length directly — a null would crash the page.
	if v.Zones == nil {
		v.Zones = []ZoneRow{}
	}
	if v.Topics == nil {
		v.Topics = []TopicRow{}
	}
	if v.Tariffs == nil {
		v.Tariffs = []TariffRow{}
	}
	if v.Products == nil {
		v.Products = []ProductRow{}
	}
	if v.Contacts == nil {
		v.Contacts = []ContactRow{}
	}
	if v.Policies == nil {
		v.Policies = []PolicyRow{}
	}
	return v, nil
}

func filterTopics(in []TopicRow, deleted map[string]bool) []TopicRow {
	out := in[:0]
	for _, t := range in {
		if !deleted["topic:"+t.Slug] {
			out = append(out, t)
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// Draft CRUD — each mutates the blob under a row lock and records its actor
// as kbd_draft.updated_by (DECISIONS.md: "Last operator who changed the
// draft") — the authenticated user for the Playground editor, the verified
// Principal.UserID for an MCP caller.
// ---------------------------------------------------------------------------

// ConfigPatch carries optional config edits (nil pointer = leave unchanged).
type ConfigPatch struct {
	Persona        *string
	Mission        *string
	Guardrails     *string
	LanguagePolicy *string
	ReplyMaxWords  *int
}

// PatchConfig stages config edits in the draft blob (only non-nil fields).
func (s *Store) PatchConfig(ctx context.Context, orgID uuid.UUID, actor uuid.UUID, p ConfigPatch) error {
	return s.writeDraftBlob(ctx, orgID, actor, func(db dbtx, b *DraftBlob) error {
		if p.Persona != nil {
			b.Config.Persona = p.Persona
		}
		if p.Mission != nil {
			b.Config.Mission = p.Mission
		}
		if p.Guardrails != nil {
			b.Config.Guardrails = p.Guardrails
		}
		if p.LanguagePolicy != nil {
			b.Config.LanguagePolicy = p.LanguagePolicy
		}
		if p.ReplyMaxWords != nil {
			b.Config.ReplyMaxWords = p.ReplyMaxWords
		}
		return nil
	})
}

// TopicInput is an upsert payload for a draft topic.
type TopicInput struct {
	Slug, Title, BodyMD string
}

// UpsertTopic stages a topic create/update in the draft blob, by slug. Starts
// from the topic's current merged shape (currentTopic) so this text-only
// caller — the Playground editor, which has no media inputs — can never
// blank out media an MCP tool already staged on the same topic.
func (s *Store) UpsertTopic(ctx context.Context, orgID uuid.UUID, actor uuid.UUID, in TopicInput) error {
	return s.writeDraftBlob(ctx, orgID, actor, func(db dbtx, b *DraftBlob) error {
		cur, err := s.currentTopic(ctx, db, orgID, in.Slug, b)
		if err != nil {
			return err
		}
		cur.Title, cur.BodyMD = in.Title, in.BodyMD
		b.upsertTopic(cur)
		return nil
	})
}

// DeleteTopic stages a topic removal by slug (drops any pending edit and marks
// the live row, if any, for deletion at approve).
func (s *Store) DeleteTopic(ctx context.Context, orgID uuid.UUID, actor uuid.UUID, slug string) error {
	return s.writeDraftBlob(ctx, orgID, actor, func(db dbtx, b *DraftBlob) error {
		b.removeTopic(slug)
		b.addDelete("topic", slug)
		return nil
	})
}

// --- typed facts: tariffs / products / contacts -----------------------------

// TariffInput is an upsert payload for a draft tariff.
type TariffInput struct {
	Ref, Name, Price, LimitText, Fee, Summary, PricingType, Advantages, Disadvantages, SalesStatus string
}

// UpsertTariff stages a tariff create/update in the draft blob, by ref.
// Merges onto the tariff's current shape (currentTariff) so this text-only
// caller never blanks out media an MCP tool already staged.
func (s *Store) UpsertTariff(ctx context.Context, orgID uuid.UUID, actor uuid.UUID, in TariffInput) error {
	return s.writeDraftBlob(ctx, orgID, actor, func(db dbtx, b *DraftBlob) error {
		cur, err := s.currentTariff(ctx, db, orgID, in.Ref, b)
		if err != nil {
			return err
		}
		cur.Name, cur.Price, cur.LimitText = in.Name, in.Price, in.LimitText
		cur.Fee, cur.Summary = in.Fee, in.Summary
		cur.PricingType = orDefault(in.PricingType, "fixed")
		cur.Advantages, cur.Disadvantages = in.Advantages, in.Disadvantages
		cur.SalesStatus = orDefault(in.SalesStatus, "active")
		b.upsertTariff(cur)
		return nil
	})
}

// DeleteTariff stages removal of a tariff by ref.
func (s *Store) DeleteTariff(ctx context.Context, orgID uuid.UUID, actor uuid.UUID, ref string) error {
	return s.writeDraftBlob(ctx, orgID, actor, func(db dbtx, b *DraftBlob) error {
		b.removeTariff(ref)
		b.addDelete("tariff", ref)
		return nil
	})
}

// ProductInput is an upsert payload for a draft product. InStock is nil-able:
// nil leaves the column at its current merged value, matching the same
// nil-means-unchanged idiom PutLiveProduct uses (live.go).
type ProductInput struct {
	Ref, Name, Price, Description, Category, SalesStatus string
	InStock                                              *bool
}

// UpsertProduct stages a product create/update in the draft blob, by ref.
// Merges onto the product's current shape (currentProduct) so this caller
// cannot blank out media an MCP tool already staged.
func (s *Store) UpsertProduct(ctx context.Context, orgID uuid.UUID, actor uuid.UUID, in ProductInput) error {
	return s.writeDraftBlob(ctx, orgID, actor, func(db dbtx, b *DraftBlob) error {
		cur, err := s.currentProduct(ctx, db, orgID, in.Ref, b)
		if err != nil {
			return err
		}
		cur.Name, cur.Price = in.Name, in.Price
		cur.Description, cur.Category = in.Description, in.Category
		cur.SalesStatus = orDefault(in.SalesStatus, "active")
		if in.InStock != nil {
			cur.InStock = *in.InStock
		}
		b.upsertProduct(cur)
		return nil
	})
}

// DeleteProduct stages removal of a product by ref.
func (s *Store) DeleteProduct(ctx context.Context, orgID uuid.UUID, actor uuid.UUID, ref string) error {
	return s.writeDraftBlob(ctx, orgID, actor, func(db dbtx, b *DraftBlob) error {
		b.removeProduct(ref)
		b.addDelete("product", ref)
		return nil
	})
}

// DeliveryZoneInput is an upsert payload for a draft delivery zone — the
// Playground/draft counterpart to zones.go's live-only ZoneInput.
type DeliveryZoneInput struct {
	Ref, Name, ZoneLevel, ParentRef, DeliveryCost, DeliveryInDays, Notes, SalesStatus string
	DeliveryAvailable                                                                bool
}

// UpsertZone stages a delivery-zone create/update in the draft blob, by ref.
// Merges onto the zone's current shape (currentZone) so this caller never
// blanks out fields an MCP tool already staged.
func (s *Store) UpsertZone(ctx context.Context, orgID uuid.UUID, actor uuid.UUID, in DeliveryZoneInput) error {
	return s.writeDraftBlob(ctx, orgID, actor, func(db dbtx, b *DraftBlob) error {
		cur, err := s.currentZone(ctx, db, orgID, in.Ref, b)
		if err != nil {
			return err
		}
		cur.Name, cur.ZoneLevel, cur.ParentRef = in.Name, in.ZoneLevel, in.ParentRef
		cur.DeliveryAvailable = in.DeliveryAvailable
		cur.DeliveryCost, cur.DeliveryInDays, cur.Notes = in.DeliveryCost, in.DeliveryInDays, in.Notes
		cur.SalesStatus = orDefault(in.SalesStatus, "active")
		b.upsertZone(cur)
		return nil
	})
}

// DeleteZone stages removal of a delivery zone by ref.
func (s *Store) DeleteZone(ctx context.Context, orgID uuid.UUID, actor uuid.UUID, ref string) error {
	return s.writeDraftBlob(ctx, orgID, actor, func(db dbtx, b *DraftBlob) error {
		b.removeZone(ref)
		b.addDelete("delivery_zone", ref)
		return nil
	})
}

// ContactPatch carries optional edits to the org's singleton contact row (nil
// = leave unchanged).
type ContactPatch struct {
	WhatsApp         *string
	Email            *string
	Address          *string
	LegalInformation *string
	CallbackTime     *string
	WorkingHours     *string
	Phone            *string
	Website          *string
	Instagram        *string
}

// PatchContacts stages an edit to the org's singleton support-contact row,
// starting from its current merged shape so omitted fields stay unchanged.
func (s *Store) PatchContacts(ctx context.Context, orgID uuid.UUID, actor uuid.UUID, p ContactPatch) error {
	return s.writeDraftBlob(ctx, orgID, actor, func(db dbtx, b *DraftBlob) error {
		cur, err := s.currentContact(ctx, db, orgID, b)
		if err != nil {
			return err
		}
		if p.WhatsApp != nil {
			cur.WhatsApp = *p.WhatsApp
		}
		if p.Email != nil {
			cur.Email = *p.Email
		}
		if p.Address != nil {
			cur.Address = *p.Address
		}
		if p.LegalInformation != nil {
			cur.LegalInformation = *p.LegalInformation
		}
		if p.CallbackTime != nil {
			cur.CallbackTime = *p.CallbackTime
		}
		if p.WorkingHours != nil {
			cur.WorkingHours = *p.WorkingHours
		}
		if p.Phone != nil {
			cur.Phone = *p.Phone
		}
		if p.Website != nil {
			cur.Website = *p.Website
		}
		if p.Instagram != nil {
			cur.Instagram = *p.Instagram
		}
		b.upsertContact(cur)
		return nil
	})
}

// PolicyPatch carries optional edits to the org's singleton commerce-policy
// row (nil = leave) — a structural clone of ContactPatch.
type PolicyPatch struct {
	DeliveryCost       *string
	DeliveryInDays     *string
	FreeDeliveryFrom   *string
	MinOrder           *string
	Prepayment         *string
	Installment        *string
	ReturnPeriodInDays *string
	Warranty           *string
	OutsideZonesNote   *string
}

// PatchPolicies stages an edit to the org's singleton commerce-policy row,
// starting from its current merged shape so omitted fields stay unchanged — an
// exact clone of PatchContacts.
func (s *Store) PatchPolicies(ctx context.Context, orgID uuid.UUID, actor uuid.UUID, p PolicyPatch) error {
	return s.writeDraftBlob(ctx, orgID, actor, func(db dbtx, b *DraftBlob) error {
		cur, err := s.currentPolicy(ctx, db, orgID, b)
		if err != nil {
			return err
		}
		if p.DeliveryCost != nil {
			cur.DeliveryCost = *p.DeliveryCost
		}
		if p.DeliveryInDays != nil {
			cur.DeliveryInDays = *p.DeliveryInDays
		}
		if p.FreeDeliveryFrom != nil {
			cur.FreeDeliveryFrom = *p.FreeDeliveryFrom
		}
		if p.MinOrder != nil {
			cur.MinOrder = *p.MinOrder
		}
		if p.Prepayment != nil {
			cur.Prepayment = *p.Prepayment
		}
		if p.Installment != nil {
			cur.Installment = *p.Installment
		}
		if p.ReturnPeriodInDays != nil {
			cur.ReturnPeriodInDays = *p.ReturnPeriodInDays
		}
		if p.Warranty != nil {
			cur.Warranty = *p.Warranty
		}
		if p.OutsideZonesNote != nil {
			cur.OutsideZonesNote = *p.OutsideZonesNote
		}
		b.upsertPolicy(cur)
		return nil
	})
}

// SetFactField upserts a SINGLE field on a typed fact (tariff/product/contact),
// starting from the entity's current merged shape so the other columns are
// preserved. This is the confirm_fact write path: a detected price is confirmed
// into e.g. tariff <slug>.price without blanking the rest of the row.
func (s *Store) SetFactField(ctx context.Context, orgID uuid.UUID, actor uuid.UUID, table, slug, field, value string) error {
	switch table {
	case "tariff":
		return s.writeDraftBlob(ctx, orgID, actor, func(db dbtx, b *DraftBlob) error {
			cur, err := s.currentTariff(ctx, db, orgID, slug, b)
			if err != nil {
				return err
			}
			if !setTariffField(&cur, field, value) {
				return ErrUnknownKind
			}
			b.upsertTariff(cur)
			return nil
		})
	case "product":
		return s.writeDraftBlob(ctx, orgID, actor, func(db dbtx, b *DraftBlob) error {
			cur, err := s.currentProduct(ctx, db, orgID, slug, b)
			if err != nil {
				return err
			}
			if !setProductField(&cur, field, value) {
				return ErrUnknownKind
			}
			b.upsertProduct(cur)
			return nil
		})
	case "contact":
		p := ContactPatch{}
		if !setContactPatchField(&p, field, value) {
			return ErrUnknownKind
		}
		return s.PatchContacts(ctx, orgID, actor, p)
	case "policy":
		p := PolicyPatch{}
		if !setPolicyPatchField(&p, field, value) {
			return ErrUnknownKind
		}
		return s.PatchPolicies(ctx, orgID, actor, p)
	}
	return ErrUnknownKind
}

func setTariffField(t *DraftTariff, field, value string) bool {
	switch field {
	case "name":
		t.Name = value
	case "price":
		t.Price = value
	case "limit_text":
		t.LimitText = value
	case "fee":
		t.Fee = value
	case "summary":
		t.Summary = value
	default:
		return false
	}
	return true
}

func setProductField(p *DraftProduct, field, value string) bool {
	switch field {
	case "name":
		p.Name = value
	case "price":
		p.Price = value
	case "description":
		p.Description = value
	case "category":
		p.Category = value
	default:
		return false
	}
	return true
}

func setContactPatchField(p *ContactPatch, field, value string) bool {
	switch field {
	case "whatsapp":
		p.WhatsApp = &value
	case "email":
		p.Email = &value
	case "address":
		p.Address = &value
	case "legal_information":
		p.LegalInformation = &value
	case "callback_time":
		p.CallbackTime = &value
	case "working_hours":
		p.WorkingHours = &value
	case "phone":
		p.Phone = &value
	case "website":
		p.Website = &value
	case "instagram":
		p.Instagram = &value
	default:
		return false
	}
	return true
}

func setPolicyPatchField(p *PolicyPatch, field, value string) bool {
	switch field {
	case "delivery_cost":
		p.DeliveryCost = &value
	case "delivery_in_days":
		p.DeliveryInDays = &value
	case "free_delivery_from":
		p.FreeDeliveryFrom = &value
	case "min_order":
		p.MinOrder = &value
	case "prepayment":
		p.Prepayment = &value
	case "installment":
		p.Installment = &value
	case "return_period_in_days":
		p.ReturnPeriodInDays = &value
	case "warranty":
		p.Warranty = &value
	default:
		return false
	}
	return true
}

// currentTopic / currentTariff / currentProduct resolve the merged current
// shape of an entity: the pending blob entry, else the COMPLETE live row
// (every canonical column, including media/sales_status/in_stock), else a
// blank scaffold. Every caller that stages a draft entry — the legacy
// whole-row Upsert* methods below, SetFactField, and the MCP patch-based
// upserts (mcp.go) — starts here, so a partial edit never blanks out a
// field it did not intend to touch.
func (s *Store) currentTopic(ctx context.Context, db dbtx, orgID uuid.UUID, slug string, b *DraftBlob) (DraftTopic, error) {
	for _, t := range b.Topics {
		if t.Slug == slug {
			return t, nil
		}
	}
	var t DraftTopic
	err := db.QueryRow(ctx, `SELECT slug, title, body_md, featured_image, illustration_images,
		explainer_videos, reference_documents
		FROM xchats.ai_topics WHERE organization_id=$1 AND slug=$2`, orgID, slug).
		Scan(&t.Slug, &t.Title, &t.BodyMD, &t.FeaturedImage, &t.IllustrationImages,
			&t.ExplainerVideos, &t.ReferenceDocuments)
	if errors.Is(err, pgx.ErrNoRows) {
		return DraftTopic{Slug: slug}, nil
	}
	return t, err
}

func (s *Store) currentTariff(ctx context.Context, db dbtx, orgID uuid.UUID, ref string, b *DraftBlob) (DraftTariff, error) {
	for _, t := range b.Tariffs {
		if t.Ref == ref {
			return t, nil
		}
	}
	var t DraftTariff
	err := db.QueryRow(ctx, `SELECT ref, name, price, limit_text, fee, summary, pricing_type, advantages,
		disadvantages, sales_status, featured_image, pricing_images, explainer_videos, terms_documents
		FROM xchats.ai_tariffs WHERE organization_id=$1 AND ref=$2`, orgID, ref).
		Scan(&t.Ref, &t.Name, &t.Price, &t.LimitText, &t.Fee, &t.Summary, &t.PricingType, &t.Advantages,
			&t.Disadvantages, &t.SalesStatus, &t.FeaturedImage, &t.PricingImages, &t.ExplainerVideos, &t.TermsDocuments)
	if errors.Is(err, pgx.ErrNoRows) {
		return DraftTariff{Ref: ref, PricingType: "fixed"}, nil
	}
	return t, err
}

func (s *Store) currentProduct(ctx context.Context, db dbtx, orgID uuid.UUID, ref string, b *DraftBlob) (DraftProduct, error) {
	for _, p := range b.Products {
		if p.Ref == ref {
			return p, nil
		}
	}
	var p DraftProduct
	err := db.QueryRow(ctx, `SELECT ref, name, price, description, category, in_stock, sales_status,
		featured_image, gallery_images, demo_videos, certificate_documents, guarantee_documents
		FROM xchats.ai_products WHERE organization_id=$1 AND ref=$2`, orgID, ref).
		Scan(&p.Ref, &p.Name, &p.Price, &p.Description, &p.Category, &p.InStock, &p.SalesStatus,
			&p.FeaturedImage, &p.GalleryImages, &p.DemoVideos, &p.CertificateDocuments, &p.GuaranteeDocuments)
	if errors.Is(err, pgx.ErrNoRows) {
		return DraftProduct{Ref: ref, InStock: true}, nil
	}
	return p, err
}

// currentZone resolves the merged current shape of a delivery zone — the
// same pattern as currentTariff/currentProduct, over ai_delivery_zones.
func (s *Store) currentZone(ctx context.Context, db dbtx, orgID uuid.UUID, ref string, b *DraftBlob) (DraftDeliveryZone, error) {
	for _, z := range b.DeliveryZones {
		if z.Ref == ref {
			return z, nil
		}
	}
	var z DraftDeliveryZone
	err := db.QueryRow(ctx, `SELECT ref, name, zone_level, parent_ref, delivery_available, delivery_cost,
		delivery_in_days, notes, sales_status
		FROM xchats.ai_delivery_zones WHERE organization_id=$1 AND ref=$2`, orgID, ref).
		Scan(&z.Ref, &z.Name, &z.ZoneLevel, &z.ParentRef, &z.DeliveryAvailable, &z.DeliveryCost,
			&z.DeliveryInDays, &z.Notes, &z.SalesStatus)
	if errors.Is(err, pgx.ErrNoRows) {
		return DraftDeliveryZone{Ref: ref}, nil
	}
	return z, err
}

// currentContact resolves the merged current shape of the org's singleton
// contact row: the pending blob entry if one exists, else the live row, else
// a blank scaffold.
func (s *Store) currentContact(ctx context.Context, db dbtx, orgID uuid.UUID, b *DraftBlob) (DraftContact, error) {
	if len(b.Contacts) > 0 {
		return b.Contacts[0], nil
	}
	var c DraftContact
	var legalInfo *string
	err := db.QueryRow(ctx, `SELECT whatsapp, email, address, legal_information, callback_time,
		working_hours, phone, website, instagram, contact_card_image, location_map_image,
		company_legal_documents
		FROM xchats.ai_contacts WHERE organization_id = $1`, orgID).
		Scan(&c.WhatsApp, &c.Email, &c.Address, &legalInfo, &c.CallbackTime,
			&c.WorkingHours, &c.Phone, &c.Website, &c.Instagram, &c.ContactCardImage, &c.LocationMapImage,
			&c.CompanyLegalDocuments)
	if errors.Is(err, pgx.ErrNoRows) {
		return DraftContact{}, nil
	}
	c.LegalInformation = strOrEmpty(legalInfo)
	return c, err
}

// currentPolicy resolves the merged current shape of the org's singleton
// commerce-policy row — an exact clone of currentContact.
func (s *Store) currentPolicy(ctx context.Context, db dbtx, orgID uuid.UUID, b *DraftBlob) (DraftPolicy, error) {
	if len(b.Policies) > 0 {
		return b.Policies[0], nil
	}
	var p DraftPolicy
	var deliveryInDays, returnPeriodInDays *string
	err := db.QueryRow(ctx, `SELECT delivery_cost, delivery_in_days, free_delivery_from, min_order,
		prepayment, installment, return_period_in_days, warranty, outside_zones_note, commerce_policy_documents
		FROM xchats.ai_policies WHERE organization_id = $1`, orgID).
		Scan(&p.DeliveryCost, &deliveryInDays, &p.FreeDeliveryFrom, &p.MinOrder,
			&p.Prepayment, &p.Installment, &returnPeriodInDays, &p.Warranty, &p.OutsideZonesNote,
			&p.CommercePolicyDocuments)
	if errors.Is(err, pgx.ErrNoRows) {
		return DraftPolicy{}, nil
	}
	p.DeliveryInDays = strOrEmpty(deliveryInDays)
	p.ReturnPeriodInDays = strOrEmpty(returnPeriodInDays)
	return p, err
}

func orDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

// ---------------------------------------------------------------------------
// Approve — validate → materialize into live tables → clear from the blob (15
// Decision 4). The ONLY write path to live.
// ---------------------------------------------------------------------------

// ApproveSelector picks what to materialize: a zero-value selector (Kind=="")
// selects the WHOLE draft; a non-empty kind+key picks one entity. For the
// singleton contacts/policies kinds, Key is the fixed
// domain.ContactSlug/domain.PolicySlug constant (there is nothing else to key
// on — the natural key IS the org).
type ApproveSelector struct {
	Kind string // "" | "topics" | "tariffs" | "products" | "contacts" | "policies" | "delivery_zones" | "config"
	Key  string // slug | ref | ref | domain.ContactSlug | domain.PolicySlug | ref | NaturalKeyMain
}

type approveSet struct {
	topics   []DraftTopic
	tariffs  []DraftTariff
	products []DraftProduct
	contacts []DraftContact
	policies []DraftPolicy
	zones    []DraftDeliveryZone
	config   bool // a pending assistant-config edit is targeted by this selector
	deletes  []DraftDelete
}

func (a approveSet) empty() bool {
	return len(a.topics)+len(a.tariffs)+len(a.products)+len(a.contacts)+len(a.policies)+len(a.zones)+len(a.deletes) == 0 && !a.config
}

// errApproveNothingPending is an internal control-flow sentinel:
// selectApproved found nothing to do for an entity-scoped approve (sel.Kind
// != ""). ApproveVersioned's mutate closure returns it to make
// writeDraftBlobVersioned roll back without bumping base_version — a call
// that materializes nothing should not consume a version, matching the
// pre-existing "idempotent no-op" behavior. The wrapper below translates it
// back to a plain nil error; it never escapes this file.
var errApproveNothingPending = errors.New("kbstore: nothing pending for that selector")

// Approve is ApproveVersioned with no optimistic-concurrency check and no
// audit actor — the shape every pre-existing caller (tests,
// internal/playground, and any future non-HTTP caller) uses.
func (s *Store) Approve(ctx context.Context, orgID uuid.UUID, sel ApproveSelector) error {
	return s.ApproveVersioned(ctx, orgID, sel, nil, uuid.Nil)
}

// ApproveVersioned validates the resulting live set against the
// deterministic gate (including the zone/policy exclusivity invariant
// zoneGateReasons enforces), then materializes the selection into the live
// typed tables on their natural key, applies matching deletes, appends an
// audit-log row, and clears exactly the approved entries from the draft —
// all inside the SAME transaction writeDraftBlobVersioned opens and
// row-locks kbd_draft under for its whole duration.
//
// This closes a real data-loss bug the previous version had: it read the
// blob UNLOCKED (a plain s.pool query, no FOR UPDATE), computed the gate and
// materialized into live in one transaction, and only THEN cleared the
// approved entries from the blob in a SECOND, separately-committed
// transaction — by natural key, not by the value that was actually
// approved. A concurrent MCP write landing in the gap between the first read
// and the second commit was either published as its now-stale value and
// then deleted outright (an MCP update racing an approve of the same
// entity), or — for a brand-new entity — deleted having never been
// published at all (an MCP create racing a whole-draft approve). Folding
// everything into writeDraftBlobVersioned's single locked transaction makes
// that gap impossible: nothing can observe or mutate the draft between this
// function's read and its own commit, because both happen under the same
// row lock.
//
// expectedVersion, when non-nil, must equal the draft's CURRENT
// base_version or the whole call fails with ErrStale (plan/mcp.md's
// optimistic concurrency, extended to the browser-facing approve endpoints
// via If-Match — the check now runs atomically inside the same locked
// transaction, unlike httpapi.pgWrite's pre-existing check-then-act pattern
// for the manual editor's own draft writes).
//
// actorUserID is recorded on the ai_audit_log row; uuid.Nil omits it
// (auditRow already treats a nil UUID as NULL) for callers with no
// authenticated human attached, such as internal tests.
func (s *Store) ApproveVersioned(ctx context.Context, orgID uuid.UUID, sel ApproveSelector, expectedVersion *int64, actorUserID uuid.UUID) error {
	_, err := s.writeDraftBlobVersioned(ctx, orgID, expectedVersion, actorUserID, func(db dbtx, b *DraftBlob) error {
		set := selectApproved(*b, sel)
		if set.empty() && sel.Kind != "" {
			return errApproveNothingPending
		}

		live, err := loadLive(ctx, db, orgID)
		if err != nil {
			return err
		}
		resulting := mergeForGate(live, set.topics, set.deletes)
		// Pending requests block the WHOLE-draft approve (sel.Kind == "") —
		// but an unrelated unanswered popup must not hold a single row's
		// approval hostage, so a per-entity approve skips that reason
		// (content checks below still run).
		var pending int
		if sel.Kind == "" {
			if pending, err = pendingRequestCount(ctx, db, orgID); err != nil {
				return err
			}
		}
		reasons := gate(resulting, pending)
		liveZones, err := loadZoneRows(ctx, db, orgID)
		if err != nil {
			return err
		}
		resultPolicies, err := resultingPolicyForGate(ctx, db, orgID, set.policies)
		if err != nil {
			return err
		}
		reasons = append(reasons, zoneGateReasons(resultingZonesForGate(liveZones, set.zones, set.deletes), resultPolicies)...)
		if len(reasons) > 0 {
			return &GateError{Reasons: reasons}
		}

		for _, t := range set.topics {
			if err := upsertTopicRow(ctx, db, orgID, t); err != nil {
				return err
			}
		}
		for _, t := range set.tariffs {
			if err := upsertTariffRow(ctx, db, orgID, t); err != nil {
				return err
			}
		}
		for _, p := range set.products {
			if err := upsertProductRow(ctx, db, orgID, p); err != nil {
				return err
			}
		}
		for _, c := range set.contacts {
			if err := upsertContactRow(ctx, db, orgID, c); err != nil {
				return err
			}
		}
		for _, p := range set.policies {
			if err := upsertPolicyRow(ctx, db, orgID, p); err != nil {
				return err
			}
		}
		for _, z := range set.zones {
			if err := upsertZoneRow(ctx, db, orgID, ZoneInput{
				Ref: z.Ref, Name: z.Name, ZoneLevel: z.ZoneLevel, ParentRef: z.ParentRef,
				DeliveryAvailable: z.DeliveryAvailable, DeliveryCost: z.DeliveryCost, DeliveryInDays: z.DeliveryInDays,
				Notes: z.Notes, SalesStatus: z.SalesStatus,
			}); err != nil {
				return err
			}
		}
		// Config has no natural key of its own — NaturalKeyMain stands in for it,
		// same as every other singleton — so set.config is true either for a
		// whole-draft approve that happens to include a pending config edit, or
		// for an entity-scoped approve of kind "config".
		//
		// Must be upsertConfigRow, not a bare UPDATE: an org with no live
		// ai_assistants row yet (nothing auto-seeds it — see kbstore/seed_demo.go)
		// matches zero rows on a plain UPDATE, which silently no-ops while the
		// pending patch is still cleared from the draft below regardless — a
		// first-ever config approve looked like it worked and lost the edit
		// entirely. live.go's PatchLiveConfig already carries this exact fix;
		// this path just never got the same one.
		if set.config {
			if err := upsertConfigRow(ctx, db, orgID, ConfigPatch{
				Persona: b.Config.Persona, Mission: b.Config.Mission, Guardrails: b.Config.Guardrails,
				LanguagePolicy: b.Config.LanguagePolicy, ReplyMaxWords: b.Config.ReplyMaxWords,
			}); err != nil {
				return err
			}
		}
		for _, d := range set.deletes {
			if err := applyDelete(ctx, db, orgID, d); err != nil {
				return err
			}
		}
		if err := auditRow(ctx, db, orgID, actorUserID, "approve", approveNote(sel, set)); err != nil {
			return err
		}

		// Clear exactly the approved delta from the SAME blob this closure was
		// handed — never a second, separately-committed pass (see the doc
		// comment above).
		for _, t := range set.topics {
			b.removeTopic(t.Slug)
		}
		for _, t := range set.tariffs {
			b.removeTariff(t.Ref)
		}
		for _, p := range set.products {
			b.removeProduct(p.Ref)
		}
		if len(set.contacts) > 0 {
			b.removeContact()
		}
		if len(set.policies) > 0 {
			b.removePolicy()
		}
		for _, z := range set.zones {
			b.removeZone(z.Ref)
		}
		for _, d := range set.deletes {
			b.removeDelete(d.Kind, d.Key)
		}
		if set.config {
			b.Config = DraftConfigPatch{}
		}
		return nil
	})
	if errors.Is(err, errApproveNothingPending) {
		return nil
	}
	return err
}

// applyDelete removes a live entity by its natural key at approve time.
// contact/policy are singletons — the whole org row goes, Key unused. Takes
// execer (not pgx.Tx): ApproveVersioned calls it with the dbtx its enclosing
// writeDraftBlobVersioned closure was handed, not a concrete pgx.Tx.
func applyDelete(ctx context.Context, tx execer, orgID uuid.UUID, d DraftDelete) error {
	switch d.Kind {
	case "topic":
		_, err := tx.Exec(ctx, `DELETE FROM xchats.ai_topics WHERE organization_id=$1 AND slug=$2`, orgID, d.Key)
		return err
	case "tariff":
		_, err := tx.Exec(ctx, `DELETE FROM xchats.ai_tariffs WHERE organization_id=$1 AND ref=$2`, orgID, d.Key)
		return err
	case "product":
		_, err := tx.Exec(ctx, `DELETE FROM xchats.ai_products WHERE organization_id=$1 AND ref=$2`, orgID, d.Key)
		return err
	case "contact":
		_, err := tx.Exec(ctx, `DELETE FROM xchats.ai_contacts WHERE organization_id=$1`, orgID)
		return err
	case "policy":
		_, err := tx.Exec(ctx, `DELETE FROM xchats.ai_policies WHERE organization_id=$1`, orgID)
		return err
	case "delivery_zone":
		_, err := tx.Exec(ctx, `DELETE FROM xchats.ai_delivery_zones WHERE organization_id=$1 AND ref=$2`, orgID, d.Key)
		return err
	}
	return nil
}

func approveNote(sel ApproveSelector, set approveSet) string {
	if sel.Kind != "" {
		singular := sel.Kind
		if s, ok := SingularDeleteKind(sel.Kind); ok {
			singular = s
		}
		return fmt.Sprintf("approved %s %s", singular, sel.Key)
	}
	return fmt.Sprintf("approved %d topic(s), %d tariff(s), %d product(s), %d contact(s), %d policy(-ies), %d zone(s), %d deletion(s)",
		len(set.topics), len(set.tariffs), len(set.products), len(set.contacts), len(set.policies), len(set.zones), len(set.deletes))
}

// selectApproved picks the blob entries an ApproveSelector targets. Deletes are
// keyed by entity kind (singular): topic|tariff|product|contact|policy|delivery_zone.
func selectApproved(b DraftBlob, sel ApproveSelector) approveSet {
	if sel.Kind == "" {
		return approveSet{
			topics: b.Topics, tariffs: b.Tariffs, products: b.Products,
			contacts: b.Contacts, policies: b.Policies, zones: b.DeliveryZones,
			config: b.Config.hasPending(), deletes: b.Deletes,
		}
	}
	var set approveSet
	if singular, ok := SingularDeleteKind(sel.Kind); ok {
		for _, d := range b.Deletes {
			if deleteMatches(d, singular, sel.Key) {
				set.deletes = append(set.deletes, d)
			}
		}
	}
	switch sel.Kind {
	case "topics":
		for _, t := range b.Topics {
			if t.Slug == sel.Key {
				set.topics = append(set.topics, t)
			}
		}
	case "tariffs":
		for _, t := range b.Tariffs {
			if t.Ref == sel.Key {
				set.tariffs = append(set.tariffs, t)
			}
		}
	case "products":
		for _, p := range b.Products {
			if p.Ref == sel.Key {
				set.products = append(set.products, p)
			}
		}
	case "contacts":
		if sel.Key == domain.ContactSlug || sel.Key == NaturalKeyMain {
			set.contacts = b.Contacts
		}
	case "policies":
		if sel.Key == domain.PolicySlug || sel.Key == NaturalKeyMain {
			set.policies = b.Policies
		}
	case "delivery_zones":
		for _, z := range b.DeliveryZones {
			if z.Ref == sel.Key {
				set.zones = append(set.zones, z)
			}
		}
	case "config":
		if sel.Key == NaturalKeyMain {
			set.config = b.Config.hasPending()
		}
	}
	return set
}

// mergeForGate builds the resulting live snapshot the gate validates: live
// topics with the approved entries applied on top, matching deletes removed.
// Facts are typed columns validated at reply-render time (fail closed), so the
// gate — and this merge — do not touch them.
func mergeForGate(live *domain.Snapshot, topics []DraftTopic, deletes []DraftDelete) *domain.Snapshot {
	out := &domain.Snapshot{Config: live.Config}
	del := map[string]bool{}
	for _, d := range deletes {
		del[d.Kind+":"+d.Key] = true
	}

	tIdx := map[string]int{}
	for _, t := range live.Topics {
		if del["topic:"+t.Slug] {
			continue
		}
		out.Topics = append(out.Topics, t)
		tIdx[t.Slug] = len(out.Topics) - 1
	}
	for _, t := range topics {
		nt := domain.Topic{Slug: t.Slug, Title: t.Title, BodyMD: t.BodyMD}
		if i, ok := tIdx[t.Slug]; ok {
			out.Topics[i] = nt
		} else {
			out.Topics = append(out.Topics, nt)
		}
	}
	return out
}
