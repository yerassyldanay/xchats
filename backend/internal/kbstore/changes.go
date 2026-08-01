package kbstore

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/internal/brain/domain"
)

// ---------------------------------------------------------------------------
// changeKinds is THE mapping between the HTTP-facing plural entity vocabulary
// (ApproveSelector.Kind, DraftChangeSet/DraftChangeDelete below, the
// /playground/draft/approve/:kind and /playground/draft/changes/:kind routes)
// and the blob's singular DraftDelete.Kind. Every consumer — selectApproved,
// CancelChange, DraftChanges, approveNote's audit summary, error text — goes
// through SingularDeleteKind/PluralChangeKind rather than manipulating the
// string itself, so a future entity kind can never silently inherit the
// fragile strings.TrimSuffix(kind, "s") this replaces, which turned
// "policies" into "policie" (draft.go's pre-existing selectApproved and
// approveNote bug: publishing a staged policy removal silently no-op'd).
// ---------------------------------------------------------------------------

var changeKinds = map[string]string{
	"topics": "topic", "tariffs": "tariff", "products": "product",
	"contacts": "contact", "policies": "policy", "delivery_zones": "delivery_zone",
}

var pluralChangeKinds = func() map[string]string {
	m := make(map[string]string, len(changeKinds))
	for plural, singular := range changeKinds {
		m[singular] = plural
	}
	return m
}()

// SingularDeleteKind maps an HTTP-facing plural entity kind — as
// ApproveSelector.Kind / DraftChangeDelete.Kind spell it — to the blob's
// singular DraftDelete.Kind vocabulary. ok is false for "config" (never
// deletable — it has no DraftDelete counterpart at all) and for anything
// outside the closed six-entity deletable vocabulary.
func SingularDeleteKind(kind string) (string, bool) {
	s, ok := changeKinds[kind]
	return s, ok
}

// PluralChangeKind is SingularDeleteKind's inverse: a blob DraftDelete.Kind
// back to the HTTP-facing plural vocabulary. Returns singular unchanged if
// it is not one of the six deletable kinds (defensive only — every
// DraftDelete this package writes already uses one of them).
func PluralChangeKind(singular string) string {
	if p, ok := pluralChangeKinds[singular]; ok {
		return p
	}
	return singular
}

// IsSingletonKind reports whether a SINGULAR delete kind identifies a true
// singleton (contact|policy) — one row per org, no natural key of its own.
// A singleton's DraftDelete.Key is therefore never part of its identity; see
// deleteMatches.
func IsSingletonKind(singular string) bool {
	return singular == "contact" || singular == "policy"
}

// deleteMatches reports whether a staged DraftDelete d addresses the same
// (singular kind, key) pair a caller is asking about. For a singleton kind,
// key is ignored entirely: applyDelete (draft.go) already ignores it when
// materializing the delete, so matching on it here was always a bug — a
// singleton delete marker exists in at least three spellings across this
// codebase's writers ("" from the pre-existing Playground path, "main" from
// MCPDelete, the natural-key slug the approve/cancel HTTP routes carry) and
// every one of them must be treated as the SAME marker.
func deleteMatches(d DraftDelete, singular, key string) bool {
	if d.Kind != singular {
		return false
	}
	return IsSingletonKind(singular) || d.Key == key
}

// deletedSingleton reports whether ANY delete marker is staged for a
// singleton kind, regardless of what key it was written with — the
// draftRowsFromBlob/mergedView read-side counterpart to deleteMatches,
// replacing the pre-existing literal deleted["contact:"] / deleted["policy:"]
// lookups (which only ever matched an empty-string key, so an MCP-authored
// "main"-keyed marker was invisible to both views).
func deletedSingleton(b DraftBlob, singular string) bool {
	for _, d := range b.Deletes {
		if d.Kind == singular {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// DraftChangeSet — the review payload behind GET /playground/draft (Черновик)
// ---------------------------------------------------------------------------

// DraftChangeSet is the review payload behind GET /playground/draft: ONLY
// what kbd_draft has staged, plus explicit deletion entries. Unlike
// DraftView it carries no live overlay, no materials and no requests — an
// unchanged published row can never appear here. The published counterpart a
// reviewer diffs against comes from GET /kb, never from this payload.
type DraftChangeSet struct {
	BaseVersion int64             `json:"base_version"`
	UpdatedAt   time.Time         `json:"updated_at"`
	Config      *DraftConfigPatch `json:"config"` // nil = no pending config edit at all
	Topics      []TopicRow        `json:"topics"`
	Tariffs     []TariffRow       `json:"tariffs"`
	Products    []ProductRow      `json:"products"`
	Contacts    []ContactRow      `json:"contacts"`
	Policies    []PolicyRow       `json:"policies"`
	Zones       []ZoneRow         `json:"zones"`
	Deletes     []DraftChangeDelete `json:"deletes"`
}

// DraftChangeDelete is one staged removal, addressed in the SAME plural
// vocabulary POST /playground/draft/approve/:kind/:id and DELETE
// /playground/draft/changes/:kind/:key use.
type DraftChangeDelete struct {
	Kind string `json:"kind"` // topics|tariffs|products|contacts|policies|delivery_zones
	Key  string `json:"key"`
}

// DraftChanges returns the org's pending change set — one blob read, no live
// query, so an unchanged published row can never appear in it.
func (s *Store) DraftChanges(ctx context.Context, orgID uuid.UUID) (*DraftChangeSet, error) {
	blob, ver, updatedAt, err := s.readDraftBlob(ctx, s.pool, orgID)
	if err != nil {
		return nil, err
	}
	rows := draftRowsFromBlob(orgID, blob, ver, updatedAt)
	out := &DraftChangeSet{
		BaseVersion: ver, UpdatedAt: updatedAt,
		Topics: rows.Topics, Tariffs: rows.Tariffs, Products: rows.Products,
		Contacts: rows.Contacts, Policies: rows.Policies, Zones: rows.Zones,
		Deletes: make([]DraftChangeDelete, 0, len(blob.Deletes)),
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

// draftRowsFromBlob projects a draft blob into a DraftView with no live
// overlay: every entry it returns is exactly what is pending, nothing
// published shows through. Pure and side-effect-free — the shared core
// draftOnly (DraftOnly/kb_read(source=draft), the MCP KB Manager widget's
// own contract) and DraftChanges (Черновик) both build on, so the two can
// never disagree about what "pending" means.
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
			ExplainerVideos: t.ExplainerVideos, NarrationAudioFiles: t.NarrationAudioFiles,
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
			AudioDescriptionFiles: p.AudioDescriptionFiles, CertificateDocuments: p.CertificateDocuments,
			ManualDocuments: p.ManualDocuments, GuaranteeDocuments: p.GuaranteeDocuments,
			SpecificationDocuments: p.SpecificationDocuments,
			Draft:                  true, UpdatedAt: updatedAt})
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
// CancelChange — «Отменить изменение» on a Черновик card
// ---------------------------------------------------------------------------

// errDraftUnchanged is a mutate-closure sentinel any writeDraftBlobVersioned
// caller may return: the transaction commits — releasing the row lock,
// persisting nothing — WITHOUT bumping base_version, and the version it read
// under that lock is handed back unchanged. General facility, not specific
// to CancelChange: any future write whose outcome can already hold before it
// runs can reuse it to stay genuinely idempotent instead of consuming a
// version on a no-op.
var errDraftUnchanged = errors.New("kbstore: draft unchanged by this write")

// CancelResult reports whether a CancelChange call actually changed
// anything. A cancel whose outcome already holds performs NO write and does
// NOT advance the version — repeating a cancellation is free, so a client
// retry after a lost response can never inflate base_version and stale every
// other open tab.
type CancelResult struct {
	Changed     bool
	BaseVersion int64
}

// CancelChange drops ONE pending change addressed by the same (kind, key)
// pair the card's Publish uses: the staged blob entry (an addition or an
// update) AND any delete marker for it (a staged removal), returning that
// entity to exactly whatever live says. kind == "config" takes a FIELD name
// as key (persona|mission|guardrails|language_policy|reply_max_words) and
// clears just that pointer; key == NaturalKeyMain clears the whole pending
// config patch ("Отменить все изменения ассистента"). ErrUnknownKind for
// anything else. Live tables are never touched, so no KB-cache invalidation.
//
// Ordering (evaluated inside ONE locked transaction — see
// writeDraftBlobVersioned):
//  1. the target's presence is decided against the blob read under the SAME
//     advisory lock every other draft write serializes on;
//  2. absent → {Changed:false, BaseVersion:current}, no write at all,
//     REGARDLESS of expectedVersion — a stale If-Match here is not a
//     conflict: there is no payload to clobber and no lost update to
//     prevent, and the response's fresh version/change-set resyncs a stale
//     client anyway;
//  3. present and expectedVersion stale → ErrStale (a real write is about to
//     happen, so THIS conflict is genuine and must not be hidden);
//  4. present and version matches (or no If-Match sent) → remove, persist,
//     bump, {Changed:true, BaseVersion:new}.
//
// Cancelling an unknown (kind,key) never lazily creates an empty kbd_draft
// row: step 2 returns before any write, and readDraftBlob already tolerates
// no row existing at all.
func (s *Store) CancelChange(ctx context.Context, orgID, actor uuid.UUID, kind, key string, expectedVersion *int64) (CancelResult, error) {
	if !validCancelTarget(kind, key) {
		return CancelResult{}, ErrUnknownKind
	}
	var changed bool
	newVersion, err := s.writeDraftBlobVersioned(ctx, orgID, nil, actor, func(_ dbtx, b *DraftBlob, currentVersion int64) error {
		if !cancelTargetPresent(*b, kind, key) {
			return errDraftUnchanged
		}
		if expectedVersion != nil && *expectedVersion != currentVersion {
			return ErrStale
		}
		cancelTargetRemove(b, kind, key)
		changed = true
		return nil
	})
	if err != nil {
		return CancelResult{}, err
	}
	return CancelResult{Changed: changed, BaseVersion: newVersion}, nil
}

var cancelConfigFields = map[string]bool{
	"persona": true, "mission": true, "guardrails": true, "language_policy": true, "reply_max_words": true,
}

// validCancelTarget guards CancelChange's (kind,key) against the same closed
// vocabulary ApproveVersioned's HTTP handler enforces, plus config's own
// field-name-or-NaturalKeyMain key shape.
func validCancelTarget(kind, key string) bool {
	if kind == "config" {
		return key == NaturalKeyMain || cancelConfigFields[key]
	}
	_, ok := SingularDeleteKind(kind)
	return ok
}

// cancelTargetPresent reports whether there is anything to cancel for
// (kind,key): a pending upsert in the blob, or a staged delete marker — see
// CancelChange's doc comment, steps 1/2. kind/key are already validated by
// validCancelTarget before this is ever called.
func cancelTargetPresent(b DraftBlob, kind, key string) bool {
	if kind == "config" {
		if key == NaturalKeyMain {
			return b.Config.hasPending()
		}
		return configFieldSet(b.Config, key)
	}
	singular, _ := SingularDeleteKind(kind)
	if hasDeleteMarker(b, singular, key) {
		return true
	}
	switch kind {
	case "topics":
		return topicPending(b, key)
	case "tariffs":
		return tariffPending(b, key)
	case "products":
		return productPending(b, key)
	case "delivery_zones":
		return zonePending(b, key)
	case "contacts":
		return len(b.Contacts) > 0
	case "policies":
		return len(b.Policies) > 0
	}
	return false
}

// cancelTargetRemove performs the removal cancelTargetPresent already found:
// both the blob entry (if any) and any matching delete marker, so a staged
// update AND a staged removal of the SAME entity — impossible in practice,
// since upserting already cancels a pending delete (see MCPUpsertTopic's doc
// comment), but cheap to cover regardless — both clear in one call.
func cancelTargetRemove(b *DraftBlob, kind, key string) {
	if kind == "config" {
		if key == NaturalKeyMain {
			b.Config = DraftConfigPatch{}
		} else {
			clearConfigField(&b.Config, key)
		}
		return
	}
	switch kind {
	case "topics":
		b.removeTopic(key)
	case "tariffs":
		b.removeTariff(key)
	case "products":
		b.removeProduct(key)
	case "delivery_zones":
		b.removeZone(key)
	case "contacts":
		b.removeContact()
	case "policies":
		b.removePolicy()
	}
	singular, _ := SingularDeleteKind(kind)
	b.removeDeleteMatching(singular, key)
}

func topicPending(b DraftBlob, slug string) bool {
	for _, t := range b.Topics {
		if t.Slug == slug {
			return true
		}
	}
	return false
}

func tariffPending(b DraftBlob, ref string) bool {
	for _, t := range b.Tariffs {
		if t.Ref == ref {
			return true
		}
	}
	return false
}

func productPending(b DraftBlob, ref string) bool {
	for _, p := range b.Products {
		if p.Ref == ref {
			return true
		}
	}
	return false
}

func zonePending(b DraftBlob, ref string) bool {
	for _, z := range b.DeliveryZones {
		if z.Ref == ref {
			return true
		}
	}
	return false
}

func hasDeleteMarker(b DraftBlob, singular, key string) bool {
	for _, d := range b.Deletes {
		if deleteMatches(d, singular, key) {
			return true
		}
	}
	return false
}

func configFieldSet(p DraftConfigPatch, field string) bool {
	switch field {
	case "persona":
		return p.Persona != nil
	case "mission":
		return p.Mission != nil
	case "guardrails":
		return p.Guardrails != nil
	case "language_policy":
		return p.LanguagePolicy != nil
	case "reply_max_words":
		return p.ReplyMaxWords != nil
	}
	return false
}

func clearConfigField(p *DraftConfigPatch, field string) {
	switch field {
	case "persona":
		p.Persona = nil
	case "mission":
		p.Mission = nil
	case "guardrails":
		p.Guardrails = nil
	case "language_policy":
		p.LanguagePolicy = nil
	case "reply_max_words":
		p.ReplyMaxWords = nil
	}
}
