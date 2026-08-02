// Package responsestore implements backend/response's repository interfaces
// over the canonical ai_*/kbd_materials columns (migration 0011 onward — see
// plan/DECISIONS.md's "Canonical knowledge-base schema"). It is the ONLY
// place SQL row shapes get translated into aiprompt/response types —
// backend/response itself never imports pgx or the schema.
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
// existing ai_* tables, mapped directly into aiprompt.KB. internal/kbstore
// (the Playground/live-editor path) is a separate consumer of the same
// tables — untouched by this package.
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
	if kb.Materials, err = loadMaterials(ctx, tx, orgID); err != nil {
		return nil, err
	}

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

// mediaArray reads a uuid[] media column into a []string of material ids —
// pgx scans a Postgres uuid[] into []uuid.UUID, so every plural media column
// is scanned that way and converted here.
func mediaArray(ids []uuid.UUID) []string {
	if len(ids) == 0 {
		return nil
	}
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = id.String()
	}
	return out
}

func loadTopics(ctx context.Context, tx pgx.Tx, orgID uuid.UUID) ([]aiprompt.Topic, error) {
	rows, err := tx.Query(ctx, `
		SELECT slug, title, body_md, featured_image, illustration_images, explainer_videos, reference_documents
		FROM xchats.ai_topics WHERE organization_id = $1 ORDER BY created_at`, orgID)
	if err != nil {
		return nil, fmt.Errorf("responsestore: load topics: %w", err)
	}
	defer rows.Close()
	var out []aiprompt.Topic
	for rows.Next() {
		var t aiprompt.Topic
		var featuredImage uuid.NullUUID
		var illustration, explainer, reference []uuid.UUID
		if err := rows.Scan(&t.Slug, &t.Title, &t.BodyMD, &featuredImage,
			&illustration, &explainer, &reference); err != nil {
			return nil, fmt.Errorf("responsestore: scan topic: %w", err)
		}
		if featuredImage.Valid {
			t.FeaturedImage = featuredImage.UUID.String()
		}
		t.IllustrationImages = mediaArray(illustration)
		t.ExplainerVideos = mediaArray(explainer)
		t.ReferenceDocuments = mediaArray(reference)
		out = append(out, t)
	}
	return out, rows.Err()
}

func loadProducts(ctx context.Context, tx pgx.Tx, orgID uuid.UUID) ([]aiprompt.Product, error) {
	rows, err := tx.Query(ctx, `
		SELECT ref, name, price, description, category, sales_status, in_stock,
		       featured_image, gallery_images, demo_videos, certificate_documents, guarantee_documents
		FROM xchats.ai_products WHERE organization_id = $1 ORDER BY created_at`, orgID)
	if err != nil {
		return nil, fmt.Errorf("responsestore: load products: %w", err)
	}
	defer rows.Close()
	var out []aiprompt.Product
	for rows.Next() {
		var p aiprompt.Product
		var featuredImage uuid.NullUUID
		var gallery, demo, cert, guarantee []uuid.UUID
		if err := rows.Scan(&p.Ref, &p.Name, &p.Price, &p.Description, &p.Category, &p.SalesStatus, &p.InStock,
			&featuredImage, &gallery, &demo, &cert, &guarantee); err != nil {
			return nil, fmt.Errorf("responsestore: scan product: %w", err)
		}
		if featuredImage.Valid {
			p.FeaturedImage = featuredImage.UUID.String()
		}
		p.GalleryImages = mediaArray(gallery)
		p.DemoVideos = mediaArray(demo)
		p.CertificateDocuments = mediaArray(cert)
		p.GuaranteeDocuments = mediaArray(guarantee)
		out = append(out, p)
	}
	return out, rows.Err()
}

func loadTariffs(ctx context.Context, tx pgx.Tx, orgID uuid.UUID) ([]aiprompt.Tariff, error) {
	rows, err := tx.Query(ctx, `
		SELECT ref, name, price, limit_text, fee, summary, pricing_type, advantages, disadvantages, sales_status,
		       featured_image, pricing_images, explainer_videos, terms_documents
		FROM xchats.ai_tariffs WHERE organization_id = $1 ORDER BY created_at`, orgID)
	if err != nil {
		return nil, fmt.Errorf("responsestore: load tariffs: %w", err)
	}
	defer rows.Close()
	var out []aiprompt.Tariff
	for rows.Next() {
		var t aiprompt.Tariff
		var featuredImage uuid.NullUUID
		var pricing, explainer, terms []uuid.UUID
		if err := rows.Scan(&t.Ref, &t.Name, &t.Price, &t.LimitText, &t.Fee, &t.Summary,
			&t.PricingType, &t.Advantages, &t.Disadvantages, &t.SalesStatus,
			&featuredImage, &pricing, &explainer, &terms); err != nil {
			return nil, fmt.Errorf("responsestore: scan tariff: %w", err)
		}
		if featuredImage.Valid {
			t.FeaturedImage = featuredImage.UUID.String()
		}
		t.PricingImages = mediaArray(pricing)
		t.ExplainerVideos = mediaArray(explainer)
		t.TermsDocuments = mediaArray(terms)
		out = append(out, t)
	}
	return out, rows.Err()
}

func loadContacts(ctx context.Context, tx pgx.Tx, orgID uuid.UUID) (*aiprompt.Contacts, error) {
	var c aiprompt.Contacts
	var legalInfo *string
	var contactCard, locationMap uuid.NullUUID
	var companyLegal []uuid.UUID
	err := tx.QueryRow(ctx, `
		SELECT whatsapp, email, address, legal_information, callback_time, working_hours, phone, website, instagram,
		       contact_card_image, location_map_image, company_legal_documents
		FROM xchats.ai_contacts WHERE organization_id = $1`, orgID).
		Scan(&c.WhatsApp, &c.Email, &c.Address, &legalInfo, &c.CallbackTime,
			&c.WorkingHours, &c.Phone, &c.Website, &c.Instagram,
			&contactCard, &locationMap, &companyLegal)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("responsestore: load contacts: %w", err)
	}
	c.LegalInformation = strOrEmpty(legalInfo)
	if contactCard.Valid {
		c.ContactCardImage = contactCard.UUID.String()
	}
	if locationMap.Valid {
		c.LocationMapImage = locationMap.UUID.String()
	}
	c.CompanyLegalDocuments = mediaArray(companyLegal)
	return &c, nil
}

func loadPolicies(ctx context.Context, tx pgx.Tx, orgID uuid.UUID) (*aiprompt.Policies, error) {
	var p aiprompt.Policies
	var deliveryInDays, returnPeriodInDays *string
	var commerceDocs []uuid.UUID
	err := tx.QueryRow(ctx, `
		SELECT delivery_cost, delivery_in_days, free_delivery_from, min_order,
		       prepayment, installment, return_period_in_days, warranty, outside_zones_note,
		       commerce_policy_documents
		FROM xchats.ai_policies WHERE organization_id = $1`, orgID).
		Scan(&p.DeliveryCost, &deliveryInDays, &p.FreeDeliveryFrom, &p.MinOrder,
			&p.Prepayment, &p.Installment, &returnPeriodInDays, &p.Warranty, &p.OutsideZonesNote,
			&commerceDocs)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("responsestore: load policies: %w", err)
	}
	p.DeliveryInDays = strOrEmpty(deliveryInDays)
	p.ReturnPeriodInDays = strOrEmpty(returnPeriodInDays)
	p.CommercePolicyDocuments = mediaArray(commerceDocs)
	return &p, nil
}

func loadDeliveryZones(ctx context.Context, tx pgx.Tx, orgID uuid.UUID) ([]aiprompt.DeliveryZone, error) {
	rows, err := tx.Query(ctx, `
		SELECT ref, name, zone_level, parent_ref, delivery_available, delivery_cost, delivery_in_days, notes, sales_status
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

func loadMaterials(ctx context.Context, tx pgx.Tx, orgID uuid.UUID) ([]aiprompt.Material, error) {
	rows, err := tx.Query(ctx, `
		SELECT id, source_type, source_ref, filename, mime_type, size_bytes,
		       storage_backend, storage_key, processing_status, customer_visibility
		FROM xchats.kbd_materials WHERE organization_id = $1 ORDER BY created_at`, orgID)
	if err != nil {
		return nil, fmt.Errorf("responsestore: load materials: %w", err)
	}
	defer rows.Close()
	var out []aiprompt.Material
	for rows.Next() {
		var m aiprompt.Material
		var id uuid.UUID
		var sourceRef, filename, mimeType, storageBackend, storageKey, customerVisibility *string
		var sizeBytes *int64
		if err := rows.Scan(&id, &m.SourceType, &sourceRef, &filename, &mimeType, &sizeBytes,
			&storageBackend, &storageKey, &m.ProcessingStatus, &customerVisibility); err != nil {
			return nil, fmt.Errorf("responsestore: scan material: %w", err)
		}
		m.ID = id.String()
		m.OrganizationID = orgID.String()
		m.SourceRef = strOrEmpty(sourceRef)
		m.Filename = strOrEmpty(filename)
		m.MimeType = strOrEmpty(mimeType)
		m.StorageBackend = strOrEmpty(storageBackend)
		m.StorageKey = strOrEmpty(storageKey)
		m.CustomerVisibility = strOrEmpty(customerVisibility)
		if sizeBytes != nil {
			m.SizeBytes = *sizeBytes
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func strOrEmpty(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
