package kbstore

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/yerassyldanay/xchats/backend/internal/brain/domain"
)

// ---------------------------------------------------------------------------
// Live writes — the /kb/* editor surface. Every write here lands directly in
// the live ai_ tables: no draft blob, no approve step. This is the ONLY write
// path LiveView/the /kb/* HTTP handlers use, so a live edit can never leak into
// — or be confused with — a Playground draft (see plan "Playground redesign").
// Content is held to the same bar the approve gate enforces (gateTopicBody /
// gateAsset, kbstore.go), just checked immediately instead of at approve time.
//
// Every function here takes the acting user's id (actor) and runs its write
// plus an ai_audit_log row inside ONE transaction (pool.Begin), so the audit
// trail can never desync from what was actually written — either both commit
// or neither does. A read-modify-write function (PatchLiveAsset/Contacts/
// Policies) additionally takes its "current row" read with SELECT ... FOR
// UPDATE inside that same transaction, closing a lost-update race an earlier
// version of this file had (read + compute on the pool, upsert as a separate
// statement — a concurrent writer's change in between was silently clobbered).
// ---------------------------------------------------------------------------

// PutLiveTopic upserts a topic directly into the live table. Rejects a body
// that is not pure prose (a fact token or a literal currency amount) — the
// exact check the approve gate runs, just enforced at write time here.
func (s *Store) PutLiveTopic(ctx context.Context, orgID uuid.UUID, actor uuid.UUID, in TopicInput) error {
	if reasons := gateTopicBody(in.Slug, in.BodyMD); len(reasons) > 0 {
		return &GateError{Reasons: reasons}
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `INSERT INTO xchats.ai_topics
		(organization_id, slug, lang, title, keywords, body_md)
		VALUES ($1,$2,$3,$4,$5,$6)
		ON CONFLICT (organization_id, slug) DO UPDATE SET
			lang=EXCLUDED.lang, title=EXCLUDED.title, keywords=EXCLUDED.keywords,
			body_md=EXCLUDED.body_md, updated_at=now()`,
		orgID, in.Slug, orDefault(in.Lang, "ru"), in.Title, in.Keywords, in.BodyMD); err != nil {
		return err
	}
	if err := auditRow(ctx, tx, orgID, actor, "edit", "topic:"+in.Slug); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// DeleteLiveTopic removes a live topic by slug.
func (s *Store) DeleteLiveTopic(ctx context.Context, orgID uuid.UUID, actor uuid.UUID, slug string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `DELETE FROM xchats.ai_topics WHERE organization_id=$1 AND slug=$2`, orgID, slug); err != nil {
		return err
	}
	if err := auditRow(ctx, tx, orgID, actor, "delete", "topic:"+slug); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// putLiveAssetRow writes one ai_assets row — shared by PutLiveAsset (its own
// transaction) and PatchLiveAsset (the transaction its FOR UPDATE read opened).
func putLiveAssetRow(ctx context.Context, db dbtx, orgID uuid.UUID, in AssetInput) error {
	_, err := db.Exec(ctx, `INSERT INTO xchats.ai_assets
		(organization_id, ref, asset_kind, owner_kind, owner_ref, title, description, asset_url, lang)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		ON CONFLICT (organization_id, ref) DO UPDATE SET
			asset_kind=EXCLUDED.asset_kind, owner_kind=EXCLUDED.owner_kind, owner_ref=EXCLUDED.owner_ref,
			title=EXCLUDED.title, description=EXCLUDED.description, asset_url=EXCLUDED.asset_url,
			lang=EXCLUDED.lang, updated_at=now()`,
		orgID, in.Ref, orDefault(in.Kind, "image"), in.OwnerKind, in.OwnerRef, in.Title, in.Description, in.URL, orDefault(in.Lang, "ru"))
	return err
}

// PutLiveAsset upserts an asset directly into the live table. Rejects an empty
// description — an undescribed live asset would fail every future Playground
// approve anyway (gate.go), so this refuses it up front instead of leaving a
// landmine in the live KB.
func (s *Store) PutLiveAsset(ctx context.Context, orgID uuid.UUID, actor uuid.UUID, in AssetInput) error {
	if reasons := gateAsset(domain.Asset{Ref: in.Ref, Description: in.Description}, nil); len(reasons) > 0 {
		return &GateError{Reasons: reasons}
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := putLiveAssetRow(ctx, tx, orgID, in); err != nil {
		return err
	}
	if err := auditRow(ctx, tx, orgID, actor, "edit", "asset:"+in.Ref); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// PatchLiveAsset edits an existing live asset's description and/or owner,
// starting from its current row (locked FOR UPDATE for the duration of this
// transaction, so a concurrent patch of the same asset serializes instead of
// one silently clobbering the other) so an omitted field stays unchanged.
func (s *Store) PatchLiveAsset(ctx context.Context, orgID uuid.UUID, actor uuid.UUID, ref string, p AssetPatch) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	cur, err := currentLiveAssetTx(ctx, tx, orgID, ref)
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
	in := AssetInput{Ref: cur.Ref, Kind: cur.Kind, OwnerKind: cur.OwnerKind, OwnerRef: cur.OwnerRef,
		Title: cur.Title, Description: cur.Description, URL: cur.URL, Lang: cur.Lang}
	if reasons := gateAsset(domain.Asset{Ref: in.Ref, Description: in.Description}, nil); len(reasons) > 0 {
		return &GateError{Reasons: reasons}
	}
	if err := putLiveAssetRow(ctx, tx, orgID, in); err != nil {
		return err
	}
	if err := auditRow(ctx, tx, orgID, actor, "edit", "asset:"+ref); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func currentLiveAssetTx(ctx context.Context, tx pgx.Tx, orgID uuid.UUID, ref string) (DraftAsset, error) {
	var a DraftAsset
	err := tx.QueryRow(ctx, `SELECT ref, asset_kind, owner_kind, owner_ref, title, description, asset_url, lang
		FROM xchats.ai_assets WHERE organization_id = $1 AND ref = $2 FOR UPDATE`, orgID, ref).
		Scan(&a.Ref, &a.Kind, &a.OwnerKind, &a.OwnerRef, &a.Title, &a.Description, &a.URL, &a.Lang)
	if errors.Is(err, pgx.ErrNoRows) {
		return DraftAsset{}, ErrUnknownKind
	}
	return a, err
}

// DeleteLiveAsset removes a live asset by ref.
func (s *Store) DeleteLiveAsset(ctx context.Context, orgID uuid.UUID, actor uuid.UUID, ref string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `DELETE FROM xchats.ai_assets WHERE organization_id=$1 AND ref=$2`, orgID, ref); err != nil {
		return err
	}
	if err := auditRow(ctx, tx, orgID, actor, "delete", "asset:"+ref); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// PutLiveTariff upserts a tariff row directly into the live table (no content
// gate — typed fact columns are validated at reply-render time, fail closed).
func (s *Store) PutLiveTariff(ctx context.Context, orgID uuid.UUID, actor uuid.UUID, in TariffInput) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := upsertTariffRow(ctx, tx, orgID, domain.Tariff{
		Ref: in.Ref, Lang: in.Lang, Name: in.Name, Price: in.Price, LimitText: in.LimitText, Fee: in.Fee,
		Summary: in.Summary, PricingType: in.PricingType, Advantages: in.Advantages, Disadvantages: in.Disadvantages,
	}); err != nil {
		return err
	}
	if err := auditRow(ctx, tx, orgID, actor, "edit", "tariff:"+in.Ref); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// DeleteLiveTariff removes a live tariff row by ref.
func (s *Store) DeleteLiveTariff(ctx context.Context, orgID uuid.UUID, actor uuid.UUID, ref string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `DELETE FROM xchats.ai_tariffs WHERE organization_id=$1 AND ref=$2`, orgID, ref); err != nil {
		return err
	}
	if err := auditRow(ctx, tx, orgID, actor, "delete", "tariff:"+ref); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// PutLiveProduct upserts a product row directly into the live table.
func (s *Store) PutLiveProduct(ctx context.Context, orgID uuid.UUID, actor uuid.UUID, in ProductInput) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := upsertProductRow(ctx, tx, orgID, domain.Product{
		Ref: in.Ref, Lang: in.Lang, Name: in.Name, Price: in.Price, Description: in.Description,
		Category: in.Category, Availability: in.Availability,
	}, in.InStock); err != nil {
		return err
	}
	if err := auditRow(ctx, tx, orgID, actor, "edit", "product:"+in.Ref); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// DeleteLiveProduct removes a live product row by ref.
func (s *Store) DeleteLiveProduct(ctx context.Context, orgID uuid.UUID, actor uuid.UUID, ref string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `DELETE FROM xchats.ai_products WHERE organization_id=$1 AND ref=$2`, orgID, ref); err != nil {
		return err
	}
	if err := auditRow(ctx, tx, orgID, actor, "delete", "product:"+ref); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// PatchLiveContacts edits the org's live support-contact row for a language,
// starting from its current row (locked FOR UPDATE for this transaction) so
// an omitted field stays unchanged and a concurrent patch serializes instead
// of racing.
func (s *Store) PatchLiveContacts(ctx context.Context, orgID uuid.UUID, actor uuid.UUID, p ContactPatch) error {
	lang := orDefault(p.Lang, "*")
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	cur, err := currentLiveContactTx(ctx, tx, orgID, lang)
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
	if err := upsertContactRow(ctx, tx, orgID, domain.Contact{
		Lang: cur.Lang, WhatsApp: cur.WhatsApp, Email: cur.Email, Address: cur.Address,
		Legal: cur.Legal, CallbackTime: cur.CallbackTime,
		WorkingHours: cur.WorkingHours, Phone: cur.Phone, Website: cur.Website, Instagram: cur.Instagram,
	}); err != nil {
		return err
	}
	if err := auditRow(ctx, tx, orgID, actor, "edit", "contacts:"+lang); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func currentLiveContactTx(ctx context.Context, tx pgx.Tx, orgID uuid.UUID, lang string) (DraftContact, error) {
	var c DraftContact
	err := tx.QueryRow(ctx, `SELECT lang, whatsapp, email, address, legal, callback_time,
		working_hours, phone, website, instagram
		FROM xchats.ai_contacts WHERE organization_id = $1 AND lang = $2 FOR UPDATE`, orgID, lang).
		Scan(&c.Lang, &c.WhatsApp, &c.Email, &c.Address, &c.Legal, &c.CallbackTime,
			&c.WorkingHours, &c.Phone, &c.Website, &c.Instagram)
	if errors.Is(err, pgx.ErrNoRows) {
		return DraftContact{Lang: lang}, nil
	}
	return c, err
}

// PatchLivePolicies edits the org's live commerce-policy row for a language,
// starting from its current row (locked FOR UPDATE for this transaction) so
// an omitted field stays unchanged — a near-clone of PatchLiveContacts, plus
// a validateZoneWorld re-check before commit: a policy edit that leaves the
// zone/policy world inconsistent (e.g. setting a non-blank flat delivery_cost
// while delivery zones exist) is rejected atomically, never landing half-written.
func (s *Store) PatchLivePolicies(ctx context.Context, orgID uuid.UUID, actor uuid.UUID, p PolicyPatch) error {
	lang := orDefault(p.Lang, "*")
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	cur, err := currentLivePolicyTx(ctx, tx, orgID, lang)
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
	if p.OutsideZonesNote != nil {
		cur.OutsideZonesNote = *p.OutsideZonesNote
	}
	if err := upsertPolicyRow(ctx, tx, orgID, domain.Policy{
		Lang: cur.Lang, DeliveryCost: cur.DeliveryCost, DeliveryTime: cur.DeliveryTime,
		FreeDeliveryFrom: cur.FreeDeliveryFrom, MinOrder: cur.MinOrder, Prepayment: cur.Prepayment,
		Installment: cur.Installment, ReturnPeriod: cur.ReturnPeriod, Warranty: cur.Warranty,
	}, cur.OutsideZonesNote); err != nil {
		return err
	}
	if err := validateZoneWorld(ctx, tx, orgID); err != nil {
		return err
	}
	if err := auditRow(ctx, tx, orgID, actor, "edit", "policies:"+lang); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func currentLivePolicyTx(ctx context.Context, tx pgx.Tx, orgID uuid.UUID, lang string) (DraftPolicy, error) {
	var p DraftPolicy
	err := tx.QueryRow(ctx, `SELECT lang, delivery_cost, delivery_time, free_delivery_from, min_order,
		prepayment, installment, return_period, warranty, outside_zones_note
		FROM xchats.ai_policies WHERE organization_id = $1 AND lang = $2 FOR UPDATE`, orgID, lang).
		Scan(&p.Lang, &p.DeliveryCost, &p.DeliveryTime, &p.FreeDeliveryFrom, &p.MinOrder,
			&p.Prepayment, &p.Installment, &p.ReturnPeriod, &p.Warranty, &p.OutsideZonesNote)
	if errors.Is(err, pgx.ErrNoRows) {
		return DraftPolicy{Lang: lang}, nil
	}
	return p, err
}

// PatchLiveConfig edits the org's live assistant config (only non-nil fields).
// Uses upsertConfigRow (not a bare UPDATE) so a fresh org with no ai_assistants
// row yet gets one created instead of the patch silently hitting zero rows.
func (s *Store) PatchLiveConfig(ctx context.Context, orgID uuid.UUID, actor uuid.UUID, p ConfigPatch) error {
	tx, err := s.pool.Begin(ctx)
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
