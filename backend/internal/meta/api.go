package meta

import (
	"context"
	"net/url"
)

// API is Client's complete public surface as an interface — the seam the
// hermetic profiling/load harness (cmd/xchats' mock-externals mode) swaps
// for backend/internal/meta/metamock.Client, and otherwise satisfied by the
// real *Client with zero changes (var _ API = (*Client)(nil) below). Every
// consumer that used to hold a concrete *Client (whatsappcloud.Client.http,
// messengerish.Client.http, worker.Worker.MetaClient, httpapi's own
// metaClient field) now holds this interface instead, so whatsappcloud/
// messengerish's own real Send/Profile/Subscribe/... logic keeps running
// unmodified against either a real Client or a mock one — only the actual
// network transport underneath ever differs.
type API interface {
	Version() string
	GraphURL(path string) string
	InstagramGraphURL(path string) string
	Get(ctx context.Context, rawURL, token string, out any) error
	Delete(ctx context.Context, rawURL, token string, out any) error
	GetBytes(ctx context.Context, rawURL, token string) ([]byte, string, error)
	PostForm(ctx context.Context, rawURL string, form url.Values, token string, out any) error
	PostJSON(ctx context.Context, rawURL string, body any, token string, out any) error

	ExchangeInstagramCode(ctx context.Context, instagramAppID, instagramAppSecret, redirectURI, code string) (InstagramTokenResult, error)
	ExchangeInstagramLongLived(ctx context.Context, instagramAppSecret, shortLivedToken string) (InstagramLongLivedResult, error)
	RefreshInstagramLongLived(ctx context.Context, token string) (InstagramLongLivedResult, error)
	FacebookAuthorizeURL(appID, redirectURI, state string, scopes []string) string
	EmbeddedSignupAuthorizeURL(appID, configID, redirectURI, state, extrasJSON string) string
	ExchangeFacebookCode(ctx context.Context, appID, appSecret, redirectURI, code string) (FacebookTokenResult, error)
	ExchangeFacebookLongLived(ctx context.Context, appID, appSecret, shortLivedToken string) (FacebookTokenResult, error)
	PageAccounts(ctx context.Context, userToken string) ([]PageAccount, error)
	DebugToken(ctx context.Context, inputToken, appToken string) (DebugTokenResult, error)
}

var _ API = (*Client)(nil)
