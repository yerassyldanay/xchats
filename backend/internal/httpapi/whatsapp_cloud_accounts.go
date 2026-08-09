package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/internal/config"
	"github.com/yerassyldanay/xchats/backend/internal/meta"
	"github.com/yerassyldanay/xchats/backend/internal/store"
	"github.com/yerassyldanay/xchats/backend/internal/whatsappcloud"
	"github.com/yerassyldanay/xchats/backend/messaging"
)

// metaConnectTimeout bounds every Graph API call the WhatsApp Cloud connect
// flow makes, mirroring telegramConnectTimeout.
const metaConnectTimeout = 15 * time.Second

// metaWhatsAppWebhookPath is the WhatsApp Cloud ingress path — registered
// with Meta via subscribed_apps (registerWhatsAppCloudWebhook), and where
// handleWhatsAppCloudWebhook listens (see server.go's route table). Unlike
// Telegram's per-account webhook path this is NOT account-specific: WhatsApp
// Cloud identifies the target account from the payload's own
// phone_number_id, not the URL (see handleWhatsAppCloudWebhook's doc
// comment).
const metaWhatsAppWebhookPath = "/meta/api/v1/webhook/whatsapp"

// pin6Digits is WhatsApp Cloud's two-step-verification PIN shape.
var pin6Digits = regexp.MustCompile(`^[0-9]{6}$`)

type discoverWhatsAppCloudNumbersReq struct {
	WABAID        string `json:"waba_id"`
	BusinessToken string `json:"business_token"`
}

// handleDiscoverWhatsAppCloudNumbers is the connect flow's first step: prove
// the pasted WABA id + business token are real and list the WABA's
// registered numbers for the operator to pick from. It is a read-only Graph
// call (PhoneNumbers) — nothing is persisted here, and nothing rate-limited
// (Register's PIN-attempt budget) is spent — so an abandoned or mistyped
// attempt leaves no debris, exactly like handleCreateTelegramAccount's own
// getMe-before-anything-is-stored shape.
func (s *Server) handleDiscoverWhatsAppCloudNumbers(c *gin.Context) {
	if _, okOrg := s.orgOf(c); !okOrg {
		return
	}
	var req discoverWhatsAppCloudNumbersReq
	_ = c.ShouldBindJSON(&req)
	wabaID := strings.TrimSpace(req.WABAID)
	token := strings.TrimSpace(req.BusinessToken)
	if wabaID == "" || token == "" {
		fail(c, http.StatusBadRequest, ErrValidation, "waba_id and business_token are required")
		return
	}

	tctx, cancel := context.WithTimeout(ctx(c), metaConnectTimeout)
	defer cancel()
	nums, err := s.waCloud.PhoneNumbers(tctx, wabaID, token)
	if err != nil {
		metaFail(c, err)
		return
	}
	ok(c, gin.H{"phone_numbers": nums})
}

type connectWhatsAppCloudAccountReq struct {
	WABAID        string `json:"waba_id"`
	BusinessToken string `json:"business_token"`
	PhoneNumberID string `json:"phone_number_id"`
	PIN           string `json:"pin"`
	DisplayName   string `json:"display_name"`
	// DisplayPhoneNumber is the E.164-ish label the discover step's own
	// PhoneNumbers response already showed the operator (e.g. "+1 555 000
	// 1111") — phone_number_id itself is an opaque Graph API id, not a
	// human-readable number, so the frontend carries this forward rather
	// than the backend re-fetching it. Purely cosmetic (channel_accounts.
	// handle, GET /accounts' external_handle): never used for auth or
	// provider calls.
	DisplayPhoneNumber string `json:"display_phone_number"`
}

// whatsAppCloudProviderMeta is channel_accounts.provider_meta's shape for a
// whatsapp_cloud row — just the owning WABA id, the one provider-specific
// fact the generic core has nowhere else to keep (external_account_id is
// already spent on phone_number_id — see config.WhatsAppCloudOwnerRef).
type whatsAppCloudProviderMeta struct {
	WABAID string `json:"waba_id"`
}

// handleConnectWhatsAppCloudAccount is the connect flow's second, committing
// step: activate Cloud API sending on the chosen number (Register, which
// spends one of Meta's limited PIN attempts — see Client.Register's own doc
// comment), then claim the account, seal the business token, and point the
// WABA's webhook at this install.
func (s *Server) handleConnectWhatsAppCloudAccount(c *gin.Context) {
	org, okOrg := s.orgOf(c)
	if !okOrg {
		return
	}
	var req connectWhatsAppCloudAccountReq
	_ = c.ShouldBindJSON(&req)
	wabaID := strings.TrimSpace(req.WABAID)
	token := strings.TrimSpace(req.BusinessToken)
	phoneNumberID := strings.TrimSpace(req.PhoneNumberID)
	pin := strings.TrimSpace(req.PIN)
	if wabaID == "" || token == "" || phoneNumberID == "" {
		fail(c, http.StatusBadRequest, ErrValidation, "waba_id, business_token and phone_number_id are required")
		return
	}
	if !pin6Digits.MatchString(pin) {
		fail(c, http.StatusBadRequest, ErrValidation, "pin must be exactly 6 digits")
		return
	}
	// Catch the setup-ordering mistake here rather than let the operator
	// connect a number whose inbound webhook can never be verified (the
	// signature check in handleWhatsAppCloudWebhook needs the same App
	// Secret).
	if _, okApp := s.metaAppCredentials(c); !okApp {
		return
	}

	rctx, rcancel := context.WithTimeout(ctx(c), metaConnectTimeout)
	defer rcancel()
	if err := s.waCloud.Register(rctx, phoneNumberID, pin, token); err != nil {
		metaFail(c, err)
		return
	}

	providerMeta, _ := json.Marshal(whatsAppCloudProviderMeta{WABAID: wabaID})
	acct, err := s.store.ClaimChannelAccount(ctx(c), store.ChannelAccountClaim{
		ID:                config.ChannelAccountID(config.WhatsAppCloudOwnerRef(phoneNumberID)),
		OrganizationID:    org.ID,
		Channel:           string(messaging.ChannelWhatsAppCloud),
		ExternalAccountID: phoneNumberID,
		DisplayName:       strings.TrimSpace(req.DisplayName),
		Handle:            strings.TrimSpace(req.DisplayPhoneNumber),
		ProviderMeta:      providerMeta,
	})
	if errors.Is(err, store.ErrChannelAccountClaimed) {
		fail(c, http.StatusConflict, ErrConflict, "этот номер уже подключён в другой организации")
		return
	}
	if err != nil {
		fail(c, http.StatusInternalServerError, ErrInternal, err.Error())
		return
	}

	if err := s.store.SetChannelCredentials(ctx(c), store.ChannelCredentialsWrite{
		AccountID: acct.ID, Secret: token, TokenKind: "business_token",
	}); err != nil {
		if errors.Is(err, store.ErrNoCredentialsKey) {
			fail(c, http.StatusBadRequest, ErrValidation, "credentials encryption is not configured — токен негде хранить безопасно")
			return
		}
		fail(c, http.StatusInternalServerError, ErrInternal, err.Error())
		return
	}

	s.registerWhatsAppCloudWebhook(c, acct.ID, wabaID, token)
	created(c, s.channelAccountPayload(c, acct.ID))
}

// handleDeleteWhatsAppCloudAccount disconnects a WhatsApp Cloud account:
// best-effort clear the WABA's webhook override, then hard-delete the
// credential and soft-delete the account (store.ConfirmChannelDisconnect) —
// history stays, exactly like handleDeleteTelegramAccount.
//
// Unlike Telegram, a failed provider-side clear is not worth blocking the
// disconnect over: there is no risk of Meta continuing to "deliver to
// nobody" in a way that costs anything, and the operator's own token is
// about to be deleted from our side regardless — so this always proceeds to
// ConfirmChannelDisconnect rather than leaving the account stuck in a
// disconnect_error limbo the way Telegram's DeleteWebhook failure does.
func (s *Server) handleDeleteWhatsAppCloudAccount(c *gin.Context) {
	acct, okAcct := s.orgChannelAccount(c)
	if !okAcct {
		return
	}
	if acct.Channel != string(messaging.ChannelWhatsAppCloud) {
		fail(c, http.StatusNotFound, ErrNotFound, "account not found")
		return
	}

	if token, err := s.store.ChannelCredentialsSecret(ctx(c), acct.ID); err == nil {
		if wabaID := whatsAppCloudWABAID(acct); wabaID != "" {
			tctx, cancel := context.WithTimeout(ctx(c), metaConnectTimeout)
			err := s.waCloud.ClearOverride(tctx, wabaID, token)
			cancel()
			if err != nil {
				s.log.Warn("whatsapp cloud ClearOverride failed", "account_id", acct.ID, "err", err)
			}
		}
	} else if !errors.Is(err, store.ErrNotFound) {
		s.log.Warn("whatsapp cloud disconnect: resolve token failed", "account_id", acct.ID, "err", err)
	}

	if err := s.store.ConfirmChannelDisconnect(ctx(c), acct.ID); err != nil {
		fail(c, http.StatusInternalServerError, ErrInternal, err.Error())
		return
	}
	s.log.Info("whatsapp cloud account disconnected", "account_id", acct.ID)
	ok(c, gin.H{"id": acct.ID.String(), "deleted": true})
}

// registerWhatsAppCloudWebhook points the WABA's webhook override at this
// install's own public URL and records the resulting connection/webhook
// state — the WA Cloud analogue of registerTelegramWebhook. Unlike Telegram
// there is no per-account secret to embed in the URL: the callback path is
// fixed (metaWhatsAppWebhookPath) and every inbound POST is authenticated by
// X-Hub-Signature-256 (the shared App Secret), not by anything in the URL —
// VerifyToken here only has to be reproducible at the one-time GET handshake
// (see meta.DeriveVerifyToken).
func (s *Server) registerWhatsAppCloudWebhook(c *gin.Context, accountID uuid.UUID, wabaID, token string) string {
	base := s.cfg.MetaResolvedPublicBaseURL()
	if base == "" {
		_ = s.store.SetChannelWebhookState(ctx(c), accountID, store.ChannelWebhookState{
			State: "webhook_error",
			LastError: "нет публичного HTTPS-адреса (meta.webhook_public_base_url или ngrok-туннель) — " +
				"Meta не сможет доставлять сообщения",
		})
		return "webhook_error"
	}
	callbackURL := base + metaWhatsAppWebhookPath

	tctx, cancel := context.WithTimeout(ctx(c), metaConnectTimeout)
	defer cancel()
	err := s.waCloud.SetOverride(tctx, wabaID, whatsappcloud.SubscribedAppsOverride{
		CallbackURI: callbackURL, VerifyToken: meta.DeriveVerifyToken(s.cfg.SessionSecret),
	}, token)
	if err != nil {
		_ = s.store.SetChannelWebhookState(ctx(c), accountID, store.ChannelWebhookState{
			State: "webhook_error", URL: callbackURL, LastError: err.Error(), Checked: true,
		})
		s.log.Warn("whatsapp cloud SetOverride failed", "account_id", accountID, "err", err)
		return "webhook_error"
	}
	_ = s.store.SetChannelWebhookState(ctx(c), accountID, store.ChannelWebhookState{
		State: "connected", URL: callbackURL, Registered: true, Checked: true,
	})
	s.log.Info("whatsapp cloud webhook registered", "account_id", accountID)
	return "connected"
}

// whatsAppCloudWABAID reads a channel_accounts row's owning WABA id back out
// of provider_meta — see whatsAppCloudProviderMeta.
func whatsAppCloudWABAID(acct store.ChannelAccount) string {
	var m whatsAppCloudProviderMeta
	if err := json.Unmarshal(acct.ProviderMeta, &m); err != nil {
		return ""
	}
	return m.WABAID
}
