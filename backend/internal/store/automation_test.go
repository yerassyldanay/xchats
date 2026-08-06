package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/internal/config"
	"github.com/yerassyldanay/xchats/backend/internal/dbtest"
	"github.com/yerassyldanay/xchats/backend/internal/store"
)

// seedOneChat builds one organization with a single connected WhatsApp
// account and one inbound message, returning the account and chat ids — the
// minimal fixture most automation store tests need.
func seedOneChat(t *testing.T, st *store.Store) (orgID, accountID, chatID uuid.UUID) {
	t.Helper()
	ctx := context.Background()

	org, err := st.SeedOrganization(ctx, t.Name())
	if err != nil {
		t.Fatalf("seed org: %v", err)
	}
	orgID = org.ID

	ownerJID := t.Name() + "@s.whatsapp.net"
	accountID = config.AccountID(ownerJID)
	if _, err := st.SeedAccount(ctx, store.Account{
		ID: accountID, OrganizationID: uuid.NullUUID{UUID: orgID, Valid: true},
		DisplayName: "WhatsApp", ExternalAccountRef: ownerJID,
		ExternalHandle: "77000000000", ConnectionState: "connected",
	}); err != nil {
		t.Fatalf("seed account: %v", err)
	}
	res, err := st.UpsertInbound(ctx, store.InboundUpsert{
		AccountID: accountID, PhoneJID: "77011111111@s.whatsapp.net",
		RemoteJID: "77011111111@s.whatsapp.net", PhoneNumber: "77011111111",
		PushName: "Клиент", Direction: "in", SenderKind: "contact",
		ExternalMessageID: uuid.NewString(), MessageKind: "conversation", Body: "привет",
		Preview: "привет", Source: "live_webhook", MessageTS: time.Now(),
	})
	if err != nil {
		t.Fatalf("inbound: %v", err)
	}
	return orgID, accountID, res.ChatID
}

// inbound sends one more inbound customer message into an already-seeded
// chat and returns its message id.
func inbound(t *testing.T, st *store.Store, accountID, chatContactRemoteJID uuid.UUID, body string, ts time.Time) uuid.UUID {
	t.Helper()
	res, err := st.UpsertInbound(context.Background(), store.InboundUpsert{
		AccountID: accountID, PhoneJID: "77011111111@s.whatsapp.net",
		RemoteJID: "77011111111@s.whatsapp.net", PhoneNumber: "77011111111",
		Direction: "in", SenderKind: "contact",
		ExternalMessageID: uuid.NewString(), MessageKind: "conversation", Body: body,
		Preview: body, Source: "live_webhook", MessageTS: ts,
	})
	if err != nil {
		t.Fatalf("inbound: %v", err)
	}
	return res.MessageID
}

func TestAutomationSettingsDefaultsToSuggestions(t *testing.T) {
	st := dbtest.New(t)
	ctx := context.Background()
	_, accountID, _ := seedOneChat(t, st)

	got, err := st.AutomationSettingsForAccount(ctx, accountID)
	if err != nil {
		t.Fatalf("AutomationSettingsForAccount: %v", err)
	}
	if got.Mode != "suggestions" {
		t.Errorf("default mode = %q, want suggestions", got.Mode)
	}
	if got.WaitSecondsOverride != nil {
		t.Errorf("default override = %v, want nil", got.WaitSecondsOverride)
	}

	// An account never configured is simply absent from the bulk map too.
	bulk, err := st.AutomationSettingsForAccounts(ctx, []uuid.UUID{accountID})
	if err != nil {
		t.Fatalf("AutomationSettingsForAccounts: %v", err)
	}
	if _, present := bulk[accountID]; present {
		t.Errorf("unconfigured account unexpectedly present in bulk map: %+v", bulk[accountID])
	}
}

func TestSetAutomationSettingsPersistsModeOverrideAndWindows(t *testing.T) {
	st := dbtest.New(t)
	ctx := context.Background()
	_, accountID, _ := seedOneChat(t, st)

	override := 30
	settings, windows, err := st.SetAutomationSettings(ctx, accountID, "scheduled_auto", &override, []store.AutomationWindowInput{
		{Weekday: 1, StartMinute: 540, EndMinute: 1020},
		{Weekday: 2, StartMinute: 540, EndMinute: 1020},
	})
	if err != nil {
		t.Fatalf("SetAutomationSettings: %v", err)
	}
	if settings.Mode != "scheduled_auto" {
		t.Errorf("mode = %q", settings.Mode)
	}
	if settings.WaitSecondsOverride == nil || *settings.WaitSecondsOverride != 30 {
		t.Errorf("override = %v, want 30", settings.WaitSecondsOverride)
	}
	if len(windows) != 2 {
		t.Fatalf("windows = %d, want 2", len(windows))
	}

	reread, err := st.AutomationSettingsForAccount(ctx, accountID)
	if err != nil {
		t.Fatalf("reread: %v", err)
	}
	if reread.Mode != "scheduled_auto" || reread.WaitSecondsOverride == nil || *reread.WaitSecondsOverride != 30 {
		t.Fatalf("reread settings = %+v", reread)
	}
	rereadWindows, err := st.AutomationWindowsForAccount(ctx, accountID)
	if err != nil {
		t.Fatalf("reread windows: %v", err)
	}
	if len(rereadWindows) != 2 {
		t.Fatalf("reread windows = %d, want 2", len(rereadWindows))
	}

	// A second save fully replaces the schedule rather than accumulating.
	_, windows2, err := st.SetAutomationSettings(ctx, accountID, "scheduled_auto", nil, []store.AutomationWindowInput{
		{Weekday: 6, StartMinute: 0, EndMinute: 1440},
	})
	if err != nil {
		t.Fatalf("second SetAutomationSettings: %v", err)
	}
	if len(windows2) != 1 {
		t.Fatalf("windows after replace = %d, want 1", len(windows2))
	}
	final, err := st.AutomationSettingsForAccount(ctx, accountID)
	if err != nil {
		t.Fatalf("final reread: %v", err)
	}
	if final.WaitSecondsOverride != nil {
		t.Errorf("override after clearing = %v, want nil", final.WaitSecondsOverride)
	}
}

func TestSetAutomationSettingsOffInvalidatesPendingWork(t *testing.T) {
	st := dbtest.New(t)
	ctx := context.Background()
	_, accountID, chatID := seedOneChat(t, st)

	if err := st.ArmDebounce(ctx, chatID, accountID, "whatsapp", time.Now().Add(5*time.Second)); err != nil {
		t.Fatalf("ArmDebounce: %v", err)
	}
	if _, err := st.CreateDispatchJob(ctx, chatID, accountID, "whatsapp", 1); err != nil {
		t.Fatalf("CreateDispatchJob: %v", err)
	}

	if _, _, err := st.SetAutomationSettings(ctx, accountID, "off", nil, nil); err != nil {
		t.Fatalf("SetAutomationSettings(off): %v", err)
	}

	due, err := st.ClaimDueDebounceJobs(ctx, time.Now().Add(time.Hour), 10)
	if err != nil {
		t.Fatalf("ClaimDueDebounceJobs: %v", err)
	}
	for _, j := range due {
		if j.ChatID == chatID {
			t.Fatal("switching to off should have deleted the pending debounce job")
		}
	}

	recoverable, err := st.RecoverableDispatchJobs(ctx, time.Now().Add(time.Hour), 10)
	if err != nil {
		t.Fatalf("RecoverableDispatchJobs: %v", err)
	}
	for _, j := range recoverable {
		if j.ChatID == chatID {
			t.Fatal("switching to off should have deleted the pending dispatch job")
		}
	}
}

func TestArmDebounceResetsDeadlineAndBumpsVersion(t *testing.T) {
	st := dbtest.New(t)
	ctx := context.Background()
	_, accountID, chatID := seedOneChat(t, st)

	first := time.Now().Add(5 * time.Second)
	if err := st.ArmDebounce(ctx, chatID, accountID, "whatsapp", first); err != nil {
		t.Fatalf("ArmDebounce #1: %v", err)
	}
	v1, err := st.CurrentBurstVersion(ctx, chatID)
	if err != nil {
		t.Fatalf("CurrentBurstVersion: %v", err)
	}
	if v1 != 1 {
		t.Fatalf("version after first arm = %d, want 1", v1)
	}

	// A second inbound message resets the deadline forward and bumps the
	// version — no maximum burst cap, this can repeat indefinitely.
	second := first.Add(5 * time.Second)
	if err := st.ArmDebounce(ctx, chatID, accountID, "whatsapp", second); err != nil {
		t.Fatalf("ArmDebounce #2: %v", err)
	}
	v2, err := st.CurrentBurstVersion(ctx, chatID)
	if err != nil {
		t.Fatalf("CurrentBurstVersion #2: %v", err)
	}
	if v2 != 2 {
		t.Fatalf("version after second arm = %d, want 2", v2)
	}

	due, err := st.ClaimDueDebounceJobs(ctx, first.Add(time.Millisecond), 10)
	if err != nil {
		t.Fatalf("ClaimDueDebounceJobs: %v", err)
	}
	if len(due) != 0 {
		t.Fatalf("the reset deadline should not be due yet at the OLD deadline: got %d due jobs", len(due))
	}
	due, err = st.ClaimDueDebounceJobs(ctx, second.Add(time.Millisecond), 10)
	if err != nil {
		t.Fatalf("ClaimDueDebounceJobs at the new deadline: %v", err)
	}
	if len(due) != 1 || due[0].ChatID != chatID || due[0].BurstVersion != 2 {
		t.Fatalf("due jobs = %+v, want exactly chat %s at version 2", due, chatID)
	}
}

func TestDebounceChannelIsolation(t *testing.T) {
	st := dbtest.New(t)
	ctx := context.Background()
	_, acctA, chatA := seedOneChat(t, st)

	// A second, independent org+account+chat.
	org2, err := st.SeedOrganization(ctx, t.Name()+"-2")
	if err != nil {
		t.Fatalf("seed org 2: %v", err)
	}
	ownerJID2 := t.Name() + "-2@s.whatsapp.net"
	acctB := config.AccountID(ownerJID2)
	if _, err := st.SeedAccount(ctx, store.Account{
		ID: acctB, OrganizationID: uuid.NullUUID{UUID: org2.ID, Valid: true},
		DisplayName: "WhatsApp 2", ExternalAccountRef: ownerJID2,
		ExternalHandle: "77022222222", ConnectionState: "connected",
	}); err != nil {
		t.Fatalf("seed account 2: %v", err)
	}
	res2, err := st.UpsertInbound(ctx, store.InboundUpsert{
		AccountID: acctB, PhoneJID: "77033333333@s.whatsapp.net",
		RemoteJID: "77033333333@s.whatsapp.net", PhoneNumber: "77033333333",
		Direction: "in", SenderKind: "contact", ExternalMessageID: uuid.NewString(),
		MessageKind: "conversation", Body: "hi", Preview: "hi", Source: "live_webhook", MessageTS: time.Now(),
	})
	if err != nil {
		t.Fatalf("inbound 2: %v", err)
	}
	chatB := res2.ChatID

	now := time.Now()
	if err := st.ArmDebounce(ctx, chatA, acctA, "whatsapp", now.Add(1*time.Second)); err != nil {
		t.Fatalf("arm A: %v", err)
	}
	if err := st.ArmDebounce(ctx, chatB, acctB, "whatsapp", now.Add(100*time.Second)); err != nil {
		t.Fatalf("arm B: %v", err)
	}

	// Only chat A's deadline has passed; chat B's independent, much later
	// deadline must not be affected by (or claimable alongside) it.
	due, err := st.ClaimDueDebounceJobs(ctx, now.Add(2*time.Second), 10)
	if err != nil {
		t.Fatalf("ClaimDueDebounceJobs: %v", err)
	}
	if len(due) != 1 || due[0].ChatID != chatA {
		t.Fatalf("due jobs = %+v, want exactly chat A", due)
	}

	vA, _ := st.CurrentBurstVersion(ctx, chatA)
	vB, _ := st.CurrentBurstVersion(ctx, chatB)
	if vA != 1 || vB != 1 {
		t.Fatalf("versions should be independent: A=%d B=%d", vA, vB)
	}
}

func TestDispatchJobRestartRecovery(t *testing.T) {
	st := dbtest.New(t)
	ctx := context.Background()
	_, accountID, chatID := seedOneChat(t, st)

	pendingID, err := st.CreateDispatchJob(ctx, chatID, accountID, "whatsapp", 1)
	if err != nil {
		t.Fatalf("CreateDispatchJob: %v", err)
	}

	// A job that never even started (still 'pending') is recoverable
	// immediately — no stuck-cutoff needed, since it never ran at all.
	recoverable, err := st.RecoverableDispatchJobs(ctx, time.Now(), 10)
	if err != nil {
		t.Fatalf("RecoverableDispatchJobs: %v", err)
	}
	if !containsDispatchID(recoverable, pendingID) {
		t.Fatalf("a pending dispatch job should be immediately recoverable: %+v", recoverable)
	}

	if err := st.MarkDispatchProcessing(ctx, pendingID); err != nil {
		t.Fatalf("MarkDispatchProcessing: %v", err)
	}

	// Freshly marked 'processing' — not yet past any reasonable stuck cutoff.
	notYetStuck, err := st.RecoverableDispatchJobs(ctx, time.Now().Add(-time.Hour), 10)
	if err != nil {
		t.Fatalf("RecoverableDispatchJobs (not stuck): %v", err)
	}
	if containsDispatchID(notYetStuck, pendingID) {
		t.Fatal("a just-started processing job should not be recoverable yet")
	}

	// Simulate a crash: past a stuck-cutoff in the future, it must be
	// recoverable — this is what a restart's reconcile pass relies on.
	stuck, err := st.RecoverableDispatchJobs(ctx, time.Now().Add(time.Hour), 10)
	if err != nil {
		t.Fatalf("RecoverableDispatchJobs (stuck): %v", err)
	}
	if !containsDispatchID(stuck, pendingID) {
		t.Fatal("a processing job stuck past the cutoff should be recoverable")
	}

	if err := st.DeleteDispatchJob(ctx, pendingID); err != nil {
		t.Fatalf("DeleteDispatchJob: %v", err)
	}
	if _, err := st.DispatchJobByID(ctx, pendingID); err != store.ErrNotFound {
		t.Fatalf("DispatchJobByID after delete = %v, want ErrNotFound", err)
	}
}

func containsDispatchID(jobs []store.DispatchJob, id uuid.UUID) bool {
	for _, j := range jobs {
		if j.ID == id {
			return true
		}
	}
	return false
}

func TestWriteDraftSetIfVersionCurrentRejectsStaleGeneration(t *testing.T) {
	st := dbtest.New(t)
	ctx := context.Background()
	_, accountID, chatID := seedOneChat(t, st)

	if err := st.ArmDebounce(ctx, chatID, accountID, "whatsapp", time.Now()); err != nil {
		t.Fatalf("ArmDebounce: %v", err)
	}
	// A new message arrives (say, while the first burst's LLM call is still
	// running) and bumps the version to 2.
	if err := st.ArmDebounce(ctx, chatID, accountID, "whatsapp", time.Now()); err != nil {
		t.Fatalf("ArmDebounce #2: %v", err)
	}

	// The stale (version 1) generation tries to persist and must be
	// discarded — no draft written, no error.
	drafts, written, err := st.WriteDraftSetIfVersionCurrent(ctx, "whatsapp", chatID, 1, uuid.NullUUID{}, []store.DraftOption{
		{Ordinal: 1, Text: "stale answer", ReplyLanguage: "ru"},
	})
	if err != nil {
		t.Fatalf("stale write: %v", err)
	}
	if written || len(drafts) != 0 {
		t.Fatalf("stale write should discard: written=%v drafts=%v", written, drafts)
	}
	pending, err := st.PendingDrafts(ctx, chatID)
	if err != nil {
		t.Fatalf("PendingDrafts: %v", err)
	}
	if len(pending) != 0 {
		t.Fatalf("a discarded stale draft must never become visible: %+v", pending)
	}

	// The fresh (version 2) generation persists normally.
	drafts, written, err = st.WriteDraftSetIfVersionCurrent(ctx, "whatsapp", chatID, 2, uuid.NullUUID{}, []store.DraftOption{
		{Ordinal: 1, Text: "fresh answer", ReplyLanguage: "ru"},
	})
	if err != nil {
		t.Fatalf("fresh write: %v", err)
	}
	if !written || len(drafts) != 1 || drafts[0].DraftText != "fresh answer" {
		t.Fatalf("fresh write = written=%v drafts=%+v", written, drafts)
	}
	pending, err = st.PendingDrafts(ctx, chatID)
	if err != nil || len(pending) != 1 || pending[0].DraftText != "fresh answer" {
		t.Fatalf("pending after fresh write = %+v, %v", pending, err)
	}
}

func TestClaimDraftForAutoSendHappyPath(t *testing.T) {
	st := dbtest.New(t)
	ctx := context.Background()
	_, accountID, chatID := seedOneChat(t, st)

	trigger, err := st.LatestInboundMessageID(ctx, chatID)
	if err != nil || !trigger.Valid {
		t.Fatalf("LatestInboundMessageID: %v, %v", trigger, err)
	}
	if _, _, err := st.SetAutomationSettings(ctx, accountID, "scheduled_auto", nil, nil); err != nil {
		t.Fatalf("SetAutomationSettings: %v", err)
	}
	drafts, err := st.WriteDraftSet(ctx, "whatsapp", chatID, trigger, []store.DraftOption{
		{Ordinal: 1, Text: "auto answer", ReplyLanguage: "ru"},
	})
	if err != nil || len(drafts) != 1 {
		t.Fatalf("WriteDraftSet: %v, %+v", err, drafts)
	}

	sent, ok, err := st.ClaimDraftForAutoSend(ctx, drafts[0].ID, chatID, accountID, trigger)
	if err != nil {
		t.Fatalf("ClaimDraftForAutoSend: %v", err)
	}
	if !ok {
		t.Fatal("expected auto-send to be allowed on the happy path")
	}
	if sent.DraftState != "sent" {
		t.Fatalf("draft state = %q, want sent", sent.DraftState)
	}
}

func TestClaimDraftForAutoSendBlockedByNonScheduledAutoMode(t *testing.T) {
	st := dbtest.New(t)
	ctx := context.Background()
	_, accountID, chatID := seedOneChat(t, st)
	trigger, _ := st.LatestInboundMessageID(ctx, chatID)

	// Default mode is "suggestions" — never auto-send.
	drafts, err := st.WriteDraftSet(ctx, "whatsapp", chatID, trigger, []store.DraftOption{
		{Ordinal: 1, Text: "answer", ReplyLanguage: "ru"},
	})
	if err != nil {
		t.Fatalf("WriteDraftSet: %v", err)
	}
	_, ok, err := st.ClaimDraftForAutoSend(ctx, drafts[0].ID, chatID, accountID, trigger)
	if err != nil {
		t.Fatalf("ClaimDraftForAutoSend: %v", err)
	}
	if ok {
		t.Fatal("suggestions mode must never auto-send")
	}
	pending, err := st.PendingDrafts(ctx, chatID)
	if err != nil || len(pending) != 1 {
		t.Fatalf("the draft should remain suggested/visible: %+v, %v", pending, err)
	}
}

func TestClaimDraftForAutoSendBlockedByHumanReply(t *testing.T) {
	st := dbtest.New(t)
	ctx := context.Background()
	_, accountID, chatID := seedOneChat(t, st)
	trigger, _ := st.LatestInboundMessageID(ctx, chatID)
	if _, _, err := st.SetAutomationSettings(ctx, accountID, "scheduled_auto", nil, nil); err != nil {
		t.Fatalf("SetAutomationSettings: %v", err)
	}
	drafts, err := st.WriteDraftSet(ctx, "whatsapp", chatID, trigger, []store.DraftOption{
		{Ordinal: 1, Text: "answer", ReplyLanguage: "ru"},
	})
	if err != nil {
		t.Fatalf("WriteDraftSet: %v", err)
	}

	// An operator (or the WhatsApp number's own owner) replies manually
	// before the auto-send recheck runs.
	if _, err := st.InsertOutbound(ctx, "whatsapp", chatID, accountID, "user", uuid.NullUUID{}, "conversation", "оператор уже ответил", "оператор уже ответил"); err != nil {
		t.Fatalf("InsertOutbound (human): %v", err)
	}

	_, ok, err := st.ClaimDraftForAutoSend(ctx, drafts[0].ID, chatID, accountID, trigger)
	if err != nil {
		t.Fatalf("ClaimDraftForAutoSend: %v", err)
	}
	if ok {
		t.Fatal("a human reply since the trigger must cancel auto-send")
	}
}

func TestClaimDraftForAutoSendBlockedByNewerInboundMessage(t *testing.T) {
	st := dbtest.New(t)
	ctx := context.Background()
	_, accountID, chatID := seedOneChat(t, st)
	trigger, _ := st.LatestInboundMessageID(ctx, chatID)
	if _, _, err := st.SetAutomationSettings(ctx, accountID, "scheduled_auto", nil, nil); err != nil {
		t.Fatalf("SetAutomationSettings: %v", err)
	}
	drafts, err := st.WriteDraftSet(ctx, "whatsapp", chatID, trigger, []store.DraftOption{
		{Ordinal: 1, Text: "answer", ReplyLanguage: "ru"},
	})
	if err != nil {
		t.Fatalf("WriteDraftSet: %v", err)
	}

	// The customer sends another message before the auto-send recheck runs.
	inbound(t, st, accountID, chatID, "ещё один вопрос", time.Now().Add(time.Second))

	_, ok, err := st.ClaimDraftForAutoSend(ctx, drafts[0].ID, chatID, accountID, trigger)
	if err != nil {
		t.Fatalf("ClaimDraftForAutoSend: %v", err)
	}
	if ok {
		t.Fatal("a newer inbound message must cancel auto-send for the stale draft")
	}
}

func TestClaimDraftForAutoSendBlockedByConcurrentManualApprove(t *testing.T) {
	st := dbtest.New(t)
	ctx := context.Background()
	_, accountID, chatID := seedOneChat(t, st)
	trigger, _ := st.LatestInboundMessageID(ctx, chatID)
	if _, _, err := st.SetAutomationSettings(ctx, accountID, "scheduled_auto", nil, nil); err != nil {
		t.Fatalf("SetAutomationSettings: %v", err)
	}
	drafts, err := st.WriteDraftSet(ctx, "whatsapp", chatID, trigger, []store.DraftOption{
		{Ordinal: 1, Text: "answer", ReplyLanguage: "ru"},
	})
	if err != nil {
		t.Fatalf("WriteDraftSet: %v", err)
	}

	// An operator manually approves (claims) the draft first.
	if _, err := st.ClaimDraft(ctx, drafts[0].ID); err != nil {
		t.Fatalf("ClaimDraft (manual): %v", err)
	}

	_, ok, err := st.ClaimDraftForAutoSend(ctx, drafts[0].ID, chatID, accountID, trigger)
	if err != nil {
		t.Fatalf("ClaimDraftForAutoSend: %v", err)
	}
	if ok {
		t.Fatal("a draft already claimed by a manual approve must not also be auto-sent")
	}
}

func TestInsertAutomationOutboundDoesNotClearUnread(t *testing.T) {
	st := dbtest.New(t)
	ctx := context.Background()
	_, accountID, chatID := seedOneChat(t, st)

	before, err := st.ChatByID(ctx, chatID)
	if err != nil {
		t.Fatalf("ChatByID: %v", err)
	}
	if before.UnreadCount == 0 {
		t.Fatal("fixture should start with unread > 0")
	}

	if _, err := st.InsertAutomationOutbound(ctx, "whatsapp", chatID, accountID, "auto reply", "auto reply"); err != nil {
		t.Fatalf("InsertAutomationOutbound: %v", err)
	}

	after, err := st.ChatByID(ctx, chatID)
	if err != nil {
		t.Fatalf("ChatByID after auto-send: %v", err)
	}
	if after.UnreadCount != before.UnreadCount {
		t.Fatalf("unread_count changed after an automated send: before=%d after=%d", before.UnreadCount, after.UnreadCount)
	}
	if after.LastMessagePreview != "auto reply" {
		t.Fatalf("last_message_preview = %q, want the auto-reply text", after.LastMessagePreview)
	}

	// Contrast: the ordinary InsertOutbound path (used by a manual/operator
	// or approved-AI send) DOES clear it — this pins the existing behavior
	// automation deliberately diverges from.
	if _, err := st.InsertOutbound(ctx, "whatsapp", chatID, accountID, "ai", uuid.NullUUID{}, "conversation", "approved reply", "approved reply"); err != nil {
		t.Fatalf("InsertOutbound: %v", err)
	}
	afterApprove, err := st.ChatByID(ctx, chatID)
	if err != nil {
		t.Fatalf("ChatByID after approve: %v", err)
	}
	if afterApprove.UnreadCount != 0 {
		t.Fatalf("a manually-approved send should still clear unread_count, got %d", afterApprove.UnreadCount)
	}
}
