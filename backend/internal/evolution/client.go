// Package evolution is the REST client for the Evolution WhatsApp gateway,
// ported from plan/scripts/evolution_client.py (tested vs evolution-api v2.3.7).
package evolution

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
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
}

// Client is the Evolution port. Workers and boot depend on this interface so
// tests can substitute a fake.
type Client interface {
	SendText(ctx context.Context, number, text string) (SendResult, error)
	SendMedia(ctx context.Context, number, mediatype, mimetype, base64Data, fileName, caption string) (SendResult, error)
	GetBase64(ctx context.Context, messageID string) (b64, fileName, mimetype string, err error)
	FetchInstances(ctx context.Context) ([]Instance, error)
	SetWebhook(ctx context.Context, url, tokenHeader, token string, events []string) error
	OnWhatsApp(ctx context.Context, number string) (bool, error)
}

// HTTP is the real Evolution REST client.
type HTTP struct {
	base     string
	apiKey   string
	instance string
	hc       *http.Client
}

// NewHTTP builds a client against base with the shared apikey and instance name.
func NewHTTP(base, apiKey, instance string) *HTTP {
	return &HTTP{
		base:     base,
		apiKey:   apiKey,
		instance: instance,
		hc:       &http.Client{Timeout: 60 * time.Second},
	}
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
	resp, err := c.hc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return fmt.Errorf("evolution %s %s: http %d: %s", method, path, resp.StatusCode, truncate(raw, 300))
	}
	if out != nil && len(raw) > 0 {
		return json.Unmarshal(raw, out)
	}
	return nil
}

type sendResp struct {
	Key    struct{ ID string `json:"id"` } `json:"key"`
	Status string                          `json:"status"`
}

func (c *HTTP) SendText(ctx context.Context, number, text string) (SendResult, error) {
	var r sendResp
	err := c.do(ctx, http.MethodPost, "/message/sendText/"+c.instance,
		map[string]any{"number": number, "text": text}, &r)
	return SendResult{KeyID: r.Key.ID, Status: r.Status}, err
}

func (c *HTTP) SendMedia(ctx context.Context, number, mediatype, mimetype, base64Data, fileName, caption string) (SendResult, error) {
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
	err := c.do(ctx, http.MethodPost, "/message/sendMedia/"+c.instance, body, &r)
	return SendResult{KeyID: r.Key.ID, Status: r.Status}, err
}

func (c *HTTP) GetBase64(ctx context.Context, messageID string) (string, string, string, error) {
	var r struct {
		Base64   string `json:"base64"`
		FileName string `json:"fileName"`
		Mimetype string `json:"mimetype"`
	}
	body := map[string]any{
		"message":      map[string]any{"key": map[string]any{"id": messageID}},
		"convertToMp4": false,
	}
	err := c.do(ctx, http.MethodPost, "/chat/getBase64FromMediaMessage/"+c.instance, body, &r)
	return r.Base64, r.FileName, r.Mimetype, err
}

func (c *HTTP) FetchInstances(ctx context.Context) ([]Instance, error) {
	var raw json.RawMessage
	if err := c.do(ctx, http.MethodGet, "/instance/fetchInstances", nil, &raw); err != nil {
		return nil, err
	}
	return parseInstances(raw), nil
}

func (c *HTTP) SetWebhook(ctx context.Context, url, tokenHeader, token string, events []string) error {
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
	return c.do(ctx, http.MethodPost, "/webhook/set/"+c.instance, body, nil)
}

// OnWhatsApp reports whether a phone number is a registered WhatsApp account,
// via POST /chat/whatsappNumbers/{instance}. Used to give the composer immediate
// feedback instead of a silent failed send.
func (c *HTTP) OnWhatsApp(ctx context.Context, number string) (bool, error) {
	var out []struct {
		JID    string `json:"jid"`
		Exists bool   `json:"exists"`
		Number string `json:"number"`
	}
	if err := c.do(ctx, http.MethodPost, "/chat/whatsappNumbers/"+c.instance,
		map[string]any{"numbers": []string{number}}, &out); err != nil {
		return false, err
	}
	for _, r := range out {
		return r.Exists, nil
	}
	return false, nil
}

// parseInstances tolerates the v2.x flat shape and the older nested shape.
func parseInstances(raw json.RawMessage) []Instance {
	var flat []struct {
		ID               string `json:"id"`
		Name             string `json:"name"`
		ConnectionStatus string `json:"connectionStatus"`
		OwnerJid         string `json:"ownerJid"`
		Number           string `json:"number"`
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

func truncate(b []byte, n int) string {
	if len(b) > n {
		return string(b[:n])
	}
	return string(b)
}
