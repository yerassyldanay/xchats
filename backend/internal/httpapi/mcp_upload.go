package httpapi

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/internal/blob"
	"github.com/yerassyldanay/xchats/backend/internal/kbstore"
	"github.com/yerassyldanay/xchats/backend/internal/mcpserver"
)

// mcpUploadMaxBytes mirrors mcpserver.MaxMediaUploadBytes — the hard safety
// cap on the ACTUAL byte transfer, independent of the declared size_bytes.
// kb_media_upload (mcpserver.handleKBMediaUpload) already rejects a declared
// size over this same limit at staging time; this is the backstop that
// still applies even if that earlier check were ever bypassed.
const mcpUploadMaxBytes = mcpserver.MaxMediaUploadBytes

// uploadCORS allows any origin on the signed-upload route only: its
// authentication is the unguessable signed token in the query string, never
// a cookie, so there is no credentialed cross-origin request to protect
// against (unlike every other route, which stays on the configured
// CORSOrigins allowlist) — the KB Manager widget's iframe, hosted on
// chatgpt.com/claude.ai's own origin, must be able to PUT bytes here
// directly (plan/mcp.md §6: "The widget uploads bytes directly to object
// storage").
func (s *Server) uploadCORS() gin.HandlerFunc { return permissiveSignedCORS("PUT,OPTIONS") }

// mediaCORS is uploadCORS's read-direction twin, for GET /mcp/media/:id.
// Same justification verbatim: the only credential is the unguessable signed
// token in the query string, never a cookie, so there is no credentialed
// cross-origin request to protect and the widget's unpredictable sandbox
// origin can be echoed back safely.
func (s *Server) mediaCORS() gin.HandlerFunc { return permissiveSignedCORS("GET,OPTIONS") }

// permissiveSignedCORS echoes any Origin for a route whose authentication is
// a signed token rather than a cookie. Access-Control-Allow-Credentials is
// deliberately NEVER set: that is what keeps "echo any origin" from becoming
// an ambient-authority hole.
func permissiveSignedCORS(allowMethods string) gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin != "" {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Vary", "Origin")
		}
		if c.Request.Method == http.MethodOptions {
			c.Header("Access-Control-Allow-Methods", allowMethods)
			c.Header("Access-Control-Allow-Headers", "Content-Type")
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

// handleMCPUpload receives the bytes for a material kb_media_upload staged,
// authenticated by the signed token in the query string (never the OAuth
// bearer token — this route is reachable from an unauthenticated widget
// iframe). It re-verifies size, a MIME sanity check against the sniffed
// bytes, and checksum before marking the material ready to attach.
func (s *Server) handleMCPUpload(c *gin.Context) {
	if !s.mcpAuthEnabled() {
		fail(c, http.StatusServiceUnavailable, ErrInternal, "MCP connector not configured")
		return
	}
	materialID, err := uuid.Parse(c.Param("material_id"))
	if err != nil {
		fail(c, http.StatusBadRequest, ErrValidation, "invalid material_id")
		return
	}
	token := c.Query("token")
	if token == "" || !s.mcpUploadSigner.Verify(materialID.String(), token) {
		fail(c, http.StatusForbidden, ErrUnauthorized, "invalid or expired upload token")
		return
	}

	mat, err := s.kb.GetUploadMaterial(ctx(c), materialID)
	if err != nil {
		fail(c, http.StatusNotFound, ErrNotFound, "material not found")
		return
	}
	// Fast-fail replay/retry-after-success without reading a (possibly
	// large) body first — a courtesy check only: the signed token stays
	// valid for its whole TTL (not single-use), so this is NOT the actual
	// safety boundary. CompleteMaterialUpload's atomic, WHERE-guarded update
	// below is — it is what actually prevents a second PUT from ever
	// overwriting bytes already attached to a live/draft KB record, even
	// under a genuine race between two concurrent PUTs to the same target.
	if mat.ProcessingStatus != "uploaded" {
		fail(c, http.StatusConflict, ErrConflict, "this material's upload already completed — the signed URL cannot be reused")
		return
	}

	limit := mcpUploadMaxBytes
	if mat.DeclaredSizeBytes > 0 && mat.DeclaredSizeBytes < int64(limit) {
		limit = int(mat.DeclaredSizeBytes)
	}
	data, err := io.ReadAll(io.LimitReader(c.Request.Body, int64(limit)+1))
	if err != nil {
		fail(c, http.StatusBadRequest, ErrValidation, "unreadable body")
		return
	}
	if len(data) > limit {
		fail(c, http.StatusRequestEntityTooLarge, ErrValidation, "upload exceeds the declared size")
		return
	}
	if mat.DeclaredSizeBytes > 0 && int64(len(data)) != mat.DeclaredSizeBytes {
		fail(c, http.StatusBadRequest, ErrValidation, "uploaded size does not match the declared size_bytes")
		return
	}

	if reason := blob.MimeSanityCheck(mat.MimeType, data); reason != "" {
		fail(c, http.StatusBadRequest, ErrValidation, reason)
		return
	}
	sum := sha256.Sum256(data)
	sumHex := hex.EncodeToString(sum[:])
	if mat.DeclaredChecksum != "" && sumHex != strings.ToLower(mat.DeclaredChecksum) {
		fail(c, http.StatusBadRequest, ErrValidation, "sha256_checksum does not match the uploaded bytes")
		return
	}

	// The object key is derived from the CONTENT hash, not the bare
	// material_id, so two different byte payloads racing for the same
	// signed target can never land on the same on-disk key — only
	// CompleteMaterialUpload's atomic guard below decides which attempt
	// (if either) gets to finalize, but this ensures the LOSING attempt's
	// bytes, even if written to disk, can never overwrite the winner's.
	storageKey, err := s.blob.Put(materialID.String()+"-"+sumHex[:16], data, blob.Meta{
		Mimetype: mat.MimeType, FileName: mat.Filename, FileSize: int64(len(data)),
	})
	if err != nil {
		fail(c, http.StatusInternalServerError, ErrInternal, "store failed")
		return
	}
	if err := s.kb.CompleteMaterialUpload(ctx(c), materialID, "disk", storageKey, int64(len(data)), sumHex); err != nil {
		if errors.Is(err, kbstore.ErrUploadAlreadyCompleted) {
			fail(c, http.StatusConflict, ErrConflict, "this material's upload already completed — the signed URL cannot be reused")
			return
		}
		fail(c, http.StatusInternalServerError, ErrInternal, "finalize failed")
		return
	}
	ok(c, gin.H{"material_id": materialID, "processing_status": "parsed"})
}
