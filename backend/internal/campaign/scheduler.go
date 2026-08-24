package campaign

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"

	purecampaign "github.com/yerassyldanay/xchats/backend/campaign"
	"github.com/yerassyldanay/xchats/backend/internal/dto"
	"github.com/yerassyldanay/xchats/backend/internal/store"
)

// Config is the Scheduler's runtime tunables — see each field's own doc
// comment. NewScheduler fills in a sane default for every zero-valued
// field, mirroring internal/automation.NewScheduler's own dispatchBuffer
// convention.
type Config struct {
	// TickEvery is how often the claim/send pass runs across every account
	// with a RUNNING campaign. The account's own pacing (min interval,
	// jitter, tiers) — not this interval — governs the real send cadence:
	// see Scheduler.tick's own doc comment.
	TickEvery time.Duration
	// MaintenanceEvery is how often two low-frequency, non-realtime passes
	// run: promoting a 'scheduled' campaign whose schedule_at has arrived,
	// and completing a 'running' campaign left with nothing pending or
	// mid-send (the common case is already handled as a send's own side
	// effect — see Runner.completeIfDone — so this pass only matters for a
	// campaign that never had an eligible recipient to claim at all).
	MaintenanceEvery time.Duration
	// DisconnectCheckEvery is how often every account with a RUNNING
	// campaign has its connection state sampled.
	DisconnectCheckEvery time.Duration
	// DisconnectAfter is how long an account must be CONTINUOUSLY observed
	// disconnected (sampled every DisconnectCheckEvery) before its running
	// campaigns are auto-paused. Tracked in-memory, not durably — a restart
	// simply restarts the timer, which only ever delays an auto-pause that
	// is itself a soft operational safety net, never a correctness
	// requirement: a disconnected account's ClaimNextRecipient already only
	// ever wastes an attempt (the provider call fails, the recipient is
	// retried later), it never sends anything wrong.
	DisconnectAfter time.Duration
	// PruneEvery is how often campaign_send_log is swept for rows older
	// than pruneRetention. See PruneCampaignSendLog and
	// backend/campaign.MaxTierWindowSeconds' own doc comments for why that
	// retention is load-bearing and therefore not a Config field.
	PruneEvery time.Duration
	// AccountConcurrency bounds how many accounts' claim/send passes run
	// concurrently within one tick.
	AccountConcurrency int
}

func (c Config) withDefaults() Config {
	if c.TickEvery <= 0 {
		c.TickEvery = 2 * time.Second
	}
	if c.MaintenanceEvery <= 0 {
		c.MaintenanceEvery = 15 * time.Second
	}
	if c.DisconnectCheckEvery <= 0 {
		c.DisconnectCheckEvery = 15 * time.Second
	}
	if c.DisconnectAfter <= 0 {
		c.DisconnectAfter = 60 * time.Second
	}
	if c.PruneEvery <= 0 {
		c.PruneEvery = time.Hour
	}
	if c.AccountConcurrency <= 0 {
		c.AccountConcurrency = 8
	}
	return c
}

// pruneRetention is PruneEvery's own retention window — pinned to
// backend/campaign.MaxTierWindowSeconds (7 days): see that constant's own
// doc comment for why a shorter retention would corrupt a wide tier's live
// usage count, and why this is therefore never a free Config tunable.
var pruneRetention = time.Duration(purecampaign.MaxTierWindowSeconds) * time.Second

// Scheduler is the restart-safe orchestration around Runner: it discovers
// which accounts currently have work and drains each one's eligible
// recipients, promotes due 'scheduled' campaigns, watches for and
// auto-pauses a persistently disconnected account's campaigns, and
// periodically prunes the operational send ledger.
type Scheduler struct {
	Store  *store.Store
	Runner *Runner
	Cfg    Config
	Log    *slog.Logger

	wg sync.WaitGroup

	disconnectMu      sync.Mutex
	disconnectedSince map[uuid.UUID]time.Time
}

// NewScheduler builds a Scheduler ready for Start.
func NewScheduler(st *store.Store, runner *Runner, cfg Config, log *slog.Logger) *Scheduler {
	if log == nil {
		log = slog.Default()
	}
	return &Scheduler{
		Store: st, Runner: runner, Cfg: cfg.withDefaults(), Log: log,
		disconnectedSince: map[uuid.UUID]time.Time{},
	}
}

// Start reconciles crash-interrupted sends once (ReconcileStuckSending),
// then launches the tick/maintenance/disconnect-watch/prune loops. Every
// launched goroutine stops once ctx is done; call Stop afterward to block
// until they actually have — and, exactly like
// internal/automation.Scheduler.Stop's own documented contract, do so
// BEFORE anything Runner.Hub/Runner.Senders depend on is torn down, since a
// tick already in flight when ctx is canceled still finishes its current
// claim.
func (s *Scheduler) Start(ctx context.Context) {
	if n, err := s.Store.ReconcileStuckSending(ctx); err != nil {
		s.Log.Error("campaign: reconcile stuck sending failed", "err", err)
	} else if n > 0 {
		s.Log.Warn("campaign: reconciled interrupted sends", "count", n)
	}

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		tick := time.NewTicker(s.Cfg.TickEvery)
		maintenance := time.NewTicker(s.Cfg.MaintenanceEvery)
		disconnect := time.NewTicker(s.Cfg.DisconnectCheckEvery)
		prune := time.NewTicker(s.Cfg.PruneEvery)
		defer tick.Stop()
		defer maintenance.Stop()
		defer disconnect.Stop()
		defer prune.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
				s.tick(ctx)
			case <-maintenance.C:
				s.runMaintenance(ctx)
			case <-disconnect.C:
				s.checkDisconnects(ctx)
			case <-prune.C:
				s.prune(ctx)
			}
		}
	}()
}

// Stop blocks until every goroutine Start launched has exited.
func (s *Scheduler) Stop() {
	s.wg.Wait()
}

// tick attempts a claim/send cycle for every account currently running a
// campaign, bounded to AccountConcurrency concurrent accounts. Each account
// is DRAINED — claimed repeatedly until a claim reports nothing left to
// send right now — so TickEvery only controls how promptly a newly running
// campaign (or a newly due retry) is noticed; the actual send cadence is
// entirely governed by ClaimNextRecipient's own pacing math.
func (s *Scheduler) tick(ctx context.Context) {
	accountIDs, err := s.Store.ListRunningCampaignAccounts(ctx)
	if err != nil {
		s.Log.Error("campaign: list running accounts failed", "err", err)
		return
	}
	sem := make(chan struct{}, s.Cfg.AccountConcurrency)
	var wg sync.WaitGroup
	for _, id := range accountIDs {
		select {
		case <-ctx.Done():
			wg.Wait()
			return
		case sem <- struct{}{}:
		}
		wg.Add(1)
		go func(accountID uuid.UUID) {
			defer wg.Done()
			defer func() { <-sem }()
			s.drainAccount(ctx, accountID)
		}(id)
	}
	wg.Wait()
}

// drainAccountMaxClaims bounds one tick's worth of claims for a single
// account — generous enough that a burst of newly-due retries (or a
// just-resumed campaign) catches up quickly, but never lets one account's
// tight pacing starve that tick's other accounts indefinitely.
const drainAccountMaxClaims = 20

func (s *Scheduler) drainAccount(ctx context.Context, accountID uuid.UUID) {
	for i := 0; i < drainAccountMaxClaims; i++ {
		select {
		case <-ctx.Done():
			return
		default:
		}
		if !s.Runner.HandleAccount(ctx, accountID) {
			return
		}
	}
}

// runMaintenance promotes every 'scheduled' campaign whose schedule_at has
// arrived, and completes every 'running' campaign left with nothing pending
// or mid-send — see Config.MaintenanceEvery's own doc comment.
func (s *Scheduler) runMaintenance(ctx context.Context) {
	due, err := s.Store.DueScheduledCampaigns(ctx, time.Now())
	if err != nil {
		s.Log.Error("campaign: list due scheduled campaigns failed", "err", err)
	}
	for _, id := range due {
		if _, err := s.Store.SetCampaignStatus(ctx, id, purecampaign.StatusRunning, uuid.NullUUID{}, "auto_started",
			map[string]any{"reason": "schedule_at reached"}); err != nil {
			s.Log.Error("campaign: auto-start scheduled campaign failed", "campaign_id", id, "err", err)
			continue
		}
		s.Runner.Hub.Broadcast("campaign.status_changed", dto.CampaignStatusEvent{CampaignID: id.String(), Status: string(purecampaign.StatusRunning)})
	}

	running, err := s.Store.RunningCampaignIDs(ctx)
	if err != nil {
		s.Log.Error("campaign: list running campaigns failed", "err", err)
		return
	}
	for _, id := range running {
		s.Runner.completeIfDone(ctx, id)
	}
}

// checkDisconnects samples every account currently running a campaign; once
// one has been continuously observed non-'connected' for at least
// DisconnectAfter, every one of its running campaigns is auto-paused.
func (s *Scheduler) checkDisconnects(ctx context.Context) {
	accountIDs, err := s.Store.ListRunningCampaignAccounts(ctx)
	if err != nil {
		s.Log.Error("campaign: list running accounts failed", "err", err)
		return
	}
	seen := make(map[uuid.UUID]bool, len(accountIDs))
	now := time.Now()
	for _, id := range accountIDs {
		seen[id] = true
		acct, err := s.Store.AccountByID(ctx, id)
		if err != nil {
			s.Log.Error("campaign: load account for disconnect check failed", "account_id", id, "err", err)
			continue
		}
		if acct.ConnectionState == "connected" {
			s.clearDisconnect(id)
			continue
		}
		since := s.markDisconnect(id, now)
		if now.Sub(since) < s.Cfg.DisconnectAfter {
			continue
		}
		s.clearDisconnect(id)
		n, err := s.Store.AutoPauseCampaignsForAccount(ctx, id, "account disconnected for "+s.Cfg.DisconnectAfter.String())
		if err != nil {
			s.Log.Error("campaign: auto-pause failed", "account_id", id, "err", err)
			continue
		}
		if n > 0 {
			s.Log.Warn("campaign: auto-paused campaigns on disconnected account", "account_id", id, "count", n)
			s.Runner.Hub.Broadcast("campaign.account_auto_paused", accountAutoPausedEvent{AccountID: id.String(), Count: n})
		}
	}
	s.forgetReconnected(seen)
}

// markDisconnect records now as accountID's first-observed-disconnected time
// if it isn't already tracked, and returns that (possibly earlier) time.
func (s *Scheduler) markDisconnect(accountID uuid.UUID, now time.Time) time.Time {
	s.disconnectMu.Lock()
	defer s.disconnectMu.Unlock()
	since, ok := s.disconnectedSince[accountID]
	if !ok {
		s.disconnectedSince[accountID] = now
		return now
	}
	return since
}

func (s *Scheduler) clearDisconnect(accountID uuid.UUID) {
	s.disconnectMu.Lock()
	defer s.disconnectMu.Unlock()
	delete(s.disconnectedSince, accountID)
}

// forgetReconnected drops any tracked account that no longer has a running
// campaign at all — otherwise a resumed-then-later-re-disconnected account
// could inherit a stale timer from a previous, unrelated disconnect.
func (s *Scheduler) forgetReconnected(stillRunning map[uuid.UUID]bool) {
	s.disconnectMu.Lock()
	defer s.disconnectMu.Unlock()
	for id := range s.disconnectedSince {
		if !stillRunning[id] {
			delete(s.disconnectedSince, id)
		}
	}
}

func (s *Scheduler) prune(ctx context.Context) {
	n, err := s.Store.PruneCampaignSendLog(ctx, time.Now().Add(-pruneRetention))
	if err != nil {
		s.Log.Error("campaign: prune send log failed", "err", err)
		return
	}
	if n > 0 {
		s.Log.Info("campaign: pruned send log", "count", n)
	}
}

// accountAutoPausedEvent is broadcast whenever a disconnected account's
// running campaigns are auto-paused — ids and a count only.
type accountAutoPausedEvent struct {
	AccountID string `json:"account_id"`
	Count     int    `json:"count"`
}
