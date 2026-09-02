package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/internal/config"
	"github.com/yerassyldanay/xchats/backend/internal/messengerish"
	"github.com/yerassyldanay/xchats/backend/internal/store"
	"github.com/yerassyldanay/xchats/backend/messaging"
)

// messengerCallbackPath is registered with Meta as Messenger's plain
// Facebook Login redirect_uri — see instagramCallbackPath's identical
// reasoning for why it is PUBLIC.
const messengerCallbackPath = "/meta/api/v1/oauth/messenger/callback"

// errNoFacebookPages/errMultipleFacebookPages are finishMessengerConnect's
// two distinguishable page-count failures — sentinels rather than plain
// errors.New calls so handleMessengerOAuthCallback can map each to its own
// stable metaOAuthErr* code (errors.Is, via the %w-wrapped count below) for
// the frontend, instead of both falling into the generic CONNECT_FAILED
// bucket (docs/ux/flows/03b-connect-instagram-messenger.md, friction point 5).
var (
	errNoFacebookPages       = errors.New("нет доступных Facebook Pages — предоставьте доступ хотя бы к одной Page и попробуйте снова")
	errMultipleFacebookPages = errors.New("предоставлен доступ более чем к одной Facebook Page — предоставьте доступ ровно к одной Page и попробуйте снова")
)

// messengerScopes is the permission set Messenger's connect flow requests:
// pages_show_list so PageAccounts can enumerate the operator's Pages,
// pages_messaging to send/receive DMs, pages_manage_metadata to subscribe
// the Page to this app's webhook.
var messengerScopes = []string{"pages_show_list", "pages_messaging", "pages_manage_metadata"}

// handleStartMessengerOAuth mints an oauth state and hands back Facebook
// Login's authorize_url — byte-for-byte the same shape as
// handleStartInstagramOAuth (see its doc comment for the redirect/state
// reasoning); the only difference is which authorize URL builder and scope
// list this channel needs.
func (s *Server) handleStartMessengerOAuth(c *gin.Context) {
	org, okOrg := s.orgOf(c)
	if !okOrg {
		return
	}
	u := currentUser(c)
	creds, okApp := s.metaAppCredentials(c)
	if !okApp {
		return
	}
	base := s.cfg.MetaResolvedPublicBaseURL()
	if base == "" {
		fail(c, http.StatusBadRequest, ErrValidation,
			"нет публичного HTTPS-адреса (meta.webhook_public_base_url или ngrok-туннель) — Meta не сможет вернуть подключение")
		return
	}
	redirectURI := base + messengerCallbackPath

	state, err := randomOAuthState()
	if err != nil {
		fail(c, http.StatusInternalServerError, ErrInternal, err.Error())
		return
	}
	if err := s.store.CreateMetaOAuthState(ctx(c), store.CreateMetaOAuthStateInput{
		State: state, Channel: string(messaging.ChannelMessenger),
		OrganizationID: org.ID, UserID: u.ID, RedirectURI: redirectURI, TTL: metaOAuthStateTTL,
	}); err != nil {
		fail(c, http.StatusInternalServerError, ErrInternal, err.Error())
		return
	}
	ok(c, gin.H{"authorize_url": s.metaClient.FacebookAuthorizeURL(creds.AppID, redirectURI, state, messengerScopes)})
}

// handleMessengerOAuthCallback is facebook.com's redirect target — same
// public-route trust model as handleInstagramOAuthCallback (see its doc
// comment); every exit path redirects back to /accounts with a one-shot
// query flag.
func (s *Server) handleMessengerOAuthCallback(c *gin.Context) {
	code := c.Query("code")
	stateParam := c.Query("state")
	if code == "" || stateParam == "" {
		s.redirectAccountsErr(c, "messenger_error", metaOAuthErrMissingParams, "Meta не передала code/state — попробуйте подключить заново.")
		return
	}
	st, err := s.store.MetaOAuthStateByID(ctx(c), stateParam)
	if err != nil {
		s.redirectAccountsErr(c, "messenger_error", metaOAuthErrSessionExpired, "Сессия подключения истекла или уже использована. Попробуйте снова.")
		return
	}

	acct, connectErr := s.finishMessengerConnect(ctx(c), st, code)
	if connectErr != nil {
		s.log.Warn("messenger oauth callback failed", "org_id", st.OrganizationID, "err", connectErr)
		_ = s.store.SettleMetaOAuthState(ctx(c), st.State, "failed", uuid.NullUUID{}, connectErr.Error())
		errCode := metaOAuthErrConnectFailed
		switch {
		case errors.Is(connectErr, errNoFacebookPages):
			errCode = metaOAuthErrNoPages
		case errors.Is(connectErr, errMultipleFacebookPages):
			errCode = metaOAuthErrMultiplePages
		}
		s.redirectAccountsErr(c, "messenger_error", errCode, "Не удалось подключить Messenger: "+connectErr.Error())
		return
	}
	_ = s.store.SettleMetaOAuthState(ctx(c), st.State, "connected", uuid.NullUUID{UUID: acct.ID, Valid: true}, "")
	s.redirectAccounts(c, "messenger_connected", "1")
}

// finishMessengerConnect exchanges the code, resolves exactly which Page the
// operator granted access to, claims it, seals its token, and subscribes to
// its webhook.
//
// Unlike Instagram Login (one token, one account, no picker needed), a
// Facebook user token can carry access to any number of Pages — this
// codebase deliberately does not build a Page-picker UI (it would need its
// own temporary-selection schema/step) and deliberately does not
// silently auto-pick the first Page (a silent-wrong-account footgun): the
// operator is expected to grant access to exactly one Page in Facebook's own
// consent screen. Zero or two-or-more Pages both fail the connect attempt
// with a message telling them to fix that and retry — nothing is persisted
// in either case.
func (s *Server) finishMessengerConnect(ctx context.Context, st store.MetaOAuthState, code string) (store.ChannelAccount, error) {
	creds, okApp := s.metaCreds.MetaAppCredentials(ctx)
	if !okApp {
		return store.ChannelAccount{}, errors.New("meta app id/secret не настроены")
	}

	tctx, cancel := context.WithTimeout(ctx, metaConnectTimeout)
	defer cancel()
	short, err := s.metaClient.ExchangeFacebookCode(tctx, creds.AppID, creds.AppSecret, st.RedirectURI, code)
	if err != nil {
		return store.ChannelAccount{}, err
	}
	// A long-lived USER token first, so the PAGE token PageAccounts hands
	// back inherits its non-expiring lifetime — see
	// ExchangeFacebookLongLived's own doc comment.
	long, err := s.metaClient.ExchangeFacebookLongLived(tctx, creds.AppID, creds.AppSecret, short.AccessToken)
	if err != nil {
		return store.ChannelAccount{}, err
	}
	pages, err := s.metaClient.PageAccounts(tctx, long.AccessToken)
	if err != nil {
		return store.ChannelAccount{}, err
	}
	switch len(pages) {
	case 0:
		return store.ChannelAccount{}, errNoFacebookPages
	case 1:
		// exactly one — the unambiguous, supported case.
	default:
		return store.ChannelAccount{}, fmt.Errorf("%w (%d Pages)", errMultipleFacebookPages, len(pages))
	}
	page := pages[0]

	providerMeta, _ := json.Marshal(map[string]string{})
	acct, err := s.store.ClaimChannelAccount(ctx, store.ChannelAccountClaim{
		ID:                config.ChannelAccountID(config.MessengerOwnerRef(page.ID)),
		OrganizationID:    st.OrganizationID,
		Channel:           string(messaging.ChannelMessenger),
		ExternalAccountID: page.ID,
		DisplayName:       page.Name,
		Handle:            page.Name,
		ProviderMeta:      providerMeta,
	})
	if err != nil {
		return store.ChannelAccount{}, err
	}

	// No ExpiresAt: a Page token minted from a long-lived User token does not
	// expire on its own (see ExchangeFacebookLongLived) — mirrors WhatsApp
	// Cloud's business-token credential write, unlike Instagram's.
	if err := s.store.SetChannelCredentials(ctx, store.ChannelCredentialsWrite{
		AccountID: acct.ID, Secret: page.AccessToken, TokenKind: "page_token",
	}); err != nil {
		return store.ChannelAccount{}, err
	}

	fbClient := messengerish.NewClient(s.metaClient)
	if err := fbClient.Subscribe(tctx, false, page.ID, page.AccessToken); err != nil {
		_ = s.store.SetChannelWebhookState(ctx, acct.ID, store.ChannelWebhookState{
			State: "webhook_error", LastError: err.Error(), Checked: true,
		})
		s.log.Warn("messenger subscribe failed", "account_id", acct.ID, "err", err)
		return acct, nil // the account IS connected; only the live-webhook subscription needs a retry
	}
	callbackURL := s.cfg.MetaResolvedPublicBaseURL() + "/meta/api/v1/webhook/messenger"
	_ = s.store.SetChannelWebhookState(ctx, acct.ID, store.ChannelWebhookState{
		State: "connected", URL: callbackURL, Registered: true, Checked: true,
	})
	return acct, nil
}

// handleDeleteMessengerAccount unsubscribes the Page's webhook (best-effort
// — see handleDeleteWhatsAppCloudAccount's identical reasoning) and
// soft-deletes the row.
func (s *Server) handleDeleteMessengerAccount(c *gin.Context) {
	acct, okAcct := s.orgChannelAccount(c)
	if !okAcct {
		return
	}
	if acct.Channel != string(messaging.ChannelMessenger) {
		fail(c, http.StatusNotFound, ErrNotFound, "account not found")
		return
	}
	if token, err := s.store.ChannelCredentialsSecret(ctx(c), acct.ID); err == nil {
		fbClient := messengerish.NewClient(s.metaClient)
		tctx, cancel := context.WithTimeout(ctx(c), metaConnectTimeout)
		unsubErr := fbClient.Unsubscribe(tctx, false, acct.ExternalAccountID, token)
		cancel()
		if unsubErr != nil {
			s.log.Warn("messenger unsubscribe failed", "account_id", acct.ID, "err", unsubErr)
		}
	}
	if err := s.store.ConfirmChannelDisconnect(ctx(c), acct.ID); err != nil {
		fail(c, http.StatusInternalServerError, ErrInternal, err.Error())
		return
	}
	s.log.Info("messenger account disconnected", "account_id", acct.ID)
	ok(c, gin.H{"id": acct.ID.String(), "deleted": true})
}
