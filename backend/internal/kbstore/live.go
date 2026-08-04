package kbstore

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/internal/dbx"
)

// ---------------------------------------------------------------------------
// Live writes — the /kb/* editor surface. Every write here lands directly in
// the live ai_ tables: no draft blob, no approve step. This is the ONLY write
// path LiveView/the /kb/* HTTP handlers use, so a live edit can never leak into
// — or be confused with — a Playground draft (see plan "Playground redesign").
// Content is held to the same bar the approve gate enforces (gateTopicBody,
// kbstore.go), just checked immediately instead of at approve time.
//
// Every function here takes the acting user's id (actor) and runs its write
// plus an ai_audit_log row inside ONE transaction (db.Begin), so the audit
// trail can never desync from what was actually written — either both commit
// or neither does. A read-modify-write function (PatchLiveContacts/
// Policies) additionally takes its "current row" read inside that same
// transaction, closing a lost-update race an earlier version of this file
// had (read + compute on the pool, upsert as a separate statement — a
// concurrent writer's change in between was silently clobbered). Under
// internal/dbx's single-connection design every write transaction already
// serializes at BEGIN (see dbx's package doc), so this same-transaction read
// is sufficient without a PG-style "SELECT ... FOR UPDATE" row lock.
// ---------------------------------------------------------------------------

// PutLiveTopic upserts a topic directly into the live table. Rejects a body
// that is not pure prose (a fact token or a literal currency amount) — the
// exact check the approve gate runs, just enforced at write time here. Reads
// the row FOR UPDATE first (currentLiveTopicTx) and merges only title/body_md
// from in, so this text-only caller (the live editor has no media inputs)
// cannot blank out media an MCP tool already published.
func (s *Store) PutLiveTopic(ctx context.Context, orgID uuid.UUID, actor uuid.UUID, in TopicInput) error {
	if reasons := gateTopicBody(in.Slug, in.BodyMD); len(reasons) > 0 {
		return &GateError{Reasons: reasons}
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	cur, err := currentLiveTopicTx(ctx, tx, orgID, in.Slug)
	if err != nil {
		return err
	}
	cur.Title, cur.BodyMD = in.Title, in.BodyMD
	if err := upsertTopicRow(ctx, tx, orgID, cur); err != nil {
		return err
	}
	if err := auditRow(ctx, tx, orgID, actor, "edit", "topic:"+in.Slug); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func currentLiveTopicTx(ctx context.Context, tx dbtx, orgID uuid.UUID, slug string) (DraftTopic, error) {
	t := DraftTopic{Slug: slug}
	err := tx.QueryRow(ctx, `SELECT title, body_md, featured_image, illustration_images,
		explainer_videos, reference_documents
		FROM ai_topics WHERE organization_id=$1 AND slug=$2`, orgID, slug).
		Scan(&t.Title, &t.BodyMD, &t.FeaturedImage, (*dbx.UUIDArray)(&t.IllustrationImages),
			(*dbx.UUIDArray)(&t.ExplainerVideos), (*dbx.UUIDArray)(&t.ReferenceDocuments))
	if errors.Is(err, dbx.ErrNoRows) {
		return DraftTopic{Slug: slug}, nil
	}
	return t, err
}

// DeleteLiveTopic removes a live topic by slug.
func (s *Store) DeleteLiveTopic(ctx context.Context, orgID uuid.UUID, actor uuid.UUID, slug string) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `DELETE FROM ai_topics WHERE organization_id=$1 AND slug=$2`, orgID, slug); err != nil {
		return err
	}
	if err := auditRow(ctx, tx, orgID, actor, "delete", "topic:"+slug); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// PutLiveTariff upserts a tariff row directly into the live table (no content
// gate — typed fact columns are validated at reply-render time, fail closed).
// Reads the row FOR UPDATE first and merges only the text fields TariffInput
// carries, so this caller (no media inputs) cannot blank out media/
// sales_status an MCP tool already published.
func (s *Store) PutLiveTariff(ctx context.Context, orgID uuid.UUID, actor uuid.UUID, in TariffInput) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	cur, err := currentLiveTariffTx(ctx, tx, orgID, in.Ref)
	if err != nil {
		return err
	}
	cur.Name, cur.Price, cur.LimitText = in.Name, in.Price, in.LimitText
	cur.Fee, cur.Summary, cur.PricingType = in.Fee, in.Summary, in.PricingType
	cur.Advantages, cur.Disadvantages = in.Advantages, in.Disadvantages
	cur.SalesStatus = orDefault(in.SalesStatus, "active")
	if err := upsertTariffRow(ctx, tx, orgID, cur); err != nil {
		return err
	}
	if err := auditRow(ctx, tx, orgID, actor, "edit", "tariff:"+in.Ref); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func currentLiveTariffTx(ctx context.Context, tx dbtx, orgID uuid.UUID, ref string) (DraftTariff, error) {
	t := DraftTariff{Ref: ref, PricingType: "fixed"}
	err := tx.QueryRow(ctx, `SELECT name, price, limit_text, fee, summary, pricing_type, advantages, disadvantages,
		sales_status, featured_image, pricing_images, explainer_videos, terms_documents
		FROM ai_tariffs WHERE organization_id=$1 AND ref=$2`, orgID, ref).
		Scan(&t.Name, &t.Price, &t.LimitText, &t.Fee, &t.Summary, &t.PricingType, &t.Advantages, &t.Disadvantages,
			&t.SalesStatus, &t.FeaturedImage, (*dbx.UUIDArray)(&t.PricingImages), (*dbx.UUIDArray)(&t.ExplainerVideos), (*dbx.UUIDArray)(&t.TermsDocuments))
	if errors.Is(err, dbx.ErrNoRows) {
		return DraftTariff{Ref: ref, PricingType: "fixed"}, nil
	}
	return t, err
}

// DeleteLiveTariff removes a live tariff row by ref.
func (s *Store) DeleteLiveTariff(ctx context.Context, orgID uuid.UUID, actor uuid.UUID, ref string) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `DELETE FROM ai_tariffs WHERE organization_id=$1 AND ref=$2`, orgID, ref); err != nil {
		return err
	}
	if err := auditRow(ctx, tx, orgID, actor, "delete", "tariff:"+ref); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// PutLiveProduct upserts a product row directly into the live table. Reads
// the row FOR UPDATE first and merges the text fields plus InStock (when
// given — nil preserves the current value, same contract as before), so
// this caller cannot blank out sales_status/media an MCP tool already
// published.
func (s *Store) PutLiveProduct(ctx context.Context, orgID uuid.UUID, actor uuid.UUID, in ProductInput) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	cur, err := currentLiveProductTx(ctx, tx, orgID, in.Ref)
	if err != nil {
		return err
	}
	cur.Name, cur.Price = in.Name, in.Price
	cur.Description, cur.Category = in.Description, in.Category
	cur.SalesStatus = orDefault(in.SalesStatus, "active")
	if in.InStock != nil {
		cur.InStock = *in.InStock
	}
	if err := upsertProductRow(ctx, tx, orgID, cur); err != nil {
		return err
	}
	if err := auditRow(ctx, tx, orgID, actor, "edit", "product:"+in.Ref); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func currentLiveProductTx(ctx context.Context, tx dbtx, orgID uuid.UUID, ref string) (DraftProduct, error) {
	p := DraftProduct{Ref: ref, InStock: true}
	err := tx.QueryRow(ctx, `SELECT name, price, description, category, in_stock, sales_status,
		featured_image, gallery_images, demo_videos, certificate_documents, guarantee_documents
		FROM ai_products WHERE organization_id=$1 AND ref=$2`, orgID, ref).
		Scan(&p.Name, &p.Price, &p.Description, &p.Category, &p.InStock, &p.SalesStatus,
			&p.FeaturedImage, (*dbx.UUIDArray)(&p.GalleryImages), (*dbx.UUIDArray)(&p.DemoVideos), (*dbx.UUIDArray)(&p.CertificateDocuments), (*dbx.UUIDArray)(&p.GuaranteeDocuments))
	if errors.Is(err, dbx.ErrNoRows) {
		return DraftProduct{Ref: ref, InStock: true}, nil
	}
	return p, err
}

// DeleteLiveProduct removes a live product row by ref.
func (s *Store) DeleteLiveProduct(ctx context.Context, orgID uuid.UUID, actor uuid.UUID, ref string) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `DELETE FROM ai_products WHERE organization_id=$1 AND ref=$2`, orgID, ref); err != nil {
		return err
	}
	if err := auditRow(ctx, tx, orgID, actor, "delete", "product:"+ref); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// PatchLiveContacts edits the org's live support-contact row — a true
// singleton — starting from its current row (locked FOR UPDATE for this
// transaction) so an omitted field stays unchanged and a concurrent patch
// serializes instead of racing.
func (s *Store) PatchLiveContacts(ctx context.Context, orgID uuid.UUID, actor uuid.UUID, p ContactPatch) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	cur, err := currentLiveContactTx(ctx, tx, orgID)
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
	if err := upsertContactRow(ctx, tx, orgID, cur); err != nil {
		return err
	}
	if err := auditRow(ctx, tx, orgID, actor, "edit", "contacts"); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func currentLiveContactTx(ctx context.Context, tx dbtx, orgID uuid.UUID) (DraftContact, error) {
	var c DraftContact
	var legalInfo *string
	err := tx.QueryRow(ctx, `SELECT whatsapp, email, address, legal_information, callback_time,
		working_hours, phone, website, instagram, contact_card_image, location_map_image,
		company_legal_documents
		FROM ai_contacts WHERE organization_id = $1`, orgID).
		Scan(&c.WhatsApp, &c.Email, &c.Address, &legalInfo, &c.CallbackTime,
			&c.WorkingHours, &c.Phone, &c.Website, &c.Instagram, &c.ContactCardImage, &c.LocationMapImage,
			(*dbx.UUIDArray)(&c.CompanyLegalDocuments))
	if errors.Is(err, dbx.ErrNoRows) {
		return DraftContact{}, nil
	}
	c.LegalInformation = strOrEmpty(legalInfo)
	return c, err
}

// PatchLivePolicies edits the org's live commerce-policy row — a true
// singleton — starting from its current row (locked FOR UPDATE for this
// transaction) so an omitted field stays unchanged — a near-clone of
// PatchLiveContacts, plus a validateZoneWorld re-check before commit: a
// policy edit that leaves the zone/policy world inconsistent (e.g. setting a
// non-blank flat delivery_cost while delivery zones exist) is rejected
// atomically, never landing half-written.
func (s *Store) PatchLivePolicies(ctx context.Context, orgID uuid.UUID, actor uuid.UUID, p PolicyPatch) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	cur, err := currentLivePolicyTx(ctx, tx, orgID)
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
	if err := upsertPolicyRow(ctx, tx, orgID, cur); err != nil {
		return err
	}
	if err := validateZoneWorld(ctx, tx, orgID); err != nil {
		return err
	}
	if err := auditRow(ctx, tx, orgID, actor, "edit", "policies"); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func currentLivePolicyTx(ctx context.Context, tx dbtx, orgID uuid.UUID) (DraftPolicy, error) {
	var p DraftPolicy
	var deliveryInDays, returnPeriodInDays *string
	err := tx.QueryRow(ctx, `SELECT delivery_cost, delivery_in_days, free_delivery_from, min_order,
		prepayment, installment, return_period_in_days, warranty, outside_zones_note, commerce_policy_documents
		FROM ai_policies WHERE organization_id = $1`, orgID).
		Scan(&p.DeliveryCost, &deliveryInDays, &p.FreeDeliveryFrom, &p.MinOrder,
			&p.Prepayment, &p.Installment, &returnPeriodInDays, &p.Warranty, &p.OutsideZonesNote,
			(*dbx.UUIDArray)(&p.CommercePolicyDocuments))
	if errors.Is(err, dbx.ErrNoRows) {
		return DraftPolicy{}, nil
	}
	p.DeliveryInDays = strOrEmpty(deliveryInDays)
	p.ReturnPeriodInDays = strOrEmpty(returnPeriodInDays)
	return p, err
}

// PatchLiveConfig edits the org's live assistant config (only non-nil fields).
// Uses upsertConfigRow (not a bare UPDATE) so a fresh org with no ai_assistants
// row yet gets one created instead of the patch silently hitting zero rows.
func (s *Store) PatchLiveConfig(ctx context.Context, orgID uuid.UUID, actor uuid.UUID, p ConfigPatch) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := upsertConfigRow(ctx, tx, orgID, p); err != nil {
		return err
	}
	if err := auditRow(ctx, tx, orgID, actor, "edit", "config"); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
