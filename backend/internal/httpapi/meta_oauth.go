package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/internal/config"
	"github.com/yerassyldanay/xchats/backend/internal/messengerish"
	"github.com/yerassyldanay/xchats/backend/internal/meta"
	"github.com/yerassyldanay/xchats/backend/internal/store"
	"github.com/yerassyldanay/xchats/backend/messaging"
)

// metaOAuthStateTTL bounds how long a connect attempt stays redeemable —
// generous enough for an operator to read Meta's consent screen, short
// enough that an abandoned attempt does not linger.
const metaOAuthStateTTL = 10 * time.Minute

// instagramCallbackPath is registered with Meta as Instagram Login's
// redirect_uri — see server.go's route table. It is PUBLIC and does not
// require the browser's session cookie to have survived the round trip to
// instagram.com and back (some browsers/SameSite policies would drop it):
// see handleInstagramOAuthCallback's own doc comment for why
// meta_oauth_states carries the caller's identity instead.
const instagramCallbackPath = "/meta/api/v1/oauth/instagram/callback"

// randomOAuthState mints an unguessable, single-use nonce — the entire
// security boundary of the public callback below (see
// store.CreateMetaOAuthState's own doc comment).
func randomOAuthState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// handleStartInstagramOAuth is the connect flow's first step: mint a state
// bound to the caller's own organization/user (while they are still an
// authenticated in-app request), and hand back the URL to redirect the
// browser's TOP-LEVEL navigation to — never fetched via XHR/fetch, Meta's
// consent dialog refuses to render inside anything but a real top-level
// page.
func (s *Server) handleStartInstagramOAuth(c *gin.Context) {
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
	redirectURI := base + instagramCallbackPath

	state, err := randomOAuthState()
	if err != nil {
		fail(c, http.StatusInternalServerError, ErrInternal, err.Error())
		return
	}
	if err := s.store.CreateMetaOAuthState(ctx(c), store.CreateMetaOAuthStateInput{
		State: state, Channel: string(messaging.ChannelInstagram),
		OrganizationID: org.ID, UserID: u.ID, RedirectURI: redirectURI, TTL: metaOAuthStateTTL,
	}); err != nil {
		fail(c, http.StatusInternalServerError, ErrInternal, err.Error())
		return
	}
	ok(c, gin.H{"authorize_url": meta.InstagramAuthorizeURL(creds.AppID, redirectURI, state)})
}

// handleInstagramOAuthCallback lands here directly from instagram.com's
// consent redirect — a plain browser GET, not a fetch from the SPA, so it
// can carry NO Authorization header and its session cookie may or may not
// have survived the cross-site hop depending on the browser's SameSite
// enforcement. It is deliberately a PUBLIC route: every fact it needs about
// WHO is connecting (organization_id, user_id) was already captured in
// meta_oauth_states back when handleStartInstagramOAuth ran as a real
// authenticated request — state is the sole credential this handler trusts,
// and it is an unguessable, single-use, short-lived nonce (see
// store.MetaOAuthStateByID/SettleMetaOAuthState).
//
// Every exit path is an HTTP redirect back to the frontend SPA (never a
// JSON envelope — there is no fetch caller waiting on one), landing on
// /accounts with a query flag the UI reads once to show a toast.
func (s *Server) handleInstagramOAuthCallback(c *gin.Context) {
	code := c.Query("code")
	stateParam := c.Query("state")
	if code == "" || stateParam == "" {
		s.redirectAccounts(c, "instagram_error", "Meta не передала code/state — попробуйте подключить заново.")
		return
	}
	st, err := s.store.MetaOAuthStateByID(ctx(c), stateParam)
	if err != nil {
		s.redirectAccounts(c, "instagram_error", "Сессия подключения истекла или уже использована. Попробуйте снова.")
		return
	}

	acct, connectErr := s.finishInstagramConnect(ctx(c), st, code)
	if connectErr != nil {
		s.log.Warn("instagram oauth callback failed", "org_id", st.OrganizationID, "err", connectErr)
		_ = s.store.SettleMetaOAuthState(ctx(c), st.State, "failed", uuid.NullUUID{}, connectErr.Error())
		s.redirectAccounts(c, "instagram_error", "Не удалось подключить Instagram: "+connectErr.Error())
		return
	}
	_ = s.store.SettleMetaOAuthState(ctx(c), st.State, "connected", uuid.NullUUID{UUID: acct.ID, Valid: true}, "")
	s.redirectAccounts(c, "instagram_connected", "1")
}

// finishInstagramConnect does the actual work once a valid state is in
// hand: exchange the code, resolve the connected account's own identity,
// claim it, seal the token, and subscribe to its webhook. Nothing is
// written before ExchangeInstagramCode succeeds (a stale/replayed code
// fails there first), mirroring handleCreateTelegramAccount's own
// nothing-stored-before-getMe-succeeds shape.
func (s *Server) finishInstagramConnect(ctx context.Context, st store.MetaOAuthState, code string) (store.ChannelAccount, error) {
	creds, okApp := s.metaCreds.MetaAppCredentials(ctx)
	if !okApp {
		return store.ChannelAccount{}, errors.New("meta app id/secret не настроены")
	}

	tctx, cancel := context.WithTimeout(ctx, metaConnectTimeout)
	defer cancel()
	short, err := s.metaClient.ExchangeInstagramCode(tctx, creds.AppID, creds.AppSecret, st.RedirectURI, code)
	if err != nil {
		return store.ChannelAccount{}, err
	}
	long, err := s.metaClient.ExchangeInstagramLongLived(tctx, creds.AppSecret, short.AccessToken)
	if err != nil {
		return store.ChannelAccount{}, err
	}
	igClient := messengerish.NewClient(s.metaClient)
	me, err := igClient.Me(tctx, true, long.AccessToken)
	if err != nil {
		return store.ChannelAccount{}, err
	}

	handle := ""
	if me.Username != "" {
		handle = "@" + me.Username
	}
	providerMeta, _ := json.Marshal(map[string]string{})
	acct, err := s.store.ClaimChannelAccount(ctx, store.ChannelAccountClaim{
		ID:                config.ChannelAccountID(config.InstagramOwnerRef(me.UserID)),
		OrganizationID:    st.OrganizationID,
		Channel:           string(messaging.ChannelInstagram),
		ExternalAccountID: me.UserID,
		DisplayName:       me.Name,
		Handle:            handle,
		ProviderMeta:      providerMeta,
	})
	if err != nil {
		return store.ChannelAccount{}, err
	}

	var expiresAt *time.Time
	if long.ExpiresIn > 0 {
		t := time.Now().Add(time.Duration(long.ExpiresIn) * time.Second)
		expiresAt = &t
	}
	if err := s.store.SetChannelCredentials(ctx, store.ChannelCredentialsWrite{
		AccountID: acct.ID, Secret: long.AccessToken, TokenKind: "long_lived_user_token", ExpiresAt: expiresAt,
	}); err != nil {
		return store.ChannelAccount{}, err
	}

	if err := igClient.Subscribe(tctx, true, me.UserID, long.AccessToken); err != nil {
		_ = s.store.SetChannelWebhookState(ctx, acct.ID, store.ChannelWebhookState{
			State: "webhook_error", LastError: err.Error(), Checked: true,
		})
		s.log.Warn("instagram subscribe failed", "account_id", acct.ID, "err", err)
		return acct, nil // the account IS connected; only the live-webhook subscription needs a retry
	}
	callbackURL := s.cfg.MetaResolvedPublicBaseURL() + "/meta/api/v1/webhook/instagram"
	_ = s.store.SetChannelWebhookState(ctx, acct.ID, store.ChannelWebhookState{
		State: "connected", URL: callbackURL, Registered: true, Checked: true,
	})
	return acct, nil
}

// handleDeleteInstagramAccount unsubscribes the account's webhook
// (best-effort — see handleDeleteWhatsAppCloudAccount's identical
// reasoning for why a failure here still proceeds to disconnect) and
// soft-deletes the row.
func (s *Server) handleDeleteInstagramAccount(c *gin.Context) {
	acct, okAcct := s.orgChannelAccount(c)
	if !okAcct {
		return
	}
	if acct.Channel != string(messaging.ChannelInstagram) {
		fail(c, http.StatusNotFound, ErrNotFound, "account not found")
		return
	}
	if token, err := s.store.ChannelCredentialsSecret(ctx(c), acct.ID); err == nil {
		igClient := messengerish.NewClient(s.metaClient)
		tctx, cancel := context.WithTimeout(ctx(c), metaConnectTimeout)
		unsubErr := igClient.Unsubscribe(tctx, true, acct.ExternalAccountID, token)
		cancel()
		if unsubErr != nil {
			s.log.Warn("instagram unsubscribe failed", "account_id", acct.ID, "err", unsubErr)
		}
	}
	if err := s.store.ConfirmChannelDisconnect(ctx(c), acct.ID); err != nil {
		fail(c, http.StatusInternalServerError, ErrInternal, err.Error())
		return
	}
	s.log.Info("instagram account disconnected", "account_id", acct.ID)
	ok(c, gin.H{"id": acct.ID.String(), "deleted": true})
}

// redirectAccounts sends the browser back to the SPA's Каналы page with a
// one-shot query param — key is "instagram_connected" (value "1") or
// "instagram_error" (value is the message to show). Always an explicit
// key=value pair, deliberately never a bare "?key" flag: a valueless query
// param parses as null in vue-router, which reads exactly as falsy/absent
// as skipping the param entirely — see Accounts.vue's onMounted handler.
func (s *Server) redirectAccounts(c *gin.Context, key, value string) {
	u := s.cfg.ResolvedFrontendBaseURL() + "/accounts?" + key + "=" + url.QueryEscape(value)
	c.Redirect(http.StatusFound, u)
}
