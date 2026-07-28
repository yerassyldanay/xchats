// Package responsestore implements backend/response's repository interfaces
// over the existing PostgreSQL tables (no renames). It is the ONLY place SQL
// row shapes get translated into aiprompt/response types — backend/response
// itself never imports pgx or the schema.
package responsestore

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/yerassyldanay/xchats/backend/aiprompt"
)

// ErrKBNotConfigured means the organization has no ai_assistants row yet — a
// distinct, expected condition (not a database error) so the caller can
// degrade to a holding draft instead of rendering an empty or broken prompt.
var ErrKBNotConfigured = errors.New("responsestore: knowledge base not configured for this organization")

// KnowledgeBaseRepo implements response.KnowledgeBaseRepository over the
// existing ai_* tables, mapped directly into aiprompt.KB. It is independent of
// internal/kbstore's older domain.Snapshot read path (different column
// semantics: in_stock vs. availability, per-zone delivery, outside_zones_note)
// — kbstore's playground/live-editor path is untouched by this package.
type KnowledgeBaseRepo struct {
	Pool *pgxpool.Pool
}

// Load reads a read-only-transaction-consistent snapshot of organizationID's
// approved knowledge base.
func (r *KnowledgeBaseRepo) Load(ctx context.Context, organizationID string) (*aiprompt.KB, error) {
	orgID, err := uuid.Parse(organizationID)
	if err != nil {
		return nil, fmt.Errorf("responsestore: invalid organization id %q: %w", organizationID, err)
	}

	tx, err := r.Pool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, fmt.Errorf("responsestore: begin read-only tx: %w", err)
	}
	defer tx.Rollback(ctx)

	assistant, err := loadAssistant(ctx, tx, orgID)
	if err != nil {
		return nil, err
	}
	if assistant == nil {
		return nil, ErrKBNotConfigured
	}

	kb := &aiprompt.KB{OrganizationID: organizationID, Assistant: assistant}
	if kb.Topics, err = loadTopics(ctx, tx, orgID); err != nil {
		return nil, err
	}
	if kb.Products, err = loadProducts(ctx, tx, orgID); err != nil {
		return nil, err
	}
	if kb.Tariffs, err = loadTariffs(ctx, tx, orgID); err != nil {
		return nil, err
	}
	if kb.Contacts, err = loadContacts(ctx, tx, orgID); err != nil {
		return nil, err
	}
	if kb.Policies, err = loadPolicies(ctx, tx, orgID); err != nil {
		return nil, err
	}
	if kb.DeliveryZones, err = loadDeliveryZones(ctx, tx, orgID); err != nil {
		return nil, err
	}
	// Materials intentionally stays nil: no typed media arrays or
	// material-storage columns exist yet (the media milestone adds them);
	// empty Materials is valid aiprompt input.

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("responsestore: commit read-only tx: %w", err)
	}
	return kb, nil
}

func loadAssistant(ctx context.Context, tx pgx.Tx, orgID uuid.UUID) (*aiprompt.Assistant, error) {
	var a aiprompt.Assistant
	err := tx.QueryRow(ctx, `
		SELECT persona, mission, guardrails, language_policy, reply_max_words
		FROM xchats.ai_assistants WHERE organization_id = $1`, orgID).
		Scan(&a.Persona, &a.Mission, &a.Guardrails, &a.LanguagePolicy, &a.ReplyMaxWords)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("responsestore: load assistant: %w", err)
	}
	return &a, nil
}

func loadTopics(ctx context.Context, tx pgx.Tx, orgID uuid.UUID) ([]aiprompt.Topic, error) {
	rows, err := tx.Query(ctx, `
		SELECT slug, title, body_md
		FROM xchats.ai_topics WHERE organization_id = $1 AND lang = 'ru' ORDER BY created_at`, orgID)
	if err != nil {
		return nil, fmt.Errorf("responsestore: load topics: %w", err)
	}
	defer rows.Close()
	var out []aiprompt.Topic
	for rows.Next() {
		var t aiprompt.Topic
		if err := rows.Scan(&t.Slug, &t.Title, &t.BodyMD); err != nil {
			return nil, fmt.Errorf("responsestore: scan topic: %w", err)
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func loadProducts(ctx context.Context, tx pgx.Tx, orgID uuid.UUID) ([]aiprompt.Product, error) {
	rows, err := tx.Query(ctx, `
		SELECT ref, name, price, description, category, status, in_stock
		FROM xchats.ai_products WHERE organization_id = $1 AND lang = 'ru' ORDER BY created_at`, orgID)
	if err != nil {
		return nil, fmt.Errorf("responsestore: load products: %w", err)
	}
	defer rows.Close()
	var out []aiprompt.Product
	for rows.Next() {
		var p aiprompt.Product
		if err := rows.Scan(&p.Ref, &p.Name, &p.Price, &p.Description, &p.Category, &p.SalesStatus, &p.InStock); err != nil {
			return nil, fmt.Errorf("responsestore: scan product: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func loadTariffs(ctx context.Context, tx pgx.Tx, orgID uuid.UUID) ([]aiprompt.Tariff, error) {
	rows, err := tx.Query(ctx, `
		SELECT ref, name, price, limit_text, fee, summary, pricing_type, advantages, disadvantages, status
		FROM xchats.ai_tariffs WHERE organization_id = $1 AND lang = 'ru' ORDER BY created_at`, orgID)
	if err != nil {
		return nil, fmt.Errorf("responsestore: load tariffs: %w", err)
	}
	defer rows.Close()
	var out []aiprompt.Tariff
	for rows.Next() {
		var t aiprompt.Tariff
		if err := rows.Scan(&t.Ref, &t.Name, &t.Price, &t.LimitText, &t.Fee, &t.Summary,
			&t.PricingType, &t.Advantages, &t.Disadvantages, &t.SalesStatus); err != nil {
			return nil, fmt.Errorf("responsestore: scan tariff: %w", err)
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func loadContacts(ctx context.Context, tx pgx.Tx, orgID uuid.UUID) (*aiprompt.Contacts, error) {
	var c aiprompt.Contacts
	err := tx.QueryRow(ctx, `
		SELECT whatsapp, email, address, legal, callback_time, working_hours, phone, website, instagram
		FROM xchats.ai_contacts WHERE organization_id = $1 AND lang = '*'`, orgID).
		Scan(&c.WhatsApp, &c.Email, &c.Address, &c.LegalInformation, &c.CallbackTime,
			&c.WorkingHours, &c.Phone, &c.Website, &c.Instagram)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("responsestore: load contacts: %w", err)
	}
	return &c, nil
}

func loadPolicies(ctx context.Context, tx pgx.Tx, orgID uuid.UUID) (*aiprompt.Policies, error) {
	var p aiprompt.Policies
	err := tx.QueryRow(ctx, `
		SELECT delivery_cost, delivery_time, free_delivery_from, min_order,
		       prepayment, installment, return_period, warranty, outside_zones_note
		FROM xchats.ai_policies WHERE organization_id = $1 AND lang = '*'`, orgID).
		Scan(&p.DeliveryCost, &p.DeliveryInDays, &p.FreeDeliveryFrom, &p.MinOrder,
			&p.Prepayment, &p.Installment, &p.ReturnPeriodInDays, &p.Warranty, &p.OutsideZonesNote)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("responsestore: load policies: %w", err)
	}
	return &p, nil
}

func loadDeliveryZones(ctx context.Context, tx pgx.Tx, orgID uuid.UUID) ([]aiprompt.DeliveryZone, error) {
	rows, err := tx.Query(ctx, `
		SELECT ref, name, zone_level, parent_ref, delivery_available, delivery_cost, delivery_in_days, notes, status
		FROM xchats.ai_delivery_zones WHERE organization_id = $1 ORDER BY created_at`, orgID)
	if err != nil {
		return nil, fmt.Errorf("responsestore: load delivery zones: %w", err)
	}
	defer rows.Close()
	var out []aiprompt.DeliveryZone
	for rows.Next() {
		var z aiprompt.DeliveryZone
		if err := rows.Scan(&z.Ref, &z.Name, &z.ZoneLevel, &z.ParentRef, &z.DeliveryAvailable,
			&z.DeliveryCost, &z.DeliveryInDays, &z.Notes, &z.SalesStatus); err != nil {
			return nil, fmt.Errorf("responsestore: scan delivery zone: %w", err)
		}
		out = append(out, z)
	}
	return out, rows.Err()
}
