package httpapi

import (
	"bytes"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/internal/kbstore"
)

// handleKBMaterialContent serves one org's kbd_materials bytes to the
// logged-in SPA — session-cookie authenticated, unlike GET /mcp/media/:id
// (a signed, single-material token for an unpredictable widget iframe
// origin) and unlike GET /xchats/api/v1/media/:id (chat-message media,
// resolved through message_media/blob metadata that KB uploads never
// populate — see mcp_upload.go). Plain <img>/<video>/<a> tags on
// /knowledge-base and /playground send the session cookie automatically
// (same-origin in both dev's Vite proxy and prod's nginx), so no token in
// the URL is needed here.
//
// Organization scoping happens in SQL (kbstore.GetMaterialContentRef), not
// as an app-side filter: a logged-in user can request any UUID, so a
// cross-org id and an unknown id must be indistinguishable — both are a
// plain 404.
func (s *Server) handleKBMaterialContent(c *gin.Context) {
	if !s.kbReady(c) {
		return
	}
	orgID, proceed := s.pgOrg(c)
	if !proceed {
		return
	}
	materialID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		fail(c, http.StatusBadRequest, ErrValidation, "invalid material id")
		return
	}

	ref, err := s.kb.GetMaterialContentRef(ctx(c), orgID, materialID)
	if errors.Is(err, kbstore.ErrUnknownKind) {
		fail(c, http.StatusNotFound, ErrNotFound, "material not found")
		return
	}
	if err != nil {
		fail(c, http.StatusInternalServerError, ErrInternal, "kb: "+err.Error())
		return
	}
	if ref.StorageKey == "" {
		fail(c, http.StatusNotFound, ErrNotFound, "material has no stored bytes")
		return
	}

	// The declared checksum is a stable ETag once bytes have landed (they are
	// immutable from that point on — see CompleteMaterialUpload). Answer 304
	// before touching the blob store, the same ordering handleMCPMediaRead
	// uses and for the same reason: blob.Store has no streaming read yet.
	if ref.Checksum != "" {
		etag := `"` + ref.Checksum + `"`
		c.Header("ETag", etag)
		if match := c.GetHeader("If-None-Match"); match != "" && strings.Contains(match, etag) {
			c.Status(http.StatusNotModified)
			return
		}
	}

	data, meta, err := s.blob.Get(ref.StorageKey)
	if err != nil {
		fail(c, http.StatusInternalServerError, ErrInternal, "read failed")
		return
	}

	mimeType := ref.MimeType
	if mimeType == "" {
		mimeType = meta.Mimetype
	}
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	c.Header("Content-Type", mimeType)
	// See handleMCPMediaRead's identical header block: nosniff and a locked
	// CSP are mandatory defence in depth for user-supplied bytes served from
	// the same origin as the app's session cookie, not optional hygiene.
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Content-Security-Policy", "default-src 'none'; sandbox")
	c.Header("Referrer-Policy", "no-referrer")
	c.Header("Content-Disposition", contentDisposition(mimeType, filenameOr(ref.Filename, meta.FileName)))
	c.Header("Cache-Control", "private, max-age=600, immutable")

	// ServeContent (not c.Data) so a <video> tag's Range requests work.
	http.ServeContent(c.Writer, c.Request, "", time.Time{}, bytes.NewReader(data))
}
