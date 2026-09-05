package automation

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	pureautomation "github.com/yerassyldanay/xchats/backend/automation"
	"github.com/yerassyldanay/xchats/backend/internal/dto"
	"github.com/yerassyldanay/xchats/backend/internal/queue"
	"github.com/yerassyldanay/xchats/backend/internal/store"
	"github.com/yerassyldanay/xchats/backend/internal/worker"
	"github.com/yerassyldanay/xchats/backend/messaging"
	"github.com/yerassyldanay/xchats/backend/response"
)

// maxDispatchAttempts bounds retries for a dispatch job that keeps failing
// (a persistent generation error, not staleness — staleness is a clean
// discard, never a failure). Past this, the job is dropped with an error
// log rather than retried forever — the next customer message re-arms a
// fresh burst anyway.
const maxDispatchAttempts = 5

// Broadcaster is the realtime surface Runner needs — satisfied by
// *realtime.Hub with zero adapter code, mirroring internal/tgingest's own
// Broadcaster.
type Broadcaster interface {
	Broadcast(name string, data any)
}

// Runner executes one dispatch job: generate a draft (version-gated against
// the chat's current burst so a superseded generation is discarded, never
// shown), and — in scheduled_auto mode, in-window, after an atomic recheck —
// auto-send it through the existing channel-neutral outbound pipeline.
type Runner struct {
	Store *store.Store
	// Response is the shared multichannel response service (the SAME
	// instance production wires into internal/worker) — Runner clones it
	// per call with a version-gated Drafts repository swapped in, reusing
	// its Conversations/KnowledgeBase/Engine untouched. See
	// versionGatedDrafts.
	Response *response.Service
	Queue    queue.Queue
	Hub      Broadcaster
	Log      *slog.Logger
}

// HandleRun executes dispatchJobID. Errors are logged and handled internally
// (retried via RecordDispatchFailure up to maxDispatchAttempts, then
// dropped) rather than returned — there is no caller that would do anything
// with a returned error beyond logging it too; the dispatch worker pool
// treats every call as fire-and-forget, exactly like internal/queue's own
// panic-recovering worker loop.
func (r *Runner) HandleRun(ctx context.Context, dispatchJobID uuid.UUID) {
	job, err := r.Store.DispatchJobByID(ctx, dispatchJobID)
	if errors.Is(err, store.ErrNotFound) {
		return // already completed/invalidated (e.g. the channel was switched off)
	}
	if err != nil {
		r.Log.Error("automation: load dispatch job failed", "dispatch_job_id", dispatchJobID, "err", err)
		return
	}
	// A duplicate delivery that observes an existing owner must not run the
	// job again. It may still clean up work that was already processing when
	// an operator switched the account off.
	if job.Status == "processing" {
		settings, settingsErr := r.Store.AutomationSettingsForAccount(ctx, job.AccountID)
		if settingsErr != nil {
			r.Log.Error("automation: load settings for owned dispatch job failed", "dispatch_job_id", job.ID, "err", settingsErr)
			return
		}
		if settings.Mode == string(pureautomation.ModeOff) {
			_ = r.Store.DeleteDispatchJob(ctx, job.ID)
		}
		return
	}
	claimed, err := r.Store.ClaimDispatchJob(ctx, job.ID)
	if err != nil {
		r.Log.Error("automation: claim dispatch job failed", "dispatch_job_id", job.ID, "err", err)
		return
	}
	if !claimed {
		return // another worker already owns this queue delivery
	}

	settings, err := r.Store.AutomationSettingsForAccount(ctx, job.AccountID)
	if err != nil {
		r.giveUpOrRetry(ctx, job, fmt.Errorf("load automation settings: %w", err))
		return
	}
	if settings.Mode == string(pureautomation.ModeOff) {
		// The channel was switched off after this job was claimed —
		// invalidate rather than generate. SetAutomationSettings already
		// deletes 'pending' jobs proactively; this is the defense-in-depth
		// catch for one already 'processing' at the moment of the switch.
		_ = r.Store.DeleteDispatchJob(ctx, job.ID)
		return
	}

	persisted, discarded, err := r.generate(ctx, job)
	if err != nil {
		r.giveUpOrRetry(ctx, job, err)
		return
	}
	if discarded || len(persisted) == 0 {
		// A newer message re-armed this chat's debounce (and bumped its
		// burst version) while generation was running — that newer burst's
		// own dispatch job produces the real answer; this one contributed
		// nothing and is simply done.
		_ = r.Store.DeleteDispatchJob(ctx, job.ID)
		return
	}

	for _, p := range persisted {
		id, perr := uuid.Parse(p.ID)
		if perr != nil {
			r.Log.Error("automation: persisted draft id is not a uuid", "draft_id", p.ID, "err", perr)
			continue
		}
		d, derr := r.Store.DraftByID(ctx, id)
		if derr != nil {
			r.Log.Error("automation: reload persisted draft failed", "draft_id", p.ID, "err", derr)
			continue
		}
		r.Hub.Broadcast("ai_draft.created", dto.MapDraft(d))
		if settings.Mode == string(pureautomation.ModeScheduledAuto) {
			r.maybeAutoSend(ctx, job, d, settings)
		}
	}
	_ = r.Store.DeleteDispatchJob(ctx, job.ID)
}

// giveUpOrRetry records a real generation failure (never called for a clean
// stale-discard). Below maxDispatchAttempts the job is left 'pending' for
// the next recovery pass to retry; at the cap it is dropped with an error
// log — a stuck chat is not retried forever, and the next customer message
// arms a brand-new burst regardless.
func (r *Runner) giveUpOrRetry(ctx context.Context, job store.DispatchJob, cause error) {
	attempts, rerr := r.Store.RecordDispatchFailure(ctx, job.ID, cause.Error())
	if rerr != nil {
		r.Log.Error("automation: record dispatch failure failed", "dispatch_job_id", job.ID, "err", rerr)
	}
	if attempts >= maxDispatchAttempts {
		r.Log.Error("automation: dispatch job exceeded retry limit, dropping", "dispatch_job_id", job.ID, "chat_id", job.ChatID, "attempts", attempts, "cause", cause)
		_ = r.Store.DeleteDispatchJob(ctx, job.ID)
		return
	}
	r.Log.Warn("automation: dispatch job failed, will retry", "dispatch_job_id", job.ID, "chat_id", job.ChatID, "attempts", attempts, "cause", cause)
}

// generate runs the shared response engine for job's chat, persisting
// through a version-gated writer instead of the shared Service's own
// unconditional one — see versionGatedDrafts.
func (r *Runner) generate(ctx context.Context, job store.DispatchJob) (persisted []response.PersistedDraft, discarded bool, err error) {
	vw := &versionGatedDrafts{store: r.Store, chatID: job.ChatID, expectedVersion: job.BurstVersion}
	runSvc := *r.Response // shallow copy: same Conversations/KnowledgeBase/Engine, different Drafts
	runSvc.Drafts = vw
	persisted, err = runSvc.Respond(ctx, messaging.Channel(job.Channel), job.ChatID.String(), response.RespondOptions{})
	return persisted, vw.discarded, err
}

// maybeAutoSend applies scheduled_auto's send gate: outside the account's
// current schedule, a draft is left as a suggestion (the "otherwise display
// the suggestion" fallback); inside it, ClaimDraftForAutoSend performs one
// more atomic recheck (mode, human replies, the latest inbound message,
// draft state) immediately before claiming — any failed check falls back to
// the exact same "leave it as a suggestion" outcome, never an error.
func (r *Runner) maybeAutoSend(ctx context.Context, job store.DispatchJob, d store.Draft, settings store.AutomationSettings) {
	windows, err := r.Store.AutomationWindowsForAccount(ctx, job.AccountID)
	if err != nil {
		r.Log.Error("automation: load schedule windows failed", "account_id", job.AccountID, "err", err)
		return
	}
	if !pureautomation.InSchedule(toPureWindows(windows), time.Now().UTC()) {
		return
	}

	sent, ok, err := r.Store.ClaimDraftForAutoSend(ctx, d.ID, job.ChatID, job.AccountID, d.TriggerMessageID)
	if err != nil {
		r.Log.Error("automation: claim draft for auto-send failed", "draft_id", d.ID, "err", err)
		return
	}
	if !ok {
		return
	}
	r.dispatchSend(ctx, job, sent)
}

// dispatchSend records the auto-sent message (without clearing unread — see
// store.InsertAutomationOutbound) and hands it to the same channel-neutral
// outbound queue every manual and approved-AI send already uses.
func (r *Runner) dispatchSend(ctx context.Context, job store.DispatchJob, d store.Draft) {
	chat, err := r.Store.ChatByID(ctx, job.ChatID)
	if err != nil {
		r.Log.Error("automation: load chat for auto-send failed", "chat_id", job.ChatID, "err", err)
		return
	}
	msgID, err := r.Store.InsertAutomationOutbound(ctx, job.Channel, job.ChatID, job.AccountID, d.DraftText, previewText(d.DraftText))
	if err != nil {
		r.Log.Error("automation: insert auto-send outbound failed", "chat_id", job.ChatID, "err", err)
		return
	}
	if err := r.Store.SetDraftSent(ctx, d.ID, msgID); err != nil {
		r.Log.Error("automation: set draft sent failed", "draft_id", d.ID, "err", err)
	}
	if msg, merr := r.Store.MessageByID(ctx, msgID); merr == nil {
		r.Hub.Broadcast("message.created", dto.MapMessage(msg))
	}
	r.Hub.Broadcast("ai_draft.updated", dto.MapDraft(d))

	pctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if err := r.Queue.Publish(pctx, queue.Message{Kind: queue.KindOutboundSend, Payload: worker.OutboundTask{
		MessageID: msgID, AccountID: job.AccountID, Channel: messaging.Channel(job.Channel),
		Destination: chat.ExternalConversationRef, Text: d.DraftText,
	}}); err != nil {
		r.Log.Error("automation: enqueue auto-send failed", "message_id", msgID, "err", err)
	}
}

func toPureWindows(ws []store.AutomationWindow) []pureautomation.Window {
	out := make([]pureautomation.Window, 0, len(ws))
	for _, w := range ws {
		out = append(out, pureautomation.Window{
			Weekday:     time.Weekday(w.Weekday),
			StartMinute: w.StartMinute,
			EndMinute:   w.EndMinute,
		})
	}
	return out
}

func previewText(text string) string {
	r := []rune(text)
	if len(r) > 120 {
		return string(r[:120])
	}
	return text
}

// versionGatedDrafts implements response.DraftRepository like
// internal/responsestore.DraftRepo, except the write is skipped — discarded,
// not an error — when the chat's burst has moved on since generation
// started (see store.WriteDraftSetIfVersionCurrent). This is a deliberate,
// small duplication of DraftRepo's mapping logic rather than adding a
// version parameter to the shared response.DraftRepository interface, which
// would leak an automation-specific concern into the channel-neutral
// response package.
type versionGatedDrafts struct {
	store           *store.Store
	chatID          uuid.UUID
	expectedVersion int64
	discarded       bool
}

// draftOptionFrom flattens a response.DraftToPersist into the single
// store.DraftOption WriteDraftSetIfVersionCurrent writes — including its
// optional kb-gap diagnostic (0018_kb_gap_telemetry). Deliberately
// duplicated from internal/responsestore.DraftRepo's identical helper
// rather than shared, for the same reason this whole type is duplicated
// rather than parameterized — see this type's own doc comment.
func draftOptionFrom(draft response.DraftToPersist) store.DraftOption {
	opt := store.DraftOption{
		Ordinal:          1,
		Text:             draft.Text,
		ReplyLanguage:    draft.ReplyLanguage,
		Confidence:       draft.Confidence,
		Escalate:         draft.Escalate,
		EscalationReason: draft.EscalationReason,
	}
	if gap := draft.KBGap; gap != nil {
		opt.KBGapReasonCode = gap.ReasonCode
		opt.KBGapTargetEntityType = gap.TargetEntityType
		opt.KBGapTargetEntityRef = gap.TargetEntityRef
		opt.KBGapMissingFields = gap.MissingFields
		opt.KBGapSource = gap.Source
	}
	return opt
}

func (d *versionGatedDrafts) ReplaceSuggested(ctx context.Context, draft response.DraftToPersist) ([]response.PersistedDraft, error) {
	chatID, err := uuid.Parse(draft.ConversationID)
	if err != nil {
		return nil, fmt.Errorf("automation: invalid conversation id %q: %w", draft.ConversationID, err)
	}
	var trigger uuid.NullUUID
	if draft.TriggerMessageID != "" {
		tid, err := uuid.Parse(draft.TriggerMessageID)
		if err != nil {
			return nil, fmt.Errorf("automation: invalid trigger message id %q: %w", draft.TriggerMessageID, err)
		}
		trigger = uuid.NullUUID{UUID: tid, Valid: true}
	}

	written, ok, err := d.store.WriteDraftSetIfVersionCurrent(ctx, string(draft.Channel), chatID, d.expectedVersion, trigger, []store.DraftOption{
		draftOptionFrom(draft),
	})
	if err != nil {
		return nil, fmt.Errorf("automation: write draft: %w", err)
	}
	if !ok {
		d.discarded = true
		return nil, nil
	}

	out := make([]response.PersistedDraft, 0, len(written))
	for _, p := range written {
		out = append(out, response.PersistedDraft{
			ID:               p.ID.String(),
			ConversationID:   p.ChatID.String(),
			TriggerMessageID: nullUUIDString(p.TriggerMessageID),
			Text:             p.DraftText,
			ReplyLanguage:    p.ReplyLanguage,
			Confidence:       p.Confidence,
			Escalate:         p.Escalate,
			EscalationReason: p.EscalationReason,
			CreatedAt:        p.CreatedAt,
		})
	}
	return out, nil
}

func nullUUIDString(id uuid.NullUUID) string {
	if !id.Valid {
		return ""
	}
	return id.UUID.String()
}
