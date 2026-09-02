package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	purecampaign "github.com/yerassyldanay/xchats/backend/campaign"
	"github.com/yerassyldanay/xchats/backend/internal/dbtest"
	"github.com/yerassyldanay/xchats/backend/internal/store"
)

// seedCampaignFixture seeds an org, an admin user, and a wa_accounts row
// (channel defaults to "whatsapp") — the common starting point for every
// test in this file.
func seedCampaignFixture(t *testing.T, st *store.Store, ctx context.Context) (orgID, userID, accountID uuid.UUID) {
	t.Helper()
	org, err := st.SeedOrganization(ctx, "campaigns-test-org-"+uuid.NewString())
	if err != nil {
		t.Fatalf("seed org: %v", err)
	}
	user, err := st.SeedUser(ctx, org.ID, uuid.NewString()+"@example.com", "hash", "Tester")
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}
	acctID := uuid.New()
	acct, err := st.SeedAccount(ctx, store.Account{
		ID: acctID, OrganizationID: uuid.NullUUID{UUID: org.ID, Valid: true},
		DisplayName: "Test WA", ExternalAccountRef: "7770000000@s.whatsapp.net", ExternalHandle: "77700000000",
		ConnectionState: "connected",
	})
	if err != nil {
		t.Fatalf("seed account: %v", err)
	}
	return org.ID, user.ID, acct.ID
}

func mustCreateCampaign(t *testing.T, st *store.Store, ctx context.Context, orgID, accountID, userID uuid.UUID, name, body string) store.Campaign {
	t.Helper()
	c, err := st.CreateCampaign(ctx, store.Campaign{
		OrganizationID: orgID, Name: name, AccountID: accountID, Channel: "whatsapp",
		MessageBody: body, CreatedBy: userID,
	})
	if err != nil {
		t.Fatalf("create campaign: %v", err)
	}
	return c
}

func TestCreateAndReadCampaign(t *testing.T) {
	ctx := context.Background()
	st := dbtest.New(t)
	orgID, userID, acctID := seedCampaignFixture(t, st, ctx)

	c := mustCreateCampaign(t, st, ctx, orgID, acctID, userID, "Summer promo", "Hi {{name}}, 20% off!")
	if c.Status != "draft" {
		t.Errorf("Status = %q, want draft", c.Status)
	}
	if len(c.Variables) != 1 || c.Variables[0] != "name" {
		t.Errorf("Variables = %v, want [name]", c.Variables)
	}

	got, err := st.CampaignByID(ctx, c.ID)
	if err != nil {
		t.Fatalf("CampaignByID: %v", err)
	}
	if got.Name != "Summer promo" {
		t.Errorf("Name = %q, want Summer promo", got.Name)
	}

	if _, err := st.CampaignByIDForOrg(ctx, c.ID, uuid.New()); err != store.ErrNotFound {
		t.Errorf("CampaignByIDForOrg with wrong org = %v, want ErrNotFound", err)
	}
	if _, err := st.CampaignByIDForOrg(ctx, c.ID, orgID); err != nil {
		t.Errorf("CampaignByIDForOrg with correct org: %v", err)
	}

	list, total, err := st.ListCampaignsForOrg(ctx, orgID, 50, 0)
	if err != nil {
		t.Fatalf("ListCampaignsForOrg: %v", err)
	}
	if total != 1 || len(list) != 1 {
		t.Errorf("ListCampaignsForOrg = %d/%d, want 1/1", len(list), total)
	}
}

func TestUpdateCampaign(t *testing.T) {
	ctx := context.Background()
	st := dbtest.New(t)
	orgID, userID, acctID := seedCampaignFixture(t, st, ctx)
	c := mustCreateCampaign(t, st, ctx, orgID, acctID, userID, "Original", "Hi!")

	newName := "Renamed"
	newBody := "Hi {{name}} and {{city}}!"
	updated, err := st.UpdateCampaign(ctx, c.ID, store.CampaignPatch{Name: &newName, MessageBody: &newBody})
	if err != nil {
		t.Fatalf("UpdateCampaign: %v", err)
	}
	if updated.Name != "Renamed" || updated.MessageBody != newBody {
		t.Errorf("updated = %+v", updated)
	}
	if len(updated.Variables) != 2 {
		t.Errorf("Variables = %v, want 2 entries", updated.Variables)
	}

	// Set a pace override.
	interval, jitter := 45, 10
	updated, err = st.UpdateCampaign(ctx, c.ID, store.CampaignPatch{HasPace: true, MinIntervalSeconds: &interval, JitterSeconds: &jitter})
	if err != nil {
		t.Fatalf("UpdateCampaign (set pace): %v", err)
	}
	if updated.MinIntervalSeconds == nil || *updated.MinIntervalSeconds != 45 {
		t.Errorf("MinIntervalSeconds = %v, want 45", updated.MinIntervalSeconds)
	}

	// Clear the pace override.
	updated, err = st.UpdateCampaign(ctx, c.ID, store.CampaignPatch{HasPace: true, MinIntervalSeconds: nil, JitterSeconds: nil})
	if err != nil {
		t.Fatalf("UpdateCampaign (clear pace): %v", err)
	}
	if updated.MinIntervalSeconds != nil {
		t.Errorf("MinIntervalSeconds after clear = %v, want nil", updated.MinIntervalSeconds)
	}

	// A patch that doesn't touch pace must leave it alone.
	otherName := "Still renamed"
	updated, err = st.UpdateCampaign(ctx, c.ID, store.CampaignPatch{Name: &otherName})
	if err != nil {
		t.Fatalf("UpdateCampaign (no pace touch): %v", err)
	}
	if updated.MinIntervalSeconds != nil {
		t.Errorf("MinIntervalSeconds after untouched patch = %v, want nil", updated.MinIntervalSeconds)
	}

	// Windows replace.
	updated, err = st.UpdateCampaign(ctx, c.ID, store.CampaignPatch{HasWindows: true, Windows: []store.CampaignWindowInput{
		{Weekday: 1, StartMinute: 540, EndMinute: 1020},
	}})
	if err != nil {
		t.Fatalf("UpdateCampaign (windows): %v", err)
	}
	windows, err := st.CampaignWindowsFor(ctx, c.ID)
	if err != nil {
		t.Fatalf("CampaignWindowsFor: %v", err)
	}
	if len(windows) != 1 || windows[0].Weekday != 1 {
		t.Errorf("windows = %+v", windows)
	}
}

func TestSetCampaignStatus(t *testing.T) {
	ctx := context.Background()
	st := dbtest.New(t)
	orgID, userID, acctID := seedCampaignFixture(t, st, ctx)
	c := mustCreateCampaign(t, st, ctx, orgID, acctID, userID, "Campaign", "Hi!")

	// Invalid transition rejected.
	if _, err := st.SetCampaignStatus(ctx, c.ID, purecampaign.StatusCompleted, uuid.NullUUID{}, "invalid", nil); err != store.ErrInvalidTransition {
		t.Errorf("draft->completed = %v, want ErrInvalidTransition", err)
	}

	running, err := st.SetCampaignStatus(ctx, c.ID, purecampaign.StatusRunning, uuid.NullUUID{UUID: userID, Valid: true}, "started", nil)
	if err != nil {
		t.Fatalf("draft->running: %v", err)
	}
	if running.Status != "running" || running.StartedAt == nil {
		t.Fatalf("running = %+v, want status=running with StartedAt set", running)
	}
	firstStartedAt := *running.StartedAt

	paused, err := st.SetCampaignStatus(ctx, c.ID, purecampaign.StatusPaused, uuid.NullUUID{UUID: userID, Valid: true}, "paused", nil)
	if err != nil {
		t.Fatalf("running->paused: %v", err)
	}
	if paused.Status != "paused" {
		t.Errorf("Status = %q, want paused", paused.Status)
	}

	resumed, err := st.SetCampaignStatus(ctx, c.ID, purecampaign.StatusRunning, uuid.NullUUID{UUID: userID, Valid: true}, "resumed", nil)
	if err != nil {
		t.Fatalf("paused->running: %v", err)
	}
	if resumed.StartedAt == nil || !resumed.StartedAt.Equal(firstStartedAt) {
		t.Errorf("StartedAt changed on resume: got %v, want unchanged %v", resumed.StartedAt, firstStartedAt)
	}

	cancelled, err := st.SetCampaignStatus(ctx, c.ID, purecampaign.StatusCancelled, uuid.NullUUID{UUID: userID, Valid: true}, "stopped", nil)
	if err != nil {
		t.Fatalf("running->cancelled: %v", err)
	}
	if cancelled.Status != "cancelled" {
		t.Errorf("Status = %q, want cancelled", cancelled.Status)
	}

	// Terminal: no further transition allowed.
	if _, err := st.SetCampaignStatus(ctx, c.ID, purecampaign.StatusRunning, uuid.NullUUID{}, "x", nil); err != store.ErrInvalidTransition {
		t.Errorf("cancelled->running = %v, want ErrInvalidTransition", err)
	}

	events, total, err := st.ListCampaignEvents(ctx, c.ID, 50, 0)
	if err != nil {
		t.Fatalf("ListCampaignEvents: %v", err)
	}
	if total != 4 {
		t.Errorf("event count = %d, want 4 (started, paused, resumed, stopped)", total)
	}
	if len(events) != 4 || events[0].Event != "stopped" {
		t.Errorf("events (newest first) = %+v", events)
	}
}

func TestReplaceCampaignRecipientsAndClaimLifecycle(t *testing.T) {
	ctx := context.Background()
	st := dbtest.New(t)
	orgID, userID, acctID := seedCampaignFixture(t, st, ctx)
	c := mustCreateCampaign(t, st, ctx, orgID, acctID, userID, "Campaign", "Hi {{name}}!")

	if err := st.ReplaceCampaignRecipients(ctx, c.ID, []store.CampaignRecipientInput{
		{NormalizedIdentity: "77011111111", Name: "Aigul"},
		{NormalizedIdentity: "77022222222", Name: "Bota"},
		{NormalizedIdentity: "77033333333", Name: "Nurlan"},
	}); err != nil {
		t.Fatalf("ReplaceCampaignRecipients: %v", err)
	}

	counts, err := st.CampaignRecipientCounts(ctx, c.ID)
	if err != nil {
		t.Fatalf("CampaignRecipientCounts: %v", err)
	}
	if counts["pending"] != 3 {
		t.Fatalf("pending count = %d, want 3", counts["pending"])
	}

	// A second replace drops the row not present in the new set, keeps the
	// other two, and adds a new one — all while still pending.
	if err := st.ReplaceCampaignRecipients(ctx, c.ID, []store.CampaignRecipientInput{
		{NormalizedIdentity: "77011111111", Name: "Aigul Updated"},
		{NormalizedIdentity: "77022222222", Name: "Bota"},
		{NormalizedIdentity: "77044444444", Name: "Yerlan"},
	}); err != nil {
		t.Fatalf("ReplaceCampaignRecipients (2nd): %v", err)
	}
	recipients, total, err := st.ListCampaignRecipients(ctx, c.ID, "whatsapp", "", 50, 0)
	if err != nil {
		t.Fatalf("ListCampaignRecipients: %v", err)
	}
	if total != 3 {
		t.Fatalf("total = %d, want 3 (77033333333 pruned, 77044444444 added)", total)
	}
	byIdentity := map[string]store.CampaignRecipient{}
	for _, r := range recipients {
		byIdentity[r.NormalizedIdentity] = r
	}
	if _, ok := byIdentity["77033333333"]; ok {
		t.Error("77033333333 should have been pruned")
	}
	if byIdentity["77011111111"].Name != "Aigul Updated" {
		t.Errorf("77011111111 name = %q, want updated", byIdentity["77011111111"].Name)
	}

	// Start the campaign and claim its one allowed send under a 1-per-window tier.
	if _, _, _, err := st.SetCampaignAccountLimits(ctx, acctID,
		store.CampaignAccountSettingsInput{LimitMode: "custom", MinIntervalSeconds: 1, JitterSeconds: 0},
		[]purecampaign.Tier{{WindowSeconds: 3600, MaxSends: 1}}, nil); err != nil {
		t.Fatalf("SetCampaignAccountLimits: %v", err)
	}
	if _, err := st.SetCampaignStatus(ctx, c.ID, purecampaign.StatusRunning, uuid.NullUUID{UUID: userID, Valid: true}, "started", nil); err != nil {
		t.Fatalf("start campaign: %v", err)
	}

	now := time.Now().UTC()
	claim, ok, err := st.ClaimNextRecipient(ctx, acctID, now)
	if err != nil {
		t.Fatalf("ClaimNextRecipient: %v", err)
	}
	if !ok {
		t.Fatal("ClaimNextRecipient: ok = false, want true")
	}
	if claim.CampaignID != c.ID || claim.Attempts != 1 {
		t.Errorf("claim = %+v", claim)
	}
	if claim.MessageBody != "Hi {{name}}!" {
		t.Errorf("claim.MessageBody = %q", claim.MessageBody)
	}

	claimedRecipient, _, err := st.ListCampaignRecipients(ctx, c.ID, "whatsapp", "sending", 50, 0)
	if err != nil {
		t.Fatalf("ListCampaignRecipients(sending): %v", err)
	}
	if len(claimedRecipient) != 1 {
		t.Fatalf("sending count = %d, want 1", len(claimedRecipient))
	}

	// The tier is now at capacity (1/1h) — the next claim must be refused.
	_, ok, err = st.ClaimNextRecipient(ctx, acctID, now)
	if err != nil {
		t.Fatalf("ClaimNextRecipient (2nd): %v", err)
	}
	if ok {
		t.Error("ClaimNextRecipient (2nd) = claimed, want throttled (tier at capacity)")
	}

	// Finalize the first claim as sent.
	chatID := uuid.New()
	msgID := uuid.New()
	if err := st.FinalizeAttempt(ctx, store.FinalizeAttemptParams{
		LogID: claim.LogID, RecipientID: claim.RecipientID, NewStatus: purecampaign.RecipientSent,
		ChatID: uuid.NullUUID{UUID: chatID, Valid: true}, MessageID: uuid.NullUUID{UUID: msgID, Valid: true},
	}); err != nil {
		t.Fatalf("FinalizeAttempt: %v", err)
	}
	sentRecipient, _, err := st.ListCampaignRecipients(ctx, c.ID, "whatsapp", "sent", 50, 0)
	if err != nil {
		t.Fatalf("ListCampaignRecipients(sent): %v", err)
	}
	if len(sentRecipient) != 1 || !sentRecipient[0].ChatID.Valid || sentRecipient[0].ChatID.UUID != chatID {
		t.Errorf("sentRecipient = %+v", sentRecipient)
	}

	// A "replace" after a send must never touch the now-sent row, even if
	// it's omitted from the new list.
	if err := st.ReplaceCampaignRecipients(ctx, c.ID, []store.CampaignRecipientInput{
		{NormalizedIdentity: "77044444444", Name: "Yerlan"},
	}); err != nil {
		t.Fatalf("ReplaceCampaignRecipients (after send): %v", err)
	}
	counts, err = st.CampaignRecipientCounts(ctx, c.ID)
	if err != nil {
		t.Fatalf("CampaignRecipientCounts (after replace): %v", err)
	}
	if counts["sent"] != 1 {
		t.Errorf("sent count after replace = %d, want 1 (sent rows are immutable)", counts["sent"])
	}
}

func TestClaimNextRecipient_FIFOByCampaignStartTime(t *testing.T) {
	ctx := context.Background()
	st := dbtest.New(t)
	orgID, userID, acctID := seedCampaignFixture(t, st, ctx)

	older := mustCreateCampaign(t, st, ctx, orgID, acctID, userID, "Older", "A")
	newer := mustCreateCampaign(t, st, ctx, orgID, acctID, userID, "Newer", "B")
	for _, c := range []store.Campaign{older, newer} {
		if err := st.ReplaceCampaignRecipients(ctx, c.ID, []store.CampaignRecipientInput{{NormalizedIdentity: "7700000000" + c.ID.String()[:1]}}); err != nil {
			t.Fatalf("ReplaceCampaignRecipients: %v", err)
		}
	}

	// Start the NEWER campaign first, then the OLDER one — started_at order
	// (not creation order) is what FIFO claims by.
	if _, err := st.SetCampaignStatus(ctx, newer.ID, purecampaign.StatusRunning, uuid.NullUUID{}, "started", nil); err != nil {
		t.Fatalf("start newer: %v", err)
	}
	time.Sleep(10 * time.Millisecond) // ensure a distinct started_at ordering
	if _, err := st.SetCampaignStatus(ctx, older.ID, purecampaign.StatusRunning, uuid.NullUUID{}, "started", nil); err != nil {
		t.Fatalf("start older: %v", err)
	}

	now := time.Now().UTC()
	claim, ok, err := st.ClaimNextRecipient(ctx, acctID, now)
	if err != nil || !ok {
		t.Fatalf("ClaimNextRecipient: ok=%v err=%v", ok, err)
	}
	if claim.CampaignID != newer.ID {
		t.Errorf("claimed campaign = %s, want the one started FIRST (newer.ID=%s, its started_at is earlier)", claim.CampaignID, newer.ID)
	}
}

func TestClaimNextRecipient_PausedAccount(t *testing.T) {
	ctx := context.Background()
	st := dbtest.New(t)
	orgID, userID, acctID := seedCampaignFixture(t, st, ctx)
	c := mustCreateCampaign(t, st, ctx, orgID, acctID, userID, "Campaign", "Hi!")
	if err := st.ReplaceCampaignRecipients(ctx, c.ID, []store.CampaignRecipientInput{{NormalizedIdentity: "77011111111"}}); err != nil {
		t.Fatalf("ReplaceCampaignRecipients: %v", err)
	}
	if _, err := st.SetCampaignStatus(ctx, c.ID, purecampaign.StatusRunning, uuid.NullUUID{}, "started", nil); err != nil {
		t.Fatalf("start: %v", err)
	}
	if _, _, _, err := st.SetCampaignAccountLimits(ctx, acctID,
		store.CampaignAccountSettingsInput{LimitMode: "custom", MinIntervalSeconds: 1, Paused: true}, nil, nil); err != nil {
		t.Fatalf("SetCampaignAccountLimits: %v", err)
	}

	_, ok, err := st.ClaimNextRecipient(ctx, acctID, time.Now().UTC())
	if err != nil {
		t.Fatalf("ClaimNextRecipient: %v", err)
	}
	if ok {
		t.Error("ClaimNextRecipient on a manually paused account = claimed, want refused")
	}
}

// TestClaimNextRecipient_PoolsBudgetAcrossConcurrentCampaigns proves the
// account-wide tier cap is a single SHARED pool across every 'running'
// campaign on one account — never a separate allowance per campaign. This is
// what makes several campaigns launched on the same account "just work"
// without double-spending a real channel's rate limit: campaignSendAttempts
// (backend/internal/store/campaigns.go) reads campaign_send_log filtered
// only by account_id, not campaign_id, so Budget always sees every
// campaign's attempts on that account together.
//
// The account is given a tier of 2 sends/hour. Campaign A (started first,
// so FIFO drains it first) has 3 pending recipients — enough to exceed the
// shared cap on its own — and Campaign B (started second) has 1. After
// Campaign A's first two claims exhaust the shared 2/hour tier, a third
// claim must be refused even though Campaign B has made ZERO attempts of
// its own and still has an untouched pending recipient: if the budget were
// (incorrectly) scoped per campaign instead of per account, this claim
// would succeed by drawing from Campaign B's "unused" allowance. Finally,
// once the tier's window rolls over, claims resume and both campaigns'
// remaining recipients eventually get served — proving this is throttling,
// not starvation.
func TestClaimNextRecipient_PoolsBudgetAcrossConcurrentCampaigns(t *testing.T) {
	ctx := context.Background()
	st := dbtest.New(t)
	orgID, userID, acctID := seedCampaignFixture(t, st, ctx)

	campA := mustCreateCampaign(t, st, ctx, orgID, acctID, userID, "Campaign A", "Hi A {{name}}!")
	campB := mustCreateCampaign(t, st, ctx, orgID, acctID, userID, "Campaign B", "Hi B {{name}}!")
	if err := st.ReplaceCampaignRecipients(ctx, campA.ID, []store.CampaignRecipientInput{
		{NormalizedIdentity: "77011110001", Name: "A1"},
		{NormalizedIdentity: "77011110002", Name: "A2"},
		{NormalizedIdentity: "77011110003", Name: "A3"},
	}); err != nil {
		t.Fatalf("ReplaceCampaignRecipients(A): %v", err)
	}
	if err := st.ReplaceCampaignRecipients(ctx, campB.ID, []store.CampaignRecipientInput{
		{NormalizedIdentity: "77011110004", Name: "B1"},
	}); err != nil {
		t.Fatalf("ReplaceCampaignRecipients(B): %v", err)
	}

	// One account-wide tier — 2 sends/hour, shared by whichever campaign's
	// recipients happen to be claimed. MinIntervalSeconds is kept small (and
	// jitter-free) so advancing `now` by a couple of seconds between claims
	// is enough to clear it without ever leaving the 1h tier window.
	if _, _, _, err := st.SetCampaignAccountLimits(ctx, acctID,
		store.CampaignAccountSettingsInput{LimitMode: "custom", MinIntervalSeconds: 1, JitterSeconds: 0},
		[]purecampaign.Tier{{WindowSeconds: 3600, MaxSends: 2}}, nil); err != nil {
		t.Fatalf("SetCampaignAccountLimits: %v", err)
	}

	if _, err := st.SetCampaignStatus(ctx, campA.ID, purecampaign.StatusRunning, uuid.NullUUID{}, "started", nil); err != nil {
		t.Fatalf("start A: %v", err)
	}
	time.Sleep(10 * time.Millisecond) // distinct started_at ordering
	if _, err := st.SetCampaignStatus(ctx, campB.ID, purecampaign.StatusRunning, uuid.NullUUID{}, "started", nil); err != nil {
		t.Fatalf("start B: %v", err)
	}

	base := time.Now().UTC()

	// Claim 1: FIFO by started_at picks Campaign A (started first).
	c1, ok, err := st.ClaimNextRecipient(ctx, acctID, base)
	if err != nil || !ok {
		t.Fatalf("claim 1: ok=%v err=%v", ok, err)
	}
	if c1.CampaignID != campA.ID {
		t.Fatalf("claim 1 campaign = %s, want A (%s)", c1.CampaignID, campA.ID)
	}
	if err := st.FinalizeAttempt(ctx, store.FinalizeAttemptParams{LogID: c1.LogID, RecipientID: c1.RecipientID, NewStatus: purecampaign.RecipientSent}); err != nil {
		t.Fatalf("finalize 1: %v", err)
	}

	// Claim 2, a few seconds later (past min-interval, still deep inside the
	// 1h tier window): still Campaign A's turn under strict FIFO drain — and
	// this is the shared tier's SECOND (and last) slot.
	t2 := base.Add(5 * time.Second)
	c2, ok, err := st.ClaimNextRecipient(ctx, acctID, t2)
	if err != nil || !ok {
		t.Fatalf("claim 2: ok=%v err=%v", ok, err)
	}
	if c2.CampaignID != campA.ID {
		t.Fatalf("claim 2 campaign = %s, want A (%s)", c2.CampaignID, campA.ID)
	}
	if err := st.FinalizeAttempt(ctx, store.FinalizeAttemptParams{LogID: c2.LogID, RecipientID: c2.RecipientID, NewStatus: purecampaign.RecipientSent}); err != nil {
		t.Fatalf("finalize 2: %v", err)
	}

	// Claim 3: the shared tier is now at capacity (2/2 in the last hour).
	// Campaign B has never sent anything and still has its one recipient
	// untouched — a per-campaign budget bug would let this claim succeed by
	// drawing from Campaign B's "fresh" allowance. It must not.
	t3 := base.Add(10 * time.Second)
	_, ok, err = st.ClaimNextRecipient(ctx, acctID, t3)
	if err != nil {
		t.Fatalf("claim 3: %v", err)
	}
	if ok {
		t.Error("claim 3 = claimed, want refused (shared account tier at capacity, regardless of which campaign has headroom of its own)")
	}

	// Both campaigns' still-pending recipients are untouched by the refusal.
	countsA, err := st.CampaignRecipientCounts(ctx, campA.ID)
	if err != nil {
		t.Fatalf("CampaignRecipientCounts(A): %v", err)
	}
	if countsA["pending"] != 1 || countsA["sent"] != 2 {
		t.Errorf("Campaign A counts = %+v, want 1 pending / 2 sent", countsA)
	}
	countsB, err := st.CampaignRecipientCounts(ctx, campB.ID)
	if err != nil {
		t.Fatalf("CampaignRecipientCounts(B): %v", err)
	}
	if countsB["pending"] != 1 || countsB["sent"] != 0 {
		t.Errorf("Campaign B counts = %+v, want its 1 recipient still untouched (never got a shot at the shared budget)", countsB)
	}

	// Once the 1h tier window has fully rolled over, the shared budget opens
	// back up — proving this was throttling, not starvation. FIFO still
	// drains Campaign A's last recipient first...
	t4 := base.Add(3601 * time.Second)
	c4, ok, err := st.ClaimNextRecipient(ctx, acctID, t4)
	if err != nil || !ok {
		t.Fatalf("claim 4 (after window rollover): ok=%v err=%v", ok, err)
	}
	if c4.CampaignID != campA.ID {
		t.Fatalf("claim 4 campaign = %s, want A's last recipient (%s)", c4.CampaignID, campA.ID)
	}
	if err := st.FinalizeAttempt(ctx, store.FinalizeAttemptParams{LogID: c4.LogID, RecipientID: c4.RecipientID, NewStatus: purecampaign.RecipientSent}); err != nil {
		t.Fatalf("finalize 4: %v", err)
	}

	// ...and then Campaign B, which was never starved, finally gets served.
	t5 := t4.Add(5 * time.Second)
	c5, ok, err := st.ClaimNextRecipient(ctx, acctID, t5)
	if err != nil || !ok {
		t.Fatalf("claim 5: ok=%v err=%v", ok, err)
	}
	if c5.CampaignID != campB.ID {
		t.Fatalf("claim 5 campaign = %s, want B (%s) — it must eventually be served, not starved", c5.CampaignID, campB.ID)
	}
	if err := st.FinalizeAttempt(ctx, store.FinalizeAttemptParams{LogID: c5.LogID, RecipientID: c5.RecipientID, NewStatus: purecampaign.RecipientSent}); err != nil {
		t.Fatalf("finalize 5: %v", err)
	}

	for _, c := range []store.Campaign{campA, campB} {
		counts, err := st.CampaignRecipientCounts(ctx, c.ID)
		if err != nil {
			t.Fatalf("CampaignRecipientCounts(%s): %v", c.ID, err)
		}
		if counts["pending"] != 0 || counts["sending"] != 0 {
			t.Errorf("campaign %s final counts = %+v, want everything resolved", c.ID, counts)
		}
	}
}

func TestClaimNextRecipient_CampaignWindowNarrowsAccountWindow(t *testing.T) {
	ctx := context.Background()
	st := dbtest.New(t)
	orgID, userID, acctID := seedCampaignFixture(t, st, ctx)
	c := mustCreateCampaign(t, st, ctx, orgID, acctID, userID, "Campaign", "Hi!")
	if err := st.ReplaceCampaignRecipients(ctx, c.ID, []store.CampaignRecipientInput{{NormalizedIdentity: "77011111111"}}); err != nil {
		t.Fatalf("ReplaceCampaignRecipients: %v", err)
	}
	// Account allows all of Monday AND Tuesday; campaign narrows further to
	// Tuesday only — Monday is inside the account's hours but outside the
	// campaign's, Tuesday is inside both.
	if _, _, _, err := st.SetCampaignAccountLimits(ctx, acctID,
		store.CampaignAccountSettingsInput{LimitMode: "custom", MinIntervalSeconds: 1},
		nil, []store.CampaignWindowInput{
			{Weekday: 1, StartMinute: 0, EndMinute: 1440},
			{Weekday: 2, StartMinute: 0, EndMinute: 1440},
		}); err != nil {
		t.Fatalf("SetCampaignAccountLimits: %v", err)
	}
	if _, err := st.UpdateCampaign(ctx, c.ID, store.CampaignPatch{HasWindows: true, Windows: []store.CampaignWindowInput{
		{Weekday: 2, StartMinute: 0, EndMinute: 1440},
	}}); err != nil {
		t.Fatalf("UpdateCampaign windows: %v", err)
	}
	if _, err := st.SetCampaignStatus(ctx, c.ID, purecampaign.StatusRunning, uuid.NullUUID{}, "started", nil); err != nil {
		t.Fatalf("start: %v", err)
	}

	monday := time.Date(2026, 1, 5, 12, 0, 0, 0, time.UTC) // a Monday — inside the account window, outside the campaign's
	_, ok, err := st.ClaimNextRecipient(ctx, acctID, monday)
	if err != nil {
		t.Fatalf("ClaimNextRecipient: %v", err)
	}
	if ok {
		t.Error("ClaimNextRecipient on Monday = claimed, want refused (campaign window is Tuesday-only)")
	}

	tuesday := time.Date(2026, 1, 6, 12, 0, 0, 0, time.UTC)
	_, ok, err = st.ClaimNextRecipient(ctx, acctID, tuesday)
	if err != nil {
		t.Fatalf("ClaimNextRecipient (Tuesday): %v", err)
	}
	if !ok {
		t.Error("ClaimNextRecipient on Tuesday = refused, want claimed")
	}
}

func TestFinalizeAttempt_TransientRetrySetsBackoff(t *testing.T) {
	ctx := context.Background()
	st := dbtest.New(t)
	orgID, userID, acctID := seedCampaignFixture(t, st, ctx)
	c := mustCreateCampaign(t, st, ctx, orgID, acctID, userID, "Campaign", "Hi!")
	if err := st.ReplaceCampaignRecipients(ctx, c.ID, []store.CampaignRecipientInput{{NormalizedIdentity: "77011111111"}}); err != nil {
		t.Fatalf("ReplaceCampaignRecipients: %v", err)
	}
	// Zero jitter and a tiny interval: this test is about the backoff floor,
	// not pacing — a nonzero jitter draw could otherwise occasionally push
	// the account's own min-interval requirement past the 90s mark this
	// test claims at, making it flaky for a reason unrelated to backoff.
	if _, _, _, err := st.SetCampaignAccountLimits(ctx, acctID,
		store.CampaignAccountSettingsInput{LimitMode: "custom", MinIntervalSeconds: 1, JitterSeconds: 0}, nil, nil); err != nil {
		t.Fatalf("SetCampaignAccountLimits: %v", err)
	}
	if _, err := st.SetCampaignStatus(ctx, c.ID, purecampaign.StatusRunning, uuid.NullUUID{}, "started", nil); err != nil {
		t.Fatalf("start: %v", err)
	}
	now := time.Now().UTC()
	claim, ok, err := st.ClaimNextRecipient(ctx, acctID, now)
	if err != nil || !ok {
		t.Fatalf("ClaimNextRecipient: ok=%v err=%v", ok, err)
	}

	next := now.Add(time.Minute)
	if err := st.FinalizeAttempt(ctx, store.FinalizeAttemptParams{
		LogID: claim.LogID, RecipientID: claim.RecipientID, NewStatus: purecampaign.RecipientPending,
		FailureReason: "timeout", NextAttemptAt: &next,
	}); err != nil {
		t.Fatalf("FinalizeAttempt: %v", err)
	}

	// Not yet eligible: claiming again before `next` must find nothing.
	_, ok, err = st.ClaimNextRecipient(ctx, acctID, now.Add(30*time.Second))
	if err != nil {
		t.Fatalf("ClaimNextRecipient (before backoff): %v", err)
	}
	if ok {
		t.Error("ClaimNextRecipient before the backoff floor = claimed, want refused")
	}

	// Eligible again once next_attempt_at has passed.
	claim2, ok, err := st.ClaimNextRecipient(ctx, acctID, now.Add(90*time.Second))
	if err != nil {
		t.Fatalf("ClaimNextRecipient (after backoff): %v", err)
	}
	if !ok {
		t.Fatal("ClaimNextRecipient after the backoff floor = refused, want claimed")
	}
	if claim2.Attempts != 2 {
		t.Errorf("Attempts = %d, want 2", claim2.Attempts)
	}
}

func TestReconcileStuckSending(t *testing.T) {
	ctx := context.Background()
	st := dbtest.New(t)
	orgID, userID, acctID := seedCampaignFixture(t, st, ctx)
	c := mustCreateCampaign(t, st, ctx, orgID, acctID, userID, "Campaign", "Hi!")
	if err := st.ReplaceCampaignRecipients(ctx, c.ID, []store.CampaignRecipientInput{{NormalizedIdentity: "77011111111"}}); err != nil {
		t.Fatalf("ReplaceCampaignRecipients: %v", err)
	}
	if _, err := st.SetCampaignStatus(ctx, c.ID, purecampaign.StatusRunning, uuid.NullUUID{}, "started", nil); err != nil {
		t.Fatalf("start: %v", err)
	}
	if _, ok, err := st.ClaimNextRecipient(ctx, acctID, time.Now().UTC()); err != nil || !ok {
		t.Fatalf("ClaimNextRecipient: ok=%v err=%v", ok, err)
	}
	// Simulate a crash: the recipient is stuck 'sending', never finalized.

	n, err := st.ReconcileStuckSending(ctx)
	if err != nil {
		t.Fatalf("ReconcileStuckSending: %v", err)
	}
	if n != 1 {
		t.Fatalf("ReconcileStuckSending reconciled %d, want 1", n)
	}
	failed, _, err := st.ListCampaignRecipients(ctx, c.ID, "whatsapp", "failed", 50, 0)
	if err != nil {
		t.Fatalf("ListCampaignRecipients(failed): %v", err)
	}
	if len(failed) != 1 || failed[0].FailureReason == "" {
		t.Errorf("failed = %+v, want one row with a failure reason", failed)
	}

	// Never auto-retried.
	_, ok, err := st.ClaimNextRecipient(ctx, acctID, time.Now().UTC())
	if err != nil {
		t.Fatalf("ClaimNextRecipient: %v", err)
	}
	if ok {
		t.Error("a reconciled-as-failed recipient must never be auto-claimed again")
	}
}

func TestSuppressPendingForIdentity(t *testing.T) {
	ctx := context.Background()
	st := dbtest.New(t)
	orgID, userID, acctID := seedCampaignFixture(t, st, ctx)

	running := mustCreateCampaign(t, st, ctx, orgID, acctID, userID, "Running", "Hi!")
	draft := mustCreateCampaign(t, st, ctx, orgID, acctID, userID, "Draft", "Hi!")
	for _, c := range []store.Campaign{running, draft} {
		if err := st.ReplaceCampaignRecipients(ctx, c.ID, []store.CampaignRecipientInput{{NormalizedIdentity: "77011111111"}}); err != nil {
			t.Fatalf("ReplaceCampaignRecipients: %v", err)
		}
	}
	if _, err := st.SetCampaignStatus(ctx, running.ID, purecampaign.StatusRunning, uuid.NullUUID{}, "started", nil); err != nil {
		t.Fatalf("start running campaign: %v", err)
	}

	n, err := st.SuppressPendingForIdentity(ctx, acctID, "77011111111", "customer replied")
	if err != nil {
		t.Fatalf("SuppressPendingForIdentity: %v", err)
	}
	if n != 1 {
		t.Fatalf("suppressed %d rows, want 1 (only the running campaign's)", n)
	}

	runningCounts, err := st.CampaignRecipientCounts(ctx, running.ID)
	if err != nil {
		t.Fatalf("CampaignRecipientCounts(running): %v", err)
	}
	if runningCounts["skipped"] != 1 {
		t.Errorf("running campaign skipped count = %d, want 1", runningCounts["skipped"])
	}

	draftCounts, err := st.CampaignRecipientCounts(ctx, draft.ID)
	if err != nil {
		t.Fatalf("CampaignRecipientCounts(draft): %v", err)
	}
	if draftCounts["pending"] != 1 {
		t.Errorf("draft (not-yet-running) campaign pending count = %d, want 1 (untouched)", draftCounts["pending"])
	}
}

func TestRetryFailedRecipients(t *testing.T) {
	ctx := context.Background()
	st := dbtest.New(t)
	orgID, userID, acctID := seedCampaignFixture(t, st, ctx)
	c := mustCreateCampaign(t, st, ctx, orgID, acctID, userID, "Campaign", "Hi!")
	if err := st.ReplaceCampaignRecipients(ctx, c.ID, []store.CampaignRecipientInput{
		{NormalizedIdentity: "77011111111"}, {NormalizedIdentity: "77022222222"},
	}); err != nil {
		t.Fatalf("ReplaceCampaignRecipients: %v", err)
	}
	if _, err := st.SetCampaignStatus(ctx, c.ID, purecampaign.StatusRunning, uuid.NullUUID{}, "started", nil); err != nil {
		t.Fatalf("start: %v", err)
	}
	now := time.Now().UTC()
	claim, ok, err := st.ClaimNextRecipient(ctx, acctID, now)
	if err != nil || !ok {
		t.Fatalf("ClaimNextRecipient: ok=%v err=%v", ok, err)
	}
	if err := st.FinalizeAttempt(ctx, store.FinalizeAttemptParams{
		LogID: claim.LogID, RecipientID: claim.RecipientID, NewStatus: purecampaign.RecipientFailed, FailureReason: "blocked",
	}); err != nil {
		t.Fatalf("FinalizeAttempt: %v", err)
	}

	n, err := st.RetryFailedRecipients(ctx, c.ID, nil)
	if err != nil {
		t.Fatalf("RetryFailedRecipients: %v", err)
	}
	if n != 1 {
		t.Fatalf("retried %d, want 1", n)
	}
	recipients, _, err := st.ListCampaignRecipients(ctx, c.ID, "whatsapp", "", 50, 0)
	if err != nil {
		t.Fatalf("ListCampaignRecipients: %v", err)
	}
	for _, r := range recipients {
		if r.ID == claim.RecipientID {
			if r.Status != "pending" || r.Attempts != 0 || r.FailureReason != "" {
				t.Errorf("retried recipient = %+v, want pending/attempts=0/no reason", r)
			}
		}
	}
}

func TestDuplicateCampaign(t *testing.T) {
	ctx := context.Background()
	st := dbtest.New(t)
	orgID, userID, acctID := seedCampaignFixture(t, st, ctx)
	src := mustCreateCampaign(t, st, ctx, orgID, acctID, userID, "Original", "Hi {{name}}!")
	if err := st.ReplaceCampaignRecipients(ctx, src.ID, []store.CampaignRecipientInput{
		{NormalizedIdentity: "77011111111", Name: "Aigul"},
		{NormalizedIdentity: "77022222222", Name: "Bota"},
	}); err != nil {
		t.Fatalf("ReplaceCampaignRecipients: %v", err)
	}
	if _, err := st.SetCampaignStatus(ctx, src.ID, purecampaign.StatusRunning, uuid.NullUUID{}, "started", nil); err != nil {
		t.Fatalf("start: %v", err)
	}
	claim, ok, err := st.ClaimNextRecipient(ctx, acctID, time.Now().UTC())
	if err != nil || !ok {
		t.Fatalf("ClaimNextRecipient: ok=%v err=%v", ok, err)
	}
	if err := st.FinalizeAttempt(ctx, store.FinalizeAttemptParams{
		LogID: claim.LogID, RecipientID: claim.RecipientID, NewStatus: purecampaign.RecipientSent,
	}); err != nil {
		t.Fatalf("FinalizeAttempt: %v", err)
	}

	dup, err := st.DuplicateCampaign(ctx, src.ID, userID)
	if err != nil {
		t.Fatalf("DuplicateCampaign: %v", err)
	}
	if dup.Status != "draft" {
		t.Errorf("duplicate status = %q, want draft", dup.Status)
	}
	if dup.MessageBody != src.MessageBody {
		t.Errorf("duplicate MessageBody = %q, want %q", dup.MessageBody, src.MessageBody)
	}
	counts, err := st.CampaignRecipientCounts(ctx, dup.ID)
	if err != nil {
		t.Fatalf("CampaignRecipientCounts: %v", err)
	}
	if counts["pending"] != 1 || counts["sent"] != 0 {
		t.Errorf("duplicate recipient counts = %+v, want only the never-contacted one carried over", counts)
	}
}

func TestCampaignAccountSettingsDefaults(t *testing.T) {
	ctx := context.Background()
	st := dbtest.New(t)
	_, _, acctID := seedCampaignFixture(t, st, ctx)

	settings, err := st.CampaignAccountSettingsFor(ctx, acctID, "whatsapp")
	if err != nil {
		t.Fatalf("CampaignAccountSettingsFor: %v", err)
	}
	if settings.MinIntervalSeconds != purecampaign.DefaultMinIntervalSeconds || settings.JitterSeconds != purecampaign.DefaultJitterSeconds {
		t.Errorf("settings = %+v, want the built-in whatsapp defaults", settings)
	}
	if settings.Paused {
		t.Error("Paused = true for an unconfigured account, want false")
	}

	tiers, err := st.CampaignAccountLimitsFor(ctx, acctID, "whatsapp")
	if err != nil {
		t.Fatalf("CampaignAccountLimitsFor: %v", err)
	}
	if len(tiers) != len(purecampaign.DefaultTiers) {
		t.Errorf("tiers = %+v, want the %d built-in defaults", tiers, len(purecampaign.DefaultTiers))
	}

	// Simulator stands in for a real whatsmeow-backed WhatsApp account, so it
	// gets the exact same built-in defaults as any real channel — a campaign
	// sent through it is throttled/paced exactly the way a live one would be
	// unless an operator overrides it (see budget.go's own doc comments).
	simSettings, err := st.CampaignAccountSettingsFor(ctx, acctID, "simulator")
	if err != nil {
		t.Fatalf("CampaignAccountSettingsFor(simulator): %v", err)
	}
	if simSettings.MinIntervalSeconds != purecampaign.DefaultMinIntervalSeconds || simSettings.JitterSeconds != purecampaign.DefaultJitterSeconds {
		t.Errorf("simulator settings = %+v, want the same built-in defaults as a real channel", simSettings)
	}

	simTiers, err := st.CampaignAccountLimitsFor(ctx, acctID, "simulator")
	if err != nil {
		t.Fatalf("CampaignAccountLimitsFor(simulator): %v", err)
	}
	if len(simTiers) != len(purecampaign.DefaultTiers) {
		t.Errorf("simulator default tiers = %+v, want the %d built-in defaults (same as a real channel)", simTiers, len(purecampaign.DefaultTiers))
	}
}

func TestSetCampaignAccountLimitsRoundTrip(t *testing.T) {
	ctx := context.Background()
	st := dbtest.New(t)
	_, _, acctID := seedCampaignFixture(t, st, ctx)

	settings, tiers, windows, err := st.SetCampaignAccountLimits(ctx, acctID,
		store.CampaignAccountSettingsInput{LimitMode: "custom", MinIntervalSeconds: 120, JitterSeconds: 15},
		[]purecampaign.Tier{{WindowSeconds: 3600, MaxSends: 3}},
		[]store.CampaignWindowInput{{Weekday: 3, StartMinute: 480, EndMinute: 1020}})
	if err != nil {
		t.Fatalf("SetCampaignAccountLimits: %v", err)
	}
	if settings.MinIntervalSeconds != 120 || settings.LimitMode != "custom" {
		t.Errorf("settings = %+v", settings)
	}
	if len(tiers) != 1 || tiers[0].MaxSends != 3 {
		t.Errorf("tiers = %+v", tiers)
	}
	if len(windows) != 1 || windows[0].Weekday != 3 {
		t.Errorf("windows = %+v", windows)
	}

	gotSettings, err := st.CampaignAccountSettingsFor(ctx, acctID, "whatsapp")
	if err != nil {
		t.Fatalf("CampaignAccountSettingsFor: %v", err)
	}
	if gotSettings.MinIntervalSeconds != 120 {
		t.Errorf("re-read MinIntervalSeconds = %d, want 120", gotSettings.MinIntervalSeconds)
	}
	gotWindows, err := st.CampaignAccountWindowsFor(ctx, acctID)
	if err != nil {
		t.Fatalf("CampaignAccountWindowsFor: %v", err)
	}
	if len(gotWindows) != 1 {
		t.Errorf("re-read windows = %+v", gotWindows)
	}
}

func TestSendingBudget(t *testing.T) {
	ctx := context.Background()
	st := dbtest.New(t)
	orgID, userID, acctID := seedCampaignFixture(t, st, ctx)
	if _, _, _, err := st.SetCampaignAccountLimits(ctx, acctID,
		store.CampaignAccountSettingsInput{LimitMode: "custom", MinIntervalSeconds: 1},
		[]purecampaign.Tier{{WindowSeconds: 3600, MaxSends: 2}}, nil); err != nil {
		t.Fatalf("SetCampaignAccountLimits: %v", err)
	}

	now := time.Now().UTC()
	budget, err := st.SendingBudget(ctx, acctID, "whatsapp", now)
	if err != nil {
		t.Fatalf("SendingBudget: %v", err)
	}
	if !budget.Allowed {
		t.Errorf("budget = %+v, want Allowed with no prior sends", budget)
	}
	if len(budget.Tiers) != 1 || budget.Tiers[0].Used != 0 {
		t.Errorf("Tiers = %+v", budget.Tiers)
	}

	c := mustCreateCampaign(t, st, ctx, orgID, acctID, userID, "Campaign", "Hi!")
	if err := st.ReplaceCampaignRecipients(ctx, c.ID, []store.CampaignRecipientInput{
		{NormalizedIdentity: "77011111111"}, {NormalizedIdentity: "77022222222"},
	}); err != nil {
		t.Fatalf("ReplaceCampaignRecipients: %v", err)
	}
	if _, err := st.SetCampaignStatus(ctx, c.ID, purecampaign.StatusRunning, uuid.NullUUID{}, "started", nil); err != nil {
		t.Fatalf("start: %v", err)
	}
	for i := 0; i < 2; i++ {
		// Each claim is spaced past the 1s min interval, or the 2nd would be
		// refused by pacing rather than by the tier this test means to check.
		claimAt := now.Add(time.Duration(i) * 2 * time.Second)
		claim, ok, err := st.ClaimNextRecipient(ctx, acctID, claimAt)
		if err != nil || !ok {
			t.Fatalf("ClaimNextRecipient[%d]: ok=%v err=%v", i, ok, err)
		}
		if err := st.FinalizeAttempt(ctx, store.FinalizeAttemptParams{LogID: claim.LogID, RecipientID: claim.RecipientID, NewStatus: purecampaign.RecipientSent}); err != nil {
			t.Fatalf("FinalizeAttempt[%d]: %v", i, err)
		}
	}
	now = now.Add(2 * time.Second) // read the budget from just after the last claim, matching SendingBudget's own "now" below

	budget, err = st.SendingBudget(ctx, acctID, "whatsapp", now)
	if err != nil {
		t.Fatalf("SendingBudget (after 2 sends): %v", err)
	}
	if budget.Allowed {
		t.Errorf("budget = %+v, want NOT Allowed (tier at capacity)", budget)
	}
	if budget.Tiers[0].Used != 2 {
		t.Errorf("Tiers[0].Used = %d, want 2", budget.Tiers[0].Used)
	}
	if !budget.NextSendAt.After(now) {
		t.Errorf("NextSendAt = %v, want after now (%v)", budget.NextSendAt, now)
	}
}

func TestPruneCampaignSendLog(t *testing.T) {
	ctx := context.Background()
	st := dbtest.New(t)
	orgID, userID, acctID := seedCampaignFixture(t, st, ctx)
	c := mustCreateCampaign(t, st, ctx, orgID, acctID, userID, "Campaign", "Hi!")
	if err := st.ReplaceCampaignRecipients(ctx, c.ID, []store.CampaignRecipientInput{{NormalizedIdentity: "77011111111"}}); err != nil {
		t.Fatalf("ReplaceCampaignRecipients: %v", err)
	}
	if _, err := st.SetCampaignStatus(ctx, c.ID, purecampaign.StatusRunning, uuid.NullUUID{}, "started", nil); err != nil {
		t.Fatalf("start: %v", err)
	}
	now := time.Now().UTC()
	claim, ok, err := st.ClaimNextRecipient(ctx, acctID, now)
	if err != nil || !ok {
		t.Fatalf("ClaimNextRecipient: ok=%v err=%v", ok, err)
	}
	if err := st.FinalizeAttempt(ctx, store.FinalizeAttemptParams{LogID: claim.LogID, RecipientID: claim.RecipientID, NewStatus: purecampaign.RecipientSent}); err != nil {
		t.Fatalf("FinalizeAttempt: %v", err)
	}

	// Not old enough yet.
	n, err := st.PruneCampaignSendLog(ctx, now.Add(-7*24*time.Hour))
	if err != nil {
		t.Fatalf("PruneCampaignSendLog (too recent): %v", err)
	}
	if n != 0 {
		t.Errorf("pruned %d rows that aren't old enough, want 0", n)
	}

	// Prune everything up to just after now.
	n, err = st.PruneCampaignSendLog(ctx, now.Add(time.Minute))
	if err != nil {
		t.Fatalf("PruneCampaignSendLog: %v", err)
	}
	if n != 1 {
		t.Errorf("pruned %d, want 1", n)
	}
}

func TestUpsertInbound_SuppressesRunningCampaignRecipients(t *testing.T) {
	ctx := context.Background()
	st := dbtest.New(t)
	orgID, userID, acctID := seedCampaignFixture(t, st, ctx)

	running := mustCreateCampaign(t, st, ctx, orgID, acctID, userID, "Running", "Hi!")
	draft := mustCreateCampaign(t, st, ctx, orgID, acctID, userID, "Draft", "Hi!")
	for _, c := range []store.Campaign{running, draft} {
		if err := st.ReplaceCampaignRecipients(ctx, c.ID, []store.CampaignRecipientInput{{NormalizedIdentity: "77011234567"}}); err != nil {
			t.Fatalf("ReplaceCampaignRecipients: %v", err)
		}
	}
	if _, err := st.SetCampaignStatus(ctx, running.ID, purecampaign.StatusRunning, uuid.NullUUID{}, "started", nil); err != nil {
		t.Fatalf("start running campaign: %v", err)
	}

	externalMessageID := uuid.NewString()
	res, err := st.UpsertInbound(ctx, store.InboundUpsert{
		AccountID: acctID, PhoneJID: "77011234567@s.whatsapp.net", RemoteJID: "77011234567@s.whatsapp.net",
		PhoneNumber: "77011234567", Direction: "in", SenderKind: "contact",
		ExternalMessageID: externalMessageID, MessageKind: "conversation", Body: "hi there",
	})
	if err != nil {
		t.Fatalf("UpsertInbound: %v", err)
	}
	if !res.MessageInserted {
		t.Fatal("precondition: MessageInserted = false, want true")
	}

	runningCounts, err := st.CampaignRecipientCounts(ctx, running.ID)
	if err != nil {
		t.Fatalf("CampaignRecipientCounts(running): %v", err)
	}
	if runningCounts["skipped"] != 1 {
		t.Errorf("running campaign skipped count = %d, want 1", runningCounts["skipped"])
	}

	draftCounts, err := st.CampaignRecipientCounts(ctx, draft.ID)
	if err != nil {
		t.Fatalf("CampaignRecipientCounts(draft): %v", err)
	}
	if draftCounts["pending"] != 1 {
		t.Errorf("draft campaign pending count = %d, want 1 (not yet running, untouched)", draftCounts["pending"])
	}

	// A duplicate delivery of the SAME external message must not re-run
	// suppression a second time in a way that would error (there is nothing
	// left to suppress — MessageInserted is false on the redelivery).
	res2, err := st.UpsertInbound(ctx, store.InboundUpsert{
		AccountID: acctID, PhoneJID: "77011234567@s.whatsapp.net", RemoteJID: "77011234567@s.whatsapp.net",
		PhoneNumber: "77011234567", Direction: "in", SenderKind: "contact",
		ExternalMessageID: externalMessageID, MessageKind: "conversation", Body: "hi there",
	})
	if err != nil {
		t.Fatalf("UpsertInbound (redelivery): %v", err)
	}
	if res2.MessageInserted {
		t.Error("redelivery: MessageInserted = true, want false (dedup)")
	}
}

func TestAppendCampaignEvent(t *testing.T) {
	ctx := context.Background()
	st := dbtest.New(t)
	orgID, userID, acctID := seedCampaignFixture(t, st, ctx)
	c := mustCreateCampaign(t, st, ctx, orgID, acctID, userID, "Campaign", "Hi!")

	if err := st.AppendCampaignEvent(ctx, c.ID, "recipients_replaced", uuid.NullUUID{UUID: userID, Valid: true}, map[string]any{"added": 3}); err != nil {
		t.Fatalf("AppendCampaignEvent: %v", err)
	}
	events, total, err := st.ListCampaignEvents(ctx, c.ID, 50, 0)
	if err != nil {
		t.Fatalf("ListCampaignEvents: %v", err)
	}
	if total != 1 || events[0].Event != "recipients_replaced" {
		t.Fatalf("events = %+v", events)
	}
	if !events[0].ActorUserID.Valid || events[0].ActorUserID.UUID != userID {
		t.Errorf("ActorUserID = %+v, want %s", events[0].ActorUserID, userID)
	}
	if added, ok := events[0].Detail["added"].(float64); !ok || added != 3 {
		t.Errorf("Detail[added] = %v, want 3", events[0].Detail["added"])
	}
}

func TestListRunningCampaignAccounts(t *testing.T) {
	ctx := context.Background()
	st := dbtest.New(t)
	orgID, userID, acctID := seedCampaignFixture(t, st, ctx)

	draft := mustCreateCampaign(t, st, ctx, orgID, acctID, userID, "Draft", "Hi!")
	running := mustCreateCampaign(t, st, ctx, orgID, acctID, userID, "Running", "Hi!")
	_ = draft

	accts, err := st.ListRunningCampaignAccounts(ctx)
	if err != nil {
		t.Fatalf("ListRunningCampaignAccounts: %v", err)
	}
	if len(accts) != 0 {
		t.Fatalf("before start: accts = %v, want none", accts)
	}

	if _, err := st.SetCampaignStatus(ctx, running.ID, purecampaign.StatusRunning, uuid.NullUUID{}, "started", nil); err != nil {
		t.Fatalf("start campaign: %v", err)
	}
	accts, err = st.ListRunningCampaignAccounts(ctx)
	if err != nil {
		t.Fatalf("ListRunningCampaignAccounts: %v", err)
	}
	if len(accts) != 1 || accts[0] != acctID {
		t.Fatalf("accts = %v, want [%s]", accts, acctID)
	}
}

func TestDueScheduledCampaigns(t *testing.T) {
	ctx := context.Background()
	st := dbtest.New(t)
	orgID, userID, acctID := seedCampaignFixture(t, st, ctx)

	c := mustCreateCampaign(t, st, ctx, orgID, acctID, userID, "Scheduled", "Hi!")
	if _, err := st.SetCampaignStatus(ctx, c.ID, purecampaign.StatusScheduled, uuid.NullUUID{}, "scheduled", nil); err != nil {
		t.Fatalf("move to scheduled: %v", err)
	}

	past := time.Now().UTC().Add(-1 * time.Hour)
	future := time.Now().UTC().Add(1 * time.Hour)

	due, err := st.DueScheduledCampaigns(ctx, past)
	if err != nil {
		t.Fatalf("DueScheduledCampaigns (before schedule_at set): %v", err)
	}
	if len(due) != 0 {
		t.Fatalf("due = %v, want none (schedule_at is still NULL)", due)
	}

	scheduleAt := time.Now().UTC().Add(-30 * time.Minute)
	if _, err := st.UpdateCampaign(ctx, c.ID, store.CampaignPatch{HasScheduleAt: true, ScheduleAt: &scheduleAt}); err != nil {
		t.Fatalf("UpdateCampaign (set schedule_at): %v", err)
	}

	due, err = st.DueScheduledCampaigns(ctx, future)
	if err != nil {
		t.Fatalf("DueScheduledCampaigns (future now): %v", err)
	}
	if len(due) != 1 || due[0] != c.ID {
		t.Fatalf("due = %v, want [%s]", due, c.ID)
	}

	due, err = st.DueScheduledCampaigns(ctx, past)
	if err != nil {
		t.Fatalf("DueScheduledCampaigns (past now): %v", err)
	}
	if len(due) != 0 {
		t.Fatalf("due = %v, want none (now is before schedule_at)", due)
	}
}

func TestAutoPauseCampaignsForAccount(t *testing.T) {
	ctx := context.Background()
	st := dbtest.New(t)
	orgID, userID, acctID := seedCampaignFixture(t, st, ctx)

	running1 := mustCreateCampaign(t, st, ctx, orgID, acctID, userID, "Running1", "Hi!")
	running2 := mustCreateCampaign(t, st, ctx, orgID, acctID, userID, "Running2", "Hi!")
	draft := mustCreateCampaign(t, st, ctx, orgID, acctID, userID, "Draft", "Hi!")
	for _, c := range []store.Campaign{running1, running2} {
		if _, err := st.SetCampaignStatus(ctx, c.ID, purecampaign.StatusRunning, uuid.NullUUID{}, "started", nil); err != nil {
			t.Fatalf("start campaign %s: %v", c.Name, err)
		}
	}

	n, err := st.AutoPauseCampaignsForAccount(ctx, acctID, "account disconnected 60s+")
	if err != nil {
		t.Fatalf("AutoPauseCampaignsForAccount: %v", err)
	}
	if n != 2 {
		t.Fatalf("paused count = %d, want 2", n)
	}

	for _, c := range []store.Campaign{running1, running2} {
		got, err := st.CampaignByID(ctx, c.ID)
		if err != nil {
			t.Fatalf("CampaignByID(%s): %v", c.Name, err)
		}
		if got.Status != string(purecampaign.StatusPaused) {
			t.Errorf("%s status = %q, want paused", c.Name, got.Status)
		}
		events, _, err := st.ListCampaignEvents(ctx, c.ID, 50, 0)
		if err != nil {
			t.Fatalf("ListCampaignEvents(%s): %v", c.Name, err)
		}
		if len(events) != 2 || events[0].Event != "auto_paused" {
			t.Fatalf("%s events = %+v, want [auto_paused, started]", c.Name, events)
		}
	}

	got, err := st.CampaignByID(ctx, draft.ID)
	if err != nil {
		t.Fatalf("CampaignByID(draft): %v", err)
	}
	if got.Status != string(purecampaign.StatusDraft) {
		t.Errorf("draft status = %q, want untouched draft", got.Status)
	}

	// Nothing left running — a second call is a no-op, not an error.
	n, err = st.AutoPauseCampaignsForAccount(ctx, acctID, "still disconnected")
	if err != nil {
		t.Fatalf("AutoPauseCampaignsForAccount (2nd): %v", err)
	}
	if n != 0 {
		t.Fatalf("2nd paused count = %d, want 0", n)
	}
}

func TestExistingChatForIdentity(t *testing.T) {
	ctx := context.Background()
	st := dbtest.New(t)
	_, _, acctID := seedCampaignFixture(t, st, ctx)

	if _, ok, err := st.ExistingChatForIdentity(ctx, acctID, "77011234567"); err != nil {
		t.Fatalf("ExistingChatForIdentity (no chat yet): %v", err)
	} else if ok {
		t.Fatal("ExistingChatForIdentity (no chat yet): ok = true, want false")
	}

	res, err := st.UpsertInbound(ctx, store.InboundUpsert{
		AccountID: acctID, PhoneJID: "77011234567@s.whatsapp.net", RemoteJID: "77011234567@s.whatsapp.net",
		PhoneNumber: "77011234567", Direction: "in", SenderKind: "contact",
		ExternalMessageID: uuid.NewString(), MessageKind: "conversation", Body: "hi",
	})
	if err != nil {
		t.Fatalf("UpsertInbound: %v", err)
	}

	chatID, ok, err := st.ExistingChatForIdentity(ctx, acctID, "77011234567")
	if err != nil {
		t.Fatalf("ExistingChatForIdentity: %v", err)
	}
	if !ok || chatID != res.ChatID {
		t.Fatalf("ExistingChatForIdentity = (%s, %v), want (%s, true)", chatID, ok, res.ChatID)
	}

	// A different account never matches, even for the identical identity.
	otherAcct, err := st.SeedAccount(ctx, store.Account{
		ID: uuid.New(), DisplayName: "Other WA",
		ExternalAccountRef: "7770000001@s.whatsapp.net", ExternalHandle: "77700000001",
		ConnectionState: "connected",
	})
	if err != nil {
		t.Fatalf("seed other account: %v", err)
	}
	if _, ok, err := st.ExistingChatForIdentity(ctx, otherAcct.ID, "77011234567"); err != nil {
		t.Fatalf("ExistingChatForIdentity (other account): %v", err)
	} else if ok {
		t.Fatal("ExistingChatForIdentity (other account): ok = true, want false (account-scoped)")
	}
}

func TestUnreachableForWarmChannel(t *testing.T) {
	ctx := context.Background()
	st := dbtest.New(t)
	_, _, acctID := seedCampaignFixture(t, st, ctx)

	if _, err := st.UpsertInbound(ctx, store.InboundUpsert{
		AccountID: acctID, PhoneJID: "77011234567@s.whatsapp.net", RemoteJID: "77011234567@s.whatsapp.net",
		PhoneNumber: "77011234567", Direction: "in", SenderKind: "contact",
		ExternalMessageID: uuid.NewString(), MessageKind: "conversation", Body: "hi",
	}); err != nil {
		t.Fatalf("UpsertInbound: %v", err)
	}

	unreachable, err := st.UnreachableForWarmChannel(ctx, acctID, []string{"77011234567", "77099999999", "77088888888"})
	if err != nil {
		t.Fatalf("UnreachableForWarmChannel: %v", err)
	}
	if len(unreachable) != 2 || !unreachable["77099999999"] || !unreachable["77088888888"] {
		t.Fatalf("unreachable = %v, want exactly {77099999999, 77088888888}", unreachable)
	}
	if unreachable["77011234567"] {
		t.Error("77011234567 has an existing chat and must not be reported unreachable")
	}

	empty, err := st.UnreachableForWarmChannel(ctx, acctID, nil)
	if err != nil {
		t.Fatalf("UnreachableForWarmChannel(nil): %v", err)
	}
	if len(empty) != 0 {
		t.Errorf("UnreachableForWarmChannel(nil) = %v, want empty", empty)
	}
}

func TestListChatsForOrg_ExcludesCampaignChatsByDefault(t *testing.T) {
	ctx := context.Background()
	st := dbtest.New(t)
	orgID, _, acctID := seedCampaignFixture(t, st, ctx)

	chatID, _, err := st.FindOrCreateChat(ctx, acctID, "77011234567@s.whatsapp.net", "77011234567")
	if err != nil {
		t.Fatalf("FindOrCreateChat: %v", err)
	}
	if err := st.MarkChatCampaignOnly(ctx, "whatsapp", chatID); err != nil {
		t.Fatalf("MarkChatCampaignOnly: %v", err)
	}
	otherChatID, _, err := st.FindOrCreateChat(ctx, acctID, "77022222222@s.whatsapp.net", "77022222222")
	if err != nil {
		t.Fatalf("FindOrCreateChat (other): %v", err)
	}

	chats, total, err := st.ListChatsForOrg(ctx, store.ChatFilter{OrgID: orgID, Limit: 50})
	if err != nil {
		t.Fatalf("ListChatsForOrg: %v", err)
	}
	if total != 1 || len(chats) != 1 || chats[0].ID != otherChatID {
		t.Fatalf("default list = %+v (total=%d), want exactly [%s]", chats, total, otherChatID)
	}

	campaignOnly, total, err := st.ListChatsForOrg(ctx, store.ChatFilter{OrgID: orgID, Status: "campaign", Limit: 50})
	if err != nil {
		t.Fatalf("ListChatsForOrg(status=campaign): %v", err)
	}
	if total != 1 || len(campaignOnly) != 1 || campaignOnly[0].ID != chatID {
		t.Fatalf("campaign-only list = %+v (total=%d), want exactly [%s]", campaignOnly, total, chatID)
	}

	// A reply graduates the chat back to 'open' — visible in the default
	// listing again, and no longer under the campaign-only filter.
	if _, err := st.UpsertInbound(ctx, store.InboundUpsert{
		AccountID: acctID, PhoneJID: "77011234567@s.whatsapp.net", RemoteJID: "77011234567@s.whatsapp.net",
		PhoneNumber: "77011234567", Direction: "in", SenderKind: "contact",
		ExternalMessageID: uuid.NewString(), MessageKind: "conversation", Body: "hi back",
	}); err != nil {
		t.Fatalf("UpsertInbound: %v", err)
	}
	chats, total, err = st.ListChatsForOrg(ctx, store.ChatFilter{OrgID: orgID, Limit: 50})
	if err != nil {
		t.Fatalf("ListChatsForOrg (after reply): %v", err)
	}
	if total != 2 || len(chats) != 2 {
		t.Fatalf("default list after reply = %+v (total=%d), want both chats", chats, total)
	}
	campaignOnly, total, err = st.ListChatsForOrg(ctx, store.ChatFilter{OrgID: orgID, Status: "campaign", Limit: 50})
	if err != nil {
		t.Fatalf("ListChatsForOrg(status=campaign) after reply: %v", err)
	}
	if total != 0 || len(campaignOnly) != 0 {
		t.Fatalf("campaign-only list after reply = %+v (total=%d), want none", campaignOnly, total)
	}
}

func TestInsertCampaignOutbound(t *testing.T) {
	ctx := context.Background()
	st := dbtest.New(t)
	_, _, acctID := seedCampaignFixture(t, st, ctx)

	chatID, _, err := st.FindOrCreateChat(ctx, acctID, "77011234567@s.whatsapp.net", "77011234567")
	if err != nil {
		t.Fatalf("FindOrCreateChat: %v", err)
	}

	msgID, err := st.InsertCampaignOutbound(ctx, "whatsapp", chatID, acctID, "Hi Aigul, 20% off!", "Hi Aigul, 20% off!")
	if err != nil {
		t.Fatalf("InsertCampaignOutbound: %v", err)
	}
	msg, err := st.MessageByID(ctx, msgID)
	if err != nil {
		t.Fatalf("MessageByID: %v", err)
	}
	if msg.SenderKind != "campaign" || msg.Direction != "out" || msg.SenderUserID.Valid {
		t.Errorf("msg = %+v, want sender_kind=campaign, direction=out, no sender_user_id", msg)
	}
	if msg.Body != "Hi Aigul, 20% off!" {
		t.Errorf("msg.Body = %q", msg.Body)
	}

	chat, err := st.ChatByID(ctx, chatID)
	if err != nil {
		t.Fatalf("ChatByID: %v", err)
	}
	if chat.LastMessagePreview != "Hi Aigul, 20% off!" {
		t.Errorf("LastMessagePreview = %q", chat.LastMessagePreview)
	}
	if chat.UnreadCount != 0 {
		t.Errorf("UnreadCount = %d, want 0 (a fresh chat starts at 0; campaign sends must not bump it)", chat.UnreadCount)
	}
}

// TestListCampaignRecipients_MessageDeliveryStateTracksTheLinkedMessage
// proves ListCampaignRecipients' own LEFT JOIN: a recipient with no message
// yet reports an empty MessageDeliveryState; once InsertCampaignOutbound +
// StampOutboundSent link a real message, it reports that message's own
// delivery_state — and tracks it live as AdvanceDeliveryState (the exact
// mechanism a real channel's delivery-receipt webhook, or Simulator's own
// ReceiptSimulator, calls) moves it forward. Uses channel='simulator'
// throughout (both the campaign and ListCampaignRecipients' own channel
// argument) since that is the join's new caller.
func TestListCampaignRecipients_MessageDeliveryStateTracksTheLinkedMessage(t *testing.T) {
	ctx := context.Background()
	st := dbtest.New(t)
	org, err := st.SeedOrganization(ctx, "campaigns-test-org-"+uuid.NewString())
	if err != nil {
		t.Fatalf("seed org: %v", err)
	}
	user, err := st.SeedUser(ctx, org.ID, uuid.NewString()+"@example.com", "hash", "Tester")
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}
	acct, err := st.GetOrCreateSimulatorAccount(ctx, org.ID)
	if err != nil {
		t.Fatalf("GetOrCreateSimulatorAccount: %v", err)
	}

	c, err := st.CreateCampaign(ctx, store.Campaign{
		OrganizationID: org.ID, Name: "Sim promo", AccountID: acct.ID, Channel: "simulator",
		MessageBody: "Hi!", CreatedBy: user.ID,
	})
	if err != nil {
		t.Fatalf("create campaign: %v", err)
	}
	if err := st.ReplaceCampaignRecipients(ctx, c.ID, []store.CampaignRecipientInput{
		{NormalizedIdentity: "77011234563", RawInput: "77011234563"},
	}); err != nil {
		t.Fatalf("ReplaceCampaignRecipients: %v", err)
	}

	before, _, err := st.ListCampaignRecipients(ctx, c.ID, "simulator", "", 50, 0)
	if err != nil {
		t.Fatalf("ListCampaignRecipients (before send): %v", err)
	}
	if len(before) != 1 || before[0].MessageDeliveryState != "" {
		t.Fatalf("before send = %+v, want one recipient with an empty MessageDeliveryState", before)
	}

	if _, err := st.SetCampaignStatus(ctx, c.ID, purecampaign.StatusRunning, uuid.NullUUID{UUID: user.ID, Valid: true}, "started", nil); err != nil {
		t.Fatalf("start campaign: %v", err)
	}
	claim, ok, err := st.ClaimNextRecipient(ctx, acct.ID, time.Now().UTC())
	if err != nil {
		t.Fatalf("ClaimNextRecipient: %v", err)
	}
	if !ok {
		t.Fatal("ClaimNextRecipient: ok = false, want true")
	}

	chatID, _, err := st.FindOrCreateChat(ctx, acct.ID, "77011234563@s.whatsapp.net", "77011234563")
	if err != nil {
		t.Fatalf("FindOrCreateChat: %v", err)
	}
	msgID, err := st.InsertCampaignOutbound(ctx, "simulator", chatID, acct.ID, "Hi!", "Hi!")
	if err != nil {
		t.Fatalf("InsertCampaignOutbound: %v", err)
	}
	externalID := "sim-" + msgID.String()
	if err := st.StampOutboundSent(ctx, "simulator", msgID, externalID); err != nil {
		t.Fatalf("StampOutboundSent: %v", err)
	}
	if err := st.FinalizeAttempt(ctx, store.FinalizeAttemptParams{
		LogID: claim.LogID, RecipientID: claim.RecipientID, NewStatus: purecampaign.RecipientSent,
		ChatID: uuid.NullUUID{UUID: chatID, Valid: true}, MessageID: uuid.NullUUID{UUID: msgID, Valid: true},
	}); err != nil {
		t.Fatalf("FinalizeAttempt: %v", err)
	}

	afterSend, _, err := st.ListCampaignRecipients(ctx, c.ID, "simulator", "", 50, 0)
	if err != nil {
		t.Fatalf("ListCampaignRecipients (after send): %v", err)
	}
	if len(afterSend) != 1 || afterSend[0].MessageDeliveryState != "sent" {
		t.Fatalf("after send = %+v, want MessageDeliveryState=sent", afterSend)
	}

	if _, _, err := st.AdvanceDeliveryState(ctx, "simulator", acct.ID, externalID, "delivered", 2); err != nil {
		t.Fatalf("AdvanceDeliveryState(delivered): %v", err)
	}
	afterDelivered, _, err := st.ListCampaignRecipients(ctx, c.ID, "simulator", "", 50, 0)
	if err != nil {
		t.Fatalf("ListCampaignRecipients (after delivered): %v", err)
	}
	if len(afterDelivered) != 1 || afterDelivered[0].MessageDeliveryState != "delivered" {
		t.Fatalf("after delivered = %+v, want MessageDeliveryState=delivered", afterDelivered)
	}

	if _, _, err := st.AdvanceDeliveryState(ctx, "simulator", acct.ID, externalID, "read", 3); err != nil {
		t.Fatalf("AdvanceDeliveryState(read): %v", err)
	}
	afterRead, _, err := st.ListCampaignRecipients(ctx, c.ID, "simulator", "", 50, 0)
	if err != nil {
		t.Fatalf("ListCampaignRecipients (after read): %v", err)
	}
	if len(afterRead) != 1 || afterRead[0].MessageDeliveryState != "read" {
		t.Fatalf("after read = %+v, want MessageDeliveryState=read", afterRead)
	}
	// The recipient's own coarse Status never grows a fourth value — it
	// stays 'sent' regardless of how far delivery_state advances.
	if afterRead[0].Status != string(purecampaign.RecipientSent) {
		t.Errorf("Status = %q, want unaffected sent", afterRead[0].Status)
	}
}

// TestSimulatorMessagesAwaitingReceipt proves the ReceiptSimulator sweep
// query itself: only simulator-channel, outbound, sent-or-delivered
// messages older than the cutoff come back, and a whatsapp-channel message
// (same shape, different account channel) is excluded even though it lives
// in the very same wa_messages table.
func TestSimulatorMessagesAwaitingReceipt(t *testing.T) {
	ctx := context.Background()
	st := dbtest.New(t)
	org, err := st.SeedOrganization(ctx, "sim-receipts-org-"+uuid.NewString())
	if err != nil {
		t.Fatalf("seed org: %v", err)
	}
	simAcct, err := st.GetOrCreateSimulatorAccount(ctx, org.ID)
	if err != nil {
		t.Fatalf("GetOrCreateSimulatorAccount: %v", err)
	}
	waAcct, err := st.SeedAccount(ctx, store.Account{
		ID: uuid.New(), OrganizationID: uuid.NullUUID{UUID: org.ID, Valid: true},
		DisplayName: "Real WA", ExternalAccountRef: "7770000001@s.whatsapp.net", ExternalHandle: "77700000001",
		ConnectionState: "connected",
	})
	if err != nil {
		t.Fatalf("seed wa account: %v", err)
	}

	simChat, _, err := st.FindOrCreateChat(ctx, simAcct.ID, "77011234563@s.whatsapp.net", "77011234563")
	if err != nil {
		t.Fatalf("FindOrCreateChat (sim): %v", err)
	}
	simMsgID, err := st.InsertCampaignOutbound(ctx, "simulator", simChat, simAcct.ID, "hi", "hi")
	if err != nil {
		t.Fatalf("InsertCampaignOutbound (sim): %v", err)
	}
	if err := st.StampOutboundSent(ctx, "simulator", simMsgID, "sim-"+simMsgID.String()); err != nil {
		t.Fatalf("StampOutboundSent (sim): %v", err)
	}

	waChat, _, err := st.FindOrCreateChat(ctx, waAcct.ID, "77011234564@s.whatsapp.net", "77011234564")
	if err != nil {
		t.Fatalf("FindOrCreateChat (wa): %v", err)
	}
	waMsgID, err := st.InsertCampaignOutbound(ctx, "whatsapp", waChat, waAcct.ID, "hi", "hi")
	if err != nil {
		t.Fatalf("InsertCampaignOutbound (wa): %v", err)
	}
	if err := st.StampOutboundSent(ctx, "whatsapp", waMsgID, "wamid-"+waMsgID.String()); err != nil {
		t.Fatalf("StampOutboundSent (wa): %v", err)
	}

	// A cutoff in the future includes every not-yet-advanced message —
	// exactly the simulator one, never the whatsapp one.
	future := time.Now().Add(time.Hour)
	candidates, err := st.SimulatorMessagesAwaitingReceipt(ctx, future)
	if err != nil {
		t.Fatalf("SimulatorMessagesAwaitingReceipt: %v", err)
	}
	if len(candidates) != 1 {
		t.Fatalf("candidates = %+v, want exactly the one simulator message", candidates)
	}
	got := candidates[0]
	if got.MessageID != simMsgID || got.AccountID != simAcct.ID || got.DeliveryState != "sent" {
		t.Errorf("candidate = %+v", got)
	}
	if got.Destination != "77011234563@s.whatsapp.net" {
		t.Errorf("Destination = %q", got.Destination)
	}

	// A cutoff strictly in the past excludes it again (too recent).
	past := time.Now().Add(-time.Hour)
	candidates, err = st.SimulatorMessagesAwaitingReceipt(ctx, past)
	if err != nil {
		t.Fatalf("SimulatorMessagesAwaitingReceipt (past cutoff): %v", err)
	}
	if len(candidates) != 0 {
		t.Fatalf("candidates = %+v, want none before their own update time", candidates)
	}
}
