// Package evolution is the REST client for the Evolution WhatsApp gateway,
// tested against evolution-api v2.3.7.
package evolution

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

// SendResult is the normalized outcome of a send call. KeyID is response.key.id —
// stored as evolution_message_id so messages.update can correlate delivery/read.
type SendResult struct {
	KeyID  string
	Status string
}

// Instance is a row from fetchInstances.
type Instance struct {
	Name             string
	OwnerJID         string
	ID               string
	ConnectionStatus string
	PhoneNumber      string
	CreatedAt        time.Time // for the stale-instance maintenance rule
}

// QR is a connect response: a base64 PNG (data URI) the UI renders, the raw
// pairing code, and the textual QR payload. QR is fetched live, never stored.
type QR struct {
	Code        string // textual QR payload ("2@…")
	Base64      string // data:image/png;base64,… for <img src>
	PairingCode string
}

// WebhookEvents are the event names we subscribe to, in the UPPER_SNAKE form
// Evolution v2.3.7 requires (the dotted lowercase form returns HTTP 400 on /webhook/set).
var WebhookEvents = []string{
	"MESSAGES_UPSERT", "MESSAGES_UPDATE", "SEND_MESSAGE",
	"CONNECTION_UPDATE", "CONTACTS_UPDATE", "CHATS_UPSERT",
}

// Client is the Evolution port. Workers, boot, and the accounts manager depend on
// this interface so tests can substitute a fake. Every per-instance call takes the
// instance name explicitly (multi-account); the HTTP client falls back to its
// configured default instance when the argument is empty.
type Client interface {
	SendText(ctx context.Context, instance, number, text string) (SendResult, error)
	SendMedia(ctx context.Context, instance, number, mediatype, mimetype, base64Data, fileName, caption string) (SendResult, error)
	GetBase64(ctx context.Context, instance, messageID string) (b64, fileName, mimetype string, err error)
	FetchInstances(ctx context.Context) ([]Instance, error)
	SetWebhook(ctx context.Context, instance, url, tokenHeader, token string, events []string) error
	OnWhatsApp(ctx context.Context, instance, number string) (bool, error)

	// --- lifecycle (accounts manager) ---
	CreateInstance(ctx context.Context, instance, displayName string) error
	ConnectQR(ctx context.Context, instance string) (QR, error)
	ConnectionState(ctx context.Context, instance string) (string, error)
	DeleteInstance(ctx context.Context, instance string) error
	LogoutInstance(ctx context.Context, instance string) error
}

// HTTP is the real Evolution REST client.
type HTTP struct {
	base     string
	apiKey   string
	instance string
	hc       *http.Client
	log      *slog.Logger
}

// NewHTTP builds a client against base with the shared apikey and a default
// instance name (used when a per-call instance argument is empty). log may be nil
// (falls back to slog.Default); every REST call is logged at debug, non-2xx at warn.
func NewHTTP(base, apiKey, instance string, log *slog.Logger) *HTTP {
	if log == nil {
		log = slog.Default()
	}
	return &HTTP{
		base:     base,
		apiKey:   apiKey,
		instance: instance,
		hc:       &http.Client{Timeout: 60 * time.Second},
		log:      log,
	}
}

// instOr returns the per-call instance, defaulting to the configured one.
func (c *HTTP) instOr(instance string) string {
	if instance == "" {
		return c.instance
	}
	return instance
}

func (c *HTTP) do(ctx context.Context, method, path string, body any, out any) error {
	var rdr io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, rdr)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("apikey", c.apiKey)
	start := time.Now()
	resp, err := c.hc.Do(req)
	if err != nil {
		// Transport failure (DNS, refused, timeout) — the gateway is unreachable.
		c.log.Warn("evolution request error", "method", method, "path", path,
			"latency_ms", time.Since(start).Milliseconds(), "err", err)
		return err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	c.log.Debug("evolution request", "method", method, "path", path,
		"status", resp.StatusCode, "latency_ms", time.Since(start).Milliseconds(),
		"resp", truncate(raw, 300))
	if resp.StatusCode >= 300 {
		c.log.Warn("evolution non-2xx", "method", method, "path", path,
			"status", resp.StatusCode, "resp", truncate(raw, 300))
		return fmt.Errorf("evolution %s %s: http %d: %s", method, path, resp.StatusCode, truncate(raw, 300))
	}
	if out != nil && len(raw) > 0 {
		return json.Unmarshal(raw, out)
	}
	return nil
}

type sendResp struct {
	Key struct {
		ID string `json:"id"`
	} `json:"key"`
	Status string `json:"status"`
}

func (c *HTTP) SendText(ctx context.Context, instance, number, text string) (SendResult, error) {
	var r sendResp
	err := c.do(ctx, http.MethodPost, "/message/sendText/"+c.instOr(instance),
		map[string]any{"number": number, "text": text}, &r)
	return SendResult{KeyID: r.Key.ID, Status: r.Status}, err
}

func (c *HTTP) SendMedia(ctx context.Context, instance, number, mediatype, mimetype, base64Data, fileName, caption string) (SendResult, error) {
	body := map[string]any{
		"number":    number,
		"mediatype": mediatype,
		"mimetype":  mimetype,
		"media":     base64Data,
		"fileName":  fileName,
	}
	if caption != "" {
		body["caption"] = caption
	}
	var r sendResp
	err := c.do(ctx, http.MethodPost, "/message/sendMedia/"+c.instOr(instance), body, &r)
	return SendResult{KeyID: r.Key.ID, Status: r.Status}, err
}

func (c *HTTP) GetBase64(ctx context.Context, instance, messageID string) (string, string, string, error) {
	var r struct {
		Base64   string `json:"base64"`
		FileName string `json:"fileName"`
		Mimetype string `json:"mimetype"`
	}
	body := map[string]any{
		"message":      map[string]any{"key": map[string]any{"id": messageID}},
		"convertToMp4": false,
	}
	err := c.do(ctx, http.MethodPost, "/chat/getBase64FromMediaMessage/"+c.instOr(instance), body, &r)
	return r.Base64, r.FileName, r.Mimetype, err
}

func (c *HTTP) FetchInstances(ctx context.Context) ([]Instance, error) {
	var raw json.RawMessage
	if err := c.do(ctx, http.MethodGet, "/instance/fetchInstances", nil, &raw); err != nil {
		return nil, err
	}
	return parseInstances(raw), nil
}

func (c *HTTP) SetWebhook(ctx context.Context, instance, url, tokenHeader, token string, events []string) error {
	body := map[string]any{
		"webhook": map[string]any{
			"enabled":         true,
			"url":             url,
			"webhookByEvents": false,
			"webhookBase64":   true,
			"headers":         map[string]string{tokenHeader: token},
			"events":          events,
		},
	}
	return c.do(ctx, http.MethodPost, "/webhook/set/"+c.instOr(instance), body, nil)
}

// OnWhatsApp reports whether a phone number is a registered WhatsApp account,
// via POST /chat/whatsappNumbers/{instance}. Used to give the composer immediate
// feedback instead of a silent failed send.
func (c *HTTP) OnWhatsApp(ctx context.Context, instance, number string) (bool, error) {
	var out []struct {
		JID    string `json:"jid"`
		Exists bool   `json:"exists"`
		Number string `json:"number"`
	}
	if err := c.do(ctx, http.MethodPost, "/chat/whatsappNumbers/"+c.instOr(instance),
		map[string]any{"numbers": []string{number}}, &out); err != nil {
		return false, err
	}
	for _, r := range out {
		return r.Exists, nil
	}
	return false, nil
}

// CreateInstance provisions a new Baileys instance with QR pairing and live-only
// sync (no history backfill — matches the live-only ingest model).
func (c *HTTP) CreateInstance(ctx context.Context, instance, displayName string) error {
	return c.do(ctx, http.MethodPost, "/instance/create", map[string]any{
		"instanceName":    instance,
		"integration":     "WHATSAPP-BAILEYS",
		"qrcode":          true,
		"syncFullHistory": false,
	}, nil)
}

// ConnectQR returns the current QR (base64 PNG + pairing code) for an instance
// that is not yet connected. It is fetched live and never persisted.
func (c *HTTP) ConnectQR(ctx context.Context, instance string) (QR, error) {
	var r struct {
		Code        string `json:"code"`
		Base64      string `json:"base64"`
		PairingCode string `json:"pairingCode"`
		// already-connected shape
		Instance struct {
			State string `json:"state"`
		} `json:"instance"`
	}
	if err := c.do(ctx, http.MethodGet, "/instance/connect/"+c.instOr(instance), nil, &r); err != nil {
		return QR{}, err
	}
	return QR{Code: r.Code, Base64: r.Base64, PairingCode: r.PairingCode}, nil
}

// ConnectionState returns the raw Evolution connection state for an instance
// ("open" | "connecting" | "close"). Tolerates the flat and nested shapes.
func (c *HTTP) ConnectionState(ctx context.Context, instance string) (string, error) {
	var r struct {
		State    string `json:"state"`
		Instance struct {
			State string `json:"state"`
		} `json:"instance"`
	}
	if err := c.do(ctx, http.MethodGet, "/instance/connectionState/"+c.instOr(instance), nil, &r); err != nil {
		return "", err
	}
	if r.State != "" {
		return r.State, nil
	}
	return r.Instance.State, nil
}

// DeleteInstance removes an instance from Evolution entirely.
func (c *HTTP) DeleteInstance(ctx context.Context, instance string) error {
	return c.do(ctx, http.MethodDelete, "/instance/delete/"+c.instOr(instance), nil, nil)
}

// LogoutInstance ends the WhatsApp session (precedes delete on a clean).
func (c *HTTP) LogoutInstance(ctx context.Context, instance string) error {
	return c.do(ctx, http.MethodDelete, "/instance/logout/"+c.instOr(instance), nil, nil)
}

// parseInstances tolerates the v2.x flat shape and the older nested shape.
func parseInstances(raw json.RawMessage) []Instance {
	var flat []struct {
		ID               string `json:"id"`
		Name             string `json:"name"`
		ConnectionStatus string `json:"connectionStatus"`
		OwnerJid         string `json:"ownerJid"`
		Number           string `json:"number"`
		CreatedAt        string `json:"createdAt"`
		Instance         struct {
			InstanceName string `json:"instanceName"`
			InstanceID   string `json:"instanceId"`
			Owner        string `json:"owner"`
			Status       string `json:"status"`
		} `json:"instance"`
	}
	if err := json.Unmarshal(raw, &flat); err != nil {
		return nil
	}
	out := make([]Instance, 0, len(flat))
	for _, f := range flat {
		inst := Instance{
			ID:               f.ID,
			Name:             f.Name,
			ConnectionStatus: f.ConnectionStatus,
			OwnerJID:         f.OwnerJid,
			PhoneNumber:      f.Number,
			CreatedAt:        parseTime(f.CreatedAt),
		}
		if inst.Name == "" {
			inst.Name = f.Instance.InstanceName
		}
		if inst.OwnerJID == "" {
			inst.OwnerJID = f.Instance.Owner
		}
		if inst.ID == "" {
			inst.ID = f.Instance.InstanceID
		}
		if inst.ConnectionStatus == "" {
			inst.ConnectionStatus = f.Instance.Status
		}
		out = append(out, inst)
	}
	return out
}

// parseTime tolerates the common Evolution timestamp encodings; a zero time means
// "unknown" (the stale rule treats unknown as not-stale).
func parseTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02T15:04:05.000Z"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC()
		}
	}
	return time.Time{}
}

func truncate(b []byte, n int) string {
	if len(b) > n {
		return string(b[:n])
	}
	return string(b)
}
