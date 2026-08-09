// Package whatsappcloud is the WhatsApp Cloud API channel adapter — the
// telegram/client.go analogue for this specific channel, built on the
// shared backend/internal/meta transport. Provider vocabulary (WABA ids,
// phone_number_id, wamid) stops here — everything above it sees the
// channel-neutral contracts in backend/messaging and the generic
// channel_* tables (see backend/internal/store/channels.go).
package whatsappcloud

import (
	"context"

	"github.com/yerassyldanay/xchats/backend/internal/meta"
)

// Client is WhatsApp Cloud API's own Graph operations, layered on the
// shared meta.Client transport.
type Client struct {
	http *meta.Client
}

// NewClient wraps an already-constructed meta.Client.
func NewClient(http *meta.Client) *Client {
	return &Client{http: http}
}

// PhoneNumber is one WABA-registered number.
type PhoneNumber struct {
	ID                 string `json:"id"`
	DisplayPhoneNumber string `json:"display_phone_number"`
	VerifiedName       string `json:"verified_name"`
	QualityRating      string `json:"quality_rating"`
}

// PhoneNumbers lists a WABA's registered numbers (GET
// /<WABA_ID>/phone_numbers) — the number-picker step after WABA discovery
// (see internal/meta.WABAIDsFromDebugToken).
func (c *Client) PhoneNumbers(ctx context.Context, wabaID, token string) ([]PhoneNumber, error) {
	var out struct {
		Data []PhoneNumber `json:"data"`
	}
	err := c.http.Get(ctx, c.http.GraphURL(wabaID+"/phone_numbers"), token, &out)
	return out.Data, err
}

// registerBody is POST /<PHONE_NUMBER_ID>/register's request body.
type registerBody struct {
	MessagingProduct string `json:"messaging_product"`
	PIN              string `json:"pin"`
}

// Register activates a phone number for Cloud API sending with its 6-digit
// two-step-verification PIN. Meta allows at most 10 attempts per 72 hours
// per number — see internal/httpapi's registration handler for the
// user-facing warning this backs.
func (c *Client) Register(ctx context.Context, phoneNumberID, pin, token string) error {
	body := registerBody{MessagingProduct: "whatsapp", PIN: pin}
	var out struct {
		Success bool `json:"success"`
	}
	return c.http.PostJSON(ctx, c.http.GraphURL(phoneNumberID+"/register"), body, token, &out)
}

// SubscribedAppsOverride is one WABA's per-account webhook override — the
// self-healing mechanism: re-pushed via API whenever the public origin
// changes, never a manual Dashboard edit. Override precedence is phone
// number > WABA > app, so setting this at the WABA level is what makes
// THIS install's callback URL win over any shared app-level default.
type SubscribedAppsOverride struct {
	CallbackURI string `json:"override_callback_uri"`
	VerifyToken string `json:"verify_token"`
}

// SetOverride registers (or replaces) the WABA's per-account webhook
// override AND subscribes the app to the WABA's webhook events — WhatsApp
// Cloud does both in the one call, unlike Instagram/Messenger's plain
// subscribed_apps (see internal/messengerish).
func (c *Client) SetOverride(ctx context.Context, wabaID string, override SubscribedAppsOverride, token string) error {
	var out map[string]any
	return c.http.PostJSON(ctx, c.http.GraphURL(wabaID+"/subscribed_apps"), override, token, &out)
}

type subscribedApp struct {
	OverrideCallbackURI string `json:"override_callback_uri"`
}

// GetOverride reads back the WABA's currently registered callback override
// — the health-check / stale-tunnel-origin detection read (see §4: compare
// against config.MetaResolvedPublicBaseURL() at boot and on every tunnel
// start).
func (c *Client) GetOverride(ctx context.Context, wabaID, token string) (callbackURI string, err error) {
	var out struct {
		Data []subscribedApp `json:"data"`
	}
	if err := c.http.Get(ctx, c.http.GraphURL(wabaID+"/subscribed_apps"), token, &out); err != nil {
		return "", err
	}
	if len(out.Data) == 0 {
		return "", nil
	}
	return out.Data[0].OverrideCallbackURI, nil
}

// ClearOverride removes the WABA's per-account override by POSTing an
// empty one, per the API reference — falls back to the WABA/app-level
// default, used when disconnecting an account.
func (c *Client) ClearOverride(ctx context.Context, wabaID, token string) error {
	var out map[string]any
	return c.http.PostJSON(ctx, c.http.GraphURL(wabaID+"/subscribed_apps"), SubscribedAppsOverride{}, token, &out)
}

// sendBody is POST /<PHONE_NUMBER_ID>/messages' request shape — one struct
// covers text and every media kind; only the field matching Type is set.
type sendBody struct {
	MessagingProduct string     `json:"messaging_product"`
	RecipientType    string     `json:"recipient_type"`
	To               string     `json:"to"`
	Type             string     `json:"type"`
	Text             *sendText  `json:"text,omitempty"`
	Image            *sendMedia `json:"image,omitempty"`
	Video            *sendMedia `json:"video,omitempty"`
	Audio            *sendMedia `json:"audio,omitempty"`
	Document         *sendMedia `json:"document,omitempty"`
}

type sendText struct {
	Body string `json:"body"`
}

// sendMedia sends by public Link — the same org-scoped signed-URL mechanism
// Instagram/Messenger use (internal/inboxmedia), reused here rather than
// building a second, upload-based media path solely for this channel.
type sendMedia struct {
	Link     string `json:"link,omitempty"`
	Caption  string `json:"caption,omitempty"`
	Filename string `json:"filename,omitempty"` // documents only
}

// SendResult is the Cloud API's response to a successful send.
type SendResult struct {
	Messages []struct {
		ID string `json:"id"` // wamid.…
	} `json:"messages"`
}

// SendText sends a free-form text message. Callers must check the 24-hour
// service window themselves before calling this (see
// messaging.ErrOutsideServiceWindow) — Meta enforces it server-side too,
// but failing closer to the operator gives a clearer error.
func (c *Client) SendText(ctx context.Context, phoneNumberID, to, text, token string) (SendResult, error) {
	body := sendBody{MessagingProduct: "whatsapp", RecipientType: "individual", To: to, Type: "text", Text: &sendText{Body: text}}
	var out SendResult
	err := c.http.PostJSON(ctx, c.http.GraphURL(phoneNumberID+"/messages"), body, token, &out)
	return out, err
}

// MediaKind is the closed set of attachment kinds SendMedia accepts —
// exactly WhatsApp Cloud's own vocabulary, which xchats' internal media
// kinds (image|video|audio|document) already match one-for-one.
type MediaKind string

const (
	MediaImage    MediaKind = "image"
	MediaVideo    MediaKind = "video"
	MediaAudio    MediaKind = "audio"
	MediaDocument MediaKind = "document"
)

// SendMedia sends an attachment by public URL (link), with an optional
// caption/filename.
func (c *Client) SendMedia(ctx context.Context, phoneNumberID, to string, kind MediaKind, link, caption, filename, token string) (SendResult, error) {
	body := sendBody{MessagingProduct: "whatsapp", RecipientType: "individual", To: to, Type: string(kind)}
	m := &sendMedia{Link: link, Caption: caption}
	if kind == MediaDocument {
		m.Filename = filename
	}
	switch kind {
	case MediaImage:
		body.Image = m
	case MediaVideo:
		body.Video = m
	case MediaAudio:
		body.Audio = m
	case MediaDocument:
		body.Document = m
	}
	var out SendResult
	err := c.http.PostJSON(ctx, c.http.GraphURL(phoneNumberID+"/messages"), body, token, &out)
	return out, err
}

// MediaInfo is GET /<media_id>'s response: a short-lived download URL plus
// descriptive metadata. The URL alone is not fetchable without the SAME
// bearer token — see DownloadMedia.
type MediaInfo struct {
	URL      string `json:"url"`
	MimeType string `json:"mime_type"`
	SHA256   string `json:"sha256"`
	FileSize int64  `json:"file_size"`
	ID       string `json:"id"`
}

// MediaInfo resolves an inbound attachment's media id to its short-lived
// download URL — the first of WhatsApp Cloud's two-step media fetch (§10):
// the webhook payload carries only an id, never a URL directly.
func (c *Client) MediaInfo(ctx context.Context, mediaID, token string) (MediaInfo, error) {
	var out MediaInfo
	err := c.http.Get(ctx, c.http.GraphURL(mediaID), token, &out)
	return out, err
}

// DownloadMedia fetches the actual bytes from a MediaInfo.URL — the SAME
// bearer token is required on this second GET too; the URL 401s without it.
func (c *Client) DownloadMedia(ctx context.Context, mediaURL, token string) ([]byte, string, error) {
	return c.http.GetBytes(ctx, mediaURL, token)
}
