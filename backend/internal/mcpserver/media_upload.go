package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/internal/kbstore"
)

// MaxMediaUploadBytes is the hard cap on a single kb_media_upload'd file —
// no legitimate KB media file needs more than this. Enforced HERE at
// staging time (a fast, clear tool error instead of letting the model/widget
// attempt a large PUT that only fails afterward) and again, independently,
// as a hard backstop on the actual byte transfer in
// internal/httpapi.handleMCPUpload — that second check is what actually
// matters for safety (a declared size is just a claim), this one is purely
// fail-fast UX.
const MaxMediaUploadBytes = 50 << 20 // 50 MiB

// handleKBMediaUpload implements kb_media_upload (plan/mcp.md §5): stages a
// pending kbd_materials row and returns a short-lived signed PUT target the
// widget uploads bytes to directly — never through this JSON tool-call
// payload. The actual signing secret and base URL live in
// internal/httpapi (Deps.SignUpload/UploadBaseURL), so this package never
// holds them.
func (s *Server) handleKBMediaUpload(ctx context.Context, orgID, userID uuid.UUID, args map[string]json.RawMessage) (map[string]any, error) {
	filename := stringField(args, "filename")
	mimeType := stringField(args, "mime_type")
	sizeBytes := int64(intField(args, "size_bytes"))
	checksum := stringField(args, "sha256_checksum")
	if filename == "" || mimeType == "" || sizeBytes <= 0 {
		return nil, fmt.Errorf("filename, mime_type, and a positive size_bytes are required")
	}
	if sizeBytes > MaxMediaUploadBytes {
		return toolError(fmt.Sprintf("size_bytes %d exceeds the %d byte (%dMiB) limit for a single KB media file", sizeBytes, int64(MaxMediaUploadBytes), MaxMediaUploadBytes>>20)), nil
	}

	visibility := "auto"
	if target, terr := rawObject(args["target"]); terr == nil && len(target) > 0 {
		targetType := stringField(target, "type")
		field := stringField(target, "field")
		// Validated against the SAME registry kb_media_attach enforces (not
		// just MediaFieldKind's flat field→kind lookup) — a target must name
		// a real attachment field ON THE NAMED TYPE, so e.g. {type:"topic",
		// field:"gallery_images"} (a product-only field) or
		// {type:"product", field:"featured_image"} (not an attachment
		// target — see mediaAttachmentFields' doc comment) is rejected here,
		// at staging time, rather than only discovered later when the
		// eventual kb_media_attach call fails.
		fieldDef, ok := kbstore.LookupMediaAttachmentField(targetType, field)
		if !ok {
			return toolError(fmt.Sprintf("target {type:%q, field:%q} is not a valid attachment target — call kb_info for the current media_attachment_fields", targetType, field)), nil
		}
		visibility = "visible" // an explicit, valid target means this upload is meant for a live/draft KB media column
		if !mimeRoughlyMatches(mimeType, fieldDef.Kind) {
			return toolError(fmt.Sprintf("mime_type %q does not look like a %s, which target field %q expects", mimeType, fieldDef.Kind, field)), nil
		}
	}

	materialID, err := s.Deps.KB.CreateUploadMaterial(ctx, orgID, kbstore.UploadMaterialInput{
		Filename: filename, MimeType: mimeType, SizeBytes: sizeBytes,
		SHA256Checksum: checksum, CustomerVisibility: visibility,
	})
	if err != nil {
		return nil, err
	}

	ttl := s.Deps.UploadTTLSeconds
	if ttl <= 0 {
		ttl = 900
	}
	expiresAt := time.Now().Add(time.Duration(ttl) * time.Second)
	token := ""
	if s.Deps.SignUpload != nil {
		token = s.Deps.SignUpload(materialID.String(), expiresAt.Unix())
	}
	uploadURL := fmt.Sprintf("%s/mcp/uploads/%s?token=%s",
		s.publicBaseURL(), materialID, url.QueryEscape(token))

	structured := map[string]any{
		"material_id":       materialID.String(),
		"upload_url":        uploadURL,
		"upload_method":     "PUT",
		"upload_headers":    map[string]string{"Content-Type": mimeType},
		"expires_at":        expiresAt.UTC().Format(time.RFC3339),
		"max_size_bytes":    sizeBytes,
		"processing_status": "uploaded",
	}
	return s.toolResult(
		fmt.Sprintf("Upload target created (material_id=%s), but no file bytes have been uploaded yet. PUT the bytes to upload_url within %d seconds; only a successful PUT completes the upload. Then attach this material_id to the target record.", materialID, ttl),
		structured, "media", orgID, userID,
	), nil
}

// mediaIndexFor builds kb_read's _meta["xchats/media"]: per-material display
// metadata plus a short-lived signed preview URL, keyed by material id.
//
// Costs exactly ZERO extra queries for a page that references no media, and
// one batch query otherwise — ReadRecords itself stays free of per-record
// database work. A failure here is logged and swallowed: missing previews are
// a degradation, but a failed kb_read is an outage.
func (s *Server) mediaIndexFor(ctx context.Context, orgID uuid.UUID, page kbstore.ReadPage) map[string]any {
	ids := kbstore.MediaIDsIn(page)
	if len(ids) == 0 {
		return nil
	}
	previews, err := s.Deps.KB.MaterialPreviews(ctx, orgID, ids)
	if err != nil {
		if s.Deps.Log != nil {
			s.Deps.Log.Warn("mcpserver: media previews failed", "err", err)
		}
		return nil
	}
	if len(previews) == 0 {
		return nil
	}
	ttl := s.Deps.MediaTTLSeconds
	if ttl <= 0 {
		ttl = 3600
	}
	expiresAt := time.Now().Add(time.Duration(ttl) * time.Second)
	out := make(map[string]any, len(previews))
	for id, p := range previews {
		entry := map[string]any{
			"id": p.ID.String(), "filename": p.Filename, "mime_type": p.MimeType,
			"size_bytes": p.SizeBytes, "kind": p.Kind, "status": p.Status,
		}
		if s.Deps.SignMediaRead != nil {
			// Built against UploadBaseURL, NOT a separate media host: the CSP
			// resourceDomains the host applies to this widget is derived from
			// exactly originOf(UploadBaseURL) — see widgetResourceMeta in
			// resources.go. A different origin here would silently break every
			// preview.
			token := s.Deps.SignMediaRead(id.String(), expiresAt.Unix())
			entry["url"] = fmt.Sprintf("%s/mcp/media/%s?token=%s",
				s.publicBaseURL(), id, url.QueryEscape(token))
			entry["expires_at"] = expiresAt.UTC().Format(time.RFC3339)
		}
		out[id.String()] = entry
	}
	return out
}

// handleKBMediaAttach implements kb_media_attach: attaches an
// already-uploaded material to one media field of an existing draft/live
// record. All domain validation (registry pairing, record existence and
// pending-deletion, material ownership/completeness/visibility/MIME) lives
// in kbstore.MCPAttachMedia — this handler only parses arguments and maps
// the outcome to the standard tool-result shape.
func (s *Server) handleKBMediaAttach(ctx context.Context, orgID, userID uuid.UUID, args map[string]json.RawMessage) (map[string]any, error) {
	materialIDRaw := stringField(args, "material_id")
	kbType := stringField(args, "type")
	key := stringField(args, "key")
	field := stringField(args, "field")
	if materialIDRaw == "" || kbType == "" || key == "" || field == "" {
		return nil, fmt.Errorf("material_id, type, key, and field are all required")
	}
	materialID, err := uuid.Parse(materialIDRaw)
	if err != nil {
		return nil, fmt.Errorf("material_id: invalid material_id %q: %w", materialIDRaw, err)
	}
	expected, err := optExpectedVersion(args)
	if err != nil {
		return nil, err
	}

	res, kerr := s.Deps.KB.MCPAttachMedia(ctx, orgID, userID, kbstore.MediaAttachInput{
		MaterialID: materialID, Type: kbType, Key: key, Field: field,
	}, expected)
	if kerr != nil {
		return mapKBError(kerr), nil
	}
	msg := fmt.Sprintf("Attached material %s to %s %s.%s (draft_version=%d).", res.MaterialID, res.Type, res.Key, res.Field, res.DraftVersion)
	if res.AlreadyPresent {
		msg = fmt.Sprintf("Material %s was already attached to %s %s.%s — no change (draft_version=%d).", res.MaterialID, res.Type, res.Key, res.Field, res.DraftVersion)
	}
	return s.toolResult(msg, res, "draft", orgID, userID), nil
}

// mimeRoughlyMatches mirrors kbstore's own (unexported) mimeMatchesKind —
// duplicated rather than exported: it is a two-line convenience check here
// (should the widget's declared mime_type even bother uploading?), while
// kbstore's copy is the actual security boundary re-checked against the
// SNIFFED bytes at PUT time (internal/httpapi's upload handler) and again at
// draft-write time (kbstore.validateMediaRef).
func mimeRoughlyMatches(mime, kind string) bool {
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
