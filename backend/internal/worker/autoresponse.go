package worker

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/yerassyldanay/xchats/backend/internal/autoresponse"
	"github.com/yerassyldanay/xchats/backend/internal/queue"
	"github.com/yerassyldanay/xchats/backend/internal/store"
	"github.com/yerassyldanay/xchats/backend/messaging"
	"github.com/yerassyldanay/xchats/backend/response"
)

// ProcessDueAutoResponseJobs claims up to limit due jobs and fires each
// through its guard chain. Returns how many were claimed this call (fired,
// or terminally cancelled/failed) — a recovery/test seam mirroring
// RunDebounceSweep.
func (w *Worker) ProcessDueAutoResponseJobs(ctx context.Context, limit int) (int, error) {
	jobs, err := w.Store.ClaimDueAutoResponseJobs(ctx, limit, w.autoResponseMaxDeliveryLag())
	if err != nil {
		return 0, err
	}
	for _, job := range jobs {
		w.fireAutoResponseJob(ctx, job)
	}
	return len(jobs), nil
}

// fireAutoResponseJob runs one claimed job through its guard chain, in
// order. Each failed guard is a terminal cancel with a specific reason; a
// send error is a terminal fail. Nothing here is ever retried — a
// cancelled/failed job just leaves the suggestion sitting in the panel for
// an operator, which is the fail-safe direction.
func (w *Worker) fireAutoResponseJob(ctx context.Context, job store.AutoResponseJob) {
	if job.State == "cancelled" {
		// ClaimDueAutoResponseJobs already cancelled this one (too_late) in
		// the same statement that claimed the batch.
		return
	}

	account, err := w.Store.AccountByID(ctx, job.AccountID)
	if err != nil || !account.OrganizationID.Valid || account.OrganizationID.UUID != job.OrganizationID {
		w.cancelJob(ctx, job.ID, "account_gone")
		return
	}
	policy, err := w.Store.AutoResponsePolicy(ctx, job.AccountID)
	if err != nil || !policy.Enabled {
		w.cancelJob(ctx, job.ID, "policy_off")
		return
	}
	chat, err := w.Store.ChatByID(ctx, job.ChatID)
	if err != nil {
		w.cancelJob(ctx, job.ID, "account_gone")
		return
	}
	if policy.PauseWhenAssigned && chat.AssigneeUserID.Valid {
		w.cancelJob(ctx, job.ID, "assigned")
		return
	}
	if policy.CooldownSeconds > 0 {
		since := time.Now().Add(-time.Duration(policy.CooldownSeconds) * time.Second)
		recent, err := w.Store.HasOutboundSince(ctx, job.ChatID, since)
		if err != nil {
			w.Log.Error("auto-response: cooldown check", "job_id", job.ID, "err", err)
			w.failJob(ctx, job.ID, err.Error())
			return
		}
		if recent {
			w.cancelJob(ctx, job.ID, "cooldown")
			return
		}
	}

	if !job.DraftID.Valid {
		w.cancelJob(ctx, job.ID, "draft_stale")
		return
	}
	draft, err := w.Store.DraftByID(ctx, job.DraftID.UUID)
	if err != nil {
		w.cancelJob(ctx, job.ID, "draft_stale")
		return
	}

	// Escalation, above the mode switch so the UI's ai/fixed choice stays
	// honest about what it actually skips.
	if draft.Escalate {
		engineError := strings.HasPrefix(draft.EscalationReason, response.EngineErrorPrefix)
		if policy.ReplyMode == autoresponse.ReplyModeFixed {
			// A canned away-message has no dependency on the LLM having
			// worked, so an engine_error escalation never blocks it — only a
			// model's OWN evaluated escalation does, and only when configured to.
			if policy.SkipWhenEscalated && !engineError {
				w.cancelJob(ctx, job.ID, "escalated")
				return
			}
		} else {
			// engine_error is unconditional: holdingDraft sets it on ANY
			// KB/LLM failure, so without this an outage would broadcast the
			// holding text to every customer on every configured account,
			// regardless of skip_when_escalated.
			if policy.SkipWhenEscalated || engineError {
				w.cancelJob(ctx, job.ID, "escalated")
				return
			}
		}
	}

	if policy.ReplyMode == autoresponse.ReplyModeAI && draft.DraftState != "suggested" {
		// The draft was persisted before the job was armed, so "not ready
		// yet" cannot happen — the only way it is missing is that an
		// operator already acted on it.
		w.cancelJob(ctx, job.ID, "draft_stale")
		return
	}

	msgID, text, err := w.Store.SendAutoResponse(ctx, store.SendAutoResponseArg{
		JobID: job.ID, ChatID: job.ChatID, AccountID: job.AccountID, Channel: job.Channel,
		TriggerArrivedAt: job.TriggerArrivedAt, ReplyMode: policy.ReplyMode,
		DraftID: job.DraftID.UUID, FixedText: policy.FixedText,
	})
	if err != nil {
		w.failJob(ctx, job.ID, err.Error())
		return
	}

	w.emitMessage(ctx, "message.created", msgID)
	_ = w.publish(ctx, queue.Message{Kind: queue.KindOutboundSend, Payload: OutboundTask{
		MessageID: msgID, AccountID: job.AccountID, Channel: messaging.Channel(job.Channel),
		Instance: account.InstanceName, PhoneJID: chat.ExternalConversationRef, Text: text,
	}})
}

func (w *Worker) cancelJob(ctx context.Context, jobID uuid.UUID, reason string) {
	if err := w.Store.CancelAutoResponseJob(ctx, jobID, reason); err != nil {
		w.Log.Error("auto-response: cancel job", "job_id", jobID, "reason", reason, "err", err)
	}
}

func (w *Worker) failJob(ctx context.Context, jobID uuid.UUID, lastError string) {
	if err := w.Store.FailAutoResponseJob(ctx, jobID, lastError); err != nil {
		w.Log.Error("auto-response: fail job", "job_id", jobID, "err", err)
	}
}

// --- sweeper ----------------------------------------------------------------

// StartSweeper runs one goroutine that, at startup and then on every tick:
// recovers due debounce rows, claims and fires due auto-response jobs, reaps
// stale claims, and purges old terminal job rows. Modelled on
// StartTelegramMediaSweeper.
//
// Unlike that sweeper, this one's done channel is meant to be waited on: it
// closes only once the goroutine has fully exited (ctx.Done() observed, no
// sweep in flight), so a caller can bound how long it waits before calling
// queue.Close() — queue.InMem.Publish is a bare channel send outside
// process's recover, so a sweeper publishing during shutdown would otherwise
// race a concurrent Close() and could panic.
func (w *Worker) StartSweeper(ctx context.Context, every time.Duration) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		defer close(done)
		w.sweepOnce(ctx)
		t := time.NewTicker(every)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				w.sweepOnce(ctx)
			}
		}
	}()
	return done
}

func (w *Worker) sweepOnce(ctx context.Context) {
	if _, err := w.RunDebounceSweep(ctx, w.debounceSweepBatch()); err != nil {
		w.Log.Error("debounce sweep", "err", err)
	}
	if _, err := w.ProcessDueAutoResponseJobs(ctx, w.autoResponseSweepBatch()); err != nil {
		w.Log.Error("auto-response claim sweep", "err", err)
	}
	if n, err := w.Store.ReapStaleAutoResponseClaims(ctx, w.autoResponseClaimTimeout()); err != nil {
		w.Log.Error("auto-response reap", "err", err)
	} else if n > 0 {
		w.Log.Info("auto-response reap", "failed", n)
	}
	if n, err := w.Store.PurgeAutoResponseJobs(ctx, w.autoResponseJobRetention()); err != nil {
		w.Log.Error("auto-response purge", "err", err)
	} else if n > 0 {
		w.Log.Info("auto-response purge", "deleted", n)
	}
}

// --- tuning defaults ---------------------------------------------------------
// Every one of these is normally set explicitly from a main.go const; the
// fallback here only guards a harness/test wiring that leaves a field zero.

func (w *Worker) debounceSweepBatch() int {
	if w.DebounceSweepBatch <= 0 {
		return 50
	}
	return w.DebounceSweepBatch
}

func (w *Worker) autoResponseSweepBatch() int {
	if w.AutoResponseSweepBatch <= 0 {
		return 50
	}
	return w.AutoResponseSweepBatch
}

func (w *Worker) autoResponseMaxDeliveryLag() time.Duration {
	if w.AutoResponseMaxDeliveryLag <= 0 {
		return 10 * time.Minute
	}
	return w.AutoResponseMaxDeliveryLag
}

func (w *Worker) autoResponseClaimTimeout() time.Duration {
	if w.AutoResponseClaimTimeout <= 0 {
		return 5 * time.Minute
	}
	return w.AutoResponseClaimTimeout
}

func (w *Worker) autoResponseJobRetention() time.Duration {
	if w.AutoResponseJobRetention <= 0 {
		return 30 * 24 * time.Hour
	}
	return w.AutoResponseJobRetention
}
