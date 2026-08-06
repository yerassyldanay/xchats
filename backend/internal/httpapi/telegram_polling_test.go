package httpapi_test

import (
	"bytes"
	"context"
	"net/http"
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/internal/store"
)

// fakeTGPoller records every Upsert/Remove call the telegram accounts
// lifecycle makes in polling mode — the httpapi.Deps.TGPoller double for
// these tests, standing in for internal/tgpoller.Manager (its own package
// has the real thing's own, much deeper test suite; these only prove
// httpapi calls it correctly).
type fakeTGPoller struct {
	mu      sync.Mutex
	upserts []store.TelegramPollBot
	removes []uuid.UUID
}

func (f *fakeTGPoller) Upsert(spec store.TelegramPollBot) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.upserts = append(f.upserts, spec)
}

func (f *fakeTGPoller) Remove(id uuid.UUID) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.removes = append(f.removes, id)
}

func (f *fakeTGPoller) upsertsFor(id uuid.UUID) []store.TelegramPollBot {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []store.TelegramPollBot
	for _, u := range f.upserts {
		if u.Account.ID == id {
			out = append(out, u)
		}
	}
	return out
}

func (f *fakeTGPoller) removedCount(id uuid.UUID) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, r := range f.removes {
		if r == id {
			n++
		}
	}
	return n
}

// usePolling forces the harness's Telegram mode resolution to "polling" —
// the explicit Telegram.Mode override wins over TelegramWebhookPublicBaseURL
// (which newHarnessWithLLM already sets non-empty for the webhook-mode
// suite), mirroring how setTelegramBase mutates the same shared *cfg mid-test.
func usePolling(h *harness) {
	h.cfg.Telegram.Mode = "polling"
}

func TestTelegramPollingCreateStartsPollerViaUpsertNeverSetWebhook(t *testing.T) {
	h := newHarness(t)
	usePolling(h)

	id := h.connectBot(t, "Бот", false)

	if calls := h.tg.CallsFor("setWebhook"); len(calls) != 0 {
		t.Fatalf("setWebhook called %d times in polling mode, want 0", len(calls))
	}
	upserts := h.tgPoller.upsertsFor(id)
	if len(upserts) != 1 {
		t.Fatalf("Upsert called %d times for the new account, want exactly 1: %+v", len(upserts), upserts)
	}
	if upserts[0].Account.BotUsername != testBotUsername {
		t.Fatalf("Upsert spec = %+v, want bot username %q", upserts[0], testBotUsername)
	}
}

func TestTelegramPollingTokenReplaceReUpserts(t *testing.T) {
	h := newHarness(t)
	usePolling(h)
	id := h.connectBot(t, "Бот", false)
	if n := len(h.tgPoller.upsertsFor(id)); n != 1 {
		t.Fatalf("upserts after connect = %d, want 1", n)
	}

	req, _ := http.NewRequest("PUT", h.srv.URL+"/xchats/api/v1/telegram-accounts/"+id.String()+"/token",
		bytes.NewReader([]byte(`{"bot_token":"`+testBotToken+`"}`)))
	req.Header.Set("Content-Type", "application/json")
	resp, err := h.client.Do(req)
	if err != nil {
		t.Fatalf("PUT token: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d, want 200", resp.StatusCode)
	}
	if n := len(h.tgPoller.upsertsFor(id)); n != 2 {
		t.Fatalf("upserts after token replace = %d, want 2 (connect + replace)", n)
	}
	if calls := h.tg.CallsFor("setWebhook"); len(calls) != 0 {
		t.Fatalf("setWebhook called %d times in polling mode, want 0", len(calls))
	}
}

func TestTelegramPollingDeleteRemovesAndSkipsWebhookTeardown(t *testing.T) {
	h := newHarness(t)
	usePolling(h)
	id := h.connectBot(t, "Бот", false)

	req, _ := http.NewRequest("DELETE", h.srv.URL+"/xchats/api/v1/telegram-accounts/"+id.String(), nil)
	resp, err := h.client.Do(req)
	if err != nil {
		t.Fatalf("DELETE: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d, want 200", resp.StatusCode)
	}
	if n := h.tgPoller.removedCount(id); n != 1 {
		t.Fatalf("Remove called %d times, want 1", n)
	}
	// Polling mode never registers a webhook, so deleting an account must
	// never attempt to tear one down.
	if calls := h.tg.CallsFor("deleteWebhook"); len(calls) != 0 {
		t.Fatalf("deleteWebhook called %d times in polling mode, want 0", len(calls))
	}
	if _, err := h.store.TgAccountByID(context.Background(), id); err == nil {
		t.Fatal("account still visible via TgAccountByID after delete")
	}
}

func TestTelegramPollingCheckValidatesTokenAndReportsState(t *testing.T) {
	h := newHarness(t)
	usePolling(h)
	id := h.connectBot(t, "Бот", false)
	h.tgPoller.mu.Lock()
	h.tgPoller.upserts = nil // isolate the check's own (re-)Upsert from connect's
	h.tgPoller.mu.Unlock()

	resp, env := h.postJSON("/xchats/api/v1/telegram-accounts/"+id.String()+"/check", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("check: status %d, body %s", resp.StatusCode, env["message"])
	}
	// A healthy check must not call any webhook-only endpoint.
	if calls := h.tg.CallsFor("getWebhookInfo"); len(calls) != 0 {
		t.Fatalf("getWebhookInfo called %d times in polling mode, want 0", len(calls))
	}
	if calls := h.tg.CallsFor("getMe"); len(calls) == 0 {
		t.Fatal("check did not validate the token via getMe")
	}
	// A healthy check ensures a runner is registered (idempotent Upsert).
	if n := len(h.tgPoller.upsertsFor(id)); n != 1 {
		t.Fatalf("check's own Upsert calls = %d, want 1", n)
	}
}
