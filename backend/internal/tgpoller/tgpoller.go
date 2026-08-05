// Package tgpoller is the Telegram long-polling ingress: an alternative to
// the webhook ingress (internal/httpapi + internal/tgingest) for an install
// with no public URL at all. It mirrors internal/whatsmeow's manager shape
// — a registry of per-account live sessions, replace-and-stop on token
// rotation, a boot-time reconcile-all — but a poller session additionally
// has a hard single-instance invariant a WhatsApp connection does not: two
// concurrent getUpdates calls for the same bot answer each other with a 409
// Conflict, so the old runner must FULLY exit (not just be told to) before
// a new one for the same account starts.
//
// Every update still flows through internal/tgingest.Processor — the exact
// same ingest core the webhook handler uses — so a bot's history is
// identical regardless of which ingress delivered it.
package tgpoller

import (
	"context"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/internal/store"
	"github.com/yerassyldanay/xchats/backend/internal/telegram"
	"github.com/yerassyldanay/xchats/backend/internal/tgingest"
)

// TokenSource resolves a bot's live token by account id — satisfied by
// *store.Store (TelegramBotToken) with zero adapter code.
type TokenSource interface {
	TelegramBotToken(ctx context.Context, accountID uuid.UUID) (string, error)
}

// OffsetStore is the poll-offset persistence surface — satisfied by
// *store.Store with zero adapter code.
type OffsetStore interface {
	TelegramPollOffset(ctx context.Context, accountID uuid.UUID) (int64, bool, error)
	AdvanceTelegramPollOffset(ctx context.Context, accountID uuid.UUID, updateID int64) error
}

// UpdateProcessor is the shared ingest core — satisfied by
// *tgingest.Processor with zero adapter code.
type UpdateProcessor interface {
	Process(ctx context.Context, acct store.TelegramAccount, upd *telegram.Update, raw []byte) (tgingest.Outcome, error)
}

// StateRecorder records connection-state transitions — satisfied by
// *store.Store (SetTelegramWebhookState) with zero adapter code.
type StateRecorder interface {
	SetTelegramWebhookState(ctx context.Context, id uuid.UUID, st store.TelegramWebhookState) error
}

// BotLister lists which bots should have a live poller — satisfied by
// *store.Store (ListTelegramPollBots) with zero adapter code.
type BotLister interface {
	ListTelegramPollBots(ctx context.Context) ([]store.TelegramPollBot, error)
}

// SleepFunc abstracts waiting out a backoff/retry delay — the seam tests
// override to make delays instant (or to observe what was asked for)
// instead of actually waiting. The default respects ctx: a canceled
// context returns immediately rather than completing the full delay.
type SleepFunc func(ctx context.Context, d time.Duration)

func defaultSleep(ctx context.Context, d time.Duration) {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
	case <-ctx.Done():
	}
}

// PollConfig tunes the poller loop. Zero-valued fields fall back to the
// documented defaults (see withDefaults) — a caller only sets what it wants
// to override.
type PollConfig struct {
	// TimeoutSeconds is the getUpdates long-poll wait requested from
	// Telegram. Default 50 (safely under the transport's own 30s of extra
	// deadline headroom past this value — see telegram.HTTP.GetUpdates).
	TimeoutSeconds int
	// ConflictBackoff is the delay after a 409 (another live getUpdates
	// session for the same bot — a webhook still registered elsewhere, or a
	// second poller from an overlapping deployment). Default 5s.
	ConflictBackoff time.Duration
	// MinBackoff/MaxBackoff bound the exponential-with-jitter backoff for
	// otherwise unclassified errors. Default 1s / 30s.
	MinBackoff time.Duration
	MaxBackoff time.Duration
	// MaxConsecutiveFailures is how many unclassified failures in a row
	// before the runner records a sanitized webhook_error on the account —
	// a single blip should not flip the operator-visible status. Default 5.
	MaxConsecutiveFailures int
	// DefaultRetryAfter is used on a 429 that carries no
	// parameters.retry_after of its own. Default 5s.
	DefaultRetryAfter time.Duration
	// MinRetryAfter/MaxRetryAfter clamp whatever retry_after Telegram (or
	// DefaultRetryAfter) provides, so a misbehaving upstream can never stall
	// a runner for an unreasonable span. Default 1s / 5m.
	MinRetryAfter time.Duration
	MaxRetryAfter time.Duration
}

func (c PollConfig) withDefaults() PollConfig {
	if c.TimeoutSeconds <= 0 {
		c.TimeoutSeconds = 50
	}
	if c.ConflictBackoff <= 0 {
		c.ConflictBackoff = 5 * time.Second
	}
	if c.MinBackoff <= 0 {
		c.MinBackoff = 1 * time.Second
	}
	if c.MaxBackoff <= 0 {
		c.MaxBackoff = 30 * time.Second
	}
	if c.MaxConsecutiveFailures <= 0 {
		c.MaxConsecutiveFailures = 5
	}
	if c.DefaultRetryAfter <= 0 {
		c.DefaultRetryAfter = 5 * time.Second
	}
	if c.MinRetryAfter <= 0 {
		c.MinRetryAfter = 1 * time.Second
	}
	if c.MaxRetryAfter <= 0 {
		c.MaxRetryAfter = 5 * time.Minute
	}
	return c
}

// Deps is Manager's construction input.
type Deps struct {
	TG        telegram.Client
	Tokens    TokenSource
	Offsets   OffsetStore
	Processor UpdateProcessor
	State     StateRecorder
	Bots      BotLister
	Log       *slog.Logger
	// Sleep defaults to a real, ctx-aware wait when nil.
	Sleep SleepFunc
	Poll  PollConfig
}

// runner is one account's live poller session.
type runner struct {
	spec   store.TelegramPollBot
	cancel context.CancelFunc
	done   chan struct{}
	// stopped is true once the runner has permanently given up (a 401 —
	// the token is dead) rather than merely being asked to shut down.
	// Diagnostic/test use: Upsert's replace-and-stop decision is driven
	// entirely by TokenVersion, never by this flag.
	stopped atomic.Bool
}

// Manager is the poller's composition root: a registry of per-account
// runners, plus Start/Upsert/Remove/Reconcile/Close, the httpapi
// telegram_accounts.go handlers (polling mode) and main.go drive it
// through.
type Manager struct {
	deps Deps
	log  *slog.Logger

	mu   sync.Mutex
	bots map[uuid.UUID]*runner
}

// NewManager builds a Manager. It does not start any poller yet — call
// Start for the boot-time reconcile-all.
func NewManager(d Deps) *Manager {
	if d.Log == nil {
		d.Log = slog.Default()
	}
	if d.Sleep == nil {
		d.Sleep = defaultSleep
	}
	d.Poll = d.Poll.withDefaults()
	return &Manager{deps: d, log: d.Log, bots: map[uuid.UUID]*runner{}}
}

// Start reconciles every bot ListTelegramPollBots currently returns — the
// boot-time pass that gets every eligible bot a live poller with no
// external trigger (the `go waMgr.Start(ctx)` analogue in
// internal/whatsmeow). Best-effort: a listing failure is logged and Start
// returns with nothing running.
func (m *Manager) Start(ctx context.Context) {
	bots, err := m.deps.Bots.ListTelegramPollBots(ctx)
	if err != nil {
		m.log.Error("tgpoller: list bots failed", "err", err)
		return
	}
	m.Reconcile(bots)
}

// Upsert ensures a live runner exists for spec, replacing any existing one
// whose TokenVersion differs. Same-version Upsert — including over an
// already-stopped (e.g. 401'd) runner — is a no-op; a runner revives only
// when the token actually changed underneath it.
//
// The old runner is FULLY stopped — canceled, its goroutine's exit (done)
// awaited — before the new one starts: Telegram answers a second
// concurrent getUpdates session for the same bot with a 409 Conflict, so
// overlap is not just wasteful but actively breaks both sessions. mu is
// released while waiting so Close/Remove/an Upsert for a DIFFERENT account
// are never blocked behind this one bot's shutdown.
func (m *Manager) Upsert(spec store.TelegramPollBot) {
	m.mu.Lock()
	existing, ok := m.bots[spec.Account.ID]
	if ok && existing.spec.TokenVersion == spec.TokenVersion {
		m.mu.Unlock()
		return
	}
	m.mu.Unlock()

	if ok {
		existing.cancel()
		<-existing.done
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	// Recheck: a concurrent caller (Remove, or another Upsert racing on the
	// same account) may have changed things while we waited.
	if current, stillOk := m.bots[spec.Account.ID]; stillOk {
		if current.spec.TokenVersion == spec.TokenVersion {
			return
		}
		current.cancel()
		<-current.done
	}
	m.bots[spec.Account.ID] = m.spawn(spec)
}

// Remove stops and unregisters the runner for accountID, if any.
func (m *Manager) Remove(id uuid.UUID) {
	m.mu.Lock()
	r, ok := m.bots[id]
	if ok {
		delete(m.bots, id)
	}
	m.mu.Unlock()
	if ok {
		r.cancel()
		<-r.done
	}
}

// Reconcile drives the manager's registry to exactly `want`: Upsert every
// listed bot, then Remove any currently-registered bot no longer present
// (deleted, or reassigned, since the last reconcile).
func (m *Manager) Reconcile(want []store.TelegramPollBot) {
	wantIDs := make(map[uuid.UUID]bool, len(want))
	for _, spec := range want {
		wantIDs[spec.Account.ID] = true
		m.Upsert(spec)
	}
	m.mu.Lock()
	var stale []uuid.UUID
	for id := range m.bots {
		if !wantIDs[id] {
			stale = append(stale, id)
		}
	}
	m.mu.Unlock()
	for _, id := range stale {
		m.Remove(id)
	}
}

// Close stops every running poller and waits for them to exit. Call this
// BEFORE closing the queue (the poller publishes to it via
// internal/tgingest, so it must stop producing before the queue stops
// accepting).
func (m *Manager) Close() {
	m.mu.Lock()
	runners := make([]*runner, 0, len(m.bots))
	for _, r := range m.bots {
		runners = append(runners, r)
	}
	m.bots = map[uuid.UUID]*runner{}
	m.mu.Unlock()

	for _, r := range runners {
		r.cancel()
	}
	for _, r := range runners {
		<-r.done
	}
}

// Status reports whether a runner is registered for accountID and, if so,
// whether it has permanently stopped (a 401) rather than merely shut down.
// Diagnostic/test use.
func (m *Manager) Status(accountID uuid.UUID) (registered, stopped bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.bots[accountID]
	if !ok {
		return false, false
	}
	return true, r.stopped.Load()
}

func (m *Manager) spawn(spec store.TelegramPollBot) *runner {
	ctx, cancel := context.WithCancel(context.Background())
	r := &runner{spec: spec, cancel: cancel, done: make(chan struct{})}
	go func() {
		defer close(r.done)
		m.run(ctx, r)
	}()
	return r
}

// sleep waits for d or ctx cancellation, whichever comes first. It reports
// whether the full wait completed — false means ctx was canceled and the
// caller must return, not continue its loop.
func (m *Manager) sleep(ctx context.Context, d time.Duration) bool {
	m.deps.Sleep(ctx, d)
	return ctx.Err() == nil
}

// resolveToken resolves the bot's live token, retrying a transient store
// failure with backoff. The token lives only in this local variable and
// whatever telegram.Client call it is immediately passed to — it is never
// logged. ok is false only when ctx was canceled while retrying.
func (m *Manager) resolveToken(ctx context.Context, accountID uuid.UUID) (string, bool) {
	backoff := newBackoff(m.deps.Poll.MinBackoff, m.deps.Poll.MaxBackoff)
	for {
		token, err := m.deps.Tokens.TelegramBotToken(ctx, accountID)
		if err == nil {
			return token, true
		}
		if ctx.Err() != nil {
			return "", false
		}
		m.log.Warn("tgpoller: resolve token failed; retrying", "account_id", accountID, "err", err)
		if !m.sleep(ctx, backoff.next()) {
			return "", false
		}
	}
}

// webhookDeleteResult is deleteWebhookUntilSuccess's tri-state outcome — a
// plain bool cannot distinguish "give up, the token is dead" (401, which
// the caller must treat exactly like a 401 from getUpdates itself) from
// "give up, we were asked to stop" (ctx canceled).
type webhookDeleteResult int

const (
	webhookDeleteOK webhookDeleteResult = iota
	webhookDeleteCanceled
	webhookDeleteUnauthorized
)

// deleteWebhookUntilSuccess unregisters any webhook for this bot, retrying
// transient failures with backoff — polling and webhook delivery are
// mutually exclusive, so a stale registration from a prior webhook-mode
// connect must be cleared before the first getUpdates call, not left to
// race it. A 401 here means exactly what a 401 from getUpdates means (the
// token itself is dead) and is reported rather than retried forever.
func (m *Manager) deleteWebhookUntilSuccess(ctx context.Context, token string, accountID uuid.UUID) webhookDeleteResult {
	backoff := newBackoff(m.deps.Poll.MinBackoff, m.deps.Poll.MaxBackoff)
	for {
		err := m.deps.TG.DeleteWebhook(ctx, token, false)
		if err == nil {
			return webhookDeleteOK
		}
		if ctx.Err() != nil {
			return webhookDeleteCanceled
		}
		if telegram.IsUnauthorized(err) {
			return webhookDeleteUnauthorized
		}
		m.log.Warn("tgpoller: deleteWebhook failed; retrying", "account_id", accountID, "err", err)
		if !m.sleep(ctx, backoff.next()) {
			return webhookDeleteCanceled
		}
	}
}

// recordTokenError persists the "this bot's token is dead" transition and
// marks the runner permanently stopped — the single place both the
// deleteWebhook and getUpdates 401 paths converge on, so the two can never
// drift into recording it differently.
func (m *Manager) recordTokenError(ctx context.Context, r *runner, accountID uuid.UUID, token string, err error) {
	msg := "unauthorized"
	if err != nil {
		msg = sanitizeForStorage(err, token)
	}
	if setErr := m.deps.State.SetTelegramWebhookState(ctx, accountID, store.TelegramWebhookState{
		State: "token_error", LastError: msg, Checked: true,
	}); setErr != nil {
		m.log.Warn("tgpoller: record token_error failed", "account_id", accountID, "err", setErr)
	}
	r.stopped.Store(true)
}

// run is one account's poller session, from token resolution through the
// getUpdates loop, until ctx is canceled (Upsert/Remove/Reconcile/Close) or
// the token is confirmed dead (401).
func (m *Manager) run(ctx context.Context, r *runner) {
	accountID := r.spec.Account.ID

	token, ok := m.resolveToken(ctx, accountID)
	if !ok {
		return
	}
	switch m.deleteWebhookUntilSuccess(ctx, token, accountID) {
	case webhookDeleteCanceled:
		return
	case webhookDeleteUnauthorized:
		m.recordTokenError(ctx, r, accountID, token, nil)
		return
	}
	if err := m.deps.State.SetTelegramWebhookState(ctx, accountID, store.TelegramWebhookState{
		State: "connected", Checked: true,
	}); err != nil {
		m.log.Warn("tgpoller: clear webhook state failed", "account_id", accountID, "err", err)
	}

	off, hadOffset, err := m.deps.Offsets.TelegramPollOffset(ctx, accountID)
	if err != nil {
		m.log.Error("tgpoller: read poll offset failed; starting from whatever is queued", "account_id", accountID, "err", err)
	}
	next := int64(0)
	if hadOffset {
		next = off + 1
	}

	backoff := newBackoff(m.deps.Poll.MinBackoff, m.deps.Poll.MaxBackoff)
	consecutiveFailures := 0

	for {
		if ctx.Err() != nil {
			return
		}
		updates, err := m.deps.TG.GetUpdates(ctx, token, telegram.GetUpdatesRequest{
			Offset: next, Limit: 100, TimeoutSeconds: m.deps.Poll.TimeoutSeconds,
			AllowedUpdates: telegram.DefaultAllowedUpdates,
		})
		if err != nil {
			// Classify ctx cancellation before anything else — see
			// telegram.HTTP.GetUpdates's own doc comment on why this must
			// not rely on error-string matching through redaction.
			if ctx.Err() != nil {
				return
			}
			switch {
			case telegram.IsUnauthorized(err):
				m.recordTokenError(ctx, r, accountID, token, err)
				return
			case telegram.APIErrorCode(err) == http.StatusConflict:
				// A 409 means the token itself is fine (Telegram would
				// answer 401 otherwise) — another session is the conflict —
				// but re-delete defensively anyway and still honor an
				// unauthorized verdict if the token died in the meantime.
				if m.deleteWebhookUntilSuccess(ctx, token, accountID) == webhookDeleteUnauthorized {
					m.recordTokenError(ctx, r, accountID, token, err)
					return
				}
				_ = m.deps.State.SetTelegramWebhookState(ctx, accountID, store.TelegramWebhookState{
					State: "webhook_error", LastError: sanitizeForStorage(err, token), Checked: true,
				})
				if !m.sleep(ctx, m.deps.Poll.ConflictBackoff) {
					return
				}
				continue
			case telegram.APIErrorCode(err) == http.StatusTooManyRequests:
				d, hasRetryAfter := telegram.RetryAfter(err)
				if !hasRetryAfter {
					d = m.deps.Poll.DefaultRetryAfter
				}
				d = clampDuration(d, m.deps.Poll.MinRetryAfter, m.deps.Poll.MaxRetryAfter)
				if !m.sleep(ctx, d) {
					return
				}
				continue
			default:
				consecutiveFailures++
				if consecutiveFailures >= m.deps.Poll.MaxConsecutiveFailures {
					_ = m.deps.State.SetTelegramWebhookState(ctx, accountID, store.TelegramWebhookState{
						State: "webhook_error", LastError: sanitizeForStorage(err, token), Checked: true,
					})
				}
				m.log.Warn("tgpoller: getUpdates failed", "account_id", accountID, "err", err, "consecutive_failures", consecutiveFailures)
				if !m.sleep(ctx, backoff.next()) {
					return
				}
				continue
			}
		}

		backoff.reset()
		consecutiveFailures = 0

		batchFailed := false
		for _, u := range updates {
			if u.Update == nil {
				continue
			}
			if _, ignorable := u.Update.Ignorable(); ignorable {
				if err := m.deps.Offsets.AdvanceTelegramPollOffset(ctx, accountID, u.UpdateID); err != nil {
					m.log.Error("tgpoller: advance offset (ignorable) failed", "account_id", accountID, "update_id", u.UpdateID, "err", err)
					batchFailed = true
					break
				}
				next = u.UpdateID + 1
				continue
			}

			if _, err := m.deps.Processor.Process(ctx, r.spec.Account, u.Update, u.Raw); err != nil {
				// Do NOT advance: the next GetUpdates call refetches this
				// same update_id, and it lands on the settled-duplicate
				// path once whatever failed recovers. A deterministically
				// failing update stalls this bot under backoff — the same
				// semantics as the webhook ingress's own 500-and-redeliver
				// loop.
				m.log.Error("tgpoller: process update failed; not advancing", "account_id", accountID, "update_id", u.UpdateID, "err", err)
				batchFailed = true
				break
			}
			if err := m.deps.Offsets.AdvanceTelegramPollOffset(ctx, accountID, u.UpdateID); err != nil {
				m.log.Error("tgpoller: advance offset failed", "account_id", accountID, "update_id", u.UpdateID, "err", err)
				batchFailed = true
				break
			}
			next = u.UpdateID + 1
		}

		if batchFailed {
			if !m.sleep(ctx, backoff.next()) {
				return
			}
			continue
		}
	}
}

// --- helpers ---------------------------------------------------------------

// backoff tracks an exponential-with-jitter delay sequence: each call to
// next returns a uniformly random value in [ceiling/2, ceiling], then
// doubles the ceiling (capped at max) for the following call.
type backoff struct {
	min, max time.Duration
	ceiling  time.Duration
}

func newBackoff(min, max time.Duration) *backoff {
	return &backoff{min: min, max: max, ceiling: min}
}

func (b *backoff) next() time.Duration {
	ceiling := b.ceiling
	if ceiling < b.min {
		ceiling = b.min
	}
	lo := ceiling / 2
	if lo < b.min {
		lo = b.min
	}
	delay := ceiling
	if ceiling > lo {
		delay = lo + time.Duration(rand.Int64N(int64(ceiling-lo)+1))
	}
	b.ceiling *= 2
	if b.ceiling > b.max || b.ceiling <= 0 {
		b.ceiling = b.max
	}
	return delay
}

func (b *backoff) reset() { b.ceiling = b.min }

func clampDuration(d, min, max time.Duration) time.Duration {
	if d < min {
		return min
	}
	if d > max {
		return max
	}
	return d
}

// sanitizeForStorage strips the bot token from an error message before it is
// persisted (webhook_last_error) or logged — defense in depth:
// internal/telegram's own redaction should already guarantee this, but a
// webhook_last_error column read back by an operator is exactly the kind of
// place a slip here would be worst.
func sanitizeForStorage(err error, token string) string {
	msg := err.Error()
	if token != "" {
		msg = strings.ReplaceAll(msg, token, "<redacted>")
	}
	return msg
}
