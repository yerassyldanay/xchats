package campaign

import (
	"context"
	"errors"
	"testing"
	"time"

	purecampaign "github.com/yerassyldanay/xchats/backend/campaign"
	"github.com/yerassyldanay/xchats/backend/internal/dbtest"
	"github.com/yerassyldanay/xchats/backend/messaging"
)

// useFastRetryBackoff shrinks backend/campaign's real 1m/5m/25m ladder to a
// few milliseconds for the duration of one test, restoring it on cleanup —
// the actual backoff is far too long to wait out in a test, and
// store.Store deliberately exposes no "skip the backoff" method (there is
// no legitimate product reason to jump a queue).
func useFastRetryBackoff(t *testing.T) {
	t.Helper()
	orig := purecampaign.TransientBackoff
	purecampaign.TransientBackoff = []time.Duration{5 * time.Millisecond, 5 * time.Millisecond, 5 * time.Millisecond}
	t.Cleanup(func() { purecampaign.TransientBackoff = orig })
}

// TestRunner_RetryReusesExistingMessage_NoDuplicate is the regression test
// for the bug this feature fixes: before Claim carried ChatID/MessageID,
// EVERY claim (including a retry) called resolveChat+InsertCampaignOutbound
// again, so a recipient that failed once and then succeeded produced TWO
// message rows in the conversation for one logical send. Now a retry must
// reuse the exact same chat/message row and only re-attempt delivery.
func TestRunner_RetryReusesExistingMessage_NoDuplicate(t *testing.T) {
	useFastRetryBackoff(t)
	ctx := context.Background()
	st := dbtest.New(t)
	orgID, userID, acctID := seedWAFixture(t, st)
	c := startCampaignWithFastPacing(t, st, orgID, acctID, userID, "whatsapp", "Promo", "Hi!", "77011234567")

	hub := &fakeHub{}
	senders := messaging.NewSenderRegistry()
	sender := &fakeSender{err: errors.New("transient hiccup")}
	senders.Register(messaging.ChannelWhatsApp, sender)
	r := testRunner(st, hub, senders, t)

	// Attempt 1: fails transiently, scheduled for retry.
	if claimed := r.HandleAccount(ctx, acctID); !claimed {
		t.Fatal("HandleAccount (1st): claimed = false, want true")
	}
	pending, _, err := st.ListCampaignRecipients(ctx, c.ID, "whatsapp", "pending", 50, 0)
	if err != nil || len(pending) != 1 {
		t.Fatalf("ListCampaignRecipients (pending): %v, %+v", err, pending)
	}
	recipientID := pending[0].ID
	firstMessageID := pending[0].MessageID
	firstChatID := pending[0].ChatID
	if !firstMessageID.Valid || !firstChatID.Valid {
		t.Fatalf("first attempt did not resolve chat/message: %+v", pending[0])
	}

	// Attempt 2: the same sender now succeeds, once both the (shrunk) retry
	// backoff AND the account's own 1s min-send-interval pacing
	// (startCampaignWithFastPacing's own setting) have passed.
	sender.mu.Lock()
	sender.err = nil
	sender.result = messaging.SendResult{ExternalID: "WAMID-RETRY", Delivered: true}
	sender.mu.Unlock()
	time.Sleep(1100 * time.Millisecond)

	if claimed := r.HandleAccount(ctx, acctID); !claimed {
		t.Fatal("HandleAccount (2nd): claimed = false, want true")
	}

	sent, _, err := st.ListCampaignRecipients(ctx, c.ID, "whatsapp", "sent", 50, 0)
	if err != nil || len(sent) != 1 {
		t.Fatalf("ListCampaignRecipients (sent): %v, %+v", err, sent)
	}
	if sent[0].MessageID != firstMessageID {
		t.Errorf("message_id after retry = %v, want the SAME message id from attempt 1 (%v) — a retry must never create a second message", sent[0].MessageID, firstMessageID)
	}
	if sent[0].ChatID != firstChatID {
		t.Errorf("chat_id after retry = %v, want the SAME chat id from attempt 1 (%v)", sent[0].ChatID, firstChatID)
	}
	if sent[0].Attempts != 2 {
		t.Errorf("Attempts = %d, want 2", sent[0].Attempts)
	}

	// Exactly one message must exist in the chat's history — not two.
	msgs, _, err := st.MessagesForChat(ctx, firstChatID.UUID, time.Time{}, 50)
	if err != nil {
		t.Fatalf("MessagesForChat: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("messages in chat = %d, want exactly 1 (got %+v)", len(msgs), msgs)
	}
	if msgs[0].ID != firstMessageID.UUID {
		t.Errorf("the one message in the chat is %v, want %v", msgs[0].ID, firstMessageID.UUID)
	}
	if msgs[0].DeliveryState != "sent" {
		t.Errorf("delivery_state = %q, want sent (the retry's own success)", msgs[0].DeliveryState)
	}

	// The sender itself was invoked twice (once per attempt) even though
	// only one message row and one chat exist.
	sender.mu.Lock()
	n := len(sender.calls)
	sender.mu.Unlock()
	if n != 2 {
		t.Errorf("sender.calls = %d, want 2 (one per attempt)", n)
	}

	// Two distinct campaign_send_attempts rows trace both tries against the
	// SAME recipient, in order.
	attempts, err := st.ListAttemptsForRecipient(ctx, recipientID)
	if err != nil {
		t.Fatalf("ListAttemptsForRecipient: %v", err)
	}
	if len(attempts) != 2 {
		t.Fatalf("attempts = %d, want 2", len(attempts))
	}
	if attempts[0].Outcome != "failed" || attempts[0].AttemptNumber != 1 {
		t.Errorf("attempt 1 = %+v, want failed/#1", attempts[0])
	}
	if attempts[1].Outcome != "provider_accepted" || attempts[1].AttemptNumber != 2 {
		t.Errorf("attempt 2 = %+v, want provider_accepted/#2", attempts[1])
	}
	if attempts[1].ProviderMessageID != "WAMID-RETRY" {
		t.Errorf("attempt 2 provider_message_id = %q, want WAMID-RETRY", attempts[1].ProviderMessageID)
	}
}

// TestRunner_AmbiguousTimeoutIsUnknownAndStopsRetries proves a send timeout
// is treated as an unresolved outcome, not a confirmed failure eligible for
// the normal transient ladder: the coarse recipient status still becomes
// 'failed' (existing, unchanged vocabulary), but the attempt itself is
// recorded 'unknown', and — critically — next_attempt_at is never set, so a
// later tick can never blindly re-claim and re-send it.
func TestRunner_AmbiguousTimeoutIsUnknownAndStopsRetries(t *testing.T) {
	ctx := context.Background()
	st := dbtest.New(t)
	orgID, userID, acctID := seedWAFixture(t, st)
	c := startCampaignWithFastPacing(t, st, orgID, acctID, userID, "whatsapp", "Promo", "Hi!", "77011234567")

	hub := &fakeHub{}
	senders := messaging.NewSenderRegistry()
	sender := &fakeSender{err: errors.Join(messaging.ErrSendTimeout, errors.New("context deadline exceeded"))}
	senders.Register(messaging.ChannelWhatsApp, sender)
	r := testRunner(st, hub, senders, t)

	if claimed := r.HandleAccount(ctx, acctID); !claimed {
		t.Fatal("HandleAccount: claimed = false, want true")
	}

	failed, _, err := st.ListCampaignRecipients(ctx, c.ID, "whatsapp", "failed", 50, 0)
	if err != nil || len(failed) != 1 {
		t.Fatalf("ListCampaignRecipients (failed): %v, %+v", err, failed)
	}
	if failed[0].NextAttemptAt != nil {
		t.Errorf("NextAttemptAt = %v, want nil — a timeout must never be scheduled for automatic retry", failed[0].NextAttemptAt)
	}

	attempts, err := st.ListAttemptsForRecipient(ctx, failed[0].ID)
	if err != nil || len(attempts) != 1 {
		t.Fatalf("ListAttemptsForRecipient: %v, %+v", err, attempts)
	}
	if attempts[0].Outcome != "unknown" {
		t.Errorf("attempt outcome = %q, want unknown", attempts[0].Outcome)
	}
	if attempts[0].ErrorCode != string(ErrorCodeTimeout) {
		t.Errorf("attempt error_code = %q, want %q", attempts[0].ErrorCode, ErrorCodeTimeout)
	}
	if attempts[0].Retryable == nil || *attempts[0].Retryable {
		t.Errorf("attempt retryable = %v, want false", attempts[0].Retryable)
	}

	// A later tick (pacing/backoff aside) must not re-claim this recipient:
	// its status is 'failed', which ClaimNextRecipient's own WHERE clause
	// (status = 'pending') can never match.
	if claimed := r.HandleAccount(ctx, acctID); claimed {
		t.Error("HandleAccount (2nd): claimed = true, want false — an unresolved timeout must never be blindly resent")
	}
	sender.mu.Lock()
	n := len(sender.calls)
	sender.mu.Unlock()
	if n != 1 {
		t.Errorf("sender.calls = %d, want 1 (no automatic resend after an ambiguous timeout)", n)
	}
}

// TestRunner_ErrorClassificationProducesStructuredCodes is a table test over
// every messaging.Err* sentinel an adapter can wrap a send failure in,
// confirming Runner.finalize persists the matching structured error_code on
// campaign_send_attempts (not just a free-text reason) and the right
// retryable/terminal disposition for each.
func TestRunner_ErrorClassificationProducesStructuredCodes(t *testing.T) {
	cases := []struct {
		name          string
		err           error
		wantCode      ErrorCode
		wantRecipient purecampaign.RecipientStatus
		wantRetryable bool
	}{
		{"disconnected", messaging.ErrAccountDisconnected, ErrorCodeWhatsAppDisconnected, purecampaign.RecipientPending, true},
		{"invalid_recipient", messaging.ErrInvalidRecipient, ErrorCodeInvalidRecipient, purecampaign.RecipientFailed, false},
		{"service_window", messaging.ErrOutsideServiceWindow, ErrorCodeServiceWindow, purecampaign.RecipientFailed, false},
		{"network", messaging.ErrNetwork, ErrorCodeNetwork, purecampaign.RecipientPending, true},
		{"message_build_failed", messaging.ErrMessageBuildFailed, ErrorCodeMessageCreationFailed, purecampaign.RecipientPending, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			st := dbtest.New(t)
			orgID, userID, acctID := seedWAFixture(t, st)
			c := startCampaignWithFastPacing(t, st, orgID, acctID, userID, "whatsapp", "Promo", "Hi!", "77011234567")

			hub := &fakeHub{}
			senders := messaging.NewSenderRegistry()
			sender := &fakeSender{err: tc.err}
			senders.Register(messaging.ChannelWhatsApp, sender)
			r := testRunner(st, hub, senders, t)

			if claimed := r.HandleAccount(ctx, acctID); !claimed {
				t.Fatal("HandleAccount: claimed = false, want true")
			}
			recipients, _, err := st.ListCampaignRecipients(ctx, c.ID, "whatsapp", "", 50, 0)
			if err != nil || len(recipients) != 1 {
				t.Fatalf("ListCampaignRecipients: %v, %+v", err, recipients)
			}
			if purecampaign.RecipientStatus(recipients[0].Status) != tc.wantRecipient {
				t.Errorf("recipient status = %q, want %q", recipients[0].Status, tc.wantRecipient)
			}
			attempts, err := st.ListAttemptsForRecipient(ctx, recipients[0].ID)
			if err != nil || len(attempts) != 1 {
				t.Fatalf("ListAttemptsForRecipient: %v, %+v", err, attempts)
			}
			if attempts[0].ErrorCode != string(tc.wantCode) {
				t.Errorf("error_code = %q, want %q", attempts[0].ErrorCode, tc.wantCode)
			}
			if attempts[0].Retryable == nil || *attempts[0].Retryable != tc.wantRetryable {
				t.Errorf("retryable = %v, want %v", attempts[0].Retryable, tc.wantRetryable)
			}
		})
	}
}

// TestReconcileStuckSending_AttemptRecordedUnknown proves a crash-recovered
// recipient's OPEN attempt (claimed but never finalized — the process died
// between ClaimNextRecipient's commit and the provider call returning) is
// closed out as outcome='unknown', exactly like a timeout, and produces a
// campaign_events row so the reconciliation is visible on the campaign's own
// timeline, not only in process logs.
func TestReconcileStuckSending_AttemptRecordedUnknown(t *testing.T) {
	ctx := context.Background()
	st := dbtest.New(t)
	orgID, userID, acctID := seedWAFixture(t, st)
	c := startCampaignWithFastPacing(t, st, orgID, acctID, userID, "whatsapp", "Promo", "Hi!", "77011234567")

	// Simulate a claim that committed but was never finalized (a crash
	// between ClaimNextRecipient's commit and the provider call returning):
	// call ClaimNextRecipient directly, without ever running Runner.send.
	claim, ok, err := st.ClaimNextRecipient(ctx, acctID, time.Now())
	if err != nil || !ok {
		t.Fatalf("ClaimNextRecipient: %v, ok=%v", err, ok)
	}

	n, err := st.ReconcileStuckSending(ctx)
	if err != nil {
		t.Fatalf("ReconcileStuckSending: %v", err)
	}
	if n != 1 {
		t.Fatalf("reconciled = %d, want 1", n)
	}

	failed, _, err := st.ListCampaignRecipients(ctx, c.ID, "whatsapp", "failed", 50, 0)
	if err != nil || len(failed) != 1 {
		t.Fatalf("ListCampaignRecipients: %v, %+v", err, failed)
	}
	if failed[0].NextAttemptAt != nil {
		t.Errorf("NextAttemptAt = %v, want nil — never auto-retried", failed[0].NextAttemptAt)
	}

	attempts, err := st.ListAttemptsForRecipient(ctx, claim.RecipientID)
	if err != nil || len(attempts) != 1 {
		t.Fatalf("ListAttemptsForRecipient: %v, %+v", err, attempts)
	}
	if attempts[0].Outcome != "unknown" {
		t.Errorf("attempt outcome = %q, want unknown", attempts[0].Outcome)
	}
	if attempts[0].CompletedAt == nil {
		t.Error("attempt CompletedAt = nil, want set (reconciliation closes the open attempt)")
	}

	events, _, err := st.ListCampaignEvents(ctx, c.ID, 50, 0)
	if err != nil {
		t.Fatalf("ListCampaignEvents: %v", err)
	}
	found := false
	for _, e := range events {
		if e.Event == "recipient_outcome_unknown" && e.CampaignRecipientID.UUID == claim.RecipientID {
			found = true
		}
	}
	if !found {
		t.Errorf("no recipient_outcome_unknown event found for recipient %v in %+v", claim.RecipientID, events)
	}
}
