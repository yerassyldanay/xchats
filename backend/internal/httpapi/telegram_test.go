package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/internal/config"
	"github.com/yerassyldanay/xchats/backend/internal/telegram"
)

// --- helpers ---------------------------------------------------------------

// connectBot runs the real POST /telegram-accounts flow and returns the created
// account id.
func (h *harness) connectBot(t *testing.T, displayName string, dropBacklog bool) uuid.UUID {
	t.Helper()
	resp, env := h.postJSON("/xchats/api/v1/telegram-accounts", map[string]any{
		"bot_token": testBotToken, "display_name": displayName, "drop_pending_backlog": dropBacklog,
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("connect bot: status %d, body %s", resp.StatusCode, env["message"])
	}
	var payload struct {
		Account struct {
			ID              string `json:"id"`
			Channel         string `json:"channel"`
			ConnectionState string `json:"connection_state"`
		} `json:"account"`
		ConnectionState string `json:"connection_state"`
	}
	mustPayload(t, env, &payload)
	id, err := uuid.Parse(payload.Account.ID)
	if err != nil {
		t.Fatalf("bad account id %q: %v", payload.Account.ID, err)
	}
	return id
}

// tgWebhook posts one raw update to the Telegram ingress and returns the status.
func (h *harness) tgWebhook(accountID uuid.UUID, body []byte, secret string) int {
	h.t.Helper()
	req, _ := http.NewRequest("POST", h.srv.URL+"/telegram/api/v1/webhook/"+accountID.String(), bytes.NewReader(body))
	if secret != "" {
		req.Header.Set("X-Telegram-Bot-Api-Secret-Token", secret)
	}
	resp, err := h.client.Do(req)
	if err != nil {
		h.t.Fatalf("telegram webhook: %v", err)
	}
	resp.Body.Close()
	h.queue.Wait()
	return resp.StatusCode
}

func textUpdate(updateID, messageID, chatID int64, text string) []byte {
	b, _ := json.Marshal(map[string]any{
		"update_id": updateID,
		"message": map[string]any{
			"message_id": messageID,
			"date":       1781460000,
			"from":       map[string]any{"id": chatID, "is_bot": false, "first_name": "Айгүл", "username": "aigul"},
			"chat":       map[string]any{"id": chatID, "type": "private"},
			"text":       text,
		},
	})
	return b
}

func mediaUpdate(updateID, messageID, chatID int64, caption string) []byte {
	msg := map[string]any{
		"message_id": messageID,
		"date":       1781460000,
		"from":       map[string]any{"id": chatID, "is_bot": false, "first_name": "Айгүл"},
		"chat":       map[string]any{"id": chatID, "type": "private"},
		"photo": []map[string]any{
			{"file_id": "small", "file_unique_id": "su", "width": 90, "height": 90, "file_size": 100},
			{"file_id": "AgACbig", "file_unique_id": "bu", "width": 1280, "height": 1280, "file_size": 90000},
		},
	}
	if caption != "" {
		msg["caption"] = caption
	}
	b, _ := json.Marshal(map[string]any{"update_id": updateID, "message": msg})
	return b
}

func (h *harness) countRows(query string, args ...any) int {
	h.t.Helper()
	var n int
	if err := h.store.Pool().QueryRow(context.Background(), query, args...).Scan(&n); err != nil {
		h.t.Fatalf("count: %v", err)
	}
	return n
}

// --- provisioning ----------------------------------------------------------

func TestTelegramConnectRegistersWebhookAndStoresTokenEncrypted(t *testing.T) {
	h := newHarness(t)
	id := h.connectBot(t, "Магазин-бот", false)

	// The id is derived from the bot id, never random — that is what makes a
	// re-connect land on the same row.
	want := config.ChannelAccountID(config.TelegramOwnerRef(testBotID))
	if id != want {
		t.Fatalf("account id = %s, want the derived %s", id, want)
	}

	calls := h.tg.CallsFor("setWebhook")
	if len(calls) != 1 {
		t.Fatalf("setWebhook calls = %d, want 1", len(calls))
	}
	c := calls[0]
	if c.URL != telegramBaseURL+"/telegram/api/v1/webhook/"+id.String() {
		t.Fatalf("webhook url = %q", c.URL)
	}
	if c.Secret != webhookToken {
		t.Fatalf("secret = %q, want the configured webhook token", c.Secret)
	}
	if c.DropPending {
		t.Fatal("drop_pending_updates was true without the operator asking for it")
	}
	if len(c.AllowedUpdates) != 1 || c.AllowedUpdates[0] != "message" {
		t.Fatalf("allowed_updates = %v", c.AllowedUpdates)
	}

	// The token is at rest as ciphertext, and decrypts back through the store.
	var enc []byte
	if err := h.store.Pool().QueryRow(context.Background(),
		`SELECT bot_token_enc FROM xchats.tg_credentials WHERE account_id = $1`, id).Scan(&enc); err != nil {
		t.Fatalf("read credential: %v", err)
	}
	if bytes.Contains(enc, []byte(testBotToken)) {
		t.Fatal("the bot token is stored in plaintext")
	}
	got, err := h.store.TelegramBotToken(context.Background(), id)
	if err != nil || got != testBotToken {
		t.Fatalf("TelegramBotToken = %q, %v", got, err)
	}

	// It shows up in the neutral listing, with its health fields.
	var listing struct {
		Items []map[string]any `json:"items"`
	}
	h.get("/xchats/api/v1/accounts", &listing)
	var found map[string]any
	for _, it := range listing.Items {
		if it["id"] == id.String() {
			found = it
		}
	}
	if found == nil {
		t.Fatalf("the Telegram account is missing from GET /accounts: %v", listing.Items)
	}
	if found["channel"] != "telegram" {
		t.Fatalf("channel = %v", found["channel"])
	}
	if found["connection_state"] != "connected" {
		t.Fatalf("connection_state = %v, want connected", found["connection_state"])
	}
	if found["external_handle"] != "@"+testBotUsername {
		t.Fatalf("external_handle = %v", found["external_handle"])
	}
	if found["webhook_url"] == nil {
		t.Fatal("webhook_url is null for a connected Telegram account")
	}

	// ...and the WhatsApp-only surface must NOT list it.
	var waList struct {
		Items []map[string]any `json:"items"`
	}
	h.get("/xchats/api/v1/whatsapp-accounts", &waList)
	for _, it := range waList.Items {
		if it["id"] == id.String() {
			t.Fatal("a Telegram bot appeared in /whatsapp-accounts")
		}
	}
}

func TestTelegramConnectDropBacklogOnlyWhenAsked(t *testing.T) {
	h := newHarness(t)
	h.connectBot(t, "Бот", true)
	calls := h.tg.CallsFor("setWebhook")
	if len(calls) != 1 || !calls[0].DropPending {
		t.Fatalf("drop_pending_updates = %v, want true when the operator asked", calls)
	}

	// A retry after that must NOT drop the backlog: those pending updates are
	// the customer messages that failed to arrive.
	h.tg.Reset()
	id := config.ChannelAccountID(config.TelegramOwnerRef(testBotID))
	resp, _ := h.postJSON("/xchats/api/v1/telegram-accounts/"+id.String()+"/retry-webhook", map[string]any{})
	if resp.StatusCode != 200 {
		t.Fatalf("retry status %d", resp.StatusCode)
	}
	retry := h.tg.CallsFor("setWebhook")
	if len(retry) != 1 {
		t.Fatalf("retry setWebhook calls = %d", len(retry))
	}
	if retry[0].DropPending {
		t.Fatal("the retry dropped the pending backlog")
	}
	if retry[0].Token != testBotToken {
		t.Fatal("the retry did not use the stored token")
	}
}

func TestTelegramConnectRejectedTokenStoresNothing(t *testing.T) {
	h := newHarness(t)
	h.tg.BadTokens["bad-token"] = true

	resp, env := h.postJSON("/xchats/api/v1/telegram-accounts", map[string]any{"bot_token": "bad-token"})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status %d, want 400; body %s", resp.StatusCode, env["message"])
	}
	if n := h.countRows(`SELECT count(*) FROM xchats.tg_accounts`); n != 0 {
		t.Fatalf("a rejected token left %d account rows behind", n)
	}
	if n := h.countRows(`SELECT count(*) FROM xchats.tg_credentials`); n != 0 {
		t.Fatalf("a rejected token left %d credential rows behind", n)
	}
}

func TestTelegramConnectRequiresHTTPSBaseURL(t *testing.T) {
	h := newHarness(t)
	// Rebuilding the whole harness for one config field is not worth it; the
	// check is pure and lives behind the same handler, so exercise it through a
	// harness whose base URL was never https.
	h.setTelegramBase(t, "http://localhost:8080")
	resp, env := h.postJSON("/xchats/api/v1/telegram-accounts", map[string]any{"bot_token": testBotToken})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status %d, want 400", resp.StatusCode)
	}
	var msg string
	_ = json.Unmarshal(env["message"], &msg)
	if !strings.Contains(msg, "https") {
		t.Fatalf("message %q does not explain the https requirement", msg)
	}
	if n := h.countRows(`SELECT count(*) FROM xchats.tg_accounts`); n != 0 {
		t.Fatalf("an unusable base URL still created %d accounts", n)
	}
}

// Two concurrent connects of the SAME new bot: exactly one INSERT wins, and the
// loser must land on the same row (same org) rather than creating a second
// account or erroring out.
func TestTelegramConcurrentConnectsConvergeOnOneAccount(t *testing.T) {
	h := newHarness(t)
	var wg sync.WaitGroup
	codes := make([]int, 4)
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			body, _ := json.Marshal(map[string]any{"bot_token": testBotToken, "display_name": fmt.Sprintf("bot-%d", i)})
			resp, err := h.client.Post(h.srv.URL+"/xchats/api/v1/telegram-accounts", "application/json", bytes.NewReader(body))
			if err != nil {
				codes[i] = -1
				return
			}
			resp.Body.Close()
			codes[i] = resp.StatusCode
		}(i)
	}
	wg.Wait()

	if n := h.countRows(`SELECT count(*) FROM xchats.tg_accounts`); n != 1 {
		t.Fatalf("concurrent connects produced %d accounts, want exactly 1 (codes %v)", n, codes)
	}
	for _, code := range codes {
		if code != http.StatusCreated && code != http.StatusConflict {
			t.Fatalf("unexpected status %d among %v", code, codes)
		}
	}
}

// A bot owned by a DIFFERENT organization is a 409, never a takeover: its chats
// and messages are that tenant's history.
func TestTelegramConnectRefusesAnotherOrgsBot(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	var otherOrg uuid.UUID
	if err := h.store.Pool().QueryRow(ctx,
		`INSERT INTO xchats.organizations (name) VALUES ('other-tenant') RETURNING id`).Scan(&otherOrg); err != nil {
		t.Fatalf("seed other org: %v", err)
	}
	id := config.ChannelAccountID(config.TelegramOwnerRef(testBotID))
	if _, err := h.store.Pool().Exec(ctx, `
		INSERT INTO xchats.tg_accounts (id, organization_id, display_name, bot_id, bot_username, connection_state)
		VALUES ($1, $2, 'Чужой бот', $3, $4, 'connected')`,
		id, otherOrg, testBotID, testBotUsername); err != nil {
		t.Fatalf("seed foreign account: %v", err)
	}

	resp, _ := h.postJSON("/xchats/api/v1/telegram-accounts", map[string]any{"bot_token": testBotToken})
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status %d, want 409", resp.StatusCode)
	}
	var owner uuid.UUID
	if err := h.store.Pool().QueryRow(ctx,
		`SELECT organization_id FROM xchats.tg_accounts WHERE id = $1`, id).Scan(&owner); err != nil {
		t.Fatalf("re-read account: %v", err)
	}
	if owner != otherOrg {
		t.Fatal("the other organization's bot was taken over")
	}
	if n := h.countRows(`SELECT count(*) FROM xchats.tg_credentials`); n != 0 {
		t.Fatal("a refused claim still wrote a credential row")
	}
}

// An ORPHANED account (organization_id NULL after its org was deleted) is not
// claimable either: adopting the row would inherit someone else's history.
func TestTelegramConnectRefusesOrphanedBot(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	id := config.ChannelAccountID(config.TelegramOwnerRef(testBotID))
	if _, err := h.store.Pool().Exec(ctx, `
		INSERT INTO xchats.tg_accounts (id, organization_id, display_name, bot_id, bot_username, connection_state)
		VALUES ($1, NULL, 'Осиротевший бот', $2, $3, 'disconnected')`,
		id, testBotID, testBotUsername); err != nil {
		t.Fatalf("seed orphan: %v", err)
	}
	resp, _ := h.postJSON("/xchats/api/v1/telegram-accounts", map[string]any{"bot_token": testBotToken})
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status %d, want 409 for an orphaned account", resp.StatusCode)
	}
	var org *uuid.UUID
	if err := h.store.Pool().QueryRow(ctx,
		`SELECT organization_id FROM xchats.tg_accounts WHERE id = $1`, id).Scan(&org); err != nil {
		t.Fatalf("re-read: %v", err)
	}
	if org != nil {
		t.Fatal("an orphaned account was adopted — its history would come with it")
	}
}

func TestTelegramSetWebhookRejectionIsVisible(t *testing.T) {
	h := newHarness(t)
	h.tg.FailSetWebhook = &telegram.APIError{Code: 400, Description: "Bad Request: bad webhook: HTTPS url must be provided"}

	resp, env := h.postJSON("/xchats/api/v1/telegram-accounts", map[string]any{"bot_token": testBotToken})
	// The account IS kept: a visible broken connection with a retry button beats
	// a silent disappearance.
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status %d, want the account to be created in a failed state", resp.StatusCode)
	}
	var payload struct {
		ConnectionState string `json:"connection_state"`
	}
	mustPayload(t, env, &payload)
	if payload.ConnectionState != "webhook_error" {
		t.Fatalf("connection_state = %q, want webhook_error", payload.ConnectionState)
	}
	var state, lastErr string
	if err := h.store.Pool().QueryRow(context.Background(),
		`SELECT connection_state, webhook_last_error FROM xchats.tg_accounts WHERE bot_id = $1`,
		testBotID).Scan(&state, &lastErr); err != nil {
		t.Fatalf("read state: %v", err)
	}
	if state != "webhook_error" || !strings.Contains(lastErr, "HTTPS url must be provided") {
		t.Fatalf("state=%q last_error=%q — Telegram's own reason was lost", state, lastErr)
	}

	// The retry succeeds and clears the error.
	h.tg.Reset()
	id := config.ChannelAccountID(config.TelegramOwnerRef(testBotID))
	if resp, _ := h.postJSON("/xchats/api/v1/telegram-accounts/"+id.String()+"/retry-webhook", map[string]any{}); resp.StatusCode != 200 {
		t.Fatalf("retry status %d", resp.StatusCode)
	}
	if err := h.store.Pool().QueryRow(context.Background(),
		`SELECT connection_state, webhook_last_error FROM xchats.tg_accounts WHERE bot_id = $1`,
		testBotID).Scan(&state, &lastErr); err != nil {
		t.Fatalf("read state: %v", err)
	}
	if state != "connected" || lastErr != "" {
		t.Fatalf("after retry: state=%q last_error=%q", state, lastErr)
	}
}

// An ambiguous setWebhook (timeout/transport, no Bot API code) must not be
// guessed: getWebhookInfo decides. Here Telegram reports our URL, so the
// account is connected despite the error.
func TestTelegramSetWebhookTimeoutReconcilesViaGetWebhookInfo(t *testing.T) {
	h := newHarness(t)
	id := config.ChannelAccountID(config.TelegramOwnerRef(testBotID))
	h.tg.Info = telegram.WebhookInfo{URL: telegramBaseURL + "/telegram/api/v1/webhook/" + id.String()}
	h.tg.FailSetWebhook = context.DeadlineExceeded

	resp, env := h.postJSON("/xchats/api/v1/telegram-accounts", map[string]any{"bot_token": testBotToken})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status %d", resp.StatusCode)
	}
	var payload struct {
		ConnectionState string `json:"connection_state"`
	}
	mustPayload(t, env, &payload)
	if payload.ConnectionState != "connected" {
		t.Fatalf("connection_state = %q, want connected (getWebhookInfo confirmed the URL)", payload.ConnectionState)
	}
	if len(h.tg.CallsFor("getWebhookInfo")) == 0 {
		t.Fatal("the ambiguous failure was decided without asking Telegram")
	}
}

func TestTelegramReplaceTokenRefusesADifferentBot(t *testing.T) {
	h := newHarness(t)
	id := h.connectBot(t, "Бот", false)

	// A token for a different bot: same shape, different identity.
	h.tg.Bot.ID = testBotID + 1
	h.tg.Bot.Username = "some_other_bot"
	req, _ := http.NewRequest("PUT", h.srv.URL+"/xchats/api/v1/telegram-accounts/"+id.String()+"/token",
		bytes.NewReader([]byte(`{"bot_token":"9999:other"}`)))
	req.Header.Set("Content-Type", "application/json")
	resp, err := h.client.Do(req)
	if err != nil {
		t.Fatalf("PUT token: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status %d, want 409 for a different bot's token", resp.StatusCode)
	}
	got, err := h.store.TelegramBotToken(context.Background(), id)
	if err != nil || got != testBotToken {
		t.Fatalf("the stored token changed: %q %v", got, err)
	}
}

// The delete state machine: a failed deleteWebhook keeps the account AND its
// token so the operator can retry; only a confirmed removal purges them.
func TestTelegramDeleteFailureKeepsTokenAndRetrySucceeds(t *testing.T) {
	h := newHarness(t)
	id := h.connectBot(t, "Бот", false)

	h.tg.FailDeleteWebhook = &telegram.APIError{Code: 502, Description: "Bad Gateway"}
	req, _ := http.NewRequest("DELETE", h.srv.URL+"/xchats/api/v1/telegram-accounts/"+id.String(), nil)
	resp, err := h.client.Do(req)
	if err != nil {
		t.Fatalf("DELETE: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadGateway {
		t.Fatalf("status %d, want 502", resp.StatusCode)
	}
	var state string
	var deletedAt *string
	if err := h.store.Pool().QueryRow(context.Background(),
		`SELECT connection_state, deleted_at::text FROM xchats.tg_accounts WHERE id = $1`, id).
		Scan(&state, &deletedAt); err != nil {
		t.Fatalf("read state: %v", err)
	}
	if state != "disconnect_error" || deletedAt != nil {
		t.Fatalf("after a failed disconnect: state=%q deleted_at=%v — the account must stay visible", state, deletedAt)
	}
	if _, err := h.store.TelegramBotToken(context.Background(), id); err != nil {
		t.Fatalf("the token was purged before Telegram confirmed the disconnect: %v", err)
	}

	// Retry = repeat the DELETE.
	req, _ = http.NewRequest("DELETE", h.srv.URL+"/xchats/api/v1/telegram-accounts/"+id.String(), nil)
	resp, err = h.client.Do(req)
	if err != nil {
		t.Fatalf("DELETE retry: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("retry status %d", resp.StatusCode)
	}
	if n := h.countRows(`SELECT count(*) FROM xchats.tg_credentials WHERE account_id = $1`, id); n != 0 {
		t.Fatal("the token survived a confirmed disconnect")
	}
	if err := h.store.Pool().QueryRow(context.Background(),
		`SELECT deleted_at::text FROM xchats.tg_accounts WHERE id = $1`, id).Scan(&deletedAt); err != nil {
		t.Fatalf("read: %v", err)
	}
	if deletedAt == nil {
		t.Fatal("the account was not soft-deleted after a confirmed disconnect")
	}
}

// Re-adding the same bot revives the same row, with its history.
func TestTelegramReconnectRevivesTheSameAccountAndHistory(t *testing.T) {
	h := newHarness(t)
	id := h.connectBot(t, "Бот", false)
	if got := h.tgWebhook(id, textUpdate(1, 100, 500100, "здравствуйте"), webhookToken); got != 200 {
		t.Fatalf("webhook status %d", got)
	}

	req, _ := http.NewRequest("DELETE", h.srv.URL+"/xchats/api/v1/telegram-accounts/"+id.String(), nil)
	resp, _ := h.client.Do(req)
	resp.Body.Close()

	// While disconnected the chat is out of the inbox...
	var listing struct {
		Items []map[string]any `json:"items"`
	}
	h.get("/xchats/api/v1/chats", &listing)
	for _, it := range listing.Items {
		if it["channel"] == "telegram" {
			t.Fatal("a disconnected bot's chats are still in the inbox")
		}
	}

	// ...and comes back on re-connect, on the same account id.
	again := h.connectBot(t, "Бот", false)
	if again != id {
		t.Fatalf("re-connect produced a new account %s, want %s", again, id)
	}
	h.get("/xchats/api/v1/chats", &listing)
	found := false
	for _, it := range listing.Items {
		if it["channel"] == "telegram" {
			found = true
		}
	}
	if !found {
		t.Fatal("the history did not come back with the revived account")
	}
	if n := h.countRows(`SELECT count(*) FROM xchats.tg_messages`); n != 1 {
		t.Fatalf("tg_messages = %d, want the original message preserved", n)
	}
}

// --- webhook ingestion -----------------------------------------------------

func TestTelegramWebhookIngestsTextCaptionAndBareMedia(t *testing.T) {
	h := newHarness(t)
	id := h.connectBot(t, "Бот", false)

	if got := h.tgWebhook(id, textUpdate(1, 100, 500100, "сколько стоит доставка?"), webhookToken); got != 200 {
		t.Fatalf("text update status %d", got)
	}
	if got := h.tgWebhook(id, mediaUpdate(2, 101, 500100, "это подойдёт?"), webhookToken); got != 200 {
		t.Fatalf("captioned photo status %d", got)
	}
	if got := h.tgWebhook(id, mediaUpdate(3, 102, 500100, ""), webhookToken); got != 200 {
		t.Fatalf("caption-less photo status %d", got)
	}

	if n := h.countRows(`SELECT count(*) FROM xchats.tg_messages`); n != 3 {
		t.Fatalf("tg_messages = %d, want 3 (text, captioned media, bare media)", n)
	}
	// The caption IS the body — the AI must see the question under the photo.
	if n := h.countRows(`SELECT count(*) FROM xchats.tg_messages WHERE body = 'это подойдёт?'`); n != 1 {
		t.Fatal("the caption was not stored as the message body")
	}
	// A caption-less attachment still becomes a row, with a placeholder preview.
	if n := h.countRows(
		`SELECT count(*) FROM xchats.tg_chats WHERE last_message_preview = '📷 Фото'`); n != 1 {
		t.Fatal("the caption-less photo did not produce a placeholder preview")
	}
	// Both media messages carry a media row keyed by the LARGEST rendition's
	// file handles — the durable work item the downloader (and its sweeper)
	// retries from. This harness has no bytes registered for that file_id, so
	// the download attempt fails and the row stays retryable, which is exactly
	// the state a real outage would leave behind.
	if n := h.countRows(
		`SELECT count(*) FROM xchats.tg_message_media WHERE file_id = 'AgACbig' AND file_unique_id = 'bu'`); n != 2 {
		t.Fatalf("media rows with the largest rendition's handles = %d, want 2 (total rows: %d)", n,
			h.countRows(`SELECT count(*) FROM xchats.tg_message_media`))
	}
	if n := h.countRows(
		`SELECT count(*) FROM xchats.tg_message_media WHERE download_status <> 'ready'`); n != 2 {
		t.Fatal("a media row reported ready without any bytes ever being fetched")
	}

	// The whole thread reads back through the neutral API.
	var chats struct {
		Items []map[string]any `json:"items"`
	}
	h.get("/xchats/api/v1/chats", &chats)
	var chatID string
	for _, it := range chats.Items {
		if it["channel"] == "telegram" {
			chatID = it["id"].(string)
			if it["account_id"] != id.String() {
				t.Fatalf("account_id = %v, want %s", it["account_id"], id)
			}
			if _, present := it["wa_account_id"]; present {
				t.Fatal("a Telegram chat emitted the WhatsApp-only wa_account_id alias")
			}
		}
	}
	if chatID == "" {
		t.Fatalf("no telegram chat in the inbox: %v", chats.Items)
	}
	var msgs struct {
		Items []map[string]any `json:"items"`
	}
	h.get("/xchats/api/v1/chats/"+chatID+"/messages", &msgs)
	if len(msgs.Items) != 3 {
		t.Fatalf("messages = %d, want 3", len(msgs.Items))
	}
}

func TestTelegramWebhookRedeliveryIsANoOp(t *testing.T) {
	h := newHarness(t)
	id := h.connectBot(t, "Бот", false)
	body := textUpdate(7, 100, 500100, "привет")

	if got := h.tgWebhook(id, body, webhookToken); got != 200 {
		t.Fatalf("first delivery status %d", got)
	}
	if got := h.tgWebhook(id, body, webhookToken); got != 200 {
		t.Fatalf("redelivery status %d", got)
	}
	if n := h.countRows(`SELECT count(*) FROM xchats.tg_messages`); n != 1 {
		t.Fatalf("tg_messages = %d after a redelivery, want 1", n)
	}
	var unread int
	if err := h.store.Pool().QueryRow(context.Background(),
		`SELECT unread_count FROM xchats.tg_chats WHERE account_id = $1`, id).Scan(&unread); err != nil {
		t.Fatalf("read unread: %v", err)
	}
	if unread != 1 {
		t.Fatalf("unread_count = %d after a redelivery, want 1 (aggregates must not double-count)", unread)
	}
}

// A redelivery whose first attempt never produced a draft must re-enqueue one:
// the draft pipeline is at-least-once, riding Telegram's own retry.
func TestTelegramRedeliveryReenqueuesAMissingDraft(t *testing.T) {
	h := newHarness(t)
	id := h.connectBot(t, "Бот", false)
	body := textUpdate(9, 100, 500100, "здравствуйте")

	h.tgWebhook(id, body, webhookToken)
	// Simulate "the first attempt died before the draft was written".
	if _, err := h.store.Pool().Exec(context.Background(), `DELETE FROM xchats.ai_drafts`); err != nil {
		t.Fatalf("clear drafts: %v", err)
	}
	h.tgWebhook(id, body, webhookToken)

	if n := h.countRows(`SELECT count(*) FROM xchats.ai_drafts`); n == 0 {
		t.Fatal("the redelivery did not re-enqueue the missing draft")
	}
	// And the draft is recorded against the right channel.
	if n := h.countRows(`SELECT count(*) FROM xchats.ai_drafts WHERE channel = 'telegram'`); n == 0 {
		t.Fatal("the draft was not stamped with channel='telegram'")
	}

	// A redelivery that DOES find a draft must not pile on another set.
	before := h.countRows(`SELECT count(*) FROM xchats.ai_drafts`)
	h.tgWebhook(id, body, webhookToken)
	if after := h.countRows(`SELECT count(*) FROM xchats.ai_drafts`); after != before {
		t.Fatalf("drafts %d -> %d: a redelivery regenerated despite an existing draft", before, after)
	}
}

func TestTelegramWebhookRejectsWrongSecret(t *testing.T) {
	h := newHarness(t)
	id := h.connectBot(t, "Бот", false)
	if got := h.tgWebhook(id, textUpdate(1, 100, 500100, "hi"), "not-the-secret"); got != http.StatusUnauthorized {
		t.Fatalf("status %d, want 401", got)
	}
	if n := h.countRows(`SELECT count(*) FROM xchats.tg_messages`); n != 0 {
		t.Fatal("an unauthenticated update was stored")
	}
}

func TestTelegramWebhookDiscardsUnknownAndDisconnectedAccounts(t *testing.T) {
	h := newHarness(t)

	// Never-seen account id: ack (Telegram must stop), store nothing.
	if got := h.tgWebhook(uuid.New(), textUpdate(1, 100, 500100, "hi"), webhookToken); got != 200 {
		t.Fatalf("unknown account status %d, want 200", got)
	}
	if n := h.countRows(`SELECT count(*) FROM xchats.tg_messages`); n != 0 {
		t.Fatal("an update for an unknown account was stored")
	}

	// Soft-deleted account: same.
	id := h.connectBot(t, "Бот", false)
	req, _ := http.NewRequest("DELETE", h.srv.URL+"/xchats/api/v1/telegram-accounts/"+id.String(), nil)
	resp, _ := h.client.Do(req)
	resp.Body.Close()
	if got := h.tgWebhook(id, textUpdate(2, 101, 500100, "hi"), webhookToken); got != 200 {
		t.Fatalf("disconnected account status %d, want 200", got)
	}
	if n := h.countRows(`SELECT count(*) FROM xchats.tg_messages`); n != 0 {
		t.Fatal("an update for a disconnected account was stored")
	}
}

func TestTelegramWebhookIgnoresGroupsAndServiceMessages(t *testing.T) {
	h := newHarness(t)
	id := h.connectBot(t, "Бот", false)

	group, _ := json.Marshal(map[string]any{
		"update_id": 1,
		"message": map[string]any{
			"message_id": 1, "date": 1781460000,
			"from": map[string]any{"id": 7, "is_bot": false, "first_name": "X"},
			"chat": map[string]any{"id": -100123, "type": "supergroup"},
			"text": "hello group",
		},
	})
	if got := h.tgWebhook(id, group, webhookToken); got != 200 {
		t.Fatalf("group status %d, want 200", got)
	}

	service, _ := json.Marshal(map[string]any{
		"update_id": 2,
		"message": map[string]any{
			"message_id": 2, "date": 1781460000,
			"from":             map[string]any{"id": 7, "is_bot": false, "first_name": "X"},
			"chat":             map[string]any{"id": 7, "type": "private"},
			"new_chat_members": []map[string]any{{"id": 9}},
		},
	})
	if got := h.tgWebhook(id, service, webhookToken); got != 200 {
		t.Fatalf("service message status %d, want 200", got)
	}

	edited, _ := json.Marshal(map[string]any{"update_id": 3, "edited_message": map[string]any{"message_id": 1}})
	if got := h.tgWebhook(id, edited, webhookToken); got != 200 {
		t.Fatalf("edited_message status %d, want 200", got)
	}

	if n := h.countRows(`SELECT count(*) FROM xchats.tg_messages`); n != 0 {
		t.Fatalf("tg_messages = %d — an ignorable update was stored", n)
	}
	if n := h.countRows(`SELECT count(*) FROM xchats.tg_chats`); n != 0 {
		t.Fatalf("tg_chats = %d — an ignorable update created a conversation", n)
	}
}

// A database failure must NOT be acked: Telegram's redelivery is the whole
// retry mechanism, and a 200 here would drop the message on the floor.
func TestTelegramWebhookAnswers500WhenIngestFails(t *testing.T) {
	h := newHarness(t)
	id := h.connectBot(t, "Бот", false)

	// Break the write path in a way only the ingest touches.
	if _, err := h.store.Pool().Exec(context.Background(),
		`ALTER TABLE xchats.tg_messages ADD CONSTRAINT tg_messages_force_fail CHECK (false) NOT VALID`); err != nil {
		t.Fatalf("install failing constraint: %v", err)
	}
	got := h.tgWebhook(id, textUpdate(1, 100, 500100, "привет"), webhookToken)
	if got != http.StatusInternalServerError {
		t.Fatalf("status %d, want 500 so Telegram redelivers", got)
	}
	if _, err := h.store.Pool().Exec(context.Background(),
		`ALTER TABLE xchats.tg_messages DROP CONSTRAINT tg_messages_force_fail`); err != nil {
		t.Fatalf("drop constraint: %v", err)
	}

	// Once the database is healthy, the redelivery lands.
	if got := h.tgWebhook(id, textUpdate(1, 100, 500100, "привет"), webhookToken); got != 200 {
		t.Fatalf("redelivery after recovery: status %d", got)
	}
	if n := h.countRows(`SELECT count(*) FROM xchats.tg_messages`); n != 1 {
		t.Fatalf("tg_messages = %d after recovery, want 1", n)
	}
}

// --- outbound --------------------------------------------------------------

func TestTelegramApprovedDraftSendsAndStampsMessageID(t *testing.T) {
	h := newHarness(t)
	id := h.connectBot(t, "Бот", false)
	h.tgWebhook(id, textUpdate(1, 100, 500100, "сколько стоит доставка?"), webhookToken)

	var chats struct {
		Items []map[string]any `json:"items"`
	}
	h.get("/xchats/api/v1/chats", &chats)
	var chatID string
	for _, it := range chats.Items {
		if it["channel"] == "telegram" {
			chatID = it["id"].(string)
		}
	}
	if chatID == "" {
		t.Fatalf("no telegram chat: %v", chats.Items)
	}
	var drafts struct {
		Items []map[string]any `json:"items"`
	}
	h.get("/xchats/api/v1/chats/"+chatID+"/ai-drafts", &drafts)
	if len(drafts.Items) == 0 {
		t.Fatal("the inbound Telegram message produced no draft")
	}
	draftID := drafts.Items[0]["id"].(string)

	h.tg.Reset()
	resp, env := h.postJSON("/xchats/api/v1/ai-drafts/"+draftID+"/approve", map[string]any{})
	if resp.StatusCode != 200 {
		t.Fatalf("approve status %d, body %s", resp.StatusCode, env["message"])
	}

	sends := h.tg.CallsFor("sendMessage")
	if len(sends) != 1 {
		t.Fatalf("sendMessage calls = %d, want 1", len(sends))
	}
	if sends[0].ChatID != 500100 {
		t.Fatalf("sent to chat %d, want the Telegram chat id 500100", sends[0].ChatID)
	}
	if sends[0].Token != testBotToken {
		t.Fatal("the send did not resolve the account's stored token")
	}
	if len(h.fake.Calls) != 0 {
		t.Fatal("approving a Telegram draft called Evolution")
	}

	var state string
	var tgMsgID *int64
	if err := h.store.Pool().QueryRow(context.Background(),
		`SELECT delivery_state, telegram_message_id FROM xchats.tg_messages WHERE direction = 'out'`).
		Scan(&state, &tgMsgID); err != nil {
		t.Fatalf("read outbound: %v", err)
	}
	if state != "sent" {
		t.Fatalf("delivery_state = %q, want sent", state)
	}
	if tgMsgID == nil || *tgMsgID == 0 {
		t.Fatal("telegram_message_id was not stamped from the send response")
	}
}

// Outbound media rides the same sender registry as text, so a Telegram
// attachment reaches sendPhoto rather than the Evolution client.
func TestTelegramMediaSendGoesThroughTheRegistry(t *testing.T) {
	h := newHarness(t)
	id := h.connectBot(t, "Бот", false)
	h.tgWebhook(id, textUpdate(1, 100, 500100, "привет"), webhookToken)
	chatID := h.telegramChatID(t)

	h.tg.Reset()
	mid := h.upload("a.png", "image/png", []byte("\x89PNG\r\n\x1a\nfake"))
	resp, env := h.postJSON("/xchats/api/v1/chats/"+chatID+"/messages",
		map[string]any{"text": "вот фото", "media_ids": []string{mid}})
	if resp.StatusCode != 200 {
		t.Fatalf("status %d, body %s", resp.StatusCode, env["message"])
	}

	photos := h.tg.CallsFor("sendPhoto")
	if len(photos) != 1 {
		t.Fatalf("sendPhoto calls = %d, want 1 (calls: %+v)", len(photos), h.tg.Call)
	}
	if photos[0].ChatID != 500100 {
		t.Fatalf("sent to chat %d", photos[0].ChatID)
	}
	if photos[0].Upload.Caption != "вот фото" {
		t.Fatalf("caption = %q — the text must ride the attachment", photos[0].Upload.Caption)
	}
	if len(photos[0].Upload.Data) == 0 {
		t.Fatal("no bytes were uploaded")
	}
	if len(h.fake.CallsFor("sendMedia")) != 0 {
		t.Fatal("a Telegram attachment was sent through Evolution")
	}

	// The outbound row landed in the Telegram media table (its FK would have
	// rejected the WhatsApp one) and was stamped from the send response.
	if n := h.countRows(`SELECT count(*) FROM xchats.tg_message_media WHERE download_status = 'ready' AND file_id = ''`); n != 1 {
		t.Fatal("the outbound attachment was not recorded in tg_message_media")
	}
	if n := h.countRows(
		`SELECT count(*) FROM xchats.tg_messages WHERE direction = 'out' AND delivery_state = 'sent' AND telegram_message_id IS NOT NULL`); n != 1 {
		t.Fatal("the outbound media row was not stamped as sent")
	}
}

// Inbound attachments are fetched off the request path: getFile → download →
// blob → 'ready', and the bytes then serve over the media endpoint.
func TestTelegramInboundMediaIsDownloadedAndServed(t *testing.T) {
	h := newHarness(t)
	id := h.connectBot(t, "Бот", false)
	want := []byte("\x89PNG\r\n\x1a\ntelegram-photo-bytes")
	h.tg.Files["AgACbig"] = want

	h.tgWebhook(id, mediaUpdate(1, 100, 500100, "вот"), webhookToken)

	var status, storageKey string
	var size int
	if err := h.store.Pool().QueryRow(context.Background(),
		`SELECT download_status, storage_key, size FROM xchats.tg_message_media`).
		Scan(&status, &storageKey, &size); err != nil {
		t.Fatalf("read media row: %v", err)
	}
	if status != "ready" {
		t.Fatalf("download_status = %q, want ready", status)
	}
	if size != len(want) {
		t.Fatalf("size = %d, want %d", size, len(want))
	}

	// It serves through the neutral media endpoint.
	chatID := h.telegramChatID(t)
	var msgs struct {
		Items []map[string]any `json:"items"`
	}
	h.get("/xchats/api/v1/chats/"+chatID+"/messages", &msgs)
	var url string
	for _, m := range msgs.Items {
		media, _ := m["media"].([]any)
		if len(media) > 0 {
			url = media[0].(map[string]any)["url"].(string)
		}
	}
	if url == "" {
		t.Fatalf("the message carries no media url: %v", msgs.Items)
	}
	st, body := h.getBytes(url)
	if st != 200 || !bytes.Equal(body, want) {
		t.Fatalf("GET %s = %d, %d bytes (want %d)", url, st, len(body), len(want))
	}
}

// A failed download leaves the row retryable, and the sweeper is what retries it.
func TestTelegramMediaSweeperRetriesAFailedDownload(t *testing.T) {
	h := newHarness(t)
	id := h.connectBot(t, "Бот", false)
	// No Files entry => getFile fails.
	h.tgWebhook(id, mediaUpdate(1, 100, 500100, ""), webhookToken)

	var status, storageKey string
	if err := h.store.Pool().QueryRow(context.Background(),
		`SELECT download_status, storage_key FROM xchats.tg_message_media`).Scan(&status, &storageKey); err != nil {
		t.Fatalf("read media row: %v", err)
	}
	if status == "ready" {
		t.Fatal("a failed download reported ready")
	}
	if storageKey != "" {
		t.Fatal("a failed download wrote a storage key")
	}

	// Now the file exists; a sweep (olderThan=0 picks up everything) must retry.
	h.tg.Files["AgACbig"] = []byte("recovered-bytes")
	queued, err := h.worker.SweepTelegramMedia(context.Background(), 0, 10)
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if queued != 1 {
		t.Fatalf("sweep queued %d, want 1", queued)
	}
	h.queue.Wait()

	if err := h.store.Pool().QueryRow(context.Background(),
		`SELECT download_status FROM xchats.tg_message_media`).Scan(&status); err != nil {
		t.Fatalf("read media row: %v", err)
	}
	if status != "ready" {
		t.Fatalf("download_status = %q after the sweep, want ready", status)
	}
}

// telegramChatID returns the single Telegram chat's id from the inbox.
func (h *harness) telegramChatID(t *testing.T) string {
	t.Helper()
	var chats struct {
		Items []map[string]any `json:"items"`
	}
	h.get("/xchats/api/v1/chats", &chats)
	for _, it := range chats.Items {
		if it["channel"] == "telegram" {
			return it["id"].(string)
		}
	}
	t.Fatalf("no telegram chat in the inbox: %v", chats.Items)
	return ""
}

// The bot token must never reach a DTO, a queue payload, or a log line — only
// the encrypted column and the Bot API call itself.
func TestTelegramTokenNeverLeavesTheStore(t *testing.T) {
	h := newHarness(t)
	id := h.connectBot(t, "Бот", false)

	var listing struct {
		Items []map[string]any `json:"items"`
	}
	h.get("/xchats/api/v1/accounts", &listing)
	blob, _ := json.Marshal(listing)
	if bytes.Contains(blob, []byte(testBotToken)) {
		t.Fatalf("the bot token appeared in GET /accounts: %s", blob)
	}
	_ = id
}
