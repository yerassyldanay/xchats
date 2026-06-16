package kbstore

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// ErrUnknownKind is returned for an unrecognized review/row kind.
var ErrUnknownKind = errors.New("kbstore: unknown row kind")

// ---------------------------------------------------------------------------
// Materials (Stage-1 ingest staging: the NormalizedMaterial contract)
// ---------------------------------------------------------------------------

// Material is one dropped input, driven toward extracted_text by an adapter.
type Material struct {
	ID            uuid.UUID `json:"id"`
	SourceType    string    `json:"source_type"`
	SourceRef     string    `json:"source_ref"`
	BlobID        string    `json:"blob_id"`
	ExtractedText string    `json:"extracted_text"`
	MediaKind     string    `json:"media_kind"`
	Status        string    `json:"status"`
	Extraction    string    `json:"extraction"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// MaterialInput creates one ai_materials row.
type MaterialInput struct {
	SourceType string
	SourceRef  string
	BlobID     string
	MediaKind  string
	// Text is the pass-through content for source_type='text' (status starts ready).
	Text string
}

// CreateMaterial stages one input on the open draft. A text material is born
// ready; everything else is pending until an extraction job fills it.
func (s *Store) CreateMaterial(ctx context.Context, orgID uuid.UUID, in MaterialInput) (Material, error) {
	id, err := s.draftID(ctx, orgID)
	if err != nil {
		return Material{}, err
	}
	status := "pending"
	if in.SourceType == "text" {
		status = "ready"
	}
	var m Material
	err = s.pool.QueryRow(ctx, `INSERT INTO xchats.ai_materials
		(snapshot_id, source_type, source_ref, blob_id, extracted_text, media_kind, status)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		RETURNING id, source_type, source_ref, blob_id, extracted_text, media_kind, status, extraction::text, created_at, updated_at`,
		id, in.SourceType, in.SourceRef, in.BlobID, in.Text, in.MediaKind, status).
		Scan(&m.ID, &m.SourceType, &m.SourceRef, &m.BlobID, &m.ExtractedText, &m.MediaKind, &m.Status, &m.Extraction, &m.CreatedAt, &m.UpdatedAt)
	return m, err
}

// GetMaterial returns one material by id.
func (s *Store) GetMaterial(ctx context.Context, id uuid.UUID) (Material, error) {
	var m Material
	err := s.pool.QueryRow(ctx, `SELECT id, source_type, source_ref, blob_id, extracted_text, media_kind, status, extraction::text, created_at, updated_at
		FROM xchats.ai_materials WHERE id = $1`, id).
		Scan(&m.ID, &m.SourceType, &m.SourceRef, &m.BlobID, &m.ExtractedText, &m.MediaKind, &m.Status, &m.Extraction, &m.CreatedAt, &m.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return m, ErrUnknownKind
	}
	return m, err
}

// MaterialExtraction is an adapter's result landing on a material.
type MaterialExtraction struct {
	Status        string // 'ready' | 'needs_human' | 'failed'
	ExtractedText string
	MediaKind     string
	Extraction    string // jsonb { method, model, confidence, error }
}

// UpdateMaterialExtraction records an adapter's output on a material.
func (s *Store) UpdateMaterialExtraction(ctx context.Context, id uuid.UUID, ex MaterialExtraction) error {
	_, err := s.pool.Exec(ctx, `UPDATE xchats.ai_materials SET
		status = $2, extracted_text = $3, media_kind = COALESCE(NULLIF($4,''), media_kind),
		extraction = $5::jsonb, updated_at = now() WHERE id = $1`,
		id, ex.Status, ex.ExtractedText, ex.MediaKind, orDefault(ex.Extraction, "{}"))
	return err
}

// ReadyMaterials returns the draft's extraction-ready materials (synthesis input).
func (s *Store) ReadyMaterials(ctx context.Context, orgID uuid.UUID) ([]Material, error) {
	id, err := s.draftID(ctx, orgID)
	if err != nil {
		return nil, err
	}
	all, err := s.listMaterials(ctx, id)
	if err != nil {
		return nil, err
	}
	var ready []Material
	for _, m := range all {
		if m.Status == "ready" {
			ready = append(ready, m)
		}
	}
	return ready, nil
}

func (s *Store) listMaterials(ctx context.Context, snapID uuid.UUID) ([]Material, error) {
	rows, err := s.pool.Query(ctx, `SELECT id, source_type, source_ref, blob_id, extracted_text, media_kind, status, extraction::text, created_at, updated_at
		FROM xchats.ai_materials WHERE snapshot_id = $1 ORDER BY created_at`, snapID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Material
	for rows.Next() {
		var m Material
		if err := rows.Scan(&m.ID, &m.SourceType, &m.SourceRef, &m.BlobID, &m.ExtractedText, &m.MediaKind, &m.Status, &m.Extraction, &m.CreatedAt, &m.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// ---------------------------------------------------------------------------
// Builder requests (the popup / human-in-the-loop queue)
// ---------------------------------------------------------------------------

// Request is one popup keyed to the draft.
type Request struct {
	ID         uuid.UUID  `json:"id"`
	MaterialID *uuid.UUID `json:"material_id"`
	ReqType    string     `json:"req_type"`
	Prompt     string     `json:"prompt"`
	Context    string     `json:"context"`
	Target     string     `json:"target"`
	State      string     `json:"state"`
	Resolution *string    `json:"resolution"`
	CreatedAt  time.Time  `json:"created_at"`
	ResolvedAt *time.Time `json:"resolved_at"`
}

// RequestInput creates one popup.
type RequestInput struct {
	MaterialID *uuid.UUID
	ReqType    string
	Prompt     string
	Context    string // jsonb
	Target     string // jsonb
}

// CreateRequest raises a popup on the open draft.
func (s *Store) CreateRequest(ctx context.Context, orgID uuid.UUID, in RequestInput) (Request, error) {
	id, err := s.draftID(ctx, orgID)
	if err != nil {
		return Request{}, err
	}
	var r Request
	err = s.pool.QueryRow(ctx, `INSERT INTO xchats.ai_builder_requests
		(snapshot_id, material_id, req_type, prompt, context, target)
		VALUES ($1,$2,$3,$4,$5::jsonb,$6::jsonb)
		RETURNING id, material_id, req_type, prompt, context::text, target::text, state, resolution::text, created_at, resolved_at`,
		id, in.MaterialID, in.ReqType, in.Prompt, orDefault(in.Context, "{}"), orDefault(in.Target, "{}")).
		Scan(&r.ID, &r.MaterialID, &r.ReqType, &r.Prompt, &r.Context, &r.Target, &r.State, &r.Resolution, &r.CreatedAt, &r.ResolvedAt)
	return r, err
}

// ResolveRequest marks a popup resolved (or dismissed) with the operator's answer.
// The caller applies the resulting draft mutation separately.
func (s *Store) ResolveRequest(ctx context.Context, id uuid.UUID, state, resolution string) error {
	_, err := s.pool.Exec(ctx, `UPDATE xchats.ai_builder_requests SET
		state = $2, resolution = $3::jsonb, resolved_at = now() WHERE id = $1`,
		id, orDefault(state, "resolved"), orDefault(resolution, "{}"))
	return err
}

func (s *Store) listRequests(ctx context.Context, snapID uuid.UUID) ([]Request, error) {
	rows, err := s.pool.Query(ctx, `SELECT id, material_id, req_type, prompt, context::text, target::text, state, resolution::text, created_at, resolved_at
		FROM xchats.ai_builder_requests WHERE snapshot_id = $1 ORDER BY created_at`, snapID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Request
	for rows.Next() {
		var r Request
		if err := rows.Scan(&r.ID, &r.MaterialID, &r.ReqType, &r.Prompt, &r.Context, &r.Target, &r.State, &r.Resolution, &r.CreatedAt, &r.ResolvedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
