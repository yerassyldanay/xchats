package kbstore

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/internal/dbx"
)

// ---------------------------------------------------------------------------
// Materials (Stage-1 ingest staging: the NormalizedMaterial contract) — the
// kbd_ group (was ai_materials). Playground-only; keyed directly on the org.
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

	// The fields below belong to the newer upload-material lineage
	// (CreateUploadMaterial / mcp_upload.go's two-phase PUT flow) — populated
	// once a file actually lands, zero-valued for materials still on the
	// legacy text/url path above (BlobID/ExtractedText/etc.).
	Filename           string `json:"filename"`
	MimeType           string `json:"mime_type"`
	SizeBytes          int64  `json:"size_bytes"`
	ProcessingStatus   string `json:"processing_status"`
	CustomerVisibility string `json:"customer_visibility"`
	VisualSummary      string `json:"visual_summary"`
	TranscriptText     string `json:"transcript_text"`
	OperatorNote       string `json:"operator_note"`
	// HasContent reports whether bytes are actually retrievable (storage_key
	// set — the same test GetUploadMaterial/MaterialPreviews use). The
	// storage_key itself is never exposed here: it is an internal blob-store
	// locator, not a stable public identifier.
	HasContent bool `json:"has_content"`
}

// MaterialInput creates one kbd_materials row.
type MaterialInput struct {
	SourceType string
	SourceRef  string
	BlobID     string
	MediaKind  string
	// Text is the pass-through content for source_type='text' (status starts ready).
	Text string
	// Description is the operator's comment, staged alongside the file/link —
	// "what it is and when to send it". For a type with no independent way to read
	// its own content (image/pdf/doc/video/audio — captioning needs a vision client
	// this deployment may not have), a non-empty Description IS the content: the
	// material is born ready immediately, no extraction job, no describe_file
	// popup. For url (which has a real, usually-richer fetch+extract path) it is
	// instead kept as a fallback: extraction still runs, and only a failed fetch
	// falls back to this comment instead of needs_human.
	Description string
}

// CreateMaterial stages one input for the org. A text material is born ready; a
// described file/media material is also born ready (the description substitutes
// for auto-extraction); everything else is pending until an extraction job fills
// it in (see MaterialInput.Description for the url/comment-as-fallback case).
func (s *Store) CreateMaterial(ctx context.Context, orgID uuid.UUID, in MaterialInput) (Material, error) {
	status := "pending"
	text := in.Text
	extraction := "{}"
	desc := strings.TrimSpace(in.Description)
	switch {
	case in.SourceType == "text":
		status = "ready"
	case in.SourceType == "url":
		if desc != "" {
			extraction = operatorCommentJSON(desc)
		}
	case desc != "":
		status = "ready"
		text = desc
		extraction = `{"method":"operator"}`
	}
	var m Material
	err := s.db.QueryRow(ctx, `INSERT INTO kbd_materials
		(organization_id, source_type, source_ref, blob_id, extracted_text, media_kind, status, extraction)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		RETURNING id, source_type, source_ref, blob_id, extracted_text, media_kind, status, extraction, created_at, updated_at`,
		orgID, in.SourceType, in.SourceRef, in.BlobID, text, in.MediaKind, status, extraction).
		Scan(&m.ID, &m.SourceType, &m.SourceRef, &m.BlobID, &m.ExtractedText, &m.MediaKind, &m.Status, &m.Extraction, &m.CreatedAt, &m.UpdatedAt)
	return m, err
}

// operatorCommentJSON builds the `{"operator_comment": "..."}` extraction blob a
// url material carries until extraction resolves it — encoding/json.Marshal so an
// operator's free text (quotes, backslashes, unicode) is always valid jsonb.
func operatorCommentJSON(comment string) string {
	out, err := json.Marshal(map[string]string{"operator_comment": comment})
	if err != nil {
		return "{}"
	}
	return string(out)
}

// OperatorComment reads back the `operator_comment` an extractor left as a
// fallback (see MaterialInput.Description), or "" if none was stored.
func OperatorComment(extractionJSON string) string {
	var m map[string]string
	if extractionJSON == "" {
		return ""
	}
	_ = json.Unmarshal([]byte(extractionJSON), &m)
	return m["operator_comment"]
}

// GetMaterial returns one material by id.
func (s *Store) GetMaterial(ctx context.Context, id uuid.UUID) (Material, error) {
	var m Material
	err := s.db.QueryRow(ctx, `SELECT id, source_type, source_ref, blob_id, extracted_text, media_kind, status, extraction, created_at, updated_at
		FROM kbd_materials WHERE id = $1`, id).
		Scan(&m.ID, &m.SourceType, &m.SourceRef, &m.BlobID, &m.ExtractedText, &m.MediaKind, &m.Status, &m.Extraction, &m.CreatedAt, &m.UpdatedAt)
	if errors.Is(err, dbx.ErrNoRows) {
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
	_, err := s.db.Exec(ctx, `UPDATE kbd_materials SET
		status = $2, extracted_text = $3, media_kind = COALESCE(NULLIF($4,''), media_kind),
		extraction = $5, updated_at = strftime('%Y-%m-%d %H:%M:%f','now') WHERE id = $1`,
		id, ex.Status, ex.ExtractedText, ex.MediaKind, orDefault(ex.Extraction, "{}"))
	return err
}

// ReadyMaterials returns the org's extraction-ready materials (synthesis input).
func (s *Store) ReadyMaterials(ctx context.Context, orgID uuid.UUID) ([]Material, error) {
	all, err := s.listMaterials(ctx, orgID)
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

// MarkMaterialsBuilt flips the given materials to status 'built' so a later build
// turn does not re-synthesize them (ReadyMaterials only returns 'ready' rows). This
// is what makes RunTurn idempotent: re-running the same instruction is a no-op once
// its materials are consumed.
func (s *Store) MarkMaterialsBuilt(ctx context.Context, ids []uuid.UUID) error {
	if len(ids) == 0 {
		return nil
	}
	_, err := s.db.Exec(ctx, `UPDATE kbd_materials
		SET status = 'built', updated_at = strftime('%Y-%m-%d %H:%M:%f','now')
		WHERE id IN (SELECT value FROM json_each($1)) AND status = 'ready'`, dbx.UUIDArray(ids))
	return err
}

// ListLiveMaterials returns the org's kbd_materials rows for GET
// /kb/materials — kept as its own route for a caller that wants JUST the
// materials table without the rest of DraftView, even though GET /kb
// (LiveView, below) now carries the exact same rows inline (see liveView's
// own doc comment on why: materials have no draft/live split at all, so
// there is nothing Playground-specific about them).
func (s *Store) ListLiveMaterials(ctx context.Context, orgID uuid.UUID) ([]Material, error) {
	materials, err := s.listMaterials(ctx, orgID)
	if err != nil {
		return nil, err
	}
	if materials == nil {
		materials = []Material{}
	}
	return materials, nil
}

// listMaterials is the s.db-bound convenience wrapper every non-transactional
// caller (ListLiveMaterials, Draft()) uses. liveView needs the tx-parameterized
// listMaterialsTx instead — see that function's own doc comment.
func (s *Store) listMaterials(ctx context.Context, orgID uuid.UUID) ([]Material, error) {
	return listMaterialsTx(ctx, s.db, orgID)
}

// listMaterialsTx is listMaterials' db-parameterized core (see identityIndex's
// doc comment, mcp_read.go, for why a caller already inside
// writeDraftBlobVersioned's locked transaction must pass that same tx here
// instead of letting this reach for s.db — liveView is reachable from
// exactly that context via identityIndex, so it cannot call the s.db-bound
// listMaterials wrapper above).
func listMaterialsTx(ctx context.Context, db dbtx, orgID uuid.UUID) ([]Material, error) {
	rows, err := db.Query(ctx, `SELECT id, source_type, source_ref, blob_id, extracted_text, media_kind, status, extraction, created_at, updated_at,
		filename, mime_type, size_bytes, processing_status, customer_visibility, visual_summary, transcript_text, operator_note,
		CASE WHEN storage_key IS NOT NULL AND storage_key <> '' THEN 1 ELSE 0 END
		FROM kbd_materials WHERE organization_id = $1 ORDER BY created_at`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Material
	for rows.Next() {
		var m Material
		var filename, mimeType, customerVisibility, visualSummary, transcriptText, operatorNote *string
		var sizeBytes *int64
		var hasContent int
		if err := rows.Scan(&m.ID, &m.SourceType, &m.SourceRef, &m.BlobID, &m.ExtractedText, &m.MediaKind, &m.Status, &m.Extraction, &m.CreatedAt, &m.UpdatedAt,
			&filename, &mimeType, &sizeBytes, &m.ProcessingStatus, &customerVisibility, &visualSummary, &transcriptText, &operatorNote, &hasContent); err != nil {
			return nil, err
		}
		m.Filename, m.MimeType = strOrEmpty(filename), strOrEmpty(mimeType)
		m.CustomerVisibility = strOrEmpty(customerVisibility)
		m.VisualSummary, m.TranscriptText, m.OperatorNote = strOrEmpty(visualSummary), strOrEmpty(transcriptText), strOrEmpty(operatorNote)
		if sizeBytes != nil {
			m.SizeBytes = *sizeBytes
		}
		m.HasContent = hasContent == 1
		out = append(out, m)
	}
	return out, rows.Err()
}

// MaterialContentRef locates one org-scoped material's stored bytes for the
// session-authenticated content endpoint (GET /kb/materials/:id/content).
// Unlike GetUploadMaterial (used by the signed, per-material MCP media
// token, which is itself the authorization), this filters by organization_id
// in SQL: a logged-in user can guess any UUID, so absence and cross-org
// mismatch must be indistinguishable — both simply return ErrUnknownKind.
type MaterialContentRef struct {
	StorageKey string
	MimeType   string
	Filename   string
	Checksum   string
}

func (s *Store) GetMaterialContentRef(ctx context.Context, orgID, id uuid.UUID) (MaterialContentRef, error) {
	var ref MaterialContentRef
	var storageKey, mimeType, filename, checksum *string
	err := s.db.QueryRow(ctx, `SELECT storage_key, mime_type, filename, sha256_checksum
		FROM kbd_materials WHERE id = $1 AND organization_id = $2`, id, orgID).
		Scan(&storageKey, &mimeType, &filename, &checksum)
	if errors.Is(err, dbx.ErrNoRows) {
		return MaterialContentRef{}, ErrUnknownKind
	}
	if err != nil {
		return MaterialContentRef{}, err
	}
	ref.StorageKey, ref.MimeType, ref.Filename, ref.Checksum = strOrEmpty(storageKey), strOrEmpty(mimeType), strOrEmpty(filename), strOrEmpty(checksum)
	return ref, nil
}

// ---------------------------------------------------------------------------
// Builder requests (the popup / human-in-the-loop queue) — kbd_requests (was
// ai_builder_requests). Playground-only; keyed directly on the org.
// ---------------------------------------------------------------------------

// Request is one popup for the org.
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

// CreateRequest raises a popup for the org.
func (s *Store) CreateRequest(ctx context.Context, orgID uuid.UUID, in RequestInput) (Request, error) {
	var r Request
	err := s.db.QueryRow(ctx, `INSERT INTO kbd_requests
		(organization_id, material_id, req_type, prompt, context, target)
		VALUES ($1,$2,$3,$4,$5,$6)
		RETURNING id, material_id, req_type, prompt, context, target, state, resolution, created_at, resolved_at`,
		orgID, in.MaterialID, in.ReqType, in.Prompt, orDefault(in.Context, "{}"), orDefault(in.Target, "{}")).
		Scan(&r.ID, &r.MaterialID, &r.ReqType, &r.Prompt, &r.Context, &r.Target, &r.State, &r.Resolution, &r.CreatedAt, &r.ResolvedAt)
	return r, err
}

// GetRequest returns one popup by id.
func (s *Store) GetRequest(ctx context.Context, id uuid.UUID) (Request, error) {
	var r Request
	err := s.db.QueryRow(ctx, `SELECT id, material_id, req_type, prompt, context, target, state, resolution, created_at, resolved_at
		FROM kbd_requests WHERE id = $1`, id).
		Scan(&r.ID, &r.MaterialID, &r.ReqType, &r.Prompt, &r.Context, &r.Target, &r.State, &r.Resolution, &r.CreatedAt, &r.ResolvedAt)
	if errors.Is(err, dbx.ErrNoRows) {
		return r, ErrUnknownKind
	}
	return r, err
}

// ResolveRequest marks a popup resolved (or dismissed) with the operator's answer.
// The caller applies the resulting draft mutation separately.
func (s *Store) ResolveRequest(ctx context.Context, id uuid.UUID, state, resolution string) error {
	_, err := s.db.Exec(ctx, `UPDATE kbd_requests SET
		state = $2, resolution = $3, resolved_at = strftime('%Y-%m-%d %H:%M:%f','now') WHERE id = $1`,
		id, orDefault(state, "resolved"), orDefault(resolution, "{}"))
	return err
}

func (s *Store) listRequests(ctx context.Context, orgID uuid.UUID) ([]Request, error) {
	rows, err := s.db.Query(ctx, `SELECT id, material_id, req_type, prompt, context, target, state, resolution, created_at, resolved_at
		FROM kbd_requests WHERE organization_id = $1 ORDER BY created_at`, orgID)
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
