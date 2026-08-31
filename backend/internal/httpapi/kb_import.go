package httpapi

import (
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/internal/credentials"
	"github.com/yerassyldanay/xchats/backend/internal/extractor"
	"github.com/yerassyldanay/xchats/backend/internal/kbimport"
	"github.com/yerassyldanay/xchats/backend/internal/mcpserver"
)

// kbImportMaxFileBytes mirrors mcpserver.MaxMediaUploadBytes — "what MCP can
// upload, the browser can too" (mcp_upload.go's own doc comment on
// mcpUploadMaxBytes), and this is the same discipline applied a third time.
const kbImportMaxFileBytes = mcpserver.MaxMediaUploadBytes // 50 MiB

// kbImportMaxBodyBytes bounds the WHOLE multipart body: up to 10 files at
// kbImportMaxFileBytes each, plus slack for the url/provider/target_type/
// guidance form fields. Wrapped around the request body BEFORE
// c.MultipartForm() is ever called, so an oversized upload is rejected
// while streaming instead of after fully buffering it.
const kbImportMaxBodyBytes = 10*kbImportMaxFileBytes + 1<<20

const kbImportMaxGuidanceRunes = 2000

// handleKBCreateImport is POST /kb/imports: submit any mix of URLs and
// files for the structured import pipeline (internal/kbimport). All
// server-side validation (vocabulary, SSRF, provider/source compatibility,
// credential precheck, one-active-run-per-org) lives in
// kbimport.Service.Submit — this handler is a thin multipart-parsing
// adapter in front of it, translating the parsed form into a
// kbimport.SubmitInput and Submit's typed errors into HTTP status codes.
func (s *Server) handleKBCreateImport(c *gin.Context) {
	orgID, proceed := s.kbWrite(c)
	if !proceed {
		return
	}
	if s.kbImport == nil {
		fail(c, http.StatusServiceUnavailable, ErrInternal, "the import pipeline is not available")
		return
	}

	// Wrap the body BEFORE anything reads it — c.MultipartForm() is what
	// triggers the read (mirrors handleUploadMedia/handleKBUploadMaterial's
	// own ordering).
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, kbImportMaxBodyBytes)
	form, err := c.MultipartForm()
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			fail(c, http.StatusRequestEntityTooLarge, ErrValidation, "upload exceeds the maximum allowed size")
			return
		}
		fail(c, http.StatusBadRequest, ErrValidation, "invalid multipart form")
		return
	}

	in := kbimport.SubmitInput{
		Provider:   strings.TrimSpace(firstFormValue(form, "provider")),
		TargetType: strings.TrimSpace(firstFormValue(form, "target_type")),
		Guidance:   firstFormValue(form, "guidance"),
	}
	if n := len([]rune(in.Guidance)); n > kbImportMaxGuidanceRunes {
		fail(c, http.StatusBadRequest, ErrValidation, "guidance exceeds the maximum length")
		return
	}
	for _, u := range form.Value["url"] {
		if u = strings.TrimSpace(u); u != "" {
			in.URLs = append(in.URLs, u)
		}
	}

	fileHeaders := form.File["file"]
	if len(fileHeaders) > 10 {
		fail(c, http.StatusBadRequest, ErrValidation, "at most 10 files per import")
		return
	}
	for _, fh := range fileHeaders {
		if fh.Size > kbImportMaxFileBytes {
			fail(c, http.StatusRequestEntityTooLarge, ErrValidation, fh.Filename+" exceeds the maximum file size")
			return
		}
		f, err := fh.Open()
		if err != nil {
			fail(c, http.StatusBadRequest, ErrValidation, "cannot open "+fh.Filename)
			return
		}
		data, readErr := io.ReadAll(io.LimitReader(f, kbImportMaxFileBytes+1))
		_ = f.Close()
		if readErr != nil || int64(len(data)) > kbImportMaxFileBytes {
			fail(c, http.StatusRequestEntityTooLarge, ErrValidation, "cannot read "+fh.Filename)
			return
		}
		_, mimeType := detectMedia(fh.Filename, fh.Header.Get("Content-Type"))
		in.Files = append(in.Files, kbimport.SubmitFile{Filename: fh.Filename, MimeType: mimeType, Data: data})
	}

	run, err := s.kbImport.Submit(ctx(c), orgID, currentUser(c).ID, in)
	if err != nil {
		s.kbImportFail(c, err)
		return
	}
	accepted(c, run)
}

// handleKBListImports is GET /kb/imports: the org's most recent import
// runs, newest first.
func (s *Server) handleKBListImports(c *gin.Context) {
	if !s.kbReady(c) {
		return
	}
	orgID, proceed := s.pgOrg(c)
	if !proceed {
		return
	}
	if s.kbImport == nil {
		fail(c, http.StatusServiceUnavailable, ErrInternal, "the import pipeline is not available")
		return
	}
	limit := 20
	if v := c.Query("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	runs, err := s.kbImport.ListRuns(ctx(c), orgID, limit)
	if err != nil {
		s.kbImportFail(c, err)
		return
	}
	ok(c, gin.H{"runs": runs})
}

// handleKBGetImport is GET /kb/imports/:id: one run's current status —
// per-material processing_status/handle/error, and a synthesis block once
// pass 2 has started. Org scoping happens inside kbimport.Service.RunStatus
// (via kbstore's own WHERE organization_id = ...), so a cross-org id is an
// ordinary 404, indistinguishable from an unknown one.
func (s *Server) handleKBGetImport(c *gin.Context) {
	if !s.kbReady(c) {
		return
	}
	orgID, proceed := s.pgOrg(c)
	if !proceed {
		return
	}
	if s.kbImport == nil {
		fail(c, http.StatusServiceUnavailable, ErrInternal, "the import pipeline is not available")
		return
	}
	runID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		fail(c, http.StatusBadRequest, ErrValidation, "invalid run id")
		return
	}
	run, err := s.kbImport.RunStatus(ctx(c), orgID, runID)
	if err != nil {
		s.kbImportFail(c, err)
		return
	}
	ok(c, run)
}

// handleKBCancelImport is POST /kb/imports/:id/cancel: stop a run in
// progress. A write like Submit itself (kbWrite, not kbReady — cancelling
// mutates kbd_materials), so it is refused the same way Submit is while
// the KB gate is closed.
func (s *Server) handleKBCancelImport(c *gin.Context) {
	orgID, proceed := s.kbWrite(c)
	if !proceed {
		return
	}
	if s.kbImport == nil {
		fail(c, http.StatusServiceUnavailable, ErrInternal, "the import pipeline is not available")
		return
	}
	runID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		fail(c, http.StatusBadRequest, ErrValidation, "invalid run id")
		return
	}
	if err := s.kbImport.Cancel(ctx(c), orgID, runID); err != nil {
		s.kbImportFail(c, err)
		return
	}
	run, err := s.kbImport.RunStatus(ctx(c), orgID, runID)
	if err != nil {
		s.kbImportFail(c, err)
		return
	}
	ok(c, run)
}

// kbImportProviderSummary is one entry of GET /kb/import/providers.
type kbImportProviderSummary struct {
	ID                 string   `json:"id"`
	DisplayName        string   `json:"display_name"`
	Families           []string `json:"families"`
	RequiresCredential bool     `json:"requires_credential"`
	Configured         bool     `json:"configured"`
}

// handleKBImportProviders is GET /kb/import/providers: the one source of
// truth for the operator-facing parser dropdown — which source families
// each provider reads AND whether it is currently usable. Families come
// from extractor.Capabilities(), data-derived from each provider's real
// Supports() method rather than a second hand-maintained table.
// Configured/RequiresCredential come from kbImport.ProviderStatus, which
// runs through the EXACT SAME requireCredential call Submit's own
// resolveProvider makes (kbimport.go) — so this endpoint can never disagree
// with Submit's own precheck, and a just-saved key takes effect here with
// no restart, same as everywhere else.
func (s *Server) handleKBImportProviders(c *gin.Context) {
	if !s.kbReady(c) {
		return
	}
	if _, proceed := s.pgOrg(c); !proceed {
		return
	}
	if s.kbImport == nil {
		fail(c, http.StatusServiceUnavailable, ErrInternal, "the import pipeline is not available")
		return
	}
	caps := extractor.Capabilities()
	out := make([]kbImportProviderSummary, 0, len(caps))
	for _, cp := range caps {
		requiresCredential, configured := s.kbImport.ProviderStatus(ctx(c), cp.Name)
		sum := kbImportProviderSummary{
			ID: cp.Name, DisplayName: cp.Name, Families: familyStrings(cp),
			RequiresCredential: requiresCredential, Configured: configured,
		}
		if p, known := credentials.ProviderByID(cp.Name); known {
			sum.DisplayName = p.DisplayName
		}
		out = append(out, sum)
	}
	ok(c, gin.H{"providers": out})
}

// familyStrings extracts cp's supported families, in extractor.Families'
// fixed order, as plain strings for the wire — the frontend's MIME->family
// mapping compares against these string values directly.
func familyStrings(cp extractor.ProviderCapabilities) []string {
	out := make([]string, 0, len(cp.Families))
	for _, f := range extractor.Families {
		if cp.Families[f] {
			out = append(out, string(f))
		}
	}
	return out
}

// kbImportFail maps kbimport's typed errors to the right HTTP status +
// machine code — no new errcodes: 422/409 pair with the existing
// ErrValidation/ErrConflict strings, distinguished by HTTP status alone,
// the same pattern kbFail already uses for GateError (422 + ErrValidation).
func (s *Server) kbImportFail(c *gin.Context, err error) {
	switch {
	case errors.Is(err, kbimport.ErrProviderNotConfigured):
		fail(c, http.StatusUnprocessableEntity, ErrValidation, err.Error())
	case errors.Is(err, kbimport.ErrValidation):
		fail(c, http.StatusBadRequest, ErrValidation, err.Error())
	case errors.Is(err, kbimport.ErrRunActive):
		fail(c, http.StatusConflict, ErrConflict, err.Error())
	case errors.Is(err, kbimport.ErrCancelNotAllowed):
		fail(c, http.StatusConflict, ErrConflict, err.Error())
	case errors.Is(err, kbimport.ErrNotFound):
		fail(c, http.StatusNotFound, ErrNotFound, err.Error())
	default:
		fail(c, http.StatusInternalServerError, ErrInternal, "kb import: "+err.Error())
	}
}

// firstFormValue reads a single-value multipart form field, "" if absent.
func firstFormValue(form *multipart.Form, key string) string {
	if v := form.Value[key]; len(v) > 0 {
		return v[0]
	}
	return ""
}
