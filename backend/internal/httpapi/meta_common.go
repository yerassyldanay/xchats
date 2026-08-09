package httpapi

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/internal/credentials"
	"github.com/yerassyldanay/xchats/backend/internal/dto"
	"github.com/yerassyldanay/xchats/backend/internal/meta"
	"github.com/yerassyldanay/xchats/backend/internal/store"
)

// metaCredentialsAdapter satisfies meta.Source by reading the "meta"
// provider's two fields off the shared credentials.Chain — the same store
// every other integration's API key lives in (internal/credentials'
// Providers list). It is a small adapter here, rather than a method on
// *credentials.Chain itself, so that foundational package stays ignorant of
// any specific provider's shape — see meta.Source's own doc comment. A nil
// chain (no secure credential store available this boot) makes every lookup
// report "not configured" rather than panicking.
type metaCredentialsAdapter struct {
	chain *credentials.Chain
}

func (a metaCredentialsAdapter) MetaAppCredentials(ctx context.Context) (meta.AppCredentials, bool) {
	if a.chain == nil {
		return meta.AppCredentials{}, false
	}
	appID, err := a.chain.Get(ctx, "meta.app_id")
	if err != nil || appID == "" {
		return meta.AppCredentials{}, false
	}
	appSecret, err := a.chain.Get(ctx, "meta.app_secret")
	if err != nil || appSecret == "" {
		return meta.AppCredentials{}, false
	}
	return meta.AppCredentials{AppID: appID, AppSecret: appSecret}, true
}

// metaAppCredentials resolves the operator's Meta App ID/Secret, failing the
// request with META_APP_NOT_CONFIGURED when Settings has none saved yet —
// the one guard every Meta-channel handler that talks to the Graph API (or
// verifies a webhook signature) starts with.
func (s *Server) metaAppCredentials(c *gin.Context) (meta.AppCredentials, bool) {
	creds, okCreds := s.metaCreds.MetaAppCredentials(ctx(c))
	if !okCreds {
		fail(c, http.StatusPreconditionFailed, ErrMetaAppNotConfigured,
			"Meta App ID/Secret не настроены — добавьте их в Настройках")
		return meta.AppCredentials{}, false
	}
	return creds, true
}

// metaFail maps a Graph API call's error to an HTTP response: a decodable
// APIError becomes its own readable message; anything else (transport,
// timeout) still answers with the same errcode so the frontend has one
// branch to handle either way.
func metaFail(c *gin.Context, err error) {
	var ae *meta.APIError
	if errors.As(err, &ae) {
		fail(c, http.StatusBadGateway, ErrMetaError, ae.Error())
		return
	}
	fail(c, http.StatusBadGateway, ErrMetaError, err.Error())
}

// orgChannelAccount resolves a generic channel-core account id under the
// caller's org — orgAccount/orgTelegramAccount's analogue for accounts on
// channel_accounts rather than wa_*/tg_*.
func (s *Server) orgChannelAccount(c *gin.Context) (store.ChannelAccount, bool) {
	id, okID := parseUUID(c, "id")
	if !okID {
		return store.ChannelAccount{}, false
	}
	org, okOrg := s.orgOf(c)
	if !okOrg {
		return store.ChannelAccount{}, false
	}
	acct, err := s.store.ChannelAccountByIDForOrg(ctx(c), id, org.ID)
	if err != nil {
		fail(c, http.StatusNotFound, ErrNotFound, "account not found")
		return store.ChannelAccount{}, false
	}
	return acct, true
}

// channelAccountPayload re-reads a channel-core account through the neutral
// view so the response is byte-identical to a row from GET /accounts — see
// telegramAccountPayload's identical shape.
func (s *Server) channelAccountPayload(c *gin.Context, id uuid.UUID) gin.H {
	if acct, err := s.store.AccountByID(ctx(c), id); err == nil {
		return gin.H{"account": dto.MapNeutralAccount(acct, ""), "connection_state": acct.ConnectionState}
	}
	return gin.H{"account": gin.H{"id": id.String()}, "connection_state": ""}
}
