package httpapi

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/internal/blob"
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

// handleKBUploadMaterial is the browser's own upload avenue for KB media —
// session + org authenticated (kbWrite, kb_live.go — the same preamble
// every other /kb/* write uses; NOT admin-gated, matching the rest of this
// non-admin group), single-shot multipart (unlike kb_media_upload's
// two-phase MCP flow, whose signed-URL dance exists solely because the KB
// Manager widget is cross-origin — a same-origin session request has no
// such need). Internally still goes through CreateUploadMaterial ->
// blob.Put -> CompleteMaterialUpload, the exact same two-phase STORE calls
// the MCP path uses, so browser- and MCP-authored materials share one
// lineage end to end: has_content, MaterialPreviews, GetMaterialContentRef,
// and validateMediaRef's storage_key check all behave identically either
// way.
//
// Reusing PUT /mcp/uploads/:id (mcp_upload.go) instead was rejected: it is
// gated on s.mcpAuthEnabled()/s.mcpUploadSigner, so a deployment without the
// MCP connector configured would lose browser KB uploads too, and its
// uploadCORS is permissive-by-design specifically because that route never
// sees a session cookie — reusing it here would need a second, cookie-aware
// CORS carve-out for no real benefit.
func (s *Server) handleKBUploadMaterial(c *gin.Context) {
	orgID, proceed := s.kbWrite(c)
	if !proceed {
		return
	}

	// Must wrap the body before anything touches it — c.FormFile below is
	// what triggers the read. Mirrors handleUploadMedia's ordering
	// (inbox.go) and its own doc comment on why gin's MaxMultipartMemory
	// alone (a parser spill-to-disk threshold, not a body-size cap) isn't
	// enough. mcpUploadMaxBytes (mcp_upload.go) is mcpserver.MaxMediaUploadBytes
	// (50 MiB) — deliberately NOT maxUploadBytes (64 MiB, server.go's chat
	// media cap): "what MCP can upload, the browser can too" should mean the
	// same limit either way.
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, mcpUploadMaxBytes)
	fh, err := c.FormFile("file")
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			fail(c, http.StatusRequestEntityTooLarge, ErrValidation, "upload exceeds the maximum allowed size")
			return
		}
		fail(c, http.StatusBadRequest, ErrValidation, "file part required")
		return
	}
	f, err := fh.Open()
	if err != nil {
		fail(c, http.StatusBadRequest, ErrValidation, "cannot open file")
		return
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil {
		fail(c, http.StatusBadRequest, ErrValidation, "cannot read file")
		return
	}

	_, mimeType := detectMedia(fh.Filename, fh.Header.Get("Content-Type"))
	// Same HTML-masquerade / declared-vs-sniffed check the MCP upload path
	// runs (mcp_upload.go) and handleUploadMedia reuses for chat media —
	// the classic stored-XSS-via-mislabeled-upload vector applies here just
	// as much, since these bytes are later served back on this app's own
	// origin (handleKBMaterialContent above).
	if reason := mimeSanityCheck(mimeType, data); reason != "" {
		fail(c, http.StatusBadRequest, ErrValidation, reason)
		return
	}
	// A mime_type kbstore.KindOfMime cannot classify would be permanently
	// un-attachable — every validateMediaRef call rejects it as "not a
	// recognized media field" (mcp_media.go), so reject it now instead of
	// leaving an orphaned, never-attachable kbd_materials row behind.
	if kbstore.KindOfMime(mimeType) == "" {
		fail(c, http.StatusBadRequest, ErrValidation, "unsupported file type "+mimeType+" — must be an image, video, audio, or document")
		return
	}

	reqCtx := ctx(c)
	// customer_visibility: "visible", unlike kb_media_upload's "auto" default
	// when no target field is declared (mcp_media.go) — that branch exists
	// for an AI agent's exploratory upload with uncertain intent; a human
	// clicking this endpoint's dedicated upload button already has one
	// (attach it to the field the picker is editing, a later separate
	// draft-lane write — Stage 3), even though this endpoint itself takes no
	// target param.
	materialID, err := s.kb.CreateUploadMaterial(reqCtx, orgID, kbstore.UploadMaterialInput{
		Filename: fh.Filename, MimeType: mimeType, SizeBytes: int64(len(data)), CustomerVisibility: "visible",
	})
	if err != nil {
		s.kbFail(c, err)
		return
	}

	// The object key is derived from the CONTENT hash, exactly like
	// handleMCPUpload's storageKey — see that handler's comment on why two
	// different payloads can never collide on the same on-disk key.
	sum := sha256.Sum256(data)
	sumHex := hex.EncodeToString(sum[:])
	storageKey, err := s.blob.Put(materialID.String()+"-"+sumHex[:16], data, blob.Meta{
		Mimetype: mimeType, FileName: fh.Filename, FileSize: int64(len(data)),
	})
	if err != nil {
		fail(c, http.StatusInternalServerError, ErrInternal, "store failed")
		return
	}
	if err := s.kb.CompleteMaterialUpload(reqCtx, materialID, "disk", storageKey, int64(len(data)), sumHex); err != nil {
		fail(c, http.StatusInternalServerError, ErrInternal, "finalize failed")
		return
	}
	ok(c, gin.H{"material_id": materialID, "processing_status": "parsed"})
}
