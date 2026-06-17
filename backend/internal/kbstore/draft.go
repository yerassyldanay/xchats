package kbstore

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// ---------------------------------------------------------------------------
// Rich draft view (the editor reads this; carries ids + review_state + provenance)
// ---------------------------------------------------------------------------

// DraftConfig is the draft snapshot's config block.
type DraftConfig struct {
	SnapshotID     uuid.UUID `json:"snapshot_id"`
	Version        int       `json:"version"`
	State          string    `json:"state"`
	Persona        string    `json:"persona"`
	Mission        string    `json:"mission"`
	Guardrails     string    `json:"guardrails"`
	LanguagePolicy string    `json:"language_policy"`
	ReplyMaxWords  int       `json:"reply_max_words"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// TopicRow / AssetRow / ValueRow are editor-facing KB rows (review + provenance).
type TopicRow struct {
	ID          uuid.UUID `json:"id"`
	Slug        string    `json:"slug"`
	Lang        string    `json:"lang"`
	Title       string    `json:"title"`
	Keywords    string    `json:"keywords"`
	BodyMD      string    `json:"body_md"`
	ReviewState string    `json:"review_state"`
	Provenance  string    `json:"provenance"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type AssetRow struct {
	ID          uuid.UUID `json:"id"`
	Ref         string    `json:"ref"`
	Kind        string    `json:"kind"`
	TopicSlug   string    `json:"topic_slug"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	URL         string    `json:"url"`
	Lang        string    `json:"lang"`
	ReviewState string    `json:"review_state"`
	Provenance  string    `json:"provenance"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type ValueRow struct {
	ID          uuid.UUID `json:"id"`
	Token       string    `json:"token"`
	Lang        string    `json:"lang"`
	ValueText   string    `json:"value_text"`
	Description string    `json:"description"`
	ReviewState string    `json:"review_state"`
	Provenance  string    `json:"provenance"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// DraftView is the whole working draft for the editor.
type DraftView struct {
	Config    DraftConfig `json:"config"`
	Topics    []TopicRow  `json:"topics"`
	Assets    []AssetRow  `json:"assets"`
	Values    []ValueRow  `json:"values"`
	Materials []Material  `json:"materials"`
	Requests  []Request   `json:"requests"`
}

// GetDraft assembles the full draft view, opening a draft if none is present.
// Use it on write paths (which need a working copy); the read-only GET endpoint
// uses ReadDraft so a plain read never mutates state.
func (s *Store) GetDraft(ctx context.Context, orgID uuid.UUID) (*DraftView, error) {
	draftID, err := s.OpenDraft(ctx, orgID)
	if err != nil {
		return nil, err
	}
	return s.buildDraftView(ctx, draftID)
}

// ReadDraft assembles the draft view WITHOUT opening one — a side-effect-free read.
// Returns ErrNoDraft when no draft is open (the caller decides how to present that).
func (s *Store) ReadDraft(ctx context.Context, orgID uuid.UUID) (*DraftView, error) {
	draftID, err := s.draftID(ctx, orgID)
	if err != nil {
		return nil, err
	}
	return s.buildDraftView(ctx, draftID)
}

// DraftUpdatedAt returns the open draft's updated_at — the optimistic-concurrency
// token clients echo via If-Match. ErrNoDraft when none is open.
func (s *Store) DraftUpdatedAt(ctx context.Context, orgID uuid.UUID) (time.Time, error) {
	id, err := s.draftID(ctx, orgID)
	if err != nil {
		return time.Time{}, err
	}
	var t time.Time
	err = s.pool.QueryRow(ctx, `SELECT updated_at FROM xchats.ai_snapshots WHERE id = $1`, id).Scan(&t)
	return t, err
}

// TouchDraft bumps the open draft's updated_at so the concurrency token advances
// after any row mutation (called once per successful write). No-op if no draft.
func (s *Store) TouchDraft(ctx context.Context, orgID uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `UPDATE xchats.ai_snapshots SET updated_at = now()
		WHERE organization_id = $1 AND snapshot_state = 'draft'`, orgID)
	return err
}

func (s *Store) buildDraftView(ctx context.Context, draftID uuid.UUID) (*DraftView, error) {
	v := &DraftView{}
	if err := s.pool.QueryRow(ctx, `
		SELECT id, version, snapshot_state, persona, mission, guardrails, language_policy, reply_max_words, updated_at
		FROM xchats.ai_snapshots WHERE id = $1`, draftID).
		Scan(&v.Config.SnapshotID, &v.Config.Version, &v.Config.State, &v.Config.Persona,
			&v.Config.Mission, &v.Config.Guardrails, &v.Config.LanguagePolicy,
			&v.Config.ReplyMaxWords, &v.Config.UpdatedAt); err != nil {
		return nil, err
	}

	trows, err := s.pool.Query(ctx, `SELECT id, slug, lang, title, keywords, body_md, review_state, provenance::text, updated_at
		FROM xchats.ai_topics WHERE snapshot_id = $1 ORDER BY created_at`, draftID)
	if err != nil {
		return nil, err
	}
	for trows.Next() {
		var t TopicRow
		if err := trows.Scan(&t.ID, &t.Slug, &t.Lang, &t.Title, &t.Keywords, &t.BodyMD, &t.ReviewState, &t.Provenance, &t.UpdatedAt); err != nil {
			trows.Close()
			return nil, err
		}
		v.Topics = append(v.Topics, t)
	}
	trows.Close()
	if err := trows.Err(); err != nil {
		return nil, err
	}

	arows, err := s.pool.Query(ctx, `SELECT id, ref, asset_kind, topic_slug, title, description, asset_url, lang, review_state, provenance::text, updated_at
		FROM xchats.ai_assets WHERE snapshot_id = $1 ORDER BY created_at`, draftID)
	if err != nil {
		return nil, err
	}
	for arows.Next() {
		var a AssetRow
		if err := arows.Scan(&a.ID, &a.Ref, &a.Kind, &a.TopicSlug, &a.Title, &a.Description, &a.URL, &a.Lang, &a.ReviewState, &a.Provenance, &a.UpdatedAt); err != nil {
			arows.Close()
			return nil, err
		}
		v.Assets = append(v.Assets, a)
	}
	arows.Close()
	if err := arows.Err(); err != nil {
		return nil, err
	}

	vrows, err := s.pool.Query(ctx, `SELECT id, token, lang, value_text, description, review_state, provenance::text, updated_at
		FROM xchats.ai_values WHERE snapshot_id = $1 ORDER BY created_at`, draftID)
	if err != nil {
		return nil, err
	}
	for vrows.Next() {
		var vr ValueRow
		if err := vrows.Scan(&vr.ID, &vr.Token, &vr.Lang, &vr.ValueText, &vr.Description, &vr.ReviewState, &vr.Provenance, &vr.UpdatedAt); err != nil {
			vrows.Close()
			return nil, err
		}
		v.Values = append(v.Values, vr)
	}
	vrows.Close()
	if err := vrows.Err(); err != nil {
		return nil, err
	}

	if v.Materials, err = s.listMaterials(ctx, draftID); err != nil {
		return nil, err
	}
	if v.Requests, err = s.listRequests(ctx, draftID); err != nil {
		return nil, err
	}
	return v, nil
}

// ---------------------------------------------------------------------------
// Draft CRUD (each resolves the open draft; agent rows pass review='proposed')
// ---------------------------------------------------------------------------

// ConfigPatch carries optional config edits (nil pointer = leave unchanged).
type ConfigPatch struct {
	Persona        *string
	Mission        *string
	Guardrails     *string
	LanguagePolicy *string
	ReplyMaxWords  *int
}

// PatchConfig updates the draft's config block (only non-nil fields).
func (s *Store) PatchConfig(ctx context.Context, orgID uuid.UUID, p ConfigPatch) error {
	id, err := s.draftID(ctx, orgID)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `UPDATE xchats.ai_snapshots SET
		persona = COALESCE($2, persona),
		mission = COALESCE($3, mission),
		guardrails = COALESCE($4, guardrails),
		language_policy = COALESCE($5, language_policy),
		reply_max_words = COALESCE($6, reply_max_words),
		updated_at = now() WHERE id = $1`,
		id, p.Persona, p.Mission, p.Guardrails, p.LanguagePolicy, p.ReplyMaxWords)
	return err
}

// TopicInput is an upsert payload for a draft topic.
type TopicInput struct {
	Slug, Lang, Title, Keywords, BodyMD string
	ReviewState                         string // "" → 'approved'
	Provenance                          string // "" → '{}'
}

// UpsertTopic creates or updates a draft topic by slug.
func (s *Store) UpsertTopic(ctx context.Context, orgID uuid.UUID, in TopicInput) error {
	id, err := s.draftID(ctx, orgID)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `INSERT INTO xchats.ai_topics
		(snapshot_id, slug, lang, title, keywords, body_md, review_state, provenance)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8::jsonb)
		ON CONFLICT (snapshot_id, slug) DO UPDATE SET
			lang = EXCLUDED.lang, title = EXCLUDED.title, keywords = EXCLUDED.keywords,
			body_md = EXCLUDED.body_md, review_state = EXCLUDED.review_state,
			provenance = EXCLUDED.provenance, updated_at = now()`,
		id, in.Slug, orDefault(in.Lang, "ru"), in.Title, in.Keywords, in.BodyMD,
		orDefault(in.ReviewState, "approved"), orDefault(in.Provenance, "{}"))
	return err
}

// DeleteTopic removes a draft topic by slug.
func (s *Store) DeleteTopic(ctx context.Context, orgID uuid.UUID, slug string) error {
	id, err := s.draftID(ctx, orgID)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `DELETE FROM xchats.ai_topics WHERE snapshot_id = $1 AND slug = $2`, id, slug)
	return err
}

// AssetInput is an upsert payload for a draft asset.
type AssetInput struct {
	Ref, Kind, TopicSlug, Title, Description, URL, Lang string
	ReviewState                                         string
	Provenance                                          string
}

// UpsertAsset creates or updates a draft asset by ref.
func (s *Store) UpsertAsset(ctx context.Context, orgID uuid.UUID, in AssetInput) error {
	id, err := s.draftID(ctx, orgID)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `INSERT INTO xchats.ai_assets
		(snapshot_id, ref, asset_kind, topic_slug, title, description, asset_url, lang, review_state, provenance)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10::jsonb)
		ON CONFLICT (snapshot_id, ref) DO UPDATE SET
			asset_kind = EXCLUDED.asset_kind, topic_slug = EXCLUDED.topic_slug, title = EXCLUDED.title,
			description = EXCLUDED.description, asset_url = EXCLUDED.asset_url, lang = EXCLUDED.lang,
			review_state = EXCLUDED.review_state, provenance = EXCLUDED.provenance, updated_at = now()`,
		id, in.Ref, orDefault(in.Kind, "image"), in.TopicSlug, in.Title, in.Description,
		in.URL, orDefault(in.Lang, "ru"), orDefault(in.ReviewState, "approved"), orDefault(in.Provenance, "{}"))
	return err
}

// AssetPatch edits an asset's description and/or reassigns its topic (nil = leave).
type AssetPatch struct {
	Description *string
	TopicSlug   *string
}

// PatchAsset edits an existing draft asset by ref.
func (s *Store) PatchAsset(ctx context.Context, orgID uuid.UUID, ref string, p AssetPatch) error {
	id, err := s.draftID(ctx, orgID)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `UPDATE xchats.ai_assets SET
		description = COALESCE($3, description),
		topic_slug = COALESCE($4, topic_slug),
		updated_at = now() WHERE snapshot_id = $1 AND ref = $2`,
		id, ref, p.Description, p.TopicSlug)
	return err
}

// DeleteAsset removes a draft asset by ref.
func (s *Store) DeleteAsset(ctx context.Context, orgID uuid.UUID, ref string) error {
	id, err := s.draftID(ctx, orgID)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `DELETE FROM xchats.ai_assets WHERE snapshot_id = $1 AND ref = $2`, id, ref)
	return err
}

// ValueInput is an upsert payload for a draft value token.
type ValueInput struct {
	Token, Lang, ValueText, Description string
	ReviewState                         string
	Provenance                          string
}

// UpsertValue creates or updates a draft value by (token, lang).
func (s *Store) UpsertValue(ctx context.Context, orgID uuid.UUID, in ValueInput) error {
	id, err := s.draftID(ctx, orgID)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `INSERT INTO xchats.ai_values
		(snapshot_id, token, lang, value_text, description, review_state, provenance)
		VALUES ($1,$2,$3,$4,$5,$6,$7::jsonb)
		ON CONFLICT (snapshot_id, token, lang) DO UPDATE SET
			value_text = EXCLUDED.value_text, description = EXCLUDED.description,
			review_state = EXCLUDED.review_state, provenance = EXCLUDED.provenance, updated_at = now()`,
		id, in.Token, orDefault(in.Lang, "*"), in.ValueText, in.Description,
		orDefault(in.ReviewState, "approved"), orDefault(in.Provenance, "{}"))
	return err
}

// DeleteValue removes a draft value by (token, lang).
func (s *Store) DeleteValue(ctx context.Context, orgID uuid.UUID, token, lang string) error {
	id, err := s.draftID(ctx, orgID)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `DELETE FROM xchats.ai_values WHERE snapshot_id = $1 AND token = $2 AND lang = $3`,
		id, token, orDefault(lang, "*"))
	return err
}

// SetReviewState flips a draft row's review_state (kind ∈ topics|assets|values).
// proposed → approved | rejected. Returns ErrNoDraft if no draft is open.
func (s *Store) SetReviewState(ctx context.Context, orgID uuid.UUID, kind string, rowID uuid.UUID, state string) error {
	id, err := s.draftID(ctx, orgID)
	if err != nil {
		return err
	}
	tbl, ok := reviewTables[kind]
	if !ok {
		return ErrUnknownKind
	}
	_, err = s.pool.Exec(ctx, `UPDATE xchats.`+tbl+
		` SET review_state = $3, updated_at = now() WHERE snapshot_id = $1 AND id = $2`, id, rowID, state)
	return err
}

var reviewTables = map[string]string{
	"topics": "ai_topics",
	"assets": "ai_assets",
	"values": "ai_values",
}

func orDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}
