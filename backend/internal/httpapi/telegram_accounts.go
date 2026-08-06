package httpapi

import (
	"context"
	"errors"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/internal/config"
	"github.com/yerassyldanay/xchats/backend/internal/dto"
	"github.com/yerassyldanay/xchats/backend/internal/store"
	"github.com/yerassyldanay/xchats/backend/internal/telegram"
)

// webhookSecretRe is Telegram's own charset rule for secret_token. Validating it
// here turns a silent setWebhook rejection into a startup-visible config error.
var webhookSecretRe = regexp.MustCompile(`^[A-Za-z0-9_-]{1,256}$`)

// telegramConnectTimeout bounds each Bot API call made while provisioning, so a
// hung Telegram cannot hold an HTTP handler (and its DB transaction) open.
const telegramConnectTimeout = 15 * time.Second

// telegramWebhookPath is the ingress path; the account uuid is the last segment.
// It is derived from the account id — which is derived from the bot id — so
// re-adding the same bot lands on the same URL and no re-registration is needed
// beyond the one this flow does anyway.
const telegramWebhookPath = "/telegram/api/v1/webhook/"

type createTelegramAccountReq struct {
	BotToken    string `json:"bot_token"`
	DisplayName string `json:"display_name"`
	// DropPendingBacklog discards whatever Telegram queued while the bot had no
	// webhook. Honoured ONLY on a first connect and only when explicitly asked:
	// on a retry it would silently delete real customer messages that piled up
	// while the webhook was broken.
	DropPendingBacklog bool `json:"drop_pending_backlog"`
}

// handleCreateTelegramAccount is the whole connect flow:
//
//	getMe (is the token real?) → claim the bot for this org + store the token in
//	one transaction → setWebhook → record the resulting state.
//
// Nothing is stored before getMe succeeds, so a typo leaves no debris. The claim
// is a single conditional upsert, so two concurrent connects of the same new bot
// cannot both win, and a bot owned by another organization is a 409 rather than
// a takeover of their history.
func (s *Server) handleCreateTelegramAccount(c *gin.Context) {
	if s.tg == nil {
		fail(c, http.StatusBadGateway, ErrTelegram, "telegram client not configured")
		return
	}
	org, okOrg := s.orgOf(c)
	if !okOrg {
		return
	}
	var req createTelegramAccountReq
	_ = c.ShouldBindJSON(&req)
	token := strings.TrimSpace(req.BotToken)
	if token == "" {
		fail(c, http.StatusBadRequest, ErrValidation, "bot_token required (получите его у @BotFather)")
		return
	}

	// Polling mode needs no public URL or webhook secret at all — that is
	// its whole point (Track 1's zero-config path). Only webhook mode
	// validates them, and only webhook mode needs baseURL below.
	polling := s.cfg.TelegramResolvedMode() == "polling"
	var baseURL string
	if !polling {
		var err error
		baseURL, err = s.telegramWebhookBase()
		if err != nil {
			fail(c, http.StatusBadRequest, ErrValidation, err.Error())
			return
		}
		if err := s.telegramSecretUsable(); err != nil {
			fail(c, http.StatusBadRequest, ErrValidation, err.Error())
			return
		}
	}

	tctx, cancel := context.WithTimeout(ctx(c), telegramConnectTimeout)
	defer cancel()
	bot, err := s.tg.GetMe(tctx, token)
	if err != nil {
		// Nothing is written: an unusable token has no account to attach to.
		s.log.Warn("telegram getMe rejected", "err", err)
		fail(c, http.StatusBadRequest, ErrTelegramToken, "Telegram отклонил токен: "+err.Error())
		return
	}

	acct, err := s.store.ClaimTelegramAccount(ctx(c), store.TelegramClaim{
		ID:             config.ChannelAccountID(config.TelegramOwnerRef(bot.ID)),
		OrganizationID: org.ID,
		DisplayName:    strings.TrimSpace(req.DisplayName),
		BotID:          bot.ID,
		BotUsername:    bot.Username,
		BotToken:       token,
	})
	if errors.Is(err, store.ErrAccountClaimed) {
		fail(c, http.StatusConflict, ErrConflict,
			"Этот бот уже подключён в другой организации. Создайте отдельного бота у @BotFather.")
		return
	}
	if errors.Is(err, store.ErrNoCredentialsKey) {
		fail(c, http.StatusBadRequest, ErrValidation, "TG_CREDENTIALS_ENC_KEY не настроен — токен негде хранить безопасно")
		return
	}
	if err != nil {
		fail(c, http.StatusInternalServerError, ErrInternal, err.Error())
		return
	}

	var state string
	if polling {
		state = s.startTelegramPolling(c, acct.ID)
	} else {
		state = s.registerTelegramWebhook(c, acct.ID, token, baseURL, req.DropPendingBacklog)
	}
	created(c, s.telegramAccountPayload(c, acct.ID, state))
}

// handleRetryTelegramWebhook re-runs setWebhook with the STORED token. It never
// drops the pending backlog: by definition this runs after a failure, and those
// pending updates are the messages that failed to arrive.
//
// In polling mode there is no webhook to re-register — see
// telegramPollingStatus's own doc comment for what "retry" means there.
func (s *Server) handleRetryTelegramWebhook(c *gin.Context) {
	acct, token, okAcct := s.telegramAccountWithToken(c)
	if !okAcct {
		return
	}
	if s.cfg.TelegramResolvedMode() == "polling" {
		ok(c, s.telegramAccountPayload(c, acct.ID, s.telegramPollingStatus(c, acct, token)))
		return
	}
	baseURL, err := s.telegramWebhookBase()
	if err != nil {
		fail(c, http.StatusBadRequest, ErrValidation, err.Error())
		return
	}
	state := s.registerTelegramWebhook(c, acct.ID, token, baseURL, false)
	ok(c, s.telegramAccountPayload(c, acct.ID, state))
}

// handleCheckTelegramAccount is the health probe: ask Telegram what it thinks
// the webhook is, and whether the token still works, then reconcile our state
// with the answer.
//
// In polling mode there is no webhook to ask about at all — see
// telegramPollingStatus's own doc comment for the polling-mode equivalent.
func (s *Server) handleCheckTelegramAccount(c *gin.Context) {
	acct, token, okAcct := s.telegramAccountWithToken(c)
	if !okAcct {
		return
	}
	if s.cfg.TelegramResolvedMode() == "polling" {
		ok(c, s.telegramAccountPayload(c, acct.ID, s.telegramPollingStatus(c, acct, token)))
		return
	}
	tctx, cancel := context.WithTimeout(ctx(c), telegramConnectTimeout)
	defer cancel()

	if _, err := s.tg.GetMe(tctx, token); err != nil {
		if telegram.IsUnauthorized(err) {
			_ = s.store.SetTelegramWebhookState(ctx(c), acct.ID, store.TelegramWebhookState{
				State: "token_error", LastError: err.Error(), Checked: true,
			})
			ok(c, s.telegramAccountPayload(c, acct.ID, "token_error"))
			return
		}
		fail(c, http.StatusBadGateway, ErrTelegram, err.Error())
		return
	}

	info, err := s.tg.GetWebhookInfo(tctx, token)
	if err != nil {
		fail(c, http.StatusBadGateway, ErrTelegram, err.Error())
		return
	}
	want := s.telegramWebhookURL(acct.ID)
	st := store.TelegramWebhookState{State: "connected", URL: info.URL, Checked: true, LastError: info.LastErrorMessage}
	if info.URL != want {
		st.State = "webhook_error"
		st.LastError = "Telegram шлёт обновления на другой адрес: " + orPlaceholder(info.URL)
	}
	if err := s.store.SetTelegramWebhookState(ctx(c), acct.ID, st); err != nil {
		fail(c, http.StatusInternalServerError, ErrInternal, err.Error())
		return
	}
	payload := s.telegramAccountPayload(c, acct.ID, st.State)
	payload["pending_update_count"] = info.PendingUpdateCount
	payload["expected_webhook_url"] = want
	ok(c, payload)
}

type replaceTelegramTokenReq struct {
	BotToken string `json:"bot_token"`
}

// handleReplaceTelegramToken swaps a rotated token in place. It must be the SAME
// bot: the account id is derived from bot_id, so a different bot would inherit
// this one's conversation history.
func (s *Server) handleReplaceTelegramToken(c *gin.Context) {
	if s.tg == nil {
		fail(c, http.StatusBadGateway, ErrTelegram, "telegram client not configured")
		return
	}
	acct, okAcct := s.orgTelegramAccount(c)
	if !okAcct {
		return
	}
	var req replaceTelegramTokenReq
	_ = c.ShouldBindJSON(&req)
	token := strings.TrimSpace(req.BotToken)
	if token == "" {
		fail(c, http.StatusBadRequest, ErrValidation, "bot_token required")
		return
	}
	tctx, cancel := context.WithTimeout(ctx(c), telegramConnectTimeout)
	defer cancel()
	bot, err := s.tg.GetMe(tctx, token)
	if err != nil {
		fail(c, http.StatusBadRequest, ErrTelegramToken, "Telegram отклонил токен: "+err.Error())
		return
	}
	if bot.ID != acct.BotID {
		fail(c, http.StatusConflict, ErrConflict,
			"Этот токен принадлежит другому боту (@"+bot.Username+"). Подключите его как отдельный аккаунт.")
		return
	}
	if err := s.store.ReplaceTelegramToken(ctx(c), acct.ID, bot.ID, bot.Username, token); err != nil {
		if errors.Is(err, store.ErrAccountClaimed) {
			fail(c, http.StatusConflict, ErrConflict, "аккаунт не найден или принадлежит другому боту")
			return
		}
		fail(c, http.StatusInternalServerError, ErrInternal, err.Error())
		return
	}

	var state string
	if s.cfg.TelegramResolvedMode() == "polling" {
		// The stored token changed, so tg_credentials.updated_at (the
		// TokenVersion ListTelegramPollBots reports) changed with it —
		// Upsert sees a version mismatch against whatever runner is
		// currently registered (including a 401-stopped one) and replaces
		// it, exactly like a fresh connect.
		state = s.startTelegramPolling(c, acct.ID)
	} else {
		baseURL, err := s.telegramWebhookBase()
		if err != nil {
			fail(c, http.StatusBadRequest, ErrValidation, err.Error())
			return
		}
		state = s.registerTelegramWebhook(c, acct.ID, token, baseURL, false)
	}
	ok(c, s.telegramAccountPayload(c, acct.ID, state))
}

// handleDeleteTelegramAccount is the disconnect state machine. The order is the
// point: the row is marked disconnect_pending FIRST, the webhook is removed
// SECOND, and only a confirmed removal purges the token and soft-deletes the
// account. If Telegram refuses, the account stays visible in disconnect_error
// WITH its token, so repeating the DELETE is the retry — the alternative
// (delete our row anyway) would leave a bot forever posting to an endpoint that
// no longer knows it.
func (s *Server) handleDeleteTelegramAccount(c *gin.Context) {
	acct, token, okAcct := s.telegramAccountWithToken(c)
	if !okAcct {
		return
	}
	polling := s.cfg.TelegramResolvedMode() == "polling"
	if polling {
		// Stop the live runner FIRST: it must not still be mid-getUpdates
		// (or about to start one) when ConfirmTelegramDisconnect below
		// deletes the very credentials row TelegramBotToken would resolve.
		s.tgPoller.Remove(acct.ID)
	}

	if err := s.store.SetTelegramWebhookState(ctx(c), acct.ID, store.TelegramWebhookState{
		State: "disconnect_pending",
	}); err != nil {
		fail(c, http.StatusInternalServerError, ErrInternal, err.Error())
		return
	}

	// Polling mode never registered a webhook, so there is nothing to tear
	// down here — go straight to confirming the disconnect.
	if !polling {
		tctx, cancel := context.WithTimeout(ctx(c), telegramConnectTimeout)
		defer cancel()
		if err := s.tg.DeleteWebhook(tctx, token, false); err != nil && !telegram.IsUnauthorized(err) {
			// Not unauthorized => Telegram may still be delivering. Keep the token.
			_ = s.store.SetTelegramWebhookState(ctx(c), acct.ID, store.TelegramWebhookState{
				State: "disconnect_error", LastError: err.Error(),
			})
			s.log.Warn("telegram deleteWebhook failed", "account_id", acct.ID, "err", err)
			fail(c, http.StatusBadGateway, ErrTelegram,
				"Не удалось отключить вебхук в Telegram — попробуйте ещё раз. "+err.Error())
			return
		}
		// An unauthorized token means the bot is already unusable: there is
		// nothing left to deliver to us, so the disconnect is effectively
		// confirmed.
	}

	if err := s.store.ConfirmTelegramDisconnect(ctx(c), acct.ID); err != nil {
		fail(c, http.StatusInternalServerError, ErrInternal, err.Error())
		return
	}
	s.log.Info("telegram account disconnected", "account_id", acct.ID, "bot", acct.BotUsername)
	ok(c, gin.H{"id": acct.ID.String(), "deleted": true})
}

// --- helpers --------------------------------------------------------------

// registerTelegramWebhook performs setWebhook and records the resulting state,
// returning it.
//
// The interesting branch is the ambiguous one. An explicit rejection is easy:
// Telegram said no, record webhook_error with its reason. A TIMEOUT is not — the
// call may well have landed. Guessing "failed" there would leave a working bot
// permanently displayed as broken, so getWebhookInfo is asked what Telegram
// actually believes and the URL comparison decides.
func (s *Server) registerTelegramWebhook(c *gin.Context, accountID uuid.UUID, token, baseURL string, dropPending bool) string {
	url := strings.TrimRight(baseURL, "/") + telegramWebhookPath + accountID.String()

	tctx, cancel := context.WithTimeout(ctx(c), telegramConnectTimeout)
	defer cancel()

	err := s.tg.SetWebhook(tctx, token, url, s.cfg.TelegramResolvedWebhookSecret(), telegram.DefaultAllowedUpdates, dropPending)
	if err == nil {
		_ = s.store.SetTelegramWebhookState(ctx(c), accountID, store.TelegramWebhookState{
			State: "connected", URL: url, Registered: true, Checked: true,
		})
		s.log.Info("telegram webhook registered", "account_id", accountID, "drop_pending", dropPending)
		return "connected"
	}

	if code := telegram.APIErrorCode(err); code != 0 {
		// Telegram answered, and its answer was no.
		_ = s.store.SetTelegramWebhookState(ctx(c), accountID, store.TelegramWebhookState{
			State: telegramFailState(code), URL: url, LastError: err.Error(), Checked: true,
		})
		s.log.Warn("telegram setWebhook rejected", "account_id", accountID, "code", code, "err", err)
		return telegramFailState(code)
	}

	// Ambiguous (timeout / transport): ask Telegram what it believes.
	rctx, rcancel := context.WithTimeout(ctx(c), telegramConnectTimeout)
	defer rcancel()
	if info, ierr := s.tg.GetWebhookInfo(rctx, token); ierr == nil && info.URL == url {
		_ = s.store.SetTelegramWebhookState(ctx(c), accountID, store.TelegramWebhookState{
			State: "connected", URL: url, Registered: true, Checked: true,
		})
		s.log.Info("telegram setWebhook timed out but reconciled as registered", "account_id", accountID)
		return "connected"
	}
	_ = s.store.SetTelegramWebhookState(ctx(c), accountID, store.TelegramWebhookState{
		State: "webhook_error", URL: url, LastError: err.Error(), Checked: true,
	})
	s.log.Warn("telegram setWebhook failed", "account_id", accountID, "err", err)
	return "webhook_error"
}

// startTelegramPolling is registerTelegramWebhook's polling-mode
// counterpart: instead of registering a webhook URL with Telegram, it hands
// the account to the poller manager (internal/tgpoller) and returns the
// account's current connection_state. Unlike registerTelegramWebhook this
// does not itself connect anything synchronously — Upsert just spawns (or
// replaces) the runner goroutine, which is what actually clears any stale
// webhook and starts polling — so the returned state is whatever
// ClaimTelegramAccount/ReplaceTelegramToken already set ("connecting"),
// not a premature "connected".
func (s *Server) startTelegramPolling(c *gin.Context, accountID uuid.UUID) string {
	spec, ok := s.telegramPollSpecFor(ctx(c), accountID)
	if !ok {
		s.log.Error("telegram: could not resolve a poll spec right after claiming", "account_id", accountID)
		return "webhook_error"
	}
	s.tgPoller.Upsert(spec)
	if acct, err := s.store.TgAccountByID(ctx(c), accountID); err == nil {
		return acct.ConnectionState
	}
	return "connecting"
}

// telegramPollingStatus is the polling-mode equivalent of a webhook health
// check (handleRetryTelegramWebhook/handleCheckTelegramAccount): there is no
// webhook to inspect or re-register in this mode, so it validates the token
// with GetMe, ensures a runner is actually registered for it (covering a
// boot race or a previously failed Upsert), and reports the account's
// current connection_state — which the poller runner itself keeps current
// via SetTelegramWebhookState as it works.
func (s *Server) telegramPollingStatus(c *gin.Context, acct store.TelegramAccount, token string) string {
	tctx, cancel := context.WithTimeout(ctx(c), telegramConnectTimeout)
	defer cancel()
	if _, err := s.tg.GetMe(tctx, token); err != nil {
		if telegram.IsUnauthorized(err) {
			_ = s.store.SetTelegramWebhookState(ctx(c), acct.ID, store.TelegramWebhookState{
				State: "token_error", LastError: err.Error(), Checked: true,
			})
			return "token_error"
		}
		s.log.Warn("telegram polling status: getMe failed", "account_id", acct.ID, "err", err)
		return acct.ConnectionState
	}
	if spec, ok := s.telegramPollSpecFor(ctx(c), acct.ID); ok {
		s.tgPoller.Upsert(spec)
	}
	if fresh, err := s.store.TgAccountByID(ctx(c), acct.ID); err == nil {
		return fresh.ConnectionState
	}
	return acct.ConnectionState
}

// telegramPollSpecFor resolves the store.TelegramPollBot Upsert needs for
// accountID — the exact same shape ListTelegramPollBots produces at boot,
// so a direct Upsert from an HTTP handler and a later boot/Reconcile from
// ListTelegramPollBots can never disagree about the bot's current
// TokenVersion. Scans the full eligible-bots list rather than adding a
// narrower store method: this only runs on a rare, operator-triggered
// action (connect/replace-token/retry), never a hot path.
func (s *Server) telegramPollSpecFor(ctx context.Context, accountID uuid.UUID) (store.TelegramPollBot, bool) {
	bots, err := s.store.ListTelegramPollBots(ctx)
	if err != nil {
		s.log.Warn("telegram: list poll bots failed", "account_id", accountID, "err", err)
		return store.TelegramPollBot{}, false
	}
	for _, b := range bots {
		if b.Account.ID == accountID {
			return b, true
		}
	}
	return store.TelegramPollBot{}, false
}

// telegramFailState separates "your token is dead" from "your URL is wrong":
// the operator fixes those in completely different places.
func telegramFailState(code int) string {
	if code == http.StatusUnauthorized {
		return "token_error"
	}
	return "webhook_error"
}

// telegramWebhookBase validates the configured public base URL. Telegram will
// only call a public HTTPS endpoint, so an http:// or empty value is caught here
// with an actionable message instead of surfacing as an opaque Bot API 400.
func (s *Server) telegramWebhookBase() (string, error) {
	base := strings.TrimSpace(s.cfg.TelegramWebhookPublicBaseURL)
	if base == "" {
		return "", errors.New("TG_WEBHOOK_PUBLIC_BASE_URL не настроен — Telegram не сможет доставлять сообщения")
	}
	if !strings.HasPrefix(strings.ToLower(base), "https://") {
		return "", errors.New("TG_WEBHOOK_PUBLIC_BASE_URL должен начинаться с https:// — Telegram не принимает http")
	}
	return base, nil
}

// telegramSecretUsable checks the effective webhook secret (TG_WEBHOOK_SECRET,
// falling back to WEBHOOK_TOKEN) against Telegram's charset rule. An empty
// secret is allowed (dev): registration then omits secret_token and the
// ingress skips the check.
func (s *Server) telegramSecretUsable() error {
	secret := s.cfg.TelegramResolvedWebhookSecret()
	if secret == "" {
		return nil
	}
	if !webhookSecretRe.MatchString(secret) {
		return errors.New("TG_WEBHOOK_SECRET (или WEBHOOK_TOKEN как fallback) содержит недопустимые символы для Telegram (разрешены A-Z a-z 0-9 _ -, до 256 символов)")
	}
	return nil
}

func (s *Server) telegramWebhookURL(accountID uuid.UUID) string {
	return strings.TrimRight(s.cfg.TelegramWebhookPublicBaseURL, "/") + telegramWebhookPath + accountID.String()
}

// orgTelegramAccount resolves a Telegram account id under the caller's org.
func (s *Server) orgTelegramAccount(c *gin.Context) (store.TelegramAccount, bool) {
	id, okID := parseUUID(c, "id")
	if !okID {
		return store.TelegramAccount{}, false
	}
	org, okOrg := s.orgOf(c)
	if !okOrg {
		return store.TelegramAccount{}, false
	}
	acct, err := s.store.TgAccountByIDForOrg(ctx(c), id, org.ID)
	if err != nil {
		fail(c, http.StatusNotFound, ErrNotFound, "account not found")
		return store.TelegramAccount{}, false
	}
	return acct, true
}

// telegramAccountWithToken is orgTelegramAccount plus the decrypted token every
// lifecycle call needs.
func (s *Server) telegramAccountWithToken(c *gin.Context) (store.TelegramAccount, string, bool) {
	if s.tg == nil {
		fail(c, http.StatusBadGateway, ErrTelegram, "telegram client not configured")
		return store.TelegramAccount{}, "", false
	}
	acct, okAcct := s.orgTelegramAccount(c)
	if !okAcct {
		return store.TelegramAccount{}, "", false
	}
	token, err := s.store.TelegramBotToken(ctx(c), acct.ID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			fail(c, http.StatusConflict, ErrConflict, "токен не сохранён — подключите бота заново")
			return store.TelegramAccount{}, "", false
		}
		fail(c, http.StatusInternalServerError, ErrInternal, err.Error())
		return store.TelegramAccount{}, "", false
	}
	return acct, token, true
}

// telegramAccountPayload re-reads the account through the neutral view so the
// response is byte-identical to a row from GET /accounts. state is the
// just-computed connection state, used if the re-read fails.
func (s *Server) telegramAccountPayload(c *gin.Context, id uuid.UUID, state string) gin.H {
	if acct, err := s.store.AccountByID(ctx(c), id); err == nil {
		return gin.H{"account": dto.MapNeutralAccount(acct, ""), "connection_state": acct.ConnectionState}
	}
	return gin.H{"account": gin.H{"id": id.String(), "channel": "telegram"}, "connection_state": state}
}

func orPlaceholder(s string) string {
	if s == "" {
		return "(пусто)"
	}
	return s
}
