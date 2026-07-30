package kbstore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
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

// Draft* are pending blob entries — the same fields as their live row, plus
// authoring provenance. Media fields mirror the canonical ai_* columns
// (plan/DECISIONS.md "Concrete media-column naming"): a nullable singular
// reference is *uuid.UUID (nil = none), a plural reference is []uuid.UUID
// (never nil — always at least an empty slice, matching the live column's
// NOT NULL DEFAULT '{}').
type DraftTopic struct {
	Slug                string      `json:"slug"`
	Title               string      `json:"title"`
	BodyMD              string      `json:"body_md"`
	FeaturedImage       *uuid.UUID  `json:"featured_image"`
	IllustrationImages  []uuid.UUID `json:"illustration_images"`
	ExplainerVideos     []uuid.UUID `json:"explainer_videos"`
	NarrationAudioFiles []uuid.UUID `json:"narration_audio_files"`
	ReferenceDocuments  []uuid.UUID `json:"reference_documents"`
	Provenance          string      `json:"provenance,omitempty"`
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
	Provenance      string      `json:"provenance,omitempty"`
}

type DraftProduct struct {
	Ref                    string      `json:"ref"`
	Name                   string      `json:"name"`
	Price                  string      `json:"price"`
	Description            string      `json:"description"`
	Category               string      `json:"category"`
	InStock                bool        `json:"in_stock"`
	SalesStatus            string      `json:"sales_status"`
	FeaturedImage          *uuid.UUID  `json:"featured_image"`
	GalleryImages          []uuid.UUID `json:"gallery_images"`
	DemoVideos             []uuid.UUID `json:"demo_videos"`
	AudioDescriptionFiles  []uuid.UUID `json:"audio_description_files"`
	CertificateDocuments   []uuid.UUID `json:"certificate_documents"`
	ManualDocuments        []uuid.UUID `json:"manual_documents"`
	GuaranteeDocuments     []uuid.UUID `json:"guarantee_documents"`
	SpecificationDocuments []uuid.UUID `json:"specification_documents"`
	Provenance             string      `json:"provenance,omitempty"`
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
	Provenance            string      `json:"provenance,omitempty"`
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
	Provenance              string      `json:"provenance,omitempty"`
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
	Provenance        string `json:"provenance,omitempty"`
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
func (s *Store) writeDraftBlob(ctx context.Context, orgID uuid.UUID, mutate func(dbtx, *DraftBlob) error) error {
	_, err := s.writeDraftBlobVersioned(ctx, orgID, nil, mutate)
	return err
}

// writeDraftBlobVersioned is writeDraftBlob's superset for the MCP write
// path: when expectedVersion is non-nil, the write is rejected with ErrStale
// unless the blob's CURRENT base_version (read under the same row lock)
// matches exactly (plan/mcp.md's `expected_draft_version?` optimistic
// concurrency — an MCP-only concern; writeDraftBlob's nil-expectedVersion
// callers never conflict on version, unchanged from before this existed).
// Returns the resulting base_version either way, so a caller can report it
// back (kb_summary's draft_version, an upsert result's new version).
func (s *Store) writeDraftBlobVersioned(ctx context.Context, orgID uuid.UUID, expectedVersion *int64, mutate func(dbtx, *DraftBlob) error) (int64, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	var raw []byte
	var currentVersion int64
	err = tx.QueryRow(ctx, `SELECT draft, base_version FROM xchats.kbd_draft WHERE organization_id = $1 FOR UPDATE`, orgID).
		Scan(&raw, &currentVersion)
	blob := DraftBlob{}
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		// no row yet — version 0, nothing to lock against; the upsert below creates it.
	case err != nil:
		return 0, err
	case len(raw) > 0:
		if err := json.Unmarshal(raw, &blob); err != nil {
			return 0, err
		}
	}
	if expectedVersion != nil && *expectedVersion != currentVersion {
		return 0, ErrStale
	}

	if err := mutate(tx, &blob); err != nil {
		return 0, err
	}

	out, err := json.Marshal(blob)
	if err != nil {
		return 0, err
	}
	newVersion := currentVersion + 1
	if _, err := tx.Exec(ctx, `
		INSERT INTO xchats.kbd_draft (organization_id, draft, base_version, updated_at)
		VALUES ($1, $2::jsonb, 1, now())
		ON CONFLICT (organization_id) DO UPDATE SET
			draft = EXCLUDED.draft, base_version = xchats.kbd_draft.base_version + 1, updated_at = now()`,
		orgID, string(out)); err != nil {
		return 0, err
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return newVersion, nil
}

// ClearDraft discards every pending edit ("Отменить изменения") — a plain reset
// of the blob to empty. Live rows are untouched.
func (s *Store) ClearDraft(ctx context.Context, orgID uuid.UUID) error {
	return s.writeDraftBlob(ctx, orgID, func(db dbtx, b *DraftBlob) error {
		*b = DraftBlob{}
		return nil
	})
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
	ID                  string      `json:"id"`
	Slug                string      `json:"slug"`
	Title               string      `json:"title"`
	BodyMD              string      `json:"body_md"`
	FeaturedImage       *uuid.UUID  `json:"featured_image"`
	IllustrationImages  []uuid.UUID `json:"illustration_images"`
	ExplainerVideos     []uuid.UUID `json:"explainer_videos"`
	NarrationAudioFiles []uuid.UUID `json:"narration_audio_files"`
	ReferenceDocuments  []uuid.UUID `json:"reference_documents"`
	Draft               bool        `json:"draft"`
	Provenance          string      `json:"provenance,omitempty"`
	UpdatedAt           time.Time   `json:"updated_at"`
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
	Provenance      string      `json:"provenance,omitempty"`
	UpdatedAt       time.Time   `json:"updated_at"`
}

type ProductRow struct {
	ID                     string      `json:"id"`
	Ref                    string      `json:"ref"`
	Name                   string      `json:"name"`
	Price                  string      `json:"price"`
	Description            string      `json:"description"`
	Category               string      `json:"category"`
	InStock                bool        `json:"in_stock"`
	SalesStatus            string      `json:"sales_status"`
	FeaturedImage          *uuid.UUID  `json:"featured_image"`
	GalleryImages          []uuid.UUID `json:"gallery_images"`
	DemoVideos             []uuid.UUID `json:"demo_videos"`
	AudioDescriptionFiles  []uuid.UUID `json:"audio_description_files"`
	CertificateDocuments   []uuid.UUID `json:"certificate_documents"`
	ManualDocuments        []uuid.UUID `json:"manual_documents"`
	GuaranteeDocuments     []uuid.UUID `json:"guarantee_documents"`
	SpecificationDocuments []uuid.UUID `json:"specification_documents"`
	Draft                  bool        `json:"draft"`
	Provenance             string      `json:"provenance,omitempty"`
	UpdatedAt              time.Time   `json:"updated_at"`
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
	Provenance            string      `json:"provenance,omitempty"`
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
	Provenance              string      `json:"provenance,omitempty"`
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
	// Zones is populated straight from the live ai_delivery_zones table, with
	// no blob-overlay step: the draft blob has no zones concept yet (draft
	// milestone later), so Draft() and LiveView() show identical zone data —
	// every row's Draft field stays false.
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
			ExplainerVideos: t.ExplainerVideos, NarrationAudioFiles: t.NarrationAudioFiles,
			ReferenceDocuments: t.ReferenceDocuments,
			Draft:              true, Provenance: t.Provenance, UpdatedAt: updatedAt})
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
			Draft:          true, Provenance: t.Provenance, UpdatedAt: updatedAt})
	}
	for _, p := range blob.Products {
		if deleted["product:"+p.Ref] {
			continue
		}
		v.Products = append(v.Products, ProductRow{ID: p.Ref, Ref: p.Ref, Name: p.Name, Price: p.Price,
			Description: p.Description, Category: p.Category, InStock: p.InStock, SalesStatus: p.SalesStatus,
			FeaturedImage: p.FeaturedImage, GalleryImages: p.GalleryImages, DemoVideos: p.DemoVideos,
			AudioDescriptionFiles: p.AudioDescriptionFiles, CertificateDocuments: p.CertificateDocuments,
			ManualDocuments: p.ManualDocuments, GuaranteeDocuments: p.GuaranteeDocuments,
			SpecificationDocuments: p.SpecificationDocuments,
			Draft:                  true, Provenance: p.Provenance, UpdatedAt: updatedAt})
	}
	if len(blob.Contacts) > 0 && !deleted["contact:"] {
		c := blob.Contacts[0]
		v.Contacts = append(v.Contacts, ContactRow{ID: domain.ContactSlug, Slug: domain.ContactSlug,
			WhatsApp: c.WhatsApp, Email: c.Email, Address: c.Address, LegalInformation: c.LegalInformation,
			CallbackTime: c.CallbackTime, WorkingHours: c.WorkingHours, Phone: c.Phone, Website: c.Website,
			Instagram: c.Instagram, ContactCardImage: c.ContactCardImage, LocationMapImage: c.LocationMapImage,
			CompanyLegalDocuments: c.CompanyLegalDocuments,
			Draft:                 true, Provenance: c.Provenance, UpdatedAt: updatedAt})
	}
	if len(blob.Policies) > 0 && !deleted["policy:"] {
		p := blob.Policies[0]
		v.Policies = append(v.Policies, PolicyRow{ID: domain.PolicySlug, Slug: domain.PolicySlug,
			DeliveryCost: p.DeliveryCost, DeliveryInDays: p.DeliveryInDays, FreeDeliveryFrom: p.FreeDeliveryFrom,
			MinOrder: p.MinOrder, Prepayment: p.Prepayment, Installment: p.Installment,
			ReturnPeriodInDays: p.ReturnPeriodInDays, Warranty: p.Warranty, OutsideZonesNote: p.OutsideZonesNote,
			CommercePolicyDocuments: p.CommercePolicyDocuments,
			Draft:                   true, Provenance: p.Provenance, UpdatedAt: updatedAt})
	}
	for _, z := range blob.DeliveryZones {
		if deleted["delivery_zone:"+z.Ref] {
			continue
		}
		v.Zones = append(v.Zones, ZoneRow{ID: z.Ref, Ref: z.Ref, Name: z.Name, ZoneLevel: z.ZoneLevel,
			ParentRef: z.ParentRef, DeliveryAvailable: z.DeliveryAvailable, DeliveryCost: z.DeliveryCost,
			DeliveryInDays: z.DeliveryInDays, Notes: z.Notes, SalesStatus: orDefault(z.SalesStatus, "active"),
			Draft: true, Provenance: z.Provenance, UpdatedAt: updatedAt})
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
	return v, nil
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
		explainer_videos, narration_audio_files, reference_documents, updated_at
		FROM xchats.ai_topics WHERE organization_id = $1 ORDER BY created_at`, orgID)
	if err != nil {
		return nil, err
	}
	for trows.Next() {
		var t TopicRow
		if err := trows.Scan(&t.Slug, &t.Title, &t.BodyMD, &t.FeaturedImage, &t.IllustrationImages,
			&t.ExplainerVideos, &t.NarrationAudioFiles, &t.ReferenceDocuments, &t.UpdatedAt); err != nil {
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
			ExplainerVideos: bt.ExplainerVideos, NarrationAudioFiles: bt.NarrationAudioFiles,
			ReferenceDocuments: bt.ReferenceDocuments,
			Draft:              true, Provenance: bt.Provenance, UpdatedAt: updatedAt}
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
			Draft: true, Provenance: bt.Provenance, UpdatedAt: updatedAt}
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
		featured_image, gallery_images, demo_videos, audio_description_files, certificate_documents,
		manual_documents, guarantee_documents, specification_documents, updated_at
		FROM xchats.ai_products WHERE organization_id = $1 ORDER BY created_at`, orgID)
	if err != nil {
		return nil, err
	}
	for prows.Next() {
		var p ProductRow
		if err := prows.Scan(&p.Ref, &p.Name, &p.Price, &p.Description, &p.Category, &p.InStock, &p.SalesStatus,
			&p.FeaturedImage, &p.GalleryImages, &p.DemoVideos, &p.AudioDescriptionFiles, &p.CertificateDocuments,
			&p.ManualDocuments, &p.GuaranteeDocuments, &p.SpecificationDocuments, &p.UpdatedAt); err != nil {
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
			AudioDescriptionFiles: bp.AudioDescriptionFiles, CertificateDocuments: bp.CertificateDocuments,
			ManualDocuments: bp.ManualDocuments, GuaranteeDocuments: bp.GuaranteeDocuments,
			SpecificationDocuments: bp.SpecificationDocuments,
			Draft:                  true, Provenance: bp.Provenance, UpdatedAt: updatedAt}
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
			Draft:                 true, Provenance: bc.Provenance, UpdatedAt: updatedAt}
		if len(v.Contacts) > 0 {
			v.Contacts[0] = row
		} else {
			v.Contacts = append(v.Contacts, row)
		}
	}
	if deleted["contact:"] {
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
			Draft: true, Provenance: bp.Provenance, UpdatedAt: updatedAt}
		if len(v.Policies) > 0 {
			v.Policies[0] = row
		} else {
			v.Policies = append(v.Policies, row)
		}
	}
	if deleted["policy:"] {
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
			Draft: true, Provenance: bz.Provenance, UpdatedAt: updatedAt}
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
// Draft CRUD — each mutates the blob under a row lock (agent rows pass a
// provenance string; manual edits from the editor leave it empty)
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
func (s *Store) PatchConfig(ctx context.Context, orgID uuid.UUID, p ConfigPatch) error {
	return s.writeDraftBlob(ctx, orgID, func(db dbtx, b *DraftBlob) error {
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
	Provenance          string // "" → '{}'
}

// UpsertTopic stages a topic create/update in the draft blob, by slug. Starts
// from the topic's current merged shape (currentTopic) so this text-only
// caller — the Playground editor, which has no media inputs — can never
// blank out media an MCP tool already staged on the same topic.
func (s *Store) UpsertTopic(ctx context.Context, orgID uuid.UUID, in TopicInput) error {
	return s.writeDraftBlob(ctx, orgID, func(db dbtx, b *DraftBlob) error {
		cur, err := s.currentTopic(ctx, db, orgID, in.Slug, b)
		if err != nil {
			return err
		}
		cur.Title, cur.BodyMD = in.Title, in.BodyMD
		cur.Provenance = orDefault(in.Provenance, "{}")
		b.upsertTopic(cur)
		return nil
	})
}

// DeleteTopic stages a topic removal by slug (drops any pending edit and marks
// the live row, if any, for deletion at approve).
func (s *Store) DeleteTopic(ctx context.Context, orgID uuid.UUID, slug string) error {
	return s.writeDraftBlob(ctx, orgID, func(db dbtx, b *DraftBlob) error {
		b.removeTopic(slug)
		b.addDelete("topic", slug)
		return nil
	})
}

// --- typed facts: tariffs / products / contacts -----------------------------

// TariffInput is an upsert payload for a draft tariff.
type TariffInput struct {
	Ref, Name, Price, LimitText, Fee, Summary, PricingType, Advantages, Disadvantages string
	Provenance                                                                        string
}

// UpsertTariff stages a tariff create/update in the draft blob, by ref.
// Merges onto the tariff's current shape (currentTariff) so this text-only
// caller never blanks out media/sales_status an MCP tool already staged.
func (s *Store) UpsertTariff(ctx context.Context, orgID uuid.UUID, in TariffInput) error {
	return s.writeDraftBlob(ctx, orgID, func(db dbtx, b *DraftBlob) error {
		cur, err := s.currentTariff(ctx, db, orgID, in.Ref, b)
		if err != nil {
			return err
		}
		cur.Name, cur.Price, cur.LimitText = in.Name, in.Price, in.LimitText
		cur.Fee, cur.Summary = in.Fee, in.Summary
		cur.PricingType = orDefault(in.PricingType, "fixed")
		cur.Advantages, cur.Disadvantages = in.Advantages, in.Disadvantages
		cur.Provenance = orDefault(in.Provenance, "{}")
		b.upsertTariff(cur)
		return nil
	})
}

// DeleteTariff stages removal of a tariff by ref.
func (s *Store) DeleteTariff(ctx context.Context, orgID uuid.UUID, ref string) error {
	return s.writeDraftBlob(ctx, orgID, func(db dbtx, b *DraftBlob) error {
		b.removeTariff(ref)
		b.addDelete("tariff", ref)
		return nil
	})
}

// ProductInput is an upsert payload for a draft product. InStock is nil-able
// and read only by the live-write path (PutLiveProduct → upsertProductRow):
// nil leaves the column at its schema default/current value, so
// UpsertProduct (the Playground draft path, which never sets this field) is
// completely unaffected.
type ProductInput struct {
	Ref, Name, Price, Description, Category string
	InStock                                 *bool
	Provenance                              string
}

// UpsertProduct stages a product create/update in the draft blob, by ref.
// Merges onto the product's current shape (currentProduct) so this caller —
// InStock is nil-able and, true to ProductInput's existing contract, still
// never read here — cannot blank out in_stock/sales_status/media an MCP tool
// already staged.
func (s *Store) UpsertProduct(ctx context.Context, orgID uuid.UUID, in ProductInput) error {
	return s.writeDraftBlob(ctx, orgID, func(db dbtx, b *DraftBlob) error {
		cur, err := s.currentProduct(ctx, db, orgID, in.Ref, b)
		if err != nil {
			return err
		}
		cur.Name, cur.Price = in.Name, in.Price
		cur.Description, cur.Category = in.Description, in.Category
		cur.Provenance = orDefault(in.Provenance, "{}")
		b.upsertProduct(cur)
		return nil
	})
}

// DeleteProduct stages removal of a product by ref.
func (s *Store) DeleteProduct(ctx context.Context, orgID uuid.UUID, ref string) error {
	return s.writeDraftBlob(ctx, orgID, func(db dbtx, b *DraftBlob) error {
		b.removeProduct(ref)
		b.addDelete("product", ref)
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
	Provenance       string
}

// PatchContacts stages an edit to the org's singleton support-contact row,
// starting from its current merged shape so omitted fields stay unchanged.
func (s *Store) PatchContacts(ctx context.Context, orgID uuid.UUID, p ContactPatch) error {
	return s.writeDraftBlob(ctx, orgID, func(db dbtx, b *DraftBlob) error {
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
		cur.Provenance = orDefault(p.Provenance, orDefault(cur.Provenance, "{}"))
		b.upsertContact(cur)
		return nil
	})
}

// PolicyPatch carries optional edits to the org's singleton commerce-policy
// row (nil = leave) — a structural clone of ContactPatch. OutsideZonesNote is
// applied by the live-write path (PatchLivePolicies); PatchPolicies (the
// Playground draft path) never sets it, so it stays a no-op there.
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
	Provenance         string
}

// PatchPolicies stages an edit to the org's singleton commerce-policy row,
// starting from its current merged shape so omitted fields stay unchanged — an
// exact clone of PatchContacts.
func (s *Store) PatchPolicies(ctx context.Context, orgID uuid.UUID, p PolicyPatch) error {
	return s.writeDraftBlob(ctx, orgID, func(db dbtx, b *DraftBlob) error {
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
		cur.Provenance = orDefault(p.Provenance, orDefault(cur.Provenance, "{}"))
		b.upsertPolicy(cur)
		return nil
	})
}

// SetFactField upserts a SINGLE field on a typed fact (tariff/product/contact),
// starting from the entity's current merged shape so the other columns are
// preserved. This is the confirm_fact write path: a detected price is confirmed
// into e.g. tariff <slug>.price without blanking the rest of the row.
func (s *Store) SetFactField(ctx context.Context, orgID uuid.UUID, table, slug, field, value string) error {
	prov := `{"source":"confirm_fact"}`
	switch table {
	case "tariff":
		return s.writeDraftBlob(ctx, orgID, func(db dbtx, b *DraftBlob) error {
			cur, err := s.currentTariff(ctx, db, orgID, slug, b)
			if err != nil {
				return err
			}
			if !setTariffField(&cur, field, value) {
				return ErrUnknownKind
			}
			cur.Provenance = prov
			b.upsertTariff(cur)
			return nil
		})
	case "product":
		return s.writeDraftBlob(ctx, orgID, func(db dbtx, b *DraftBlob) error {
			cur, err := s.currentProduct(ctx, db, orgID, slug, b)
			if err != nil {
				return err
			}
			if !setProductField(&cur, field, value) {
				return ErrUnknownKind
			}
			cur.Provenance = prov
			b.upsertProduct(cur)
			return nil
		})
	case "contact":
		p := ContactPatch{Provenance: prov}
		if !setContactPatchField(&p, field, value) {
			return ErrUnknownKind
		}
		return s.PatchContacts(ctx, orgID, p)
	case "policy":
		p := PolicyPatch{Provenance: prov}
		if !setPolicyPatchField(&p, field, value) {
			return ErrUnknownKind
		}
		return s.PatchPolicies(ctx, orgID, p)
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
		explainer_videos, narration_audio_files, reference_documents
		FROM xchats.ai_topics WHERE organization_id=$1 AND slug=$2`, orgID, slug).
		Scan(&t.Slug, &t.Title, &t.BodyMD, &t.FeaturedImage, &t.IllustrationImages,
			&t.ExplainerVideos, &t.NarrationAudioFiles, &t.ReferenceDocuments)
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
		featured_image, gallery_images, demo_videos, audio_description_files, certificate_documents,
		manual_documents, guarantee_documents, specification_documents
		FROM xchats.ai_products WHERE organization_id=$1 AND ref=$2`, orgID, ref).
		Scan(&p.Ref, &p.Name, &p.Price, &p.Description, &p.Category, &p.InStock, &p.SalesStatus,
			&p.FeaturedImage, &p.GalleryImages, &p.DemoVideos, &p.AudioDescriptionFiles, &p.CertificateDocuments,
			&p.ManualDocuments, &p.GuaranteeDocuments, &p.SpecificationDocuments)
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
	Kind string // "" | "topics" | "tariffs" | "products" | "contacts" | "policies" | "delivery_zones"
	Key  string // slug | ref | ref | domain.ContactSlug | domain.PolicySlug | ref
}

type approveSet struct {
	topics   []DraftTopic
	tariffs  []DraftTariff
	products []DraftProduct
	contacts []DraftContact
	policies []DraftPolicy
	zones    []DraftDeliveryZone
	deletes  []DraftDelete
}

func (a approveSet) empty() bool {
	return len(a.topics)+len(a.tariffs)+len(a.products)+len(a.contacts)+len(a.policies)+len(a.zones)+len(a.deletes) == 0
}

// Approve validates the resulting live set against the deterministic gate
// (including the zone/policy exclusivity invariant zoneGateReasons enforces),
// then materializes the selection into the live typed tables on their natural
// key, applies matching deletes, removes the applied entries from the blob,
// and appends an audit-log row.
func (s *Store) Approve(ctx context.Context, orgID uuid.UUID, sel ApproveSelector) error {
	blob, _, _, err := s.readDraftBlob(ctx, s.pool, orgID)
	if err != nil {
		return err
	}
	set := selectApproved(blob, sel)
	if set.empty() && sel.Kind != "" {
		return nil // nothing pending for that key — idempotent no-op
	}

	live, err := s.LoadLive(ctx, orgID)
	if err != nil {
		return err
	}
	resulting := mergeForGate(live, set.topics, set.deletes)
	// Pending requests block the WHOLE-draft approve (sel.Kind == "") — but an
	// unrelated unanswered popup must not hold a single row's approval hostage,
	// so a per-entity approve skips that reason (content checks below still run).
	var pending int
	if sel.Kind == "" {
		if pending, err = s.pendingRequestCount(ctx, orgID); err != nil {
			return err
		}
	}
	reasons := gate(resulting, pending)
	liveZones, err := loadZoneRows(ctx, s.pool, orgID)
	if err != nil {
		return err
	}
	resultPolicies, err := resultingPolicyForGate(ctx, s.pool, orgID, set.policies)
	if err != nil {
		return err
	}
	reasons = append(reasons, zoneGateReasons(resultingZonesForGate(liveZones, set.zones, set.deletes), resultPolicies)...)
	if len(reasons) > 0 {
		return &GateError{Reasons: reasons}
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	for _, t := range set.topics {
		if err := upsertTopicRow(ctx, tx, orgID, t); err != nil {
			return err
		}
	}
	for _, t := range set.tariffs {
		if err := upsertTariffRow(ctx, tx, orgID, t); err != nil {
			return err
		}
	}
	for _, p := range set.products {
		if err := upsertProductRow(ctx, tx, orgID, p); err != nil {
			return err
		}
	}
	for _, c := range set.contacts {
		if err := upsertContactRow(ctx, tx, orgID, c); err != nil {
			return err
		}
	}
	for _, p := range set.policies {
		if err := upsertPolicyRow(ctx, tx, orgID, p); err != nil {
			return err
		}
	}
	for _, z := range set.zones {
		if err := upsertZoneRow(ctx, tx, orgID, ZoneInput{
			Ref: z.Ref, Name: z.Name, ZoneLevel: z.ZoneLevel, ParentRef: z.ParentRef,
			DeliveryAvailable: z.DeliveryAvailable, DeliveryCost: z.DeliveryCost, DeliveryInDays: z.DeliveryInDays,
			Notes: z.Notes, SalesStatus: z.SalesStatus,
		}); err != nil {
			return err
		}
	}
	// Config has no natural key of its own, so it only ever rides the whole-draft
	// approve (there is no per-entity "config" kind in Правки).
	if sel.Kind == "" {
		if _, err := tx.Exec(ctx, `UPDATE xchats.ai_assistants SET
			persona = COALESCE($2, persona), mission = COALESCE($3, mission),
			guardrails = COALESCE($4, guardrails), language_policy = COALESCE($5, language_policy),
			reply_max_words = COALESCE($6, reply_max_words), updated_at = now()
			WHERE organization_id = $1`,
			orgID, blob.Config.Persona, blob.Config.Mission, blob.Config.Guardrails,
			blob.Config.LanguagePolicy, blob.Config.ReplyMaxWords); err != nil {
			return err
		}
	}
	for _, d := range set.deletes {
		if err := applyDelete(ctx, tx, orgID, d); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(ctx, `INSERT INTO xchats.ai_audit_log (organization_id, action, note) VALUES ($1,'approve',$2)`,
		orgID, approveNote(sel, set)); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}

	// Remove the applied entries from the blob. A crash between the two commits
	// leaves an already-materialized entry sitting in the blob; a re-approve of it
	// is a harmless no-op upsert (same values), so this need not share the tx.
	return s.writeDraftBlob(ctx, orgID, func(db dbtx, b *DraftBlob) error {
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
		if sel.Kind == "" {
			b.Config = DraftConfigPatch{}
		}
		return nil
	})
}

// applyDelete removes a live entity by its natural key at approve time.
// contact/policy are singletons — the whole org row goes, Key unused.
func applyDelete(ctx context.Context, tx pgx.Tx, orgID uuid.UUID, d DraftDelete) error {
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
		return fmt.Sprintf("approved %s %s", strings.TrimSuffix(sel.Kind, "s"), sel.Key)
	}
	return fmt.Sprintf("approved %d topic(s), %d tariff(s), %d product(s), %d contact(s), %d policy(-ies), %d zone(s), %d deletion(s)",
		len(set.topics), len(set.tariffs), len(set.products), len(set.contacts), len(set.policies), len(set.zones), len(set.deletes))
}

// selectApproved picks the blob entries an ApproveSelector targets. Deletes are
// keyed by entity kind (singular): topic|tariff|product|contact|policy|delivery_zone.
func selectApproved(b DraftBlob, sel ApproveSelector) approveSet {
	if sel.Kind == "" {
		return approveSet{b.Topics, b.Tariffs, b.Products, b.Contacts, b.Policies, b.DeliveryZones, b.Deletes}
	}
	var set approveSet
	singular := strings.TrimSuffix(sel.Kind, "s")
	for _, d := range b.Deletes {
		if d.Kind == singular && d.Key == sel.Key {
			set.deletes = append(set.deletes, d)
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
		if sel.Key == domain.ContactSlug {
			set.contacts = b.Contacts
		}
	case "policies":
		if sel.Key == domain.PolicySlug {
			set.policies = b.Policies
		}
	case "delivery_zones":
		for _, z := range b.DeliveryZones {
			if z.Ref == sel.Key {
				set.zones = append(set.zones, z)
			}
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
