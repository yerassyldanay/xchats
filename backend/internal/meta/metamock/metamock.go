// Package metamock is an in-memory meta.API implementation for cmd/xchats'
// mock-externals mode (see backend/internal/config's SystemConfig.
// MockExternals). It never dials a network: every method returns a
// realistic canned value directly, and the four generic transport methods
// (Get/Delete/PostForm/PostJSON) — the ones whatsappcloud.Client and
// messengerish.Client call internally — are served by a small path-based
// dispatcher that recognizes the handful of Graph endpoints those two
// packages actually hit, so their own real Send/Profile/Subscribe/...
// business logic keeps running completely unmodified on top of this fake
// transport (see meta.API's own doc comment for the full reasoning).
package metamock

import (
	"context"
	"encoding/json"
	"net/url"
	"strings"
	"sync"

	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/internal/meta"
)

// maxRecordedCalls bounds Client.Calls() so a long profiling run's heap
// doesn't grow with the mock's own bookkeeping — a fake that grows without
// bound would itself distort the very heap profile the harness exists to
// capture.
const maxRecordedCalls = 200

// mockPNG is a minimal valid 1x1 transparent PNG — used wherever a real
// caller might decode media bytes as an image (e.g. the response engine's
// vision path), so "small valid payload" is not just a claim.
var mockPNG = []byte{
	0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
	0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, 0x08, 0x06, 0x00, 0x00, 0x00, 0x1f, 0x15, 0xc4,
	0x89, 0x00, 0x00, 0x00, 0x0a, 0x49, 0x44, 0x41, 0x54, 0x78, 0x9c, 0x63, 0x00, 0x01, 0x00, 0x00,
	0x05, 0x00, 0x01, 0x0d, 0x0a, 0x2d, 0xb4, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4e, 0x44, 0xae,
	0x42, 0x60, 0x82,
}

// Fixed, realistic-looking identities every mock response reuses — the
// point is plausible shape, not entropy, so these stay constant rather than
// randomized per Client.
const (
	mockAppID  = "999900000000001"
	mockWABAID = "999900000000002"
	mockPageID = "999900000000003"
	mockIGUser = "17841400000000004"
)

// Call records one dispatched request for test/introspection use — Method
// is the meta.API method name (or the HTTP verb for the four generic
// transport methods), URL is the request URL where one exists.
type Call struct {
	Method string
	URL    string
}

// Client is an in-memory meta.API. Zero value is ready to use.
type Client struct {
	mu sync.Mutex
	// calls is the bounded call history — see maxRecordedCalls.
	calls []Call
	// overrides is the WhatsApp Cloud subscribed_apps override state,
	// keyed by WABA id — SetOverride/ClearOverride write it, GetOverride
	// (and the generic GET dispatch behind whatsappcloud.Client.GetOverride)
	// reads it back, so "subscription operations update their observable
	// state" holds for a mock, not just the real Graph API.
	overrides map[string]string
}

// New returns a ready-to-use in-memory meta.API.
func New() *Client {
	return &Client{overrides: map[string]string{}}
}

var _ meta.API = (*Client)(nil)

func (c *Client) record(method, url string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls = append(c.calls, Call{Method: method, URL: url})
	if over := len(c.calls) - maxRecordedCalls; over > 0 {
		c.calls = c.calls[over:]
	}
}

// Calls returns a snapshot of every recorded call, oldest first.
func (c *Client) Calls() []Call {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]Call, len(c.calls))
	copy(out, c.calls)
	return out
}

// --- pure/local (no dispatch needed) --------------------------------------

func (c *Client) Version() string { return "v21.0-mock" }

func (c *Client) GraphURL(path string) string {
	return "https://graph.mock.internal/" + c.Version() + "/" + strings.TrimPrefix(path, "/")
}

func (c *Client) InstagramGraphURL(path string) string {
	return "https://graph.instagram.mock.internal/" + c.Version() + "/" + strings.TrimPrefix(path, "/")
}

func (c *Client) FacebookAuthorizeURL(appID, redirectURI, state string, scopes []string) string {
	v := url.Values{}
	v.Set("client_id", appID)
	v.Set("redirect_uri", redirectURI)
	v.Set("state", state)
	v.Set("scope", strings.Join(scopes, ","))
	return "https://www.facebook.com/mock/dialog/oauth?" + v.Encode()
}

func (c *Client) EmbeddedSignupAuthorizeURL(appID, configID, redirectURI, state, extrasJSON string) string {
	v := url.Values{}
	v.Set("client_id", appID)
	v.Set("config_id", configID)
	v.Set("redirect_uri", redirectURI)
	v.Set("state", state)
	return "https://www.facebook.com/mock/dialog/oauth?" + v.Encode()
}

// --- direct business methods (bypass the generic dispatcher entirely) ----

func (c *Client) ExchangeInstagramCode(ctx context.Context, instagramAppID, instagramAppSecret, redirectURI, code string) (meta.InstagramTokenResult, error) {
	c.record("ExchangeInstagramCode", redirectURI)
	return meta.InstagramTokenResult{AccessToken: "mock-ig-short-" + uuid.NewString(), UserID: mockIGUser}, nil
}

func (c *Client) ExchangeInstagramLongLived(ctx context.Context, instagramAppSecret, shortLivedToken string) (meta.InstagramLongLivedResult, error) {
	c.record("ExchangeInstagramLongLived", "")
	return meta.InstagramLongLivedResult{AccessToken: "mock-ig-long-" + uuid.NewString(), TokenType: "bearer", ExpiresIn: 5184000}, nil
}

func (c *Client) RefreshInstagramLongLived(ctx context.Context, token string) (meta.InstagramLongLivedResult, error) {
	c.record("RefreshInstagramLongLived", "")
	return meta.InstagramLongLivedResult{AccessToken: "mock-ig-refreshed-" + uuid.NewString(), TokenType: "bearer", ExpiresIn: 5184000}, nil
}

func (c *Client) ExchangeFacebookCode(ctx context.Context, appID, appSecret, redirectURI, code string) (meta.FacebookTokenResult, error) {
	c.record("ExchangeFacebookCode", redirectURI)
	return meta.FacebookTokenResult{AccessToken: "mock-fb-short-" + uuid.NewString(), TokenType: "bearer", ExpiresIn: 3600}, nil
}

func (c *Client) ExchangeFacebookLongLived(ctx context.Context, appID, appSecret, shortLivedToken string) (meta.FacebookTokenResult, error) {
	c.record("ExchangeFacebookLongLived", "")
	return meta.FacebookTokenResult{AccessToken: "mock-fb-long-" + uuid.NewString(), TokenType: "bearer", ExpiresIn: 5184000}, nil
}

func (c *Client) PageAccounts(ctx context.Context, userToken string) ([]meta.PageAccount, error) {
	c.record("PageAccounts", "")
	return []meta.PageAccount{{ID: mockPageID, Name: "Mock Page", AccessToken: "mock-page-token-" + uuid.NewString()}}, nil
}

func (c *Client) DebugToken(ctx context.Context, inputToken, appToken string) (meta.DebugTokenResult, error) {
	c.record("DebugToken", "")
	return meta.DebugTokenResult{
		AppID:   mockAppID,
		IsValid: true,
		Scopes:  []string{"whatsapp_business_management", "pages_messaging", "instagram_basic"},
		GranularScopes: []struct {
			Scope     string   `json:"scope"`
			TargetIDs []string `json:"target_ids"`
		}{{Scope: "whatsapp_business_management", TargetIDs: []string{mockWABAID}}},
	}, nil
}

// --- generic transport, dispatched by URL/body shape ----------------------

func (c *Client) Get(ctx context.Context, rawURL, token string, out any) error {
	return c.dispatch("GET", rawURL, nil, out)
}

func (c *Client) Delete(ctx context.Context, rawURL, token string, out any) error {
	return c.dispatch("DELETE", rawURL, nil, out)
}

func (c *Client) PostForm(ctx context.Context, rawURL string, form url.Values, token string, out any) error {
	return c.dispatch("POST", rawURL, nil, out)
}

func (c *Client) PostJSON(ctx context.Context, rawURL string, body any, token string, out any) error {
	return c.dispatch("POST", rawURL, body, out)
}

// GetBytes is whatsappcloud.Client.DownloadMedia's transport and Instagram/
// Messenger's own CDN-URL media fetch — always answers with the same small
// valid PNG regardless of URL.
func (c *Client) GetBytes(ctx context.Context, rawURL, token string) ([]byte, string, error) {
	c.record("GETBYTES", rawURL)
	return mockPNG, "image/png", nil
}

// dispatch serves the handful of Graph endpoints whatsappcloud.Client and
// messengerish.Client build via GraphURL/InstagramGraphURL — recognized by
// the URL's last path segment(s), since every one of them uses a distinct,
// stable suffix except "/messages" (shared by both channels, disambiguated
// by request body shape in dispatchSend) and a bare id (shared by
// WhatsApp Cloud's MediaInfo and Instagram/Messenger's Profile, disambiguated
// by the presence of a `fields` query parameter, which only Profile/Me set).
func (c *Client) dispatch(method, rawURL string, body, out any) error {
	c.record(method, rawURL)
	u, err := url.Parse(rawURL)
	if err != nil {
		return err
	}
	path := strings.Trim(u.Path, "/")
	segs := strings.Split(path, "/")
	last := segs[len(segs)-1]

	switch {
	case last == "phone_numbers":
		return writeJSON(out, map[string]any{"data": []map[string]any{{
			"id": mockWABAID + "01", "display_phone_number": "+1 555 0100",
			"verified_name": "Mock Business", "quality_rating": "GREEN",
		}}})

	case last == "register":
		return writeJSON(out, map[string]any{"success": true})

	case last == "subscribed_apps":
		wabaID := ""
		if len(segs) >= 2 {
			wabaID = segs[len(segs)-2]
		}
		if method == "GET" {
			c.mu.Lock()
			uri := c.overrides[wabaID]
			c.mu.Unlock()
			if uri == "" {
				return writeJSON(out, map[string]any{"data": []map[string]any{}})
			}
			return writeJSON(out, map[string]any{"data": []map[string]any{{"override_callback_uri": uri}}})
		}
		if method == "POST" {
			// SetOverride(non-empty)/ClearOverride(empty) both PostJSON a
			// whatsappcloud.SubscribedAppsOverride; messengerish's Subscribe
			// PostForms with a nil body instead — only the former carries an
			// override_callback_uri key to record.
			if raw, err := json.Marshal(body); err == nil {
				var probe struct {
					CallbackURI string `json:"override_callback_uri"`
				}
				if json.Unmarshal(raw, &probe) == nil {
					c.mu.Lock()
					c.overrides[wabaID] = probe.CallbackURI
					c.mu.Unlock()
				}
			}
		}
		// POST (Set/ClearOverride, Subscribe) and DELETE (Unsubscribe) all
		// answer with the same generic Graph success envelope.
		return writeJSON(out, map[string]any{"success": true})

	case last == "messages":
		return c.dispatchSend(out, body)

	case last == "me":
		return writeJSON(out, mockIdentityBlob())

	default:
		// A bare id: Profile/Me set "?fields=..."; MediaInfo never does.
		if strings.Contains(u.RawQuery, "fields=") {
			return writeJSON(out, mockIdentityBlob())
		}
		return writeJSON(out, map[string]any{
			"id": last, "url": "https://media.mock.internal/" + last,
			"mime_type": "image/png", "sha256": strings.Repeat("0", 64), "file_size": len(mockPNG),
		})
	}
}

// mockIdentityBlob answers BOTH messengerish.Profile ({name,username}) and
// messengerish.Me ({user_id,username,name}) — each struct only reads the
// fields it declares, so one blob satisfies both shapes.
func mockIdentityBlob() map[string]any {
	return map[string]any{"user_id": mockIGUser, "username": "mock_user", "name": "Mock Contact"}
}

// dispatchSend answers POST .../messages for either channel — WhatsApp
// Cloud's sendBody always carries "messaging_product"; messengerish's always
// carries a "recipient" object — disambiguated by marshaling body (the
// caller's own private struct type) back to JSON and probing its keys,
// since neither struct is visible from this package.
func (c *Client) dispatchSend(out, body any) error {
	raw, err := json.Marshal(body)
	if err != nil {
		return err
	}
	var probe struct {
		MessagingProduct string `json:"messaging_product"`
		Recipient        struct {
			ID string `json:"id"`
		} `json:"recipient"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil {
		return err
	}
	if probe.MessagingProduct != "" {
		return writeJSON(out, map[string]any{"messages": []map[string]any{{"id": "wamid.mock-" + uuid.NewString()}}})
	}
	recipientID := probe.Recipient.ID
	if recipientID == "" {
		recipientID = "mock-recipient"
	}
	return writeJSON(out, map[string]any{"recipient_id": recipientID, "message_id": "mock-msg-" + uuid.NewString()})
}

// writeJSON round-trips v through JSON into out — out's concrete type is
// private to whichever package called in (whatsappcloud/messengerish's own
// unexported response structs), so this is the only way to populate it
// generically without this package importing either.
func writeJSON(out, v any) error {
	if out == nil {
		return nil
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, out)
}
