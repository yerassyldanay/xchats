package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/yerassyldanay/xchats/backend/internal/config"
	"github.com/yerassyldanay/xchats/backend/internal/dbtest"
	"github.com/yerassyldanay/xchats/backend/internal/secretbox"
	"github.com/yerassyldanay/xchats/backend/internal/store"
)

func newPollTestAccount(t *testing.T, st *store.Store, botID int64) store.TelegramAccount {
	t.Helper()
	ctx := context.Background()
	box, err := secretbox.FromEnvValue(viewsEncKey)
	if err != nil {
		t.Fatalf("secretbox: %v", err)
	}
	st.UseCredentialsBox(box)
	org, err := st.SeedOrganization(ctx, "tg-poll-test-org")
	if err != nil {
		t.Fatalf("seed org: %v", err)
	}
	acct, err := st.ClaimTelegramAccount(ctx, store.TelegramClaim{
		ID:             config.ChannelAccountID(config.TelegramOwnerRef(botID)),
		OrganizationID: org.ID,
		BotID:          botID,
		BotUsername:    "poll_test_bot",
		BotToken:       "poll-test-token",
	})
	if err != nil {
		t.Fatalf("claim telegram account: %v", err)
	}
	return acct
}

func TestTelegramPollOffsetRoundTrip(t *testing.T) {
	st := dbtest.New(t)
	acct := newPollTestAccount(t, st, 5001)
	ctx := context.Background()

	if _, ok, err := st.TelegramPollOffset(ctx, acct.ID); err != nil || ok {
		t.Fatalf("offset for a never-polled bot: ok=%v err=%v, want ok=false err=nil", ok, err)
	}

	if err := st.AdvanceTelegramPollOffset(ctx, acct.ID, 100); err != nil {
		t.Fatalf("AdvanceTelegramPollOffset: %v", err)
	}
	last, ok, err := st.TelegramPollOffset(ctx, acct.ID)
	if err != nil || !ok || last != 100 {
		t.Fatalf("offset after advance = %d, ok=%v, err=%v; want 100, true, nil", last, ok, err)
	}
}

func TestAdvanceTelegramPollOffsetIsMonotonic(t *testing.T) {
	st := dbtest.New(t)
	acct := newPollTestAccount(t, st, 5002)
	ctx := context.Background()

	if err := st.AdvanceTelegramPollOffset(ctx, acct.ID, 50); err != nil {
		t.Fatalf("advance to 50: %v", err)
	}
	// A lower/equal update_id must be a silent no-op — never move the offset
	// backwards, which would replay already-settled updates.
	if err := st.AdvanceTelegramPollOffset(ctx, acct.ID, 10); err != nil {
		t.Fatalf("advance to 10 (stale): %v", err)
	}
	if last, _, _ := st.TelegramPollOffset(ctx, acct.ID); last != 50 {
		t.Fatalf("offset after a stale/backwards advance = %d, want unchanged 50", last)
	}
	if err := st.AdvanceTelegramPollOffset(ctx, acct.ID, 50); err != nil {
		t.Fatalf("advance to 50 (equal): %v", err)
	}
	if last, _, _ := st.TelegramPollOffset(ctx, acct.ID); last != 50 {
		t.Fatalf("offset after a repeated equal advance = %d, want unchanged 50", last)
	}
	// A genuinely higher update_id must apply.
	if err := st.AdvanceTelegramPollOffset(ctx, acct.ID, 75); err != nil {
		t.Fatalf("advance to 75: %v", err)
	}
	if last, _, _ := st.TelegramPollOffset(ctx, acct.ID); last != 75 {
		t.Fatalf("offset after a forward advance = %d, want 75", last)
	}
}

func TestListTelegramPollBotsFiltering(t *testing.T) {
	// dbtest.Open (not New): the tokenless-credentials fixture below needs
	// the raw *dbx.DB handle to delete a tg_credentials row directly — there
	// is no public Store method for that (credentials are always written
	// alongside a claim/replace, never removed except via disconnect, which
	// also soft-deletes the account).
	st, db := dbtest.Open(t)
	ctx := context.Background()

	eligible := newPollTestAccount(t, st, 6001)
	webhookErrBot := newPollTestAccount(t, st, 6002)
	tokenErrBot := newPollTestAccount(t, st, 6003)
	disconnectPendingBot := newPollTestAccount(t, st, 6004)
	deletedBot := newPollTestAccount(t, st, 6005)

	// webhook_error must still be eligible: the poller itself can write this
	// state after transient failures, and the bot must get a fresh runner on
	// the next boot rather than being treated as permanently dead.
	if err := st.SetTelegramWebhookState(ctx, webhookErrBot.ID, store.TelegramWebhookState{State: "webhook_error"}); err != nil {
		t.Fatalf("set webhook_error: %v", err)
	}
	if err := st.SetTelegramWebhookState(ctx, tokenErrBot.ID, store.TelegramWebhookState{State: "token_error"}); err != nil {
		t.Fatalf("set token_error: %v", err)
	}
	if err := st.SetTelegramWebhookState(ctx, disconnectPendingBot.ID, store.TelegramWebhookState{State: "disconnect_pending"}); err != nil {
		t.Fatalf("set disconnect_pending: %v", err)
	}
	if err := st.ConfirmTelegramDisconnect(ctx, deletedBot.ID); err != nil {
		t.Fatalf("confirm disconnect (soft delete + drop credentials): %v", err)
	}

	// A claimed-but-never-had-a-token scenario cannot happen through the
	// public API (ClaimTelegramAccount always writes credentials in the same
	// transaction), so "tokenless" coverage lives at the SQL level: deleting
	// just the credentials row directly proves the INNER JOIN skips it.
	tokenless := newPollTestAccount(t, st, 6006)
	if _, err := db.Exec(ctx, `DELETE FROM tg_credentials WHERE account_id = $1`, tokenless.ID); err != nil {
		t.Fatalf("drop tokenless credentials: %v", err)
	}

	bots, err := st.ListTelegramPollBots(ctx)
	if err != nil {
		t.Fatalf("ListTelegramPollBots: %v", err)
	}
	want := map[string]bool{eligible.ID.String(): true, webhookErrBot.ID.String(): true}
	notWant := []store.TelegramAccount{tokenErrBot, disconnectPendingBot, deletedBot, tokenless}

	ids := map[string]bool{}
	for _, b := range bots {
		ids[b.Account.ID.String()] = true
	}
	for id := range want {
		if !ids[id] {
			t.Errorf("expected account %s to be listed, was not; got %d bots", id, len(bots))
		}
	}
	for _, nw := range notWant {
		if ids[nw.ID.String()] {
			t.Errorf("account %s (state=%s) should have been excluded but was listed", nw.ID, nw.ConnectionState)
		}
	}
	if len(bots) != 2 {
		t.Fatalf("len(bots) = %d, want exactly 2 (eligible + webhook_error)", len(bots))
	}
}

func TestListTelegramPollBotsTokenVersionChangesOnReplace(t *testing.T) {
	st := dbtest.New(t)
	ctx := context.Background()
	acct := newPollTestAccount(t, st, 7001)

	before, err := st.ListTelegramPollBots(ctx)
	if err != nil || len(before) != 1 {
		t.Fatalf("ListTelegramPollBots (before): %v, %d bots", err, len(before))
	}
	v1 := before[0].TokenVersion

	// updated_at has millisecond resolution (strftime('%f')); a real token
	// replace is always separated from the original claim by far more than
	// this in practice (a human or a network round-trip), but a tight test
	// loop can land both writes in the same millisecond without it.
	time.Sleep(5 * time.Millisecond)
	if err := st.ReplaceTelegramToken(ctx, acct.ID, acct.BotID, acct.BotUsername, "a-new-rotated-token"); err != nil {
		t.Fatalf("ReplaceTelegramToken: %v", err)
	}

	after, err := st.ListTelegramPollBots(ctx)
	if err != nil || len(after) != 1 {
		t.Fatalf("ListTelegramPollBots (after): %v, %d bots", err, len(after))
	}
	v2 := after[0].TokenVersion
	if v1 == v2 {
		t.Fatalf("TokenVersion did not change after ReplaceTelegramToken (both %q)", v1)
	}
}
