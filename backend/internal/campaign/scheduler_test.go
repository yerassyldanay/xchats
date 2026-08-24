package campaign

import (
	"context"
	"runtime"
	"testing"
	"time"

	"github.com/google/uuid"

	purecampaign "github.com/yerassyldanay/xchats/backend/campaign"
	"github.com/yerassyldanay/xchats/backend/internal/dbtest"
	"github.com/yerassyldanay/xchats/backend/internal/store"
	"github.com/yerassyldanay/xchats/backend/messaging"
)

func testScheduler(st *store.Store, runner *Runner, cfg Config) *Scheduler {
	return NewScheduler(st, runner, cfg, testLogger())
}

func TestSchedulerTickClaimsAndSends(t *testing.T) {
	ctx := context.Background()
	st := dbtest.New(t)
	orgID, userID, acctID := seedWAFixture(t, st)
	c := startCampaignWithFastPacing(t, st, orgID, acctID, userID, "whatsapp", "Promo", "Hi!", "77011234567")

	hub := &fakeHub{}
	senders := messaging.NewSenderRegistry()
	senders.Register(messaging.ChannelWhatsApp, &fakeSender{result: messaging.SendResult{ExternalID: "1", Delivered: true}})
	r := testRunner(st, hub, senders, t)
	s := testScheduler(st, r, Config{})

	s.tick(ctx)

	recipients, _, err := st.ListCampaignRecipients(ctx, c.ID, "sent", 50, 0)
	if err != nil {
		t.Fatalf("ListCampaignRecipients: %v", err)
	}
	if len(recipients) != 1 {
		t.Fatalf("sent recipients = %d, want 1 (tick should drain the account)", len(recipients))
	}
}

func TestSchedulerRunMaintenancePromotesScheduledCampaign(t *testing.T) {
	ctx := context.Background()
	st := dbtest.New(t)
	orgID, userID, acctID := seedWAFixture(t, st)
	c, err := st.CreateCampaign(ctx, store.Campaign{
		OrganizationID: orgID, Name: "Later", AccountID: acctID, Channel: "whatsapp",
		MessageBody: "Hi!", CreatedBy: userID,
	})
	if err != nil {
		t.Fatalf("create campaign: %v", err)
	}
	// A pending recipient, so promotion to running does not ALSO immediately
	// satisfy this same runMaintenance call's own completeIfDone sweep —
	// this test is specifically about the promotion half.
	if err := st.ReplaceCampaignRecipients(ctx, c.ID, []store.CampaignRecipientInput{
		{NormalizedIdentity: "77011234567", Name: "Aigul"},
	}); err != nil {
		t.Fatalf("replace recipients: %v", err)
	}
	if _, err := st.SetCampaignStatus(ctx, c.ID, purecampaign.StatusScheduled, uuid.NullUUID{}, "scheduled", nil); err != nil {
		t.Fatalf("move to scheduled: %v", err)
	}
	past := time.Now().UTC().Add(-1 * time.Minute)
	if _, err := st.UpdateCampaign(ctx, c.ID, store.CampaignPatch{HasScheduleAt: true, ScheduleAt: &past}); err != nil {
		t.Fatalf("set schedule_at: %v", err)
	}

	hub := &fakeHub{}
	r := testRunner(st, hub, messaging.NewSenderRegistry(), t)
	s := testScheduler(st, r, Config{})
	s.runMaintenance(ctx)

	got, err := st.CampaignByID(ctx, c.ID)
	if err != nil {
		t.Fatalf("CampaignByID: %v", err)
	}
	if got.Status != string(purecampaign.StatusRunning) {
		t.Errorf("status = %q, want running", got.Status)
	}
	if hub.count("campaign.status_changed") == 0 {
		t.Error("runMaintenance should broadcast campaign.status_changed on auto-start")
	}
}

func TestSchedulerRunMaintenanceCompletesEmptyRunningCampaign(t *testing.T) {
	ctx := context.Background()
	st := dbtest.New(t)
	orgID, userID, acctID := seedWAFixture(t, st)
	c, err := st.CreateCampaign(ctx, store.Campaign{
		OrganizationID: orgID, Name: "Empty", AccountID: acctID, Channel: "whatsapp",
		MessageBody: "Hi!", CreatedBy: userID,
	})
	if err != nil {
		t.Fatalf("create campaign: %v", err)
	}
	// No recipients at all: ClaimNextRecipient will never fire for this
	// campaign, so only the maintenance sweep (not a send's own side effect)
	// can ever complete it.
	if _, err := st.SetCampaignStatus(ctx, c.ID, purecampaign.StatusRunning, uuid.NullUUID{}, "started", nil); err != nil {
		t.Fatalf("start campaign: %v", err)
	}

	hub := &fakeHub{}
	r := testRunner(st, hub, messaging.NewSenderRegistry(), t)
	s := testScheduler(st, r, Config{})
	s.runMaintenance(ctx)

	got, err := st.CampaignByID(ctx, c.ID)
	if err != nil {
		t.Fatalf("CampaignByID: %v", err)
	}
	if got.Status != string(purecampaign.StatusCompleted) {
		t.Errorf("status = %q, want completed", got.Status)
	}
}

func TestSchedulerCheckDisconnectsAutoPausesAfterThreshold(t *testing.T) {
	ctx := context.Background()
	st := dbtest.New(t)
	orgID, userID, acctID := seedWAFixture(t, st)
	c := startCampaignWithFastPacing(t, st, orgID, acctID, userID, "whatsapp", "Promo", "Hi!", "77011234567")
	if err := st.SetAccountState(ctx, acctID, "disconnected"); err != nil {
		t.Fatalf("SetAccountState: %v", err)
	}

	hub := &fakeHub{}
	r := testRunner(st, hub, messaging.NewSenderRegistry(), t)
	// A 1-nanosecond threshold makes the SECOND observation (however soon
	// after the first) already past due, without needing to sleep out a
	// realistic threshold in a test.
	s := testScheduler(st, r, Config{DisconnectAfter: 1})

	s.checkDisconnects(ctx) // first observation: starts the timer, no pause yet
	got, err := st.CampaignByID(ctx, c.ID)
	if err != nil {
		t.Fatalf("CampaignByID: %v", err)
	}
	if got.Status != string(purecampaign.StatusRunning) {
		t.Fatalf("status after 1st observation = %q, want still running", got.Status)
	}

	time.Sleep(time.Millisecond)
	s.checkDisconnects(ctx) // second observation: threshold now exceeded
	got, err = st.CampaignByID(ctx, c.ID)
	if err != nil {
		t.Fatalf("CampaignByID: %v", err)
	}
	if got.Status != string(purecampaign.StatusPaused) {
		t.Fatalf("status after 2nd observation = %q, want paused", got.Status)
	}
	if hub.count("campaign.account_auto_paused") == 0 {
		t.Error("checkDisconnects should broadcast campaign.account_auto_paused")
	}
}

func TestSchedulerCheckDisconnectsClearsOnReconnect(t *testing.T) {
	ctx := context.Background()
	st := dbtest.New(t)
	orgID, userID, acctID := seedWAFixture(t, st)
	c := startCampaignWithFastPacing(t, st, orgID, acctID, userID, "whatsapp", "Promo", "Hi!", "77011234567")
	if err := st.SetAccountState(ctx, acctID, "disconnected"); err != nil {
		t.Fatalf("SetAccountState: %v", err)
	}

	hub := &fakeHub{}
	r := testRunner(st, hub, messaging.NewSenderRegistry(), t)
	s := testScheduler(st, r, Config{DisconnectAfter: 1})

	s.checkDisconnects(ctx) // starts the timer

	if err := st.SetAccountState(ctx, acctID, "connected"); err != nil {
		t.Fatalf("SetAccountState (reconnect): %v", err)
	}
	time.Sleep(time.Millisecond)
	s.checkDisconnects(ctx) // reconnected before the threshold fired -> timer cleared

	got, err := st.CampaignByID(ctx, c.ID)
	if err != nil {
		t.Fatalf("CampaignByID: %v", err)
	}
	if got.Status != string(purecampaign.StatusRunning) {
		t.Fatalf("status = %q, want still running (reconnected in time)", got.Status)
	}
	s.disconnectMu.Lock()
	_, tracked := s.disconnectedSince[acctID]
	s.disconnectMu.Unlock()
	if tracked {
		t.Error("disconnectedSince should have been cleared on reconnect")
	}
}

// TestSchedulerStartStopLifecycle is the only test in this package that
// actually calls Start/Stop — every other test drives tick/runMaintenance/
// checkDisconnects/prune directly. It exercises the real ticker-driven claim
// loop end to end (a running campaign's recipient gets sent), then asserts
// Stop() returns promptly after ctx is canceled and leaves no goroutines
// behind — mirrors internal/automation's own TestSchedulerStartStopLifecycle.
func TestSchedulerStartStopLifecycle(t *testing.T) {
	ctx := context.Background()
	st := dbtest.New(t)
	orgID, userID, acctID := seedWAFixture(t, st)
	c := startCampaignWithFastPacing(t, st, orgID, acctID, userID, "whatsapp", "Promo", "Hi!", "77011234567")

	hub := &fakeHub{}
	senders := messaging.NewSenderRegistry()
	senders.Register(messaging.ChannelWhatsApp, &fakeSender{result: messaging.SendResult{ExternalID: "1", Delivered: true}})
	r := testRunner(st, hub, senders, t)
	s := testScheduler(st, r, Config{TickEvery: 10 * time.Millisecond, MaintenanceEvery: time.Hour, DisconnectCheckEvery: time.Hour, PruneEvery: time.Hour})

	before := runtime.NumGoroutine()

	runCtx, cancel := context.WithCancel(ctx)
	s.Start(runCtx)

	deadline := time.After(2 * time.Second)
	for {
		recipients, _, err := st.ListCampaignRecipients(ctx, c.ID, "sent", 50, 0)
		if err != nil {
			t.Fatalf("ListCampaignRecipients: %v", err)
		}
		if len(recipients) == 1 {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("Start's tick loop never sent the recipient within 2s")
		case <-time.After(10 * time.Millisecond):
		}
	}

	cancel()
	stopped := make(chan struct{})
	go func() {
		s.Stop()
		close(stopped)
	}()
	select {
	case <-stopped:
	case <-time.After(2 * time.Second):
		t.Fatal("Stop() did not return within 2s of ctx being canceled")
	}

	var after int
	for i := 0; i < 20; i++ {
		after = runtime.NumGoroutine()
		if after <= before {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if after > before {
		t.Fatalf("goroutine leak after Stop(): before=%d after=%d", before, after)
	}
}
