package kbstore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// mediaColumnKind classifies every canonical media column (plan/DECISIONS.md
// "Concrete media-column naming") by broad MIME category — the closed v1
// list, across all seven KB types. Used both to validate an MCP upsert's
// media references and to hint kb_media_upload's accept filter for a given
// `target.field`.
var mediaColumnKind = map[string]string{
	"featured_image":            "image",
	"gallery_images":            "image",
	"pricing_images":            "image",
	"illustration_images":       "image",
	"contact_card_image":        "image",
	"location_map_image":        "image",
	"demo_videos":               "video",
	"explainer_videos":          "video",
	"narration_audio_files":     "audio",
	"audio_description_files":   "audio",
	"reference_documents":       "document",
	"certificate_documents":     "document",
	"manual_documents":          "document",
	"guarantee_documents":       "document",
	"specification_documents":   "document",
	"terms_documents":           "document",
	"company_legal_documents":   "document",
	"commerce_policy_documents": "document",
}

// MediaFieldKind returns the broad category ("image"|"video"|"audio"|
// "document") a canonical media column expects, or "" if field is not a
// recognized media column.
func MediaFieldKind(field string) string { return mediaColumnKind[field] }

func mimeMatchesKind(mime, kind string) bool {
	switch kind {
	case "image":
		return strings.HasPrefix(mime, "image/")
	case "video":
		return strings.HasPrefix(mime, "video/")
	case "audio":
		return strings.HasPrefix(mime, "audio/")
	case "document":
		return strings.HasPrefix(mime, "application/") || strings.HasPrefix(mime, "text/")
	default:
		return false
	}
}

// materialRef is the narrow projection of a kbd_materials row every media
// validation needs.
type materialRef struct {
	OrganizationID     uuid.UUID
	MimeType           string
	StorageKey         string
	CustomerVisibility string
}

// lookupMaterialRef takes db (not s.pool): every caller runs inside
// writeDraftBlobVersioned's already-locked transaction (see
// identityIndex's doc comment in mcp_read.go for the pool-exhaustion
// deadlock this avoids — c8c0774 fixed that class for identityIndex but
// left this exact same pattern here, still calling s.pool from inside a
// held row lock, until this fix).
func lookupMaterialRef(ctx context.Context, db dbtx, id uuid.UUID) (materialRef, bool, error) {
	var m materialRef
	var mime, storageKey, vis *string
	err := db.QueryRow(ctx, `SELECT organization_id, mime_type, storage_key, customer_visibility
		FROM xchats.kbd_materials WHERE id = $1`, id).
		Scan(&m.OrganizationID, &mime, &storageKey, &vis)
	if errors.Is(err, pgx.ErrNoRows) {
		return materialRef{}, false, nil
	}
	if err != nil {
		return materialRef{}, false, err
	}
	m.MimeType, m.StorageKey, m.CustomerVisibility = strOrEmpty(mime), strOrEmpty(storageKey), strOrEmpty(vis)
	return m, true, nil
}

// validateMediaRef checks that materialID may be written into KB media
// column field for orgID (plan/mcp.md §9: "same-organization media
// references"): it must exist, belong to this org, have completed upload
// (a non-empty storage_key — kb_media_upload's PUT target fills this in),
// not be marked customer_visibility='invisible', and its MIME type must
// match the column's declared category.
func validateMediaRef(ctx context.Context, db dbtx, orgID uuid.UUID, field string, materialID uuid.UUID) error {
	kind, ok := mediaColumnKind[field]
	if !ok {
		return &ErrMediaReference{MaterialID: materialID, Field: field, Reason: "not a recognized media field"}
	}
	m, found, err := lookupMaterialRef(ctx, db, materialID)
	if err != nil {
		return err
	}
	if !found {
		return &ErrMediaReference{MaterialID: materialID, Field: field, Reason: "material not found"}
	}
	if m.OrganizationID != orgID {
		return &ErrMediaReference{MaterialID: materialID, Field: field, Reason: "belongs to a different organization"}
	}
	if m.StorageKey == "" {
		return &ErrMediaReference{MaterialID: materialID, Field: field, Reason: "upload has not completed"}
	}
	if m.CustomerVisibility == "invisible" {
		return &ErrMediaReference{MaterialID: materialID, Field: field, Reason: "marked invisible — cannot attach to a KB field"}
	}
	if m.MimeType != "" && !mimeMatchesKind(m.MimeType, kind) {
		return &ErrMediaReference{MaterialID: materialID, Field: field,
			Reason: fmt.Sprintf("mime type %q is not a %s", m.MimeType, kind)}
	}
	return nil
}

// validateMediaRefs validates a batch of {field: [material ids]} — every
// MCPUpsert* method's media fields, checked before the draft write commits,
// on the SAME dbtx that method's writeDraftBlobVersioned closure holds the
// kbd_draft row lock on.
func validateMediaRefs(ctx context.Context, db dbtx, orgID uuid.UUID, refs map[string][]uuid.UUID) error {
	for field, ids := range refs {
		for _, id := range ids {
			if err := validateMediaRef(ctx, db, orgID, field, id); err != nil {
				return err
			}
		}
	}
	return nil
}

// singularRef packs an optional single media reference into the same
// map-of-slices shape validateMediaRefs expects.
func singularRef(field string, id *uuid.UUID) map[string][]uuid.UUID {
	if id == nil {
		return nil
	}
	return map[string][]uuid.UUID{field: {*id}}
}

// mergeRefs combines several {field: [ids]} maps into one, for a single
// validateMediaRefs call per upsert.
func mergeRefs(maps ...map[string][]uuid.UUID) map[string][]uuid.UUID {
	out := map[string][]uuid.UUID{}
	for _, m := range maps {
		for k, v := range m {
			if len(v) > 0 {
				out[k] = append(out[k], v...)
			}
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// kb_media_upload — the pending kbd_materials row + its later completion.
// ---------------------------------------------------------------------------

// UploadMaterialInput is what kb_media_upload stages before any bytes arrive
// (plan/mcp.md §5's kb_media_upload: "creates a pending kbd_materials row").
type UploadMaterialInput struct {
	Filename           string
	MimeType           string
	SizeBytes          int64
	SHA256Checksum     string
	CustomerVisibility string // "visible" when a target field was given, else "auto"
}

// CreateUploadMaterial stages a pending, canonical-shape kbd_materials row
// (source_type='file' — the DECISIONS.md canonical value; the legacy
// Playground extraction pipeline never sees these rows, so there is no
// vocabulary collision to worry about) with processing_status='uploaded':
// bytes have not arrived yet.
func (s *Store) CreateUploadMaterial(ctx context.Context, orgID uuid.UUID, in UploadMaterialInput) (uuid.UUID, error) {
	var id uuid.UUID
	err := s.pool.QueryRow(ctx, `INSERT INTO xchats.kbd_materials
		(organization_id, source_type, filename, mime_type, size_bytes, sha256_checksum,
		 processing_status, customer_visibility)
		VALUES ($1,'file',$2,$3,$4,$5,'uploaded',$6)
		RETURNING id`,
		orgID, in.Filename, in.MimeType, in.SizeBytes, nullIfEmpty(in.SHA256Checksum),
		orDefault(in.CustomerVisibility, "auto")).
		Scan(&id)
	return id, err
}

// UploadMaterial is the signed-upload handler's view of a staged material.
type UploadMaterial struct {
	ID                uuid.UUID
	OrganizationID    uuid.UUID
	Filename          string
	MimeType          string
	DeclaredSizeBytes int64
	DeclaredChecksum  string
	ProcessingStatus  string
}

// GetUploadMaterial reads back a staged material for the signed PUT handler
// to check the incoming bytes against (declared size/mime/checksum, org
// ownership).
func (s *Store) GetUploadMaterial(ctx context.Context, id uuid.UUID) (UploadMaterial, error) {
	var m UploadMaterial
	var filename, mimeType, checksum *string
	var sizeBytes *int64
	err := s.pool.QueryRow(ctx, `SELECT id, organization_id, filename, mime_type, size_bytes,
		sha256_checksum, processing_status
		FROM xchats.kbd_materials WHERE id = $1`, id).
		Scan(&m.ID, &m.OrganizationID, &filename, &mimeType, &sizeBytes, &checksum, &m.ProcessingStatus)
	if errors.Is(err, pgx.ErrNoRows) {
		return UploadMaterial{}, ErrUnknownKind
	}
	if err != nil {
		return UploadMaterial{}, err
	}
	m.Filename, m.MimeType, m.DeclaredChecksum = strOrEmpty(filename), strOrEmpty(mimeType), strOrEmpty(checksum)
	if sizeBytes != nil {
		m.DeclaredSizeBytes = *sizeBytes
	}
	return m, nil
}

// ErrUploadAlreadyCompleted means the material named by kb_media_upload's
// signed PUT target already has bytes attached (processing_status is no
// longer 'uploaded'). A signed upload URL stays individually valid for its
// whole TTL, not single-use, so without this guard a second PUT to the same
// URL — replay, retry-after-success, or a deliberate swap attempt — could
// silently replace bytes already attached to a live/draft KB record.
var ErrUploadAlreadyCompleted = errors.New("kbstore: material upload already completed")

// CompleteMaterialUpload finalizes a material once its bytes are stored:
// records the real storage locator/size/checksum and flips
// processing_status to 'parsed' — ready to attach. A directly uploaded MCP
// media file needs no AI extraction pass (unlike the Playground pipeline),
// so 'parsed' is its terminal ready state.
//
// The WHERE clause makes this a one-time, atomic pending→completed
// transition: it only ever affects a row still in the initial 'uploaded'
// staging state, so two concurrent PUTs to the same signed target can never
// both succeed — the loser's write affects zero rows and gets
// ErrUploadAlreadyCompleted, never overwriting the winner's already-recorded
// storage_key.
func (s *Store) CompleteMaterialUpload(ctx context.Context, id uuid.UUID, storageBackend, storageKey string, sizeBytes int64, sha256Checksum string) error {
	tag, err := s.pool.Exec(ctx, `UPDATE xchats.kbd_materials SET
		storage_backend = $2, storage_key = $3, size_bytes = $4, sha256_checksum = $5,
		processing_status = 'parsed', updated_at = now()
		WHERE id = $1 AND processing_status = 'uploaded'`, id, storageBackend, storageKey, sizeBytes, nullIfEmpty(sha256Checksum))
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrUploadAlreadyCompleted
	}
	return nil
}

func nullIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// ---------------------------------------------------------------------------
// Provenance — plan/mcp.md's "Common write behavior" provenance{source_url?,
// material_ids?} argument, recorded in kbd_materials, NEVER as a field on the
// draft entry itself.
//
// The draft document's shape is fixed by plan/DECISIONS.md: "Entries contain
// business columns only: no database id, organization id, or timestamps."
// An earlier version of this file added a Provenance string straight onto
// every Draft* struct instead, and every MCP upsert wrote plan/mcp.md's
// provenance argument into it — but no live ai_* table has ever had a
// provenance column (migration 0004_kb_living.up.sql: "live tables hold LIVE
// ROWS ONLY — no review_state, no provenance, no drafted_at"), so that value
// was silently discarded the moment a draft entry was approved into live.
// The legacy manual-editor and confirm_fact write paths had the exact same
// gap for exactly as long — this was never MCP-specific, just newly
// EXERCISED by MCP's tool contract explicitly promising callers a durable
// audit/authoring trail.
//
// This is the fix: nothing is added to the draft/live business-column shape.
// Instead, a source_url becomes its own kbd_materials row (source_type=
// 'url'), and every material_id (already an existing kbd_materials row) gets
// tagged — both via extraction_metadata.mcp_target, a plain JSON
// association rather than a dedicated table (out of scope for V1; revisit
// only if provenance ever needs reliable querying or multiple targets per
// material).
// ---------------------------------------------------------------------------

// MCPProvenance is plan/mcp.md's provenance{source_url?, material_ids?}
// argument, parsed (internal/mcpserver/args.go) into a typed value instead
// of the opaque pre-built JSON string this used to be.
type MCPProvenance struct {
	SourceURL   string
	MaterialIDs []uuid.UUID
}

func (p MCPProvenance) empty() bool { return p.SourceURL == "" && len(p.MaterialIDs) == 0 }

// recordProvenance implements the association described above, called from
// inside an MCPUpsert*'s writeDraftBlobVersioned closure — after the
// record's key is resolved, so the association names the actual key the
// content landed under — on that SAME already-locked db, never s.pool (the
// pool-exhaustion deadlock identityIndex's doc comment describes, mcp_read.go,
// applies here exactly the same way it did to media validation before c8c0774
// and this file's own dbtx threading fixed it).
//
// A material_id that does not exist or belongs to a different organization
// is rejected with ErrMediaReference — the same class of error, and the
// same same-organization discipline, validateMediaRef already enforces for
// an attachment reference (mcp_media.go); a provenance citation gets no
// looser a check than an actual attachment just because it is "only" an
// audit trail — silently accepting a foreign org's material id would let
// one organization probe/tag another's rows.
func (s *Store) recordProvenance(ctx context.Context, db dbtx, orgID uuid.UUID, kbType, key string, prov MCPProvenance) error {
	if prov.empty() {
		return nil
	}
	target, err := json.Marshal(map[string]any{
		"mcp_target": map[string]string{"type": kbType, "key": key},
	})
	if err != nil {
		return err
	}
	if prov.SourceURL != "" {
		if _, err := db.Exec(ctx, `INSERT INTO xchats.kbd_materials
			(organization_id, source_type, source_ref, extraction_metadata, processing_status, customer_visibility)
			VALUES ($1, 'url', $2, $3::jsonb, 'parsed', 'invisible')`,
			orgID, prov.SourceURL, string(target)); err != nil {
			return fmt.Errorf("record source_url provenance: %w", err)
		}
	}
	for _, id := range prov.MaterialIDs {
		tag, err := db.Exec(ctx, `UPDATE xchats.kbd_materials
			SET extraction_metadata = extraction_metadata || $3::jsonb
			WHERE id = $1 AND organization_id = $2`,
			id, orgID, string(target))
		if err != nil {
			return fmt.Errorf("tag material_id %s with provenance: %w", id, err)
		}
		if tag.RowsAffected() == 0 {
			return &ErrMediaReference{MaterialID: id, Field: "provenance.material_ids", Reason: "not found or belongs to a different organization"}
		}
	}
	return nil
}
