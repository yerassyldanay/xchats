package httpapi_test

import (
	"net/http"
	"testing"
)

// TestTelegramWebhookRejectsWhenSecretUnconfigured pins A6: an unconfigured
// webhook secret must reject every call, not silently accept it. Before this
// fix, TelegramResolvedWebhookSecret() == "" skipped the auth check
// entirely — a deployment that ended up here with no secret resolvable
// served every webhook call unauthenticated. h.cfg is the SAME
// *config.Config pointer httpapi.New's Server holds (see httpapi.Deps.Cfg),
// so mutating it after newHarness returns takes effect on the next request
// without rebuilding the whole harness.
func TestTelegramWebhookRejectsWhenSecretUnconfigured(t *testing.T) {
	h := newHarness(t)
	id := h.connectBot(t, "Бот", false)
	// Blank it AFTER connecting — connectBot's own provisioning call is
	// unrelated to the inbound check under test here.
	h.cfg.TelegramWebhookSecret = ""

	if got := h.tgWebhook(id, textUpdate(1, 100, 500100, "hi"), ""); got != http.StatusUnauthorized {
		t.Errorf("status=%d, want 401 (no secret header, none configured)", got)
	}
	// Even a caller who somehow guesses/replays a plausible-looking header
	// must not get through purely because none is configured server-side.
	if got := h.tgWebhook(id, textUpdate(2, 101, 500100, "hi"), "anything-at-all"); got != http.StatusUnauthorized {
		t.Errorf("status=%d, want 401 (secret header present but none configured server-side)", got)
	}
	if n := h.countRows(`SELECT count(*) FROM tg_messages`); n != 0 {
		t.Error("an update was stored despite no webhook secret being configured")
	}
}
