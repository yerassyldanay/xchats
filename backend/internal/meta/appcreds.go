package meta

import "context"

// AppCredentials is one operator's Meta Developer App identity — the
// BYO-App model's whole point: App ID + Secret are entered once in Settings
// (internal/credentials' "meta" provider) and never leave this install.
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

// Source resolves the operator's configured Meta app credentials. Satisfied
// by internal/credentials.Chain in production; a channel adapter depends on
// this interface rather than a concrete credentials type so it can be
// exercised against a fake in tests without pulling in the credentials
// package's file-store/keyring machinery.
type Source interface {
	MetaAppCredentials(ctx context.Context) (AppCredentials, bool)
}
