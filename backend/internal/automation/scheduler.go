// Package automation is the restart-safe orchestration around
// backend/automation's pure decisions: a Scheduler that arms/resets each
// chat's durable debounce deadline on every inbound customer message and
// promotes due deadlines into durable dispatch jobs, and a Runner that
// executes one dispatch job — generate, version-gate the write, and (in
// scheduled_auto mode, in-window, after an atomic recheck) auto-send.
//
// Both durable tables (automation_debounce_jobs, automation_dispatch_jobs —
// see internal/store/automation.go) are "the row is the queue": this package
// owns no long-lived in-memory state a crash could lose, mirroring
// internal/worker.StartMediaSweeper's own recovery philosophy. It
// deliberately does not reuse internal/queue's Kind-dispatched Queue for its
// OWN claim→generate step (a durability model built around a *different*
// durable table would just be redundant machinery); it DOES publish the
// existing queue.KindOutboundSend for an auto-send, reusing the channel-
// neutral send pipeline internal/worker.handleOutboundSend already runs for
// every manual and approved-AI send.
package automation

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"

	pureautomation "github.com/yerassyldanay/xchats/backend/automation"
	"github.com/yerassyldanay/xchats/backend/internal/store"
)

// Config is the Scheduler's one runtime tunable: the system-wide default
// debounce wait (config.yaml's system.customer_message_wait_seconds), used
// whenever a channel has no wait_seconds_override.
type Config struct {
	SystemDefaultWaitSeconds int
}

// Scheduler arms per-chat debounce deadlines and promotes due ones into
// dispatch jobs for a Runner to execute.
type Scheduler struct {
	Store  *store.Store
	Cfg    Config
	Runner *Runner
	Log    *slog.Logger

	dispatch chan uuid.UUID
	// wg tracks every goroutine Start launches (the claim/recovery loop and
	// each dispatch worker) — see Stop.
	wg sync.WaitGroup
}

// NewScheduler builds a Scheduler. dispatchBuffer sizes the in-process
// hand-off channel between the claim loop and the dispatch worker pool — a
// full buffer only ever delays a claim already durably recorded in
// automation_dispatch_jobs, never loses it (see Start's recovery pass).
func NewScheduler(st *store.Store, cfg Config, runner *Runner, log *slog.Logger, dispatchBuffer int) *Scheduler {
	if dispatchBuffer <= 0 {
		dispatchBuffer = 256
	}
	if log == nil {
		log = slog.Default()
	}
	return &Scheduler{Store: st, Cfg: cfg, Runner: runner, Log: log, dispatch: make(chan uuid.UUID, dispatchBuffer)}
}

// OnInboundMessage is called for every genuinely new inbound customer
// message on an automation-tracked channel: whatsmeow's ingestMessage and
// tgingest's Process call this in place of directly enqueueing an AI-draft
// task. In "off" mode it is a deliberate no-op — no debounce is armed, so
// the chat generates nothing until the channel is switched back on. In
// "suggestions" or "scheduled_auto" mode it atomically resets the chat's
// debounce deadline to lastMessageAt + the channel's effective wait, with no
// maximum burst cap.
func (s *Scheduler) OnInboundMessage(ctx context.Context, chatID, accountID uuid.UUID, channel string, lastMessageAt time.Time) error {
	if lastMessageAt.IsZero() {
		lastMessageAt = time.Now()
	}
	settings, err := s.Store.AutomationSettingsForAccount(ctx, accountID)
	if err != nil {
		return err
	}
	if settings.Mode == string(pureautomation.ModeOff) {
		return nil
	}
	wait := pureautomation.EffectiveWaitSeconds(s.Cfg.SystemDefaultWaitSeconds, settings.WaitSecondsOverride)
	deadline := pureautomation.Deadline(lastMessageAt, wait)
	return s.Store.ArmDebounce(ctx, chatID, accountID, channel, deadline)
}

// Start launches the dispatch worker pool and the claim/recovery loop.
// claimEvery is how often due debounce deadlines are promoted into dispatch
// jobs (short — debounce waits are single-digit-to-60 seconds, so polling
// latency should be too). recoverEvery/stuckAfter bound the boot-time-and-
// periodic reconcile pass that re-dispatches a job a crash left mid-flight
// ('processing' with no update in stuckAfter) or never started ('pending') —
// mirroring worker.StartMediaSweeper's own startup-pass-then-ticker
// shape. Every launched goroutine stops once ctx is done; call Stop
// afterward to block until they actually have (see Stop's own doc comment).
func (s *Scheduler) Start(ctx context.Context, dispatchWorkers int, claimEvery, recoverEvery, stuckAfter time.Duration) {
	if dispatchWorkers <= 0 {
		dispatchWorkers = 4
	}
	for i := 0; i < dispatchWorkers; i++ {
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			s.runDispatchWorker(ctx)
		}()
	}

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.recoverDispatchJobs(ctx, stuckAfter) // startup recovery pass
		claim := time.NewTicker(claimEvery)
		recoverTick := time.NewTicker(recoverEvery)
		defer claim.Stop()
		defer recoverTick.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-claim.C:
				s.claimDue(ctx)
			case <-recoverTick.C:
				s.recoverDispatchJobs(ctx, stuckAfter)
			}
		}
	}()
}

// Stop blocks until every goroutine Start launched has exited. Call this
// with ctx already canceled (or about to be), and BEFORE closing the
// queue: a dispatch worker mid-HandleRun publishes an auto-send's outbound
// task to it (Runner.dispatchSend), so — exactly like
// tgpoller.Manager.Close's own documented contract — this producer must
// stop before the queue stops accepting, or a send racing the queue's
// Close can panic on a send to a closed channel.
func (s *Scheduler) Stop() {
	s.wg.Wait()
}

// claimDue promotes every currently-due debounce deadline into a durable
// dispatch job and hands it to a worker.
func (s *Scheduler) claimDue(ctx context.Context) {
	jobs, err := s.Store.ClaimDueDispatchJobs(ctx, time.Now(), 200)
	if err != nil {
		s.Log.Error("automation: claim due debounce jobs failed", "err", err)
		return
	}
	for _, j := range jobs {
		s.enqueue(ctx, j.ID)
	}
}

// recoverDispatchJobs re-dispatches every job a previous process instance
// left unfinished — the row IS the durable queue; nothing else needs to
// survive a restart.
func (s *Scheduler) recoverDispatchJobs(ctx context.Context, stuckAfter time.Duration) {
	jobs, err := s.Store.RecoverableDispatchJobs(ctx, time.Now().Add(-stuckAfter), 200)
	if err != nil {
		s.Log.Error("automation: recover dispatch jobs failed", "err", err)
		return
	}
	if len(jobs) > 0 {
		s.Log.Info("automation: recovering dispatch jobs", "count", len(jobs))
	}
	for _, j := range jobs {
		s.enqueue(ctx, j.ID)
	}
}

// enqueue hands a dispatch job id to the worker pool, bounded so a full
// buffer cannot block the claim/recovery loop forever — a dropped enqueue
// here is never a lost job (it stays durably 'pending'/'processing' in
// automation_dispatch_jobs and is picked up by the next recovery pass).
func (s *Scheduler) enqueue(ctx context.Context, dispatchJobID uuid.UUID) {
	select {
	case s.dispatch <- dispatchJobID:
	case <-ctx.Done():
	case <-time.After(2 * time.Second):
		s.Log.Warn("automation: dispatch worker pool full; leaving job for the next recovery pass", "dispatch_job_id", dispatchJobID)
	}
}

func (s *Scheduler) runDispatchWorker(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case id, ok := <-s.dispatch:
			if !ok {
				return
			}
			s.Runner.HandleRun(ctx, id)
		}
	}
}
