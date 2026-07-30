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

// handleKBMediaUpload implements kb_media_upload (plan/mcp.md §5): stages a
// pending kbd_materials row and returns a short-lived signed PUT target the
// widget uploads bytes to directly — never through this JSON tool-call
// payload. The actual signing secret and base URL live in
// internal/httpapi (Deps.SignUpload/UploadBaseURL), so this package never
// holds them.
func (s *Server) handleKBMediaUpload(ctx context.Context, orgID uuid.UUID, args map[string]json.RawMessage) (map[string]any, error) {
	filename := stringField(args, "filename")
	mimeType := stringField(args, "mime_type")
	sizeBytes := int64(intField(args, "size_bytes"))
	checksum := stringField(args, "sha256_checksum")
	if filename == "" || mimeType == "" || sizeBytes <= 0 {
		return nil, fmt.Errorf("filename, mime_type, and a positive size_bytes are required")
	}

	visibility := "auto"
	if target, terr := rawObject(args["target"]); terr == nil && len(target) > 0 {
		field := stringField(target, "field")
		if kind := kbstore.MediaFieldKind(field); kind != "" {
			visibility = "visible" // an explicit target field means this upload is meant for a live/draft KB media column
			if !mimeRoughlyMatches(mimeType, kind) {
				return toolError(fmt.Sprintf("mime_type %q does not look like a %s, which target field %q expects", mimeType, kind, field)), nil
			}
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
		strings.TrimRight(s.Deps.UploadBaseURL, "/"), materialID, url.QueryEscape(token))

	structured := map[string]any{
		"material_id":       materialID.String(),
		"upload_url":        uploadURL,
		"upload_method":     "PUT",
		"upload_headers":    map[string]string{"Content-Type": mimeType},
		"expires_at":        expiresAt.UTC().Format(time.RFC3339),
		"max_size_bytes":    sizeBytes,
		"processing_status": "uploaded",
	}
	return toolResult(
		fmt.Sprintf("Upload target created (material_id=%s). PUT the file bytes to upload_url within %d seconds, then reference this material_id in a kb_*_upsert call.", materialID, ttl),
		structured, "media",
	), nil
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
