// Package kbstore is the writable Knowledge Base data layer: it loads the LIVE
// KB the brain reasons from (ai_assistants/ai_topics/ai_assets/ai_values, each
// keyed DIRECTLY on organization_id — 15 Decision 1), and owns the Playground's
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
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/yerassyldanay/xchats/backend/internal/brain/domain"
)

// ErrStale is returned when an optimistic-concurrency check (If-Match) fails: the
// draft blob moved since the client loaded it.
var ErrStale = errors.New("kbstore: stale draft write")

// ErrUnknownKind is returned for an unrecognized entity kind (topics|assets|values).
var ErrUnknownKind = errors.New("kbstore: unknown row kind")

// Store wraps the pgx pool with KB operations.
type Store struct {
	pool *pgxpool.Pool
}

// New builds a KBStore over an existing pool.
func New(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

// ---------------------------------------------------------------------------
// Live load (the brain's source) + seed
// ---------------------------------------------------------------------------

// LoadLive returns the org's live KB as a brain-ready *domain.Snapshot. Live
// tables hold live rows only, so there is no review/version filter to apply.
func (s *Store) LoadLive(ctx context.Context, orgID uuid.UUID) (*domain.Snapshot, error) {
	snap := &domain.Snapshot{Loaded: time.Now()}
	err := s.pool.QueryRow(ctx, `
		SELECT persona, mission, guardrails, language_policy, reply_max_words
		FROM xchats.ai_assistants WHERE organization_id = $1`, orgID).
		Scan(&snap.Config.Persona, &snap.Config.Mission, &snap.Config.Guardrails,
			&snap.Config.LanguagePolicy, &snap.Config.ReplyMaxWords)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	if err := s.loadLiveContent(ctx, orgID, snap); err != nil {
		return nil, err
	}
	return snap, nil
}

// loadLiveContent fills Topics/Assets/Values from the live ai_ tables for an org.
func (s *Store) loadLiveContent(ctx context.Context, orgID uuid.UUID, snap *domain.Snapshot) error {
	trows, err := s.pool.Query(ctx, `SELECT slug, lang, title, keywords, body_md
		FROM xchats.ai_topics WHERE organization_id = $1 ORDER BY created_at`, orgID)
	if err != nil {
		return err
	}
	for trows.Next() {
		var t domain.Topic
		if err := trows.Scan(&t.Slug, &t.Language, &t.Title, &t.Keywords, &t.BodyMD); err != nil {
			trows.Close()
			return err
		}
		snap.Topics = append(snap.Topics, t)
	}
	trows.Close()
	if err := trows.Err(); err != nil {
		return err
	}

	arows, err := s.pool.Query(ctx, `SELECT ref, asset_kind, owner_kind, owner_ref, title, description, asset_url, lang
		FROM xchats.ai_assets WHERE organization_id = $1 ORDER BY created_at`, orgID)
	if err != nil {
		return err
	}
	for arows.Next() {
		var a domain.Asset
		var ownerKind, ownerRef string
		if err := arows.Scan(&a.Ref, &a.Kind, &ownerKind, &ownerRef, &a.Title, &a.Description, &a.URL, &a.Language); err != nil {
			arows.Close()
			return err
		}
		// domain.Asset.TopicSlug is the only owner the v1 prompt renders; a
		// non-topic owner (product/tariff, added with the typed facts) simply
		// leaves it empty for now.
		if ownerKind == "" || ownerKind == "topic" {
			a.TopicSlug = ownerRef
		}
		snap.Assets = append(snap.Assets, a)
	}
	arows.Close()
	if err := arows.Err(); err != nil {
		return err
	}

	trows2, err := s.pool.Query(ctx, `SELECT ref, lang, name, price, limit_text, fee, summary, pricing_type, advantages, disadvantages
		FROM xchats.ai_tariffs WHERE organization_id = $1 ORDER BY created_at`, orgID)
	if err != nil {
		return err
	}
	for trows2.Next() {
		var t domain.Tariff
		if err := trows2.Scan(&t.Ref, &t.Lang, &t.Name, &t.Price, &t.LimitText, &t.Fee, &t.Summary, &t.PricingType, &t.Advantages, &t.Disadvantages); err != nil {
			trows2.Close()
			return err
		}
		snap.Tariffs = append(snap.Tariffs, t)
	}
	trows2.Close()
	if err := trows2.Err(); err != nil {
		return err
	}

	prows, err := s.pool.Query(ctx, `SELECT ref, lang, name, price, description, category, availability
		FROM xchats.ai_products WHERE organization_id = $1 ORDER BY created_at`, orgID)
	if err != nil {
		return err
	}
	for prows.Next() {
		var p domain.Product
		if err := prows.Scan(&p.Ref, &p.Lang, &p.Name, &p.Price, &p.Description, &p.Category, &p.Availability); err != nil {
			prows.Close()
			return err
		}
		snap.Products = append(snap.Products, p)
	}
	prows.Close()
	if err := prows.Err(); err != nil {
		return err
	}

	crows, err := s.pool.Query(ctx, `SELECT lang, whatsapp, email, address, legal, callback_time,
		working_hours, phone, website, instagram
		FROM xchats.ai_contacts WHERE organization_id = $1 ORDER BY created_at`, orgID)
	if err != nil {
		return err
	}
	for crows.Next() {
		var c domain.Contact
		if err := crows.Scan(&c.Lang, &c.WhatsApp, &c.Email, &c.Address, &c.Legal, &c.CallbackTime,
			&c.WorkingHours, &c.Phone, &c.Website, &c.Instagram); err != nil {
			crows.Close()
			return err
		}
		snap.Contacts = append(snap.Contacts, c)
	}
	crows.Close()
	if err := crows.Err(); err != nil {
		return err
	}

	polrows, err := s.pool.Query(ctx, `SELECT lang, delivery_cost, delivery_time, free_delivery_from, min_order,
		prepayment, installment, return_period, warranty
		FROM xchats.ai_policies WHERE organization_id = $1 ORDER BY created_at`, orgID)
	if err != nil {
		return err
	}
	for polrows.Next() {
		var p domain.Policy
		if err := polrows.Scan(&p.Lang, &p.DeliveryCost, &p.DeliveryTime, &p.FreeDeliveryFrom, &p.MinOrder,
			&p.Prepayment, &p.Installment, &p.ReturnPeriod, &p.Warranty); err != nil {
			polrows.Close()
			return err
		}
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
	if err := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM xchats.ai_topics WHERE organization_id = $1)`,
		orgID).Scan(&exists); err != nil {
		return err
	}
	if exists {
		return nil
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `INSERT INTO xchats.ai_assistants
		(organization_id, persona, mission, guardrails, language_policy, reply_max_words)
		VALUES ($1,$2,$3,$4,$5,$6)
		ON CONFLICT (organization_id) DO UPDATE SET
			persona = EXCLUDED.persona, mission = EXCLUDED.mission, guardrails = EXCLUDED.guardrails,
			language_policy = EXCLUDED.language_policy, reply_max_words = EXCLUDED.reply_max_words, updated_at = now()`,
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
func insertLiveContent(ctx context.Context, tx pgx.Tx, orgID uuid.UUID, snap *domain.Snapshot) error {
	for _, t := range snap.Topics {
		if _, err := tx.Exec(ctx, `INSERT INTO xchats.ai_topics
			(organization_id, slug, lang, title, keywords, body_md)
			VALUES ($1,$2,$3,$4,$5,$6)
			ON CONFLICT (organization_id, slug) DO UPDATE SET
				lang = EXCLUDED.lang, title = EXCLUDED.title, keywords = EXCLUDED.keywords,
				body_md = EXCLUDED.body_md, updated_at = now()`,
			orgID, t.Slug, t.Language, t.Title, t.Keywords, t.BodyMD); err != nil {
			return fmt.Errorf("insert topic %s: %w", t.Slug, err)
		}
	}
	for _, a := range snap.Assets {
		ownerKind, ownerRef := "", ""
		if a.TopicSlug != "" {
			ownerKind, ownerRef = "topic", a.TopicSlug
		}
		if _, err := tx.Exec(ctx, `INSERT INTO xchats.ai_assets
			(organization_id, ref, asset_kind, owner_kind, owner_ref, title, description, asset_url, lang)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
			ON CONFLICT (organization_id, ref) DO UPDATE SET
				asset_kind = EXCLUDED.asset_kind, owner_kind = EXCLUDED.owner_kind, owner_ref = EXCLUDED.owner_ref,
				title = EXCLUDED.title, description = EXCLUDED.description, asset_url = EXCLUDED.asset_url,
				lang = EXCLUDED.lang, updated_at = now()`,
			orgID, a.Ref, a.Kind, ownerKind, ownerRef, a.Title, a.Description, a.URL, a.Language); err != nil {
			return fmt.Errorf("insert asset %s: %w", a.Ref, err)
		}
	}
	for _, t := range snap.Tariffs {
		if err := upsertTariffRow(ctx, tx, orgID, t); err != nil {
			return err
		}
	}
	for _, p := range snap.Products {
		if err := upsertProductRow(ctx, tx, orgID, p); err != nil {
			return err
		}
	}
	for _, c := range snap.Contacts {
		if err := upsertContactRow(ctx, tx, orgID, c); err != nil {
			return err
		}
	}
	for _, p := range snap.Policies {
		if err := upsertPolicyRow(ctx, tx, orgID, p); err != nil {
			return err
		}
	}
	return nil
}

// execer is satisfied by both *pgxpool.Pool and pgx.Tx (same Exec shape), so the
// row-upsert helpers below run identically inside a multi-statement transaction
// (Approve, SeedLiveIfEmpty) or directly on the pool (the /kb/* live-write path,
// where each call is its own single-statement write — no cross-row atomicity to
// preserve).
type execer interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
}

// upsertTariffRow / upsertProductRow / upsertContactRow write one typed fact row
// (verbatim columns). Shared by insertLiveContent (seed), Approve (materialize),
// and the /kb/* live-write path (live.go).
func upsertTariffRow(ctx context.Context, tx execer, orgID uuid.UUID, t domain.Tariff) error {
	if _, err := tx.Exec(ctx, `INSERT INTO xchats.ai_tariffs
		(organization_id, ref, lang, name, price, limit_text, fee, summary, pricing_type, advantages, disadvantages)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		ON CONFLICT (organization_id, ref, lang) DO UPDATE SET
			name=EXCLUDED.name, price=EXCLUDED.price, limit_text=EXCLUDED.limit_text, fee=EXCLUDED.fee,
			summary=EXCLUDED.summary, pricing_type=EXCLUDED.pricing_type, advantages=EXCLUDED.advantages,
			disadvantages=EXCLUDED.disadvantages, updated_at=now()`,
		orgID, t.Ref, orDefault(t.Lang, "ru"), t.Name, t.Price, t.LimitText, t.Fee, t.Summary,
		orDefault(t.PricingType, "fixed"), t.Advantages, t.Disadvantages); err != nil {
		return fmt.Errorf("insert tariff %s/%s: %w", t.Ref, t.Lang, err)
	}
	return nil
}

func upsertProductRow(ctx context.Context, tx execer, orgID uuid.UUID, p domain.Product) error {
	if _, err := tx.Exec(ctx, `INSERT INTO xchats.ai_products
		(organization_id, ref, lang, name, price, description, category, availability)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		ON CONFLICT (organization_id, ref, lang) DO UPDATE SET
			name=EXCLUDED.name, price=EXCLUDED.price, description=EXCLUDED.description,
			category=EXCLUDED.category, availability=EXCLUDED.availability, updated_at=now()`,
		orgID, p.Ref, orDefault(p.Lang, "ru"), p.Name, p.Price, p.Description, p.Category, p.Availability); err != nil {
		return fmt.Errorf("insert product %s/%s: %w", p.Ref, p.Lang, err)
	}
	return nil
}

func upsertContactRow(ctx context.Context, tx execer, orgID uuid.UUID, c domain.Contact) error {
	if _, err := tx.Exec(ctx, `INSERT INTO xchats.ai_contacts
		(organization_id, slug, lang, whatsapp, email, address, legal, callback_time,
		 working_hours, phone, website, instagram)
		VALUES ($1,'support',$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		ON CONFLICT (organization_id, lang) DO UPDATE SET
			whatsapp=EXCLUDED.whatsapp, email=EXCLUDED.email, address=EXCLUDED.address,
			legal=EXCLUDED.legal, callback_time=EXCLUDED.callback_time,
			working_hours=EXCLUDED.working_hours, phone=EXCLUDED.phone,
			website=EXCLUDED.website, instagram=EXCLUDED.instagram, updated_at=now()`,
		orgID, orDefault(c.Lang, "*"), c.WhatsApp, c.Email, c.Address, c.Legal, c.CallbackTime,
		c.WorkingHours, c.Phone, c.Website, c.Instagram); err != nil {
		return fmt.Errorf("insert contact %s: %w", c.Lang, err)
	}
	return nil
}

// upsertPolicyRow writes one ai_policies row — an exact clone of upsertContactRow
// (singleton slug 'main', keyed by lang).
func upsertPolicyRow(ctx context.Context, tx execer, orgID uuid.UUID, p domain.Policy) error {
	if _, err := tx.Exec(ctx, `INSERT INTO xchats.ai_policies
		(organization_id, slug, lang, delivery_cost, delivery_time, free_delivery_from, min_order,
		 prepayment, installment, return_period, warranty)
		VALUES ($1,'main',$2,$3,$4,$5,$6,$7,$8,$9,$10)
		ON CONFLICT (organization_id, lang) DO UPDATE SET
			delivery_cost=EXCLUDED.delivery_cost, delivery_time=EXCLUDED.delivery_time,
			free_delivery_from=EXCLUDED.free_delivery_from, min_order=EXCLUDED.min_order,
			prepayment=EXCLUDED.prepayment, installment=EXCLUDED.installment,
			return_period=EXCLUDED.return_period, warranty=EXCLUDED.warranty, updated_at=now()`,
		orgID, orDefault(p.Lang, "*"), p.DeliveryCost, p.DeliveryTime, p.FreeDeliveryFrom, p.MinOrder,
		p.Prepayment, p.Installment, p.ReturnPeriod, p.Warranty); err != nil {
		return fmt.Errorf("insert policy %s: %w", p.Lang, err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Deterministic gate (the Approve safety boundary — see draft.go · Approve)
// ---------------------------------------------------------------------------

// GateError reports the deterministic approve-gate violations (plan/12 · gate).
type GateError struct{ Reasons []string }

func (e *GateError) Error() string { return "publish gate failed: " + strings.Join(e.Reasons, "; ") }

// gate is the deterministic approve gate (pure, testable): over the resulting
// LIVE set (live ∪ approved, minus deletes), every asset is described, every topic
// body is pure prose (no fact tokens, no literal currency), no blob dangles, no
// request pends. Facts are typed columns (validated at reply-render time, fail
// closed), so the gate does not touch them.
func gate(snap *domain.Snapshot, pendingRequests int, blobExists func(ref string) bool) []string {
	var reasons []string
	for _, a := range snap.Assets {
		reasons = append(reasons, gateAsset(a, blobExists)...)
	}
	for _, t := range snap.Topics {
		reasons = append(reasons, gateTopicBody(t.Slug, t.BodyMD)...)
	}
	if pendingRequests > 0 {
		reasons = append(reasons, fmt.Sprintf("%d unresolved request(s)", pendingRequests))
	}
	return reasons
}

// gateAsset is the per-asset half of the gate — also reused stand-alone by the
// /kb/* live-write path (live.go), so a single asset write is held to the exact
// same bar as an approve.
func gateAsset(a domain.Asset, blobExists func(ref string) bool) []string {
	var reasons []string
	if strings.TrimSpace(a.Description) == "" {
		reasons = append(reasons, fmt.Sprintf("asset %q has no description", a.Ref))
	}
	if blobExists != nil && a.Ref != "" && !blobExists(a.Ref) {
		reasons = append(reasons, fmt.Sprintf("asset %q has no stored bytes", a.Ref))
	}
	return reasons
}

// gateTopicBody is the per-topic half of the gate — also reused stand-alone by
// the /kb/* live-write path (live.go).
func gateTopicBody(slug, bodyMD string) []string {
	var reasons []string
	// Topic bodies are pure prose (14 Decision 3): a fact token in a body means
	// stored knowledge is carrying a value — it belongs in a typed column, quoted
	// only in replies.
	if strings.Contains(bodyMD, "{{") {
		reasons = append(reasons, fmt.Sprintf("topic %q body must be pure prose — no {{...}} tokens", slug))
	}
	// A literal price/currency amount in a body is an unconfirmed number shipping
	// to customers — the fact belongs in a typed tariff/product column.
	if lit := rawCurrencyRE.FindString(bodyMD); lit != "" {
		reasons = append(reasons, fmt.Sprintf("topic %q has a literal amount %q — put the fact in a typed column", slug, strings.TrimSpace(lit)))
	}
	return reasons
}

// rawCurrencyRE matches a number immediately followed by a currency marker
// ("25 000 ₸", "9900тг", "$50"): the class of unconfirmed amount that must live in
// a typed fact column, never as a literal in a rendered reply body. Step numbers
// ("1) 2) 3)") and bare counts are intentionally NOT matched.
var rawCurrencyRE = regexp.MustCompile(`(?:[0-9][0-9 \x{00a0}.,]*\s*(?:₸|₽|€|£|тг|тенге|руб)|[$€£]\s*[0-9])`)

func (s *Store) pendingRequestCount(ctx context.Context, orgID uuid.UUID) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx, `SELECT count(*) FROM xchats.kbd_requests
		WHERE organization_id = $1 AND state = 'pending'`, orgID).Scan(&n)
	return n, err
}

func orDefaultInt(v, def int) int {
	if v == 0 {
		return def
	}
	return v
}
