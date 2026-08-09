package httpapi

import (
	"bytes"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// handleMetaMediaRead serves an OUTBOUND attachment's bytes to Meta's own
// fetch — the route a WhatsApp Cloud/Instagram/Messenger send hands Meta as
// a public "link" (see internal/whatsappcloud.ChannelSender.sendMedia and
// internal/inboxmedia's package doc for why this cannot be
// mcpauth.MediaReadTokenSigner). Authenticated purely by the signed query
// string — no session cookie, no CORS allowlist to satisfy, since the
// fetcher is Meta's own server, not a browser.
//
// Deliberately distinct from both GET /xchats/api/v1/media/:id (cookie
// authenticated, this org's logged-in users) and GET /mcp/media/:material_id
// (the KB widget's own signed route, mcpauth-scoped, never org-bound — see
// inboxmedia's package doc for exactly why THIS route needs an org-bound
// signature and that one does not).
func (s *Server) handleMetaMediaRead(c *gin.Context) {
	mediaID, err := uuid.Parse(c.Param("media_id"))
	if err != nil {
		fail(c, http.StatusBadRequest, ErrValidation, "invalid media id")
		return
	}
	exp, err := strconv.ParseInt(c.Query("exp"), 10, 64)
	token := c.Query("token")
	if err != nil || token == "" {
		fail(c, http.StatusForbidden, ErrUnauthorized, "invalid or expired media token")
		return
	}

	// Resolve the row's TRUE owning organization FIRST, independent of
	// anything in the request — see ChannelMediaByID's own doc comment for
	// why this ordering is the actual security boundary, not the signature
	// check below on its own.
	storageKey, mimetype, fileName, orgID, err := s.store.ChannelMediaByID(ctx(c), mediaID)
	if err != nil {
		fail(c, http.StatusNotFound, ErrNotFound, "media not found")
		return
	}
	if !s.inboxSigner.Verify(orgID, mediaID, exp, token) {
		fail(c, http.StatusForbidden, ErrUnauthorized, "invalid or expired media token")
		return
	}
	if storageKey == "" {
		fail(c, http.StatusNotFound, ErrNotFound, "media not ready")
		return
	}

	data, meta, err := s.blob.Get(storageKey)
	if err != nil {
		fail(c, http.StatusInternalServerError, ErrInternal, "read failed")
		return
	}
	mt := mimetype
	if mt == "" {
		mt = meta.Mimetype
	}
	if mt == "" {
		mt = "application/octet-stream"
	}
	c.Header("Content-Type", mt)
	// nosniff + a locked-down CSP: this serves user-supplied bytes on this
	// install's own origin — see handleMCPMediaRead's identical reasoning.
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Content-Security-Policy", "default-src 'none'; sandbox")
	c.Header("Referrer-Policy", "no-referrer")
	c.Header("Content-Disposition", contentDisposition(mt, filenameOr(fileName, meta.FileName)))
	c.Header("Cache-Control", "private, max-age=600")
	http.ServeContent(c.Writer, c.Request, "", time.Time{}, bytes.NewReader(data))
}
