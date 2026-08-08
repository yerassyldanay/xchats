// Package meta is the shared Meta platform layer behind Instagram Direct,
// Facebook Messenger and the WhatsApp Cloud API — the telegram/client.go
// analogue, generalized across three products that speak close enough to
// the same Graph-API-style HTTP, OAuth, webhook-signature and
// webhook-verification shape that duplicating it three times would be pure
// waste. Provider vocabulary that is NOT shared (message send bodies, media
// models, account discovery specifics) stays out of this package — see
// backend/internal/messengerish (Instagram + Messenger) and
// backend/internal/whatsappcloud.
package meta

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client is the low-level Graph API HTTP transport. It carries no base host
// of its own — Instagram's login/token hosts (api.instagram.com,
// graph.instagram.com) differ from Facebook's (www.facebook.com,
// graph.facebook.com), so every call site supplies a full URL (GraphURL/
// InstagramGraphURL build the two Graph hosts; the OAuth dialog/token hosts
// in oauth.go are literals). Client owns the shared http.Client, JSON
// envelope decoding, and token redaction on error paths.
type Client struct {
	version string
	hc      *http.Client
	log     *slog.Logger
}

// NewHTTP returns a Client. version is the "/vXX.X/" path segment GraphURL/
// InstagramGraphURL build every Graph API URL from (see
// config.Config.MetaResolvedGraphAPIVersion — never hardcode a version
// string at a call site).
func NewHTTP(version string, log *slog.Logger) *Client {
	return &Client{
		version: version,
		hc:      &http.Client{Timeout: 20 * time.Second},
		log:     log,
	}
}

// Version returns the configured Graph API version segment ("v21.0").
func (c *Client) Version() string { return c.version }

// GraphURL builds https://graph.facebook.com/<version>/<path> — the host
// Messenger and WhatsApp Cloud's own Graph calls (Page/WABA/phone-number
// discovery, subscribed_apps, message sends) use.
func (c *Client) GraphURL(path string) string {
	return "https://graph.facebook.com/" + c.version + "/" + strings.TrimPrefix(path, "/")
}

// InstagramGraphURL builds https://graph.instagram.com/<version>/<path> —
// Instagram-with-Instagram-Login's own Graph host for message sends and
// account discovery, distinct from graph.facebook.com.
func (c *Client) InstagramGraphURL(path string) string {
	return "https://graph.instagram.com/" + c.version + "/" + strings.TrimPrefix(path, "/")
}

// Get performs a GET and decodes the response into out (nil discards a
// successful body). token, when non-empty, is sent as a Bearer Authorization
// header — Graph API's documented preferred form over an access_token query
// parameter, since a header never ends up in a URL, access log, or
// *url.Error text. Calls that must authenticate via a query parameter
// instead (DebugToken, the OAuth token/refresh endpoints) pass token="" and
// build the parameter into rawURL themselves; redact() still guards those.
func (c *Client) Get(ctx context.Context, rawURL, token string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return c.do(req, token, out)
}

// GetBytes performs a GET and returns the raw response body plus its
// Content-Type, never JSON-decoded — for fetching provider-hosted media (a
// CDN URL Instagram/Messenger hand back directly in a webhook payload, or a
// WhatsApp Cloud media URL resolved via its own media-id lookup).
func (c *Client) GetBytes(ctx context.Context, rawURL, token string) ([]byte, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, "", err
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, "", redactErr(err, token)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", redactErr(err, token)
	}
	if resp.StatusCode >= 400 {
		if ae := decodeAPIError(body); ae != nil {
			return nil, "", ae
		}
		return nil, "", fmt.Errorf("meta: GET %s: status %d", redact(rawURL, token), resp.StatusCode)
	}
	return body, resp.Header.Get("Content-Type"), nil
}

// PostForm performs a POST with an application/x-www-form-urlencoded body
// (Instagram's short-lived code exchange, and a handful of provider
// endpoints that require form encoding over JSON) and decodes the JSON
// response into out.
func (c *Client) PostForm(ctx context.Context, rawURL string, form url.Values, token string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rawURL, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return c.do(req, token, out)
}

// PostJSON performs a POST with a JSON body (every message-send call, and
// subscribed_apps' override registration) and decodes the JSON response
// into out.
func (c *Client) PostJSON(ctx context.Context, rawURL string, body any, token string, out any) error {
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rawURL, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return c.do(req, token, out)
}

func (c *Client) do(req *http.Request, token string, out any) error {
	resp, err := c.hc.Do(req)
	if err != nil {
		return redactErr(err, token)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return redactErr(err, token)
	}
	// Checked before the status code: Graph API errors are documented as
	// always non-2xx, but treating the envelope itself as authoritative is
	// more robust than trusting status-code hygiene on every one of a dozen
	// endpoints across three products, and a genuine success body never
	// happens to parse as {"error": {...}}.
	if ae := decodeAPIError(body); ae != nil {
		return ae
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("meta: %s %s: status %d: %s",
			req.Method, redact(req.URL.String(), token), resp.StatusCode, redact(string(body), token))
	}
	if out == nil || len(body) == 0 {
		return nil
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("meta: decode %s %s response: %w", req.Method, redact(req.URL.String(), token), err)
	}
	return nil
}

type errorEnvelope struct {
	Error *struct {
		Message      string `json:"message"`
		Type         string `json:"type"`
		Code         int    `json:"code"`
		ErrorSubcode int    `json:"error_subcode"`
		FBTraceID    string `json:"fbtrace_id"`
	} `json:"error"`
}

func decodeAPIError(body []byte) *APIError {
	var env errorEnvelope
	if err := json.Unmarshal(body, &env); err != nil || env.Error == nil {
		return nil
	}
	return &APIError{
		Message: env.Error.Message, Type: env.Error.Type, Code: env.Error.Code,
		ErrorSubcode: env.Error.ErrorSubcode, FBTraceID: env.Error.FBTraceID,
	}
}

// redactedError formats its cause's error text with the token redacted,
// while preserving the cause ONE level down via Unwrap — mirrors
// internal/telegram's own redactedError, so errors.Is(err, context.Canceled)
// and similar checks against the underlying transport error still work
// after redaction.
type redactedError struct {
	text  string
	cause error
}

func (e *redactedError) Error() string { return e.text }
func (e *redactedError) Unwrap() error { return e.cause }

func redactErr(err error, token string) error {
	if err == nil || token == "" {
		return err
	}
	return &redactedError{text: redact(err.Error(), token), cause: err}
}

// redactAll applies redactErr for every non-empty secret in secrets, in
// order — the multi-secret case (DebugToken's input_token AND access_token
// query parameters, ExchangeInstagramLongLived's client_secret AND
// access_token) a single call cannot cover. Each hop preserves Unwrap, so
// errors.Is against the original cause still works through the whole chain.
func redactAll(err error, secrets ...string) error {
	for _, s := range secrets {
		err = redactErr(err, s)
	}
	return err
}
