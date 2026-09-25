// Package kbstore is the writable Knowledge Base data layer: it loads the LIVE
// KB the brain reasons from (ai_assistants/ai_topics and the typed fact tables,
// each keyed DIRECTLY on organization_id — 15 Decision 1), and owns the Playground's
// single draft blob (kbd_draft) + Approve (validate → materialize → clear — 15
// Decisions 3–4). There is no version, no snapshot clone, no publish/rollback:
// live tables hold live rows only; a pending edit lives in the blob until
// approved.
//
// The brain's read contract (*domain.Snapshot) is unchanged — only its SOURCE
// moves from the Go literal (internal/brain/seed.go) to these tables. The brain
// never touches the draft blob (see draft.go).
package kbstore

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/aiprompt"
	"github.com/yerassyldanay/xchats/backend/internal/brain/domain"
	"github.com/yerassyldanay/xchats/backend/internal/dbx"
	sqlitemigrations "github.com/yerassyldanay/xchats/backend/migrations/sqlite"
)

// ErrStale is returned when an optimistic-concurrency check (If-Match) fails: the
// draft blob moved since the client loaded it.
var ErrStale = errors.New("kbstore: stale draft write")

// ErrUnknownKind is returned for an unrecognized entity kind.
var ErrUnknownKind = errors.New("kbstore: unknown row kind")

// dbtx is satisfied by both *dbx.DB and *dbx.Tx — an alias, not a new type,
// so every "db dbtx" / "tx dbtx" parameter declared across this package
// resolves structurally with zero further changes.
type dbtx = dbx.DBTX

// New opens (or attaches to an already shared-by-path) dbPath and returns a
// ready Store. Safe to call more than once for the same path within this
// process — see internal/store.New's doc comment for how the persistence
// packages end up sharing one physical connection via internal/dbx.Open.
func New(ctx context.Context, dbPath string) (*Store, error) {
	db, err := dbx.Open(ctx, dbPath)
	if err != nil {
		return nil, err
	}
	if err := dbx.RunMigrations(ctx, db, sqlitemigrations.FS); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}

// Store wraps the SQLite pool with KB operations.
type Store struct {
	db *dbx.DB
}

// Close releases this Store's reference to the pool.
func (s *Store) Close() { _ = s.db.Close() }

// ---------------------------------------------------------------------------
// Live load (the brain's source) + seed
// ---------------------------------------------------------------------------

// LoadLive returns the org's live KB as a brain-ready *domain.Snapshot. Live
// tables hold live rows only, so there is no review/version filter to apply.
func (s *Store) LoadLive(ctx context.Context, orgID uuid.UUID) (*domain.Snapshot, error) {
	return loadLive(ctx, s.db, orgID)
}

// loadLive is LoadLive's dbtx-parameterized core. ApproveVersioned
// (draft.go) calls this directly with its own already-locked transaction
// instead of going through the Store method, which would check out a
// SEPARATE pool connection while that transaction holds the kbd_draft row
// lock — the exact pool-exhaustion deadlock class identityIndex's doc
// comment (mcp_read.go) describes, just with Approve's lock instead of
// writeDraftBlobVersioned's.
func loadLive(ctx context.Context, db dbtx, orgID uuid.UUID) (*domain.Snapshot, error) {
	snap := &domain.Snapshot{Loaded: time.Now()}
	err := db.QueryRow(ctx, `
		SELECT persona, mission, guardrails, language_policy, reply_max_words
		FROM ai_assistants WHERE organization_id = $1`, orgID).
		Scan(&snap.Config.Persona, &snap.Config.Mission, &snap.Config.Guardrails,
			&snap.Config.LanguagePolicy, &snap.Config.ReplyMaxWords)
	if err != nil && !errors.Is(err, dbx.ErrNoRows) {
		return nil, err
	}
	if err := loadLiveContent(ctx, db, orgID, snap); err != nil {
		return nil, err
	}
	return snap, nil
}

// loadLiveContent fills Topics and the typed fact tables from the live ai_
// tables for an org.
func loadLiveContent(ctx context.Context, db dbtx, orgID uuid.UUID, snap *domain.Snapshot) error {
	trows, err := db.Query(ctx, `SELECT slug, title, body_md
		FROM ai_topics WHERE organization_id = $1 ORDER BY created_at`, orgID)
	if err != nil {
		return err
	}
	for trows.Next() {
		var t domain.Topic
		if err := trows.Scan(&t.Slug, &t.Title, &t.BodyMD); err != nil {
			trows.Close()
			return err
		}
		snap.Topics = append(snap.Topics, t)
	}
	trows.Close()
	if err := trows.Err(); err != nil {
		return err
	}

	trows2, err := db.Query(ctx, `SELECT ref, name, price, limit_text, fee, summary, pricing_type, advantages, disadvantages
		FROM ai_tariffs WHERE organization_id = $1 ORDER BY created_at`, orgID)
	if err != nil {
		return err
	}
	for trows2.Next() {
		var t domain.Tariff
		if err := trows2.Scan(&t.Ref, &t.Name, &t.Price, &t.LimitText, &t.Fee, &t.Summary, &t.PricingType, &t.Advantages, &t.Disadvantages); err != nil {
			trows2.Close()
			return err
		}
		snap.Tariffs = append(snap.Tariffs, t)
	}
	trows2.Close()
	if err := trows2.Err(); err != nil {
		return err
	}

	// availability is a dead legacy column (plan/database-schema.md: not part
	// of the target) — no longer read; domain.Product.Availability stays
	// permanently empty for a DB-backed snapshot.
	prows, err := db.Query(ctx, `SELECT ref, name, price, description, category
		FROM ai_products WHERE organization_id = $1 ORDER BY created_at`, orgID)
	if err != nil {
		return err
	}
	for prows.Next() {
		var p domain.Product
		if err := prows.Scan(&p.Ref, &p.Name, &p.Price, &p.Description, &p.Category); err != nil {
			prows.Close()
			return err
		}
		snap.Products = append(snap.Products, p)
	}
	prows.Close()
	if err := prows.Err(); err != nil {
		return err
	}

	crows, err := db.Query(ctx, `SELECT whatsapp, email, address, legal_information, callback_time,
		working_hours, phone, website, instagram
		FROM ai_contacts WHERE organization_id = $1 ORDER BY created_at`, orgID)
	if err != nil {
		return err
	}
	for crows.Next() {
		var c domain.Contact
		var legalInfo *string
		if err := crows.Scan(&c.WhatsApp, &c.Email, &c.Address, &legalInfo, &c.CallbackTime,
			&c.WorkingHours, &c.Phone, &c.Website, &c.Instagram); err != nil {
			crows.Close()
			return err
		}
		c.Legal = strOrEmpty(legalInfo)
		snap.Contacts = append(snap.Contacts, c)
	}
	crows.Close()
	if err := crows.Err(); err != nil {
		return err
	}

	polrows, err := db.Query(ctx, `SELECT delivery_cost, delivery_in_days, free_delivery_from, min_order,
		prepayment, installment, return_period_in_days, warranty
		FROM ai_policies WHERE organization_id = $1 ORDER BY created_at`, orgID)
	if err != nil {
		return err
	}
	for polrows.Next() {
		var p domain.Policy
		var deliveryInDays, returnPeriodInDays *string
		if err := polrows.Scan(&p.DeliveryCost, &deliveryInDays, &p.FreeDeliveryFrom, &p.MinOrder,
			&p.Prepayment, &p.Installment, &returnPeriodInDays, &p.Warranty); err != nil {
			polrows.Close()
			return err
		}
		p.DeliveryTime = strOrEmpty(deliveryInDays)
		p.ReturnPeriod = strOrEmpty(returnPeriodInDays)
		snap.Policies = append(snap.Policies, p)
	}
	polrows.Close()
	if err := polrows.Err(); err != nil {
		return err
	}

	snap.Facts = domain.NewFactBook(snap.Tariffs, snap.Products, snap.Contacts, snap.Policies)
	return nil
}

// SeedLiveIfEmpty inserts the given snapshot as the org's live KB when it has no
// topics yet — so the brain keeps answering from the DB on first boot. Idempotent:
// a no-op once the org has any live topic.
func (s *Store) SeedLiveIfEmpty(ctx context.Context, orgID uuid.UUID, seed *domain.Snapshot) error {
	var exists bool
	if err := s.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM ai_topics WHERE organization_id = $1)`,
		orgID).Scan(&exists); err != nil {
		return err
	}
	if exists {
		return nil
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `INSERT INTO ai_assistants
		(organization_id, persona, mission, guardrails, language_policy, reply_max_words)
		VALUES ($1,$2,$3,$4,$5,$6)
		ON CONFLICT (organization_id) DO UPDATE SET
			persona = EXCLUDED.persona, mission = EXCLUDED.mission, guardrails = EXCLUDED.guardrails,
			language_policy = EXCLUDED.language_policy, reply_max_words = EXCLUDED.reply_max_words, updated_at = strftime('%Y-%m-%d %H:%M:%f','now')`,
		orgID, seed.Config.Persona, seed.Config.Mission, seed.Config.Guardrails,
		seed.Config.LanguagePolicy, orDefaultInt(seed.Config.ReplyMaxWords, 120)); err != nil {
		return err
	}
	if err := insertLiveContent(ctx, tx, orgID, seed); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// insertLiveContent upserts a snapshot's topics/assets/typed-facts as live rows.
// Shared by SeedLiveIfEmpty and Approve (both write the same live shape).
// SeedLiveIfEmpty only ever reaches here for a brand-new org (guarded on
// ai_topics being empty), so the ON CONFLICT branch below is never hit in
// production (main.go never even calls it — test-only path); the domain
// snapshot carries no media/sales_status/in_stock opinion, so this seeds the
// schema defaults (true / 'active' / no media) exactly as before this file
// gained those columns.
func insertLiveContent(ctx context.Context, tx execer, orgID uuid.UUID, snap *domain.Snapshot) error {
	for _, t := range snap.Topics {
		if err := upsertTopicRow(ctx, tx, orgID, DraftTopic{Slug: t.Slug, Title: t.Title, BodyMD: t.BodyMD}); err != nil {
			return err
		}
	}
	for _, t := range snap.Tariffs {
		if err := upsertTariffRow(ctx, tx, orgID, DraftTariff{
			Ref: t.Ref, Name: t.Name, Price: t.Price, LimitText: t.LimitText, Fee: t.Fee,
			Summary: t.Summary, PricingType: t.PricingType, Advantages: t.Advantages, Disadvantages: t.Disadvantages,
		}); err != nil {
			return err
		}
	}
	for _, p := range snap.Products {
		if err := upsertProductRow(ctx, tx, orgID, DraftProduct{
			Ref: p.Ref, Name: p.Name, Price: p.Price, Description: p.Description, Category: p.Category,
			AvailabilityStatus: "in_stock",
		}); err != nil {
			return err
		}
	}
	for _, c := range snap.Contacts {
		if err := upsertContactRow(ctx, tx, orgID, DraftContact{
			WhatsApp: c.WhatsApp, Email: c.Email, Address: c.Address, LegalInformation: c.Legal,
			CallbackTime: c.CallbackTime, WorkingHours: c.WorkingHours, Phone: c.Phone,
			Website: c.Website, Instagram: c.Instagram,
		}); err != nil {
			return err
		}
	}
	for _, p := range snap.Policies {
		// OutsideZonesNote "": the brain-seed snapshot has no such concept.
		if err := upsertPolicyRow(ctx, tx, orgID, DraftPolicy{
			DeliveryCost: p.DeliveryCost, DeliveryInDays: p.DeliveryTime, FreeDeliveryFrom: p.FreeDeliveryFrom,
			MinOrder: p.MinOrder, Prepayment: p.Prepayment, Installment: p.Installment,
			ReturnPeriodInDays: p.ReturnPeriod, Warranty: p.Warranty,
		}); err != nil {
			return err
		}
	}
	return nil
}

// execer is satisfied by both *dbx.DB and *dbx.Tx (same Exec shape), so the
// row-upsert helpers below run identically inside a multi-statement transaction
// (Approve, SeedLiveIfEmpty) or directly on the pool (the /kb/* live-write path,
// where each call is its own single-statement write — no cross-row atomicity to
// preserve).
type execer = dbx.DBTX

// upsertTopicRow / upsertTariffRow / upsertProductRow / upsertContactRow /
// upsertPolicyRow write one complete typed-fact row (verbatim columns,
// including the canonical media/sales_status/in_stock columns). Shared by
// insertLiveContent (seed), Approve (materialize), and the /kb/* live-write
// path (live.go) — every caller already resolves the COMPLETE row it wants
// written (read-modify-write happens upstream: currentTopic/currentTariff/
// currentProduct for the draft path, currentLive*Tx for the live-write path),
// so these helpers never need a "leave unchanged" sentinel of their own.
func upsertTopicRow(ctx context.Context, tx execer, orgID uuid.UUID, t DraftTopic) error {
	if _, err := tx.Exec(ctx, `INSERT INTO ai_topics
		(organization_id, slug, title, body_md, featured_image, illustration_images,
		 explainer_videos, reference_documents)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		ON CONFLICT (organization_id, slug) DO UPDATE SET
			title=EXCLUDED.title, body_md=EXCLUDED.body_md,
			featured_image=EXCLUDED.featured_image, illustration_images=EXCLUDED.illustration_images,
			explainer_videos=EXCLUDED.explainer_videos, reference_documents=EXCLUDED.reference_documents,
			updated_at=strftime('%Y-%m-%d %H:%M:%f','now')`,
		orgID, t.Slug, t.Title, t.BodyMD, t.FeaturedImage, dbx.UUIDArray(nonNilUUIDs(t.IllustrationImages)),
		dbx.UUIDArray(nonNilUUIDs(t.ExplainerVideos)), dbx.UUIDArray(nonNilUUIDs(t.ReferenceDocuments))); err != nil {
		return fmt.Errorf("insert topic %s: %w", t.Slug, err)
	}
	return nil
}

func upsertTariffRow(ctx context.Context, tx execer, orgID uuid.UUID, t DraftTariff) error {
	if _, err := tx.Exec(ctx, `INSERT INTO ai_tariffs
		(organization_id, ref, name, price, limit_text, fee, summary, pricing_type, advantages, disadvantages,
		 best_for, not_for, additional_facts,
		 sales_status, featured_image, pricing_images, explainer_videos, terms_documents)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18)
		ON CONFLICT (organization_id, ref) DO UPDATE SET
			name=EXCLUDED.name, price=EXCLUDED.price, limit_text=EXCLUDED.limit_text, fee=EXCLUDED.fee,
			summary=EXCLUDED.summary, pricing_type=EXCLUDED.pricing_type, advantages=EXCLUDED.advantages,
			disadvantages=EXCLUDED.disadvantages, best_for=EXCLUDED.best_for, not_for=EXCLUDED.not_for,
			additional_facts=EXCLUDED.additional_facts, sales_status=EXCLUDED.sales_status,
			featured_image=EXCLUDED.featured_image, pricing_images=EXCLUDED.pricing_images,
			explainer_videos=EXCLUDED.explainer_videos, terms_documents=EXCLUDED.terms_documents, updated_at=strftime('%Y-%m-%d %H:%M:%f','now')`,
		orgID, t.Ref, t.Name, t.Price, t.LimitText, t.Fee, t.Summary,
		orDefault(t.PricingType, "fixed"), t.Advantages, t.Disadvantages, t.BestFor, t.NotFor,
		aiprompt.FactsColumn(t.AdditionalFacts), orDefault(t.SalesStatus, "active"),
		t.FeaturedImage, dbx.UUIDArray(nonNilUUIDs(t.PricingImages)), dbx.UUIDArray(nonNilUUIDs(t.ExplainerVideos)), dbx.UUIDArray(nonNilUUIDs(t.TermsDocuments))); err != nil {
		return fmt.Errorf("insert tariff %s: %w", t.Ref, err)
	}
	return nil
}

// upsertProductRow writes one ai_products row.
func upsertProductRow(ctx context.Context, tx execer, orgID uuid.UUID, p DraftProduct) error {
	if _, err := tx.Exec(ctx, `INSERT INTO ai_products
		(organization_id, ref, name, price, description, category, brand, advantages, disadvantages,
		 best_for, not_for, availability_status, availability_note, installation_terms, warranty_terms,
		 additional_facts, sales_status,
		 featured_image, gallery_images, demo_videos, certificate_documents, guarantee_documents)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22)
		ON CONFLICT (organization_id, ref) DO UPDATE SET
			name=EXCLUDED.name, price=EXCLUDED.price, description=EXCLUDED.description,
			category=EXCLUDED.category, brand=EXCLUDED.brand, advantages=EXCLUDED.advantages,
			disadvantages=EXCLUDED.disadvantages, best_for=EXCLUDED.best_for, not_for=EXCLUDED.not_for,
			availability_status=EXCLUDED.availability_status, availability_note=EXCLUDED.availability_note,
			installation_terms=EXCLUDED.installation_terms, warranty_terms=EXCLUDED.warranty_terms,
			additional_facts=EXCLUDED.additional_facts, sales_status=EXCLUDED.sales_status,
			featured_image=EXCLUDED.featured_image, gallery_images=EXCLUDED.gallery_images,
			demo_videos=EXCLUDED.demo_videos, certificate_documents=EXCLUDED.certificate_documents,
			guarantee_documents=EXCLUDED.guarantee_documents,
			updated_at=strftime('%Y-%m-%d %H:%M:%f','now')`,
		orgID, p.Ref, p.Name, p.Price, p.Description, p.Category, p.Brand, p.Advantages, p.Disadvantages,
		p.BestFor, p.NotFor, orDefault(p.AvailabilityStatus, "in_stock"), p.AvailabilityNote, p.InstallationTerms, p.WarrantyTerms,
		aiprompt.FactsColumn(p.AdditionalFacts), orDefault(p.SalesStatus, "active"),
		p.FeaturedImage, dbx.UUIDArray(nonNilUUIDs(p.GalleryImages)), dbx.UUIDArray(nonNilUUIDs(p.DemoVideos)),
		dbx.UUIDArray(nonNilUUIDs(p.CertificateDocuments)), dbx.UUIDArray(nonNilUUIDs(p.GuaranteeDocuments))); err != nil {
		return fmt.Errorf("insert product %s: %w", p.Ref, err)
	}
	return nil
}

// upsertTariffInfoRow writes the org's ai_tariff_info singleton row — a
// smaller clone of upsertContactRow/upsertPolicyRow (ON CONFLICT on
// organization_id alone, no natural key).
func upsertTariffInfoRow(ctx context.Context, tx execer, orgID uuid.UUID, ti DraftTariffInfo) error {
	if _, err := tx.Exec(ctx, `INSERT INTO ai_tariff_info (organization_id, additional_facts)
		VALUES ($1,$2)
		ON CONFLICT (organization_id) DO UPDATE SET
			additional_facts=EXCLUDED.additional_facts, updated_at=strftime('%Y-%m-%d %H:%M:%f','now')`,
		orgID, aiprompt.FactsColumn(ti.AdditionalFacts)); err != nil {
		return fmt.Errorf("insert tariff_info: %w", err)
	}
	return nil
}

func upsertContactRow(ctx context.Context, tx execer, orgID uuid.UUID, c DraftContact) error {
	if _, err := tx.Exec(ctx, `INSERT INTO ai_contacts
		(organization_id, whatsapp, email, address, legal_information, callback_time,
		 working_hours, phone, website, instagram, contact_card_image, location_map_image,
		 company_legal_documents, booking_url, schedule)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)
		ON CONFLICT (organization_id) DO UPDATE SET
			whatsapp=EXCLUDED.whatsapp, email=EXCLUDED.email, address=EXCLUDED.address,
			legal_information=EXCLUDED.legal_information, callback_time=EXCLUDED.callback_time,
			working_hours=EXCLUDED.working_hours, phone=EXCLUDED.phone,
			website=EXCLUDED.website, instagram=EXCLUDED.instagram,
			contact_card_image=EXCLUDED.contact_card_image, location_map_image=EXCLUDED.location_map_image,
			company_legal_documents=EXCLUDED.company_legal_documents,
			booking_url=EXCLUDED.booking_url, schedule=EXCLUDED.schedule, updated_at=strftime('%Y-%m-%d %H:%M:%f','now')`,
		orgID, c.WhatsApp, c.Email, c.Address, c.LegalInformation, c.CallbackTime,
		c.WorkingHours, c.Phone, c.Website, c.Instagram, c.ContactCardImage, c.LocationMapImage,
		dbx.UUIDArray(nonNilUUIDs(c.CompanyLegalDocuments)), c.BookingURL, aiprompt.ScheduleColumn(c.Schedule)); err != nil {
		return fmt.Errorf("insert contact: %w", err)
	}
	return nil
}

// upsertSpecialistRow writes one ai_specialists row — upsertProductRow's own
// shape (salon vertical, PLAN.md).
func upsertSpecialistRow(ctx context.Context, tx execer, orgID uuid.UUID, sp DraftSpecialist) error {
	if _, err := tx.Exec(ctx, `INSERT INTO ai_specialists
		(organization_id, ref, full_name, title, experience, schedule, booking_url, portfolio_images, sales_status)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		ON CONFLICT (organization_id, ref) DO UPDATE SET
			full_name=EXCLUDED.full_name, title=EXCLUDED.title, experience=EXCLUDED.experience,
			schedule=EXCLUDED.schedule, booking_url=EXCLUDED.booking_url,
			portfolio_images=EXCLUDED.portfolio_images, sales_status=EXCLUDED.sales_status,
			updated_at=strftime('%Y-%m-%d %H:%M:%f','now')`,
		orgID, sp.Ref, sp.FullName, sp.Title, sp.Experience, aiprompt.ScheduleColumn(sp.Schedule), sp.BookingURL,
		dbx.UUIDArray(nonNilUUIDs(sp.PortfolioImages)), orDefault(sp.SalesStatus, "active")); err != nil {
		return fmt.Errorf("insert specialist %s: %w", sp.Ref, err)
	}
	return nil
}

// upsertServiceRow writes one ai_services row — upsertProductRow's own
// shape (salon vertical, PLAN.md). Duration binds directly as *int: nil
// persists SQL NULL, a non-nil pointer persists that integer (both
// directions confirmed against modernc.org/sqlite's database/sql support
// for a **T scan/bind target).
func upsertServiceRow(ctx context.Context, tx execer, orgID uuid.UUID, sv DraftService) error {
	if _, err := tx.Exec(ctx, `INSERT INTO ai_services
		(organization_id, ref, parent_ref, service_type, category, name, price, duration, description,
		 specialist_refs, sales_status)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		ON CONFLICT (organization_id, ref) DO UPDATE SET
			parent_ref=EXCLUDED.parent_ref, service_type=EXCLUDED.service_type, category=EXCLUDED.category,
			name=EXCLUDED.name, price=EXCLUDED.price, duration=EXCLUDED.duration, description=EXCLUDED.description,
			specialist_refs=EXCLUDED.specialist_refs, sales_status=EXCLUDED.sales_status,
			updated_at=strftime('%Y-%m-%d %H:%M:%f','now')`,
		orgID, sv.Ref, sv.ParentRef, orDefault(sv.ServiceType, "base"), sv.Category, sv.Name, sv.Price, sv.Duration,
		sv.Description, dbx.StringArray(nonNilStrings(sv.SpecialistRefs)), orDefault(sv.SalesStatus, "active")); err != nil {
		return fmt.Errorf("insert service %s: %w", sv.Ref, err)
	}
	return nil
}

// loadSpecialistRows reads every specialist for the org, live-only (no
// draft concept) — loadZoneRows' own shape (zones.go), used both by
// mergedView (draft.go, the overlay base) and — via mergedView(blob=empty)
// — by LiveView.
func loadSpecialistRows(ctx context.Context, db dbtx, orgID uuid.UUID) ([]SpecialistRow, error) {
	rows, err := db.Query(ctx, `SELECT ref, full_name, title, experience, schedule, booking_url, portfolio_images,
		sales_status, updated_at
		FROM ai_specialists WHERE organization_id = $1 ORDER BY created_at`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SpecialistRow
	for rows.Next() {
		var sp SpecialistRow
		if err := rows.Scan(&sp.Ref, &sp.FullName, &sp.Title, &sp.Experience, (*aiprompt.ScheduleColumn)(&sp.Schedule),
			&sp.BookingURL, (*dbx.UUIDArray)(&sp.PortfolioImages), &sp.SalesStatus, &sp.UpdatedAt); err != nil {
			return nil, err
		}
		sp.ID = sp.Ref
		out = append(out, sp)
	}
	return out, rows.Err()
}

// loadServiceRows is loadSpecialistRows' twin for ai_services.
func loadServiceRows(ctx context.Context, db dbtx, orgID uuid.UUID) ([]ServiceRow, error) {
	rows, err := db.Query(ctx, `SELECT ref, parent_ref, service_type, category, name, price, duration, description,
		specialist_refs, sales_status, updated_at
		FROM ai_services WHERE organization_id = $1 ORDER BY created_at`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ServiceRow
	for rows.Next() {
		var sv ServiceRow
		if err := rows.Scan(&sv.Ref, &sv.ParentRef, &sv.ServiceType, &sv.Category, &sv.Name, &sv.Price, &sv.Duration,
			&sv.Description, (*dbx.StringArray)(&sv.SpecialistRefs), &sv.SalesStatus, &sv.UpdatedAt); err != nil {
			return nil, err
		}
		sv.ID = sv.Ref
		out = append(out, sv)
	}
	return out, rows.Err()
}

// resultingServicesForGate is resultingZonesForGate's (zones.go) twin for
// ai_services: live, overridden/extended by this approve batch's pending
// service upserts, minus anything staged for deletion — the same
// live-∪-approved-minus-deletes shape serviceGateReasons is checked against.
func resultingServicesForGate(live []ServiceRow, upserts []DraftService, deletes []DraftDelete) []ServiceRow {
	del := map[string]bool{}
	for _, d := range deletes {
		if d.Kind == deleteKindFor(KBTypeService) {
			del[d.Key] = true
		}
	}
	idx := map[string]int{}
	var out []ServiceRow
	for _, sv := range live {
		if del[sv.Ref] {
			continue
		}
		out = append(out, sv)
		idx[sv.Ref] = len(out) - 1
	}
	for _, u := range upserts {
		row := ServiceRow{
			Ref: u.Ref, ParentRef: u.ParentRef, ServiceType: orDefault(u.ServiceType, "base"),
			Category: u.Category, Name: u.Name, Price: u.Price, Duration: u.Duration,
			Description: u.Description, SpecialistRefs: u.SpecialistRefs, SalesStatus: orDefault(u.SalesStatus, "active"),
		}
		if i, ok := idx[u.Ref]; ok {
			out[i] = row
		} else {
			out = append(out, row)
			idx[u.Ref] = len(out) - 1
		}
	}
	return out
}

// serviceGateReasons is the pure, table-tested invariant over one org's
// resulting (post-approve) service tree — mirrors aiprompt.buildServiceFacts'
// own fail-closed rule (backend/aiprompt/catalog.go: "archiving a base must
// archive its active children"), enforced here at approve time instead of
// only being discovered later as a hard error breaking every customer reply
// for the org. The live-write path (PutLiveService, live.go) already
// cascades a base's archival to its children automatically; the draft-approve
// path deliberately does not (a staged entry is still under human review —
// see ApproveVersioned's own comment), so this gate is what turns "publish
// an archived base without also archiving its active children" into a clear,
// rejected-up-front error instead of a silently corrupted live KB.
func serviceGateReasons(services []ServiceRow) []GateReason {
	var reasons []GateReason
	byRef := make(map[string]ServiceRow, len(services))
	for _, sv := range services {
		byRef[sv.Ref] = sv
	}
	for _, sv := range services {
		if sv.ServiceType == "base" || sv.ParentRef == "" || sv.SalesStatus != "active" {
			continue
		}
		parent, ok := byRef[sv.ParentRef]
		if !ok || parent.SalesStatus != "active" {
			reasons = append(reasons, GateReason{Kind: "services", Key: sv.Ref, Message: fmt.Sprintf(
				"service %q is active but its base service %q is not — archive %q too, or restore %q, before publishing",
				sv.Ref, sv.ParentRef, sv.Ref, sv.ParentRef)})
		}
	}
	return reasons
}

// upsertPolicyRow writes one ai_policies row — an exact clone of upsertContactRow.
func upsertPolicyRow(ctx context.Context, tx execer, orgID uuid.UUID, p DraftPolicy) error {
	if _, err := tx.Exec(ctx, `INSERT INTO ai_policies
		(organization_id, delivery_cost, delivery_in_days, free_delivery_from, min_order,
		 prepayment, installment, return_period_in_days, warranty, outside_zones_note,
		 commerce_policy_documents)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		ON CONFLICT (organization_id) DO UPDATE SET
			delivery_cost=EXCLUDED.delivery_cost, delivery_in_days=EXCLUDED.delivery_in_days,
			free_delivery_from=EXCLUDED.free_delivery_from, min_order=EXCLUDED.min_order,
			prepayment=EXCLUDED.prepayment, installment=EXCLUDED.installment,
			return_period_in_days=EXCLUDED.return_period_in_days, warranty=EXCLUDED.warranty,
			outside_zones_note=EXCLUDED.outside_zones_note,
			commerce_policy_documents=EXCLUDED.commerce_policy_documents, updated_at=strftime('%Y-%m-%d %H:%M:%f','now')`,
		orgID, p.DeliveryCost, p.DeliveryInDays, p.FreeDeliveryFrom, p.MinOrder,
		p.Prepayment, p.Installment, p.ReturnPeriodInDays, p.Warranty, p.OutsideZonesNote,
		dbx.UUIDArray(nonNilUUIDs(p.CommercePolicyDocuments))); err != nil {
		return fmt.Errorf("insert policy: %w", err)
	}
	return nil
}

// nonNilUUIDs guarantees a non-nil slice for a uuid[] NOT NULL DEFAULT '{}'
// column: a nil Go slice and an empty one are equivalent input to pgx, but
// staying explicit here documents that these columns are never SQL NULL.
func nonNilUUIDs(v []uuid.UUID) []uuid.UUID {
	if v == nil {
		return []uuid.UUID{}
	}
	return v
}

// nonNilStrings is nonNilUUIDs' twin for a text[] NOT NULL DEFAULT '{}'
// column (ai_services.specialist_refs, dbx.StringArray).
func nonNilStrings(v []string) []string {
	if v == nil {
		return []string{}
	}
	return v
}

// upsertConfigRow upserts the org's ai_assistants row. It fixes the silent
// no-op PatchLiveConfig had (live.go): a bare UPDATE hits zero rows when the
// org has no ai_assistants row yet (e.g. a fresh org with no seed/kb-load run
// yet), so a first-ever PATCH /kb/config appeared to succeed but changed
// nothing. ON CONFLICT DO UPDATE with COALESCE mirrors PatchLiveConfig's own
// "only non-nil fields change" contract on the insert path too, using each
// column's own schema default when the row is being created for the first time.
func upsertConfigRow(ctx context.Context, tx execer, orgID uuid.UUID, p ConfigPatch) error {
	_, err := tx.Exec(ctx, `INSERT INTO ai_assistants
		(organization_id, persona, mission, guardrails, language_policy, reply_max_words)
		VALUES ($1, COALESCE($2,''), COALESCE($3,''), COALESCE($4,''), COALESCE($5,''), COALESCE($6,120))
		ON CONFLICT (organization_id) DO UPDATE SET
			persona = COALESCE($2, ai_assistants.persona),
			mission = COALESCE($3, ai_assistants.mission),
			guardrails = COALESCE($4, ai_assistants.guardrails),
			language_policy = COALESCE($5, ai_assistants.language_policy),
			reply_max_words = COALESCE($6, ai_assistants.reply_max_words),
			updated_at = strftime('%Y-%m-%d %H:%M:%f','now')`,
		orgID, p.Persona, p.Mission, p.Guardrails, p.LanguagePolicy, p.ReplyMaxWords)
	return err
}

// auditRow appends one ai_audit_log row — action is 'edit'|'delete' for
// the /kb/* live-write path (no CHECK constraint pins the vocabulary; 'approve'
// remains the Playground Approve's own action). actor is nil-able: a zero
// uuid.UUID (no authenticated user in context) is stored as SQL NULL rather
// than a literal zero UUID, since actor_user_id's FK would otherwise reject it.
func auditRow(ctx context.Context, tx execer, orgID uuid.UUID, actor uuid.UUID, action, note string) error {
	_, err := tx.Exec(ctx, `INSERT INTO ai_audit_log (organization_id, action, actor_user_id, note)
		VALUES ($1,$2,$3,$4)`,
		orgID, action, uuid.NullUUID{UUID: actor, Valid: actor != uuid.Nil}, note)
	return err
}

// ---------------------------------------------------------------------------
// Deterministic gate (the Approve safety boundary — see draft.go · Approve)
// ---------------------------------------------------------------------------

// GateReason is one deterministic approve-gate violation. Kind/Key name the
// offending entity — Kind is one of the same kind strings the frontend's
// own ChangeKind union already uses ("topics", "delivery_zones",
// "policies", ...; draftChanges.ts), Key is that entity's own natural key
// — so a caller can point the operator straight at it rather than reciting
// the message alone (KB-09: "Publishing blocked: Delivery Zone 'Almaty
// Region' is missing a parent zone. [Fix in Delivery Zones Tab →]"). Both
// are "" for a violation with no single offending entity (an
// unresolved-request count spans the whole org, not one row) — a caller
// shows Message alone then.
type GateReason struct {
	Kind    string
	Key     string
	Message string
}

// GateError reports the deterministic approve-gate violations (plan/12 · gate).
type GateError struct{ Reasons []GateReason }

func (e *GateError) Error() string {
	msgs := make([]string, len(e.Reasons))
	for i, r := range e.Reasons {
		msgs[i] = r.Message
	}
	return "publish gate failed: " + strings.Join(msgs, "; ")
}

// gate is the deterministic approve gate (pure, testable): over the resulting
// LIVE set (live ∪ approved, minus deletes), every topic body is pure prose (no
// fact tokens, no literal currency), no request pends. Facts are typed columns
// (validated at reply-render time, fail closed), so the gate does not touch them.
func gate(snap *domain.Snapshot, pendingRequests int) []GateReason {
	var reasons []GateReason
	for _, t := range snap.Topics {
		reasons = append(reasons, gateTopicBody(t.Slug, t.BodyMD)...)
	}
	if pendingRequests > 0 {
		reasons = append(reasons, GateReason{Message: fmt.Sprintf("%d unresolved request(s)", pendingRequests)})
	}
	return reasons
}

// gateTopicBody is the per-topic half of the gate — also reused stand-alone by
// the /kb/* live-write path (live.go).
func gateTopicBody(slug, bodyMD string) []GateReason {
	var reasons []GateReason
	// Topic bodies are pure prose (14 Decision 3): a fact token in a body means
	// stored knowledge is carrying a value — it belongs in a typed column, quoted
	// only in replies.
	if strings.Contains(bodyMD, "{{") {
		reasons = append(reasons, GateReason{Kind: "topics", Key: slug, Message: fmt.Sprintf("topic %q body must be pure prose — no {{...}} tokens", slug)})
	}
	// A literal price/currency amount in a body is an unconfirmed number shipping
	// to customers — the fact belongs in a typed tariff/product column.
	if lit := rawCurrencyRE.FindString(bodyMD); lit != "" {
		reasons = append(reasons, GateReason{Kind: "topics", Key: slug, Message: fmt.Sprintf("topic %q has a literal amount %q — put the fact in a typed column", slug, strings.TrimSpace(lit))})
	}
	return reasons
}

// rawCurrencyRE matches a number immediately followed by a currency marker
// ("25 000 ₸", "9900тг", "$50"): the class of unconfirmed amount that must live in
// a typed fact column, never as a literal in a rendered reply body. Step numbers
// ("1) 2) 3)") and bare counts are intentionally NOT matched.
var rawCurrencyRE = regexp.MustCompile(`(?:[0-9][0-9 \x{00a0}.,]*\s*(?:₸|₽|€|£|тг|тенге|руб)|[$€£]\s*[0-9])`)

// pendingRequestCount is dbtx-parameterized for the same reason loadLive is
// — ApproveVersioned calls it on its own locked transaction, never s.db.
func pendingRequestCount(ctx context.Context, db dbtx, orgID uuid.UUID) (int, error) {
	var n int
	err := db.QueryRow(ctx, `SELECT count(*) FROM kbd_requests
		WHERE organization_id = $1 AND state = 'pending'`, orgID).Scan(&n)
	return n, err
}

// strOrEmpty converts a nullable text column (legal_information,
// delivery_in_days, return_period_in_days — added nullable by migration
// 0011) into the plain string every row/patch type here uses, "" for NULL.
func strOrEmpty(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func orDefaultInt(v, def int) int {
	if v == 0 {
		return def
	}
	return v
}
