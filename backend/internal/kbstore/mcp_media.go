package kbstore

import (
	"context"
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

func (s *Store) lookupMaterialRef(ctx context.Context, id uuid.UUID) (materialRef, bool, error) {
	var m materialRef
	var mime, storageKey, vis *string
	err := s.pool.QueryRow(ctx, `SELECT organization_id, mime_type, storage_key, customer_visibility
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
func (s *Store) validateMediaRef(ctx context.Context, orgID uuid.UUID, field string, materialID uuid.UUID) error {
	kind, ok := mediaColumnKind[field]
	if !ok {
		return &ErrMediaReference{MaterialID: materialID, Field: field, Reason: "not a recognized media field"}
	}
	m, found, err := s.lookupMaterialRef(ctx, materialID)
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
// MCPUpsert* method's media fields, checked before the draft write commits.
func (s *Store) validateMediaRefs(ctx context.Context, orgID uuid.UUID, refs map[string][]uuid.UUID) error {
	for field, ids := range refs {
		for _, id := range ids {
			if err := s.validateMediaRef(ctx, orgID, field, id); err != nil {
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

// CompleteMaterialUpload finalizes a material once its bytes are stored:
// records the real storage locator/size/checksum and flips
// processing_status to 'parsed' — ready to attach. A directly uploaded MCP
// media file needs no AI extraction pass (unlike the Playground pipeline),
// so 'parsed' is its terminal ready state.
func (s *Store) CompleteMaterialUpload(ctx context.Context, id uuid.UUID, storageBackend, storageKey string, sizeBytes int64, sha256Checksum string) error {
	_, err := s.pool.Exec(ctx, `UPDATE xchats.kbd_materials SET
		storage_backend = $2, storage_key = $3, size_bytes = $4, sha256_checksum = $5,
		processing_status = 'parsed', updated_at = now()
		WHERE id = $1`, id, storageBackend, storageKey, sizeBytes, nullIfEmpty(sha256Checksum))
	return err
}

func nullIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
