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
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/yerassyldanay/xchats/backend/internal/brain/domain"
)

// ErrStale is returned when an optimistic-concurrency check (If-Match) fails: the
// draft blob moved since the client loaded it.
var ErrStale = errors.New("kbstore: stale draft write")

// ErrUnknownKind is returned for an unrecognized entity kind.
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

// loadLiveContent fills Topics and the typed fact tables from the live ai_
// tables for an org.
func (s *Store) loadLiveContent(ctx context.Context, orgID uuid.UUID, snap *domain.Snapshot) error {
	trows, err := s.pool.Query(ctx, `SELECT slug, lang, title, body_md
		FROM xchats.ai_topics WHERE organization_id = $1 ORDER BY created_at`, orgID)
	if err != nil {
		return err
	}
	for trows.Next() {
		var t domain.Topic
		if err := trows.Scan(&t.Slug, &t.Language, &t.Title, &t.BodyMD); err != nil {
			trows.Close()
			return err
		}
		snap.Topics = append(snap.Topics, t)
	}
	trows.Close()
	if err := trows.Err(); err != nil {
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

	// availability is a dead legacy column (plan/database-schema.md: not part
	// of the target) — no longer read; domain.Product.Availability stays
	// permanently empty for a DB-backed snapshot.
	prows, err := s.pool.Query(ctx, `SELECT ref, lang, name, price, description, category
		FROM xchats.ai_products WHERE organization_id = $1 ORDER BY created_at`, orgID)
	if err != nil {
		return err
	}
	for prows.Next() {
		var p domain.Product
		if err := prows.Scan(&p.Ref, &p.Lang, &p.Name, &p.Price, &p.Description, &p.Category); err != nil {
			prows.Close()
			return err
		}
		snap.Products = append(snap.Products, p)
	}
	prows.Close()
	if err := prows.Err(); err != nil {
		return err
	}

	crows, err := s.pool.Query(ctx, `SELECT lang, whatsapp, email, address, legal_information, callback_time,
		working_hours, phone, website, instagram
		FROM xchats.ai_contacts WHERE organization_id = $1 ORDER BY created_at`, orgID)
	if err != nil {
		return err
	}
	for crows.Next() {
		var c domain.Contact
		var legalInfo *string
		if err := crows.Scan(&c.Lang, &c.WhatsApp, &c.Email, &c.Address, &legalInfo, &c.CallbackTime,
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

	polrows, err := s.pool.Query(ctx, `SELECT lang, delivery_cost, delivery_in_days, free_delivery_from, min_order,
		prepayment, installment, return_period_in_days, warranty
		FROM xchats.ai_policies WHERE organization_id = $1 ORDER BY created_at`, orgID)
	if err != nil {
		return err
	}
	for polrows.Next() {
		var p domain.Policy
		var deliveryInDays, returnPeriodInDays *string
		if err := polrows.Scan(&p.Lang, &p.DeliveryCost, &deliveryInDays, &p.FreeDeliveryFrom, &p.MinOrder,
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
			(organization_id, slug, lang, title, body_md)
			VALUES ($1,$2,$3,$4,$5)
			ON CONFLICT (organization_id, slug) DO UPDATE SET
				lang = EXCLUDED.lang, title = EXCLUDED.title,
				body_md = EXCLUDED.body_md, updated_at = now()`,
			orgID, t.Slug, t.Language, t.Title, t.BodyMD); err != nil {
			return fmt.Errorf("insert topic %s: %w", t.Slug, err)
		}
	}
	for _, t := range snap.Tariffs {
		if err := upsertTariffRow(ctx, tx, orgID, t); err != nil {
			return err
		}
	}
	for _, p := range snap.Products {
		// nil: the brain-seed snapshot has no stock signal — let the schema
		// default (true) or, on update, the existing value stand.
		if err := upsertProductRow(ctx, tx, orgID, p, nil); err != nil {
			return err
		}
	}
	for _, c := range snap.Contacts {
		if err := upsertContactRow(ctx, tx, orgID, c); err != nil {
			return err
		}
	}
	for _, p := range snap.Policies {
		// "": the brain-seed snapshot has no outside_zones_note concept.
		if err := upsertPolicyRow(ctx, tx, orgID, p, ""); err != nil {
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

// upsertProductRow writes one ai_products row. inStock is nil-able so a caller
// with no opinion on stock (legacy brain-seed/Approve materialize paths)
// leaves it at its column default on insert and PRESERVES the existing value
// on update, instead of silently resetting it — only the /kb/* live-write path
// (PutLiveProduct) ever passes a non-nil value. availability is a dead legacy
// column (plan/database-schema.md: not part of the target) and is no longer
// written — p.Availability is ignored. sales_status is likewise untouched
// here: no caller of this function has an opinion on it yet (no live-write
// payload exposes it), so it stays at whatever migration 0011 backfilled
// (its schema DEFAULT 'active' otherwise).
func upsertProductRow(ctx context.Context, tx execer, orgID uuid.UUID, p domain.Product, inStock *bool) error {
	if _, err := tx.Exec(ctx, `INSERT INTO xchats.ai_products
		(organization_id, ref, lang, name, price, description, category, in_stock)
		VALUES ($1,$2,$3,$4,$5,$6,$7,COALESCE($8,true))
		ON CONFLICT (organization_id, ref, lang) DO UPDATE SET
			name=EXCLUDED.name, price=EXCLUDED.price, description=EXCLUDED.description,
			category=EXCLUDED.category,
			in_stock=COALESCE($8, xchats.ai_products.in_stock), updated_at=now()`,
		orgID, p.Ref, orDefault(p.Lang, "ru"), p.Name, p.Price, p.Description, p.Category, inStock); err != nil {
		return fmt.Errorf("insert product %s/%s: %w", p.Ref, p.Lang, err)
	}
	return nil
}

func upsertContactRow(ctx context.Context, tx execer, orgID uuid.UUID, c domain.Contact) error {
	if _, err := tx.Exec(ctx, `INSERT INTO xchats.ai_contacts
		(organization_id, slug, lang, whatsapp, email, address, legal_information, callback_time,
		 working_hours, phone, website, instagram)
		VALUES ($1,'support',$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		ON CONFLICT (organization_id, lang) DO UPDATE SET
			whatsapp=EXCLUDED.whatsapp, email=EXCLUDED.email, address=EXCLUDED.address,
			legal_information=EXCLUDED.legal_information, callback_time=EXCLUDED.callback_time,
			working_hours=EXCLUDED.working_hours, phone=EXCLUDED.phone,
			website=EXCLUDED.website, instagram=EXCLUDED.instagram, updated_at=now()`,
		orgID, orDefault(c.Lang, "*"), c.WhatsApp, c.Email, c.Address, c.Legal, c.CallbackTime,
		c.WorkingHours, c.Phone, c.Website, c.Instagram); err != nil {
		return fmt.Errorf("insert contact %s: %w", c.Lang, err)
	}
	return nil
}

// upsertPolicyRow writes one ai_policies row — an exact clone of upsertContactRow
// (singleton slug 'main', keyed by lang). outsideZonesNote is a plain string,
// not a pointer: every caller already resolves it read-modify-write style
// (currentLivePolicy/currentPolicy) before reaching here, so there is no
// "leave unchanged" case left to express at this layer.
func upsertPolicyRow(ctx context.Context, tx execer, orgID uuid.UUID, p domain.Policy, outsideZonesNote string) error {
	if _, err := tx.Exec(ctx, `INSERT INTO xchats.ai_policies
		(organization_id, slug, lang, delivery_cost, delivery_in_days, free_delivery_from, min_order,
		 prepayment, installment, return_period_in_days, warranty, outside_zones_note)
		VALUES ($1,'main',$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		ON CONFLICT (organization_id, lang) DO UPDATE SET
			delivery_cost=EXCLUDED.delivery_cost, delivery_in_days=EXCLUDED.delivery_in_days,
			free_delivery_from=EXCLUDED.free_delivery_from, min_order=EXCLUDED.min_order,
			prepayment=EXCLUDED.prepayment, installment=EXCLUDED.installment,
			return_period_in_days=EXCLUDED.return_period_in_days, warranty=EXCLUDED.warranty,
			outside_zones_note=EXCLUDED.outside_zones_note, updated_at=now()`,
		orgID, orDefault(p.Lang, "*"), p.DeliveryCost, p.DeliveryTime, p.FreeDeliveryFrom, p.MinOrder,
		p.Prepayment, p.Installment, p.ReturnPeriod, p.Warranty, outsideZonesNote); err != nil {
		return fmt.Errorf("insert policy %s: %w", p.Lang, err)
	}
	return nil
}

// upsertConfigRow upserts the org's ai_assistants row. It fixes the silent
// no-op PatchLiveConfig had (live.go): a bare UPDATE hits zero rows when the
// org has no ai_assistants row yet (e.g. a fresh org with no seed/kb-load run
// yet), so a first-ever PATCH /kb/config appeared to succeed but changed
// nothing. ON CONFLICT DO UPDATE with COALESCE mirrors PatchLiveConfig's own
// "only non-nil fields change" contract on the insert path too, using each
// column's own schema default when the row is being created for the first time.
func upsertConfigRow(ctx context.Context, tx execer, orgID uuid.UUID, p ConfigPatch) error {
	_, err := tx.Exec(ctx, `INSERT INTO xchats.ai_assistants
		(organization_id, persona, mission, guardrails, language_policy, reply_max_words)
		VALUES ($1, COALESCE($2,''), COALESCE($3,''), COALESCE($4,''), COALESCE($5,''), COALESCE($6,120))
		ON CONFLICT (organization_id) DO UPDATE SET
			persona = COALESCE($2, xchats.ai_assistants.persona),
			mission = COALESCE($3, xchats.ai_assistants.mission),
			guardrails = COALESCE($4, xchats.ai_assistants.guardrails),
			language_policy = COALESCE($5, xchats.ai_assistants.language_policy),
			reply_max_words = COALESCE($6, xchats.ai_assistants.reply_max_words),
			updated_at = now()`,
		orgID, p.Persona, p.Mission, p.Guardrails, p.LanguagePolicy, p.ReplyMaxWords)
	return err
}

// auditRow appends one xchats.ai_audit_log row — action is 'edit'|'delete' for
// the /kb/* live-write path (no CHECK constraint pins the vocabulary; 'approve'
// remains the Playground Approve's own action). actor is nil-able: a zero
// uuid.UUID (no authenticated user in context) is stored as SQL NULL rather
// than a literal zero UUID, since actor_user_id's FK would otherwise reject it.
func auditRow(ctx context.Context, tx execer, orgID uuid.UUID, actor uuid.UUID, action, note string) error {
	_, err := tx.Exec(ctx, `INSERT INTO xchats.ai_audit_log (organization_id, action, actor_user_id, note)
		VALUES ($1,$2,$3,$4)`,
		orgID, action, uuid.NullUUID{UUID: actor, Valid: actor != uuid.Nil}, note)
	return err
}

// ---------------------------------------------------------------------------
// Deterministic gate (the Approve safety boundary — see draft.go · Approve)
// ---------------------------------------------------------------------------

// GateError reports the deterministic approve-gate violations (plan/12 · gate).
type GateError struct{ Reasons []string }

func (e *GateError) Error() string { return "publish gate failed: " + strings.Join(e.Reasons, "; ") }

// gate is the deterministic approve gate (pure, testable): over the resulting
// LIVE set (live ∪ approved, minus deletes), every topic body is pure prose (no
// fact tokens, no literal currency), no request pends. Facts are typed columns
// (validated at reply-render time, fail closed), so the gate does not touch them.
func gate(snap *domain.Snapshot, pendingRequests int) []string {
	var reasons []string
	for _, t := range snap.Topics {
		reasons = append(reasons, gateTopicBody(t.Slug, t.BodyMD)...)
	}
	if pendingRequests > 0 {
		reasons = append(reasons, fmt.Sprintf("%d unresolved request(s)", pendingRequests))
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
