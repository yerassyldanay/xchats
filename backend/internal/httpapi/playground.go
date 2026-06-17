package httpapi

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/internal/blob"
	"github.com/yerassyldanay/xchats/backend/internal/brain/domain"
	"github.com/yerassyldanay/xchats/backend/internal/kbstore"
)

// kbFail maps a kbstore error to the right HTTP status + machine code, so callers
// don't bucket everything as 500. Unknown errors stay 500.
func (s *Server) kbFail(c *gin.Context, err error) {
	var ge *kbstore.GateError
	switch {
	case errors.As(err, &ge):
		fail(c, http.StatusUnprocessableEntity, ErrValidation, ge.Error())
	case errors.Is(err, kbstore.ErrNoDraft):
		fail(c, http.StatusBadRequest, ErrValidation, "no open draft")
	case errors.Is(err, kbstore.ErrUnknownKind):
		fail(c, http.StatusBadRequest, ErrValidation, "unknown row kind")
	case errors.Is(err, kbstore.ErrStale):
		fail(c, http.StatusConflict, ErrDraftStale, "draft changed since you loaded it; reload and retry")
	case errors.Is(err, kbstore.ErrNoPublished):
		fail(c, http.StatusNotFound, ErrNotFound, "no published version")
	default:
		fail(c, http.StatusInternalServerError, ErrInternal, "kb: "+err.Error())
	}
}

// brainReloader is implemented by the real drafter: it hot-swaps the published KB
// the brain drafts from. The stub drafter does not implement it (no-op reload).
type brainReloader interface {
	SetSnapshot(*domain.Snapshot)
}

// kbReady fails the request if the KB layer isn't wired (stub/no-DB boot).
func (s *Server) kbReady(c *gin.Context) bool {
	if s.kb == nil {
		fail(c, http.StatusServiceUnavailable, ErrInternal, "knowledge base not available")
		return false
	}
	return true
}

// pgOrg resolves the org for a playground request (v1: the operator's single org).
func (s *Server) pgOrg(c *gin.Context) (uuid.UUID, bool) {
	org, ok := s.orgOf(c)
	if !ok {
		return uuid.Nil, false
	}
	return org.ID, true
}

// reloadBrain reloads the published KB into the drafter after a publish/rollback.
func (s *Server) reloadBrain(c *gin.Context, orgID uuid.UUID) {
	r, ok := s.drafter.(brainReloader)
	if !ok {
		return
	}
	snap, err := s.kb.LoadPublished(ctx(c), orgID)
	if err != nil {
		s.log.Warn("reload brain after publish failed", "err", err)
		return
	}
	r.SetSnapshot(snap)
	s.log.Info("brain KB reloaded", "version", snap.Config.Version)
}

// --- draft read / lifecycle ------------------------------------------------

// handlePlaygroundDraft is a side-effect-free read: it returns the open draft if
// one exists, or {has_draft:false} otherwise. Opening a draft is the explicit job
// of POST /draft, so a GET never clones the published snapshot.
func (s *Server) handlePlaygroundDraft(c *gin.Context) {
	if !s.kbReady(c) {
		return
	}
	orgID, proceed := s.pgOrg(c)
	if !proceed {
		return
	}
	view, err := s.kb.ReadDraft(ctx(c), orgID)
	if errors.Is(err, kbstore.ErrNoDraft) {
		ok(c, gin.H{"has_draft": false})
		return
	}
	if err != nil {
		s.kbFail(c, err)
		return
	}
	ok(c, view)
}

func (s *Server) handlePlaygroundOpenDraft(c *gin.Context) {
	if !s.kbReady(c) {
		return
	}
	orgID, proceed := s.pgOrg(c)
	if !proceed {
		return
	}
	if _, err := s.kb.OpenDraft(ctx(c), orgID); err != nil {
		s.kbFail(c, err)
		return
	}
	view, err := s.kb.GetDraft(ctx(c), orgID)
	if err != nil {
		s.kbFail(c, err)
		return
	}
	created(c, view)
}

func (s *Server) handlePlaygroundDiscardDraft(c *gin.Context) {
	if !s.kbReady(c) {
		return
	}
	orgID, proceed := s.pgOrg(c)
	if !proceed {
		return
	}
	if err := s.kb.DiscardDraft(ctx(c), orgID); err != nil {
		s.kbFail(c, err)
		return
	}
	ok(c, nil)
}

// --- topics ----------------------------------------------------------------

type topicReq struct {
	Slug     string `json:"slug"`
	Lang     string `json:"lang"`
	Title    string `json:"title"`
	Keywords string `json:"keywords"`
	BodyMD   string `json:"body_md"`
}

func (s *Server) handlePlaygroundUpsertTopic(c *gin.Context) {
	orgID, proceed := s.pgWrite(c)
	if !proceed {
		return
	}
	var req topicReq
	if err := c.ShouldBindJSON(&req); err != nil || req.Slug == "" {
		fail(c, http.StatusBadRequest, ErrValidation, "slug required")
		return
	}
	if err := s.kb.UpsertTopic(ctx(c), orgID, kbstore.TopicInput{
		Slug: req.Slug, Lang: req.Lang, Title: req.Title, Keywords: req.Keywords, BodyMD: req.BodyMD,
		Provenance: `{"source":"manual"}`,
	}); err != nil {
		s.kbFail(c, err)
		return
	}
	s.kbChanged(c, orgID)
}

func (s *Server) handlePlaygroundDeleteTopic(c *gin.Context) {
	orgID, proceed := s.pgWrite(c)
	if !proceed {
		return
	}
	if err := s.kb.DeleteTopic(ctx(c), orgID, c.Param("slug")); err != nil {
		s.kbFail(c, err)
		return
	}
	s.kbChanged(c, orgID)
}

// --- assets (upload bytes + meta) ------------------------------------------

func (s *Server) handlePlaygroundUploadAsset(c *gin.Context) {
	orgID, proceed := s.pgWrite(c)
	if !proceed {
		return
	}
	fh, err := c.FormFile("file")
	if err != nil {
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
	mediaType, mimetype := detectMedia(fh.Filename, fh.Header.Get("Content-Type"))
	ref := uuid.NewString()
	if _, err := s.blob.Put(ref, data, blob.Meta{MediaType: mediaType, Mimetype: mimetype, FileName: fh.Filename, FileSize: int64(len(data))}); err != nil {
		fail(c, http.StatusBadGateway, ErrMediaUnavailable, "store failed")
		return
	}
	if err := s.kb.UpsertAsset(ctx(c), orgID, kbstore.AssetInput{
		Ref: ref, Kind: mediaType, TopicSlug: c.PostForm("topic_slug"),
		Title: fh.Filename, Description: c.PostForm("description"),
		URL: "/xchats/api/v1/media/" + ref, Lang: c.PostForm("lang"),
		Provenance: `{"source":"manual"}`,
	}); err != nil {
		s.kbFail(c, err)
		return
	}
	s.kbChanged(c, orgID)
}

type assetPatchReq struct {
	Description *string `json:"description"`
	TopicSlug   *string `json:"topic_slug"`
}

func (s *Server) handlePlaygroundPatchAsset(c *gin.Context) {
	orgID, proceed := s.pgWrite(c)
	if !proceed {
		return
	}
	var req assetPatchReq
	_ = c.ShouldBindJSON(&req)
	if err := s.kb.PatchAsset(ctx(c), orgID, c.Param("ref"), kbstore.AssetPatch{
		Description: req.Description, TopicSlug: req.TopicSlug,
	}); err != nil {
		s.kbFail(c, err)
		return
	}
	s.kbChanged(c, orgID)
}

func (s *Server) handlePlaygroundDeleteAsset(c *gin.Context) {
	orgID, proceed := s.pgWrite(c)
	if !proceed {
		return
	}
	if err := s.kb.DeleteAsset(ctx(c), orgID, c.Param("ref")); err != nil {
		s.kbFail(c, err)
		return
	}
	s.kbChanged(c, orgID)
}

// --- values ----------------------------------------------------------------

type valueReq struct {
	Token       string `json:"token"`
	Lang        string `json:"lang"`
	ValueText   string `json:"value_text"`
	Description string `json:"description"`
}

func (s *Server) handlePlaygroundUpsertValue(c *gin.Context) {
	orgID, proceed := s.pgWrite(c)
	if !proceed {
		return
	}
	var req valueReq
	if err := c.ShouldBindJSON(&req); err != nil || req.Token == "" {
		fail(c, http.StatusBadRequest, ErrValidation, "token required")
		return
	}
	if err := s.kb.UpsertValue(ctx(c), orgID, kbstore.ValueInput{
		Token: req.Token, Lang: req.Lang, ValueText: req.ValueText, Description: req.Description,
		Provenance: `{"source":"manual"}`,
	}); err != nil {
		s.kbFail(c, err)
		return
	}
	s.kbChanged(c, orgID)
}

func (s *Server) handlePlaygroundDeleteValue(c *gin.Context) {
	orgID, proceed := s.pgWrite(c)
	if !proceed {
		return
	}
	if err := s.kb.DeleteValue(ctx(c), orgID, c.Param("token"), c.Query("lang")); err != nil {
		s.kbFail(c, err)
		return
	}
	s.kbChanged(c, orgID)
}

// --- config ----------------------------------------------------------------

type configReq struct {
	Persona        *string `json:"persona"`
	Mission        *string `json:"mission"`
	Guardrails     *string `json:"guardrails"`
	LanguagePolicy *string `json:"language_policy"`
	ReplyMaxWords  *int    `json:"reply_max_words"`
}

func (s *Server) handlePlaygroundPatchConfig(c *gin.Context) {
	orgID, proceed := s.pgWrite(c)
	if !proceed {
		return
	}
	var req configReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, ErrValidation, "bad config")
		return
	}
	if err := s.kb.PatchConfig(ctx(c), orgID, kbstore.ConfigPatch{
		Persona: req.Persona, Mission: req.Mission, Guardrails: req.Guardrails,
		LanguagePolicy: req.LanguagePolicy, ReplyMaxWords: req.ReplyMaxWords,
	}); err != nil {
		s.kbFail(c, err)
		return
	}
	s.kbChanged(c, orgID)
}

// --- review (accept / deny a proposed row) ---------------------------------

type reviewReq struct {
	State string `json:"state"` // 'approved' | 'rejected'
}

func (s *Server) handlePlaygroundReview(c *gin.Context) {
	orgID, proceed := s.pgWrite(c)
	if !proceed {
		return
	}
	rowID, proceed := parseUUID(c, "id")
	if !proceed {
		return
	}
	var req reviewReq
	_ = c.ShouldBindJSON(&req)
	if req.State != "approved" && req.State != "rejected" {
		fail(c, http.StatusBadRequest, ErrValidation, "state must be approved|rejected")
		return
	}
	err := s.kb.SetReviewState(ctx(c), orgID, c.Param("kind"), rowID, req.State)
	if errors.Is(err, kbstore.ErrUnknownKind) {
		fail(c, http.StatusBadRequest, ErrValidation, "kind must be topics|assets|values")
		return
	}
	if err != nil {
		s.kbFail(c, err)
		return
	}
	s.kbChanged(c, orgID)
}

// --- publish / rollback ----------------------------------------------------

func (s *Server) handlePlaygroundPublish(c *gin.Context) {
	if !s.kbReady(c) {
		return
	}
	orgID, proceed := s.pgOrg(c)
	if !proceed {
		return
	}
	version, err := s.kb.Publish(ctx(c), orgID, s.blob.Exists)
	var ge *kbstore.GateError
	if errors.As(err, &ge) {
		fail(c, http.StatusUnprocessableEntity, ErrValidation, ge.Error())
		return
	}
	if errors.Is(err, kbstore.ErrNoDraft) {
		fail(c, http.StatusBadRequest, ErrValidation, "no open draft to publish")
		return
	}
	if err != nil {
		s.kbFail(c, err)
		return
	}
	s.reloadBrain(c, orgID)
	s.hub.Broadcast("kb.published", gin.H{"version": version})
	ok(c, gin.H{"version": version})
}

type rollbackReq struct {
	Version int `json:"version"`
}

func (s *Server) handlePlaygroundRollback(c *gin.Context) {
	if !s.kbReady(c) {
		return
	}
	orgID, proceed := s.pgOrg(c)
	if !proceed {
		return
	}
	var req rollbackReq
	if err := c.ShouldBindJSON(&req); err != nil || req.Version < 1 {
		fail(c, http.StatusBadRequest, ErrValidation, "version required")
		return
	}
	version, err := s.kb.Rollback(ctx(c), orgID, req.Version)
	if errors.Is(err, kbstore.ErrNoPublished) {
		fail(c, http.StatusNotFound, ErrNotFound, "no such published version")
		return
	}
	if err != nil {
		s.kbFail(c, err)
		return
	}
	s.reloadBrain(c, orgID)
	s.hub.Broadcast("kb.published", gin.H{"version": version, "rolled_back_from": req.Version})
	ok(c, gin.H{"version": version})
}

// --- shared helpers --------------------------------------------------------

// pgWrite is the common preamble for a draft write: KB ready + org resolved, plus
// an OPTIONAL optimistic-concurrency check. A client that sends If-Match (the
// draft's updated_at from its last load) gets a 409 DRAFT_STALE if the draft has
// since moved — so concurrent edits don't silently clobber each other. Clients
// that omit the header keep last-write-wins (v1 single-operator default).
func (s *Server) pgWrite(c *gin.Context) (uuid.UUID, bool) {
	if !s.kbReady(c) {
		return uuid.Nil, false
	}
	orgID, proceed := s.pgOrg(c)
	if !proceed {
		return uuid.Nil, false
	}
	if tok := strings.Trim(strings.TrimSpace(c.GetHeader("If-Match")), `"`); tok != "" {
		cur, err := s.kb.DraftUpdatedAt(ctx(c), orgID)
		if err != nil {
			s.kbFail(c, err)
			return uuid.Nil, false
		}
		want, perr := time.Parse(time.RFC3339Nano, tok)
		if perr != nil || !want.Equal(cur) {
			fail(c, http.StatusConflict, ErrDraftStale, "draft changed since you loaded it; reload and retry")
			return uuid.Nil, false
		}
	}
	return orgID, true
}

// kbChanged is the common write epilogue: it advances the draft's concurrency
// token, broadcasts the row change, and returns the refreshed draft view so the
// editor and the chat stay in sync over the same draft.
func (s *Server) kbChanged(c *gin.Context, orgID uuid.UUID) {
	if err := s.kb.TouchDraft(ctx(c), orgID); err != nil {
		s.kbFail(c, err)
		return
	}
	view, err := s.kb.GetDraft(ctx(c), orgID)
	if err != nil {
		s.kbFail(c, err)
		return
	}
	s.hub.Broadcast("kb.row.changed", gin.H{"version": view.Config.Version})
	ok(c, view)
}
