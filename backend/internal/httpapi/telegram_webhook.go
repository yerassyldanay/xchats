package httpapi

import (
	"crypto/subtle"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/internal/telegram"
	"github.com/yerassyldanay/xchats/backend/internal/tgingest"
)

// telegramSecretHeader is the header Telegram echoes back the secret_token in.
const telegramSecretHeader = "X-Telegram-Bot-Api-Secret-Token"

// handleTelegramWebhook is the Telegram ingress. It does NOT enqueue-and-ack:
// the normalized rows are committed BEFORE the 200. The actual ingest logic
// — normalize, upsert contact/chat/message, broadcast, enqueue — lives in
// internal/tgingest.Processor (s.tgProc), shared with the long-poll ingress
// (internal/tgpoller) for an install with no public URL; this handler is
// only the HTTP adaptation of tgingest's (Outcome, error) return.
//
// That is the entire at-least-once story. Telegram redelivers any update it
// does not get a 2xx for, so:
//   - a durability failure (tgProc.Process returning a non-nil error)
//     answers 500 and the update comes back;
//   - a crash after the commit but before the ack also comes back, and lands
//     on UNIQUE(account_id, telegram_update_id) as a no-op;
//   - anything we correctly cannot store (a group chat, a service message, an
//     unknown account) answers 200, because a non-2xx would have Telegram
//     retry it forever.
//
// It is why there is no inbound_events table: the provider already owns the
// retry buffer, and duplicating it here would only add a second thing to lose.
func (s *Server) handleTelegramWebhook(c *gin.Context) {
	rawID := c.Param("account_id")

	// Secret check first: an attacker who guesses the URL must not be able to
	// probe which account ids exist.
	//
	// An UNCONFIGURED secret rejects too — it used to skip this check
	// entirely, which meant a deployment that ended up here with no secret
	// resolvable (main.go's openCredentialsAndProvisionSecrets degrading
	// silently on a from-source install with no OS keychain, before that was
	// itself hardened to fail loudly) served every webhook call
	// unauthenticated with no signal anything was wrong. Failing closed here
	// is the backstop: a real deployment now gets a loud, continuous 401
	// instead of quietly accepting spoofed updates.
	secret := s.cfg.TelegramResolvedWebhookSecret()
	if secret == "" {
		s.log.Error("telegram webhook rejected: no webhook secret configured", "account_id", rawID)
		fail(c, http.StatusUnauthorized, ErrWebhookUnauthorized, "webhook secret not configured")
		return
	}
	if subtle.ConstantTimeCompare([]byte(c.GetHeader(telegramSecretHeader)), []byte(secret)) != 1 {
		s.log.Warn("telegram webhook auth rejected", "account_id", rawID, "reason", "bad secret token")
		fail(c, http.StatusUnauthorized, ErrWebhookUnauthorized, "bad webhook secret")
		return
	}

	accountID, err := uuid.Parse(rawID)
	if err != nil {
		fail(c, http.StatusBadRequest, ErrValidation, "invalid account id")
		return
	}

	acct, err := s.store.TgAccountByID(ctx(c), accountID)
	if err != nil {
		// Unknown or already-disconnected bot: Telegram is still pointed at us
		// from an old registration. Ack so it stops, and store nothing.
		s.log.Info("telegram webhook discarded", "account_id", accountID, "result", "unknown_account")
		c.Status(http.StatusOK)
		return
	}

	raw, err := c.GetRawData()
	if err != nil {
		fail(c, http.StatusBadRequest, ErrValidation, "unreadable body")
		return
	}
	update, err := telegram.ParseUpdate(raw)
	if err != nil {
		// Malformed body: retrying it would produce the same parse failure, so
		// ack and log rather than looping.
		s.log.Warn("telegram webhook unparsable update", "account_id", accountID, "err", err)
		c.Status(http.StatusOK)
		return
	}

	outcome, err := s.tgProc.Process(ctx(c), acct, update, raw)
	if err != nil {
		reason := "processing failed"
		if errors.Is(err, tgingest.ErrIngest) {
			reason = "ingest failed"
		} else if errors.Is(err, tgingest.ErrEnqueue) {
			reason = "enqueue failed"
		}
		s.log.Error("telegram webhook "+reason+"; not acking", "account_id", accountID,
			"update_id", update.UpdateID, "err", err)
		fail(c, http.StatusInternalServerError, ErrInternal, reason)
		return
	}

	switch outcome.Status {
	case "ignored":
		s.log.Info("telegram update dropped", "account_id", accountID, "result", outcome.Reason)
	case "duplicate":
		s.log.Info("telegram update processed", "account_id", accountID,
			"update_id", update.UpdateID, "result", "duplicate")
	case "stored":
		s.log.Info("telegram update processed", "account_id", accountID,
			"update_id", update.UpdateID, "chat_id", outcome.Result.ChatID, "result", "new",
			"bot", acct.BotUsername)
	}
	c.Status(http.StatusOK)
}
