// Package kbstore is the writable Knowledge Base data layer: it loads the
// PUBLISHED snapshot the brain reasons from, and owns the Playground's draft
// lifecycle (open = clone published → single draft, CRUD, publish = atomic swap +
// version, rollback, discard) over the ai_* tables from migration 0003.
//
// The brain's read contract (*domain.Snapshot) is unchanged — only its SOURCE
// moves from the Go literal (internal/brain/seed.go) to these tables. Writes live
// entirely here; the brain never sees the draft columns (review_state/provenance).
package kbstore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/yerassyldanay/xchats/backend/internal/brain/domain"
)

// jsonObj marshals a small map to a compact JSON string for a jsonb column —
// safer than fmt formatting, which only produces valid JSON for safe-charset input.
func jsonObj(m map[string]any) string {
	b, err := json.Marshal(m)
	if err != nil {
		return "{}"
	}
	return string(b)
}

// ErrNoDraft is returned when an operation needs an open draft but none exists.
var ErrNoDraft = errors.New("kbstore: no open draft")

// ErrNoPublished is returned when no published snapshot exists for an org.
var ErrNoPublished = errors.New("kbstore: no published snapshot")

// ErrStale is returned when an optimistic-concurrency check (If-Match) fails: the
// draft moved since the client loaded it.
var ErrStale = errors.New("kbstore: stale draft write")

// draftVersion is the sentinel version of the single open draft; publishing
// stamps it with the next real (monotonic, >0) version.
const draftVersion = 0

// Store wraps the pgx pool with KB operations.
type Store struct {
	pool *pgxpool.Pool
}

// New builds a KBStore over an existing pool.
func New(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

// ---------------------------------------------------------------------------
// Published load (the brain's source) + seed
// ---------------------------------------------------------------------------

// LoadPublished returns the highest-version published snapshot for an org as a
// brain-ready *domain.Snapshot (only approved rows ever reach a published
// snapshot, so no review filter is needed here).
func (s *Store) LoadPublished(ctx context.Context, orgID uuid.UUID) (*domain.Snapshot, error) {
	var snapID uuid.UUID
	snap := &domain.Snapshot{Loaded: time.Now()}
	err := s.pool.QueryRow(ctx, `
		SELECT id, version, persona, mission, guardrails, language_policy, reply_max_words
		FROM xchats.ai_snapshots
		WHERE organization_id = $1 AND snapshot_state = 'published'
		ORDER BY version DESC LIMIT 1`, orgID).
		Scan(&snapID, &snap.Config.Version, &snap.Config.Persona, &snap.Config.Mission,
			&snap.Config.Guardrails, &snap.Config.LanguagePolicy, &snap.Config.ReplyMaxWords)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNoPublished
	}
	if err != nil {
		return nil, err
	}
	if err := s.loadSnapshotContent(ctx, snapID, snap, false); err != nil {
		return nil, err
	}
	return snap, nil
}

// loadSnapshotContent fills Topics/Assets/Values for a snapshot. When
// approvedOnly is set, rejected/proposed rows are excluded (used by the publish
// copy); a published snapshot already holds only approved rows.
func (s *Store) loadSnapshotContent(ctx context.Context, snapID uuid.UUID, snap *domain.Snapshot, approvedOnly bool) error {
	filter := ""
	if approvedOnly {
		filter = ` AND review_state = 'approved'`
	}

	trows, err := s.pool.Query(ctx, `SELECT slug, lang, title, keywords, body_md
		FROM xchats.ai_topics WHERE snapshot_id = $1`+filter+` ORDER BY created_at`, snapID)
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

	arows, err := s.pool.Query(ctx, `SELECT ref, asset_kind, topic_slug, title, description, asset_url, lang
		FROM xchats.ai_assets WHERE snapshot_id = $1`+filter+` ORDER BY created_at`, snapID)
	if err != nil {
		return err
	}
	for arows.Next() {
		var a domain.Asset
		if err := arows.Scan(&a.Ref, &a.Kind, &a.TopicSlug, &a.Title, &a.Description, &a.URL, &a.Language); err != nil {
			arows.Close()
			return err
		}
		snap.Assets = append(snap.Assets, a)
	}
	arows.Close()
	if err := arows.Err(); err != nil {
		return err
	}

	vrows, err := s.pool.Query(ctx, `SELECT token, lang, value_text, description
		FROM xchats.ai_values WHERE snapshot_id = $1`+filter+` ORDER BY created_at`, snapID)
	if err != nil {
		return err
	}
	vb := domain.NewValueBook()
	for vrows.Next() {
		var v domain.Value
		if err := vrows.Scan(&v.Token, &v.Lang, &v.Text, &v.Description); err != nil {
			vrows.Close()
			return err
		}
		vb.Add(v)
	}
	vrows.Close()
	if err := vrows.Err(); err != nil {
		return err
	}
	snap.Values = vb
	return nil
}

// SeedIfEmpty inserts the given snapshot as version 1, published, when the org has
// no published snapshot yet — so the brain keeps answering from the DB on first
// boot. It is idempotent: a no-op once a published snapshot exists.
func (s *Store) SeedIfEmpty(ctx context.Context, orgID uuid.UUID, seed *domain.Snapshot) error {
	var exists bool
	if err := s.pool.QueryRow(ctx, `SELECT EXISTS(
		SELECT 1 FROM xchats.ai_snapshots WHERE organization_id = $1 AND snapshot_state = 'published')`,
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

	var snapID uuid.UUID
	if err := tx.QueryRow(ctx, `
		INSERT INTO xchats.ai_snapshots
			(organization_id, version, snapshot_state, persona, mission, guardrails, language_policy, reply_max_words, published_at)
		VALUES ($1, 1, 'published', $2, $3, $4, $5, $6, now())
		RETURNING id`,
		orgID, seed.Config.Persona, seed.Config.Mission, seed.Config.Guardrails,
		seed.Config.LanguagePolicy, orDefaultInt(seed.Config.ReplyMaxWords, 120)).Scan(&snapID); err != nil {
		return err
	}
	if err := insertContent(ctx, tx, snapID, seed, "approved", `{"source":"seed"}`); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// insertContent writes a snapshot's topics/assets/values with a fixed review_state
// + provenance. Used by seed, draft-clone, publish-copy and rollback.
func insertContent(ctx context.Context, tx pgx.Tx, snapID uuid.UUID, snap *domain.Snapshot, review, provenance string) error {
	for _, t := range snap.Topics {
		if _, err := tx.Exec(ctx, `INSERT INTO xchats.ai_topics
			(snapshot_id, slug, lang, title, keywords, body_md, review_state, provenance)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8::jsonb)`,
			snapID, t.Slug, t.Language, t.Title, t.Keywords, t.BodyMD, review, provenance); err != nil {
			return fmt.Errorf("insert topic %s: %w", t.Slug, err)
		}
	}
	for _, a := range snap.Assets {
		if _, err := tx.Exec(ctx, `INSERT INTO xchats.ai_assets
			(snapshot_id, ref, asset_kind, topic_slug, title, description, asset_url, lang, review_state, provenance)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10::jsonb)`,
			snapID, a.Ref, a.Kind, a.TopicSlug, a.Title, a.Description, a.URL, a.Language, review, provenance); err != nil {
			return fmt.Errorf("insert asset %s: %w", a.Ref, err)
		}
	}
	for _, v := range snap.Values.List() {
		if _, err := tx.Exec(ctx, `INSERT INTO xchats.ai_values
			(snapshot_id, token, lang, value_text, description, review_state, provenance)
			VALUES ($1,$2,$3,$4,$5,$6,$7::jsonb)`,
			snapID, v.Token, v.Lang, v.Text, v.Description, review, provenance); err != nil {
			return fmt.Errorf("insert value %s/%s: %w", v.Token, v.Lang, err)
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Draft lifecycle
// ---------------------------------------------------------------------------

// OpenDraft returns the org's open draft snapshot id, cloning the published
// snapshot into a fresh draft if none is open. Idempotent (one draft per org,
// enforced by the partial unique index).
func (s *Store) OpenDraft(ctx context.Context, orgID uuid.UUID) (uuid.UUID, error) {
	var draftID uuid.UUID
	err := s.pool.QueryRow(ctx, `SELECT id FROM xchats.ai_snapshots
		WHERE organization_id = $1 AND snapshot_state = 'draft'`, orgID).Scan(&draftID)
	if err == nil {
		return draftID, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, err
	}

	published, perr := s.LoadPublished(ctx, orgID)
	if perr != nil && !errors.Is(perr, ErrNoPublished) {
		return uuid.Nil, perr
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	defer tx.Rollback(ctx)

	cfg := domain.AssistantConfig{ReplyMaxWords: 120}
	if published != nil {
		cfg = published.Config
	}
	if err := tx.QueryRow(ctx, `
		INSERT INTO xchats.ai_snapshots
			(organization_id, version, snapshot_state, persona, mission, guardrails, language_policy, reply_max_words)
		VALUES ($1, $2, 'draft', $3, $4, $5, $6, $7)
		RETURNING id`,
		orgID, draftVersion, cfg.Persona, cfg.Mission, cfg.Guardrails, cfg.LanguagePolicy,
		orDefaultInt(cfg.ReplyMaxWords, 120)).Scan(&draftID); err != nil {
		return uuid.Nil, err
	}
	if published != nil {
		// Clone the published baseline as approved rows (the draft starts from live).
		if err := insertContent(ctx, tx, draftID, published, "approved", `{"source":"clone"}`); err != nil {
			return uuid.Nil, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return uuid.Nil, err
	}
	return draftID, nil
}

// draftID resolves the org's open draft id or ErrNoDraft.
func (s *Store) draftID(ctx context.Context, orgID uuid.UUID) (uuid.UUID, error) {
	var id uuid.UUID
	err := s.pool.QueryRow(ctx, `SELECT id FROM xchats.ai_snapshots
		WHERE organization_id = $1 AND snapshot_state = 'draft'`, orgID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, ErrNoDraft
	}
	return id, err
}

// DiscardDraft deletes the open draft (cascade removes its rows/materials/requests).
func (s *Store) DiscardDraft(ctx context.Context, orgID uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM xchats.ai_snapshots
		WHERE organization_id = $1 AND snapshot_state = 'draft'`, orgID)
	return err
}

// ---------------------------------------------------------------------------
// Publish + rollback
// ---------------------------------------------------------------------------

// GateError reports the deterministic publish-gate violations (plan/12 · gate).
type GateError struct{ Reasons []string }

func (e *GateError) Error() string { return "publish gate failed: " + strings.Join(e.Reasons, "; ") }

// Publish runs the deterministic gate over the draft's APPROVED rows and, on pass,
// atomically promotes the draft to a new published version (dropping non-approved
// rows). blobExists reports whether an asset's bytes are present (nil skips the
// dangling-blob check). Returns the new version.
func (s *Store) Publish(ctx context.Context, orgID uuid.UUID, blobExists func(ref string) bool) (int, error) {
	draftID, err := s.draftID(ctx, orgID)
	if err != nil {
		return 0, err
	}

	// Build a snapshot of the APPROVED draft content to gate + publish.
	approved := &domain.Snapshot{}
	if err := s.loadSnapshotContent(ctx, draftID, approved, true); err != nil {
		return 0, err
	}
	// The gate is a safety boundary, so a failed pending-request count fails closed
	// (block the publish) rather than silently treating it as zero.
	pending, err := s.pendingRequestCount(ctx, draftID)
	if err != nil {
		return 0, err
	}
	if reasons := gate(approved, pending, blobExists); len(reasons) > 0 {
		return 0, &GateError{Reasons: reasons}
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	var nextVersion int
	if err := tx.QueryRow(ctx, `SELECT COALESCE(MAX(version),0)+1
		FROM xchats.ai_snapshots WHERE organization_id = $1`, orgID).Scan(&nextVersion); err != nil {
		return 0, err
	}
	// Drop non-approved rows, then promote the draft snapshot in place.
	for _, tbl := range []string{"ai_topics", "ai_assets", "ai_values"} {
		if _, err := tx.Exec(ctx, `DELETE FROM xchats.`+tbl+
			` WHERE snapshot_id = $1 AND review_state <> 'approved'`, draftID); err != nil {
			return 0, err
		}
	}
	if _, err := tx.Exec(ctx, `UPDATE xchats.ai_snapshots
		SET version = $2, snapshot_state = 'published', published_at = now(), updated_at = now()
		WHERE id = $1`, draftID, nextVersion); err != nil {
		return 0, err
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return nextVersion, nil
}

// gate is the deterministic publish gate (pure, testable): over approved rows,
// every asset is described, every token in a body resolves, no blob dangles, no
// request pends.
func gate(snap *domain.Snapshot, pendingRequests int, blobExists func(ref string) bool) []string {
	var reasons []string
	for _, a := range snap.Assets {
		if strings.TrimSpace(a.Description) == "" {
			reasons = append(reasons, fmt.Sprintf("asset %q has no description", a.Ref))
		}
		if blobExists != nil && a.Ref != "" && !blobExists(a.Ref) {
			reasons = append(reasons, fmt.Sprintf("asset %q has no stored bytes", a.Ref))
		}
	}
	for _, t := range snap.Topics {
		if _, err := snap.Values.Render(t.BodyMD, t.Language); err != nil {
			reasons = append(reasons, fmt.Sprintf("topic %q: %v", t.Slug, err))
		}
		// A literal price/currency amount in a body is an unconfirmed number shipping
		// to customers — it must be a value token instead. (This enforces the subset
		// of the "no digits in bodies" rule that is safety-critical; broader numeric
		// tokenization is the LLM synthesizer's job.)
		if lit := rawCurrencyRE.FindString(t.BodyMD); lit != "" {
			reasons = append(reasons, fmt.Sprintf("topic %q has a literal amount %q — use a value token", t.Slug, strings.TrimSpace(lit)))
		}
	}
	if pendingRequests > 0 {
		reasons = append(reasons, fmt.Sprintf("%d unresolved request(s)", pendingRequests))
	}
	return reasons
}

// rawCurrencyRE matches a number immediately followed by a currency marker
// ("25 000 ₸", "9900тг", "$50"): the class of unconfirmed amount that must live in
// ai_values as a token, never as a literal in a rendered reply body. Step numbers
// ("1) 2) 3)") and bare counts are intentionally NOT matched.
var rawCurrencyRE = regexp.MustCompile(`(?:[0-9][0-9 \x{00a0}.,]*\s*(?:₸|₽|€|£|тг|тенге|руб)|[$€£]\s*[0-9])`)

func (s *Store) pendingRequestCount(ctx context.Context, snapID uuid.UUID) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx, `SELECT count(*) FROM xchats.ai_builder_requests
		WHERE snapshot_id = $1 AND state = 'pending'`, snapID).Scan(&n)
	return n, err
}

// Rollback re-publishes a prior version's content as a new highest version, making
// it live again (LoadPublished reads the highest version).
func (s *Store) Rollback(ctx context.Context, orgID uuid.UUID, version int) (int, error) {
	var srcID uuid.UUID
	prior := &domain.Snapshot{}
	err := s.pool.QueryRow(ctx, `
		SELECT id, persona, mission, guardrails, language_policy, reply_max_words
		FROM xchats.ai_snapshots
		WHERE organization_id = $1 AND snapshot_state = 'published' AND version = $2`,
		orgID, version).Scan(&srcID, &prior.Config.Persona, &prior.Config.Mission,
		&prior.Config.Guardrails, &prior.Config.LanguagePolicy, &prior.Config.ReplyMaxWords)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrNoPublished
	}
	if err != nil {
		return 0, err
	}
	if err := s.loadSnapshotContent(ctx, srcID, prior, false); err != nil {
		return 0, err
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	var nextVersion int
	if err := tx.QueryRow(ctx, `SELECT COALESCE(MAX(version),0)+1
		FROM xchats.ai_snapshots WHERE organization_id = $1`, orgID).Scan(&nextVersion); err != nil {
		return 0, err
	}
	var newID uuid.UUID
	if err := tx.QueryRow(ctx, `
		INSERT INTO xchats.ai_snapshots
			(organization_id, version, snapshot_state, persona, mission, guardrails, language_policy, reply_max_words, published_at)
		VALUES ($1,$2,'published',$3,$4,$5,$6,$7, now()) RETURNING id`,
		orgID, nextVersion, prior.Config.Persona, prior.Config.Mission, prior.Config.Guardrails,
		prior.Config.LanguagePolicy, orDefaultInt(prior.Config.ReplyMaxWords, 120)).Scan(&newID); err != nil {
		return 0, err
	}
	if err := insertContent(ctx, tx, newID, prior, "approved", jsonObj(map[string]any{"source": "rollback", "from_version": version})); err != nil {
		return 0, err
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return nextVersion, nil
}

func orDefaultInt(v, def int) int {
	if v == 0 {
		return def
	}
	return v
}
