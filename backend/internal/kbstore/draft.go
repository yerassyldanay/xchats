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
// authoring provenance.
type DraftTopic struct {
	Slug       string `json:"slug"`
	Lang       string `json:"lang"`
	Title      string `json:"title"`
	Keywords   string `json:"keywords"`
	BodyMD     string `json:"body_md"`
	Provenance string `json:"provenance,omitempty"`
}

type DraftAsset struct {
	Ref         string `json:"ref"`
	Kind        string `json:"kind"`
	OwnerKind   string `json:"owner_kind"`
	OwnerRef    string `json:"owner_ref"`
	Title       string `json:"title"`
	Description string `json:"description"`
	URL         string `json:"url"`
	Lang        string `json:"lang"`
	Provenance  string `json:"provenance,omitempty"`
}

type DraftTariff struct {
	Ref           string `json:"ref"`
	Lang          string `json:"lang"`
	Name          string `json:"name"`
	Price         string `json:"price"`
	LimitText     string `json:"limit_text"`
	Fee           string `json:"fee"`
	Summary       string `json:"summary"`
	PricingType   string `json:"pricing_type"`
	Advantages    string `json:"advantages"`
	Disadvantages string `json:"disadvantages"`
	Provenance    string `json:"provenance,omitempty"`
}

type DraftProduct struct {
	Ref          string `json:"ref"`
	Lang         string `json:"lang"`
	Name         string `json:"name"`
	Price        string `json:"price"`
	Description  string `json:"description"`
	Category     string `json:"category"`
	Availability string `json:"availability"`
	Provenance   string `json:"provenance,omitempty"`
}

type DraftContact struct {
	Lang         string `json:"lang"`
	WhatsApp     string `json:"whatsapp"`
	Email        string `json:"email"`
	Address      string `json:"address"`
	Legal        string `json:"legal"`
	CallbackTime string `json:"callback_time"`
	WorkingHours string `json:"working_hours"`
	Phone        string `json:"phone"`
	Website      string `json:"website"`
	Instagram    string `json:"instagram"`
	Provenance   string `json:"provenance,omitempty"`
}

// DraftPolicy is a pending ai_policies entry — a structural clone of
// DraftContact (singleton slug 'main', keyed by lang).
type DraftPolicy struct {
	Lang             string `json:"lang"`
	DeliveryCost     string `json:"delivery_cost"`
	DeliveryTime     string `json:"delivery_time"`
	FreeDeliveryFrom string `json:"free_delivery_from"`
	MinOrder         string `json:"min_order"`
	Prepayment       string `json:"prepayment"`
	Installment      string `json:"installment"`
	ReturnPeriod     string `json:"return_period"`
	Warranty         string `json:"warranty"`
	Provenance       string `json:"provenance,omitempty"`
}

// DraftDelete marks a live entity for removal at approve. Key is the entity's
// natural key: topic slug, asset ref, tariff/product ref, contact/policy lang.
type DraftDelete struct {
	Kind string `json:"kind"` // 'topic'|'asset'|'tariff'|'product'|'contact'|'policy'
	Key  string `json:"key"`
}

// DraftBlob is the whole pending KB — the exact shape of kbd_draft.draft.
type DraftBlob struct {
	Config   DraftConfigPatch `json:"config"`
	Topics   []DraftTopic     `json:"topics"`
	Assets   []DraftAsset     `json:"assets"`
	Tariffs  []DraftTariff    `json:"tariffs"`
	Products []DraftProduct   `json:"products"`
	Contacts []DraftContact   `json:"contacts"`
	Policies []DraftPolicy    `json:"policies"`
	Deletes  []DraftDelete    `json:"deletes"`
}

func refLangKey(ref, lang string) string { return ref + "|" + lang }

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

func (b *DraftBlob) upsertAsset(a DraftAsset) {
	for i := range b.Assets {
		if b.Assets[i].Ref == a.Ref {
			b.Assets[i] = a
			return
		}
	}
	b.Assets = append(b.Assets, a)
}

func (b *DraftBlob) removeAsset(ref string) {
	out := b.Assets[:0]
	for _, a := range b.Assets {
		if a.Ref != ref {
			out = append(out, a)
		}
	}
	b.Assets = out
}

func (b *DraftBlob) upsertTariff(t DraftTariff) {
	for i := range b.Tariffs {
		if b.Tariffs[i].Ref == t.Ref && b.Tariffs[i].Lang == t.Lang {
			b.Tariffs[i] = t
			return
		}
	}
	b.Tariffs = append(b.Tariffs, t)
}

// removeTariff drops all pending language rows of a ref (v1 has one lang/ref).
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
		if b.Products[i].Ref == p.Ref && b.Products[i].Lang == p.Lang {
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

func (b *DraftBlob) upsertContact(c DraftContact) {
	for i := range b.Contacts {
		if b.Contacts[i].Lang == c.Lang {
			b.Contacts[i] = c
			return
		}
	}
	b.Contacts = append(b.Contacts, c)
}

func (b *DraftBlob) removeContact(lang string) {
	out := b.Contacts[:0]
	for _, c := range b.Contacts {
		if c.Lang != lang {
			out = append(out, c)
		}
	}
	b.Contacts = out
}

// upsertPolicy / removePolicy — exact clone of upsertContact/removeContact,
// keyed by lang (ai_policies is a singleton table like ai_contacts).
func (b *DraftBlob) upsertPolicy(p DraftPolicy) {
	for i := range b.Policies {
		if b.Policies[i].Lang == p.Lang {
			b.Policies[i] = p
			return
		}
	}
	b.Policies = append(b.Policies, p)
}

func (b *DraftBlob) removePolicy(lang string) {
	out := b.Policies[:0]
	for _, p := range b.Policies {
		if p.Lang != lang {
			out = append(out, p)
		}
	}
	b.Policies = out
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
// the first write).
func (s *Store) readDraftBlob(ctx context.Context, orgID uuid.UUID) (DraftBlob, int64, time.Time, error) {
	var raw []byte
	var ver int64
	var updatedAt time.Time
	err := s.pool.QueryRow(ctx, `SELECT draft, base_version, updated_at FROM xchats.kbd_draft WHERE organization_id = $1`, orgID).
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
	_, ver, _, err := s.readDraftBlob(ctx, orgID)
	return ver, err
}

// writeDraftBlob runs mutate over the org's draft blob inside a row-locked
// transaction (SELECT ... FOR UPDATE), so concurrent writers serialize instead
// of racing, then persists the result and bumps base_version. This is the ONLY
// way the blob is written.
func (s *Store) writeDraftBlob(ctx context.Context, orgID uuid.UUID, mutate func(*DraftBlob) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var raw []byte
	err = tx.QueryRow(ctx, `SELECT draft FROM xchats.kbd_draft WHERE organization_id = $1 FOR UPDATE`, orgID).Scan(&raw)
	blob := DraftBlob{}
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		// no row yet — nothing to lock against; the upsert below creates it.
	case err != nil:
		return err
	case len(raw) > 0:
		if err := json.Unmarshal(raw, &blob); err != nil {
			return err
		}
	}

	if err := mutate(&blob); err != nil {
		return err
	}

	out, err := json.Marshal(blob)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO xchats.kbd_draft (organization_id, draft, base_version, updated_at)
		VALUES ($1, $2::jsonb, 1, now())
		ON CONFLICT (organization_id) DO UPDATE SET
			draft = EXCLUDED.draft, base_version = xchats.kbd_draft.base_version + 1, updated_at = now()`,
		orgID, string(out)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// ClearDraft discards every pending edit ("Отменить изменения") — a plain reset
// of the blob to empty. Live rows are untouched.
func (s *Store) ClearDraft(ctx context.Context, orgID uuid.UUID) error {
	return s.writeDraftBlob(ctx, orgID, func(b *DraftBlob) error {
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

// TopicRow / AssetRow / TariffRow / ProductRow / ContactRow are editor-facing KB
// rows. ID is the entity's natural key (slug / ref / ref / ref / lang) — blob
// entries carry no DB row id. UpdatedAt is the live row's own timestamp for a
// live entity, or the whole draft blob's timestamp for a pending one.
type TopicRow struct {
	ID         string    `json:"id"`
	Slug       string    `json:"slug"`
	Lang       string    `json:"lang"`
	Title      string    `json:"title"`
	Keywords   string    `json:"keywords"`
	BodyMD     string    `json:"body_md"`
	Draft      bool      `json:"draft"`
	Provenance string    `json:"provenance,omitempty"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type AssetRow struct {
	ID          string    `json:"id"`
	Ref         string    `json:"ref"`
	Kind        string    `json:"kind"`
	OwnerKind   string    `json:"owner_kind"`
	OwnerRef    string    `json:"owner_ref"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	URL         string    `json:"url"`
	Lang        string    `json:"lang"`
	Draft       bool      `json:"draft"`
	Provenance  string    `json:"provenance,omitempty"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type TariffRow struct {
	ID            string    `json:"id"`
	Ref           string    `json:"ref"`
	Lang          string    `json:"lang"`
	Name          string    `json:"name"`
	Price         string    `json:"price"`
	LimitText     string    `json:"limit_text"`
	Fee           string    `json:"fee"`
	Summary       string    `json:"summary"`
	PricingType   string    `json:"pricing_type"`
	Advantages    string    `json:"advantages"`
	Disadvantages string    `json:"disadvantages"`
	Draft         bool      `json:"draft"`
	Provenance    string    `json:"provenance,omitempty"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type ProductRow struct {
	ID           string    `json:"id"`
	Ref          string    `json:"ref"`
	Lang         string    `json:"lang"`
	Name         string    `json:"name"`
	Price        string    `json:"price"`
	Description  string    `json:"description"`
	Category     string    `json:"category"`
	Availability string    `json:"availability"`
	Draft        bool      `json:"draft"`
	Provenance   string    `json:"provenance,omitempty"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type ContactRow struct {
	ID           string    `json:"id"`
	Slug         string    `json:"slug"`
	Lang         string    `json:"lang"`
	WhatsApp     string    `json:"whatsapp"`
	Email        string    `json:"email"`
	Address      string    `json:"address"`
	Legal        string    `json:"legal"`
	CallbackTime string    `json:"callback_time"`
	WorkingHours string    `json:"working_hours"`
	Phone        string    `json:"phone"`
	Website      string    `json:"website"`
	Instagram    string    `json:"instagram"`
	Draft        bool      `json:"draft"`
	Provenance   string    `json:"provenance,omitempty"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// PolicyRow is the editor-facing ai_policies row — a structural clone of
// ContactRow (ID = lang, Slug = the singleton domain.PolicySlug).
type PolicyRow struct {
	ID               string    `json:"id"`
	Slug             string    `json:"slug"`
	Lang             string    `json:"lang"`
	DeliveryCost     string    `json:"delivery_cost"`
	DeliveryTime     string    `json:"delivery_time"`
	FreeDeliveryFrom string    `json:"free_delivery_from"`
	MinOrder         string    `json:"min_order"`
	Prepayment       string    `json:"prepayment"`
	Installment      string    `json:"installment"`
	ReturnPeriod     string    `json:"return_period"`
	Warranty         string    `json:"warranty"`
	Draft            bool      `json:"draft"`
	Provenance       string    `json:"provenance,omitempty"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// DraftView is the whole working KB for the editor + builder: live rows merged
// with pending blob entries.
type DraftView struct {
	Config    DraftConfig  `json:"config"`
	Topics    []TopicRow   `json:"topics"`
	Assets    []AssetRow   `json:"assets"`
	Tariffs   []TariffRow  `json:"tariffs"`
	Products  []ProductRow `json:"products"`
	Contacts  []ContactRow `json:"contacts"`
	Policies  []PolicyRow  `json:"policies"`
	Materials []Material   `json:"materials"`
	Requests  []Request    `json:"requests"`
}

// Draft assembles the merged working view: live rows, overlaid by pending blob
// entries, with entities under a Deletes[] marker suppressed. Side-effect-free.
func (s *Store) Draft(ctx context.Context, orgID uuid.UUID) (*DraftView, error) {
	blob, ver, updatedAt, err := s.readDraftBlob(ctx, orgID)
	if err != nil {
		return nil, err
	}
	v, err := s.mergedView(ctx, orgID, blob, ver, updatedAt)
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
	v, err := s.mergedView(ctx, orgID, DraftBlob{}, 0, time.Time{})
	if err != nil {
		return nil, err
	}
	v.Materials = []Material{}
	v.Requests = []Request{}
	return v, nil
}

// mergedView loads live rows and overlays the given blob's pending entries —
// shared by Draft (a real blob) and LiveView (an always-empty blob, so every
// overlay loop below is a no-op and every row stays Draft:false). It fills
// everything except Materials/Requests, which only Draft queries (LiveView has
// no playground concept of them).
func (s *Store) mergedView(ctx context.Context, orgID uuid.UUID, blob DraftBlob, ver int64, updatedAt time.Time) (*DraftView, error) {
	v := &DraftView{Config: DraftConfig{OrganizationID: orgID, ReplyMaxWords: 120, BaseVersion: ver, UpdatedAt: updatedAt}}
	err := s.pool.QueryRow(ctx, `SELECT persona, mission, guardrails, language_policy, reply_max_words
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
	trows, err := s.pool.Query(ctx, `SELECT slug, lang, title, keywords, body_md, updated_at
		FROM xchats.ai_topics WHERE organization_id = $1 ORDER BY created_at`, orgID)
	if err != nil {
		return nil, err
	}
	for trows.Next() {
		var t TopicRow
		if err := trows.Scan(&t.Slug, &t.Lang, &t.Title, &t.Keywords, &t.BodyMD, &t.UpdatedAt); err != nil {
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
		row := TopicRow{ID: bt.Slug, Slug: bt.Slug, Lang: bt.Lang, Title: bt.Title, Keywords: bt.Keywords,
			BodyMD: bt.BodyMD, Draft: true, Provenance: bt.Provenance, UpdatedAt: updatedAt}
		if i, ok := topicIdx[bt.Slug]; ok {
			v.Topics[i] = row
		} else {
			v.Topics = append(v.Topics, row)
			topicIdx[bt.Slug] = len(v.Topics) - 1
		}
	}
	v.Topics = filterTopics(v.Topics, deleted)

	// assets
	assetIdx := map[string]int{}
	arows, err := s.pool.Query(ctx, `SELECT ref, asset_kind, owner_kind, owner_ref, title, description, asset_url, lang, updated_at
		FROM xchats.ai_assets WHERE organization_id = $1 ORDER BY created_at`, orgID)
	if err != nil {
		return nil, err
	}
	for arows.Next() {
		var a AssetRow
		if err := arows.Scan(&a.Ref, &a.Kind, &a.OwnerKind, &a.OwnerRef, &a.Title, &a.Description, &a.URL, &a.Lang, &a.UpdatedAt); err != nil {
			arows.Close()
			return nil, err
		}
		a.ID = a.Ref
		v.Assets = append(v.Assets, a)
		assetIdx[a.Ref] = len(v.Assets) - 1
	}
	arows.Close()
	if err := arows.Err(); err != nil {
		return nil, err
	}
	for _, ba := range blob.Assets {
		row := AssetRow{ID: ba.Ref, Ref: ba.Ref, Kind: ba.Kind, OwnerKind: ba.OwnerKind, OwnerRef: ba.OwnerRef,
			Title: ba.Title, Description: ba.Description, URL: ba.URL, Lang: ba.Lang, Draft: true, Provenance: ba.Provenance, UpdatedAt: updatedAt}
		if i, ok := assetIdx[ba.Ref]; ok {
			v.Assets[i] = row
		} else {
			v.Assets = append(v.Assets, row)
			assetIdx[ba.Ref] = len(v.Assets) - 1
		}
	}
	v.Assets = filterAssets(v.Assets, deleted)

	// tariffs
	tariffIdx := map[string]int{}
	trrows, err := s.pool.Query(ctx, `SELECT ref, lang, name, price, limit_text, fee, summary, pricing_type, advantages, disadvantages, updated_at
		FROM xchats.ai_tariffs WHERE organization_id = $1 ORDER BY created_at`, orgID)
	if err != nil {
		return nil, err
	}
	for trrows.Next() {
		var t TariffRow
		if err := trrows.Scan(&t.Ref, &t.Lang, &t.Name, &t.Price, &t.LimitText, &t.Fee, &t.Summary, &t.PricingType, &t.Advantages, &t.Disadvantages, &t.UpdatedAt); err != nil {
			trrows.Close()
			return nil, err
		}
		t.ID = t.Ref
		v.Tariffs = append(v.Tariffs, t)
		tariffIdx[refLangKey(t.Ref, t.Lang)] = len(v.Tariffs) - 1
	}
	trrows.Close()
	if err := trrows.Err(); err != nil {
		return nil, err
	}
	for _, bt := range blob.Tariffs {
		row := TariffRow{ID: bt.Ref, Ref: bt.Ref, Lang: bt.Lang, Name: bt.Name, Price: bt.Price, LimitText: bt.LimitText,
			Fee: bt.Fee, Summary: bt.Summary, PricingType: bt.PricingType, Advantages: bt.Advantages,
			Disadvantages: bt.Disadvantages, Draft: true, Provenance: bt.Provenance, UpdatedAt: updatedAt}
		k := refLangKey(bt.Ref, bt.Lang)
		if i, ok := tariffIdx[k]; ok {
			v.Tariffs[i] = row
		} else {
			v.Tariffs = append(v.Tariffs, row)
			tariffIdx[k] = len(v.Tariffs) - 1
		}
	}
	kt := v.Tariffs[:0]
	for _, t := range v.Tariffs {
		if !deleted["tariff:"+t.Ref] {
			kt = append(kt, t)
		}
	}
	v.Tariffs = kt

	// products
	productIdx := map[string]int{}
	prows, err := s.pool.Query(ctx, `SELECT ref, lang, name, price, description, category, availability, updated_at
		FROM xchats.ai_products WHERE organization_id = $1 ORDER BY created_at`, orgID)
	if err != nil {
		return nil, err
	}
	for prows.Next() {
		var p ProductRow
		if err := prows.Scan(&p.Ref, &p.Lang, &p.Name, &p.Price, &p.Description, &p.Category, &p.Availability, &p.UpdatedAt); err != nil {
			prows.Close()
			return nil, err
		}
		p.ID = p.Ref
		v.Products = append(v.Products, p)
		productIdx[refLangKey(p.Ref, p.Lang)] = len(v.Products) - 1
	}
	prows.Close()
	if err := prows.Err(); err != nil {
		return nil, err
	}
	for _, bp := range blob.Products {
		row := ProductRow{ID: bp.Ref, Ref: bp.Ref, Lang: bp.Lang, Name: bp.Name, Price: bp.Price,
			Description: bp.Description, Category: bp.Category, Availability: bp.Availability,
			Draft: true, Provenance: bp.Provenance, UpdatedAt: updatedAt}
		k := refLangKey(bp.Ref, bp.Lang)
		if i, ok := productIdx[k]; ok {
			v.Products[i] = row
		} else {
			v.Products = append(v.Products, row)
			productIdx[k] = len(v.Products) - 1
		}
	}
	kp := v.Products[:0]
	for _, p := range v.Products {
		if !deleted["product:"+p.Ref] {
			kp = append(kp, p)
		}
	}
	v.Products = kp

	// contacts
	contactIdx := map[string]int{}
	crows, err := s.pool.Query(ctx, `SELECT slug, lang, whatsapp, email, address, legal, callback_time,
		working_hours, phone, website, instagram, updated_at
		FROM xchats.ai_contacts WHERE organization_id = $1 ORDER BY created_at`, orgID)
	if err != nil {
		return nil, err
	}
	for crows.Next() {
		var c ContactRow
		if err := crows.Scan(&c.Slug, &c.Lang, &c.WhatsApp, &c.Email, &c.Address, &c.Legal, &c.CallbackTime,
			&c.WorkingHours, &c.Phone, &c.Website, &c.Instagram, &c.UpdatedAt); err != nil {
			crows.Close()
			return nil, err
		}
		c.ID = c.Lang
		v.Contacts = append(v.Contacts, c)
		contactIdx[c.Lang] = len(v.Contacts) - 1
	}
	crows.Close()
	if err := crows.Err(); err != nil {
		return nil, err
	}
	for _, bc := range blob.Contacts {
		row := ContactRow{ID: bc.Lang, Slug: domain.ContactSlug, Lang: bc.Lang, WhatsApp: bc.WhatsApp, Email: bc.Email,
			Address: bc.Address, Legal: bc.Legal, CallbackTime: bc.CallbackTime,
			WorkingHours: bc.WorkingHours, Phone: bc.Phone, Website: bc.Website, Instagram: bc.Instagram,
			Draft: true, Provenance: bc.Provenance, UpdatedAt: updatedAt}
		if i, ok := contactIdx[bc.Lang]; ok {
			v.Contacts[i] = row
		} else {
			v.Contacts = append(v.Contacts, row)
			contactIdx[bc.Lang] = len(v.Contacts) - 1
		}
	}
	kc := v.Contacts[:0]
	for _, c := range v.Contacts {
		if !deleted["contact:"+c.Lang] {
			kc = append(kc, c)
		}
	}
	v.Contacts = kc

	// policies — an exact clone of the contacts section above (singleton table
	// keyed by lang, slug domain.PolicySlug).
	policyIdx := map[string]int{}
	polrows, err := s.pool.Query(ctx, `SELECT slug, lang, delivery_cost, delivery_time, free_delivery_from, min_order,
		prepayment, installment, return_period, warranty, updated_at
		FROM xchats.ai_policies WHERE organization_id = $1 ORDER BY created_at`, orgID)
	if err != nil {
		return nil, err
	}
	for polrows.Next() {
		var p PolicyRow
		if err := polrows.Scan(&p.Slug, &p.Lang, &p.DeliveryCost, &p.DeliveryTime, &p.FreeDeliveryFrom, &p.MinOrder,
			&p.Prepayment, &p.Installment, &p.ReturnPeriod, &p.Warranty, &p.UpdatedAt); err != nil {
			polrows.Close()
			return nil, err
		}
		p.ID = p.Lang
		v.Policies = append(v.Policies, p)
		policyIdx[p.Lang] = len(v.Policies) - 1
	}
	polrows.Close()
	if err := polrows.Err(); err != nil {
		return nil, err
	}
	for _, bp := range blob.Policies {
		row := PolicyRow{ID: bp.Lang, Slug: domain.PolicySlug, Lang: bp.Lang, DeliveryCost: bp.DeliveryCost,
			DeliveryTime: bp.DeliveryTime, FreeDeliveryFrom: bp.FreeDeliveryFrom, MinOrder: bp.MinOrder,
			Prepayment: bp.Prepayment, Installment: bp.Installment, ReturnPeriod: bp.ReturnPeriod, Warranty: bp.Warranty,
			Draft: true, Provenance: bp.Provenance, UpdatedAt: updatedAt}
		if i, ok := policyIdx[bp.Lang]; ok {
			v.Policies[i] = row
		} else {
			v.Policies = append(v.Policies, row)
			policyIdx[bp.Lang] = len(v.Policies) - 1
		}
	}
	kpol := v.Policies[:0]
	for _, p := range v.Policies {
		if !deleted["policy:"+p.Lang] {
			kpol = append(kpol, p)
		}
	}
	v.Policies = kpol

	// Guarantee non-nil slices: every collection must serialize as a JSON array
	// ([]), never null. A nil slice (empty table + empty blob) marshals to null,
	// and the client reads d.<coll>.length directly — a null would crash the page.
	if v.Topics == nil {
		v.Topics = []TopicRow{}
	}
	if v.Assets == nil {
		v.Assets = []AssetRow{}
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

func filterAssets(in []AssetRow, deleted map[string]bool) []AssetRow {
	out := in[:0]
	for _, a := range in {
		if !deleted["asset:"+a.Ref] {
			out = append(out, a)
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
	return s.writeDraftBlob(ctx, orgID, func(b *DraftBlob) error {
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
	Slug, Lang, Title, Keywords, BodyMD string
	Provenance                          string // "" → '{}'
}

// UpsertTopic stages a topic create/update in the draft blob, by slug.
func (s *Store) UpsertTopic(ctx context.Context, orgID uuid.UUID, in TopicInput) error {
	return s.writeDraftBlob(ctx, orgID, func(b *DraftBlob) error {
		b.upsertTopic(DraftTopic{
			Slug: in.Slug, Lang: orDefault(in.Lang, "ru"), Title: in.Title,
			Keywords: in.Keywords, BodyMD: in.BodyMD, Provenance: orDefault(in.Provenance, "{}"),
		})
		return nil
	})
}

// DeleteTopic stages a topic removal by slug (drops any pending edit and marks
// the live row, if any, for deletion at approve).
func (s *Store) DeleteTopic(ctx context.Context, orgID uuid.UUID, slug string) error {
	return s.writeDraftBlob(ctx, orgID, func(b *DraftBlob) error {
		b.removeTopic(slug)
		b.addDelete("topic", slug)
		return nil
	})
}

// AssetInput is an upsert payload for a draft asset.
type AssetInput struct {
	Ref, Kind, OwnerKind, OwnerRef, Title, Description, URL, Lang string
	Provenance                                                    string
}

// UpsertAsset stages an asset create/update in the draft blob, by ref.
func (s *Store) UpsertAsset(ctx context.Context, orgID uuid.UUID, in AssetInput) error {
	return s.writeDraftBlob(ctx, orgID, func(b *DraftBlob) error {
		b.upsertAsset(DraftAsset{
			Ref: in.Ref, Kind: orDefault(in.Kind, "image"), OwnerKind: in.OwnerKind, OwnerRef: in.OwnerRef,
			Title: in.Title, Description: in.Description, URL: in.URL, Lang: orDefault(in.Lang, "ru"),
			Provenance: orDefault(in.Provenance, "{}"),
		})
		return nil
	})
}

// AssetPatch edits an asset's description and/or reassigns its owner (nil = leave).
type AssetPatch struct {
	Description *string
	OwnerKind   *string
	OwnerRef    *string
}

// PatchAsset stages an edit to an existing asset by ref — starting from its
// current merged row (draft entry if pending, else the live row) so a partial
// patch never blanks the untouched fields.
func (s *Store) PatchAsset(ctx context.Context, orgID uuid.UUID, ref string, p AssetPatch) error {
	return s.writeDraftBlob(ctx, orgID, func(b *DraftBlob) error {
		cur, err := s.currentAsset(ctx, orgID, ref, b)
		if err != nil {
			return err
		}
		if p.Description != nil {
			cur.Description = *p.Description
		}
		if p.OwnerKind != nil {
			cur.OwnerKind = *p.OwnerKind
		}
		if p.OwnerRef != nil {
			cur.OwnerRef = *p.OwnerRef
		}
		b.upsertAsset(cur)
		return nil
	})
}

// currentAsset resolves the entity's merged current shape: the pending blob
// entry if one exists, else the live row, else a blank scaffold (new asset).
func (s *Store) currentAsset(ctx context.Context, orgID uuid.UUID, ref string, b *DraftBlob) (DraftAsset, error) {
	for _, a := range b.Assets {
		if a.Ref == ref {
			return a, nil
		}
	}
	var a DraftAsset
	err := s.pool.QueryRow(ctx, `SELECT ref, asset_kind, owner_kind, owner_ref, title, description, asset_url, lang
		FROM xchats.ai_assets WHERE organization_id = $1 AND ref = $2`, orgID, ref).
		Scan(&a.Ref, &a.Kind, &a.OwnerKind, &a.OwnerRef, &a.Title, &a.Description, &a.URL, &a.Lang)
	if errors.Is(err, pgx.ErrNoRows) {
		return DraftAsset{Ref: ref, Kind: "image", Lang: "ru"}, nil
	}
	return a, err
}

// DeleteAsset stages an asset removal by ref.
func (s *Store) DeleteAsset(ctx context.Context, orgID uuid.UUID, ref string) error {
	return s.writeDraftBlob(ctx, orgID, func(b *DraftBlob) error {
		b.removeAsset(ref)
		b.addDelete("asset", ref)
		return nil
	})
}

// --- typed facts: tariffs / products / contacts -----------------------------

// TariffInput is an upsert payload for a draft tariff (one language row).
type TariffInput struct {
	Ref, Lang, Name, Price, LimitText, Fee, Summary, PricingType, Advantages, Disadvantages string
	Provenance                                                                              string
}

// UpsertTariff stages a tariff create/update in the draft blob, by (ref, lang).
func (s *Store) UpsertTariff(ctx context.Context, orgID uuid.UUID, in TariffInput) error {
	return s.writeDraftBlob(ctx, orgID, func(b *DraftBlob) error {
		b.upsertTariff(DraftTariff{
			Ref: in.Ref, Lang: orDefault(in.Lang, "ru"), Name: in.Name, Price: in.Price, LimitText: in.LimitText,
			Fee: in.Fee, Summary: in.Summary, PricingType: orDefault(in.PricingType, "fixed"),
			Advantages: in.Advantages, Disadvantages: in.Disadvantages, Provenance: orDefault(in.Provenance, "{}"),
		})
		return nil
	})
}

// DeleteTariff stages removal of a tariff (all language rows of ref).
func (s *Store) DeleteTariff(ctx context.Context, orgID uuid.UUID, ref string) error {
	return s.writeDraftBlob(ctx, orgID, func(b *DraftBlob) error {
		b.removeTariff(ref)
		b.addDelete("tariff", ref)
		return nil
	})
}

// ProductInput is an upsert payload for a draft product (one language row).
type ProductInput struct {
	Ref, Lang, Name, Price, Description, Category, Availability string
	Provenance                                                  string
}

// UpsertProduct stages a product create/update in the draft blob, by (ref, lang).
func (s *Store) UpsertProduct(ctx context.Context, orgID uuid.UUID, in ProductInput) error {
	return s.writeDraftBlob(ctx, orgID, func(b *DraftBlob) error {
		b.upsertProduct(DraftProduct{
			Ref: in.Ref, Lang: orDefault(in.Lang, "ru"), Name: in.Name, Price: in.Price,
			Description: in.Description, Category: in.Category, Availability: in.Availability,
			Provenance: orDefault(in.Provenance, "{}"),
		})
		return nil
	})
}

// DeleteProduct stages removal of a product (all language rows of ref).
func (s *Store) DeleteProduct(ctx context.Context, orgID uuid.UUID, ref string) error {
	return s.writeDraftBlob(ctx, orgID, func(b *DraftBlob) error {
		b.removeProduct(ref)
		b.addDelete("product", ref)
		return nil
	})
}

// ContactPatch carries optional contacts edits for one language row (nil = leave).
type ContactPatch struct {
	Lang         string // which row; "" → '*'
	WhatsApp     *string
	Email        *string
	Address      *string
	Legal        *string
	CallbackTime *string
	WorkingHours *string
	Phone        *string
	Website      *string
	Instagram    *string
	Provenance   string
}

// PatchContacts stages an edit to the org support-contact row for a language,
// starting from its current merged shape so omitted fields stay unchanged.
func (s *Store) PatchContacts(ctx context.Context, orgID uuid.UUID, p ContactPatch) error {
	lang := orDefault(p.Lang, "*")
	return s.writeDraftBlob(ctx, orgID, func(b *DraftBlob) error {
		cur, err := s.currentContact(ctx, orgID, lang, b)
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
		if p.Legal != nil {
			cur.Legal = *p.Legal
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

// PolicyPatch carries optional commerce-policy edits for one language row (nil
// = leave) — a structural clone of ContactPatch.
type PolicyPatch struct {
	Lang             string // which row; "" → '*'
	DeliveryCost     *string
	DeliveryTime     *string
	FreeDeliveryFrom *string
	MinOrder         *string
	Prepayment       *string
	Installment      *string
	ReturnPeriod     *string
	Warranty         *string
	Provenance       string
}

// PatchPolicies stages an edit to the org commerce-policy row for a language,
// starting from its current merged shape so omitted fields stay unchanged — an
// exact clone of PatchContacts.
func (s *Store) PatchPolicies(ctx context.Context, orgID uuid.UUID, p PolicyPatch) error {
	lang := orDefault(p.Lang, "*")
	return s.writeDraftBlob(ctx, orgID, func(b *DraftBlob) error {
		cur, err := s.currentPolicy(ctx, orgID, lang, b)
		if err != nil {
			return err
		}
		if p.DeliveryCost != nil {
			cur.DeliveryCost = *p.DeliveryCost
		}
		if p.DeliveryTime != nil {
			cur.DeliveryTime = *p.DeliveryTime
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
		if p.ReturnPeriod != nil {
			cur.ReturnPeriod = *p.ReturnPeriod
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
func (s *Store) SetFactField(ctx context.Context, orgID uuid.UUID, table, slug, field, lang, value string) error {
	prov := `{"source":"confirm_fact"}`
	switch table {
	case "tariff":
		return s.writeDraftBlob(ctx, orgID, func(b *DraftBlob) error {
			cur, err := s.currentTariff(ctx, orgID, slug, orDefault(lang, "ru"), b)
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
		return s.writeDraftBlob(ctx, orgID, func(b *DraftBlob) error {
			cur, err := s.currentProduct(ctx, orgID, slug, orDefault(lang, "ru"), b)
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
		p := ContactPatch{Lang: lang, Provenance: prov}
		if !setContactPatchField(&p, field, value) {
			return ErrUnknownKind
		}
		return s.PatchContacts(ctx, orgID, p)
	case "policy":
		p := PolicyPatch{Lang: lang, Provenance: prov}
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
	case "availability":
		p.Availability = value
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
	case "legal":
		p.Legal = &value
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
	case "delivery_time":
		p.DeliveryTime = &value
	case "free_delivery_from":
		p.FreeDeliveryFrom = &value
	case "min_order":
		p.MinOrder = &value
	case "prepayment":
		p.Prepayment = &value
	case "installment":
		p.Installment = &value
	case "return_period":
		p.ReturnPeriod = &value
	case "warranty":
		p.Warranty = &value
	default:
		return false
	}
	return true
}

// currentTariff / currentProduct resolve the merged current shape of a typed fact
// row: the pending blob entry, else the live row, else a blank scaffold.
func (s *Store) currentTariff(ctx context.Context, orgID uuid.UUID, ref, lang string, b *DraftBlob) (DraftTariff, error) {
	for _, t := range b.Tariffs {
		if t.Ref == ref && t.Lang == lang {
			return t, nil
		}
	}
	var t DraftTariff
	err := s.pool.QueryRow(ctx, `SELECT ref, lang, name, price, limit_text, fee, summary, pricing_type, advantages, disadvantages
		FROM xchats.ai_tariffs WHERE organization_id=$1 AND ref=$2 AND lang=$3`, orgID, ref, lang).
		Scan(&t.Ref, &t.Lang, &t.Name, &t.Price, &t.LimitText, &t.Fee, &t.Summary, &t.PricingType, &t.Advantages, &t.Disadvantages)
	if errors.Is(err, pgx.ErrNoRows) {
		return DraftTariff{Ref: ref, Lang: lang, PricingType: "fixed"}, nil
	}
	return t, err
}

func (s *Store) currentProduct(ctx context.Context, orgID uuid.UUID, ref, lang string, b *DraftBlob) (DraftProduct, error) {
	for _, p := range b.Products {
		if p.Ref == ref && p.Lang == lang {
			return p, nil
		}
	}
	var p DraftProduct
	err := s.pool.QueryRow(ctx, `SELECT ref, lang, name, price, description, category, availability
		FROM xchats.ai_products WHERE organization_id=$1 AND ref=$2 AND lang=$3`, orgID, ref, lang).
		Scan(&p.Ref, &p.Lang, &p.Name, &p.Price, &p.Description, &p.Category, &p.Availability)
	if errors.Is(err, pgx.ErrNoRows) {
		return DraftProduct{Ref: ref, Lang: lang}, nil
	}
	return p, err
}

// currentContact resolves the merged current contact row for a language: the
// pending blob entry if one exists, else the live row, else a blank scaffold.
func (s *Store) currentContact(ctx context.Context, orgID uuid.UUID, lang string, b *DraftBlob) (DraftContact, error) {
	for _, c := range b.Contacts {
		if c.Lang == lang {
			return c, nil
		}
	}
	var c DraftContact
	err := s.pool.QueryRow(ctx, `SELECT lang, whatsapp, email, address, legal, callback_time,
		working_hours, phone, website, instagram
		FROM xchats.ai_contacts WHERE organization_id = $1 AND lang = $2`, orgID, lang).
		Scan(&c.Lang, &c.WhatsApp, &c.Email, &c.Address, &c.Legal, &c.CallbackTime,
			&c.WorkingHours, &c.Phone, &c.Website, &c.Instagram)
	if errors.Is(err, pgx.ErrNoRows) {
		return DraftContact{Lang: lang}, nil
	}
	return c, err
}

// currentPolicy resolves the merged current commerce-policy row for a
// language — an exact clone of currentContact.
func (s *Store) currentPolicy(ctx context.Context, orgID uuid.UUID, lang string, b *DraftBlob) (DraftPolicy, error) {
	for _, p := range b.Policies {
		if p.Lang == lang {
			return p, nil
		}
	}
	var p DraftPolicy
	err := s.pool.QueryRow(ctx, `SELECT lang, delivery_cost, delivery_time, free_delivery_from, min_order,
		prepayment, installment, return_period, warranty
		FROM xchats.ai_policies WHERE organization_id = $1 AND lang = $2`, orgID, lang).
		Scan(&p.Lang, &p.DeliveryCost, &p.DeliveryTime, &p.FreeDeliveryFrom, &p.MinOrder,
			&p.Prepayment, &p.Installment, &p.ReturnPeriod, &p.Warranty)
	if errors.Is(err, pgx.ErrNoRows) {
		return DraftPolicy{Lang: lang}, nil
	}
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
// selects the WHOLE draft; a non-empty kind+key picks one entity.
type ApproveSelector struct {
	Kind string // "" | "topics" | "assets" | "tariffs" | "products" | "contacts" | "policies"
	Key  string // slug | ref | ref | ref | lang | lang
}

type approveSet struct {
	topics   []DraftTopic
	assets   []DraftAsset
	tariffs  []DraftTariff
	products []DraftProduct
	contacts []DraftContact
	policies []DraftPolicy
	deletes  []DraftDelete
}

func (a approveSet) empty() bool {
	return len(a.topics)+len(a.assets)+len(a.tariffs)+len(a.products)+len(a.contacts)+len(a.policies)+len(a.deletes) == 0
}

// Approve validates the resulting live set against the deterministic gate, then
// materializes the selection into the live typed tables on their natural key,
// applies matching deletes, removes the applied entries from the blob, and
// appends an audit-log row. blobExists reports whether an asset's bytes are
// present (nil skips the dangling-blob check).
func (s *Store) Approve(ctx context.Context, orgID uuid.UUID, sel ApproveSelector, blobExists func(ref string) bool) error {
	blob, _, _, err := s.readDraftBlob(ctx, orgID)
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
	resulting := mergeForGate(live, set.topics, set.assets, set.deletes)
	// Pending requests block the WHOLE-draft approve (sel.Kind == "") — but an
	// unrelated unanswered popup must not hold a single row's approval hostage,
	// so a per-entity approve skips that reason (content checks below still run).
	var pending int
	if sel.Kind == "" {
		if pending, err = s.pendingRequestCount(ctx, orgID); err != nil {
			return err
		}
	}
	if reasons := gate(resulting, pending, blobExists); len(reasons) > 0 {
		return &GateError{Reasons: reasons}
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	for _, t := range set.topics {
		if _, err := tx.Exec(ctx, `INSERT INTO xchats.ai_topics
			(organization_id, slug, lang, title, keywords, body_md)
			VALUES ($1,$2,$3,$4,$5,$6)
			ON CONFLICT (organization_id, slug) DO UPDATE SET
				lang=EXCLUDED.lang, title=EXCLUDED.title, keywords=EXCLUDED.keywords,
				body_md=EXCLUDED.body_md, updated_at=now()`,
			orgID, t.Slug, orDefault(t.Lang, "ru"), t.Title, t.Keywords, t.BodyMD); err != nil {
			return err
		}
	}
	for _, a := range set.assets {
		if _, err := tx.Exec(ctx, `INSERT INTO xchats.ai_assets
			(organization_id, ref, asset_kind, owner_kind, owner_ref, title, description, asset_url, lang)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
			ON CONFLICT (organization_id, ref) DO UPDATE SET
				asset_kind=EXCLUDED.asset_kind, owner_kind=EXCLUDED.owner_kind, owner_ref=EXCLUDED.owner_ref,
				title=EXCLUDED.title, description=EXCLUDED.description, asset_url=EXCLUDED.asset_url,
				lang=EXCLUDED.lang, updated_at=now()`,
			orgID, a.Ref, orDefault(a.Kind, "image"), a.OwnerKind, a.OwnerRef, a.Title, a.Description, a.URL, orDefault(a.Lang, "ru")); err != nil {
			return err
		}
	}
	for _, t := range set.tariffs {
		if err := upsertTariffRow(ctx, tx, orgID, domain.Tariff{
			Ref: t.Ref, Lang: t.Lang, Name: t.Name, Price: t.Price, LimitText: t.LimitText, Fee: t.Fee,
			Summary: t.Summary, PricingType: t.PricingType, Advantages: t.Advantages, Disadvantages: t.Disadvantages,
		}); err != nil {
			return err
		}
	}
	for _, p := range set.products {
		if err := upsertProductRow(ctx, tx, orgID, domain.Product{
			Ref: p.Ref, Lang: p.Lang, Name: p.Name, Price: p.Price, Description: p.Description, Category: p.Category,
		}); err != nil {
			return err
		}
	}
	for _, c := range set.contacts {
		if err := upsertContactRow(ctx, tx, orgID, domain.Contact{
			Lang: c.Lang, WhatsApp: c.WhatsApp, Email: c.Email, Address: c.Address, Legal: c.Legal, CallbackTime: c.CallbackTime,
			WorkingHours: c.WorkingHours, Phone: c.Phone, Website: c.Website, Instagram: c.Instagram,
		}); err != nil {
			return err
		}
	}
	for _, p := range set.policies {
		if err := upsertPolicyRow(ctx, tx, orgID, domain.Policy{
			Lang: p.Lang, DeliveryCost: p.DeliveryCost, DeliveryTime: p.DeliveryTime, FreeDeliveryFrom: p.FreeDeliveryFrom,
			MinOrder: p.MinOrder, Prepayment: p.Prepayment, Installment: p.Installment,
			ReturnPeriod: p.ReturnPeriod, Warranty: p.Warranty,
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
	return s.writeDraftBlob(ctx, orgID, func(b *DraftBlob) error {
		for _, t := range set.topics {
			b.removeTopic(t.Slug)
		}
		for _, a := range set.assets {
			b.removeAsset(a.Ref)
		}
		for _, t := range set.tariffs {
			b.removeTariff(t.Ref)
		}
		for _, p := range set.products {
			b.removeProduct(p.Ref)
		}
		for _, c := range set.contacts {
			b.removeContact(c.Lang)
		}
		for _, p := range set.policies {
			b.removePolicy(p.Lang)
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
func applyDelete(ctx context.Context, tx pgx.Tx, orgID uuid.UUID, d DraftDelete) error {
	var q string
	switch d.Kind {
	case "topic":
		q = `DELETE FROM xchats.ai_topics WHERE organization_id=$1 AND slug=$2`
	case "asset":
		q = `DELETE FROM xchats.ai_assets WHERE organization_id=$1 AND ref=$2`
	case "tariff":
		q = `DELETE FROM xchats.ai_tariffs WHERE organization_id=$1 AND ref=$2`
	case "product":
		q = `DELETE FROM xchats.ai_products WHERE organization_id=$1 AND ref=$2`
	case "contact":
		q = `DELETE FROM xchats.ai_contacts WHERE organization_id=$1 AND lang=$2`
	case "policy":
		q = `DELETE FROM xchats.ai_policies WHERE organization_id=$1 AND lang=$2`
	default:
		return nil
	}
	_, err := tx.Exec(ctx, q, orgID, d.Key)
	return err
}

func approveNote(sel ApproveSelector, set approveSet) string {
	if sel.Kind != "" {
		return fmt.Sprintf("approved %s %s", strings.TrimSuffix(sel.Kind, "s"), sel.Key)
	}
	return fmt.Sprintf("approved %d topic(s), %d asset(s), %d tariff(s), %d product(s), %d contact(s), %d policy(-ies), %d deletion(s)",
		len(set.topics), len(set.assets), len(set.tariffs), len(set.products), len(set.contacts), len(set.policies), len(set.deletes))
}

// selectApproved picks the blob entries an ApproveSelector targets. Deletes are
// keyed by entity kind (singular): topic|asset|tariff|product|contact|policy.
func selectApproved(b DraftBlob, sel ApproveSelector) approveSet {
	if sel.Kind == "" {
		return approveSet{b.Topics, b.Assets, b.Tariffs, b.Products, b.Contacts, b.Policies, b.Deletes}
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
	case "assets":
		for _, a := range b.Assets {
			if a.Ref == sel.Key {
				set.assets = append(set.assets, a)
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
		for _, c := range b.Contacts {
			if c.Lang == sel.Key {
				set.contacts = append(set.contacts, c)
			}
		}
	case "policies":
		for _, p := range b.Policies {
			if p.Lang == sel.Key {
				set.policies = append(set.policies, p)
			}
		}
	}
	return set
}

// mergeForGate builds the resulting live snapshot the gate validates: live topics
// + assets with the approved entries applied on top, matching deletes removed.
// Facts are typed columns validated at reply-render time (fail closed), so the
// gate — and this merge — do not touch them.
func mergeForGate(live *domain.Snapshot, topics []DraftTopic, assets []DraftAsset, deletes []DraftDelete) *domain.Snapshot {
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
		nt := domain.Topic{Slug: t.Slug, Language: t.Lang, Title: t.Title, Keywords: t.Keywords, BodyMD: t.BodyMD}
		if i, ok := tIdx[t.Slug]; ok {
			out.Topics[i] = nt
		} else {
			out.Topics = append(out.Topics, nt)
		}
	}

	aIdx := map[string]int{}
	for _, a := range live.Assets {
		if del["asset:"+a.Ref] {
			continue
		}
		out.Assets = append(out.Assets, a)
		aIdx[a.Ref] = len(out.Assets) - 1
	}
	for _, a := range assets {
		na := domain.Asset{Ref: a.Ref, Kind: a.Kind, Title: a.Title, Description: a.Description, URL: a.URL, Language: a.Lang}
		if a.OwnerKind == "" || a.OwnerKind == "topic" {
			na.TopicSlug = a.OwnerRef
		}
		if i, ok := aIdx[a.Ref]; ok {
			out.Assets[i] = na
		} else {
			out.Assets = append(out.Assets, na)
		}
	}
	return out
}
