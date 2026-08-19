package meta

import (
	"context"
	"net/url"
	"strings"
)

// InstagramAuthorizeURL builds the "Instagram API with Instagram Login"
// authorization dialog URL — no Facebook Page required. No Graph API
// version segment: this host's authorize path is unversioned.
// instagramAppID is the Instagram App ID from the Meta Dashboard's
// "Instagram > API setup with Instagram login" panel — NOT the Meta
// Developer App ID Messenger/WhatsApp Cloud use (see
// meta.InstagramAppCredentials' own doc comment); passing the wrong one is
// exactly what makes instagram.com answer "Sorry, this page isn't
// available."
func InstagramAuthorizeURL(instagramAppID, redirectURI, state string) string {
	v := url.Values{}
	v.Set("client_id", instagramAppID)
	v.Set("redirect_uri", redirectURI)
	v.Set("response_type", "code")
	v.Set("scope", "instagram_business_basic,instagram_business_manage_messages")
	v.Set("state", state)
	return "https://www.instagram.com/oauth/authorize?" + v.Encode()
}

// InstagramTokenResult is the short-lived code exchange's response shape.
type InstagramTokenResult struct {
	AccessToken string `json:"access_token"`
	UserID      string `json:"user_id"`
}

// ExchangeInstagramCode trades an authorization code for a SHORT-LIVED
// (1 hour) token. The code itself is single-use and short-lived — call this
// synchronously from the OAuth callback handler, never deferred.
// instagramAppID/instagramAppSecret are the Instagram app pair — see
// InstagramAuthorizeURL's own doc comment on why this must never be the
// Meta Developer App pair.
func (c *Client) ExchangeInstagramCode(ctx context.Context, instagramAppID, instagramAppSecret, redirectURI, code string) (InstagramTokenResult, error) {
	form := url.Values{}
	form.Set("client_id", instagramAppID)
	form.Set("client_secret", instagramAppSecret)
	form.Set("grant_type", "authorization_code")
	form.Set("redirect_uri", redirectURI)
	form.Set("code", code)
	var out InstagramTokenResult
	err := c.PostForm(ctx, c.instagramAPIHostBase()+"/oauth/access_token", form, "", &out)
	return out, err
}

// InstagramLongLivedResult is the short->long token exchange's (and the
// refresh call's) response shape.
type InstagramLongLivedResult struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"` // seconds; ~60 days
}

// ExchangeInstagramLongLived trades a short-lived (1h) token for a
// long-lived (60 day) one. instagramAppSecret is the Instagram app pair's
// secret — see InstagramAuthorizeURL's own doc comment.
func (c *Client) ExchangeInstagramLongLived(ctx context.Context, instagramAppSecret, shortLivedToken string) (InstagramLongLivedResult, error) {
	v := url.Values{}
	v.Set("grant_type", "ig_exchange_token")
	v.Set("client_secret", instagramAppSecret)
	v.Set("access_token", shortLivedToken)
	var out InstagramLongLivedResult
	// Both secrets ride the URL's query string, not a header, so Get's own
	// token-redaction (which only ever scrubs what IT was told is a bearer
	// token) cannot see them — redact both explicitly from whatever error
	// comes back.
	err := redactAll(c.Get(ctx, c.instagramGraphHostBase()+"/access_token?"+v.Encode(), "", &out), instagramAppSecret, shortLivedToken)
	return out, err
}

// RefreshInstagramLongLived extends a long-lived token by another 60 days.
// token must already be at least 24 hours old, or Meta rejects the call —
// see worker/meta_tokens.go's refresh-eligibility window.
func (c *Client) RefreshInstagramLongLived(ctx context.Context, token string) (InstagramLongLivedResult, error) {
	v := url.Values{}
	v.Set("grant_type", "ig_refresh_token")
	v.Set("access_token", token)
	var out InstagramLongLivedResult
	err := redactAll(c.Get(ctx, c.instagramGraphHostBase()+"/refresh_access_token?"+v.Encode(), "", &out), token)
	return out, err
}

// FacebookAuthorizeURL builds the plain Facebook Login dialog URL —
// Messenger's connect flow: Page access is granted via scopes, no Embedded
// Signup configuration involved.
func (c *Client) FacebookAuthorizeURL(appID, redirectURI, state string, scopes []string) string {
	v := url.Values{}
	v.Set("client_id", appID)
	v.Set("redirect_uri", redirectURI)
	v.Set("response_type", "code")
	v.Set("scope", strings.Join(scopes, ","))
	v.Set("state", state)
	return "https://www.facebook.com/" + c.version + "/dialog/oauth?" + v.Encode()
}

// EmbeddedSignupAuthorizeURL builds WhatsApp Cloud's Embedded Signup URL —
// Facebook Login for Business: permissions come from the pre-created
// configID, not a scope list; override_default_response_type asks for a
// code (rather than a token) redirect. extrasJSON is the raw `extras` query
// value (e.g. `{"setup":{},"featureType":"","sessionInfoVersion":"3"}`,
// optionally with a pre-filled "setup" business/phone block) — building it
// is the WhatsApp Cloud channel's own concern, not this shared package's.
func (c *Client) EmbeddedSignupAuthorizeURL(appID, configID, redirectURI, state, extrasJSON string) string {
	v := url.Values{}
	v.Set("client_id", appID)
	v.Set("config_id", configID)
	v.Set("response_type", "code")
	v.Set("override_default_response_type", "true")
	v.Set("redirect_uri", redirectURI)
	v.Set("state", state)
	if extrasJSON != "" {
		v.Set("extras", extrasJSON)
	}
	return "https://www.facebook.com/" + c.version + "/dialog/oauth?" + v.Encode()
}

// FacebookTokenResult is the Facebook code-exchange response shape, shared
// by both Messenger's plain login and WhatsApp Cloud's Embedded Signup —
// same endpoint, same shape.
type FacebookTokenResult struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

// ExchangeFacebookCode trades an authorization code for a token. For
// WhatsApp Cloud's Embedded Signup the code expires in 30 SECONDS — call
// this synchronously from the OAuth callback handler, never deferred to a
// background job.
func (c *Client) ExchangeFacebookCode(ctx context.Context, appID, appSecret, redirectURI, code string) (FacebookTokenResult, error) {
	v := url.Values{}
	v.Set("client_id", appID)
	v.Set("client_secret", appSecret)
	v.Set("redirect_uri", redirectURI)
	v.Set("code", code)
	var out FacebookTokenResult
	err := redactAll(c.Get(ctx, c.GraphURL("oauth/access_token")+"?"+v.Encode(), "", &out), appSecret, code)
	return out, err
}

// ExchangeFacebookLongLived trades a short-lived Facebook user token for a
// long-lived (~60 day) one. Messenger's connect flow calls this before
// PageAccounts: a Page token minted from a long-lived User token does not
// itself expire, whereas one minted straight from the short-lived code-
// exchange token would inherit that short lifetime — doing this exchange
// first is what lets Messenger get away with no background token refresher
// at all (contrast Instagram Login's long-lived USER token, which DOES
// expire on its own 60-day clock and needs worker/meta_tokens.go's active
// renewal).
func (c *Client) ExchangeFacebookLongLived(ctx context.Context, appID, appSecret, shortLivedToken string) (FacebookTokenResult, error) {
	v := url.Values{}
	v.Set("grant_type", "fb_exchange_token")
	v.Set("client_id", appID)
	v.Set("client_secret", appSecret)
	v.Set("fb_exchange_token", shortLivedToken)
	var out FacebookTokenResult
	err := redactAll(c.Get(ctx, c.GraphURL("oauth/access_token")+"?"+v.Encode(), "", &out), appSecret, shortLivedToken)
	return out, err
}

// PageAccount is one Facebook Page a user token can act as.
type PageAccount struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	AccessToken string `json:"access_token"` // the PAGE token — the actual sending credential
}

// PageAccounts lists the Pages a user token has a role on (GET /me/accounts)
// — Messenger's page-selection step, after the plain Facebook Login
// exchange.
func (c *Client) PageAccounts(ctx context.Context, userToken string) ([]PageAccount, error) {
	var out struct {
		Data []PageAccount `json:"data"`
	}
	err := c.Get(ctx, c.GraphURL("me/accounts"), userToken, &out)
	return out.Data, err
}

// DebugTokenResult is /debug_token's response.data shape, trimmed to the
// fields this codebase reads: granular_scopes for WABA discovery,
// is_valid/scopes for the Settings credential Test button.
type DebugTokenResult struct {
	AppID          string   `json:"app_id"`
	IsValid        bool     `json:"is_valid"`
	ExpiresAt      int64    `json:"expires_at"`
	Scopes         []string `json:"scopes"`
	GranularScopes []struct {
		Scope     string   `json:"scope"`
		TargetIDs []string `json:"target_ids"`
	} `json:"granular_scopes"`
}

// DebugToken inspects inputToken using appToken (AppCredentials.AppToken())
// as the CALLING credential. Two uses: (1) WhatsApp Cloud's WABA-discovery
// step — see WABAIDsFromDebugToken; (2) called with the app token as BOTH
// the input and the caller, a zero-side-effect proof that an App ID/Secret
// pair entered in Settings is real (an invalid secret makes the app token
// itself malformed, and Meta rejects the call outright) — the Settings
// "meta" credential provider's Validate hook.
func (c *Client) DebugToken(ctx context.Context, inputToken, appToken string) (DebugTokenResult, error) {
	v := url.Values{}
	v.Set("input_token", inputToken)
	v.Set("access_token", appToken)
	var out struct {
		Data DebugTokenResult `json:"data"`
	}
	err := redactAll(c.Get(ctx, c.GraphURL("debug_token")+"?"+v.Encode(), "", &out), inputToken, appToken)
	return out.Data, err
}

// WABAIDsFromDebugToken extracts the WhatsApp Business Account ids a
// business token is authorized to manage from DebugToken's own result:
// granular_scopes[] where scope=="whatsapp_business_management" ->
// target_ids[].
func WABAIDsFromDebugToken(d DebugTokenResult) []string {
	for _, gs := range d.GranularScopes {
		if gs.Scope == "whatsapp_business_management" {
			return gs.TargetIDs
		}
	}
	return nil
}
