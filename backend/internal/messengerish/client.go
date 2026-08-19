package messengerish

import (
	"context"

	"github.com/yerassyldanay/xchats/backend/internal/meta"
)

// Client is Instagram Direct's and Messenger's shared Graph operations,
// layered on the shared meta.Client transport — see whatsappcloud.Client's
// identical shape. graphURL selects WHICH Graph host a call resolves
// against: Instagram Login's own graph.instagram.com for Instagram, plain
// graph.facebook.com for Messenger — passed in per-call rather than baked
// into the Client, since one process may run both channels at once with the
// SAME underlying meta.Client.
type Client struct {
	http *meta.Client
}

// NewClient wraps an already-constructed meta.Client.
func NewClient(http *meta.Client) *Client {
	return &Client{http: http}
}

// graphURL resolves accountID's own Graph host: instagram.com's Login
// product routes every call through graph.instagram.com; Messenger (Page-
// token-authenticated) uses the plain graph.facebook.com host. Callers pass
// their channel explicitly rather than this package guessing from shape.
func (c *Client) graphURL(instagramHost bool, path string) string {
	if instagramHost {
		return c.http.InstagramGraphURL(path)
	}
	return c.http.GraphURL(path)
}

// Profile is a connected participant's (an IGSID or a PSID) display
// identity — Instagram/Messenger include no contact info in the webhook
// payload itself (unlike WhatsApp Cloud's inline `contacts[]`), so this is
// a live lookup the ingest path makes for a never-seen-before contact.
type Profile struct {
	Name     string `json:"name"`
	Username string `json:"username,omitempty"` // Instagram only
}

// Profile resolves userID's display identity, authenticated as accountID
// (the connected IG account / Page whose token this is).
func (c *Client) Profile(ctx context.Context, instagramHost bool, userID, token string) (Profile, error) {
	fields := "name"
	if instagramHost {
		fields = "name,username"
	}
	var out Profile
	err := c.http.Get(ctx, c.graphURL(instagramHost, userID)+"?fields="+fields, token, &out)
	return out, err
}

// Me resolves the connected account's OWN identity right after OAuth —
// Instagram Login's equivalent of WhatsApp Cloud's phone-number discovery,
// except there is exactly one account per token (no picker step needed).
type Me struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	Name     string `json:"name"`
}

func (c *Client) Me(ctx context.Context, instagramHost bool, token string) (Me, error) {
	var out Me
	err := c.http.Get(ctx, c.graphURL(instagramHost, "me")+"?fields=user_id,username,name", token, &out)
	return out, err
}

// Subscribe subscribes accountID to the "messages" webhook field — the
// per-account opt-in Instagram Login and Messenger both require after
// connect (distinct from the App-level webhook product subscription, done
// once in the Meta Dashboard by the operator).
func (c *Client) Subscribe(ctx context.Context, instagramHost bool, accountID, token string) error {
	var out map[string]any
	return c.http.PostForm(ctx, c.graphURL(instagramHost, accountID+"/subscribed_apps")+"?subscribed_fields=messages", nil, token, &out)
}

// Unsubscribe removes accountID's webhook subscription — the disconnect-
// time counterpart to Subscribe.
func (c *Client) Unsubscribe(ctx context.Context, instagramHost bool, accountID, token string) error {
	var out map[string]any
	return c.http.Delete(ctx, c.graphURL(instagramHost, accountID+"/subscribed_apps"), token, &out)
}

// sendBody is POST /{account-id}/messages' request shape — one struct
// covers text and media; only the field matching the send actually sent.
type sendBody struct {
	Recipient     party       `json:"recipient"`
	Message       sendMessage `json:"message"`
	MessagingType string      `json:"messaging_type"`
}

type party struct {
	ID string `json:"id"`
}

type sendMessage struct {
	Text       string          `json:"text,omitempty"`
	Attachment *sendAttachment `json:"attachment,omitempty"`
}

type sendAttachment struct {
	Type    string `json:"type"`
	Payload struct {
		URL        string `json:"url"`
		IsReusable bool   `json:"is_reusable"`
	} `json:"payload"`
}

// SendResult is the Graph API's response to a successful send.
type SendResult struct {
	RecipientID string `json:"recipient_id"`
	MessageID   string `json:"message_id"`
}

// SendText sends a free-form text message. "RESPONSE" is the only
// messaging_type this codebase uses — every send here is a reply inside the
// provider's own service window, never a proactive/subscription message.
func (c *Client) SendText(ctx context.Context, instagramHost bool, accountID, recipientID, text, token string) (SendResult, error) {
	body := sendBody{Recipient: party{ID: recipientID}, Message: sendMessage{Text: text}, MessagingType: "RESPONSE"}
	var out SendResult
	err := c.http.PostJSON(ctx, c.graphURL(instagramHost, accountID+"/messages"), body, token, &out)
	return out, err
}

// MediaKind is the closed set of attachment kinds SendMedia accepts.
type MediaKind string

const (
	MediaImage MediaKind = "image"
	MediaVideo MediaKind = "video"
	MediaAudio MediaKind = "audio"
	MediaFile  MediaKind = "file"
)

// SendMedia sends an attachment by public URL — Meta fetches the bytes
// itself, exactly like WhatsApp Cloud's own link-based send (see
// internal/inboxmedia). is_reusable is always false: this codebase never
// re-sends the same attachment_id across conversations.
func (c *Client) SendMedia(ctx context.Context, instagramHost bool, accountID, recipientID string, kind MediaKind, url, token string) (SendResult, error) {
	att := &sendAttachment{Type: string(kind)}
	att.Payload.URL = url
	body := sendBody{Recipient: party{ID: recipientID}, Message: sendMessage{Attachment: att}, MessagingType: "RESPONSE"}
	var out SendResult
	err := c.http.PostJSON(ctx, c.graphURL(instagramHost, accountID+"/messages"), body, token, &out)
	return out, err
}
