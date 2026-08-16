package meta

import "context"

// AppCredentials is one operator's Meta Developer App identity — the
// BYO-App model's whole point: App ID + Secret are entered once in Settings
// (internal/credentials' "meta" provider) and never leave this install.
// Messenger's plain Facebook Login and WhatsApp Cloud's Embedded Signup both
// authenticate as THIS app; Instagram Login does not — see
// InstagramAppCredentials below.
type AppCredentials struct {
	AppID     string
	AppSecret string
}

// AppToken is Meta's own "app access token" shorthand — <app_id>|<app_secret>
// — accepted anywhere an unauthenticated, app-level call needs a token (for
// example DebugToken's own access_token parameter, which authenticates the
// CALLER, not the token being inspected).
func (c AppCredentials) AppToken() string {
	return c.AppID + "|" + c.AppSecret
}

// InstagramAppCredentials is the Instagram App ID/Secret issued by the Meta
// Dashboard's "Instagram > API setup with Instagram login" panel — a
// SEPARATE identity from AppCredentials above. Business Login for Instagram
// (InstagramAuthorizeURL, ExchangeInstagramCode, ExchangeInstagramLongLived)
// rejects the Meta Developer App id outright: per Meta's Instagram Platform
// docs, "Apps that use Business Login for Instagram will use the Instagram
// app ID displayed on the Instagram > API setup with Instagram login section
// of the dashboard" — not "the Meta app ID displayed at the top of the Meta
// App Dashboard". Deliberately no AppToken() method: Meta's <id>|<secret>
// app-access-token shorthand is a graph.facebook.com convention that no
// Instagram host accepts.
type InstagramAppCredentials struct {
	AppID     string
	AppSecret string
}

// Source resolves the operator's configured Meta and Instagram app
// credentials. Satisfied by internal/credentials.Chain in production; a
// channel adapter depends on this interface rather than a concrete
// credentials type so it can be exercised against a fake in tests without
// pulling in the credentials package's file-store/keyring machinery.
type Source interface {
	MetaAppCredentials(ctx context.Context) (AppCredentials, bool)
	InstagramAppCredentials(ctx context.Context) (InstagramAppCredentials, bool)
}
