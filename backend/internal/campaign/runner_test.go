package campaign

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	purecampaign "github.com/yerassyldanay/xchats/backend/campaign"
	"github.com/yerassyldanay/xchats/backend/internal/blob"
	"github.com/yerassyldanay/xchats/backend/internal/config"
	"github.com/yerassyldanay/xchats/backend/internal/dbtest"
	"github.com/yerassyldanay/xchats/backend/internal/store"
	"github.com/yerassyldanay/xchats/backend/messaging"
)

func testLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

// --- test doubles -----------------------------------------------------

type fakeHub struct {
	mu    sync.Mutex
	calls []string
}

func (h *fakeHub) Broadcast(name string, data any) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.calls = append(h.calls, name)
}

func (h *fakeHub) count(name string) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	n := 0
	for _, c := range h.calls {
		if c == name {
			n++
		}
	}
	return n
}

type fakeSender struct {
	result messaging.SendResult
	err    error

	mu    sync.Mutex
	calls []messaging.OutboundMessage
}

func (f *fakeSender) Send(ctx context.Context, out messaging.OutboundMessage) (messaging.SendResult, error) {
	f.mu.Lock()
	f.calls = append(f.calls, out)
	f.mu.Unlock()
	return f.result, f.err
}

func testBlob(t *testing.T) blob.Store {
	t.Helper()
	b, err := blob.NewDisk(t.TempDir())
	if err != nil {
		t.Fatalf("blob.NewDisk: %v", err)
	}
	return b
}

func testRunner(st *store.Store, hub *fakeHub, senders *messaging.SenderRegistry, t *testing.T) *Runner {
	return &Runner{Store: st, Blob: testBlob(t), Senders: senders, Hub: hub, Log: testLogger()}
}

// --- fixtures -----------------------------------------------------------

func seedWAFixture(t *testing.T, st *store.Store) (orgID, userID, accountID uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	org, err := st.SeedOrganization(ctx, "campaign-runner-"+uuid.NewString())
	if err != nil {
		t.Fatalf("seed org: %v", err)
	}
	user, err := st.SeedUser(ctx, org.ID, uuid.NewString()+"@example.com", "hash", "Tester")
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}
	ownerJID := t.Name() + "@s.whatsapp.net"
	accountID = config.AccountID(ownerJID)
	if _, err := st.SeedAccount(ctx, store.Account{
		ID: accountID, OrganizationID: uuid.NullUUID{UUID: org.ID, Valid: true},
		DisplayName: "WhatsApp", ExternalAccountRef: ownerJID,
		ExternalHandle: "77000000000", ConnectionState: "connected",
	}); err != nil {
		t.Fatalf("seed account: %v", err)
	}
	return org.ID, user.ID, accountID
}

// startCampaignWithFastPacing creates a campaign with one recipient, gives
// its account effectively unthrottled pacing, and moves it to 'running'.
func startCampaignWithFastPacing(t *testing.T, st *store.Store, orgID, accountID, userID uuid.UUID, channel, name, body, identity string) store.Campaign {
	t.Helper()
	ctx := context.Background()
	c, err := st.CreateCampaign(ctx, store.Campaign{
		OrganizationID: orgID, Name: name, AccountID: accountID, Channel: channel,
		MessageBody: body, CreatedBy: userID,
	})
	if err != nil {
		t.Fatalf("create campaign: %v", err)
	}
	if err := st.ReplaceCampaignRecipients(ctx, c.ID, []store.CampaignRecipientInput{
		{NormalizedIdentity: identity, Name: "Aigul"},
	}); err != nil {
		t.Fatalf("replace recipients: %v", err)
	}
	if _, _, _, err := st.SetCampaignAccountLimits(ctx, accountID,
		store.CampaignAccountSettingsInput{LimitMode: "custom", MinIntervalSeconds: 1, JitterSeconds: 0},
		[]purecampaign.Tier{{WindowSeconds: 1, MaxSends: 1000}}, nil); err != nil {
		t.Fatalf("set account limits: %v", err)
	}
	if _, err := st.SetCampaignStatus(ctx, c.ID, purecampaign.StatusRunning, uuid.NullUUID{UUID: userID, Valid: true}, "started", nil); err != nil {
		t.Fatalf("start campaign: %v", err)
	}
	return c
}

// --- tests ----------------------------------------------------------------

func TestRunner_ColdSendCreatesChatAndCompletesCampaign(t *testing.T) {
	ctx := context.Background()
	st := dbtest.New(t)
	orgID, userID, acctID := seedWAFixture(t, st)
	c := startCampaignWithFastPacing(t, st, orgID, acctID, userID, "whatsapp", "Promo", "Hi {{name}}, 20% off!", "77011234567")

	hub := &fakeHub{}
	senders := messaging.NewSenderRegistry()
	sender := &fakeSender{result: messaging.SendResult{ExternalID: "WAMID1", Delivered: true}}
	senders.Register(messaging.ChannelWhatsApp, sender)
	r := testRunner(st, hub, senders, t)

	if claimed := r.HandleAccount(ctx, acctID); !claimed {
		t.Fatal("HandleAccount: claimed = false, want true")
	}

	sender.mu.Lock()
	if len(sender.calls) != 1 {
		t.Fatalf("sender calls = %d, want 1", len(sender.calls))
	}
	call := sender.calls[0]
	sender.mu.Unlock()
	if call.Text != "Hi Aigul, 20% off!" {
		t.Errorf("rendered text = %q", call.Text)
	}
	if call.To != "77011234567@s.whatsapp.net" {
		t.Errorf("destination = %q, want the cold-send JID", call.To)
	}

	recipients, _, err := st.ListCampaignRecipients(ctx, c.ID, "whatsapp", "sent", 50, 0)
	if err != nil {
		t.Fatalf("ListCampaignRecipients: %v", err)
	}
	if len(recipients) != 1 || !recipients[0].ChatID.Valid || !recipients[0].MessageID.Valid {
		t.Fatalf("recipients = %+v", recipients)
	}

	// The freshly cold-send-created chat is flagged campaign-only — hidden
	// from the default inbox listing until the recipient replies.
	chat, err := st.ChatByID(ctx, recipients[0].ChatID.UUID)
	if err != nil {
		t.Fatalf("ChatByID: %v", err)
	}
	if chat.ChatState != "campaign" {
		t.Errorf("chat_state = %q, want campaign", chat.ChatState)
	}

	// Campaign had exactly one recipient, now terminal -> auto-completed.
	got, err := st.CampaignByID(ctx, c.ID)
	if err != nil {
		t.Fatalf("CampaignByID: %v", err)
	}
	if got.Status != string(purecampaign.StatusCompleted) {
		t.Errorf("campaign status = %q, want completed", got.Status)
	}

	for _, want := range []string{"chat.created", "message.created", "message.updated", "campaign.recipient_updated", "campaign.status_changed"} {
		if hub.count(want) == 0 {
			t.Errorf("hub never broadcast %q (calls: %v)", want, hub.calls)
		}
	}

	// A second HandleAccount call has nothing left to claim.
	if claimed := r.HandleAccount(ctx, acctID); claimed {
		t.Error("HandleAccount (2nd): claimed = true, want false (nothing left)")
	}
}

func TestRunner_TransientFailureSchedulesRetry(t *testing.T) {
	ctx := context.Background()
	st := dbtest.New(t)
	orgID, userID, acctID := seedWAFixture(t, st)
	c := startCampaignWithFastPacing(t, st, orgID, acctID, userID, "whatsapp", "Promo", "Hi!", "77011234567")

	hub := &fakeHub{}
	senders := messaging.NewSenderRegistry()
	sender := &fakeSender{err: errors.New("provider timeout")}
	senders.Register(messaging.ChannelWhatsApp, sender)
	r := testRunner(st, hub, senders, t)

	before := time.Now()
	if claimed := r.HandleAccount(ctx, acctID); !claimed {
		t.Fatal("HandleAccount: claimed = false, want true")
	}

	recipients, _, err := st.ListCampaignRecipients(ctx, c.ID, "whatsapp", "pending", 50, 0)
	if err != nil {
		t.Fatalf("ListCampaignRecipients: %v", err)
	}
	if len(recipients) != 1 {
		t.Fatalf("pending recipients = %d, want 1 (transient failure retried, not terminal)", len(recipients))
	}
	rec := recipients[0]
	if rec.Attempts != 1 {
		t.Errorf("Attempts = %d, want 1", rec.Attempts)
	}
	if rec.FailureReason != "provider timeout" {
		t.Errorf("FailureReason = %q", rec.FailureReason)
	}
	if rec.NextAttemptAt == nil || rec.NextAttemptAt.Before(before.Add(50*time.Second)) {
		t.Errorf("NextAttemptAt = %v, want ~1m after %v", rec.NextAttemptAt, before)
	}

	// The campaign must NOT be auto-completed while a recipient is still
	// pending a retry.
	got, err := st.CampaignByID(ctx, c.ID)
	if err != nil {
		t.Fatalf("CampaignByID: %v", err)
	}
	if got.Status != string(purecampaign.StatusRunning) {
		t.Errorf("campaign status = %q, want still running", got.Status)
	}
}

func TestRunner_PermanentFailureNeverRetries(t *testing.T) {
	ctx := context.Background()
	st := dbtest.New(t)
	orgID, userID, acctID := seedWAFixture(t, st)
	c := startCampaignWithFastPacing(t, st, orgID, acctID, userID, "whatsapp", "Promo", "Hi!", "77011234567")

	hub := &fakeHub{}
	senders := messaging.NewSenderRegistry()
	sender := &fakeSender{err: messaging.ErrOutsideServiceWindow}
	senders.Register(messaging.ChannelWhatsApp, sender)
	r := testRunner(st, hub, senders, t)

	if claimed := r.HandleAccount(ctx, acctID); !claimed {
		t.Fatal("HandleAccount: claimed = false, want true")
	}

	recipients, _, err := st.ListCampaignRecipients(ctx, c.ID, "whatsapp", "failed", 50, 0)
	if err != nil {
		t.Fatalf("ListCampaignRecipients: %v", err)
	}
	if len(recipients) != 1 {
		t.Fatalf("failed recipients = %d, want 1 (permanent failure, no retry)", len(recipients))
	}
	if recipients[0].NextAttemptAt != nil {
		t.Errorf("NextAttemptAt = %v, want nil (terminal)", recipients[0].NextAttemptAt)
	}

	// A single recipient, now terminal -> the campaign still auto-completes
	// even though its one send failed.
	got, err := st.CampaignByID(ctx, c.ID)
	if err != nil {
		t.Fatalf("CampaignByID: %v", err)
	}
	if got.Status != string(purecampaign.StatusCompleted) {
		t.Errorf("campaign status = %q, want completed", got.Status)
	}
}

// TestRunner_ErrRecipientUnreachableNeverRetries mirrors
// TestRunner_PermanentFailureNeverRetries exactly, for the OTHER permanent
// sentinel (messaging.ErrRecipientUnreachable, added for the Simulator
// channel's own deterministic "this destination is unreachable" outcome —
// see backend/internal/simulator.Outcome) — proving finalize() treats it
// identically to ErrOutsideServiceWindow: no retry, immediate terminal
// failure, campaign still auto-completes.
func TestRunner_ErrRecipientUnreachableNeverRetries(t *testing.T) {
	ctx := context.Background()
	st := dbtest.New(t)
	orgID, userID, acctID := seedWAFixture(t, st)
	c := startCampaignWithFastPacing(t, st, orgID, acctID, userID, "whatsapp", "Promo", "Hi!", "77011234567")

	hub := &fakeHub{}
	senders := messaging.NewSenderRegistry()
	sender := &fakeSender{err: messaging.ErrRecipientUnreachable}
	senders.Register(messaging.ChannelWhatsApp, sender)
	r := testRunner(st, hub, senders, t)

	if claimed := r.HandleAccount(ctx, acctID); !claimed {
		t.Fatal("HandleAccount: claimed = false, want true")
	}

	recipients, _, err := st.ListCampaignRecipients(ctx, c.ID, "whatsapp", "failed", 50, 0)
	if err != nil {
		t.Fatalf("ListCampaignRecipients: %v", err)
	}
	if len(recipients) != 1 {
		t.Fatalf("failed recipients = %d, want 1 (permanent failure, no retry)", len(recipients))
	}
	if recipients[0].NextAttemptAt != nil {
		t.Errorf("NextAttemptAt = %v, want nil (terminal)", recipients[0].NextAttemptAt)
	}

	got, err := st.CampaignByID(ctx, c.ID)
	if err != nil {
		t.Fatalf("CampaignByID: %v", err)
	}
	if got.Status != string(purecampaign.StatusCompleted) {
		t.Errorf("campaign status = %q, want completed", got.Status)
	}
}

func TestRunner_WarmOnlyChannelWithNoExistingChatFailsPermanently(t *testing.T) {
	ctx := context.Background()
	st := dbtest.New(t)
	org, err := st.SeedOrganization(ctx, "campaign-runner-warm-"+uuid.NewString())
	if err != nil {
		t.Fatalf("seed org: %v", err)
	}
	user, err := st.SeedUser(ctx, org.ID, uuid.NewString()+"@example.com", "hash", "Tester")
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}
	// Telegram is warm-only (ColdSendCapable == false); no chat has ever been
	// opened on this account, so resolveChat must fail before any send is
	// attempted at all — a defensive re-check of what preview should already
	// have filtered out.
	acctID := uuid.New()
	c := startCampaignWithFastPacing(t, st, org.ID, acctID, user.ID, "telegram", "Promo", "Hi!", "123456789")

	hub := &fakeHub{}
	senders := messaging.NewSenderRegistry()
	sender := &fakeSender{result: messaging.SendResult{ExternalID: "1", Delivered: true}}
	senders.Register(messaging.ChannelTelegram, sender)
	r := testRunner(st, hub, senders, t)

	if claimed := r.HandleAccount(ctx, acctID); !claimed {
		t.Fatal("HandleAccount: claimed = false, want true")
	}

	sender.mu.Lock()
	n := len(sender.calls)
	sender.mu.Unlock()
	if n != 0 {
		t.Errorf("sender.calls = %d, want 0 (must never attempt delivery with no chat)", n)
	}

	recipients, _, err := st.ListCampaignRecipients(ctx, c.ID, "whatsapp", "failed", 50, 0)
	if err != nil {
		t.Fatalf("ListCampaignRecipients: %v", err)
	}
	if len(recipients) != 1 {
		t.Fatalf("failed recipients = %d, want 1", len(recipients))
	}
	if recipients[0].FailureReason != errNoExistingChat.Error() {
		t.Errorf("FailureReason = %q, want %q", recipients[0].FailureReason, errNoExistingChat.Error())
	}
	if recipients[0].ChatID.Valid || recipients[0].MessageID.Valid {
		t.Errorf("recipient = %+v, want no chat/message linked", recipients[0])
	}
}

func TestRunner_WarmOnlyChannelReusesExistingChat(t *testing.T) {
	ctx := context.Background()
	st := dbtest.New(t)
	org, err := st.SeedOrganization(ctx, "campaign-runner-warm-ok-"+uuid.NewString())
	if err != nil {
		t.Fatalf("seed org: %v", err)
	}
	user, err := st.SeedUser(ctx, org.ID, uuid.NewString()+"@example.com", "hash", "Tester")
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}
	// WhatsApp Cloud is warm-only (ColdSendCapable == false) but, unlike
	// Instagram/Messenger, its channel_contacts.external_contact_id IS the
	// customer's E.164 phone digits — the same shape ParseRecipients
	// produces for every non-Telegram channel — so a pasted phone number
	// genuinely matches a real prior conversation here.
	acct, err := st.ClaimChannelAccount(ctx, store.ChannelAccountClaim{
		ID: config.ChannelAccountID(config.WhatsAppCloudOwnerRef("phone-id-1")), OrganizationID: org.ID,
		Channel: "whatsapp_cloud", ExternalAccountID: "phone-id-1",
	})
	if err != nil {
		t.Fatalf("claim channel account: %v", err)
	}
	ingestRes, err := st.IngestChannelInbound(ctx, store.ChannelInbound{
		AccountID: acct.ID, ExternalContactID: "77011234567", ContactHandle: "77011234567",
		ExternalThreadID: "77011234567", Direction: "in", SenderKind: "contact",
		ExternalMessageID: uuid.NewString(), MessageKind: "conversation", Body: "hi", Preview: "hi",
		MessageTS: time.Now(),
	})
	if err != nil {
		t.Fatalf("IngestChannelInbound: %v", err)
	}

	c := startCampaignWithFastPacing(t, st, org.ID, acct.ID, user.ID, "whatsapp_cloud", "Promo", "Hi!", "77011234567")

	hub := &fakeHub{}
	senders := messaging.NewSenderRegistry()
	sender := &fakeSender{result: messaging.SendResult{ExternalID: "1", Delivered: true}}
	senders.Register(messaging.ChannelWhatsAppCloud, sender)
	r := testRunner(st, hub, senders, t)

	if claimed := r.HandleAccount(ctx, acct.ID); !claimed {
		t.Fatal("HandleAccount: claimed = false, want true")
	}
	sender.mu.Lock()
	n := len(sender.calls)
	sender.mu.Unlock()
	if n != 1 {
		t.Fatalf("sender.calls = %d, want 1", n)
	}

	recipients, _, err := st.ListCampaignRecipients(ctx, c.ID, "whatsapp", "sent", 50, 0)
	if err != nil {
		t.Fatalf("ListCampaignRecipients: %v", err)
	}
	if len(recipients) != 1 || recipients[0].ChatID.UUID != ingestRes.ChatID {
		t.Fatalf("recipients = %+v, want chat_id = %s", recipients, ingestRes.ChatID)
	}
}
