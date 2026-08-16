package httpapi

import (
	"bytes"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// handleMCPMediaRead serves the bytes of one already-uploaded KB material to
// the KB Manager widget, authenticated by the signed read token in the query
// string — never the OAuth bearer and never a session cookie, because this
// route is fetched by an <img> (or an explicit open) from an iframe on a host
// origin we cannot predict.
//
// The token authorises exactly one material, read-only, for a bounded TTL.
// There is no ambient authority: possession of the URL IS the authorisation,
// which is what makes mediaCORS's permissive policy safe. Organization scoping
// is structural — a URL is only ever minted while answering an authorised
// kb_read for that org's principal (see mcpserver.mediaIndexFor), so nobody
// can obtain a URL for a material they could not already read.
//
// Deliberately distinct from GET /xchats/api/v1/media/:id: that one is cookie
// authenticated, CORS-allowlisted, and resolves through the chat inbox view.
func (s *Server) handleMCPMediaRead(c *gin.Context) {
	if !s.mcpAuthEnabled() || s.mcpMediaSigner == nil {
		fail(c, http.StatusServiceUnavailable, ErrInternal, "MCP connector not configured")
		return
	}
	materialID, err := uuid.Parse(c.Param("material_id"))
	if err != nil {
		fail(c, http.StatusBadRequest, ErrValidation, "invalid material_id")
		return
	}
	token := c.Query("token")
	if token == "" || !s.mcpMediaSigner.Verify(materialID.String(), token) {
		fail(c, http.StatusForbidden, ErrUnauthorized, "invalid or expired media token")
		return
	}

	mat, err := s.kb.GetUploadMaterial(ctx(c), materialID)
	if err != nil {
		fail(c, http.StatusNotFound, ErrNotFound, "material not found")
		return
	}
	// An empty storage_key means the PUT never completed. 404 rather than a
	// 409/425: from the reader's point of view there is simply nothing here.
	if mat.StorageKey == "" {
		fail(c, http.StatusNotFound, ErrNotFound, "material has no stored bytes")
		return
	}

	// A material's bytes are immutable once CompleteMaterialUpload's guarded
	// transition has run, so the declared checksum is a perfect ETag. Answer
	// 304 BEFORE touching the blob — that is the one path that avoids reading
	// the whole file into memory (blob.Store has no streaming read yet), and
	// the widget rebuilds its DOM often enough for it to matter.
	if mat.DeclaredChecksum != "" {
		etag := `"` + mat.DeclaredChecksum + `"`
		c.Header("ETag", etag)
		if match := c.GetHeader("If-None-Match"); match != "" && strings.Contains(match, etag) {
			c.Status(http.StatusNotModified)
			return
		}
	}

	data, meta, err := s.blob.Get(mat.StorageKey)
	if err != nil {
		fail(c, http.StatusInternalServerError, ErrInternal, "read failed")
		return
	}

	mimeType := mat.MimeType
	if mimeType == "" {
		mimeType = meta.Mimetype
	}
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	c.Header("Content-Type", mimeType)
	// nosniff is mandatory, not hygiene: this serves user-supplied bytes from
	// the same origin that holds the app's session cookie, and blob.MimeSanityCheck
	// only rejects HTML-sniffing payloads when text/* was NOT declared — so a
	// text/plain file whose contents look like a web page can legitimately be
	// stored. The next two headers are defence in depth on the response itself.
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Content-Security-Policy", "default-src 'none'; sandbox")
	c.Header("Referrer-Policy", "no-referrer")
	c.Header("Content-Disposition", contentDisposition(mimeType, filenameOr(mat.Filename, meta.FileName)))
	c.Header("Cache-Control", "private, max-age=600, immutable")

	// ServeContent rather than c.Data: the widget never embeds a player, but a
	// file the user explicitly opens is handed to the browser's own viewer,
	// which does issue Range requests.
	//
	// Range does NOT bound server memory — blob.Store.Get already returned the
	// whole file. Adding a streaming OpenReader to blob.Store is the real fix
	// and is tracked separately; what bounds exposure today is that the widget
	// auto-fetches images only.
	http.ServeContent(c.Writer, c.Request, "", time.Time{}, bytes.NewReader(data))
}

func filenameOr(primary, fallback string) string {
	if strings.TrimSpace(primary) != "" {
		return primary
	}
	return fallback
}

// contentDisposition renders images/video/audio inline and everything else as
// an attachment. The filename goes out RFC 5987-encoded because KB filenames
// are routinely Cyrillic, and a raw non-ASCII header value is invalid.
func contentDisposition(mimeType, filename string) string {
	kind := "attachment"
	switch {
	case strings.HasPrefix(mimeType, "image/"),
		strings.HasPrefix(mimeType, "video/"),
		strings.HasPrefix(mimeType, "audio/"):
		kind = "inline"
	}
	if strings.TrimSpace(filename) == "" {
		return kind
	}
	return kind + "; filename*=UTF-8''" + url.PathEscape(filename)
}
