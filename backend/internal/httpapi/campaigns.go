// campaigns.go is the HTTP edge for Campaigns (plan/DECISIONS.md): campaign
// CRUD, recipient preview/persistence, lifecycle transitions, and the
// per-account sending-budget/sending-limits surface. Every handler here
// talks to internal/store directly — exactly like automation.go's own
// handleUpdateAccountAutomation — never to internal/campaign's
// Scheduler/Runner, which run as an independent background process (wired
// in cmd/xchats/main.go) and pick up a status change (or a newly replaced
// recipient set) on their own next tick.
package httpapi

import (
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/automation"
	purecampaign "github.com/yerassyldanay/xchats/backend/campaign"
	"github.com/yerassyldanay/xchats/backend/internal/dto"
	"github.com/yerassyldanay/xchats/backend/internal/store"
)

// campaignRecipientsMaxBodyBytes bounds POST /campaigns/:id/preview and PUT
// /campaigns/:id/recipients' request body — generous for even tens of
// thousands of pasted or CSV-uploaded rows, mirroring kb_import.go's own
// "generous but bounded" reasoning for a very different kind of upload.
const campaignRecipientsMaxBodyBytes = 8 << 20 // 8 MiB

// orgCampaign resolves a campaign id and enforces org ownership — the
// scoping root for every /campaigns/:id route, mirroring orgAnyAccount
// (automation.go).
func (s *Server) orgCampaign(c *gin.Context, id uuid.UUID) (store.Campaign, bool) {
	org, okOrg := s.orgOf(c)
	if !okOrg {
		return store.Campaign{}, false
	}
	camp, err := s.store.CampaignByIDForOrg(ctx(c), id, org.ID)
	if err != nil {
		fail(c, http.StatusNotFound, ErrNotFound, "campaign not found")
		return store.Campaign{}, false
	}
	return camp, true
}

// campaignDTO assembles a campaign's full wire shape — its own recurring
// windows and its live recipient-status counts are separate reads (see
// dto.MapCampaign's own doc comment on why).
func (s *Server) campaignDTO(c *gin.Context, camp store.Campaign) dto.Campaign {
	windows, err := s.store.CampaignWindowsFor(ctx(c), camp.ID)
	if err != nil {
		s.log.Error("campaign: load windows failed", "campaign_id", camp.ID, "err", err)
	}
	counts, err := s.store.CampaignRecipientCounts(ctx(c), camp.ID)
	if err != nil {
		s.log.Error("campaign: load recipient counts failed", "campaign_id", camp.ID, "err", err)
	}
	return dto.MapCampaign(camp, windows, counts)
}

// ---------------------------------------------------------------------------
// Campaign CRUD
// ---------------------------------------------------------------------------

func (s *Server) handleListCampaigns(c *gin.Context) {
	org, okOrg := s.orgOf(c)
	if !okOrg {
		return
	}
	limit, offset, pageNum, pageSize := s.pageParams(c)
	items, total, err := s.store.ListCampaignsForOrg(ctx(c), org.ID, limit, offset)
	if err != nil {
		fail(c, http.StatusInternalServerError, ErrInternal, err.Error())
		return
	}
	out := make([]dto.Campaign, 0, len(items))
	for _, it := range items {
		out = append(out, s.campaignDTO(c, it))
	}
	ok(c, page{Items: out, Page: pageNum, PageSize: pageSize, Total: total})
}

type createCampaignReq struct {
	Name        string `json:"name"`
	AccountID   string `json:"account_id"`
	MessageBody string `json:"message_body"`
}

func (s *Server) handleCreateCampaign(c *gin.Context) {
	org, okOrg := s.orgOf(c)
	if !okOrg {
		return
	}
	var req createCampaignReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, ErrValidation, "invalid request body")
		return
	}
	name := strings.TrimSpace(req.Name)
	if err := purecampaign.ValidateName(name); err != nil {
		fail(c, http.StatusBadRequest, ErrValidation, err.Error())
		return
	}
	if err := purecampaign.ValidateMessageBody(req.MessageBody); err != nil {
		fail(c, http.StatusBadRequest, ErrValidation, err.Error())
		return
	}
	acctID, err := uuid.Parse(req.AccountID)
	if err != nil {
		fail(c, http.StatusBadRequest, ErrValidation, "invalid account_id")
		return
	}
	acct, okAcct := s.orgAnyAccount(c, acctID)
	if !okAcct {
		return
	}
	camp, err := s.store.CreateCampaign(ctx(c), store.Campaign{
		OrganizationID: org.ID, Name: name, AccountID: acct.ID, Channel: acct.Channel,
		MessageBody: req.MessageBody, CreatedBy: currentUser(c).ID,
	})
	if err != nil {
		fail(c, http.StatusInternalServerError, ErrInternal, err.Error())
		return
	}
	created(c, s.campaignDTO(c, camp))
}

func (s *Server) handleGetCampaign(c *gin.Context) {
	id, okID := parseUUID(c, "id")
	if !okID {
		return
	}
	camp, okC := s.orgCampaign(c, id)
	if !okC {
		return
	}
	ok(c, s.campaignDTO(c, camp))
}

// updateCampaignReq is PATCH /campaigns/:id's body, bound twice: once via
// ShouldBindJSON for straightforward *string fields, and once as a raw
// key->value map so the handler can tell "field absent" (leave alone) apart
// from "field present as null" (explicit clear) for the three genuinely
// nullable groups — pace, schedule_at, and windows. See store.CampaignPatch's
// own doc comment for why that three-state distinction is load-bearing.
type updateCampaignReq struct {
	Name        *string `json:"name"`
	MessageBody *string `json:"message_body"`
	AccountID   *string `json:"account_id"`
}

// handleUpdateCampaign applies a partial update. Two independent gates: a
// change to message_body/account_id needs backend/campaign.CanEditContent
// (frozen forever once any message has sent); a change to name/pace/
// schedule_at/windows needs CanEditPacing (blocked only while running).
func (s *Server) handleUpdateCampaign(c *gin.Context) {
	id, okID := parseUUID(c, "id")
	if !okID {
		return
	}
	camp, okC := s.orgCampaign(c, id)
	if !okC {
		return
	}

	var raw map[string]any
	if err := c.ShouldBindBodyWithJSON(&raw); err != nil {
		fail(c, http.StatusBadRequest, ErrValidation, "invalid request body")
		return
	}
	var req updateCampaignReq
	if err := c.ShouldBindBodyWithJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, ErrValidation, "invalid request body")
		return
	}

	patch := store.CampaignPatch{}
	touchesContent, touchesPacing := false, false

	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if err := purecampaign.ValidateName(name); err != nil {
			fail(c, http.StatusBadRequest, ErrValidation, err.Error())
			return
		}
		patch.Name = &name
		touchesPacing = true
	}
	if req.MessageBody != nil {
		if err := purecampaign.ValidateMessageBody(*req.MessageBody); err != nil {
			fail(c, http.StatusBadRequest, ErrValidation, err.Error())
			return
		}
		patch.MessageBody = req.MessageBody
		touchesContent = true
	}
	if req.AccountID != nil {
		newAcctID, err := uuid.Parse(*req.AccountID)
		if err != nil {
			fail(c, http.StatusBadRequest, ErrValidation, "invalid account_id")
			return
		}
		acct, okAcct := s.orgAnyAccount(c, newAcctID)
		if !okAcct {
			return
		}
		patch.AccountID = &acct.ID
		patch.Channel = &acct.Channel
		touchesContent = true
	}

	_, hasMin := raw["min_interval_seconds"]
	_, hasJitter := raw["jitter_seconds"]
	if hasMin != hasJitter {
		fail(c, http.StatusBadRequest, ErrValidation, "min_interval_seconds and jitter_seconds must be set (or cleared) together")
		return
	}
	if hasMin {
		type paceBody struct {
			MinIntervalSeconds *int `json:"min_interval_seconds"`
			JitterSeconds      *int `json:"jitter_seconds"`
		}
		var pb paceBody
		if err := c.ShouldBindBodyWithJSON(&pb); err != nil {
			fail(c, http.StatusBadRequest, ErrValidation, "invalid request body")
			return
		}
		if (pb.MinIntervalSeconds == nil) != (pb.JitterSeconds == nil) {
			fail(c, http.StatusBadRequest, ErrValidation, "min_interval_seconds and jitter_seconds must both be null or both be set")
			return
		}
		if pb.MinIntervalSeconds != nil {
			if err := purecampaign.ValidatePace(*pb.MinIntervalSeconds, *pb.JitterSeconds); err != nil {
				fail(c, http.StatusBadRequest, ErrValidation, err.Error())
				return
			}
		}
		patch.HasPace = true
		patch.MinIntervalSeconds = pb.MinIntervalSeconds
		patch.JitterSeconds = pb.JitterSeconds
		touchesPacing = true
	}

	if _, has := raw["schedule_at"]; has {
		type scheduleBody struct {
			ScheduleAt *string `json:"schedule_at"`
		}
		var sb scheduleBody
		if err := c.ShouldBindBodyWithJSON(&sb); err != nil {
			fail(c, http.StatusBadRequest, ErrValidation, "invalid request body")
			return
		}
		patch.HasScheduleAt = true
		if sb.ScheduleAt != nil {
			t, err := time.Parse(time.RFC3339, *sb.ScheduleAt)
			if err != nil {
				fail(c, http.StatusBadRequest, ErrValidation, "schedule_at must be an RFC3339 timestamp")
				return
			}
			patch.ScheduleAt = &t
		}
		touchesPacing = true
	}

	if _, has := raw["windows"]; has {
		type windowsBody struct {
			Windows []scheduleWindowReq `json:"windows"`
		}
		var wb windowsBody
		if err := c.ShouldBindBodyWithJSON(&wb); err != nil {
			fail(c, http.StatusBadRequest, ErrValidation, "invalid request body")
			return
		}
		windows := make([]store.CampaignWindowInput, 0, len(wb.Windows))
		for i, w := range wb.Windows {
			win := automation.Window{Weekday: time.Weekday(w.Weekday), StartMinute: w.StartMinute, EndMinute: w.EndMinute}
			if err := automation.ValidateWindow(win); err != nil {
				fail(c, http.StatusBadRequest, ErrValidation, "windows["+strconv.Itoa(i)+"]: "+err.Error())
				return
			}
			windows = append(windows, store.CampaignWindowInput{Weekday: w.Weekday, StartMinute: w.StartMinute, EndMinute: w.EndMinute})
		}
		patch.HasWindows = true
		patch.Windows = windows
		touchesPacing = true
	}

	if !touchesContent && !touchesPacing {
		ok(c, s.campaignDTO(c, camp))
		return
	}

	counts, err := s.store.CampaignRecipientCounts(ctx(c), camp.ID)
	if err != nil {
		fail(c, http.StatusInternalServerError, ErrInternal, err.Error())
		return
	}
	if touchesContent && !purecampaign.CanEditContent(counts["sent"]) {
		fail(c, http.StatusConflict, ErrCampaignLocked, "message body and sending account can no longer be edited — this campaign has already sent messages")
		return
	}
	if touchesPacing && !purecampaign.CanEditPacing(purecampaign.Status(camp.Status)) {
		fail(c, http.StatusConflict, ErrCampaignLocked, "name, pace, windows and recipients can only be edited while the campaign is draft, scheduled, or paused")
		return
	}

	updated, err := s.store.UpdateCampaign(ctx(c), camp.ID, patch)
	if err != nil {
		fail(c, http.StatusInternalServerError, ErrInternal, err.Error())
		return
	}
	ok(c, s.campaignDTO(c, updated))
}

func (s *Server) handleDuplicateCampaign(c *gin.Context) {
	id, okID := parseUUID(c, "id")
	if !okID {
		return
	}
	camp, okC := s.orgCampaign(c, id)
	if !okC {
		return
	}
	dup, err := s.store.DuplicateCampaign(ctx(c), camp.ID, currentUser(c).ID)
	if err != nil {
		fail(c, http.StatusInternalServerError, ErrInternal, err.Error())
		return
	}
	created(c, s.campaignDTO(c, dup))
}

// handleDeleteCampaign removes a campaign the operator no longer wants in
// the list — in practice a draft abandoned partway through the wizard, which
// creates the campaign up front so the recipient preview has a real id to
// resolve its channel against.
//
// A running or paused campaign must be stopped first rather than deleted out
// from under the scheduler, and a campaign that already delivered is never
// removable at all: store.DeleteCampaign refuses one with send-ledger rows,
// because that ledger is what the per-account rate limiter counts against.
func (s *Server) handleDeleteCampaign(c *gin.Context) {
	id, okID := parseUUID(c, "id")
	if !okID {
		return
	}
	camp, okC := s.orgCampaign(c, id)
	if !okC {
		return
	}
	switch purecampaign.Status(camp.Status) {
	case purecampaign.StatusRunning, purecampaign.StatusPaused:
		fail(c, http.StatusConflict, ErrCampaignLocked, "stop the campaign before deleting it")
		return
	}
	switch err := s.store.DeleteCampaign(ctx(c), camp.ID); {
	case err == nil:
		ok(c, gin.H{"deleted": true})
	case errors.Is(err, store.ErrCampaignHasSends):
		fail(c, http.StatusConflict, ErrCampaignLocked, "this campaign has already sent messages and cannot be deleted")
	case errors.Is(err, store.ErrNotFound):
		fail(c, http.StatusNotFound, ErrNotFound, "campaign not found")
	default:
		fail(c, http.StatusInternalServerError, ErrInternal, err.Error())
	}
}

// ---------------------------------------------------------------------------
// Recipients: preview (parse-only) and persist
// ---------------------------------------------------------------------------

// campaignRecipientsInput reads preview/recipients' shared multipart body: a
// pasted "text" field, an uploaded "file" field, or both (file wins — the
// common accidental-double-submit case, not a documented feature).
func (s *Server) campaignRecipientsInput(c *gin.Context) (string, bool) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, campaignRecipientsMaxBodyBytes)
	form, err := c.MultipartForm()
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			fail(c, http.StatusRequestEntityTooLarge, ErrValidation, "upload exceeds the maximum allowed size")
			return "", false
		}
		fail(c, http.StatusBadRequest, ErrValidation, "invalid multipart form")
		return "", false
	}
	if fhs := form.File["file"]; len(fhs) > 0 {
		f, err := fhs[0].Open()
		if err != nil {
			fail(c, http.StatusBadRequest, ErrValidation, "cannot open "+fhs[0].Filename)
			return "", false
		}
		data, readErr := io.ReadAll(io.LimitReader(f, campaignRecipientsMaxBodyBytes+1))
		_ = f.Close()
		if readErr != nil || int64(len(data)) > campaignRecipientsMaxBodyBytes {
			fail(c, http.StatusRequestEntityTooLarge, ErrValidation, "cannot read "+fhs[0].Filename)
			return "", false
		}
		return string(data), true
	}
	return firstFormValue(form, "text"), true
}

// parseCampaignRecipients parses raw against camp's own channel and applies
// the live reachability check that channel actually supports:
//
//   - whatsapp (the QR-paired whatsmeow leg): Manager.IsOnWhatsApp, a live
//     registration check against the paired phone's own connection.
//   - simulator: nothing to check. It is cold-send-capable like whatsapp,
//     but it has no provider and no connection — every parsed identity is
//     reachable by construction. Gating this branch on ColdSendCapable
//     instead of the channel itself is what made a simulator preview fail
//     with "whatsmeow: account ... is not connected".
//   - every warm-only channel: store.UnreachableForWarmChannel, an
//     existing-conversation check.
//
// ok is false only on an actual infrastructure error (already reported to
// c); "nothing to check" (an empty valid set, or no live WhatsApp manager
// configured) is not an error — the rows are returned as parsed,
// unreachability simply not checked.
func (s *Server) parseCampaignRecipients(c *gin.Context, camp store.Campaign, raw string) (purecampaign.ParseResult, bool) {
	result := purecampaign.ParseRecipients(raw, camp.Channel, purecampaign.DefaultCountryCode)

	valid := make([]string, 0, result.Valid)
	for _, r := range result.Rows {
		if r.Status == purecampaign.PreviewValid {
			valid = append(valid, r.NormalizedIdentity)
		}
	}
	if len(valid) == 0 {
		return result, true
	}

	if camp.Channel == purecampaign.ChannelSimulator {
		return result, true
	}

	if camp.Channel == purecampaign.ChannelWhatsApp {
		if s.wa == nil {
			return result, true
		}
		registered, err := s.wa.IsOnWhatsApp(ctx(c), camp.AccountID.String(), valid)
		if err != nil {
			fail(c, http.StatusInternalServerError, ErrInternal, err.Error())
			return result, false
		}
		unreachable := map[string]bool{}
		for _, id := range valid {
			if reachable, known := registered[id]; known && !reachable {
				unreachable[id] = true
			}
		}
		return purecampaign.ApplyUnreachable(result, unreachable, "not registered on WhatsApp"), true
	}

	unreachable, err := s.store.UnreachableForWarmChannel(ctx(c), camp.AccountID, valid)
	if err != nil {
		fail(c, http.StatusInternalServerError, ErrInternal, err.Error())
		return result, false
	}
	return purecampaign.ApplyUnreachable(result, unreachable, "no existing conversation on this account"), true
}

func mapCampaignPreviewResult(r purecampaign.ParseResult) dto.CampaignRecipientPreviewResult {
	out := dto.CampaignRecipientPreviewResult{Total: r.Total(), Valid: r.Valid, Invalid: r.Invalid, Duplicate: r.Duplicate}
	out.Rows = make([]dto.CampaignRecipientPreview, 0, len(r.Rows))
	for _, row := range r.Rows {
		out.Rows = append(out.Rows, dto.CampaignRecipientPreview{
			Raw: row.Raw, Name: row.Name, Attributes: row.Attributes,
			NormalizedIdentity: row.NormalizedIdentity, Status: string(row.Status), Reason: row.Reason,
		})
	}
	return out
}

// handleCampaignPreview parses and reachability-checks a pasted/uploaded
// recipient list WITHOUT persisting anything — the wizard's live preview.
func (s *Server) handleCampaignPreview(c *gin.Context) {
	id, okID := parseUUID(c, "id")
	if !okID {
		return
	}
	camp, okC := s.orgCampaign(c, id)
	if !okC {
		return
	}
	raw, okIn := s.campaignRecipientsInput(c)
	if !okIn {
		return
	}
	result, okP := s.parseCampaignRecipients(c, camp, raw)
	if !okP {
		return
	}
	ok(c, mapCampaignPreviewResult(result))
}

// handleReplaceCampaignRecipients re-parses the SAME shape preview accepts
// (never trusting a client-computed "valid" list) and persists every VALID
// row via store.ReplaceCampaignRecipients.
func (s *Server) handleReplaceCampaignRecipients(c *gin.Context) {
	id, okID := parseUUID(c, "id")
	if !okID {
		return
	}
	camp, okC := s.orgCampaign(c, id)
	if !okC {
		return
	}
	if !purecampaign.CanEditPacing(purecampaign.Status(camp.Status)) {
		fail(c, http.StatusConflict, ErrCampaignLocked, "recipients can only be edited while the campaign is draft, scheduled, or paused")
		return
	}
	raw, okIn := s.campaignRecipientsInput(c)
	if !okIn {
		return
	}
	result, okP := s.parseCampaignRecipients(c, camp, raw)
	if !okP {
		return
	}

	rows := make([]store.CampaignRecipientInput, 0, result.Valid)
	for _, r := range result.Rows {
		if r.Status != purecampaign.PreviewValid {
			continue
		}
		rows = append(rows, store.CampaignRecipientInput{
			NormalizedIdentity: r.NormalizedIdentity, RawInput: r.Raw, Name: r.Name, Attributes: r.Attributes,
		})
	}
	if err := s.store.ReplaceCampaignRecipients(ctx(c), camp.ID, rows); err != nil {
		fail(c, http.StatusInternalServerError, ErrInternal, err.Error())
		return
	}
	_ = s.store.AppendCampaignEvent(ctx(c), camp.ID, "recipients_replaced", uuid.NullUUID{UUID: currentUser(c).ID, Valid: true},
		map[string]any{"valid": result.Valid, "invalid": result.Invalid, "duplicate": result.Duplicate})
	ok(c, mapCampaignPreviewResult(result))
}

func (s *Server) handleListCampaignRecipients(c *gin.Context) {
	id, okID := parseUUID(c, "id")
	if !okID {
		return
	}
	camp, okC := s.orgCampaign(c, id)
	if !okC {
		return
	}
	limit, offset, pageNum, pageSize := s.pageParams(c)
	items, total, err := s.store.ListCampaignRecipients(ctx(c), camp.ID, c.Query("status"), limit, offset)
	if err != nil {
		fail(c, http.StatusInternalServerError, ErrInternal, err.Error())
		return
	}
	out := make([]dto.CampaignRecipient, 0, len(items))
	for _, r := range items {
		out = append(out, dto.MapCampaignRecipient(r))
	}
	ok(c, page{Items: out, Page: pageNum, PageSize: pageSize, Total: total})
}

type retryFailedCampaignRecipientsReq struct {
	RecipientIDs []string `json:"recipient_ids"`
}

func (s *Server) handleRetryFailedCampaignRecipients(c *gin.Context) {
	id, okID := parseUUID(c, "id")
	if !okID {
		return
	}
	camp, okC := s.orgCampaign(c, id)
	if !okC {
		return
	}
	var req retryFailedCampaignRecipientsReq
	_ = c.ShouldBindJSON(&req)
	ids := make([]uuid.UUID, 0, len(req.RecipientIDs))
	for _, raw := range req.RecipientIDs {
		rid, err := uuid.Parse(raw)
		if err != nil {
			fail(c, http.StatusBadRequest, ErrValidation, "invalid recipient id "+raw)
			return
		}
		ids = append(ids, rid)
	}
	n, err := s.store.RetryFailedRecipients(ctx(c), camp.ID, ids)
	if err != nil {
		fail(c, http.StatusInternalServerError, ErrInternal, err.Error())
		return
	}
	_ = s.store.AppendCampaignEvent(ctx(c), camp.ID, "retried_failed", uuid.NullUUID{UUID: currentUser(c).ID, Valid: true}, map[string]any{"count": n})
	ok(c, gin.H{"retried": n})
}

func (s *Server) handleListCampaignEvents(c *gin.Context) {
	id, okID := parseUUID(c, "id")
	if !okID {
		return
	}
	camp, okC := s.orgCampaign(c, id)
	if !okC {
		return
	}
	limit, offset, pageNum, pageSize := s.pageParams(c)
	items, total, err := s.store.ListCampaignEvents(ctx(c), camp.ID, limit, offset)
	if err != nil {
		fail(c, http.StatusInternalServerError, ErrInternal, err.Error())
		return
	}
	out := make([]dto.CampaignEvent, 0, len(items))
	for _, e := range items {
		out = append(out, dto.MapCampaignEvent(e))
	}
	ok(c, page{Items: out, Page: pageNum, PageSize: pageSize, Total: total})
}

// ---------------------------------------------------------------------------
// Lifecycle: start / pause / resume / stop
// ---------------------------------------------------------------------------

// campaignStatusTransition is pause/resume/stop's shared core — start has
// its own handler below (it also needs the pending-recipients and
// schedule_at checks CanTransition alone doesn't cover).
func (s *Server) campaignStatusTransition(c *gin.Context, to purecampaign.Status, event string) {
	id, okID := parseUUID(c, "id")
	if !okID {
		return
	}
	camp, okC := s.orgCampaign(c, id)
	if !okC {
		return
	}
	updated, err := s.store.SetCampaignStatus(ctx(c), camp.ID, to, uuid.NullUUID{UUID: currentUser(c).ID, Valid: true}, event, nil)
	if errors.Is(err, store.ErrInvalidTransition) {
		fail(c, http.StatusConflict, ErrCampaignInvalidTransition, "cannot move campaign from "+camp.Status+" to "+string(to))
		return
	}
	if err != nil {
		fail(c, http.StatusInternalServerError, ErrInternal, err.Error())
		return
	}
	s.hub.Broadcast("campaign.status_changed", dto.CampaignStatusEvent{CampaignID: updated.ID.String(), Status: updated.Status})
	ok(c, s.campaignDTO(c, updated))
}

func (s *Server) handlePauseCampaign(c *gin.Context) {
	s.campaignStatusTransition(c, purecampaign.StatusPaused, "paused")
}
func (s *Server) handleResumeCampaign(c *gin.Context) {
	s.campaignStatusTransition(c, purecampaign.StatusRunning, "resumed")
}
func (s *Server) handleStopCampaign(c *gin.Context) {
	s.campaignStatusTransition(c, purecampaign.StatusCancelled, "stopped")
}

// handleStartCampaign moves a draft (or a previously started, since-paused
// campaign has its own separate resume route) campaign to running, or to
// scheduled when it carries a future schedule_at — the campaign Scheduler
// itself promotes scheduled -> running once that time arrives (see
// backend/campaign.CanTransition's own doc comment).
func (s *Server) handleStartCampaign(c *gin.Context) {
	id, okID := parseUUID(c, "id")
	if !okID {
		return
	}
	camp, okC := s.orgCampaign(c, id)
	if !okC {
		return
	}
	counts, err := s.store.CampaignRecipientCounts(ctx(c), camp.ID)
	if err != nil {
		fail(c, http.StatusInternalServerError, ErrInternal, err.Error())
		return
	}
	if counts["pending"] == 0 {
		fail(c, http.StatusConflict, ErrCampaignEmpty, "campaign has no pending recipients to send to")
		return
	}
	to := purecampaign.StatusRunning
	if camp.ScheduleAt != nil && camp.ScheduleAt.After(time.Now()) {
		to = purecampaign.StatusScheduled
	}
	updated, err := s.store.SetCampaignStatus(ctx(c), camp.ID, to, uuid.NullUUID{UUID: currentUser(c).ID, Valid: true}, "started", nil)
	if errors.Is(err, store.ErrInvalidTransition) {
		fail(c, http.StatusConflict, ErrCampaignInvalidTransition, "cannot start campaign from status "+camp.Status)
		return
	}
	if err != nil {
		fail(c, http.StatusInternalServerError, ErrInternal, err.Error())
		return
	}
	s.hub.Broadcast("campaign.status_changed", dto.CampaignStatusEvent{CampaignID: updated.ID.String(), Status: updated.Status})
	ok(c, s.campaignDTO(c, updated))
}

// ---------------------------------------------------------------------------
// Account sending budget + limits
// ---------------------------------------------------------------------------

func (s *Server) handleAccountSendingBudget(c *gin.Context) {
	id, okID := parseUUID(c, "id")
	if !okID {
		return
	}
	acct, okAcct := s.orgAnyAccount(c, id)
	if !okAcct {
		return
	}
	budget, err := s.store.SendingBudget(ctx(c), acct.ID, acct.Channel, time.Now())
	if err != nil {
		fail(c, http.StatusInternalServerError, ErrInternal, err.Error())
		return
	}
	ok(c, dto.MapSendingBudget(budget))
}

func (s *Server) handleGetAccountSendingLimits(c *gin.Context) {
	id, okID := parseUUID(c, "id")
	if !okID {
		return
	}
	acct, okAcct := s.orgAnyAccount(c, id)
	if !okAcct {
		return
	}
	settings, err := s.store.CampaignAccountSettingsFor(ctx(c), acct.ID, acct.Channel)
	if err != nil {
		fail(c, http.StatusInternalServerError, ErrInternal, err.Error())
		return
	}
	tiers, err := s.store.CampaignAccountLimitsFor(ctx(c), acct.ID, acct.Channel)
	if err != nil {
		fail(c, http.StatusInternalServerError, ErrInternal, err.Error())
		return
	}
	windows, err := s.store.CampaignAccountWindowsFor(ctx(c), acct.ID)
	if err != nil {
		fail(c, http.StatusInternalServerError, ErrInternal, err.Error())
		return
	}
	ok(c, dto.MapCampaignAccountSettings(settings, tiers, windows))
}

type campaignTierReq struct {
	WindowSeconds int `json:"window_seconds"`
	MaxSends      int `json:"max_sends"`
}

type setAccountSendingLimitsReq struct {
	LimitMode          string              `json:"limit_mode"`
	MinIntervalSeconds int                 `json:"min_interval_seconds"`
	JitterSeconds      int                 `json:"jitter_seconds"`
	Paused             bool                `json:"paused"`
	Tiers              []campaignTierReq   `json:"tiers"`
	Windows            []scheduleWindowReq `json:"windows"`
}

func (s *Server) handleSetAccountSendingLimits(c *gin.Context) {
	id, okID := parseUUID(c, "id")
	if !okID {
		return
	}
	acct, okAcct := s.orgAnyAccount(c, id)
	if !okAcct {
		return
	}
	var req setAccountSendingLimitsReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, ErrValidation, "invalid request body")
		return
	}
	if req.LimitMode != "default" && req.LimitMode != "custom" {
		fail(c, http.StatusBadRequest, ErrValidation, "limit_mode must be default or custom")
		return
	}
	if err := purecampaign.ValidatePace(req.MinIntervalSeconds, req.JitterSeconds); err != nil {
		fail(c, http.StatusBadRequest, ErrValidation, err.Error())
		return
	}
	tiers := make([]purecampaign.Tier, 0, len(req.Tiers))
	for _, t := range req.Tiers {
		tiers = append(tiers, purecampaign.Tier{WindowSeconds: t.WindowSeconds, MaxSends: t.MaxSends})
	}
	if err := purecampaign.ValidateTiers(tiers); err != nil {
		fail(c, http.StatusBadRequest, ErrValidation, err.Error())
		return
	}
	windows := make([]store.CampaignWindowInput, 0, len(req.Windows))
	for i, w := range req.Windows {
		win := automation.Window{Weekday: time.Weekday(w.Weekday), StartMinute: w.StartMinute, EndMinute: w.EndMinute}
		if err := automation.ValidateWindow(win); err != nil {
			fail(c, http.StatusBadRequest, ErrValidation, "windows["+strconv.Itoa(i)+"]: "+err.Error())
			return
		}
		windows = append(windows, store.CampaignWindowInput{Weekday: w.Weekday, StartMinute: w.StartMinute, EndMinute: w.EndMinute})
	}

	settings, savedTiers, savedWindows, err := s.store.SetCampaignAccountLimits(ctx(c), acct.ID,
		store.CampaignAccountSettingsInput{
			LimitMode: req.LimitMode, MinIntervalSeconds: req.MinIntervalSeconds, JitterSeconds: req.JitterSeconds, Paused: req.Paused,
		}, tiers, windows)
	if err != nil {
		fail(c, http.StatusInternalServerError, ErrInternal, err.Error())
		return
	}
	ok(c, dto.MapCampaignAccountSettings(settings, savedTiers, savedWindows))
}
