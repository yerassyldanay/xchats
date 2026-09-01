package httpapi

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/yerassyldanay/xchats/backend/automation"
	"github.com/yerassyldanay/xchats/backend/internal/dto"
	"github.com/yerassyldanay/xchats/backend/internal/queue"
	"github.com/yerassyldanay/xchats/backend/internal/store"
	"github.com/yerassyldanay/xchats/backend/internal/worker"
)

// handleSuggest is the on-demand "Подсказать ответ" trigger. Idempotent: if a
// pending suggestion already exists it is returned rather than creating a second.
// Manual Suggest/Regenerate bypasses debounce (this always fires immediately,
// on every channel mode) EXCEPT "off", which rejects it outright — off means
// no automatic OR manual drafts, full stop.
func (s *Server) handleSuggest(c *gin.Context) {
	chatID, okID := parseUUID(c, "id")
	if !okID {
		return
	}
	chat, okChat := s.orgChat(c, chatID)
	if !okChat {
		return
	}
	settings, err := s.store.AutomationSettingsForAccount(ctx(c), chat.AccountID)
	if err != nil {
		fail(c, http.StatusInternalServerError, ErrInternal, err.Error())
		return
	}
	if settings.Mode == string(automation.ModeOff) {
		fail(c, http.StatusConflict, ErrAutomationOff, "automation is off for this channel; manual suggestions are disabled")
		return
	}
	// force=true (the panel's "Regenerate") skips the idempotent return and asks for
	// a brand-new draft; the worker's WriteDraftSet supersedes the prior pending row.
	if c.Query("force") != "true" {
		pending, err := s.store.PendingDrafts(ctx(c), chatID)
		if err != nil {
			fail(c, http.StatusInternalServerError, ErrInternal, err.Error())
			return
		}
		if len(pending) > 0 {
			ok(c, gin.H{"items": mapDrafts(pending), "trigger_message_id": triggerOf(pending)})
			return
		}
	}
	trigger, _ := s.store.LatestInboundMessageID(ctx(c), chatID)
	s.publishOrLog(ctx(c), queue.Message{Kind: queue.KindAIDraft, Payload: worker.AIDraftTask{ChatID: chatID}})
	var trig *string
	if trigger.Valid {
		t := trigger.UUID.String()
		trig = &t
	}
	accepted(c, gin.H{"trigger_message_id": trig})
}

func (s *Server) handleListDrafts(c *gin.Context) {
	chatID, okID := parseUUID(c, "id")
	if !okID {
		return
	}
	if _, ok := s.orgChat(c, chatID); !ok {
		return
	}
	pending, err := s.store.PendingDrafts(ctx(c), chatID)
	if err != nil {
		fail(c, http.StatusInternalServerError, ErrInternal, err.Error())
		return
	}
	ok(c, gin.H{"items": mapDrafts(pending)})
}

type approveReq struct {
	EditedText *string   `json:"edited_text"`
	MediaIDs   *[]string `json:"media_ids"`
}

// handleApprove is the guarded single-send. The conditional claim (UPDATE …
// WHERE draft_state='suggested') wins at most once; a lost claim is classified
// CONFLICT (already approved) or DRAFT_STALE (superseded by a newer inbound).
func (s *Server) handleApprove(c *gin.Context) {
	draftID, okID := parseUUID(c, "id")
	if !okID {
		return
	}
	var req approveReq
	_ = c.ShouldBindJSON(&req)

	// Resolve the draft's chat and enforce org membership *before* the guarded
	// claim, so a cross-org caller can never flip a draft to 'sent'.
	d0, err := s.store.DraftByID(ctx(c), draftID)
	if errors.Is(err, store.ErrNotFound) {
		fail(c, http.StatusNotFound, ErrNotFound, "draft not found")
		return
	}
	if err != nil {
		fail(c, http.StatusInternalServerError, ErrInternal, err.Error())
		return
	}
	chat, okChat := s.orgChat(c, d0.ChatID)
	if !okChat {
		return
	}

	claim, err := s.store.ClaimDraft(ctx(c), draftID)
	if errors.Is(err, store.ErrNotFound) {
		s.classifyLostClaim(c, draftID)
		return
	}
	if err != nil {
		fail(c, http.StatusInternalServerError, ErrInternal, err.Error())
		return
	}

	text := claim.DraftText
	if req.EditedText != nil {
		text = *req.EditedText
	}
	var mediaIDs []string
	if req.MediaIDs != nil {
		mediaIDs = *req.MediaIDs
	}

	items, err := s.sendParts(c, chat, "ai", uuid.NullUUID{}, text, mediaIDs)
	if err != nil {
		_ = s.store.ReopenDraft(ctx(c), draftID)
		fail(c, http.StatusBadGateway, ErrSendFailed, err.Error())
		return
	}
	if len(items) > 0 {
		if mid, perr := uuid.Parse(items[0].ID); perr == nil {
			_ = s.store.SetDraftSent(ctx(c), draftID, mid)
		}
	}
	// Clear the option cards everywhere.
	s.hub.Broadcast("ai_draft.updated", dto.MapDraft(claim))
	ok(c, gin.H{"items": items})
}

// handleDismissDrafts persists INB-14's Dismiss action. Previously Dismiss
// only cleared Pinia state locally (inbox.drafts = []), so the same options
// reappeared the moment they were refetched — on reload or reselecting the
// chat. This is an explicit backend state transition instead.
func (s *Server) handleDismissDrafts(c *gin.Context) {
	chatID, okID := parseUUID(c, "id")
	if !okID {
		return
	}
	if _, ok := s.orgChat(c, chatID); !ok {
		return
	}
	dismissed, err := s.store.DismissDrafts(ctx(c), chatID)
	if err != nil {
		fail(c, http.StatusInternalServerError, ErrInternal, err.Error())
		return
	}
	// Same event/shape handleApprove already broadcasts on send — the
	// frontend's existing ai_draft.updated handler clears the panel for
	// every operator viewing this chat, not just the one who clicked Dismiss.
	for _, d := range dismissed {
		s.hub.Broadcast("ai_draft.updated", dto.MapDraft(d))
	}
	ok(c, gin.H{"items": mapDrafts(dismissed)})
}

func (s *Server) classifyLostClaim(c *gin.Context, draftID uuid.UUID) {
	d, err := s.store.DraftByID(ctx(c), draftID)
	if errors.Is(err, store.ErrNotFound) {
		fail(c, http.StatusNotFound, ErrNotFound, "draft not found")
		return
	}
	if err != nil {
		fail(c, http.StatusInternalServerError, ErrInternal, err.Error())
		return
	}
	switch d.DraftState {
	case "superseded":
		fail(c, http.StatusConflict, ErrDraftStale, "draft superseded by a newer inbound")
	default: // sent / rejected
		fail(c, http.StatusConflict, ErrConflict, "draft already approved")
	}
}

func mapDrafts(ds []store.Draft) []dto.AiDraft {
	out := make([]dto.AiDraft, 0, len(ds))
	for _, d := range ds {
		out = append(out, dto.MapDraft(d))
	}
	return out
}

func triggerOf(ds []store.Draft) *string {
	for _, d := range ds {
		if d.TriggerMessageID.Valid {
			s := d.TriggerMessageID.UUID.String()
			return &s
		}
	}
	return nil
}
