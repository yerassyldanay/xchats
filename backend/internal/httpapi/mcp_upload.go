package httpapi

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/internal/blob"
)

// mcpUploadMaxBytes is a hard safety cap independent of the declared
// size_bytes — no legitimate KB media file needs more than this.
const mcpUploadMaxBytes = 50 << 20 // 50 MiB

// uploadCORS allows any origin on the signed-upload route only: its
// authentication is the unguessable signed token in the query string, never
// a cookie, so there is no credentialed cross-origin request to protect
// against (unlike every other route, which stays on the configured
// CORSOrigins allowlist) — the KB Manager widget's iframe, hosted on
// chatgpt.com/claude.ai's own origin, must be able to PUT bytes here
// directly (plan/mcp.md §6: "The widget uploads bytes directly to object
// storage").
func (s *Server) uploadCORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin != "" {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Vary", "Origin")
		}
		if c.Request.Method == http.MethodOptions {
			c.Header("Access-Control-Allow-Methods", "PUT,OPTIONS")
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

	if reason := mimeSanityCheck(mat.MimeType, data); reason != "" {
		fail(c, http.StatusBadRequest, ErrValidation, reason)
		return
	}
	if mat.DeclaredChecksum != "" {
		sum := sha256.Sum256(data)
		if hex.EncodeToString(sum[:]) != strings.ToLower(mat.DeclaredChecksum) {
			fail(c, http.StatusBadRequest, ErrValidation, "sha256_checksum does not match the uploaded bytes")
			return
		}
	}

	storageKey, err := s.blob.Put(materialID.String(), data, blob.Meta{
		Mimetype: mat.MimeType, FileName: mat.Filename, FileSize: int64(len(data)),
	})
	if err != nil {
		fail(c, http.StatusInternalServerError, ErrInternal, "store failed")
		return
	}
	sum := sha256.Sum256(data)
	if err := s.kb.CompleteMaterialUpload(ctx(c), materialID, "disk", storageKey, int64(len(data)), hex.EncodeToString(sum[:])); err != nil {
		fail(c, http.StatusInternalServerError, ErrInternal, "finalize failed")
		return
	}
	ok(c, gin.H{"material_id": materialID, "processing_status": "parsed"})
}

// mimeSanityCheck is a lightweight guard against a renamed/mislabeled
// upload, not a full content-type authority: for image/video/audio the
// sniffed top-level type must agree with what was declared (these are the
// types a widget will render inline), and a payload that sniffs as HTML is
// always rejected unless HTML was actually declared (the classic
// stored-XSS-via-mislabeled-upload vector) — everything else (documents:
// application/*, text/* other than html) is accepted as declared, since
// office/PDF formats sniff too inconsistently to cross-check usefully.
func mimeSanityCheck(declared string, data []byte) string {
	sniffed := http.DetectContentType(data)
	sniffedTop := strings.SplitN(strings.SplitN(sniffed, ";", 2)[0], "/", 2)[0]
	declaredTop := strings.SplitN(declared, "/", 2)[0]
	if (declaredTop == "image" || declaredTop == "video" || declaredTop == "audio") && sniffedTop != declaredTop {
		return fmt.Sprintf("declared mime_type %q does not match the uploaded content (looks like %s)", declared, sniffed)
	}
	if strings.HasPrefix(sniffed, "text/html") && declaredTop != "text" {
		return "uploaded content looks like HTML, which is not an accepted KB media type"
	}
	return ""
}
