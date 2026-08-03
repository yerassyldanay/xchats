package worker

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/yerassyldanay/xchats/backend/internal/queue"
)

// handleDebounceTouch is the queue handler for KindDebounceTouch.
func (w *Worker) handleDebounceTouch(ctx context.Context, t DebounceTouchTask) error {
	return w.TouchDebounce(ctx, t.ChatID, t.Channel, t.TriggerMessageID)
}

// TouchDebounce records a fresh inbound message for chatID: it upserts the
// durable chat_draft_debounce row (LEAST-capped so a chat that never falls
// quiet is still answered eventually — see store.TouchDraftDebounce) and
// (re)arms an in-process timer to fire at exactly the resulting instant. The
// row is the durability guarantee; the timer only makes the common case fast
// instead of waiting for the next sweep tick.
func (w *Worker) TouchDebounce(ctx context.Context, chatID uuid.UUID, channel string, triggerMessageID uuid.UUID) error {
	generateAfter, err := w.Store.TouchDraftDebounce(ctx, chatID, channel, triggerMessageID, w.DebounceQuiet, w.DebounceCap)
	if err != nil {
		return err
	}
	w.armDebounceTimer(chatID, time.Until(generateAfter))
	return nil
}

// armDebounceTimer (re)schedules chatID's debounce timer. A touch that
// arrives while a timer is already pending stops and replaces it — this is
// what makes each new message push the fire time back out (up to the DB's
// cap), rather than racing the earlier timer.
func (w *Worker) armDebounceTimer(chatID uuid.UUID, d time.Duration) {
	if d < 0 {
		d = 0
	}
	w.debounceMu.Lock()
	defer w.debounceMu.Unlock()
	if w.debounceTimers == nil {
		w.debounceTimers = map[uuid.UUID]*time.Timer{}
	}
	w.stopTimerLocked(chatID)
	w.debounceWG.Add(1)
	w.debounceTimers[chatID] = time.AfterFunc(d, func() {
		defer w.debounceWG.Done()
		w.fireDebounce(chatID)
	})
}

// stopTimerLocked stops chatID's pending debounce timer, if any, and removes
// it from the map. When Stop() actually prevents the callback from running,
// it compensates debounceWG for the Add(1) armDebounceTimer made — the
// callback that would have called Done() will now never run. Callers must
// hold debounceMu.
func (w *Worker) stopTimerLocked(chatID uuid.UUID) {
	t, ok := w.debounceTimers[chatID]
	if !ok {
		return
	}
	delete(w.debounceTimers, chatID)
	if t.Stop() {
		w.debounceWG.Done()
	}
}

// fireDebounce runs when a chat's debounce timer expires. It conditionally
// deletes the row only if it is actually due: Timer.Reset/Stop cannot fully
// rule out a race against a touch that landed just as this fired, so the
// delete's WHERE clause — not the timer — is the real guard. A miss (ok=false)
// means a newer touch pushed generate_after out after this timer was
// scheduled; that newer timer will fire later and finish the job.
func (w *Worker) fireDebounce(chatID uuid.UUID) {
	w.debounceMu.Lock()
	delete(w.debounceTimers, chatID)
	w.debounceMu.Unlock()

	ctx := context.Background()
	row, ok, err := w.Store.FireDueDraftDebounce(ctx, chatID)
	if err != nil {
		w.Log.Error("debounce fire", "chat_id", chatID, "err", err)
		return
	}
	if !ok {
		return
	}
	_ = w.publish(ctx, queue.Message{Kind: queue.KindAIDraft, Payload: AIDraftTask{
		ChatID: row.ChatID, FromDebounce: true,
	}})
}

// RunDebounceSweep claims every debounce row whose generate_after has
// passed and publishes its draft generation — the recovery path for a
// message that arrived shortly before a restart (the in-process timer map is
// empty on a fresh process) and the periodic safety net alongside it.
func (w *Worker) RunDebounceSweep(ctx context.Context, limit int) (int, error) {
	rows, err := w.Store.ClaimDueDraftDebounce(ctx, limit)
	if err != nil {
		return 0, err
	}
	for _, row := range rows {
		w.debounceMu.Lock()
		w.stopTimerLocked(row.ChatID)
		w.debounceMu.Unlock()
		if err := w.publish(ctx, queue.Message{Kind: queue.KindAIDraft, Payload: AIDraftTask{
			ChatID: row.ChatID, FromDebounce: true,
		}}); err != nil {
			w.Log.Error("debounce sweep publish failed", "chat_id", row.ChatID, "err", err)
		}
	}
	if len(rows) > 0 {
		w.Log.Info("debounce sweep", "fired", len(rows))
	}
	return len(rows), nil
}

// StopDebounceTimers cancels every pending debounce timer and returns a
// channel that closes once every in-flight callback (one that had already
// started firing when Stop lost that race) has finished. Call after
// httpServer.Shutdown (no new HTTP requests, so no new KindDebounceTouch task
// can arm a fresh timer) and before queue.Close() — bounded, like the
// sweeper done channels, and for the same reason: a debounce firing
// mid-shutdown must never publish on a closed queue. (queue.InMem.Publish
// now also recovers from that race defensively, but stopping timers first is
// still what keeps a live process from doing pointless post-shutdown work.)
func (w *Worker) StopDebounceTimers() <-chan struct{} {
	w.debounceMu.Lock()
	for chatID := range w.debounceTimers {
		w.stopTimerLocked(chatID)
	}
	w.debounceMu.Unlock()
	done := make(chan struct{})
	go func() {
		w.debounceWG.Wait()
		close(done)
	}()
	return done
}
