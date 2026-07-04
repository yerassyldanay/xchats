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
// ---------------------------------------------------------------------------

// PutLiveTopic upserts a topic directly into the live table. Rejects a body
// that is not pure prose (a fact token or a literal currency amount) — the
// exact check the approve gate runs, just enforced at write time here.
func (s *Store) PutLiveTopic(ctx context.Context, orgID uuid.UUID, in TopicInput) error {
	if reasons := gateTopicBody(in.Slug, in.BodyMD); len(reasons) > 0 {
		return &GateError{Reasons: reasons}
	}
	_, err := s.pool.Exec(ctx, `INSERT INTO xchats.ai_topics
		(organization_id, slug, lang, title, keywords, body_md)
		VALUES ($1,$2,$3,$4,$5,$6)
		ON CONFLICT (organization_id, slug) DO UPDATE SET
			lang=EXCLUDED.lang, title=EXCLUDED.title, keywords=EXCLUDED.keywords,
			body_md=EXCLUDED.body_md, updated_at=now()`,
		orgID, in.Slug, orDefault(in.Lang, "ru"), in.Title, in.Keywords, in.BodyMD)
	return err
}

// DeleteLiveTopic removes a live topic by slug.
func (s *Store) DeleteLiveTopic(ctx context.Context, orgID uuid.UUID, slug string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM xchats.ai_topics WHERE organization_id=$1 AND slug=$2`, orgID, slug)
	return err
}

// PutLiveAsset upserts an asset directly into the live table. Rejects an empty
// description — an undescribed live asset would fail every future Playground
// approve anyway (gate.go), so this refuses it up front instead of leaving a
// landmine in the live KB.
func (s *Store) PutLiveAsset(ctx context.Context, orgID uuid.UUID, in AssetInput) error {
	if reasons := gateAsset(domain.Asset{Ref: in.Ref, Description: in.Description}, nil); len(reasons) > 0 {
		return &GateError{Reasons: reasons}
	}
	_, err := s.pool.Exec(ctx, `INSERT INTO xchats.ai_assets
		(organization_id, ref, asset_kind, owner_kind, owner_ref, title, description, asset_url, lang)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		ON CONFLICT (organization_id, ref) DO UPDATE SET
			asset_kind=EXCLUDED.asset_kind, owner_kind=EXCLUDED.owner_kind, owner_ref=EXCLUDED.owner_ref,
			title=EXCLUDED.title, description=EXCLUDED.description, asset_url=EXCLUDED.asset_url,
			lang=EXCLUDED.lang, updated_at=now()`,
		orgID, in.Ref, orDefault(in.Kind, "image"), in.OwnerKind, in.OwnerRef, in.Title, in.Description, in.URL, orDefault(in.Lang, "ru"))
	return err
}

// PatchLiveAsset edits an existing live asset's description and/or owner,
// starting from its current row so an omitted field stays unchanged.
func (s *Store) PatchLiveAsset(ctx context.Context, orgID uuid.UUID, ref string, p AssetPatch) error {
	cur, err := s.currentLiveAsset(ctx, orgID, ref)
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
	return s.PutLiveAsset(ctx, orgID, AssetInput{
		Ref: cur.Ref, Kind: cur.Kind, OwnerKind: cur.OwnerKind, OwnerRef: cur.OwnerRef,
		Title: cur.Title, Description: cur.Description, URL: cur.URL, Lang: cur.Lang,
	})
}

func (s *Store) currentLiveAsset(ctx context.Context, orgID uuid.UUID, ref string) (DraftAsset, error) {
	var a DraftAsset
	err := s.pool.QueryRow(ctx, `SELECT ref, asset_kind, owner_kind, owner_ref, title, description, asset_url, lang
		FROM xchats.ai_assets WHERE organization_id = $1 AND ref = $2`, orgID, ref).
		Scan(&a.Ref, &a.Kind, &a.OwnerKind, &a.OwnerRef, &a.Title, &a.Description, &a.URL, &a.Lang)
	if errors.Is(err, pgx.ErrNoRows) {
		return DraftAsset{}, ErrUnknownKind
	}
	return a, err
}

// DeleteLiveAsset removes a live asset by ref.
func (s *Store) DeleteLiveAsset(ctx context.Context, orgID uuid.UUID, ref string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM xchats.ai_assets WHERE organization_id=$1 AND ref=$2`, orgID, ref)
	return err
}

// PutLiveTariff upserts a tariff row directly into the live table (no content
// gate — typed fact columns are validated at reply-render time, fail closed).
func (s *Store) PutLiveTariff(ctx context.Context, orgID uuid.UUID, in TariffInput) error {
	return upsertTariffRow(ctx, s.pool, orgID, domain.Tariff{
		Ref: in.Ref, Lang: in.Lang, Name: in.Name, Price: in.Price, LimitText: in.LimitText, Fee: in.Fee,
		Summary: in.Summary, PricingType: in.PricingType, Advantages: in.Advantages, Disadvantages: in.Disadvantages,
	})
}

// DeleteLiveTariff removes a live tariff row by ref.
func (s *Store) DeleteLiveTariff(ctx context.Context, orgID uuid.UUID, ref string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM xchats.ai_tariffs WHERE organization_id=$1 AND ref=$2`, orgID, ref)
	return err
}

// PutLiveProduct upserts a product row directly into the live table.
func (s *Store) PutLiveProduct(ctx context.Context, orgID uuid.UUID, in ProductInput) error {
	return upsertProductRow(ctx, s.pool, orgID, domain.Product{
		Ref: in.Ref, Lang: in.Lang, Name: in.Name, Price: in.Price, Description: in.Description, Category: in.Category,
	})
}

// DeleteLiveProduct removes a live product row by ref.
func (s *Store) DeleteLiveProduct(ctx context.Context, orgID uuid.UUID, ref string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM xchats.ai_products WHERE organization_id=$1 AND ref=$2`, orgID, ref)
	return err
}

// PatchLiveContacts edits the org's live support-contact row for a language,
// starting from its current shape so an omitted field stays unchanged.
func (s *Store) PatchLiveContacts(ctx context.Context, orgID uuid.UUID, p ContactPatch) error {
	lang := orDefault(p.Lang, "*")
	cur, err := s.currentLiveContact(ctx, orgID, lang)
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
	return upsertContactRow(ctx, s.pool, orgID, domain.Contact{
		Lang: cur.Lang, WhatsApp: cur.WhatsApp, Email: cur.Email, Address: cur.Address,
		Legal: cur.Legal, CallbackTime: cur.CallbackTime,
	})
}

func (s *Store) currentLiveContact(ctx context.Context, orgID uuid.UUID, lang string) (DraftContact, error) {
	var c DraftContact
	err := s.pool.QueryRow(ctx, `SELECT lang, whatsapp, email, address, legal, callback_time
		FROM xchats.ai_contacts WHERE organization_id = $1 AND lang = $2`, orgID, lang).
		Scan(&c.Lang, &c.WhatsApp, &c.Email, &c.Address, &c.Legal, &c.CallbackTime)
	if errors.Is(err, pgx.ErrNoRows) {
		return DraftContact{Lang: lang}, nil
	}
	return c, err
}

// PatchLiveConfig edits the org's live assistant config (only non-nil fields).
func (s *Store) PatchLiveConfig(ctx context.Context, orgID uuid.UUID, p ConfigPatch) error {
	_, err := s.pool.Exec(ctx, `UPDATE xchats.ai_assistants SET
		persona = COALESCE($2, persona), mission = COALESCE($3, mission),
		guardrails = COALESCE($4, guardrails), language_policy = COALESCE($5, language_policy),
		reply_max_words = COALESCE($6, reply_max_words), updated_at = now()
		WHERE organization_id = $1`,
		orgID, p.Persona, p.Mission, p.Guardrails, p.LanguagePolicy, p.ReplyMaxWords)
	return err
}
